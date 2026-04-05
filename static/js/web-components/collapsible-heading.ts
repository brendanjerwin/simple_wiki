import { html, css, LitElement } from 'lit';
import { property, state } from 'lit/decorators.js';
import { colorCSS } from './shared-styles.js';

// localStorage value constants for persisted state
const STATE_EXPANDED = 'expanded';
const STATE_COLLAPSED = 'collapsed';

/**
 * CollapsibleHeading — a web component that wraps a heading and its section content
 * in a collapsible container. Rendered by the Go markdown processor from `#^ Heading` syntax.
 *
 * The heading element (with `slot="heading"`) is always visible. The section content
 * (default slot) is shown or hidden based on the collapsed state. The state is persisted
 * to localStorage using a key derived from the page name and heading ID.
 *
 * @example
 * <collapsible-heading heading-level="2">
 *   <h2 slot="heading" id="my-section">My Section</h2>
 *   <p>Section content...</p>
 * </collapsible-heading>
 */
export class CollapsibleHeading extends LitElement {
  static override readonly styles = [
    colorCSS,
    css`
    :host {
      display: block;
    }

    .ch-header {
      display: flex;
      align-items: center;
      gap: 0.4em;
    }

    .ch-toggle {
      background: none;
      border: none;
      cursor: pointer;
      padding: 0.1em 0.3em;
      font-size: 0.75em;
      color: var(--color-text-secondary);
      line-height: 1;
      flex-shrink: 0;
      border-radius: 3px;
      transition: background-color 0.15s ease;
    }

    .ch-toggle:hover {
      background-color: var(--color-hover-overlay);
    }

    .ch-toggle:focus-visible {
      outline: 2px solid var(--color-border-focus);
      outline-offset: 2px;
    }

    .ch-header ::slotted(h1),
    .ch-header ::slotted(h2),
    .ch-header ::slotted(h3),
    .ch-header ::slotted(h4),
    .ch-header ::slotted(h5),
    .ch-header ::slotted(h6) {
      margin: 0;
      flex: 1;
    }

    .ch-content {
      display: block;
    }

    .ch-content[hidden] {
      display: none;
    }
  `];

  @property({ type: Number, attribute: 'heading-level' })
  declare headingLevel: number;

  @state()
  declare collapsed: boolean;

  constructor() {
    super();
    this.headingLevel = 1;
    this.collapsed = true;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    this._loadState();
  }

  private _storageKey(): string | null {
    // Query light DOM directly — accessible immediately at connectedCallback time
    const headingEl = this.querySelector('[slot="heading"]');
    const headingId = headingEl?.id;
    if (!headingId) {
      return null;
    }
    const pageName = globalThis.simple_wiki?.pageName ?? '';
    return `collapsible-heading-${pageName}-${headingId}`;
  }

  private _loadState(): void {
    const key = this._storageKey();
    if (key) {
      const stored = localStorage.getItem(key);
      this.collapsed = stored !== STATE_EXPANDED;
    }
  }

  private _saveState(): void {
    const key = this._storageKey();
    if (key) {
      const value = this.collapsed ? STATE_COLLAPSED : STATE_EXPANDED;
      localStorage.setItem(key, value);
    }
  }

  _handleToggle(): void {
    this.collapsed = !this.collapsed;
    this._saveState();
  }

  override render() {
    const icon = this.collapsed ? '▶' : '▼';
    return html`
      <div class="ch-header">
        <button
          type="button"
          class="ch-toggle"
          aria-expanded="${!this.collapsed}"
          @click="${this._handleToggle}"
        >${icon}</button>
        <slot name="heading"></slot>
      </div>
      <div class="ch-content" ?hidden="${this.collapsed}">
        <slot></slot>
      </div>
    `;
  }
}

customElements.define('collapsible-heading', CollapsibleHeading);

declare global {
  interface HTMLElementTagNameMap {
    'collapsible-heading': CollapsibleHeading;
  }
}
