import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { sharedStyles, dialogStyles } from './shared-styles.js';
import { InventoryItemCreatorMover } from './inventory-item-creator-mover.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import { SearchService, SearchContentRequestSchema, type SearchResult } from '../gen/api/v1/search_pb.js';
import './inventory-qr-scanner.js';
import type { ItemScannedEventDetail, ScannedItemInfo, InventoryQrScanner } from './inventory-qr-scanner.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import './error-display.js';

/**
 * Information about a scanned container result (alias for ScannedItemInfo)
 */
export type ScannedResultInfo = ScannedItemInfo;

/**
 * InventoryMoveItemDialog - Modal dialog for moving inventory items between containers
 *
 * Search-based destination selection: user types to search for containers,
 * results appear as "Move To" buttons that execute the move on click.
 */
export class InventoryMoveItemDialog extends LitElement {
  static override readonly styles = dialogStyles(css`
      :host {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        z-index: var(--z-modal);
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
        background: var(--color-surface-elevated);
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

      .search-results {
        margin-top: 8px;
        border: 1px solid var(--color-border-subtle);
        border-radius: 4px;
        max-height: 250px;
        overflow-y: auto;
      }

      .search-results-header {
        padding: 8px 12px;
        background: var(--color-surface-sunken);
        border-bottom: 1px solid var(--color-border-subtle);
        font-size: 12px;
        font-weight: 500;
        color: var(--color-text-secondary);
      }

      .search-result-item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 10px 12px;
        border-bottom: 1px solid var(--color-border-subtle);
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
        color: var(--color-text-primary);
        margin-bottom: 2px;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }

      .result-container {
        font-size: 12px;
        color: var(--color-text-secondary);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }

      .move-to-button {
        padding: 6px 12px;
        border: none;
        border-radius: 4px;
        background: var(--color-action-confirm);
        color: var(--color-text-inverse);
        font-size: 13px;
        font-weight: 500;
        cursor: pointer;
        white-space: nowrap;
        transition: background-color 0.15s;
      }

      .move-to-button:hover:not(:disabled) {
        background: var(--color-action-confirm-hover);
      }

      .move-to-button:disabled {
        background: var(--color-action-primary);
        cursor: not-allowed;
      }

      .no-results {
        padding: 16px 12px;
        text-align: center;
        color: var(--color-text-secondary);
        font-size: 14px;
      }

      .footer {
        display: flex;
        gap: 12px;
        padding: 16px 20px;
        border-top: 1px solid var(--color-border-subtle);
        justify-content: flex-end;
      }

      .footer-hint {
        flex: 1;
        font-size: 13px;
        color: var(--color-text-secondary);
        display: flex;
        align-items: center;
      }

      .search-row {
        display: flex;
        gap: 8px;
        align-items: stretch;
      }

      .search-row input {
        flex: 1;
      }

      .qr-scan-button {
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 0 12px;
        background: var(--color-surface-sunken);
        border: 1px solid var(--color-border-default);
        border-radius: 4px;
        cursor: pointer;
        color: var(--color-text-primary);
        font-size: 16px;
        transition: background-color 0.15s;
      }

      .qr-scan-button:hover:not(:disabled) {
        background: var(--color-hover-overlay);
      }

      .qr-scan-button:disabled {
        cursor: not-allowed;
        opacity: 0.6;
      }

      .scanned-result {
        margin-top: 12px;
        border: 2px solid var(--color-success);
        border-radius: 4px;
        background: var(--color-success-bg);
      }

      .scanned-result-header {
        padding: 8px 12px;
        background: var(--color-success-bg);
        border-bottom: 1px solid var(--color-success);
        font-size: 12px;
        font-weight: 500;
        color: var(--color-success-text);
        display: flex;
        align-items: center;
        gap: 6px;
      }

      .scanned-result-item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 10px 12px;
        gap: 12px;
      }

    `
  );

  @property({ type: Boolean, reflect: true })
  declare open: boolean;

  @property({ type: String })
  declare itemIdentifier: string;

  @property({ type: String })
  declare currentContainer: string;

  @property({ type: String })
  declare searchQuery: string;

  @state()
  declare searchResults: SearchResult[];

  @state()
  declare searchLoading: boolean;

  @state()
  declare movingTo: string | null;

  @state()
  declare error: AugmentedError | null;

  @state()
  declare scannerMode: boolean;

  @state()
  declare scannedDestination: string | null;

  @state()
  declare scannedResult: ScannedResultInfo | null;

  @state()
  declare scanError: AugmentedError | null;

  private readonly _searchDebounceTimeoutMs = 300;
  private _searchDebounceTimer?: ReturnType<typeof setTimeout>;
  private readonly searchClient = createClient(SearchService, getGrpcWebTransport());
  private readonly inventoryItemCreatorMover = new InventoryItemCreatorMover();

  constructor() {
    super();
    this.open = false;
    this.itemIdentifier = '';
    this.currentContainer = '';
    this.searchQuery = '';
    this.searchResults = [];
    this.searchLoading = false;
    this.movingTo = null;
    this.error = null;
    // QR scanner state
    this.scannerMode = false;
    this.scannedDestination = null;
    this.scannedResult = null;
    this.scanError = null;
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
      delete this._searchDebounceTimer;
    }
  }

  private readonly _handleKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape' && this.open) {
      if (this.scannerMode) {
        this._exitScannerMode();
      } else {
        this.close();
      }
    }
  };

  public openDialog(itemIdentifier: string, currentContainer: string): void {
    this.itemIdentifier = itemIdentifier;
    this.currentContainer = currentContainer;
    this.searchQuery = '';
    this.searchResults = [];
    this.searchLoading = false;
    this.movingTo = null;
    this.error = null;
    // Reset QR scanner state
    this.scannerMode = false;
    this.scannedDestination = null;
    this.scannedResult = null;
    this.scanError = null;
    this.open = true;

    // Focus search field after render
    this.updateComplete.then(() => {
      const searchField = this.shadowRoot?.querySelector<HTMLInputElement>('input[name="searchQuery"]');
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
    this.error = null;
    // Reset QR scanner state
    this.scannerMode = false;
    this.scannedDestination = null;
    this.scannedResult = null;
    this.scanError = null;
  }

  private readonly _handleBackdropClick = (): void => {
    if (!this.movingTo) {
      this.close();
    }
  };

  private readonly _handleDialogClick = (event: Event): void => {
    event.stopPropagation();
  };

  private readonly _handleSearchInput = (event: Event): void => {
    if (!(event.target instanceof HTMLInputElement)) {
      return;
    }
    const input = event.target;
    this.searchQuery = input.value;
    this.error = null;
    // Clear scan state when user starts typing - they're switching to search mode
    this.scanError = null;
    this.scannedDestination = null;
    this.scannedResult = null;

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
    } catch (err) {
      this.searchResults = [];
      this.error = AugmentErrorService.augmentError(err, 'search containers');
    } finally {
      this.searchLoading = false;
    }
  }

  private readonly _handleCancel = (): void => {
    if (!this.movingTo) {
      this.close();
    }
  };

  /**
   * Handle item scanned from inventory-qr-scanner
   */
  private readonly _handleItemScanned = (event: CustomEvent<ItemScannedEventDetail>): void => {
    const { item } = event.detail;

    // Clear any previous scan state
    this.scanError = null;
    this.scannedDestination = null;
    this.scannedResult = null;

    // Exit scanner mode first
    this._exitScannerMode();

    // Validate: is it a container?
    if (!item.isContainer) {
      this.scanError = AugmentErrorService.augmentError(
        new Error(`"${item.identifier}" is not marked as a container`),
        'scan item'
      );
      return;
    }

    // Validate: not the current container?
    if (item.identifier === this.currentContainer) {
      this.scanError = AugmentErrorService.augmentError(
        new Error('Cannot move to current location'),
        'scan item'
      );
      return;
    }

    // Success! Set the scanned result
    this.scannedDestination = item.identifier;
    this.scannedResult = item;
  };

  /**
   * Handle cancelled event from inventory-qr-scanner
   */
  private readonly _handleScannerCancelled = (): void => {
    this._exitScannerMode();
  };

  /**
   * Clear the scanned result
   */
  private readonly _clearScannedResult = (): void => {
    this.scannedDestination = null;
    this.scannedResult = null;
    this.scanError = null;
  };

  /**
   * Enter scanner mode - replaces search UI with scanner component
   */
  private readonly _enterScannerMode = (): void => {
    this.scannerMode = true;
    this._clearScannedResult();
    // Clear search state when switching to scanner mode
    this.searchQuery = '';
    this.searchResults = [];
    // Wait for DOM update, then expand the scanner
    this.updateComplete.then(() => {
      const scanner = this.shadowRoot?.querySelector<InventoryQrScanner>('inventory-qr-scanner');
      if (scanner) {
        scanner.expand();
      }
    });
  };

  /**
   * Exit scanner mode - returns to search UI
   */
  private readonly _exitScannerMode = (): void => {
    this.scannerMode = false;
  };

  /**
   * Handle "Scan Again" button click
   */
  private readonly _handleScanAgain = (): void => {
    this.scanError = null;
    // Re-enter scanner mode
    this._enterScannerMode();
  };

  private readonly _handleMoveToClick = async (containerIdentifier: string): Promise<void> => {
    if (this.movingTo) return;

    this.movingTo = containerIdentifier;
    this.error = null;

    const result = await this.inventoryItemCreatorMover.moveItem(
      this.itemIdentifier,
      containerIdentifier
    );

    if (result.success) {
      this.dispatchEvent(new CustomEvent('item-moved', {
        detail: { itemIdentifier: this.itemIdentifier, containerIdentifier },
        bubbles: true,
        composed: true,
      }));
      this.inventoryItemCreatorMover.showSuccess(
        result.summary || `Moved ${this.itemIdentifier} to ${containerIdentifier}`,
        () => globalThis.location.reload()
      );
      this.close();
    } else {
      this.error = result.error ? AugmentErrorService.augmentError(result.error, 'move item') : null;
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
                ${result.frontmatter['inventory.container']
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

  private _renderScannedResult() {
    if (!this.scannedResult) {
      return nothing;
    }

    const scannedResult = this.scannedResult;

    return html`
      <div class="scanned-result">
        <div class="scanned-result-header">
          <i class="fa-solid fa-qrcode"></i>
          Scanned Destination
        </div>
        <div class="scanned-result-item">
          <div class="result-info">
            <div class="result-title">${this.scannedResult.title}</div>
            ${this.scannedResult.container
              ? html`<div class="result-container">Found In: ${this.scannedResult.container}</div>`
              : ''}
          </div>
          <button
            class="move-to-button"
            @click=${() => this._handleMoveToClick(scannedResult.identifier)}
            ?disabled=${this.movingTo !== null}
          >
            ${this.movingTo === this.scannedResult.identifier ? 'Moving...' : 'Move To'}
          </button>
        </div>
      </div>
    `;
  }

  private _renderScanError() {
    if (!this.scanError) {
      return nothing;
    }

    return html`
      <error-display
        .augmentedError=${this.scanError}
        .action=${{ label: 'Scan Again', onClick: () => this._handleScanAgain() }}
      ></error-display>
    `;
  }

  override render() {
    return html`
      ${sharedStyles}
      <div class="backdrop" @click=${this._handleBackdropClick}></div>
      <div class="dialog system-font border-radius box-shadow" @click=${this._handleDialogClick}>
        <div class="dialog-header">
          <h2 class="dialog-title">Move Item: ${this.itemIdentifier}</h2>
        </div>

        <div class="content">
          ${this.error
            ? html`<error-display
                .augmentedError=${this.error}
                .action=${{ label: 'Dismiss', onClick: () => { this.error = null; } }}
              ></error-display>`
            : ''}

          <div class="form-group">
            <label for="searchQuery">Destination</label>
            ${this.scannerMode
              ? html`
                  <inventory-qr-scanner
                    @item-scanned=${this._handleItemScanned}
                    @cancelled=${this._handleScannerCancelled}
                  ></inventory-qr-scanner>
                `
              : html`
                  <div class="search-row">
                    <input
                      type="text"
                      id="searchQuery"
                      name="searchQuery"
                      .value=${this.searchQuery}
                      @input=${this._handleSearchInput}
                      placeholder="Type to search for containers..."
                      ?disabled=${this.movingTo !== null}
                    />
                    <button
                      class="qr-scan-button"
                      @click=${this._enterScannerMode}
                      ?disabled=${this.movingTo !== null}
                      title="Scan QR code"
                      aria-label="Scan QR code to select destination"
                    >
                      <i class="fa-solid fa-qrcode"></i>
                    </button>
                  </div>
                  ${this._renderScanError()}
                  <div class="help-text">Search or scan a QR code to find a container</div>
                `
            }
          </div>

          ${this._renderScannedResult()}
          ${this.scannedResult ? nothing : this._renderSearchResults()}
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

declare global {
  interface HTMLElementTagNameMap {
    'inventory-move-item-dialog': InventoryMoveItemDialog;
  }
}
