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
  tag?: string;
}

export interface ChecklistData {
  items: ChecklistItem[];
  groupOrder: string[] | null;
}

export interface GroupedItems {
  tag: string;
  items: Array<{ item: ChecklistItem; index: number }>;
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

      .view-toggle {
        background: none;
        border: 1px solid #ddd;
        border-radius: 4px;
        padding: 4px 8px;
        font-size: 12px;
        cursor: pointer;
        color: #555;
        transition: all 0.2s ease;
      }

      .view-toggle:hover {
        background: #f0f0f0;
        border-color: #aaa;
      }

      .view-toggle[aria-pressed='true'] {
        background: #6c757d;
        color: white;
        border-color: #6c757d;
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
        cursor: pointer;
        border: none;
        font-family: inherit;
        transition: background 0.1s ease;
      }

      .item-tag-badge:hover {
        background: #d0d0d0;
      }

      .item-tag-input {
        font-size: 11px;
        padding: 2px 6px;
        border: 1px solid #aaa;
        border-radius: 12px;
        width: 80px;
        font-family: inherit;
      }

      .item-tag-input:focus {
        outline: none;
        border-color: #6c757d;
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

      .group-section {
        margin-bottom: 12px;
      }

      .group-header {
        position: relative;
        font-size: 12px;
        font-weight: 600;
        color: #6c757d;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        padding: 4px 4px 2px;
        border-bottom: 1px solid #eee;
        margin-bottom: 4px;
        cursor: grab;
      }

      .group-header.drag-over-before::before {
        content: '';
        position: absolute;
        top: -1px;
        left: 0;
        right: 0;
        height: 2px;
        background: #0d6efd;
        border-radius: 1px;
      }

      .group-header.drag-over-after::after {
        content: '';
        position: absolute;
        bottom: -1px;
        left: 0;
        right: 0;
        height: 2px;
        background: #0d6efd;
        border-radius: 1px;
      }

      .group-header.dragging {
        opacity: 0.4;
      }

      .add-item {
        display: flex;
        gap: 8px;
        margin-top: 12px;
        align-items: flex-start;
        flex-wrap: wrap;
      }

      .add-item-inputs {
        display: flex;
        flex-direction: column;
        gap: 4px;
        flex: 1;
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

      .add-tag-row {
        display: flex;
        align-items: center;
        gap: 6px;
      }

      .add-tag-input {
        padding: 4px 8px;
        border: 1px solid #ddd;
        border-radius: 4px;
        font-size: 12px;
        font-family: inherit;
        width: 120px;
        list-style: none;
        box-sizing: border-box;
      }

      .add-tag-input:focus {
        outline: none;
        border-color: #6c757d;
      }

      .add-tag-label {
        font-size: 12px;
        color: #888;
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
        align-self: flex-start;
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

        .add-item {
          flex-direction: column;
        }

        .add-btn {
          width: 100%;
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
  declare groupOrder: string[] | null;

  @state()
  declare groupedView: boolean;

  @state()
  declare loading: boolean;

  @state()
  declare saving: boolean;

  @state()
  declare error: AugmentedError | null;

  // Track which item's tag is being edited
  @state()
  private declare editingTagIndex: number | null;

  // Value of the new item text input
  @state()
  private declare newItemText: string;

  // Value of the new item tag input
  @state()
  private declare newItemTag: string;

  // Drag-and-drop state for items
  @state()
  private declare _dragSourceItemIndex: number | null;

  @state()
  private declare _dragOverItemIndex: number | null;

  @state()
  private declare _dragOverItemPosition: 'before' | 'after';

  // Drag-and-drop state for group headings
  @state()
  private declare _dragSourceGroupTag: string | null;

  @state()
  private declare _dragOverGroupTag: string | null;

  @state()
  private declare _dragOverGroupPosition: 'before' | 'after';

  private pollingTimer: ReturnType<typeof setInterval> | null = null;

  readonly client = createClient(Frontmatter, getGrpcWebTransport());

  constructor() {
    super();
    this.listName = '';
    this.page = '';
    this.items = [];
    this.groupOrder = null;
    this.groupedView = false;
    this.loading = false;
    this.saving = false;
    this.error = null;
    this.editingTagIndex = null;
    this.newItemText = '';
    this.newItemTag = '';
    this._dragSourceItemIndex = null;
    this._dragOverItemIndex = null;
    this._dragOverItemPosition = 'before';
    this._dragSourceGroupTag = null;
    this._dragOverGroupTag = null;
    this._dragOverGroupPosition = 'before';
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
   * e.g. "grocery_list" → "Grocery List", "my-checklist" → "My Checklist"
   */
  formatTitle(listName: string): string {
    if (!listName) return '';
    return listName
      .replace(/[_-]/g, ' ')
      .replace(/\b\w/g, c => c.toUpperCase());
  }

  /**
   * Extract ChecklistData from the raw frontmatter object.
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
      return { items: [], groupOrder: null };
    }
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- narrowed above: non-null, non-array object
    const checklistsObj = checklists as Record<string, unknown>;
    const listData = checklistsObj[listName];
    if (!listData || typeof listData !== 'object' || Array.isArray(listData)) {
      return { items: [], groupOrder: null };
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
          };
          if (typeof r['tag'] === 'string' && r['tag']) {
            item.tag = r['tag'];
          }
          items.push(item);
        }
      }
    }
    const rawGroupOrder = listObj['group_order'];
    let groupOrder: string[] | null = null;
    if (Array.isArray(rawGroupOrder)) {
      groupOrder = rawGroupOrder.filter(
        (g): g is string => typeof g === 'string'
      );
    }
    return { items, groupOrder };
  }

  /**
   * Return sorted unique tags from current items.
   */
  getExistingTags(): string[] {
    const tagSet = new Set<string>();
    for (const item of this.items) {
      if (item.tag) tagSet.add(item.tag);
    }
    return Array.from(tagSet).sort();
  }

  /**
   * Return items grouped by tag for grouped view.
   * Preserves absolute indices into the items array.
   */
  getGroupedItems(): GroupedItems[] {
    const groupMap = new Map<
      string,
      Array<{ item: ChecklistItem; index: number }>
    >();

    for (let i = 0; i < this.items.length; i++) {
      const item = this.items[i];
      if (!item) continue;
      const tag = item.tag || 'Other';
      if (!groupMap.has(tag)) {
        groupMap.set(tag, []);
      }
      const group = groupMap.get(tag);
      if (group) group.push({ item, index: i });
    }

    const allTags = Array.from(groupMap.keys());

    let orderedTags: string[];
    if (this.groupOrder && this.groupOrder.length > 0) {
      // Start with the ordered tags that exist, then append remaining ones
      orderedTags = [
        ...this.groupOrder.filter(t => groupMap.has(t)),
        ...allTags
          .filter(t => !this.groupOrder!.includes(t) && t !== 'Other')
          .sort(),
      ];
      // Append "Other" at the end if it exists
      if (groupMap.has('Other')) {
        orderedTags.push('Other');
      }
    } else {
      // Alphabetical, with "Other" at the end
      orderedTags = [
        ...allTags.filter(t => t !== 'Other').sort(),
        ...(groupMap.has('Other') ? ['Other'] : []),
      ];
    }

    return orderedTags
      .map(tag => {
        const items = groupMap.get(tag);
        if (!items) return null;
        return { tag, items };
      })
      .filter((g): g is GroupedItems => g !== null);
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
      const { items, groupOrder } = this.extractChecklistData(
        response.frontmatter ?? {},
        this.listName
      );
      this.items = items;
      this.groupOrder = groupOrder;
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
    newItems: ChecklistItem[],
    newGroupOrder: string[] | null
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
          if (item.tag) obj['tag'] = item.tag;
          return obj;
        }),
      };
      if (newGroupOrder && newGroupOrder.length > 0) {
        checklistData['group_order'] = newGroupOrder;
      }

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
        const { items, groupOrder } = this.extractChecklistData(
          mergeResponse.frontmatter,
          this.listName
        );
        this.items = items;
        this.groupOrder = groupOrder;
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
    await this.persistData(newItems, this.groupOrder);
  }

  private async _handleRemoveItem(index: number): Promise<void> {
    const newItems = this.items.filter((_, i) => i !== index);
    this.items = newItems;
    await this.persistData(newItems, this.groupOrder);
  }

  private _handleItemTextChange(index: number, value: string): void {
    const newItems = this.items.map((item, i) =>
      i === index ? { ...item, text: value } : item
    );
    this.items = newItems;
  }

  private async _handleItemTextBlur(index: number, value: string): Promise<void> {
    const trimmed = value.trim();
    if (!trimmed) return;
    const newItems = this.items.map((item, i) =>
      i === index ? { ...item, text: trimmed } : item
    );
    this.items = newItems;
    await this.persistData(newItems, this.groupOrder);
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

  private _handleTagBadgeClick(index: number): void {
    this.editingTagIndex = index;
  }

  private async _handleTagInputBlur(
    index: number,
    value: string
  ): Promise<void> {
    this.editingTagIndex = null;
    const trimmed = value.trim();
    const newItems = this.items.map((item, i) => {
      if (i !== index) return item;
      const updated = { ...item };
      if (trimmed) {
        updated.tag = trimmed;
      } else {
        delete updated.tag;
      }
      return updated;
    });
    this.items = newItems;
    await this.persistData(newItems, this.groupOrder);
  }

  private _handleTagInputKeydown(
    index: number,
    value: string,
    event: KeyboardEvent
  ): void {
    if (event.key === 'Enter') {
      void this._handleTagInputBlur(index, value);
      if (event.target instanceof HTMLElement) {
        event.target.blur();
      }
    }
    if (event.key === 'Escape') {
      this.editingTagIndex = null;
    }
  }

  private async _handleAddItem(): Promise<void> {
    const text = this.newItemText.trim();
    if (!text) return;
    const tag = this.newItemTag.trim();
    const newItem: ChecklistItem = { text, checked: false };
    if (tag) newItem.tag = tag;
    const newItems = [...this.items, newItem];
    this.items = newItems;
    this.newItemText = '';
    this.newItemTag = '';
    await this.persistData(newItems, this.groupOrder);
  }

  private _handleNewItemKeydown(event: KeyboardEvent): void {
    if (event.key === 'Enter') {
      void this._handleAddItem();
    }
  }

  private _handleToggleView(): void {
    this.groupedView = !this.groupedView;
  }

  /**
   * Move an item from fromIndex to the position specified by toInsertIndex
   * in the resulting array (before removal of the source item).
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

  /**
   * Reorder a group tag within the given ordered tag list.
   * Returns a new array with fromTag moved to be before or after toTag.
   */
  computeNewGroupOrder(
    currentTags: string[],
    fromTag: string,
    toTag: string,
    position: 'before' | 'after'
  ): string[] {
    const fromIdx = currentTags.indexOf(fromTag);
    const toIdx = currentTags.indexOf(toTag);
    if (fromIdx === -1 || toIdx === -1) return [...currentTags];
    const toInsertIndex = position === 'before' ? toIdx : toIdx + 1;
    const result = [...currentTags];
    result.splice(fromIdx, 1);
    const adjustedIndex =
      fromIdx < toInsertIndex ? toInsertIndex - 1 : toInsertIndex;
    result.splice(adjustedIndex, 0, fromTag);
    return result;
  }

  private _clearDragState(): void {
    this._dragSourceItemIndex = null;
    this._dragOverItemIndex = null;
    this._dragSourceGroupTag = null;
    this._dragOverGroupTag = null;
  }

  private _handleItemDragStart(e: DragEvent, index: number): void {
    this._dragSourceItemIndex = index;
    this._dragSourceGroupTag = null;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', String(index));
    }
  }

  private _handleItemDragOver(
    e: DragEvent,
    index: number
  ): void {
    if (
      this._dragSourceItemIndex === null &&
      this._dragSourceGroupTag === null
    ) {
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
    targetIndex: number,
    groupTag?: string
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

    let newItems = [...this.items];

    // Handle cross-group drop: update the item's tag
    if (groupTag !== undefined) {
      const sourceItem = newItems[sourceIndex];
      if (!sourceItem) {
        this._clearDragState();
        return;
      }
      const updatedItem = { ...sourceItem };
      if (groupTag === 'Other') {
        delete updatedItem.tag;
      } else {
        updatedItem.tag = groupTag;
      }
      newItems[sourceIndex] = updatedItem;
    }

    newItems = this.reorderItems(newItems, sourceIndex, insertIndex);

    this._clearDragState();
    this.items = newItems;
    await this.persistData(newItems, this.groupOrder);
  }

  private _handleItemDragEnd(): void {
    this._clearDragState();
  }

  private _handleGroupDragStart(e: DragEvent, tag: string): void {
    this._dragSourceGroupTag = tag;
    this._dragSourceItemIndex = null;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', tag);
    }
  }

  private _handleGroupDragOver(e: DragEvent, tag: string): void {
    if (this._dragSourceGroupTag === null) return;
    e.preventDefault();
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = 'move';
    }
    if (!(e.currentTarget instanceof HTMLElement)) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const midY = rect.top + rect.height / 2;
    this._dragOverGroupTag = tag;
    this._dragOverGroupPosition = e.clientY < midY ? 'before' : 'after';
  }

  private async _handleGroupDrop(e: DragEvent, targetTag: string): Promise<void> {
    e.preventDefault();
    const sourceTag = this._dragSourceGroupTag;
    if (!sourceTag) {
      this._clearDragState();
      return;
    }
    if (sourceTag === targetTag) {
      this._clearDragState();
      return;
    }

    // Build ordered tag list from current grouped items (excluding 'Other')
    const groups = this.getGroupedItems();
    const currentTags = groups.map(g => g.tag).filter(t => t !== 'Other');

    const newGroupOrder = this.computeNewGroupOrder(
      currentTags,
      sourceTag,
      targetTag,
      this._dragOverGroupPosition
    );

    this._clearDragState();
    this.groupOrder = newGroupOrder;
    await this.persistData(this.items, newGroupOrder);
  }

  private _handleGroupDragEnd(): void {
    this._clearDragState();
  }

  private _renderItem(
    item: ChecklistItem,
    index: number,
    tagSuggestionsId: string,
    groupTag?: string
  ) {
    const isEditingTag = this.editingTagIndex === index;
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
        @drop="${(e: DragEvent) => this._handleItemDrop(e, index, groupTag)}"
        @dragend="${() => this._handleItemDragEnd()}"
      >
        <span class="drag-handle" aria-hidden="true">⠿</span>
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
          .value="${item.text}"
          aria-label="Edit item text"
          @input="${(e: InputEvent) => {
            if (!(e.target instanceof HTMLInputElement)) return;
            this._handleItemTextChange(index, e.target.value);
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
        ${isEditingTag
          ? html`
              <input
                type="text"
                class="item-tag-input"
                .value="${item.tag ?? ''}"
                placeholder="tag"
                list="${tagSuggestionsId}"
                aria-label="Edit item tag"
                @blur="${(e: FocusEvent) => {
                  if (!(e.target instanceof HTMLInputElement)) return;
                  void this._handleTagInputBlur(index, e.target.value);
                }}"
                @keydown="${(e: KeyboardEvent) => {
                  if (!(e.currentTarget instanceof HTMLInputElement)) return;
                  this._handleTagInputKeydown(index, e.currentTarget.value, e);
                }}"
              />
            `
          : html`
              <button
                class="item-tag-badge"
                title="${item.tag ? `Tag: ${item.tag}. Click to edit` : 'Click to add tag'}"
                aria-label="${item.tag ? `Tag: ${item.tag}. Click to edit` : 'Add tag'}"
                @click="${() => this._handleTagBadgeClick(index)}"
              >
                ${item.tag ?? '+tag'}
              </button>
            `}
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

  private _renderFlatItems(tagSuggestionsId: string) {
    return html`
      <ul class="items-list" role="list">
        ${this.items.map((item, i) => this._renderItem(item, i, tagSuggestionsId))}
      </ul>
    `;
  }

  private _renderGroupedItems(tagSuggestionsId: string) {
    const groups = this.getGroupedItems();
    return html`
      ${groups.map(group => {
        const isGroupDragging = this._dragSourceGroupTag === group.tag;
        const isGroupDragOver = this._dragOverGroupTag === group.tag;
        const groupDragOverClass = isGroupDragOver
          ? `drag-over-${this._dragOverGroupPosition}`
          : '';
        return html`
          <div class="group-section">
            <div
              class="group-header ${isGroupDragging ? 'dragging' : ''} ${groupDragOverClass}"
              role="heading"
              aria-level="3"
              draggable="true"
              @dragstart="${(e: DragEvent) =>
                this._handleGroupDragStart(e, group.tag)}"
              @dragover="${(e: DragEvent) =>
                this._handleGroupDragOver(e, group.tag)}"
              @drop="${(e: DragEvent) =>
                this._handleGroupDrop(e, group.tag)}"
              @dragend="${() => this._handleGroupDragEnd()}"
            >
              <span class="drag-handle" aria-hidden="true">⠿</span>
              ${group.tag}
            </div>
            <ul class="items-list" role="list">
              ${group.items.map(({ item, index }) =>
                this._renderItem(item, index, tagSuggestionsId, group.tag)
              )}
            </ul>
          </div>
        `;
      })}
    `;
  }

  private _renderAddItem(tagSuggestionsId: string) {
    return html`
      <div class="add-item">
        <div class="add-item-inputs">
          <input
            type="text"
            class="add-text-input"
            .value="${this.newItemText}"
            placeholder="Add new item…"
            aria-label="New item text"
            ?disabled="${this.saving}"
            @input="${(e: InputEvent) => {
              if (!(e.target instanceof HTMLInputElement)) return;
              this.newItemText = e.target.value;
            }}"
            @keydown="${this._handleNewItemKeydown}"
          />
          <div class="add-tag-row">
            <span class="add-tag-label">Tag (optional):</span>
            <input
              type="text"
              class="add-tag-input"
              .value="${this.newItemTag}"
              placeholder="e.g. Dairy"
              list="${tagSuggestionsId}"
              aria-label="New item tag"
              ?disabled="${this.saving}"
              @input="${(e: InputEvent) => {
                if (!(e.target instanceof HTMLInputElement)) return;
                this.newItemTag = e.target.value;
              }}"
            />
          </div>
        </div>
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
    const tagSuggestionsId = `tag-suggestions-${this.listName}`;
    const tags = this.getExistingTags();

    return html`
      ${sharedStyles}
      <datalist id="${tagSuggestionsId}">
        ${tags.map(t => html`<option value="${t}"></option>`)}
      </datalist>
      <div class="checklist-container system-font">
        <div class="checklist-header">
          <h2 class="checklist-title">${this.formatTitle(this.listName)}</h2>
          <div class="header-actions">
            ${this.saving
              ? html`<span class="saving-indicator">Saving…</span>`
              : nothing}
            ${this.items.length > 0
              ? html`
                  <button
                    class="view-toggle"
                    aria-pressed="${this.groupedView}"
                    aria-label="${this.groupedView
                      ? 'Switch to flat view'
                      : 'Switch to grouped view'}"
                    title="${this.groupedView
                      ? 'Switch to flat view'
                      : 'Switch to grouped view'}"
                    @click="${this._handleToggleView}"
                  >
                    ${this.groupedView ? 'Flat' : 'Group'}
                  </button>
                `
              : nothing}
          </div>
        </div>

        ${this.loading
          ? html`
              <div class="loading" role="status" aria-live="polite">
                <i class="fas fa-spinner fa-spin" aria-hidden="true"></i>
                Loading checklist…
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
                  : this.groupedView
                    ? this._renderGroupedItems(tagSuggestionsId)
                    : this._renderFlatItems(tagSuggestionsId)}
                ${this._renderAddItem(tagSuggestionsId)}
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
