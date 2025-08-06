import { html, css, LitElement } from 'lit';
import { sharedStyles } from './shared-styles.js';
import './wiki-search-results.js';
import { searchClient, SearchResultWithHTML } from '../services/search-client.js';

export class WikiSearch extends LitElement {
  static override styles = css`
    div#container {
        position: relative;
        display: inline-block;
        padding: 0;
        margin: 0;
        max-width: 100%;
    }

    :host([no-results]) div#container {
        animation: shake 0.5s linear;
    }

    @keyframes shake {
        0% { transform: translate(1px, 1px) rotate(0deg); }
        10% { transform: translate(-1px, -2px) rotate(-1deg); }
        20% { transform: translate(-3px, 0px) rotate(1deg); }
        30% { transform: translate(3px, 2px) rotate(0deg); }
        40% { transform: translate(1px, -1px) rotate(1deg); }
        50% { transform: translate(-1px, 2px) rotate(-1deg); }
        60% { transform: translate(-3px, 1px) rotate(0deg); }
        70% { transform: translate(3px, 1px) rotate(-1deg); }
        80% { transform: translate(-1px, -1px) rotate(1deg); }
        90% { transform: translate(1px, 2px) rotate(0deg); }
        100% { transform: translate(1px, -2px) rotate(-1deg); }
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
    `;

  static override properties = {
    results: { type: Array },
    noResults: { type: Boolean, reflect: true, attribute: 'no-results' },
    loading: { type: Boolean },
    error: { type: String },
  };

  declare results: SearchResultWithHTML[];
  declare noResults: boolean;
  declare loading: boolean;
  declare error?: string;

  constructor() {
    super();
    this.results = [];
    this.noResults = false;
    this.loading = false;
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
    
    this.loading = true;
    
    try {
      const results = await searchClient.search(searchTerm);
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

  override render() {
    return html`
        ${sharedStyles}
        <div id="container">
            <form @submit="${this.handleFormSubmit}" action=".">
                <input type="search" name="search" placeholder="Search..." required @focus="${this.handleSearchInputFocused}">
                <button type="submit"><i class="fa-solid fa-search"></i></button>
            </form>
            <wiki-search-results 
                .results="${this.results}" 
                .open="${this.results.length > 0}" 
                @search-results-closed="${this.handleSearchResultsClosed}">
            </wiki-search-results>
        </div>
        `;
  }
}
customElements.define('wiki-search', WikiSearch);