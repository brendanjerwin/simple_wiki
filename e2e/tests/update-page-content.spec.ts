import { test, expect, APIRequestContext } from '@playwright/test';

// Tests for UpdatePageContent find-and-replace behavior.
// These tests call the Connect protocol API directly (no browser UI) to verify
// the precise gRPC-level semantics introduced in PR #327.

const TEST_PAGE = 'e2e_update_page_content_test';

// Initial markdown body used to seed the test page before each test.
const INITIAL_MARKDOWN = '# Section One\n\nOriginal content here.\n\n# Section Two\n\nMore content.';

// Call a PageManagementService method via the Connect protocol (JSON over HTTP).
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

// Ensure the test page exists with the given markdown body.
// Tries CreatePage first; if creation fails or the page already exists, falls back to
// UpdatePageContent in full-replacement mode to reset the body — no frontmatter required.
async function setupTestPage(request: APIRequestContext, markdown: string): Promise<void> {
  const createResp = await callPageAPI(request, 'CreatePage', {
    pageName: TEST_PAGE,
    contentMarkdown: markdown,
  });
  if (createResp.ok()) {
    const body = await createResp.json() as { success: boolean };
    if (body.success) return;
  }

  // Page already exists (CreatePage returns HTTP 200 with success=false for existing pages)
  // or the HTTP request itself failed — reset body with full-replacement UpdatePageContent.
  const resetResp = await callPageAPI(request, 'UpdatePageContent', {
    pageName: TEST_PAGE,
    newContentMarkdown: markdown,
  });
  expect(resetResp.ok()).toBeTruthy();
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
  // Run serially: tests share a single mutable TEST_PAGE and beforeEach resets it.
  // Serial mode prevents races between the reset and the next test's reads/writes.
  test.describe.configure({ mode: 'serial' });
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
    expect(contentMarkdown).toContain('# Section One\n\nOriginal content here.');
    expect(contentMarkdown).toContain('# Section Two\n\nMore content.');
  });

  test('version conflict: rejects the update when expected_version_hash does not match', async ({ request }) => {
    const { versionHash } = await readTestPage(request);

    // Advance the page via find-and-replace so the captured hash is now stale.
    const intermediateContent = '# Intermediate Edit\n\nThis update makes the original hash stale.';
    const intermediateResp = await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      oldContentMarkdown: INITIAL_MARKDOWN,
      newContentMarkdown: intermediateContent,
    });
    expect(intermediateResp.ok()).toBeTruthy();

    // Attempt a find-and-replace with the stale hash — this must be rejected.
    const conflictResp = await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      oldContentMarkdown: intermediateContent,
      expectedVersionHash: versionHash,
      newContentMarkdown: '# Conflicting Edit',
    });

    // Connect protocol maps gRPC codes.Aborted to HTTP 409.
    expect(conflictResp.status()).toBe(409);
    const body = await conflictResp.json() as { code: string; message: string };
    expect(body.code).toBe('aborted');
    expect(body.message).toContain('version mismatch');
  });

  test('version conflict (full-replacement): rejects the update when expectedVersionHash does not match', async ({ request }) => {
    const { versionHash } = await readTestPage(request);

    // Advance the page with a full-replacement so the captured hash is now stale.
    const advanceResp = await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      newContentMarkdown: '# Intermediate Full Replacement\n\nThis makes the original hash stale.',
    });
    expect(advanceResp.ok()).toBeTruthy();

    // Attempt a full-replacement with the stale hash — this must be rejected.
    const conflictResp = await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      expectedVersionHash: versionHash,
      newContentMarkdown: '# Conflicting Full Replacement',
    });

    // Connect protocol maps gRPC codes.Aborted to HTTP 409.
    expect(conflictResp.status()).toBe(409);
    const body = await conflictResp.json() as { code: string; message: string };
    expect(body.code).toBe('aborted');
    expect(body.message).toContain('version mismatch');
  });

  test('correct hash (find-and-replace): accepts the update and returns a new version hash', async ({ request }) => {
    const { versionHash: originalHash, contentMarkdown } = await readTestPage(request);

    const resp = await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      oldContentMarkdown: contentMarkdown,
      expectedVersionHash: originalHash,
      newContentMarkdown: '# Section One\n\nUpdated with correct hash.',
    });

    expect(resp.status()).toBe(200);
    const body = await resp.json() as { success: boolean; versionHash: string };
    expect(body.success).toBe(true);
    // Response body must contain a new version hash reflecting the updated content.
    expect(body.versionHash).toBeTruthy();
    expect(body.versionHash).not.toBe(originalHash);

    // The page content must reflect the find-and-replace.
    const { contentMarkdown: updatedMarkdown } = await readTestPage(request);
    expect(updatedMarkdown).toContain('Updated with correct hash.');
    expect(updatedMarkdown).not.toContain('Original content here.');
  });

  test('correct hash (full-replacement): accepts the update and returns a new version hash', async ({ request }) => {
    const { versionHash: originalHash } = await readTestPage(request);

    const resp = await callPageAPI(request, 'UpdatePageContent', {
      pageName: TEST_PAGE,
      expectedVersionHash: originalHash,
      newContentMarkdown: '# Fully replaced with correct hash.',
    });

    expect(resp.status()).toBe(200);
    const body = await resp.json() as { success: boolean; versionHash: string };
    expect(body.success).toBe(true);
    // Response body must contain a new version hash reflecting the updated content.
    expect(body.versionHash).toBeTruthy();
    expect(body.versionHash).not.toBe(originalHash);

    // The page content must reflect the full replacement.
    const { contentMarkdown } = await readTestPage(request);
    expect(contentMarkdown).toContain('Fully replaced with correct hash.');
    expect(contentMarkdown).not.toContain('Section One');
  });
});
