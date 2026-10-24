/**
 * Renderer for parsed OpenUI Lang artifact programs over the wiki library.
 *
 * Renders `ParseResult` trees: parses a program source string (or a merged
 * patch), evaluates `$variables` and Query()/Mutation() statements, and maps
 * `ElementNode.typeName` to the wiki library's component render functions.
 */
import { html, nothing } from 'lit';
import {
 createParser,
 createQueryManager,
 createStore,
 evaluate,
 evaluateRoot,
 mergeStatements,
 type ASTNode,
 type ElementNode,
 type EvaluationContext,
 type EvalContext,
 type ParseResult,
 type QueryManager,
 type Store,
} from '@openuidev/lang-core';
import type { ToolProvider } from '@openuidev/lang-core';
import {
 WIKI_ROOT_COMPONENT,
 wikiLibrary,
 type WikiRenderer,
} from './library.js';

/** A tool provider: tool name and args in, result promise out. */
export type ToolCaller = (toolName: string, args: Record<string, unknown>) => Promise<unknown>;

/** Context handed to interactive components when they fire actions. */
export interface ArtifactActionContext {
 /** Execute a `set` step's value and persist it through the host. */
 setState(values: Record<string, unknown>): void;
 /** Clear the declared defaults for `targets` back to their initial values. */
 resetState(targets: string[]): void;
 /** Send the user's message to the assistant chat for the page. */
 toAssistant(message: string): void;
 /** Open a URL in a new tab. */
 openUrl(url: string): void;
 /** Refire the Query()/Mutation() named by the statement. */
 rerun(statementId: string): void;
}

/**
 * Merge a full artifact program with a patch program. Delegates to
 * lang-core's `mergeStatements`.
 */
export function mergeArtifactPatch(existing: string, patch: string, rootId?: string): string {
 return mergeStatements(existing, patch, rootId);
}

function isElementNode(value: unknown): value is ElementNode {
 return (
  typeof value === 'object'
  && value !== null
  && (value as ElementNode).type === 'element'
  && typeof (value as ElementNode).typeName === 'string'
 );
}

/**
 * Build the artifact renderer. Each call of the returned render function
 * parses the program, hydrates persisted `$variable` state, executes
 * Query()/Mutation() statements through the tool provider, and returns a
 * Lit template for the evaluated tree.
 */
export function generateWikiRenderer(
 callTool: ToolCaller | null,
 actionContext: ArtifactActionContext,
): WikiArtefactRenderFn {
 const parser = createParser(wikiLibrary.toJSONSchema(), WIKI_ROOT_COMPONENT);
 const queryManager = createQueryManager(
  callTool === null ? null : { callTool } satisfies ToolProvider,
 );
 const store = createStore();

 const render: WikiArtefactRenderFn = (source: string, persistedState: Record<string, unknown>) => {
  const result = parser.parse(source);
  store.initialize(result.stateDeclarations, persistedState ?? {});

  const evaluationContext: EvaluationContext = {
   getState: (name) => store.get(name),
   resolveRef: (name) => store.get(name),
  };

  hydrateQueriesAndMutations(result, { store, queryManager, evaluationContext });

  const evalContext: EvalContext = {
   ctx: { ...evaluationContext, extraScope: queryManager.getSnapshot() },
   library: wikiLibrary,
   store,
  };
  const evaluatedRoot = evaluateRoot(result, evalContext);
  return renderNode(evaluatedRoot);
 };

 void actionContext;
 return render;
}

/** A render entry: program source + persisted state in, Lit template out. */
export type WikiArtefactRenderFn = (
 source: string,
 persistedState: Record<string, unknown>,
) => TemplateResult | typeof nothing;

function hydrateQueriesAndMutations(
 result: ParseResult,
 runtime: { store: Store; queryManager: QueryManager; evaluationContext: EvaluationContext },
): void {
 const { queryManager, evaluationContext } = runtime;

 const toolNameOf = (ast: ASTNode | null): string | null => {
  if (!ast) return null;
  const value = evaluate(ast, evaluationContext);
  return typeof value === 'string' ? value : null;
 };

 queryManager.registerMutations(
  result.mutationStatements.flatMap((mutation) => {
   const toolName = toolNameOf(mutation.toolAST);
   return toolName === null ? [] : [{ statementId: mutation.statementId, toolName }];
  }),
 );

 const queryNodes = result.queryStatements.flatMap((query) => {
  const toolName = toolNameOf(query.toolAST);
  if (toolName === null) return [];
  return [
   {
    statementId: query.statementId,
    toolName,
    args: query.argsAST ? evaluate(query.argsAST, evaluationContext) : {},
    defaults: query.defaultsAST ? evaluate(query.defaultsAST, evaluationContext) : null,
    deps: null,
    complete: query.complete,
   },
  ];
 });
 if (queryNodes.length > 0) {
  queryManager.evaluateQueries(queryNodes);
 }
}

/**
 * Render the evaluated root tree into a Lit template. Unknown components
 * (dropped by the parser; errors land in `parseResult.meta.errors`) render
 * as nothing.
 */
export function renderNode(node: unknown): TemplateResult | unknown {
 if (node == null) return nothing;
 if (Array.isArray(node)) {
  return node.map((child) => renderNode(child));
 }
 if (!isElementNode(node)) return node;
 const renderer = wikiLibrary.components[node.typeName]?.component as WikiRenderer | null | undefined;
 if (!renderer) return nothing;
 const renderProps: Record<string, unknown> = {};
 for (const [key, value] of Object.entries(node.props)) {
  renderProps[key] = isReactiveAssignMarker(value) ? value : renderPropValue(value);
 }
 return renderer(renderProps);
}

/** lang-core's reactive-assign marker for $variable-bound props. */
interface ReactiveAssignMarker {
 __reactive: 'assign';
 target: string;
 expr: unknown;
}

function isReactiveAssignMarker(value: unknown): value is ReactiveAssignMarker {
 return (
  typeof value === 'object'
  && value !== null
  && (value as ReactiveAssignMarker).__reactive === 'assign'
 );
}

function renderPropValue(value: unknown): unknown {
 if (value == null) return nothing;
 if (Array.isArray(value)) return value.map((v) => renderNode(v));
 if (isElementNode(value)) return renderNode(value);
 return value;
}