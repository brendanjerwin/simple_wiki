import { html, css, LitElement, nothing } from 'lit';
import { sharedStyles, foundationCSS, dialogCSS, responsiveCSS, buttonCSS } from './shared-styles.js';
import { InventoryActionService } from './inventory-action-service.js';
import { createClient } from '@connectrpc/connect';
import { getGrpcWebTransport } from './grpc-transport.js';
import { SearchService } from '../gen/api/v1/search_connect.js';
import type { SearchResult } from '../gen/api/v1/search_pb.js';
import { SearchContentRequest } from '../gen/api/v1/search_pb.js';
import type { ExistingPageInfo } from '../gen/api/v1/page_management_pb.js';

/**
 * InventoryAddItemDialog - Modal dialog for adding new inventory items
 *
 * Title-first workflow: user enters a title, identifier is auto-generated
 * via server call (automagic mode). User can click the sparkle button to
 * switch to manual mode for editing the identifier. Also includes Description
 * field and inline search results to help find existing items.
 */
export class InventoryAddItemDialog extends LitElement {
  static override styles = [
    foundationCSS,
    dialogCSS,
    responsiveCSS,
    buttonCSS,
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
        max-height: 90vh;
        display: flex;
        flex-direction: column;
        position: relative;
        z-index: 1;
        animation: slideIn 0.2s ease-out;
        border-radius: 8px;
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
          max-height: none;
          border-radius: 0;
          margin: 0;
        }
      }

      .content {
        padding: 20px;
        overflow-y: auto;
        flex: 1;
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

      .automagic-button:hover {
        background: #e8e8e8;
        border-color: #ccc;
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

      .error-message {
        background: #fef2f2;
        border: 1px solid #fecaca;
        color: #dc2626;
        padding: 12px;
        border-radius: 4px;
        margin-bottom: 16px;
        font-size: 14px;
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

      .search-results {
        margin-top: 16px;
        border: 1px solid #e5e7eb;
        border-radius: 4px;
        max-height: 200px;
        overflow-y: auto;
      }

      .search-results-header {
        padding: 8px 12px;
        background: #f9fafb;
        border-bottom: 1px solid #e5e7eb;
        font-size: 12px;
        font-weight: 500;
        color: #6b7280;
      }

      .search-result-item {
        display: block;
        padding: 10px 12px;
        border-bottom: 1px solid #e5e7eb;
        text-decoration: none;
        color: inherit;
        cursor: pointer;
        transition: background-color 0.15s;
      }

      .search-result-item:last-child {
        border-bottom: none;
      }

      .search-result-item:hover {
        background: #f3f4f6;
      }

      .search-result-title {
        font-weight: 500;
        color: #1f2937;
        margin-bottom: 2px;
      }

      .search-result-container {
        font-size: 12px;
        color: #6b7280;
      }

      .search-result-container a {
        color: #4a90d9;
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
    container: { type: String },
    itemTitle: { type: String },
    itemIdentifier: { type: String },
    description: { type: String },
    automagicMode: { type: Boolean },
    loading: { state: true },
    error: { state: true },
    isUnique: { state: true },
    existingPage: { state: true },
    searchResults: { state: true },
    searchLoading: { state: true },
  };

  declare open: boolean;
  declare container: string;
  declare itemTitle: string;
  declare itemIdentifier: string;
  declare description: string;
  declare automagicMode: boolean;
  declare loading: boolean;
  declare error?: string;
  declare isUnique: boolean;
  declare existingPage?: ExistingPageInfo;
  declare searchResults: SearchResult[];
  declare searchLoading: boolean;

  private _titleDebounceTimeoutMs = 300;
  private _titleDebounceTimer?: ReturnType<typeof setTimeout>;
  private _identifierDebounceTimer?: ReturnType<typeof setTimeout>;
  private searchClient = createClient(SearchService, getGrpcWebTransport());
  private inventoryActionService = new InventoryActionService();

  constructor() {
    super();
    this.open = false;
    this.container = '';
    this.itemTitle = '';
    this.itemIdentifier = '';
    this.description = '';
    this.automagicMode = true;
    this.loading = false;
    this.error = undefined;
    this.isUnique = true;
    this.existingPage = undefined;
    this.searchResults = [];
    this.searchLoading = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('keydown', this._handleKeydown);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('keydown', this._handleKeydown);
    this._clearDebounceTimers();
  }

  private _clearDebounceTimers(): void {
    if (this._titleDebounceTimer) {
      clearTimeout(this._titleDebounceTimer);
      this._titleDebounceTimer = undefined;
    }
    if (this._identifierDebounceTimer) {
      clearTimeout(this._identifierDebounceTimer);
      this._identifierDebounceTimer = undefined;
    }
  }

  public _handleKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape' && this.open) {
      this.close();
    }
  };

  public openDialog(container: string): void {
    this.container = container;
    this.itemTitle = '';
    this.itemIdentifier = '';
    this.description = '';
    this.automagicMode = true;
    this.error = undefined;
    this.loading = false;
    this.isUnique = true;
    this.existingPage = undefined;
    this.searchResults = [];
    this.searchLoading = false;
    this.open = true;

    // Focus title field after render
    this.updateComplete.then(() => {
      const titleField = this.shadowRoot?.querySelector('input[name="title"]') as HTMLInputElement;
      titleField?.focus();
    });
  }

  public close(): void {
    this.open = false;
    this._clearDebounceTimers();
    this.itemTitle = '';
    this.itemIdentifier = '';
    this.description = '';
    this.error = undefined;
    this.loading = false;
    this.isUnique = true;
    this.existingPage = undefined;
    this.searchResults = [];
    this.searchLoading = false;
  }

  private _handleBackdropClick = (): void => {
    this.close();
  };

  private _handleDialogClick = (event: Event): void => {
    event.stopPropagation();
  };

  private _handleTitleInput = (event: Event): void => {
    const input = event.target as HTMLInputElement;
    this.itemTitle = input.value;

    // Clear existing timer
    if (this._titleDebounceTimer) {
      clearTimeout(this._titleDebounceTimer);
    }

    // Debounce the API calls
    this._titleDebounceTimer = setTimeout(() => {
      this._onTitleChanged();
    }, this._titleDebounceTimeoutMs);
  };

  private async _onTitleChanged(): Promise<void> {
    const title = this.itemTitle.trim();

    if (!title) {
      this.itemIdentifier = '';
      this.isUnique = true;
      this.existingPage = undefined;
      this.searchResults = [];
      return;
    }

    // Generate identifier if in automagic mode
    if (this.automagicMode) {
      const result = await this.inventoryActionService.generateIdentifier(title);
      if (!result.error) {
        this.itemIdentifier = result.identifier;
        this.isUnique = result.isUnique;
        this.existingPage = result.existingPage;
      }
    }

    // Search for similar items
    await this._performSearch(title);
  }

  private _handleIdentifierInput = (event: Event): void => {
    // Only allow editing in manual mode (not automagic)
    if (this.automagicMode) return;

    const input = event.target as HTMLInputElement;
    this.itemIdentifier = input.value;

    // Clear existing timer
    if (this._identifierDebounceTimer) {
      clearTimeout(this._identifierDebounceTimer);
    }

    // Debounce the API call to check availability
    this._identifierDebounceTimer = setTimeout(() => {
      this._checkIdentifierAvailability();
    }, this._titleDebounceTimeoutMs);
  };

  private async _checkIdentifierAvailability(): Promise<void> {
    const identifier = this.itemIdentifier.trim();

    if (!identifier) {
      this.isUnique = true;
      this.existingPage = undefined;
      return;
    }

    // We call generateIdentifier with ensure_unique=false just to check availability
    const result = await this.inventoryActionService.generateIdentifier(identifier);
    if (!result.error) {
      this.isUnique = result.isUnique;
      this.existingPage = result.existingPage;
    }
  }

  private _handleDescriptionInput = (event: Event): void => {
    const input = event.target as HTMLTextAreaElement;
    this.description = input.value;
  };

  private _handleAutomagicToggle = (): void => {
    this.automagicMode = !this.automagicMode;

    // If switching back to automagic, regenerate identifier from title
    if (this.automagicMode && this.itemTitle.trim()) {
      this._onTitleChanged();
    }
  };

  private async _performSearch(query: string): Promise<void> {
    if (!query) {
      this.searchResults = [];
      return;
    }

    this.searchLoading = true;

    try {
      const request = new SearchContentRequest({
        query,
        frontmatterKeyIncludeFilters: ['inventory.container'],
        frontmatterKeyExcludeFilters: ['inventory.is_container'],
        frontmatterKeysToReturnInResults: ['inventory.container'],
      });

      const response = await this.searchClient.searchContent(request);
      this.searchResults = response.results;
    } catch {
      // Silently fail - search is a nice-to-have
      this.searchResults = [];
    } finally {
      this.searchLoading = false;
    }
  }

  private _handleCancel = (): void => {
    this.close();
  };

  private get canSubmit(): boolean {
    return (
      this.itemTitle.trim().length > 0 &&
      this.itemIdentifier.trim().length > 0 &&
      this.isUnique &&
      !this.loading
    );
  }

  private _handleSubmit = async (): Promise<void> => {
    if (!this.canSubmit) return;

    this.loading = true;
    this.error = undefined;

    const result = await this.inventoryActionService.addItem(
      this.container,
      this.itemIdentifier.trim(),
      this.itemTitle.trim(),
      this.description.trim() || undefined
    );

    this.loading = false;

    if (result.success) {
      this.inventoryActionService.showSuccess(
        result.summary || `Added ${this.itemTitle} to ${this.container}`,
        () => window.location.reload()
      );
      this.close();
    } else {
      this.error = result.error;
    }
  };

  private _renderConflictWarning() {
    if (this.isUnique || !this.existingPage) {
      return nothing;
    }

    return html`
      <div class="conflict-warning">
        <strong>Identifier already exists:</strong>
        <a href="/${this.existingPage.identifier}">${this.existingPage.title || this.existingPage.identifier}</a>
        ${this.existingPage.container
          ? html` (Found In: <a href="/${this.existingPage.container}">${this.existingPage.container}</a>)`
          : ''}
      </div>
    `;
  }

  private _renderSearchResults() {
    if (this.searchResults.length === 0 && !this.searchLoading) {
      return nothing;
    }

    return html`
      <div class="search-results">
        <div class="search-results-header">
          ${this.searchLoading
            ? 'Searching...'
            : `${this.searchResults.length} similar item${this.searchResults.length === 1 ? '' : 's'} found`}
        </div>
        ${this.searchResults.map(
          result => html`
            <a class="search-result-item" href="/${result.identifier}">
              <div class="search-result-title">${result.title || result.identifier}</div>
              ${result.frontmatter?.['inventory.container']
                ? html`<div class="search-result-container">Found In: ${result.frontmatter['inventory.container']}</div>`
                : ''}
            </a>
          `
        )}
      </div>
    `;
  }

  override render() {
    return html`
      ${sharedStyles}
      <div class="backdrop" @click=${this._handleBackdropClick}></div>
      <div class="dialog system-font border-radius box-shadow" @click=${this._handleDialogClick}>
        <div class="dialog-header">
          <h2 class="dialog-title">Add Item to Container</h2>
        </div>

        <div class="content">
          ${this.error
            ? html`<div class="error-message">${this.error}</div>`
            : ''}

          <div class="form-group">
            <label for="container">Container</label>
            <input
              type="text"
              id="container"
              name="container"
              .value=${this.container}
              readonly
            />
            <div class="help-text">The container this item will be added to</div>
          </div>

          <div class="form-group">
            <label for="title">Title *</label>
            <input
              type="text"
              id="title"
              name="title"
              .value=${this.itemTitle}
              @input=${this._handleTitleInput}
              placeholder="e.g., Phillips Head Screwdriver"
              ?disabled=${this.loading}
            />
            <div class="help-text">Human-readable name for the item (required)</div>
          </div>

          <div class="form-group">
            <label for="itemIdentifier">Identifier *</label>
            <div class="identifier-field">
              <input
                type="text"
                id="itemIdentifier"
                name="itemIdentifier"
                .value=${this.itemIdentifier}
                @input=${this._handleIdentifierInput}
                placeholder=${this.automagicMode ? 'Auto-generated from title' : 'Enter identifier manually'}
                ?disabled=${this.loading}
                ?readonly=${this.automagicMode}
                tabindex=${this.automagicMode ? '-1' : '0'}
              />
              <button
                type="button"
                class="automagic-button ${this.automagicMode ? 'automagic' : 'manual'}"
                @click=${this._handleAutomagicToggle}
                title=${this.automagicMode ? 'Click to edit identifier manually' : 'Click to auto-generate from title'}
                ?disabled=${this.loading}
              >
                <i class="fa-solid ${this.automagicMode ? 'fa-wand-magic-sparkles' : 'fa-pen'}"></i>
              </button>
            </div>
            <div class="help-text">
              ${this.automagicMode
                ? 'Auto-generated from title. Click sparkle to edit manually.'
                : 'Manual mode. Click pen to auto-generate from title.'}
            </div>
            ${this._renderConflictWarning()}
          </div>

          <div class="form-group">
            <label for="description">Description (optional)</label>
            <textarea
              id="description"
              name="description"
              .value=${this.description}
              @input=${this._handleDescriptionInput}
              placeholder="Optional description of the item"
              ?disabled=${this.loading}
            ></textarea>
            <div class="help-text">Additional details about the item</div>
          </div>

          ${this._renderSearchResults()}
        </div>

        <div class="footer">
          <button
            class="button-base button-secondary button-large border-radius-small"
            @click=${this._handleCancel}
            ?disabled=${this.loading}
          >
            Cancel
          </button>
          <button
            class="button-base button-primary button-large border-radius-small"
            @click=${this._handleSubmit}
            ?disabled=${!this.canSubmit}
          >
            ${this.loading ? 'Adding...' : 'Add Item'}
          </button>
        </div>
      </div>
    `;
  }
}

customElements.define('inventory-add-item-dialog', InventoryAddItemDialog);
