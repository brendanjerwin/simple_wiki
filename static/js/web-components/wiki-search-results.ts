import { html, css, LitElement } from 'lit';
import { sharedStyles, foundationCSS } from './shared-styles.js';
import type { SearchResult, HighlightSpan } from '../gen/api/v1/search_pb.js';

class WikiSearchResults extends LitElement {
  static override styles = [
    foundationCSS,
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
        .filter-divider {
            color: #ccc;
            margin: 0 8px;
        }
        .inventory-filter {
            display: flex;
            align-items: center;
            gap: 4px;
            font-size: 13px;
            color: #666;
            cursor: pointer;
            white-space: nowrap;
        }
        .inventory-filter input[type="checkbox"] {
            cursor: pointer;
        }
        .item_content {
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
        .item_content br {
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
        .found-in {
            font-size: 12px;
            margin-bottom: 4px;
            color: #666;
            display: flex;
            flex-direction: row;
            align-items: baseline;
            gap: 4px;
        }
        .found-in strong {
            color: #333;
        }
        .found-in a {
            color: #0066cc;
            text-decoration: none;
            font-weight: normal;
        }
        .found-in a:hover {
            text-decoration: underline;
        }
        .no-results {
            text-align: center;
            padding: 20px;
            color: #666;
            font-style: italic;
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
    open: { type: Boolean, reflect: true },
    inventoryOnly: { type: Boolean }
  };

  declare results: SearchResult[];
  declare open: boolean;
  declare inventoryOnly: boolean;

  private _handleClickOutside: (event: Event) => void;
  private _handleKeydown: (event: KeyboardEvent) => void;

  constructor() {
    super();
    this.results = [];
    this.open = false;
    this.inventoryOnly = false;
    this._handleClickOutside = this.handleClickOutside.bind(this);
    this._handleKeydown = this.handleKeydown.bind(this);
  }

  override connectedCallback() {
    super.connectedCallback();
    document.addEventListener('click', this._handleClickOutside);
    document.addEventListener('keydown', this._handleKeydown);
  }

  override disconnectedCallback() {
    document.removeEventListener('click', this._handleClickOutside);
    document.removeEventListener('keydown', this._handleKeydown);
    super.disconnectedCallback();
  }

  handleKeydown(event: KeyboardEvent) {
    if (this.open && event.key === 'Escape') {
      event.preventDefault();
      this.close();
    }
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

  private _handleInventoryOnlyChange(event: Event) {
    const target = event.target as HTMLInputElement;
    this.inventoryOnly = target.checked;
    this.dispatchEvent(new CustomEvent('inventory-filter-changed', {
      detail: { inventoryOnly: this.inventoryOnly },
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

  /**
   * Render a fragment with highlights as HTML template
   * @param fragment - Plain text fragment
   * @param highlights - Array of highlight spans
   * @returns HTML template with marked highlights
   */
  private renderFragment(fragment: string, highlights: HighlightSpan[]) {
    if (!fragment) return html`N/A`;
    if (!highlights || highlights.length === 0) {
      // No highlights, escape and convert newlines
      return html`${this.escapeAndFormatText(fragment)}`;
    }

    // Sort highlights by start position
    const sortedHighlights = [...highlights].sort((a, b) => a.start - b.start);
    
    const parts = [];
    let lastEnd = 0;
    
    for (const highlight of sortedHighlights) {
      // Add text before the highlight
      if (highlight.start > lastEnd) {
        const beforeText = fragment.substring(lastEnd, highlight.start);
        parts.push(html`${this.escapeAndFormatText(beforeText)}`);
      }
      
      // Add the highlighted text
      const highlightedText = fragment.substring(highlight.start, highlight.end);
      parts.push(html`<mark>${this.escapeAndFormatText(highlightedText)}</mark>`);
      lastEnd = highlight.end;
    }
    
    // Add any remaining text after the last highlight
    if (lastEnd < fragment.length) {
      const afterText = fragment.substring(lastEnd);
      parts.push(html`${this.escapeAndFormatText(afterText)}`);
    }
    
    return parts;
  }

  /**
   * Escape HTML and convert newlines to line breaks
   * @param text - Text to process
   * @returns Array of template parts with line breaks
   */
  private escapeAndFormatText(text: string) {
    // Split by newlines and create template parts
    const lines = text.split('\n');
    const parts = [];
    
    for (let i = 0; i < lines.length; i++) {
      if (i > 0) {
        parts.push(html`<br>`);
      }
      parts.push(lines[i]); // Lit automatically escapes plain strings
    }
    
    return parts;
  }

  override render() {
    return html`
            ${sharedStyles}
            <div class="popover border-radius-large box-shadow-light" @click="${this.handlePopoverClick}">
                <div class="title-bar">
                    <h2><i class="fa-solid fa-search"></i> Search Results</h2>
                    <span class="filter-divider">|</span>
                    <label class="inventory-filter">
                        <input type="checkbox"
                               .checked="${this.inventoryOnly}"
                               @change="${this._handleInventoryOnlyChange}">
                        Inventory Only?
                    </label>
                    <button class="close border-radius-small" @click="${this.close}"><i class="fa-solid fa-xmark"></i></button>
                </div>
                <div id="results">
                ${this.results.length === 0
                  ? html`<div class="no-results">No results found</div>`
                  : this.results.map(result => html`
                    <a href="/${result.identifier}" class="border-radius-small">${result.title}</a>
                    <div class="item_content border-radius-small">
                        ${result.frontmatter?.['inventory.container']
                          ? html`<div class="found-in"><strong>Found In:</strong> <a href="/${result.frontmatter['inventory.container']}">${result.frontmatter['inventory.container.title'] || result.frontmatter['inventory.container']}</a></div>`
                          : ''}
                        ${this.renderFragment(result.fragment, result.highlights)}
                    </div>
                `)}
                </div>
            </div>
        `;
  }
}

customElements.define('wiki-search-results', WikiSearchResults);