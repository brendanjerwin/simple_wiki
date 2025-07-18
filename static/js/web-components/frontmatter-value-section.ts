import { html, css, LitElement } from 'lit';
import './frontmatter-key.js';
import './frontmatter-value-string.js';

export class FrontmatterValueSection extends LitElement {
  static override styles = css`
    :host {
      display: block;
    }

    .section-container {
      border: 1px solid #e0e0e0;
      border-radius: 4px;
      padding: 16px;
      background: #f9f9f9;
    }

    .section-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 12px;
      padding-bottom: 8px;
      border-bottom: 1px solid #e0e0e0;
    }

    .section-title {
      font-weight: 600;
      color: #333;
      font-size: 14px;
    }

    .add-field-button {
      padding: 4px 8px;
      font-size: 12px;
      border: 1px solid #28a745;
      border-radius: 2px;
      cursor: pointer;
      transition: all 0.2s;
      background: #28a745;
      color: white;
    }

    .add-field-button:hover:not(:disabled) {
      background: #218838;
      border-color: #218838;
    }

    .add-field-button:disabled {
      background: #6c757d;
      border-color: #6c757d;
      cursor: not-allowed;
    }

    .section-fields {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    .field-row {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px;
      background: #fff;
      border: 1px solid #e0e0e0;
      border-radius: 2px;
    }

    .field-row frontmatter-key {
      flex: 0 0 150px;
    }

    .field-row frontmatter-value-string {
      flex: 1;
    }

    .remove-field-button {
      padding: 4px 8px;
      font-size: 12px;
      border: 1px solid #dc3545;
      border-radius: 2px;
      cursor: pointer;
      transition: all 0.2s;
      background: #dc3545;
      color: white;
    }

    .remove-field-button:hover:not(:disabled) {
      background: #c82333;
      border-color: #c82333;
    }

    .remove-field-button:disabled {
      background: #6c757d;
      border-color: #6c757d;
      cursor: not-allowed;
    }

    .empty-section-message {
      text-align: center;
      color: #666;
      font-style: italic;
      padding: 16px;
    }
  `;

  static override properties = {
    fields: { type: Object },
    disabled: { type: Boolean },
  };

  declare fields: Record<string, string>;
  declare disabled: boolean;

  constructor() {
    super();
    this.fields = {};
    this.disabled = false;
  }

  private _generateUniqueKey(baseKey: string): string {
    let counter = 1;
    let newKey = baseKey;
    
    while (this.fields[newKey] !== undefined) {
      newKey = `${baseKey}_${counter}`;
      counter++;
    }
    
    return newKey;
  }

  private _handleAddField = (): void => {
    const oldFields = { ...this.fields };
    const newKey = this._generateUniqueKey('new_field');
    const newFields = { ...this.fields, [newKey]: '' };
    
    this.fields = newFields;
    
    this._dispatchSectionChange(oldFields, newFields);
    this.requestUpdate();
  };

  private _handleRemoveField = (key: string): void => {
    const oldFields = { ...this.fields };
    const newFields = { ...this.fields };
    delete newFields[key];
    
    this.fields = newFields;
    
    this._dispatchSectionChange(oldFields, newFields);
    this.requestUpdate();
  };

  private _handleKeyChange = (event: CustomEvent): void => {
    const { oldKey, newKey } = event.detail;
    
    if (oldKey === newKey || !newKey.trim()) return;
    
    const oldFields = { ...this.fields };
    const newFields = { ...this.fields };
    
    // Move the value from old key to new key
    newFields[newKey] = newFields[oldKey];
    delete newFields[oldKey];
    
    this.fields = newFields;
    
    this._dispatchSectionChange(oldFields, newFields);
    this.requestUpdate();
  };

  private _handleValueChange = (event: CustomEvent, key: string): void => {
    const { newValue } = event.detail;
    
    const oldFields = { ...this.fields };
    const newFields = { ...this.fields, [key]: newValue };
    
    this.fields = newFields;
    
    this._dispatchSectionChange(oldFields, newFields);
  };

  private _dispatchSectionChange(oldFields: Record<string, string>, newFields: Record<string, string>): void {
    this.dispatchEvent(new CustomEvent('section-change', {
      detail: {
        oldFields,
        newFields,
      },
      bubbles: true,
    }));
  }

  private renderSectionFields() {
    const fieldEntries = Object.entries(this.fields);
    
    if (fieldEntries.length === 0) {
      return html`
        <div class="empty-section-message">No fields in section</div>
      `;
    }

    return html`
      <div class="section-fields">
        ${fieldEntries.map(([key, value]) => html`
          <div class="field-row">
            <frontmatter-key
              .key="${key}"
              .editable="${!this.disabled}"
              placeholder="Field name"
              @key-change="${this._handleKeyChange}"
            ></frontmatter-key>
            <frontmatter-value-string
              .value="${value}"
              .disabled="${this.disabled}"
              placeholder="Field value"
              @value-change="${(e: CustomEvent) => this._handleValueChange(e, key)}"
            ></frontmatter-value-string>
            <button
              class="remove-field-button"
              .disabled="${this.disabled}"
              @click="${() => this._handleRemoveField(key)}"
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
      <div class="section-container">
        <div class="section-header">
          <span class="section-title">Section Fields (${Object.keys(this.fields).length})</span>
          <button
            class="add-field-button"
            .disabled="${this.disabled}"
            @click="${this._handleAddField}"
          >
            Add Field
          </button>
        </div>
        ${this.renderSectionFields()}
      </div>
    `;
  }
}

customElements.define('frontmatter-value-section', FrontmatterValueSection);

declare global {
  interface HTMLElementTagNameMap {
    'frontmatter-value-section': FrontmatterValueSection;
  }
}