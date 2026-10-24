/**
 * E2E workflow tests for the artifact validation funnel (issue #1272).
 *
 * Both write doors (SetArtifact and UpdatePageContent-with-fence) must reject
 * programs lacking a root statement, and rejected writes must leave the
 * page's original artifact intact.
 */
import { expect, test, type APIRequestContext } from '@playwright/test';
import { callAPI, counterProgram, getArtifact, setupPage } from './artifact-helpers';

const TEST_PAGE = 'e2e_artifact_validation';

const GOOD_PROGRAM = counterProgram('Counter', '$n');

function pageBody(program: string): string {
  return `Prose.\n\n\`\`\`wiki-artifact\n${program}\n\`\`\`\n`;
}

test.describe.serial('Artifact validation funnel', () => {
  test.setTimeout(90000);
  let requestContext: APIRequestContext;

  test.beforeAll(async ({ request }) => {
    requestContext = request;
    await callAPI(request, 'ArtifactService', 'ClearArtifact', { page: TEST_PAGE });
    await setupPage(request, TEST_PAGE, pageBody(GOOD_PROGRAM));
  });

  test.afterAll(async () => {
    await callAPI(requestContext, 'ArtifactService', 'ClearArtifact', { page: TEST_PAGE });
    await callAPI(requestContext, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  });

  test.describe('when SetArtifact is called with a rootless program', () => {
    test('the RPC is rejected mentioning root', async () => {
      const resp = await callAPI(requestContext, 'ArtifactService', 'SetArtifact', {
        page: TEST_PAGE,
        source: 'no root statement here',
      });
      expect(resp.status).toBeGreaterThanOrEqual(400);
      expect(resp.status).toBeLessThan(500);
      expect(resp.text).toContain('root');
    });
  });

  test.describe('when UpdatePageContent carries a rootless fence program', () => {
    test('the write is rejected mentioning root', async () => {
      const resp = await callAPI(requestContext, 'PageManagementService', 'UpdatePageContent', {
        page: TEST_PAGE,
        newContentMarkdown: pageBody('$n = 0\nStack([Text("orphan")])'),
      });
      expect(resp.status).toBeGreaterThanOrEqual(400);
      expect(resp.status).toBeLessThan(500);
      expect(resp.text).toContain('root');
    });
  });

  test.describe('after the rejected writes', () => {
    test('the original artifact is unchanged', async () => {
      const artifact = await getArtifact(requestContext, TEST_PAGE);
      expect(artifact.status).toBe(200);
      expect(artifactSource(artifact.json)).toContain('root = Card("Counter"');
    });
  });
});

/** Read `source` from a GetArtifact response payload. */
function artifactSource(payload: unknown): string {
  if (typeof payload === 'object' && payload !== null && 'source' in payload) {
    const value = (payload as Record<string, unknown>).source;
    return typeof value === 'string' ? value : '';
  }
  return '';
}