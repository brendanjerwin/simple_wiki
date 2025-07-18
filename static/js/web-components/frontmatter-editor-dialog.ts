import { html, css, LitElement } from 'lit';
import { createClient } from '@connectrpc/connect';
import { Struct } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import { Frontmatter } from '../gen/api/v1/frontmatter_connect.js';
import { GetFrontmatterRequest, GetFrontmatterResponse } from '../gen/api/v1/frontmatter_pb.js';
import { sharedStyles, foundationCSS, dialogCSS, responsiveCSS } from './shared-styles.js';
import './frontmatter-key.js';
import './frontmatter-value.js';

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

      .top-level-field {
        display: flex;
        flex-direction: column;
        gap: 8px;
        margin-bottom: 16px;
        padding: 12px;
        background: #fff;
        border: 1px solid #e0e0e0;
        border-radius: 4px;
        position: relative;
      }

      .top-level-field frontmatter-key {
        align-self: flex-start;
      }

      .top-level-field frontmatter-value {
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

      .remove-field-button:hover {
        background: #c82333;
        border-color: #c82333;
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

  private _handleKeyChange = (event: CustomEvent): void => {
    const { oldKey, newKey } = event.detail;
    
    if (!newKey.trim() || newKey === oldKey) return;
    
    // Get the current value at the old key
    const currentValue = this.workingFrontmatter[oldKey];
    
    // Remove the old key and set the new one
    delete this.workingFrontmatter[oldKey];
    this.workingFrontmatter[newKey] = currentValue;
    
    this.requestUpdate();
  };

  private _handleValueChange = (event: CustomEvent, key: string): void => {
    const { newValue } = event.detail;
    this.workingFrontmatter[key] = newValue;
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
    if (!this.dropdownOpen) return;
    
    const dropdown = this.shadowRoot?.querySelector('.dropdown-container');
    if (dropdown && !dropdown.contains(event.target as Node)) {
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

  private _handleRemoveField = (key: string): void => {
    delete this.workingFrontmatter[key];
    this.requestUpdate();
  };

  private renderTopLevelField(key: string, value: unknown): unknown {
    return html`
      <div class="top-level-field">
        <frontmatter-key
          .key="${key}"
          .editable="${true}"
          placeholder="Field name"
          @key-change="${(e: CustomEvent) => this._handleTopLevelKeyChange(e, key)}"
        ></frontmatter-key>
        <frontmatter-value
          .value="${value}"
          placeholder="Field value"
          @value-change="${(e: CustomEvent) => this._handleTopLevelValueChange(e, key)}"
        ></frontmatter-value>
        <button 
          class="remove-field-button" 
          @click="${() => this._handleRemoveField(key)}"
        >
          Remove
        </button>
      </div>
    `;
  }

  private _handleTopLevelKeyChange = (event: CustomEvent, oldKey: string): void => {
    const { newKey } = event.detail;
    
    if (!newKey.trim() || newKey === oldKey) return;
    
    // Get the current value at the old key
    const currentValue = this.workingFrontmatter[oldKey];
    
    // Remove the old key and set the new one
    delete this.workingFrontmatter[oldKey];
    this.workingFrontmatter[newKey] = currentValue;
    
    this.requestUpdate();
  };

  private _handleTopLevelValueChange = (event: CustomEvent, key: string): void => {
    const { newValue } = event.detail;
    this.workingFrontmatter[key] = newValue;
    this.requestUpdate();
  };

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
      this.renderTopLevelField(key, value)
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