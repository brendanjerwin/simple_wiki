import { expect } from '@open-wc/testing';
import {
  extractTableData,
  getUniqueColumnValues,
  getColumnNumericRange,
} from './table-data-extractor.js';
import type { ExtractedTableData, TableRowData } from './table-data-extractor.js';

function createTable(html: string): HTMLTableElement {
  const container = document.createElement('div');
  container.innerHTML = html;
  return container.querySelector('table')!;
}

describe('table-data-extractor', () => {

  describe('extractTableData', () => {
    let result: ExtractedTableData;

    describe('when given a basic table with thead and tbody', () => {
      beforeEach(() => {
        const table = createTable(`
          <table>
            <thead><tr><th>Name</th><th>Price</th></tr></thead>
            <tbody>
              <tr><td>Widget</td><td>$9.99</td></tr>
              <tr><td>Gadget</td><td>$24.50</td></tr>
            </tbody>
          </table>
        `);
        result = extractTableData(table);
      });

      it('should extract two columns', () => {
        expect(result.columns).to.have.length(2);
      });

      it('should extract header text for first column', () => {
        expect(result.columns[0]!.headerText).to.equal('Name');
      });

      it('should extract header text for second column', () => {
        expect(result.columns[1]!.headerText).to.equal('Price');
      });

      it('should set column indices', () => {
        expect(result.columns[0]!.columnIndex).to.equal(0);
        expect(result.columns[1]!.columnIndex).to.equal(1);
      });

      it('should detect column types', () => {
        expect(result.columns[0]!.typeInfo.detectedType).to.equal('text');
        expect(result.columns[1]!.typeInfo.detectedType).to.equal('currency');
      });

      it('should extract two rows', () => {
        expect(result.rows).to.have.length(2);
      });

      it('should extract cell values for first row', () => {
        expect(result.rows[0]!.cells).to.deep.equal(['Widget', '$9.99']);
      });

      it('should extract cell values for second row', () => {
        expect(result.rows[1]!.cells).to.deep.equal(['Gadget', '$24.50']);
      });

      it('should set original indices', () => {
        expect(result.rows[0]!.originalIndex).to.equal(0);
        expect(result.rows[1]!.originalIndex).to.equal(1);
      });
    });

    describe('when given a table without thead', () => {
      beforeEach(() => {
        const table = createTable(`
          <table>
            <tr><td>A</td><td>B</td></tr>
            <tr><td>1</td><td>2</td></tr>
          </table>
        `);
        result = extractTableData(table);
      });

      it('should use first row as headers', () => {
        expect(result.columns[0]!.headerText).to.equal('A');
        expect(result.columns[1]!.headerText).to.equal('B');
      });

      it('should extract remaining rows as data', () => {
        expect(result.rows).to.have.length(1);
        expect(result.rows[0]!.cells).to.deep.equal(['1', '2']);
      });
    });

    describe('when given a single-row table', () => {
      beforeEach(() => {
        const table = createTable(`
          <table>
            <thead><tr><th>Name</th><th>Value</th></tr></thead>
            <tbody></tbody>
          </table>
        `);
        result = extractTableData(table);
      });

      it('should extract column definitions', () => {
        expect(result.columns).to.have.length(2);
      });

      it('should have empty rows', () => {
        expect(result.rows).to.have.length(0);
      });
    });

    describe('when given a table with numeric data', () => {
      beforeEach(() => {
        const table = createTable(`
          <table>
            <thead><tr><th>Item</th><th>Count</th><th>Completion</th></tr></thead>
            <tbody>
              <tr><td>Alpha</td><td>42</td><td>75%</td></tr>
              <tr><td>Beta</td><td>17</td><td>50%</td></tr>
              <tr><td>Gamma</td><td>99</td><td>100%</td></tr>
            </tbody>
          </table>
        `);
        result = extractTableData(table);
      });

      it('should detect integer type for count column', () => {
        expect(result.columns[1]!.typeInfo.detectedType).to.equal('integer');
      });

      it('should detect percentage type for completion column', () => {
        expect(result.columns[2]!.typeInfo.detectedType).to.equal('percentage');
      });
    });

    describe('when given a table with inline HTML in cells', () => {
      beforeEach(() => {
        const table = createTable(`
          <table>
            <thead><tr><th>Name</th><th>Link</th></tr></thead>
            <tbody>
              <tr><td><strong>Widget</strong></td><td><a href="/page">Details</a></td></tr>
              <tr><td><em>Gadget</em></td><td><code>G-100</code></td></tr>
            </tbody>
          </table>
        `);
        result = extractTableData(table);
      });

      it('should extract plain text for cells', () => {
        expect(result.rows[0]!.cells).to.deep.equal(['Widget', 'Details']);
        expect(result.rows[1]!.cells).to.deep.equal(['Gadget', 'G-100']);
      });

      it('should preserve innerHTML in htmlCells', () => {
        expect(result.rows[0]!.htmlCells[0]).to.equal('<strong>Widget</strong>');
        expect(result.rows[0]!.htmlCells[1]).to.equal('<a href="/page">Details</a>');
        expect(result.rows[1]!.htmlCells[0]).to.equal('<em>Gadget</em>');
        expect(result.rows[1]!.htmlCells[1]).to.equal('<code>G-100</code>');
      });
    });

    describe('when given a table with whitespace in cells', () => {
      beforeEach(() => {
        const table = createTable(`
          <table>
            <thead><tr><th>  Name  </th><th>Value</th></tr></thead>
            <tbody>
              <tr><td>  Widget  </td><td>42</td></tr>
            </tbody>
          </table>
        `);
        result = extractTableData(table);
      });

      it('should trim header whitespace', () => {
        expect(result.columns[0]!.headerText).to.equal('Name');
      });

      it('should trim cell whitespace', () => {
        expect(result.rows[0]!.cells[0]).to.equal('Widget');
      });
    });
  });

  describe('getUniqueColumnValues', () => {

    function makeRows(values: string[][]): TableRowData[] {
      return values.map((cells, i) => ({ cells, htmlCells: cells, originalIndex: i }));
    }

    describe('when column has unique values', () => {
      let result: string[];

      beforeEach(() => {
        const rows = makeRows([['Cherry'], ['Apple'], ['Banana']]);
        result = getUniqueColumnValues(rows, 0);
      });

      it('should return sorted unique values', () => {
        expect(result).to.deep.equal(['Apple', 'Banana', 'Cherry']);
      });
    });

    describe('when column has duplicate values', () => {
      let result: string[];

      beforeEach(() => {
        const rows = makeRows([['Apple'], ['Banana'], ['Apple'], ['Banana']]);
        result = getUniqueColumnValues(rows, 0);
      });

      it('should deduplicate values', () => {
        expect(result).to.deep.equal(['Apple', 'Banana']);
      });
    });

    describe('when column has empty values', () => {
      let result: string[];

      beforeEach(() => {
        const rows = makeRows([['Apple'], [''], ['Banana'], ['']]);
        result = getUniqueColumnValues(rows, 0);
      });

      it('should exclude empty values', () => {
        expect(result).to.deep.equal(['Apple', 'Banana']);
      });
    });

    describe('when column is empty', () => {
      let result: string[];

      beforeEach(() => {
        const rows = makeRows([[''], [''], ['']]);
        result = getUniqueColumnValues(rows, 0);
      });

      it('should return empty array', () => {
        expect(result).to.deep.equal([]);
      });
    });
  });

  describe('getColumnNumericRange', () => {

    function makeRows(values: string[][]): TableRowData[] {
      return values.map((cells, i) => ({ cells, htmlCells: cells, originalIndex: i }));
    }

    describe('when column is numeric', () => {
      let result: { min: number; max: number } | null;

      beforeEach(() => {
        const rows = makeRows([['5'], ['10'], ['3'], ['20']]);
        result = getColumnNumericRange(rows, 0, 'integer');
      });

      it('should return min and max', () => {
        expect(result).to.deep.equal({ min: 3, max: 20 });
      });
    });

    describe('when column is currency', () => {
      let result: { min: number; max: number } | null;

      beforeEach(() => {
        const rows = makeRows([['$9.99'], ['$24.50'], ['$1.50']]);
        result = getColumnNumericRange(rows, 0, 'currency');
      });

      it('should return parsed currency min and max', () => {
        expect(result).to.deep.equal({ min: 1.5, max: 24.5 });
      });
    });

    describe('when column is percentage', () => {
      let result: { min: number; max: number } | null;

      beforeEach(() => {
        const rows = makeRows([['25%'], ['75%'], ['50%']]);
        result = getColumnNumericRange(rows, 0, 'percentage');
      });

      it('should return parsed percentage min and max', () => {
        expect(result).to.deep.equal({ min: 25, max: 75 });
      });
    });

    describe('when column is date', () => {
      let result: { min: number; max: number } | null;

      beforeEach(() => {
        const rows = makeRows([['2024-01-15'], ['2024-06-20'], ['2024-03-10']]);
        result = getColumnNumericRange(rows, 0, 'date');
      });

      it('should return null', () => {
        expect(result).to.equal(null);
      });
    });

    describe('when column is text', () => {
      let result: { min: number; max: number } | null;

      beforeEach(() => {
        const rows = makeRows([['Apple'], ['Banana']]);
        result = getColumnNumericRange(rows, 0, 'text');
      });

      it('should return null', () => {
        expect(result).to.equal(null);
      });
    });

    describe('when no values are parseable', () => {
      let result: { min: number; max: number } | null;

      beforeEach(() => {
        const rows = makeRows([['abc'], ['xyz']]);
        result = getColumnNumericRange(rows, 0, 'integer');
      });

      it('should return null', () => {
        expect(result).to.equal(null);
      });
    });

    describe('when some values are unparseable', () => {
      let result: { min: number; max: number } | null;

      beforeEach(() => {
        const rows = makeRows([['5'], ['abc'], ['15']]);
        result = getColumnNumericRange(rows, 0, 'integer');
      });

      it('should compute range from parseable values only', () => {
        expect(result).to.deep.equal({ min: 5, max: 15 });
      });
    });
  });
});
