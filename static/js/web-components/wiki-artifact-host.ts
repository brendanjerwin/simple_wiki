import { LitElement, html, css, nothing, type TemplateResult } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  ArtifactService,
  WatchArtifactRequestSchema,
  type WatchArtifactResponse,
} from '../gen/api/v1/artifact_pb.js';
import {
  createLibrary,
  defineComponent,
  parse,
  evaluateRoot,
  createStore,
  type ElementNode,
  type ComponentRenderProps,
  type Library,
  type EvalContext,
  type EvaluationContext,
} from '@openuidev/lang-core';
import { z } from 'zod';

// ---- Event types ----

/** Detail carried by 'artifact-to-assistant' dispatched by wiki-artifact-host. */
export interface ArtifactToAssistantDetail {
  message: string;
  page: string;
}

declare global {
  interface HTMLElementTagNameMap {
    'wiki-artifact-host': WikiArtifactHost;
  }

  interface GlobalEventHandlersEventMap {
    'artifact-to-assistant': CustomEvent<ArtifactToAssistantDetail>;
  }
}

const INITIAL_RECONNECT_DELAY_MS = 1000;
const MAX_RECONNECT_DELAY_MS = 30000;

// ---- wiki-core@1 component library (Lit renderer) ----

type LitRenderer = (props: ComponentRenderProps<Record<string, unknown>, TemplateResult>) => TemplateResult;

function renderChildren(renderNode: (value: unknown) => TemplateResult, children: unknown): TemplateResult {
  if (Array.isArray(children)) {
    return html`${children.map(c => renderNode(c))}`;
  }
  if (children != null) {
    return renderNode(children);
  }
  return html``;
}

function isElementNode(value: unknown): value is ElementNode {
  return (
    typeof value === 'object' &&
    value !== null &&
    'type' in value &&
    (value as { type: unknown }).type === 'element'
  );
}

const Stack = defineComponent<z.ZodObject<{ children: z.ZodUnknown }>, LitRenderer>({
  name: 'Stack',
  description: 'Vertical stack of elements.',
  props: z.object({ children: z.unknown().optional() }),
  component: ({ props, renderNode }) =>
    html`<div class="openui-stack" style="display:flex;flex-direction:column;gap:8px;">${renderChildren(renderNode, props['children'])}</div>`,
});

const Row = defineComponent<z.ZodObject<{ children: z.ZodUnknown }>, LitRenderer>({
  name: 'Row',
  description: 'Horizontal row of elements.',
  props: z.object({ children: z.unknown().optional() }),
  component: ({ props, renderNode }) =>
    html`<div class="openui-row" style="display:flex;flex-direction:row;gap:8px;flex-wrap:wrap;">${renderChildren(renderNode, props['children'])}</div>`,
});

const Text = defineComponent<z.ZodObject<{ label: z.ZodOptional<z.ZodString> }>, LitRenderer>({
  name: 'Text',
  description: 'Plain text content.',
  props: z.object({ label: z.string().optional() }),
  component: ({ props }) => html`<span class="openui-text">${String(props['label'] ?? '')}</span>`,
});

function renderHeadingByLevel(text: string, level: number): TemplateResult {
  switch (level) {
    case 1: return html`<h1 class="openui-heading">${text}</h1>`;
    case 3: return html`<h3 class="openui-heading">${text}</h3>`;
    case 4: return html`<h4 class="openui-heading">${text}</h4>`;
    case 5: return html`<h5 class="openui-heading">${text}</h5>`;
    case 6: return html`<h6 class="openui-heading">${text}</h6>`;
    default: return html`<h2 class="openui-heading">${text}</h2>`;
  }
}

const Heading = defineComponent<z.ZodObject<{ text: z.ZodOptional<z.ZodString>; level: z.ZodOptional<z.ZodNumber> }>, LitRenderer>({
  name: 'Heading',
  description: 'Heading element (h1-h6).',
  props: z.object({ text: z.string().optional(), level: z.number().int().min(1).max(6).optional() }),
  component: ({ props }) => {
    const level = Math.max(1, Math.min(6, Number(props['level'] ?? 2)));
    const text = String(props['text'] ?? '');
    return renderHeadingByLevel(text, level);
  },
});

const Button = defineComponent<z.ZodObject<{ label: z.ZodOptional<z.ZodString>; disabled: z.ZodOptional<z.ZodBoolean> }>, LitRenderer>({
  name: 'Button',
  description: 'Clickable button.',
  props: z.object({
    label: z.string().optional(),
    disabled: z.boolean().optional(),
    onClick: z.unknown().optional(),
  }),
  component: ({ props }) =>
    html`<button class="openui-button" ?disabled=${Boolean(props['disabled'])}>${String(props['label'] ?? 'Button')}</button>`,
});

const TextInput = defineComponent<z.ZodObject<{ value: z.ZodOptional<z.ZodString>; placeholder: z.ZodOptional<z.ZodString>; label: z.ZodOptional<z.ZodString> }>, LitRenderer>({
  name: 'TextInput',
  description: 'Text input field.',
  props: z.object({
    value: z.string().optional(),
    placeholder: z.string().optional(),
    label: z.string().optional(),
  }),
  component: ({ props }) =>
    html`<div class="openui-text-input"><label>${String(props['label'] ?? '')}</label><input type="text" .value=${String(props['value'] ?? '')} placeholder=${String(props['placeholder'] ?? '')}></div>`,
});

const Badge = defineComponent<z.ZodObject<{ label: z.ZodOptional<z.ZodString> }>, LitRenderer>({
  name: 'Badge',
  description: 'Small status badge.',
  props: z.object({ label: z.string().optional() }),
  component: ({ props }) =>
    html`<span class="openui-badge" style="background:#e0e0e0;border-radius:12px;padding:2px 8px;font-size:.75em;">${String(props['label'] ?? '')}</span>`,
});

const Card = defineComponent<z.ZodObject<{ title: z.ZodOptional<z.ZodString>; children: z.ZodOptional<z.ZodUnknown> }>, LitRenderer>({
  name: 'Card',
  description: 'Container card with optional title.',
  props: z.object({
    title: z.string().optional(),
    children: z.unknown().optional(),
  }),
  component: ({ props, renderNode }) =>
    html`<div class="openui-card" style="border:1px solid var(--border-color,#ccc);border-radius:6px;padding:12px;">
      ${props['title'] ? html`<div class="openui-card-title" style="font-weight:600;margin-bottom:8px;">${String(props['title'])}</div>` : nothing}
      ${renderChildren(renderNode, props['children'])}
    </div>`,
});

const Divider = defineComponent<z.ZodObject<Record<never, never>>, LitRenderer>({
  name: 'Divider',
  description: 'Horizontal divider.',
  props: z.object({}),
  component: () => html`<hr class="openui-divider" style="border:none;border-top:1px solid var(--border-color,#ccc);margin:8px 0;">`,
});

const wikiCoreLibrary: Library<LitRenderer> = createLibrary<LitRenderer>({
  components: [Stack, Row, Text, Heading, Button, TextInput, Badge, Card, Divider],
  root: 'Stack',
});

// ---- Renderer ----

function buildRenderNode(library: Library<LitRenderer>): (value: unknown) => TemplateResult {
  function renderNode(value: unknown): TemplateResult {
    if (value == null) return html``;
    if (Array.isArray(value)) {
      return html`${value.map(v => renderNode(v))}`;
    }
    if (isElementNode(value)) {
      const component = library.components[value.typeName];
      if (!component) {
        return html`<span style="color:red;">[Unknown component: ${value.typeName}]</span>`;
      }
      return component.component({ props: value.props, renderNode, statementId: value.statementId });
    }
    return html`${String(value)}`;
  }
  return renderNode;
}

function buildEvalCtx(store: ReturnType<typeof createStore>): EvalContext {
  const ctx: EvaluationContext = {
    getState: (name: string) => store.get(name),
    resolveRef: (_name: string) => null,
  };
  return { ctx, library: wikiCoreLibrary, store };
}

function parseAndRender(source: string): TemplateResult {
  if (!source.trim()) return html``;

  try {
    const result = parse(source, new Map(), 'Stack');
    if (!result.root) {
      return html`<pre class="openui-parse-error" style="color:var(--error-color,red);font-size:.85em;">No root element found in artifact program.</pre>`;
    }

    const store = createStore();
    const rootEl = evaluateRoot(result, buildEvalCtx(store));
    if (!rootEl) return html``;

    const renderNode = buildRenderNode(wikiCoreLibrary);
    const component = wikiCoreLibrary.components[rootEl.typeName];
    if (!component) {
      return html`<span style="color:red;">[Unknown root component: ${rootEl.typeName}]</span>`;
    }
    return component.component({ props: rootEl.props, renderNode, statementId: rootEl.statementId });
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return html`<pre class="openui-render-error" style="color:var(--error-color,red);font-size:.85em;white-space:pre-wrap;">Render error: ${msg}</pre>`;
  }
}

// ---- Component ----

/**
 * wiki-artifact-host — mounts an OpenUI Lang program from a wiki page's
 * artifact fence and renders it above the page's prose. Drives hot-swap via
 * WatchArtifact; state is persisted server-side via ArtifactStateService.
 */
export class WikiArtifactHost extends LitElement {
  static override styles = css`
    :host {
      display: block;
      margin-bottom: 1.5rem;
    }
    .artifact-container {
      border: 1px solid var(--border-color, #e0e0e0);
      border-radius: 6px;
      padding: 1rem;
      background: var(--surface-color, #fff);
    }
    .artifact-error {
      color: var(--error-color, #c00);
      font-size: 0.875rem;
      padding: 0.5rem;
      border: 1px solid currentColor;
      border-radius: 4px;
    }
    .artifact-loading {
      color: var(--muted-color, #888);
      font-style: italic;
    }
  `;

  @property({ type: String })
  declare page: string;

  @state()
  private declare _source: string;

  @state()
  private declare _loadError: Error | null;

  @state()
  private declare _isLoading: boolean;

  private _artifactAbort: AbortController | undefined;

  private readonly _artifactClient = createClient(ArtifactService, getGrpcWebTransport());

  constructor() {
    super();
    this.page = '';
    this._source = '';
    this._loadError = null;
    this._isLoading = true;
  }

  override connectedCallback() {
    super.connectedCallback();
    if (this.page) {
      void this._startWatching();
    }
  }

  override disconnectedCallback() {
    super.disconnectedCallback();
    this._stopWatching();
  }

  override updated(changed: Map<string, unknown>) {
    super.updated(changed);
    if (changed.has('page')) {
      this._stopWatching();
      if (this.page) void this._startWatching();
    }
  }

  private _stopWatching() {
    this._artifactAbort?.abort();
    this._artifactAbort = undefined;
  }

  private async _startWatching(): Promise<void> {
    this._stopWatching();
    this._artifactAbort = new AbortController();
    const signal = this._artifactAbort.signal;

    let reconnectDelayMs = INITIAL_RECONNECT_DELAY_MS;

    while (!signal.aborted) {
      try {
        const req = create(WatchArtifactRequestSchema, { page: this.page });
        for await (const res of this._artifactClient.watchArtifact(req, { signal })) {
          this._handleArtifactEvent(res);
        }
        break;
      } catch (err) {
        if ((err instanceof Error && err.name === 'AbortError') || signal.aborted) break;
        this._loadError = err instanceof Error ? err : new Error(String(err));
        this._isLoading = false;
        await this._delay(signal, reconnectDelayMs);
        reconnectDelayMs = Math.min(reconnectDelayMs * 2, MAX_RECONNECT_DELAY_MS);
      }
    }
  }

  private _handleArtifactEvent(res: WatchArtifactResponse) {
    this._isLoading = false;
    this._loadError = null;
    this._source = res.cleared ? '' : res.source;
  }

  private _delay(signal: AbortSignal, ms: number): Promise<void> {
    return new Promise(resolve => {
      const timer = setTimeout(resolve, ms);
      signal.addEventListener('abort', () => { clearTimeout(timer); resolve(); });
    });
  }

  override render() {
    if (this._isLoading) {
      return html`<div class="artifact-loading">Loading artifact…</div>`;
    }
    if (this._loadError) {
      return html`<div class="artifact-error">Failed to load artifact: ${this._loadError.message}</div>`;
    }
    if (!this._source) {
      return html``;
    }
    return html`<div class="artifact-container">${parseAndRender(this._source)}</div>`;
  }
}

customElements.define('wiki-artifact-host', WikiArtifactHost);
