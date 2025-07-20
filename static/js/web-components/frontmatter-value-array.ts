import { html, css, LitElement } from 'lit';
import { buttonCSS, foundationCSS } from './shared-styles.js';
import './frontmatter-value-string.js';

export class FrontmatterValueArray extends LitElement {
  static override styles = [
    foundationCSS,
    buttonCSS,
    css`
      :host {
        display: block;
      }

      .array-container {
        border: 1px solid #e0e0e0;
        padding-left: 4px;
        padding-top: 4px;
        background: #f9f9f9;
        margin-left: 4px;
      }

      .array-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 4px;
        padding-bottom: 2px;
        border-bottom: 1px solid #e0e0e0;
      }

      .array-title {
        font-weight: normal;
        color: #888;
        font-size: 11px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
      }

    .array-items {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }

    .array-item {
      display: flex;
      align-items: center;
      gap: 4px;
      padding-left: 4px;
      padding-top: 3px;
      background: #fff;
      border: 1px solid #e0e0e0;
      border-radius: 2px;
    }

    .array-item frontmatter-value-string {
      flex: 1;
    }

    .empty-array-message {
      text-align: center;
      color: #666;
      font-style: italic;
      padding: 16px;
    }
  `
];

  static override properties = {
    values: { type: Array },
    disabled: { type: Boolean },
    placeholder: { type: String },
  };

  declare values: string[];
  declare disabled: boolean;
  declare placeholder: string;

  constructor() {
    super();
    this.values = [];
    this.disabled = false;
    this.placeholder = '';
  }

  private _handleAddItem = (): void => {
    const oldArray = [...this.values];
    const newArray = [...this.values, ''];
    
    this.values = newArray;
    this._dispatchArrayChange(oldArray, newArray);
    this.requestUpdate();
  };

  private _handleRemoveItem = (index: number): void => {
    const oldArray = [...this.values];
    const newArray = this.values.filter((_, i) => i !== index);
    
    this.values = newArray;
    this._dispatchArrayChange(oldArray, newArray);
    this.requestUpdate();
  };

  private _handleItemChange = (event: CustomEvent, index: number): void => {
    const oldArray = [...this.values];
    const newArray = [...this.values];
    newArray[index] = event.detail.newValue;
    
    this.values = newArray;
    this._dispatchArrayChange(oldArray, newArray);
  };

  private _dispatchArrayChange(oldArray: string[], newArray: string[]): void {
    this.dispatchEvent(new CustomEvent('array-change', {
      detail: {
        oldArray,
        newArray,
      },
      bubbles: true,
    }));
  }

  private renderArrayItems() {
    if (this.values.length === 0) {
      return html`
        <div class="empty-array-message">No items in array</div>
      `;
    }

    return html`
      <div class="array-items">
        ${this.values.map((value, index) => html`
          <div class="array-item">
            <frontmatter-value-string
              .value="${value}"
              .placeholder="${this.placeholder}"
              .disabled="${this.disabled}"
              @value-change="${(e: CustomEvent) => this._handleItemChange(e, index)}"
            ></frontmatter-value-string>
            <button
              class="button-base button-primary button-small border-radius-small remove-item-button"
              .disabled="${this.disabled}"
              @click="${() => this._handleRemoveItem(index)}"
            >
              Remove
            </button>
          </div>
        `)}
      </div>
    `;
  }

  override render() {
    return html`
      <div class="array-container">
        <div class="array-header">
          <span class="array-title">Array Items (${this.values.length})</span>
          <button
            class="button-base button-primary button-small border-radius-small add-item-button"
            .disabled="${this.disabled}"
            @click="${this._handleAddItem}"
          >
            Add Item
          </button>
        </div>
        ${this.renderArrayItems()}
      </div>
    `;
  }
}

customElements.define('frontmatter-value-array', FrontmatterValueArray);

declare global {
  interface HTMLElementTagNameMap {
    'frontmatter-value-array': FrontmatterValueArray;
  }
}