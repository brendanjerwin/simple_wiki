/**
 * Shared helpers for the artifact e2e specs.
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

/** A minimal valid wiki-artifact program. */
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
 if (createResp.status === 200) return;

 const resetResp = await callAPI(request, 'PageManagementService', 'UpdatePageContent', {
  page,
  newContentMarkdown: markdown,
 });
 expect(resetResp.status, `reset ${page}: ${resetResp.text}`).toBe(200);
}

/** Fetch the artifact program for a page; 404 when it has none. */
export async function getArtifact(request: APIRequestContext, page: string) {
 return callAPI(request, 'ArtifactService', 'GetArtifact', { page });
}