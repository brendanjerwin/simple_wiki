import { html, css, LitElement, nothing } from 'lit';
import { sharedStyles, foundationCSS, dialogCSS, responsiveCSS, buttonCSS } from './shared-styles.js';
import { inventoryActionService } from './inventory-action-service.js';

interface FindResults {
  found: boolean;
  locations: Array<{ container: string; path: string[] }>;
  summary?: string;
}

/**
 * InventoryFindItemDialog - Modal dialog for searching inventory item locations
 *
 * Allows users to search for an item and displays where it's located,
 * including warnings for anomalies like multiple containers.
 */
export class InventoryFindItemDialog extends LitElement {
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
        max-width: 550px;
        width: 90%;
        max-height: 80vh;
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

      .search-form {
        display: flex;
        gap: 10px;
        margin-bottom: 20px;
      }

      .search-form input {
        flex: 1;
        padding: 10px 12px;
        border: 1px solid #ddd;
        border-radius: 4px;
        font-size: 14px;
      }

      .search-form input:focus {
        outline: none;
        border-color: #4a90d9;
        box-shadow: 0 0 0 2px rgba(74, 144, 217, 0.2);
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

      .results {
        margin-top: 16px;
      }

      .results-header {
        font-weight: 600;
        margin-bottom: 12px;
        color: #333;
      }

      .anomaly-warning {
        background: #fef3cd;
        border: 1px solid #ffc107;
        color: #856404;
        padding: 12px;
        border-radius: 4px;
        margin-bottom: 16px;
        font-size: 14px;
        display: flex;
        align-items: center;
        gap: 8px;
      }

      .location-list {
        list-style: none;
        padding: 0;
        margin: 0;
      }

      .location-item {
        padding: 12px;
        background: #f8f9fa;
        border: 1px solid #e9ecef;
        border-radius: 4px;
        margin-bottom: 8px;
      }

      .location-item:last-child {
        margin-bottom: 0;
      }

      .location-link {
        color: #4a90d9;
        text-decoration: none;
        font-weight: 500;
        display: block;
        margin-bottom: 4px;
      }

      .location-link:hover {
        text-decoration: underline;
      }

      .location-path {
        font-size: 12px;
        color: #666;
        font-family: monospace;
      }

      .not-found {
        text-align: center;
        padding: 24px;
        color: #666;
      }

      .not-found .not-found-icon {
        font-size: 32px;
        margin-bottom: 12px;
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
    searchQuery: { type: String },
    loading: { state: true },
    error: { state: true },
    results: { state: true },
  };

  declare open: boolean;
  declare searchQuery: string;
  declare loading: boolean;
  declare error?: string;
  declare results?: FindResults;

  constructor() {
    super();
    this.open = false;
    this.searchQuery = '';
    this.loading = false;
    this.error = undefined;
    this.results = undefined;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('keydown', this._handleKeydown);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('keydown', this._handleKeydown);
  }

  public _handleKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape' && this.open) {
      this.close();
    }
  };

  public openDialog(prefilledQuery?: string): void {
    this.searchQuery = prefilledQuery || '';
    this.error = undefined;
    this.results = undefined;
    this.loading = false;
    this.open = true;
  }

  public close(): void {
    this.open = false;
    this.searchQuery = '';
    this.error = undefined;
    this.results = undefined;
    this.loading = false;
  }

  private _handleBackdropClick = (): void => {
    this.close();
  };

  private _handleDialogClick = (event: Event): void => {
    event.stopPropagation();
  };

  private _handleSearchQueryInput = (event: Event): void => {
    const input = event.target as HTMLInputElement;
    this.searchQuery = input.value;
  };

  private _handleSearchKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Enter' && this.canSearch) {
      this._handleSearch();
    }
  };

  private _handleClose = (): void => {
    this.close();
  };

  private get canSearch(): boolean {
    return this.searchQuery.trim().length > 0 && !this.loading;
  }

  private _handleSearch = async (): Promise<void> => {
    if (!this.canSearch) return;

    this.loading = true;
    this.error = undefined;
    this.results = undefined;

    const result = await inventoryActionService.findItem(this.searchQuery.trim());

    this.loading = false;

    if (result.success) {
      this.results = {
        found: result.found ?? false,
        locations: result.locations ?? [],
        summary: result.summary,
      };
    } else {
      this.error = result.error;
    }
  };

  private _navigateToContainer(container: string): void {
    window.location.href = `/${container}`;
  }

  private _renderResults() {
    if (!this.results) return nothing;

    if (!this.results.found || this.results.locations.length === 0) {
      return html`
        <div class="not-found">
          <div class="not-found-icon"><i class="fa-solid fa-inbox"></i></div>
          <div>Item "${this.searchQuery}" has no container assignment</div>
        </div>
      `;
    }

    const isAnomaly = this.results.locations.length > 1;

    return html`
      <div class="results">
        <div class="results-header">
          Found in ${this.results.locations.length} location${this.results.locations.length > 1 ? 's' : ''}
        </div>

        ${isAnomaly ? html`
          <div class="anomaly-warning">
            <i class="fa-solid fa-triangle-exclamation"></i>
            <span>Anomaly: Item appears in multiple containers</span>
          </div>
        ` : nothing}

        <ul class="location-list">
          ${this.results.locations.map(loc => html`
            <li class="location-item">
              <a
                class="location-link"
                href="/${loc.container}"
                @click=${(e: Event) => {
                  e.preventDefault();
                  this._navigateToContainer(loc.container);
                }}
              >
                ${loc.container}
              </a>
              ${loc.path.length > 0 ? html`
                <div class="location-path">
                  ${loc.path.join(' → ')}
                </div>
              ` : nothing}
            </li>
          `)}
        </ul>
      </div>
    `;
  }

  override render() {
    return html`
      ${sharedStyles}
      <div class="backdrop" @click=${this._handleBackdropClick}></div>
      <div class="dialog system-font border-radius box-shadow" @click=${this._handleDialogClick}>
        <div class="dialog-header">
          <h2 class="dialog-title">Find Item</h2>
        </div>

        <div class="content">
          ${this.error
            ? html`<div class="error-message">${this.error}</div>`
            : nothing}

          <div class="search-form">
            <input
              type="text"
              name="searchQuery"
              .value=${this.searchQuery}
              @input=${this._handleSearchQueryInput}
              @keydown=${this._handleSearchKeydown}
              placeholder="Enter item identifier to search"
              ?disabled=${this.loading}
            />
            <button
              class="button-base button-primary button-large border-radius-small"
              @click=${this._handleSearch}
              ?disabled=${!this.canSearch}
            >
              ${this.loading ? 'Searching...' : 'Search'}
            </button>
          </div>

          ${this._renderResults()}
        </div>

        <div class="footer">
          <button
            class="button-base button-secondary button-large border-radius-small"
            @click=${this._handleClose}
          >
            Close
          </button>
        </div>
      </div>
    `;
  }
}

customElements.define('inventory-find-item-dialog', InventoryFindItemDialog);
