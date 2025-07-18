import { html, css, LitElement } from 'lit';
import { createClient } from '@connectrpc/connect';
import { Struct } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import { Frontmatter } from '../gen/api/v1/frontmatter_connect.js';
import { GetFrontmatterRequest, GetFrontmatterResponse } from '../gen/api/v1/frontmatter_pb.js';
import { sharedStyles, foundationCSS, dialogCSS, responsiveCSS } from './shared-styles.js';

export class FrontmatterEditorDialog extends LitElement {
  static override styles = [
    foundationCSS,
    dialogCSS,
    responsiveCSS,
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
        from {
          opacity: 0;
        }
        to {
          opacity: 1;
        }
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
        max-width: 600px;
        width: 90%;
        max-height: 80vh;
        display: flex;
        flex-direction: column;
        position: relative;
        z-index: 1;
        animation: slideIn 0.2s ease-out;
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

      .content {
        flex: 1;
        padding: 20px;
        overflow-y: auto;
      }

      .frontmatter-display {
        width: 100%;
        min-height: 200px;
        padding: 12px;
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 14px;
        line-height: 1.4;
        background: #f8f9fa;
        border: 1px solid #e9ecef;
        resize: vertical;
        box-sizing: border-box;
        white-space: pre-wrap;
        word-wrap: break-word;
      }

      .loading,
      .error {
        display: flex;
        align-items: center;
        justify-content: center;
        min-height: 200px;
        font-size: 16px;
      }

      .loading {
        color: #666;
      }

      .error {
        color: #dc3545;
        flex-direction: column;
        gap: 8px;
      }

      .footer {
        display: flex;
        gap: 12px;
        padding: 16px 20px;
        border-top: 1px solid #e0e0e0;
        justify-content: flex-end;
      }

      .button {
        padding: 8px 16px;
        border: 1px solid;
        cursor: pointer;
        font-size: 14px;
        font-weight: 500;
        transition: all 0.2s;
      }

      .button:hover {
        transform: translateY(-1px);
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
      }

      .button-cancel {
        background: white;
        color: #666;
        border-color: #ddd;
      }

      .button-cancel:hover {
        background: #f8f9fa;
        border-color: #999;
      }

      .button-save {
        background: #007bff;
        color: white;
        border-color: #007bff;
      }

      .button-save:hover {
        background: #0056b3;
        border-color: #0056b3;
      }

      .form-field {
        margin-bottom: 16px;
      }

      .form-field label {
        display: block;
        margin-bottom: 4px;
        font-weight: 500;
        color: #333;
      }

      .form-field input {
        width: 100%;
        padding: 8px 12px;
        border: 1px solid #ddd;
        border-radius: 4px;
        font-size: 14px;
        font-family: inherit;
        box-sizing: border-box;
      }

      .form-field input:focus {
        outline: none;
        border-color: #007bff;
        box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.1);
      }

      .field-section {
        margin-bottom: 24px;
        padding: 16px;
        border: 1px solid #e0e0e0;
        border-radius: 4px;
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
      }

      .section-title-input {
        font-weight: 600;
        color: #333;
        background: transparent;
        border: none;
        border-bottom: 1px solid #ddd;
        padding: 2px 4px;
        font-size: inherit;
        font-family: inherit;
      }

      .section-title-input:focus {
        outline: none;
        border-bottom-color: #007bff;
        background: #f9f9f9;
      }

      .section-controls {
        display: flex;
        gap: 8px;
      }

      .add-field-button,
      .remove-section-button,
      .remove-field-button,
      .save-new-field-button {
        padding: 4px 8px;
        font-size: 12px;
        border: 1px solid;
        border-radius: 2px;
        cursor: pointer;
        transition: all 0.2s;
      }

      .add-field-button {
        background: #28a745;
        color: white;
        border-color: #28a745;
      }

      .add-field-button:hover {
        background: #218838;
        border-color: #218838;
      }

      .remove-section-button,
      .remove-field-button {
        background: #dc3545;
        color: white;
        border-color: #dc3545;
      }

      .remove-section-button:hover,
      .remove-field-button:hover {
        background: #c82333;
        border-color: #c82333;
      }

      .save-new-field-button {
        background: #007bff;
        color: white;
        border-color: #007bff;
      }

      .save-new-field-button:hover {
        background: #0056b3;
        border-color: #0056b3;
      }

      .field-row {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 8px;
      }

      .field-row .form-field {
        flex: 1;
        margin-bottom: 0;
      }

      .key-value-row {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 8px;
      }

      .key-value-row .key-input {
        flex: 0 0 150px;
        padding: 8px 12px;
        border: 1px solid #ddd;
        border-radius: 4px;
        font-size: 14px;
        font-family: inherit;
        box-sizing: border-box;
        font-weight: 500;
        background: #f8f9fa;
      }

      .key-value-row .key-input:focus {
        outline: none;
        border-color: #007bff;
        box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.1);
        background: white;
      }

      .key-value-row .value-input {
        flex: 1;
        padding: 8px 12px;
        border: 1px solid #ddd;
        border-radius: 4px;
        font-size: 14px;
        font-family: inherit;
        box-sizing: border-box;
      }

      .key-value-row .value-input:focus {
        outline: none;
        border-color: #007bff;
        box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.1);
      }

      .top-level-controls {
        margin-bottom: 20px;
        padding: 16px;
        border: 1px solid #e0e0e0;
        border-radius: 4px;
        background: #f9f9f9;
      }

      .dropdown-container {
        position: relative;
        display: inline-block;
      }

      .dropdown-button {
        padding: 8px 16px;
        font-size: 14px;
        border: 1px solid #28a745;
        border-radius: 4px;
        cursor: pointer;
        background: #28a745;
        color: white;
        display: flex;
        align-items: center;
        gap: 8px;
      }

      .dropdown-button:hover {
        background: #218838;
        border-color: #218838;
      }

      .dropdown-menu {
        position: absolute;
        top: 100%;
        left: 0;
        background: white;
        border: 1px solid #ddd;
        border-radius: 4px;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
        z-index: 1000;
        min-width: 150px;
      }

      .dropdown-item {
        padding: 8px 12px;
        cursor: pointer;
        border: none;
        background: none;
        width: 100%;
        text-align: left;
        font-size: 14px;
      }

      .dropdown-item:hover {
        background: #f8f9fa;
      }

      .dropdown-item:first-child {
        border-radius: 4px 4px 0 0;
      }

      .dropdown-item:last-child {
        border-radius: 0 0 4px 4px;
      }

      .new-field-row {
        display: flex;
        gap: 8px;
        margin-top: 12px;
        padding-top: 12px;
        border-top: 1px solid #e0e0e0;
      }

      .new-field-row input {
        flex: 1;
        padding: 6px 10px;
        font-size: 13px;
      }

      .array-section {
        border-left: 3px solid #007bff;
      }

      .array-item {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 8px;
        padding: 8px;
        background: #fff;
        border: 1px solid #e0e0e0;
        border-radius: 2px;
      }

      .array-item .form-field {
        flex: 1;
        margin-bottom: 0;
      }

      .array-items {
        margin-left: 16px;
      }
    `
  ];

  static override properties = {
    page: { type: String },
    open: { type: Boolean, reflect: true },
    loading: { state: true },
    error: { state: true },
    frontmatter: { state: true },
    workingFrontmatter: { state: true },
    dropdownOpen: { state: true },
  };

  declare page: string;
  declare open: boolean;
  declare loading: boolean;
  declare error?: string | undefined;
  declare frontmatter?: GetFrontmatterResponse | undefined;
  declare workingFrontmatter?: Record<string, unknown>;
  declare dropdownOpen: boolean;

  private client = createClient(Frontmatter, getGrpcWebTransport());

  constructor() {
    super();
    this.page = '';
    this.open = false;
    this.loading = false;
    this.workingFrontmatter = {};
    this.dropdownOpen = false;
  }

  private convertStructToPlainObject(struct?: Struct): Record<string, unknown> {
    if (!struct) return {};
    
    try {
      return struct.toJson() as Record<string, unknown>;
    } catch (err) {
      console.error('Error converting struct to plain object:', err);
      return {};
    }
  }

  private updateWorkingFrontmatter(): void {
    if (this.frontmatter?.frontmatter) {
      this.workingFrontmatter = this.convertStructToPlainObject(this.frontmatter.frontmatter);
    } else {
      this.workingFrontmatter = {};
    }
  }

  private _handleKeyChange = (oldPath: string, newKey: string): void => {
    if (!newKey.trim() || newKey === oldPath.split('.').pop()) return;
    
    // Get the current value at the old path
    const currentValue = this._getValueAtPath(oldPath);
    
    // Create the new path
    const pathParts = oldPath.split('.');
    pathParts[pathParts.length - 1] = newKey.trim();
    const newPath = pathParts.join('.');
    
    // Remove the old path and set the new one
    this._removeValueAtPath(oldPath);
    this._setValueAtPath(newPath, currentValue);
    
    this.requestUpdate();
  };

  private _handleToggleDropdown = (): void => {
    this.dropdownOpen = !this.dropdownOpen;
  };

  private _handleAddTopLevelField = (): void => {
    this._addTopLevelField('field');
    this.dropdownOpen = false;
  };

  private _handleAddTopLevelArray = (): void => {
    this._addTopLevelField('array');
    this.dropdownOpen = false;
  };

  private _handleAddTopLevelSection = (): void => {
    this._addTopLevelField('section');
    this.dropdownOpen = false;
  };

  private _addTopLevelField(type: 'field' | 'array' | 'section'): void {
    const newKey = this._generateUniqueKey(type === 'field' ? 'new_field' : type === 'array' ? 'new_array' : 'new_section');
    
    switch (type) {
      case 'field':
        this.workingFrontmatter[newKey] = '';
        break;
      case 'array':
        this.workingFrontmatter[newKey] = [];
        break;
      case 'section':
        this.workingFrontmatter[newKey] = {};
        break;
    }
    
    this.requestUpdate();
  };

  private _generateUniqueKey(baseKey: string): string {
    let counter = 1;
    let newKey = baseKey;
    
    while (this.workingFrontmatter[newKey] !== undefined) {
      newKey = `${baseKey}_${counter}`;
      counter++;
    }
    
    return newKey;
  };

  override connectedCallback(): void {
    super.connectedCallback();
    // Handle escape key to close dialog
    document.addEventListener('keydown', this._handleKeydown);
    // Close dropdown when clicking outside
    document.addEventListener('click', this._handleClickOutside);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('keydown', this._handleKeydown);
    document.removeEventListener('click', this._handleClickOutside);
  }

  private _handleClickOutside = (event: Event): void => {
    if (!this.dropdownOpen || !this.shadowRoot) return;
    
    const dropdown = this.shadowRoot.querySelector('.dropdown-container');
    const target = event.target as Node;
    
    if (dropdown && target && !dropdown.contains(target)) {
      this.dropdownOpen = false;
    }
  };

  public _handleKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape' && this.open) {
      this._handleCancel();
    }
  };

  public openDialog(page: string): void {
    this.page = page;
    this.open = true;
    this.loadFrontmatter();
  }

  public close(): void {
    this.open = false;
    this.frontmatter = undefined;
    this.error = undefined;
    this.loading = false;
  }

  public async loadFrontmatter(): Promise<void> {
    if (!this.page) return;

    try {
      this.loading = true;
      this.error = undefined;
      this.frontmatter = undefined;
      this.workingFrontmatter = {};
      this.requestUpdate();

      const request = new GetFrontmatterRequest({ page: this.page });
      const response = await this.client.getFrontmatter(request);
      this.frontmatter = response;
      this.updateWorkingFrontmatter();
    } catch (err) {
      this.error = err instanceof Error ? err.message : 'Failed to load frontmatter';
    } finally {
      this.loading = false;
      this.requestUpdate();
    }
  }

  private _handleCancel = (): void => {
    this.close();
  };

  private _handleSaveClick = (): void => {
    // For now, just close the dialog
    // In future iterations, this would save the frontmatter
    this.close();
  };

  private formatFrontmatter(frontmatter?: GetFrontmatterResponse): string {
    if (!frontmatter?.frontmatter) {
      return '';
    }

    try {
      // Convert the protobuf Struct to a plain JavaScript object
      const jsonObject = frontmatter.frontmatter.toJson();
      return JSON.stringify(jsonObject, null, 2);
    } catch (err) {
      return `Error formatting frontmatter: ${err instanceof Error ? err.message : 'Unknown error'}`;
    }
  }

  private _handleFieldChange = (event: Event): void => {
    const target = event.target as HTMLInputElement;
    const fieldPath = target.name;
    const value = target.value;
    
    this._updateFieldValue(fieldPath, value);
  };

  private _updateFieldValue(fieldPath: string, value: string): void {
    const pathParts = fieldPath.split('.');
    let current = this.workingFrontmatter;
    
    // Navigate to the parent object
    for (let i = 0; i < pathParts.length - 1; i++) {
      if (!current[pathParts[i]]) {
        current[pathParts[i]] = {};
      }
      current = current[pathParts[i]];
    }
    
    // Set the value
    current[pathParts[pathParts.length - 1]] = value;
    this.requestUpdate();
  }

  private _handleAddField = (sectionKey: string): void => {
    const sectionContainer = this.shadowRoot?.querySelector(`.field-section[data-key="${sectionKey}"]`);
    if (!sectionContainer) return;

    // Show the new field inputs
    const newFieldRow = sectionContainer.querySelector('.new-field-row') as HTMLElement;
    if (newFieldRow) {
      newFieldRow.style.display = 'flex';
    }
  };

  private _handleSaveNewField = (sectionKey: string): void => {
    const sectionContainer = this.shadowRoot?.querySelector(`.field-section[data-key="${sectionKey}"]`);
    if (!sectionContainer) return;

    const keyInput = sectionContainer.querySelector('.new-field-key') as HTMLInputElement;
    const valueInput = sectionContainer.querySelector('.new-field-value') as HTMLInputElement;
    
    if (!keyInput || !valueInput || !keyInput.value.trim()) return;

    // Add the new field to the working frontmatter
    if (!this.workingFrontmatter[sectionKey]) {
      this.workingFrontmatter[sectionKey] = {};
    }
    this.workingFrontmatter[sectionKey][keyInput.value.trim()] = valueInput.value;
    
    // Clear the inputs and hide the new field row
    keyInput.value = '';
    valueInput.value = '';
    const newFieldRow = sectionContainer.querySelector('.new-field-row') as HTMLElement;
    if (newFieldRow) {
      newFieldRow.style.display = 'none';
    }
    
    this.requestUpdate();
  };

  private _handleRemoveField = (fieldPath: string): void => {
    const pathParts = fieldPath.split('.');
    let current = this.workingFrontmatter;
    
    // Navigate to the parent object
    for (let i = 0; i < pathParts.length - 1; i++) {
      if (!current[pathParts[i]]) return; // Path doesn't exist
      current = current[pathParts[i]];
    }
    
    // Remove the field
    delete current[pathParts[pathParts.length - 1]];
    this.requestUpdate();
  };

  private renderValue(key: string, value: unknown, path: string = '', isTopLevel: boolean = false): unknown {
    const fullPath = path ? `${path}.${key}` : key;
    
    if (typeof value === 'string') {
      return this.renderStringField(key, value, fullPath, isTopLevel);
    } else if (Array.isArray(value)) {
      return this.renderArrayField(key, value, fullPath, isTopLevel);
    } else if (typeof value === 'object' && value !== null) {
      return this.renderMapField(key, value as Record<string, unknown>, fullPath, isTopLevel);
    } else {
      // For other types (numbers, booleans), render as string for now
      return this.renderStringField(key, String(value), fullPath, isTopLevel);
    }
  }

  private renderStringField(key: string, value: string, path: string, isTopLevel: boolean = false): unknown {
    const keyParts = path.split('.');
    const currentKey = keyParts[keyParts.length - 1];
    
    return html`
      <div class="key-value-row">
        <input 
          type="text" 
          class="key-input"
          .value="${currentKey}" 
          @input="${(e: Event) => this._handleKeyChange(path, (e.target as HTMLInputElement).value)}"
          placeholder="Field name"
        />
        <input 
          type="text" 
          class="value-input"
          name="${path}" 
          .value="${value}" 
          @input="${this._handleFieldChange}"
          placeholder="Field value"
        />
        ${isTopLevel ? html`
          <button 
            class="remove-field-button" 
            @click="${() => this._handleRemoveField(path)}"
          >
            Remove
          </button>
        ` : ''}
      </div>
    `;
  }

  private renderMapField(key: string, value: Record<string, unknown>, path: string, isTopLevel: boolean = false): unknown {
    const fields = Object.entries(value).map(([subKey, subValue]) => {
      const currentPath = path || key;
      const fieldPath = `${currentPath}.${subKey}`;
      return html`
        <div class="field-row">
          ${this.renderValue(subKey, subValue, currentPath)}
          <button 
            class="remove-field-button" 
            data-field="${fieldPath}"
            @click="${() => this._handleRemoveField(fieldPath)}"
          >
            Remove
          </button>
        </div>
      `;
    });

    return html`
      <div class="field-section" data-key="${key}">
        <div class="section-header">
          <input 
            type="text" 
            class="section-title-input" 
            .value="${key}" 
            @input="${(e: Event) => this._handleSectionNameChange(path || key, (e.target as HTMLInputElement).value)}"
          />
          <div class="section-controls">
            <button 
              class="add-field-button" 
              @click="${() => this._handleAddField(key)}"
            >
              Add Field
            </button>
            <button 
              class="remove-section-button"
              @click="${() => this._handleRemoveField(path || key)}"
            >
              Remove Section
            </button>
          </div>
        </div>
        ${fields}
        <div class="new-field-row" style="display: none;">
          <input 
            type="text" 
            class="new-field-key" 
            placeholder="Field name"
          />
          <input 
            type="text" 
            class="new-field-value" 
            placeholder="Field value"
          />
          <button 
            class="save-new-field-button"
            @click="${() => this._handleSaveNewField(key)}"
          >
            Save
          </button>
        </div>
      </div>
    `;
  }

  private renderArrayField(key: string, value: unknown[], path: string, isTopLevel: boolean = false): unknown {
    const arrayItems = value.map((item, index) => {
      const itemPath = `${path}[${index}]`;
      return html`
        <div class="array-item">
          <input 
            type="text" 
            class="value-input"
            name="${itemPath}" 
            .value="${String(item)}" 
            @input="${this._handleArrayItemChange}"
            placeholder="Array item"
          />
          <button 
            class="remove-field-button" 
            @click="${() => this._handleRemoveArrayItem(path, index)}"
          >
            Remove
          </button>
        </div>
      `;
    });

    return html`
      <div class="field-section array-section" data-key="${key}">
        <div class="section-header">
          <span class="section-title">${key} (Array)</span>
          <div class="section-controls">
            <button 
              class="add-field-button" 
              @click="${() => this._handleAddArrayItem(path)}"
            >
              Add Item
            </button>
            ${isTopLevel ? html`
              <button 
                class="remove-section-button"
                @click="${() => this._handleRemoveField(path)}"
              >
                Remove Array
              </button>
            ` : html`
              <button 
                class="remove-section-button"
                @click="${() => this._handleRemoveField(path)}"
              >
                Remove Array
              </button>
            `}
          </div>
        </div>
        <div class="array-items">
          ${arrayItems}
        </div>
      </div>
    `;
  }

  private _handleSectionNameChange = (oldPath: string, newName: string): void => {
    if (!newName.trim() || newName === oldPath) return;
    
    // Get the current value of the section
    const currentValue = this._getValueAtPath(oldPath);
    
    // Remove the old section
    this._removeValueAtPath(oldPath);
    
    // Add the new section with the same value
    this._setValueAtPath(newName, currentValue);
    
    this.requestUpdate();
  };

  private _handleArrayItemChange = (event: Event): void => {
    const target = event.target as HTMLInputElement;
    const fieldPath = target.name;
    const value = target.value;
    
    // Parse array path like "inventory.items[0]"
    const match = fieldPath.match(/^(.+)\[(\d+)\]$/);
    if (!match) return;
    
    const arrayPath = match[1];
    const index = parseInt(match[2], 10);
    
    const array = this._getValueAtPath(arrayPath) as unknown[];
    if (Array.isArray(array) && index >= 0 && index < array.length) {
      array[index] = value;
      this.requestUpdate();
    }
  };

  private _handleAddArrayItem = (arrayPath: string): void => {
    const array = this._getValueAtPath(arrayPath) as unknown[];
    if (Array.isArray(array)) {
      array.push('');
      this.requestUpdate();
    }
  };

  private _handleRemoveArrayItem = (arrayPath: string, index: number): void => {
    const array = this._getValueAtPath(arrayPath) as unknown[];
    if (Array.isArray(array) && index >= 0 && index < array.length) {
      array.splice(index, 1);
      this.requestUpdate();
    }
  };

  private _getValueAtPath(path: string): unknown {
    const pathParts = path.split('.');
    let current = this.workingFrontmatter as Record<string, unknown>;
    
    for (const part of pathParts) {
      if (!current || typeof current !== 'object') return undefined;
      current = current[part] as Record<string, unknown>;
    }
    
    return current;
  }

  private _setValueAtPath(path: string, value: unknown): void {
    const pathParts = path.split('.');
    let current = this.workingFrontmatter as Record<string, unknown>;
    
    // Navigate to the parent object
    for (let i = 0; i < pathParts.length - 1; i++) {
      if (!current[pathParts[i]]) {
        current[pathParts[i]] = {};
      }
      current = current[pathParts[i]] as Record<string, unknown>;
    }
    
    // Set the value
    current[pathParts[pathParts.length - 1]] = value;
  }

  private _removeValueAtPath(path: string): void {
    const pathParts = path.split('.');
    let current = this.workingFrontmatter as Record<string, unknown>;
    
    // Navigate to the parent object
    for (let i = 0; i < pathParts.length - 1; i++) {
      if (!current[pathParts[i]]) return; // Path doesn't exist
      current = current[pathParts[i]] as Record<string, unknown>;
    }
    
    // Remove the value
    delete current[pathParts[pathParts.length - 1]];
  }

  private renderFrontmatterEditor(): unknown {
    if (!this.workingFrontmatter || Object.keys(this.workingFrontmatter).length === 0) {
      return html`
        <div class="top-level-controls">
          <div class="dropdown-container">
            <button 
              class="dropdown-button" 
              @click="${(e: Event) => { e.stopPropagation(); this._handleToggleDropdown(); }}"
            >
              Add Field ▼
            </button>
            ${this.dropdownOpen ? html`
              <div class="dropdown-menu">
                <button class="dropdown-item" @click="${this._handleAddTopLevelField}">Add Field</button>
                <button class="dropdown-item" @click="${this._handleAddTopLevelArray}">Add Array</button>
                <button class="dropdown-item" @click="${this._handleAddTopLevelSection}">Add Section</button>
              </div>
            ` : ''}
          </div>
        </div>
        <div class="loading">No frontmatter to edit - use "Add Field" to get started</div>
      `;
    }

    const fields = Object.entries(this.workingFrontmatter).map(([key, value]) => 
      this.renderValue(key, value, '', true)
    );

    return html`
      <div class="frontmatter-editor">
        <div class="top-level-controls">
          <div class="dropdown-container">
            <button 
              class="dropdown-button" 
              @click="${(e: Event) => { e.stopPropagation(); this._handleToggleDropdown(); }}"
            >
              Add Field ▼
            </button>
            ${this.dropdownOpen ? html`
              <div class="dropdown-menu">
                <button class="dropdown-item" @click="${this._handleAddTopLevelField}">Add Field</button>
                <button class="dropdown-item" @click="${this._handleAddTopLevelArray}">Add Array</button>
                <button class="dropdown-item" @click="${this._handleAddTopLevelSection}">Add Section</button>
              </div>
            ` : ''}
          </div>
        </div>
        ${fields}
      </div>
    `;
  }

  override render() {
    return html`
      ${sharedStyles}
      <div class="backdrop"></div>
      <div class="dialog system-font border-radius box-shadow">
        <div class="dialog-header">
          <h2 class="dialog-title">Edit Frontmatter</h2>
        </div>
        <div class="content">
          ${this.loading ? html`
            <div class="loading">
              <i class="fas fa-spinner fa-spin"></i>
              Loading frontmatter...
            </div>
          ` : this.error ? html`
            <div class="error">
              <i class="fas fa-exclamation-triangle"></i>
              ${this.error}
            </div>
          ` : html`
            ${this.renderFrontmatterEditor()}
          `}
        </div>
        <div class="footer">
          <button class="button button-cancel border-radius-small" @click="${this._handleCancel}">
            Cancel
          </button>
          <button class="button button-save border-radius-small" @click="${this._handleSaveClick}">
            Save
          </button>
        </div>
      </div>
    `;
  }
}

customElements.define('frontmatter-editor-dialog', FrontmatterEditorDialog);

declare global {
  interface HTMLElementTagNameMap {
    'frontmatter-editor-dialog': FrontmatterEditorDialog;
  }
}