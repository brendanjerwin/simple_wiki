import { html, css, LitElement } from 'lit';
import './frontmatter-key.js';
import './frontmatter-value.js';
import './frontmatter-add-field-button.js';

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

    .section-container.root-section {
      border: none;
      background: transparent;
      padding: 0;
    }

    .section-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 12px;
      padding-bottom: 8px;
      border-bottom: 1px solid #e0e0e0;
    }

    .section-header.root-header {
      border-bottom: none;
      padding-bottom: 0;
    }

    .section-title {
      font-weight: 600;
      color: #333;
      font-size: 14px;
    }

    .section-fields {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    .field-row {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 12px;
      background: #fff;
      border: 1px solid #e0e0e0;
      border-radius: 4px;
      position: relative;
    }

    .field-row frontmatter-key {
      align-self: flex-start;
    }

    .field-row frontmatter-value-string {
      width: 100%;
    }

    .remove-field-button {
      position: absolute;
      top: 8px;
      right: 8px;
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
    isRoot: { type: Boolean },
    title: { type: String },
  };

  declare fields: Record<string, unknown>;
  declare disabled: boolean;
  declare isRoot: boolean;
  declare title: string;

  constructor() {
    super();
    this.fields = {};
    this.disabled = false;
    this.isRoot = false;
    this.title = 'Section Fields';
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

  private _handleAddField = (event: CustomEvent): void => {
    const { type } = event.detail;
    const oldFields = { ...this.fields };
    const newKey = this._generateUniqueKey(
      type === 'field' ? 'new_field' : 
      type === 'array' ? 'new_array' : 
      'new_section'
    );
    
    let newValue: unknown;
    switch (type) {
      case 'field':
        newValue = '';
        break;
      case 'array':
        newValue = [];
        break;
      case 'section':
        newValue = {};
        break;
      default:
        return;
    }
    
    const newFields = { ...this.fields, [newKey]: newValue };
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

  private _dispatchSectionChange(oldFields: Record<string, unknown>, newFields: Record<string, unknown>): void {
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
            <frontmatter-value
              .value="${value}"
              .disabled="${this.disabled}"
              placeholder="Field value"
              @value-change="${(e: CustomEvent) => this._handleValueChange(e, key)}"
            ></frontmatter-value>
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
    const containerClass = this.isRoot ? 'section-container root-section' : 'section-container';
    const headerClass = this.isRoot ? 'section-header root-header' : 'section-header';
    const fieldCount = Object.keys(this.fields).length;

    return html`
      <div class="${containerClass}">
        <div class="${headerClass}">
          ${!this.isRoot ? html`
            <span class="section-title">${this.title} (${fieldCount})</span>
          ` : ''}
          <frontmatter-add-field-button
            .disabled="${this.disabled}"
            @add-field="${this._handleAddField}"
          ></frontmatter-add-field-button>
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