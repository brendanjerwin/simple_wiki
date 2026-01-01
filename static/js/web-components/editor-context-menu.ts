import { html, css, LitElement } from 'lit';
import { buttonCSS, foundationCSS, menuCSS } from './shared-styles.js';

export interface ContextMenuPosition {
  x: number;
  y: number;
}

export class EditorContextMenu extends LitElement {
  static override styles = [
    foundationCSS,
    buttonCSS,
    menuCSS,
    css`
      :host {
        --menu-x: 0;
        --menu-y: 0;
        position: fixed;
        z-index: 10000;
        display: none;
        left: 0;
        top: 0;
        pointer-events: none;
      }

      :host([open]) {
        display: block;
      }

      .context-menu {
        min-width: 160px;
        background: var(--color-background-primary, #2d2d2d);
        border: 1px solid var(--color-border, #444);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
        pointer-events: auto;
        /* Use transform with min() to constrain within viewport bounds */
        transform: translateX(min(var(--menu-x), calc(100vw - 100% - 8px)))
                   translateY(min(var(--menu-y), calc(100vh - 100% - 8px)));
      }

      .menu-section {
        border-bottom: 1px solid var(--color-border, #444);
        padding: 4px 0;
      }

      .menu-section:last-child {
        border-bottom: none;
      }

      .menu-item {
        display: flex;
        align-items: center;
        gap: 8px;
        width: 100%;
        padding: 8px 12px;
        border: none;
        background: transparent;
        color: var(--color-text-primary, #e9ecef);
        font-size: 14px;
        text-align: left;
        cursor: pointer;
      }

      .menu-item:hover {
        background: var(--color-background-hover, #3d3d3d);
      }

      .menu-item-icon {
        width: 16px;
        text-align: center;
        opacity: 0.8;
      }
    `
  ];

  static override properties = {
    open: { type: Boolean, reflect: true },
    isMobile: { type: Boolean },
    hasSelection: { type: Boolean },
  };

  declare open: boolean;
  declare isMobile: boolean;
  declare hasSelection: boolean;

  constructor() {
    super();
    this.open = false;
    this.isMobile = false;
    this.hasSelection = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('click', this._handleClickOutside);
    document.addEventListener('keydown', this._handleKeyDown);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('click', this._handleClickOutside);
    document.removeEventListener('keydown', this._handleKeyDown);
  }

  private _handleClickOutside = (event: Event): void => {
    if (!this.open) return;

    if (event.target instanceof Node && !this.contains(event.target)) {
      this.close();
    }
  };

  private _handleKeyDown = (event: KeyboardEvent): void => {
    if (!this.open) return;

    if (event.key === 'Escape') {
      this.close();
    }
  };

  openAt(position: ContextMenuPosition): void {
    // Set CSS custom properties for positioning
    // The CSS min() function handles viewport bounds automatically
    this.style.setProperty('--menu-x', `${position.x}px`);
    this.style.setProperty('--menu-y', `${position.y}px`);
    this.open = true;
  }

  close(): void {
    this.open = false;
  }

  private _dispatchMenuEvent(eventName: string): void {
    this.dispatchEvent(new CustomEvent(eventName, {
      bubbles: true,
      composed: true,
    }));
    this.close();
  }

  private _handleUploadImage = (): void => {
    this._dispatchMenuEvent('upload-image-requested');
  };

  private _handleUploadFile = (): void => {
    this._dispatchMenuEvent('upload-file-requested');
  };

  private _handleTakePhoto = (): void => {
    this._dispatchMenuEvent('take-photo-requested');
  };

  private _handleBold = (): void => {
    this._dispatchMenuEvent('format-bold-requested');
  };

  private _handleItalic = (): void => {
    this._dispatchMenuEvent('format-italic-requested');
  };

  private _handleInsertLink = (): void => {
    this._dispatchMenuEvent('insert-link-requested');
  };

  private _handleInsertNewPage = (): void => {
    this._dispatchMenuEvent('insert-new-page-requested');
  };

  override render() {
    if (!this.open) {
      return html``;
    }

    return html`
      <div class="context-menu dropdown-menu border-radius">
        <div class="menu-section">
          <button class="menu-item" @click="${this._handleUploadImage}">
            <span class="menu-item-icon">&#128247;</span>
            Upload Image
          </button>
          <button class="menu-item" @click="${this._handleUploadFile}">
            <span class="menu-item-icon">&#128196;</span>
            Upload File
          </button>
          ${this.isMobile ? html`
            <button class="menu-item" @click="${this._handleTakePhoto}">
              <span class="menu-item-icon">&#128248;</span>
              Take Photo
            </button>
          ` : ''}
        </div>
        <div class="menu-section">
          <button class="menu-item" @click="${this._handleBold}">
            <span class="menu-item-icon"><strong>B</strong></span>
            Bold
          </button>
          <button class="menu-item" @click="${this._handleItalic}">
            <span class="menu-item-icon"><em>I</em></span>
            Italic
          </button>
          <button class="menu-item" @click="${this._handleInsertLink}">
            <span class="menu-item-icon">&#128279;</span>
            Insert Link
          </button>
        </div>
        <div class="menu-section">
          <button class="menu-item" @click="${this._handleInsertNewPage}">
            <span class="menu-item-icon">&#10010;</span>
            Create & Link New Page
          </button>
        </div>
      </div>
    `;
  }
}

customElements.define('editor-context-menu', EditorContextMenu);

declare global {
  interface HTMLElementTagNameMap {
    'editor-context-menu': EditorContextMenu;
  }
}
