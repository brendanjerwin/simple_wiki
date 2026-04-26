import { expect, waitUntil } from '@open-wc/testing';
import sinon, { type SinonStub } from 'sinon';
import './wiki-checklist.js';
import type { WikiChecklist } from './wiki-checklist.js';
import { AugmentedError, AugmentErrorService } from './augment-error-service.js';
import { create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import { ConnectError, Code } from '@connectrpc/connect';
import {
  ListItemsResponseSchema,
  AddItemResponseSchema,
  UpdateItemResponseSchema,
  ToggleItemResponseSchema,
  DeleteItemResponseSchema,
  ReorderItemResponseSchema,
} from '../gen/api/v1/checklist_pb.js';
import type { ChecklistItem } from '../gen/api/v1/checklist_pb.js';
import { makeChecklist, makeChecklistItem } from './checklist-test-fixtures.js';

interface WikiChecklistInternal {
  _handleItemDragStart(e: DragEvent, index: number): void;
  _handleItemDragOver(e: DragEvent, index: number): void;
  _handleItemDragLeave(e: DragEvent): void;
  _handleItemDrop(e: DragEvent, targetIndex: number): Promise<void>;
  _handleItemDragEnd(e: DragEvent): void;
  _handleDragHandleMousedown(e: MouseEvent): void;
  _handleDragHandleKeydown(e: KeyboardEvent, index: number): void;
  _dragSourceItemIndex: number | null;
  _dragOverItemIndex: number | null;
  _dragOverItemPosition: 'before' | 'after';

  // Touch drag internals
  _handleTouchStart(e: TouchEvent, index: number): void;
  _handleTouchMove(e: TouchEvent): void;
  _handleTouchEnd(): Promise<void>;
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

  function stubListItems(
    target: WikiChecklist,
    items: ChecklistItem[] = [],
    updatedAtMs?: number,
  ): SinonStub {
    const checklist = updatedAtMs === undefined
      ? makeChecklist({ name: 'grocery_list', items })
      : makeChecklist({ name: 'grocery_list', items, updatedAtMs });
    return sinon
      .stub(target.client, 'listItems')
      .resolves(create(ListItemsResponseSchema, { checklist }));
  }

  beforeEach(async () => {
    el = buildElement();
    stubListItems(el);
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

  describe('when element has server-rendered fallback content', () => {
    beforeEach(async () => {
      el.remove();
      el = buildElement();
      el.innerHTML = '<span class="checklist-item">[ ] Buy milk</span><span class="checklist-item">[x] Walk dog</span>';
      stubListItems(el);
      document.body.appendChild(el);
      await el.updateComplete;
    });

    it('should remove all light DOM children after connecting', () => {
      expect(el.innerHTML).to.equal('');
    });

    it('should have no child nodes in light DOM', () => {
      expect(el.childNodes.length).to.equal(0);
    });
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

  describe('getExistingTags', () => {
    describe('when items have multiple tags', () => {
      beforeEach(() => {
        el.items = [
          makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
          makeChecklistItem({ text: 'Apples', tags: ['produce'] }),
          makeChecklistItem({ text: 'Eggs', checked: true, tags: ['dairy', 'fridge'] }),
          makeChecklistItem({ text: 'Bread' }),
        ];
      });

      it('should return unique tags sorted alphabetically', () => {
        expect(el.getExistingTags()).to.deep.equal(['dairy', 'fridge', 'produce']);
      });
    });

    describe('when items have no tags', () => {
      beforeEach(() => {
        el.items = [
          makeChecklistItem({ text: 'Item 1' }),
          makeChecklistItem({ text: 'Item 2' }),
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
          makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
          makeChecklistItem({ text: 'Apples', tags: ['produce'] }),
          makeChecklistItem({ text: 'Bread' }),
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
          makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
          makeChecklistItem({ text: 'Apples', tags: ['produce'] }),
          makeChecklistItem({ text: 'Eggs', tags: ['dairy', 'fridge'] }),
          makeChecklistItem({ text: 'Bread' }),
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
          makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
          makeChecklistItem({ text: 'Eggs', tags: ['dairy', 'fridge'] }),
          makeChecklistItem({ text: 'Cheese', tags: ['dairy', 'fridge'] }),
          makeChecklistItem({ text: 'Apples', tags: ['produce'] }),
          makeChecklistItem({ text: 'Butter', tags: ['dairy'] }),
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
        el.items = [makeChecklistItem({ text: 'Milk', tags: ['dairy'] })];
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
          makeChecklistItem({ text: 'Milk' }),
          makeChecklistItem({ text: 'Eggs', checked: true }),
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
        el.items = [makeChecklistItem({ text: 'Done', checked: true })];
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
          makeChecklistItem({ text: 'Milk', tags: ['dairy', 'fridge'] }),
          makeChecklistItem({ text: 'Bread' }),
        ];
        await el.updateComplete;
        tagBadges = el.shadowRoot?.querySelectorAll('.item-tag-badge');
      });

      it('should render a tag badge for each tag', () => {
        expect(tagBadges!.length).to.equal(2);
      });
    });

    describe('item text display and editing', () => {
      describe('when not editing an item', () => {
        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [makeChecklistItem({ text: 'Milk', tags: ['dairy'] })];
          await el.updateComplete;
        });

        it('should render a span for item text, not an input', () => {
          const span = el.shadowRoot?.querySelector('.item-display-text');
          const input = el.shadowRoot?.querySelector('.item-text');
          expect(span).to.exist;
          expect(input).to.be.null;
        });

        it('should display the item text in the span', () => {
          const span = el.shadowRoot?.querySelector('.item-display-text');
          expect(span?.textContent).to.equal('Milk');
        });
      });

      describe('when clicking the display text', () => {
        let textInput: HTMLInputElement | null | undefined;

        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [makeChecklistItem({ text: 'Milk', tags: ['dairy', 'fridge'] })];
          await el.updateComplete;

          const displaySpan = el.shadowRoot?.querySelector('.item-display-text');
          (displaySpan as HTMLElement)?.click();
          await el.updateComplete;
          textInput = el.shadowRoot?.querySelector<HTMLInputElement>('.item-text');
        });

        it('should switch to an input for editing', () => {
          expect(textInput).to.exist;
        });

        it('should show composed tagged text in the input', () => {
          expect(textInput?.value).to.equal('Milk #dairy #fridge');
        });

        it('should hide tag badges while editing', () => {
          const badges = el.shadowRoot?.querySelectorAll('.item-tag-badge');
          expect(badges?.length).to.equal(0);
        });

        it('should hide the display span while editing', () => {
          const displaySpan = el.shadowRoot?.querySelector('.item-display-text');
          expect(displaySpan).to.be.null;
        });
      });

      describe('when blurring the edit input', () => {
        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [makeChecklistItem({ text: 'Milk' })];
          await el.updateComplete;

          // Enter edit mode
          const displaySpan = el.shadowRoot?.querySelector('.item-display-text');
          (displaySpan as HTMLElement)?.click();
          await el.updateComplete;

          // Stub updateItem so the persist completes
          sinon.stub(el.client, 'updateItem').resolves(
            create(UpdateItemResponseSchema, { checklist: makeChecklist() })
          );

          const textInput = el.shadowRoot?.querySelector<HTMLInputElement>('.item-text');
          if (textInput) {
            textInput.value = 'Milk';
            textInput.dispatchEvent(new FocusEvent('blur'));
          }
          await el.updateComplete;
        });

        it('should switch back to a span', () => {
          const span = el.shadowRoot?.querySelector('.item-display-text');
          const input = el.shadowRoot?.querySelector('.item-text');
          expect(span).to.exist;
          expect(input).to.be.null;
        });
      });

      describe('when item text is long', () => {
        beforeEach(async () => {
          el.error = null;
          el.loading = false;
          el.items = [makeChecklistItem({
            text: 'This is a very long checklist item text that should wrap on mobile screens',
          })];
          await el.updateComplete;
        });

        it('should render text in a span that can wrap', () => {
          const span = el.shadowRoot?.querySelector('.item-display-text');
          const input = el.shadowRoot?.querySelector('.item-text');
          expect(span).to.exist;
          expect(input).to.be.null;
          expect(span?.textContent).to.equal('This is a very long checklist item text that should wrap on mobile screens');
        });
      });
    });

    describe('when blurring an item after editing tags', () => {
      let updateItemStub: SinonStub;

      beforeEach(async () => {
        sinon.restore();
        el.remove();
        el = buildElement();

        stubListItems(el, [
          makeChecklistItem({ uid: 'uid-milk', text: 'Milk', tags: ['dairy'] }),
        ]);
        updateItemStub = sinon
          .stub(el.client, 'updateItem')
          .callsFake(async () =>
            create(UpdateItemResponseSchema, { checklist: makeChecklist() })
          );

        document.body.appendChild(el);
        await el.updateComplete;
        await el.updateComplete;

        const displaySpan = el.shadowRoot?.querySelector('.item-display-text');
        (displaySpan as HTMLElement)?.click();
        await el.updateComplete;

        const textInput = el.shadowRoot?.querySelector<HTMLInputElement>('.item-text');
        if (textInput) {
          textInput.value = 'Milk #dairy #fridge';
          textInput.dispatchEvent(new FocusEvent('blur'));
        }
        await waitUntil(
          () => updateItemStub.callCount > 0,
          'updateItem should be called',
          { timeout: 2000 }
        );
        await el.updateComplete;
      });

      it('should send the updated tags in the request', () => {
        const args = updateItemStub.getCall(0).args[0] as { tags: string[] };
        expect(args.tags).to.deep.equal(['dairy', 'fridge']);
      });

      it('should send the targeted item uid', () => {
        const args = updateItemStub.getCall(0).args[0] as { uid: string };
        expect(args.uid).to.equal('uid-milk');
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
            makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
            makeChecklistItem({ text: 'Apples', tags: ['produce'] }),
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
          el.items = [makeChecklistItem({ text: 'Milk' })];
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
            makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
            makeChecklistItem({ text: 'Apples', tags: ['produce'] }),
            makeChecklistItem({ text: 'Eggs', tags: ['dairy'] }),
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
            makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
            makeChecklistItem({ text: 'Apples', tags: ['produce'] }),
            makeChecklistItem({ text: 'Eggs', tags: ['dairy'] }),
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
            makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
            makeChecklistItem({ text: 'Apples', tags: ['produce'] }),
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
            makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
            makeChecklistItem({ text: 'Apples', tags: ['produce'] }),
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
            makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
            makeChecklistItem({ text: 'Apples', tags: ['produce'] }),
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
          el.items = [makeChecklistItem({ text: 'Milk', tags: ['dairy'] })];
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
            makeChecklistItem({ text: 'Milk', checked: true }),
            makeChecklistItem({ text: 'Eggs' }),
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
          el.items = [makeChecklistItem({ text: 'Milk' })];
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
        let deleteItemStub: SinonStub;

        beforeEach(async () => {
          sinon.restore();
          el.remove();
          el = buildElement();

          stubListItems(el, [
            makeChecklistItem({ uid: 'uid-milk', text: 'Milk', checked: true }),
            makeChecklistItem({ uid: 'uid-eggs', text: 'Eggs' }),
            makeChecklistItem({ uid: 'uid-bread', text: 'Bread', checked: true }),
          ]);
          deleteItemStub = sinon
            .stub(el.client, 'deleteItem')
            .callsFake(async () =>
              create(DeleteItemResponseSchema, { checklist: makeChecklist() })
            );

          document.body.appendChild(el);
          await el.updateComplete;
          await el.updateComplete;

          const clearCheckedBtn = Array.from(
            el.shadowRoot?.querySelectorAll<HTMLButtonElement>('.delete-checked-btn') ?? []
          ).find(btn => btn.textContent?.trim() === 'delete checked');
          clearCheckedBtn?.click();
          await waitUntil(
            () => deleteItemStub.callCount === 2,
            'deleteItem should be called for each checked item',
            { timeout: 2000 }
          );
          await el.updateComplete;
        });

        it('should call deleteItem once per checked item', () => {
          expect(deleteItemStub.callCount).to.equal(2);
        });

        it('should target the checked uids', () => {
          const uids = deleteItemStub.getCalls().map(c => (c.args[0] as { uid: string }).uid);
          expect(uids).to.have.members(['uid-milk', 'uid-bread']);
        });
      });
    });
  });

  describe('when ListItems returns checklist items', () => {
    let items: ChecklistItem[];
    let listItemsStub: SinonStub;
    let requestedPage: string;
    let requestedListName: string;

    beforeEach(async () => {
      sinon.restore();
      el.remove();

      el = buildElement();
      listItemsStub = stubListItems(el, [
        makeChecklistItem({ text: 'Milk' }),
        makeChecklistItem({ text: 'Eggs', checked: true, tags: ['dairy'] }),
      ]);
      document.body.appendChild(el);
      await el.updateComplete;
      items = el.items;
      requestedPage = (listItemsStub.getCall(0).args[0] as { page: string }).page;
      requestedListName = (listItemsStub.getCall(0).args[0] as { listName: string }).listName;
    });

    it('should call listItems', () => {
      expect(listItemsStub.callCount).to.be.greaterThan(0);
    });

    it('should request the configured page', () => {
      expect(requestedPage).to.equal('test-page');
    });

    it('should request the configured listName', () => {
      expect(requestedListName).to.equal('grocery_list');
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

  describe('when ListItems fails', () => {
    beforeEach(async () => {
      sinon.restore();
      el.remove();

      el = buildElement();
      sinon
        .stub(el.client, 'listItems')
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
    let toggleItemStub: SinonStub;

    beforeEach(async () => {
      sinon.restore();
      el.remove();
      el = buildElement();

      stubListItems(el, [
        makeChecklistItem({ uid: 'uid-milk', text: 'Milk' }),
        makeChecklistItem({ uid: 'uid-eggs', text: 'Eggs' }),
      ]);
      toggleItemStub = sinon
        .stub(el.client, 'toggleItem')
        .resolves(create(ToggleItemResponseSchema, { checklist: makeChecklist() }));

      document.body.appendChild(el);
      await el.updateComplete;
      await el.updateComplete;

      const checkbox = el.shadowRoot?.querySelector<HTMLInputElement>(
        'input[type="checkbox"]'
      );
      checkbox?.click();
      await waitUntil(
        () => toggleItemStub.callCount > 0,
        'toggleItem should be called',
        { timeout: 2000 }
      );
      await el.updateComplete;
    });

    it('should call toggleItem exactly once', () => {
      expect(toggleItemStub).to.have.been.calledOnce;
    });

    it('should pass the toggled item uid', () => {
      const args = toggleItemStub.getCall(0).args[0] as { uid: string };
      expect(args.uid).to.equal('uid-milk');
    });

    it('should pass the listName and page', () => {
      const args = toggleItemStub.getCall(0).args[0] as { page: string; listName: string };
      expect(args.page).to.equal('test-page');
      expect(args.listName).to.equal('grocery_list');
    });

    it('should clear saving state after completion', () => {
      expect(el.saving).to.be.false;
    });
  });

  describe('when saving state is active', () => {
    let savingDuringMutation: boolean;

    beforeEach(async () => {
      sinon.restore();
      el.remove();

      el = buildElement();
      stubListItems(el, [
        makeChecklistItem({ uid: 'uid-milk', text: 'Milk' }),
      ]);
      savingDuringMutation = false;
      sinon
        .stub(el.client, 'toggleItem')
        .callsFake(async () => {
          savingDuringMutation = el.saving;
          return create(ToggleItemResponseSchema, { checklist: makeChecklist() });
        });
      document.body.appendChild(el);
      await el.updateComplete;
      await el.updateComplete;

      const checkbox = el.shadowRoot!.querySelector('input[type="checkbox"]') as HTMLInputElement;
      checkbox.click();
      await el.updateComplete;
    });

    it('should be in saving state during the mutation', () => {
      expect(savingDuringMutation).to.be.true;
    });
  });

  describe('when persist fails via checkbox toggle', () => {
    let originalItems: ChecklistItem[];

    beforeEach(async () => {
      sinon.restore();
      el.remove();

      el = buildElement();
      stubListItems(el, [
        makeChecklistItem({ uid: 'uid-milk', text: 'Milk' }),
      ]);
      sinon
        .stub(el.client, 'toggleItem')
        .rejects(new Error('Save failed'));
      document.body.appendChild(el);
      await el.updateComplete;
      await el.updateComplete;
      originalItems = el.items;

      const checkbox = el.shadowRoot!.querySelector('input[type="checkbox"]') as HTMLInputElement;
      checkbox.click();
      await waitUntil(() => el.error !== null, 'error should be set', { timeout: 2000 });
      await el.updateComplete;
    });

    it('should set error to an AugmentedError', () => {
      expect(el.error).to.be.instanceOf(AugmentedError);
    });

    it('should describe the failed goal as toggling item', () => {
      expect(el.error?.failedGoalDescription).to.equal('toggling item');
    });

    it('should revert items to the previous state', () => {
      expect(el.items).to.deep.equal(originalItems);
    });

    it('should clear saving state', () => {
      expect(el.saving).to.be.false;
    });
  });

  describe('when a mutation fails with FailedPrecondition', () => {
    let toggleItemStub: SinonStub;
    let listItemsStub: SinonStub;

    beforeEach(async () => {
      sinon.restore();
      el.remove();
      el = buildElement();

      // First listItems response — initial fetch.
      const initialChecklist = makeChecklist({
        name: 'grocery_list',
        updatedAtMs: 1000,
        items: [
          makeChecklistItem({
            uid: 'uid-milk',
            text: 'Milk',
            updatedAtMs: 1000,
          }),
        ],
      });
      // Second listItems response — refetch after OCC mismatch (newer updated_at).
      const refreshedChecklist = makeChecklist({
        name: 'grocery_list',
        updatedAtMs: 2000,
        items: [
          makeChecklistItem({
            uid: 'uid-milk',
            text: 'Milk',
            updatedAtMs: 2000,
          }),
        ],
      });
      listItemsStub = sinon.stub(el.client, 'listItems');
      listItemsStub.onFirstCall().resolves(
        create(ListItemsResponseSchema, { checklist: initialChecklist })
      );
      listItemsStub.resolves(
        create(ListItemsResponseSchema, { checklist: refreshedChecklist })
      );

      // First call rejects with FailedPrecondition; retry succeeds.
      toggleItemStub = sinon.stub(el.client, 'toggleItem');
      toggleItemStub.onFirstCall().rejects(
        new ConnectError('expected_updated_at mismatch', Code.FailedPrecondition)
      );
      toggleItemStub.resolves(
        create(ToggleItemResponseSchema, { checklist: refreshedChecklist })
      );

      document.body.appendChild(el);
      await el.updateComplete;
      await el.updateComplete;

      const checkbox = el.shadowRoot!.querySelector('input[type="checkbox"]') as HTMLInputElement;
      checkbox.click();
      await waitUntil(
        () => toggleItemStub.callCount === 2,
        'toggleItem should be retried',
        { timeout: 2000 }
      );
      await el.updateComplete;
    });

    it('should refetch the checklist before retrying', () => {
      // listItemsStub is called once for initial load + once for OCC refetch
      expect(listItemsStub.callCount).to.be.at.least(2);
    });

    it('should retry the toggleItem call exactly once', () => {
      expect(toggleItemStub.callCount).to.equal(2);
    });

    it('should not surface an error on successful retry', () => {
      expect(el.error).to.be.null;
    });

    it('should show the OCC retry toast briefly', () => {
      const toast = el.shadowRoot?.querySelector('.occ-retry-toast');
      expect(toast).to.exist;
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
      let listItemsStub: SinonStub;
      let freshEl: WikiChecklist;

      beforeEach(() => {
        freshEl = buildElement('test-page', 'test_list');
        listItemsStub = stubListItems(freshEl);
        document.body.appendChild(freshEl);
        clock.tick(10001);
      });

      afterEach(() => {
        freshEl.remove();
      });

      it('should call listItems at regular intervals', () => {
        expect(listItemsStub.callCount).to.be.greaterThan(0);
      });
    });

    describe('when element is disconnected', () => {
      let listItemsStub: SinonStub;
      let countAfterDisconnect: number;

      beforeEach(() => {
        const freshEl = buildElement('test-page', 'test_list');
        listItemsStub = stubListItems(freshEl);
        document.body.appendChild(freshEl);
        freshEl.remove();
        countAfterDisconnect = listItemsStub.callCount;
        clock.tick(10000);
      });

      it('should stop polling after disconnect', () => {
        expect(listItemsStub.callCount).to.equal(countAfterDisconnect);
      });
    });

    describe('when external change arrives via poll (different updated_at)', () => {
      let pollingEl: WikiChecklist;

      beforeEach(async () => {
        pollingEl = buildElement('test-page', 'grocery_list');
        const stub = sinon.stub(pollingEl.client, 'listItems');
        stub.onFirstCall().resolves(
          create(ListItemsResponseSchema, {
            checklist: makeChecklist({
              name: 'grocery_list',
              updatedAtMs: 1000,
              items: [makeChecklistItem({ uid: 'uid-milk', text: 'Milk', updatedAtMs: 1000 })],
            }),
          })
        );
        stub.resolves(
          create(ListItemsResponseSchema, {
            checklist: makeChecklist({
              name: 'grocery_list',
              updatedAtMs: 2000,
              items: [
                makeChecklistItem({ uid: 'uid-milk', text: 'Milk', checked: true, updatedAtMs: 2000 }),
                makeChecklistItem({ uid: 'uid-eggs', text: 'Eggs', updatedAtMs: 2000 }),
              ],
            }),
          })
        );

        document.body.appendChild(pollingEl);
        await pollingEl.updateComplete;
        await pollingEl.updateComplete;

        await clock.tickAsync(10001);
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

    describe('when poll returns the same updated_at (short-circuit)', () => {
      let pollingEl: WikiChecklist;
      let listItemsStub: SinonStub;
      let preItems: ChecklistItem[];

      beforeEach(async () => {
        pollingEl = buildElement('test-page', 'grocery_list');
        const sameChecklist = makeChecklist({
          name: 'grocery_list',
          updatedAtMs: 1000,
          items: [makeChecklistItem({ uid: 'uid-milk', text: 'Milk', updatedAtMs: 1000 })],
        });
        listItemsStub = sinon.stub(pollingEl.client, 'listItems').resolves(
          create(ListItemsResponseSchema, { checklist: sameChecklist })
        );

        document.body.appendChild(pollingEl);
        await pollingEl.updateComplete;
        await pollingEl.updateComplete;
        preItems = pollingEl.items;

        await clock.tickAsync(10001);
        await pollingEl.updateComplete;
      });

      afterEach(() => {
        pollingEl.remove();
      });

      it('should still call listItems', () => {
        expect(listItemsStub.callCount).to.be.greaterThan(1);
      });

      it('should not replace the items array reference (short-circuit)', () => {
        expect(pollingEl.items).to.equal(preItems);
      });
    });

    describe('when tab is hidden during polling interval', () => {
      let listItemsStub: SinonStub;
      let freshEl: WikiChecklist;
      let callCountWhenHidden: number;

      beforeEach(() => {
        sinon.stub(document, 'hidden').get(() => true);

        freshEl = buildElement('test-page', 'test_list');
        listItemsStub = stubListItems(freshEl);
        document.body.appendChild(freshEl);
        callCountWhenHidden = listItemsStub.callCount;
        clock.tick(10001);
      });

      afterEach(() => {
        freshEl.remove();
      });

      it('should not call listItems during the polling interval', () => {
        expect(listItemsStub.callCount).to.equal(callCountWhenHidden);
      });
    });

    describe('when tab becomes visible after being hidden', () => {
      let listItemsStub: SinonStub;
      let freshEl: WikiChecklist;
      let callCountBeforeVisible: number;

      beforeEach(async () => {
        let isHidden = true;
        sinon.stub(document, 'hidden').get(() => isHidden);

        freshEl = buildElement('test-page', 'test_list');
        listItemsStub = stubListItems(freshEl);
        document.body.appendChild(freshEl);

        await clock.tickAsync(10001);
        callCountBeforeVisible = listItemsStub.callCount;

        isHidden = false;
        document.dispatchEvent(new Event('visibilitychange'));
        await freshEl.updateComplete;
      });

      afterEach(() => {
        freshEl.remove();
      });

      it('should immediately call listItems when tab becomes visible', () => {
        expect(listItemsStub.callCount).to.be.greaterThan(callCountBeforeVisible);
      });
    });

    describe('when a save is in progress during polling interval', () => {
      let listItemsStub: SinonStub;
      let freshEl: WikiChecklist;
      let callCountBeforeSave: number;

      beforeEach(async () => {
        freshEl = buildElement('test-page', 'test_list');
        listItemsStub = stubListItems(freshEl);
        document.body.appendChild(freshEl);
        await freshEl.updateComplete;

        callCountBeforeSave = listItemsStub.callCount;

        freshEl.saving = true;
        clock.tick(10001);
      });

      afterEach(() => {
        freshEl.remove();
      });

      it('should not call listItems while saving', () => {
        expect(listItemsStub.callCount).to.equal(callCountBeforeSave);
      });
    });
  });

  describe('when adding an item', () => {
    let addItemStub: SinonStub;

    beforeEach(async () => {
      sinon.restore();
      el.remove();
      el = buildElement();

      stubListItems(el, [makeChecklistItem({ uid: 'uid-milk', text: 'Milk' })]);
      addItemStub = sinon
        .stub(el.client, 'addItem')
        .resolves(create(AddItemResponseSchema, {
          item: makeChecklistItem({ uid: 'uid-bread', text: 'Bread' }),
          checklist: makeChecklist({
            items: [
              makeChecklistItem({ uid: 'uid-milk', text: 'Milk' }),
              makeChecklistItem({ uid: 'uid-bread', text: 'Bread' }),
            ],
          }),
        }));

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
        () => addItemStub.callCount > 0,
        'addItem should be called',
        { timeout: 2000 }
      );
      await el.updateComplete;
    });

    it('should send the new item text', () => {
      const args = addItemStub.getCall(0).args[0] as { text: string };
      expect(args.text).to.equal('Bread');
    });

    it('should clear the add input after adding', () => {
      const addInputAfter =
        el.shadowRoot?.querySelector<HTMLInputElement>('.add-text-input');
      expect(addInputAfter?.value).to.equal('');
    });
  });

  describe('when adding an item with a tag', () => {
    let addItemStub: SinonStub;

    beforeEach(async () => {
      sinon.restore();
      el.remove();
      el = buildElement();

      stubListItems(el, []);
      addItemStub = sinon
        .stub(el.client, 'addItem')
        .resolves(create(AddItemResponseSchema, {
          item: makeChecklistItem({ uid: 'uid-milk', text: 'Milk', tags: ['dairy'] }),
          checklist: makeChecklist({
            items: [makeChecklistItem({ uid: 'uid-milk', text: 'Milk', tags: ['dairy'] })],
          }),
        }));

      document.body.appendChild(el);
      await el.updateComplete;
      await el.updateComplete;

      const addInput =
        el.shadowRoot?.querySelector<HTMLInputElement>('.add-text-input');
      if (addInput) {
        addInput.value = 'Milk #dairy';
        addInput.dispatchEvent(new InputEvent('input', { bubbles: true }));
      }
      await el.updateComplete;

      const addBtn =
        el.shadowRoot?.querySelector<HTMLButtonElement>('.add-btn');
      addBtn?.click();
      await waitUntil(
        () => addItemStub.callCount > 0,
        'addItem should be called',
        { timeout: 2000 }
      );
      await el.updateComplete;
    });

    it('should include the tags array in the AddItem request', () => {
      const args = addItemStub.getCall(0).args[0] as { tags: string[] };
      expect(args.tags).to.deep.equal(['dairy']);
    });
  });

  describe('when removing an item', () => {
    let deleteItemStub: SinonStub;

    beforeEach(async () => {
      sinon.restore();
      el.remove();
      el = buildElement();

      stubListItems(el, [
        makeChecklistItem({ uid: 'uid-milk', text: 'Milk' }),
        makeChecklistItem({ uid: 'uid-eggs', text: 'Eggs' }),
      ]);
      deleteItemStub = sinon
        .stub(el.client, 'deleteItem')
        .resolves(create(DeleteItemResponseSchema, {
          checklist: makeChecklist({
            items: [makeChecklistItem({ uid: 'uid-eggs', text: 'Eggs' })],
          }),
        }));

      document.body.appendChild(el);
      await el.updateComplete;
      await el.updateComplete;

      const removeBtn =
        el.shadowRoot?.querySelector<HTMLButtonElement>('.remove-btn');
      removeBtn?.click();
      await waitUntil(
        () => deleteItemStub.callCount > 0,
        'deleteItem should be called',
        { timeout: 2000 }
      );
      await el.updateComplete;
    });

    it('should call deleteItem with the targeted uid', () => {
      const args = deleteItemStub.getCall(0).args[0] as { uid: string };
      expect(args.uid).to.equal('uid-milk');
    });
  });

  describe('when editing item text', () => {
    let updateItemStub: SinonStub;

    beforeEach(async () => {
      sinon.restore();
      el.remove();
      el = buildElement();

      stubListItems(el, [makeChecklistItem({ uid: 'uid-milk', text: 'Milk' })]);
      updateItemStub = sinon
        .stub(el.client, 'updateItem')
        .resolves(create(UpdateItemResponseSchema, {
          checklist: makeChecklist({
            items: [makeChecklistItem({ uid: 'uid-milk', text: 'Whole Milk' })],
          }),
        }));

      document.body.appendChild(el);
      await el.updateComplete;
      await el.updateComplete;

      const displaySpan = el.shadowRoot?.querySelector('.item-display-text');
      (displaySpan as HTMLElement)?.click();
      await el.updateComplete;

      const textInput =
        el.shadowRoot?.querySelector<HTMLInputElement>('.item-text');
      if (textInput) {
        textInput.value = 'Whole Milk';
        textInput.dispatchEvent(new InputEvent('input', { bubbles: true }));
        textInput.dispatchEvent(new FocusEvent('blur', { bubbles: true }));
      }
      await waitUntil(
        () => updateItemStub.callCount > 0,
        'updateItem should be called',
        { timeout: 2000 }
      );
      await el.updateComplete;
    });

    it('should call updateItem with the updated text on blur', () => {
      const args = updateItemStub.getCall(0).args[0] as { text: string };
      expect(args.text).to.equal('Whole Milk');
    });

    it('should pass the targeted uid', () => {
      const args = updateItemStub.getCall(0).args[0] as { uid: string };
      expect(args.uid).to.equal('uid-milk');
    });
  });

  describe('drag-and-drop reordering', () => {
    describe('rendering with drag support', () => {
      let handles: NodeListOf<Element> | undefined;
      let rows: NodeListOf<Element> | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
          makeChecklistItem({ text: 'Bread', tags: ['bakery'] }),
          makeChecklistItem({ text: 'Eggs', tags: ['dairy'] }),
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

      it('should not render item rows as statically draggable', () => {
        expect(rows?.[0]?.getAttribute('draggable')).to.not.equal('true');
      });

      it('should render drag handles with tabindex="0" for keyboard access', () => {
        expect(handles?.[0]?.getAttribute('tabindex')).to.equal('0');
      });

      it('should render drag handles with role="button"', () => {
        expect(handles?.[0]?.getAttribute('role')).to.equal('button');
      });

      it('should render drag handles with an aria-label', () => {
        expect(handles?.[0]?.getAttribute('aria-label')).to.exist;
        expect(handles?.[0]?.getAttribute('aria-label')).to.not.be.empty;
      });

      it('should NOT have aria-hidden on drag handles', () => {
        expect(handles?.[0]?.getAttribute('aria-hidden')).to.be.null;
      });
    });

    describe('keyboard reorder', () => {
      let internal: WikiChecklistInternal;
      let reorderItemStub: SinonStub;

      beforeEach(async () => {
        internal = el as unknown as WikiChecklistInternal;

        el.error = null;
        el.loading = false;
        el.items = [
          makeChecklistItem({ uid: 'a', text: 'Item A', sortOrder: 1000n }),
          makeChecklistItem({ uid: 'b', text: 'Item B', sortOrder: 2000n }),
          makeChecklistItem({ uid: 'c', text: 'Item C', sortOrder: 3000n }),
        ];

        // The stub echoes the current optimistic state back, mirroring
        // what the real server would do (return the now-mutated list).
        reorderItemStub = sinon
          .stub(el.client, 'reorderItem')
          .callsFake(async () =>
            create(ReorderItemResponseSchema, {
              checklist: makeChecklist({ items: el.items }),
            })
          );

        await el.updateComplete;
      });

      describe('when ArrowUp is pressed on a drag handle (not the first item)', () => {
        beforeEach(async () => {
          const event = new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true, cancelable: true });
          internal._handleDragHandleKeydown(event, 1);
          await waitUntil(
            () => reorderItemStub.callCount > 0,
            'reorderItem should be called',
            { timeout: 2000 }
          );
          await el.updateComplete;
        });

        it('should move the item up by one position', () => {
          expect(el.items[0]?.text).to.equal('Item B');
          expect(el.items[1]?.text).to.equal('Item A');
          expect(el.items[2]?.text).to.equal('Item C');
        });

        it('should call reorderItem with the moved uid', () => {
          const args = reorderItemStub.getCall(0).args[0] as { uid: string };
          expect(args.uid).to.equal('b');
        });
      });

      describe('when ArrowDown is pressed on a drag handle (not the last item)', () => {
        beforeEach(async () => {
          const event = new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true });
          internal._handleDragHandleKeydown(event, 1);
          await waitUntil(
            () => reorderItemStub.callCount > 0,
            'reorderItem should be called',
            { timeout: 2000 }
          );
          await el.updateComplete;
        });

        it('should move the item down by one position', () => {
          expect(el.items[0]?.text).to.equal('Item A');
          expect(el.items[1]?.text).to.equal('Item C');
          expect(el.items[2]?.text).to.equal('Item B');
        });
      });

      describe('when ArrowUp is pressed on the first item', () => {
        let originalItems: ChecklistItem[];

        beforeEach(async () => {
          originalItems = [...el.items];
          const event = new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true, cancelable: true });
          internal._handleDragHandleKeydown(event, 0);
          await el.updateComplete;
        });

        it('should not reorder items', () => {
          expect(el.items.map(i => i.text)).to.deep.equal(originalItems.map(i => i.text));
        });

        it('should not call reorderItem', () => {
          expect(reorderItemStub.callCount).to.equal(0);
        });
      });

      describe('when ArrowDown is pressed on the last item', () => {
        let originalItems: ChecklistItem[];

        beforeEach(async () => {
          originalItems = [...el.items];
          const event = new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true });
          internal._handleDragHandleKeydown(event, 2);
          await el.updateComplete;
        });

        it('should not reorder items', () => {
          expect(el.items.map(i => i.text)).to.deep.equal(originalItems.map(i => i.text));
        });
      });

      describe('when other keys are pressed on a drag handle', () => {
        let originalItems: ChecklistItem[];

        beforeEach(async () => {
          originalItems = [...el.items];
          const event = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true });
          internal._handleDragHandleKeydown(event, 1);
          await el.updateComplete;
        });

        it('should not reorder items', () => {
          expect(el.items.map(i => i.text)).to.deep.equal(originalItems.map(i => i.text));
        });
      });
    });

    describe('when mousedown on drag handle', () => {
      let row: Element | null | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
          makeChecklistItem({ text: 'Bread', tags: ['bakery'] }),
        ];
        await el.updateComplete;
        row = el.shadowRoot?.querySelector('.item-row');
        const handle = el.shadowRoot?.querySelector('.drag-handle');
        handle?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      });

      it('should set draggable on the parent row', () => {
        expect((row as HTMLElement)?.draggable).to.be.true;
      });
    });

    describe('when dragend fires after a drag', () => {
      let row: HTMLElement | null | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
          makeChecklistItem({ text: 'Bread', tags: ['bakery'] }),
        ];
        await el.updateComplete;
        row = el.shadowRoot?.querySelector<HTMLElement>('.item-row');
        row!.draggable = true;
        const dragEndEvent = new DragEvent('dragend', { bubbles: true });
        Object.defineProperty(dragEndEvent, 'currentTarget', { value: row });
        (el as unknown as WikiChecklistInternal)._handleItemDragEnd(dragEndEvent);
      });

      it('should clear draggable on the row', () => {
        expect(row?.draggable).to.be.false;
      });
    });

    describe('drop handler: flat view reordering', () => {
      let reorderItemStub: SinonStub;

      beforeEach(async () => {
        sinon.restore();
        sinon.stub(el.client, 'listItems').resolves(
          create(ListItemsResponseSchema, { checklist: makeChecklist() })
        );
        el.items = [
          makeChecklistItem({ uid: 'a', text: 'Milk', sortOrder: 1000n }),
          makeChecklistItem({ uid: 'b', text: 'Bread', sortOrder: 2000n }),
          makeChecklistItem({ uid: 'c', text: 'Eggs', sortOrder: 3000n }),
        ];
        reorderItemStub = sinon
          .stub(el.client, 'reorderItem')
          .callsFake(async () =>
            create(ReorderItemResponseSchema, {
              checklist: makeChecklist({ items: el.items }),
            })
          );
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

        it('should call reorderItem to persist', () => {
          expect(reorderItemStub).to.have.been.calledOnce;
        });

        it('should pass the moved uid', () => {
          const args = reorderItemStub.getCall(0).args[0] as { uid: string };
          expect(args.uid).to.equal('c');
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

        it('should not call reorderItem', () => {
          expect(reorderItemStub).not.to.have.been.called;
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
    let reorderItemStub: SinonStub;

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
      sinon.stub(el.client, 'listItems').resolves(
        create(ListItemsResponseSchema, { checklist: makeChecklist() })
      );
      el.items = [
        makeChecklistItem({ uid: 'a', text: 'Milk', sortOrder: 1000n }),
        makeChecklistItem({ uid: 'b', text: 'Bread', sortOrder: 2000n }),
        makeChecklistItem({ uid: 'c', text: 'Eggs', sortOrder: 3000n }),
      ];
      reorderItemStub = sinon
        .stub(el.client, 'reorderItem')
        .callsFake(async () =>
          create(ReorderItemResponseSchema, {
            checklist: makeChecklist({ items: el.items }),
          })
        );
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
        const touch = makeTouch(100, 200);
        internal._startTouchDrag(0, touch);

        const row = el.shadowRoot?.querySelectorAll('.item-row')[2];
        if (row instanceof HTMLElement) {
          row.getBoundingClientRect = () =>
            ({ top: 280, bottom: 320, height: 40, left: 0, right: 200, width: 200 }) as DOMRect;
        }
        sinon.stub(el.shadowRoot!, 'elementFromPoint').returns(row ?? null);

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
        const touch = makeTouch(100, 200);
        internal._startTouchDrag(2, touch);

        internal._dragOverItemIndex = 0;
        internal._dragOverItemPosition = 'before';

        await internal._handleTouchEnd();

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

      it('should call reorderItem', () => {
        expect(reorderItemStub.called).to.be.true;
      });
    });

    describe('when touch is cancelled during active drag', () => {
      let originalItems: ChecklistItem[];

      beforeEach(() => {
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
      beforeEach(async () => {
        const touchEvent = makeTouchEvent('touchstart', 100, 200);
        internal._handleTouchStart(touchEvent, 1);

        await internal._handleTouchEnd();
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

  describe('metadata display', () => {
    describe('when an item has completed_by and completed_at', () => {
      let caption: Element | null | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          makeChecklistItem({
            text: 'Done task',
            checked: true,
            completedBy: 'alice@example.com',
            completedAtMs: Date.now() - 60_000,
          }),
        ];
        await el.updateComplete;
        caption = el.shadowRoot?.querySelector('[data-meta="completed"]');
      });

      it('should render the completion caption', () => {
        expect(caption).to.exist;
      });

      it('should include the completed_by name', () => {
        expect(caption?.textContent).to.contain('alice@example.com');
      });
    });

    describe('when an automated item is completed', () => {
      let caption: Element | null | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          makeChecklistItem({
            text: 'Sync task',
            checked: true,
            automated: true,
            completedBy: 'wiki-cli',
            completedAtMs: Date.now() - 60_000,
          }),
        ];
        await el.updateComplete;
        caption = el.shadowRoot?.querySelector('[data-meta="completed"]');
      });

      it('should attribute completion to "an agent"', () => {
        expect(caption?.textContent).to.contain('an agent');
      });

      it('should NOT show the agent’s raw completed_by', () => {
        expect(caption?.textContent).to.not.contain('wiki-cli');
      });
    });

    describe('when an unchecked item has a description', () => {
      let description: Element | null | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          makeChecklistItem({
            text: 'Buy oat milk',
            description: 'The brand Kirsten likes',
          }),
        ];
        await el.updateComplete;
        description = el.shadowRoot?.querySelector('.item-description');
      });

      it('should render the description', () => {
        expect(description?.textContent?.trim()).to.equal('The brand Kirsten likes');
      });
    });

    describe('when an item has no description', () => {
      let description: Element | null | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [makeChecklistItem({ text: 'Plain task' })];
        await el.updateComplete;
        description = el.shadowRoot?.querySelector('.item-description');
      });

      it('should not render an empty description element', () => {
        expect(description).to.not.exist;
      });
    });

    describe('when an item is past-due (overdue)', () => {
      let due: Element | null | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          makeChecklistItem({
            text: 'Renew passport',
            dueMs: Date.now() - 24 * 60 * 60 * 1000,
          }),
        ];
        await el.updateComplete;
        due = el.shadowRoot?.querySelector('[data-meta="due"]');
      });

      it('should render the due indicator', () => {
        expect(due).to.exist;
      });

      it('should mark it as overdue', () => {
        expect(due?.classList.contains('item-due-overdue')).to.be.true;
      });
    });

    describe('when an item is due in the future', () => {
      let due: Element | null | undefined;

      beforeEach(async () => {
        el.error = null;
        el.loading = false;
        el.items = [
          makeChecklistItem({
            text: 'Future task',
            dueMs: Date.now() + 60 * 60 * 1000,
          }),
        ];
        await el.updateComplete;
        due = el.shadowRoot?.querySelector('[data-meta="due"]');
      });

      it('should render the due indicator', () => {
        expect(due).to.exist;
      });

      it('should not mark it as overdue', () => {
        expect(due?.classList.contains('item-due-overdue')).to.be.false;
      });
    });

    describe('when localStorage debug flag is enabled', () => {
      let listBadge: Element | null | undefined;
      let itemBadge: Element | null | undefined;

      beforeEach(async () => {
        try { globalThis.localStorage.setItem('wiki-checklist-debug', '1'); } catch { /* noop */ }
        el.error = null;
        el.loading = false;
        el._syncToken = 42n;
        el._listUpdatedAt = timestampFromMs(1700_000_000_000);
        el.items = [
          makeChecklistItem({
            uid: 'uid-debug',
            text: 'Item with debug',
            sortOrder: 1500n,
            updatedAtMs: 1700_000_001_000,
          }),
        ];
        el.requestUpdate();
        await el.updateComplete;
        listBadge = el.shadowRoot?.querySelector('[data-debug="list"]');
        itemBadge = el.shadowRoot?.querySelector('.item-debug-badge');
      });

      afterEach(() => {
        try { globalThis.localStorage.removeItem('wiki-checklist-debug'); } catch { /* noop */ }
      });

      it('should render the list-level debug badge', () => {
        expect(listBadge).to.exist;
      });

      it('should include the sync_token in the list badge', () => {
        expect(listBadge?.textContent).to.contain('sync:42');
      });

      it('should render the item-level debug badge', () => {
        expect(itemBadge).to.exist;
      });

      it('should include the item uid in the item badge', () => {
        expect(itemBadge?.textContent).to.contain('uid:uid-debu');
      });
    });

    describe('when localStorage debug flag is NOT enabled', () => {
      let listBadge: Element | null | undefined;

      beforeEach(async () => {
        try { globalThis.localStorage.removeItem('wiki-checklist-debug'); } catch { /* noop */ }
        el.error = null;
        el.loading = false;
        el.items = [makeChecklistItem({ text: 'Plain' })];
        el.requestUpdate();
        await el.updateComplete;
        listBadge = el.shadowRoot?.querySelector('[data-debug="list"]');
      });

      it('should NOT render the debug badge', () => {
        expect(listBadge).to.not.exist;
      });
    });
  });
});
