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
  sharedStyles,
} from './shared-styles.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import './error-display.js';

// Polling interval in milliseconds
const POLL_INTERVAL_MS = 3000;

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
  static override styles = [
    foundationCSS,
    buttonCSS,
    inputCSS,
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
        align-items: center;
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
        cursor: pointer;
        accent-color: #6c757d;
      }

      .item-text {
        flex: 1;
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

      .item-tag-badge {
        font-size: 11px;
        padding: 2px 6px;
        background: #e9ecef;
        border-radius: 12px;
        color: #555;
        white-space: nowrap;
        border: none;
        font-family: inherit;
        display: inline-block;
      }

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

      .tag-filter-bar {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
        margin-bottom: 8px;
      }

      .tag-pill {
        font-size: 12px;
        padding: 3px 10px;
        background: #e9ecef;
        border: 1px solid #dee2e6;
        border-radius: 16px;
        color: #555;
        cursor: pointer;
        font-family: inherit;
        transition: all 0.15s ease;
      }

      .tag-pill:hover {
        background: #d0d0d0;
        border-color: #aaa;
      }

      .tag-pill-active {
        background: #0d6efd;
        color: white;
        border-color: #0d6efd;
      }

      .tag-pill-active:hover {
        background: #0b5ed7;
        border-color: #0b5ed7;
      }

      .tag-filter-clear {
        font-size: 12px;
        padding: 3px 7px;
        background: none;
        border: 1px solid #dee2e6;
        border-radius: 16px;
        color: #dc3545;
        cursor: pointer;
        font-family: inherit;
        line-height: 1;
        transition: all 0.15s ease;
      }

      .tag-filter-clear:hover {
        background: #dc3545;
        color: white;
        border-color: #dc3545;
      }

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
  }

  override connectedCallback(): void {
    super.connectedCallback();
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
   * Extract ChecklistData from the raw frontmatter object.
   * Backward-compatible: reads both `tag` (old string) and `tags` (new array).
   */
  extractChecklistData(
    frontmatter: JsonObject,
    listName: string
  ): ChecklistData {
    const checklists = frontmatter['checklists'];
    if (
      !checklists ||
      typeof checklists !== 'object' ||
      Array.isArray(checklists)
    ) {
      return { items: [] };
    }
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- narrowed above: non-null, non-array object
    const checklistsObj = checklists as Record<string, unknown>;
    const listData = checklistsObj[listName];
    if (!listData || typeof listData !== 'object' || Array.isArray(listData)) {
      return { items: [] };
    }
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- narrowed above: non-null, non-array object
    const listObj = listData as Record<string, unknown>;
    const rawItems = listObj['items'];
    const items: ChecklistItem[] = [];
    if (Array.isArray(rawItems)) {
      for (const raw of rawItems) {
        if (raw && typeof raw === 'object' && !Array.isArray(raw)) {
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- narrowed above: non-null, non-array object
          const r = raw as Record<string, unknown>;
          const item: ChecklistItem = {
            text: typeof r['text'] === 'string' ? r['text'] : '',
            checked: Boolean(r['checked']),
            tags: [],
          };
          // Prefer new `tags` array format, fall back to old `tag` string
          if (Array.isArray(r['tags'])) {
            item.tags = r['tags'].filter(
              (t): t is string => typeof t === 'string' && t !== ''
            );
          } else if (typeof r['tag'] === 'string' && r['tag']) {
            item.tags = [r['tag']];
          }
          items.push(item);
        }
      }
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
    return Array.from(tagSet).sort();
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
      const existingChecklists =
        typeof currentFrontmatter['checklists'] === 'object' &&
        !Array.isArray(currentFrontmatter['checklists']) &&
        currentFrontmatter['checklists'] !== null
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- narrowed above: non-null, non-array object matching JsonObject structure
          ? (currentFrontmatter['checklists'] as JsonObject)
          : {};
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

  private _handleItemFocus(index: number, inputEl: HTMLInputElement): void {
    this.editingIndex = index;
    const item = this.items[index];
    if (item) {
      inputEl.value = this.composeTaggedText(item);
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
  parseTaggedInput(input: string): { tags: string[]; text: string } {
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

  private _handleItemDragEnd(): void {
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
        draggable="true"
        @dragstart="${(e: DragEvent) => this._handleItemDragStart(e, index)}"
        @dragover="${(e: DragEvent) =>
          this._handleItemDragOver(e, index)}"
        @dragleave="${(e: DragEvent) => this._handleItemDragLeave(e)}"
        @drop="${(e: DragEvent) => this._handleItemDrop(e, index)}"
        @dragend="${() => this._handleItemDragEnd()}"
      >
        <span class="drag-handle" aria-hidden="true">\u2807</span>
        <input
          type="checkbox"
          class="item-checkbox"
          .checked="${item.checked}"
          aria-label="${item.text}"
          ?disabled="${this.saving}"
          @change="${() => this._handleToggleItem(index)}"
        />
        <input
          type="text"
          class="item-text"
          .value="${this.editingIndex === index ? this.composeTaggedText(item) : item.text}"
          aria-label="Edit item text and tags"
          @focus="${(e: FocusEvent) => {
            if (!(e.target instanceof HTMLInputElement)) return;
            this._handleItemFocus(index, e.target);
          }}"
          @blur="${(e: FocusEvent) => {
            if (!(e.target instanceof HTMLInputElement)) return;
            void this._handleItemTextBlur(index, e.target.value);
          }}"
          @keydown="${(e: KeyboardEvent) => {
            if (!(e.currentTarget instanceof HTMLInputElement)) return;
            this._handleItemTextKeydown(index, e.currentTarget.value, e);
          }}"
        />
        ${this.editingIndex !== index
          ? item.tags.map(
              tag => html`<span class="item-tag-badge">${tag}</span>`
            )
          : nothing}
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

        ${this.loading
          ? html`
              <div class="loading" role="status" aria-live="polite">
                <i class="fas fa-spinner fa-spin" aria-hidden="true"></i>
                Loading checklist\u2026
              </div>
            `
          : this.error
            ? html`
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
              `
            : html`
                ${this.items.length === 0
                  ? html`<div class="empty-state">No items yet. Add one below!</div>`
                  : html`
                      ${this._renderTagFilterBar()}
                      ${this._renderItems()}
                    `}
                ${this._renderAddItem()}
              `}
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
