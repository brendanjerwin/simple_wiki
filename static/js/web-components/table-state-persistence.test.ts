import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import {
  computeTableHash,
  saveTableState,
  loadTableState,
  serializeFilter,
  deserializeFilter,
} from './table-state-persistence.js';
import type { ColumnFilterState } from './table-sorter-filterer.js';

describe('table-state-persistence', () => {

  describe('computeTableHash', () => {

    describe('when given headers and cell values', () => {
      let result: string;

      beforeEach(() => {
        result = computeTableHash(
          ['Name', 'Price'],
          [['Widget', '$9.99'], ['Gadget', '$24.50']],
        );
      });

      it('should return a non-empty string', () => {
        expect(result).to.be.a('string');
        expect(result.length).to.be.greaterThan(0);
      });
    });

    describe('when given the same data twice', () => {
      let hash1: string;
      let hash2: string;

      beforeEach(() => {
        const headers = ['Name', 'Value'];
        const cells = [['A', '1'], ['B', '2']];
        hash1 = computeTableHash(headers, cells);
        hash2 = computeTableHash(headers, cells);
      });

      it('should produce the same hash', () => {
        expect(hash1).to.equal(hash2);
      });
    });

    describe('when given different data', () => {
      let hash1: string;
      let hash2: string;

      beforeEach(() => {
        hash1 = computeTableHash(['Name'], [['A']]);
        hash2 = computeTableHash(['Name'], [['B']]);
      });

      it('should produce different hashes', () => {
        expect(hash1).to.not.equal(hash2);
      });
    });

    describe('when given different headers', () => {
      let hash1: string;
      let hash2: string;

      beforeEach(() => {
        hash1 = computeTableHash(['Name'], [['A']]);
        hash2 = computeTableHash(['Title'], [['A']]);
      });

      it('should produce different hashes', () => {
        expect(hash1).to.not.equal(hash2);
      });
    });
  });

  describe('serializeFilter', () => {

    describe('when serializing a checkbox filter', () => {
      let result: ReturnType<typeof serializeFilter>;

      beforeEach(() => {
        const filter: ColumnFilterState = {
          kind: 'checkbox',
          excludedValues: new Set(['Apple', 'Banana']),
        };
        result = serializeFilter(filter);
      });

      it('should have kind checkbox', () => {
        expect(result.kind).to.equal('checkbox');
      });

      it('should convert Set to sorted array', () => {
        expect(result.excludedValues).to.deep.equal(['Apple', 'Banana']);
      });
    });

    describe('when serializing a range filter', () => {
      let result: ReturnType<typeof serializeFilter>;

      beforeEach(() => {
        const filter: ColumnFilterState = { kind: 'range', min: 10, max: 20 };
        result = serializeFilter(filter);
      });

      it('should have kind range', () => {
        expect(result.kind).to.equal('range');
      });

      it('should preserve min and max', () => {
        expect(result.min).to.equal(10);
        expect(result.max).to.equal(20);
      });
    });

    describe('when serializing a text-search filter', () => {
      let result: ReturnType<typeof serializeFilter>;

      beforeEach(() => {
        const filter: ColumnFilterState = { kind: 'text-search', searchText: 'hello' };
        result = serializeFilter(filter);
      });

      it('should have kind text-search', () => {
        expect(result.kind).to.equal('text-search');
      });

      it('should preserve searchText', () => {
        expect(result.searchText).to.equal('hello');
      });
    });
  });

  describe('deserializeFilter', () => {

    describe('when deserializing a checkbox filter', () => {
      let result: ColumnFilterState;

      beforeEach(() => {
        result = deserializeFilter({ kind: 'checkbox', excludedValues: ['Apple', 'Banana'] });
      });

      it('should have kind checkbox', () => {
        expect(result.kind).to.equal('checkbox');
      });

      it('should convert array back to Set', () => {
        if (result.kind !== 'checkbox') throw new Error('wrong kind');
        expect(result.excludedValues).to.be.instanceOf(Set);
        expect(result.excludedValues.has('Apple')).to.be.true;
        expect(result.excludedValues.has('Banana')).to.be.true;
      });
    });

    describe('when deserializing a range filter', () => {
      let result: ColumnFilterState;

      beforeEach(() => {
        result = deserializeFilter({ kind: 'range', min: 5, max: 15 });
      });

      it('should have kind range', () => {
        expect(result.kind).to.equal('range');
      });

      it('should preserve min and max', () => {
        if (result.kind !== 'range') throw new Error('wrong kind');
        expect(result.min).to.equal(5);
        expect(result.max).to.equal(15);
      });
    });

    describe('when deserializing a text-search filter', () => {
      let result: ColumnFilterState;

      beforeEach(() => {
        result = deserializeFilter({ kind: 'text-search', searchText: 'world' });
      });

      it('should have kind text-search', () => {
        expect(result.kind).to.equal('text-search');
      });

      it('should preserve searchText', () => {
        if (result.kind !== 'text-search') throw new Error('wrong kind');
        expect(result.searchText).to.equal('world');
      });
    });
  });

  describe('saveTableState and loadTableState', () => {

    afterEach(() => {
      sinon.restore();
      localStorage.clear();
    });

    describe('when saving and loading state', () => {
      let loaded: ReturnType<typeof loadTableState>;

      beforeEach(() => {
        const filters = new Map<number, ColumnFilterState>([
          [0, { kind: 'checkbox', excludedValues: new Set(['X']) }],
          [1, { kind: 'range', min: 5, max: 10 }],
        ]);
        saveTableState('test-hash', 2, 'ascending', filters);
        loaded = loadTableState('test-hash');
      });

      it('should return the saved state', () => {
        expect(loaded).to.not.be.null;
      });

      it('should preserve sortColumnIndex', () => {
        expect(loaded!.sortColumnIndex).to.equal(2);
      });

      it('should preserve sortDirection', () => {
        expect(loaded!.sortDirection).to.equal('ascending');
      });

      it('should preserve filters', () => {
        expect(loaded!.filters).to.have.length(2);
      });
    });

    describe('when no state is saved', () => {
      let loaded: ReturnType<typeof loadTableState>;

      beforeEach(() => {
        loaded = loadTableState('nonexistent-hash');
      });

      it('should return null', () => {
        expect(loaded).to.be.null;
      });
    });

    describe('when state is expired', () => {
      let loaded: ReturnType<typeof loadTableState>;
      let clock: sinon.SinonFakeTimers;

      beforeEach(() => {
        clock = sinon.useFakeTimers();
        saveTableState('expired-hash', 0, 'ascending', new Map());
        clock.tick(91 * 24 * 60 * 60 * 1000); // 91 days
        loaded = loadTableState('expired-hash');
        clock.restore();
      });

      it('should return null', () => {
        expect(loaded).to.be.null;
      });

      it('should remove the expired entry from localStorage', () => {
        expect(localStorage.getItem('wiki-table-state:expired-hash')).to.be.null;
      });
    });

    describe('when localStorage has invalid JSON', () => {
      let loaded: ReturnType<typeof loadTableState>;

      beforeEach(() => {
        localStorage.setItem('wiki-table-state:bad-data', 'not json');
        loaded = loadTableState('bad-data');
      });

      it('should return null', () => {
        expect(loaded).to.be.null;
      });
    });

    describe('when saving with no active filters and no sort', () => {
      let loaded: ReturnType<typeof loadTableState>;

      beforeEach(() => {
        saveTableState('empty-hash', null, 'none', new Map());
        loaded = loadTableState('empty-hash');
      });

      it('should return the saved state', () => {
        expect(loaded).to.not.be.null;
        expect(loaded!.sortColumnIndex).to.be.null;
        expect(loaded!.sortDirection).to.equal('none');
        expect(loaded!.filters).to.have.length(0);
      });
    });
  });
});
