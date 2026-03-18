import { test, expect, APIRequestContext } from '@playwright/test';

// Tests for UpdatePageContent find-and-replace behavior.
// These tests call the Connect protocol API directly (no browser UI) to verify
// the precise gRPC-level semantics introduced in PR #327.

const TEST_PAGE = 'e2e-update-page-content-test';

// Initial markdown body used to seed the test page before each test.
const INITIAL_MARKDOWN = '# Section One\n\nOriginal content here.\n\n# Section Two\n\nMore content.';

// Call a PageManagementService method via the Connect protocol (JSON over HTTP).
async function callPageAPI(
  request: APIRequestContext,
  method: string,
  body: Record<string, unknown>,
) {
  return request.post(`/api.v1.PageManagementService/${method}`, {
    headers: { 'Content-Type': 'application/json' },
    data: body,
  });
}

// Ensure the test page exists with the given markdown body.
// Tries CreatePage first; if the page already exists, falls back to UpdateWholePage.
async function setupTestPage(request: APIRequestContext, markdown: string): Promise<void> {
  const createResp = await callPageAPI(request, 'CreatePage', {
    pageName: TEST_PAGE,
    contentMarkdown: markdown,
  });

  if (createResp.ok()) {
    const body = await createResp.json() as { success: boolean };
    if (body.success) {
      return;
    }
  }

  // Page already exists — overwrite it with fresh content.
  const wholePageMarkdown = `+++\nidentifier = "${TEST_PAGE}"\n+++\n\n${markdown}`;
  const updateResp = await callPageAPI(request, 'UpdateWholePage', {
    pageName: TEST_PAGE,
    newWholeMarkdown: wholePageMarkdown,
  });
  expect(updateResp.ok()).toBeTruthy();
}

// Read the current markdown body and version hash of the test page.
async function readTestPage(
  request: APIRequestContext,
): Promise<{ contentMarkdown: string; versionHash: string }> {
  const resp = await callPageAPI(request, 'ReadPage', { pageName: TEST_PAGE });
  expect(resp.ok()).toBeTruthy();
  const body = await resp.json() as { contentMarkdown: string; versionHash: string };
  return body;
}

test.describe('UpdatePageContent find-and-replace behavior', () => {
  test.setTimeout(30000);

  test.beforeEach(async ({ request }) => {
    await setupTestPage(request, INITIAL_MARKDOWN);
  });

  test('happy path: replaces only the matched substring, preserving surrounding content', async ({ request }) => {
    const oldContent = '# Section One\n\nOriginal content here.';
    const newContent = '# Section One\n\nUpdated content here.';

    const resp = await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      oldContentMarkdown: oldContent,
      newContentMarkdown: newContent,
    });

    expect(resp.status()).toBe(200);
    const body = await resp.json() as { success: boolean; versionHash: string };
    expect(body.success).toBe(true);
    expect(body.versionHash).toBeTruthy();

    // Verify only the matched section was replaced; the rest of the page is preserved.
    const { contentMarkdown } = await readTestPage(request);
    expect(contentMarkdown).toContain('# Section One\n\nUpdated content here.');
    expect(contentMarkdown).toContain('# Section Two\n\nMore content.');
    expect(contentMarkdown).not.toContain('Original content here.');
  });

  test('no-match case: returns not_found error when old_content_markdown is absent from the page', async ({ request }) => {
    const resp = await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      oldContentMarkdown: '# Section That Does Not Exist',
      newContentMarkdown: '# Replacement',
    });

    // Connect protocol maps gRPC codes.NotFound to HTTP 404.
    expect(resp.status()).toBe(404);
    const body = await resp.json() as { code: string; message: string };
    expect(body.code).toBe('not_found');
    expect(body.message).toContain('old_content_markdown not found');

    // The page content must remain unchanged.
    const { contentMarkdown } = await readTestPage(request);
    expect(contentMarkdown).toBe(INITIAL_MARKDOWN);
  });

  test('version conflict: rejects the update when expected_version_hash does not match', async ({ request }) => {
    const { versionHash } = await readTestPage(request);

    // Advance the page using full-replacement mode (no oldContentMarkdown) so the hash is stale.
    // UpdatePageContent supports both full-replacement (oldContentMarkdown omitted) and
    // find-and-replace (oldContentMarkdown provided) modes.  The version-hash check fires
    // before the mode branch, so either mode exercises the conflict logic equally.
    const intermediateResp = await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      newContentMarkdown: '# Intermediate Edit\n\nThis update makes the original hash stale.',
    });
    expect(intermediateResp.ok()).toBeTruthy();

    // Now attempt an update using the stale hash — this must be rejected.
    const conflictResp = await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      expectedVersionHash: versionHash,
      newContentMarkdown: '# Conflicting Edit',
    });

    // Connect protocol maps gRPC codes.Aborted to HTTP 409.
    expect(conflictResp.status()).toBe(409);
    const body = await conflictResp.json() as { code: string; message: string };
    expect(body.code).toBe('aborted');
    expect(body.message).toContain('version mismatch');
  });
});
