import { html, css, LitElement, nothing } from 'lit';
import { buttonCSS, foundationCSS } from './shared-styles.js';

const DEFAULT_DISARM_TIMEOUT_MS = 5000;

/**
 * Position for the confirmation popup relative to the trigger button.
 */
export type PopupPosition = 'auto' | 'left' | 'right';

/**
 * Timer provider interface for dependency injection.
 * Allows tests to control timer behavior deterministically.
 */
export interface TimerProvider {
  setTimeout(callback: () => void, delayMs: number): number;
  clearTimeout(id: number): void;
}

/**
 * Default timer provider using browser's native setTimeout/clearTimeout.
 */
export const defaultTimerProvider: TimerProvider = {
  setTimeout: (callback, delayMs) => window.setTimeout(callback, delayMs),
  clearTimeout: (id) => window.clearTimeout(id),
};

/**
 * ConfirmationInterlockButton - A reusable inline confirmation button
 *
 * This component transforms in place from a simple button to a confirmation prompt
 * with Yes/No options. Use it to confirm destructive or important actions inline
 * without a modal dialog.
 *
 * States:
 * - Normal: Shows `[label]` button
 * - Armed: Shows `[confirmLabel] [Yes] [No]`
 *
 * Events:
 * - `confirmed`: Dispatched when user clicks Yes
 * - `cancelled`: Dispatched when user clicks No
 *
 * Usage:
 * ```html
 * <confirmation-interlock-button
 *   label="Change"
 *   confirmLabel="Clear frontmatter?"
 *   yesLabel="Yes"
 *   noLabel="No"
 *   @confirmed=${this._handleConfirmed}
 * ></confirmation-interlock-button>
 * ```
 */
export class ConfirmationInterlockButton extends LitElement {
  static override styles = [
    foundationCSS,
    buttonCSS,
    css`
      :host {
        display: inline-block;
        position: relative;
      }

      .confirm-label {
        font-size: 13px;
        font-weight: 500;
        color: var(--color-text-secondary, #adb5bd);
        white-space: nowrap;
      }

      .button-yes {
        background: #28a745;
        color: white;
        border: 1px solid #28a745;
      }

      .button-yes:hover:not(:disabled) {
        background: #218838;
        border-color: #218838;
      }

      .button-no {
        background: #dc3545;
        color: white;
        border: 1px solid #dc3545;
      }

      .button-no:hover:not(:disabled) {
        background: #c82333;
        border-color: #c82333;
      }

      .confirm-buttons {
        display: inline-flex;
        gap: 4px;
      }

      .confirm-backdrop {
        position: fixed;
        inset: 0;
        z-index: 10000;
        /* Semi-transparent blur to indicate click-to-dismiss */
        background: rgba(0, 0, 0, 0.15);
        backdrop-filter: blur(1px);
        cursor: pointer;
        /* Ensure touch events work on mobile */
        touch-action: manipulation;
        -webkit-tap-highlight-color: transparent;
      }

      .confirm-popup {
        position: absolute;
        top: 100%;
        margin-top: 4px;
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 8px 12px;
        background: var(--color-background-secondary, #3d3d3d);
        border: 1px solid var(--color-border, #555);
        border-radius: 6px;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
        z-index: 10001;
        white-space: nowrap;
        animation: popupFadeIn 0.15s ease-out;
      }

      /* Position variants - applied via class */
      .confirm-popup.position-left {
        left: 0;
      }

      .confirm-popup.position-right {
        right: 0;
      }

      @keyframes popupFadeIn {
        from {
          opacity: 0;
          transform: translateY(-4px);
        }
        to {
          opacity: 1;
          transform: translateY(0);
        }
      }
    `,
  ];

  static override properties = {
    label: { type: String },
    confirmLabel: { type: String },
    yesLabel: { type: String },
    noLabel: { type: String },
    armed: { type: Boolean, reflect: true },
    disabled: { type: Boolean, reflect: true },
    disarmTimeoutMs: { type: Number },
    timerProvider: { type: Object },
    popupPosition: { type: String },
    _computedPosition: { state: true },
  };

  declare label: string;
  declare confirmLabel: string;
  declare yesLabel: string;
  declare noLabel: string;
  declare armed: boolean;
  declare disabled: boolean;
  declare disarmTimeoutMs: number;
  declare timerProvider: TimerProvider;
  declare popupPosition: PopupPosition;
  declare _computedPosition: 'left' | 'right';

  private _disarmTimerId: number | undefined;

  constructor() {
    super();
    this.label = 'Confirm';
    this.confirmLabel = 'Are you sure?';
    this.yesLabel = 'Yes';
    this.noLabel = 'No';
    this.armed = false;
    this.disabled = false;
    this.disarmTimeoutMs = DEFAULT_DISARM_TIMEOUT_MS;
    this.timerProvider = defaultTimerProvider;
    this.popupPosition = 'auto';
    this._computedPosition = 'left';
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this._clearDisarmTimer();
  }

  private _clearDisarmTimer(): void {
    if (this._disarmTimerId !== undefined) {
      this.timerProvider.clearTimeout(this._disarmTimerId);
      this._disarmTimerId = undefined;
    }
  }

  private _handleBackdropClick = (): void => {
    this.disarm();
  };

  /**
   * Finds the closest containing element that constrains overflow (dialog, modal, scrollable container).
   */
  private _findContainingBounds(): DOMRect {
    let element: Element | null = this.parentElement;

    while (element) {
      // Check for dialog elements
      if (element.tagName === 'DIALOG' ||
          element.getAttribute('role') === 'dialog' ||
          element.classList.contains('dialog') ||
          element.classList.contains('modal')) {
        return element.getBoundingClientRect();
      }

      // Check for elements with overflow that would clip content
      const style = getComputedStyle(element);
      if (style.overflow !== 'visible' ||
          style.overflowX !== 'visible' ||
          style.overflowY !== 'visible') {
        return element.getBoundingClientRect();
      }

      element = element.parentElement;
    }

    // Fall back to viewport
    return new DOMRect(0, 0, window.innerWidth, window.innerHeight);
  }

  /**
   * Estimates popup width based on actual content labels.
   * Uses approximate character width for the font.
   */
  private _estimatePopupWidth(): number {
    const charWidthPx = 7; // Approximate width per character at 13px font
    const buttonPaddingPx = 24; // Button horizontal padding (12px * 2)
    const buttonGapPx = 4; // Gap between buttons
    const popupPaddingPx = 24; // Popup horizontal padding (12px * 2)
    const labelGapPx = 8; // Gap between label and buttons
    const minWidthPx = 150;

    const labelWidth = this.confirmLabel.length * charWidthPx;
    const yesButtonWidth = this.yesLabel.length * charWidthPx + buttonPaddingPx;
    const noButtonWidth = this.noLabel.length * charWidthPx + buttonPaddingPx;
    const buttonsWidth = yesButtonWidth + noButtonWidth + buttonGapPx;

    const totalWidth = labelWidth + labelGapPx + buttonsWidth + popupPaddingPx;
    return Math.max(totalWidth, minWidthPx);
  }

  /**
   * Computes optimal popup position based on available space within containing bounds.
   */
  private _computePopupPosition(): 'left' | 'right' {
    if (this.popupPosition === 'left') return 'left';
    if (this.popupPosition === 'right') return 'right';

    // Auto mode: check available space within containing bounds
    const buttonRect = this.getBoundingClientRect();
    const containerBounds = this._findContainingBounds();
    const estimatedPopupWidth = this._estimatePopupWidth();

    // Calculate space on each side relative to the container
    const spaceOnRight = containerBounds.right - buttonRect.left;
    const spaceOnLeft = buttonRect.right - containerBounds.left;

    // Prefer left alignment, but switch to right if not enough space
    if (spaceOnRight >= estimatedPopupWidth) {
      return 'left';
    } else if (spaceOnLeft >= estimatedPopupWidth) {
      return 'right';
    }

    // Default to left if neither has enough space
    return 'left';
  }

  /**
   * Arms the button, showing the confirmation prompt.
   * Starts auto-disarm timer if disarmTimeoutMs > 0.
   */
  public arm(): void {
    if (this.disabled) return;
    this._computedPosition = this._computePopupPosition();
    this.armed = true;
    this._startDisarmTimer();
  }

  /**
   * Disarms the button, returning to normal state.
   */
  public disarm(): void {
    this._clearDisarmTimer();
    this.armed = false;
  }

  private _startDisarmTimer(): void {
    this._clearDisarmTimer();
    if (this.disarmTimeoutMs > 0) {
      this._disarmTimerId = this.timerProvider.setTimeout(() => {
        this.disarm();
      }, this.disarmTimeoutMs);
    }
  }

  private _handleTriggerClick = (): void => {
    this.arm();
  };

  private _handleYesClick = (): void => {
    this._clearDisarmTimer();
    this.armed = false;
    this.dispatchEvent(new CustomEvent('confirmed', { bubbles: true, composed: true }));
  };

  private _handleNoClick = (): void => {
    this.disarm();
    this.dispatchEvent(new CustomEvent('cancelled', { bubbles: true, composed: true }));
  };

  private _renderTriggerButton() {
    return html`
      <button
        class="button-base button-secondary button-small border-radius-small"
        @click=${this._handleTriggerClick}
        ?disabled=${this.disabled}
      >
        ${this.label}
      </button>
    `;
  }

  private _renderConfirmPopup() {
    const positionClass = `position-${this._computedPosition}`;
    return html`
      <div class="confirm-backdrop" @pointerdown=${this._handleBackdropClick}></div>
      <div class="confirm-popup ${positionClass}">
        <span class="confirm-label">${this.confirmLabel}</span>
        <div class="confirm-buttons">
          <button
            class="button-base button-yes button-small border-radius-small"
            @click=${this._handleYesClick}
            ?disabled=${this.disabled}
          >
            ${this.yesLabel}
          </button>
          <button
            class="button-base button-no button-small border-radius-small"
            @click=${this._handleNoClick}
            ?disabled=${this.disabled}
          >
            ${this.noLabel}
          </button>
        </div>
      </div>
    `;
  }

  override render() {
    return html`
      ${this._renderTriggerButton()}
      ${this.armed ? this._renderConfirmPopup() : nothing}
    `;
  }
}

customElements.define('confirmation-interlock-button', ConfirmationInterlockButton);

declare global {
  interface HTMLElementTagNameMap {
    'confirmation-interlock-button': ConfirmationInterlockButton;
  }
}
