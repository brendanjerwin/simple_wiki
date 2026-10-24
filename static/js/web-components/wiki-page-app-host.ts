/**
 * WikiPageAppHost — renders an OpenUI Lang page app program for a page.
 *
 * Lifecycle: GetPageApp → parse → render; GetPageAppState hydrates
 * persisted $variables; MergePageAppState flushes the LIVE lang-core store
 * snapshot with state_version_hash CAS (refetch + remerge once on Aborted;
 * per the server contract the page OCC hash excludes wiki.page_app_state,
 * so a successful flush never invalidates the program). WatchArtifact
 * (Connect server streaming) hot-swaps the program without a page reload.
 * Interactive components dispatch `wiki-page-app-action` events, which the
 * host executes (state sets, chat messages, URL opens, query reruns).
 * "Remove the app" clears the page app and reloads plain markdown.
 */
import { html, css, LitElement, nothing, type CSSResultArray, type TemplateResult } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient, ConnectError, Code, type Client } from '@connectrpc/connect';
import { create, type JsonObject } from '@bufbuild/protobuf';
import {
 PageAppService,
 PageAppStateService,
 ClearPageAppRequestSchema,
 DeletePageAppStateRequestSchema,
 GetPageAppRequestSchema,
 GetPageAppStateRequestSchema,
 MergePageAppStateRequestSchema,
 WatchPageAppRequestSchema,
 WatchPageAppResponse_Kind,
 type GetPageAppResponse,
} from '../gen/api/v1/page_app_pb.js';
import {
 ChatService,
 SendChatMessageRequestSchema,
} from '../gen/api/v1/chat_pb.js';
import {
 PageHistoryService,
 ListPageVersionsRequestSchema,
} from '../gen/api/v1/history_pb.js';
import {
 SearchService,
 SearchContentRequestSchema,
} from '../gen/api/v1/search_pb.js';
import {
 PageManagementService,
 ReadPageOutlineRequestSchema,
} from '../gen/api/v1/page_management_pb.js';
import {
 Frontmatter as FrontmatterService,
 GetFrontmatterRequestSchema,
} from '../gen/api/v1/frontmatter_pb.js';
import {
 MapService,
 ListMapElementsRequestSchema,
} from '../gen/api/v1/map_pb.js';
import {
 ChecklistService,
 ListItemsRequestSchema,
} from '../gen/api/v1/checklist_pb.js';
import {
 SurveyService,
 GetSurveyRequestSchema,
} from '../gen/api/v1/survey_pb.js';
import { getGrpcWebTransport } from './grpc-transport.js';
import { AugmentErrorService } from './augment-error-service.js';
import './error-display.js';
import {
 PAGE_APP_ACTION_EVENT,
 type PageAppActionEventDetail,
 type PlanStep,
} from '../openui-lang/library.js';
import {
 generateWikiPageAppRenderer,
 type WikiPageAppRuntime,
} from '../openui-lang/renderer.js';
import { createToolProvider } from '../openui-lang/toolProvider.js';
import {
 wikiPageAppDesignSystem,
} from '../openui-lang/page-app-design-system.js';

/** Read the page name from a tool args record, defaulting to this page. */
function argPage(args: Record<string, unknown>, fallbackPage: string): string {
 const page = args['page'];
 return typeof page === 'string' && page !== '' ? page : fallbackPage;
}

/** Read a required string tool arg. Programs follow the taught contract
 * (camelCase, matching the proto JSON names); snake_case is accepted as a
 * fallback since it appears in Go-side docs. */
function argString(args: Record<string, unknown>, camel: string, snake?: string): string {
 const direct = args[camel];
 if (typeof direct === 'string' && direct !== '') return direct;
 if (snake !== undefined) {
  const alt = args[snake];
  if (typeof alt === 'string' && alt !== '') return alt;
 }
 return '';
}

/**
 * An page app rendered from the page's OpenUI Lang program.
 */
export class WikiPageAppHost extends LitElement {
 static override readonly styles: CSSResultArray = [
  wikiPageAppDesignSystem,
  css`
      :host {
        display: block;
      }
      .wiki-page-app-app {
        position: relative;
        box-sizing: border-box;
        width: 100%;
        padding: clamp(10px, 2.5vw, 18px);
        border: 1px solid var(--color-border-subtle, #d4d4d8);
        border-radius: 12px;
        background: var(--color-surface-sunken, #f8f9fa);
        overflow: hidden;
      }
      .wiki-page-app-body {
        display: flex;
        flex-direction: column;
        gap: 10px;
        min-width: 0;
      }
      .wiki-page-app-remove {
        position: absolute;
        top: 8px;
        right: 8px;
        border: none;
        background: transparent;
        color: var(--color-text-muted, #71717a);
        cursor: pointer;
        font-size: 12px;
      }
      .wiki-page-app-remove:hover {
        color: var(--color-danger, #ef4444);
      }
      .wiki-page-app-parse-errors {
        margin-top: 8px;
        padding: 8px;
        font-size: 12px;
        color: var(--color-warning, #ca8a04);
      }
      .reconnecting-note {
        margin: 8px 0 0;
        padding: 6px 10px;
        font-size: 0.8rem;
        color: var(--color-text-muted, #71717a);
        background: var(--color-surface-sunken, #f8f9fa);
        border-radius: 6px;
        animation: page-app-reconnect-pulse 1.5s ease-in-out infinite;
      }
      .wiki-page-app-reconnecting .reconnecting-note {
        margin: 0;
      }
      @keyframes page-app-reconnect-pulse {
        0%, 100% { opacity: 0.6; }
        50% { opacity: 1; }
      }
    `,
 ];

 /** The page app's page identifier. */
 @property({ type: String })
 declare page: string;

 @state()
 declare private programSource: string;

 @state()
 declare private pageApp: GetPageAppResponse | null;

 @state()
 declare private stateVersionHash: string;

 @state()
 declare private loadError: Error | null;

 @state()
 declare private actionError: Error | null;

 @state()
 declare private removed: boolean;

 @state()
 declare private renderTick: number;

 @state()
 declare private hydrated: boolean;

 private readonly pageAppClient: Client<typeof PageAppService>;
 private readonly stateClient: Client<typeof PageAppStateService>;
 private readonly chatClient: Client<typeof ChatService>;
 private readonly historyClient: Client<typeof PageHistoryService>;
 private readonly searchClient: Client<typeof SearchService>;
 private readonly pageManagementClient: Client<typeof PageManagementService>;
 private readonly frontmatterClient: Client<typeof FrontmatterService>;
 private readonly mapClient: Client<typeof MapService>;
 private readonly checklistClient: Client<typeof ChecklistService>;
 private readonly surveyClient: Client<typeof SurveyService>;

 private stopWatch: (() => void) | null = null;
 private unsubs: (() => void)[] = [];
 private runtime: WikiPageAppRuntime | null = null;
 /** Exponential-backoff retry machinery for transient load/watch failures. */
 private retryTimer: number | null = null;
 private retryAttempt = 0;
 /** True while a reconnect is pending — renders "reconnecting" instead of an error. */
 @state()
 declare private reconnecting: boolean;
 /** Window/document wake triggers, bound once and removed on disconnect. */
 private readonly wakeOnline = (): void => { this.recoverIfDegraded(); };
 private readonly wakeVisible = (): void => {
  if (document.visibilityState === 'visible') this.recoverIfDegraded();
 };

 constructor() {
  super();
  this.page = '';
  this.programSource = '';
  this.pageApp = null;
  this.stateVersionHash = '';
  this.loadError = null;
  this.actionError = null;
  this.removed = false;
  this.renderTick = 0;
  this.hydrated = false;
  const transport = getGrpcWebTransport();
  this.pageAppClient = createClient(this.createPageAppService(), transport);
  this.stateClient = createClient(this.createPageAppStateService(), transport);
  this.chatClient = createClient(ChatService, transport);
  this.historyClient = createClient(PageHistoryService, transport);
  this.searchClient = createClient(SearchService, transport);
  this.pageManagementClient = createClient(PageManagementService, transport);
  this.frontmatterClient = createClient(FrontmatterService, transport);
  this.mapClient = createClient(MapService, transport);
  this.checklistClient = createClient(ChecklistService, transport);
  this.surveyClient = createClient(SurveyService, transport);
 }

 /** Test seam: which service descriptors the clients use. */
 protected createPageAppService() {
  return PageAppService;
 }

 /** Test seam: which service descriptors the clients use. */
 protected createPageAppStateService() {
  return PageAppStateService;
 }

 static get is(): string {
  return 'wiki-page-app-host';
 }

 override connectedCallback(): void {
  super.connectedCallback();
  // Wake triggers: on network return or tab re-visible, attempt an
  // immediate recovery if the ladder is degraded. These make reconnects
  // happen at the MOMENT connectivity returns rather than waiting out
  // the backoff clock (critical when WiFi reassociates slower than the
  // 5 remaining retries).
  window.addEventListener('online', this.wakeOnline);
  document.addEventListener('visibilitychange', this.wakeVisible);
  // Library templates render into this host's shadow DOM via
  // .wiki-page-app-body; the design system is included in static styles
  // below so theme variables + responsive rules apply to them.
  this.runtime = generateWikiPageAppRenderer(
   this.toolProvider(),
   this.actionContext(),
  );
  // Execute page app action events from the rendered tree.
  this.addEventListener(PAGE_APP_ACTION_EVENT, (event) => {
   event.stopPropagation();
   // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- addEventListener can't type a string-const event name
   this.handlePageAppAction(event as CustomEvent<PageAppActionEventDetail>);
  });
  void this.load();
 }

 override disconnectedCallback(): void {
  super.disconnectedCallback();
  window.removeEventListener('online', this.wakeOnline);
  document.removeEventListener('visibilitychange', this.wakeVisible);
  this.stopWatch?.();
  this.stopWatch = null;
  for (const unsub of this.unsubs) unsub();
  this.unsubs = [];
  this.cancelRetry();
 }

 // ---------------------------------------------------------------------------
 // transient-failure recovery
 //
 // Browser suspension (tab park, laptop sleep) kills the WatchArtifact
 // stream and can fail the initial/hydrate RPCs. Those failures are
 // transient: retry with exponential backoff + jitter instead of parking
 // the app behind a manual-refresh error screen. NotFound is terminal —
 // the page genuinely has no page app.

 /** Terminal ConnectError codes that must NOT be retried. */
 private static isTerminal(err: unknown): boolean {
  if (err instanceof ConnectError) {
   return err.code === Code.NotFound
    || err.code === Code.InvalidArgument
    || err.code === Code.PermissionDenied
    || err.code === Code.Unauthenticated
    || err.code === Code.FailedPrecondition;
  }
  return false;
 }

 private static readonly RETRY_MAX_ATTEMPTS = 6;
 private static readonly RETRY_BASE_MS = 1000;
 private static readonly RETRY_MAX_MS = 30_000;

 /**
  * Compute the next backoff delay: exponential base with full jitter
  * (random between 0 and min(cap, base*2^attempt)) — full jitter avoids
  * synchronized retry storms across many tabs.
  */
 private static backoffDelayMs(attempt: number): number {
  const exp = WikiPageAppHost.RETRY_BASE_MS * 2 ** attempt;
  const bounded = Math.min(exp, WikiPageAppHost.RETRY_MAX_MS);
  return Math.floor(Math.random() * bounded);
 }

 private cancelRetry(): void {
  if (this.retryTimer !== null) {
   clearTimeout(this.retryTimer);
   this.retryTimer = null;
  }
 }

 /**
  * Wake trigger (window online / tab re-visible): if the host is
  * degraded — ladder running, exhausted, or errored — attempt an
  * immediate full recovery instead of waiting out the remaining backoff
  * (or sitting parked after the ladder gave up). The ladder reset here
  * is deliberate: connectivity returning is exactly the condition that
  * makes retries viable again.
  */
 private recoverIfDegraded(): void {
  if (!this.isConnected) return;
  if (this.reconnecting || this.loadError != null || this.retryAttempt > 0) {
   this.cancelRetry();
   this.retryAttempt = 0;
   void this.load();
  }
 }

 /**
  * Schedule the next recovery attempt for a transient failure. Shows the
  * reconnecting banner (clearing any prior error display) while waiting.
  * Gives up after RETRY_MAX_ATTEMPTS consecutive failures and surfaces
  * the last error.
  */
 private scheduleRetry(stage: 'load' | 'watch', lastError: unknown): void {
  // Post-disconnect failures must never resurrect the ladder — Lit
  // flips isConnected when the element leaves the DOM.
  if (!this.isConnected) return;
  if (this.retryAttempt >= WikiPageAppHost.RETRY_MAX_ATTEMPTS) {
   this.retryAttempt = 0;
   this.reconnecting = false;
   this.loadError = AugmentErrorService.augmentError(lastError, `page app ${stage} failed after ${WikiPageAppHost.RETRY_MAX_ATTEMPTS} retries — refresh to retry again`);
   this.renderTick += 1;
   return;
  }
  const delay = WikiPageAppHost.backoffDelayMs(this.retryAttempt);
  this.retryAttempt += 1;
  this.loadError = null;
  this.reconnecting = true;
  this.renderTick += 1;
  this.retryTimer = window.setTimeout(() => {
   this.retryTimer = null;
   if (!this.isConnected) return;
   if (stage === 'watch') {
    this.subscribeWatch();
   } else {
    void this.load();
   }
  }, delay);
 }

 /**
  * One recovery attempt of the full ladder: GetPageApp → hydrate →
  * rewatch. Resets the ladder (retryAttempt, reconnecting, loadError) only
  * on a fully recovered GetPageApp + hydrate + rewatch — consecutive
  * hydrate failures still escalate to the cap.
  */
 private async load(): Promise<void> {
  if (!this.page) {
   this.loadError = new Error('wiki-page-app-host requires a page attribute');
   return;
  }
  this.cancelRetry();
  try {
   const response = await this.pageAppClient.getPageApp(
    create(GetPageAppRequestSchema, { page: this.page }),
   );
   await this.applyLoadedProgram(response);
   this.renderTick += 1;
  } catch (err) {
   if (err instanceof Error && err.message.includes('NotFound')) {
    // Terminal: this page genuinely has no page app.
    this.reconnecting = false;
    this.renderTick += 1;
    return;
   }
   if (!this.isConnected) return;
   this.scheduleRetry('load', err);
  }
 }

 private async applyLoadedProgram(response: GetPageAppResponse): Promise<void> {
  if (response.source === '') {
   // Full success path (no page app): clear the ladder only here.
   this.retryAttempt = 0;
   this.reconnecting = false;
   this.loadError = null;
   this.pageApp = null;
   this.programSource = '';
  } else {
   this.pageApp = response;
   this.programSource = response.source;
   this.hydrated = true;
   await this.hydrateState();
   if (this.loadError != null) {
    // hydrateState hit a transient failure it couldn't retry inline —
    // it has already scheduled the retry ladder. The ladder reset
    // happens on the retry's success, NOT here, so consecutive hydrate
    // failures still escalate to the cap.
    return;
   }
   // Fully recovered (GetPageApp + hydrate + rewatch): only now reset
   // the ladder. Stale unsubs from the pre-suspension runtime are torn
   // down before re-subscribing (they'd double-fire notifications).
   this.retryAttempt = 0;
   this.reconnecting = false;
   this.loadError = null;
   this.subscribeWatch();
   // Query results ($queries) resolve asynchronously; without these
   // subscriptions the rendered table never repaints after its RPC
   // completes. Store subscription covers @Set/@Reset repaints too.
   const runtime = this.runtime;
   if (runtime != null) {
    for (const unsub of this.unsubs) unsub();
    this.unsubs = [
     runtime.queryManager.subscribe(() => { this.renderTick += 1; }),
     runtime.store.subscribe(() => { this.renderTick += 1; }),
    ];
   }
  }
  this.renderTick += 1;
 }

 private async hydrateState(): Promise<void> {
  try {
   const response = await this.stateClient.getPageAppState(
    create(GetPageAppStateRequestSchema, { page: this.page }),
   );
   this.stateVersionHash = response.stateVersionHash;
   // Seed the live store with the persisted state (cross-device truth).
   // The program's declarations are merged over these in render() as
   // initialize(defaults, persisted) — persisted wins, pristine keys
   // restore defaults, which is the designed hydrate rule.
   // response.state is already a JsonObject in protobuf-es v2 — use it
   // directly; the outbound direction needs plainToJsonObject.
   const persistedState = (response.state ?? {}) as Record<string, unknown>;
   if (this.runtime != null && Object.keys(persistedState).length > 0) {
    this.runtime.store.initialize({}, persistedState);
   }
  } catch (err) {
   if (WikiPageAppHost.isTerminal(err)) {
    // No persisted state yet is fine; a merge will mint the first hash.
    return;
   }
   if (!this.isConnected) return;
   // Transient (suspension-wake network hiccup): retry the WHOLE load
   // with backoff — the ladder re-runs GetPageApp + hydrate together.
   this.scheduleRetry('load', err);
  }
 }

 private subscribeWatch(): void {
  this.stopWatch?.();
  const abort = new AbortController();
  const signal = abort.signal;
  this.stopWatch = () => abort.abort();
  void (async () => {
   let watchError: unknown = null;
   try {
    const request = create(WatchPageAppRequestSchema, { page: this.page });
    for await (const evt of this.pageAppClient.watchPageApp(request, { signal })) {
     this.handleWatchEvent(evt.kind, evt.source);
    }
   } catch (err) {
    watchError = err;
   }
   if (signal.aborted) return;
   // The stream died (suspension, network flap, server restart).
   // A graceful end-of-stream also lands here; the recovery ladder
   // re-runs the full load (GetPageApp → hydrate → watch), which gets
   // the current program + a fresh stream either way.
   this.stopWatch = null;
   if (watchError !== null && WikiPageAppHost.isTerminal(watchError)) {
    this.loadError = AugmentErrorService.augmentError(watchError, 'watching page app');
    this.renderTick += 1;
    return;
   }
   if (!this.isConnected) return;
   this.scheduleRetry('load', watchError ?? new Error('page app watch stream ended unexpectedly'));
  })();
 }

 private handleWatchEvent(kind: WatchPageAppResponse_Kind, source: string): void {
  switch (kind) {
   case WatchPageAppResponse_Kind.INITIAL:
   case WatchPageAppResponse_Kind.UPDATED:
    this.programSource = source;
    break;
   case WatchPageAppResponse_Kind.CLEARED:
    this.removed = true;
    this.programSource = '';
    break;
   default:
    break;
  }
  this.renderTick += 1;
 }

 private toolProvider() {
  return createToolProvider(async (qualifiedToolId, args) => {
   const page = argPage(args, this.page);
   switch (qualifiedToolId) {
    case 'api_v1_PageAppService_GetPageApp':
     return this.pageAppClient.getPageApp(create(GetPageAppRequestSchema, { page }));
    case 'api_v1_PageAppStateService_GetPageAppState':
     return this.stateClient.getPageAppState(create(GetPageAppStateRequestSchema, { page }));
    case 'api_v1_PageAppStateService_MergePageAppState':
     return this.stateClient.mergePageAppState(
      create(MergePageAppStateRequestSchema, {
       page,
       state: (() => {
        const raw = args['state'];
        return raw != null && typeof raw === 'object' && !Array.isArray(raw)
         ? // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- jsonValueToPlain guarantees an object result for object input
         jsonValueToPlain(raw, new Set()) as JsonObject
         : {};
       })(),
       stateExpectedVersionHash: argString(args, 'stateExpectedVersionHash'),
      }),
     );
    case 'api_v1_PageAppStateService_DeletePageAppState':
     return this.stateClient.deletePageAppState(
      create(DeletePageAppStateRequestSchema, {
       page,
       stateExpectedVersionHash: argString(args, 'stateExpectedVersionHash'),
      }),
     );
    case 'api_v1_PageHistoryService_ListPageVersions':
     return this.historyClient.listPageVersions(
      create(ListPageVersionsRequestSchema, { page }),
     );
    case 'api_v1_SearchService_SearchContent':
     return this.searchClient.searchContent(
      create(SearchContentRequestSchema, { query: argString(args, 'query') }),
     );
    case 'api_v1_PageManagementService_ReadPageOutline':
     return this.pageManagementClient.readPageOutline(
      create(ReadPageOutlineRequestSchema, { page }),
     );
    case 'api_v1_Frontmatter_GetFrontmatter':
     return this.frontmatterClient.getFrontmatter(
      create(GetFrontmatterRequestSchema, { page }),
     );
    case 'api_v1_MapService_ListMapElements':
     return this.mapClient.listMapElements(
      create(ListMapElementsRequestSchema, { page, mapName: argString(args, 'mapName', 'map_name') }),
     );
    case 'api_v1_ChecklistService_ListItems':
     return this.checklistClient.listItems(
      create(ListItemsRequestSchema, {
       page,
       listName: argString(args, 'listName', 'list_name'),
      }),
     );
    case 'api_v1_SurveyService_GetSurvey':
     return this.surveyClient.getSurvey(
      create(GetSurveyRequestSchema, { page, name: argString(args, 'name') }),
     );
    default:
     throw new Error(`no rpc wired for allowlisted tool: ${qualifiedToolId}`);
   }
  });
 }

 /**
  * CAS'd state flush: merge the LIVE store snapshot, and on an OCC
  * Aborted conflict refetch ONLY the fresh state hash (never re-seed the
  * store — that would revert the in-memory $vars mid-flush) and re-merge
  * from the live snapshot. Exactly one retry; a second conflict
  * surfaces as an action error.
  */
 private async mergeStateWithRetry(snapshot: Record<string, unknown>): Promise<void> {
  const attempt = async (): Promise<void> => {
   const response = await this.stateClient.mergePageAppState(
    create(MergePageAppStateRequestSchema, {
     page: this.page,
     state: plainToJsonObject(snapshot),
     stateExpectedVersionHash: this.stateVersionHash,
    }),
   );
   this.stateVersionHash = response.stateVersionHash;
   this.renderTick += 1;
  };
  try {
   await attempt();
  } catch (err) {
   if (err instanceof ConnectError && err.code === Code.Aborted) {
    // Conflict: refetch the hash only, then re-merge the live snapshot.
    const fresh = await this.stateClient.getPageAppState(
     create(GetPageAppStateRequestSchema, { page: this.page }),
    );
    this.stateVersionHash = fresh.stateVersionHash;
    try {
     await attempt();
    } catch (retryErr) {
     this.actionError = AugmentErrorService.augmentError(retryErr, 'saving page app state');
    }
    return;
   }
   throw err;
  }
 }

 private actionContext() {
  return {
   // Flush the LIVE store snapshot with the CAS hash. Per server
   // semantics the page hash excludes page app_state so no program
   // refetch is needed after a successful merge.
   setState: (snapshot: Record<string, unknown>) => {
    void (async () => {
     try {
      await this.mergeStateWithRetry(snapshot);
     } catch (err) {
      // mergeStateWithRetry surfaces its own terminal error; anything
      // thrown before that (arg building) is a programming error.
      this.actionError = AugmentErrorService.augmentError(err, 'saving page app state');
     }
    })();
   },
   toAssistant: (message: string) => {
    void (async () => {
     try {
      await this.chatClient.sendMessage(
       create(SendChatMessageRequestSchema, { page: this.page, content: message }),
      );
     } catch (err) {
      this.actionError = AugmentErrorService.augmentError(err, 'sending page app message');
     }
    })();
   },
   openUrl: (url: string) => {
    // Scheme guard: an agent-written program is data, but @OpenUrl is a
    // viewer-side side effect — javascript:/data: executes in this
    // origin. Only http(s) opens; anything else surfaces as an
    // action error.
    let parsed: URL;
    try {
     parsed = new URL(url, globalThis.location.origin);
    } catch {
     this.actionError = new Error(`@OpenUrl: invalid URL ${JSON.stringify(url)}`);
     return;
    }
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
     this.actionError = new Error(`@OpenUrl: only http(s) URLs can be opened, got ${parsed.protocol}`);
     return;
    }
    globalThis.open(parsed.toString(), '_blank', 'noopener');
   },
   rerun: (_statementIds?: string[]) => {
    this.renderTick += 1;
   },
  };
 }

 /** Restore one reset target to its DECLARED default and re-mark it
  * pristine, so the next render's initialize() restores the default
  * instead of the pre-reset persisted value. */
 private applyResetStep(step: PlanStep): void {
  const runtime = this.runtime;
  if (runtime == null || this.lastDeclarations == null) return;
  const declarations = this.lastDeclarations as Record<string, unknown>;
  for (const target of step.targets ?? []) {
   if (target in declarations) {
    runtime.store.set(target, declarations[target], { pristine: true });
   }
  }
 }

 /** Apply one @Set step: form controls dispatch actions with the current
  * control value (`detail.value`); lang-core's onChange steps reference it
  * as `$value` — e.g. Button/Input with @Set($var, $value). */
 private applySetStep(step: PlanStep, inputValue: unknown): void {
  const runtime = this.runtime;
  if (runtime == null) return;
  const value = runtime.evaluateStepValue(step.valueAST, { $value: inputValue });
  if ((value === undefined || value === null) || step.target == null) return;
  runtime.store.set(step.target, value);
 }

 private runToAssistantStep(step: PlanStep): void {
  if (step.message != null) this.actionContext().toAssistant(step.message);
 }

 private runOpenUrlStep(step: PlanStep): void {
  if (step.url != null) this.actionContext().openUrl(step.url);
 }

 private runRefetchStep(step: PlanStep): void {
  const runtime = this.runtime;
  if (runtime != null && step.statementId != null) runtime.refetch([step.statementId]);
 }

 /** Execute one action-plan step against the live runtime. Store
  * mutations go through apply*Step; side-effect steps route through
  * actionContext instead. */
 private executePlanStep(step: PlanStep, inputValue: unknown): void {
  const mutators: Record<string, (step: PlanStep, value: unknown) => void> = {
   set: (st, value) => this.applySetStep(st, value),
   reset: (st) => this.applyResetStep(st),
  };
  const sideEffects: Record<string, (step: PlanStep) => void> = {
   continue_conversation: (st) => this.runToAssistantStep(st),
   open_url: (st) => this.runOpenUrlStep(st),
   run: (st) => this.runRefetchStep(st),
  };
  // Unknown step types (incl. custom_action) are inert by design.
  const mutator = mutators[step.type];
  if (mutator != null) mutator(step, inputValue);
  sideEffects[step.type]?.(step);
 }

 private handlePageAppAction(event: CustomEvent<PageAppActionEventDetail>): void {
  const plan = event.detail.plan;
  if (this.runtime == null) return;
  for (const step of plan?.steps ?? []) {
   this.executePlanStep(step, event.detail.value);
  }
  // Flush once per event (not per step): the plan may batch several @Set
  // steps and each merge is a separate CAS'd RPC. Every action event
  // flushes — matching the pre-refactor behavior — even when no step
  // mutated the store.
  this.actionContext().setState(this.runtime.store.getSnapshot());
 }

 private async removeApp(): Promise<void> {
  try {
   await this.pageAppClient.clearPageApp(
    create(ClearPageAppRequestSchema, {
     page: this.page,
     expectedVersionHash: this.pageApp?.versionHash ?? '',
    }),
   );
   this.removed = true;
   // The page app is gone: the page shell reloads plain markdown. The
   // host never reloads itself (a reload inside WTR/preview kills the
   // surrounding page's state); listeners decide what to do.
   this.dispatchEvent(
    new CustomEvent('wiki-page-app-removed', { bubbles: true, composed: true }),
   );
   this.requestUpdate();
  } catch (err) {
   this.actionError = AugmentErrorService.augmentError(err, 'removing page app');
  }
 }

 private lastErrors: { code: string; message: string }[] = [];
 /** The current program's $variable declarations — @Reset's defaults. */
 private lastDeclarations: Record<string, unknown> | null = null;

 override render(): TemplateResult {
  if (this.removed) {
   return html`<p class="plain-markdown-note">Artifact removed — reload to return to plain markdown.</p>`;
  }
  // Reconnecting (transient failure during retry ladder): show a
  // non-blocking banner INSTEAD of the error screen — the app comes
  // back on its own when the connection re-establishes.
  if (this.reconnecting) {
   if (!this.hydrated) {
    return html`<div class="wiki-page-app-app wiki-page-app-reconnecting" aria-busy="true">
        <p class="reconnecting-note">Reconnecting to page app…</p>
      </div>`;
   }
   // Hydrated: keep rendering the current app, banner inline below it.
   return html`<div class="wiki-page-app-app">
      ${this.renderAppBody()}
      <p class="reconnecting-note" role="status">Connection interrupted — reconnecting…</p>
    </div>`;
  }
  if (this.loadError != null) {
   return html`<error-display .augmentedError=${this.loadError}></error-display>`;
  }
  if (this.actionError != null) {
   return html`<error-display .augmentedError=${this.actionError}></error-display>`;
  }
  if (!this.hydrated || this.programSource === '' || this.runtime == null) {
   return html`<div class="wiki-page-app-app" aria-busy="true"></div>`;
  }
  return html`<div class="wiki-page-app-app">
      ${this.renderAppBody()}
    </div>`;
 }

 /** The rendered app tree: remove button, program body, parse errors. */
 private renderAppBody(): TemplateResult {
  // The live store is the in-session truth: hydration from
  // GetPageAppState happened once in hydrateState(); re-renders pass an
  // empty persisted map, so a RESET key re-marked pristine keeps its
  // declared default instead of being re-imported from the old snapshot.
  const rendered = this.runtime!.render(this.programSource, {});
  this.lastErrors = rendered.errors;
  this.lastDeclarations = rendered.stateDeclarations;
  return html`<button class="wiki-page-app-remove" @click=${() => void this.removeApp()}>Remove the app</button>
      <div class="wiki-page-app-body">${rendered.template}</div>
      ${this.renderParseErrors()}`;
 }

 private renderParseErrors(): TemplateResult | typeof nothing {
  if (this.lastErrors.length === 0) return nothing;
  return html`<div class="wiki-page-app-parse-errors">
      ${this.lastErrors.map((err) => html`<div>${err.message}</div>`)}
    </div>`;
 }
}

/** Convert a plain Record to a strict JsonObject, recursively (keeps
 * nested objects/arrays — the state service deep-merges both). */
function plainToJsonObject(value: Record<string, unknown>): JsonObject {
 const out = jsonValueToPlain(value, new Set());
 if (out != null && typeof out === 'object' && !Array.isArray(out)) {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- jsonValueToPlain guarantees an object result for object input
  return out as JsonObject;
 }
 return {};
}

const MAX_JSON_DEPTH = 16;

function jsonValueToPlain(value: unknown, seen: Set<object>): unknown {
 if (value === null || typeof value === 'string' || typeof value === 'boolean') return value;
 if (typeof value === 'number') return Number.isFinite(value) ? value : null;
 if (Array.isArray(value)) {
  if (seen.has(value) || seen.size >= MAX_JSON_DEPTH) return null;
  seen.add(value);
  return value.map((v) => jsonValueToPlain(v, seen));
 }
 if (typeof value === 'object') {
  if (seen.has(value) || seen.size >= MAX_JSON_DEPTH) return null;
  seen.add(value);
  const out: Record<string, unknown> = {};
  if (value === null || typeof value !== 'object') return null;
  for (const [key, entry] of Object.entries(value)) {
   out[key] = jsonValueToPlain(entry, seen);
  }
  return out;
 }
 // Functions, symbols, undefined: not JSON — drop.
 return null;
}

customElements.define('wiki-page-app-host', WikiPageAppHost);

declare global {
 interface HTMLElementTagNameMap {
  'wiki-page-app-host': WikiPageAppHost;
 }
}