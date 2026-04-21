import type { TemplateResult } from 'lit';
import { html, css, LitElement, nothing } from 'lit';
import { state } from 'lit/decorators.js';
import { unsafeHTML } from 'lit/directives/unsafe-html.js';
import DOMPurify from 'dompurify';
import { extractTableData, getUniqueColumnValues, getColumnNumericRange } from './table-data-extractor.js';
import { sortRows, applyAllFilters, hasActiveFilters, isFilterActive } from './table-sorter-filterer.js';
import { computeTableHash, saveTableState, loadTableState, deserializeFilter } from './table-state-persistence.js';
import { pillCSS, colorCSS } from './shared-styles.js';
import './table-filter-popover.js';
import type { SortDirectionChangedEventDetail, FilterChangedEventDetail } from './table-filter-popover.js';
import type { ExtractedTableData } from './table-data-extractor.js';
import type { SortDirection, TableFilterState } from './table-sorter-filterer.js';

export class WikiTable extends LitElement {
  static override readonly styles = [
    pillCSS,
    colorCSS,
    css`
      :host {
        display: block;
      }

      .status-bar {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 4px 8px;
        font-size: 12px;
        color: var(--color-text-muted);
        border-bottom: 1px solid var(--color-border-subtle);
        container-type: inline-size;
        container-name: status-bar;
      }

      .row-count {
        white-space: nowrap;
      }

      .row-count-filtered {
        font-weight: 600;
        color: var(--color-text-primary);
      }

      .status-pills {
        display: flex;
        gap: 6px;
        align-items: center;
      }

      .table-wrapper {
        position: relative;
      }

      .table-scroll-container {
        overflow-x: auto;
        -webkit-overflow-scrolling: touch;
      }

      :host([scroll-middle]) .table-wrapper::before,
      :host([scroll-end]) .table-wrapper::before {
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

      :host([scroll-start]) .table-wrapper::after,
      :host([scroll-middle]) .table-wrapper::after {
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

      .popover-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background: rgba(0, 0, 0, 0.15);
        z-index: 9998;
      }

      .view-toggle {
        display: inline-flex;
        border: 1px solid var(--color-border-default);
        border-radius: 16px;
        overflow: hidden;
        font-size: 12px;
        user-select: none;
      }

      .view-toggle-option {
        padding: 4px 10px;
        color: var(--color-text-secondary);
        transition: all 0.15s ease;
        white-space: nowrap;
        border: none;
        background: none;
        cursor: pointer;
        font-size: inherit;
        font-family: inherit;
        line-height: inherit;
      }

      .view-toggle-option:focus-visible {
        outline: 2px solid var(--color-action-link);
        outline-offset: -2px;
      }

      .view-toggle-active {
        background: var(--color-action-link);
        color: var(--color-text-inverse);
      }

      .pill-icon {
        display: inline;
      }

      .pill-text {
        display: inline;
      }

      @container status-bar (max-width: 300px) {
        .pill-text {
          display: none;
        }

        .view-toggle-text {
          display: none;
        }
      }

      @media (max-width: 400px) {
        .pill-text {
          display: none;
        }

        .view-toggle-text {
          display: none;
        }
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
        background: var(--color-surface-sunken);
        padding: 0;
        text-align: left;
        border-bottom: 2px solid var(--color-border-default);
        user-select: none;
        white-space: nowrap;
      }

      thead th:hover {
        background: var(--color-hover-overlay);
      }

      .header-cell {
        display: flex;
        align-items: center;
      }

      .header-main {
        flex: 1;
        padding: 8px 4px 8px 12px;
        cursor: pointer;
        display: flex;
        align-items: center;
        gap: 4px;
        background: none;
        border: none;
        font: inherit;
        color: inherit;
        text-align: left;
      }

      .filter-dot {
        display: inline-block;
        width: 6px;
        height: 6px;
        border-radius: 50%;
        background: var(--color-action-link);
        flex-shrink: 0;
      }

      .sort-arrows {
        padding: 8px 8px 8px 4px;
        cursor: pointer;
        opacity: 0.4;
        flex-shrink: 0;
        background: none;
        border: none;
        font: inherit;
        color: inherit;
      }

      .sort-arrows:hover {
        opacity: 0.8;
      }

      thead th.sorted .sort-arrows {
        opacity: 1;
      }

      tbody td {
        padding: 8px 12px;
        border-bottom: 1px solid var(--color-border-subtle);
      }

      tbody tr:hover {
        background: var(--color-hover-overlay);
      }

      .card-view {
        display: flex;
        flex-direction: column;
        gap: 8px;
        padding: 8px;
        background: var(--color-surface-sunken);
      }

      .card {
        border: 1px solid var(--color-border-default);
        border-radius: 8px;
        padding: 12px;
        background: var(--color-surface-primary);
        box-shadow: var(--shadow-subtle);
      }

      .card-row {
        display: flex;
        justify-content: space-between;
        padding: 4px 0;
        border-bottom: 1px solid var(--color-border-subtle);
      }

      .card-row:last-child {
        border-bottom: none;
      }

      .card-label {
        font-weight: bold;
        font-variant: small-caps;
        font-size: 0.85rem;
        color: var(--color-text-secondary);
      }

      .card-value {
        text-align: right;
      }

      .no-results {
        padding: 16px;
        text-align: center;
        color: var(--color-text-muted);
        font-style: italic;
      }

      .column-picker-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background: rgba(0, 0, 0, 0.3);
        z-index: 9998;
        display: flex;
        align-items: center;
        justify-content: center;
      }

      .column-picker {
        background: var(--color-surface-elevated);
        border-radius: 8px;
        box-shadow: var(--shadow-medium);
        padding: 8px 0;
        min-width: 180px;
        max-height: 60vh;
        overflow-y: auto;
      }

      .column-picker-title {
        padding: 6px 14px;
        font-size: 11px;
        font-weight: 600;
        color: var(--color-text-muted);
        text-transform: uppercase;
        letter-spacing: 0.5px;
      }

      .column-picker-item {
        padding: 8px 14px;
        cursor: pointer;
        font-size: 13px;
        color: var(--color-text-primary);
        border: none;
        background: none;
        width: 100%;
        text-align: left;
        font-family: inherit;
      }

      .column-picker-item:hover {
        background: var(--color-hover-overlay);
      }
    `,
  ];

  @state()
  declare extractedData: ExtractedTableData | null;

  @state()
  declare sortColumnIndex: number | null;

  @state()
  declare sortDirection: SortDirection;

  @state()
  declare tableFilters: TableFilterState;

  @state()
  declare cardViewActive: boolean;

  @state()
  declare popoverColumnIndex: number | null;

  @state()
  declare columnPickerOpen: boolean;

  private _mediaQuery: MediaQueryList | null = null;
  private _scrollContainer: HTMLElement | null = null;
  private _sourceTable: HTMLTableElement | null = null;
  private _tableHash: string | null = null;

  constructor() {
    super();
    this.extractedData = null;
    this.sortColumnIndex = null;
    this.sortDirection = 'none';
    this.tableFilters = new Map();
    this.cardViewActive = false;
    this.popoverColumnIndex = null;
    this.columnPickerOpen = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    this._parseSourceTable();
    this._mediaQuery = globalThis.matchMedia('(max-width: 600px)');
    this._mediaQuery.addEventListener('change', this._handleMediaChange);
    this.cardViewActive = this._mediaQuery.matches;
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this._mediaQuery?.removeEventListener('change', this._handleMediaChange);
    this._scrollContainer?.removeEventListener('scroll', this._handleScroll);
  }

  private readonly _handleMediaChange = (e: MediaQueryListEvent): void => {
    this.cardViewActive = e.matches;
  };

  private readonly _handleScroll = (): void => {
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
    this._sourceTable = table;
    this._sourceTable.style.display = 'none';
    this.extractedData = extractTableData(this._sourceTable);
    for (const row of this.extractedData.rows) {
      row.htmlCells = row.htmlCells.map(cell => DOMPurify.sanitize(cell));
    }

    const headerTexts = this.extractedData.columns.map(c => c.headerText);
    const cellValues = this.extractedData.rows.map(r => r.cells.map(String));
    this._tableHash = computeTableHash(headerTexts, cellValues);

    const saved = loadTableState(this._tableHash);
    if (saved) {
      this.sortColumnIndex = saved.sortColumnIndex;
      this.sortDirection = saved.sortDirection;
      const restoredFilters: TableFilterState = new Map();
      for (const [colIndex, serialized] of saved.filters) {
        restoredFilters.set(colIndex, deserializeFilter(serialized));
      }
      this.tableFilters = restoredFilters;
    }
  }

  private _saveState(): void {
    if (!this._tableHash) return;
    saveTableState(this._tableHash, this.sortColumnIndex, this.sortDirection, this.tableFilters);
  }

  private _getProcessedRows() {
    if (!this.extractedData) return [];
    let rows = this.extractedData.rows;

    if (hasActiveFilters(this.tableFilters)) {
      rows = applyAllFilters(rows, this.tableFilters, this.extractedData.columns);
    }

    if (this.sortColumnIndex !== null && this.sortDirection !== 'none') {
      const colDef = this.extractedData.columns[this.sortColumnIndex];
      if (colDef) {
        rows = sortRows(rows, this.sortColumnIndex, this.sortDirection, colDef.typeInfo.detectedType);
      }
    }

    return rows;
  }

  private _handleHeaderMainClick(columnIndex: number): void {
    this.popoverColumnIndex = columnIndex;
  }

  private _handleSortArrowClick(columnIndex: number, e: Event): void {
    e.stopPropagation();
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
    this._saveState();
  }

  private _handleFilterDotClick(columnIndex: number, e: Event): void {
    e.stopPropagation();
    this.popoverColumnIndex = columnIndex;
  }

  private _setView(isCardView: boolean): void {
    this.cardViewActive = isCardView;
  }

  private readonly _handleViewToggleKeydown = (e: KeyboardEvent): void => {
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
      e.preventDefault();
      if (!this.cardViewActive) {
        this.cardViewActive = true;
        this.shadowRoot?.querySelector<HTMLElement>('[data-view="cards"]')?.focus();
      }
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
      e.preventDefault();
      if (this.cardViewActive) {
        this.cardViewActive = false;
        this.shadowRoot?.querySelector<HTMLElement>('[data-view="table"]')?.focus();
      }
    }
  };

  private _clearAllFilters(): void {
    this.tableFilters = new Map();
    this._saveState();
  }

  private _hasActiveFilters(): boolean {
    return hasActiveFilters(this.tableFilters);
  }

  private _isColumnFiltered(columnIndex: number): boolean {
    const filter = this.tableFilters.get(columnIndex);
    return filter !== undefined && isFilterActive(filter);
  }

  private _getSortIndicator(columnIndex: number): string {
    if (this.sortColumnIndex !== columnIndex || this.sortDirection === 'none') {
      return '\u21C5';
    }
    return this.sortDirection === 'ascending' ? '\u2191' : '\u2193';
  }

  private _handlePopoverSortChanged(e: CustomEvent<SortDirectionChangedEventDetail>): void {
    const { direction } = e.detail;
    if (direction === 'none') {
      this.sortDirection = 'none';
      this.sortColumnIndex = null;
    } else if (this.popoverColumnIndex !== null) {
      this.sortColumnIndex = this.popoverColumnIndex;
      this.sortDirection = direction;
    }
    this._saveState();
  }

  private _handlePopoverFilterChanged(e: CustomEvent<FilterChangedEventDetail>): void {
    if (this.popoverColumnIndex === null) return;
    const { filter } = e.detail;
    const newFilters = new Map(this.tableFilters);
    if (filter === null) {
      newFilters.delete(this.popoverColumnIndex);
    } else {
      newFilters.set(this.popoverColumnIndex, filter);
    }
    this.tableFilters = newFilters;
    this._saveState();
  }

  private _handlePopoverClosed(): void {
    this.popoverColumnIndex = null;
  }

  private _openColumnPicker(): void {
    this.columnPickerOpen = true;
  }

  private _handleColumnPickerSelect(columnIndex: number): void {
    this.columnPickerOpen = false;
    this.popoverColumnIndex = columnIndex;
  }

  private _handleColumnPickerOverlayClick(e: Event): void {
    if (e.target instanceof HTMLElement && e.target.classList.contains('column-picker-overlay')) {
      this.columnPickerOpen = false;
    }
  }

  private _handlePopoverOverlayClick(e: Event): void {
    if (e.target instanceof HTMLElement && e.target.classList.contains('popover-overlay')) {
      this._handlePopoverClosed();
    }
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

  override render(): TemplateResult {
    if (!this.extractedData) {
      return html`<slot></slot>`;
    }

    const processedRows = this._getProcessedRows();
    const totalRows = this.extractedData.rows.length;
    const shownRows = processedRows.length;
    const filtered = this._hasActiveFilters();

    return html`
      <div class="status-bar">
        <span class="row-count ${filtered ? 'row-count-filtered' : ''}">
          ${filtered
            ? `${shownRows} of ${totalRows} rows`
            : `${totalRows} rows`}
        </span>
        <div class="status-pills">
          ${filtered ? html`
            <button
              type="button"
              class="tag-filter-clear"
              @click=${this._clearAllFilters}
              aria-label="Clear all filters"
            ><span class="pill-icon">\u2715</span><span class="pill-text"> clear</span></button>
          ` : nothing}
          ${this.cardViewActive ? html`
            <button
              type="button"
              class="tag-pill"
              @click=${this._openColumnPicker}
              aria-label="Sort and filter"
            ><span class="pill-icon">\u2699</span><span class="pill-text"> sort/filter</span></button>
          ` : nothing}
          <div
            class="view-toggle"
            role="radiogroup"
            aria-label="View mode"
            @keydown=${this._handleViewToggleKeydown}
          >
            <button
              type="button"
              role="radio"
              class="view-toggle-option ${this.cardViewActive ? '' : 'view-toggle-active'}"
              aria-checked="${!this.cardViewActive}"
              tabindex="${this.cardViewActive ? -1 : 0}"
              data-view="table"
              @click=${() => this._setView(false)}
            >\u25A4<span class="view-toggle-text"> table</span></button>
            <button
              type="button"
              role="radio"
              class="view-toggle-option ${this.cardViewActive ? 'view-toggle-active' : ''}"
              aria-checked="${this.cardViewActive}"
              tabindex="${this.cardViewActive ? 0 : -1}"
              data-view="cards"
              @click=${() => this._setView(true)}
            >\u229E<span class="view-toggle-text"> cards</span></button>
          </div>
        </div>
      </div>
      ${this.cardViewActive
        ? this._renderCardView(processedRows)
        : this._renderTableView(processedRows)}
      ${this._renderPopover()}
      ${this.columnPickerOpen ? this._renderColumnPicker() : nothing}
      <slot style="display:none"></slot>
    `;
  }

  private _renderPopover(): TemplateResult {
    if (this.popoverColumnIndex === null || !this.extractedData) {
      return html``;
    }

    const col = this.extractedData.columns[this.popoverColumnIndex];
    if (!col) return html``;

    const uniqueValues = getUniqueColumnValues(this.extractedData.rows, this.popoverColumnIndex);
    const numericRange = getColumnNumericRange(
      this.extractedData.rows,
      this.popoverColumnIndex,
      col.typeInfo.detectedType,
    );
    const currentFilter = this.tableFilters.get(this.popoverColumnIndex) ?? null;
    const currentSortDirection = this.sortColumnIndex === this.popoverColumnIndex
      ? this.sortDirection
      : 'none';

    return html`
      <div class="popover-overlay" @click=${this._handlePopoverOverlayClick}>
        <table-filter-popover
          .columnDefinition=${col}
          .uniqueValues=${uniqueValues}
          .numericRange=${numericRange}
          .currentFilter=${currentFilter}
          .currentSortDirection=${currentSortDirection}
          .open=${true}
          @sort-direction-changed=${this._handlePopoverSortChanged}
          @filter-changed=${this._handlePopoverFilterChanged}
          @popover-closed=${this._handlePopoverClosed}
        ></table-filter-popover>
      </div>
    `;
  }

  private _renderColumnPicker(): TemplateResult {
    if (!this.extractedData) {
      throw new Error('_renderColumnPicker called without extractedData — programming bug');
    }
    return html`
      <div class="column-picker-overlay" @click=${this._handleColumnPickerOverlayClick}>
        <div class="column-picker">
          <div class="column-picker-title">Select column</div>
          ${this.extractedData.columns.map(col => html`
            <button
              type="button"
              class="column-picker-item"
              @click=${() => this._handleColumnPickerSelect(col.columnIndex)}
            >${col.headerText}</button>
          `)}
        </div>
      </div>
    `;
  }

  private _renderTableView(processedRows: ReturnType<typeof this._getProcessedRows>): TemplateResult {
    if (!this.extractedData) {
      throw new Error('_renderTableView called without extractedData — programming bug');
    }
    return html`
      <div class="table-wrapper">
      <div class="table-scroll-container">
        <table>
          <thead>
            <tr>
              ${this.extractedData.columns.map((col, i) => html`
                <th
                  scope="col"
                  class="${this.sortColumnIndex === i ? 'sorted' : ''}"
                  title="Detected type: ${col.typeInfo.detectedType}"
                  aria-sort="${this.sortColumnIndex === i ? this.sortDirection : 'none'}"
                >
                  <div class="header-cell">
                    <button
                      type="button"
                      class="header-main"
                      @click=${() => this._handleHeaderMainClick(i)}
                    >
                      ${col.headerText}
                      ${this._isColumnFiltered(i) ? html`<span class="filter-dot" @click=${(e: Event) => this._handleFilterDotClick(i, e)}></span>` : nothing}
                    </button>
                    <button
                      type="button"
                      class="sort-arrows"
                      aria-label="Sort by ${col.headerText}"
                      @click=${(e: Event) => this._handleSortArrowClick(i, e)}
                    ><span aria-hidden="true">${this._getSortIndicator(i)}</span></button>
                  </div>
                </th>
              `)}
            </tr>
          </thead>
          <tbody>
            ${processedRows.length === 0 ? html`
              <tr><td colspan="${this.extractedData.columns.length}" class="no-results">No matching rows</td></tr>
            ` : processedRows.map(row => html`
              <tr>
                ${row.htmlCells.map(cell => html`<td>${unsafeHTML(cell)}</td>`)}
              </tr>
            `)}
          </tbody>
        </table>
      </div>
      </div>
    `;
  }

  private _renderCardView(processedRows: ReturnType<typeof this._getProcessedRows>): TemplateResult {
    if (!this.extractedData) {
      throw new Error('_renderCardView called without extractedData — programming bug');
    }
    const { extractedData } = this;
    if (processedRows.length === 0) {
      return html`<div class="no-results">No matching rows</div>`;
    }

    return html`
      <div class="card-view">
        ${processedRows.map(row => html`
          <div class="card">
            ${extractedData.columns.map((col, i) => html`
              <div class="card-row">
                <span class="card-label">${col.headerText}</span>
                <span class="card-value">${unsafeHTML(row.htmlCells[i] ?? '')}</span>
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
