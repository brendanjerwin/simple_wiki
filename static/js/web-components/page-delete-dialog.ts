import { html, css, LitElement } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { getGrpcWebTransport } from './grpc-transport.js';
import { PageManagementService } from '../gen/api/v1/page_management_connect.js';
import { DeletePageRequest } from '../gen/api/v1/page_management_pb.js';
import { foundationCSS, dialogCSS, responsiveCSS, buttonCSS } from './shared-styles.js';
import { showToastAfter } from './toast-message.js';
import './error-display.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';

/**
 * PageDeleteDialog - A modal dialog for confirming page deletion
 * 
 * This component provides a confirmation dialog before deleting a page via gRPC.
 * It follows the same pattern as frontmatter-editor-dialog for consistency.
 * 
 * Features:
 * - Shows confirmation dialog with page name
 * - Calls DeletePage gRPC service
 * - Shows success toast and redirects to homepage on success
 * - Shows error toast on failure
 * - Handles all error cases gracefully
 */
export class PageDeleteDialog extends LitElement {
  static override styles = [
    foundationCSS,
    dialogCSS,
    responsiveCSS,
    buttonCSS,
    css`
      .dialog-content {
        padding: 24px;
        min-width: 400px;
        max-width: 500px;
      }

      .warning-icon {
        color: #d9534f;
        font-size: 48px;
        text-align: center;
        margin-bottom: 16px;
      }

      .confirmation-message {
        text-align: center;
        margin-bottom: 24px;
        line-height: 1.5;
      }

      .page-name {
        font-weight: bold;
        color: #333;
        background: #f8f9fa;
        padding: 4px 8px;
        border-radius: 4px;
        font-family: monospace;
      }

      .dialog-actions {
        display: flex;
        gap: 12px;
        justify-content: flex-end;
        margin-top: 24px;
      }

      .button {
        padding: 8px 16px;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 14px;
        font-weight: 500;
        transition: background-color 0.2s;
      }

      .button:disabled {
        opacity: 0.6;
        cursor: not-allowed;
      }

      .button-cancel {
        background: #6c757d;
        color: white;
      }

      .button-cancel:hover:not(:disabled) {
        background: #5a6268;
      }

      .button-delete {
        background: #d9534f;
        color: white;
      }

      .button-delete:hover:not(:disabled) {
        background: #c9302c;
      }

      .dialog-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background: rgba(0, 0, 0, 0.5);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 1000;
      }

      .dialog-box {
        background: white;
        border-radius: 8px;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
        max-height: 90vh;
        overflow-y: auto;
      }

      :host([hidden]) {
        display: none !important;
      }
    `
  ];

  @property({ type: String }) pageName = '';
  @state() private loading = false;
  @state() private augmentedError: AugmentedError | undefined;
  @state() private open = false;

  private client = createClient(PageManagementService, getGrpcWebTransport());

  /**
   * Opens the delete confirmation dialog for the specified page
   */
  openDialog(pageName: string) {
    this.pageName = pageName;
    this.augmentedError = undefined;
    this.open = true;
    this.hidden = false;
  }

  /**
   * Closes the dialog
   */
  private closeDialog() {
    this.open = false;
    this.hidden = true;
    this.loading = false;
    this.augmentedError = undefined;
  }

  /**
   * Handles the delete confirmation
   */
  private async handleDelete() {
    if (!this.pageName || this.loading) {
      return;
    }

    this.loading = true;
    this.augmentedError = undefined;

    try {
      const request = new DeletePageRequest({
        pageName: this.pageName,
      });

      const response = await this.client.deletePage(request);

      if (response.success) {
        // Close dialog and show success message
        this.closeDialog();
        
        // Use showToastAfter to handle the toast display after redirect
        showToastAfter(() => {
          window.location.href = '/';
        }, 'Page deleted successfully', 'success');
      } else {
        // Handle server-side error response
        const error = new Error(response.error || 'Failed to delete page');
        this.augmentedError = AugmentErrorService.augmentError(error, 'delete page');
      }
    } catch (err) {
      // Handle gRPC/network errors
      this.augmentedError = AugmentErrorService.augmentError(err, 'delete page');
    } finally {
      this.loading = false;
    }
  }

  /**
   * Handles the cancel action
   */
  private handleCancel() {
    this.closeDialog();
  }

  /**
   * Handles clicking outside the dialog
   */
  private handleOverlayClick(event: Event) {
    if (event.target === event.currentTarget) {
      this.handleCancel();
    }
  }

  override render() {
    if (!this.open) {
      return html``;
    }

    return html`
      <div class="dialog-overlay" @click=${this.handleOverlayClick}>
        <div class="dialog-box">
          <div class="dialog-content">
            <div class="warning-icon">⚠️</div>
            
            <div class="confirmation-message">
              <p>Are you sure you want to delete this page?</p>
              <p>Page: <span class="page-name">${this.pageName}</span></p>
              <p><strong>This action cannot be undone.</strong></p>
            </div>

            ${this.augmentedError ? html`
              <error-display .augmentedError=${this.augmentedError}></error-display>
            ` : ''}

            <div class="dialog-actions">
              <button 
                class="button button-cancel" 
                @click=${this.handleCancel}
                ?disabled=${this.loading}
              >
                Cancel
              </button>
              <button 
                class="button button-delete" 
                @click=${this.handleDelete}
                ?disabled=${this.loading}
              >
                ${this.loading ? 'Deleting...' : 'Delete Page'}
              </button>
            </div>
          </div>
        </div>
      </div>
    `;
  }
}

customElements.define('page-delete-dialog', PageDeleteDialog);