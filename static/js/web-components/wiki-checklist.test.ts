import { expect } from '@open-wc/testing';
import sinon, { type SinonStub } from 'sinon';
import './wiki-checklist.js';
import type { WikiChecklist } from './wiki-checklist.js';
import { create } from '@bufbuild/protobuf';
import {
  GetFrontmatterResponseSchema,
  MergeFrontmatterResponseSchema,
} from '../gen/api/v1/frontmatter_pb.js';
import type { JsonObject } from '@bufbuild/protobuf';

describe('WikiChecklist', () => {
  let el: WikiChecklist;

  function timeout(ms: number, message: string): Promise<never> {
    return new Promise((_, reject) =>
      setTimeout(() => reject(new Error(message)), ms)
    );
  }

  beforeEach(async () => {
    // Create element without connecting so we can stub the client first
    el = document.createElement('wiki-checklist') as WikiChecklist;
    el.setAttribute('list-name', 'grocery_list');
    el.setAttribute('page', 'test-page');

    // Stub client.getFrontmatter before connectedCallback runs
    sinon.stub(el.client, 'getFrontmatter').resolves(
      create(GetFrontmatterResponseSchema, { frontmatter: {} })
    );

    // Now connect (triggers connectedCallback)
    document.body.appendChild(el);

    await Promise.race([
      el.updateComplete,
      timeout(5000, 'Component fixture timed out'),
    ]);
  });

  afterEach(() => {
    sinon.restore();
    if (el) {
      el.remove();
    }
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  it('should be an instance of WikiChecklist', async () => {
    const { WikiChecklist: WC } = await import('./wiki-checklist.js');
    expect(el).to.be.instanceOf(WC);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('wiki-checklist');
  });

  describe('when component is initialized', () => {
    it('should have the listName property set', () => {
      expect(el.listName).to.equal('grocery_list');
    });

    it('should have the page property set', () => {
      expect(el.page).to.equal('test-page');
    });

    it('should default to empty items array', () => {
      expect(el.items).to.deep.equal([]);
    });

    it('should default groupOrder to null', () => {
      expect(el.groupOrder).to.be.null;
    });

    it('should default to flat view', () => {
      expect(el.groupedView).to.be.false;
    });

    it('should not be loading by default (after initial fetch stub)', () => {
      // loading starts true, then completes - stub prevents fetch
      expect(el.loading).to.be.false;
    });

    it('should not be saving by default', () => {
      expect(el.saving).to.be.false;
    });

    it('should have no error by default', () => {
      expect(el.error).to.be.null;
    });
  });

  describe('formatTitle', () => {
    it('should convert snake_case to Title Case', () => {
      expect(el.formatTitle('grocery_list')).to.equal('Grocery List');
    });

    it('should convert kebab-case to Title Case', () => {
      expect(el.formatTitle('my-checklist')).to.equal('My Checklist');
    });

    it('should handle single word', () => {
      expect(el.formatTitle('tasks')).to.equal('Tasks');
    });

    it('should handle mixed snake and kebab case', () => {
      expect(el.formatTitle('my_todo-list')).to.equal('My Todo List');
    });

    it('should handle empty string', () => {
      expect(el.formatTitle('')).to.equal('');
    });
  });

  describe('extractChecklistData', () => {
    it('should return empty items when frontmatter has no checklists', () => {
      const frontmatter: JsonObject = { title: 'Test' };
      const result = el.extractChecklistData(frontmatter, 'grocery_list');
      expect(result.items).to.deep.equal([]);
      expect(result.groupOrder).to.be.null;
    });

    it('should return empty items when listName not in checklists', () => {
      const frontmatter: JsonObject = {
        checklists: { other_list: { items: [] } },
      };
      const result = el.extractChecklistData(frontmatter, 'grocery_list');
      expect(result.items).to.deep.equal([]);
    });

    it('should extract items from checklists', () => {
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
      const result = el.extractChecklistData(frontmatter, 'grocery_list');
      expect(result.items).to.have.length(2);
      expect(result.items[0]).to.deep.equal({ text: 'Milk', checked: false });
      expect(result.items[1]).to.deep.equal({
        text: 'Eggs',
        checked: true,
        tag: 'Dairy',
      });
    });

    it('should extract group_order from checklists', () => {
      const frontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [],
            group_order: ['Dairy', 'Produce'],
          },
        },
      };
      const result = el.extractChecklistData(frontmatter, 'grocery_list');
      expect(result.groupOrder).to.deep.equal(['Dairy', 'Produce']);
    });
  });

  describe('getExistingTags', () => {
    beforeEach(() => {
      el.items = [
        { text: 'Milk', checked: false, tag: 'Dairy' },
        { text: 'Apples', checked: false, tag: 'Produce' },
        { text: 'Eggs', checked: true, tag: 'Dairy' },
        { text: 'Bread', checked: false },
      ];
    });

    it('should return unique tags sorted alphabetically', () => {
      expect(el.getExistingTags()).to.deep.equal(['Dairy', 'Produce']);
    });

    it('should exclude undefined/empty tags', () => {
      el.items = [
        { text: 'Item 1', checked: false },
        { text: 'Item 2', checked: false, tag: '' },
        { text: 'Item 3', checked: false, tag: 'TagA' },
      ];
      expect(el.getExistingTags()).to.deep.equal(['TagA']);
    });
  });

  describe('getGroupedItems', () => {
    beforeEach(() => {
      el.items = [
        { text: 'Milk', checked: false, tag: 'Dairy' },
        { text: 'Bread', checked: false, tag: 'Bakery' },
        { text: 'Apples', checked: false, tag: 'Produce' },
        { text: 'Eggs', checked: true, tag: 'Dairy' },
        { text: 'Towels', checked: false },
      ];
    });

    it('should group items by tag', () => {
      const groups = el.getGroupedItems();
      const dairyGroup = groups.find(g => g.tag === 'Dairy');
      expect(dairyGroup).to.exist;
      expect(dairyGroup!.items).to.have.length(2);
    });

    it('should put untagged items in "Other" group', () => {
      const groups = el.getGroupedItems();
      const otherGroup = groups.find(g => g.tag === 'Other');
      expect(otherGroup).to.exist;
      expect(otherGroup!.items).to.have.length(1);
      const firstItem = otherGroup!.items[0];
      expect(firstItem).to.exist;
      expect(firstItem!.item.text).to.equal('Towels');
    });

    it('should respect groupOrder when provided', () => {
      el.groupOrder = ['Produce', 'Dairy', 'Bakery'];
      const groups = el.getGroupedItems();
      expect(groups[0]?.tag).to.equal('Produce');
      expect(groups[1]?.tag).to.equal('Dairy');
      expect(groups[2]?.tag).to.equal('Bakery');
    });

    it('should sort groups alphabetically when no groupOrder', () => {
      el.groupOrder = null;
      const groups = el.getGroupedItems();
      const taggedGroups = groups.filter(g => g.tag !== 'Other');
      const tags = taggedGroups.map(g => g.tag);
      expect(tags).to.deep.equal(['Bakery', 'Dairy', 'Produce']);
    });

    it('should preserve absolute indices for items', () => {
      const groups = el.getGroupedItems();
      const dairyGroup = groups.find(g => g.tag === 'Dairy');
      const indices = dairyGroup!.items.map(i => i.index);
      expect(indices).to.include(0); // Milk is index 0
      expect(indices).to.include(3); // Eggs is index 3
    });
  });

  describe('rendering', () => {
    it('should render the formatted title', async () => {
      await el.updateComplete;
      const title = el.shadowRoot?.querySelector('h2, h3, .checklist-title');
      expect(title).to.exist;
      expect(title!.textContent?.trim()).to.contain('Grocery List');
    });

    it('should render loading state when loading is true', async () => {
      el.loading = true;
      await el.updateComplete;
      const loadingEl = el.shadowRoot?.querySelector('.loading');
      expect(loadingEl).to.exist;
    });

    it('should render error state when error is set', async () => {
      el.error = new Error('Test error');
      await el.updateComplete;
      const errorEl = el.shadowRoot?.querySelector('error-display, .error');
      expect(errorEl).to.exist;
    });

    it('should render items when items are present', async () => {
      el.error = null;
      el.loading = false;
      el.items = [
        { text: 'Milk', checked: false },
        { text: 'Eggs', checked: true },
      ];
      await el.updateComplete;
      const checkboxes = el.shadowRoot?.querySelectorAll('input[type="checkbox"]');
      expect(checkboxes).to.have.length(2);
    });

    it('should render checked items with strikethrough/fade class', async () => {
      el.error = null;
      el.loading = false;
      el.items = [{ text: 'Done', checked: true }];
      await el.updateComplete;
      const checkedItem = el.shadowRoot?.querySelector('.item-checked');
      expect(checkedItem).to.exist;
    });

    it('should render empty state when items array is empty and not loading', async () => {
      el.loading = false;
      el.error = null;
      el.items = [];
      await el.updateComplete;
      const emptyState = el.shadowRoot?.querySelector('.empty-state');
      expect(emptyState).to.exist;
    });

    it('should render a view toggle button when items are present', async () => {
      el.error = null;
      el.loading = false;
      el.items = [
        { text: 'Milk', checked: false, tag: 'Dairy' },
      ];
      await el.updateComplete;
      const toggleButton = el.shadowRoot?.querySelector('.view-toggle');
      expect(toggleButton).to.exist;
    });

    it('should render group headings in grouped view', async () => {
      el.error = null;
      el.loading = false;
      el.items = [
        { text: 'Milk', checked: false, tag: 'Dairy' },
        { text: 'Apples', checked: false, tag: 'Produce' },
      ];
      el.groupedView = true;
      await el.updateComplete;
      const groupHeadings = el.shadowRoot?.querySelectorAll('.group-header');
      expect(groupHeadings!.length).to.be.greaterThan(0);
    });

    it('should render add-item input at bottom', async () => {
      el.error = null;
      el.loading = false;
      await el.updateComplete;
      const addInput = el.shadowRoot?.querySelector('.add-text-input');
      expect(addInput).to.exist;
    });
  });

  describe('fetchData', () => {
    let getFrontmatterStub: SinonStub;

    beforeEach(() => {
      sinon.restore(); // remove stub from outer beforeEach
      // Create a fresh element for this suite
      el.remove();
      el = document.createElement('wiki-checklist') as WikiChecklist;
      el.setAttribute('list-name', 'grocery_list');
      el.setAttribute('page', 'test-page');
      // Stub before connecting
      getFrontmatterStub = sinon.stub(el.client, 'getFrontmatter').resolves(
        create(GetFrontmatterResponseSchema, { frontmatter: {} })
      );
      document.body.appendChild(el);
    });

    it('should call getFrontmatter with the page', async () => {
      const mockFrontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [{ text: 'Milk', checked: false }],
          },
        },
      };
      getFrontmatterStub.resolves(
        create(GetFrontmatterResponseSchema, { frontmatter: mockFrontmatter })
      );
      await el.fetchData();
      expect(getFrontmatterStub.callCount).to.be.greaterThan(0);
    });

    it('should update items from response', async () => {
      const mockFrontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [
              { text: 'Milk', checked: false },
              { text: 'Eggs', checked: true, tag: 'Dairy' },
            ],
          },
        },
      };
      getFrontmatterStub.resolves(
        create(GetFrontmatterResponseSchema, { frontmatter: mockFrontmatter })
      );
      await el.fetchData();
      expect(el.items).to.have.length(2);
      expect(el.items[0]?.text).to.equal('Milk');
      expect(el.items[1]?.tag).to.equal('Dairy');
    });

    it('should set error when fetch fails', async () => {
      getFrontmatterStub.rejects(new Error('Network error'));
      await el.fetchData();
      expect(el.error).to.be.instanceOf(Error);
    });

    it('should clear loading after fetch', async () => {
      getFrontmatterStub.resolves(
        create(GetFrontmatterResponseSchema, { frontmatter: {} })
      );
      await el.fetchData();
      expect(el.loading).to.be.false;
    });
  });

  describe('persistData', () => {
    let getFrontmatterStub: SinonStub;
    let mergeFrontmatterStub: SinonStub;

    beforeEach(() => {
      sinon.restore();
      getFrontmatterStub = sinon.stub(el.client, 'getFrontmatter').resolves(
        create(GetFrontmatterResponseSchema, { frontmatter: {} })
      );
      mergeFrontmatterStub = sinon.stub(el.client, 'mergeFrontmatter').resolves(
        create(MergeFrontmatterResponseSchema, { frontmatter: {} })
      );
    });

    it('should call mergeFrontmatter with updated checklists', async () => {
      const newItems = [{ text: 'Milk', checked: true }];
      await el.persistData(newItems, null);
      expect(mergeFrontmatterStub).to.have.been.calledOnce;
    });

    it('should read-modify-write: get then merge', async () => {
      await el.persistData([{ text: 'Item', checked: false }], null);
      expect(getFrontmatterStub).to.have.been.calledBefore(mergeFrontmatterStub);
    });

    it('should set saving state during persist', async () => {
      let savingDuringCall = false;
      mergeFrontmatterStub.callsFake(async () => {
        savingDuringCall = el.saving;
        return create(MergeFrontmatterResponseSchema, { frontmatter: {} });
      });
      await el.persistData([], null);
      expect(savingDuringCall).to.be.true;
    });

    it('should clear saving state after persist', async () => {
      await el.persistData([], null);
      expect(el.saving).to.be.false;
    });

    it('should set error when persist fails', async () => {
      mergeFrontmatterStub.rejects(new Error('Save failed'));
      await el.persistData([], null);
      expect(el.error).to.be.instanceOf(Error);
    });
  });

  describe('polling', () => {
    let clock: sinon.SinonFakeTimers;

    beforeEach(() => {
      sinon.restore();
      clock = sinon.useFakeTimers({ shouldAdvanceTime: false });
    });

    afterEach(() => {
      clock.restore();
    });

    it('should poll fetchData at regular intervals', async () => {
      // Create fresh element with fake timers active
      const freshEl = document.createElement('wiki-checklist') as WikiChecklist;
      freshEl.setAttribute('list-name', 'test_list');
      freshEl.setAttribute('page', 'test-page');
      const fetchStub = sinon.stub(freshEl, 'fetchData').resolves();
      document.body.appendChild(freshEl);

      // Advance past one poll interval
      clock.tick(3001);
      expect(fetchStub.callCount).to.be.greaterThan(0);
      freshEl.remove();
    });

    it('should stop polling on disconnect', async () => {
      const freshEl = document.createElement('wiki-checklist') as WikiChecklist;
      freshEl.setAttribute('list-name', 'test_list');
      freshEl.setAttribute('page', 'test-page');
      const fetchStub = sinon.stub(freshEl, 'fetchData').resolves();
      document.body.appendChild(freshEl);
      freshEl.remove();

      const countAfterDisconnect = fetchStub.callCount;
      clock.tick(10000);
      expect(fetchStub.callCount).to.equal(countAfterDisconnect);
    });
  });
});

