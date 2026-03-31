import type { TemplateResult } from 'lit';
import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import type { TableColumnDefinition } from './table-data-extractor.js';
import type { SortDirection, ColumnFilterState, CheckboxFilterState } from './table-sorter-filterer.js';

const CHECKBOX_THRESHOLD = 15;

export interface SortDirectionChangedEventDetail {
  direction: SortDirection;
}

export interface FilterChangedEventDetail {
  filter: ColumnFilterState | null;
}

export class TableFilterPopover extends LitElement {
  static override styles = [
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

      .popover-footer {
        display: flex;
        justify-content: flex-end;
        gap: 8px;
        padding: 10px 14px;
        border-top: 1px solid #e0e0e0;
      }

      .footer-btn {
        padding: 6px 16px;
        border-radius: 4px;
        font-size: 13px;
        font-family: inherit;
        cursor: pointer;
        border: 1px solid #ddd;
        background: #f5f5f5;
        color: #333;
      }

      .footer-btn:hover {
        background: #e8e8e8;
      }

      .footer-btn-primary {
        background: #0d6efd;
        color: white;
        border-color: #0d6efd;
      }

      .footer-btn-primary:hover {
        background: #0b5ed7;
        border-color: #0b5ed7;
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

  @state()
  declare _pendingSortDirection: SortDirection;

  private _pendingTimerId: ReturnType<typeof setTimeout> | null = null;

  public readonly _handleClickOutside = (event: Event): void => {
    const path = event.composedPath();
    const popover = this.shadowRoot?.querySelector('.popover');

    if (this.open && popover && !path.includes(popover)) {
      this._cancel();
    }
  };

  public readonly _handleKeydown = (event: KeyboardEvent): void => {
    if (this.open && event.key === 'Escape') {
      event.preventDefault();
      this._cancel();
    }
  };

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
    this._pendingSortDirection = 'none';
  }

  override disconnectedCallback(): void {
    if (this._pendingTimerId !== null) {
      clearTimeout(this._pendingTimerId);
      this._pendingTimerId = null;
    }
    document.removeEventListener('click', this._handleClickOutside);
    document.removeEventListener('keydown', this._handleKeydown);
    super.disconnectedCallback();
  }

  override willUpdate(changed: Map<string, unknown>): void {
    if (changed.has('currentFilter') || changed.has('open') || changed.has('currentSortDirection')) {
      this._syncFromInputs();
    }
    if (changed.has('open')) {
      if (this.open) {
        this._pendingTimerId = setTimeout(() => {
          this._pendingTimerId = null;
          document.addEventListener('click', this._handleClickOutside);
          document.addEventListener('keydown', this._handleKeydown);
        }, 0);
      } else {
        if (this._pendingTimerId !== null) {
          clearTimeout(this._pendingTimerId);
          this._pendingTimerId = null;
        }
        document.removeEventListener('click', this._handleClickOutside);
        document.removeEventListener('keydown', this._handleKeydown);
      }
    }
  }

  private _syncFromInputs(): void {
    this._pendingSortDirection = this.currentSortDirection;

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
    if (this.uniqueValues.length <= CHECKBOX_THRESHOLD) return 'checkbox';
    if (this.numericRange !== null) return 'range';
    return 'text-search';
  }

  private _cancel(): void {
    this.dispatchEvent(new CustomEvent('popover-closed', {
      bubbles: true,
      composed: true,
    }));
  }

  private _handlePopoverClick(event: Event): void {
    event.stopPropagation();
  }

  private _handleSortClick(direction: SortDirection): void {
    this._pendingSortDirection = this._pendingSortDirection === direction ? 'none' : direction;
  }

  private _buildCurrentFilter(): ColumnFilterState | null {
    const filterKind = this._getFilterKind();

    switch (filterKind) {
      case 'checkbox':
        return this._excludedValues.size === 0
          ? null
          : { kind: 'checkbox', excludedValues: new Set(this._excludedValues) };
      case 'range':
        return this._rangeMin === null && this._rangeMax === null
          ? null
          : { kind: 'range', min: this._rangeMin, max: this._rangeMax };
      case 'text-search':
        return this._searchText.trim() === ''
          ? null
          : { kind: 'text-search', searchText: this._searchText };
    }
  }

  private _checkboxFiltersChanged(newFilter: CheckboxFilterState, oldFilter: CheckboxFilterState): boolean {
    if (newFilter.excludedValues.size !== oldFilter.excludedValues.size) return true;
    for (const v of newFilter.excludedValues) {
      if (!oldFilter.excludedValues.has(v)) return true;
    }
    return false;
  }

  private _filtersChanged(): boolean {
    const newFilter = this._buildCurrentFilter();
    const oldFilter = this.currentFilter;

    if (newFilter === null && oldFilter === null) return false;
    if (newFilter === null || oldFilter === null) return true;
    if (newFilter.kind !== oldFilter.kind) return true;

    if (newFilter.kind === 'checkbox' && oldFilter.kind === 'checkbox') {
      return this._checkboxFiltersChanged(newFilter, oldFilter);
    }
    if (newFilter.kind === 'range' && oldFilter.kind === 'range') {
      return newFilter.min !== oldFilter.min || newFilter.max !== oldFilter.max;
    }
    if (newFilter.kind === 'text-search' && oldFilter.kind === 'text-search') {
      return newFilter.searchText !== oldFilter.searchText;
    }

    return true;
  }

  private _handleOk(): void {
    const sortChanged = this._pendingSortDirection !== this.currentSortDirection;
    const filterChanged = this._filtersChanged();

    if (sortChanged) {
      this.dispatchEvent(new CustomEvent<SortDirectionChangedEventDetail>('sort-direction-changed', {
        detail: { direction: this._pendingSortDirection },
        bubbles: true,
        composed: true,
      }));
    }

    if (filterChanged) {
      this.dispatchEvent(new CustomEvent<FilterChangedEventDetail>('filter-changed', {
        detail: { filter: this._buildCurrentFilter() },
        bubbles: true,
        composed: true,
      }));
    }

    this.dispatchEvent(new CustomEvent('popover-closed', {
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
  }

  private _handleSelectAll(): void {
    this._excludedValues = new Set();
  }

  private _handleSelectNone(): void {
    this._excludedValues = new Set(this.uniqueValues);
  }

  private _handleRangeMinChange(value: string): void {
    const parsed = Number(value);
    this._rangeMin = value === '' || Number.isNaN(parsed) ? null : parsed;
  }

  private _handleRangeMaxChange(value: string): void {
    const parsed = Number(value);
    this._rangeMax = value === '' || Number.isNaN(parsed) ? null : parsed;
  }

  private _handleRangeSliderMinChange(value: string): void {
    this._rangeMin = Number(value);
  }

  private _handleRangeSliderMaxChange(value: string): void {
    this._rangeMax = Number(value);
  }

  private _handleSearchInput(value: string): void {
    this._searchText = value;
  }

  override render(): TemplateResult {
    return html`
      <div class="popover" @click=${this._handlePopoverClick}>
        ${this.open && this.columnDefinition ? this._renderContent() : nothing}
      </div>
    `;
  }

  private _renderContent(): TemplateResult {
    if (!this.columnDefinition) {
      throw new Error('_renderContent called without columnDefinition — programming bug');
    }
    const col = this.columnDefinition;
    const filterKind = this._getFilterKind();

    return html`
      <div class="popover-header">
        <span class="popover-title">
          ${col.headerText}
          <span class="popover-type">(${col.typeInfo.detectedType})</span>
        </span>
        <button type="button" class="close-btn" @click=${this._cancel} aria-label="Close">
          \u2715
        </button>
      </div>
      <div class="popover-body">
        <div class="section-label">Sort</div>
        <div class="sort-controls">
          <button
            type="button"
            class="sort-pill ${this._pendingSortDirection === 'ascending' ? 'sort-pill-active' : ''}"
            @click=${() => this._handleSortClick('ascending')}
            aria-label="Sort ascending"
          >\u2191 Ascending</button>
          <button
            type="button"
            class="sort-pill ${this._pendingSortDirection === 'descending' ? 'sort-pill-active' : ''}"
            @click=${() => this._handleSortClick('descending')}
            aria-label="Sort descending"
          >\u2193 Descending</button>
          ${this._pendingSortDirection !== 'none' ? html`
            <button
              type="button"
              class="sort-pill"
              @click=${() => this._handleSortClick(this._pendingSortDirection)}
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
      <div class="popover-footer">
        <button type="button" class="footer-btn" @click=${this._cancel} aria-label="Cancel">Cancel</button>
        <button type="button" class="footer-btn footer-btn-primary" @click=${this._handleOk} aria-label="Apply">OK</button>
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

  private _getRangeStep(): string {
    const type = this.columnDefinition?.typeInfo.detectedType;
    if (type === 'integer') return '1';
    if (type === 'currency') return '0.01';
    return 'any';
  }

  private _renderRangeFilter(): TemplateResult {
    const dataMin = this.numericRange?.min ?? 0;
    const dataMax = this.numericRange?.max ?? 100;
    const sliderMin = this._rangeMin ?? dataMin;
    const sliderMax = this._rangeMax ?? dataMax;
    const step = this._getRangeStep();

    return html`
      <div class="range-container">
        <input
          type="range"
          class="range-slider"
          min=${dataMin}
          max=${dataMax}
          step=${step}
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
          step=${step}
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
