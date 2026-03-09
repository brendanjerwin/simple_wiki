import type { TemplateResult } from 'lit';
import { html, css, LitElement } from 'lit';
import { state } from 'lit/decorators.js';
import { extractTableData } from './table-data-extractor.js';
import { sortRows, filterRows } from './table-sorter-filterer.js';
import type { ExtractedTableData } from './table-data-extractor.js';
import type { SortDirection } from './table-sorter-filterer.js';

export class WikiTable extends LitElement {
  static override styles = css`
    :host {
      display: block;
    }

    .wiki-table-toolbar {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 4px 8px;
      font-size: 0.85rem;
      color: #555;
      border-bottom: 1px solid #e0e0e0;
      flex-wrap: wrap;
    }

    .toolbar-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-height: 44px;
      min-width: 44px;
      padding: 4px 8px;
      border: 1px solid #ccc;
      border-radius: 4px;
      background: #f8f8f8;
      cursor: pointer;
      font-size: 0.85rem;
    }

    .toolbar-btn:hover {
      background: #e8e8e8;
    }

    .toolbar-btn.active {
      background: #d0e0f0;
      border-color: #80a0c0;
    }

    .row-count {
      margin-left: auto;
      white-space: nowrap;
    }

    .table-scroll-container {
      overflow-x: auto;
      -webkit-overflow-scrolling: touch;
      position: relative;
    }

    :host([scroll-middle]) .table-scroll-container::before,
    :host([scroll-end]) .table-scroll-container::before {
      content: '';
      position: absolute;
      left: 0;
      top: 0;
      bottom: 0;
      width: 16px;
      background: linear-gradient(to right, rgba(0,0,0,0.08), transparent);
      pointer-events: none;
      z-index: 3;
    }

    :host([scroll-start]) .table-scroll-container::after,
    :host([scroll-middle]) .table-scroll-container::after {
      content: '';
      position: absolute;
      right: 0;
      top: 0;
      bottom: 0;
      width: 16px;
      background: linear-gradient(to left, rgba(0,0,0,0.08), transparent);
      pointer-events: none;
      z-index: 3;
    }

    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 0.95rem;
    }

    thead th {
      position: sticky;
      top: 0;
      z-index: 2;
      background: #f5f5f5;
      padding: 8px 12px;
      text-align: left;
      border-bottom: 2px solid #ddd;
      cursor: pointer;
      user-select: none;
      white-space: nowrap;
    }

    thead th:hover {
      background: #e8e8e8;
    }

    .sort-indicator {
      margin-left: 4px;
      opacity: 0.4;
    }

    thead th.sorted .sort-indicator {
      opacity: 1;
    }

    .filter-row th {
      padding: 4px;
      position: sticky;
      top: 0;
      background: #f5f5f5;
      cursor: default;
    }

    .filter-input {
      width: 100%;
      min-height: 44px;
      padding: 4px 8px;
      border: 1px solid #ccc;
      border-radius: 4px;
      font-size: 0.85rem;
      box-sizing: border-box;
    }

    tbody td {
      padding: 8px 12px;
      border-bottom: 1px solid #eee;
    }

    tbody tr:hover {
      background: #f9f9f9;
    }

    .card-view {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    .card {
      border: 1px solid #ddd;
      border-radius: 8px;
      padding: 12px;
      background: #fff;
    }

    .card-row {
      display: flex;
      justify-content: space-between;
      padding: 4px 0;
      border-bottom: 1px solid #f0f0f0;
    }

    .card-row:last-child {
      border-bottom: none;
    }

    .card-label {
      font-weight: bold;
      font-variant: small-caps;
      font-size: 0.85rem;
      color: #666;
    }

    .card-value {
      text-align: right;
    }

    .no-results {
      padding: 16px;
      text-align: center;
      color: #888;
      font-style: italic;
    }
  `;

  @state()
  declare extractedData: ExtractedTableData | null;

  @state()
  declare sortColumnIndex: number | null;

  @state()
  declare sortDirection: SortDirection;

  @state()
  declare columnFilters: Map<number, string>;

  @state()
  declare filtersVisible: boolean;

  @state()
  declare cardViewActive: boolean;

  private _mediaQuery: MediaQueryList | null = null;
  private _scrollContainer: HTMLElement | null = null;
  private _sourceTable: HTMLTableElement | null = null;

  constructor() {
    super();
    this.extractedData = null;
    this.sortColumnIndex = null;
    this.sortDirection = 'none';
    this.columnFilters = new Map();
    this.filtersVisible = false;
    this.cardViewActive = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    this._parseSourceTable();
    this._mediaQuery = window.matchMedia('(max-width: 600px)');
    this._mediaQuery.addEventListener('change', this._handleMediaChange);
    this.cardViewActive = this._mediaQuery.matches;
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this._mediaQuery?.removeEventListener('change', this._handleMediaChange);
    this._scrollContainer?.removeEventListener('scroll', this._handleScroll);
  }

  private _handleMediaChange = (e: MediaQueryListEvent): void => {
    this.cardViewActive = e.matches;
  };

  private _handleScroll = (): void => {
    if (!this._scrollContainer) return;
    const { scrollLeft, scrollWidth, clientWidth } = this._scrollContainer;
    const atStart = scrollLeft <= 1;
    const atEnd = scrollLeft + clientWidth >= scrollWidth - 1;

    if (atStart && !atEnd) {
      this._setScrollAttribute('scroll-start');
    } else if (!atStart && !atEnd) {
      this._setScrollAttribute('scroll-middle');
    } else if (!atStart && atEnd) {
      this._setScrollAttribute('scroll-end');
    } else {
      this._removeScrollAttributes();
    }
  };

  private _setScrollAttribute(attr: string): void {
    this.removeAttribute('scroll-start');
    this.removeAttribute('scroll-middle');
    this.removeAttribute('scroll-end');
    this.setAttribute(attr, '');
  }

  private _removeScrollAttributes(): void {
    this.removeAttribute('scroll-start');
    this.removeAttribute('scroll-middle');
    this.removeAttribute('scroll-end');
  }

  private _parseSourceTable(): void {
    const table = this.querySelector('table');
    if (!table) return;
    this._sourceTable = table as HTMLTableElement;
    this._sourceTable.style.display = 'none';
    this.extractedData = extractTableData(this._sourceTable);
  }

  private _getProcessedRows() {
    if (!this.extractedData) return [];
    let rows = this.extractedData.rows;

    for (const [colIndex, filterText] of this.columnFilters) {
      if (filterText.trim() !== '') {
        const colDef = this.extractedData.columns[colIndex];
        if (colDef) {
          rows = filterRows(rows, colIndex, filterText, colDef.typeInfo.detectedType);
        }
      }
    }

    if (this.sortColumnIndex !== null && this.sortDirection !== 'none') {
      const colDef = this.extractedData.columns[this.sortColumnIndex];
      if (colDef) {
        rows = sortRows(rows, this.sortColumnIndex, this.sortDirection, colDef.typeInfo.detectedType);
      }
    }

    return rows;
  }

  private _handleHeaderClick(columnIndex: number): void {
    if (this.sortColumnIndex === columnIndex) {
      if (this.sortDirection === 'none') {
        this.sortDirection = 'ascending';
      } else if (this.sortDirection === 'ascending') {
        this.sortDirection = 'descending';
      } else {
        this.sortDirection = 'none';
        this.sortColumnIndex = null;
      }
    } else {
      this.sortColumnIndex = columnIndex;
      this.sortDirection = 'ascending';
    }
  }

  private _handleFilterInput(columnIndex: number, value: string): void {
    const newFilters = new Map(this.columnFilters);
    if (value.trim() === '') {
      newFilters.delete(columnIndex);
    } else {
      newFilters.set(columnIndex, value);
    }
    this.columnFilters = newFilters;
  }

  private _toggleFilters(): void {
    this.filtersVisible = !this.filtersVisible;
  }

  private _toggleCardView(): void {
    this.cardViewActive = !this.cardViewActive;
  }

  private _clearFilters(): void {
    this.columnFilters = new Map();
  }

  private _getSortIndicator(columnIndex: number): string {
    if (this.sortColumnIndex !== columnIndex || this.sortDirection === 'none') {
      return '\u21C5';
    }
    return this.sortDirection === 'ascending' ? '\u2191' : '\u2193';
  }

  override updated(): void {
    const container = this.shadowRoot?.querySelector<HTMLElement>('.table-scroll-container') ?? null;
    if (container && container !== this._scrollContainer) {
      this._scrollContainer?.removeEventListener('scroll', this._handleScroll);
      this._scrollContainer = container;
      this._scrollContainer.addEventListener('scroll', this._handleScroll, { passive: true });
      this._handleScroll();
    }
  }

  private _hasActiveFilters(): boolean {
    return this.columnFilters.size > 0;
  }

  override render(): TemplateResult {
    if (!this.extractedData) {
      return html`<slot></slot>`;
    }

    const processedRows = this._getProcessedRows();
    const totalRows = this.extractedData.rows.length;
    const shownRows = processedRows.length;

    return html`
      <div class="wiki-table-toolbar">
        <button
          class="toolbar-btn ${this.filtersVisible ? 'active' : ''}"
          @click=${this._toggleFilters}
          title="Toggle filters"
          aria-label="Toggle filters"
        >\u2AF7</button>
        <button
          class="toolbar-btn ${this.cardViewActive ? 'active' : ''}"
          @click=${this._toggleCardView}
          title="Toggle card view"
          aria-label="Toggle card view"
        >${this.cardViewActive ? '\u2637' : '\u2636'}</button>
        ${this._hasActiveFilters() ? html`
          <button
            class="toolbar-btn"
            @click=${this._clearFilters}
            title="Clear filters"
            aria-label="Clear filters"
          >\u2715</button>
        ` : ''}
        <span class="row-count">${shownRows === totalRows
          ? `${totalRows} rows`
          : `${shownRows} of ${totalRows} rows`}</span>
      </div>
      ${this.cardViewActive
        ? this._renderCardView(processedRows)
        : this._renderTableView(processedRows)}
      <slot style="display:none"></slot>
    `;
  }

  private _renderTableView(processedRows: ReturnType<typeof this._getProcessedRows>): TemplateResult {
    return html`
      <div class="table-scroll-container">
        <table>
          <thead>
            <tr>
              ${this.extractedData!.columns.map((col, i) => html`
                <th
                  class="${this.sortColumnIndex === i ? 'sorted' : ''}"
                  @click=${() => this._handleHeaderClick(i)}
                  aria-sort="${this.sortColumnIndex === i ? this.sortDirection : 'none'}"
                >
                  ${col.headerText}
                  <span class="sort-indicator">${this._getSortIndicator(i)}</span>
                </th>
              `)}
            </tr>
            ${this.filtersVisible ? html`
              <tr class="filter-row">
                ${this.extractedData!.columns.map((_, i) => html`
                  <th>
                    <input
                      class="filter-input"
                      type="text"
                      placeholder="Filter..."
                      .value=${this.columnFilters.get(i) ?? ''}
                      @input=${(e: InputEvent) => {
                        const target = e.target;
                        if (target instanceof HTMLInputElement) {
                          this._handleFilterInput(i, target.value);
                        }
                      }}
                    />
                  </th>
                `)}
              </tr>
            ` : ''}
          </thead>
          <tbody>
            ${processedRows.length === 0 ? html`
              <tr><td colspan="${this.extractedData!.columns.length}" class="no-results">No matching rows</td></tr>
            ` : processedRows.map(row => html`
              <tr>
                ${row.cells.map(cell => html`<td>${cell}</td>`)}
              </tr>
            `)}
          </tbody>
        </table>
      </div>
    `;
  }

  private _renderCardView(processedRows: ReturnType<typeof this._getProcessedRows>): TemplateResult {
    if (processedRows.length === 0) {
      return html`<div class="no-results">No matching rows</div>`;
    }

    return html`
      <div class="card-view">
        ${processedRows.map(row => html`
          <div class="card">
            ${this.extractedData!.columns.map((col, i) => html`
              <div class="card-row">
                <span class="card-label">${col.headerText}</span>
                <span class="card-value">${row.cells[i]}</span>
              </div>
            `)}
          </div>
        `)}
      </div>
    `;
  }
}

customElements.define('wiki-table', WikiTable);

declare global {
  interface HTMLElementTagNameMap {
    'wiki-table': WikiTable;
  }
}
