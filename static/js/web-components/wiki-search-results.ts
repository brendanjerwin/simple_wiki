import { html, css, LitElement } from 'lit';
import { unsafeHTML } from 'lit/directives/unsafe-html.js';
import { sharedStyles, sharedCSS } from './shared-styles.js';

interface SearchResult {
  Identifier: string;
  Title: string;
  FragmentHTML?: string;
}

class WikiSearchResults extends LitElement {
  static override styles = [
    sharedCSS,
    css`
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
    `
  ];

  static override properties = {
    results: { type: Array },
    open: { type: Boolean, reflect: true }
  };

  declare results: SearchResult[];
  declare open: boolean;

  private _handleClickOutside: (event: Event) => void;

  constructor() {
    super();
    this.results = [];
    this.open = false;
    this._handleClickOutside = this.handleClickOutside.bind(this);
  }

  override connectedCallback() {
    super.connectedCallback();
    document.addEventListener('click', this._handleClickOutside);
  }

  override disconnectedCallback() {
    document.removeEventListener('click', this._handleClickOutside);
    super.disconnectedCallback();
  }

  handleClickOutside(event: Event) {
    const path = (event as Event & { composedPath(): EventTarget[] }).composedPath();
    const popover = this.shadowRoot!.querySelector('.popover');
    if (this.open && popover && !path.includes(popover)) {
      this.close();
    }
  }

  close() {
    this.dispatchEvent(new CustomEvent('search-results-closed', {
      bubbles: true,
      composed: true
    }));
  }

  handlePopoverClick(event: Event) {
    // Stop the click event from bubbling up to the document
    event.stopPropagation();
  }

  override updated(changedProperties: Map<PropertyKey, unknown>) {
    if (changedProperties.has('results') && this.results.length > 0) {
      const firstLink = this.shadowRoot!.querySelector('a');
      if (firstLink) {
        firstLink.focus();
      }
    }
  }

  override render() {
    return html`
            ${sharedStyles}
            <div class="popover border-radius-large box-shadow-light" @click="${this.handlePopoverClick}">
                <div class="title-bar">
                    <h2><i class="fa-solid fa-search"></i> Search Results</h2>
                    <button class="close border-radius-small" @click="${this.close}"><i class="fa-solid fa-xmark"></i></button>
                </div>
                <div id="results">
                ${this.results.map(result => html`
                    <a href="/${result.Identifier}" class="border-radius-small">${result.Title}</a>
                    <div class="fragment border-radius-small">${unsafeHTML(result.FragmentHTML) || "N/A"}</div> 
                `)}
                </div>
            </div>
        `;
  }
}

customElements.define('wiki-search-results', WikiSearchResults);