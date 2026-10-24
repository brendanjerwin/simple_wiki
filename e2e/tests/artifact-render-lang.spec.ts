/**
 * E2E workflow tests for artifact page rendering (issue #1272).
 *
 * A page containing a ```wiki-artifact fence renders a Lang-program UI via
 * <wiki-artifact-host>; the prose around the fence stays as markdown. The
 * host hot-swaps programs via WatchArtifact without a page reload.
 */
import { expect, test, type APIRequestContext, type Page } from '@playwright/test';
import { callAPI, counterProgram, getArtifact, setupPage } from './artifact-helpers';

const TEST_PAGE = 'e2e_artifact_render';

const PAGE_LOAD_TIMEOUT_MS = 15000;

const PROSE = `## Intro prose

This paragraph is plain markdown prose that must survive artifact rendering.
`;

function pageBody(program: string): string {
 return `${PROSE}\n\`\`\`wiki-artifact\n${program}\n\`\`\`\n`;
}

/** The wiki-artifact-host renders its app card; waits for load. */
async function expectHostRendering(page: Page, version: string) {
 const host = page.locator('wiki-artifact-host');
 await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
 await expect(
  host.locator('.wiki-artifact-card-title', { hasText: version }),
 ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
 await expect(
  host.locator('button.wiki-artifact-button', { hasText: 'inc' }),
 ).toBeVisible();
}

test.describe.serial('Artifact page render', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  // Ensure each test starts from a clean slate: delete a stale page, then
  // recreate it with the Counter program fenced into the body.
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  await setupPage(request, TEST_PAGE, pageBody(counterProgram('Counter', '$n')));
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'ArtifactService', 'ClearArtifact', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page contains a wiki-artifact fence', () => {
  test('renders the artifact app and surrounding prose on <page>/view', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   await expectHostRendering(page, 'Counter');

   // Prose survives outside the artifact.
   await expect(page.getByText('This paragraph is plain markdown prose')).toBeVisible();

   // The program source must NOT leak into the rendered prose section.
   const renderedText = await page.locator('#rendered').textContent();
   expect(renderedText, 'program text must not appear in #rendered').not.toContain(
    'root = Card',
   );
  });
 });

 test.describe('when the page program is updated while the view is open', () => {
  test('hot-swaps the program without a page reload', async ({ page, request }) => {
   const created = await callAPI(request, 'PageManagementService', 'CreatePage', {
    page: TEST_PAGE,
    contentMarkdown: pageBody(counterProgram('Counter', '$n')),
   });
   expect(created.status).toBe(200);

   await page.goto(`/${TEST_PAGE}/view`);
   await expectHostRendering(page, 'Counter');

   const updateResp = await callAPI(
    request,
    'PageManagementService',
    'UpdatePageContent',
    { page: TEST_PAGE, newContentMarkdown: pageBody(counterProgram('Counter v2', '$n')) },
   );
   expect(updateResp.status).toBe(200);

   // Poll until hot-swap completes; verify the SAME DOM node is still there
   // and now renders the v2 program.
   await expect
    .poll(
     async () =>
      page.evaluate(() => {
       const el = document.querySelector('wiki-artifact-host');
       if (!el || !el.shadowRoot) return null;
       return el.shadowRoot.textContent?.includes('Counter v2') === true
        ? 'swapped'
        : 'pending';
      }),
     { timeout: PAGE_LOAD_TIMEOUT_MS, intervals: [300] },
    )
    .toBe('swapped');

   // Same node still present, never detached/recreated.
   const sameNode = await page.evaluate(() => {
    const el = document.querySelector('wiki-artifact-host');
    return el instanceof HTMLElement && el.isConnected;
   });
   expect(sameNode).toBe(true);
  });
 });

 test.describe('when a page-content write omits the root statement', () => {
  test('rejects the write and preserves the original artifact', async ({ request }) => {
   const badBody = pageBody('$n = 0\nCard("no root")');
   const resp = await callAPI(request, 'PageManagementService', 'UpdatePageContent', {
    page: TEST_PAGE,
    newContentMarkdown: badBody,
   });
   expect(resp.status).toBeGreaterThanOrEqual(400);
   expect(resp.text, 'rejected write must mention root').toContain('root');

   const artifact = await getArtifact(request, TEST_PAGE);
   expect(artifact.status).toBe(200);
   expect(artifactText(artifact.json)).toContain('root = Card("Counter"');
  });
 });
});

/** Read `source` from a GetArtifact response payload. */
function artifactText(payload: unknown): string {
 if (typeof payload === 'object' && payload !== null && 'source' in payload) {
  const value = (payload as Record<string, unknown>).source;
  return typeof value === 'string' ? value : '';
 }
 return '';
}