import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { buttonCSS, colorCSS, foundationCSS } from './shared-styles.js';

/**
 * EditorToolbar provides a horizontal toolbar for mobile editing.
 * It displays formatting and upload buttons that are always visible.
 *
 * Keyboard navigation:
 * - ArrowLeft / ArrowRight moves focus between toolbar buttons.
 * - Escape closes the upload dropdown menu and returns focus to the toggle.
 */
export class EditorToolbar extends LitElement {
  @property({ type: Boolean, attribute: 'has-selection' })
  declare hasSelection: boolean;

  @property({ type: Boolean, attribute: 'hide-exit' })
  declare hideExit: boolean;

  @state()
  declare _uploadMenuOpen: boolean;

  constructor() {
    super();
    this.hasSelection = false;
    this.hideExit = false;
    this._uploadMenuOpen = false;
  }

  static override readonly styles = [
    foundationCSS,
    colorCSS,
    buttonCSS,
    css`
      :host {
        display: block;
        background: var(--color-editor-surface);
        border-bottom: 1px solid var(--color-editor-border);
        padding: 6px 8px;
        box-sizing: border-box;
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
        border: 1px solid var(--color-editor-border);
        border-radius: 4px;
        background: var(--color-editor-surface-elevated);
        color: var(--color-editor-text);
        font-size: 14px;
        cursor: pointer;
        transition: background-color 0.15s ease;
      }

      .toolbar-btn:hover,
      .toolbar-btn:active {
        background: var(--color-editor-surface-hover);
      }

      .toolbar-btn:active {
        transform: scale(0.98);
      }

      .toolbar-btn:disabled {
        opacity: 0.4;
        cursor: default;
      }

      .toolbar-btn:disabled:hover,
      .toolbar-btn:disabled:active {
        background: var(--color-editor-surface-elevated);
        transform: none;
      }

      .toolbar-btn.exit-btn {
        background: var(--color-editor-surface-deep);
      }

      .separator {
        width: 1px;
        height: 24px;
        background: var(--color-editor-border);
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
        border: 1px solid var(--color-editor-border);
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
        background: var(--color-editor-surface-elevated);
        color: var(--color-editor-text);
        font-size: 14px;
        cursor: pointer;
        transition: background-color 0.15s ease;
      }

      .upload-btn-main:hover,
      .upload-btn-main:active {
        background: var(--color-editor-surface-hover);
      }

      .upload-btn-toggle {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 24px;
        height: 36px;
        padding: 0;
        border: none;
        border-left: 1px solid var(--color-editor-border);
        background: var(--color-editor-surface-elevated);
        color: var(--color-editor-text);
        font-size: 10px;
        cursor: pointer;
        transition: background-color 0.15s ease;
      }

      .upload-btn-toggle:hover,
      .upload-btn-toggle:active {
        background: var(--color-editor-surface-hover);
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
        background: var(--color-editor-surface-elevated);
        border: 1px solid var(--color-editor-border);
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
        color: var(--color-editor-text);
        transition: background-color 0.2s ease;
      }

      .upload-dropdown-item:hover {
        background: var(--color-editor-surface-hover);
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

  private readonly _handleDocumentClick = (event: Event): void => {
    // Close dropdown if clicking outside (composedPath handles shadow DOM)
    if (this._uploadMenuOpen && !event.composedPath().includes(this)) {
      this._uploadMenuOpen = false;
    }
  };

  private _handleArrowNavigation(event: KeyboardEvent): void {
    event.preventDefault();
    const buttons = Array.from(
      this.shadowRoot?.querySelectorAll<HTMLButtonElement>('button:not([disabled])') ?? []
    );
    if (buttons.length === 0) return;

    const activeEl = this.shadowRoot?.activeElement;
    const currentIndex =
      activeEl instanceof HTMLButtonElement ? buttons.indexOf(activeEl) : -1;

    let nextIndex: number;
    if (event.key === 'ArrowRight') {
      nextIndex = currentIndex < buttons.length - 1 ? currentIndex + 1 : 0;
    } else {
      nextIndex = currentIndex > 0 ? currentIndex - 1 : buttons.length - 1;
    }
    buttons[nextIndex]?.focus();
  }

  readonly _handleToolbarKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape' && this._uploadMenuOpen) {
      event.preventDefault();
      this._uploadMenuOpen = false;
      this.shadowRoot?.querySelector<HTMLButtonElement>('.upload-btn-toggle')?.focus();
      return;
    }

    if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
      this._handleArrowNavigation(event);
    }
  };

  private _dispatchEvent(eventName: string): void {
    this.dispatchEvent(new CustomEvent(eventName, {
      bubbles: true,
      composed: true,
    }));
  }

  private readonly _handleBold = (): void => {
    this._dispatchEvent('format-bold-requested');
  };

  private readonly _handleItalic = (): void => {
    this._dispatchEvent('format-italic-requested');
  };

  private readonly _handleLink = (): void => {
    this._dispatchEvent('insert-link-requested');
  };

  private readonly _handleUploadImage = (): void => {
    this._uploadMenuOpen = false;
    this._dispatchEvent('upload-image-requested');
  };

  private readonly _handleUploadFile = (): void => {
    this._uploadMenuOpen = false;
    this._dispatchEvent('upload-file-requested');
  };

  private readonly _handleToggleUploadMenu = (): void => {
    this._uploadMenuOpen = !this._uploadMenuOpen;
  };

  private readonly _handleNewPage = (): void => {
    this._dispatchEvent('insert-new-page-requested');
  };

  private readonly _handleExit = (): void => {
    this._dispatchEvent('exit-requested');
  };

  override render() {
    return html`
      <div
        class="toolbar"
        role="toolbar"
        aria-label="Editor toolbar"
        @keydown="${this._handleToolbarKeydown}"
      >
        <button
          class="toolbar-btn"
          data-action="bold"
          @click="${this._handleBold}"
          ?disabled="${!this.hasSelection}"
          title="Bold"
          aria-label="Bold"
        >
          <strong class="btn-icon">B</strong>
        </button>
        <button
          class="toolbar-btn"
          data-action="italic"
          @click="${this._handleItalic}"
          ?disabled="${!this.hasSelection}"
          title="Italic"
          aria-label="Italic"
        >
          <em class="btn-icon">I</em>
        </button>
        <button
          class="toolbar-btn"
          data-action="link"
          @click="${this._handleLink}"
          ?disabled="${!this.hasSelection}"
          title="Insert Link"
          aria-label="Insert Link"
        >
          <span class="btn-icon">&#128279;</span>
        </button>

        <div class="separator" role="separator"></div>

        <!-- Upload dropdown: main button for image, dropdown for file -->
        <div class="upload-dropdown">
          <div class="upload-btn-group">
            <button
              class="upload-btn-main"
              data-action="upload-image"
              @click="${this._handleUploadImage}"
              title="Upload Image"
              aria-label="Upload Image"
            >
              <span class="btn-icon">&#128247;</span>
            </button>
            <button
              class="upload-btn-toggle"
              @click="${this._handleToggleUploadMenu}"
              title="More upload options"
              aria-label="More upload options"
              aria-expanded="${this._uploadMenuOpen ? 'true' : 'false'}"
              aria-haspopup="menu"
            >
              <span class="dropdown-arrow ${this._uploadMenuOpen ? 'open' : ''}">&#9660;</span>
            </button>
          </div>
          ${this._uploadMenuOpen ? html`
            <div class="upload-dropdown-menu" role="menu" aria-label="Upload options">
              <button
                class="upload-dropdown-item"
                data-action="upload-file"
                @click="${this._handleUploadFile}"
                role="menuitem"
              >
                <span>&#128196;</span> Upload File
              </button>
            </div>
          ` : nothing}
        </div>

        <button
          class="toolbar-btn"
          data-action="new-page"
          @click="${this._handleNewPage}"
          title="Create &amp; Link New Page"
          aria-label="Create and Link New Page"
        >
          <span class="btn-icon">&#10010;</span>
        </button>

        <div class="spacer"></div>

        ${this.hideExit ? nothing : html`
          <button
            class="toolbar-btn exit-btn"
            data-action="exit"
            @click="${this._handleExit}"
            title="Done Editing"
            aria-label="Done Editing"
          >
            <span class="btn-icon">&#10003;</span> Done
          </button>
        `}
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
