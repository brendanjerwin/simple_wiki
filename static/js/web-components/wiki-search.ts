import { html, css, LitElement } from 'lit';
import { sharedStyles } from './shared-styles.js';
import './wiki-search-results.js';

interface SearchResult {
  Identifier: string;
  Title: string;
  FragmentHTML?: string;
}

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
    searchEndpoint: { type: String, attribute: 'search-endpoint' },
    resultArrayPath: { type: String, attribute: 'result-array-path' },
    results: { type: Array },
    noResults: { type: Boolean, reflect: true, attribute: 'no-results' },
  };

  declare searchEndpoint?: string;
  declare resultArrayPath?: string;
  declare results: SearchResult[];
  declare noResults: boolean;

  constructor() {
    super();
    this.resultArrayPath = "results";
    this.results = [];
    this.noResults = false;
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

  handleFormSubmit(e: Event) {
    e.preventDefault();
    this.noResults = false;

    const form = e.target as HTMLFormElement;
    const formData = new FormData(form);
    const searchTerm = formData.get('search') as string;
    
    if (!this.searchEndpoint) {
      console.error('Search endpoint not configured');
      return;
    }
    
    const url = `${this.searchEndpoint}?q=${encodeURIComponent(searchTerm)}`;

    fetch(url)
      .then((response) => response.json())
      .then((data) => {
        if (this.resultArrayPath) {
          data = this.getNestedProperty(data, this.resultArrayPath);
          if (!Array.isArray(data)) {
            data = [];
          }
        }
        this.results = [...data];
        if (data.length > 0) {
          this.noResults = false;
        } else {
          this.noResults = true;
          const searchInput = this.shadowRoot!.querySelector('input[type="search"]') as HTMLInputElement;
          searchInput.select();
        }
      })
      .catch((error) => {
        this.results = [];
        console.error('Error:', error);
      });
  }

  getNestedProperty(obj: unknown, path: string): unknown {
    return path.split('.').reduce((o: unknown, p: string) => (o && typeof o === 'object' && p in o) ? (o as Record<string, unknown>)[p] : null, obj);
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