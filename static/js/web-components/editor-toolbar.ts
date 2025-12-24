import { html, css, LitElement } from 'lit';
import { buttonCSS, foundationCSS } from './shared-styles.js';

/**
 * EditorToolbar provides a horizontal toolbar for mobile editing.
 * It displays formatting and upload buttons that are always visible.
 */
export class EditorToolbar extends LitElement {

  static override styles = [
    foundationCSS,
    buttonCSS,
    css`
      :host {
        display: none; /* Hidden by default (desktop) */
      }

      /* Show toolbar on mobile/tablet - fixed at top */
      @media (max-width: 1024px) {
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
        flex-wrap: wrap;
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
    `
  ];

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
    this._dispatchEvent('upload-image-requested');
  };

  private _handleUploadFile = (): void => {
    this._dispatchEvent('upload-file-requested');
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

        <button class="toolbar-btn" data-action="upload-image" @click="${this._handleUploadImage}" title="Upload Image">
          <span class="btn-icon">&#128247;</span>
        </button>
        <button class="toolbar-btn" data-action="upload-file" @click="${this._handleUploadFile}" title="Upload File">
          <span class="btn-icon">&#128196;</span>
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
