import {
  parseNumericValue,
  parseCurrencyValue,
  parsePercentageValue,
  parseDateValue,
} from './column-type-detector.js';
import type { ColumnDataType } from './column-type-detector.js';
import type { TableRowData } from './table-data-extractor.js';

export type SortDirection = 'ascending' | 'descending' | 'none';

function parseForType(text: string, columnType: ColumnDataType): number {
  switch (columnType) {
    case 'number': return parseNumericValue(text);
    case 'currency': return parseCurrencyValue(text);
    case 'percentage': return parsePercentageValue(text);
    case 'date': return parseDateValue(text);
    default: return NaN;
  }
}

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

const operatorPattern = /^(>=|<=|>|<|=)\s*(-?\d+\.?\d*)$/;

function matchesNumericFilter(
  cellText: string,
  filterText: string,
  columnType: ColumnDataType,
): boolean | null {
  const match = operatorPattern.exec(filterText.trim());
  if (!match) return null;

  const operator = match[1]!;
  const threshold = Number(match[2]);
  const cellValue = parseForType(cellText, columnType);

  if (Number.isNaN(cellValue)) return false;

  switch (operator) {
    case '>': return cellValue > threshold;
    case '<': return cellValue < threshold;
    case '>=': return cellValue >= threshold;
    case '<=': return cellValue <= threshold;
    case '=': return cellValue === threshold;
    default: return false;
  }
}

export function filterRows(
  rows: TableRowData[],
  columnIndex: number,
  filterText: string,
  columnType: ColumnDataType,
): TableRowData[] {
  if (filterText.trim() === '') {
    return [...rows];
  }

  const isNumericType = columnType === 'number' || columnType === 'currency' || columnType === 'percentage';

  return rows.filter(row => {
    const cellText = row.cells[columnIndex] ?? '';

    if (isNumericType) {
      const numericResult = matchesNumericFilter(cellText, filterText, columnType);
      if (numericResult !== null) return numericResult;
    }

    return cellText.toLowerCase().includes(filterText.trim().toLowerCase());
  });
}
