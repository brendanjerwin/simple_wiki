/**
 * Shared helpers for the page app e2e specs.
 */
import { expect, type APIRequestContext } from '@playwright/test';

/** Standard Connect JSON headers for /api.v1.* calls. */
export function connectHeaders(): Record<string, string> {
 return { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1' };
}

/** POST a Connect JSON RPC to a v1 service and return the raw response. */
export async function callAPI(
 request: APIRequestContext,
 service: string,
 method: string,
 body: Record<string, unknown>,
): Promise<{ status: number; json: unknown; text: string }> {
 const resp = await request.post(`/api.v1.${service}/${method}`, {
  headers: connectHeaders(),
  data: body,
 });
 const text = await resp.text();
 let json: unknown = null;
 try {
  json = JSON.parse(text);
 } catch {
  // non-JSON body; keep null
 }
 return { status: resp.status(), json, text };
}

/** Error code extracted from a Connect error body, if present. */
export function connectCode(payload: unknown): string | undefined {
 const err = (payload as { error?: { code?: string } } | null)?.error;
 return err?.code;
}

/** A minimal valid wiki-page-app program. No trailing newline: callers
 * embedding it in a ```wiki-page-app fence must put the closing fence
 * on its own line (see the pageBody() helpers in the specs). */
export function counterProgram(version: string, expr: string): string {
 return [
  `$n = 0`,
  `root = Card("${version}", Stack([Text("count: " + ${expr}), Button("inc")]))`,
 ].join('\n');
}

/** Create (or reset via UpdatePageContent) a test page with the given markdown body. */
export async function setupPage(
 request: APIRequestContext,
 page: string,
 markdown: string,
): Promise<void> {
 const createResp = await callAPI(request, 'PageManagementService', 'CreatePage', {
  page,
  contentMarkdown: markdown,
 });
 const created = createResp.json as { success?: boolean; error?: string } | null;
 if (createResp.status !== 200 || created?.success !== true) {
  // The server reports "page already exists" as HTTP 200 + success:false.
  // Any other failure (e.g. the page-app fence validation) is a broken
  // fixture; fail fast with the actual server error instead of silently
  // falling through to the reset path below.
  if (!created?.error?.startsWith('page already exists')) {
   throw new Error(`CreatePage ${page} failed: ${createResp.text}`);
  }
 }

 // Bounded retry on 404 guards against any read-after-create visibility
 // lag on the reset path.
 const UPDATE_404_RETRIES = 5;
 let resetResp: { status: number; json: unknown; text: string } | undefined;
 for (let attempt = 0; attempt <= UPDATE_404_RETRIES; attempt++) {
  resetResp = await callAPI(request, 'PageManagementService', 'UpdatePageContent', {
   page,
   newContentMarkdown: markdown,
   allowShrink: true,
  });
  if (resetResp.status !== 404) break;
  await new Promise((resolve) => setTimeout(resolve, attempt * 100));
 }
 expect(resetResp!.status, `reset ${page}: ${resetResp!.text}`).toBe(200);
}

/** Fetch the page app program for a page; 404 when it has none. */
export async function getPageApp(request: APIRequestContext, page: string) {
 return callAPI(request, 'PageAppService', 'GetPageApp', { page });
}