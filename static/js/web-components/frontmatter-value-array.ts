import { html, css, LitElement } from 'lit';
import './frontmatter-value-string.js';

export class FrontmatterValueArray extends LitElement {
  static override styles = css`
    :host {
      display: block;
    }

    .array-container {
      border: 1px solid #e0e0e0;
      border-radius: 4px;
      padding: 12px;
      background: #f9f9f9;
    }

    .array-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 12px;
      padding-bottom: 8px;
      border-bottom: 1px solid #e0e0e0;
    }

    .array-title {
      font-weight: 600;
      color: #333;
      font-size: 14px;
    }

    .add-item-button {
      padding: 4px 8px;
      font-size: 12px;
      border: 1px solid #6c757d;
      border-radius: 2px;
      cursor: pointer;
      transition: all 0.2s;
      background: #6c757d;
      color: white;
    }

    .add-item-button:hover:not(:disabled) {
      background: #5a6268;
      border-color: #5a6268;
    }

    .add-item-button:disabled {
      background: #6c757d;
      border-color: #6c757d;
      cursor: not-allowed;
    }

    .array-items {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    .array-item {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px;
      background: #fff;
      border: 1px solid #e0e0e0;
      border-radius: 2px;
    }

    .array-item frontmatter-value-string {
      flex: 1;
    }

    .remove-item-button {
      padding: 4px 8px;
      font-size: 12px;
      border: 1px solid #6c757d;
      border-radius: 2px;
      cursor: pointer;
      transition: all 0.2s;
      background: #6c757d;
      color: white;
    }

    .remove-item-button:hover:not(:disabled) {
      background: #5a6268;
      border-color: #5a6268;
    }

    .remove-item-button:disabled {
      background: #6c757d;
      border-color: #6c757d;
      cursor: not-allowed;
    }

    .empty-array-message {
      text-align: center;
      color: #666;
      font-style: italic;
      padding: 16px;
    }
  `;

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
    
    // Debug logging for data structure changes
    console.log('[FrontmatterValueArray] Add item:', {
      oldArray,
      newArray,
      newItemIndex: newArray.length - 1
    });
    
    this._dispatchArrayChange(oldArray, newArray);
    this.requestUpdate();
  };

  private _handleRemoveItem = (index: number): void => {
    const oldArray = [...this.values];
    const newArray = this.values.filter((_, i) => i !== index);
    
    this.values = newArray;
    
    // Debug logging for data structure changes
    console.log('[FrontmatterValueArray] Remove item:', {
      removedIndex: index,
      removedValue: oldArray[index],
      oldArray,
      newArray
    });
    
    this._dispatchArrayChange(oldArray, newArray);
    this.requestUpdate();
  };

  private _handleItemChange = (event: CustomEvent, index: number): void => {
    const oldArray = [...this.values];
    const newArray = [...this.values];
    newArray[index] = event.detail.newValue;
    
    this.values = newArray;
    
    // Debug logging for data structure changes
    console.log('[FrontmatterValueArray] Item changed:', {
      index,
      oldValue: oldArray[index],
      newValue: event.detail.newValue,
      oldArray,
      newArray
    });
    
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
              class="remove-item-button"
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
            class="add-item-button"
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