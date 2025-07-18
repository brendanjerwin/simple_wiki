import { html, css, LitElement } from 'lit';
import { createClient } from '@connectrpc/connect';
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
    `
  ];

  static override properties = {
    page: { type: String },
    open: { type: Boolean, reflect: true },
    loading: { state: true },
    error: { state: true },
    frontmatter: { state: true },
    workingFrontmatter: { state: true },
  };

  declare page: string;
  declare open: boolean;
  declare loading: boolean;
  declare error?: string | undefined;
  declare frontmatter?: GetFrontmatterResponse | undefined;
  declare workingFrontmatter?: any;

  private client = createClient(Frontmatter, getGrpcWebTransport());

  constructor() {
    super();
    this.page = '';
    this.open = false;
    this.loading = false;
    this.workingFrontmatter = {};
  }

  private convertStructToPlainObject(struct?: Struct): any {
    if (!struct) return {};
    
    try {
      return struct.toJson();
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

  override connectedCallback(): void {
    super.connectedCallback();
    // Handle escape key to close dialog
    document.addEventListener('keydown', this._handleKeydown);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('keydown', this._handleKeydown);
  }

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

  private renderValue(key: string, value: any, path: string = ''): any {
    const fullPath = path ? `${path}.${key}` : key;
    
    if (typeof value === 'string') {
      return this.renderStringField(key, value, fullPath);
    } else if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
      return this.renderMapField(key, value, fullPath);
    } else {
      // For other types (numbers, booleans, arrays), render as string for now
      return this.renderStringField(key, String(value), fullPath);
    }
  }

  private renderStringField(key: string, value: string, path: string): any {
    return html`
      <div class="form-field">
        <label for="${path}">${key}</label>
        <input 
          type="text" 
          name="${path}" 
          id="${path}"
          .value="${value}" 
          @input="${this._handleFieldChange}"
        />
      </div>
    `;
  }

  private renderMapField(key: string, value: any, path: string): any {
    const fields = Object.entries(value).map(([subKey, subValue]) => {
      const fieldPath = `${path}.${subKey}`;
      return html`
        <div class="field-row">
          ${this.renderValue(subKey, subValue, path)}
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
          <span class="section-title">${key}</span>
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

  private renderFrontmatterEditor(): any {
    if (!this.workingFrontmatter || Object.keys(this.workingFrontmatter).length === 0) {
      return html`<div class="loading">No frontmatter to edit</div>`;
    }

    const fields = Object.entries(this.workingFrontmatter).map(([key, value]) => 
      this.renderValue(key, value)
    );

    return html`<div class="frontmatter-editor">${fields}</div>`;
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