/**
 * E2E workflow tests for page app state persistence (issue #1272).
 *
 * Artifact state lives server-side under wiki.pageApp_state; MergePageAppState
 * CAS's it via state_expected_version_hash. The app hydrates from persisted
 * state on page load.
 */
import { expect, test } from '@playwright/test';
import { callAPI, counterProgram, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_state';

const PAGE_LOAD_TIMEOUT_MS = 15000;

function pageBody(program: string): string {
 return `Some prose.\n\n\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('Artifact state persistence', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  // Each test starts from a clean page with the Counter page app.
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  await setupPage(request, TEST_PAGE, pageBody(counterProgram('Counter', '$n')));
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when persisted state is merged', () => {
  test('the app hydrates from server state on reload', async ({ page, request }) => {
   const mergeResp = await callAPI(request, 'PageAppStateService', 'MergePageAppState', {
    page: TEST_PAGE,
    state: { $n: 5 },
   });
   expect(mergeResp.status).toBe(200);

   await page.goto(`/${TEST_PAGE}/view`);
   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-text', { hasText: 'count: 5' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
  });
 });

 test.describe('when a merge races on a stale version hash', () => {
  test('the write is rejected as Aborted', async ({ request }) => {
   // First merge succeeds and advances the state version.
   const first = await callAPI(request, 'PageAppStateService', 'MergePageAppState', {
    page: TEST_PAGE,
    state: { $n: 6 },
   });
   expect(first.status).toBe(200);

   // Second merge with a bogus precondition hash must be rejected.
   const second = await callAPI(
    request,
    'PageAppStateService',
    'MergePageAppState',
    {
     page: TEST_PAGE,
     state: { $n: 7 },
     state_expected_version_hash: 'sha256:totally-not-current',
    },
   );
   expect(second.status).toBeGreaterThanOrEqual(400);
   const code =
    secondCode(second.json) ??
    (second.text.includes('Aborted') ? 'aborted' : second.text);
   expect(String(code)).toMatch(/aborted|mismatch/i);
  });
 });
});

/** Extract the Connect error code from a payload, tolerating varied shapes. */
function secondCode(payload: unknown): string | undefined {
 if (typeof payload === 'object' && payload !== null && 'code' in payload) {
  const value = (payload as Record<string, unknown>).code;
  if (typeof value === 'string') return value;
 }
 if (typeof payload === 'object' && payload !== null && 'error' in payload) {
  const err = (payload as Record<string, unknown>).error;
  if (typeof err === 'object' && err !== null && 'code' in err) {
   const code = (err as Record<string, unknown>).code;
   if (typeof code === 'string') return code;
  }
 }
 return undefined;
}