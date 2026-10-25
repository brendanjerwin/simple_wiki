/**
 * E2E workflow tests for page app clearing (issue #1272).
 *
 * ClearPageApp removes the page app from a page, leaving its prose. The
 * page returns to plain markdown: no host element, GetPageApp not found.
 */
import { expect, test, type APIRequestContext } from '@playwright/test';
import { callAPI, counterProgram, getPageApp, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_clear';

const PAGE_LOAD_TIMEOUT_MS = 15000;

function pageBody(program: string): string {
 return `Keep this prose paragraph after clearing.\n\n\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('Artifact clear', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  await setupPage(request, TEST_PAGE, pageBody(counterProgram('Counter', '$n')));
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when ClearPageApp is called on a page with prose and a page app', () => {
  test('prose stays, the host is gone, and GetPageApp is NotFound', async ({ page, request }) => {
   // Sanity: page app present before the clear.
   const before = await getPageApp(request, TEST_PAGE);
   expect(before.status).toBe(200);

   const clearResp = await callAPI(request, 'PageAppService', 'ClearPageApp', {
    page: TEST_PAGE,
   });
   expect(clearResp.status).toBe(200);

   const after = await getPageApp(request, TEST_PAGE);
   expect(after.status).toBe(404);

   await page.goto(`/${TEST_PAGE}/view`);
   await expect(page.getByText('Keep this prose paragraph after clearing.')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
   await expect(page.locator('wiki-page-app-host')).toHaveCount(0, {
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });
 });

 test.describe('when the page app is already cleared', () => {
  test('ClearPageApp is idempotent', async ({ request }) => {
   // The page app is already gone after the previous test; the fence must
   // also be gone from the body, so re-add it to exercise the clear path.
   const setupResp = await callAPI(
    request,
    'PageManagementService',
    'UpdatePageContent',
    { page: TEST_PAGE, newContentMarkdown: pageBody(counterProgram('Counter', '$n')) },
   );
   expect(setupResp.status).toBe(200);

   const first = await callAPI(request, 'PageAppService', 'ClearPageApp', {
    page: TEST_PAGE,
   });
   expect(first.status, first.text).toBe(200);
   const second = await callAPI(request, 'PageAppService', 'ClearPageApp', {
    page: TEST_PAGE,
   });
   // Idempotent server returns success (200) or NotFound (404) — both mean
   // no page app remains.
   expect([200, 404]).toContain(second.status);
   const after = await getPageApp(request, TEST_PAGE);
   expect(after.status).toBe(404);
  });
 });
});