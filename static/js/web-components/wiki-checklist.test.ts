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
    describe('when given snake_case input', () => {
      let result: string;

      beforeEach(() => {
        result = el.formatTitle('grocery_list');
      });

      it('should convert to Title Case', () => {
        expect(result).to.equal('Grocery List');
      });
    });

    describe('when given kebab-case input', () => {
      let result: string;

      beforeEach(() => {
        result = el.formatTitle('my-checklist');
      });

      it('should convert to Title Case', () => {
        expect(result).to.equal('My Checklist');
      });
    });

    describe('when given a single word', () => {
      let result: string;

      beforeEach(() => {
        result = el.formatTitle('tasks');
      });

      it('should capitalize the word', () => {
        expect(result).to.equal('Tasks');
      });
    });

    describe('when given mixed snake and kebab case', () => {
      let result: string;

      beforeEach(() => {
        result = el.formatTitle('my_todo-list');
      });

      it('should convert to Title Case', () => {
        expect(result).to.equal('My Todo List');
      });
    });

    describe('when given an empty string', () => {
      let result: string;

      beforeEach(() => {
        result = el.formatTitle('');
      });

      it('should return an empty string', () => {
        expect(result).to.equal('');
      });
    });
  });

  describe('extractChecklistData', () => {
    describe('when frontmatter has no checklists', () => {
      let result: ReturnType<WikiChecklist['extractChecklistData']>;

      beforeEach(() => {
        const frontmatter: JsonObject = { title: 'Test' };
        result = el.extractChecklistData(frontmatter, 'grocery_list');
      });

      it('should return empty items', () => {
        expect(result.items).to.deep.equal([]);
      });

      it('should return null groupOrder', () => {
        expect(result.groupOrder).to.be.null;
      });
    });

    describe('when listName is not in checklists', () => {
      let result: ReturnType<WikiChecklist['extractChecklistData']>;

      beforeEach(() => {
        const frontmatter: JsonObject = {
          checklists: { other_list: { items: [] } },
        };
        result = el.extractChecklistData(frontmatter, 'grocery_list');
      });

      it('should return empty items', () => {
        expect(result.items).to.deep.equal([]);
      });
    });

    describe('when checklists contain the requested list', () => {
      let result: ReturnType<WikiChecklist['extractChecklistData']>;

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
        result = el.extractChecklistData(frontmatter, 'grocery_list');
      });

      it('should extract the correct number of items', () => {
        expect(result.items).to.have.length(2);
      });

      it('should extract plain items correctly', () => {
        expect(result.items[0]).to.deep.equal({ text: 'Milk', checked: false });
      });

      it('should extract tagged items correctly', () => {
        expect(result.items[1]).to.deep.equal({
          text: 'Eggs',
          checked: true,
          tag: 'Dairy',
        });
      });
    });

    describe('when checklist has group_order', () => {
      let result: ReturnType<WikiChecklist['extractChecklistData']>;

      beforeEach(() => {
        const frontmatter: JsonObject = {
          checklists: {
            grocery_list: {
              items: [],
              group_order: ['Dairy', 'Produce'],
            },
          },
        };
        result = el.extractChecklistData(frontmatter, 'grocery_list');
      });

      it('should extract group_order', () => {
        expect(result.groupOrder).to.deep.equal(['Dairy', 'Produce']);
      });
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
    type GroupedResult = ReturnType<WikiChecklist['getGroupedItems']>;
    let groups: GroupedResult;

    beforeEach(() => {
      el.items = [
        { text: 'Milk', checked: false, tag: 'Dairy' },
        { text: 'Bread', checked: false, tag: 'Bakery' },
        { text: 'Apples', checked: false, tag: 'Produce' },
        { text: 'Eggs', checked: true, tag: 'Dairy' },
        { text: 'Towels', checked: false },
      ];
      groups = el.getGroupedItems();
    });

    describe('grouping by tag', () => {
      let dairyGroup: GroupedResult[number] | undefined;

      beforeEach(() => {
        dairyGroup = groups.find(g => g.tag === 'Dairy');
      });

      it('should create a Dairy group', () => {
        expect(dairyGroup).to.exist;
      });

      it('should place both Dairy items in the group', () => {
        expect(dairyGroup!.items).to.have.length(2);
      });
    });

    describe('untagged items', () => {
      let otherGroup: GroupedResult[number] | undefined;

      beforeEach(() => {
        otherGroup = groups.find(g => g.tag === 'Other');
      });

      it('should place untagged items in an "Other" group', () => {
        expect(otherGroup).to.exist;
      });

      it('should contain exactly one untagged item', () => {
        expect(otherGroup!.items).to.have.length(1);
      });

      it('should contain the Towels item', () => {
        expect(otherGroup!.items[0]!.item.text).to.equal('Towels');
      });
    });

    describe('absolute index preservation', () => {
      let dairyIndices: number[];

      beforeEach(() => {
        const dairyGroup = groups.find(g => g.tag === 'Dairy');
        dairyIndices = dairyGroup!.items.map(i => i.index);
      });

      it('should preserve the Milk absolute index', () => {
        expect(dairyIndices).to.include(0);
      });

      it('should preserve the Eggs absolute index', () => {
        expect(dairyIndices).to.include(3);
      });
    });

    describe('when groupOrder is provided', () => {
      let orderedTags: string[];

      beforeEach(() => {
        el.groupOrder = ['Produce', 'Dairy', 'Bakery'];
        groups = el.getGroupedItems();
        orderedTags = groups.map(g => g.tag);
      });

      it('should place Produce first', () => {
        expect(orderedTags[0]).to.equal('Produce');
      });

      it('should place Dairy second', () => {
        expect(orderedTags[1]).to.equal('Dairy');
      });

      it('should place Bakery third', () => {
        expect(orderedTags[2]).to.equal('Bakery');
      });
    });

    describe('when groupOrder is null', () => {
      let taggedGroupTags: string[];

      beforeEach(() => {
        el.groupOrder = null;
        groups = el.getGroupedItems();
        taggedGroupTags = groups.filter(g => g.tag !== 'Other').map(g => g.tag);
      });

      it('should sort groups alphabetically', () => {
        expect(taggedGroupTags).to.deep.equal(['Bakery', 'Dairy', 'Produce']);
      });
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

  describe('when toggling a checkbox', () => {
    let getFrontmatterStub: SinonStub;
    let mergeFrontmatterStub: SinonStub;

    beforeEach(async () => {
      sinon.restore();
      el.remove();

      el = buildElement();
      const mockFrontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [
              { text: 'Milk', checked: false },
              { text: 'Eggs', checked: true },
            ],
          },
        },
      };
      getFrontmatterStub = sinon
        .stub(el.client, 'getFrontmatter')
        .resolves(
          create(GetFrontmatterResponseSchema, { frontmatter: mockFrontmatter })
        );
      mergeFrontmatterStub = sinon
        .stub(el.client, 'mergeFrontmatter')
        .resolves(create(MergeFrontmatterResponseSchema, { frontmatter: {} }));
      document.body.appendChild(el);
      // Wait for initial fetch + render of items
      await el.updateComplete;
      await el.updateComplete;

      const checkbox = el.shadowRoot!.querySelector('input[type="checkbox"]') as HTMLInputElement;
      checkbox.click();
      await el.updateComplete;
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

  describe('when saving state is active', () => {
    let savingDuringMerge: boolean;

    beforeEach(async () => {
      sinon.restore();
      el.remove();

      el = buildElement();
      const mockFrontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [{ text: 'Milk', checked: false }],
          },
        },
      };
      sinon
        .stub(el.client, 'getFrontmatter')
        .resolves(
          create(GetFrontmatterResponseSchema, { frontmatter: mockFrontmatter })
        );
      savingDuringMerge = false;
      sinon
        .stub(el.client, 'mergeFrontmatter')
        .callsFake(async () => {
          savingDuringMerge = el.saving;
          return create(MergeFrontmatterResponseSchema, { frontmatter: {} });
        });
      document.body.appendChild(el);
      // Wait for initial fetch + render of items
      await el.updateComplete;
      await el.updateComplete;

      const checkbox = el.shadowRoot!.querySelector('input[type="checkbox"]') as HTMLInputElement;
      checkbox.click();
      await el.updateComplete;
    });

    it('should be in saving state during the merge call', () => {
      expect(savingDuringMerge).to.be.true;
    });
  });

  describe('when persist fails via checkbox toggle', () => {
    beforeEach(async () => {
      sinon.restore();
      el.remove();

      el = buildElement();
      const mockFrontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [{ text: 'Milk', checked: false }],
          },
        },
      };
      sinon
        .stub(el.client, 'getFrontmatter')
        .resolves(
          create(GetFrontmatterResponseSchema, { frontmatter: mockFrontmatter })
        );
      sinon
        .stub(el.client, 'mergeFrontmatter')
        .rejects(new Error('Save failed'));
      document.body.appendChild(el);
      // Wait for initial fetch + render of items
      await el.updateComplete;
      await el.updateComplete;

      const checkbox = el.shadowRoot!.querySelector('input[type="checkbox"]') as HTMLInputElement;
      checkbox.click();
      await el.updateComplete;
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

