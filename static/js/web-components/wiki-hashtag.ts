import { html, css, LitElement } from 'lit';
import { property } from 'lit/decorators.js';
import type { WikiSearchOpenEventDetail } from './event-types.js';

/**
 * `<wiki-hashtag>` wraps a rendered `#tag` pill in page bodies, search results,
 * and checklist items. Plain clicks open the global wiki search popup with the
 * tag pre-filled (same UX as typing into the menu search bar). Modifier-key
 * clicks (ctrl/meta/shift) and right-clicks fall back to native anchor
 * behavior so users can still "open in new tab" via the fallback href.
 *
 * The fallback href points to `/?q=#TAG` — the wiki home with a query
 * parameter the search bar can pick up on load. If JS is disabled, the link
 * still navigates somewhere sensible.
 *
 * Integration with `<wiki-search>`: this element dispatches a
 * `wiki-search-open` CustomEvent that bubbles and is composed, carrying
 * `{ query: '#tag' }`. The mounted `<wiki-search>` listens at window level
 * and runs the query immediately.
 */
export class WikiHashtag extends LitElement {
  static override readonly styles = css`
    :host {
      display: inline;
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
  `;

  /**
   * The normalized tag (no leading `#`) — e.g. `groceries`. Used to build the
   * search query and the fallback href.
   */
  @property({ type: String })
  declare tag: string;

  constructor() {
    super();
    this.tag = '';
  }

  private get fallbackHref(): string {
    // `/?q=#TAG` lets a future bootstrap step pre-fill the search bar from
    // the URL. Today it just navigates to the wiki home, which is acceptable
    // progressive-enhancement behavior when JS is disabled.
    return `/?q=${encodeURIComponent('#' + this.tag)}`;
  }

  private handleClick(e: MouseEvent): void {
    // Modifier keys signal the user wants browser-native behavior
    // (open in new tab/window). Don't intercept — let the anchor follow
    // its href.
    if (e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) {
      return;
    }

    e.preventDefault();
    const detail: WikiSearchOpenEventDetail = { query: '#' + this.tag };
    this.dispatchEvent(
      new CustomEvent<WikiSearchOpenEventDetail>('wiki-search-open', {
        detail,
        bubbles: true,
        composed: true,
      }),
    );
  }

  override render() {
    return html`<a
      class="hashtag-pill"
      href="${this.fallbackHref}"
      @click="${this.handleClick}"
    ><slot></slot></a>`;
  }
}

customElements.define('wiki-hashtag', WikiHashtag);

declare global {
  interface HTMLElementTagNameMap {
    'wiki-hashtag': WikiHashtag;
  }
}
