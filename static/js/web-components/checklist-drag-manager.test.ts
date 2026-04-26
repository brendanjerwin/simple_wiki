import { expect } from '@open-wc/testing';
import { reorderItems, computeSortOrder } from './checklist-drag-manager.js';

interface TestItem {
  text: string;
  tags: string[];
}

describe('reorderItems', () => {
  describe('when moving from earlier to later index', () => {
    let result: TestItem[];

    beforeEach(() => {
      const items: TestItem[] = [
        { text: 'A', tags: [] },
        { text: 'B', tags: [] },
        { text: 'C', tags: [] },
        { text: 'D', tags: [] },
      ];
      result = reorderItems(items, 0, 3);
    });

    it('should place the moved item before the target', () => {
      expect(result.map(i => i.text)).to.deep.equal(['B', 'C', 'A', 'D']);
    });
  });

  describe('when moving from later to earlier index', () => {
    let result: TestItem[];

    beforeEach(() => {
      const items: TestItem[] = [
        { text: 'A', tags: [] },
        { text: 'B', tags: [] },
        { text: 'C', tags: [] },
        { text: 'D', tags: [] },
      ];
      result = reorderItems(items, 3, 1);
    });

    it('should place the moved item before the target', () => {
      expect(result.map(i => i.text)).to.deep.equal(['A', 'D', 'B', 'C']);
    });
  });

  describe('when moving item to same index', () => {
    let result: TestItem[];

    beforeEach(() => {
      const items: TestItem[] = [
        { text: 'A', tags: [] },
        { text: 'B', tags: [] },
      ];
      result = reorderItems(items, 1, 1);
    });

    it('should not change the order', () => {
      expect(result.map(i => i.text)).to.deep.equal(['A', 'B']);
    });
  });

  describe('when fromIndex is out of range', () => {
    let items: TestItem[];
    let result: TestItem[];

    beforeEach(() => {
      items = [{ text: 'A', tags: [] }];
      result = reorderItems(items, 5, 0);
    });

    it('should return the original items', () => {
      expect(result).to.deep.equal(items);
    });
  });

  describe('when moving Eggs above Milk in Dairy group (plan example)', () => {
    let result: TestItem[];

    beforeEach(() => {
      const items: TestItem[] = [
        { text: 'Milk', tags: ['dairy'] },
        { text: 'Bread', tags: ['bakery'] },
        { text: 'Apples', tags: ['produce'] },
        { text: 'Eggs', tags: ['dairy'] },
      ];
      result = reorderItems(items, 3, 0);
    });

    it('should place Eggs first in the array', () => {
      expect(result.map(i => i.text)).to.deep.equal([
        'Eggs',
        'Milk',
        'Bread',
        'Apples',
      ]);
    });
  });
});

describe('computeSortOrder', () => {
  describe('when inserting into an empty list', () => {
    let result: bigint;

    beforeEach(() => {
      result = computeSortOrder([], 0, 'new');
    });

    it('should return 1000', () => {
      expect(result).to.equal(1000n);
    });
  });

  describe('when inserting at the start of the list', () => {
    let result: bigint;

    beforeEach(() => {
      const items = [
        { uid: 'a', sortOrder: 1000n },
        { uid: 'b', sortOrder: 2000n },
      ];
      result = computeSortOrder(items, 0, 'c');
    });

    it('should return a value below the first item', () => {
      expect(result).to.equal(0n);
    });
  });

  describe('when inserting at the end of the list', () => {
    let result: bigint;

    beforeEach(() => {
      const items = [
        { uid: 'a', sortOrder: 1000n },
        { uid: 'b', sortOrder: 2000n },
      ];
      result = computeSortOrder(items, 2, 'c');
    });

    it('should return last + 1000', () => {
      expect(result).to.equal(3000n);
    });
  });

  describe('when inserting between two items', () => {
    let result: bigint;

    beforeEach(() => {
      const items = [
        { uid: 'a', sortOrder: 1000n },
        { uid: 'b', sortOrder: 2000n },
      ];
      result = computeSortOrder(items, 1, 'c');
    });

    it('should return the midpoint', () => {
      expect(result).to.equal(1500n);
    });
  });

  describe('when moving an item from index 2 to index 0 (drag-up)', () => {
    let result: bigint;

    beforeEach(() => {
      const items = [
        { uid: 'a', sortOrder: 1000n },
        { uid: 'b', sortOrder: 2000n },
        { uid: 'c', sortOrder: 3000n },
      ];
      // Plan move: c -> position 0 (before a)
      result = computeSortOrder(items, 0, 'c');
    });

    it('should land below the new neighbor', () => {
      expect(result).to.equal(0n);
    });
  });

  describe('when moving an item from index 0 to index 3 in a list of 3 (drag-down to end)', () => {
    let result: bigint;

    beforeEach(() => {
      const items = [
        { uid: 'a', sortOrder: 1000n },
        { uid: 'b', sortOrder: 2000n },
        { uid: 'c', sortOrder: 3000n },
      ];
      // Plan move: a -> position 3 means "after c"; without 'a' the list is [b,c]
      // and target index becomes 2 (after the splice), which is end-of-list.
      result = computeSortOrder(items, 3, 'a');
    });

    it('should land after the last remaining item', () => {
      expect(result).to.equal(4000n);
    });
  });

  describe('when neighbors collide (sort_order values are adjacent integers)', () => {
    let result: bigint;

    beforeEach(() => {
      const items = [
        { uid: 'a', sortOrder: 5n },
        { uid: 'b', sortOrder: 6n },
      ];
      result = computeSortOrder(items, 1, 'c');
    });

    it('should fall back to before + 1000 when midpoint collides', () => {
      // Midpoint is 5; collides with 'a'. Fallback: 5 + 1000.
      expect(result).to.equal(1005n);
    });
  });
});
