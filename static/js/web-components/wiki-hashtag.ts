import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient, type Client } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { SearchService, SearchContentRequestSchema } from '../gen/api/v1/search_pb.js';
import type { SearchResult } from '../gen/api/v1/search_pb.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import { getGrpcWebTransport } from './grpc-transport.js';
import './error-display.js';

/**
 * `<wiki-hashtag>` wraps a rendered `#tag` pill in page bodies, search results,
 * and checklist items. Plain clicks open a small "tag bubble" popover anchored
 * to the pill that lists pages tagged with the same value. Modifier-key clicks
 * (ctrl/meta/shift) and right-clicks fall back to native anchor behavior so
 * users can still "open in new tab" via the fallback href.
 *
 * The fallback href points to `/?q=#TAG` — the wiki home with a query
 * parameter the search bar can pick up on load. If JS is disabled, the link
 * still navigates somewhere sensible.
 *
 * The popover queries `SearchService.SearchContent` with `#tag` and renders
 * the resulting pages as a small floating list. It closes on outside click,
 * Escape, or clicking the same pill again.
 */
export class WikiHashtag extends LitElement {
  static override readonly styles = css`
    :host {
      display: inline;
      position: relative;
    }

    /*
     * The pill styling needs to live inside the shadow root because the
     * anchor we render here is in shadow DOM and global CSS in default.css
     * cannot reach it. We mirror the visual rules from default.css's
     * legacy a.hashtag-pill so the upgraded component matches the
     * fallback rendering applied to bare <wiki-hashtag> when JS hasn't
     * loaded yet (see default.css).
     */
    a.hashtag-pill {
      display: inline-block;
      padding: 1px 8px;
      margin: 0 1px;
      font-size: 0.85em;
      background: var(--color-surface-sunken, #f0f0f0);
      border: 1px solid var(--color-border-subtle, #e0e0e0);
      border-radius: 12px;
      color: var(--color-text-link, #4078c0);
      text-decoration: none;
      white-space: nowrap;
    }

    a.hashtag-pill:hover,
    a.hashtag-pill:focus {
      background: var(--color-text-link, #4078c0);
      color: var(--color-text-inverse, #fff);
      border-color: var(--color-text-link, #4078c0);
    }

    .bubble {
      position: absolute;
      top: 100%;
      left: 0;
      margin-top: 4px;
      min-width: 220px;
      max-width: 320px;
      max-height: 320px;
      overflow-y: auto;
      background: var(--color-surface-primary, #fff);
      border: 1px solid var(--color-border-default, #d0d0d0);
      border-radius: 6px;
      box-shadow: var(--shadow-medium, 0 4px 12px rgba(0, 0, 0, 0.15));
      z-index: 300;
      padding: 6px 0;
      font-size: 0.9em;
    }

    /*
     * If the bubble would clip the right edge of the viewport, set
     * data-align="right" on .bubble to anchor it to the right side of
     * the pill instead.
     */
    .bubble[data-align="right"] {
      left: auto;
      right: 0;
    }

    .bubble-header {
      padding: 4px 12px;
      font-size: 0.85em;
      color: var(--color-text-secondary, #666);
      border-bottom: 1px solid var(--color-border-subtle, #eee);
      margin-bottom: 4px;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .bubble-list {
      list-style: none;
      margin: 0;
      padding: 0;
    }

    .bubble-list li {
      margin: 0;
      padding: 0;
    }

    .bubble-list a {
      display: block;
      padding: 6px 12px;
      color: var(--color-text-primary, #222);
      text-decoration: none;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .bubble-list a:hover,
    .bubble-list a:focus {
      background: var(--color-surface-sunken, #f0f0f0);
      outline: none;
    }

    .bubble-empty,
    .bubble-loading {
      padding: 8px 12px;
      color: var(--color-text-secondary, #666);
      font-style: italic;
    }

    .spinner {
      display: inline-block;
      width: 12px;
      height: 12px;
      border: 2px solid var(--color-border-subtle, #ddd);
      border-top-color: var(--color-text-link, #4078c0);
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
      vertical-align: middle;
      margin-right: 6px;
    }

    @keyframes spin {
      to {
        transform: rotate(360deg);
      }
    }
  `;

  /**
   * The normalized tag (no leading `#`) — e.g. `groceries`. Used to build the
   * search query and the fallback href.
   */
  @property({ type: String })
  declare tag: string;

  @state()
  declare private _open: boolean;

  @state()
  declare private _loading: boolean;

  @state()
  declare private _results: SearchResult[];

  @state()
  declare private _error: AugmentedError | null;

  @state()
  declare private _hasSearched: boolean;

  private client: Client<typeof SearchService> | null = null;

  // Bound handlers so add/remove pairs reference the same function.
  private readonly _onDocumentClick = (event: Event): void => {
    const path = event.composedPath();
    if (!path.includes(this)) {
      this._close();
    }
  };

  private readonly _onDocumentKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape' && this._open) {
      event.preventDefault();
      this._close();
    }
  };

  constructor() {
    super();
    this.tag = '';
    this._open = false;
    this._loading = false;
    this._results = [];
    this._error = null;
    this._hasSearched = false;
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    // Defensive: tear down any stray document listeners if we were closed
    // while still open.
    document.removeEventListener('click', this._onDocumentClick);
    document.removeEventListener('keydown', this._onDocumentKeydown);
  }

  private getClient(): Client<typeof SearchService> {
    this.client ??= createClient(SearchService, getGrpcWebTransport());
    return this.client;
  }

  // Exposed for tests so they can stub it without making a real network call.
  async performSearch(query: string): Promise<SearchResult[]> {
    const request = create(SearchContentRequestSchema, { query });
    const response = await this.getClient().searchContent(request);
    return response.results;
  }

  private get fallbackHref(): string {
    // `/?q=#TAG` lets a future bootstrap step pre-fill the search bar from
    // the URL. Today it just navigates to the wiki home, which is acceptable
    // progressive-enhancement behavior when JS is disabled.
    return `/?q=${encodeURIComponent('#' + this.tag)}`;
  }

  private async _open_(): Promise<void> {
    this._open = true;
    document.addEventListener('click', this._onDocumentClick);
    document.addEventListener('keydown', this._onDocumentKeydown);

    // Lazily fetch on first open. Cached results stay until the component
    // unmounts; this is fine for a transient pill popover.
    if (this._hasSearched) {
      return;
    }
    this._hasSearched = true;
    this._loading = true;
    this._error = null;

    try {
      const results = await this.performSearch('#' + this.tag);
      this._results = [...results];
    } catch (err) {
      this._results = [];
      this._error = AugmentErrorService.augmentError(err, 'looking up #' + this.tag);
    } finally {
      this._loading = false;
    }
  }

  private _close(): void {
    if (!this._open) return;
    this._open = false;
    document.removeEventListener('click', this._onDocumentClick);
    document.removeEventListener('keydown', this._onDocumentKeydown);
  }

  private async _toggle(): Promise<void> {
    if (this._open) {
      this._close();
    } else {
      await this._open_();
    }
  }

  private async handleClick(e: MouseEvent): Promise<void> {
    // Modifier keys signal the user wants browser-native behavior
    // (open in new tab/window). Don't intercept — let the anchor follow
    // its href.
    if (e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) {
      return;
    }

    e.preventDefault();
    e.stopPropagation();
    await this._toggle();
  }

  private renderBubbleContent() {
    if (this._loading) {
      return html`<div class="bubble-loading"><span class="spinner" aria-hidden="true"></span>Loading…</div>`;
    }

    if (this._error) {
      return html`<error-display .augmentedError=${this._error}></error-display>`;
    }

    if (this._results.length === 0) {
      return html`<div class="bubble-empty">No pages tagged #${this.tag}.</div>`;
    }

    return html`
      <ul class="bubble-list">
        ${this._results.map(
          (result) => html`
            <li>
              <a href="/${result.identifier}">${result.title || result.identifier}</a>
            </li>
          `,
        )}
      </ul>
    `;
  }

  private renderBubble() {
    if (!this._open) return nothing;

    return html`
      <div
        class="bubble"
        role="dialog"
        aria-label="Pages tagged #${this.tag}"
      >
        <div class="bubble-header">Pages tagged #${this.tag}</div>
        ${this.renderBubbleContent()}
      </div>
    `;
  }

  override render() {
    return html`<a
        class="hashtag-pill"
        href="${this.fallbackHref}"
        aria-haspopup="dialog"
        aria-expanded="${this._open}"
        @click="${this.handleClick}"
      ><slot></slot></a>${this.renderBubble()}`;
  }
}

customElements.define('wiki-hashtag', WikiHashtag);

declare global {
  interface HTMLElementTagNameMap {
    'wiki-hashtag': WikiHashtag;
  }
}
