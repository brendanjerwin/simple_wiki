import { html, css, LitElement } from 'lit';
import { createClient } from '@connectrpc/connect';
import { Struct } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import { Frontmatter } from '../gen/api/v1/frontmatter_connect.js';
import { GetFrontmatterRequest, GetFrontmatterResponse, ReplaceFrontmatterRequest } from '../gen/api/v1/frontmatter_pb.js';
import { sharedStyles, foundationCSS, dialogCSS, responsiveCSS, buttonCSS } from './shared-styles.js';
import './frontmatter-value-section.js';
import './kernel-panic.js';
import { showKernelPanic } from './kernel-panic.js';
import { showToastAfter } from './toast-message.js';
import './error-display.js';
import { ErrorService } from './error-service.js';
import type { ErrorIcon } from './error-display.js';

/**
 * FrontmatterEditorDialog - A modal dialog for editing page frontmatter metadata
 * 
 * WORKING THEORY:
 * This component manages the complete lifecycle of frontmatter editing through several key state variables:
 * 
 * - `frontmatter`: The original server response containing the current frontmatter data (read-only)
 * - `workingFrontmatter`: A mutable working copy of the frontmatter data that users can edit
 * - `loading`: Indicates whether the component is fetching data from the server
 * - `error`: Contains any error message from server operations
 * - `open`: Controls the visibility state of the modal dialog
 * 
 * DATA FLOW:
 * 1. When opened, the dialog fetches current frontmatter via gRPC and stores it in `frontmatter`
 * 2. `convertStructToPlainObject()` converts the protobuf Struct to a plain JavaScript object
 * 3. This converted data is copied to `workingFrontmatter` for editing
 * 4. The frontmatter-value-section component renders and manages all field editing operations
 * 5. All user modifications update `workingFrontmatter` while preserving the original `frontmatter`
 * 6. On save, `workingFrontmatter` is sent back to the server; on cancel, changes are discarded
 * 
 * COMPONENT ARCHITECTURE:
 * The dialog uses a hierarchical component structure:
 * - frontmatter-value-section: Root container that handles the main frontmatter object
 * - frontmatter-key: Manages editable key names with label-like styling
 * - frontmatter-value: Dispatcher that delegates to appropriate value components
 * - frontmatter-value-string: Handles individual string fields
 * - frontmatter-value-array: Manages arrays of string values
 * - frontmatter-add-field-button: Provides dropdown for adding new fields/arrays/sections
 * 
 * This separation allows for clean state management, proper event bubbling, and maintainable code.
 */
export class FrontmatterEditorDialog extends LitElement {
  static override styles = [
    foundationCSS,
    dialogCSS,
    responsiveCSS,
    buttonCSS,
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
        border-radius: 8px;
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

      /* Mobile-first responsive behavior */
      @media (max-width: 768px) {
        :host([open]) {
          align-items: stretch;
          justify-content: stretch;
        }

        .dialog {
          width: 100%;
          height: 100%;
          max-width: none;
          max-height: none;
          border-radius: 0;
          margin: 0;
        }
      }

      .content {
        flex: 1;
        padding: 20px;
        overflow-y: auto;
        min-height: 150px;
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
    `
  ];

  static override properties = {
    page: { type: String },
    open: { type: Boolean, reflect: true },
    loading: { state: true },
    saving: { state: true },
    error: { state: true },
    errorDetails: { state: true },
    errorIcon: { state: true },
    frontmatter: { state: true },
    workingFrontmatter: { state: true },
  };

  declare page: string;
  declare open: boolean;
  declare loading: boolean;
  declare saving: boolean;
  declare error?: string | undefined;
  declare errorDetails?: string | undefined;
  declare errorIcon?: ErrorIcon | undefined;
  declare frontmatter?: GetFrontmatterResponse | undefined;
  declare workingFrontmatter?: Record<string, unknown>;

  private client = createClient(Frontmatter, getGrpcWebTransport());

  constructor() {
    super();
    this.page = '';
    this.open = false;
    this.loading = false;
    this.saving = false;
    this.workingFrontmatter = {};
  }

  private convertStructToPlainObject(struct?: Struct): Record<string, unknown> {
    if (!struct) return {};

    try {
      return struct.toJson() as Record<string, unknown>;
    } catch (err) {
      // This is an unrecoverable error - the protobuf data is corrupted
      showKernelPanic('Failed to convert frontmatter data structure', err as Error);
      throw err;
    }
  }

  private convertPlainObjectToStruct(obj: Record<string, unknown>): Struct {
    try {
      return Struct.fromJson(obj);
    } catch (err) {
      // This is an unrecoverable error - the data is corrupted
      showKernelPanic('Failed to convert plain object to protobuf Struct', err as Error);
      throw err;
    }
  }

  private updateWorkingFrontmatter(): void {
    if (this.frontmatter?.frontmatter) {
      this.workingFrontmatter = this.convertStructToPlainObject(this.frontmatter.frontmatter);
    } else {
      this.workingFrontmatter = {};
    }
  }

  private _handleSectionChange = (event: CustomEvent): void => {
    const { newFields } = event.detail;
    this.workingFrontmatter = newFields;
    this.requestUpdate();
  };

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
    this.errorDetails = undefined;
    this.loading = false;
    this.saving = false;
  }

  public async loadFrontmatter(): Promise<void> {
    if (!this.page) return;

    try {
      this.loading = true;
      this.error = undefined;
      this.errorDetails = undefined;
      this.errorIcon = undefined;
      this.frontmatter = undefined;
      this.workingFrontmatter = {};
      this.requestUpdate();

      const request = new GetFrontmatterRequest({ page: this.page });
      const response = await this.client.getFrontmatter(request);
      this.frontmatter = response;
      this.updateWorkingFrontmatter();
    } catch (err) {
      const processedError = ErrorService.processError(err, 'load frontmatter');
      this.error = processedError.message;
      this.errorDetails = processedError.details;
      this.errorIcon = processedError.icon;
    } finally {
      this.loading = false;
      this.requestUpdate();
    }
  }

  private _handleCancel = (): void => {
    this.close();
  };

  private refreshPage(): void {
    window.location.reload();
  }

  private _handleSaveClick = async (): Promise<void> => {
    if (!this.page || !this.workingFrontmatter) return;

    try {
      this.saving = true;
      this.error = undefined;
      this.errorDetails = undefined;
      this.errorIcon = undefined;
      this.requestUpdate();

      const frontmatterStruct = this.convertPlainObjectToStruct(this.workingFrontmatter);
      const request = new ReplaceFrontmatterRequest({ 
        page: this.page, 
        frontmatter: frontmatterStruct 
      });
      
      const response = await this.client.replaceFrontmatter(request);
      
      // Update the stored frontmatter with the response to reflect any server-side changes
      if (response.frontmatter) {
        this.frontmatter = new GetFrontmatterResponse({ frontmatter: response.frontmatter });
        this.updateWorkingFrontmatter();
      }
      
      // Store success message and close dialog with page refresh
      showToastAfter('Frontmatter saved successfully!', 'success', 5, () => {
        // Close the dialog
        this.close();
        
        // Refresh the page to show updated content with new frontmatter
        this.refreshPage();
      });
    } catch (err) {
      const processedError = ErrorService.processError(err, 'save frontmatter');
      this.error = processedError.message;
      this.errorDetails = processedError.details;
      this.errorIcon = processedError.icon;
    } finally {
      this.saving = false;
      this.requestUpdate();
    }
  };

  private renderFrontmatterEditor(): unknown {
    return html`
      <frontmatter-value-section
        .fields="${this.workingFrontmatter || {}}"
        .isRoot="${true}"
        @section-change="${this._handleSectionChange}"
      ></frontmatter-value-section>
    `;
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
            <error-display 
              .message=${this.error}
              .details=${this.errorDetails}
              .icon=${this.errorIcon}>
            </error-display>
          ` : html`
            ${this.renderFrontmatterEditor()}
          `}
        </div>
        <div class="footer">
          <button class="button-base button-secondary button-large border-radius-small" @click="${this._handleCancel}" ?disabled="${this.saving}">
            Cancel
          </button>
          <button class="button-base button-primary button-large border-radius-small" @click="${this._handleSaveClick}" ?disabled="${this.saving || this.loading}">
            ${this.saving ? 'Saving...' : 'Save'}
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
