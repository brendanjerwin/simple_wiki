import type { ChecklistItem } from './checklist-tag-parser.js';

// Long-press delay in milliseconds before initiating touch drag
const LONG_PRESS_DELAY_MS = 400;

// Movement threshold in pixels; exceeding this cancels the long-press
const LONG_PRESS_MOVE_THRESHOLD_PX = 10;

export interface DragState {
  dragSourceItemIndex: number | null;
  dragOverItemIndex: number | null;
  dragOverItemPosition: 'before' | 'after';
  touchDragActive: boolean;
  longPressHandleIndex: number | null;
}

// Interface for callbacks the drag manager needs (IoC - no config struct per CLAUDE.md)
export interface DragReorderHandler {
  // Called whenever drag visual state changes (triggers re-render)
  onDragStateChanged(): void;
  // Called when a drag operation completes with a reorder needed
  onReorder(fromIndex: number, toInsertIndex: number): Promise<void>;
  // Called on touch reorder error
  onError(err: unknown): void;
  // For ghost element creation/positioning
  getShadowRoot(): ShadowRoot | null;
  // For adding/removing CSS class to host element
  getHostElement(): HTMLElement;
}

/**
 * Pure function: move item from fromIndex to toInsertIndex in resulting array.
 * reorderItems([A,B,C,D], 3, 0) -> [D,A,B,C]
 */
export function reorderItems(
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

export class ChecklistDragManager {
  private _dragSourceItemIndex: number | null = null;
  private _dragOverItemIndex: number | null = null;
  private _dragOverItemPosition: 'before' | 'after' = 'before';
  private _touchDragActive = false;
  private _longPressTimerId: ReturnType<typeof setTimeout> | null = null;
  private _longPressHandleIndex: number | null = null;
  private _touchStartX = 0;
  private _touchStartY = 0;
  private _touchGhostEl: HTMLElement | null = null;

  // Bound listener references for proper cleanup
  private _boundTouchMove: ((e: TouchEvent) => void) | null = null;
  private _boundTouchEnd: ((e: TouchEvent) => void) | null = null;
  private _boundTouchCancel: ((e: TouchEvent) => void) | null = null;

  constructor(private readonly handler: DragReorderHandler) {}

  // Getters and setters for all drag state (component reads/writes these during render and tests)
  get dragSourceItemIndex(): number | null {
    return this._dragSourceItemIndex;
  }

  set dragSourceItemIndex(value: number | null) {
    this._dragSourceItemIndex = value;
  }

  get dragOverItemIndex(): number | null {
    return this._dragOverItemIndex;
  }

  set dragOverItemIndex(value: number | null) {
    this._dragOverItemIndex = value;
  }

  get dragOverItemPosition(): 'before' | 'after' {
    return this._dragOverItemPosition;
  }

  set dragOverItemPosition(value: 'before' | 'after') {
    this._dragOverItemPosition = value;
  }

  get touchDragActive(): boolean {
    return this._touchDragActive;
  }

  get longPressHandleIndex(): number | null {
    return this._longPressHandleIndex;
  }

  // Expose internal state for testing
  get longPressTimerId(): ReturnType<typeof setTimeout> | null {
    return this._longPressTimerId;
  }

  get touchStartX(): number {
    return this._touchStartX;
  }

  get touchStartY(): number {
    return this._touchStartY;
  }

  get touchGhostEl(): HTMLElement | null {
    return this._touchGhostEl;
  }

  // Mouse drag handlers
  handleDragHandleMousedown(e: MouseEvent): void {
    if (!(e.target instanceof HTMLElement)) return;
    const row = e.target.closest('.item-row');
    if (row instanceof HTMLElement) {
      row.draggable = true;
    }
  }

  handleItemDragStart(e: DragEvent, index: number): void {
    this._dragSourceItemIndex = index;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', String(index));
    }
  }

  handleItemDragOver(e: DragEvent, index: number): void {
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
    this.handler.onDragStateChanged();
  }

  handleItemDragLeave(e: DragEvent): void {
    if (
      e.currentTarget instanceof HTMLElement &&
      e.relatedTarget instanceof Node &&
      e.currentTarget.contains(e.relatedTarget)
    ) {
      return;
    }
    this._dragOverItemIndex = null;
    this.handler.onDragStateChanged();
  }

  async handleItemDrop(e: DragEvent, targetIndex: number): Promise<void> {
    e.preventDefault();
    const sourceIndex = this._dragSourceItemIndex;
    if (sourceIndex === null) {
      this._clearDragState();
      return;
    }

    const position = this._dragOverItemPosition;
    const insertIndex =
      position === 'before' ? targetIndex : targetIndex + 1;

    this._clearDragState();
    await this.handler.onReorder(sourceIndex, insertIndex);
  }

  handleItemDragEnd(e: DragEvent): void {
    if (e.currentTarget instanceof HTMLElement) {
      e.currentTarget.draggable = false;
    }
    this._clearDragState();
  }

  // Touch drag handlers
  handleTouchStart(e: TouchEvent, index: number): void {
    const touch = e.changedTouches[0];
    if (!touch) return;

    // Cancel any existing long-press
    this._cancelLongPress();

    this._touchStartX = touch.clientX;
    this._touchStartY = touch.clientY;
    this._longPressHandleIndex = index;
    this.handler.onDragStateChanged();

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

  cleanup(): void {
    this._cleanupTouchDrag();
  }

  private _clearDragState(): void {
    this._dragSourceItemIndex = null;
    this._dragOverItemIndex = null;
    this.handler.onDragStateChanged();
  }

  // Public methods for internal touch handling (exposed for testing)
  handleTouchMove(e: TouchEvent): void {
    this._handleTouchMove(e);
  }

  async handleTouchEnd(): Promise<void> {
    await this._handleTouchEnd();
  }

  handleTouchCancel(): void {
    this._handleTouchCancel();
  }

  startTouchDrag(index: number, touch: Touch): void {
    this._startTouchDrag(index, touch);
  }

  cancelLongPress(): void {
    this._cancelLongPress();
  }

  private _handleActiveDragTouchMove(e: TouchEvent, touch: Touch): void {
    // Active drag: prevent scrolling, move ghost, compute drop target
    e.preventDefault();
    this._moveGhost(touch.clientX, touch.clientY);

    const shadowRoot = this.handler.getShadowRoot();
    const elementUnderFinger = shadowRoot?.elementFromPoint(touch.clientX, touch.clientY);

    // Walk up to find the .item-row and read data-index
    const row = elementUnderFinger?.closest('.item-row');
    if (!(row instanceof HTMLElement)) return;

    const indexAttr = row.dataset['index'];
    if (indexAttr === undefined) return;

    const targetIndex = Number.parseInt(indexAttr, 10);
    const rect = row.getBoundingClientRect();
    const midY = rect.top + rect.height / 2;
    this._dragOverItemIndex = targetIndex;
    this._dragOverItemPosition = touch.clientY < midY ? 'before' : 'after';
    this.handler.onDragStateChanged();
  }

  private _handlePreDragTouchMove(touch: Touch): void {
    // Pre-drag: check if finger moved beyond threshold (user is scrolling)
    const dx = touch.clientX - this._touchStartX;
    const dy = touch.clientY - this._touchStartY;
    const distancePx = Math.hypot(dx, dy);
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
          await this.handler.onReorder(sourceIndex, insertIndex);
        }
      } else {
        // Touch ended before long-press fired
        this._cancelLongPress();
        this._removeDocumentTouchListeners();
      }
    } catch (err) {
      this.handler.onError(err);
    }
  }

  private _handleTouchCancel(): void {
    this._cleanupTouchDrag();
  }

  private _startTouchDrag(index: number, touch: Touch): void {
    this._touchDragActive = true;
    this._dragSourceItemIndex = index;
    this._longPressHandleIndex = null;

    // Add touch-dragging class to host
    this.handler.getHostElement().classList.add('touch-dragging');
    this.handler.onDragStateChanged();

    // Create ghost element from the source row
    const shadowRoot = this.handler.getShadowRoot();
    const rows = shadowRoot?.querySelectorAll('.item-row');
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
      shadowRoot?.appendChild(ghost);
      this._touchGhostEl = ghost;
    }
  }

  private _cleanupTouchDrag(): void {
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
    this.handler.getHostElement().classList.remove('touch-dragging');
    this.handler.onDragStateChanged();
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
}
