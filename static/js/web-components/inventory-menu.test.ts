import { expect } from '@esm-bundle/chai';
import sinon from 'sinon';
import {
  extractInventoryData,
  validateInventoryResponse,
  findPageInInventoryList,
  initInventoryMenu,
} from './inventory-menu.js';

function makeJsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('Inventory Menu Logic', () => {
  describe('extractInventoryData', () => {

    describe('when frontmatter is null', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData(null);
      });

      it('should return null inventory', () => {
        expect(result.inventory).to.equal(null);
      });

      it('should not be a container', () => {
        expect(result.isContainer).to.equal(false);
      });

      it('should not be an item', () => {
        expect(result.isItem).to.equal(false);
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
        expect(result.inventory).to.equal(null);
      });

      it('should not be a container', () => {
        expect(result.isContainer).to.equal(false);
      });

      it('should not be an item', () => {
        expect(result.isItem).to.equal(false);
      });
    });

    describe('when frontmatter is empty object', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({});
      });

      it('should return null/undefined inventory', () => {
        expect(result.inventory).to.equal(undefined);
      });

      it('should not be a container', () => {
        expect(result.isContainer).to.equal(false);
      });

      it('should not be an item', () => {
        expect(result.isItem).to.equal(false);
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
        expect(result.isContainer).to.equal(false);
      });

      it('should not be an item', () => {
        expect(result.isItem).to.equal(false);
      });
    });

    describe('when frontmatter has inventory with empty items array', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({ inventory: { items: [] } });
      });

      it('should be a container', () => {
        expect(result.isContainer).to.equal(true);
      });

      it('should not be an item', () => {
        expect(result.isItem).to.equal(false);
      });
    });

    describe('when frontmatter has inventory with items array', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({ inventory: { items: ['item1', 'item2'] } });
      });

      it('should be a container', () => {
        expect(result.isContainer).to.equal(true);
      });

      it('should not be an item', () => {
        expect(result.isItem).to.equal(false);
      });
    });

    describe('when frontmatter has inventory with container string', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({ inventory: { container: 'parent_container' } });
      });

      it('should not be a container', () => {
        expect(result.isContainer).to.equal(false);
      });

      it('should be an item', () => {
        expect(result.isItem).to.equal(true);
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
        expect(result.isItem).to.equal(false);
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
        expect(result.isContainer).to.equal(true);
      });

      it('should be an item', () => {
        expect(result.isItem).to.equal(true);
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
        expect(result.inventory).to.equal(null);
      });

      it('should not be a container', () => {
        expect(result.isContainer).to.equal(false);
      });
    });

    describe('when frontmatter is a number', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData(123);
      });

      it('should return null inventory', () => {
        expect(result.inventory).to.equal(null);
      });

      it('should not be a container', () => {
        expect(result.isContainer).to.equal(false);
      });
    });

    describe('when inventory.container is not a string', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({ inventory: { container: 123 } });
      });

      it('should not be an item', () => {
        expect(result.isItem).to.equal(false);
      });
    });

    describe('when inventory.items is not an array but defined', () => {
      let result: ReturnType<typeof extractInventoryData>;

      beforeEach(() => {
        result = extractInventoryData({ inventory: { items: 'not an array' } });
      });

      it('should still be a container (items key exists)', () => {
        expect(result.isContainer).to.equal(true);
      });
    });
  });

  describe('validateInventoryResponse', () => {
    describe('when data is null', () => {
      it('should return false', () => {
        expect(validateInventoryResponse(null)).to.equal(false);
      });
    });

    describe('when data is undefined', () => {
      it('should return false', () => {
        expect(validateInventoryResponse(undefined)).to.equal(false);
      });
    });

    describe('when data has no ids property', () => {
      it('should return false', () => {
        expect(validateInventoryResponse({})).to.equal(false);
      });
    });

    describe('when data.ids is not an array', () => {
      it('should return false for string', () => {
        expect(validateInventoryResponse({ ids: 'not array' })).to.equal(false);
      });

      it('should return false for object', () => {
        expect(validateInventoryResponse({ ids: {} })).to.equal(false);
      });

      it('should return false for number', () => {
        expect(validateInventoryResponse({ ids: 123 })).to.equal(false);
      });
    });

    describe('when data.ids is an empty array', () => {
      it('should return true', () => {
        expect(validateInventoryResponse({ ids: [] })).to.equal(true);
      });
    });

    describe('when data.ids is a valid array', () => {
      it('should return true', () => {
        expect(validateInventoryResponse({ ids: [{ identifier: 'test' }] })).to.equal(true);
      });
    });
  });

  describe('findPageInInventoryList', () => {
    describe('when ids is empty', () => {
      it('should return false', () => {
        expect(findPageInInventoryList([], 'test_page')).to.equal(false);
      });
    });

    describe('when page is not in list', () => {
      it('should return false', () => {
        expect(findPageInInventoryList([{ identifier: 'other' }], 'test_page')).to.equal(false);
      });
    });

    describe('when page is in list', () => {
      it('should return true', () => {
        expect(findPageInInventoryList([{ identifier: 'test_page' }], 'test_page')).to.equal(true);
      });
    });

    describe('when list contains null items', () => {
      it('should handle null gracefully', () => {
        expect(findPageInInventoryList([null, { identifier: 'test_page' }], 'test_page')).to.equal(true);
      });
    });

    describe('when list contains undefined items', () => {
      it('should handle undefined gracefully', () => {
        expect(findPageInInventoryList([undefined, { identifier: 'test_page' }], 'test_page')).to.equal(true);
      });
    });

    describe('when list items have no identifier', () => {
      it('should handle missing identifier gracefully', () => {
        expect(findPageInInventoryList([{ other: 'prop' }], 'test_page')).to.equal(false);
      });
    });
  });
});

describe('initInventoryMenu', () => {
  let fetchStub: sinon.SinonStub;
  let utilitySection: HTMLElement;

  beforeEach(() => {
    fetchStub = sinon.stub(window, 'fetch');
    window.simple_wiki = { pageName: 'test_page' };

    utilitySection = document.createElement('hr');
    utilitySection.id = 'utilityMenuSection';
    document.body.appendChild(utilitySection);
  });

  afterEach(() => {
    fetchStub.restore();
    delete window.simple_wiki;
    utilitySection.remove();
    document.getElementById('inventory-submenu')?.remove();
  });

  describe('when utilityMenuSection is absent', () => {
    beforeEach(() => {
      utilitySection.remove();
      initInventoryMenu();
    });

    it('should not make any fetch calls', () => {
      expect(fetchStub.called).to.equal(false);
    });
  });

  describe('when pageName is empty', () => {
    beforeEach(() => {
      window.simple_wiki = { pageName: '' };
      initInventoryMenu();
    });

    it('should not make any fetch calls', () => {
      expect(fetchStub.called).to.equal(false);
    });
  });

  describe('when pageName is absent', () => {
    beforeEach(() => {
      window.simple_wiki = {};
      initInventoryMenu();
    });

    it('should not make any fetch calls', () => {
      expect(fetchStub.called).to.equal(false);
    });
  });

  describe('when current page is not in inventory list', () => {
    beforeEach(async () => {
      fetchStub.onFirstCall().resolves(makeJsonResponse({
        ids: [{ identifier: 'other_page' }],
      }));
      initInventoryMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should not inject a submenu', () => {
      expect(document.getElementById('inventory-submenu')).to.equal(null);
    });
  });

  describe('when API returns empty ids', () => {
    beforeEach(async () => {
      fetchStub.onFirstCall().resolves(makeJsonResponse({ ids: [] }));
      initInventoryMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should not inject a submenu', () => {
      expect(document.getElementById('inventory-submenu')).to.equal(null);
    });
  });

  describe('when API returns invalid response structure', () => {
    beforeEach(async () => {
      fetchStub.onFirstCall().resolves(makeJsonResponse({ notIds: [] }));
      initInventoryMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should not inject a submenu', () => {
      expect(document.getElementById('inventory-submenu')).to.equal(null);
    });
  });

  describe('when API returns non-OK status', () => {
    beforeEach(async () => {
      fetchStub.onFirstCall().resolves(new Response('error', { status: 500 }));
      initInventoryMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should not inject a submenu (silently ignores)', () => {
      expect(document.getElementById('inventory-submenu')).to.equal(null);
    });
  });

  describe('when fetch throws', () => {
    beforeEach(async () => {
      fetchStub.onFirstCall().rejects(new Error('Network error'));
      initInventoryMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should not inject a submenu (silently ignores)', () => {
      expect(document.getElementById('inventory-submenu')).to.equal(null);
    });
  });

  describe('when current page is in inventory list (container page)', () => {
    beforeEach(async () => {
      fetchStub.onFirstCall().resolves(makeJsonResponse({
        ids: [{ identifier: 'test_page' }],
      }));
      fetchStub.onSecondCall().resolves(makeJsonResponse({
        inventory: { items: ['child1', 'child2'] },
      }));
      initInventoryMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should inject an inventory submenu', () => {
      expect(document.getElementById('inventory-submenu')).to.not.equal(null);
    });

    it('should include the submenu trigger link', () => {
      expect(document.getElementById('inventory-submenu-trigger')).to.not.equal(null);
    });

    it('should initialise aria-expanded to false', () => {
      const trigger = document.getElementById('inventory-submenu-trigger');
      expect(trigger?.getAttribute('aria-expanded')).to.equal('false');
    });

    it('should include aria-controls pointing at children ul', () => {
      const trigger = document.getElementById('inventory-submenu-trigger');
      expect(trigger?.getAttribute('aria-controls')).to.equal('inventory-submenu-children');
    });

    it('should include the children ul with correct id', () => {
      expect(document.getElementById('inventory-submenu-children')).to.not.equal(null);
    });

    it('should set role="menu" on the children ul', () => {
      const children = document.getElementById('inventory-submenu-children');
      expect(children?.getAttribute('role')).to.equal('menu');
    });

    it('should include the Add Item Here link', () => {
      expect(document.getElementById('inventory-add-item')).to.not.equal(null);
    });

    it('should set role="menuitem" on the Add Item Here link', () => {
      const link = document.getElementById('inventory-add-item');
      expect(link?.getAttribute('role')).to.equal('menuitem');
    });

    it('should not include the Move This Item link for a container page', () => {
      expect(document.getElementById('inventory-move-item')).to.equal(null);
    });
  });

  describe('when current page is an inventory item (not a container)', () => {
    beforeEach(async () => {
      fetchStub.onFirstCall().resolves(makeJsonResponse({
        ids: [{ identifier: 'test_page' }],
      }));
      fetchStub.onSecondCall().resolves(makeJsonResponse({
        inventory: { container: 'parent_container' },
      }));
      initInventoryMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should inject an inventory submenu', () => {
      expect(document.getElementById('inventory-submenu')).to.not.equal(null);
    });

    it('should not include the Add Item Here link', () => {
      expect(document.getElementById('inventory-add-item')).to.equal(null);
    });

    it('should include the Move This Item link', () => {
      expect(document.getElementById('inventory-move-item')).to.not.equal(null);
    });

    it('should set role="menuitem" on the Move This Item link', () => {
      const link = document.getElementById('inventory-move-item');
      expect(link?.getAttribute('role')).to.equal('menuitem');
    });
  });

  describe('when current page is both container and item', () => {
    beforeEach(async () => {
      fetchStub.onFirstCall().resolves(makeJsonResponse({
        ids: [{ identifier: 'test_page' }],
      }));
      fetchStub.onSecondCall().resolves(makeJsonResponse({
        inventory: { items: ['child1'], container: 'parent_container' },
      }));
      initInventoryMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should include both Add Item Here and Move This Item links', () => {
      expect(document.getElementById('inventory-add-item')).to.not.equal(null);
      expect(document.getElementById('inventory-move-item')).to.not.equal(null);
    });
  });

  describe('when frontmatter fetch fails', () => {
    beforeEach(async () => {
      fetchStub.onFirstCall().resolves(makeJsonResponse({
        ids: [{ identifier: 'test_page' }],
      }));
      fetchStub.onSecondCall().rejects(new Error('Frontmatter fetch failed'));
      initInventoryMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should still inject the submenu with empty inventory data', () => {
      // buildInventoryMenu is called with {} as fallback
      expect(document.getElementById('inventory-submenu')).to.not.equal(null);
    });
  });

  describe('submenu trigger toggle', () => {
    beforeEach(async () => {
      fetchStub.onFirstCall().resolves(makeJsonResponse({
        ids: [{ identifier: 'test_page' }],
      }));
      fetchStub.onSecondCall().resolves(makeJsonResponse({
        inventory: { items: [] },
      }));
      initInventoryMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    describe('when the trigger is clicked once', () => {
      beforeEach(() => {
        const trigger = document.getElementById('inventory-submenu-trigger') as HTMLAnchorElement;
        trigger.click();
      });

      it('should set aria-expanded to true', () => {
        const trigger = document.getElementById('inventory-submenu-trigger');
        expect(trigger?.getAttribute('aria-expanded')).to.equal('true');
      });

      describe('when the trigger is clicked a second time', () => {
        beforeEach(() => {
          const trigger = document.getElementById('inventory-submenu-trigger') as HTMLAnchorElement;
          trigger.click();
        });

        it('should set aria-expanded back to false', () => {
          const trigger = document.getElementById('inventory-submenu-trigger');
          expect(trigger?.getAttribute('aria-expanded')).to.equal('false');
        });
      });

      describe('when an outside click occurs', () => {
        beforeEach(() => {
          document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        });

        it('should set aria-expanded to false', () => {
          const trigger = document.getElementById('inventory-submenu-trigger');
          expect(trigger?.getAttribute('aria-expanded')).to.equal('false');
        });
      });
    });
  });
});
