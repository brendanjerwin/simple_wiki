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
    items: ChecklistItem[]
  ): Promise<void>;
  _handleItemDragStart(e: DragEvent, index: number): void;
  _handleItemDragOver(e: DragEvent, index: number): void;
  _handleItemDragLeave(e: DragEvent): void;
  _handleItemDrop(e: DragEvent, targetIndex: number): Promise<void>;
  _handleItemDragEnd(): void;
  _dragSourceItemIndex: number | null;
  _dragOverItemIndex: number | null;
  _dragOverItemPosition: 'before' | 'after';

  // Touch drag internals
  _handleTouchStart(e: TouchEvent, index: number): void;
  _handleTouchMove(e: TouchEvent): void;
  _handleTouchEnd(e: TouchEvent): void;
  _handleTouchCancel(): void;
  _startTouchDrag(index: number, touch: Touch): void;
  _cancelLongPress(): void;
  _cleanupTouchDrag(): void;
  _touchDragActive: boolean;
  _longPressTimerId: ReturnType<typeof setTimeout> | null;
  _longPressHandleIndex: number | null;
  _touchStartX: number;
  _touchStartY: number;
  _touchGhostEl: HTMLElement | null;
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

  describe('parseTaggedInput', () => {
    describe('when input has a single :tag at the start', () => {
      let result: ReturnType<WikiChecklist['parseTaggedInput']>;

      beforeEach(() => {
        result = el.parseTaggedInput(':Dairy Buy milk');
      });

      it('should extract the tag lowercased', () => {
        expect(result.tags).to.deep.equal(['dairy']);
      });

      it('should extract the text after the tag', () => {
        expect(result.text).to.equal('Buy milk');
      });
    });

    describe('when input has a single :tag at the end', () => {
      let result: ReturnType<WikiChecklist['parseTaggedInput']>;

      beforeEach(() => {
        result = el.parseTaggedInput('Buy milk :Dairy');
      });

      it('should extract the tag lowercased', () => {
        expect(result.tags).to.deep.equal(['dairy']);
      });

      it('should extract the text without the tag', () => {
        expect(result.text).to.equal('Buy milk');
      });
    });

    describe('when input has a :tag in the middle', () => {
      let result: ReturnType<WikiChecklist['parseTaggedInput']>;

      beforeEach(() => {
        result = el.parseTaggedInput('buy :dairy milk');
      });

      it('should extract the tag lowercased', () => {
        expect(result.tags).to.deep.equal(['dairy']);
      });

      it('should join the remaining text', () => {
        expect(result.text).to.equal('buy milk');
      });
    });

    describe('when input has multiple tags', () => {
      let result: ReturnType<WikiChecklist['parseTaggedInput']>;

      beforeEach(() => {
        result = el.parseTaggedInput('milk :dairy :fridge');
      });

      it('should extract all tags lowercased', () => {
        expect(result.tags).to.deep.equal(['dairy', 'fridge']);
      });

      it('should extract the remaining text', () => {
        expect(result.text).to.equal('milk');
      });
    });

    describe('when input has multiple tags scattered', () => {
      let result: ReturnType<WikiChecklist['parseTaggedInput']>;

      beforeEach(() => {
        result = el.parseTaggedInput(':dairy milk :fridge');
      });

      it('should extract all tags lowercased', () => {
        expect(result.tags).to.deep.equal(['dairy', 'fridge']);
      });

      it('should extract the remaining text', () => {
        expect(result.text).to.equal('milk');
      });
    });

    describe('when input has no tag', () => {
      let result: ReturnType<WikiChecklist['parseTaggedInput']>;

      beforeEach(() => {
        result = el.parseTaggedInput('Buy milk');
      });

      it('should have empty tags array', () => {
        expect(result.tags).to.deep.equal([]);
      });

      it('should use the full input as text', () => {
        expect(result.text).to.equal('Buy milk');
      });
    });

    describe('when input has :tag but no item text', () => {
      let result: ReturnType<WikiChecklist['parseTaggedInput']>;

      beforeEach(() => {
        result = el.parseTaggedInput(':Dairy');
      });

      it('should extract the tag lowercased', () => {
        expect(result.tags).to.deep.equal(['dairy']);
      });

      it('should have empty text', () => {
        expect(result.text).to.equal('');
      });
    });

    describe('when input has mixed case tag', () => {
      let result: ReturnType<WikiChecklist['parseTaggedInput']>;

      beforeEach(() => {
        result = el.parseTaggedInput('milk :DAIRY');
      });

      it('should lowercase the tag', () => {
        expect(result.tags).to.deep.equal(['dairy']);
      });
    });
  });

  describe('composeTaggedText', () => {
    describe('when item has tags', () => {
      let result: string;

      beforeEach(() => {
        result = el.composeTaggedText({ text: 'milk', checked: false, tags: ['dairy', 'fridge'] });
      });

      it('should append tags with :tag syntax', () => {
        expect(result).to.equal('milk :dairy :fridge');
      });
    });

    describe('when item has no tags', () => {
      let result: string;

      beforeEach(() => {
        result = el.composeTaggedText({ text: 'milk', checked: false, tags: [] });
      });

      it('should return just the text', () => {
        expect(result).to.equal('milk');
      });
    });

    describe('when item has a single tag', () => {
      let result: string;

      beforeEach(() => {
        result = el.composeTaggedText({ text: 'eggs', checked: true, tags: ['dairy'] });
      });

      it('should append the single tag', () => {
        expect(result).to.equal('eggs :dairy');
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

    describe('when checklists contain items with new tags array format', () => {
      let result: ReturnType<WikiChecklist['extractChecklistData']>;

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
        result = el.extractChecklistData(frontmatter, 'grocery_list');
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

      it('should wrap old tag string in an array', () => {
        expect(result.items[1]?.tags).to.deep.equal(['Dairy']);
      });
    });

    describe('when item has both tag and tags (tags takes precedence)', () => {
      let result: ReturnType<WikiChecklist['extractChecklistData']>;

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
        result = el.extractChecklistData(frontmatter, 'grocery_list');
      });

      it('should prefer the tags array over the tag string', () => {
        expect(result.items[0]?.tags).to.deep.equal(['new1', 'new2']);
      });
    });
  });

  describe('getExistingTags', () => {
    describe('when items have multiple tags', () => {
      beforeEach(() => {
        el.items = [
          { text: 'Milk', checked: false, tags: ['dairy'] },
          { text: 'Apples', checked: false, tags: ['produce'] },
          { text: 'Eggs', checked: true, tags: ['dairy', 'fridge'] },
          { text: 'Bread', checked: false, tags: [] },
        ];
      });

      it('should return unique tags sorted alphabetically', () => {
        expect(el.getExistingTags()).to.deep.equal(['dairy', 'fridge', 'produce']);
      });
    });

    describe('when items have no tags', () => {
      beforeEach(() => {
        el.items = [
          { text: 'Item 1', checked: false, tags: [] },
          { text: 'Item 2', checked: false, tags: [] },
        ];
      });

      it('should return empty array', () => {
        expect(el.getExistingTags()).to.deep.equal([]);
      });
    });
  });

  describe('getFilteredItems', () => {
    describe('when filterTags is empty', () => {
      let result: ReturnType<WikiChecklist['getFilteredItems']>;

      beforeEach(() => {
        el.items = [
          { text: 'Milk', checked: false, tags: ['dairy'] },
          { text: 'Apples', checked: false, tags: ['produce'] },
          { text: 'Bread', checked: false, tags: [] },
        ];
        el.filterTags = [];
        result = el.getFilteredItems();
      });

      it('should return all items', () => {
        expect(result).to.have.length(3);
      });
    });

    describe('when filterTags has a single matching tag', () => {
      let result: ReturnType<WikiChecklist['getFilteredItems']>;

      beforeEach(() => {
        el.items = [
          { text: 'Milk', checked: false, tags: ['dairy'] },
          { text: 'Apples', checked: false, tags: ['produce'] },
          { text: 'Eggs', checked: false, tags: ['dairy', 'fridge'] },
          { text: 'Bread', checked: false, tags: [] },
        ];
        el.filterTags = ['dairy'];
        result = el.getFilteredItems();
      });

      it('should return only items with the matching tag', () => {
        expect(result).to.have.length(2);
      });

      it('should include items that have the tag among multiple tags', () => {
        const texts = result.map(r => r.item.text);
        expect(texts).to.include('Eggs');
      });

      it('should preserve the original array index', () => {
        const eggEntry = result.find(r => r.item.text === 'Eggs');
        expect(eggEntry?.index).to.equal(2);
      });
    });

    describe('when filterTags has multiple tags (AND logic)', () => {
      let result: ReturnType<WikiChecklist['getFilteredItems']>;

      beforeEach(() => {
        el.items = [
          { text: 'Milk', checked: false, tags: ['dairy'] },
          { text: 'Eggs', checked: false, tags: ['dairy', 'fridge'] },
          { text: 'Cheese', checked: false, tags: ['dairy', 'fridge'] },
          { text: 'Apples', checked: false, tags: ['produce'] },
          { text: 'Butter', checked: false, tags: ['dairy'] },
        ];
        el.filterTags = ['dairy', 'fridge'];
        result = el.getFilteredItems();
      });

      it('should return only items matching ALL filter tags', () => {
        expect(result).to.have.length(2);
      });

      it('should include items that have both tags', () => {
        const texts = result.map(r => r.item.text);
        expect(texts).to.deep.equal(['Eggs', 'Cheese']);
      });

      it('should not include items that have only one of the filter tags', () => {
        const texts = result.map(r => r.item.text);
        expect(texts).to.not.include('Milk');
      });
    });

    describe('when filterTags matches no items', () => {
      let result: ReturnType<WikiChecklist['getFilteredItems']>;

      beforeEach(() => {
        el.items = [
          { text: 'Milk', checked: false, tags: ['dairy'] },
        ];
        el.filterTags = ['nonexistent'];
        result = el.getFilteredItems();
      });

      it('should return empty array', () => {
        expect(result).to.have.length(0);
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
          { text: 'Milk', checked: false, tags: [] },
          { text: 'Eggs', checked: true, tags: [] },
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
        el.items = [{ text: 'Done', checked: true, tags: [] }];
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

    describe('when items have tags', () => {
      let tagBadges: NodeListOf<Element> | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          { text: 'Milk', checked: false, tags: ['dairy', 'fridge'] },
          { text: 'Bread', checked: false, tags: [] },
        ];
        await el.updateComplete;
        tagBadges = el.shadowRoot?.querySelectorAll('.item-tag-badge');
      });

      it('should render a tag badge for each tag', () => {
        expect(tagBadges!.length).to.equal(2);
      });
    });

    describe('when focusing an item with tags', () => {
      let textInput: HTMLInputElement | null | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          { text: 'Milk', checked: false, tags: ['dairy', 'fridge'] },
        ];
        await el.updateComplete;
        textInput = el.shadowRoot?.querySelector<HTMLInputElement>('.item-text');
        textInput?.focus();
        textInput?.dispatchEvent(new FocusEvent('focus'));
        await el.updateComplete;
      });

      it('should set editingIndex', () => {
        expect((el as unknown as { editingIndex: number | null }).editingIndex).to.equal(0);
      });

      it('should show composed tagged text in the input', () => {
        expect(textInput?.value).to.equal('Milk :dairy :fridge');
      });

      it('should hide tag badges while editing', () => {
        const badges = el.shadowRoot?.querySelectorAll('.item-tag-badge');
        expect(badges?.length).to.equal(0);
      });
    });

    describe('when blurring an item after editing tags', () => {
      let mergeFrontmatterStub: SinonStub;

      beforeEach(async () => {
        sinon.restore();
        el.remove();
        el = buildElement();

        const initialFrontmatter: JsonObject = {
          checklists: {
            grocery_list: {
              items: [
                { text: 'Milk', checked: false, tags: ['dairy'] },
              ],
            },
          },
        };

        sinon
          .stub(el.client, 'getFrontmatter')
          .resolves(
            create(GetFrontmatterResponseSchema, { frontmatter: initialFrontmatter })
          );
        mergeFrontmatterStub = sinon
          .stub(el.client, 'mergeFrontmatter')
          .callsFake(async (req: { frontmatter?: JsonObject }) =>
            create(MergeFrontmatterResponseSchema, {
              frontmatter: req.frontmatter ?? {},
            })
          );

        document.body.appendChild(el);
        await el.updateComplete;
        await el.updateComplete;

        const textInput = el.shadowRoot?.querySelector<HTMLInputElement>('.item-text');
        textInput?.focus();
        textInput?.dispatchEvent(new FocusEvent('focus'));
        await el.updateComplete;

        if (textInput) {
          textInput.value = 'Milk :dairy :fridge';
          textInput.dispatchEvent(new FocusEvent('blur'));
        }
        await waitUntil(
          () => mergeFrontmatterStub.callCount > 0,
          'mergeFrontmatter should be called',
          { timeout: 2000 }
        );
        await el.updateComplete;
      });

      it('should update the item tags', () => {
        expect(el.items[0]?.tags).to.deep.equal(['dairy', 'fridge']);
      });

      it('should persist the updated tags', () => {
        const mergeArgs = mergeFrontmatterStub.getCall(0).args[0] as { frontmatter: JsonObject };
        const checklists = mergeArgs.frontmatter['checklists'] as JsonObject;
        const list = checklists['grocery_list'] as JsonObject;
        const items = list['items'] as JsonObject[];
        expect(items[0]?.['tags']).to.deep.equal(['dairy', 'fridge']);
      });
    });

    describe('tag filter bar', () => {
      describe('when items have tags', () => {
        let filterBar: Element | null | undefined;
        let pills: NodeListOf<Element> | undefined;

        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [
            { text: 'Milk', checked: false, tags: ['dairy'] },
            { text: 'Apples', checked: false, tags: ['produce'] },
          ];
          await el.updateComplete;
          filterBar = el.shadowRoot?.querySelector('.tag-filter-bar');
          pills = el.shadowRoot?.querySelectorAll('.tag-pill');
        });

        it('should render the tag filter bar', () => {
          expect(filterBar).to.exist;
        });

        it('should render a pill for each unique tag', () => {
          expect(pills!.length).to.equal(2);
        });
      });

      describe('when no items have tags', () => {
        let filterBar: Element | null | undefined;

        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [
            { text: 'Milk', checked: false, tags: [] },
          ];
          await el.updateComplete;
          filterBar = el.shadowRoot?.querySelector('.tag-filter-bar');
        });

        it('should not render the tag filter bar', () => {
          expect(filterBar).to.not.exist;
        });
      });

      describe('when a filter tag is active', () => {
        let activePill: Element | null | undefined;
        let renderedItems: NodeListOf<Element> | undefined;

        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [
            { text: 'Milk', checked: false, tags: ['dairy'] },
            { text: 'Apples', checked: false, tags: ['produce'] },
            { text: 'Eggs', checked: false, tags: ['dairy'] },
          ];
          el.filterTags = ['dairy'];
          await el.updateComplete;
          activePill = el.shadowRoot?.querySelector('.tag-pill-active');
          renderedItems = el.shadowRoot?.querySelectorAll('.item-row');
        });

        it('should highlight the active filter pill', () => {
          expect(activePill).to.exist;
        });

        it('should only render items matching the filter', () => {
          expect(renderedItems!.length).to.equal(2);
        });
      });

      describe('when clicking a tag pill to activate filter', () => {
        let renderedItems: NodeListOf<Element> | undefined;

        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [
            { text: 'Milk', checked: false, tags: ['dairy'] },
            { text: 'Apples', checked: false, tags: ['produce'] },
            { text: 'Eggs', checked: false, tags: ['dairy'] },
          ];
          await el.updateComplete;

          const pills = el.shadowRoot?.querySelectorAll<HTMLButtonElement>('.tag-pill');
          const dairyPill = Array.from(pills ?? []).find(p => p.textContent?.trim() === 'dairy');
          dairyPill?.click();
          await el.updateComplete;
          renderedItems = el.shadowRoot?.querySelectorAll('.item-row');
        });

        it('should add clicked tag to filterTags', () => {
          expect(el.filterTags).to.include('dairy');
        });

        it('should filter displayed items', () => {
          expect(renderedItems!.length).to.equal(2);
        });
      });

      describe('when clicking the active tag pill to remove it from filter', () => {
        let renderedItems: NodeListOf<Element> | undefined;

        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [
            { text: 'Milk', checked: false, tags: ['dairy'] },
            { text: 'Apples', checked: false, tags: ['produce'] },
          ];
          el.filterTags = ['dairy'];
          await el.updateComplete;

          const activePill = el.shadowRoot?.querySelector<HTMLButtonElement>('.tag-pill-active');
          activePill?.click();
          await el.updateComplete;
          renderedItems = el.shadowRoot?.querySelectorAll('.item-row');
        });

        it('should remove the tag from filterTags', () => {
          expect(el.filterTags).to.deep.equal([]);
        });

        it('should show all items', () => {
          expect(renderedItems!.length).to.equal(2);
        });
      });

      describe('when a filter is active', () => {
        let clearBtn: HTMLButtonElement | null | undefined;

        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [
            { text: 'Milk', checked: false, tags: ['dairy'] },
            { text: 'Apples', checked: false, tags: ['produce'] },
          ];
          el.filterTags = ['dairy'];
          await el.updateComplete;
          clearBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.tag-filter-clear');
        });

        it('should show the clear filter button', () => {
          expect(clearBtn).to.exist;
        });
      });

      describe('when clicking the clear filter button', () => {
        let renderedItems: NodeListOf<Element> | undefined;

        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [
            { text: 'Milk', checked: false, tags: ['dairy'] },
            { text: 'Apples', checked: false, tags: ['produce'] },
          ];
          el.filterTags = ['dairy'];
          await el.updateComplete;

          const clearBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.tag-filter-clear');
          clearBtn?.click();
          await el.updateComplete;
          renderedItems = el.shadowRoot?.querySelectorAll('.item-row');
        });

        it('should clear filterTags', () => {
          expect(el.filterTags).to.deep.equal([]);
        });

        it('should show all items', () => {
          expect(renderedItems!.length).to.equal(2);
        });
      });

      describe('when no filter is active', () => {
        let clearBtn: HTMLButtonElement | null | undefined;

        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [
            { text: 'Milk', checked: false, tags: ['dairy'] },
          ];
          el.filterTags = [];
          await el.updateComplete;
          clearBtn = el.shadowRoot?.querySelector('.tag-filter-bar .tag-filter-clear') as HTMLButtonElement | null;
        });

        it('should not show the clear filter button', () => {
          expect(clearBtn).to.not.exist;
        });
      });
    });

    describe('delete checked button', () => {
      describe('when some items are checked', () => {
        let clearCheckedBtn: HTMLButtonElement | null | undefined;

        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [
            { text: 'Milk', checked: true, tags: [] },
            { text: 'Eggs', checked: false, tags: [] },
          ];
          await el.updateComplete;
          clearCheckedBtn = Array.from(
            el.shadowRoot?.querySelectorAll<HTMLButtonElement>('.delete-checked-btn') ?? []
          ).find(btn => btn.textContent?.trim() === 'delete checked');
        });

        it('should show the clear checked button', () => {
          expect(clearCheckedBtn).to.exist;
        });
      });

      describe('when no items are checked', () => {
        let clearCheckedBtn: HTMLButtonElement | null | undefined;

        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [
            { text: 'Milk', checked: false, tags: [] },
          ];
          await el.updateComplete;
          clearCheckedBtn = Array.from(
            el.shadowRoot?.querySelectorAll<HTMLButtonElement>('.delete-checked-btn') ?? []
          ).find(btn => btn.textContent?.trim() === 'delete checked');
        });

        it('should not show the clear checked button', () => {
          expect(clearCheckedBtn).to.not.exist;
        });
      });

      describe('when clicking clear checked', () => {
        let mergeFrontmatterStub: SinonStub;

        beforeEach(async () => {
          sinon.restore();
          el.remove();
          el = buildElement();

          const initialFrontmatter: JsonObject = {
            checklists: {
              grocery_list: {
                items: [
                  { text: 'Milk', checked: true, tags: [] },
                  { text: 'Eggs', checked: false, tags: [] },
                  { text: 'Bread', checked: true, tags: [] },
                ],
              },
            },
          };

          sinon
            .stub(el.client, 'getFrontmatter')
            .resolves(
              create(GetFrontmatterResponseSchema, { frontmatter: initialFrontmatter })
            );
          mergeFrontmatterStub = sinon
            .stub(el.client, 'mergeFrontmatter')
            .callsFake(async (req: { frontmatter?: JsonObject }) =>
              create(MergeFrontmatterResponseSchema, {
                frontmatter: req.frontmatter ?? {},
              })
            );

          document.body.appendChild(el);
          await el.updateComplete;
          await el.updateComplete;

          const clearCheckedBtn = Array.from(
            el.shadowRoot?.querySelectorAll<HTMLButtonElement>('.delete-checked-btn') ?? []
          ).find(btn => btn.textContent?.trim() === 'delete checked');
          clearCheckedBtn?.click();
          await waitUntil(
            () => mergeFrontmatterStub.callCount > 0,
            'mergeFrontmatter should be called',
            { timeout: 2000 }
          );
          await el.updateComplete;
        });

        it('should remove checked items', () => {
          expect(el.items).to.have.length(1);
        });

        it('should keep only unchecked items', () => {
          expect(el.items[0]?.text).to.equal('Eggs');
        });
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
              { text: 'Eggs', checked: true, tags: ['dairy'] },
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
      expect(items[1]?.tags).to.deep.equal(['dairy']);
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
        addInput.value = 'Milk :dairy';
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
    });

    it('should include the tags array in the persisted item', () => {
      expect(mergePayloadItems[0]?.['tags']).to.deep.equal(['dairy']);
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

  describe('drag-and-drop reordering', () => {
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
            { text: 'A', checked: false, tags: [] },
            { text: 'B', checked: false, tags: [] },
            { text: 'C', checked: false, tags: [] },
            { text: 'D', checked: false, tags: [] },
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
            { text: 'A', checked: false, tags: [] },
            { text: 'B', checked: false, tags: [] },
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
          items = [{ text: 'A', checked: false, tags: [] }];
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
            { text: 'Milk', checked: false, tags: ['dairy'] },
            { text: 'Bread', checked: false, tags: ['bakery'] },
            { text: 'Apples', checked: false, tags: ['produce'] },
            { text: 'Eggs', checked: false, tags: ['dairy'] },
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

    describe('rendering with drag support', () => {
      let handles: NodeListOf<Element> | undefined;
      let rows: NodeListOf<Element> | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          { text: 'Milk', checked: false, tags: ['dairy'] },
          { text: 'Bread', checked: false, tags: ['bakery'] },
          { text: 'Eggs', checked: false, tags: ['dairy'] },
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
    });

    describe('when dragstart originates from the text input', () => {
      let dragEvent: DragEvent;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          { text: 'Milk', checked: false, tags: ['dairy'] },
          { text: 'Bread', checked: false, tags: ['bakery'] },
        ];
        await el.updateComplete;
        const textInput = el.shadowRoot?.querySelector<HTMLInputElement>('.item-text');
        dragEvent = new DragEvent('dragstart', {
          bubbles: true,
          cancelable: true,
        });
        Object.defineProperty(dragEvent, 'target', { value: textInput });
        (el as unknown as WikiChecklistInternal)._handleItemDragStart(dragEvent, 0);
      });

      it('should cancel the drag', () => {
        expect(dragEvent.defaultPrevented).to.be.true;
      });

      it('should not set _dragSourceItemIndex', () => {
        expect((el as unknown as WikiChecklistInternal)._dragSourceItemIndex).to.be.null;
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
          { text: 'Milk', checked: false, tags: [] },
          { text: 'Bread', checked: false, tags: [] },
          { text: 'Eggs', checked: false, tags: [] },
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

    describe('dragleave handler', () => {
      let internal: WikiChecklistInternal;

      beforeEach(() => {
        internal = el as unknown as WikiChecklistInternal;
      });

      describe('when cursor leaves item row to an element outside the row', () => {
        beforeEach(() => {
          internal._dragOverItemIndex = 1;
          const itemElement = document.createElement('li');
          const outsideElement = document.createElement('div');
          const leaveEvent = new DragEvent('dragleave', { cancelable: true });
          Object.defineProperty(leaveEvent, 'currentTarget', {
            value: itemElement,
          });
          Object.defineProperty(leaveEvent, 'relatedTarget', {
            value: outsideElement,
          });
          internal._handleItemDragLeave(leaveEvent);
        });

        it('should clear _dragOverItemIndex', () => {
          expect(internal._dragOverItemIndex).to.be.null;
        });
      });

      describe('when cursor leaves item row to a child element within the row', () => {
        beforeEach(() => {
          internal._dragOverItemIndex = 1;
          const itemElement = document.createElement('li');
          const childElement = document.createElement('span');
          itemElement.appendChild(childElement);
          const leaveEvent = new DragEvent('dragleave', { cancelable: true });
          Object.defineProperty(leaveEvent, 'currentTarget', {
            value: itemElement,
          });
          Object.defineProperty(leaveEvent, 'relatedTarget', {
            value: childElement,
          });
          internal._handleItemDragLeave(leaveEvent);
        });

        it('should not clear _dragOverItemIndex', () => {
          expect(internal._dragOverItemIndex).to.equal(1);
        });
      });
    });
  });

  describe('touch drag reordering', () => {
    let internal: WikiChecklistInternal;
    let mergeFrontmatterStub: SinonStub;

    function makeTouchEvent(
      type: string,
      clientX: number,
      clientY: number,
      target?: EventTarget
    ): TouchEvent {
      const touch = new Touch({
        identifier: 0,
        target: target ?? el,
        clientX,
        clientY,
      });
      return new TouchEvent(type, {
        cancelable: true,
        bubbles: true,
        touches: type === 'touchend' || type === 'touchcancel' ? [] : [touch],
        changedTouches: [touch],
      });
    }

    function makeTouch(clientX: number, clientY: number): Touch {
      return new Touch({
        identifier: 0,
        target: el,
        clientX,
        clientY,
      });
    }

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
        { text: 'Milk', checked: false, tags: [] },
        { text: 'Bread', checked: false, tags: [] },
        { text: 'Eggs', checked: false, tags: [] },
      ];
      await el.updateComplete;
      internal = el as unknown as WikiChecklistInternal;
    });

    describe('when touching the drag handle', () => {
      beforeEach(() => {
        const touchEvent = makeTouchEvent('touchstart', 100, 200);
        internal._handleTouchStart(touchEvent, 1);
      });

      it('should start long-press timer', () => {
        expect(internal._longPressTimerId).to.not.be.null;
      });

      it('should record the handle index', () => {
        expect(internal._longPressHandleIndex).to.equal(1);
      });

      it('should record the start coordinates', () => {
        expect(internal._touchStartX).to.equal(100);
        expect(internal._touchStartY).to.equal(200);
      });

      it('should not yet activate touch drag', () => {
        expect(internal._touchDragActive).to.be.false;
      });
    });

    describe('when long-press activates (calling _startTouchDrag directly)', () => {
      beforeEach(() => {
        const touch = makeTouch(100, 200);
        internal._startTouchDrag(1, touch);
      });

      it('should set _touchDragActive to true', () => {
        expect(internal._touchDragActive).to.be.true;
      });

      it('should set _dragSourceItemIndex', () => {
        expect(internal._dragSourceItemIndex).to.equal(1);
      });

      it('should create a ghost element in shadow root', () => {
        expect(internal._touchGhostEl).to.not.be.null;
        const ghost = el.shadowRoot?.querySelector('.touch-drag-ghost');
        expect(ghost).to.not.be.null;
      });
    });

    describe('when touch moves >10px before long-press', () => {
      beforeEach(() => {
        const touchEvent = makeTouchEvent('touchstart', 100, 200);
        internal._handleTouchStart(touchEvent, 1);

        // Move 15px to the right (exceeds 10px threshold)
        const moveEvent = makeTouchEvent('touchmove', 115, 200);
        internal._handleTouchMove(moveEvent);
      });

      it('should cancel long-press timer', () => {
        expect(internal._longPressTimerId).to.be.null;
      });

      it('should keep _touchDragActive false', () => {
        expect(internal._touchDragActive).to.be.false;
      });

      it('should clear _longPressHandleIndex', () => {
        expect(internal._longPressHandleIndex).to.be.null;
      });
    });

    describe('when touch moves <10px before long-press', () => {
      beforeEach(() => {
        const touchEvent = makeTouchEvent('touchstart', 100, 200);
        internal._handleTouchStart(touchEvent, 1);

        // Move 5px (under the 10px threshold)
        const moveEvent = makeTouchEvent('touchmove', 103, 204);
        internal._handleTouchMove(moveEvent);
      });

      it('should NOT cancel long-press timer', () => {
        expect(internal._longPressTimerId).to.not.be.null;
      });

      it('should keep _longPressHandleIndex set', () => {
        expect(internal._longPressHandleIndex).to.equal(1);
      });
    });

    describe('when touch moves during active drag', () => {
      beforeEach(() => {
        // Activate touch drag directly
        const touch = makeTouch(100, 200);
        internal._startTouchDrag(0, touch);

        // Stub elementFromPoint on the shadow root to return item at index 2
        const row = el.shadowRoot?.querySelectorAll('.item-row')[2];
        if (row instanceof HTMLElement) {
          row.getBoundingClientRect = () =>
            ({ top: 280, bottom: 320, height: 40, left: 0, right: 200, width: 200 }) as DOMRect;
        }
        sinon.stub(el.shadowRoot!, 'elementFromPoint').returns(row ?? null);

        // Move to y=310 (in the lower half of item at index 2)
        const moveEvent = makeTouchEvent('touchmove', 100, 310);
        internal._handleTouchMove(moveEvent);
      });

      it('should update _dragOverItemIndex', () => {
        expect(internal._dragOverItemIndex).to.equal(2);
      });

      it('should update _dragOverItemPosition', () => {
        expect(internal._dragOverItemPosition).to.equal('after');
      });
    });

    describe('when touch ends during active drag', () => {
      beforeEach(async () => {
        // Activate touch drag directly
        const touch = makeTouch(100, 200);
        internal._startTouchDrag(2, touch);

        // Set up the drop target — simulate dragging to before item 0
        internal._dragOverItemIndex = 0;
        internal._dragOverItemPosition = 'before';

        const endEvent = makeTouchEvent('touchend', 100, 100);
        internal._handleTouchEnd(endEvent);

        // Wait for persistData's async call to complete
        await el.updateComplete;
      });

      it('should set _touchDragActive to false', () => {
        expect(internal._touchDragActive).to.be.false;
      });

      it('should remove ghost element', () => {
        expect(internal._touchGhostEl).to.be.null;
      });

      it('should reorder items', () => {
        expect(el.items[0]?.text).to.equal('Eggs');
      });

      it('should call persistData (mergeFrontmatter)', () => {
        expect(mergeFrontmatterStub.called).to.be.true;
      });
    });

    describe('when touch is cancelled during active drag', () => {
      let originalItems: ChecklistItem[];

      beforeEach(() => {
        // Activate touch drag directly
        const touch = makeTouch(100, 200);
        internal._startTouchDrag(1, touch);

        originalItems = [...el.items];
        internal._dragOverItemIndex = 0;
        internal._dragOverItemPosition = 'before';

        internal._handleTouchCancel();
      });

      it('should NOT reorder items', () => {
        expect(el.items.map(i => i.text)).to.deep.equal(originalItems.map(i => i.text));
      });

      it('should remove ghost element', () => {
        expect(internal._touchGhostEl).to.be.null;
      });

      it('should clear drag state', () => {
        expect(internal._touchDragActive).to.be.false;
        expect(internal._dragSourceItemIndex).to.be.null;
        expect(internal._dragOverItemIndex).to.be.null;
      });
    });

    describe('when touch ends before long-press fires', () => {
      beforeEach(() => {
        const touchEvent = makeTouchEvent('touchstart', 100, 200);
        internal._handleTouchStart(touchEvent, 1);

        // End before the 400ms fires — no active drag
        const endEvent = makeTouchEvent('touchend', 100, 200);
        internal._handleTouchEnd(endEvent);
      });

      it('should cancel long-press timer', () => {
        expect(internal._longPressTimerId).to.be.null;
      });

      it('should not set any drag state', () => {
        expect(internal._touchDragActive).to.be.false;
        expect(internal._dragSourceItemIndex).to.be.null;
      });
    });
  });
});
