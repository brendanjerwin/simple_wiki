import { detectColumnType } from './column-type-detector.js';
import type { ColumnTypeInfo, ColumnDataType } from './column-type-detector.js';
import {
  parseNumericValue,
  parseCurrencyValue,
  parsePercentageValue,
  parseDateValue,
} from './column-type-detector.js';

export interface TableColumnDefinition {
  headerText: string;
  typeInfo: ColumnTypeInfo;
  columnIndex: number;
}

export interface TableRowData {
  cells: string[];
  originalIndex: number;
}

export interface ExtractedTableData {
  columns: TableColumnDefinition[];
  rows: TableRowData[];
}

export function extractTableData(tableElement: HTMLTableElement): ExtractedTableData {
  const thead = tableElement.querySelector('thead');
  const tbody = tableElement.querySelector('tbody');

  let headerCells: string[];
  let dataRows: HTMLTableRowElement[];

  if (thead) {
    const headerRow = thead.querySelector('tr');
    headerCells = headerRow
      ? Array.from(headerRow.querySelectorAll('th, td')).map(cell => cell.textContent?.trim() ?? '')
      : [];
    dataRows = tbody
      ? Array.from(tbody.querySelectorAll('tr'))
      : [];
  } else {
    const allRows = Array.from(tableElement.querySelectorAll('tr'));
    const firstRow = allRows[0];
    headerCells = firstRow
      ? Array.from(firstRow.querySelectorAll('th, td')).map(cell => cell.textContent?.trim() ?? '')
      : [];
    dataRows = allRows.slice(1) as HTMLTableRowElement[];
  }

  const rows: TableRowData[] = dataRows.map((row, index) => ({
    cells: Array.from(row.querySelectorAll('td, th')).map(cell => cell.textContent?.trim() ?? ''),
    originalIndex: index,
  }));

  const columns: TableColumnDefinition[] = headerCells.map((headerText, columnIndex) => {
    const columnValues = rows.map(row => row.cells[columnIndex] ?? '');
    return {
      headerText,
      typeInfo: detectColumnType(columnValues),
      columnIndex,
    };
  });

  return { columns, rows };
}

export function getUniqueColumnValues(rows: TableRowData[], columnIndex: number): string[] {
  const seen = new Set<string>();
  for (const row of rows) {
    const value = row.cells[columnIndex] ?? '';
    if (value !== '') {
      seen.add(value);
    }
  }
  return Array.from(seen).sort();
}

function parseForColumnType(text: string, columnType: ColumnDataType): number {
  switch (columnType) {
    case 'integer':
    case 'decimal': return parseNumericValue(text);
    case 'currency': return parseCurrencyValue(text);
    case 'percentage': return parsePercentageValue(text);
    case 'date': return parseDateValue(text);
    default: return NaN;
  }
}

export function getColumnNumericRange(
  rows: TableRowData[],
  columnIndex: number,
  columnType: ColumnDataType,
): { min: number; max: number } | null {
  if (columnType === 'text') return null;

  let min = Infinity;
  let max = -Infinity;
  let hasValue = false;

  for (const row of rows) {
    const cellText = row.cells[columnIndex] ?? '';
    const value = parseForColumnType(cellText, columnType);
    if (!Number.isNaN(value)) {
      hasValue = true;
      if (value < min) min = value;
      if (value > max) max = value;
    }
  }

  return hasValue ? { min, max } : null;
}
