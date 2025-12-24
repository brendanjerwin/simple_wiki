import { EditorContextMenu } from '../web-components/editor-context-menu.js';
import { EditorUploadService } from './editor-upload-service.js';
import { TextFormattingService } from './text-formatting-service.js';

/**
 * EditorContextMenuCoordinator orchestrates the context menu lifecycle.
 * It detects triggers (right-click, long-press), shows the menu, and routes actions.
 */
export class EditorContextMenuCoordinator {
  private textarea: HTMLTextAreaElement;
  private menu: EditorContextMenu;
  private uploadService: EditorUploadService;
  private formattingService: TextFormattingService;

  // Long-press detection state
  private longPressTimeoutId: number | null = null;
  private longPressThresholdMs = 500;
  private longPressTriggered = false;
  private pointerStartX = 0;
  private pointerStartY = 0;
  private movementThresholdPx = 10;

  // Selection state for restoration
  private savedSelectionStart = 0;
  private savedSelectionEnd = 0;

  constructor(
    textarea: HTMLTextAreaElement,
    menu: EditorContextMenu,
    uploadService?: EditorUploadService,
    formattingService?: TextFormattingService
  ) {
    this.textarea = textarea;
    this.menu = menu;
    this.uploadService = uploadService || new EditorUploadService();
    this.formattingService = formattingService || new TextFormattingService();

    this.attachEventListeners();
  }

  private attachEventListeners(): void {
    // Desktop: right-click
    this.textarea.addEventListener('contextmenu', this._handleContextMenu);

    // Mobile: long-press via pointer events
    this.textarea.addEventListener('pointerdown', this._handlePointerDown);
    this.textarea.addEventListener('pointerup', this._handlePointerUp);
    this.textarea.addEventListener('pointercancel', this._handlePointerCancel);
    this.textarea.addEventListener('pointermove', this._handlePointerMove);

    // Menu action handlers
    this.menu.addEventListener('upload-image-requested', this._handleUploadImage);
    this.menu.addEventListener('upload-file-requested', this._handleUploadFile);
    this.menu.addEventListener('take-photo-requested', this._handleTakePhoto);
    this.menu.addEventListener('format-bold-requested', this._handleBold);
    this.menu.addEventListener('format-italic-requested', this._handleItalic);
    this.menu.addEventListener('insert-link-requested', this._handleInsertLink);
  }

  /**
   * Detaches all event listeners. Call this when the coordinator is no longer needed.
   */
  detach(): void {
    this.textarea.removeEventListener('contextmenu', this._handleContextMenu);
    this.textarea.removeEventListener('pointerdown', this._handlePointerDown);
    this.textarea.removeEventListener('pointerup', this._handlePointerUp);
    this.textarea.removeEventListener('pointercancel', this._handlePointerCancel);
    this.textarea.removeEventListener('pointermove', this._handlePointerMove);

    this.menu.removeEventListener('upload-image-requested', this._handleUploadImage);
    this.menu.removeEventListener('upload-file-requested', this._handleUploadFile);
    this.menu.removeEventListener('take-photo-requested', this._handleTakePhoto);
    this.menu.removeEventListener('format-bold-requested', this._handleBold);
    this.menu.removeEventListener('format-italic-requested', this._handleItalic);
    this.menu.removeEventListener('insert-link-requested', this._handleInsertLink);

    this.cancelLongPress();
  }

  private _handleContextMenu = (e: MouseEvent): void => {
    e.preventDefault();
    this.saveSelection();
    this.showMenuAt(e.clientX, e.clientY);
  };

  private _handlePointerDown = (e: PointerEvent): void => {
    this.pointerStartX = e.clientX;
    this.pointerStartY = e.clientY;
    this.longPressTriggered = false;

    this.longPressTimeoutId = window.setTimeout(() => {
      this.longPressTriggered = true;
      this.saveSelection();
      this.showMenuAt(e.clientX, e.clientY);
    }, this.longPressThresholdMs);
  };

  private _handlePointerMove = (e: PointerEvent): void => {
    const dx = Math.abs(e.clientX - this.pointerStartX);
    const dy = Math.abs(e.clientY - this.pointerStartY);

    if (dx > this.movementThresholdPx || dy > this.movementThresholdPx) {
      this.cancelLongPress();
    }
  };

  private _handlePointerUp = (): void => {
    this.cancelLongPress();
  };

  private _handlePointerCancel = (): void => {
    this.cancelLongPress();
  };

  private cancelLongPress(): void {
    if (this.longPressTimeoutId !== null) {
      clearTimeout(this.longPressTimeoutId);
      this.longPressTimeoutId = null;
    }
  }

  private saveSelection(): void {
    this.savedSelectionStart = this.textarea.selectionStart;
    this.savedSelectionEnd = this.textarea.selectionEnd;
  }

  private restoreSelection(): void {
    this.textarea.focus();
    this.textarea.selectionStart = this.savedSelectionStart;
    this.textarea.selectionEnd = this.savedSelectionEnd;
  }

  private showMenuAt(x: number, y: number): void {
    const hasSelection = this.savedSelectionStart !== this.savedSelectionEnd;
    this.menu.hasSelection = hasSelection;
    this.menu.isMobile = this.isTouchDevice();
    this.menu.openAt({ x, y });
  }

  private isTouchDevice(): boolean {
    return 'ontouchstart' in window || navigator.maxTouchPoints > 0;
  }

  private _handleUploadImage = async (): Promise<void> => {
    this.restoreSelection();
    const result = await this.uploadService.selectAndUploadImage();
    if (result) {
      this.uploadService.insertMarkdownAtCursor(this.textarea, result.markdownLink);
    }
  };

  private _handleUploadFile = async (): Promise<void> => {
    this.restoreSelection();
    const result = await this.uploadService.selectAndUploadFile();
    if (result) {
      this.uploadService.insertMarkdownAtCursor(this.textarea, result.markdownLink);
    }
  };

  private _handleTakePhoto = async (): Promise<void> => {
    this.restoreSelection();
    const result = await this.uploadService.capturePhoto();
    if (result) {
      this.uploadService.insertMarkdownAtCursor(this.textarea, result.markdownLink);
    }
  };

  private _handleBold = (): void => {
    this.restoreSelection();
    const result = this.formattingService.wrapBold(
      this.textarea.value,
      this.savedSelectionStart,
      this.savedSelectionEnd
    );
    this.applyFormattingResult(result);
  };

  private _handleItalic = (): void => {
    this.restoreSelection();
    const result = this.formattingService.wrapItalic(
      this.textarea.value,
      this.savedSelectionStart,
      this.savedSelectionEnd
    );
    this.applyFormattingResult(result);
  };

  private _handleInsertLink = (): void => {
    this.restoreSelection();
    const result = this.formattingService.insertLink(
      this.textarea.value,
      this.savedSelectionStart,
      this.savedSelectionEnd
    );
    this.applyFormattingResult(result);
  };

  private applyFormattingResult(result: { newText: string; newSelectionStart: number; newSelectionEnd: number }): void {
    this.textarea.value = result.newText;
    this.textarea.selectionStart = result.newSelectionStart;
    this.textarea.selectionEnd = result.newSelectionEnd;

    // Trigger keyup for auto-save
    this.textarea.dispatchEvent(new Event('keyup', { bubbles: true }));
  }
}
