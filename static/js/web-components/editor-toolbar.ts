import { html, css, LitElement, nothing } from 'lit';
import { state } from 'lit/decorators.js';
import { buttonCSS, foundationCSS } from './shared-styles.js';

/**
 * EditorToolbar provides a horizontal toolbar for mobile editing.
 * It displays formatting and upload buttons that are always visible.
 */
export class EditorToolbar extends LitElement {
  @state()
  declare _uploadMenuOpen: boolean;

  constructor() {
    super();
    this._uploadMenuOpen = false;
  }

  static override styles = [
    foundationCSS,
    buttonCSS,
    css`
      :host {
        display: none; /* Hidden by default (desktop) */
      }

      /* Show toolbar on touch devices - fixed at top */
      @media (pointer: coarse) {
        :host {
          display: block;
          background: var(--color-background-primary, #2d2d2d);
          border-bottom: 1px solid var(--color-border, #444);
          padding: 6px 8px;
          box-sizing: border-box;
          position: fixed;
          top: 0;
          left: 0;
          right: 0;
          z-index: 100;
        }
      }

      .toolbar {
        display: flex;
        flex-wrap: nowrap;
        gap: 4px;
        justify-content: center;
        align-items: center;
      }

      .toolbar-btn {
        display: flex;
        align-items: center;
        justify-content: center;
        min-width: 40px;
        height: 36px;
        padding: 0 12px;
        border: 1px solid var(--color-border, #555);
        border-radius: 4px;
        background: var(--color-background-secondary, #3d3d3d);
        color: var(--color-text-primary, #e9ecef);
        font-size: 14px;
        cursor: pointer;
        transition: background-color 0.15s ease;
      }

      .toolbar-btn:hover,
      .toolbar-btn:active {
        background: var(--color-background-hover, #4d4d4d);
      }

      .toolbar-btn:active {
        transform: scale(0.98);
      }

      .toolbar-btn.exit-btn {
        background: var(--color-background-tertiary, #4a4a4a);
      }

      .separator {
        width: 1px;
        height: 24px;
        background: var(--color-border, #555);
        margin: 0 4px;
      }

      .spacer {
        flex: 1;
      }

      .btn-icon {
        font-size: 16px;
      }

      /* Upload dropdown button */
      .upload-dropdown {
        position: relative;
      }

      .upload-btn-group {
        display: flex;
        align-items: stretch;
        border: 1px solid var(--color-border, #555);
        border-radius: 4px;
        overflow: hidden;
      }

      .upload-btn-main {
        display: flex;
        align-items: center;
        justify-content: center;
        min-width: 40px;
        height: 36px;
        padding: 0 10px;
        border: none;
        background: var(--color-background-secondary, #3d3d3d);
        color: var(--color-text-primary, #e9ecef);
        font-size: 14px;
        cursor: pointer;
        transition: background-color 0.15s ease;
      }

      .upload-btn-main:hover,
      .upload-btn-main:active {
        background: var(--color-background-hover, #4d4d4d);
      }

      .upload-btn-toggle {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 24px;
        height: 36px;
        padding: 0;
        border: none;
        border-left: 1px solid var(--color-border, #555);
        background: var(--color-background-secondary, #3d3d3d);
        color: var(--color-text-primary, #e9ecef);
        font-size: 10px;
        cursor: pointer;
        transition: background-color 0.15s ease;
      }

      .upload-btn-toggle:hover,
      .upload-btn-toggle:active {
        background: var(--color-background-hover, #4d4d4d);
      }

      .dropdown-arrow {
        transition: transform 0.2s ease;
      }

      .dropdown-arrow.open {
        transform: rotate(180deg);
      }

      .upload-dropdown-menu {
        position: absolute;
        top: 100%;
        left: 0;
        margin-top: 4px;
        background: var(--color-background-secondary, #3d3d3d);
        border: 1px solid var(--color-border, #555);
        border-radius: 4px;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
        z-index: 1000;
        min-width: 140px;
      }

      .upload-dropdown-item {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 10px 14px;
        cursor: pointer;
        border: none;
        background: none;
        width: 100%;
        text-align: left;
        font-size: 14px;
        color: var(--color-text-primary, #e9ecef);
        transition: background-color 0.2s ease;
      }

      .upload-dropdown-item:hover {
        background: var(--color-background-hover, #4d4d4d);
      }

      .upload-dropdown-item:first-child {
        border-radius: 4px 4px 0 0;
      }

      .upload-dropdown-item:last-child {
        border-radius: 0 0 4px 4px;
      }
    `
  ];

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('click', this._handleDocumentClick);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('click', this._handleDocumentClick);
  }

  private _handleDocumentClick = (event: Event): void => {
    // Close dropdown if clicking outside
    if (this._uploadMenuOpen && event.target instanceof Node && !this.contains(event.target)) {
      this._uploadMenuOpen = false;
    }
  };

  private _dispatchEvent(eventName: string): void {
    this.dispatchEvent(new CustomEvent(eventName, {
      bubbles: true,
      composed: true,
    }));
  }

  private _handleBold = (): void => {
    this._dispatchEvent('format-bold-requested');
  };

  private _handleItalic = (): void => {
    this._dispatchEvent('format-italic-requested');
  };

  private _handleLink = (): void => {
    this._dispatchEvent('insert-link-requested');
  };

  private _handleUploadImage = (): void => {
    this._uploadMenuOpen = false;
    this._dispatchEvent('upload-image-requested');
  };

  private _handleUploadFile = (): void => {
    this._uploadMenuOpen = false;
    this._dispatchEvent('upload-file-requested');
  };

  private _handleToggleUploadMenu = (): void => {
    this._uploadMenuOpen = !this._uploadMenuOpen;
  };

  private _handleNewPage = (): void => {
    this._dispatchEvent('insert-new-page-requested');
  };

  private _handleExit = (): void => {
    this._dispatchEvent('exit-requested');
  };

  override render() {
    return html`
      <div class="toolbar">
        <button class="toolbar-btn" data-action="bold" @click="${this._handleBold}" title="Bold">
          <strong class="btn-icon">B</strong>
        </button>
        <button class="toolbar-btn" data-action="italic" @click="${this._handleItalic}" title="Italic">
          <em class="btn-icon">I</em>
        </button>
        <button class="toolbar-btn" data-action="link" @click="${this._handleLink}" title="Insert Link">
          <span class="btn-icon">&#128279;</span>
        </button>

        <div class="separator"></div>

        <!-- Upload dropdown: main button for image, dropdown for file -->
        <div class="upload-dropdown">
          <div class="upload-btn-group">
            <button
              class="upload-btn-main"
              data-action="upload-image"
              @click="${this._handleUploadImage}"
              title="Upload Image"
            >
              <span class="btn-icon">&#128247;</span>
            </button>
            <button
              class="upload-btn-toggle"
              @click="${this._handleToggleUploadMenu}"
              title="More upload options"
            >
              <span class="dropdown-arrow ${this._uploadMenuOpen ? 'open' : ''}">&#9660;</span>
            </button>
          </div>
          ${this._uploadMenuOpen ? html`
            <div class="upload-dropdown-menu">
              <button
                class="upload-dropdown-item"
                data-action="upload-file"
                @click="${this._handleUploadFile}"
              >
                <span>&#128196;</span> Upload File
              </button>
            </div>
          ` : nothing}
        </div>

        <button class="toolbar-btn" data-action="new-page" @click="${this._handleNewPage}" title="Create &amp; Link New Page">
          <span class="btn-icon">&#10010;</span>
        </button>

        <div class="spacer"></div>

        <button class="toolbar-btn exit-btn" data-action="exit" @click="${this._handleExit}" title="Done Editing">
          <span class="btn-icon">&#10003;</span> Done
        </button>
      </div>
    `;
  }
}

customElements.define('editor-toolbar', EditorToolbar);

declare global {
  interface HTMLElementTagNameMap {
    'editor-toolbar': EditorToolbar;
  }
}
