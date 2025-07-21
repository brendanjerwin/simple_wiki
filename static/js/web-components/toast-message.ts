import { html, css, LitElement } from 'lit';
import { sharedStyles, foundationCSS, buttonCSS } from './shared-styles.js';

// Valid toast types - defined once for consistency
const TOAST_TYPES = ['success', 'error', 'warning', 'info'] as const;
type ToastType = typeof TOAST_TYPES[number];

// Storage keys for sessionStorage persistence
const STORAGE_KEYS = {
  MESSAGE: 'toast-message',
  TYPE: 'toast-type',
  TIMEOUT: 'toast-timeout'
} as const;

// Animation duration in milliseconds - matches CSS transition
const ANIMATION_DURATION_MS = 300;

/**
 * ToastMessage - A temporary notification component for user feedback
 * 
 * Features:
 * - Displays temporary success, error, warning, or info messages
 * - Auto-dismisses after a configurable timeout
 * - Supports manual closing via clicking anywhere on toast
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
        bottom: 8px;
        right: 8px;
        font-size: 48px;
        font-weight: 900;
        opacity: 0.15;
        z-index: 0;
        color: var(--toast-color);
        pointer-events: none;
        line-height: 1;
      }

      .content {
        flex: 1;
        min-width: 0;
        position: relative;
        z-index: 1;
        margin-right: 40px; /* Add margin to account for ambient icon in bottom-right */
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
    timeoutSeconds: { type: Number },
    autoClose: { type: Boolean }
  };

  declare message: string;
  declare type: ToastType;
  declare visible: boolean;
  declare timeoutSeconds: number;
  declare autoClose: boolean;

  private timeoutId?: number;

  constructor() {
    super();
    // No defaults - component must be fully configured
    // Exceptions are preferred over accidental success
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
    
    if (this.autoClose && this.timeoutSeconds > 0) {
      this.clearTimeout();
      this.timeoutId = window.setTimeout(() => {
        this.hide();
      }, this.timeoutSeconds * 1000);
    }
  }

  public hide(): void {
    this.visible = false;
    this.clearTimeout();
    
    // Remove from DOM after animation completes
    setTimeout(() => {
      this.remove();
    }, ANIMATION_DURATION_MS);
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
  type: ToastType,
  timeoutSeconds: number,
  fn: () => void
): void {
  // Store the toast message for post-execution display
  sessionStorage.setItem(STORAGE_KEYS.MESSAGE, message);
  sessionStorage.setItem(STORAGE_KEYS.TYPE, type);
  sessionStorage.setItem(STORAGE_KEYS.TIMEOUT, timeoutSeconds.toString());
  
  // Execute the provided function
  fn();
  
  // Wait a moment for any async work to complete before showing toast
  setTimeout(() => {
    showStoredToast();
  }, 100);
}

/**
 * Show a toast immediately (convenience method)
 */
export function showToast(
  message: string, 
  type: ToastType,
  timeoutSeconds: number
): void {
  // Create and show the toast immediately
  const toast = document.createElement('toast-message') as ToastMessage;
  toast.message = message;
  toast.type = type;
  toast.timeoutSeconds = timeoutSeconds;
  toast.autoClose = true;
  toast.visible = false;
  
  document.body.appendChild(toast);
  
  requestAnimationFrame(() => {
    toast.show();
  });
}

/**
 * Show a success toast stored in sessionStorage (for post-refresh notifications)
 */
export function showStoredToast(): void {
  const storedMessage = sessionStorage.getItem(STORAGE_KEYS.MESSAGE);
  const storedTypeRaw = sessionStorage.getItem(STORAGE_KEYS.TYPE);
  const storedTimeoutRaw = sessionStorage.getItem(STORAGE_KEYS.TIMEOUT);
  
  if (storedMessage) {
    // Validate the stored type against valid types
    const storedType = TOAST_TYPES.includes(storedTypeRaw as ToastType) ? storedTypeRaw as ToastType : 'info';
    const storedTimeout = storedTimeoutRaw ? parseInt(storedTimeoutRaw, 10) : 5;
    
    sessionStorage.removeItem(STORAGE_KEYS.MESSAGE);
    sessionStorage.removeItem(STORAGE_KEYS.TYPE);
    sessionStorage.removeItem(STORAGE_KEYS.TIMEOUT);
    
    // Create and show the toast immediately
    const toast = document.createElement('toast-message') as ToastMessage;
    toast.message = storedMessage;
    toast.type = storedType;
    toast.timeoutSeconds = storedTimeout;
    toast.autoClose = true;
    toast.visible = false;
    
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