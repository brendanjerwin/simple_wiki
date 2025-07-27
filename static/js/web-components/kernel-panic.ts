import { html, css, LitElement } from 'lit';
import { property } from 'lit/decorators.js';
import { foundationCSS } from './shared-styles.js';
import './error-display.js';
import type { AugmentedError } from './augment-error-service.js';

export class KernelPanic extends LitElement {
  static override styles = [
    foundationCSS,
    css`
      :host {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        z-index: 10000;
        background: #000;
        color: #fff;
        font-size: 14px;
        line-height: 1.4;
        padding: 20px;
        box-sizing: border-box;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        animation: fade-in 1.5s ease-in-out;
      }

      :host .monospace-font {
        /* Inherits monospace font from foundationCSS */
      }

      @keyframes fade-in {
      0% {
        opacity: 0;
        transform: scale(0.95);
      }
      100% {
        opacity: 1;
        transform: scale(1);
      }
    }

    .header {
      text-align: center;
      margin-bottom: 30px;
      border-bottom: 1px solid #333;
      padding-bottom: 20px;
    }

    .skull {
      font-size: 48px;
      margin-bottom: 10px;
      display: block;
    }

    .title {
      font-size: 24px;
      font-weight: bold;
      margin-bottom: 10px;
    }

    .subtitle {
      font-size: 16px;
      color: #aaa;
      margin-bottom: 20px;
    }

    .message {
      background: #111;
      border: 1px solid #333;
      border-radius: 4px;
      padding: 15px;
      margin-bottom: 20px;
      white-space: pre-wrap;
      word-break: break-word;
    }

    .error-details {
      background: #222;
      border: 1px solid #444;
      border-radius: 4px;
      padding: 15px;
      margin-bottom: 20px;
      max-height: 300px;
      overflow-y: auto;
    }

    .error-title {
      color: #ff6b6b;
      font-weight: bold;
      margin-bottom: 10px;
    }

    .error-stack {
      font-size: 12px;
      color: #ccc;
      white-space: pre-wrap;
      word-break: break-word;
    }

    .instructions {
      background: #1a1a1a;
      border: 1px solid #333;
      border-radius: 4px;
      padding: 15px;
      margin-top: auto;
    }

    .instruction-item {
      margin-bottom: 10px;
      padding-left: 20px;
      position: relative;
    }

    .instruction-item::before {
      content: '•';
      position: absolute;
      left: 0;
      color: #888;
    }

    .refresh-button {
      background: #333;
      border: 1px solid #555;
      border-radius: 4px;
      color: #fff;
      padding: 10px 20px;
      font-family: inherit;
      font-size: 14px;
      cursor: pointer;
      margin-top: 20px;
      align-self: flex-start;
    }

    .refresh-button:hover {
      background: #444;
      border-color: #666;
    }

    .refresh-button:active {
      background: #222;
    }
    `
  ];

  @property({ type: Object })
  augmentedError: AugmentedError | null = null;

  private _handleRefresh = (): void => {
    window.location.reload();
  };

  override render() {
    return html`
      <div class="header monospace-font">
        <span class="skull">💀</span>
        <div class="title">Kernel Panic</div>
        <div class="subtitle">A critical error has occurred</div>
      </div>

      ${this.augmentedError ? html`
        <error-display 
          .augmentedError=${this.augmentedError}
          style="background: #330000; border-color: #660000; color: #ffcccc;">
        </error-display>
      ` : ''}

      <div class="instructions monospace-font">
        <div class="instruction-item">The application has encountered an unrecoverable error</div>
        <div class="instruction-item">Your work may have been lost</div>
        <div class="instruction-item">Please refresh the page to restart the application</div>
        <div class="instruction-item">If this problem persists, contact system administrator</div>
        
        <button class="refresh-button" @click="${this._handleRefresh}">
          Refresh Page
        </button>
      </div>
    `;
  }
}

customElements.define('kernel-panic', KernelPanic);

/**
 * Creates and displays a kernel panic overlay for unrecoverable errors.
 * This function handles all the DOM manipulation needed to display the error.
 * 
 * @param augmentedError - The augmented error to display
 */
export function showKernelPanic(augmentedError: AugmentedError): void {
  const kernelPanic = document.createElement('kernel-panic') as HTMLElement & {
    augmentedError: AugmentedError;
  };
  kernelPanic.augmentedError = augmentedError;
  document.body.appendChild(kernelPanic);
}
