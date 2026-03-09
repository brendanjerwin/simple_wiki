import { expect } from '@open-wc/testing';
import { extractTableData } from './table-data-extractor.js';
import type { ExtractedTableData } from './table-data-extractor.js';

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

      it('should detect number type for count column', () => {
        expect(result.columns[1]!.typeInfo.detectedType).to.equal('number');
      });

      it('should detect percentage type for completion column', () => {
        expect(result.columns[2]!.typeInfo.detectedType).to.equal('percentage');
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
});
