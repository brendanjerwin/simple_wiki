import { html, css, LitElement, nothing } from 'lit';
import { sharedStyles, dialogStyles } from './shared-styles.js';
import { InventoryItemCreatorMover } from './inventory-item-creator-mover.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import { SearchService, SearchContentRequestSchema, type SearchResult } from '../gen/api/v1/search_pb.js';
import { coerceThirdPartyError } from './augment-error-service.js';
import type { TitleChangeEventDetail, IdentifierChangeEventDetail } from './event-types.js';
import './automagic-identifier-input.js';
import type { AutomagicIdentifierInput, GenerateIdentifierResult } from './automagic-identifier-input.js';

/**
 * InventoryAddItemDialog - Modal dialog for adding new inventory items
 *
 * Title-first workflow: user enters a title, identifier is auto-generated
 * via server call (automagic mode). User can click the sparkle button to
 * switch to manual mode for editing the identifier. Also includes Description
 * field and inline search results to help find existing items.
 */
export class InventoryAddItemDialog extends LitElement {
  static override styles = dialogStyles(css`
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

    .content {
      padding: 20px;
      overflow-y: auto;
      flex: 1;
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

    .form-group textarea {
      min-height: 50px;
    }

    .footer {
      display: flex;
      gap: 12px;
      padding: 16px 20px;
      border-top: 1px solid #e0e0e0;
      justify-content: flex-end;
    }
  `);

  static override properties = {
    open: { type: Boolean, reflect: true },
    container: { type: String },
    itemTitle: { type: String },
    itemIdentifier: { type: String },
    description: { type: String },
    isUnique: { state: true },
    loading: { state: true },
    error: { state: true },
    searchResults: { state: true },
    searchLoading: { state: true },
  };

  declare open: boolean;
  declare container: string;
  declare itemTitle: string;
  declare itemIdentifier: string;
  declare description: string;
  declare isUnique: boolean;
  declare loading: boolean;
  declare error: Error | null;
  declare searchResults: SearchResult[];
  declare searchLoading: boolean;

  private _searchDebounceTimer?: ReturnType<typeof setTimeout>;
  private _debounceTimeoutMs = 300;
  private searchClient = createClient(SearchService, getGrpcWebTransport());
  private inventoryItemCreatorMover = new InventoryItemCreatorMover();

  constructor() {
    super();
    this.open = false;
    this.container = '';
    this.itemTitle = '';
    this.itemIdentifier = '';
    this.description = '';
    this.isUnique = true;
    this.loading = false;
    this.error = null;
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
    this._clearSearchDebounceTimer();
  }

  private _clearSearchDebounceTimer(): void {
    if (this._searchDebounceTimer) {
      clearTimeout(this._searchDebounceTimer);
      delete this._searchDebounceTimer;
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
    this.isUnique = true;
    this.error = null;
    this.loading = false;
    this.searchResults = [];
    this.searchLoading = false;
    this.open = true;

    // Reset and focus the automagic identifier input after render
    this.updateComplete.then(() => {
      const identifierInput = this.shadowRoot?.querySelector<AutomagicIdentifierInput>('automagic-identifier-input');
      identifierInput?.reset();
      identifierInput?.focusTitleInput();
    });
  }

  public close(): void {
    this.open = false;
    this._clearSearchDebounceTimer();
    this.itemTitle = '';
    this.itemIdentifier = '';
    this.description = '';
    this.isUnique = true;
    this.error = null;
    this.loading = false;
    this.searchResults = [];
    this.searchLoading = false;
  }

  private _handleBackdropClick = (): void => {
    this.close();
  };

  private _handleDialogClick = (event: Event): void => {
    event.stopPropagation();
  };

  /**
   * Adapter function to call InventoryItemCreatorMover.generateIdentifier
   * in the format expected by AutomagicIdentifierInput.
   */
  private _generateIdentifier = async (text: string): Promise<GenerateIdentifierResult> => {
    const result = await this.inventoryItemCreatorMover.generateIdentifier(text);
    const generateResult: GenerateIdentifierResult = {
      identifier: result.identifier,
      isUnique: result.isUnique,
    };
    if (result.existingPage) {
      generateResult.existingPage = result.existingPage;
    }
    if (result.error) {
      generateResult.error = result.error;
    }
    return generateResult;
  };

  private _handleTitleChange = (event: CustomEvent<TitleChangeEventDetail>): void => {
    this.itemTitle = event.detail.title;

    // Debounce search
    if (this._searchDebounceTimer) {
      clearTimeout(this._searchDebounceTimer);
    }

    const title = this.itemTitle.trim();
    if (!title) {
      this.searchResults = [];
      return;
    }

    this._searchDebounceTimer = setTimeout(() => {
      this._performSearch(title);
    }, this._debounceTimeoutMs);
  };

  private _handleIdentifierChange = (event: CustomEvent<IdentifierChangeEventDetail>): void => {
    this.itemIdentifier = event.detail.identifier;
    this.isUnique = event.detail.isUnique;
  };

  private _handleDescriptionInput = (event: Event): void => {
    if (!(event.target instanceof HTMLTextAreaElement)) {
      return;
    }
    const input = event.target;
    this.description = input.value;
  };

  private async _performSearch(query: string): Promise<void> {
    if (!query) {
      this.searchResults = [];
      return;
    }

    this.searchLoading = true;

    try {
      const request = create(SearchContentRequestSchema, {
        query,
        frontmatterKeyIncludeFilters: ['inventory.container'],
        frontmatterKeyExcludeFilters: ['inventory.is_container'],
        frontmatterKeysToReturnInResults: ['inventory.container'],
      });

      const response = await this.searchClient.searchContent(request);
      this.searchResults = response.results;
    } catch (err) {
      this.searchResults = [];
      this.error = coerceThirdPartyError(err, 'Container search failed');
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
    this.error = null;

    const result = await this.inventoryItemCreatorMover.addItem(
      this.container,
      this.itemIdentifier.trim(),
      this.itemTitle.trim(),
      this.description.trim() || undefined
    );

    this.loading = false;

    if (result.success) {
      this.inventoryItemCreatorMover.showSuccess(
        result.summary || `Added ${this.itemTitle} to ${this.container}`,
        () => window.location.reload()
      );
      this.close();
    } else {
      if (!result.error) {
        throw new Error('InventoryItemCreatorMover.addItem returned success=false without an error');
      }
      this.error = result.error;
    }
  };

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
          <h2 class="dialog-title">Add Item to: ${this.container}</h2>
        </div>

        <div class="content">
          ${this.error
            ? html`<div class="error-message">${this.error.message}</div>`
            : ''}

          <automagic-identifier-input
            .title=${this.itemTitle}
            .identifier=${this.itemIdentifier}
            .generateIdentifier=${this._generateIdentifier}
            .disabled=${this.loading}
            titlePlaceholder="e.g., Phillips Head Screwdriver"
            titleHelpText="Human-readable name for the item (required)"
            @title-change=${this._handleTitleChange}
            @identifier-change=${this._handleIdentifierChange}
          ></automagic-identifier-input>

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

declare global {
  interface HTMLElementTagNameMap {
    'inventory-add-item-dialog': InventoryAddItemDialog;
  }
}
