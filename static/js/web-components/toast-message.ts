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
        cursor: pointer;
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
        position: absolute;
        top: 8px;
        left: 8px;
        font-size: 32px;
        opacity: 0.15;
        z-index: 0;
        color: var(--toast-color);
        pointer-events: none;
      }

      .content {
        flex: 1;
        min-width: 0;
        position: relative;
        z-index: 1;
        margin-left: 20px; /* Add margin to account for ambient icon */
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
    timeoutMs: { type: Number },
    autoClose: { type: Boolean }
  };

  declare message: string;
  declare type: 'success' | 'error' | 'warning' | 'info';
  declare visible: boolean;
  declare timeoutMs: number;
  declare autoClose: boolean;

  private timeoutId?: number;

  constructor() {
    super();
    this.message = '';
    this.type = 'info';
    this.visible = false;
    this.timeoutMs = 5000; // 5 seconds default
    this.autoClose = true;
  }

  private getIcon(): string {
    switch (this.type) {
      case 'success':
        return '✅';
      case 'error':
        return '❌';
      case 'warning':
        return '⚠️';
      case 'info':
      default:
        return 'ℹ️';
    }
  }

  public show(): void {
    this.visible = true;
    
    if (this.autoClose && this.timeoutMs > 0) {
      this.clearTimeout();
      this.timeoutId = window.setTimeout(() => {
        this.hide();
      }, this.timeoutMs);
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

  private _handleToastClick = (): void => {
    this.hide();
  };

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this.clearTimeout();
  }

  override render() {
    return html`
      ${sharedStyles}
      <div class="toast ${this.type} border-radius box-shadow system-font" @click="${this._handleToastClick}">
        <div class="icon" aria-hidden="true">
          ${this.getIcon()}
        </div>
        <div class="content">
          <p class="message">${this.message}</p>
        </div>
      </div>
    `;
  }
}

customElements.define('toast-message', ToastMessage);

/**
 * Show a toast after executing a function, with sessionStorage persistence across page refreshes
 */
export function showToastAfter(
  message: string, 
  type: 'success' | 'error' | 'warning' | 'info' = 'info',
  fn: () => void
): void {
  // Store the toast message for post-execution display
  sessionStorage.setItem('toast-message', message);
  sessionStorage.setItem('toast-type', type);
  
  // Execute the provided function
  fn();
  
  // Show the stored toast (useful if fn doesn't cause a page refresh)
  showStoredToast();
}

/**
 * Show a toast immediately (convenience method using showToastAfter)
 */
export function showToast(
  message: string, 
  type: 'success' | 'error' | 'warning' | 'info' = 'info'
): void {
  showToastAfter(message, type, () => {
    // No-op function - just show the toast immediately
  });
}

/**
 * Show a success toast stored in sessionStorage (for post-refresh notifications)
 */
export function showStoredToast(): void {
  const storedMessage = sessionStorage.getItem('toast-message');
  const storedTypeRaw = sessionStorage.getItem('toast-type');
  
  if (storedMessage) {
    // Validate the stored type, default to 'info' if invalid
    const validTypes = ['success', 'error', 'warning', 'info'] as const;
    type ValidType = typeof validTypes[number];
    const storedType = validTypes.includes(storedTypeRaw as ValidType) ? storedTypeRaw as ValidType : 'info';
    
    sessionStorage.removeItem('toast-message');
    sessionStorage.removeItem('toast-type');
    
    // Create and show the toast immediately
    const toast = document.createElement('toast-message') as ToastMessage;
    toast.message = storedMessage;
    toast.type = storedType;
    toast.timeoutMs = 5000;
    
    document.body.appendChild(toast);
    
    requestAnimationFrame(() => {
      toast.show();
    });
  }
}



declare global {
  interface HTMLElementTagNameMap {
    'toast-message': ToastMessage;
  }
}