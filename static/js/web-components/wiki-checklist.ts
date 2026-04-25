import { html, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { ConnectError, Code } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  ChecklistService,
  ListItemsRequestSchema,
  AddItemRequestSchema,
  UpdateItemRequestSchema,
  ToggleItemRequestSchema,
  DeleteItemRequestSchema,
  ReorderItemRequestSchema,
} from '../gen/api/v1/checklist_pb.js';
import type {
  Checklist,
  ChecklistItem,
} from '../gen/api/v1/checklist_pb.js';
import type { Timestamp } from '@bufbuild/protobuf/wkt';
import {
  foundationCSS,
  buttonCSS,
  inputCSS,
  pillCSS,
  sharedStyles,
  zIndexCSS,
} from './shared-styles.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import './error-display.js';
import { parseTaggedInput, composeTaggedText } from './checklist-tag-parser.js';
import {
  reorderItems,
  computeSortOrder,
  ChecklistDragManager,
} from './checklist-drag-manager.js';
import type { DragReorderHandler } from './checklist-drag-manager.js';
import { wikiChecklistStyles } from './wiki-checklist-styles.js';

export type { ChecklistItem, Checklist } from '../gen/api/v1/checklist_pb.js';

// Polling interval in milliseconds
const POLL_INTERVAL_MS = 10000;
// OCC retry happens once on FailedPrecondition; the retry uses the
// updated_at returned from the refetch.
const OCC_TOAST_DURATION_MS = 2000;

/**
 * WikiChecklist - A fully API-driven interactive checklist component.
 *
 * Mutations go through ChecklistService, which gives us:
 *   - server-stamped attribution (completed_by, automated)
 *   - per-item created_at/updated_at
 *   - optimistic concurrency via expected_updated_at
 *   - per-list sync_token bookkeeping for CalDAV
 *
 * Polls ListItems for fresh data; short-circuits the render when the
 * checklist's updated_at hasn't moved.
 *
 * @property {string} listName - Checklist name (attribute: list-name)
 * @property {string} page - Page identifier for gRPC calls
 *
 * @example
 * <wiki-checklist list-name="grocery_list" page="my-page"></wiki-checklist>
 */
export class WikiChecklist extends LitElement implements DragReorderHandler {
  static override readonly styles = [
    foundationCSS,
    buttonCSS,
    inputCSS,
    pillCSS,
    zIndexCSS,
    wikiChecklistStyles,
  ];

  @property({ type: String, attribute: 'list-name' })
  declare listName: string;

  @property({ type: String })
  declare page: string;

  @state()
  declare items: ChecklistItem[];

  @state()
  declare loading: boolean;

  @state()
  declare saving: boolean;

  @state()
  declare error: AugmentedError | null;

  @state()
  declare filterTags: string[];

  // Per-list metadata: updated_at + sync_token, used as OCC token.
  @state()
  declare _listUpdatedAt: Timestamp | undefined;

  @state()
  declare _syncToken: bigint;

  // OCC retry toast — surfaced briefly when a mutation lost a race.
  @state()
  declare _occRetryToastVisible: boolean;

  // Index of the item currently being edited (text input focused)
  @state()
  private declare editingIndex: number | null;

  // Value of the new item text input
  @state()
  private declare newItemText: string;

  // Touch drag active state - needs to be @state so renders update
  @state()
  declare _touchDragActive: boolean;

  private _boundHandleVisibilityChange: (() => void) | null = null;
  private _occToastTimer: ReturnType<typeof setTimeout> | null = null;

  private pollingTimer: ReturnType<typeof setInterval> | null = null;

  readonly client = createClient(ChecklistService, getGrpcWebTransport());

  private readonly _dragManager: ChecklistDragManager;

  constructor() {
    super();
    this.listName = '';
    this.page = '';
    this.items = [];
    this.loading = false;
    this.saving = false;
    this.error = null;
    this.filterTags = [];
    this._listUpdatedAt = undefined;
    this._syncToken = 0n;
    this._occRetryToastVisible = false;
    this.editingIndex = null;
    this.newItemText = '';
    this._touchDragActive = false;

    this._dragManager = new ChecklistDragManager(this);
  }

  // DragReorderHandler implementation
  onDragStateChanged(): void {
    this._touchDragActive = this._dragManager.touchDragActive;
    this.requestUpdate();
  }

  async onReorder(fromIndex: number, toInsertIndex: number): Promise<void> {
    const moved = this.items[fromIndex];
    if (!moved) return;
    const newSortOrder = computeSortOrder(
      this.items.map(i => ({ uid: i.uid, sortOrder: i.sortOrder })),
      toInsertIndex,
      moved.uid
    );

    // Optimistic local update
    const previousItems = this.items;
    this.items = reorderItems(this.items, fromIndex, toInsertIndex);

    await this._callMutation(
      async expectedUpdatedAt => this.client.reorderItem(create(ReorderItemRequestSchema, {
        page: this.page,
        listName: this.listName,
        uid: moved.uid,
        newSortOrder,
        expectedUpdatedAt,
      })),
      previousItems,
      'reordering item'
    );
  }

  onError(err: unknown): void {
    this.error = AugmentErrorService.augmentError(err, 'touch reorder');
  }

  getShadowRoot(): ShadowRoot | null {
    return this.shadowRoot;
  }

  getHostElement(): HTMLElement {
    return this;
  }

  // Expose drag state properties for test access (tests use WikiChecklistInternal interface)
  get _dragSourceItemIndex(): number | null {
    return this._dragManager.dragSourceItemIndex;
  }

  set _dragSourceItemIndex(value: number | null) {
    this._dragManager.dragSourceItemIndex = value;
  }

  get _dragOverItemIndex(): number | null {
    return this._dragManager.dragOverItemIndex;
  }

  set _dragOverItemIndex(value: number | null) {
    this._dragManager.dragOverItemIndex = value;
  }

  get _dragOverItemPosition(): 'before' | 'after' {
    return this._dragManager.dragOverItemPosition;
  }

  set _dragOverItemPosition(value: 'before' | 'after') {
    this._dragManager.dragOverItemPosition = value;
  }

  get _longPressHandleIndex(): number | null {
    return this._dragManager.longPressHandleIndex;
  }

  // Touch drag internals exposed for tests
  get _longPressTimerId(): ReturnType<typeof setTimeout> | null {
    return this._dragManager.longPressTimerId;
  }

  get _touchStartX(): number {
    return this._dragManager.touchStartX;
  }

  get _touchStartY(): number {
    return this._dragManager.touchStartY;
  }

  get _touchGhostEl(): HTMLElement | null {
    return this._dragManager.touchGhostEl;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    // Remove server-rendered fallback content now that JS has taken over.
    this.innerHTML = '';
    if (this.page) {
      this.loading = true;
      void this.fetchData();
    }
    this._boundHandleVisibilityChange = () => { this._handleVisibilityChange(); };
    document.addEventListener('visibilitychange', this._boundHandleVisibilityChange);
    this.pollingTimer = setInterval(() => {
      if (this.page && !document.hidden && !this.saving) {
        void this.fetchData();
      }
    }, POLL_INTERVAL_MS);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this.pollingTimer !== null) {
      clearInterval(this.pollingTimer);
      this.pollingTimer = null;
    }
    if (this._boundHandleVisibilityChange !== null) {
      document.removeEventListener('visibilitychange', this._boundHandleVisibilityChange);
      this._boundHandleVisibilityChange = null;
    }
    if (this._occToastTimer !== null) {
      clearTimeout(this._occToastTimer);
      this._occToastTimer = null;
    }
    this._dragManager.cleanup();
  }

  private _handleVisibilityChange(): void {
    if (!document.hidden && this.page && !this.saving) {
      void this.fetchData();
    }
  }

  /**
   * Format a listName (snake_case or kebab-case) into a display title.
   * e.g. "grocery_list" -> "Grocery List", "my-checklist" -> "My Checklist"
   */
  formatTitle(listName: string): string {
    if (!listName) return '';
    return listName
      .replaceAll(/[_-]/g, ' ')
      .replaceAll(/\b\w/g, c => c.toUpperCase());
  }

  /**
   * Return sorted unique tags from current items.
   */
  getExistingTags(): string[] {
    const tagSet = new Set<string>();
    for (const item of this.items) {
      for (const tag of item.tags) {
        if (tag) tagSet.add(tag);
      }
    }
    return Array.from(tagSet).sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' }) || a.localeCompare(b));
  }

  /**
   * Return items filtered by the active filterTags.
   * When filterTags is empty, returns all items.
   * When filterTags has entries, returns only items whose tags contain ALL filter tags (AND logic).
   */
  getFilteredItems(): Array<{ item: ChecklistItem; index: number }> {
    const result: Array<{ item: ChecklistItem; index: number }> = [];
    for (let i = 0; i < this.items.length; i++) {
      const item = this.items[i];
      if (!item) continue;
      if (this.filterTags.every(ft => item.tags.includes(ft))) {
        result.push({ item, index: i });
      }
    }
    return result;
  }

  /**
   * Fetch checklist data via ListItems.
   *
   * Short-circuits the render when the checklist's updated_at value is
   * unchanged, avoiding unnecessary work and preserving cursor/edit state.
   */
  private async fetchData(): Promise<void> {
    if (!this.page) {
      throw new Error('wiki-checklist: page attribute is required but not set');
    }

    try {
      const request = create(ListItemsRequestSchema, {
        page: this.page,
        listName: this.listName,
      });
      const response = await this.client.listItems(request);
      const checklist = response.checklist;
      if (!checklist) {
        this.items = [];
        this._listUpdatedAt = undefined;
        this._syncToken = 0n;
      } else if (timestampsEqual(checklist.updatedAt, this._listUpdatedAt)) {
        // Server data unchanged — skip the re-render entirely. This is
        // the polling short-circuit; it preserves edit-mode state and
        // cursor position.
        this.error = null;
        this.loading = false;
        return;
      } else {
        this.items = [...checklist.items];
        this._listUpdatedAt = checklist.updatedAt;
        this._syncToken = checklist.syncToken;
      }
      this.error = null;
    } catch (err) {
      this.error = AugmentErrorService.augmentError(err, 'loading checklist');
    } finally {
      this.loading = false;
    }
  }

  /**
   * Run a mutation with optimistic local update + OCC retry.
   *
   * The mutation closure receives the current expected_updated_at (which
   * may be undefined if the list has never been read). On
   * FailedPrecondition the component refetches, surfaces a brief toast,
   * and retries the mutation once. Any other error reverts the local
   * optimistic state and surfaces the error.
   */
  private async _callMutation(
    runMutation: (expectedUpdatedAt: Timestamp | undefined) => Promise<MutationResponse>,
    previousItems: ChecklistItem[],
    failedGoalDescription: string
  ): Promise<void> {
    try {
      this.saving = true;
      this.error = null;

      let response: MutationResponse;
      try {
        response = await runMutation(this._listUpdatedAt);
      } catch (err) {
        if (err instanceof ConnectError && err.code === Code.FailedPrecondition) {
          // OCC mismatch — refetch and retry once.
          await this._refetchForOccRetry();
          this._showOccRetryToast();
          response = await runMutation(this._listUpdatedAt);
        } else {
          throw err;
        }
      }

      if (response.checklist) {
        this.items = [...response.checklist.items];
        this._listUpdatedAt = response.checklist.updatedAt;
        this._syncToken = response.checklist.syncToken;
      }
    } catch (err) {
      // Revert the optimistic state on non-OCC failure.
      this.items = previousItems;
      this.error = AugmentErrorService.augmentError(err, failedGoalDescription);
    } finally {
      this.saving = false;
    }
  }

  private async _refetchForOccRetry(): Promise<void> {
    const request = create(ListItemsRequestSchema, {
      page: this.page,
      listName: this.listName,
    });
    const response = await this.client.listItems(request);
    if (response.checklist) {
      this._listUpdatedAt = response.checklist.updatedAt;
      this._syncToken = response.checklist.syncToken;
    }
  }

  private _showOccRetryToast(): void {
    this._occRetryToastVisible = true;
    if (this._occToastTimer !== null) {
      clearTimeout(this._occToastTimer);
    }
    this._occToastTimer = setTimeout(() => {
      this._occRetryToastVisible = false;
      this._occToastTimer = null;
    }, OCC_TOAST_DURATION_MS);
  }

  private async _handleToggleItem(index: number): Promise<void> {
    const item = this.items[index];
    if (!item) return;
    const previousItems = this.items;

    // Optimistic local update
    this.items = this.items.map((it, i) =>
      i === index ? { ...it, checked: !it.checked } : it
    );

    await this._callMutation(
      async expectedUpdatedAt => this.client.toggleItem(create(ToggleItemRequestSchema, {
        page: this.page,
        listName: this.listName,
        uid: item.uid,
        expectedUpdatedAt,
      })),
      previousItems,
      'toggling item'
    );
  }

  private async _handleRemoveItem(index: number): Promise<void> {
    const item = this.items[index];
    if (!item) return;
    const previousItems = this.items;

    // Optimistic local update
    this.items = this.items.filter((_, i) => i !== index);

    await this._callMutation(
      async expectedUpdatedAt => this.client.deleteItem(create(DeleteItemRequestSchema, {
        page: this.page,
        listName: this.listName,
        uid: item.uid,
        expectedUpdatedAt,
      })),
      previousItems,
      'deleting item'
    );
  }

  private async _enterEditMode(index: number): Promise<void> {
    this.editingIndex = index;
    await this.updateComplete;
    if (this.editingIndex !== index) {
      return;
    }
    const input = this.shadowRoot?.querySelector<HTMLInputElement>('.item-text');
    const item = this.items[index];
    if (input && item) {
      input.value = composeTaggedText(item);
      input.focus();
    }
  }

  private async _handleItemTextBlur(index: number, value: string): Promise<void> {
    this.editingIndex = null;
    const { tags, text } = parseTaggedInput(value);
    if (!text) return;
    const item = this.items[index];
    if (!item) return;
    const previousItems = this.items;

    // Optimistic local update
    this.items = this.items.map((it, i) =>
      i === index ? { ...it, text, tags } : it
    );

    await this._callMutation(
      async expectedUpdatedAt => this.client.updateItem(create(UpdateItemRequestSchema, {
        page: this.page,
        listName: this.listName,
        uid: item.uid,
        text,
        tags,
        expectedUpdatedAt,
      })),
      previousItems,
      'updating item'
    );
  }

  private _handleItemTextKeydown(
    index: number,
    value: string,
    event: KeyboardEvent
  ): void {
    if (event.key === 'Enter') {
      void this._handleItemTextBlur(index, value);
      if (event.target instanceof HTMLElement) {
        event.target.blur();
      }
      void this._returnFocusToDisplayText(index);
    }
  }

  private async _returnFocusToDisplayText(index: number): Promise<void> {
    await this.updateComplete;
    const row = this.shadowRoot?.querySelector<HTMLElement>(`.item-row[data-index="${index}"]`);
    row?.querySelector<HTMLElement>('.item-display-text')?.focus();
  }

  private async _handleAddItem(): Promise<void> {
    const { tags, text } = parseTaggedInput(this.newItemText);
    if (!text) return;
    const previousItems = this.items;
    this.newItemText = '';

    await this._callMutation(
      async expectedUpdatedAt => this.client.addItem(create(AddItemRequestSchema, {
        page: this.page,
        listName: this.listName,
        text,
        tags,
        expectedUpdatedAt,
      })),
      previousItems,
      'adding item'
    );
  }

  private _handleNewItemKeydown(event: KeyboardEvent): void {
    if (event.key === 'Enter') {
      void this._handleAddItem();
    }
  }

  private _handleFilterTagClick(tag: string): void {
    if (this.filterTags.includes(tag)) {
      this.filterTags = this.filterTags.filter(t => t !== tag);
    } else {
      this.filterTags = [...this.filterTags, tag];
    }
  }

  // Drag handler delegation methods (kept on component for template bindings and test access)
  _handleDragHandleMousedown(e: MouseEvent): void {
    this._dragManager.handleDragHandleMousedown(e);
  }

  _handleItemDragStart(e: DragEvent, index: number): void {
    this._dragManager.handleItemDragStart(e, index);
  }

  _handleItemDragOver(e: DragEvent, index: number): void {
    this._dragManager.handleItemDragOver(e, index);
  }

  _handleItemDragLeave(e: DragEvent): void {
    this._dragManager.handleItemDragLeave(e);
  }

  async _handleItemDrop(e: DragEvent, targetIndex: number): Promise<void> {
    await this._dragManager.handleItemDrop(e, targetIndex);
  }

  _handleItemDragEnd(e: DragEvent): void {
    this._dragManager.handleItemDragEnd(e);
  }

  _handleTouchStart(e: TouchEvent, index: number): void {
    this._dragManager.handleTouchStart(e, index);
  }

  _handleTouchMove(e: TouchEvent): void {
    this._dragManager.handleTouchMove(e);
  }

  async _handleTouchEnd(): Promise<void> {
    await this._dragManager.handleTouchEnd();
  }

  _handleTouchCancel(): void {
    this._dragManager.handleTouchCancel();
  }

  _startTouchDrag(index: number, touch: Touch): void {
    this._dragManager.startTouchDrag(index, touch);
  }

  _cancelLongPress(): void {
    this._dragManager.cancelLongPress();
  }

  _cleanupTouchDrag(): void {
    this._dragManager.cleanup();
  }

  _handleDragHandleKeydown(event: KeyboardEvent, index: number): void {
    if (event.key === 'ArrowUp') {
      event.preventDefault();
      if (index > 0) {
        void this._keyboardMoveItem(index, 'up');
      }
    } else if (event.key === 'ArrowDown') {
      event.preventDefault();
      if (index < this.items.length - 1) {
        void this._keyboardMoveItem(index, 'down');
      }
    }
  }

  private async _keyboardMoveItem(fromIndex: number, direction: 'up' | 'down'): Promise<void> {
    const toInsertIndex = direction === 'up' ? fromIndex - 1 : fromIndex + 2;
    const newIndex = direction === 'up' ? fromIndex - 1 : fromIndex + 1;

    await this.onReorder(fromIndex, toInsertIndex);

    await this.updateComplete;
    const handles = this.shadowRoot?.querySelectorAll<HTMLElement>('.drag-handle');
    const handle = handles?.[newIndex];
    handle?.focus();
  }

  private async _handleDeleteChecked(): Promise<void> {
    const checkedItems = this.items.filter(item => item.checked);
    for (const item of checkedItems) {
      const previousItems = this.items;
      this.items = this.items.filter(it => it.uid !== item.uid);
      // Sequential deletes — each one bumps updated_at, so we can't fan out.
      await this._callMutation(
        async expectedUpdatedAt => this.client.deleteItem(create(DeleteItemRequestSchema, {
          page: this.page,
          listName: this.listName,
          uid: item.uid,
          expectedUpdatedAt,
        })),
        previousItems,
        'deleting checked items'
      );
      // Bail if a delete failed — don't keep firing requests against
      // a list that's already in a bad state.
      if (this.error) return;
    }
  }

  // ---------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------

  private _renderItemEditInput(index: number) {
    return html`
      <input
        type="text"
        class="item-text"
        aria-label="Edit item text and tags"
        @blur="${(e: FocusEvent) => {
          if (!(e.target instanceof HTMLInputElement)) return;
          void this._handleItemTextBlur(index, e.target.value);
        }}"
        @keydown="${(e: KeyboardEvent) => {
          if (!(e.currentTarget instanceof HTMLInputElement)) return;
          this._handleItemTextKeydown(index, e.currentTarget.value, e);
        }}"
      />`;
  }

  private _renderItemDisplayText(item: ChecklistItem, _index: number) {
    return html`
      <span
        class="item-display-text"
        role="button"
        tabindex="0"
        @click="${() => void this._enterEditMode(_index)}"
        @keydown="${(e: KeyboardEvent) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            void this._enterEditMode(_index);
          }
        }}"
      >${item.text}</span>
      ${item.tags.map(
        tag => html`<span class="item-tag-badge">${tag}</span>`
      )}`;
  }

  private _renderCompletedCaption(item: ChecklistItem) {
    if (!item.checked) return nothing;
    if (!item.completedBy && !item.completedAt) return nothing;

    const who = item.automated
      ? 'an agent'
      : (item.completedBy ?? 'someone');
    const whenText = item.completedAt
      ? ` · ${formatRelativeTime(item.completedAt)}`
      : '';

    return html`
      <div class="item-meta-caption" data-meta="completed">
        Done by ${who}${whenText}
      </div>
    `;
  }

  private _renderDescription(item: ChecklistItem) {
    if (!item.description || !item.description.trim()) return nothing;
    return html`
      <div class="item-description">${item.description}</div>
    `;
  }

  private _renderDueIndicator(item: ChecklistItem) {
    if (!item.due) return nothing;
    const dueDate = timestampDate(item.due);
    const overdue = dueDate.getTime() < Date.now() && !item.checked;
    return html`
      <div class="item-due ${overdue ? 'item-due-overdue' : ''}" data-meta="due">
        <span aria-hidden="true">\u{1F4C5}</span>
        <span>${formatRelativeTime(item.due)}</span>
      </div>
    `;
  }

  private _renderDebugBadge(item: ChecklistItem) {
    if (!isDebugBadgeEnabled()) return nothing;
    const updated = item.updatedAt
      ? timestampDate(item.updatedAt).toISOString()
      : 'n/a';
    return html`
      <div class="item-debug-badge">
        <span>uid:${item.uid.slice(0, 8)}</span>
        <span>upd:${updated}</span>
        <span>so:${item.sortOrder.toString()}</span>
      </div>
    `;
  }

  private _renderItem(
    item: ChecklistItem,
    index: number
  ) {
    const isDragging = this._dragManager.dragSourceItemIndex === index;
    const isDragOver = this._dragManager.dragOverItemIndex === index;
    const dragOverClass = isDragOver
      ? `drag-over-${this._dragManager.dragOverItemPosition}`
      : '';

    return html`
      <li
        class="item-row ${item.checked ? 'item-checked' : ''} ${isDragging ? 'dragging' : ''} ${dragOverClass}"
        data-index="${index}"
        data-uid="${item.uid}"
        @dragstart="${(e: DragEvent) => this._handleItemDragStart(e, index)}"
        @dragover="${(e: DragEvent) =>
          this._handleItemDragOver(e, index)}"
        @dragleave="${(e: DragEvent) => this._handleItemDragLeave(e)}"
        @drop="${(e: DragEvent) => this._handleItemDrop(e, index)}"
        @dragend="${(e: DragEvent) => this._handleItemDragEnd(e)}"
      >
        <span
          class="drag-handle ${this._dragManager.longPressHandleIndex === index ? 'long-press-pending' : ''}"
          tabindex="0"
          role="button"
          aria-label="Reorder item. Use arrow keys to move up or down."
          @mousedown="${(e: MouseEvent) => this._handleDragHandleMousedown(e)}"
          @touchstart="${(e: TouchEvent) => this._handleTouchStart(e, index)}"
          @keydown="${(e: KeyboardEvent) => this._handleDragHandleKeydown(e, index)}"
        >⠇</span>
        <input
          type="checkbox"
          class="item-checkbox"
          .checked="${item.checked}"
          aria-label="${item.text}"
          ?disabled="${this.saving}"
          @change="${() => this._handleToggleItem(index)}"
        />
        <span class="item-content">
          ${this.editingIndex === index
            ? this._renderItemEditInput(index)
            : this._renderItemDisplayText(item, index)}
          ${this._renderDescription(item)}
          ${this._renderDueIndicator(item)}
          ${this._renderCompletedCaption(item)}
          ${this._renderDebugBadge(item)}
        </span>
        <button
          class="remove-btn"
          title="Remove item"
          aria-label="Remove item"
          ?disabled="${this.saving}"
          @click="${() => this._handleRemoveItem(index)}"
        >
          ✕
        </button>
      </li>
    `;
  }

  private _renderTagFilterBar() {
    const tags = this.getExistingTags();
    if (tags.length === 0) return nothing;

    return html`
      <div class="tag-filter-bar">
        ${tags.map(
          tag => html`
            <button
              class="tag-pill ${this.filterTags.includes(tag) ? 'tag-pill-active' : ''}"
              @click="${() => this._handleFilterTagClick(tag)}"
              aria-pressed="${this.filterTags.includes(tag)}"
              aria-label="Filter by ${tag}"
            >
              ${tag}
            </button>
          `
        )}
        ${this.filterTags.length > 0
          ? html`
              <button
                class="tag-filter-clear"
                @click="${() => { this.filterTags = []; }}"
                aria-label="Clear filter"
              >
                ✕
              </button>
            `
          : nothing}
      </div>
    `;
  }

  private _renderItems() {
    const filtered = this.getFilteredItems();
    return html`
      <ul class="items-list" role="list">
        ${filtered.map(({ item, index }) => this._renderItem(item, index))}
      </ul>
    `;
  }

  private _renderAddItem() {
    return html`
      <div class="add-item">
        <input
          type="text"
          class="add-text-input"
          .value="${this.newItemText}"
          placeholder="Add item… (use #tag for grouping)"
          aria-label="New item text, with optional #tag anywhere for grouping"
          ?disabled="${this.saving}"
          @input="${(e: InputEvent) => {
            if (!(e.target instanceof HTMLInputElement)) return;
            this.newItemText = e.target.value;
          }}"
          @keydown="${this._handleNewItemKeydown}"
        />
        <button
          class="add-btn button-base button-primary"
          ?disabled="${this.saving || !this.newItemText.trim()}"
          @click="${this._handleAddItem}"
          aria-label="Add item"
        >
          Add
        </button>
      </div>
    `;
  }

  private _renderListDebugBadge() {
    if (!isDebugBadgeEnabled()) return nothing;
    const updated = this._listUpdatedAt
      ? timestampDate(this._listUpdatedAt).toISOString()
      : 'n/a';
    return html`
      <div class="list-debug-badge" data-debug="list">
        sync:${this._syncToken.toString()} · upd:${updated}
      </div>
    `;
  }

  private _renderOccToast() {
    if (!this._occRetryToastVisible) return nothing;
    return html`
      <div class="occ-retry-toast" role="status" aria-live="polite">
        Edited concurrently — refreshed
      </div>
    `;
  }

  override render() {
    let checklistItemsContent;
    if (this.loading) {
      checklistItemsContent = html`
        <div class="loading" role="status" aria-live="polite">
          <i class="fas fa-spinner fa-spin" aria-hidden="true"></i>
          Loading checklist…
        </div>
      `;
    } else if (this.error) {
      checklistItemsContent = html`
        <div class="error-wrapper" role="alert">
          <error-display
            .augmentedError="${this.error}"
            .action="${{
              label: 'Retry',
              onClick: () => {
                this.error = null;
                this.loading = true;
                void this.fetchData();
              },
            }}"
          ></error-display>
        </div>
      `;
    } else {
      const itemsListContent = this.items.length === 0
        ? html`<div class="empty-state">No items yet. Add one below!</div>`
        : html`
            ${this._renderTagFilterBar()}
            ${this._renderItems()}
          `;
      checklistItemsContent = html`
        ${itemsListContent}
        ${this._renderAddItem()}
      `;
    }

    return html`
      ${sharedStyles}
      <div class="checklist-container system-font">
        <div class="checklist-header">
          <h2 class="checklist-title">${this.formatTitle(this.listName)}</h2>
          <div class="header-actions">
            <span
              class="sr-only"
              role="status"
              aria-live="polite"
              aria-atomic="true"
            >${this.saving ? 'Saving…' : ''}</span>
            ${this.saving
              ? html`<span class="saving-indicator" aria-hidden="true">Saving…</span>`
              : nothing}
            ${this.items.some(item => item.checked)
              ? html`
                  <button
                    class="delete-checked-btn"
                    ?disabled="${this.saving}"
                    @click="${this._handleDeleteChecked}"
                    aria-label="Delete all checked items"
                  >
                    delete checked
                  </button>
                `
              : nothing}
          </div>
        </div>

        ${this._renderListDebugBadge()}
        ${checklistItemsContent}
        ${this._renderOccToast()}
      </div>
    `;
  }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

interface MutationResponse {
  checklist?: Checklist;
}

function timestampsEqual(a: Timestamp | undefined, b: Timestamp | undefined): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  return a.seconds === b.seconds && a.nanos === b.nanos;
}

/**
 * Render a Timestamp as a short relative-time phrase, e.g. "2h ago" or
 * "in 30m". Falls back to ISO date for very old/distant times.
 */
function formatRelativeTime(ts: Timestamp): string {
  const date = timestampDate(ts);
  const diffMs = date.getTime() - Date.now();
  const absMs = Math.abs(diffMs);
  const past = diffMs < 0;

  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;

  if (absMs < minute) return past ? 'just now' : 'in <1m';
  if (absMs < hour) {
    const m = Math.round(absMs / minute);
    return past ? `${m}m ago` : `in ${m}m`;
  }
  if (absMs < day) {
    const h = Math.round(absMs / hour);
    return past ? `${h}h ago` : `in ${h}h`;
  }
  if (absMs < 7 * day) {
    const d = Math.round(absMs / day);
    return past ? `${d}d ago` : `in ${d}d`;
  }
  return date.toISOString().slice(0, 10);
}

function isDebugBadgeEnabled(): boolean {
  try {
    return globalThis.localStorage?.getItem('wiki-checklist-debug') === '1';
  } catch {
    return false;
  }
}

customElements.define('wiki-checklist', WikiChecklist);

declare global {
  interface HTMLElementTagNameMap {
    'wiki-checklist': WikiChecklist;
  }
}
