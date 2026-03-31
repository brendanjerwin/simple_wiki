import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create, type JsonObject } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  Frontmatter,
  GetFrontmatterRequestSchema,
  MergeFrontmatterRequestSchema,
} from '../gen/api/v1/frontmatter_pb.js';
import {
  foundationCSS,
  buttonCSS,
  inputCSS,
  pillCSS,
  sharedStyles,
} from './shared-styles.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import './error-display.js';

// Polling interval in milliseconds
const POLL_INTERVAL_MS = 3000;

// Long-press delay in milliseconds before initiating touch drag
const LONG_PRESS_DELAY_MS = 400;

// Movement threshold in pixels; exceeding this cancels the long-press
const LONG_PRESS_MOVE_THRESHOLD_PX = 10;

export interface ChecklistItem {
  text: string;
  checked: boolean;
  tags: string[];
}

export interface ChecklistData {
  items: ChecklistItem[];
}

/**
 * WikiChecklist - A fully API-driven interactive checklist component.
 *
 * Polls GetFrontmatter for data and persists changes via MergeFrontmatter.
 *
 * @property {string} listName - Checklist name in frontmatter (attribute: list-name)
 * @property {string} page - Page identifier for gRPC calls
 *
 * @example
 * <wiki-checklist list-name="grocery_list" page="my-page"></wiki-checklist>
 */
export class WikiChecklist extends LitElement {
  static override readonly styles = [
    foundationCSS,
    buttonCSS,
    inputCSS,
    pillCSS,
    css`
      :host {
        display: block;
        font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto',
          'Oxygen', 'Ubuntu', 'Cantarell', sans-serif;
        color: #333;
      }

      .checklist-container {
        border: 1px solid #e0e0e0;
        border-radius: 8px;
        background: #fff;
        padding: 16px;
        max-width: 600px;
      }

      .checklist-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 12px;
      }

      .checklist-title {
        font-size: 18px;
        font-weight: 600;
        margin: 0;
        color: #333;
      }

      .header-actions {
        display: flex;
        align-items: center;
        gap: 8px;
      }

      .saving-indicator {
        font-size: 12px;
        color: #6c757d;
      }

      .loading {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 16px;
        color: #666;
        font-size: 14px;
      }

      .empty-state {
        padding: 16px;
        text-align: center;
        color: #888;
        font-size: 14px;
      }

      .items-list {
        list-style: none;
        margin: 0;
        padding: 0;
      }

      .item-row {
        display: flex;
        align-items: flex-start;
        gap: 8px;
        padding: 6px 4px;
        border-radius: 4px;
        transition: background 0.1s ease;
        position: relative;
      }

      .item-row:hover {
        background: #f8f9fa;
      }

      .item-checkbox {
        flex-shrink: 0;
        width: 16px;
        height: 16px;
        margin-top: 2px;
        cursor: pointer;
        accent-color: #6c757d;
      }

      .item-content {
        display: flex;
        flex: 1;
        align-items: center;
        gap: 4px;
        flex-wrap: wrap;
        min-width: 0;
      }

      .item-text {
        flex: 1 1 auto;
        min-width: 80px;
        font-size: 14px;
        border: none;
        background: transparent;
        padding: 2px 4px;
        border-radius: 3px;
        font-family: inherit;
        transition: background 0.1s ease;
      }

      .item-text:focus {
        outline: none;
        background: #f0f0f0;
      }

      .item-checked .item-text {
        text-decoration: line-through;
        opacity: 0.6;
        color: #888;
      }

      .item-display-text {
        flex: 1 1 auto;
        min-width: 80px;
        font-size: 14px;
        padding: 2px 4px;
        cursor: text;
        overflow-wrap: break-word;
      }

      .item-display-text:focus {
        outline: 2px solid #6c757d;
        outline-offset: 1px;
        border-radius: 3px;
      }

      .item-checked .item-display-text {
        text-decoration: line-through;
        opacity: 0.6;
        color: #888;
      }

      /* .item-tag-badge styles provided by pillCSS */

      .remove-btn {
        background: none;
        border: none;
        cursor: pointer;
        color: #ccc;
        font-size: 16px;
        padding: 2px 4px;
        border-radius: 3px;
        line-height: 1;
        flex-shrink: 0;
        transition: color 0.15s ease;
      }

      .remove-btn:hover {
        color: #dc3545;
      }

      .drag-handle {
        flex-shrink: 0;
        cursor: grab;
        color: #ccc;
        font-size: 14px;
        padding: 2px 2px;
        margin-top: 2px;
        line-height: 1;
        user-select: none;
        transition: color 0.15s ease;
      }

      .drag-handle:hover {
        color: #888;
      }

      .drag-handle:active {
        cursor: grabbing;
      }

      .item-row.dragging {
        opacity: 0.4;
      }

      .item-row.drag-over-before::before {
        content: '';
        position: absolute;
        top: -1px;
        left: 0;
        right: 0;
        height: 2px;
        background: #0d6efd;
        border-radius: 1px;
      }

      .item-row.drag-over-after::after {
        content: '';
        position: absolute;
        bottom: -1px;
        left: 0;
        right: 0;
        height: 2px;
        background: #0d6efd;
        border-radius: 1px;
      }

      :host(.touch-dragging) {
        touch-action: none;
        user-select: none;
      }

      .drag-handle.long-press-pending {
        color: #0d6efd;
        transform: scale(1.2);
      }

      .touch-drag-ghost {
        position: fixed;
        z-index: 9999;
        pointer-events: none;
        opacity: 0.85;
        background: #fff;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
        border-radius: 4px;
      }

      .tag-filter-bar {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
        margin-bottom: 8px;
      }

      /* .tag-pill, .tag-pill-active, .tag-filter-clear styles provided by pillCSS */

      .delete-checked-btn {
        font-size: 12px;
        padding: 3px 8px;
        background: none;
        border: none;
        color: #888;
        cursor: pointer;
        font-family: inherit;
        transition: color 0.15s ease;
      }

      .delete-checked-btn:hover {
        color: #dc3545;
      }

      .add-item {
        display: flex;
        gap: 8px;
        margin-top: 12px;
        align-items: center;
      }

      .add-text-input {
        flex: 1;
        padding: 6px 10px;
        border: 1px solid #ddd;
        border-radius: 4px;
        font-size: 14px;
        font-family: inherit;
        min-width: 0;
        box-sizing: border-box;
      }

      .add-text-input:focus {
        outline: none;
        border-color: #6c757d;
        box-shadow: 0 0 0 2px rgba(108, 117, 125, 0.15);
      }

      .add-btn {
        padding: 6px 14px;
        background: #6c757d;
        color: white;
        border: none;
        border-radius: 4px;
        font-size: 14px;
        cursor: pointer;
        font-family: inherit;
        transition: background 0.2s ease;
        white-space: nowrap;
        flex-shrink: 0;
      }

      .add-btn:hover:not(:disabled) {
        background: #5a6268;
      }

      .add-btn:disabled {
        opacity: 0.6;
        cursor: not-allowed;
      }

      .error-wrapper {
        margin-top: 8px;
      }

      @media (max-width: 480px) {
        .checklist-container {
          padding: 12px;
        }
      }
    `,
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

  // Index of the item currently being edited (text input focused)
  @state()
  private declare editingIndex: number | null;

  // Value of the new item text input
  @state()
  private declare newItemText: string;

  // Drag-and-drop state for items
  @state()
  private declare _dragSourceItemIndex: number | null;

  @state()
  private declare _dragOverItemIndex: number | null;

  @state()
  private declare _dragOverItemPosition: 'before' | 'after';

  // Touch drag state
  @state()
  declare _touchDragActive: boolean;

  private _longPressTimerId: ReturnType<typeof setTimeout> | null = null;
  private _longPressHandleIndex: number | null = null;
  private _touchStartX = 0;
  private _touchStartY = 0;
  private _touchGhostEl: HTMLElement | null = null;

  // Bound listener references for proper cleanup
  private _boundTouchMove: ((e: TouchEvent) => void) | null = null;
  private _boundTouchEnd: ((e: TouchEvent) => void) | null = null;
  private _boundTouchCancel: ((e: TouchEvent) => void) | null = null;

  private pollingTimer: ReturnType<typeof setInterval> | null = null;

  readonly client = createClient(Frontmatter, getGrpcWebTransport());

  constructor() {
    super();
    this.listName = '';
    this.page = '';
    this.items = [];
    this.loading = false;
    this.saving = false;
    this.error = null;
    this.filterTags = [];
    this.editingIndex = null;
    this.newItemText = '';
    this._dragSourceItemIndex = null;
    this._dragOverItemIndex = null;
    this._dragOverItemPosition = 'before';
    this._touchDragActive = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    // Remove server-rendered fallback content now that JS has taken over.
    this.innerHTML = '';
    if (this.page) {
      this.loading = true;
      void this.fetchData();
    }
    this.pollingTimer = setInterval(() => {
      if (this.page) {
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
    this._cleanupTouchDrag();
  }

  /**
   * Format a listName (snake_case or kebab-case) into a display title.
   * e.g. "grocery_list" -> "Grocery List", "my-checklist" -> "My Checklist"
   */
  formatTitle(listName: string): string {
    if (!listName) return '';
    return listName
      .replace(/[_-]/g, ' ')
      .replace(/\b\w/g, c => c.toUpperCase());
  }

  /**
   * Extract tags from a raw checklist item record.
   * Prefers new `tags` array format; falls back to old `tag` string.
   */
  private _parseItemTags(r: Record<string, unknown>): string[] {
    if (Array.isArray(r['tags'])) {
      return r['tags'].filter(
        (t): t is string => typeof t === 'string' && t !== ''
      );
    }
    if (typeof r['tag'] === 'string' && r['tag']) {
      return [r['tag']];
    }
    return [];
  }

  /**
   * Parse a single raw checklist item into a ChecklistItem, or return null if invalid.
   */
  /**
   * Narrow `value` to a non-null, non-array object, or return null.
   */
  private _asRecord(value: unknown): Record<string, unknown> | null {
    if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- narrowed above: non-null, non-array object
    return value as Record<string, unknown>;
  }

  private _parseChecklistItem(raw: unknown): ChecklistItem | null {
    const r = this._asRecord(raw);
    if (!r) return null;
    return {
      text: typeof r['text'] === 'string' ? r['text'] : '',
      checked: Boolean(r['checked']),
      tags: this._parseItemTags(r),
    };
  }

  /**
   * Extract ChecklistData from the raw frontmatter object.
   * Backward-compatible: reads both `tag` (old string) and `tags` (new array).
   */
  extractChecklistData(
    frontmatter: JsonObject,
    listName: string
  ): ChecklistData {
    const checklistsObj = this._asRecord(frontmatter['checklists']);
    if (!checklistsObj) return { items: [] };

    const listObj = this._asRecord(checklistsObj[listName]);
    if (!listObj) return { items: [] };

    const rawItems = listObj['items'];
    if (!Array.isArray(rawItems)) return { items: [] };

    const items: ChecklistItem[] = [];
    for (const raw of rawItems) {
      const item = this._parseChecklistItem(raw);
      if (item) items.push(item);
    }
    return { items };
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
      if (this.filterTags.length === 0 || this.filterTags.every(ft => item.tags.includes(ft))) {
        result.push({ item, index: i });
      }
    }
    return result;
  }

  /**
   * Fetch checklist data from GetFrontmatter and update state.
   */
  private async fetchData(): Promise<void> {
    if (!this.page) {
      throw new Error('wiki-checklist: page attribute is required but not set');
    }

    try {
      const request = create(GetFrontmatterRequestSchema, { page: this.page });
      const response = await this.client.getFrontmatter(request);
      const { items } = this.extractChecklistData(
        response.frontmatter ?? {},
        this.listName
      );
      this.items = items;
      this.error = null;
    } catch (err) {
      this.error = AugmentErrorService.augmentError(err, 'loading checklist');
    } finally {
      this.loading = false;
    }
  }

  /**
   * Read-modify-write: get current frontmatter, update checklists key, merge back.
   */
  private async persistData(
    newItems: ChecklistItem[]
  ): Promise<void> {
    if (!this.page) {
      throw new Error('wiki-checklist: page attribute is required but not set');
    }

    try {
      this.saving = true;

      // Read current frontmatter
      const getRequest = create(GetFrontmatterRequestSchema, {
        page: this.page,
      });
      const currentResponse = await this.client.getFrontmatter(getRequest);
      const currentFrontmatter: JsonObject = {
        ...(currentResponse.frontmatter ?? {}),
      };

      // Build updated checklist data
      const checklistData: JsonObject = {
        items: newItems.map(item => {
          const obj: JsonObject = { text: item.text, checked: item.checked };
          if (item.tags.length > 0) {
            obj['tags'] = item.tags;
          }
          return obj;
        }),
      };

      // Update the checklists key
      const existingChecklists = this._asRecord(currentFrontmatter['checklists']) ?? {};
      const updatedChecklists: JsonObject = {
        ...existingChecklists,
        [this.listName]: checklistData,
      };

      const mergeRequest = create(MergeFrontmatterRequestSchema, {
        page: this.page,
        frontmatter: { checklists: updatedChecklists },
      });
      const mergeResponse = await this.client.mergeFrontmatter(mergeRequest);

      // Update local state from the response
      if (mergeResponse.frontmatter) {
        const { items } = this.extractChecklistData(
          mergeResponse.frontmatter,
          this.listName
        );
        this.items = items;
      }
      this.error = null;
    } catch (err) {
      this.error = AugmentErrorService.augmentError(err, 'saving checklist');
    } finally {
      this.saving = false;
    }
  }

  private async _handleToggleItem(index: number): Promise<void> {
    const newItems = this.items.map((item, i) =>
      i === index ? { ...item, checked: !item.checked } : item
    );
    this.items = newItems;
    await this.persistData(newItems);
  }

  private async _handleRemoveItem(index: number): Promise<void> {
    const newItems = this.items.filter((_, i) => i !== index);
    this.items = newItems;
    await this.persistData(newItems);
  }

  /**
   * Compose structured item data into the editable `:tag` text format.
   * e.g. { text: "milk", tags: ["dairy", "fridge"] } → "milk :dairy :fridge"
   */
  composeTaggedText(item: ChecklistItem): string {
    if (item.tags.length === 0) return item.text;
    return item.text + item.tags.map(t => ` :${t}`).join('');
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
      input.value = this.composeTaggedText(item);
      input.focus();
    }
  }

  private async _handleItemTextBlur(index: number, value: string): Promise<void> {
    this.editingIndex = null;
    const { tags, text } = this.parseTaggedInput(value);
    if (!text) return;
    const newItems = this.items.map((item, i) =>
      i === index ? { ...item, text, tags } : item
    );
    this.items = newItems;
    await this.persistData(newItems);
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
    }
  }

  /**
   * Parse all `:tag` tokens from the input string.
   * A tag token is a colon followed by a non-whitespace word.
   * Tags are lowercased for case-agnostic grouping.
   * Examples:
   *   "milk :dairy :fridge"  -> { tags: ["dairy", "fridge"], text: "milk" }
   *   ":dairy milk :fridge"  -> { tags: ["dairy", "fridge"], text: "milk" }
   *   "buy :dairy milk"      -> { tags: ["dairy"], text: "buy milk" }
   *   "just milk"            -> { tags: [], text: "just milk" }
   */
  parseTaggedInput(input: string) {
    const tags: string[] = [];
    let text = input;
    const tagPattern = /:(\S+)/g;
    let match: RegExpExecArray | null;

    while ((match = tagPattern.exec(input)) !== null) {
      const tag = match[1]?.trim().toLowerCase();
      if (tag) {
        tags.push(tag);
      }
    }

    // Remove all :tag tokens from the text
    text = input.replace(/:(\S+)/g, '').replace(/\s+/g, ' ').trim();

    return { tags, text };
  }

  private async _handleAddItem(): Promise<void> {
    const { tags, text } = this.parseTaggedInput(this.newItemText);
    if (!text) return;
    const newItem: ChecklistItem = { text, checked: false, tags };
    const newItems = [...this.items, newItem];
    this.items = newItems;
    this.newItemText = '';
    await this.persistData(newItems);
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

  /**
   * Move an item from fromIndex to the position specified by toInsertIndex
   * in the resulting array (before removal of the source item).
   *
   * For example: reorderItems([A,B,C,D], 3, 0) moves D to position 0,
   * resulting in [D,A,B,C].
   */
  reorderItems(
    items: ChecklistItem[],
    fromIndex: number,
    toInsertIndex: number
  ): ChecklistItem[] {
    if (fromIndex < 0 || fromIndex >= items.length) return items;
    if (toInsertIndex < 0 || toInsertIndex > items.length) return items;
    const result = [...items];
    const [item] = result.splice(fromIndex, 1);
    if (!item) return items;
    const adjustedIndex =
      fromIndex < toInsertIndex ? toInsertIndex - 1 : toInsertIndex;
    result.splice(adjustedIndex, 0, item);
    return result;
  }

  private _clearDragState(): void {
    this._dragSourceItemIndex = null;
    this._dragOverItemIndex = null;
  }

  private _handleDragHandleMousedown(e: MouseEvent): void {
    if (!(e.target instanceof HTMLElement)) return;
    const row = e.target.closest('.item-row');
    if (row instanceof HTMLElement) {
      row.draggable = true;
    }
  }

  private _handleItemDragStart(e: DragEvent, index: number): void {
    this._dragSourceItemIndex = index;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', String(index));
    }
  }

  private _handleItemDragOver(
    e: DragEvent,
    index: number
  ): void {
    if (this._dragSourceItemIndex === null) {
      return;
    }
    e.preventDefault();
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = 'move';
    }
    if (!(e.currentTarget instanceof HTMLElement)) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const midY = rect.top + rect.height / 2;
    this._dragOverItemIndex = index;
    this._dragOverItemPosition = e.clientY < midY ? 'before' : 'after';
  }

  private async _handleItemDrop(
    e: DragEvent,
    targetIndex: number
  ): Promise<void> {
    e.preventDefault();
    const sourceIndex = this._dragSourceItemIndex;
    if (sourceIndex === null) {
      this._clearDragState();
      return;
    }

    const position = this._dragOverItemPosition;
    const insertIndex =
      position === 'before' ? targetIndex : targetIndex + 1;

    const newItems = this.reorderItems(this.items, sourceIndex, insertIndex);

    this._clearDragState();
    this.items = newItems;
    await this.persistData(newItems);
  }

  private _handleItemDragEnd(e: DragEvent): void {
    if (e.currentTarget instanceof HTMLElement) {
      e.currentTarget.draggable = false;
    }
    this._clearDragState();
  }

  private _handleItemDragLeave(e: DragEvent): void {
    if (
      e.currentTarget instanceof HTMLElement &&
      e.relatedTarget instanceof Node &&
      e.currentTarget.contains(e.relatedTarget)
    ) {
      return;
    }
    this._dragOverItemIndex = null;
  }

  _handleTouchStart(e: TouchEvent, index: number): void {
    const touch = e.changedTouches[0];
    if (!touch) return;

    // Cancel any existing long-press
    this._cancelLongPress();

    this._touchStartX = touch.clientX;
    this._touchStartY = touch.clientY;
    this._longPressHandleIndex = index;

    // Register document-level listeners for move/end/cancel
    this._boundTouchMove = (ev: TouchEvent) => this._handleTouchMove(ev);
    this._boundTouchEnd = () => { void this._handleTouchEnd(); };
    this._boundTouchCancel = () => this._handleTouchCancel();
    document.addEventListener('touchmove', this._boundTouchMove, { passive: false });
    document.addEventListener('touchend', this._boundTouchEnd);
    document.addEventListener('touchcancel', this._boundTouchCancel);

    this._longPressTimerId = setTimeout(() => {
      this._longPressTimerId = null;
      this._startTouchDrag(index, touch);
    }, LONG_PRESS_DELAY_MS);
  }

  private _handleActiveDragTouchMove(e: TouchEvent, touch: Touch): void {
    // Active drag: prevent scrolling, move ghost, compute drop target
    e.preventDefault();
    this._moveGhost(touch.clientX, touch.clientY);

    const elementUnderFinger = this.shadowRoot?.elementFromPoint(touch.clientX, touch.clientY);

    // Walk up to find the .item-row and read data-index
    const row = elementUnderFinger?.closest('.item-row');
    if (!(row instanceof HTMLElement)) return;

    const indexAttr = row.dataset['index'];
    if (indexAttr === undefined) return;

    const targetIndex = parseInt(indexAttr, 10);
    const rect = row.getBoundingClientRect();
    const midY = rect.top + rect.height / 2;
    this._dragOverItemIndex = targetIndex;
    this._dragOverItemPosition = touch.clientY < midY ? 'before' : 'after';
  }

  private _handlePreDragTouchMove(touch: Touch): void {
    // Pre-drag: check if finger moved beyond threshold (user is scrolling)
    const dx = touch.clientX - this._touchStartX;
    const dy = touch.clientY - this._touchStartY;
    const distancePx = Math.sqrt(dx * dx + dy * dy);
    if (distancePx > LONG_PRESS_MOVE_THRESHOLD_PX) {
      this._cancelLongPress();
      this._removeDocumentTouchListeners();
    }
  }

  private _handleTouchMove(e: TouchEvent): void {
    const touch = e.changedTouches[0];
    if (!touch) return;

    if (this._touchDragActive) {
      this._handleActiveDragTouchMove(e, touch);
    } else if (this._longPressTimerId !== null) {
      this._handlePreDragTouchMove(touch);
    }
  }

  private async _handleTouchEnd(): Promise<void> {
    try {
      if (this._touchDragActive) {
        // Commit the reorder
        const sourceIndex = this._dragSourceItemIndex;
        const targetIndex = this._dragOverItemIndex;
        const position = this._dragOverItemPosition;

        this._cleanupTouchDrag();

        if (sourceIndex !== null && targetIndex !== null) {
          const insertIndex = position === 'before' ? targetIndex : targetIndex + 1;
          const newItems = this.reorderItems(this.items, sourceIndex, insertIndex);
          this.items = newItems;
          await this.persistData(newItems);
        }
      } else {
        // Touch ended before long-press fired
        this._cancelLongPress();
        this._removeDocumentTouchListeners();
      }
    } catch (err) {
      this.error = AugmentErrorService.augmentError(err, 'touch reorder');
    }
  }

  private _handleTouchCancel(): void {
    this._cleanupTouchDrag();
  }

  _startTouchDrag(index: number, touch: Touch): void {
    this._touchDragActive = true;
    this._dragSourceItemIndex = index;
    this._longPressHandleIndex = null;

    // Add touch-dragging class to host
    this.classList.add('touch-dragging');

    // Create ghost element from the source row
    const rows = this.shadowRoot?.querySelectorAll('.item-row');
    const sourceRow = rows?.[index];
    if (sourceRow instanceof HTMLElement) {
      const cloned = sourceRow.cloneNode(true);
      if (!(cloned instanceof HTMLElement)) return;
      const ghost = cloned;
      ghost.classList.add('touch-drag-ghost');
      // Size the ghost to match the source row
      const rect = sourceRow.getBoundingClientRect();
      ghost.style.width = `${rect.width}px`;
      this._moveGhost(touch.clientX, touch.clientY, ghost);
      this.shadowRoot?.appendChild(ghost);
      this._touchGhostEl = ghost;
    }
  }

  _cleanupTouchDrag(): void {
    this._cancelLongPress();

    // Remove ghost
    if (this._touchGhostEl) {
      this._touchGhostEl.remove();
      this._touchGhostEl = null;
    }

    // Remove document listeners
    this._removeDocumentTouchListeners();

    // Reset state
    this._touchDragActive = false;
    this._dragSourceItemIndex = null;
    this._dragOverItemIndex = null;
    this._longPressHandleIndex = null;

    // Remove host class
    this.classList.remove('touch-dragging');
  }

  private _cancelLongPress(): void {
    if (this._longPressTimerId !== null) {
      clearTimeout(this._longPressTimerId);
      this._longPressTimerId = null;
    }
    this._longPressHandleIndex = null;
  }

  private _removeDocumentTouchListeners(): void {
    if (this._boundTouchMove) {
      document.removeEventListener('touchmove', this._boundTouchMove);
      this._boundTouchMove = null;
    }
    if (this._boundTouchEnd) {
      document.removeEventListener('touchend', this._boundTouchEnd);
      this._boundTouchEnd = null;
    }
    if (this._boundTouchCancel) {
      document.removeEventListener('touchcancel', this._boundTouchCancel);
      this._boundTouchCancel = null;
    }
  }

  private _moveGhost(clientX: number, clientY: number, ghost?: HTMLElement): void {
    const el = ghost ?? this._touchGhostEl;
    if (!el) return;
    el.style.left = `${clientX}px`;
    el.style.top = `${clientY - 20}px`;
  }

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

  private _renderItemDisplayText(item: ChecklistItem, index: number) {
    return html`
      <span
        class="item-display-text"
        role="button"
        tabindex="0"
        @click="${() => void this._enterEditMode(index)}"
        @keydown="${(e: KeyboardEvent) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            void this._enterEditMode(index);
          }
        }}"
      >${item.text}</span>
      ${item.tags.map(
        tag => html`<span class="item-tag-badge">${tag}</span>`
      )}`;
  }

  private _renderItem(
    item: ChecklistItem,
    index: number
  ) {
    const isDragging = this._dragSourceItemIndex === index;
    const isDragOver = this._dragOverItemIndex === index;
    const dragOverClass = isDragOver
      ? `drag-over-${this._dragOverItemPosition}`
      : '';

    return html`
      <li
        class="item-row ${item.checked ? 'item-checked' : ''} ${isDragging ? 'dragging' : ''} ${dragOverClass}"
        data-index="${index}"
        @dragstart="${(e: DragEvent) => this._handleItemDragStart(e, index)}"
        @dragover="${(e: DragEvent) =>
          this._handleItemDragOver(e, index)}"
        @dragleave="${(e: DragEvent) => this._handleItemDragLeave(e)}"
        @drop="${(e: DragEvent) => this._handleItemDrop(e, index)}"
        @dragend="${(e: DragEvent) => this._handleItemDragEnd(e)}"
      >
        <span
          class="drag-handle ${this._longPressHandleIndex === index ? 'long-press-pending' : ''}"
          aria-hidden="true"
          @mousedown="${(e: MouseEvent) => this._handleDragHandleMousedown(e)}"
          @touchstart="${(e: TouchEvent) => this._handleTouchStart(e, index)}"
        >\u2807</span>
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
        </span>
        <button
          class="remove-btn"
          title="Remove item"
          aria-label="Remove item"
          ?disabled="${this.saving}"
          @click="${() => this._handleRemoveItem(index)}"
        >
          \u2715
        </button>
      </li>
    `;
  }

  private async _handleDeleteChecked(): Promise<void> {
    const newItems = this.items.filter(item => !item.checked);
    this.items = newItems;
    await this.persistData(newItems);
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
          placeholder="Add item\u2026 (use :tag for grouping)"
          aria-label="New item text, with optional :tag anywhere for grouping"
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

  override render() {
    let checklistItemsContent;
    if (this.loading) {
      checklistItemsContent = html`
        <div class="loading" role="status" aria-live="polite">
          <i class="fas fa-spinner fa-spin" aria-hidden="true"></i>
          Loading checklist\u2026
        </div>
      `;
    } else if (this.error) {
      checklistItemsContent = html`
        <div class="error-wrapper">
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
            ${this.saving
              ? html`<span class="saving-indicator">Saving\u2026</span>`
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

        ${checklistItemsContent}
      </div>
    `;
  }
}

customElements.define('wiki-checklist', WikiChecklist);

declare global {
  interface HTMLElementTagNameMap {
    'wiki-checklist': WikiChecklist;
  }
}
