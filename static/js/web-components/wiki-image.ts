import { html, css, LitElement, TemplateResult } from 'lit';
import { property } from 'lit/decorators.js';
import { ifDefined } from 'lit/directives/if-defined.js';
import { showToast, ToastMessage } from './toast-message.js';
import { AugmentErrorService } from './augment-error-service.js';

/**
 * WikiImage - Displays images from wiki content with tools overlay.
 *
 * @element wiki-image
 *
 * @property {string} src - The image source URL
 * @property {string} alt - Alternative text for the image
 * @property {string} title - Optional title/tooltip for the image
 * @property {boolean} toolsOpen - Whether the tools panel is open (for mobile tap)
 */
export class WikiImage extends LitElement {
  static override styles = css`
    :host {
      display: block;
      text-align: center;
      margin: 1.5em 0;
      position: relative;
    }

    .image-container {
      display: inline-block;
      position: relative;
      max-width: 80%;
    }

    img {
      max-width: 100%;
      max-height: 70vh;
      object-fit: contain;
      border-radius: 8px;
      box-shadow: 2px 2px 5px rgba(0, 0, 0, 0.15);
      transition: box-shadow 0.2s ease;
      display: block;
    }

    img:hover {
      box-shadow: 2px 2px 8px rgba(0, 0, 0, 0.25);
    }

    .tools-panel {
      position: absolute;
      bottom: 0;
      left: 0;
      right: 0;
      opacity: 0;
      display: flex;
      justify-content: center;
      gap: 16px;
      padding: 12px 16px;
      background: linear-gradient(to top, rgba(0, 0, 0, 0.5) 0%, rgba(0, 0, 0, 0.3) 70%, transparent 100%);
      border-radius: 0 0 8px 8px;
      transition: opacity 0.35s ease-in-out;
      pointer-events: none;
    }

    /* Desktop: show on hover (only for devices with true hover capability) */
    @media (hover: hover) {
      .image-container:hover .tools-panel {
        opacity: 1;
      }

      .image-container:hover .tool-btn {
        pointer-events: auto;
      }
    }

    /* Mobile & desktop: show when tools-open attribute is set */
    :host([tools-open]) .tools-panel {
      opacity: 1;
    }

    :host([tools-open]) .tool-btn {
      pointer-events: auto;
    }

    .tool-btn {
      background: transparent;
      border: 1px solid rgba(255, 255, 255, 0.4);
      color: rgba(255, 255, 255, 0.9);
      font-size: 0.85rem;
      padding: 6px 12px;
      min-width: 36px;
      min-height: 36px;
      cursor: pointer;
      border-radius: 6px;
      transition: all 0.15s ease;
      display: flex;
      align-items: center;
      justify-content: center;
      backdrop-filter: blur(4px);
      -webkit-backdrop-filter: blur(4px);
    }

    .tool-btn:hover {
      background: rgba(255, 255, 255, 0.15);
      border-color: rgba(255, 255, 255, 0.6);
      color: white;
    }

    .tool-btn:active {
      transform: scale(0.95);
    }

    .tool-btn:focus {
      outline: none;
      border-color: rgba(255, 255, 255, 0.8);
      box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.2);
    }

    .tool-btn svg {
      width: 18px;
      height: 18px;
      stroke: currentColor;
      stroke-width: 1.5;
      fill: none;
    }

    /* Mobile close bar - only visible on touch devices */
    .close-bar {
      display: none;
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      padding: 8px;
      background: linear-gradient(to bottom, rgba(0, 0, 0, 0.5) 0%, rgba(0, 0, 0, 0.3) 70%, transparent 100%);
      border-radius: 8px 8px 0 0;
      justify-content: flex-end;
      opacity: 0;
      transition: opacity 0.35s ease-in-out;
      pointer-events: none;
    }

    @media (pointer: coarse) {
      .close-bar {
        display: flex;
      }
    }

    :host([tools-open]) .close-bar {
      opacity: 1;
      pointer-events: auto;
    }

    .close-btn {
      background: transparent;
      border: 1px solid rgba(255, 255, 255, 0.4);
      color: rgba(255, 255, 255, 0.9);
      width: 32px;
      height: 32px;
      cursor: pointer;
      border-radius: 6px;
      transition: all 0.15s ease;
      display: flex;
      align-items: center;
      justify-content: center;
      backdrop-filter: blur(4px);
      -webkit-backdrop-filter: blur(4px);
    }

    .close-btn:hover {
      background: rgba(255, 255, 255, 0.15);
      border-color: rgba(255, 255, 255, 0.6);
      color: white;
    }

    .close-btn:active {
      transform: scale(0.95);
    }

    .close-btn svg {
      width: 16px;
      height: 16px;
      stroke: currentColor;
      stroke-width: 2;
      fill: none;
    }
  `;

  @property({ type: String })
  src = '';

  @property({ type: String })
  alt = '';

  @property({ type: String })
  title?: string;

  @property({ type: Boolean, reflect: true, attribute: 'tools-open' })
  toolsOpen = false;

  override connectedCallback(): void {
    super.connectedCallback();
    // Single document-level handler for all click logic
    // This is more reliable than coordinating component + document handlers
    document.addEventListener('click', this._handleDocumentClick);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('click', this._handleDocumentClick);
  }

  private _handleDocumentClick = (e: Event): void => {
    const path = e.composedPath();

    // Check if click was on a tool button or close button - let them handle themselves
    const isOnButton = path.some(
      el => el instanceof HTMLButtonElement
    );
    if (isOnButton) {
      return;
    }

    // Check if click was inside this component
    const clickedInside = path.includes(this);

    if (clickedInside) {
      // Open toolbar when clicking on image (don't toggle - use X to close on mobile)
      this.toolsOpen = true;
    } else if (this.toolsOpen) {
      // Close toolbar when clicking outside
      this.toolsOpen = false;
    }
  };

  private _handleClose(e: Event): void {
    e.stopPropagation();
    this.toolsOpen = false;
  }

  private _handleKeydown(e: KeyboardEvent): void {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      this.toolsOpen = true;
    }
  }

  private _handleOpenNewTab(e: Event): void {
    e.stopPropagation();
    window.open(this.src, '_blank', 'noopener,noreferrer');
  }

  private _handleDownload(e: Event): void {
    e.stopPropagation();
    const link = document.createElement('a');
    link.href = this.src;
    link.download = this._getFilename();
    link.click();
  }

  private async _handleCopyImage(e: Event): Promise<void> {
    e.stopPropagation();
    try {
      // Check for secure context (HTTPS required for Clipboard API)
      if (!window.isSecureContext) {
        throw new Error('Clipboard requires HTTPS. Copy is not available on insecure connections.');
      }

      // Check for Clipboard API support
      if (!navigator.clipboard) {
        throw new Error('Clipboard API not available in this browser.');
      }

      // Check for write() method support (needed for images)
      if (!('write' in navigator.clipboard)) {
        throw new Error('Image copying not supported in this browser. Try a different browser.');
      }

      const response = await fetch(this.src);
      if (!response.ok) {
        throw new Error(`Failed to fetch image: ${response.status} ${response.statusText}`);
      }

      const blob = await response.blob();

      // ClipboardItem requires specific MIME types - convert if needed
      // Most browsers support image/png, some support image/jpeg
      const supportedType = blob.type.startsWith('image/') ? blob.type : 'image/png';

      await navigator.clipboard.write([
        new ClipboardItem({ [supportedType]: blob })
      ]);
      showToast('Image copied!', 'success', 3);
    } catch (err: unknown) {
      const augmentedError = AugmentErrorService.augmentError(err, 'copy image to clipboard');
      const toast = document.createElement('toast-message') as ToastMessage;
      toast.type = 'error';
      toast.augmentedError = augmentedError;
      toast.visible = false;
      document.body.appendChild(toast);
      requestAnimationFrame(() => {
        toast.show();
      });
    }
  }

  private _getFilename(): string {
    return this.src.split('/').pop() || 'image';
  }

  private _canCopyToClipboard(): boolean {
    return window.isSecureContext &&
           typeof navigator.clipboard !== 'undefined' &&
           'write' in navigator.clipboard;
  }

  override render(): TemplateResult {
    return html`
      <div class="image-container">
        <div class="close-bar">
          <button
            class="close-btn"
            @click="${this._handleClose}"
            title="Close"
            aria-label="Close tools">
            <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>
        <img
          src="${this.src}"
          alt="${this.alt}"
          title=${ifDefined(this.title)}
          tabindex="0"
          role="button"
          aria-label="${this.alt || 'Image'} - Press Enter to open tools"
          @keydown="${this._handleKeydown}"
        />
        <div class="tools-panel" role="toolbar" aria-label="Image tools">
          <button
            class="tool-btn"
            @click="${this._handleOpenNewTab}"
            title="Open in new tab"
            aria-label="Open in new tab">
            <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
              <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>
              <polyline points="15 3 21 3 21 9"/>
              <line x1="10" y1="14" x2="21" y2="3"/>
            </svg>
          </button>
          <button
            class="tool-btn"
            @click="${this._handleDownload}"
            title="Download"
            aria-label="Download">
            <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
              <polyline points="7 10 12 15 17 10"/>
              <line x1="12" y1="15" x2="12" y2="3"/>
            </svg>
          </button>
          ${this._canCopyToClipboard() ? html`
            <button
              class="tool-btn"
              @click="${this._handleCopyImage}"
              title="Copy image"
              aria-label="Copy image">
              <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
              </svg>
            </button>
          ` : ''}
        </div>
      </div>
    `;
  }
}

customElements.define('wiki-image', WikiImage);

declare global {
  interface HTMLElementTagNameMap {
    'wiki-image': WikiImage;
  }
}
