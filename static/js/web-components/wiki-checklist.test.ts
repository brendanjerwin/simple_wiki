import { expect, waitUntil } from '@open-wc/testing';
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

// Helper type to access private methods for testing drag-and-drop
interface WikiChecklistInternal {
  persistData(
    items: ChecklistItem[],
    groupOrder: string[] | null
  ): Promise<void>;
  _handleItemDragStart(e: DragEvent, index: number): void;
  _handleItemDragOver(e: DragEvent, index: number): void;
  _handleItemDragLeave(e: DragEvent): void;
  _handleItemDrop(e: DragEvent, targetIndex: number, groupTag?: string): Promise<void>;
  _handleItemDragEnd(): void;
  _handleGroupDragStart(e: DragEvent, tag: string): void;
  _handleGroupDragOver(e: DragEvent, tag: string): void;
  _handleGroupDragLeave(e: DragEvent): void;
  _handleGroupDrop(e: DragEvent, tag: string): Promise<void>;
  _handleGroupDragEnd(): void;
  _dragSourceItemIndex: number | null;
  _dragOverItemIndex: number | null;
  _dragOverItemPosition: 'before' | 'after';
  _dragSourceGroupTag: string | null;
  _dragOverGroupTag: string | null;
  _dragOverGroupPosition: 'before' | 'after';
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

  /**
   * Extracts the checklist items from the first mergeFrontmatter stub call.
   * The component always merges under the 'grocery_list' key (the default list
   * name set by buildElement).
   */
  function getMergePayloadItems(stub: SinonStub): JsonObject[] {
    const mergeArgs = stub.getCall(0).args[0] as { frontmatter: JsonObject };
    const checklists = mergeArgs.frontmatter['checklists'] as JsonObject;
    const list = checklists['grocery_list'] as JsonObject;
    return list['items'] as JsonObject[];
  }

  function getMergePayloadChecklists(stub: SinonStub): JsonObject {
    const mergeArgs = stub.getCall(0).args[0] as { frontmatter: JsonObject };
    return mergeArgs.frontmatter['checklists'] as JsonObject;
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

    describe('when items have tags (tag autocomplete)', () => {
      let values: string[];

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          { text: 'Milk', checked: false, tag: 'Dairy' },
          { text: 'Apples', checked: false, tag: 'Produce' },
        ];
        await el.updateComplete;
        const options = el.shadowRoot?.querySelectorAll<HTMLOptionElement>(
          'datalist#tag-suggestions-grocery_list option'
        );
        values = Array.from(options ?? []).map(o => o.value);
      });

      it('should populate datalist with the Dairy tag', () => {
        expect(values).to.include('Dairy');
      });

      it('should populate datalist with the Produce tag', () => {
        expect(values).to.include('Produce');
      });
    });

    describe('when in flat view', () => {
      let tagBadges: NodeListOf<Element> | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.groupedView = false;
        el.items = [
          { text: 'Milk', checked: false, tag: 'Dairy' },
          { text: 'Bread', checked: false },
        ];
        await el.updateComplete;
        tagBadges = el.shadowRoot?.querySelectorAll('.item-tag-badge');
      });

      it('should render tag badges next to items', () => {
        expect(tagBadges!.length).to.be.greaterThan(0);
      });

      it('should render a badge for every item', () => {
        expect(tagBadges!.length).to.equal(el.items.length);
      });
    });
  });

  describe('when GetFrontmatter returns checklist items', () => {
    let items: ChecklistItem[];
    let getFrontmatterStub: SinonStub;
    let requestedPage: string;

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
      requestedPage = getFrontmatterStub.getCall(0).args[0].page;
    });

    it('should call getFrontmatter', () => {
      expect(getFrontmatterStub.callCount).to.be.greaterThan(0);
    });

    it('should request the configured page', () => {
      expect(requestedPage).to.equal('test-page');
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
    let mergePayloadItems: JsonObject[];
    let mergeChecklists: JsonObject;

    beforeEach(async () => {
      sinon.restore();
      el.remove();
      el = buildElement();

      const currentFrontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [
              { text: 'Milk', checked: false },
              { text: 'Eggs', checked: false },
            ],
          },
          other_list: {
            items: [{ text: 'Paper towels', checked: false }],
          },
        },
      };

      getFrontmatterStub = sinon
        .stub(el.client, 'getFrontmatter')
        .resolves(
          create(GetFrontmatterResponseSchema, {
            frontmatter: currentFrontmatter,
          })
        );
      mergeFrontmatterStub = sinon
        .stub(el.client, 'mergeFrontmatter')
        .resolves(
          create(MergeFrontmatterResponseSchema, {
            frontmatter: currentFrontmatter,
          })
        );

      document.body.appendChild(el);
      await el.updateComplete;
      await el.updateComplete;

      getFrontmatterStub.resetHistory();

      const checkbox = el.shadowRoot?.querySelector<HTMLInputElement>(
        'input[type="checkbox"]'
      );
      checkbox?.click();
      await waitUntil(
        () => mergeFrontmatterStub.callCount > 0,
        'mergeFrontmatter should be called',
        { timeout: 2000 }
      );
      await el.updateComplete;

      mergePayloadItems = getMergePayloadItems(mergeFrontmatterStub);
      mergeChecklists = getMergePayloadChecklists(mergeFrontmatterStub);
    });

    it('should call mergeFrontmatter', () => {
      expect(mergeFrontmatterStub).to.have.been.calledOnce;
    });

    it('should call getFrontmatter before mergeFrontmatter (read-modify-write)', () => {
      expect(getFrontmatterStub).to.have.been.calledBefore(mergeFrontmatterStub);
    });

    it('should send the toggled checked state in the merge payload', () => {
      expect(mergePayloadItems[0]?.['checked']).to.be.true;
    });

    it('should preserve sibling checklists in the merge payload', () => {
      expect(mergeChecklists).to.have.property('other_list');
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

    describe('when external change arrives via poll', () => {
      let pollingEl: WikiChecklist;

      beforeEach(async () => {
        const initialFrontmatter: JsonObject = {
          checklists: {
            grocery_list: {
              items: [{ text: 'Milk', checked: false }],
            },
          },
        };
        const updatedFrontmatter: JsonObject = {
          checklists: {
            grocery_list: {
              items: [
                { text: 'Milk', checked: true },
                { text: 'Eggs', checked: false },
              ],
            },
          },
        };

        pollingEl = buildElement('test-page', 'grocery_list');
        const getFrontmatterStub = sinon.stub(
          pollingEl.client,
          'getFrontmatter'
        );
        getFrontmatterStub.onFirstCall().resolves(
          create(GetFrontmatterResponseSchema, {
            frontmatter: initialFrontmatter,
          })
        );
        getFrontmatterStub.resolves(
          create(GetFrontmatterResponseSchema, {
            frontmatter: updatedFrontmatter,
          })
        );

        document.body.appendChild(pollingEl);
        await pollingEl.updateComplete;
        await pollingEl.updateComplete;

        await clock.tickAsync(3001);
        await pollingEl.updateComplete;
      });

      afterEach(() => {
        pollingEl.remove();
      });

      it('should reflect new items from the API after a poll', () => {
        expect(pollingEl.items).to.have.length(2);
      });

      it('should reflect checked state changes from the API after a poll', () => {
        expect(pollingEl.items[0]?.checked).to.be.true;
      });
    });
  });

  describe('when adding an item', () => {
    let mergeFrontmatterStub: SinonStub;
    let mergePayloadItems: JsonObject[];
    let addInputValue: string;

    beforeEach(async () => {
      sinon.restore();
      el.remove();
      el = buildElement();

      const initialFrontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [{ text: 'Milk', checked: false }],
          },
        },
      };

      sinon
        .stub(el.client, 'getFrontmatter')
        .resolves(
          create(GetFrontmatterResponseSchema, {
            frontmatter: initialFrontmatter,
          })
        );
      mergeFrontmatterStub = sinon
        .stub(el.client, 'mergeFrontmatter')
        .resolves(
          create(MergeFrontmatterResponseSchema, {
            frontmatter: initialFrontmatter,
          })
        );

      document.body.appendChild(el);
      await el.updateComplete;
      await el.updateComplete;

      const addInput =
        el.shadowRoot?.querySelector<HTMLInputElement>('.add-text-input');
      if (addInput) {
        addInput.value = 'Bread';
        addInput.dispatchEvent(new InputEvent('input', { bubbles: true }));
      }
      await el.updateComplete;

      const addBtn =
        el.shadowRoot?.querySelector<HTMLButtonElement>('.add-btn');
      addBtn?.click();
      await waitUntil(
        () => mergeFrontmatterStub.callCount > 0,
        'mergeFrontmatter should be called',
        { timeout: 2000 }
      );
      await el.updateComplete;

      mergePayloadItems = getMergePayloadItems(mergeFrontmatterStub);
      const addInputAfter =
        el.shadowRoot?.querySelector<HTMLInputElement>('.add-text-input');
      addInputValue = addInputAfter?.value ?? '';
    });

    it('should call mergeFrontmatter with the new item appended', () => {
      const lastItem = mergePayloadItems[mergePayloadItems.length - 1];
      expect(lastItem?.['text']).to.equal('Bread');
    });

    it('should clear the add input after adding', () => {
      expect(addInputValue).to.equal('');
    });
  });

  describe('when adding an item with a tag', () => {
    let mergePayloadItems: JsonObject[];

    beforeEach(async () => {
      sinon.restore();
      el.remove();
      el = buildElement();

      const initialFrontmatter: JsonObject = {
        checklists: { grocery_list: { items: [] } },
      };

      sinon
        .stub(el.client, 'getFrontmatter')
        .resolves(
          create(GetFrontmatterResponseSchema, {
            frontmatter: initialFrontmatter,
          })
        );
      const mergeFrontmatterStub = sinon
        .stub(el.client, 'mergeFrontmatter')
        .resolves(
          create(MergeFrontmatterResponseSchema, {
            frontmatter: initialFrontmatter,
          })
        );

      document.body.appendChild(el);
      await el.updateComplete;
      await el.updateComplete;

      const addInput =
        el.shadowRoot?.querySelector<HTMLInputElement>('.add-text-input');
      if (addInput) {
        addInput.value = 'Milk';
        addInput.dispatchEvent(new InputEvent('input', { bubbles: true }));
      }
      const tagInput =
        el.shadowRoot?.querySelector<HTMLInputElement>('.add-tag-input');
      if (tagInput) {
        tagInput.value = 'Dairy';
        tagInput.dispatchEvent(new InputEvent('input', { bubbles: true }));
      }
      await el.updateComplete;

      const addBtn =
        el.shadowRoot?.querySelector<HTMLButtonElement>('.add-btn');
      addBtn?.click();
      await waitUntil(
        () => mergeFrontmatterStub.callCount > 0,
        'mergeFrontmatter should be called',
        { timeout: 2000 }
      );
      await el.updateComplete;

      mergePayloadItems = getMergePayloadItems(mergeFrontmatterStub);
    });

    it('should include the tag in the persisted item', () => {
      expect(mergePayloadItems[0]?.['tag']).to.equal('Dairy');
    });
  });

  describe('when removing an item', () => {
    let mergePayloadItems: JsonObject[];

    beforeEach(async () => {
      sinon.restore();
      el.remove();
      el = buildElement();

      const initialFrontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [
              { text: 'Milk', checked: false },
              { text: 'Eggs', checked: false },
            ],
          },
        },
      };

      sinon
        .stub(el.client, 'getFrontmatter')
        .resolves(
          create(GetFrontmatterResponseSchema, {
            frontmatter: initialFrontmatter,
          })
        );
      const mergeFrontmatterStub = sinon
        .stub(el.client, 'mergeFrontmatter')
        .resolves(
          create(MergeFrontmatterResponseSchema, {
            frontmatter: initialFrontmatter,
          })
        );

      document.body.appendChild(el);
      await el.updateComplete;
      await el.updateComplete;

      const removeBtn =
        el.shadowRoot?.querySelector<HTMLButtonElement>('.remove-btn');
      removeBtn?.click();
      await waitUntil(
        () => mergeFrontmatterStub.callCount > 0,
        'mergeFrontmatter should be called',
        { timeout: 2000 }
      );
      await el.updateComplete;

      mergePayloadItems = getMergePayloadItems(mergeFrontmatterStub);
    });

    it('should reduce the item count by one', () => {
      expect(mergePayloadItems).to.have.length(1);
    });

    it('should keep the remaining item', () => {
      expect(mergePayloadItems[0]?.['text']).to.equal('Eggs');
    });
  });

  describe('when editing item text', () => {
    let mergePayloadItems: JsonObject[];

    beforeEach(async () => {
      sinon.restore();
      el.remove();
      el = buildElement();

      const initialFrontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [{ text: 'Milk', checked: false }],
          },
        },
      };

      sinon
        .stub(el.client, 'getFrontmatter')
        .resolves(
          create(GetFrontmatterResponseSchema, {
            frontmatter: initialFrontmatter,
          })
        );
      const mergeFrontmatterStub = sinon
        .stub(el.client, 'mergeFrontmatter')
        .resolves(
          create(MergeFrontmatterResponseSchema, {
            frontmatter: initialFrontmatter,
          })
        );

      document.body.appendChild(el);
      await el.updateComplete;
      await el.updateComplete;

      const textInput =
        el.shadowRoot?.querySelector<HTMLInputElement>('.item-text');
      if (textInput) {
        textInput.value = 'Whole Milk';
        textInput.dispatchEvent(new InputEvent('input', { bubbles: true }));
        textInput.dispatchEvent(new FocusEvent('blur', { bubbles: true }));
      }
      await waitUntil(
        () => mergeFrontmatterStub.callCount > 0,
        'mergeFrontmatter should be called',
        { timeout: 2000 }
      );
      await el.updateComplete;

      mergePayloadItems = getMergePayloadItems(mergeFrontmatterStub);
    });

    it('should call mergeFrontmatter with the updated text on blur', () => {
      expect(mergePayloadItems[0]?.['text']).to.equal('Whole Milk');
    });
  });

  describe('when editing item tag', () => {
    let mergePayloadItems: JsonObject[];

    beforeEach(async () => {
      sinon.restore();
      el.remove();
      el = buildElement();

      const initialFrontmatter: JsonObject = {
        checklists: {
          grocery_list: {
            items: [{ text: 'Milk', checked: false }],
          },
        },
      };

      sinon
        .stub(el.client, 'getFrontmatter')
        .resolves(
          create(GetFrontmatterResponseSchema, {
            frontmatter: initialFrontmatter,
          })
        );
      const mergeFrontmatterStub = sinon
        .stub(el.client, 'mergeFrontmatter')
        .resolves(
          create(MergeFrontmatterResponseSchema, {
            frontmatter: initialFrontmatter,
          })
        );

      document.body.appendChild(el);
      await el.updateComplete;
      await el.updateComplete;

      const tagBadge =
        el.shadowRoot?.querySelector<HTMLButtonElement>('.item-tag-badge');
      tagBadge?.click();
      await el.updateComplete;

      const tagInput =
        el.shadowRoot?.querySelector<HTMLInputElement>('.item-tag-input');
      if (tagInput) {
        tagInput.value = 'Dairy';
        tagInput.dispatchEvent(new FocusEvent('blur', { bubbles: true }));
      }
      await waitUntil(
        () => mergeFrontmatterStub.callCount > 0,
        'mergeFrontmatter should be called',
        { timeout: 2000 }
      );
      await el.updateComplete;

      mergePayloadItems = getMergePayloadItems(mergeFrontmatterStub);
    });

    it('should call mergeFrontmatter with the updated tag', () => {
      expect(mergePayloadItems[0]?.['tag']).to.equal('Dairy');
    });
  });

  describe('when toggling to grouped view', () => {
    let groupHeadings: NodeListOf<Element> | undefined;
    let headingTexts: (string | undefined)[];

    beforeEach(async () => {
      el.error = null;
      el.loading = false;
      el.groupedView = false;
      el.items = [
        { text: 'Milk', checked: false, tag: 'Dairy' },
        { text: 'Apples', checked: false, tag: 'Produce' },
        { text: 'Towels', checked: false },
      ];
      await el.updateComplete;

      const toggleBtn =
        el.shadowRoot?.querySelector<HTMLButtonElement>('.view-toggle');
      toggleBtn?.click();
      await el.updateComplete;

      groupHeadings = el.shadowRoot?.querySelectorAll('.group-header');
      headingTexts = Array.from(groupHeadings ?? []).map(
        h => h.textContent?.trim()
      );
    });

    it('should switch to grouped view', () => {
      expect(el.groupedView).to.be.true;
    });

    it('should render group headings for each tag', () => {
      expect(groupHeadings!.length).to.be.greaterThan(0);
    });

    it('should render an "Other" group for untagged items', () => {
      expect(headingTexts).to.include('Other');
    });
  });

  describe('when toggling back to flat view', () => {
    let groupHeadings: NodeListOf<Element> | undefined;

    beforeEach(async () => {
      el.error = null;
      el.loading = false;
      el.groupedView = true;
      el.items = [
        { text: 'Milk', checked: false, tag: 'Dairy' },
        { text: 'Apples', checked: false, tag: 'Produce' },
      ];
      await el.updateComplete;

      const toggleBtn =
        el.shadowRoot?.querySelector<HTMLButtonElement>('.view-toggle');
      toggleBtn?.click();
      await el.updateComplete;

      groupHeadings = el.shadowRoot?.querySelectorAll('.group-header');
    });

    it('should switch back to flat view', () => {
      expect(el.groupedView).to.be.false;
    });

    it('should not render group headings in flat view', () => {
      expect(groupHeadings!.length).to.equal(0);
    });
  });

  describe('drag-and-drop reordering', () => {
    describe('reorderItems', () => {
      describe('when moving from earlier to later index', () => {
        let result: ChecklistItem[];

        beforeEach(() => {
          const items: ChecklistItem[] = [
            { text: 'A', checked: false },
            { text: 'B', checked: false },
            { text: 'C', checked: false },
            { text: 'D', checked: false },
          ];
          result = el.reorderItems(items, 0, 3);
        });

        it('should place the moved item before the target', () => {
          expect(result.map(i => i.text)).to.deep.equal(['B', 'C', 'A', 'D']);
        });
      });

      describe('when moving from later to earlier index', () => {
        let result: ChecklistItem[];

        beforeEach(() => {
          const items: ChecklistItem[] = [
            { text: 'A', checked: false },
            { text: 'B', checked: false },
            { text: 'C', checked: false },
            { text: 'D', checked: false },
          ];
          result = el.reorderItems(items, 3, 1);
        });

        it('should place the moved item before the target', () => {
          expect(result.map(i => i.text)).to.deep.equal(['A', 'D', 'B', 'C']);
        });
      });

      describe('when moving item to same index', () => {
        let result: ChecklistItem[];

        beforeEach(() => {
          const items: ChecklistItem[] = [
            { text: 'A', checked: false },
            { text: 'B', checked: false },
          ];
          result = el.reorderItems(items, 1, 1);
        });

        it('should not change the order', () => {
          expect(result.map(i => i.text)).to.deep.equal(['A', 'B']);
        });
      });

      describe('when fromIndex is out of range', () => {
        let items: ChecklistItem[];
        let result: ChecklistItem[];

        beforeEach(() => {
          items = [{ text: 'A', checked: false }];
          result = el.reorderItems(items, 5, 0);
        });

        it('should return the original items', () => {
          expect(result).to.deep.equal(items);
        });
      });

      describe('when moving Eggs above Milk in Dairy group (plan example)', () => {
        let result: ChecklistItem[];

        beforeEach(() => {
          const items: ChecklistItem[] = [
            { text: 'Milk', checked: false, tag: 'Dairy' },
            { text: 'Bread', checked: false, tag: 'Bakery' },
            { text: 'Apples', checked: false, tag: 'Produce' },
            { text: 'Eggs', checked: false, tag: 'Dairy' },
          ];
          result = el.reorderItems(items, 3, 0);
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

    describe('computeNewGroupOrder', () => {
      describe('when moving earlier tag to before later tag', () => {
        let result: string[];

        beforeEach(() => {
          result = el.computeNewGroupOrder(
            ['Bakery', 'Dairy', 'Produce'],
            'Bakery',
            'Produce',
            'before'
          );
        });

        it('should place the tag before the target', () => {
          expect(result).to.deep.equal(['Dairy', 'Bakery', 'Produce']);
        });
      });

      describe('when moving later tag to before earlier tag', () => {
        let result: string[];

        beforeEach(() => {
          result = el.computeNewGroupOrder(
            ['Bakery', 'Dairy', 'Produce'],
            'Produce',
            'Dairy',
            'before'
          );
        });

        it('should place the tag before the target', () => {
          expect(result).to.deep.equal(['Bakery', 'Produce', 'Dairy']);
        });
      });

      describe('when moving tag after another tag', () => {
        let result: string[];

        beforeEach(() => {
          result = el.computeNewGroupOrder(
            ['Bakery', 'Dairy', 'Produce'],
            'Bakery',
            'Produce',
            'after'
          );
        });

        it('should place the tag after the target', () => {
          expect(result).to.deep.equal(['Dairy', 'Produce', 'Bakery']);
        });
      });

      describe('when fromTag is not found', () => {
        let originalTags: string[];
        let result: string[];

        beforeEach(() => {
          originalTags = ['Bakery', 'Dairy'];
          result = el.computeNewGroupOrder(originalTags, 'Missing', 'Bakery', 'before');
        });

        it('should return the original array', () => {
          expect(result).to.deep.equal(originalTags);
        });
      });

      describe('when toTag is not found', () => {
        let originalTags: string[];
        let result: string[];

        beforeEach(() => {
          originalTags = ['Bakery', 'Dairy'];
          result = el.computeNewGroupOrder(originalTags, 'Bakery', 'Missing', 'before');
        });

        it('should return the original array', () => {
          expect(result).to.deep.equal(originalTags);
        });
      });
    });

    describe('rendering with drag support', () => {
      let handles: NodeListOf<Element> | undefined;
      let rows: NodeListOf<Element> | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          { text: 'Milk', checked: false, tag: 'Dairy' },
          { text: 'Bread', checked: false, tag: 'Bakery' },
          { text: 'Eggs', checked: false, tag: 'Dairy' },
        ];
        await el.updateComplete;
        handles = el.shadowRoot?.querySelectorAll('.drag-handle');
        rows = el.shadowRoot?.querySelectorAll('.item-row');
      });

      it('should render a drag handle on each item', () => {
        expect(handles?.length).to.equal(3);
      });

      it('should render the first item with data-index 0', () => {
        expect(rows?.[0]?.getAttribute('data-index')).to.equal('0');
      });

      it('should render the second item with data-index 1', () => {
        expect(rows?.[1]?.getAttribute('data-index')).to.equal('1');
      });

      it('should render the third item with data-index 2', () => {
        expect(rows?.[2]?.getAttribute('data-index')).to.equal('2');
      });

      it('should render item rows as draggable', () => {
        expect(rows?.[0]?.getAttribute('draggable')).to.equal('true');
      });

      describe('when in grouped view', () => {
        let groupHeaders: NodeListOf<Element> | undefined;
        let groupedRows: NodeListOf<Element> | undefined;

        beforeEach(async () => {
          el.groupedView = true;
          await el.updateComplete;
          groupHeaders = el.shadowRoot?.querySelectorAll('.group-header');
          groupedRows = el.shadowRoot?.querySelectorAll('.item-row');
        });

        it('should render group headers as draggable', () => {
          expect(groupHeaders?.[0]?.getAttribute('draggable')).to.equal('true');
        });

        it('should preserve absolute data-index values on items', () => {
          const indices = Array.from(groupedRows ?? []).map(r =>
            r.getAttribute('data-index')
          );
          expect(indices).to.include('0');
          expect(indices).to.include('1');
          expect(indices).to.include('2');
        });
      });
    });

    describe('drop handler: flat view reordering', () => {
      let mergeFrontmatterStub: SinonStub;

      beforeEach(async () => {
        sinon.restore();
        sinon
          .stub(el.client, 'getFrontmatter')
          .resolves(create(GetFrontmatterResponseSchema, { frontmatter: {} }));
        mergeFrontmatterStub = sinon
          .stub(el.client, 'mergeFrontmatter')
          .callsFake(async (req: { frontmatter?: JsonObject }) =>
            create(MergeFrontmatterResponseSchema, {
              frontmatter: req.frontmatter ?? {},
            })
          );

        el.items = [
          { text: 'Milk', checked: false },
          { text: 'Bread', checked: false },
          { text: 'Eggs', checked: false },
        ];
        await el.updateComplete;
      });

      describe('when dropping item 2 before item 0', () => {
        beforeEach(async () => {
          const internal = el as unknown as WikiChecklistInternal;
          internal._dragSourceItemIndex = 2;
          internal._dragOverItemPosition = 'before';
          const dropEvent = new DragEvent('drop', { cancelable: true });
          await internal._handleItemDrop(dropEvent, 0);
        });

        it('should place Eggs first', () => {
          expect(el.items[0]?.text).to.equal('Eggs');
        });

        it('should place Milk second', () => {
          expect(el.items[1]?.text).to.equal('Milk');
        });

        it('should place Bread third', () => {
          expect(el.items[2]?.text).to.equal('Bread');
        });

        it('should call mergeFrontmatter to persist', () => {
          expect(mergeFrontmatterStub).to.have.been.called;
        });
      });

      describe('when drop completes', () => {
        let internal: WikiChecklistInternal;

        beforeEach(async () => {
          internal = el as unknown as WikiChecklistInternal;
          internal._dragSourceItemIndex = 1;
          internal._dragOverItemPosition = 'after';
          const dropEvent = new DragEvent('drop', { cancelable: true });
          await internal._handleItemDrop(dropEvent, 2);
        });

        it('should clear drag source index', () => {
          expect(internal._dragSourceItemIndex).to.be.null;
        });

        it('should clear drag over index', () => {
          expect(internal._dragOverItemIndex).to.be.null;
        });
      });

      describe('when no source item is set', () => {
        let originalItems: ChecklistItem[];

        beforeEach(async () => {
          const internal = el as unknown as WikiChecklistInternal;
          internal._dragSourceItemIndex = null;
          originalItems = [...el.items];
          const dropEvent = new DragEvent('drop', { cancelable: true });
          await internal._handleItemDrop(dropEvent, 0);
        });

        it('should not change items', () => {
          expect(el.items).to.deep.equal(originalItems);
        });

        it('should not call mergeFrontmatter', () => {
          expect(mergeFrontmatterStub).not.to.have.been.called;
        });
      });
    });

    describe('drop handler: cross-group reordering', () => {
      let mergeFrontmatterStub: SinonStub;

      beforeEach(async () => {
        sinon.restore();
        sinon
          .stub(el.client, 'getFrontmatter')
          .resolves(create(GetFrontmatterResponseSchema, { frontmatter: {} }));
        mergeFrontmatterStub = sinon
          .stub(el.client, 'mergeFrontmatter')
          .callsFake(async (req: { frontmatter?: JsonObject }) =>
            create(MergeFrontmatterResponseSchema, {
              frontmatter: req.frontmatter ?? {},
            })
          );

        el.items = [
          { text: 'Milk', checked: false, tag: 'Dairy' },
          { text: 'Bread', checked: false, tag: 'Bakery' },
          { text: 'Eggs', checked: false, tag: 'Dairy' },
        ];
        await el.updateComplete;
      });

      describe('when dropping on a different group', () => {
        let milk: ChecklistItem | undefined;

        beforeEach(async () => {
          const internal = el as unknown as WikiChecklistInternal;
          internal._dragSourceItemIndex = 0;
          internal._dragOverItemPosition = 'before';
          const dropEvent = new DragEvent('drop', { cancelable: true });
          await internal._handleItemDrop(dropEvent, 1, 'Bakery');
          milk = el.items.find(i => i.text === 'Milk');
        });

        it('should update the tag to the target group', () => {
          expect(milk?.tag).to.equal('Bakery');
        });

        it('should call mergeFrontmatter', () => {
          expect(mergeFrontmatterStub).to.have.been.called;
        });
      });

      describe('when dropping on Other group', () => {
        let milk: ChecklistItem | undefined;

        beforeEach(async () => {
          const internal = el as unknown as WikiChecklistInternal;
          internal._dragSourceItemIndex = 0;
          internal._dragOverItemPosition = 'before';
          const dropEvent = new DragEvent('drop', { cancelable: true });
          await internal._handleItemDrop(dropEvent, 1, 'Other');
          milk = el.items.find(i => i.text === 'Milk');
        });

        it('should remove the tag', () => {
          expect(milk?.tag).to.be.undefined;
        });
      });

      describe('when dropping in the same group', () => {
        beforeEach(async () => {
          const internal = el as unknown as WikiChecklistInternal;
          internal._dragSourceItemIndex = 2;
          internal._dragOverItemPosition = 'before';
          const dropEvent = new DragEvent('drop', { cancelable: true });
          await internal._handleItemDrop(dropEvent, 0, 'Dairy');
        });

        it('should not change the first item tag', () => {
          expect(el.items[0]?.tag).to.equal('Dairy');
        });

        it('should not change the second item tag', () => {
          expect(el.items[1]?.tag).to.equal('Dairy');
        });
      });
    });

    describe('drop handler: group heading reordering', () => {
      let mergeFrontmatterStub: SinonStub;

      beforeEach(async () => {
        sinon.restore();
        sinon
          .stub(el.client, 'getFrontmatter')
          .resolves(create(GetFrontmatterResponseSchema, { frontmatter: {} }));
        mergeFrontmatterStub = sinon
          .stub(el.client, 'mergeFrontmatter')
          .callsFake(async (req: { frontmatter?: JsonObject }) =>
            create(MergeFrontmatterResponseSchema, {
              frontmatter: req.frontmatter ?? {},
            })
          );

        el.items = [
          { text: 'Milk', checked: false, tag: 'Dairy' },
          { text: 'Bread', checked: false, tag: 'Bakery' },
          { text: 'Apples', checked: false, tag: 'Produce' },
        ];
        el.groupOrder = ['Bakery', 'Dairy', 'Produce'];
        await el.updateComplete;
      });

      describe('when reordering headings', () => {
        beforeEach(async () => {
          const internal = el as unknown as WikiChecklistInternal;
          internal._dragSourceGroupTag = 'Bakery';
          internal._dragOverGroupPosition = 'before';
          const dropEvent = new DragEvent('drop', { cancelable: true });
          await internal._handleGroupDrop(dropEvent, 'Produce');
        });

        it('should update groupOrder', () => {
          expect(el.groupOrder).to.deep.equal(['Dairy', 'Bakery', 'Produce']);
        });

        it('should call mergeFrontmatter', () => {
          expect(mergeFrontmatterStub).to.have.been.called;
        });
      });

      describe('when groupOrder is null', () => {
        beforeEach(async () => {
          el.groupOrder = null;
          const internal = el as unknown as WikiChecklistInternal;
          internal._dragSourceGroupTag = 'Dairy';
          internal._dragOverGroupPosition = 'before';
          const dropEvent = new DragEvent('drop', { cancelable: true });
          await internal._handleGroupDrop(dropEvent, 'Bakery');
        });

        it('should create groupOrder from alphabetical with the change applied', () => {
          expect(el.groupOrder).to.deep.equal(['Dairy', 'Bakery', 'Produce']);
        });
      });

      describe('when no source group tag is set', () => {
        beforeEach(async () => {
          const internal = el as unknown as WikiChecklistInternal;
          internal._dragSourceGroupTag = null;
          const dropEvent = new DragEvent('drop', { cancelable: true });
          await internal._handleGroupDrop(dropEvent, 'Produce');
        });

        it('should not call mergeFrontmatter', () => {
          expect(mergeFrontmatterStub).not.to.have.been.called;
        });
      });

      describe('when group drop completes', () => {
        let internal: WikiChecklistInternal;

        beforeEach(async () => {
          internal = el as unknown as WikiChecklistInternal;
          internal._dragSourceGroupTag = 'Bakery';
          internal._dragOverGroupPosition = 'before';
          const dropEvent = new DragEvent('drop', { cancelable: true });
          await internal._handleGroupDrop(dropEvent, 'Produce');
        });

        it('should clear drag source group tag', () => {
          expect(internal._dragSourceGroupTag).to.be.null;
        });

        it('should clear drag over group tag', () => {
          expect(internal._dragOverGroupTag).to.be.null;
        });
      });
    });

    describe('dragover handler', () => {
      describe('when cursor is in the upper half', () => {
        let internal: WikiChecklistInternal;

        beforeEach(() => {
          internal = el as unknown as WikiChecklistInternal;
          internal._dragSourceItemIndex = 0;

          const mockElement = document.createElement('li');
          mockElement.getBoundingClientRect = () =>
            ({ top: 100, bottom: 140, height: 40, left: 0, right: 200, width: 200 }) as DOMRect;

          const overEvent = new DragEvent('dragover', {
            cancelable: true,
            clientY: 115,
          });
          Object.defineProperty(overEvent, 'currentTarget', { value: mockElement });

          internal._handleItemDragOver(overEvent, 1);
        });

        it('should set dragOverItemIndex', () => {
          expect(internal._dragOverItemIndex).to.equal(1);
        });

        it('should set position to before', () => {
          expect(internal._dragOverItemPosition).to.equal('before');
        });
      });

      describe('when cursor is in the lower half', () => {
        let internal: WikiChecklistInternal;

        beforeEach(() => {
          internal = el as unknown as WikiChecklistInternal;
          internal._dragSourceItemIndex = 0;

          const mockElement = document.createElement('li');
          mockElement.getBoundingClientRect = () =>
            ({ top: 100, bottom: 140, height: 40, left: 0, right: 200, width: 200 }) as DOMRect;

          const overEvent = new DragEvent('dragover', {
            cancelable: true,
            clientY: 125,
          });
          Object.defineProperty(overEvent, 'currentTarget', { value: mockElement });

          internal._handleItemDragOver(overEvent, 1);
        });

        it('should set position to after', () => {
          expect(internal._dragOverItemPosition).to.equal('after');
        });
      });
    });
  });
});
