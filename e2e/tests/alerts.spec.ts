import { test, expect, type APIRequestContext } from '@playwright/test';

// E2E tests for GitHub-style alert/admonition block rendering.
// These tests verify that the alert transformer (PR #943) correctly
// renders [!NOTE], [!TIP], [!IMPORTANT], [!WARNING], and [!CAUTION]
// blockquotes as styled <div class="markdown-alert ..."> elements.

const TEST_PAGE = 'e2e_alerts_test';

const PAGE_LOAD_TIMEOUT_MS = 15000;
const SAVE_TIMEOUT_MS = 10000;

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

test.describe('GitHub-style Alert Rendering', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test.afterAll(async ({ request }) => {
    await callPageAPI(request, 'DeletePage', { pageName: TEST_PAGE });
  });

  test.describe('NOTE alert', () => {
    test.beforeEach(async ({ page, request }) => {
      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

> [!NOTE]
> Useful information that users should know.`);

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should render as a markdown-alert-note div', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert.markdown-alert-note')).toBeAttached();
    });

    test('should have role="note" for accessibility', async ({ page }) => {
      await expect(page.locator('#rendered div[role="note"]')).toBeAttached();
    });

    test('should display the "Note" label', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert-title')).toContainText('Note');
    });

    test('should render the alert content', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert-note')).toContainText('Useful information that users should know.');
    });

    test('should not render as a plain blockquote', async ({ page }) => {
      await expect(page.locator('#rendered blockquote')).not.toBeAttached();
    });
  });

  test.describe('TIP alert', () => {
    test.beforeEach(async ({ page, request }) => {
      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

> [!TIP]
> Helpful advice for doing things better.`);

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should render as a markdown-alert-tip div', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert.markdown-alert-tip')).toBeAttached();
    });

    test('should display the "Tip" label', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert-title')).toContainText('Tip');
    });
  });

  test.describe('IMPORTANT alert', () => {
    test.beforeEach(async ({ page, request }) => {
      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

> [!IMPORTANT]
> Key information users need to know.`);

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should render as a markdown-alert-important div', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert.markdown-alert-important')).toBeAttached();
    });

    test('should display the "Important" label', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert-title')).toContainText('Important');
    });
  });

  test.describe('WARNING alert', () => {
    test.beforeEach(async ({ page, request }) => {
      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

> [!WARNING]
> Urgent info that needs immediate attention.`);

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should render as a markdown-alert-warning div', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert.markdown-alert-warning')).toBeAttached();
    });

    test('should display the "Warning" label', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert-title')).toContainText('Warning');
    });
  });

  test.describe('CAUTION alert', () => {
    test.beforeEach(async ({ page, request }) => {
      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

> [!CAUTION]
> Advises about risks or negative outcomes.`);

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should render as a markdown-alert-caution div', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert.markdown-alert-caution')).toBeAttached();
    });

    test('should display the "Caution" label', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert-title')).toContainText('Caution');
    });
  });

  test.describe('case-insensitive marker', () => {
    test.beforeEach(async ({ page, request }) => {
      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

> [!note]
> Lowercase marker content.`);

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should render [!note] the same as [!NOTE]', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert.markdown-alert-note')).toBeAttached();
    });

    test('should display the "Note" label for lowercase marker', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert-title')).toContainText('Note');
    });
  });

  test.describe('plain blockquote without alert marker', () => {
    test.beforeEach(async ({ page, request }) => {
      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

> This is a plain blockquote without any alert marker.`);

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should render as a plain blockquote element', async ({ page }) => {
      await expect(page.locator('#rendered blockquote')).toBeAttached();
    });

    test('should not render as a markdown-alert div', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert')).not.toBeAttached();
    });
  });

  test.describe('icon aria-hidden attribute', () => {
    test.beforeEach(async ({ page, request }) => {
      await setupTestPage(request, `+++
identifier = "${TEST_PAGE}"
+++

> [!WARNING]
> This alert has an icon that should be hidden from screen readers.`);

      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should have aria-hidden="true" on the icon span', async ({ page }) => {
      await expect(page.locator('#rendered .markdown-alert-icon[aria-hidden="true"]')).toBeAttached();
    });
  });

  test.describe('wiki-editor save and render', () => {
    const SAVE_PAGE = 'e2e_alerts_editor_test';

    test.afterAll(async ({ request }) => {
      await callPageAPI(request, 'DeletePage', { pageName: SAVE_PAGE });
    });

    test('should render NOTE alert saved via the editor', async ({ page }) => {
      await page.goto(`/${SAVE_PAGE}/edit`);
      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      await textarea.fill(`+++
identifier = "${SAVE_PAGE}"
+++

> [!NOTE]
> Content saved through the editor.`);
      await textarea.press('Space');
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });

      await page.goto(`/${SAVE_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
      await expect(page.locator('#rendered .markdown-alert.markdown-alert-note')).toBeAttached();
      await expect(page.locator('#rendered .markdown-alert-title')).toContainText('Note');
    });
  });
});
