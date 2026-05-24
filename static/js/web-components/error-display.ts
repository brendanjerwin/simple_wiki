import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { colorCSS, typographyCSS, themeCSS, foundationCSS, buttonCSS } from './shared-styles.js';
import type { AugmentedError } from './augment-error-service.js';
import { AugmentErrorService } from './augment-error-service.js';

/**
 * Action button configuration for error recovery
 */
export interface ErrorAction {
  /** Button label text */
  label: string;
  /** Callback function when button is clicked */
  onClick: () => void;
}

/**
 * ErrorDisplay - A reusable component for displaying AugmentedError instances
 *
 * Supports optional CTA button for error recovery actions.
 */
export class ErrorDisplay extends LitElement {
  static override readonly styles = [
    colorCSS,
    typographyCSS,
    themeCSS,
    foundationCSS,
    buttonCSS,
    css`
    :host {
      display: block;
      margin: 8px 0;
      border: 1px solid var(--color-error);
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
      padding: 8px;
      background: rgba(220, 53, 69, 0.1);
      border: 1px solid rgba(220, 53, 69, 0.3);
      border-radius: 4px;
      white-space: pre-wrap;
      word-wrap: break-word;
      overflow-wrap: break-word;
    }

    .expand-button {
      margin-top: 6px;
      background: none;
      border: none;
      cursor: pointer;
      padding: 2px 0;
      text-decoration: underline;
      display: flex;
      align-items: center;
      gap: 4px;
      transition: color 0.2s ease;
    }

    .expand-button:hover {
      color: var(--color-hover-error);
    }

    .expand-button:focus {
      outline: 2px solid var(--color-error);
      outline-offset: 1px;
      border-radius: 2px;
    }

    .error-tools {
      margin-top: 6px;
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 12px;
    }

    .copy-details-button {
      background: none;
      border: none;
      cursor: pointer;
      padding: 2px 0;
      text-decoration: underline;
      transition: color 0.2s ease;
    }

    .copy-details-button:hover {
      color: var(--color-hover-error);
    }

    .copy-details-button:focus {
      outline: 2px solid var(--color-error);
      outline-offset: 1px;
      border-radius: 2px;
    }

    .expand-icon {
      font-size: 10px;
      transition: transform 0.2s ease;
    }

    .expand-icon.expanded {
      transform: rotate(180deg);
    }

    .error-actions {
      margin-top: 12px;
      display: flex;
      gap: 8px;
    }

    .copy-details-fallback {
      width: 100%;
      min-height: 120px;
      margin-top: 12px;
      padding: 8px;
      border: 1px solid rgba(220, 53, 69, 0.3);
      border-radius: 4px;
      background: rgba(220, 53, 69, 0.1);
      resize: vertical;
      white-space: pre;
    }

    .action-button {
      background: var(--color-error);
      color: white;
      border: none;
      padding: 8px 16px;
      border-radius: 4px;
      font-size: 13px;
      font-weight: 500;
      cursor: pointer;
      transition: background-color 0.2s ease;
    }

    .action-button:hover {
      background: var(--color-hover-error);
    }

    .action-button:focus {
      outline: 2px solid var(--color-error);
      outline-offset: 2px;
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
  declare augmentedError?: AugmentedError;

  @property({ type: Object })
  declare action?: ErrorAction;

  @state()
  declare private expanded: boolean;

  @state()
  declare private copied: boolean;

  @state()
  declare private fallbackCopyDetails: string | undefined;

  private copyConfirmationTimer: ReturnType<typeof setTimeout> | undefined;

  constructor() {
    super();
    this.expanded = false;
    this.copied = false;
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this.clearCopyConfirmationTimer();
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

  private _handleActionClick(): void {
    if (this.action?.onClick) {
      this.action.onClick();
    }
  }

  private async _handleCopyDetailsClick(): Promise<void> {
    const details = this.augmentedError?.copyableDetails;
    if (!details) {
      return;
    }

    const clipboard = navigator.clipboard;
    if (clipboard && 'writeText' in clipboard) {
      try {
        await clipboard.writeText(details);
        this.showCopyConfirmation();
        return;
      } catch {
        this.showFallbackCopyDetails(details);
        return;
      }
    }

    this.showFallbackCopyDetails(details);
  }

  private showCopyConfirmation(): void {
    this.fallbackCopyDetails = undefined;
    this.copied = true;
    this.clearCopyConfirmationTimer();
    this.copyConfirmationTimer = setTimeout(() => {
      this.copied = false;
    }, 1500);
  }

  private clearCopyConfirmationTimer(): void {
    if (this.copyConfirmationTimer) {
      clearTimeout(this.copyConfirmationTimer);
      this.copyConfirmationTimer = undefined;
    }
  }

  private showFallbackCopyDetails(details: string): void {
    this.fallbackCopyDetails = details;
    void this.updateComplete.then(() => {
      const textarea = this.shadowRoot?.querySelector<HTMLTextAreaElement>('.copy-details-fallback');
      textarea?.focus();
      textarea?.select();
    });
  }

  private _renderDetails(hasDetails: boolean) {
    if (!hasDetails) {
      return this._renderCopyDetailsButton();
    }

    const expandLabel = this.expanded ? 'Hide' : 'Show';
    const expandIconClass = this.expanded ? 'expand-icon expanded' : 'expand-icon';

    return html`
      <div class="error-tools">
        <button
          class="expand-button text-error font-mono text-xs"
          @click="${this._handleExpandToggle}"
          @keydown="${this._handleKeydown}"
          aria-expanded="${this.expanded}"
          aria-controls="error-details"
        >
          ${expandLabel} details
          <span class="${expandIconClass}" aria-hidden="true">▼</span>
        </button>

        ${this._renderCopyDetailsButton()}
      </div>

      <div
        id="error-details"
        class="error-details"
        aria-hidden="${!this.expanded}"
      >
        <div class="error-details-content text-muted font-mono text-xs">${this.augmentedError?.stack}</div>
      </div>
    `;
  }

  private _renderCopyDetailsButton() {
    return html`
      <button
        type="button"
        class="copy-details-button text-error font-mono text-xs"
        @click="${this._handleCopyDetailsClick}"
      >
        ${this.copied ? 'Copied!' : 'Copy details'}
      </button>
    `;
  }

  private _renderFallbackCopyDetails() {
    if (!this.fallbackCopyDetails) {
      return nothing;
    }

    return html`
      <textarea
        class="copy-details-fallback text-muted font-mono text-xs"
        readonly
        .value="${this.fallbackCopyDetails}"
      ></textarea>
    `;
  }

  private _renderAction() {
    if (!this.action) {
      return nothing;
    }

    return html`
      <div class="error-actions">
        <button
          type="button"
          class="action-button"
          @click="${this._handleActionClick}"
        >
          ${this.action.label}
        </button>
      </div>
    `;
  }

  override render() {
    if (!this.augmentedError) {
      return html``;
    }

    const displayIcon = AugmentErrorService.getIconString(this.augmentedError.icon);
    const hasDetails = Boolean(this.augmentedError.stack?.trim());

    return html`
      <div class="container container-embedded panel gap-sm">
        <div class="error-header">
          <span class="error-icon text-error" aria-hidden="true">${displayIcon}</span>
          <div class="error-content">
            <div class="error-message text-primary font-mono text-sm">
              ${this.augmentedError.failedGoalDescription
          ? html`
                  <span class="error-goal text-warning">Error while ${this.augmentedError.failedGoalDescription}:</span>
                  <span class="error-detail text-muted">${this.augmentedError.message}</span>
                `
          : html`${this.augmentedError.message}`}
            </div>
            
            ${this._renderDetails(hasDetails)}

            ${this._renderAction()}

            ${this._renderFallbackCopyDetails()}
          </div>
        </div>
      </div>
    `;
  }
}

customElements.define('error-display', ErrorDisplay);

declare global {
  interface HTMLElementTagNameMap {
    'error-display': ErrorDisplay;
  }
}
