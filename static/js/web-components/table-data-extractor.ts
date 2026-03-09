import { detectColumnType } from './column-type-detector.js';
import type { ColumnTypeInfo } from './column-type-detector.js';

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
