import { html, css, LitElement } from 'lit';
import { property, state } from 'lit/decorators.js';
import { foundationCSS, buttonCSS } from './shared-styles.js';
import { AugmentedError, AugmentErrorService } from './augment-error-service.js';

/**
 * ErrorDisplay - A reusable component for displaying AugmentedError instances
 */
export class ErrorDisplay extends LitElement {
  static override styles = [
    foundationCSS,
    buttonCSS,
    css`
    :host {
      display: block;
      background: #fef2f2;
      border: 1px solid #fca5a5;
      border-radius: 6px;
      padding: 16px;
      margin: 8px 0;
      color: #991b1b;
    }

    .error-header {
      display: flex;
      align-items: flex-start;
      gap: 12px;
    }

    .error-icon {
      font-size: 20px;
      line-height: 1;
      flex-shrink: 0;
      margin-top: 2px;
    }

    .error-content {
      flex: 1;
      min-width: 0;
    }

    .error-message {
      font-weight: 500;
      line-height: 1.4;
      margin: 0;
      word-wrap: break-word;
    }

    .error-goal {
      display: block;
      font-weight: 600;
      margin-bottom: 4px;
    }

    .error-detail {
      display: block;
      font-weight: 400;
      margin-left: 8px;
    }

    .error-details {
      margin-top: 12px;
      overflow: hidden;
      transition: all 0.3s ease-in-out;
    }

    .error-details[aria-hidden="true"] {
      max-height: 0;
      margin-top: 0;
      opacity: 0;
    }

    .error-details[aria-hidden="false"] {
      max-height: 500px;
      opacity: 1;
    }

    .error-details-content {
      padding: 12px;
      background: #fee2e2;
      border: 1px solid #fca5a5;
      border-radius: 4px;
      font-size: 13px;
      line-height: 1.4;
      white-space: pre-wrap;
      word-wrap: break-word;
      overflow-wrap: break-word;
      font-family: ui-monospace, 'SFMono-Regular', 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
      color: black;
    }

    .expand-button {
      margin-top: 8px;
      background: none;
      border: none;
      color: #991b1b;
      font-size: 13px;
      cursor: pointer;
      padding: 4px 0;
      text-decoration: underline;
      display: flex;
      align-items: center;
      gap: 4px;
    }

    .expand-button:hover {
      color: #7f1d1d;
    }

    .expand-button:focus {
      outline: 2px solid #991b1b;
      outline-offset: 2px;
      border-radius: 2px;
    }

    .expand-icon {
      font-size: 12px;
      transition: transform 0.2s ease;
    }

    .expand-icon.expanded {
      transform: rotate(180deg);
    }

    @media (prefers-contrast: high) {
      :host {
        border-width: 2px;
      }
    }

    @media (prefers-reduced-motion: reduce) {
      .error-details,
      .expand-icon {
        transition: none;
      }
    }
  `];

  @property({ type: Object })
  augmentedError?: AugmentedError;

  @state()
  private expanded = false;

  constructor() {
    super();
  }

  private _handleExpandToggle(): void {
    this.expanded = !this.expanded;
  }

  private _handleKeydown(event: KeyboardEvent): void {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      this._handleExpandToggle();
    }
  }

  override render() {
    if (!this.augmentedError) {
      return html``;
    }

    const displayIcon = AugmentErrorService.getIconString(this.augmentedError.icon);
    const hasDetails = Boolean(this.augmentedError.stack && this.augmentedError.stack.trim());

    return html`
      <div class="error-header">
        <span class="error-icon" aria-hidden="true">${displayIcon}</span>
        <div class="error-content">
          <div class="error-message">
            ${this.augmentedError.failedGoalDescription
        ? html`
                <span class="error-goal">Error while ${this.augmentedError.failedGoalDescription}:</span>
                <span class="error-detail">${this.augmentedError.message}</span>
              `
        : html`${this.augmentedError.message}`}
          </div>
          
          ${hasDetails ? html`
            <button
              class="expand-button"
              @click="${this._handleExpandToggle}"
              @keydown="${this._handleKeydown}"
              aria-expanded="${this.expanded}"
              aria-controls="error-details"
            >
              ${this.expanded ? 'Hide' : 'Show'} details
              <span class="expand-icon ${this.expanded ? 'expanded' : ''}" aria-hidden="true">▼</span>
            </button>
            
            <div
              id="error-details"
              class="error-details"
              aria-hidden="${!this.expanded}"
            >
              <div class="error-details-content">${this.augmentedError.stack}</div>
            </div>
          ` : ''}
        </div>
      </div>
    `;
  }
}

customElements.define('error-display', ErrorDisplay);
