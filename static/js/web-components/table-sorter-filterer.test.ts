import { expect } from '@open-wc/testing';
import {
  sortRows,
  filterRows,
  applyColumnFilter,
  applyAllFilters,
  isFilterActive,
  hasActiveFilters,
} from './table-sorter-filterer.js';
import type {
  CheckboxFilterState,
  RangeFilterState,
  TextSearchFilterState,
  ColumnFilterState,
  TableFilterState,
} from './table-sorter-filterer.js';
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

  describe('applyColumnFilter', () => {

    describe('when using checkbox filter', () => {

      describe('when excluding specific values', () => {
        let result: TableRowData[];

        beforeEach(() => {
          const rows = makeRows([['Apple'], ['Banana'], ['Cherry'], ['Apple']]);
          const filter: CheckboxFilterState = {
            kind: 'checkbox',
            excludedValues: new Set(['Apple']),
          };
          result = applyColumnFilter(rows, 0, filter, 'text');
        });

        it('should exclude rows with excluded values', () => {
          expect(extractColumn(result, 0)).to.deep.equal(['Banana', 'Cherry']);
        });
      });

      describe('when excluding multiple values', () => {
        let result: TableRowData[];

        beforeEach(() => {
          const rows = makeRows([['Apple'], ['Banana'], ['Cherry']]);
          const filter: CheckboxFilterState = {
            kind: 'checkbox',
            excludedValues: new Set(['Apple', 'Cherry']),
          };
          result = applyColumnFilter(rows, 0, filter, 'text');
        });

        it('should exclude all specified values', () => {
          expect(extractColumn(result, 0)).to.deep.equal(['Banana']);
        });
      });

      describe('when no values are excluded', () => {
        let result: TableRowData[];

        beforeEach(() => {
          const rows = makeRows([['Apple'], ['Banana']]);
          const filter: CheckboxFilterState = {
            kind: 'checkbox',
            excludedValues: new Set(),
          };
          result = applyColumnFilter(rows, 0, filter, 'text');
        });

        it('should return all rows', () => {
          expect(result).to.have.length(2);
        });
      });
    });

    describe('when using range filter', () => {

      describe('when filtering with min only', () => {
        let result: TableRowData[];

        beforeEach(() => {
          const rows = makeRows([['5'], ['10'], ['15'], ['20']]);
          const filter: RangeFilterState = { kind: 'range', min: 10, max: null };
          result = applyColumnFilter(rows, 0, filter, 'number');
        });

        it('should exclude rows below min', () => {
          expect(extractColumn(result, 0)).to.deep.equal(['10', '15', '20']);
        });
      });

      describe('when filtering with max only', () => {
        let result: TableRowData[];

        beforeEach(() => {
          const rows = makeRows([['5'], ['10'], ['15'], ['20']]);
          const filter: RangeFilterState = { kind: 'range', min: null, max: 15 };
          result = applyColumnFilter(rows, 0, filter, 'number');
        });

        it('should exclude rows above max', () => {
          expect(extractColumn(result, 0)).to.deep.equal(['5', '10', '15']);
        });
      });

      describe('when filtering with min and max', () => {
        let result: TableRowData[];

        beforeEach(() => {
          const rows = makeRows([['5'], ['10'], ['15'], ['20']]);
          const filter: RangeFilterState = { kind: 'range', min: 10, max: 15 };
          result = applyColumnFilter(rows, 0, filter, 'number');
        });

        it('should include only rows within range', () => {
          expect(extractColumn(result, 0)).to.deep.equal(['10', '15']);
        });
      });

      describe('when filtering currency with range', () => {
        let result: TableRowData[];

        beforeEach(() => {
          const rows = makeRows([['$5.00'], ['$10.00'], ['$20.00']]);
          const filter: RangeFilterState = { kind: 'range', min: 10, max: null };
          result = applyColumnFilter(rows, 0, filter, 'currency');
        });

        it('should parse currency and filter by range', () => {
          expect(extractColumn(result, 0)).to.deep.equal(['$10.00', '$20.00']);
        });
      });

      describe('when values cannot be parsed', () => {
        let result: TableRowData[];

        beforeEach(() => {
          const rows = makeRows([['abc'], ['10'], ['xyz']]);
          const filter: RangeFilterState = { kind: 'range', min: 5, max: 15 };
          result = applyColumnFilter(rows, 0, filter, 'number');
        });

        it('should exclude unparseable values', () => {
          expect(extractColumn(result, 0)).to.deep.equal(['10']);
        });
      });
    });

    describe('when using text-search filter', () => {

      describe('when searching for substring', () => {
        let result: TableRowData[];

        beforeEach(() => {
          const rows = makeRows([['Apple'], ['Banana'], ['Apricot'], ['Cherry']]);
          const filter: TextSearchFilterState = { kind: 'text-search', searchText: 'ap' };
          result = applyColumnFilter(rows, 0, filter, 'text');
        });

        it('should match case-insensitively', () => {
          expect(extractColumn(result, 0)).to.deep.equal(['Apple', 'Apricot']);
        });
      });

      describe('when search text is whitespace', () => {
        let result: TableRowData[];

        beforeEach(() => {
          const rows = makeRows([['Apple'], ['Banana']]);
          const filter: TextSearchFilterState = { kind: 'text-search', searchText: '   ' };
          result = applyColumnFilter(rows, 0, filter, 'text');
        });

        it('should return all rows', () => {
          expect(result).to.have.length(2);
        });
      });
    });
  });

  describe('applyAllFilters', () => {

    describe('when applying multiple column filters', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([
          ['Apple', '10'],
          ['Banana', '20'],
          ['Apple', '30'],
          ['Cherry', '5'],
        ]);
        const filters: TableFilterState = new Map([
          [0, { kind: 'checkbox', excludedValues: new Set(['Cherry']) } as ColumnFilterState],
          [1, { kind: 'range', min: 15, max: null } as ColumnFilterState],
        ]);
        const columns = [
          { columnIndex: 0, typeInfo: { detectedType: 'text' as const } },
          { columnIndex: 1, typeInfo: { detectedType: 'number' as const } },
        ];
        result = applyAllFilters(rows, filters, columns);
      });

      it('should apply both filters', () => {
        expect(result).to.have.length(2);
        expect(extractColumn(result, 0)).to.deep.equal(['Banana', 'Apple']);
      });
    });

    describe('when no filters are active', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['Apple'], ['Banana']]);
        const filters: TableFilterState = new Map([
          [0, { kind: 'checkbox', excludedValues: new Set() } as ColumnFilterState],
        ]);
        const columns = [
          { columnIndex: 0, typeInfo: { detectedType: 'text' as const } },
        ];
        result = applyAllFilters(rows, filters, columns);
      });

      it('should return all rows', () => {
        expect(result).to.have.length(2);
      });
    });

    describe('when filters map is empty', () => {
      let result: TableRowData[];

      beforeEach(() => {
        const rows = makeRows([['Apple'], ['Banana']]);
        const filters: TableFilterState = new Map();
        const columns = [
          { columnIndex: 0, typeInfo: { detectedType: 'text' as const } },
        ];
        result = applyAllFilters(rows, filters, columns);
      });

      it('should return all rows', () => {
        expect(result).to.have.length(2);
      });
    });
  });

  describe('isFilterActive', () => {

    describe('when checkbox filter has excluded values', () => {
      let result: boolean;

      beforeEach(() => {
        result = isFilterActive({ kind: 'checkbox', excludedValues: new Set(['a']) });
      });

      it('should return true', () => {
        expect(result).to.be.true;
      });
    });

    describe('when checkbox filter has no excluded values', () => {
      let result: boolean;

      beforeEach(() => {
        result = isFilterActive({ kind: 'checkbox', excludedValues: new Set() });
      });

      it('should return false', () => {
        expect(result).to.be.false;
      });
    });

    describe('when range filter has min set', () => {
      let result: boolean;

      beforeEach(() => {
        result = isFilterActive({ kind: 'range', min: 5, max: null });
      });

      it('should return true', () => {
        expect(result).to.be.true;
      });
    });

    describe('when range filter has max set', () => {
      let result: boolean;

      beforeEach(() => {
        result = isFilterActive({ kind: 'range', min: null, max: 10 });
      });

      it('should return true', () => {
        expect(result).to.be.true;
      });
    });

    describe('when range filter has neither min nor max', () => {
      let result: boolean;

      beforeEach(() => {
        result = isFilterActive({ kind: 'range', min: null, max: null });
      });

      it('should return false', () => {
        expect(result).to.be.false;
      });
    });

    describe('when text-search filter has search text', () => {
      let result: boolean;

      beforeEach(() => {
        result = isFilterActive({ kind: 'text-search', searchText: 'hello' });
      });

      it('should return true', () => {
        expect(result).to.be.true;
      });
    });

    describe('when text-search filter has empty search text', () => {
      let result: boolean;

      beforeEach(() => {
        result = isFilterActive({ kind: 'text-search', searchText: '' });
      });

      it('should return false', () => {
        expect(result).to.be.false;
      });
    });

    describe('when text-search filter has whitespace-only search text', () => {
      let result: boolean;

      beforeEach(() => {
        result = isFilterActive({ kind: 'text-search', searchText: '   ' });
      });

      it('should return false', () => {
        expect(result).to.be.false;
      });
    });
  });

  describe('hasActiveFilters', () => {

    describe('when at least one filter is active', () => {
      let result: boolean;

      beforeEach(() => {
        const filters: TableFilterState = new Map([
          [0, { kind: 'checkbox', excludedValues: new Set() }],
          [1, { kind: 'text-search', searchText: 'hello' }],
        ]);
        result = hasActiveFilters(filters);
      });

      it('should return true', () => {
        expect(result).to.be.true;
      });
    });

    describe('when no filters are active', () => {
      let result: boolean;

      beforeEach(() => {
        const filters: TableFilterState = new Map([
          [0, { kind: 'checkbox', excludedValues: new Set() }],
          [1, { kind: 'range', min: null, max: null }],
        ]);
        result = hasActiveFilters(filters);
      });

      it('should return false', () => {
        expect(result).to.be.false;
      });
    });

    describe('when filters map is empty', () => {
      let result: boolean;

      beforeEach(() => {
        result = hasActiveFilters(new Map());
      });

      it('should return false', () => {
        expect(result).to.be.false;
      });
    });
  });
});
