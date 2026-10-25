import { html, css, LitElement } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient, type Client } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { SearchService, SearchContentRequestSchema } from '../gen/api/v1/search_pb.js';
import type { SearchResult } from '../gen/api/v1/search_pb.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import { getGrpcWebTransport } from './grpc-transport.js';
import './error-display.js';
import './menu-popover.js';

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

    menu-popover::part(surface) {
      min-width: 220px;
      max-width: 320px;
      background: var(--color-surface-primary, #fff);
      border: 1px solid var(--color-border-default, #d0d0d0);
      border-radius: 6px;
      box-shadow: var(--shadow-medium, 0 4px 12px rgba(0, 0, 0, 0.15));
      padding: 6px 0;
      font-size: 0.9em;
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

  /** The pill anchor element for the popover; captured after first render. */
  @state()
  declare private _pillAnchor: HTMLElement | null;

  @state()
  declare private _loading: boolean;

  @state()
  declare private _results: SearchResult[];

  @state()
  declare private _error: AugmentedError | null;

  @state()
  declare private _hasSearched: boolean;

  private client: Client<typeof SearchService> | null = null;

  // Mutual exclusion: when any wiki-hashtag opens its popover, every other
  // instance hears a `wiki-hashtag-open` event and closes itself. Without
  // this, clicking pill B with pill A's popover open keeps both open
  // because pill B's click handler stopPropagation()s the click before
  // pill A's outside-click listener runs.
  private readonly _onAnotherHashtagOpen = (event: Event): void => {
    if (event.target === this) return;
    this._close();
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

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('wiki-hashtag-open', this._onAnotherHashtagOpen);
  }

  protected override firstUpdated(): void {
    this._pillAnchor ??= this.shadowRoot?.querySelector<HTMLElement>('.hashtag-pill') ?? null;
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('wiki-hashtag-open', this._onAnotherHashtagOpen);
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
    // The pill is purely interactive — clicks always open the popover.
    // The fallback href is a hash anchor (no destination) so right-click
    // → "open in new tab" lands on the same page rather than a 404.
    // Modifier-clicks were special-cased before but the popover is the
    // primary interaction; making it consistent simplifies behavior.
    return '#';
  }

  private async _open_(): Promise<void> {
    // Tell every other wiki-hashtag on the page to close before we open
    // ours, so only one popover is visible at a time.
    this.dispatchEvent(
      new CustomEvent('wiki-hashtag-open', { bubbles: true, composed: true }),
    );

    this._open = true;

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
  }

  private async _toggle(): Promise<void> {
    if (this._open) {
      this._close();
    } else {
      await this._open_();
    }
  }

  private async handleClick(e: MouseEvent): Promise<void> {
    // Always intercept — the popover is the click destination, regardless
    // of modifier keys. Right-click still works (context menu fires before
    // click) so users can copy the link if they want.
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
              <a href="/${result.identifier}">${displayTitle(result)}</a>
            </li>
          `,
        )}
      </ul>
    `;
  }

  private renderBubble() {
    return html`
      <div class="bubble-header">Pages tagged #${this.tag}</div>
      ${this.renderBubbleContent()}
    `;
  }

  override render() {
    return html`<a
        class="hashtag-pill"
        href="${this.fallbackHref}"
        aria-haspopup="dialog"
        aria-expanded="${this._open}"
        @click="${this.handleClick}"
      ><slot></slot></a>
      <menu-popover
        .anchor=${this._pillAnchor}
        .open=${this._open}
        surfaceRole="dialog"
        surfaceAriaLabel=${`Pages tagged #${this.tag}`}
        @menu-popover-close-request=${() => this._close()}
      >
        ${this.renderBubble()}
      </menu-popover>`;
  }
}

/**
 * Pick the user-facing display string for a search result.
 *
 * The server falls back `result.title` to `result.identifier` when no
 * frontmatter title is set, so a naive `result.title || result.identifier`
 * always shows the raw identifier (e.g. `home_lab`) for pages without an
 * explicit title. Detect that case and humanize the identifier into a
 * readable form (`Home Lab`) so the popover surfaces real titles when they
 * exist and a friendly fallback when they don't.
 *
 * Mirrors the title-or-humanized-identifier convention used by the
 * `LinkTo` template function in `templating/templating.go`.
 */
export function displayTitle(result: { title?: string; identifier: string }): string {
  const rawTitle = (result.title ?? '').trim();
  const id = result.identifier;
  if (rawTitle !== '' && rawTitle !== id) {
    return rawTitle;
  }
  return humanizeIdentifier(id);
}

/**
 * Convert an identifier like `home_lab` or `engineering-blog` into a
 * Title-Cased phrase (`Home Lab`, `Engineering Blog`).
 */
function humanizeIdentifier(id: string): string {
  if (!id) return id;
  return id
    .replace(/[-_]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .split(' ')
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ');
}

customElements.define('wiki-hashtag', WikiHashtag);

declare global {
  interface HTMLElementTagNameMap {
    'wiki-hashtag': WikiHashtag;
  }
}
