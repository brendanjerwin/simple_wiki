import { expect } from '@open-wc/testing';
import sinon, { type SinonStub } from 'sinon';
import './wiki-checklist.js';
import type { WikiChecklist, ChecklistItem } from './wiki-checklist.js';
import { AugmentedError, AugmentErrorService } from './augment-error-service.js';
import { create } from '@bufbuild/protobuf';
import {
  GetFrontmatterResponseSchema,
  MergeFrontmatterResponseSchema,
} from '../gen/api/v1/frontmatter_pb.js';
import type { JsonObject } from '@bufbuild/protobuf';

// Helper type to access private methods for testing
interface WikiChecklistInternal {
  persistData(
    items: ChecklistItem[],
    groupOrder: string[] | null
  ): Promise<void>;
}

describe('WikiChecklist', () => {
  let el: WikiChecklist;

  function buildElement(
    page = 'test-page',
    listName = 'grocery_list'
  ): WikiChecklist {
    const freshEl = document.createElement(
      'wiki-checklist'
    ) as WikiChecklist;
    freshEl.setAttribute('list-name', listName);
    freshEl.setAttribute('page', page);
    return freshEl;
  }

  function stubGetFrontmatter(
    target: WikiChecklist,
    frontmatter: JsonObject = {}
  ): SinonStub {
    return sinon
      .stub(target.client, 'getFrontmatter')
      .resolves(create(GetFrontmatterResponseSchema, { frontmatter }));
  }

  beforeEach(async () => {
    el = buildElement();
    stubGetFrontmatter(el);
    document.body.appendChild(el);
    await el.updateComplete;
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

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('wiki-checklist');
  });

  describe('after initial successful fetch', () => {
    it('should not be in loading state', () => {
      expect(el.loading).to.be.false;
    });

    it('should have no error', () => {
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
    describe('when items have multiple tags', () => {
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
    });

    describe('when items have empty or missing tags', () => {
      beforeEach(() => {
        el.items = [
          { text: 'Item 1', checked: false },
          { text: 'Item 2', checked: false, tag: '' },
          { text: 'Item 3', checked: false, tag: 'TagA' },
        ];
      });

      it('should exclude empty tags', () => {
        expect(el.getExistingTags()).to.deep.equal(['TagA']);
      });
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

    describe('when groupOrder is provided', () => {
      beforeEach(() => {
        el.groupOrder = ['Produce', 'Dairy', 'Bakery'];
      });

      it('should respect the custom group order', () => {
        const groups = el.getGroupedItems();
        expect(groups[0]?.tag).to.equal('Produce');
        expect(groups[1]?.tag).to.equal('Dairy');
        expect(groups[2]?.tag).to.equal('Bakery');
      });
    });

    describe('when groupOrder is null', () => {
      beforeEach(() => {
        el.groupOrder = null;
      });

      it('should sort groups alphabetically', () => {
        const groups = el.getGroupedItems();
        const taggedGroups = groups.filter(g => g.tag !== 'Other');
        const tags = taggedGroups.map(g => g.tag);
        expect(tags).to.deep.equal(['Bakery', 'Dairy', 'Produce']);
      });
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
    describe('title', () => {
      let title: Element | null | undefined;

      beforeEach(async () => {
        await el.updateComplete;
        title = el.shadowRoot?.querySelector('.checklist-title');
      });

      it('should render the formatted list name as a heading', () => {
        expect(title?.textContent?.trim()).to.contain('Grocery List');
      });
    });

    describe('when loading is true', () => {
      let loadingEl: Element | null | undefined;

      beforeEach(async () => {
        el.loading = true;
        await el.updateComplete;
        loadingEl = el.shadowRoot?.querySelector('.loading');
      });

      it('should render loading indicator', () => {
        expect(loadingEl).to.exist;
      });
    });

    describe('when error is set', () => {
      let errorEl: Element | null | undefined;

      beforeEach(async () => {
        el.error = AugmentErrorService.augmentError(new Error('Test error'));
        await el.updateComplete;
        errorEl = el.shadowRoot?.querySelector('error-display');
      });

      it('should render error-display component', () => {
        expect(errorEl).to.exist;
      });
    });

    describe('when items are present', () => {
      let checkboxes: NodeListOf<Element> | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          { text: 'Milk', checked: false },
          { text: 'Eggs', checked: true },
        ];
        await el.updateComplete;
        checkboxes = el.shadowRoot?.querySelectorAll('input[type="checkbox"]');
      });

      it('should render a checkbox for each item', () => {
        expect(checkboxes).to.have.length(2);
      });
    });

    describe('when an item is checked', () => {
      let checkedItem: Element | null | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [{ text: 'Done', checked: true }];
        await el.updateComplete;
        checkedItem = el.shadowRoot?.querySelector('.item-checked');
      });

      it('should apply checked styling to the item', () => {
        expect(checkedItem).to.exist;
      });
    });

    describe('when items array is empty and not loading', () => {
      let emptyState: Element | null | undefined;

      beforeEach(async () => {
        el.loading = false;
        el.error = null;
        el.items = [];
        await el.updateComplete;
        emptyState = el.shadowRoot?.querySelector('.empty-state');
      });

      it('should render empty state message', () => {
        expect(emptyState).to.exist;
      });
    });

    describe('when items are present (view toggle)', () => {
      let toggleButton: Element | null | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [{ text: 'Milk', checked: false, tag: 'Dairy' }];
        await el.updateComplete;
        toggleButton = el.shadowRoot?.querySelector('.view-toggle');
      });

      it('should render view toggle button', () => {
        expect(toggleButton).to.exist;
      });
    });

    describe('when groupedView is true', () => {
      let groupHeadings: NodeListOf<Element> | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          { text: 'Milk', checked: false, tag: 'Dairy' },
          { text: 'Apples', checked: false, tag: 'Produce' },
        ];
        el.groupedView = true;
        await el.updateComplete;
        groupHeadings = el.shadowRoot?.querySelectorAll('.group-header');
      });

      it('should render group headings', () => {
        expect(groupHeadings!.length).to.be.greaterThan(0);
      });
    });

    describe('add item form', () => {
      let addInput: Element | null | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        await el.updateComplete;
        addInput = el.shadowRoot?.querySelector('.add-text-input');
      });

      it('should always render the add-item input', () => {
        expect(addInput).to.exist;
      });
    });

    describe('datalist', () => {
      let datalists: NodeListOf<Element> | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          { text: 'Milk', checked: false, tag: 'Dairy' },
          { text: 'Eggs', checked: false, tag: 'Dairy' },
        ];
        await el.updateComplete;
        datalists = el.shadowRoot?.querySelectorAll(
          'datalist#tag-suggestions-grocery_list'
        );
      });

      it('should render exactly one datalist for tag suggestions', () => {
        expect(datalists!.length).to.equal(1);
      });
    });
  });

  describe('when GetFrontmatter returns checklist items', () => {
    let items: ChecklistItem[];
    let getFrontmatterStub: SinonStub;

    beforeEach(async () => {
      sinon.restore();
      el.remove();

      el = buildElement();
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
      getFrontmatterStub = sinon
        .stub(el.client, 'getFrontmatter')
        .resolves(
          create(GetFrontmatterResponseSchema, { frontmatter: mockFrontmatter })
        );
      document.body.appendChild(el);
      await el.updateComplete;
      items = el.items;
    });

    it('should call getFrontmatter with the configured page', () => {
      expect(getFrontmatterStub.callCount).to.be.greaterThan(0);
      expect(getFrontmatterStub.getCall(0).args[0].page).to.equal('test-page');
    });

    it('should populate items from response', () => {
      expect(items).to.have.length(2);
    });

    it('should map item text correctly', () => {
      expect(items[0]?.text).to.equal('Milk');
    });

    it('should map item tags correctly', () => {
      expect(items[1]?.tag).to.equal('Dairy');
    });

    it('should clear loading state', () => {
      expect(el.loading).to.be.false;
    });
  });

  describe('when GetFrontmatter fails', () => {
    beforeEach(async () => {
      sinon.restore();
      el.remove();

      el = buildElement();
      sinon
        .stub(el.client, 'getFrontmatter')
        .rejects(new Error('Network error'));
      document.body.appendChild(el);
      await el.updateComplete;
    });

    it('should set error to an AugmentedError', () => {
      expect(el.error).to.be.instanceOf(AugmentedError);
    });

    it('should describe the failed goal as loading checklist', () => {
      expect(el.error?.failedGoalDescription).to.equal('loading checklist');
    });

    it('should clear loading state', () => {
      expect(el.loading).to.be.false;
    });
  });

  describe('when persisting data', () => {
    let getFrontmatterStub: SinonStub;
    let mergeFrontmatterStub: SinonStub;

    beforeEach(() => {
      sinon.restore();
      getFrontmatterStub = sinon
        .stub(el.client, 'getFrontmatter')
        .resolves(create(GetFrontmatterResponseSchema, { frontmatter: {} }));
      mergeFrontmatterStub = sinon
        .stub(el.client, 'mergeFrontmatter')
        .resolves(create(MergeFrontmatterResponseSchema, { frontmatter: {} }));
    });

    describe('when saving items succeeds', () => {
      const newItems = [{ text: 'Milk', checked: true }];

      beforeEach(async () => {
        await (el as unknown as WikiChecklistInternal).persistData(
          newItems,
          null
        );
      });

      it('should call mergeFrontmatter', () => {
        expect(mergeFrontmatterStub).to.have.been.calledOnce;
      });

      it('should call getFrontmatter before mergeFrontmatter (read-modify-write)', () => {
        expect(getFrontmatterStub).to.have.been.calledBefore(
          mergeFrontmatterStub
        );
      });

      it('should clear saving state after completion', () => {
        expect(el.saving).to.be.false;
      });
    });

    describe('when save is in progress', () => {
      let savingDuringMerge: boolean;

      beforeEach(async () => {
        savingDuringMerge = false;
        mergeFrontmatterStub.callsFake(async () => {
          savingDuringMerge = el.saving;
          return create(MergeFrontmatterResponseSchema, { frontmatter: {} });
        });
        await (el as unknown as WikiChecklistInternal).persistData([], null);
      });

      it('should be in saving state during the merge call', () => {
        expect(savingDuringMerge).to.be.true;
      });
    });

    describe('when persist fails', () => {
      beforeEach(async () => {
        mergeFrontmatterStub.rejects(new Error('Save failed'));
        await (el as unknown as WikiChecklistInternal).persistData([], null);
      });

      it('should set error to an AugmentedError', () => {
        expect(el.error).to.be.instanceOf(AugmentedError);
      });

      it('should describe the failed goal as saving checklist', () => {
        expect(el.error?.failedGoalDescription).to.equal('saving checklist');
      });

      it('should clear saving state', () => {
        expect(el.saving).to.be.false;
      });
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

    describe('when element is connected', () => {
      let getFrontmatterStub: SinonStub;
      let freshEl: WikiChecklist;

      beforeEach(() => {
        freshEl = buildElement('test-page', 'test_list');
        getFrontmatterStub = stubGetFrontmatter(freshEl);
        document.body.appendChild(freshEl);
        clock.tick(3001);
      });

      afterEach(() => {
        freshEl.remove();
      });

      it('should call getFrontmatter at regular intervals', () => {
        expect(getFrontmatterStub.callCount).to.be.greaterThan(0);
      });
    });

    describe('when element is disconnected', () => {
      let getFrontmatterStub: SinonStub;
      let countAfterDisconnect: number;

      beforeEach(() => {
        const freshEl = buildElement('test-page', 'test_list');
        getFrontmatterStub = stubGetFrontmatter(freshEl);
        document.body.appendChild(freshEl);
        freshEl.remove();
        countAfterDisconnect = getFrontmatterStub.callCount;
        clock.tick(10000);
      });

      it('should stop polling after disconnect', () => {
        expect(getFrontmatterStub.callCount).to.equal(countAfterDisconnect);
      });
    });
  });
});

