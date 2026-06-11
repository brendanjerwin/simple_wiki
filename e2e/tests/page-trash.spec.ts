import { test, expect, type APIRequestContext } from '@playwright/test';
import { COMPONENT_LOAD_TIMEOUT_MS } from './constants.js';

const TEST_RUN_SUFFIX = [
  process.env.GITHUB_RUN_ID ?? String(Date.now()).slice(-8),
  process.env.TEST_WORKER_INDEX ?? String(process.pid),
].join('_').replace(/[^A-Za-z0-9_]/g, '');

const TEST_PAGE_PREFIX = 'e2e_trash_restore_';
const TEST_PAGE = `${TEST_PAGE_PREFIX}${TEST_RUN_SUFFIX}`.toLowerCase();
const TEST_TITLE = 'E2E Trash Restore';
const TEST_MARKDOWN = `+++
identifier = "${TEST_PAGE}"
title = "${TEST_TITLE}"
+++

# ${TEST_TITLE}

This page should come back from trash.`;

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

async function createTestPage(request: APIRequestContext): Promise<void> {
  const createResp = await callPageAPI(request, 'CreatePage', {
    pageName: TEST_PAGE,
    contentMarkdown: TEST_MARKDOWN,
  });
  if (createResp.ok()) {
    const body = await createResp.json() as { success: boolean };
    if (body.success) return;
  }

  const updateResp = await callPageAPI(request, 'UpdatePageContent', {
    pageName: TEST_PAGE,
    newContentMarkdown: TEST_MARKDOWN,
  });
  expect(updateResp.ok()).toBeTruthy();
}

async function purgeTrashEntries(request: APIRequestContext, identifierPrefix: string): Promise<void> {
  const trashResp = await callPageAPI(request, 'ListTrash', {});
  if (!trashResp.ok()) return;
  const trashBody = await trashResp.json() as { pages?: Array<{ trashId?: string; identifier?: string }> };
  const entries = trashBody.pages?.filter((page) => page.identifier?.startsWith(identifierPrefix)) ?? [];
  for (const entry of entries) {
    if (entry.trashId) {
      await callPageAPI(request, 'PurgePage', { trashId: entry.trashId });
    }
  }
}

test.describe('Page trash', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test.beforeEach(async ({ request }) => {
    await purgeTrashEntries(request, TEST_PAGE_PREFIX);
    await createTestPage(request);
  });

  test.afterEach(async ({ request }) => {
    await callPageAPI(request, 'DeletePage', { pageName: TEST_PAGE });
    await purgeTrashEntries(request, TEST_PAGE_PREFIX);
  });

  test('restores a deleted page from trash', async ({ page, request }) => {
    const deleteResp = await callPageAPI(request, 'DeletePage', { pageName: TEST_PAGE });
    expect(deleteResp.ok()).toBeTruthy();

    await page.goto('/trash');
    await expect(page.locator('page-trash')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await expect(page.locator('page-trash')).toContainText(TEST_PAGE, { timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const trashRow = page.locator('page-trash tr').filter({ hasText: TEST_PAGE });
    await trashRow.getByRole('button', { name: 'Restore' }).click();
    await expect(page.locator('page-trash')).not.toContainText(TEST_PAGE, { timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toContainText('This page should come back from trash', {
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });
  });
});
