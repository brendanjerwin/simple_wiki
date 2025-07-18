import { html, css, LitElement } from 'lit';
import { createClient } from '@connectrpc/connect';
import { createGrpcWebTransport } from '@connectrpc/connect-web';
import { Frontmatter } from '../gen/api/v1/frontmatter_connect.js';
import { GetFrontmatterRequest, GetFrontmatterResponse } from '../gen/api/v1/frontmatter_pb.js';
import { sharedStyles, sharedCSS } from './shared-styles.js';

export class FrontmatterEditorDialog extends LitElement {
  static override styles = [
    sharedCSS,
    css`
      :host {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        z-index: 9999;
        display: none;
      }

      :host([open]) {
        display: flex;
        align-items: center;
        justify-content: center;
        animation: fadeIn 0.2s ease-out;
      }

      @keyframes fadeIn {
        from {
          opacity: 0;
        }
        to {
          opacity: 1;
        }
      }

      .backdrop {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(0, 0, 0, 0.5);
      }

      .dialog {
        background: white;
        max-width: 600px;
        width: 90%;
        max-height: 80vh;
        display: flex;
        flex-direction: column;
        position: relative;
        z-index: 1;
        animation: slideIn 0.2s ease-out;
      }

      @keyframes slideIn {
        from {
          transform: translateY(-20px);
          opacity: 0;
        }
        to {
          transform: translateY(0);
          opacity: 1;
        }
      }

      .content {
        flex: 1;
        padding: 20px;
        overflow-y: auto;
      }

      .frontmatter-display {
        width: 100%;
        min-height: 200px;
        padding: 12px;
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 14px;
        line-height: 1.4;
        background: #f8f9fa;
        border: 1px solid #e9ecef;
        resize: vertical;
        box-sizing: border-box;
        white-space: pre-wrap;
        word-wrap: break-word;
      }

      .loading,
      .error {
        display: flex;
        align-items: center;
        justify-content: center;
        min-height: 200px;
        font-size: 16px;
      }

      .loading {
        color: #666;
      }

      .error {
        color: #dc3545;
        flex-direction: column;
        gap: 8px;
      }

      .footer {
        display: flex;
        gap: 12px;
        padding: 16px 20px;
        border-top: 1px solid #e0e0e0;
        justify-content: flex-end;
      }

      .button {
        padding: 8px 16px;
        border: 1px solid;
        cursor: pointer;
        font-size: 14px;
        font-weight: 500;
        transition: all 0.2s;
      }

      .button:hover {
        transform: translateY(-1px);
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
      }

      .button-cancel {
        background: white;
        color: #666;
        border-color: #ddd;
      }

      .button-cancel:hover {
        background: #f8f9fa;
        border-color: #999;
      }

      .button-save {
        background: #007bff;
        color: white;
        border-color: #007bff;
      }

      .button-save:hover {
        background: #0056b3;
        border-color: #0056b3;
      }

      /* Mobile responsive styles */
      @media (max-width: 768px) {
        .dialog {
          width: 100%;
          height: 100%;
          max-width: none;
          max-height: none;
          border-radius: 0;
          margin: 0;
        }

        .header {
          padding: 12px 16px;
        }

        .title {
          font-size: 16px;
        }

        .content {
          padding: 16px;
        }

        .footer {
          padding: 12px 16px;
        }
      }
    `
  ];

  static override properties = {
    page: { type: String },
    open: { type: Boolean, reflect: true },
    loading: { state: true },
    error: { state: true },
    frontmatter: { state: true },
  };

  declare page: string;
  declare open: boolean;
  declare loading: boolean;
  declare error?: string;
  declare frontmatter?: GetFrontmatterResponse;

  private client = createClient(Frontmatter, createGrpcWebTransport({
    baseUrl: window.location.origin,
  }));

  constructor() {
    super();
    this.page = '';
    this.open = false;
    this.loading = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    // Handle escape key to close dialog
    document.addEventListener('keydown', this._handleKeydown);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('keydown', this._handleKeydown);
  }

  public _handleKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape' && this.open) {
      this._handleCancel();
    }
  };

  public openDialog(page: string): void {
    this.page = page;
    this.open = true;
    this.loadFrontmatter();
  }

  public close(): void {
    this.open = false;
    this.frontmatter = undefined;
    this.error = undefined;
    this.loading = false;
  }

  public async loadFrontmatter(): Promise<void> {
    if (!this.page) return;

    try {
      this.loading = true;
      this.error = undefined;
      this.frontmatter = undefined;
      this.requestUpdate();

      const request = new GetFrontmatterRequest({ page: this.page });
      const response = await this.client.getFrontmatter(request);
      this.frontmatter = response;
    } catch (err) {
      this.error = err instanceof Error ? err.message : 'Failed to load frontmatter';
    } finally {
      this.loading = false;
      this.requestUpdate();
    }
  }

  private _handleCancel = (): void => {
    this.close();
  };

  private _handleSaveClick = (): void => {
    // For now, just close the dialog
    // In future iterations, this would save the frontmatter
    this.close();
  };

  private formatFrontmatter(frontmatter?: GetFrontmatterResponse): string {
    if (!frontmatter?.frontmatter) {
      return '';
    }

    try {
      // Convert the protobuf Struct to a plain JavaScript object
      const jsonObject = frontmatter.frontmatter.toJson();
      return JSON.stringify(jsonObject, null, 2);
    } catch (err) {
      return `Error formatting frontmatter: ${err instanceof Error ? err.message : 'Unknown error'}`;
    }
  }

  override render() {
    return html`
      ${sharedStyles}
      <div class="backdrop"></div>
      <div class="dialog system-font border-radius box-shadow">
        <div class="dialog-header">
          <h2 class="dialog-title">Edit Frontmatter</h2>
        </div>
        <div class="content">
          ${this.loading ? html`
            <div class="loading">
              <i class="fas fa-spinner fa-spin"></i>
              Loading frontmatter...
            </div>
          ` : this.error ? html`
            <div class="error">
              <i class="fas fa-exclamation-triangle"></i>
              ${this.error}
            </div>
          ` : html`
            <div class="frontmatter-display border-radius-small">${this.formatFrontmatter(this.frontmatter)}</div>
          `}
        </div>
        <div class="footer">
          <button class="button button-cancel border-radius-small" @click="${this._handleCancel}">
            Cancel
          </button>
          <button class="button button-save border-radius-small" @click="${this._handleSaveClick}">
            Save
          </button>
        </div>
      </div>
    `;
  }
}

customElements.define('frontmatter-editor-dialog', FrontmatterEditorDialog);

declare global {
  interface HTMLElementTagNameMap {
    'frontmatter-editor-dialog': FrontmatterEditorDialog;
  }
}