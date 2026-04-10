import { html, LitElement, nothing } from 'lit';
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
  zIndexCSS,
} from './shared-styles.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import './error-display.js';
import { parseTaggedInput, composeTaggedText } from './checklist-tag-parser.js';
import type { ChecklistItem, ChecklistData } from './checklist-tag-parser.js';
import { extractChecklistData, asRecord } from './checklist-data-service.js';
import { reorderItems, ChecklistDragManager } from './checklist-drag-manager.js';
import type { DragReorderHandler } from './checklist-drag-manager.js';
import { wikiChecklistStyles } from './wiki-checklist-styles.js';

export type { ChecklistItem, ChecklistData };

// Polling interval in milliseconds
const POLL_INTERVAL_MS = 10000;

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


  private pollingTimer: ReturnType<typeof setInterval> | null = null;

  readonly client = createClient(Frontmatter, getGrpcWebTransport());

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
    const newItems = reorderItems(this.items, fromIndex, toInsertIndex);
    this.items = newItems;
    await this._persistData(newItems);
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
   * Fetch checklist data from GetFrontmatter and update state.
   */
  private async fetchData(): Promise<void> {
    if (!this.page) {
      throw new Error('wiki-checklist: page attribute is required but not set');
    }

    try {
      const request = create(GetFrontmatterRequestSchema, { page: this.page });
      const response = await this.client.getFrontmatter(request);
      const { items } = extractChecklistData(response.frontmatter ?? {}, this.listName);
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
  private async _persistData(
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
        ...currentResponse.frontmatter,
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
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- asRecord narrows to non-null object; values originate from parsed JSON and are valid JsonValues
      const existingChecklists = (asRecord(currentFrontmatter['checklists']) ?? {}) as JsonObject;
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
        const { items } = extractChecklistData(
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
    await this._persistData(newItems);
  }

  private async _handleRemoveItem(index: number): Promise<void> {
    const newItems = this.items.filter((_, i) => i !== index);
    this.items = newItems;
    await this._persistData(newItems);
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
    const newItems = this.items.map((item, i) =>
      i === index ? { ...item, text, tags } : item
    );
    this.items = newItems;
    await this._persistData(newItems);
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

  private async _handleAddItem(): Promise<void> {
    const { tags, text } = parseTaggedInput(this.newItemText);
    if (!text) return;
    const newItem: ChecklistItem = { text, checked: false, tags };
    const newItems = [...this.items, newItem];
    this.items = newItems;
    this.newItemText = '';
    await this._persistData(newItems);
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
    const isDragging = this._dragManager.dragSourceItemIndex === index;
    const isDragOver = this._dragManager.dragOverItemIndex === index;
    const dragOverClass = isDragOver
      ? `drag-over-${this._dragManager.dragOverItemPosition}`
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
          class="drag-handle ${this._dragManager.longPressHandleIndex === index ? 'long-press-pending' : ''}"
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
    await this._persistData(newItems);
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
