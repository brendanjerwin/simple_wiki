import { expect } from '@open-wc/testing';
import { extractChecklistData } from './checklist-data-service.js';
import type { JsonObject } from '@bufbuild/protobuf';

describe('extractChecklistData', () => {
  describe('when frontmatter has no checklists', () => {
    let result: ReturnType<typeof extractChecklistData>;

    beforeEach(() => {
      const frontmatter: JsonObject = { title: 'Test' };
      result = extractChecklistData(frontmatter, 'grocery_list');
    });

    it('should return empty items', () => {
      expect(result.items).to.deep.equal([]);
    });
  });

  describe('when listName is not in checklists', () => {
    let result: ReturnType<typeof extractChecklistData>;

    beforeEach(() => {
      const frontmatter: JsonObject = {
        checklists: { other_list: { items: [] } },
      };
      result = extractChecklistData(frontmatter, 'grocery_list');
    });

    it('should return empty items', () => {
      expect(result.items).to.deep.equal([]);
    });
  });

  describe('when checklists contain items with new tags array format', () => {
    let result: ReturnType<typeof extractChecklistData>;

    beforeEach(() => {
      const frontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [
              { text: 'Milk', checked: false },
              { text: 'Eggs', checked: true, tags: ['dairy', 'fridge'] },
            ],
          },
        },
      };
      result = extractChecklistData(frontmatter, 'grocery_list');
    });

    it('should extract the correct number of items', () => {
      expect(result.items).to.have.length(2);
    });

    it('should extract plain items with empty tags array', () => {
      expect(result.items[0]).to.deep.equal({ text: 'Milk', checked: false, tags: [] });
    });

    it('should extract items with tags array correctly', () => {
      expect(result.items[1]).to.deep.equal({
        text: 'Eggs',
        checked: true,
        tags: ['dairy', 'fridge'],
      });
    });
  });

  describe('when checklists contain items with old tag string format (backward-compatible)', () => {
    let result: ReturnType<typeof extractChecklistData>;

    beforeEach(() => {
      const frontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [
              { text: 'Milk', checked: false },
              { text: 'Eggs', checked: true, tag: 'Dairy' },
            ],
          },
        },
      };
      result = extractChecklistData(frontmatter, 'grocery_list');
    });

    it('should extract the correct number of items', () => {
      expect(result.items).to.have.length(2);
    });

    it('should wrap old tag string in an array', () => {
      expect(result.items[1]?.tags).to.deep.equal(['Dairy']);
    });
  });

  describe('when item has both tag and tags (tags takes precedence)', () => {
    let result: ReturnType<typeof extractChecklistData>;

    beforeEach(() => {
      const frontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [
              { text: 'Eggs', checked: false, tag: 'old', tags: ['new1', 'new2'] },
            ],
          },
        },
      };
      result = extractChecklistData(frontmatter, 'grocery_list');
    });

    it('should prefer the tags array over the tag string', () => {
      expect(result.items[0]?.tags).to.deep.equal(['new1', 'new2']);
    });
  });
});
