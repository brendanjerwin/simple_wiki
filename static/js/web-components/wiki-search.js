import { html, css, LitElement } from 'lit';

export class WikiSearch extends LitElement {
  static styles = css`
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

  static properties = {
    searchEndpoint: { type: String, attribute: 'search-endpoint' },
    resultArrayPath: { type: String, attribute: 'result-array-path' },
    results: { type: Array },
    noResults: { type: Boolean, reflect: true, attribute: 'no-results' },
  };

  constructor() {
    super();
    this.resultArrayPath = "results";
    this.results = [];
    this._handleKeydown = this._handleKeydown.bind(this);
  }

  connectedCallback() {
    super.connectedCallback();

    window.addEventListener('keydown', this._handleKeydown);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    window.removeEventListener('keydown', this._handleKeydown);
  }

  _handleKeydown(e) {
    const searchInput = this.shadowRoot.querySelector('input[type="search"]');
    // Check if Ctrl (or Cmd on Macs) and K keys were pressed
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
      e.preventDefault();

      searchInput.focus();
    }
  }

  handleSearchInputFocused(e) {
    e.target.select();
  }

  handleFormSubmit(e) {
    e.preventDefault();
    this.noResults = false;

    const form = e.target;
    const searchTerm = form.search.value;
    const url = `${this.searchEndpoint}?q=${searchTerm}`;

    fetch(url)
      .then((response) => response.json())
      .then((data) => {
        if (this.resultArrayPath) {
          data = this.getNestedProperty(data, this.resultArrayPath);
          if (!Array.isArray(data)) {
            data = [];
          }
        }
        this.results = data;
        if (data.length > 0) {
          this.noResults = false;
        } else {
          this.noResults = true;
          const searchInput = this.shadowRoot.querySelector('input[type="search"]');
          searchInput.select();
        }
      })
      .catch((error) => {
        this.results = [];
        console.error('Error:', error);
      });
  }


  getNestedProperty(obj, path) {
    return path.split('.').reduce((o, p) => (o && o[p]) ? o[p] : null, obj);
  }

  handleSearchResultsClosed() {
    this.results = [];
  }

  render() {
    return html`
        <link href="/static/css/fontawesome.min.css" rel="stylesheet">
        <link href="/static/css/solid.min.css" rel="stylesheet">
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