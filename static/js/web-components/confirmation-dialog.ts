import { html, css, LitElement } from 'lit';
import { state } from 'lit/decorators.js';
import { colorCSS, typographyCSS, themeCSS, foundationCSS, dialogCSS, responsiveCSS, buttonCSS } from './shared-styles.js';
import './error-display.js';
import { type AugmentedError, type ErrorIcon, AugmentErrorService } from './augment-error-service.js';

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
  /** Icon to display (default: "warning") */
  icon?: ErrorIcon;
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
 * - Click-outside-to-close functionality
 * - Standardized icon system (supports both standard error icons and custom strings)
 * 
 * Usage:
 * ```typescript
 * const dialog = document.querySelector('confirmation-dialog');
 * dialog.openDialog({
 *   message: 'Are you sure you want to delete this item?',
 *   description: 'This action cannot be undone.',
 *   confirmText: 'Delete',
 *   confirmVariant: 'danger',
 *   icon: 'warning' // Uses standardized error icon system
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
    colorCSS,
    typographyCSS,
    themeCSS,
    foundationCSS,
    dialogCSS,
    responsiveCSS,
    buttonCSS,
    css`
      :host {
        display: none;
        font-size: 11px;
        line-height: 1.2;
      }
      
      :host([open]) {
        display: block;
      }

      .dialog-content {
        padding: 16px;
        min-width: 320px;
        max-width: 420px;
      }

      .dialog-icon {
        font-size: 24px;
        text-align: center;
        margin-bottom: 12px;
        opacity: 0.9;
      }

      .dialog-icon.warning {
        color: #dc3545;
      }

      .dialog-icon.info {
        color: #6c757d;
      }

      .dialog-message {
        text-align: center;
        margin-bottom: 12px;
        font-weight: 600;
      }

      .dialog-description {
        text-align: center;
        margin-bottom: 16px;
      }

      .dialog-description.irreversible {
        font-weight: 600;
        color: #dc3545;
      }

      .dialog-actions {
        display: flex;
        gap: 8px;
        justify-content: flex-end;
        margin-top: 16px;
      }

      .button {
        padding: 6px 12px;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-weight: 500;
        transition: all 0.2s ease;
        min-width: 64px;
      }

      .button:disabled {
        opacity: 0.6;
        cursor: not-allowed;
      }

      .button:hover:not(:disabled) {
        transform: translateY(-1px);
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
      }

      .button:active {
        transform: translateY(0);
      }

      .button-cancel {
        background: #6c757d;
        color: white;
        border: 1px solid #6c757d;
      }

      .button-cancel:hover:not(:disabled) {
        background: #5a6268;
        border-color: #5a6268;
      }

      .button-primary {
        background: #007bff;
        color: white;
        border: 1px solid #007bff;
      }

      .button-primary:hover:not(:disabled) {
        background: #0056b3;
        border-color: #0056b3;
      }

      .button-danger {
        background: #dc3545;
        color: white;
        border: 1px solid #dc3545;
      }

      .button-danger:hover:not(:disabled) {
        background: #c82333;
        border-color: #c82333;
      }

      .button-warning {
        background: #ffc107;
        color: #212529;
        border: 1px solid #ffc107;
      }

      .button-warning:hover:not(:disabled) {
        background: #e0a800;
        border-color: #e0a800;
      }

      .dialog-box {
        max-height: 90vh;
        overflow-y: auto;
      }

      /* Mobile responsive */
      @media (max-width: 768px) {
        .dialog-content {
          padding: 12px;
          min-width: 280px;
          max-width: 320px;
        }

        .dialog-icon {
          font-size: 20px;
          margin-bottom: 8px;
        }

        .dialog-message {
          font-size: 11px;
          margin-bottom: 8px;
        }

        .dialog-description {
          font-size: 10px;
          margin-bottom: 12px;
        }

        .dialog-actions {
          gap: 6px;
          margin-top: 12px;
        }

        .button {
          padding: 4px 8px;
          font-size: 10px;
          min-width: 48px;
        }
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
    this.style.setProperty('display', 'block', 'important');
  }

  /**
   * Closes the dialog and cleans up
   */
  closeDialog() {
    this.open = false;
    this.removeAttribute('open');
    this.style.setProperty('display', 'none', 'important');
    this.loading = false;
    this.augmentedError = undefined;
    this.config = null;
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
      <div class="overlay" @click=${this.handleOverlayClick}>
        <div class="container container-modal dialog-box">
          <div class="dialog-content panel gap-sm">
            <div class="dialog-icon ${iconClass}">
              ${AugmentErrorService.getIconString(config.icon || 'warning')}
            </div>
            
            <div class="dialog-message text-primary font-mono text-base">
              ${config.message}
            </div>

            ${config.description ? html`
              <div class="dialog-description text-muted font-mono text-sm ${config.irreversible ? 'irreversible' : ''}">
                ${config.description}
              </div>
            ` : ''}

            ${config.irreversible ? html`
              <div class="dialog-description text-error font-mono text-sm irreversible">
                This action cannot be undone.
              </div>
            ` : ''}

            ${this.augmentedError ? html`
              <error-display .augmentedError=${this.augmentedError}></error-display>
            ` : ''}

            <div class="dialog-actions">
              <button 
                class="button button-cancel font-mono text-sm" 
                @click=${this.handleCancel}
                ?disabled=${this.loading}
              >
                ${config.cancelText || 'Cancel'}
              </button>
              <button 
                class="${confirmButtonClass} font-mono text-sm" 
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

declare global {
  interface HTMLElementTagNameMap {
    'confirmation-dialog': ConfirmationDialog;
  }
}