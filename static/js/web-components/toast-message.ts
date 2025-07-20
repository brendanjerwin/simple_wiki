import { html, css, LitElement } from 'lit';
import { sharedStyles, foundationCSS, buttonCSS } from './shared-styles.js';

/**
 * ToastMessage - A temporary notification component for user feedback
 * 
 * Features:
 * - Displays temporary success, error, warning, or info messages
 * - Auto-dismisses after a configurable timeout
 * - Supports manual closing via close button
 * - Uses smooth animations for show/hide
 * - Follows existing component patterns and styling
 */
export class ToastMessage extends LitElement {
  static override styles = [
    foundationCSS,
    buttonCSS,
    css`
      :host {
        position: fixed;
        top: 20px;
        right: 20px;
        z-index: 10000;
        display: block;
        max-width: 400px;
        min-width: 300px;
        opacity: 0;
        transform: translateX(100%);
        transition: all 0.3s ease-in-out;
      }

      :host([visible]) {
        opacity: 1;
        transform: translateX(0);
      }

      .toast {
        background: #ffffff;
        border-left: 4px solid var(--toast-color);
        padding: 16px;
        display: flex;
        align-items: flex-start;
        gap: 12px;
        position: relative;
      }

      .toast.success {
        --toast-color: #5cb85c;
      }

      .toast.error {
        --toast-color: #d9534f;
      }

      .toast.warning {
        --toast-color: #ffc107;
      }

      .toast.info {
        --toast-color: #6c757d;
      }

      .icon {
        flex-shrink: 0;
        width: 20px;
        height: 20px;
        color: var(--toast-color);
        margin-top: 2px;
      }

      .content {
        flex: 1;
        min-width: 0;
      }

      .message {
        font-size: 14px;
        line-height: 1.4;
        color: #333;
        margin: 0;
        word-wrap: break-word;
      }

      /* Mobile responsive */
      @media (max-width: 768px) {
        :host {
          top: 10px;
          right: 10px;
          left: 10px;
          max-width: none;
          min-width: auto;
        }
      }
    `
  ];

  static override properties = {
    message: { type: String },
    type: { type: String },
    visible: { type: Boolean, reflect: true },
    timeout: { type: Number },
    autoClose: { type: Boolean }
  };

  declare message: string;
  declare type: 'success' | 'error' | 'warning' | 'info';
  declare visible: boolean;
  declare timeout: number;
  declare autoClose: boolean;

  private timeoutId?: number;

  constructor() {
    super();
    this.message = '';
    this.type = 'info';
    this.visible = false;
    this.timeout = 5000; // 5 seconds default
    this.autoClose = true;
  }

  private getIcon(): string {
    switch (this.type) {
      case 'success':
        return '✓';
      case 'error':
        return '✕';
      case 'warning':
        return '⚠';
      case 'info':
      default:
        return 'ℹ';
    }
  }

  public show(): void {
    this.visible = true;
    
    if (this.autoClose && this.timeout > 0) {
      this.clearTimeout();
      this.timeoutId = window.setTimeout(() => {
        this.hide();
      }, this.timeout);
    }
  }

  public hide(): void {
    this.visible = false;
    this.clearTimeout();
    
    // Remove from DOM after animation completes
    setTimeout(() => {
      this.remove();
    }, 300);
  }

  private clearTimeout(): void {
    if (this.timeoutId) {
      window.clearTimeout(this.timeoutId);
      this.timeoutId = undefined;
    }
  }

  private _handleClose = (): void => {
    this.hide();
  };

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this.clearTimeout();
  }

  override render() {
    return html`
      ${sharedStyles}
      <div class="toast ${this.type} border-radius box-shadow system-font">
        <div class="icon" aria-hidden="true">
          ${this.getIcon()}
        </div>
        <div class="content">
          <p class="message">${this.message}</p>
        </div>
        <button 
          class="button-icon" 
          @click="${this._handleClose}"
          aria-label="Close notification"
          title="Close"
        >
          ✕
        </button>
      </div>
    `;
  }
}

customElements.define('toast-message', ToastMessage);

/**
 * Utility function to show a toast message
 */
export function showToast(
  message: string, 
  type: 'success' | 'error' | 'warning' | 'info' = 'info',
  timeout = 5000
): ToastMessage {
  const toast = document.createElement('toast-message') as ToastMessage;
  toast.message = message;
  toast.type = type;
  toast.timeout = timeout;
  
  document.body.appendChild(toast);
  
  // Show after a brief delay to ensure proper mounting
  requestAnimationFrame(() => {
    toast.show();
  });
  
  return toast;
}

/**
 * Show a success toast stored in sessionStorage (for post-refresh notifications)
 */
export function showStoredToast(): void {
  const storedMessage = sessionStorage.getItem('toast-message');
  const storedType = sessionStorage.getItem('toast-type') as 'success' | 'error' | 'warning' | 'info' || 'info';
  
  if (storedMessage) {
    sessionStorage.removeItem('toast-message');
    sessionStorage.removeItem('toast-type');
    showToast(storedMessage, storedType);
  }
}

/**
 * Store a toast message in sessionStorage for display after page refresh
 */
export function storeToastForRefresh(
  message: string, 
  type: 'success' | 'error' | 'warning' | 'info' = 'success'
): void {
  sessionStorage.setItem('toast-message', message);
  sessionStorage.setItem('toast-type', type);
}

declare global {
  interface HTMLElementTagNameMap {
    'toast-message': ToastMessage;
  }
}