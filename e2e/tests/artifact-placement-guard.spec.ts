/**
 * E2E workflow tests for the artifact placement guard (issue #1272).
 *
 * The guard blocks only writes that would CLEAR substantial prose
 * (>1KB). Composition of a fence with existing prose succeeds; a
 * UpdateWholePage that replaces the whole body with just the artifact fence
 * must fail with a FailedPrecondition mentioning prose.
 */
import { expect, test, type APIRequestContext } from '@playwright/test';
import { callAPI, counterProgram, getArtifact, setupPage } from './artifact-helpers';

const TEST_PAGE = 'e2e_artifact_placement';

const PAGE_LOAD_TIMEOUT_MS = 15000;

const GOOD_PROGRAM = counterProgram('Counter', '$n');

const PARAGRAPH =
 'This long prose paragraph exists to exercise the placement guard. It describes the page, its history, its purpose, and several other genuinely wordy considerations that pad it well past the one kilobyte threshold. ';

function proseBody(): string {
 return `## Prose\n\n${PARAGRAPH.repeat(8)}\n`;
}

function bodyWithFence(body: string, program: string): string {
 return `${body}\n\`\`\`wiki-artifact\n${program}\n\`\`\`\n`;
}

test.describe.serial('Artifact placement guard', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  await setupPage(request, TEST_PAGE, proseBody());
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'ArtifactService', 'ClearArtifact', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when SetArtifact is called on a page with substantial prose', () => {
  test('composition succeeds and the prose survives', async ({ page, request }) => {
   const setResp = await callAPI(request, 'ArtifactService', 'SetArtifact', {
    page: TEST_PAGE,
    source: GOOD_PROGRAM,
   });
   expect(setResp.status).toBe(200);

   const artifact = await getArtifact(request, TEST_PAGE);
   expect(artifact.status).toBe(200);
   expect(artifactSource(artifact.json)).toContain('root = Card');

   // Page body still contains the prose.
   const readResp = await callAPI(request, 'PageManagementService', 'ReadPage', {
    page: TEST_PAGE,
   });
   expect(readResp.status).toBe(200);
   const content = pageContent(readResp.json);
   expect(content).toContain('placement guard');

   // Rendered view: prose AND artifact app.
   await page.goto(`/${TEST_PAGE}/view`);
   await expect(page.getByText('placement guard').first()).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
   const host = page.locator('wiki-artifact-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-artifact-card-title', { hasText: 'Counter' }),
   ).toBeVisible();
  });
 });

 test.describe('when UpdateWholePage replaces the body with only the fence', () => {
  test('the prose-clearing write is rejected', async ({ request }) => {
   const fenceOnly = `\`\`\`wiki-artifact\n${GOOD_PROGRAM}\n\`\`\`\n`;
   const resp = await callAPI(request, 'PageManagementService', 'UpdateWholePage', {
    page: TEST_PAGE,
    new_whole_markdown: fenceOnly,
   });
   // The server's shrink/size guard rejects the write (failed_precondition
   // mentions the attempted near-total wipe). Status is a 400-class error.
   expect(resp.status).toBeGreaterThanOrEqual(400);
   expect(resp.status).toBeLessThan(500);
   expect(resp.text).toContain('failed_precondition');

   // The page content was NOT changed.
   const readResp = await callAPI(request, 'PageManagementService', 'ReadPage', {
    page: TEST_PAGE,
   });
   expect(readResp.status).toBe(200);
   expect(pageContent(readResp.json)).toContain('placement guard');
  });
 });
});

/** Read `contentMarkdown` from a ReadPage payload. */
function pageContent(payload: unknown): string {
 if (typeof payload === 'object' && payload !== null && 'contentMarkdown' in payload) {
  const value = (payload as Record<string, unknown>).contentMarkdown;
  return typeof value === 'string' ? value : '';
 }
 return '';
}

/** Read `source` from a GetArtifact response payload. */
function artifactSource(payload: unknown): string {
 if (typeof payload === 'object' && payload !== null && 'source' in payload) {
  const value = (payload as Record<string, unknown>).source;
  return typeof value === 'string' ? value : '';
 }
 return '';
}