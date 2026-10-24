/**
 * Allowlist bridge from openui-lang Query()/Mutation() calls to the wiki's
 * Connect services. Hard-coded and test-pinned: artifact programs may only
 * reach state methods plus a small read-only surface; everything else
 * resolves to tool-not-found. Tool ids are the fully-qualified names the
 * wiki MCP catalog uses.
 */
import { ToolNotFoundError, type ToolProvider } from '@openuidev/lang-core';

/**
 * The complete allowlist: all unary ArtifactStateService methods plus
 * read-only wiki reads. Streaming watch surfaces and mutating wiki RPCs
 * (SetArtifact, ClearArtifact, UpdatePageContent, checklist/map/survey
 * mutations, ...) are not callable from programs.
 */
export const WIKI_TOOL_ALLOWLIST: readonly string[] = [
 // ArtifactStateService — all unary methods
 'api_v1_ArtifactStateService_GetArtifactState',
 'api_v1_ArtifactStateService_MergeArtifactState',
 'api_v1_ArtifactStateService_DeleteArtifactState',
 // Read-only artifact/page reads
 'api_v1_ArtifactService_GetArtifact',
 'api_v1_PageHistoryService_ListPageVersions',
 'api_v1_SearchService_SearchContent',
 'api_v1_PageManagementService_GetPageOutline',
 'api_v1_Frontmatter_GetFrontmatter',
 'api_v1_MapService_ListMapElements',
 'api_v1_ChecklistService_ListItems',
 'api_v1_SurveyService_GetSurvey',
] as const;

/** Test seam: the short names programs may also use, mapped to qualified ids. */
const TOOL_ID_BY_SHORT_NAME: Record<string, string> = {
 GetArtifactState: 'api_v1_ArtifactStateService_GetArtifactState',
 MergeArtifactState: 'api_v1_ArtifactStateService_MergeArtifactState',
 DeleteArtifactState: 'api_v1_ArtifactStateService_DeleteArtifactState',
 GetArtifact: 'api_v1_ArtifactService_GetArtifact',
 ListPageVersions: 'api_v1_PageHistoryService_ListPageVersions',
 SearchContent: 'api_v1_SearchService_SearchContent',
 GetPageOutline: 'api_v1_PageManagementService_GetPageOutline',
 GetFrontmatter: 'api_v1_Frontmatter_GetFrontmatter',
 ListMapElements: 'api_v1_MapService_ListMapElements',
 ListItems: 'api_v1_ChecklistService_ListItems',
 GetSurvey: 'api_v1_SurveyService_GetSurvey',
};

/**
 * Resolve a program's tool name to its qualified id. Accepts either form;
 * unknown names come back unchanged so the allowlist check rejects them.
 */
export function qualifyToolName(toolName: string): string {
 return TOOL_ID_BY_SHORT_NAME[toolName] ?? toolName;
}

export type { ToolProvider, ToolNotFoundError };

export function isToolAllowed(toolName: string): boolean {
 return WIKI_TOOL_ALLOWLIST.includes(qualifyToolName(toolName));
}

/**
 * Build a lang-core `ToolProvider` backed by the wiki Connect clients.
 * `callRpc` receives the QUALIFIED tool id and resolves the unary call.
 */
export function createToolProvider(
 callRpc: (qualifiedToolId: string, args: Record<string, unknown>) => Promise<unknown>,
): ToolProvider {
 return {
  async callTool(toolName: string, args: Record<string, unknown>): Promise<unknown> {
   const qualified = qualifyToolName(toolName);
   if (!isToolAllowed(qualified)) {
    throw new ToolNotFoundError(toolName, [...WIKI_TOOL_ALLOWLIST]);
   }
   return callRpc(qualified, args);
  },
 };
}