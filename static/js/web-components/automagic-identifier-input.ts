import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { sharedStyles, dialogCSS } from './shared-styles.js';
import type { ExistingPageInfo } from '../gen/api/v1/page_management_pb.js';
import type { AugmentedError } from './augment-error-service.js';
import { AugmentErrorService } from './augment-error-service.js';
import type { ErrorAction } from './error-display.js';
import './error-display.js';

/**
 * Result from the identifier generator function.
 */
export interface GenerateIdentifierResult {
  identifier: string;
  isUnique: boolean;
  existingPage?: ExistingPageInfo;
  error?: Error;
}

/**
 * Function signature for identifier generation.
 * Should call the GenerateIdentifier RPC and return the result.
 */
export type IdentifierGenerator = (text: string) => Promise<GenerateIdentifierResult>;

/**
 * AutomagicIdentifierInput - Reusable component for title/identifier input with automagic mode.
 *
 * Title-first workflow: user enters a title, identifier is auto-generated
 * via a generator function (automagic mode). User can click the sparkle button to
 * switch to manual mode for editing the identifier directly.
 *
 * @fires title-change - When the title changes. Detail: { title: string }
 * @fires identifier-change - When the identifier changes. Detail: { identifier: string, isUnique: boolean, existingPage?: ExistingPageInfo }
 */
export class AutomagicIdentifierInput extends LitElement {
  static override styles = [
    dialogCSS,
    css`
      :host {
        display: block;
      }

      .identifier-field {
        display: flex;
        gap: 8px;
        align-items: center;
      }

      .identifier-field input {
        flex: 1;
      }

      .automagic-button {
        padding: 10px 12px;
        border: 1px solid #ddd;
        border-radius: 4px;
        background: #f5f5f5;
        cursor: pointer;
        font-size: 14px;
        color: #666;
        transition: all 0.2s;
      }

      .automagic-button:hover:not(:disabled) {
        background: #e8e8e8;
        border-color: #ccc;
      }

      .automagic-button:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }

      .automagic-button.automagic {
        background: #e0f2fe;
        border-color: #7dd3fc;
        color: #0369a1;
      }

      .automagic-button.manual {
        background: #fff3cd;
        border-color: #ffc107;
        color: #856404;
      }

      .conflict-warning {
        background: #fffbeb;
        border: 1px solid #fcd34d;
        color: #92400e;
        padding: 12px;
        border-radius: 4px;
        margin-top: 8px;
        font-size: 13px;
      }

      .conflict-warning a {
        color: #92400e;
        font-weight: 500;
      }
    `,
  ];

  /**
   * The current title value.
   */
  @property({ type: String })
  declare title: string;

  /**
   * The current identifier value.
   */
  @property({ type: String })
  declare identifier: string;

  /**
   * Whether automagic mode is enabled (identifier auto-generated from title).
   */
  @property({ type: Boolean })
  declare automagicMode: boolean;

  /**
   * Whether the current identifier is unique (no existing page with this identifier).
   */
  @property({ type: Boolean })
  declare isUnique: boolean;

  /**
   * Info about the existing page if identifier is not unique.
   */
  @property({ type: Object })
  declare existingPage?: ExistingPageInfo;

  /**
   * Whether the inputs are disabled.
   */
  @property({ type: Boolean })
  declare disabled: boolean;

  /**
   * Placeholder text for the title input.
   */
  @property({ type: String })
  declare titlePlaceholder: string;

  /**
   * Help text displayed below the title input.
   */
  @property({ type: String })
  declare titleHelpText: string;

  /**
   * The identifier generator function to call for generating/validating identifiers.
   */
  @property({ attribute: false })
  declare generateIdentifier: IdentifierGenerator;

  /**
   * Error from automagic identifier generation.
   */
  @state()
  declare automagicError: AugmentedError | null;

  private _debounceTimeoutMs = 300;
  private _titleDebounceTimer?: ReturnType<typeof setTimeout>;
  private _identifierDebounceTimer?: ReturnType<typeof setTimeout>;

  constructor() {
    super();
    this.title = '';
    this.identifier = '';
    this.automagicMode = true;
    this.isUnique = true;
    this.disabled = false;
    this.titlePlaceholder = 'Enter a title';
    this.titleHelpText = '';
    this.automagicError = null;
    this.generateIdentifier = async () => ({ identifier: '', isUnique: true });
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this._clearDebounceTimers();
  }

  private _clearDebounceTimers(): void {
    if (this._titleDebounceTimer) {
      clearTimeout(this._titleDebounceTimer);
      delete this._titleDebounceTimer;
    }
    if (this._identifierDebounceTimer) {
      clearTimeout(this._identifierDebounceTimer);
      delete this._identifierDebounceTimer;
    }
  }

  /**
   * Focus the title input field.
   */
  public focusTitleInput(): void {
    const titleInput = this.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
    titleInput?.focus();
  }

  /**
   * Reset the component to its initial state.
   */
  public reset(): void {
    this._clearDebounceTimers();
    this.title = '';
    this.identifier = '';
    this.automagicMode = true;
    this.isUnique = true;
    delete this.existingPage;
    this.automagicError = null;
  }

  private _handleTitleInput = (event: Event): void => {
    if (!(event.target instanceof HTMLInputElement)) {
      return;
    }
    const input = event.target;
    this.title = input.value;

    // Dispatch title-change event immediately
    this.dispatchEvent(
      new CustomEvent('title-change', {
        detail: { title: this.title },
        bubbles: true,
        composed: true,
      })
    );

    // Clear existing timer
    if (this._titleDebounceTimer) {
      clearTimeout(this._titleDebounceTimer);
    }

    // Debounce the API call
    this._titleDebounceTimer = setTimeout(() => {
      this._onTitleChanged();
    }, this._debounceTimeoutMs);
  };

  private async _onTitleChanged(): Promise<void> {
    const title = this.title.trim();

    if (!title) {
      this.identifier = '';
      this.isUnique = true;
      delete this.existingPage;
      this._dispatchIdentifierChange();
      return;
    }

    // Generate identifier if in automagic mode
    if (this.automagicMode) {
      const result = await this.generateIdentifier(title);
      if (result.error) {
        this.automagicError = AugmentErrorService.augmentError(
          result.error,
          'generating identifier'
        );
      } else {
        this.automagicError = null;
        this.identifier = result.identifier;
        this.isUnique = result.isUnique;
        if (result.existingPage) {
          this.existingPage = result.existingPage;
        } else {
          delete this.existingPage;
        }
        this._dispatchIdentifierChange();
      }
    }
  }

  private _handleIdentifierInput = (event: Event): void => {
    // Only allow editing in manual mode (not automagic)
    if (this.automagicMode) return;

    if (!(event.target instanceof HTMLInputElement)) {
      return;
    }
    const input = event.target;
    this.identifier = input.value;

    // Clear existing timer
    if (this._identifierDebounceTimer) {
      clearTimeout(this._identifierDebounceTimer);
    }

    // Debounce the API call to check availability
    this._identifierDebounceTimer = setTimeout(() => {
      this._checkIdentifierAvailability();
    }, this._debounceTimeoutMs);
  };

  private async _checkIdentifierAvailability(): Promise<void> {
    const identifier = this.identifier.trim();

    if (!identifier) {
      this.isUnique = true;
      delete this.existingPage;
      this._dispatchIdentifierChange();
      return;
    }

    // Call generateIdentifier just to check availability
    const result = await this.generateIdentifier(identifier);
    if (!result.error) {
      this.isUnique = result.isUnique;
      if (result.existingPage) {
        this.existingPage = result.existingPage;
      } else {
        delete this.existingPage;
      }
      this._dispatchIdentifierChange();
    }
  }

  private _dispatchIdentifierChange(): void {
    this.dispatchEvent(
      new CustomEvent('identifier-change', {
        detail: {
          identifier: this.identifier,
          isUnique: this.isUnique,
          existingPage: this.existingPage,
        },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleAutomagicToggle = (): void => {
    this.automagicMode = !this.automagicMode;

    // If switching back to automagic, regenerate identifier from title
    if (this.automagicMode && this.title.trim()) {
      this._onTitleChanged();
    }
  };

  private _handleSwitchToManual = (): void => {
    this.automagicMode = false;
    this.automagicError = null;
    this.identifier = '';
  };

  private _renderAutomagicError() {
    if (!this.automagicError || !this.automagicMode) {
      return nothing;
    }

    const action: ErrorAction = {
      label: 'Switch to Manual',
      onClick: this._handleSwitchToManual,
    };

    return html`
      <error-display .augmentedError=${this.automagicError} .action=${action}></error-display>
    `;
  }

  private _renderConflictWarning() {
    if (this.isUnique || !this.existingPage) {
      return nothing;
    }

    return html`
      <div class="conflict-warning">
        <strong>Identifier already exists:</strong>
        <a href="/${this.existingPage.identifier}"
          >${this.existingPage.title || this.existingPage.identifier}</a
        >
        ${this.existingPage.container
          ? html` (Found In:
              <a href="/${this.existingPage.container}">${this.existingPage.container}</a>)`
          : ''}
      </div>
    `;
  }

  override render() {
    return html`
      ${sharedStyles}
      <div class="form-group">
        <label for="title">Title</label>
        <input
          type="text"
          id="title"
          name="title"
          .value=${this.title}
          @input=${this._handleTitleInput}
          placeholder=${this.titlePlaceholder}
          ?disabled=${this.disabled}
        />
        ${this.titleHelpText ? html`<div class="help-text">${this.titleHelpText}</div>` : nothing}
      </div>

      <div class="form-group">
        <label for="identifier">Identifier *</label>
        <div class="identifier-field">
          <input
            type="text"
            id="identifier"
            name="identifier"
            .value=${this.identifier}
            @input=${this._handleIdentifierInput}
            placeholder=${this.automagicMode ? 'Auto-generated from title' : 'Enter identifier manually'}
            ?disabled=${this.disabled}
            ?readonly=${this.automagicMode}
            tabindex=${this.automagicMode ? '-1' : '0'}
          />
          <button
            type="button"
            class="automagic-button ${this.automagicMode ? 'automagic' : 'manual'}"
            @click=${this._handleAutomagicToggle}
            title=${this.automagicMode
              ? 'Click to edit identifier manually'
              : 'Click to auto-generate from title'}
            ?disabled=${this.disabled}
          >
            <i class="fa-solid ${this.automagicMode ? 'fa-wand-magic-sparkles' : 'fa-pen'}"></i>
          </button>
        </div>
        ${this._renderAutomagicError()} ${this._renderConflictWarning()}
      </div>
    `;
  }
}

customElements.define('automagic-identifier-input', AutomagicIdentifierInput);

declare global {
  interface HTMLElementTagNameMap {
    'automagic-identifier-input': AutomagicIdentifierInput;
  }
}
