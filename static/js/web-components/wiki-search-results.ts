import { html, css, LitElement } from 'lit';
import { property } from 'lit/decorators.js';
import { sharedStyles, colorCSS, foundationCSS, zIndexCSS } from './shared-styles.js';
import type { SearchResult, HighlightSpan, ContainerPathElement } from '../gen/api/v1/search_pb.js';

/**
 * Extended path element that may include an ellipsis marker for truncated paths
 */
interface DisplayPathElement extends Partial<ContainerPathElement> {
  isEllipsis?: boolean;
}

class WikiSearchResults extends LitElement {
  static override readonly styles = [
    foundationCSS,
    colorCSS,
    zIndexCSS,
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
        z-index: var(--z-popover);
        background-color: var(--color-surface-primary);
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
        margin: 10px;
        text-decoration: none;
        font-weight: bold;
        transition: background-color 0.3s ease;
        cursor: pointer;
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
        }
        .popover:not(:hover) a:focus {
            outline: 2px solid var(--color-border-focus);
        }
        a:hover {
            outline: 2px solid var(--color-border-focus);
        }
        .title-bar {
            display: flex;
            justify-content: space-between;
            align-items: center;
            border-top-right-radius: 10px;
            border-top-left-radius: 10px;
            background-color: var(--color-surface-sunken);
            padding: 10px;
            border-bottom: 1px solid var(--color-border-subtle);
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
            color: var(--color-border-default);
            margin: 0 8px;
        }
        .inventory-filter {
            display: flex;
            align-items: center;
            gap: 4px;
            font-size: 13px;
            color: var(--color-text-secondary);
            cursor: pointer;
            white-space: nowrap;
        }
        .inventory-filter input[type="checkbox"] {
            cursor: pointer;
        }
        .item_content {
            background-color: var(--color-surface-elevated);
            font-size: 12px;
            margin: 10px;
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
            background-color: var(--color-highlight-bg);
            color: var(--color-text-primary);
            font-weight: bold;
            border-radius: 4px;
            padding: 2px 3px;
        }
        .found-in {
            font-size: 12px;
            margin-bottom: 4px;
            color: var(--color-text-secondary);
            display: flex;
            flex-direction: row;
            align-items: baseline;
            gap: 4px;
            flex-wrap: wrap;
        }
        .found-in strong {
            color: var(--color-text-primary);
        }
        .found-in a {
            color: var(--color-text-link);
            text-decoration: none;
            font-weight: normal;
        }
        .found-in a:hover {
            text-decoration: underline;
        }
        .path-separator {
            color: var(--color-text-muted);
            margin: 0 2px;
        }
        .path-ellipsis {
            color: var(--color-text-muted);
            font-weight: normal;
            user-select: none;
        }
        .no-results {
            text-align: center;
            padding: 20px;
            color: var(--color-text-secondary);
            font-style: italic;
        }
        .filter-warning {
            background-color: var(--color-warning-bg);
            border: 1px solid var(--color-warning);
            border-radius: 5px;
            padding: 8px 10px;
            margin: 10px;
            font-size: 13px;
            color: var(--color-warning-text);
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .filter-warning i {
            color: var(--color-warning);
        }

        @media (max-width: 410px) {
            div#results {
                width: 97%;
            }
        }
    `
  ];

  @property({ type: Array })
  declare results: SearchResult[];

  @property({ type: Boolean, reflect: true })
  declare open: boolean;

  @property({ type: Boolean })
  declare inventoryOnly: boolean;

  @property({ type: Number })
  declare totalUnfilteredCount: number;

  public readonly _handleClickOutside = (event: Event): void => {
    const path = (event as Event & { composedPath(): EventTarget[] }).composedPath();
    const popover = this.shadowRoot?.querySelector('.popover');
    if (this.open && popover && !path.includes(popover)) {
      this.close();
    }
  };

  public readonly _handleKeydown = (event: KeyboardEvent): void => {
    if (!this.open) return;

    if (event.key === 'Escape') {
      event.preventDefault();
      this.close();
      return;
    }

    if (event.key === 'Tab') {
      this._trapFocus(event);
    }
  };

  private _trapFocus(event: KeyboardEvent): void {
    const popover = this.shadowRoot?.querySelector('.popover');
    if (!popover) return;

    const focusableSelectors = [
      'a[href]',
      'button:not([disabled])',
      'input:not([disabled])',
      '[tabindex]:not([tabindex="-1"])',
    ].join(', ');

    const focusableElements = Array.from(
      popover.querySelectorAll<HTMLElement>(focusableSelectors)
    );

    if (focusableElements.length === 0) return;

    const firstFocusable = focusableElements[0]!;
    const lastFocusable = focusableElements.at(-1)!;
    const activeEl = this.shadowRoot?.activeElement;

    if (event.shiftKey) {
      if (activeEl === firstFocusable) {
        event.preventDefault();
        lastFocusable.focus();
      }
    } else if (activeEl === lastFocusable) {
      event.preventDefault();
      firstFocusable.focus();
    }
  }

  constructor() {
    super();
    this.results = [];
    this.open = false;
    this.inventoryOnly = false;
    this.totalUnfilteredCount = 0;
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

  close() {
    this.dispatchEvent(new CustomEvent('search-results-closed', {
      bubbles: true,
      composed: true
    }));
  }

  private _handleInventoryOnlyChange(event: Event) {
    const target = event.target;
    if (!(target instanceof HTMLInputElement)) {
      return;
    }
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
      const firstLink = this.shadowRoot?.querySelector('a');
      if (firstLink) {
        firstLink.focus();
      }
    }
  }

  /**
   * Process container path for display, sorting by depth and truncating if too long.
   * Keeps the last (deepest) items which are most useful, replacing early items with "...".
   * @param path - Array of container path elements
   * @returns Processed path ready for rendering
   */
  private processContainerPath(path: ContainerPathElement[]): DisplayPathElement[] {
    if (!path || path.length === 0) return [];

    // Sort by depth to ensure correct ordering
    const sorted = [...path].sort((a, b) => (a.depth || 0) - (b.depth || 0));

    const maxVisible = 4;
    if (sorted.length <= maxVisible) {
      return sorted;
    }

    // Too many items - keep the last (deepest) ones and add ellipsis
    const numToShow = maxVisible - 1; // Reserve one slot for "..."
    const visibleItems = sorted.slice(-numToShow);

    // Add ellipsis marker at the beginning
    return [{ isEllipsis: true }, ...visibleItems];
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
    const hiddenResultsCount = this.inventoryOnly
      ? Math.max(0, this.totalUnfilteredCount - this.results.length)
      : 0;
    const hiddenResultsSuffix = hiddenResultsCount === 1 ? '' : 's';

    return html`
            ${sharedStyles}
            <div class="popover border-radius-large box-shadow-light"
                 role="dialog"
                 aria-modal="true"
                 aria-label="Search Results"
                 @click="${this.handlePopoverClick}">
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
                ${hiddenResultsCount > 0 ? html`
                    <div class="filter-warning">
                        <i class="fa-solid fa-triangle-exclamation"></i>
                        <span>${hiddenResultsCount} other result${hiddenResultsSuffix} not shown.</span>
                    </div>
                ` : ''}
                <div id="results">
                ${this.results.length === 0
                  ? html`<div class="no-results">No results found</div>`
                  : this.results.map(result => {
                    const inventoryPathContent = (result.inventoryContext?.path && result.inventoryContext.path.length > 0)
                      ? this.processContainerPath(result.inventoryContext.path).map((element, index) => html`
                          ${index > 0 ? html`<span class="path-separator">›</span>` : ''}
                          ${element.isEllipsis
                            ? html`<span class="path-ellipsis">...</span>`
                            : html`<a href="/${element.identifier}">${element.title || element.identifier}</a>`
                          }
                        `)
                      : '';
                    return html`
                    <a href="/${result.identifier}" class="border-radius-small">${result.title}</a>
                    <div class="item_content border-radius-small">
                        ${result.inventoryContext?.isInventoryRelated
                          ? html`<div class="found-in">
                              <strong>In:</strong>
                              ${inventoryPathContent}
                            </div>`
                          : ''}
                        ${this.renderFragment(result.fragment, result.highlights)}
                    </div>
                  `;
                  })}
                </div>
            </div>
        `;
  }
}

customElements.define('wiki-search-results', WikiSearchResults);

declare global {
  interface HTMLElementTagNameMap {
    'wiki-search-results': WikiSearchResults;
  }
}