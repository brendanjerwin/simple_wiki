import { expect } from '@esm-bundle/chai';
import {
  extractInventoryData,
  validateInventoryResponse,
  findPageInInventoryList,
} from './inventory-menu.js';

describe('Inventory Menu Logic', () => {
  describe('extractInventoryData', () => {

    describe('when frontmatter is null', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData(null);
      });

      it('should return null inventory', () => {
        expect(result.inventory).to.be.null;
      });

      it('should not be a container', () => {
        expect(result.isContainer).to.be.false;
      });

      it('should not be an item', () => {
        expect(result.isItem).to.be.false;
      });

      it('should have empty currentContainer', () => {
        expect(result.currentContainer).to.equal('');
      });
    });

    describe('when frontmatter is undefined', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData(undefined);
      });

      it('should return null inventory', () => {
        expect(result.inventory).to.be.null;
      });

      it('should not be a container', () => {
        expect(result.isContainer).to.be.false;
      });

      it('should not be an item', () => {
        expect(result.isItem).to.be.false;
      });
    });

    describe('when frontmatter is empty object', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({});
      });

      it('should return null/undefined inventory', () => {
        expect(result.inventory).to.be.undefined;
      });

      it('should not be a container', () => {
        expect(result.isContainer).to.be.false;
      });

      it('should not be an item', () => {
        expect(result.isItem).to.be.false;
      });
    });

    describe('when frontmatter has empty inventory object', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({ inventory: {} });
      });

      it('should return inventory object', () => {
        expect(result.inventory).to.deep.equal({});
      });

      it('should not be a container', () => {
        expect(result.isContainer).to.be.false;
      });

      it('should not be an item', () => {
        expect(result.isItem).to.be.false;
      });
    });

    describe('when frontmatter has inventory with empty items array', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({ inventory: { items: [] } });
      });

      it('should be a container', () => {
        expect(result.isContainer).to.be.true;
      });

      it('should not be an item', () => {
        expect(result.isItem).to.be.false;
      });
    });

    describe('when frontmatter has inventory with items array', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({ inventory: { items: ['item1', 'item2'] } });
      });

      it('should be a container', () => {
        expect(result.isContainer).to.be.true;
      });

      it('should not be an item', () => {
        expect(result.isItem).to.be.false;
      });
    });

    describe('when frontmatter has inventory with container string', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({ inventory: { container: 'parent_container' } });
      });

      it('should not be a container', () => {
        expect(result.isContainer).to.be.false;
      });

      it('should be an item', () => {
        expect(result.isItem).to.be.true;
      });

      it('should have currentContainer set', () => {
        expect(result.currentContainer).to.equal('parent_container');
      });
    });

    describe('when frontmatter has inventory with empty container string', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({ inventory: { container: '' } });
      });

      it('should not be an item', () => {
        expect(result.isItem).to.be.false;
      });

      it('should have empty currentContainer', () => {
        expect(result.currentContainer).to.equal('');
      });
    });

    describe('when frontmatter has both items and container', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({
          inventory: {
            items: ['child1'],
            container: 'parent'
          }
        });
      });

      it('should be a container', () => {
        expect(result.isContainer).to.be.true;
      });

      it('should be an item', () => {
        expect(result.isItem).to.be.true;
      });

      it('should have currentContainer set', () => {
        expect(result.currentContainer).to.equal('parent');
      });
    });

    describe('when frontmatter is a string', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData('not an object');
      });

      it('should return null inventory', () => {
        expect(result.inventory).to.be.null;
      });

      it('should not be a container', () => {
        expect(result.isContainer).to.be.false;
      });
    });

    describe('when frontmatter is a number', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData(123);
      });

      it('should return null inventory', () => {
        expect(result.inventory).to.be.null;
      });

      it('should not be a container', () => {
        expect(result.isContainer).to.be.false;
      });
    });

    describe('when inventory.container is not a string', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({ inventory: { container: 123 } });
      });

      it('should not be an item', () => {
        expect(result.isItem).to.be.false;
      });
    });

    describe('when inventory.items is not an array but defined', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({ inventory: { items: 'not an array' } });
      });

      it('should still be a container (items key exists)', () => {
        expect(result.isContainer).to.be.true;
      });
    });
  });

  describe('validateInventoryResponse', () => {    describe('when data is null', () => {
      it('should return false', () => {
        expect(validateInventoryResponse(null)).to.be.false;
      });
    });

    describe('when data is undefined', () => {
      it('should return false', () => {
        expect(validateInventoryResponse(undefined)).to.be.false;
      });
    });

    describe('when data has no ids property', () => {
      it('should return false', () => {
        expect(validateInventoryResponse({})).to.be.false;
      });
    });

    describe('when data.ids is not an array', () => {
      it('should return false for string', () => {
        expect(validateInventoryResponse({ ids: 'not array' })).to.be.false;
      });

      it('should return false for object', () => {
        expect(validateInventoryResponse({ ids: {} })).to.be.false;
      });

      it('should return false for number', () => {
        expect(validateInventoryResponse({ ids: 123 })).to.be.false;
      });
    });

    describe('when data.ids is an empty array', () => {
      it('should return true', () => {
        expect(validateInventoryResponse({ ids: [] })).to.be.true;
      });
    });

    describe('when data.ids is a valid array', () => {
      it('should return true', () => {
        expect(validateInventoryResponse({ ids: [{ identifier: 'test' }] })).to.be.true;
      });
    });
  });

  describe('findPageInInventoryList', () => {    describe('when ids is empty', () => {
      it('should return false', () => {
        expect(findPageInInventoryList([], 'test_page')).to.be.false;
      });
    });

    describe('when page is not in list', () => {
      it('should return false', () => {
        expect(findPageInInventoryList([{ identifier: 'other' }], 'test_page')).to.be.false;
      });
    });

    describe('when page is in list', () => {
      it('should return true', () => {
        expect(findPageInInventoryList([{ identifier: 'test_page' }], 'test_page')).to.be.true;
      });
    });

    describe('when list contains null items', () => {
      it('should handle null gracefully', () => {
        expect(findPageInInventoryList([null, { identifier: 'test_page' }], 'test_page')).to.be.true;
      });
    });

    describe('when list contains undefined items', () => {
      it('should handle undefined gracefully', () => {
        expect(findPageInInventoryList([undefined, { identifier: 'test_page' }], 'test_page')).to.be.true;
      });
    });

    describe('when list items have no identifier', () => {
      it('should handle missing identifier gracefully', () => {
        expect(findPageInInventoryList([{ other: 'prop' }], 'test_page')).to.be.false;
      });
    });
  });
});
