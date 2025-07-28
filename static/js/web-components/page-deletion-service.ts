import { createClient } from '@connectrpc/connect';
import { getGrpcWebTransport } from './grpc-transport.js';
import { PageManagementService } from '../gen/api/v1/page_management_connect.js';
import { DeletePageRequest } from '../gen/api/v1/page_management_pb.js';
import { AugmentErrorService } from './augment-error-service.js';
import { showToastAfter } from './toast-message.js';
import './confirmation-dialog.js';
import { type ConfirmationConfig } from './confirmation-dialog.js';

/**
 * PageDeletionService - Handles page deletion workflow using the generic confirmation dialog
 * 
 * This service manages the complete page deletion flow:
 * 1. Shows confirmation dialog with page-specific messaging
 * 2. Handles the gRPC delete operation
 * 3. Manages success and error states
 * 4. Redirects user after successful deletion
 * 
 * Usage:
 * ```typescript
 * const service = new PageDeletionService();
 * service.confirmAndDeletePage('home');
 * ```
 */
export class PageDeletionService {
  private client = createClient(PageManagementService, getGrpcWebTransport());
  private dialog: HTMLElement & {
    openDialog: (config: ConfirmationConfig) => void;
    setLoading: (loading: boolean) => void;
    showError: (error: AugmentedError) => void;
    closeDialog: () => void;
    addEventListener: (type: string, listener: (event: Event) => void) => void;
    removeEventListener: (type: string, listener: (event: Event) => void) => void;
    dataset: { pageName?: string };
    id: string;
    hidden: boolean;
    parentNode?: { removeChild: (node: HTMLElement) => void };
  };

  constructor() {
    this.ensureDialogExists();
    this.setupEventListeners();
  }

  /**
   * Initiates the page deletion workflow
   * Shows confirmation dialog and handles the complete flow
   */
  confirmAndDeletePage(pageName: string) {
    if (!pageName) {
      console.error('PageDeletionService: pageName is required');
      return;
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
    this.dialog.dataset.pageName = pageName;
  }

  /**
   * Ensures the confirmation dialog element exists in the DOM
   */
  private ensureDialogExists() {
    this.dialog = document.querySelector('confirmation-dialog');
    
    if (!this.dialog) {
      this.dialog = document.createElement('confirmation-dialog');
      this.dialog.id = 'page-deletion-dialog';
      this.dialog.hidden = true;
      document.body.appendChild(this.dialog);
    }
  }

  /**
   * Sets up event listeners for the confirmation dialog
   */
  private setupEventListeners() {
    this.dialog.addEventListener('confirm', this.handleConfirm.bind(this));
    this.dialog.addEventListener('cancel', this.handleCancel.bind(this));
  }

  /**
   * Handles the confirm action - performs the actual page deletion
   */
  private async handleConfirm() {
    const pageName = this.dialog.dataset.pageName;
    
    if (!pageName) {
      console.error('PageDeletionService: No page name found for deletion');
      return;
    }

    // Set loading state
    this.dialog.setLoading(true);

    try {
      const request = new DeletePageRequest({
        pageName: pageName,
      });

      const response = await this.client.deletePage(request);

      if (response.success) {
        // Close dialog and show success message
        this.dialog.closeDialog();
        
        // Use showToastAfter to handle the toast display after redirect
        showToastAfter('Page deleted successfully', 'success', 5, () => {
          window.location.href = '/';
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
    delete this.dialog.dataset.pageName;
    
    // The dialog handles closing itself for cancel events
  }

  /**
   * Cleanup method - removes event listeners and dialog element
   * Call this if you need to destroy the service
   */
  destroy() {
    if (this.dialog) {
      this.dialog.removeEventListener('confirm', this.handleConfirm);
      this.dialog.removeEventListener('cancel', this.handleCancel);
      
      if (this.dialog.parentNode) {
        this.dialog.parentNode.removeChild(this.dialog);
      }
    }
  }
}

// Create a singleton instance for global use
export const pageDeleteService = new PageDeletionService();