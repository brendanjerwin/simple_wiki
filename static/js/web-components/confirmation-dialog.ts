import { html, css, LitElement } from 'lit';
import { state } from 'lit/decorators.js';
import { foundationCSS, dialogCSS, responsiveCSS, buttonCSS } from './shared-styles.js';
import './error-display.js';
import { type AugmentedError } from './augment-error-service.js';

/**
 * Configuration for the confirmation dialog
 */
export interface ConfirmationConfig {
  /** The main question or message to display */
  message: string;
  /** Optional detailed description */
  description?: string;
  /** Text for the confirm button (default: "Confirm") */
  confirmText?: string;
  /** Text for the cancel button (default: "Cancel") */
  cancelText?: string;
  /** Style variant for the confirm button (default: "danger") */
  confirmVariant?: 'primary' | 'danger' | 'warning';
  /** Icon to display (default: "⚠️") */
  icon?: string;
  /** Whether the action can be undone (affects messaging) */
  irreversible?: boolean;
}

/**
 * ConfirmationDialog - A generic modal dialog for confirming actions
 * 
 * This reusable component provides a confirmation dialog that can be customized
 * for different use cases while maintaining consistent styling and behavior.
 * 
 * Features:
 * - Configurable message, buttons, and styling
 * - Loading states during async operations
 * - Error display integration
 * - Keyboard shortcuts (Enter to confirm, Escape to cancel)
 * - Click-outside-to-close functionality
 * - Customizable icons and button variants
 * 
 * Usage:
 * ```typescript
 * const dialog = document.querySelector('confirmation-dialog');
 * dialog.openDialog({
 *   message: 'Are you sure you want to delete this item?',
 *   description: 'This action cannot be undone.',
 *   confirmText: 'Delete',
 *   confirmVariant: 'danger'
 * });
 * 
 * dialog.addEventListener('confirm', async (event) => {
 *   try {
 *     await performAction();
 *     dialog.closeDialog();
 *   } catch (error) {
 *     dialog.showError(errorService.processError(error));
 *   }
 * });
 * ```
 */
export class ConfirmationDialog extends LitElement {
  static override styles = [
    foundationCSS,
    dialogCSS,
    responsiveCSS,
    buttonCSS,
    css`
      :host {
        display: none;
      }
      
      :host([open]) {
        display: block;
      }

      .dialog-content {
        padding: 24px;
        min-width: 400px;
        max-width: 500px;
      }

      .dialog-icon {
        font-size: 48px;
        text-align: center;
        margin-bottom: 16px;
      }

      .dialog-icon.warning {
        color: #d9534f;
      }

      .dialog-icon.info {
        color: #5bc0de;
      }

      .dialog-message {
        text-align: center;
        margin-bottom: 16px;
        line-height: 1.5;
        font-size: 16px;
        font-weight: 500;
      }

      .dialog-description {
        text-align: center;
        margin-bottom: 24px;
        line-height: 1.5;
        color: #666;
      }

      .dialog-description.irreversible {
        font-weight: bold;
        color: #d9534f;
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
        min-width: 80px;
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

      .button-primary {
        background: #007bff;
        color: white;
      }

      .button-primary:hover:not(:disabled) {
        background: #0056b3;
      }

      .button-danger {
        background: #d9534f;
        color: white;
      }

      .button-danger:hover:not(:disabled) {
        background: #c9302c;
      }

      .button-warning {
        background: #f0ad4e;
        color: white;
      }

      .button-warning:hover:not(:disabled) {
        background: #ec971f;
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
    `
  ];

  @state() private config: ConfirmationConfig | null = null;
  @state() private loading = false;
  @state() private augmentedError: AugmentedError | undefined;
  @state() private open = false;

  /**
   * Opens the confirmation dialog with the specified configuration
   */
  openDialog(config: ConfirmationConfig) {
    this.config = config;
    this.augmentedError = undefined;
    this.loading = false;
    this.open = true;
    this.setAttribute('open', '');
    
    // Set up keyboard event listeners
    this.addEventListener('keydown', this.handleKeydown);
  }

  /**
   * Closes the dialog and cleans up
   */
  closeDialog() {
    this.open = false;
    this.removeAttribute('open');
    this.loading = false;
    this.augmentedError = undefined;
    this.config = null;
    
    // Clean up keyboard event listeners
    this.removeEventListener('keydown', this.handleKeydown);
  }

  /**
   * Shows an error in the dialog without closing it
   */
  showError(error: AugmentedError) {
    this.augmentedError = error;
    this.loading = false;
  }

  /**
   * Sets the loading state of the dialog
   */
  setLoading(loading: boolean) {
    this.loading = loading;
  }

  /**
   * Handles keyboard shortcuts
   */
  private handleKeydown = (event: KeyboardEvent) => {
    if (!this.open) return;
    
    switch (event.key) {
      case 'Escape':
        event.preventDefault();
        this.handleCancel();
        break;
      case 'Enter':
        if (event.ctrlKey || event.metaKey) {
          event.preventDefault();
          this.handleConfirm();
        }
        break;
    }
  };

  /**
   * Handles the confirm action
   */
  private handleConfirm() {
    if (this.loading) return;
    
    // Dispatch custom event that consumers can listen to
    const confirmEvent = new CustomEvent('confirm', {
      detail: { config: this.config },
      bubbles: true,
      composed: true
    });
    
    this.dispatchEvent(confirmEvent);
  }

  /**
   * Handles the cancel action
   */
  private handleCancel() {
    if (this.loading) return;
    
    // Dispatch custom event
    const cancelEvent = new CustomEvent('cancel', {
      detail: { config: this.config },
      bubbles: true,
      composed: true
    });
    
    this.dispatchEvent(cancelEvent);
    
    // Default behavior: close the dialog
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
    if (!this.open || !this.config) {
      return html``;
    }

    const config = this.config;
    const iconClass = config.confirmVariant === 'danger' ? 'warning' : 'info';
    const confirmButtonClass = `button button-${config.confirmVariant || 'danger'}`;
    
    return html`
      <div class="dialog-overlay" @click=${this.handleOverlayClick}>
        <div class="dialog-box">
          <div class="dialog-content">
            <div class="dialog-icon ${iconClass}">
              ${config.icon || '⚠️'}
            </div>
            
            <div class="dialog-message">
              ${config.message}
            </div>

            ${config.description ? html`
              <div class="dialog-description ${config.irreversible ? 'irreversible' : ''}">
                ${config.description}
              </div>
            ` : ''}

            ${config.irreversible ? html`
              <div class="dialog-description irreversible">
                This action cannot be undone.
              </div>
            ` : ''}

            ${this.augmentedError ? html`
              <error-display .augmentedError=${this.augmentedError}></error-display>
            ` : ''}

            <div class="dialog-actions">
              <button 
                class="button button-cancel" 
                @click=${this.handleCancel}
                ?disabled=${this.loading}
              >
                ${config.cancelText || 'Cancel'}
              </button>
              <button 
                class="${confirmButtonClass}" 
                @click=${this.handleConfirm}
                ?disabled=${this.loading}
              >
                ${this.loading ? 'Processing...' : (config.confirmText || 'Confirm')}
              </button>
            </div>
          </div>
        </div>
      </div>
    `;
  }
}

customElements.define('confirmation-dialog', ConfirmationDialog);