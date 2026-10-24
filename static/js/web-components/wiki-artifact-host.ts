/**
 * WikiArtifactHost — renders an OpenUI Lang artifact program for a page.
 *
 * Lifecycle: GetArtifact → parse → render; GetArtifactState hydrates
 * persisted $variables; MergeArtifactState flushes the LIVE lang-core store
 * snapshot with state_version_hash CAS (refetch + remerge once on Aborted;
 * per the server contract the page OCC hash excludes wiki.artifact_state,
 * so a successful flush never invalidates the program). WatchArtifact
 * (Connect server streaming) hot-swaps the program without a page reload.
 * Interactive components dispatch `wiki-artifact-action` events, which the
 * host executes (state sets, chat messages, URL opens, query reruns).
 * "Remove the app" clears the artifact and reloads plain markdown.
 */
import { html, css, LitElement, nothing, type CSSResultArray, type TemplateResult } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient, ConnectError, Code, type Client } from '@connectrpc/connect';
import { create, type JsonObject } from '@bufbuild/protobuf';
import {
 ArtifactService,
 ArtifactStateService,
 ClearArtifactRequestSchema,
 DeleteArtifactStateRequestSchema,
 GetArtifactRequestSchema,
 GetArtifactStateRequestSchema,
 MergeArtifactStateRequestSchema,
 WatchArtifactRequestSchema,
 WatchArtifactResponse_Kind,
 type GetArtifactResponse,
} from '../gen/api/v1/artifact_pb.js';
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
 ARTIFACT_ACTION_EVENT,
 type ArtifactActionEventDetail,
} from '../openui-lang/library.js';
import {
 generateWikiRenderer,
 type WikiArtifactRuntime,
} from '../openui-lang/renderer.js';
import { createToolProvider } from '../openui-lang/toolProvider.js';
import {
 wikiArtifactDesignSystem,
} from '../openui-lang/design-system.js';

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
 * An artifact app rendered from the page's OpenUI Lang program.
 */
export class WikiArtifactHost extends LitElement {
 static override readonly styles: CSSResultArray = [
  wikiArtifactDesignSystem,
  css`
      :host {
        display: block;
      }
      .wiki-artifact-app {
        position: relative;
        box-sizing: border-box;
        width: 100%;
        padding: clamp(10px, 2.5vw, 18px);
        border: 1px solid var(--color-border-subtle, #d4d4d8);
        border-radius: 12px;
        background: var(--color-surface-sunken, #f8f9fa);
        overflow: hidden;
      }
      .wiki-artifact-body {
        display: flex;
        flex-direction: column;
        gap: 10px;
        min-width: 0;
      }
      .wiki-artifact-remove {
        position: absolute;
        top: 8px;
        right: 8px;
        border: none;
        background: transparent;
        color: var(--color-text-muted, #71717a);
        cursor: pointer;
        font-size: 12px;
      }
      .wiki-artifact-remove:hover {
        color: var(--color-danger, #ef4444);
      }
      .wiki-artifact-parse-errors {
        margin-top: 8px;
        padding: 8px;
        font-size: 12px;
        color: var(--color-warning, #ca8a04);
      }
    `,
 ];

 /** The artifact's page identifier. */
 @property({ type: String })
 declare page: string;

 @state()
 declare private programSource: string;

 @state()
 declare private artifact: GetArtifactResponse | null;

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

 private artifactClient: Client<typeof ArtifactService>;
 private stateClient: Client<typeof ArtifactStateService>;
 private chatClient: Client<typeof ChatService>;
 private historyClient: Client<typeof PageHistoryService>;
 private searchClient: Client<typeof SearchService>;
 private pageManagementClient: Client<typeof PageManagementService>;
 private frontmatterClient: Client<typeof FrontmatterService>;
 private mapClient: Client<typeof MapService>;
 private checklistClient: Client<typeof ChecklistService>;
 private surveyClient: Client<typeof SurveyService>;

 private stopWatch: (() => void) | null = null;
 private unsubs: (() => void)[] = [];
 private runtime: WikiArtifactRuntime | null = null;

 constructor() {
  super();
  this.page = '';
  this.programSource = '';
  this.artifact = null;
  this.stateVersionHash = '';
  this.loadError = null;
  this.actionError = null;
  this.removed = false;
  this.renderTick = 0;
  this.hydrated = false;
  const transport = getGrpcWebTransport();
  this.artifactClient = createClient(this.createArtifactService(), transport);
  this.stateClient = createClient(this.createArtifactStateService(), transport);
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
 protected createArtifactService() {
  return ArtifactService;
 }

 /** Test seam: which service descriptors the clients use. */
 protected createArtifactStateService() {
  return ArtifactStateService;
 }

 static get is(): string {
  return 'wiki-artifact-host';
 }

 override connectedCallback(): void {
  super.connectedCallback();
  // Library templates render into this host's shadow DOM via
  // .wiki-artifact-body; the design system is included in static styles
  // below so theme variables + responsive rules apply to them.
  this.runtime = generateWikiRenderer(
   this.toolProvider(),
   this.actionContext(),
  );
  // Execute artifact action events from the rendered tree.
  this.addEventListener(ARTIFACT_ACTION_EVENT, (event) => {
   event.stopPropagation();
   this.handleArtifactAction(event as CustomEvent<ArtifactActionEventDetail>);
  });
  void this.load();
 }

 override disconnectedCallback(): void {
  super.disconnectedCallback();
  this.stopWatch?.();
  this.stopWatch = null;
  for (const unsub of this.unsubs) unsub();
  this.unsubs = [];
 }

 private async load(): Promise<void> {
  if (!this.page) {
   this.loadError = new Error('wiki-artifact-host requires a page attribute');
   return;
  }
  try {
   const response = await this.artifactClient.getArtifact(
    create(GetArtifactRequestSchema, { page: this.page }),
   );
   if (response.source === '') {
    this.artifact = null;
    this.programSource = '';
   } else {
    this.artifact = response;
    this.programSource = response.source;
    this.hydrated = true;
    await this.hydrateState();
    this.subscribeWatch();
    // Query results ($queries) resolve asynchronously; without these
    // subscriptions the rendered table never repaints after its RPC
    // completes. Store subscription covers @Set/@Reset repaints too.
    const runtime = this.runtime;
    if (runtime != null) {
     this.unsubs = [
      runtime.queryManager.subscribe(() => { this.renderTick += 1; }),
      runtime.store.subscribe(() => { this.renderTick += 1; }),
     ];
    }
   }
   this.renderTick += 1;
  } catch (err) {
   const error = AugmentErrorService.augmentError(err, 'loading artifact app');
   if (!(err instanceof Error && err.message.includes('NotFound'))) {
    this.loadError = error;
   }
  }
 }

 private async hydrateState(): Promise<void> {
  try {
   const response = await this.stateClient.getArtifactState(
    create(GetArtifactStateRequestSchema, { page: this.page }),
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
   // No persisted state yet is fine; a merge will mint the first hash.
   if (!AugmentErrorService.augmentError(err).message.includes('NotFound')) {
    this.loadError = AugmentErrorService.augmentError(err, 'reading artifact app state');
   }
  }
 }

 private subscribeWatch(): void {
  this.stopWatch?.();
  const abort = new AbortController();
  const signal = abort.signal;
  this.stopWatch = () => abort.abort();
  void (async () => {
   try {
    const request = create(WatchArtifactRequestSchema, { page: this.page });
    for await (const evt of this.artifactClient.watchArtifact(request, { signal })) {
     this.handleWatchEvent(evt.kind, evt.source);
    }
   } catch (err) {
    if (signal.aborted) return;
    this.loadError = AugmentErrorService.augmentError(err, 'watching artifact app');
   }
  })();
 }

 private handleWatchEvent(kind: WatchArtifactResponse_Kind, source: string): void {
  switch (kind) {
   case WatchArtifactResponse_Kind.INITIAL:
   case WatchArtifactResponse_Kind.UPDATED:
    this.programSource = source;
    break;
   case WatchArtifactResponse_Kind.CLEARED:
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
    case 'api_v1_ArtifactService_GetArtifact':
     return this.artifactClient.getArtifact(create(GetArtifactRequestSchema, { page }));
    case 'api_v1_ArtifactStateService_GetArtifactState':
     return this.stateClient.getArtifactState(create(GetArtifactStateRequestSchema, { page }));
    case 'api_v1_ArtifactStateService_MergeArtifactState':
     return this.stateClient.mergeArtifactState(
      create(MergeArtifactStateRequestSchema, {
       page,
       state: (args['state'] as JsonObject | undefined) ?? {},
       stateExpectedVersionHash: argString(args, 'stateExpectedVersionHash'),
      }),
     );
    case 'api_v1_ArtifactStateService_DeleteArtifactState':
     return this.stateClient.deleteArtifactState(
      create(DeleteArtifactStateRequestSchema, {
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
   const response = await this.stateClient.mergeArtifactState(
    create(MergeArtifactStateRequestSchema, {
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
    const fresh = await this.stateClient.getArtifactState(
     create(GetArtifactStateRequestSchema, { page: this.page }),
    );
    this.stateVersionHash = fresh.stateVersionHash;
    try {
     await attempt();
    } catch (retryErr) {
     this.actionError = AugmentErrorService.augmentError(retryErr, 'saving artifact app state');
    }
    return;
   }
   throw err;
  }
 }

 private actionContext() {
  return {
   // Flush the LIVE store snapshot with the CAS hash. Per server
   // semantics the page hash excludes artifact_state so no program
   // refetch is needed after a successful merge.
   setState: (snapshot: Record<string, unknown>) => {
    void (async () => {
     try {
      await this.mergeStateWithRetry(snapshot);
     } catch (err) {
      // mergeStateWithRetry surfaces its own terminal error; anything
      // thrown before that (arg building) is a programming error.
      this.actionError = AugmentErrorService.augmentError(err, 'saving artifact app state');
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
      this.actionError = AugmentErrorService.augmentError(err, 'sending artifact message');
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
   rerun: (statementIds?: string[]) => {
    void statementIds;
    this.renderTick += 1;
   },
  };
 }

 private handleArtifactAction(event: CustomEvent<ArtifactActionEventDetail>): void {
  const plan = event.detail.plan;
  if (this.runtime == null) return;
  const store = this.runtime.store;
  for (const step of plan?.steps ?? []) {
   switch (step.type) {
    case 'set': {
     const value = this.runtime.evaluateStepValue(step.valueAST);
     if (value !== undefined && value !== null) {
      store.set(step.target, value);
     }
     break;
    }
    case 'reset': {
     // Restore each target to its DECLARED default and re-mark it
     // pristine, so the next render's initialize() restores the default
     // instead of the pre-reset persisted value.
     if (this.lastDeclarations == null) break;
     for (const target of step.targets) {
      if (target in this.lastDeclarations) {
       store.set(target, (this.lastDeclarations as Record<string, unknown>)[target], { pristine: true });
      }
     }
     break;
    }
    case 'continue_conversation':
     this.actionContext().toAssistant(step.message);
     break;
    case 'open_url':
     this.actionContext().openUrl(step.url);
     break;
    case 'run':
     this.runtime.refetch([step.statementId]);
     break;
    case 'custom_action':
     break;
    default:
     break;
   }
  }
  this.actionContext().setState(store.getSnapshot());
 }

 private async removeApp(): Promise<void> {
  try {
   await this.artifactClient.clearArtifact(
    create(ClearArtifactRequestSchema, {
     page: this.page,
     expectedVersionHash: this.artifact?.versionHash ?? '',
    }),
   );
   this.removed = true;
   // The artifact is gone: the page shell reloads plain markdown. The
   // host never reloads itself (a reload inside WTR/preview kills the
   // surrounding page's state); listeners decide what to do.
   this.dispatchEvent(
    new CustomEvent('wiki-artifact-removed', { bubbles: true, composed: true }),
   );
   this.requestUpdate();
  } catch (err) {
   this.actionError = AugmentErrorService.augmentError(err, 'removing artifact app');
  }
 }

 private lastErrors: { code: string; message: string }[] = [];
 /** The current program's $variable declarations — @Reset's defaults. */
 private lastDeclarations: Record<string, unknown> | null = null;

 override render(): TemplateResult {
  if (this.removed) {
   return html`<p class="plain-markdown-note">Artifact removed — reload to return to plain markdown.</p>`;
  }
  if (this.loadError != null) {
   return html`<error-display .augmentedError=${this.loadError}></error-display>`;
  }
  if (this.actionError != null) {
   return html`<error-display .augmentedError=${this.actionError}></error-display>`;
  }
  if (!this.hydrated || this.programSource === '' || this.runtime == null) {
   return html`<div class="wiki-artifact-app" aria-busy="true"></div>`;
  }
  // The live store is the in-session truth: hydration from
  // GetArtifactState happened once in hydrateState(); re-renders pass an
  // empty persisted map, so a RESET key re-marked pristine keeps its
  // declared default instead of being re-imported from the old snapshot.
  const rendered = this.runtime.render(this.programSource, {});
  this.lastErrors = rendered.errors;
  this.lastDeclarations = rendered.stateDeclarations;
  return html`<div class="wiki-artifact-app">
      <button class="wiki-artifact-remove" @click=${() => void this.removeApp()}>Remove the app</button>
      <div class="wiki-artifact-body">${rendered.template}</div>
      ${this.renderParseErrors()}
    </div>`;
 }

 private renderParseErrors(): TemplateResult | typeof nothing {
  if (this.lastErrors.length === 0) return nothing;
  return html`<div class="wiki-artifact-parse-errors">
      ${this.lastErrors.map((err) => html`<div>${err.message}</div>`)}
    </div>`;
 }
}

/** Convert a plain Record to a strict JsonObject, recursively (keeps
 * nested objects/arrays — the state service deep-merges both). */
function plainToJsonObject(value: Record<string, unknown>): JsonObject {
 return jsonValueToPlain(value, new Set()) as JsonObject;
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
  for (const [key, entry] of Object.entries(value as Record<string, unknown>)) {
   out[key] = jsonValueToPlain(entry, seen);
  }
  return out;
 }
 // Functions, symbols, undefined: not JSON — drop.
 return null;
}

customElements.define('wiki-artifact-host', WikiArtifactHost);

declare global {
 interface HTMLElementTagNameMap {
  'wiki-artifact-host': WikiArtifactHost;
 }
}