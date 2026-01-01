import type { EditorContextMenu } from '../web-components/editor-context-menu.js';
import type { EditorToolbar } from '../web-components/editor-toolbar.js';
// Side-effect import required to register the custom element before createElement
import '../web-components/insert-new-page-dialog.js';
import type { InsertNewPageDialog } from '../web-components/insert-new-page-dialog.js';
import type { PageCreatedEventDetail } from '../web-components/event-types.js';
import { EditorUploadService } from './editor-upload-service.js';
import { TextFormattingService } from './text-formatting-service.js';

/**
 * EditorContextMenuCoordinator orchestrates the context menu and toolbar lifecycle.
 * - Desktop: right-click shows context menu
 * - Mobile: toolbar is always visible (long-press disabled to not interfere with text selection)
 */
export class EditorContextMenuCoordinator {
  private textarea: HTMLTextAreaElement;
  private menu: EditorContextMenu;
  private toolbar: EditorToolbar | null;
  private uploadService: EditorUploadService;
  private formattingService: TextFormattingService;
  private insertNewPageDialog: InsertNewPageDialog | null = null;
  private isMobile: boolean;

  // Selection state for restoration
  private savedSelectionStart = 0;
  private savedSelectionEnd = 0;

  constructor(
    textarea: HTMLTextAreaElement,
    menu: EditorContextMenu,
    uploadService?: EditorUploadService,
    formattingService?: TextFormattingService,
    toolbar?: EditorToolbar | null
  ) {
    this.textarea = textarea;
    this.menu = menu;
    this.toolbar = toolbar || null;
    this.uploadService = uploadService || new EditorUploadService();
    this.formattingService = formattingService || new TextFormattingService();
    this.isMobile = this.detectMobile();

    this.attachEventListeners();
  }

  private detectMobile(): boolean {
    return window.matchMedia('(pointer: coarse)').matches;
  }

  private attachEventListeners(): void {
    // Desktop only: right-click context menu
    // On mobile, long-press triggers contextmenu and we don't want to block native text selection
    if (!this.isMobile) {
      this.textarea.addEventListener('contextmenu', this._handleContextMenu);
    }

    // Context menu action handlers
    this.menu.addEventListener('upload-image-requested', this._handleUploadImage);
    this.menu.addEventListener('upload-file-requested', this._handleUploadFile);
    this.menu.addEventListener('take-photo-requested', this._handleTakePhoto);
    this.menu.addEventListener('format-bold-requested', this._handleBold);
    this.menu.addEventListener('format-italic-requested', this._handleItalic);
    this.menu.addEventListener('insert-link-requested', this._handleInsertLink);
    this.menu.addEventListener('insert-new-page-requested', this._handleInsertNewPage);

    // Mobile: toolbar action handlers (same events, different source)
    if (this.toolbar) {
      // Save selection before toolbar button steals focus
      this.toolbar.addEventListener('mousedown', this._handleToolbarInteractionStart);
      this.toolbar.addEventListener('touchstart', this._handleToolbarInteractionStart);

      this.toolbar.addEventListener('upload-image-requested', this._handleUploadImage);
      this.toolbar.addEventListener('upload-file-requested', this._handleUploadFile);
      this.toolbar.addEventListener('format-bold-requested', this._handleBold);
      this.toolbar.addEventListener('format-italic-requested', this._handleItalic);
      this.toolbar.addEventListener('insert-link-requested', this._handleInsertLink);
      this.toolbar.addEventListener('insert-new-page-requested', this._handleInsertNewPage);
    }
  }

  /**
   * Detaches all event listeners. Call this when the coordinator is no longer needed.
   */
  detach(): void {
    if (!this.isMobile) {
      this.textarea.removeEventListener('contextmenu', this._handleContextMenu);
    }

    this.menu.removeEventListener('upload-image-requested', this._handleUploadImage);
    this.menu.removeEventListener('upload-file-requested', this._handleUploadFile);
    this.menu.removeEventListener('take-photo-requested', this._handleTakePhoto);
    this.menu.removeEventListener('format-bold-requested', this._handleBold);
    this.menu.removeEventListener('format-italic-requested', this._handleItalic);
    this.menu.removeEventListener('insert-link-requested', this._handleInsertLink);
    this.menu.removeEventListener('insert-new-page-requested', this._handleInsertNewPage);

    if (this.toolbar) {
      this.toolbar.removeEventListener('mousedown', this._handleToolbarInteractionStart);
      this.toolbar.removeEventListener('touchstart', this._handleToolbarInteractionStart);
      this.toolbar.removeEventListener('upload-image-requested', this._handleUploadImage);
      this.toolbar.removeEventListener('upload-file-requested', this._handleUploadFile);
      this.toolbar.removeEventListener('format-bold-requested', this._handleBold);
      this.toolbar.removeEventListener('format-italic-requested', this._handleItalic);
      this.toolbar.removeEventListener('insert-link-requested', this._handleInsertLink);
      this.toolbar.removeEventListener('insert-new-page-requested', this._handleInsertNewPage);
    }

    // Clean up dialog if it exists
    if (this.insertNewPageDialog) {
      this.insertNewPageDialog.remove();
      this.insertNewPageDialog = null;
    }
  }

  private _handleContextMenu = (e: MouseEvent): void => {
    e.preventDefault();
    this.saveSelection();
    this.showMenuAt(e.clientX, e.clientY);
  };

  private _handleToolbarInteractionStart = (): void => {
    // Save selection before toolbar button steals focus
    this.saveSelection();
  };

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
    this.menu.isMobile = this.isMobile;
    this.menu.openAt({ x, y });
  }

  private _handleUploadImage = async (): Promise<void> => {
    this.captureCurrentSelection();
    this.restoreSelection();
    const result = await this.uploadService.selectAndUploadImage();
    if (result) {
      this.uploadService.insertMarkdownAtCursor(this.textarea, result.markdownLink);
    }
  };

  private _handleUploadFile = async (): Promise<void> => {
    this.captureCurrentSelection();
    this.restoreSelection();
    const result = await this.uploadService.selectAndUploadFile();
    if (result) {
      this.uploadService.insertMarkdownAtCursor(this.textarea, result.markdownLink);
    }
  };

  private _handleTakePhoto = async (): Promise<void> => {
    this.captureCurrentSelection();
    this.restoreSelection();
    const result = await this.uploadService.capturePhoto();
    if (result) {
      this.uploadService.insertMarkdownAtCursor(this.textarea, result.markdownLink);
    }
  };

  private _handleBold = (): void => {
    this.captureCurrentSelection();
    this.restoreSelection();
    const result = this.formattingService.wrapBold(
      this.textarea.value,
      this.savedSelectionStart,
      this.savedSelectionEnd
    );
    this.applyFormattingResult(result);
  };

  private _handleItalic = (): void => {
    this.captureCurrentSelection();
    this.restoreSelection();
    const result = this.formattingService.wrapItalic(
      this.textarea.value,
      this.savedSelectionStart,
      this.savedSelectionEnd
    );
    this.applyFormattingResult(result);
  };

  private _handleInsertLink = (): void => {
    this.captureCurrentSelection();
    this.restoreSelection();
    const result = this.formattingService.insertLink(
      this.textarea.value,
      this.savedSelectionStart,
      this.savedSelectionEnd
    );
    this.applyFormattingResult(result);
  };

  /**
   * Captures the current selection from the textarea.
   * This is needed for toolbar actions where we don't have a prior event to save selection.
   */
  private captureCurrentSelection(): void {
    // Only capture if textarea still has valid selection
    if (document.activeElement === this.textarea) {
      this.saveSelection();
    }
  }

  private applyFormattingResult(result: { newText: string; newSelectionStart: number; newSelectionEnd: number }): void {
    this.textarea.value = result.newText;
    this.textarea.selectionStart = result.newSelectionStart;
    this.textarea.selectionEnd = result.newSelectionEnd;

    // Trigger keyup for auto-save
    this.textarea.dispatchEvent(new Event('keyup', { bubbles: true }));
  }

  private _handleInsertNewPage = async (): Promise<void> => {
    this.captureCurrentSelection();

    // Create or get the dialog
    if (!this.insertNewPageDialog) {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
      this.insertNewPageDialog = document.createElement('insert-new-page-dialog') as InsertNewPageDialog;
      document.body.appendChild(this.insertNewPageDialog);

      // Listen for page-created events
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
      this.insertNewPageDialog.addEventListener('page-created', this._handlePageCreated as EventListener);
    }

    await this.insertNewPageDialog.openDialog();
  };

  private _handlePageCreated = (event: CustomEvent<PageCreatedEventDetail>): void => {
    const { markdownLink } = event.detail;

    // Restore selection and insert the markdown link
    this.restoreSelection();
    this.uploadService.insertMarkdownAtCursor(this.textarea, markdownLink);
  };
}
