import { expect } from '@open-wc/testing';
import { reorderItems } from './checklist-drag-manager.js';
import type { ChecklistItem } from './checklist-tag-parser.js';

describe('reorderItems', () => {
  describe('when moving from earlier to later index', () => {
    let result: ChecklistItem[];

    beforeEach(() => {
      const items: ChecklistItem[] = [
        { text: 'A', checked: false, tags: [] },
        { text: 'B', checked: false, tags: [] },
        { text: 'C', checked: false, tags: [] },
        { text: 'D', checked: false, tags: [] },
      ];
      result = reorderItems(items, 0, 3);
    });

    it('should place the moved item before the target', () => {
      expect(result.map(i => i.text)).to.deep.equal(['B', 'C', 'A', 'D']);
    });
  });

  describe('when moving from later to earlier index', () => {
    let result: ChecklistItem[];

    beforeEach(() => {
      const items: ChecklistItem[] = [
        { text: 'A', checked: false, tags: [] },
        { text: 'B', checked: false, tags: [] },
        { text: 'C', checked: false, tags: [] },
        { text: 'D', checked: false, tags: [] },
      ];
      result = reorderItems(items, 3, 1);
    });

    it('should place the moved item before the target', () => {
      expect(result.map(i => i.text)).to.deep.equal(['A', 'D', 'B', 'C']);
    });
  });

  describe('when moving item to same index', () => {
    let result: ChecklistItem[];

    beforeEach(() => {
      const items: ChecklistItem[] = [
        { text: 'A', checked: false, tags: [] },
        { text: 'B', checked: false, tags: [] },
      ];
      result = reorderItems(items, 1, 1);
    });

    it('should not change the order', () => {
      expect(result.map(i => i.text)).to.deep.equal(['A', 'B']);
    });
  });

  describe('when fromIndex is out of range', () => {
    let items: ChecklistItem[];
    let result: ChecklistItem[];

    beforeEach(() => {
      items = [{ text: 'A', checked: false, tags: [] }];
      result = reorderItems(items, 5, 0);
    });

    it('should return the original items', () => {
      expect(result).to.deep.equal(items);
    });
  });

  describe('when moving Eggs above Milk in Dairy group (plan example)', () => {
    let result: ChecklistItem[];

    beforeEach(() => {
      const items: ChecklistItem[] = [
        { text: 'Milk', checked: false, tags: ['dairy'] },
        { text: 'Bread', checked: false, tags: ['bakery'] },
        { text: 'Apples', checked: false, tags: ['produce'] },
        { text: 'Eggs', checked: false, tags: ['dairy'] },
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
