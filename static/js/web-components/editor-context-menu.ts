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
        position: fixed;
        z-index: 10000;
        display: none;
      }

      :host([open]) {
        display: block;
      }

      .context-menu {
        min-width: 160px;
        background: var(--color-background-primary, #2d2d2d);
        border: 1px solid var(--color-border, #444);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
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
    x: { type: Number },
    y: { type: Number },
    isMobile: { type: Boolean },
    hasSelection: { type: Boolean },
  };

  declare open: boolean;
  declare x: number;
  declare y: number;
  declare isMobile: boolean;
  declare hasSelection: boolean;

  constructor() {
    super();
    this.open = false;
    this.x = 0;
    this.y = 0;
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

    if (!this.contains(event.target as Node)) {
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
    this.x = position.x;
    this.y = position.y;
    this.open = true;
    this._adjustPositionIfNeeded();
  }

  close(): void {
    this.open = false;
  }

  private _adjustPositionIfNeeded(): void {
    requestAnimationFrame(() => {
      const menu = this.shadowRoot?.querySelector('.context-menu') as HTMLElement;
      if (!menu) return;

      const rect = menu.getBoundingClientRect();
      const viewportWidth = window.innerWidth;
      const viewportHeight = window.innerHeight;

      let newX = this.x;
      let newY = this.y;

      if (this.x + rect.width > viewportWidth) {
        newX = viewportWidth - rect.width - 8;
      }

      if (this.y + rect.height > viewportHeight) {
        newY = viewportHeight - rect.height - 8;
      }

      if (newX !== this.x || newY !== this.y) {
        this.x = Math.max(8, newX);
        this.y = Math.max(8, newY);
      }
    });
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

  override render() {
    if (!this.open) {
      return html``;
    }

    return html`
      <div
        class="context-menu dropdown-menu border-radius"
        style="left: ${this.x}px; top: ${this.y}px;"
      >
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
