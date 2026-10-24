/**
 * E2E workflow tests for artifact clearing (issue #1272).
 *
 * ClearArtifact removes the artifact from a page, leaving its prose. The
 * page returns to plain markdown: no host element, GetArtifact not found.
 */
import { expect, test, type APIRequestContext } from '@playwright/test';
import { callAPI, counterProgram, getArtifact, setupPage } from './artifact-helpers';

const TEST_PAGE = 'e2e_artifact_clear';

const PAGE_LOAD_TIMEOUT_MS = 15000;

function pageBody(program: string): string {
 return `Keep this prose paragraph after clearing.\n\n\`\`\`wiki-artifact\n${program}\n\`\`\`\n`;
}

test.describe.serial('Artifact clear', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  await setupPage(request, TEST_PAGE, pageBody(counterProgram('Counter', '$n')));
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'ArtifactService', 'ClearArtifact', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when ClearArtifact is called on a page with prose and an artifact', () => {
  test('prose stays, the host is gone, and GetArtifact is NotFound', async ({ page, request }) => {
   // Sanity: artifact present before the clear.
   const before = await getArtifact(request, TEST_PAGE);
   expect(before.status).toBe(200);

   const clearResp = await callAPI(request, 'ArtifactService', 'ClearArtifact', {
    page: TEST_PAGE,
   });
   expect(clearResp.status).toBe(200);

   const after = await getArtifact(request, TEST_PAGE);
   expect(after.status).toBe(404);

   await page.goto(`/${TEST_PAGE}/view`);
   await expect(page.getByText('Keep this prose paragraph after clearing.')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
   await expect(page.locator('wiki-artifact-host')).toHaveCount(0, {
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });
 });

 test.describe('when the artifact is already cleared', () => {
  test('ClearArtifact is idempotent', async ({ request }) => {
   // The artifact is already gone after the previous test; the fence must
   // also be gone from the body, so re-add it to exercise the clear path.
   const setupResp = await callAPI(
    request,
    'PageManagementService',
    'UpdatePageContent',
    { page: TEST_PAGE, newContentMarkdown: pageBody(counterProgram('Counter', '$n')) },
   );
   expect(setupResp.status).toBe(200);

   const first = await callAPI(request, 'ArtifactService', 'ClearArtifact', {
    page: TEST_PAGE,
   });
   expect(first.status, first.text).toBe(200);
   const second = await callAPI(request, 'ArtifactService', 'ClearArtifact', {
    page: TEST_PAGE,
   });
   // Idempotent server returns success (200) or NotFound (404) — both mean
   // no artifact remains.
   expect([200, 404]).toContain(second.status);
   const after = await getArtifact(request, TEST_PAGE);
   expect(after.status).toBe(404);
  });
 });
});