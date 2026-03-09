import { parseForType } from './column-type-detector.js';
import type { ColumnDataType } from './column-type-detector.js';
import type { TableRowData } from './table-data-extractor.js';

export type SortDirection = 'ascending' | 'descending' | 'none';

export interface CheckboxFilterState {
  kind: 'checkbox';
  excludedValues: Set<string>;
}

export interface RangeFilterState {
  kind: 'range';
  min: number | null;
  max: number | null;
}

export interface TextSearchFilterState {
  kind: 'text-search';
  searchText: string;
}

export type ColumnFilterState = CheckboxFilterState | RangeFilterState | TextSearchFilterState;

export type TableFilterState = Map<number, ColumnFilterState>;

export function sortRows(
  rows: TableRowData[],
  columnIndex: number,
  direction: SortDirection,
  columnType: ColumnDataType,
): TableRowData[] {
  if (direction === 'none') {
    return [...rows];
  }

  const sorted = [...rows];
  const multiplier = direction === 'ascending' ? 1 : -1;

  sorted.sort((a, b) => {
    const aText = a.cells[columnIndex] ?? '';
    const bText = b.cells[columnIndex] ?? '';

    if (columnType === 'text') {
      return multiplier * aText.localeCompare(bText, undefined, { sensitivity: 'base' });
    }

    const aVal = parseForType(aText, columnType);
    const bVal = parseForType(bText, columnType);

    if (Number.isNaN(aVal) && Number.isNaN(bVal)) return 0;
    if (Number.isNaN(aVal)) return 1;
    if (Number.isNaN(bVal)) return -1;

    return multiplier * (aVal - bVal);
  });

  return sorted;
}

export function applyColumnFilter(
  rows: TableRowData[],
  columnIndex: number,
  filterState: ColumnFilterState,
  columnType: ColumnDataType,
): TableRowData[] {
  return rows.filter(row => {
    const cellText = row.cells[columnIndex] ?? '';

    switch (filterState.kind) {
      case 'checkbox':
        return !filterState.excludedValues.has(cellText);

      case 'range': {
        const value = parseForType(cellText, columnType);
        if (Number.isNaN(value)) return false;
        if (filterState.min !== null && value < filterState.min) return false;
        if (filterState.max !== null && value > filterState.max) return false;
        return true;
      }

      case 'text-search':
        return cellText.toLowerCase().includes(filterState.searchText.trim().toLowerCase());
    }
  });
}

export function applyAllFilters(
  rows: TableRowData[],
  filters: TableFilterState,
  columns: { columnIndex: number; typeInfo: { detectedType: ColumnDataType } }[],
): TableRowData[] {
  let result = rows;

  for (const [colIndex, filterState] of filters) {
    if (!isFilterActive(filterState)) continue;
    const colDef = columns.find(c => c.columnIndex === colIndex);
    if (!colDef) continue;
    result = applyColumnFilter(result, colIndex, filterState, colDef.typeInfo.detectedType);
  }

  return result;
}

export function isFilterActive(filterState: ColumnFilterState): boolean {
  switch (filterState.kind) {
    case 'checkbox':
      return filterState.excludedValues.size > 0;
    case 'range':
      return filterState.min !== null || filterState.max !== null;
    case 'text-search':
      return filterState.searchText.trim() !== '';
  }
}

export function hasActiveFilters(filters: TableFilterState): boolean {
  for (const filterState of filters.values()) {
    if (isFilterActive(filterState)) return true;
  }
  return false;
}
