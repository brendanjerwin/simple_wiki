import { html, css, LitElement } from 'lit';
import { foundationCSS, dialogCSS, responsiveCSS, buttonCSS, inputCSS } from './shared-styles.js';
import { inventoryActionService } from './inventory-action-service.js';

/**
 * InventoryMoveItemDialog - Modal dialog for moving inventory items between containers
 *
 * Shows the current item and container, allows user to specify
 * a destination container for the move operation.
 */
export class InventoryMoveItemDialog extends LitElement {
  static override styles = [
    foundationCSS,
    dialogCSS,
    responsiveCSS,
    buttonCSS,
    inputCSS,
    css`
      :host {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        z-index: 9999;
        display: none;
      }

      :host([open]) {
        display: flex;
        align-items: center;
        justify-content: center;
        animation: fadeIn 0.2s ease-out;
      }

      @keyframes fadeIn {
        from { opacity: 0; }
        to { opacity: 1; }
      }

      .backdrop {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(0, 0, 0, 0.5);
      }

      .dialog {
        background: white;
        max-width: 500px;
        width: 90%;
        display: flex;
        flex-direction: column;
        position: relative;
        z-index: 1;
        animation: slideIn 0.2s ease-out;
        border-radius: 8px;
        box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
      }

      @keyframes slideIn {
        from {
          transform: translateY(-20px);
          opacity: 0;
        }
        to {
          transform: translateY(0);
          opacity: 1;
        }
      }

      @media (max-width: 768px) {
        :host([open]) {
          align-items: stretch;
          justify-content: stretch;
        }

        .dialog {
          width: 100%;
          height: 100%;
          max-width: none;
          border-radius: 0;
          margin: 0;
        }
      }

      .header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 16px 20px;
        border-bottom: 1px solid #e0e0e0;
      }

      .header h2 {
        margin: 0;
        font-size: 18px;
        font-weight: 600;
      }

      .content {
        padding: 20px;
      }

      .form-group {
        margin-bottom: 16px;
      }

      .form-group:last-child {
        margin-bottom: 0;
      }

      .form-group label {
        display: block;
        margin-bottom: 6px;
        font-weight: 500;
        color: #333;
      }

      .form-group input {
        width: 100%;
        padding: 10px 12px;
        border: 1px solid #ddd;
        border-radius: 4px;
        font-size: 14px;
        box-sizing: border-box;
      }

      .form-group input:focus {
        outline: none;
        border-color: #4a90d9;
        box-shadow: 0 0 0 2px rgba(74, 144, 217, 0.2);
      }

      .form-group input[readonly] {
        background: #f5f5f5;
        color: #666;
        cursor: not-allowed;
      }

      .form-group .help-text {
        margin-top: 4px;
        font-size: 12px;
        color: #666;
      }

      .move-arrow {
        display: flex;
        justify-content: center;
        align-items: center;
        padding: 8px 0;
        color: #666;
        font-size: 24px;
      }

      .error-message {
        background: #fef2f2;
        border: 1px solid #fecaca;
        color: #dc2626;
        padding: 12px;
        border-radius: 4px;
        margin-bottom: 16px;
        font-size: 14px;
      }

      .footer {
        display: flex;
        gap: 12px;
        padding: 16px 20px;
        border-top: 1px solid #e0e0e0;
        justify-content: flex-end;
      }
    `,
  ];

  static override properties = {
    open: { type: Boolean, reflect: true },
    itemIdentifier: { type: String },
    currentContainer: { type: String },
    newContainer: { type: String },
    loading: { state: true },
    error: { state: true },
  };

  declare open: boolean;
  declare itemIdentifier: string;
  declare currentContainer: string;
  declare newContainer: string;
  declare loading: boolean;
  declare error?: string;

  constructor() {
    super();
    this.open = false;
    this.itemIdentifier = '';
    this.currentContainer = '';
    this.newContainer = '';
    this.loading = false;
    this.error = undefined;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('keydown', this._handleKeydown);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('keydown', this._handleKeydown);
  }

  public _handleKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape' && this.open) {
      this.close();
    }
  };

  public openDialog(itemIdentifier: string, currentContainer: string): void {
    this.itemIdentifier = itemIdentifier;
    this.currentContainer = currentContainer;
    this.newContainer = '';
    this.error = undefined;
    this.loading = false;
    this.open = true;
  }

  public close(): void {
    this.open = false;
    this.newContainer = '';
    this.error = undefined;
    this.loading = false;
  }

  private _handleBackdropClick = (): void => {
    this.close();
  };

  private _handleDialogClick = (event: Event): void => {
    event.stopPropagation();
  };

  private _handleNewContainerInput = (event: Event): void => {
    const input = event.target as HTMLInputElement;
    this.newContainer = input.value;
  };

  private _handleCancel = (): void => {
    this.close();
  };

  private get canSubmit(): boolean {
    const hasNewContainer = this.newContainer.trim().length > 0;
    const isDifferent = this.newContainer.trim() !== this.currentContainer;
    return hasNewContainer && isDifferent && !this.loading;
  }

  private _handleSubmit = async (): Promise<void> => {
    if (!this.canSubmit) return;

    this.loading = true;
    this.error = undefined;

    const result = await inventoryActionService.moveItem(
      this.itemIdentifier,
      this.newContainer.trim()
    );

    this.loading = false;

    if (result.success) {
      inventoryActionService.showSuccess(
        result.summary || `Moved ${this.itemIdentifier} to ${this.newContainer}`,
        () => window.location.reload()
      );
      this.close();
    } else {
      this.error = result.error;
    }
  };

  override render() {
    return html`
      <div class="backdrop" @click=${this._handleBackdropClick}>
        <div class="dialog" @click=${this._handleDialogClick}>
          <div class="header">
            <h2>Move Item</h2>
          </div>

          <div class="content">
            ${this.error
              ? html`<div class="error-message">${this.error}</div>`
              : ''}

            <div class="form-group">
              <label for="itemIdentifier">Item</label>
              <input
                type="text"
                id="itemIdentifier"
                name="itemIdentifier"
                .value=${this.itemIdentifier}
                readonly
              />
              <div class="help-text">The item being moved</div>
            </div>

            <div class="form-group">
              <label for="currentContainer">Current Location</label>
              <input
                type="text"
                id="currentContainer"
                name="currentContainer"
                .value=${this.currentContainer}
                readonly
              />
              <div class="help-text">Where the item is currently stored</div>
            </div>

            <div class="move-arrow">↓</div>

            <div class="form-group">
              <label for="newContainer">New Location *</label>
              <input
                type="text"
                id="newContainer"
                name="newContainer"
                .value=${this.newContainer}
                @input=${this._handleNewContainerInput}
                placeholder="e.g., toolbox_garage"
                ?disabled=${this.loading}
              />
              <div class="help-text">Container identifier to move the item to (required)</div>
            </div>
          </div>

          <div class="footer">
            <button
              class="secondary"
              @click=${this._handleCancel}
              ?disabled=${this.loading}
            >
              Cancel
            </button>
            <button
              class="primary"
              @click=${this._handleSubmit}
              ?disabled=${!this.canSubmit}
            >
              ${this.loading ? 'Moving...' : 'Move Item'}
            </button>
          </div>
        </div>
      </div>
    `;
  }
}

customElements.define('inventory-move-item-dialog', InventoryMoveItemDialog);
