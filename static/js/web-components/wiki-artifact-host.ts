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
import { createClient, type Client } from '@connectrpc/connect';
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

/** Read the page name from a tool args record, defaulting to this page. */
function argPage(args: Record<string, unknown>, fallbackPage: string): string {
 const page = args['page'];
 return typeof page === 'string' && page !== '' ? page : fallbackPage;
}

/** Read a required string tool arg. */
function argString(args: Record<string, unknown>, key: string): string {
 const value = args[key];
 return typeof value === 'string' ? value : '';
}

/**
 * An artifact app rendered from the page's OpenUI Lang program.
 */
export class WikiArtifactHost extends LitElement {
 static override readonly styles: CSSResultArray = [
  css`
      :host {
        display: block;
      }
      .wiki-artifact-app {
        position: relative;
        padding: 12px;
        border: 1px solid var(--color-border-subtle, #d4d4d8);
        border-radius: 8px;
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
    case 'api_v1_PageManagementService_GetPageOutline':
     return this.pageManagementClient.readPageOutline(
      create(ReadPageOutlineRequestSchema, { page }),
     );
    case 'api_v1_Frontmatter_GetFrontmatter':
     return this.frontmatterClient.getFrontmatter(
      create(GetFrontmatterRequestSchema, { page }),
     );
    case 'api_v1_MapService_ListMapElements':
     return this.mapClient.listMapElements(
      create(ListMapElementsRequestSchema, { page, mapName: argString(args, 'map_name') }),
     );
    case 'api_v1_ChecklistService_ListItems':
     return this.checklistClient.listItems(
      create(ListItemsRequestSchema, {
       page,
       listName: argString(args, 'list_name'),
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

 private actionContext() {
  return {
   // Flush the LIVE store snapshot with the CAS hash. Per server
   // semantics the page hash excludes artifact_state so no program
   // refetch is needed after a successful merge.
   setState: (snapshot: Record<string, unknown>) => {
    void (async () => {
     const state = plainToJsonObject(snapshot);
     try {
      const response = await this.stateClient.mergeArtifactState(
       create(MergeArtifactStateRequestSchema, {
        page: this.page,
        state,
        stateExpectedVersionHash: this.stateVersionHash,
       }),
      );
      this.stateVersionHash = response.stateVersionHash;
      this.renderTick += 1;
     } catch (err) {
      const error = AugmentErrorService.augmentError(err, 'saving artifact app state');
      if (error.originalError instanceof Error
       && (error.originalError as { code?: unknown }).code === 10) {
       // Aborted: refetch the fresh hash and remerge once.
       await this.hydrateState();
       try {
        const retry = await this.stateClient.mergeArtifactState(
         create(MergeArtifactStateRequestSchema, {
          page: this.page,
          state,
          stateExpectedVersionHash: this.stateVersionHash,
         }),
        );
        this.stateVersionHash = retry.stateVersionHash;
        this.renderTick += 1;
       } catch (retryErr) {
        this.actionError = AugmentErrorService.augmentError(retryErr, 'saving artifact app state');
       }
      } else {
       this.actionError = error;
      }
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
    globalThis.open(url, '_blank', 'noopener');
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
     const value = readStepValue(step.valueAST, store);
     if (value !== undefined) {
      store.set(step.target, value);
     }
     break;
    }
    case 'reset': {
     for (const target of step.targets) {
      store.set(target, store.get(target) ?? null);
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
  // Hydrate with the live store snapshot so re-renders keep prior state.
  const startState = this.runtime.store.getSnapshot();
  const persisted = Object.keys(startState).length > 0 ? startState : {};
  const rendered = this.runtime.render(this.programSource, persisted);
  this.lastErrors = rendered.errors;
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

function readStepValue(valueAst: unknown, store: { get(name: string): unknown }): unknown {
 if (valueAst == null || typeof valueAst !== 'object') return valueAst ?? undefined;
 if (!('k' in valueAst)) return valueAst;
 const ast = valueAst as { k: string; n?: string; v?: unknown };
 if (ast.k === 'StateRef' && typeof ast.n === 'string') return store.get(ast.n);
 if (ast.k === 'Str' || ast.k === 'Num' || ast.k === 'Bool') return ast.v;
 return null;
}

/** Convert a plain Record to a strict JsonObject (drops non-JSON values). */
function plainToJsonObject(value: Record<string, unknown>): JsonObject {
 const out: JsonObject = {};
 for (const [key, entry] of Object.entries(value)) {
  if (entry === null || typeof entry === 'string' || typeof entry === 'boolean'
   || (typeof entry === 'number' && Number.isFinite(entry))) {
   out[key] = entry;
  }
 }
 return out;
}

customElements.define('wiki-artifact-host', WikiArtifactHost);

declare global {
 interface HTMLElementTagNameMap {
  'wiki-artifact-host': WikiArtifactHost;
 }
}