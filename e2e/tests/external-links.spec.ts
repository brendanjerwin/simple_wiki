import { test, expect, type APIRequestContext } from '@playwright/test';

// E2E tests for the external link indicator feature (issue #1141).
// External links (absolute http/https URLs) must render with a ↗ indicator
// inside a <span aria-hidden="true"> so screen readers skip the decorative glyph.
// Internal links (relative paths, anchors, wikilinks) must render unchanged.

const TEST_PAGE = 'e2e_external_links_test';

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
    page: TEST_PAGE,
    contentMarkdown: markdown,
  });
  if (createResp.ok()) {
    const body = await createResp.json() as { success: boolean };
    if (body.success) return;
  }

  const resetResp = await callPageAPI(request, 'UpdatePageContent', {
    page: TEST_PAGE,
    newContentMarkdown: markdown,
    allowShrink: true,
  });
  expect(resetResp.ok()).toBeTruthy();
}

test.describe('External Link Indicator', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test.afterAll(async ({ request }) => {
    await callPageAPI(request, 'DeletePage', { page: TEST_PAGE });
  });

  test.describe('external https link', () => {
    test.beforeEach(async ({ page, request }) => {
      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

[FreeCampsites](https://freecampsites.net/)`);

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should render a ↗ indicator inside the link', async ({ page }) => {
      const indicator = page.locator('#rendered a[href="https://freecampsites.net/"] span[aria-hidden="true"]');
      await expect(indicator).toBeAttached();
      await expect(indicator).toContainText('↗');
    });

    test('should mark the indicator span as aria-hidden', async ({ page }) => {
      const indicator = page.locator('#rendered a span[aria-hidden="true"]');
      await expect(indicator).toBeAttached();
    });

    test('should open the link in a new tab', async ({ page }) => {
      const link = page.locator('#rendered a[href="https://freecampsites.net/"]');
      await expect(link).toHaveAttribute('target', '_blank');
    });
  });

  test.describe('external http link', () => {
    test.beforeEach(async ({ page, request }) => {
      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

[Old Site](http://example.com/page)`);

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should render a ↗ indicator', async ({ page }) => {
      const indicator = page.locator('#rendered a span[aria-hidden="true"]');
      await expect(indicator).toBeAttached();
      await expect(indicator).toContainText('↗');
    });
  });

  test.describe('internal relative link', () => {
    test.beforeEach(async ({ page, request }) => {
      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

[Local Page](/some_page)`);

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should not render a ↗ indicator', async ({ page }) => {
      const indicator = page.locator('#rendered a span[aria-hidden="true"]');
      await expect(indicator).not.toBeAttached();
    });

    test('should not open in a new tab', async ({ page }) => {
      const link = page.locator('#rendered a[href="/some_page"]');
      await expect(link).not.toHaveAttribute('target', '_blank');
    });
  });

  test.describe('anchor link', () => {
    test.beforeEach(async ({ page, request }) => {
      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

[Jump to section](#section)

## Section`);

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should not render a ↗ indicator', async ({ page }) => {
      const indicator = page.locator('#rendered a span[aria-hidden="true"]');
      await expect(indicator).not.toBeAttached();
    });
  });

  test.describe('wikilink', () => {
    test.beforeEach(async ({ page, request }) => {
      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

See also [[help]]`);

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should not render a ↗ indicator', async ({ page }) => {
      const indicator = page.locator('#rendered a span[aria-hidden="true"]');
      await expect(indicator).not.toBeAttached();
    });
  });
});
