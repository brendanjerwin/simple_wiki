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

    // The page-auto-refresh component should be present and have the correct page-name
    const autoRefresh = page.locator('page-auto-refresh');
    await expect(autoRefresh).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await expect(autoRefresh).toHaveAttribute('page-name', TEST_PAGE);
  });

  test('page-auto-refresh component is not present in edit mode', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/edit`);

    // Wait for the editor to load
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // page-auto-refresh should NOT be present in edit mode
    const autoRefresh = page.locator('page-auto-refresh');
    await expect(autoRefresh).not.toBeAttached();
  });

  test('system-info panel shows page status when viewing a page', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);

    // Wait for the page-auto-refresh component to be present
    await expect(page.locator('page-auto-refresh')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Expand the system-info panel
    const systemPanel = page.locator('system-info');
    await expect(systemPanel).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Click to expand the panel - the panel is in shadow DOM
    const drawerTab = systemPanel.locator('.system-panel');
    await drawerTab.click();

    // The system-info-page component should show the page name
    // It's nested inside system-info's shadow DOM
    const pageInfo = systemPanel.locator('system-info-page');
    await expect(pageInfo).toBeAttached({ timeout: 10000 });

    // Check that the page name is displayed within the component's shadow DOM
    const pageValue = pageInfo.locator('.page-value');
    await expect(pageValue).toContainText(TEST_PAGE, { timeout: 10000 });
  });

  test('system-info panel shows version hash for the page', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);

    await expect(page.locator('page-auto-refresh')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Expand the system-info panel
    const systemPanel = page.locator('system-info');
    await expect(systemPanel).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await systemPanel.locator('.system-panel').click();

    // The hash should appear after the WatchPage stream sends the first response
    const hashValue = systemPanel.locator('system-info-page .hash');
    await expect(hashValue).toBeAttached({ timeout: 10000 });
    // Hash should be a truncated hex string (8 chars + "...")
    await expect(hashValue).toHaveText(/^[a-f0-9]{8}\.\.\.$/);
  });

  test('page content auto-refreshes when updated via API', async ({ page, request }) => {
    await page.goto(`/${TEST_PAGE}/view`);

    // Verify initial content is displayed
    await expect(page.locator('#rendered')).toContainText('Initial content for auto-refresh testing', { timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Update the page content via the API (simulating another user/session)
    const updateResp = await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      newContentMarkdown: '# Auto Refresh Test\n\nContent was updated by another session!',
    });
    expect(updateResp.ok()).toBeTruthy();

    // The page should auto-refresh and show the new content
    // The WatchPage stream checks every 1 second, so allow a few seconds
    await expect(page.locator('#rendered')).toContainText('Content was updated by another session!', { timeout: 10000 });

    // Original content should no longer be visible
    await expect(page.locator('#rendered')).not.toContainText('Initial content for auto-refresh testing');
  });

  test('scroll position is preserved during auto-refresh', async ({ page, request }) => {
    // Create a page with lots of content so it's scrollable
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
