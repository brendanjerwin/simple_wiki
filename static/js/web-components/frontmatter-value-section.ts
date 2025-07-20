import { html, css, LitElement } from 'lit';
import { buttonCSS, foundationCSS } from './shared-styles.js';
import './frontmatter-key.js';
import './frontmatter-value.js';
import './frontmatter-add-field-button.js';

export class FrontmatterValueSection extends LitElement {
  static override styles = [
    foundationCSS,
    buttonCSS,
    css`
      :host {
        display: block;
      }

      .section-container {
        border: 1px solid #e0e0e0;
        padding-left: 4px;
        padding-top: 4px;
        padding-bottom: 4px;
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
        margin-bottom: 4px;
        padding-bottom: 2px;
        border-bottom: 1px solid #e0e0e0;
      }

      .section-header.root-header {
        border-bottom: none;
        padding-bottom: 0;
        justify-content: flex-end;
      }

      .section-title {
        font-weight: normal;
        color: #888;
        font-size: 11px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
      }

      .section-fields {
        display: flex;
        flex-direction: column;
        gap: 4px;
      }

      .field-row {
        display: flex;
        flex-direction: column;
        gap: 2px;
        padding-left: 4px;
        padding-top: 4px;
        padding-bottom: 4px;
        padding-right: 4px;
        background: #fff;
        border: 1px solid #e0e0e0;
        position: relative;
        margin-left: 4px;
      }

      .field-content {
        display: flex;
        flex-direction: column;
        gap: 2px;
        padding-right: 60px; /* Space for remove button */
      }

      .field-row frontmatter-key {
        align-self: flex-start;
      }

      .field-row frontmatter-value-string {
        width: 100%;
      }

      .remove-field-button {
        position: absolute;
        top: 4px;
        right: 4px;
        flex-shrink: 0;
      }

      .empty-section-message {
        text-align: center;
        color: #666;
        font-style: italic;
        padding: 16px;
      }
    `
  ];

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
    this._pendingFocusKey = null;
  }

  private _pendingFocusKey: string | null = null;

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
    
    // Track that this key should receive focus after reordering
    this._pendingFocusKey = newKey;
    
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

  override updated(changedProperties: Map<string | number | symbol, unknown>): void {
    super.updated(changedProperties);
    
    // Handle focus management after reordering
    if (this._pendingFocusKey && changedProperties.has('fields')) {
      this._restoreFocusToField(this._pendingFocusKey);
      this._pendingFocusKey = null;
    }
  }

  private _restoreFocusToField(key: string): void {
    // Use multiple animation frames to ensure DOM is fully updated after reordering
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        const fieldRows = this.shadowRoot?.querySelectorAll('.field-row');
        const sortedEntries = this._sortFieldEntries(Object.entries(this.fields));
        
        // Find the index of the key in the sorted order
        const keyIndex = sortedEntries.findIndex(([k]) => k === key);
        
        if (keyIndex !== -1 && fieldRows && fieldRows[keyIndex]) {
          const fieldRow = fieldRows[keyIndex];
          const valueComponent = fieldRow.querySelector('frontmatter-value');
          
          if (valueComponent) {
            // First try to focus a direct input in the value component
            let valueInput = valueComponent.shadowRoot?.querySelector('input, textarea');
            
            // If not found, check for nested components (like in arrays or sections)
            if (!valueInput) {
              const stringComponent = valueComponent.shadowRoot?.querySelector('frontmatter-value-string');
              if (stringComponent) {
                valueInput = stringComponent.shadowRoot?.querySelector('input, textarea');
              }
            }
            
            if (valueInput instanceof HTMLElement) {
              valueInput.focus();
            }
          }
        }
      });
    });
  }

  private _getValueType(value: unknown): string {
    if (Array.isArray(value)) return 'array';
    if (typeof value === 'object' && value !== null) return 'object';
    return 'string';
  }

  private _sortFieldEntries(entries: [string, unknown][]): [string, unknown][] {
    return entries.sort(([keyA, valueA], [keyB, valueB]) => {
      const typeA = this._getValueType(valueA);
      const typeB = this._getValueType(valueB);
      
      // First sort by type priority: string < array < object
      const typePriority = { string: 0, array: 1, object: 2 };
      const priorityDiff = typePriority[typeA as keyof typeof typePriority] - typePriority[typeB as keyof typeof typePriority];
      
      if (priorityDiff !== 0) {
        return priorityDiff;
      }
      
      // Then sort alphabetically by key
      return keyA.localeCompare(keyB);
    });
  }

  private renderSectionFields() {
    const fieldEntries = Object.entries(this.fields);
    
    if (fieldEntries.length === 0) {
      return html`
        <div class="empty-section-message">No fields in section</div>
      `;
    }

    const sortedEntries = this._sortFieldEntries(fieldEntries);

    return html`
      <div class="section-fields">
        ${sortedEntries.map(([key, value]) => html`
          <div class="field-row">
            <div class="field-content">
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
            </div>
            <button
              class="button-base button-primary button-small border-radius-small remove-field-button"
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