import { html, css, LitElement, unsafeHTML } from 'lit';

class WikiSearchResults extends LitElement {
  static styles = css`
        :host {
            display: block;
            position: relative;
        }
        .popover {
            flex-direction: column;
            display: none;
            position: fixed;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            max-height: 95%;
            width: 400px;
            max-width: 97%;
            border-radius: 10px;
            box-shadow: 0px 5px 15px rgba(0, 0, 0, 0.3);
            z-index: 9999;
            background-color: white;
        }
        :host([open]) .popover {
            display: flex;
        }
        div#results {
            max-height: 100%;
            overflow-y: auto;
        }

        a {
            display: block;
            margin: 5px;
            text-decoration: none;
            font-weight: bold;
            border-radius: 5px;
            transition: background-color 0.3s ease;
            cursor: pointer;
            overflow: hidden;
            white-space: nowrap;
            text-overflow: ellipsis;
        }
        .popover:not(:hover) a:focus {
            outline: 2px solid #4d90fe;
        }
        a:hover {
            outline: 2px solid #4d90fe;
        }
        .title-bar {
            display: flex;
            justify-content: space-between;
            align-items: center;
            border-top-right-radius: 10px;
            border-top-left-radius: 10px;
            background-color: #f8f8f8;
            padding: 10px;
            border-bottom: 1px solid #e8e8e8;
        }
        .title-bar h2 {
            font-size: 16px;
            margin: 0;
        }
        .title-bar button {
            border: none;
            background-color: transparent;
            cursor: pointer;
            font-size: 16px;
            padding: 0;
        }
        .fragment {
            background-color: #e8e8e8;
            font-size: 12px;
            margin: 5px;
            margin-bottom: 10px;
            padding: 5px;
            width: auto; 
            max-height: 500px; 
            overflow: hidden;
            border-radius: 5px;
        }
        .fragment br {
            display: block;
            content: "";
            margin-top: 2px;
        }
        mark {
            background-color: #ffff00;
            color: black;
            font-weight: bold;
            border-radius: 4px;
            padding: 2px 3px;
        }

        @media (max-width: 410px) {
            div#results {
                width: 97%;
            }
        }
    `;

  static properties = {
    results: { type: Array },
    open: { type: Boolean, reflect: true }
  };

  constructor() {
    super();
    this.results = [];
    this.open = false;
  }

  connectedCallback() {
    super.connectedCallback();
    document.addEventListener('click', this.handleClickOutside.bind(this));
  }

  disconnectedCallback() {
    document.removeEventListener('click', this.handleClickOutside.bind(this));
    super.disconnectedCallback();
  }

  handleClickOutside(event) {
    const path = event.composedPath();
    if (this.open && !path.includes(this.shadowRoot.querySelector('.popover'))) {
      this.close();
    }
  }

  close() {
    this.dispatchEvent(new CustomEvent('search-results-closed', {
      bubbles: true,
      composed: true
    }));
  }

  handlePopoverClick(event) {
    // Stop the click event from bubbling up to the document
    event.stopPropagation();
  }

  updated(changedProperties) {
    if (changedProperties.has('results') && this.results.length > 0) {
      const firstLink = this.shadowRoot.querySelector('a');
      if (firstLink) {
        firstLink.focus();
      }
    }
  }

  render() {
    return html`
            <link href="/static/css/fontawesome.min.css" rel="stylesheet">
            <link href="/static/css/solid.min.css" rel="stylesheet">
            <div class="popover" @click="${this.handlePopoverClick}">
                <div class="title-bar">
                    <h2><i class="fa-solid fa-search"></i> Search Results</h2>
                    <button class="close" @click="${this.close}"><i class="fa-solid fa-xmark"></i></button>
                </div>
                <div id="results">
                ${this.results.map(result => html`
                    <a href="/${result.Identifier}">${result.Title}</a>
                    <div class="fragment">${unsafeHTML(result.FragmentHTML) || "N/A"}</div> 
                `)}
                </div>
            </div>
        `;
  }
}

customElements.define('wiki-search-results', WikiSearchResults);
