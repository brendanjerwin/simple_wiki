import { html, css, LitElement } from 'lit';
import { createPromiseClient, PromiseClient } from '@connectrpc/connect';
import { createGrpcWebTransport } from '@connectrpc/connect-web';
import { sharedStyles } from './shared-styles.js';
import './wiki-search-results.js';
import { SearchService } from '../gen/api/v1/search_connect.js';
import type { SearchContentRequest, SearchResult } from '../gen/api/v1/search_pb.js';

const INVENTORY_ONLY_STORAGE_KEY = 'wiki-search-inventory-only';

export class WikiSearch extends LitElement {
  private client: PromiseClient<typeof SearchService> | null = null;

  private getClient(): PromiseClient<typeof SearchService> {
    if (!this.client) {
      this.client = createPromiseClient(SearchService, createGrpcWebTransport({
        baseUrl: window.location.origin,
      }));
    }
    return this.client;
  }

  // Method that can be stubbed in tests to prevent network calls
  async performSearch(query: string): Promise<SearchResult[]> {
    const request: Partial<SearchContentRequest> = {
      query,
      frontmatterKeysToReturnInResults: ['inventory.container'],
    };

    if (this.inventoryOnly) {
      request.frontmatterKeyIncludeFilters = ['inventory.container'];
      request.frontmatterKeyExcludeFilters = ['inventory.is_container'];
    }

    const response = await this.getClient().searchContent(request);
    return response.results;
  }
  static override styles = css`
    div#container {
        position: relative;
        display: inline-block;
        padding: 0;
        margin: 0;
        max-width: 100%;
    }

    form { 
        display: flex;
        justify-content: center;
        padding: 1px;
        width: 100%;
        max-width: 500px;
        box-sizing: border-box;
    }

    input[type="search"] {
        flex-grow: 1 1 auto;
        padding: 5px;
        border: none;
        border-radius: 5px 0 0 5px;
        outline: none;
        font-size: 16px;
        max-width: 100%;
        background-color: white;
    }

    input[type="search"]:focus {
        animation: pulse .8s 1;
    }

    @keyframes pulse {
        0% { background-color: white; }
        25% { background-color: #ffff00; }
        100% { background-color: white; }
    }

    button {
        padding: 5px 15px;
        border: none;
        background-color: #6c757d;
        color: white;
        cursor: pointer;
        border-radius: 0 5px 5px 0;
        font-size: 16px;
        transition: background-color 0.3s ease;
    }
    button:hover {
        background-color: #9da5ab;
    }

    .error {
        color: #721c24;
        background-color: #f8d7da;
        border: 1px solid #f5c6cb;
        padding: 10px;
        margin: 10px 0;
        border-radius: 5px;
        text-align: center;
    }
    `;

  static override properties = {
    results: { type: Array },
    noResults: { type: Boolean, reflect: true, attribute: 'no-results' },
    loading: { type: Boolean },
    error: { type: String },
    inventoryOnly: { type: Boolean },
  };

  declare results: SearchResult[];
  declare noResults: boolean;
  declare loading: boolean;
  declare error?: string;
  declare inventoryOnly: boolean;
  private lastSearchQuery: string = '';

  constructor() {
    super();
    this.results = [];
    this.noResults = false;
    this.loading = false;
    this.inventoryOnly = localStorage.getItem(INVENTORY_ONLY_STORAGE_KEY) === 'true';
    this._handleKeydown = this._handleKeydown.bind(this);
  }

  override connectedCallback() {
    super.connectedCallback();
    window.addEventListener('keydown', this._handleKeydown);
  }

  override disconnectedCallback() {
    super.disconnectedCallback();
    window.removeEventListener('keydown', this._handleKeydown);
  }

  private _handleKeydown(e: KeyboardEvent) {
    const searchInput = this.shadowRoot!.querySelector('input[type="search"]') as HTMLInputElement;
    // Check if Ctrl (or Cmd on Macs) and K keys were pressed
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
      e.preventDefault();
      searchInput.focus();
    }
  }

  handleSearchInputFocused(e: Event) {
    const target = e.target as HTMLInputElement;
    target.select();
  }

  async handleFormSubmit(e: Event) {
    e.preventDefault();
    this.noResults = false;
    this.error = undefined;

    const form = e.target as HTMLFormElement;
    const formData = new FormData(form);
    const searchTerm = formData.get('search') as string;

    if (!searchTerm || searchTerm.trim() === '') {
      return;
    }

    this.lastSearchQuery = searchTerm;
    this.loading = true;

    try {
      const results = await this.performSearch(searchTerm);
      this.results = [...results];

      if (results.length > 0) {
        this.noResults = false;
      } else {
        this.noResults = true;
        const searchInput = this.shadowRoot!.querySelector('input[type="search"]') as HTMLInputElement;
        searchInput.select();
      }
    } catch (error) {
      this.results = [];
      this.error = error instanceof Error ? error.message : 'Search failed';
      console.error('Search error:', error);
    } finally {
      this.loading = false;
    }
  }

  handleSearchResultsClosed() {
    this.results = [];
  }

  async handleInventoryFilterChanged(e: CustomEvent<{ inventoryOnly: boolean }>) {
    this.inventoryOnly = e.detail.inventoryOnly;
    localStorage.setItem(INVENTORY_ONLY_STORAGE_KEY, String(this.inventoryOnly));

    // Re-run the search with the new filter if we have a previous query
    if (this.lastSearchQuery) {
      this.loading = true;
      this.error = undefined;

      try {
        const results = await this.performSearch(this.lastSearchQuery);
        this.results = [...results];
        this.noResults = results.length === 0;
      } catch (error) {
        this.results = [];
        this.error = error instanceof Error ? error.message : 'Search failed';
        console.error('Search error:', error);
      } finally {
        this.loading = false;
      }
    }
  }

  override render() {
    return html`
        ${sharedStyles}
        <div id="container">
            <form @submit="${this.handleFormSubmit}" action=".">
                <input type="search" name="search" placeholder="Search..." required @focus="${this.handleSearchInputFocused}">
                <button type="submit"><i class="fa-solid fa-search"></i></button>
            </form>
            ${this.error ? html`<div class="error">${this.error}</div>` : ''}
            <wiki-search-results
                .results="${this.results}"
                .open="${this.results.length > 0 || this.noResults}"
                .inventoryOnly="${this.inventoryOnly}"
                @search-results-closed="${this.handleSearchResultsClosed}"
                @inventory-filter-changed="${this.handleInventoryFilterChanged}">
            </wiki-search-results>
        </div>
        `;
  }
}
customElements.define('wiki-search', WikiSearch);