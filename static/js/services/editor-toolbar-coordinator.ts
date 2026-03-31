import type { EditorToolbar } from '../web-components/editor-toolbar.js';
// Side-effect import required to register the custom element before createElement
import '../web-components/insert-new-page-dialog.js';
import type { InsertNewPageDialog } from '../web-components/insert-new-page-dialog.js';
import type { PageCreatedEventDetail } from '../web-components/event-types.js';
import { EditorUploadService } from './editor-upload-service.js';
import { TextFormattingService } from './text-formatting-service.js';

/**
 * EditorToolbarCoordinator orchestrates the toolbar lifecycle.
 * It saves/restores textarea selection around toolbar interactions
 * and dispatches formatting, upload, and insert-new-page actions.
 */
export class EditorToolbarCoordinator {
  private readonly textarea: HTMLTextAreaElement;
  private readonly toolbar: EditorToolbar;
  private readonly uploadService: EditorUploadService;
  private readonly formattingService: TextFormattingService;
  private insertNewPageDialog: InsertNewPageDialog | null = null;

  // Selection state for restoration
  private savedSelectionStart = 0;
  private savedSelectionEnd = 0;

  constructor(
    textarea: HTMLTextAreaElement,
    toolbar: EditorToolbar,
    uploadService?: EditorUploadService,
    formattingService?: TextFormattingService
  ) {
    this.textarea = textarea;
    this.toolbar = toolbar;
    this.uploadService = uploadService ?? new EditorUploadService();
    this.formattingService = formattingService ?? new TextFormattingService();

    this.attachEventListeners();
  }

  private attachEventListeners(): void {
    // Save selection before toolbar button steals focus
    this.toolbar.addEventListener('mousedown', this._handleToolbarInteractionStart);

    this.toolbar.addEventListener('upload-image-requested', this._handleUploadImage);
    this.toolbar.addEventListener('upload-file-requested', this._handleUploadFile);
    this.toolbar.addEventListener('format-bold-requested', this._handleBold);
    this.toolbar.addEventListener('format-italic-requested', this._handleItalic);
    this.toolbar.addEventListener('insert-link-requested', this._handleInsertLink);
    this.toolbar.addEventListener('insert-new-page-requested', this._handleInsertNewPage);
  }

  /**
   * Detaches all event listeners. Call this when the coordinator is no longer needed.
   */
  detach(): void {
    this.toolbar.removeEventListener('mousedown', this._handleToolbarInteractionStart);
    this.toolbar.removeEventListener('upload-image-requested', this._handleUploadImage);
    this.toolbar.removeEventListener('upload-file-requested', this._handleUploadFile);
    this.toolbar.removeEventListener('format-bold-requested', this._handleBold);
    this.toolbar.removeEventListener('format-italic-requested', this._handleItalic);
    this.toolbar.removeEventListener('insert-link-requested', this._handleInsertLink);
    this.toolbar.removeEventListener('insert-new-page-requested', this._handleInsertNewPage);

    // Clean up dialog if it exists
    if (this.insertNewPageDialog) {
      this.insertNewPageDialog.remove();
      this.insertNewPageDialog = null;
    }
  }

  private readonly _handleToolbarInteractionStart = (): void => {
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

  private readonly _handleUploadImage = async (): Promise<void> => {
    this.restoreSelection();
    const result = await this.uploadService.selectAndUploadImage();
    if (result) {
      this.uploadService.insertMarkdownAtCursor(this.textarea, result.markdownLink);
    }
  };

  private readonly _handleUploadFile = async (): Promise<void> => {
    this.restoreSelection();
    const result = await this.uploadService.selectAndUploadFile();
    if (result) {
      this.uploadService.insertMarkdownAtCursor(this.textarea, result.markdownLink);
    }
  };

  private readonly _handleBold = (): void => {
    this.restoreSelection();
    if (this.savedSelectionStart === this.savedSelectionEnd) return;
    const result = this.formattingService.wrapBold(
      this.textarea.value,
      this.savedSelectionStart,
      this.savedSelectionEnd
    );
    this.applyFormattingResult(result);
  };

  private readonly _handleItalic = (): void => {
    this.restoreSelection();
    if (this.savedSelectionStart === this.savedSelectionEnd) return;
    const result = this.formattingService.wrapItalic(
      this.textarea.value,
      this.savedSelectionStart,
      this.savedSelectionEnd
    );
    this.applyFormattingResult(result);
  };

  private readonly _handleInsertLink = (): void => {
    this.restoreSelection();
    if (this.savedSelectionStart === this.savedSelectionEnd) return;
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

  private readonly _handleInsertNewPage = async (): Promise<void> => {
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

  private readonly _handlePageCreated = (event: CustomEvent<PageCreatedEventDetail>): void => {
    const { markdownLink } = event.detail;

    // Restore selection and insert the markdown link
    this.restoreSelection();
    this.uploadService.insertMarkdownAtCursor(this.textarea, markdownLink);
  };
}
