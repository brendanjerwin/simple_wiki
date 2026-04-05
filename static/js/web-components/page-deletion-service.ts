import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import { PageManagementService, DeletePageRequestSchema } from '../gen/api/v1/page_management_pb.js';
import { AugmentErrorService } from './augment-error-service.js';
import { showToastAfter } from './toast-message.js';
import './confirmation-dialog.js';
import type { ConfirmationDialog } from './confirmation-dialog.js';

/**
 * PageDeleter - Handles page deletion workflow using the generic confirmation dialog
 *
 * This service manages the complete page deletion flow:
 * 1. Shows confirmation dialog with page-specific messaging
 * 2. Handles the gRPC delete operation
 * 3. Manages success and error states
 * 4. Redirects user after successful deletion
 *
 * Usage:
 * ```typescript
 * const service = new PageDeleter();
 * service.confirmAndDeletePage('home');
 * ```
 */
export class PageDeleter {
  private readonly client = createClient(PageManagementService, getGrpcWebTransport());
  private readonly dialog: ConfirmationDialog;

  // Store bound event handlers for proper cleanup
  private readonly boundHandleConfirm: (event: Event) => void;
  private readonly boundHandleCancel: (event: Event) => void;

  constructor() {
    // Bind event handlers once to ensure proper cleanup
    this.boundHandleConfirm = () => { void this.handleConfirm(); };
    this.boundHandleCancel = this.handleCancel.bind(this);

    this.dialog = this.ensureDialogExists();
    this.setupEventListeners();
  }

  /**
   * Initiates the page deletion workflow
   * Shows confirmation dialog and handles the complete flow
   */
  confirmAndDeletePage(pageName: string) {
    if (!pageName) {
      throw new Error('PageDeleter: pageName is required');
    }

    this.dialog.openDialog({
      message: 'Are you sure you want to delete this page?',
      description: `Page: ${pageName}`,
      confirmText: 'Delete Page',
      cancelText: 'Cancel',
      confirmVariant: 'danger',
      icon: 'warning',
      irreversible: true
    });

    // Store the page name for the deletion operation
    this.dialog.dataset['pageName'] = pageName;
  }

  /**
   * Ensures the confirmation dialog element exists in the DOM and returns it.
   */
  private ensureDialogExists(): ConfirmationDialog {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- custom element has known interface
    const existing = document.querySelector('confirmation-dialog') as ConfirmationDialog | null;

    if (existing) {
      return existing;
    }

    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- custom element has known interface
    const newDialog = document.createElement('confirmation-dialog') as ConfirmationDialog;
    newDialog.id = 'page-deletion-dialog';
    newDialog.hidden = true;
    document.body.appendChild(newDialog);
    return newDialog;
  }

  /**
   * Sets up event listeners for the confirmation dialog
   */
  private setupEventListeners() {
    this.dialog.addEventListener('confirm', this.boundHandleConfirm);
    this.dialog.addEventListener('cancel', this.boundHandleCancel);
  }

  /**
   * Handles the confirm action - performs the actual page deletion
   */
  private async handleConfirm() {
    const pageName = this.dialog.dataset['pageName'];

    if (!pageName) {
      const error = new Error('PageDeleter: No page name found for deletion');
      const augmentedError = AugmentErrorService.augmentError(error, 'delete page');
      this.dialog.showError(augmentedError);
      return;
    }

    // Set loading state
    this.dialog.setLoading(true);

    try {
      const request = create(DeletePageRequestSchema, {
        pageName: pageName,
      });

      const response = await this.client.deletePage(request);

      if (response.success) {
        // Close dialog and show success message
        this.dialog.closeDialog();
        
        // Use showToastAfter to handle the toast display after redirect
        showToastAfter('Page deleted successfully', 'success', 5, () => {
          location.href = '/';
        });
      } else {
        // Handle server-side error response
        const error = new Error(response.error || 'Failed to delete page');
        const augmentedError = AugmentErrorService.augmentError(error, 'delete page');
        this.dialog.showError(augmentedError);
      }
    } catch (err) {
      // Handle gRPC/network errors
      const augmentedError = AugmentErrorService.augmentError(err, 'delete page');
      this.dialog.showError(augmentedError);
    }

    // Note: We don't set loading to false here because either:
    // 1. The dialog closes (on success), or
    // 2. showError() sets loading to false (on error)
  }

  /**
   * Handles the cancel action
   */
  private handleCancel() {
    // Clean up the stored page name
    delete this.dialog.dataset['pageName'];
    
    // The dialog handles closing itself for cancel events
  }

  /**
   * Cleanup method - removes event listeners and dialog element
   * Call this if you need to destroy the service
   */
  destroy() {
    if (this.dialog) {
      this.dialog.removeEventListener('confirm', this.boundHandleConfirm);
      this.dialog.removeEventListener('cancel', this.boundHandleCancel);
      
      this.dialog.remove();
    }
  }
}

// Create a singleton instance for global use
export const pageDeleteService = new PageDeleter();