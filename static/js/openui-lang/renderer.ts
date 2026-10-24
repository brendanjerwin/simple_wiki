/**
 * Renderer for parsed OpenUI Lang page app programs over the wiki library.
 *
 * Renders `ParseResult` trees: parses a program source string (or a merged
 * patch), evaluates `$variables` and Query()/Mutation() statements through a
 * lang-core `ToolProvider`, and maps `ElementNode.typeName` to the wiki
 * library's component render functions. Action plans (from @Set, @Reset,
 * @ToAssistant, @OpenUrl, @Run) are surfaced through a `wiki-page-app-action`
 * event so the host executes them against the wiki.
 */
import { nothing } from 'lit';
import type { TemplateResult } from 'lit';
import {
 createParser,
 createQueryManager,
 createStore,
 evaluate,
 evaluateRoot,
 mergeStatements,
 isReactiveAssign,
 stripReactiveAssign,
 type ASTNode,
 type ElementNode,
 type EvaluationContext,
 type EvalContext,
 type ParseResult,
 type QueryManager,
 type Store,
 type ToolNotFoundError,
 type ToolProvider,
} from '@openuidev/lang-core';
import {
 WIKI_ROOT_COMPONENT,
 PAGE_APP_ACTION_EVENT,
 wikiLibrary,
 type WikiRenderer,
} from './library.js';

/**
 * Host-side executor for page app action plans. The host subscribes to
 * `onStateChange` so every @Set/@Reset flushes the LIVE lang-core store
 * snapshot rather than a stale persisted copy.
 */
export interface PageAppActionContext {
 /** Flush the live store snapshot (all $variable values). */
 setState(snapshot: Record<string, unknown>): void;
 /** Send the user's message to the assistant chat for the page. */
 toAssistant(message: string): void;
 /** Open a URL in a new tab. */
 openUrl(url: string): void;
 /** Invalidate and refire query results. */
 rerun(statementIds?: string[]): void;
}

/** A template plus the parse diagnostics for the rendered program. */
export interface RenderedPageApp {
 template: TemplateResult | typeof nothing;
 /** Parse/runtime errors for the program (unknown components, etc.). */
 errors: { code: string; message: string; component?: string; statementId?: string }[];
 /** $variable declarations materialized by parsing the program. */
 stateDeclarations: Record<string, unknown>;
}

/**
 * Renderer handle: `render(source, persistedState)` renders one program;
 * `store` is the live lang-core store (source of truth for flushes);
 * `queryManager` tracks Query()/Mutation() results (subscribe for repaints);
 * `refetch` invalidates queries.
 */
export interface WikiPageAppRuntime {
 render(source: string, persistedState: Record<string, unknown>): RenderedPageApp;
 store: Store;
 queryManager: QueryManager;
 /** Evaluate a raw AST node (a @Set step's valueAST) against the live store. */
 evaluateStepValue(ast: unknown, extraScope?: Record<string, unknown>): unknown;
 refetch(statementIds?: string[]): void;
}

/**
 * Merge a full page app program with a patch program. Delegates to
 * lang-core's `mergeStatements`.
 */
export function mergePageAppPatch(existing: string, patch: string, rootId?: string): string {
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
 * Build a page app renderer over the wiki library.
 */
export function generateWikiPageAppRenderer(
 toolProvider: ToolProvider | null,
 actionContext: PageAppActionContext,
): WikiPageAppRuntime {
 const parser = createParser(wikiLibrary.toJSONSchema(), WIKI_ROOT_COMPONENT);
 const queryManager = createQueryManager(toolProvider);
 const store = createStore();

 let lastParsedSource: string | null = null;
 let lastResult: ParseResult | null = null;
 let lastErrors: RenderedPageApp['errors'] = [];
 let lastDeclarations: Record<string, unknown> = {};

 // evaluate+render phase shared by render/repaint: no re-parse, no
 // re-fire of Query RPCs (evaluateQueries happens only in render()).
 const evaluateAndRender = (): RenderedPageApp => {
  if (lastResult == null) {
   return { template: nothing, errors: [], stateDeclarations: {} };
  }
  const evaluationContext: EvaluationContext = {
   getState: (name) => store.get(name),
   resolveRef: (name) => store.get(name),
  };
  const evalContext: EvalContext = {
   ctx: { ...evaluationContext, extraScope: queryManager.getSnapshot() },
   library: wikiLibrary,
   store,
  };
  const evaluatedRoot = evaluateRoot(lastResult, evalContext);
  return {
   template: evaluatedRoot === null ? nothing : (renderTree(evaluatedRoot, evaluationContext) as TemplateResult),
   errors: lastErrors,
   stateDeclarations: lastDeclarations,
  };
 };

 const render = (source: string, persistedState: Record<string, unknown>): RenderedPageApp => {
  if (source === lastParsedSource && lastResult != null) {
   // Same program: re-evaluate against current snapshots (no re-parse,
   // no Query refires). Hydration state updates still apply.
   store.initialize(lastDeclarations, persistedState ?? {});
   return evaluateAndRender();
  }

  const result = parser.parse(source);
  lastParsedSource = source;
  lastResult = result;
  store.initialize(result.stateDeclarations, persistedState ?? {});
  lastDeclarations = result.stateDeclarations;

  const evaluationContext: EvaluationContext = {
   getState: (name) => store.get(name),
   resolveRef: (name) => store.get(name),
  };

  hydrateQueriesAndMutations(result, { queryManager, evaluationContext });

  lastErrors = result.meta.errors.map((err) => ({
   code: err.code,
   message: err.message,
   component: err.component,
   statementId: err.statementId ?? '',
  }));

  return evaluateAndRender();
 };

 return {
  render,
  store,
  queryManager,
  /**
   * Evaluate a raw AST node (e.g. a @Set step's valueAST — may be a
   * BinOp like `$n + 1`) against the live store; the host's hand-rolled
   * readStepValue can't do arithmetic.
   */
  evaluateStepValue: (ast: unknown, extraScope?: Record<string, unknown>): unknown => {
   if (ast == null || typeof ast !== 'object') return undefined;
   const evaluationContext: EvaluationContext = {
    getState: (name) => store.get(name),
    resolveRef: (name) => store.get(name),
    ...(extraScope !== undefined ? { extraScope } : {}),
   };
   return evaluate(ast as ASTNode, evaluationContext);
  },
  refetch: (statementIds?: string[]) => {
   queryManager.invalidate(statementIds);
   actionContext.rerun(statementIds);
  },
 };
}

function hydrateQueriesAndMutations(
 result: ParseResult,
 runtime: { queryManager: QueryManager; evaluationContext: EvaluationContext },
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
 * (dropped by the parser; see `RenderedPageApp.errors`) render as nothing.
 */
export function renderTree(node: unknown, evaluationContext?: EvaluationContext): unknown {
 if (node == null) return nothing;
 if (Array.isArray(node)) return node.map((child) => renderTree(child, evaluationContext));
 if (!isElementNode(node)) return node;
 const renderer = wikiLibrary.components[node.typeName]?.component as WikiRenderer | null | undefined;
 if (!renderer) return nothing;
 const renderProps: Record<string, unknown> = {};
 for (const [key, value] of Object.entries(node.props)) {
  renderProps[key] = renderPropValue(value, evaluationContext ?? { getState: () => undefined, resolveRef: () => undefined });
 }
 return renderer(renderProps);
}

function renderPropValue(value: unknown, evaluationContext: EvaluationContext): unknown {
 if (value == null) return nothing;
 if (Array.isArray(value)) return value.map((v) => renderPropValue(v, evaluationContext));
 if (isElementNode(value)) return renderTree(value, evaluationContext);
 if (isReactiveAssign(value)) return stripReactiveAssign(value, evaluationContext);
 return value;
}

/** Convenience single-tree render for tests and static programs. */
export function renderNode(node: unknown): unknown {
 return renderTree(node);
}

export { isReactiveAssign, PAGE_APP_ACTION_EVENT };
export type { ToolNotFoundError };