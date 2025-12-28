import { html, css, LitElement, nothing } from 'lit';
import { sharedStyles, foundationCSS, dialogCSS, responsiveCSS, buttonCSS } from './shared-styles.js';
import { InventoryActionService } from './inventory-action-service.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import { SearchService, SearchContentRequestSchema, type SearchResult } from '../gen/api/v1/search_pb.js';

/**
 * InventoryMoveItemDialog - Modal dialog for moving inventory items between containers
 *
 * Search-based destination selection: user types to search for containers,
 * results appear as "Move To" buttons that execute the move on click.
 */
export class InventoryMoveItemDialog extends LitElement {
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

      .form-group {
        margin-bottom: 16px;
      }

      .form-group:last-child {
        margin-bottom: 0;
      }

      .form-group label {
        display: block;
        margin-bottom: 6px;
        font-weight: 500;
        color: #333;
      }

      .form-group input {
        width: 100%;
        padding: 10px 12px;
        border: 1px solid #ddd;
        border-radius: 4px;
        font-size: 14px;
        box-sizing: border-box;
      }

      .form-group input:focus {
        outline: none;
        border-color: #4a90d9;
        box-shadow: 0 0 0 2px rgba(74, 144, 217, 0.2);
      }

      .form-group input[readonly] {
        background: #f5f5f5;
        color: #666;
        cursor: not-allowed;
      }

      .form-group .help-text {
        margin-top: 4px;
        font-size: 12px;
        color: #666;
      }

      .move-arrow {
        display: flex;
        justify-content: center;
        align-items: center;
        padding: 8px 0;
        color: #666;
        font-size: 24px;
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
        margin-top: 8px;
        border: 1px solid #e5e7eb;
        border-radius: 4px;
        max-height: 250px;
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
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 10px 12px;
        border-bottom: 1px solid #e5e7eb;
        gap: 12px;
      }

      .search-result-item:last-child {
        border-bottom: none;
      }

      .result-info {
        flex: 1;
        min-width: 0;
      }

      .result-title {
        font-weight: 500;
        color: #1f2937;
        margin-bottom: 2px;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }

      .result-container {
        font-size: 12px;
        color: #6b7280;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }

      .move-to-button {
        padding: 6px 12px;
        border: none;
        border-radius: 4px;
        background: #4a90d9;
        color: white;
        font-size: 13px;
        font-weight: 500;
        cursor: pointer;
        white-space: nowrap;
        transition: background-color 0.15s;
      }

      .move-to-button:hover:not(:disabled) {
        background: #3a7fc8;
      }

      .move-to-button:disabled {
        background: #9ca3af;
        cursor: not-allowed;
      }

      .no-results {
        padding: 16px 12px;
        text-align: center;
        color: #6b7280;
        font-size: 14px;
      }

      .footer {
        display: flex;
        gap: 12px;
        padding: 16px 20px;
        border-top: 1px solid #e0e0e0;
        justify-content: flex-end;
      }

      .footer-hint {
        flex: 1;
        font-size: 13px;
        color: #6b7280;
        display: flex;
        align-items: center;
      }
    `,
  ];

  static override properties = {
    open: { type: Boolean, reflect: true },
    itemIdentifier: { type: String },
    currentContainer: { type: String },
    searchQuery: { type: String },
    searchResults: { state: true },
    searchLoading: { state: true },
    movingTo: { state: true },
    error: { state: true },
  };

  declare open: boolean;
  declare itemIdentifier: string;
  declare currentContainer: string;
  declare searchQuery: string;
  declare searchResults: SearchResult[];
  declare searchLoading: boolean;
  declare movingTo: string | null;
  declare error?: string;

  private _searchDebounceTimeoutMs = 300;
  private _searchDebounceTimer?: ReturnType<typeof setTimeout>;
  private searchClient = createClient(SearchService, getGrpcWebTransport());
  private inventoryActionService = new InventoryActionService();

  constructor() {
    super();
    this.open = false;
    this.itemIdentifier = '';
    this.currentContainer = '';
    this.searchQuery = '';
    this.searchResults = [];
    this.searchLoading = false;
    this.movingTo = null;
    this.error = undefined;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('keydown', this._handleKeydown);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('keydown', this._handleKeydown);
    this._clearDebounceTimer();
  }

  private _clearDebounceTimer(): void {
    if (this._searchDebounceTimer) {
      clearTimeout(this._searchDebounceTimer);
      this._searchDebounceTimer = undefined;
    }
  }

  public _handleKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape' && this.open) {
      this.close();
    }
  };

  public openDialog(itemIdentifier: string, currentContainer: string): void {
    this.itemIdentifier = itemIdentifier;
    this.currentContainer = currentContainer;
    this.searchQuery = '';
    this.searchResults = [];
    this.searchLoading = false;
    this.movingTo = null;
    this.error = undefined;
    this.open = true;

    // Focus search field after render
    this.updateComplete.then(() => {
      const searchField = this.shadowRoot?.querySelector('input[name="searchQuery"]') as HTMLInputElement;
      searchField?.focus();
    });
  }

  public close(): void {
    this.open = false;
    this._clearDebounceTimer();
    this.searchQuery = '';
    this.searchResults = [];
    this.searchLoading = false;
    this.movingTo = null;
    this.error = undefined;
  }

  private _handleBackdropClick = (): void => {
    if (!this.movingTo) {
      this.close();
    }
  };

  private _handleDialogClick = (event: Event): void => {
    event.stopPropagation();
  };

  private _handleSearchInput = (event: Event): void => {
    const input = event.target as HTMLInputElement;
    this.searchQuery = input.value;
    this.error = undefined;

    // Clear existing timer
    this._clearDebounceTimer();

    // Debounce the search
    this._searchDebounceTimer = setTimeout(() => {
      this._performSearch();
    }, this._searchDebounceTimeoutMs);
  };

  private async _performSearch(): Promise<void> {
    const query = this.searchQuery.trim();

    if (!query) {
      this.searchResults = [];
      return;
    }

    this.searchLoading = true;

    try {
      const request = create(SearchContentRequestSchema, {
        query,
        frontmatterKeyIncludeFilters: ['inventory.is_container'],
        frontmatterKeysToReturnInResults: ['inventory.container', 'title'],
      });

      const response = await this.searchClient.searchContent(request);

      // Filter out the current container from results
      this.searchResults = response.results.filter(
        result => result.identifier !== this.currentContainer
      );
    } catch {
      this.searchResults = [];
      this.error = 'Search failed. Please try again.';
    } finally {
      this.searchLoading = false;
    }
  }

  private _handleCancel = (): void => {
    if (!this.movingTo) {
      this.close();
    }
  };

  public _handleMoveToClick = async (containerIdentifier: string): Promise<void> => {
    if (this.movingTo) return;

    this.movingTo = containerIdentifier;
    this.error = undefined;

    const result = await this.inventoryActionService.moveItem(
      this.itemIdentifier,
      containerIdentifier
    );

    if (result.success) {
      this.inventoryActionService.showSuccess(
        result.summary || `Moved ${this.itemIdentifier} to ${containerIdentifier}`,
        () => window.location.reload()
      );
      this.close();
    } else {
      this.error = result.error;
      this.movingTo = null;
    }
  };

  private _renderSearchResults() {
    const query = this.searchQuery.trim();

    if (!query) {
      return nothing;
    }

    if (this.searchLoading) {
      return html`
        <div class="search-results">
          <div class="search-results-header">Searching for containers...</div>
        </div>
      `;
    }

    if (this.searchResults.length === 0) {
      return html`
        <div class="search-results">
          <div class="no-results">No containers found matching "${query}"</div>
        </div>
      `;
    }

    return html`
      <div class="search-results">
        <div class="search-results-header">
          ${this.searchResults.length} container${this.searchResults.length === 1 ? '' : 's'} found
        </div>
        ${this.searchResults.map(
          result => html`
            <div class="search-result-item">
              <div class="result-info">
                <div class="result-title">${result.title || result.identifier}</div>
                ${result.frontmatter?.['inventory.container']
                  ? html`<div class="result-container">Found In: ${result.frontmatter['inventory.container']}</div>`
                  : ''}
              </div>
              <button
                class="move-to-button"
                @click=${() => this._handleMoveToClick(result.identifier)}
                ?disabled=${this.movingTo !== null}
              >
                ${this.movingTo === result.identifier ? 'Moving...' : 'Move To'}
              </button>
            </div>
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
          <h2 class="dialog-title">Move Item</h2>
        </div>

        <div class="content">
          ${this.error
            ? html`<div class="error-message">${this.error}</div>`
            : ''}

          <div class="form-group">
            <label for="itemIdentifier">Item</label>
            <input
              type="text"
              id="itemIdentifier"
              name="itemIdentifier"
              .value=${this.itemIdentifier}
              readonly
            />
            <div class="help-text">The item being moved</div>
          </div>

          <div class="form-group">
            <label for="currentContainer">Current Location</label>
            <input
              type="text"
              id="currentContainer"
              name="currentContainer"
              .value=${this.currentContainer}
              readonly
            />
            <div class="help-text">Where the item is currently stored</div>
          </div>

          <div class="move-arrow">
            <i class="fa-solid fa-arrow-down"></i>
          </div>

          <div class="form-group">
            <label for="searchQuery">Search for destination</label>
            <input
              type="text"
              id="searchQuery"
              name="searchQuery"
              .value=${this.searchQuery}
              @input=${this._handleSearchInput}
              placeholder="Type to search for containers..."
              ?disabled=${this.movingTo !== null}
            />
            <div class="help-text">Search for a container to move the item to</div>
          </div>

          ${this._renderSearchResults()}
        </div>

        <div class="footer">
          <span class="footer-hint">Select a destination above</span>
          <button
            class="button-base button-secondary button-large border-radius-small"
            @click=${this._handleCancel}
            ?disabled=${this.movingTo !== null}
          >
            Cancel
          </button>
        </div>
      </div>
    `;
  }
}

customElements.define('inventory-move-item-dialog', InventoryMoveItemDialog);
