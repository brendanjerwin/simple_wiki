import type { TemplateResult } from 'lit';
import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { pillCSS } from './shared-styles.js';
import type { TableColumnDefinition } from './table-data-extractor.js';
import type { SortDirection, ColumnFilterState } from './table-sorter-filterer.js';

const CHECKBOX_THRESHOLD = 15;

export interface SortDirectionChangedEventDetail {
  direction: SortDirection;
}

export interface FilterChangedEventDetail {
  filter: ColumnFilterState | null;
}

export class TableFilterPopover extends LitElement {
  static override styles = [
    pillCSS,
    css`
      :host {
        display: block;
        position: relative;
      }

      .popover {
        display: none;
        flex-direction: column;
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        max-width: 320px;
        max-height: 80vh;
        width: 90vw;
        z-index: 9999;
        background: white;
        border-radius: 8px;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
        overflow: hidden;
      }

      :host([open]) .popover {
        display: flex;
      }

      .popover-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 10px 14px;
        border-bottom: 1px solid #e0e0e0;
      }

      .popover-title {
        font-size: 14px;
        font-weight: 600;
        color: #333;
      }

      .popover-type {
        font-weight: 400;
        color: #999;
        font-size: 12px;
        margin-left: 6px;
      }

      .close-btn {
        background: none;
        border: none;
        font-size: 18px;
        cursor: pointer;
        color: #999;
        padding: 2px 6px;
        border-radius: 4px;
        line-height: 1;
      }

      .close-btn:hover {
        background: #f0f0f0;
        color: #333;
      }

      .popover-body {
        padding: 10px 14px;
        overflow-y: auto;
        flex: 1;
      }

      .section-label {
        font-size: 11px;
        font-weight: 600;
        color: #888;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        margin-bottom: 6px;
      }

      .sort-controls {
        display: flex;
        gap: 6px;
        margin-bottom: 12px;
      }

      .sort-pill {
        font-size: 12px;
        padding: 4px 10px;
        background: #f0f0f0;
        border: 1px solid #ddd;
        border-radius: 16px;
        cursor: pointer;
        font-family: inherit;
        transition: all 0.15s ease;
        color: #555;
      }

      .sort-pill:hover {
        background: #e0e0e0;
        border-color: #bbb;
      }

      .sort-pill-active {
        background: #0d6efd;
        color: white;
        border-color: #0d6efd;
      }

      .sort-pill-active:hover {
        background: #0b5ed7;
        border-color: #0b5ed7;
      }

      .divider {
        border: none;
        border-top: 1px solid #e0e0e0;
        margin: 8px 0;
      }

      .checkbox-list {
        max-height: 250px;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        gap: 2px;
      }

      .checkbox-header {
        display: flex;
        gap: 8px;
        margin-bottom: 6px;
        font-size: 12px;
      }

      .checkbox-link {
        color: #0d6efd;
        cursor: pointer;
        background: none;
        border: none;
        padding: 0;
        font-size: 12px;
        font-family: inherit;
      }

      .checkbox-link:hover {
        text-decoration: underline;
      }

      .checkbox-item {
        display: flex;
        align-items: center;
        gap: 6px;
        padding: 3px 0;
        font-size: 13px;
        color: #333;
      }

      .checkbox-item input[type="checkbox"] {
        margin: 0;
        accent-color: #0d6efd;
      }

      .search-input {
        width: 100%;
        padding: 8px 10px;
        border: 1px solid #ddd;
        border-radius: 4px;
        font-size: 13px;
        font-family: inherit;
        box-sizing: border-box;
      }

      .search-input:focus {
        outline: none;
        border-color: #0d6efd;
        box-shadow: 0 0 0 2px rgba(13, 110, 253, 0.15);
      }

      .range-container {
        display: flex;
        flex-direction: column;
        gap: 8px;
      }

      .range-inputs {
        display: flex;
        gap: 8px;
        align-items: center;
      }

      .range-input {
        flex: 1;
        padding: 6px 8px;
        border: 1px solid #ddd;
        border-radius: 4px;
        font-size: 13px;
        font-family: inherit;
        box-sizing: border-box;
        min-width: 0;
      }

      .range-input:focus {
        outline: none;
        border-color: #0d6efd;
        box-shadow: 0 0 0 2px rgba(13, 110, 253, 0.15);
      }

      .range-separator {
        color: #999;
        font-size: 12px;
        flex-shrink: 0;
      }

      .range-slider {
        width: 100%;
        margin: 4px 0;
        accent-color: #0d6efd;
      }
    `,
  ];

  @property({ attribute: false })
  declare columnDefinition: TableColumnDefinition | null;

  @property({ attribute: false })
  declare uniqueValues: string[];

  @property({ attribute: false })
  declare numericRange: { min: number; max: number } | null;

  @property({ attribute: false })
  declare currentFilter: ColumnFilterState | null;

  @property({ attribute: false })
  declare currentSortDirection: SortDirection;

  @property({ type: Boolean, reflect: true })
  declare open: boolean;

  @state()
  declare _excludedValues: Set<string>;

  @state()
  declare _rangeMin: number | null;

  @state()
  declare _rangeMax: number | null;

  @state()
  declare _searchText: string;

  private _handleClickOutside: (event: Event) => void;
  private _handleKeydown: (event: KeyboardEvent) => void;

  constructor() {
    super();
    this.columnDefinition = null;
    this.uniqueValues = [];
    this.numericRange = null;
    this.currentFilter = null;
    this.currentSortDirection = 'none';
    this.open = false;
    this._excludedValues = new Set();
    this._rangeMin = null;
    this._rangeMax = null;
    this._searchText = '';
    this._handleClickOutside = this.handleClickOutside.bind(this);
    this._handleKeydown = this.handleKeydown.bind(this);
  }

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('click', this._handleClickOutside);
    document.addEventListener('keydown', this._handleKeydown);
  }

  override disconnectedCallback(): void {
    document.removeEventListener('click', this._handleClickOutside);
    document.removeEventListener('keydown', this._handleKeydown);
    super.disconnectedCallback();
  }

  override willUpdate(changed: Map<string, unknown>): void {
    if (changed.has('currentFilter') || changed.has('open')) {
      this._syncFromCurrentFilter();
    }
  }

  private _syncFromCurrentFilter(): void {
    if (!this.currentFilter) {
      this._excludedValues = new Set();
      this._rangeMin = null;
      this._rangeMax = null;
      this._searchText = '';
      return;
    }

    switch (this.currentFilter.kind) {
      case 'checkbox':
        this._excludedValues = new Set(this.currentFilter.excludedValues);
        break;
      case 'range':
        this._rangeMin = this.currentFilter.min;
        this._rangeMax = this.currentFilter.max;
        break;
      case 'text-search':
        this._searchText = this.currentFilter.searchText;
        break;
    }
  }

  private _getFilterKind(): 'checkbox' | 'range' | 'text-search' {
    if (this.currentFilter) return this.currentFilter.kind;
    if (this.numericRange !== null) return 'range';
    if (this.uniqueValues.length <= CHECKBOX_THRESHOLD) return 'checkbox';
    return 'text-search';
  }

  handleClickOutside(event: Event): void {
    const path = event.composedPath();
    const popover = this.shadowRoot?.querySelector('.popover');

    if (this.open && popover && !path.includes(popover)) {
      this._close();
    }
  }

  handleKeydown(event: KeyboardEvent): void {
    if (this.open && event.key === 'Escape') {
      event.preventDefault();
      this._close();
    }
  }

  private _close(): void {
    this.dispatchEvent(new CustomEvent('popover-closed', {
      bubbles: true,
      composed: true,
    }));
  }

  private _handlePopoverClick(event: Event): void {
    event.stopPropagation();
  }

  private _handleSortClick(direction: SortDirection): void {
    const newDirection = this.currentSortDirection === direction ? 'none' : direction;
    this.dispatchEvent(new CustomEvent<SortDirectionChangedEventDetail>('sort-direction-changed', {
      detail: { direction: newDirection },
      bubbles: true,
      composed: true,
    }));
  }

  private _emitFilterChanged(filter: ColumnFilterState | null): void {
    this.dispatchEvent(new CustomEvent<FilterChangedEventDetail>('filter-changed', {
      detail: { filter },
      bubbles: true,
      composed: true,
    }));
  }

  private _handleCheckboxChange(value: string, checked: boolean): void {
    const newExcluded = new Set(this._excludedValues);
    if (checked) {
      newExcluded.delete(value);
    } else {
      newExcluded.add(value);
    }
    this._excludedValues = newExcluded;

    if (newExcluded.size === 0) {
      this._emitFilterChanged(null);
    } else {
      this._emitFilterChanged({ kind: 'checkbox', excludedValues: newExcluded });
    }
  }

  private _handleSelectAll(): void {
    this._excludedValues = new Set();
    this._emitFilterChanged(null);
  }

  private _handleSelectNone(): void {
    this._excludedValues = new Set(this.uniqueValues);
    this._emitFilterChanged({ kind: 'checkbox', excludedValues: this._excludedValues });
  }

  private _handleRangeMinChange(value: string): void {
    this._rangeMin = value === '' ? null : Number(value);
    this._emitRangeFilter();
  }

  private _handleRangeMaxChange(value: string): void {
    this._rangeMax = value === '' ? null : Number(value);
    this._emitRangeFilter();
  }

  private _handleRangeSliderMinChange(value: string): void {
    this._rangeMin = Number(value);
    this._emitRangeFilter();
  }

  private _handleRangeSliderMaxChange(value: string): void {
    this._rangeMax = Number(value);
    this._emitRangeFilter();
  }

  private _emitRangeFilter(): void {
    if (this._rangeMin === null && this._rangeMax === null) {
      this._emitFilterChanged(null);
    } else {
      this._emitFilterChanged({ kind: 'range', min: this._rangeMin, max: this._rangeMax });
    }
  }

  private _handleSearchInput(value: string): void {
    this._searchText = value;
    if (value.trim() === '') {
      this._emitFilterChanged(null);
    } else {
      this._emitFilterChanged({ kind: 'text-search', searchText: value });
    }
  }

  override render(): TemplateResult {
    return html`
      <div class="popover" @click=${this._handlePopoverClick}>
        ${this.open && this.columnDefinition ? this._renderContent() : nothing}
      </div>
    `;
  }

  private _renderContent(): TemplateResult {
    const col = this.columnDefinition!;
    const filterKind = this._getFilterKind();

    return html`
      <div class="popover-header">
        <span class="popover-title">
          ${col.headerText}
          <span class="popover-type">(${col.typeInfo.detectedType})</span>
        </span>
        <button type="button" class="close-btn" @click=${this._close} aria-label="Close">
          \u2715
        </button>
      </div>
      <div class="popover-body">
        <div class="section-label">Sort</div>
        <div class="sort-controls">
          <button
            type="button"
            class="sort-pill ${this.currentSortDirection === 'ascending' ? 'sort-pill-active' : ''}"
            @click=${() => this._handleSortClick('ascending')}
            aria-label="Sort ascending"
          >\u2191 Ascending</button>
          <button
            type="button"
            class="sort-pill ${this.currentSortDirection === 'descending' ? 'sort-pill-active' : ''}"
            @click=${() => this._handleSortClick('descending')}
            aria-label="Sort descending"
          >\u2193 Descending</button>
          ${this.currentSortDirection !== 'none' ? html`
            <button
              type="button"
              class="sort-pill"
              @click=${() => this._handleSortClick(this.currentSortDirection)}
              aria-label="Clear sort"
            >\u2715</button>
          ` : nothing}
        </div>

        <hr class="divider" />

        <div class="section-label">Filter</div>
        ${filterKind === 'checkbox' ? this._renderCheckboxFilter() : nothing}
        ${filterKind === 'range' ? this._renderRangeFilter() : nothing}
        ${filterKind === 'text-search' ? this._renderTextSearchFilter() : nothing}
      </div>
    `;
  }

  private _renderCheckboxFilter(): TemplateResult {
    return html`
      <div class="checkbox-header">
        <button type="button" class="checkbox-link" @click=${this._handleSelectAll}>Select All</button>
        <button type="button" class="checkbox-link" @click=${this._handleSelectNone}>Select None</button>
      </div>
      <div class="checkbox-list">
        ${this.uniqueValues.map(value => html`
          <label class="checkbox-item">
            <input
              type="checkbox"
              .checked=${!this._excludedValues.has(value)}
              @change=${(e: Event) => {
                if (!(e.target instanceof HTMLInputElement)) return;
                this._handleCheckboxChange(value, e.target.checked);
              }}
            />
            ${value}
          </label>
        `)}
      </div>
    `;
  }

  private _renderRangeFilter(): TemplateResult {
    const dataMin = this.numericRange?.min ?? 0;
    const dataMax = this.numericRange?.max ?? 100;
    const sliderMin = this._rangeMin ?? dataMin;
    const sliderMax = this._rangeMax ?? dataMax;

    return html`
      <div class="range-container">
        <input
          type="range"
          class="range-slider"
          min=${dataMin}
          max=${dataMax}
          step="any"
          .value=${String(sliderMin)}
          @input=${(e: Event) => {
            if (!(e.target instanceof HTMLInputElement)) return;
            this._handleRangeSliderMinChange(e.target.value);
          }}
          aria-label="Range minimum slider"
        />
        <input
          type="range"
          class="range-slider"
          min=${dataMin}
          max=${dataMax}
          step="any"
          .value=${String(sliderMax)}
          @input=${(e: Event) => {
            if (!(e.target instanceof HTMLInputElement)) return;
            this._handleRangeSliderMaxChange(e.target.value);
          }}
          aria-label="Range maximum slider"
        />
        <div class="range-inputs">
          <input
            type="number"
            class="range-input"
            placeholder="Min"
            .value=${this._rangeMin !== null ? String(this._rangeMin) : ''}
            @input=${(e: Event) => {
              if (!(e.target instanceof HTMLInputElement)) return;
              this._handleRangeMinChange(e.target.value);
            }}
            aria-label="Minimum value"
          />
          <span class="range-separator">to</span>
          <input
            type="number"
            class="range-input"
            placeholder="Max"
            .value=${this._rangeMax !== null ? String(this._rangeMax) : ''}
            @input=${(e: Event) => {
              if (!(e.target instanceof HTMLInputElement)) return;
              this._handleRangeMaxChange(e.target.value);
            }}
            aria-label="Maximum value"
          />
        </div>
      </div>
    `;
  }

  private _renderTextSearchFilter(): TemplateResult {
    return html`
      <input
        type="text"
        class="search-input"
        placeholder="Search\u2026"
        .value=${this._searchText}
        @input=${(e: Event) => {
          if (!(e.target instanceof HTMLInputElement)) return;
          this._handleSearchInput(e.target.value);
        }}
        aria-label="Search filter"
      />
    `;
  }
}

customElements.define('table-filter-popover', TableFilterPopover);

declare global {
  interface HTMLElementTagNameMap {
    'table-filter-popover': TableFilterPopover;
  }
}
