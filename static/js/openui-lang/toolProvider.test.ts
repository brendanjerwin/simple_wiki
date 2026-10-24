/**
 * Tests for the artifact tool allowlist bridge.
 */
import { expect } from '@open-wc/testing';
import { ToolNotFoundError } from '@openuidev/lang-core';
import {
 createToolProvider,
 isToolAllowed,
 qualifyToolName,
 WIKI_TOOL_ALLOWLIST,
} from './toolProvider.js';

describe('WIKI_TOOL_ALLOWLIST', () => {
 it('should pin the allowlist to the qualified state + read-only ids', () => {
  expect([...WIKI_TOOL_ALLOWLIST]).to.deep.equal([
   // ArtifactStateService — all unary methods
   'api_v1_ArtifactStateService_GetArtifactState',
   'api_v1_ArtifactStateService_MergeArtifactState',
   'api_v1_ArtifactStateService_DeleteArtifactState',
   // Read-only artifact/page reads
   'api_v1_ArtifactService_GetArtifact',
   'api_v1_PageHistoryService_ListPageVersions',
   'api_v1_SearchService_SearchContent',
   'api_v1_PageManagementService_ReadPageOutline',
   'api_v1_Frontmatter_GetFrontmatter',
   'api_v1_MapService_ListMapElements',
   'api_v1_ChecklistService_ListItems',
   'api_v1_SurveyService_GetSurvey',
  ]);
 });

 it('should not include streaming watch surfaces', () => {
  expect([...WIKI_TOOL_ALLOWLIST]).to.not.include('WatchArtifactState');
  expect([...WIKI_TOOL_ALLOWLIST]).to.not.include('WatchArtifact');
 });
});

describe('qualifyToolName', () => {
 it('should map short names to qualified ids', () => {
  expect(qualifyToolName('ReadPageOutline')).to.equal('api_v1_PageManagementService_ReadPageOutline');
  expect(qualifyToolName('ListItems')).to.equal('api_v1_ChecklistService_ListItems');
 });

 it('should leave unknown names unchanged', () => {
  expect(qualifyToolName('Nonexistent')).to.equal('Nonexistent');
  expect(qualifyToolName('')).to.equal('');
 });
});

describe('isToolAllowed', () => {
 it('should allow every allowlisted tool in both forms', () => {
  for (const name of WIKI_TOOL_ALLOWLIST) {
   expect(isToolAllowed(name), name).to.equal(true);
  }
  expect(isToolAllowed('SearchContent')).to.equal(true);
  expect(isToolAllowed('ReadPageOutline')).to.equal(true);
 });

 it('should block mutating wiki RPCs and unknown names', () => {
  for (const name of [
   'SetArtifact',
   'ClearArtifact',
   'UpdatePageContent',
   'DeleteItem',
   'SubmitResponse',
   'DeletePage',
   'WatchArtifactState',
   'WatchArtifact',
   '',
   'Nonexistent',
  ]) {
   expect(isToolAllowed(name), name).to.equal(false);
  }
 });
});

describe('createToolProvider', () => {
 it('should forward allowlisted calls to the RPC bridge as qualified ids', async () => {
  const forwarded: Array<[string, Record<string, unknown>]> = [];
  const provider = createToolProvider(async (qualifiedId, args) => {
   forwarded.push([qualifiedId, args]);
   return { echo: qualifiedId };
  });

  const result = await provider.callTool('ReadPageOutline', { page: 'home' });
  expect(result).to.deep.equal({ echo: 'api_v1_PageManagementService_ReadPageOutline' });
  expect(forwarded).to.deep.equal([['api_v1_PageManagementService_ReadPageOutline', { page: 'home' }]]);
 });

 it('should reject blocked tools with lang-core ToolNotFoundError', async () => {
  const provider = createToolProvider(async (name) => ({ echo: name }));
  let rejected: unknown;
  try {
   await provider.callTool('DeletePage', {});
  } catch (err) {
   rejected = err;
  }
  expect(rejected).to.be.instanceOf(ToolNotFoundError);
  expect((rejected as ToolNotFoundError).toolName).to.equal('DeletePage');
  expect((rejected as ToolNotFoundError).availableTools).to.not.be.empty;
 });

 it('should reject streaming watch tools', async () => {
  const provider = createToolProvider(async (name) => ({ echo: name }));
  let rejected: unknown;
  try {
   await provider.callTool('WatchArtifactState', {});
  } catch (err) {
   rejected = err;
  }
  expect(rejected).to.be.instanceOf(ToolNotFoundError);
 });
});