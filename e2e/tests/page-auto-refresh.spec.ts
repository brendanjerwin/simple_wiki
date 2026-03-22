import { test, expect, type APIRequestContext } from '@playwright/test';

const TEST_PAGE = 'e2e_auto_refresh_test';
const INITIAL_MARKDOWN = '# Auto Refresh Test\n\nInitial content for auto-refresh testing.';

const COMPONENT_LOAD_TIMEOUT_MS = 15000;

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

test.describe('Page auto-refresh and system-info page status', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test.beforeEach(async ({ request }) => {
    await setupTestPage(request, INITIAL_MARKDOWN);
  });

  test('page-auto-refresh component receives page-name attribute in view mode', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);

    const autoRefresh = page.locator('page-auto-refresh');
    await expect(autoRefresh).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await expect(autoRefresh).toHaveAttribute('page-name', TEST_PAGE);
  });

  test('page-auto-refresh component is not present in edit mode', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/edit`);

    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const autoRefresh = page.locator('page-auto-refresh');
    await expect(autoRefresh).not.toBeAttached();
  });

  test('page content auto-refreshes when updated via API', async ({ page, request }) => {
    await page.goto(`/${TEST_PAGE}/view`);

    await expect(page.locator('#rendered')).toContainText('Initial content for auto-refresh testing', { timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Update the page content via the API (simulating another user/session)
    const updateResp = await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      newContentMarkdown: '# Auto Refresh Test\n\nContent was updated by another session!',
    });
    expect(updateResp.ok()).toBeTruthy();

    // The WatchPage stream checks every 1 second, so allow a few seconds
    await expect(page.locator('#rendered')).toContainText('Content was updated by another session!', { timeout: 10000 });
    await expect(page.locator('#rendered')).not.toContainText('Initial content for auto-refresh testing');
  });

  test('system-info panel shows page saved time after content change', async ({ page, request }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toContainText('Initial content', { timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Trigger a content change so lastRefreshTime gets set
    const updateResp = await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      newContentMarkdown: '# Auto Refresh Test\n\nUpdated for system-info check.',
    });
    expect(updateResp.ok()).toBeTruthy();

    // Wait for auto-refresh to pick up the change
    await expect(page.locator('#rendered')).toContainText('Updated for system-info check', { timeout: 10000 });

    // Expand the system-info panel
    const systemPanel = page.locator('system-info');
    await expect(systemPanel).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await systemPanel.locator('.system-panel').click();

    // The system-info-page component should show "Page saved: Xs ago"
    const pageInfo = systemPanel.locator('system-info-page');
    await expect(pageInfo).toBeAttached({ timeout: 10000 });
    const timeValue = pageInfo.locator('.time');
    await expect(timeValue).toHaveText(/\d+s ago/, { timeout: 10000 });
  });

  test('scroll position is preserved during auto-refresh', async ({ page, request }) => {
    const longContent = '# Long Page\n\n' + Array.from({ length: 50 }, (_, i) => `## Section ${i + 1}\n\nParagraph ${i + 1} with some content to make the page long enough to scroll.\n`).join('\n');

    await setupTestPage(request, longContent);
    await page.goto(`/${TEST_PAGE}/view`);

    await expect(page.locator('#rendered')).toContainText('Section 1', { timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Scroll down significantly
    await page.evaluate(() => window.scrollTo(0, 500));
    const scrollBefore = await page.evaluate(() => window.scrollY);
    expect(scrollBefore).toBeGreaterThan(0);

    // Update content via API
    const updatedContent = longContent.replace('Section 1', 'Updated Section 1');
    await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      newContentMarkdown: updatedContent,
    });

    // Wait for the auto-refresh to pick up the change
    await expect(page.locator('#rendered')).toContainText('Updated Section 1', { timeout: 10000 });

    // Verify scroll position was preserved (within a small tolerance)
    const scrollAfter = await page.evaluate(() => window.scrollY);
    expect(Math.abs(scrollAfter - scrollBefore)).toBeLessThan(50);
  });

  // Cleanup
  test('should clean up test page', async ({ request }) => {
    await callPageAPI(request, 'DeletePage', { pageName: TEST_PAGE });
  });
});
