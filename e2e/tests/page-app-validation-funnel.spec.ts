/**
 * E2E workflow tests for the page app validation funnel (issue #1272).
 *
 * Both write doors (SetPageApp and UpdatePageContent-with-fence) must reject
 * programs lacking a root statement, and rejected writes must leave the
 * page's original page app intact.
 */
import { expect, test, type APIRequestContext } from '@playwright/test';
import { callAPI, counterProgram, getPageApp, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page app_validation';

const GOOD_PROGRAM = counterProgram('Counter', '$n');

function pageBody(program: string): string {
 return `Prose.\n\n\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('Artifact validation funnel', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  await setupPage(request, TEST_PAGE, pageBody(GOOD_PROGRAM));
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when SetPageApp is called with a rootless program', () => {
  test('the RPC is rejected mentioning root', async ({ request }) => {
   const resp = await callAPI(request, 'PageAppService', 'SetPageApp', {
    page: TEST_PAGE,
    source: 'no root statement here',
   });
   expect(resp.status).toBeGreaterThanOrEqual(400);
   expect(resp.status).toBeLessThan(500);
   expect(resp.text).toContain('root');
  });
 });

 test.describe('when UpdatePageContent carries a rootless fence program', () => {
  test('the write is rejected mentioning root', async ({ request }) => {
   const resp = await callAPI(request, 'PageManagementService', 'UpdatePageContent', {
    page: TEST_PAGE,
    newContentMarkdown: pageBody('$n = 0\nStack([Text("orphan")])'),
   });
   expect(resp.status).toBeGreaterThanOrEqual(400);
   expect(resp.status).toBeLessThan(500);
   expect(resp.text).toContain('root');
  });
 });

 test.describe('after the rejected writes', () => {
  test('the original page app is unchanged', async ({ request }) => {
   const page app = await getPageApp(request, TEST_PAGE);
   expect(page app.status).toBe(200);
   expect(page appSource(page app.json)).toContain('root = Card("Counter"');
  });
 });
});

/** Read `source` from a GetPageApp response payload. */
function page appSource(payload: unknown): string {
 if (typeof payload === 'object' && payload !== null && 'source' in payload) {
  const value = (payload as Record<string, unknown>).source;
  return typeof value === 'string' ? value : '';
 }
 return '';
}