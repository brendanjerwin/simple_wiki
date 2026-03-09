import { expect } from '@open-wc/testing';
import { sortRows, filterRows } from './table-sorter-filterer.js';
import type { TableRowData } from './table-data-extractor.js';

function makeRows(values: string[][]): TableRowData[] {
  return values.map((cells, i) => ({ cells, originalIndex: i }));
}

function extractColumn(rows: TableRowData[], colIndex: number): string[] {
  return rows.map(r => r.cells[colIndex]!);
}

describe('table-sorter-filterer', () => {

  describe('sortRows', () => {

    describe('when direction is none', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['B'], ['A'], ['C']]);
        result = sortRows(rows, 0, 'none', 'text');
      });

      it('should return rows in original order', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['B', 'A', 'C']);
      });
    });

    describe('when sorting text ascending', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['Banana'], ['Apple'], ['Cherry']]);
        result = sortRows(rows, 0, 'ascending', 'text');
      });

      it('should sort alphabetically', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['Apple', 'Banana', 'Cherry']);
      });
    });

    describe('when sorting text descending', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['Banana'], ['Apple'], ['Cherry']]);
        result = sortRows(rows, 0, 'descending', 'text');
      });

      it('should sort reverse alphabetically', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['Cherry', 'Banana', 'Apple']);
      });
    });

    describe('when sorting text case-insensitively', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['banana'], ['Apple'], ['cherry']]);
        result = sortRows(rows, 0, 'ascending', 'text');
      });

      it('should ignore case', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['Apple', 'banana', 'cherry']);
      });
    });

    describe('when sorting numbers ascending', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['10'], ['2'], ['100'], ['1']]);
        result = sortRows(rows, 0, 'ascending', 'number');
      });

      it('should sort numerically, not lexicographically', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['1', '2', '10', '100']);
      });
    });

    describe('when sorting numbers descending', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['10'], ['2'], ['100']]);
        result = sortRows(rows, 0, 'descending', 'number');
      });

      it('should sort numerically descending', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['100', '10', '2']);
      });
    });

    describe('when sorting currency ascending', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['$24.50'], ['$9.99'], ['$100.00']]);
        result = sortRows(rows, 0, 'ascending', 'currency');
      });

      it('should sort by parsed currency value', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['$9.99', '$24.50', '$100.00']);
      });
    });

    describe('when sorting percentages ascending', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['75%'], ['25%'], ['100%'], ['50%']]);
        result = sortRows(rows, 0, 'ascending', 'percentage');
      });

      it('should sort by parsed percentage value', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['25%', '50%', '75%', '100%']);
      });
    });

    describe('when sorting dates ascending', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['2024-03-10'], ['2024-01-15'], ['2024-02-20']]);
        result = sortRows(rows, 0, 'ascending', 'date');
      });

      it('should sort chronologically', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['2024-01-15', '2024-02-20', '2024-03-10']);
      });
    });

    describe('when sorting does not mutate original', () => {
      let original: TableRowData[];
      let originalCopy: string[];

      beforeEach(() => {
        original = makeRows([['C'], ['A'], ['B']]);
        originalCopy = extractColumn(original, 0);
        sortRows(original, 0, 'ascending', 'text');
      });

      it('should not modify the input array', () => {
        expect(extractColumn(original, 0)).to.deep.equal(originalCopy);
      });
    });
  });

  describe('filterRows', () => {

    describe('when filtering text with substring match', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['Apple'], ['Banana'], ['Apricot'], ['Cherry']]);
        result = filterRows(rows, 0, 'ap', 'text');
      });

      it('should return matching rows case-insensitively', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['Apple', 'Apricot']);
      });
    });

    describe('when filter text is empty', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['Apple'], ['Banana']]);
        result = filterRows(rows, 0, '', 'text');
      });

      it('should return all rows', () => {
        expect(result).to.have.length(2);
      });
    });

    describe('when filtering numbers with greater-than operator', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['5'], ['10'], ['15'], ['20']]);
        result = filterRows(rows, 0, '>10', 'number');
      });

      it('should return rows with values greater than 10', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['15', '20']);
      });
    });

    describe('when filtering numbers with less-than operator', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['5'], ['10'], ['15'], ['20']]);
        result = filterRows(rows, 0, '<15', 'number');
      });

      it('should return rows with values less than 15', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['5', '10']);
      });
    });

    describe('when filtering numbers with greater-or-equal operator', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['5'], ['10'], ['15']]);
        result = filterRows(rows, 0, '>=10', 'number');
      });

      it('should return rows with values >= 10', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['10', '15']);
      });
    });

    describe('when filtering numbers with less-or-equal operator', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['5'], ['10'], ['15']]);
        result = filterRows(rows, 0, '<=10', 'number');
      });

      it('should return rows with values <= 10', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['5', '10']);
      });
    });

    describe('when filtering numbers with equals operator', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['5'], ['10'], ['15']]);
        result = filterRows(rows, 0, '=10', 'number');
      });

      it('should return rows with value equal to 10', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['10']);
      });
    });

    describe('when filtering numbers with plain text (substring)', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['5'], ['10'], ['15'], ['100']]);
        result = filterRows(rows, 0, '10', 'number');
      });

      it('should fall back to substring match', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['10', '100']);
      });
    });

    describe('when filtering currency with greater-than operator', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['$5.00'], ['$10.00'], ['$20.00']]);
        result = filterRows(rows, 0, '>10', 'currency');
      });

      it('should compare parsed currency values', () => {
        expect(extractColumn(result, 0)).to.deep.equal(['$20.00']);
      });
    });

    describe('when filtering does not mutate original', () => {
      let original: TableRowData[];

      beforeEach(() => {
        original = makeRows([['Apple'], ['Banana']]);
        filterRows(original, 0, 'ap', 'text');
      });

      it('should not modify the input array', () => {
        expect(original).to.have.length(2);
      });
    });
  });
});
