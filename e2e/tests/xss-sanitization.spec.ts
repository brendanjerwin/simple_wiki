import { test, expect, type APIRequestContext } from '@playwright/test';

// E2E tests for XSS sanitization.
// These tests verify that security-critical sanitization introduced in PR #476
// is working correctly and will catch regressions.
//
// Sanitization happens at two layers:
//  1. Go backend: bluemonday.UGCPolicy() sanitizes goldmark HTML output before
//     the page is sent to the browser (protects the main rendered view).
//  2. wiki-table.ts: DOMPurify.sanitize() cleans each HTML table cell before
//     unsafeHTML renders it into the shadow DOM.

const TEST_PAGE = 'e2e_xss_sanitization_test';

const PAGE_LOAD_TIMEOUT_MS = 15000;

async function callPageAPI(
  request: APIRequestContext,
  method: string,
  body: Record<string, unknown>,
) {
  return request.post(`/api.v1.PageManagementService/${method}`, {
    headers: { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1' },
    data: body,
  });
}

async function setupTestPage(request: APIRequestContext, markdown: string): Promise<void> {
  const createResp = await callPageAPI(request, 'CreatePage', {
    pageName: TEST_PAGE,
    contentMarkdown: markdown,
  });
  if (createResp.ok()) {
    const body = await createResp.json() as { success: boolean };
    if (body.success) return;
  }

  const resetResp = await callPageAPI(request, 'UpdatePageContent', {
    pageName: TEST_PAGE,
    newContentMarkdown: markdown,
  });
  expect(resetResp.ok()).toBeTruthy();
}

test.describe('XSS Sanitization', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test.afterAll(async ({ request }) => {
    await callPageAPI(request, 'DeletePage', { pageName: TEST_PAGE });
  });

  test.describe('script injection via page content', () => {
    let dialogTriggered: boolean;

    test.beforeEach(async ({ page, request }) => {
      dialogTriggered = false;

      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

# XSS Script Test

<script>alert('xss-script')</script>

Some safe content below the injection.`);

      page.on('dialog', () => {
        dialogTriggered = true;
      });

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should not execute injected script tags', () => {
      expect(dialogTriggered).toBe(false);
    });

    test('should not render script tags in the DOM', async ({ page }) => {
      const scriptTags = page.locator('#rendered script');
      await expect(scriptTags).toHaveCount(0);
    });

    test('should still render safe content', async ({ page }) => {
      await expect(page.locator('#rendered')).toContainText('Some safe content below the injection');
    });
  });

  test.describe('event handler injection via img onerror', () => {
    let dialogTriggered: boolean;

    test.beforeEach(async ({ page, request }) => {
      dialogTriggered = false;

      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

# XSS Event Handler Test

<img src="nonexistent.png" onerror="alert('xss-event')">

Safe content after the injection.`);

      page.on('dialog', () => {
        dialogTriggered = true;
      });

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

      // Wait for the img element to be present in the DOM, confirming the page has fully
      // rendered and any onerror (if not stripped) would have already fired.
      await expect(page.locator('#rendered img')).toBeAttached();
    });

    test('should not execute injected event handlers', () => {
      expect(dialogTriggered).toBe(false);
    });

    test('should strip onerror attributes from img elements', async ({ page }) => {
      const imgWithOnerror = page.locator('#rendered img[onerror]');
      await expect(imgWithOnerror).toHaveCount(0);
    });

    test('should still render safe content', async ({ page }) => {
      await expect(page.locator('#rendered')).toContainText('Safe content after the injection');
    });
  });

  test.describe('javascript: URL in links', () => {
    let dialogTriggered: boolean;

    test.beforeEach(async ({ page, request }) => {
      dialogTriggered = false;

      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

# XSS URL Test

[Click me](javascript:alert('xss-url'))
[Data link](data:text/html,<script>alert('xss-data')</script>)
[VBScript link](vbscript:alert('xss-vbscript'))

Safe link: [Home](/home/view)`);

      page.on('dialog', () => {
        dialogTriggered = true;
      });

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should not have any dangerous protocol hrefs in the DOM', async ({ page }) => {
      const dangerousHrefs = await page.evaluate(() => {
        const anchors = Array.from(document.querySelectorAll('#rendered a'));
        const dangerousSchemes = ['javascript:', 'data:', 'vbscript:'];
        return anchors
          .map(a => (a as HTMLAnchorElement).getAttribute('href') ?? '')
          .filter(href => {
            const lower = href.toLowerCase();
            return dangerousSchemes.some(scheme => lower.startsWith(scheme));
          });
      });
      expect(dangerousHrefs).toHaveLength(0);
    });

    test('should not execute javascript: protocol links when clicked', async ({ page }) => {
      // Find the "Click me" link
      const link = page.locator('#rendered a', { hasText: 'Click me' });
      const count = await link.count();
      if (count > 0) {
        await link.click();
        expect(dialogTriggered).toBe(false);
      }
      // If the link was removed entirely by sanitization, the test still passes
      // (the absence of the link is also valid sanitization)
    });

    test('should still render safe links', async ({ page }) => {
      const homeLink = page.locator('#rendered a[href="/home/view"]');
      await expect(homeLink).toBeAttached();
    });
  });

  test.describe('wiki-table cell HTML injection', () => {
    let dialogTriggered: boolean;

    test.beforeEach(async ({ page, request }) => {
      dialogTriggered = false;

      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

# Wiki Table XSS Test

| Name | Value |
|------|-------|
| Safe Cell | <script>alert('xss-table-script')</script> |
| Event Cell | <img src="x" onerror="alert('xss-table-event')"> |
| JS Link Cell | <a href="javascript:alert('xss-table-link')">js-click</a> |
| Data Link Cell | <a href="data:text/html,<script>alert('xss-table-data')</script>">data-click</a> |
| VBScript Link Cell | <a href="vbscript:alert('xss-table-vbs')">vbs-click</a> |
| Normal Cell | just text |`);

      page.on('dialog', () => {
        dialogTriggered = true;
      });

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

      // Wait for wiki-table to initialize and render (it hides the original table
      // and renders its own shadow DOM version)
      await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible();
    });

    test('should not execute scripts injected into table cells', () => {
      expect(dialogTriggered).toBe(false);
    });

    test('should not render script tags inside wiki-table', async ({ page }) => {
      // Check both light DOM (original hidden table) and shadow DOM (wiki-table render)
      const scriptTagsInTable = await page.evaluate(() => {
        // Check light DOM
        const lightDomScripts = document.querySelectorAll('#rendered script');
        // Check shadow DOM of wiki-table
        const wikiTable = document.querySelector('wiki-table');
        const shadowScripts = wikiTable?.shadowRoot?.querySelectorAll('script') ?? [];
        return lightDomScripts.length + shadowScripts.length;
      });
      expect(scriptTagsInTable).toBe(0);
    });

    test('should strip onerror attributes from wiki-table cell img elements', async ({ page }) => {
      const imgWithOnerrorCount = await page.evaluate(() => {
        const wikiTable = document.querySelector('wiki-table');
        if (!wikiTable?.shadowRoot) return 0;
        return wikiTable.shadowRoot.querySelectorAll('img[onerror]').length;
      });
      expect(imgWithOnerrorCount).toBe(0);

      // The beforeEach already waited for the table to fully render, so any onerror
      // that wasn't stripped would have already fired by now.
      expect(dialogTriggered).toBe(false);
    });

    test('should strip dangerous protocol hrefs from wiki-table cell links', async ({ page }) => {
      const dangerousHrefCount = await page.evaluate(() => {
        const wikiTable = document.querySelector('wiki-table');
        if (!wikiTable?.shadowRoot) return 0;
        const anchors = Array.from(wikiTable.shadowRoot.querySelectorAll('a'));
        const dangerousSchemes = ['javascript:', 'data:', 'vbscript:'];
        return anchors.filter(a => {
          const href = (a.getAttribute('href') ?? '').toLowerCase();
          return dangerousSchemes.some(scheme => href.startsWith(scheme));
        }).length;
      });
      expect(dangerousHrefCount).toBe(0);
    });

    test('should still render normal cell content in the table', async ({ page }) => {
      // The "Normal Cell" and "just text" content should be visible
      const normalCellContent = await page.evaluate(() => {
        const wikiTable = document.querySelector('wiki-table');
        return wikiTable?.shadowRoot?.textContent ?? '';
      });
      expect(normalCellContent).toContain('just text');
    });
  });
});
