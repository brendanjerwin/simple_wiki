import { html, css, LitElement, nothing, type PropertyValues } from 'lit';
import { live } from 'lit/directives/live.js';
import { property, state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  PageManagementService,
  ReadPageRequestSchema,
  UpdateWholePageRequestSchema,
} from '../gen/api/v1/page_management_pb.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import { reconstructWholePageText } from '../services/page-content-reconstructor.js';
import {
  EditorSaveQueue,
  type SaveStatus,
} from '../services/editor-save-queue.js';
import { EditorToolbarCoordinator } from '../services/editor-toolbar-coordinator.js';
import type { EditorToolbar } from './editor-toolbar.js';
import './editor-toolbar.js';
import { foundationCSS, sharedStyles } from './shared-styles.js';
import './error-display.js';

interface FileUploadedDetail {
  uploadUrl: string;
  filename: string;
  isImage: boolean;
}

function isFileUploadedDetail(value: unknown): value is FileUploadedDetail {
  return (
    typeof value === 'object' &&
    value !== null &&
    'uploadUrl' in value &&
    'filename' in value &&
    'isImage' in value
  );
}

/**
 * WikiEditor — A Lit web component that provides a full-page markdown editor
 * backed by gRPC PageManagementService.
 *
 * Loads content via ReadPage, auto-saves via UpdateWholePage with debouncing
 * and concurrent-save prevention, and integrates with file-drop-zone for uploads
 * and EditorToolbarCoordinator for formatting.
 *
 * @element wiki-editor
 *
 * @property {string} page - The wiki page name to edit
 * @property {boolean} allowUploads - Whether file uploads are enabled
 * @property {number} maxUploadMb - Maximum file upload size in MB
 * @property {number} debounceMs - Debounce delay in ms for auto-save
 */
export class WikiEditor extends LitElement {
  static override styles = [
    foundationCSS,
    css`
      :host {
        display: block;
        height: 100%;
      }

      .editor-container {
        display: flex;
        flex-direction: column;
        height: 100%;
      }

      .status-bar {
        flex: 0 0 auto;
        display: flex;
        align-items: center;
        padding: 4px 8px;
        font-size: 0.85em;
        font-family: 'Montserrat', sans-serif;
        color: #777;
        background: #f8f8f8;
        border-bottom: 1px solid #eee;
        opacity: 0;
        transition: opacity 0.3s ease;
      }

      .status-bar.visible {
        opacity: 1;
      }

      .status-indicator {
        display: inline-flex;
        align-items: center;
        gap: 4px;
      }

      .status-indicator.saving {
        color: #f0ad4e;
      }

      .status-indicator.saved {
        color: #5cb85c;
        font-weight: bold;
      }

      .status-indicator.error {
        color: #d9534f;
        font-weight: bold;
      }

      editor-toolbar {
        flex: 0 0 auto;
      }

      .editor-area {
        flex: 1 1 auto;
        position: relative;
        min-height: 0;
      }

      .editor-area file-drop-zone {
        height: 100%;
      }

      textarea {
        width: 100%;
        height: 100%;
        box-sizing: border-box;
        position: absolute;
        inset: 0;
        border: none;
        outline: none;
        box-shadow: none;
        resize: none;
        font-size: 1em;
        font-family: "Lucida Console", Monaco, monospace;
        padding-left: 2%;
        padding-right: 2%;
        -webkit-user-select: text;
        user-select: text;
      }

      .loading-overlay {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100%;
        font-size: 1.2em;
        color: #777;
      }

      @media (max-width: 600px) {
        textarea {
          padding-bottom: 50vh;
        }
      }

      @media (min-width: 70em) {
        :host(:not([compact])) textarea {
          padding-left: 15%;
          padding-right: 15%;
        }
      }

      @media (min-width: 100em) {
        :host(:not([compact])) textarea {
          padding-left: 20%;
          padding-right: 20%;
        }
      }
    `,
  ];

  @property({ type: String })
  declare page: string;

  @property({ type: Boolean, attribute: 'allow-uploads' })
  declare allowUploads: boolean;

  @property({ type: Number, attribute: 'max-upload-mb' })
  declare maxUploadMb: number;

  @property({ type: Number, attribute: 'debounce-ms' })
  declare debounceMs: number;

  @property({ type: String, attribute: 'initial-content' })
  declare initialContent: string | undefined;

  @property({ type: Boolean, attribute: 'auto-save' })
  declare autoSave: boolean;

  @property({ type: Boolean, reflect: true })
  declare compact: boolean;

  @state()
  declare loading: boolean;

  @state()
  declare saveStatus: SaveStatus;

  @state()
  declare error: AugmentedError | null;

  @state()
  declare content: string;

  @state()
  declare _hasSelection: boolean;

  /** Stored for future optimistic concurrency control */
  versionHash = '';
  private saveQueue: EditorSaveQueue | null = null;
  private coordinator: EditorToolbarCoordinator | null = null;
  private savedTimerId: ReturnType<typeof setTimeout> | null = null;

  readonly client = createClient(PageManagementService, getGrpcWebTransport());

  constructor() {
    super();
    this.page = '';
    this.allowUploads = false;
    this.maxUploadMb = 10;
    this.debounceMs = 750;
    this.autoSave = true;
    this.compact = false;
    this.initialContent = undefined;
    this.loading = true;
    this.saveStatus = 'idle';
    this.error = null;
    this.content = '';
    this._hasSelection = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    this.initialize();
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this.teardown();
  }

  override updated(changedProperties: PropertyValues<this>): void {
    if (changedProperties.has('page') && changedProperties.get('page') !== undefined) {
      this.teardown();
      this.initialize();
    }
  }

  private initialize(): void {
    void this.loadContent().then(async () => {
      if (!this.isConnected) return;
      await this.updateComplete;
      if (this.autoSave) {
        this.setupSaveQueue();
      }
      this.setupCoordinator();
      this.focusTextarea();
    });
  }

  private teardown(): void {
    if (this.savedTimerId !== null) {
      clearTimeout(this.savedTimerId);
      this.savedTimerId = null;
    }
    this.saveQueue?.destroy();
    this.saveQueue = null;
    this.coordinator?.detach();
    this.coordinator = null;
  }

  private async loadContent(): Promise<void> {
    if (this.initialContent !== undefined) {
      this.content = this.initialContent;
      this.loading = false;
      return;
    }

    if (!this.page) {
      this.loading = false;
      return;
    }

    this.loading = true;
    this.error = null;

    try {
      const request = create(ReadPageRequestSchema, { pageName: this.page });
      const response = await this.client.readPage(request);

      this.content = reconstructWholePageText(
        response.frontMatterToml,
        response.contentMarkdown
      );
      this.versionHash = response.versionHash;
    } catch (err) {
      this.error = AugmentErrorService.augmentError(err, 'load page content');
    } finally {
      this.loading = false;
    }
  }

  private setupSaveQueue(): void {
    this.saveQueue = new EditorSaveQueue(
      this.debounceMs,
      async (content: string) => {
        const request = create(UpdateWholePageRequestSchema, {
          pageName: this.page,
          newWholeMarkdown: content,
        });
        const response = await this.client.updateWholePage(request);

        if (!response.success) {
          return {
            success: false,
            error: new Error(response.error || 'Save failed'),
          };
        }
        return { success: true };
      },
      (status, err) => {
        if (this.savedTimerId !== null) {
          clearTimeout(this.savedTimerId);
          this.savedTimerId = null;
        }

        this.saveStatus = status;
        if (status === 'error' && err) {
          this.error = AugmentErrorService.augmentError(err, 'save page');
        } else if (status !== 'error') {
          this.error = null;
        }

        if (status === 'saved') {
          this.savedTimerId = setTimeout(() => {
            this.saveStatus = 'idle';
            this.savedTimerId = null;
          }, 2000);
        }
      }
    );
  }

  private focusTextarea(): void {
    const textarea = this.shadowRoot?.querySelector('textarea');
    textarea?.focus();
  }

  private setupCoordinator(): void {
    const textarea = this.shadowRoot?.querySelector('textarea');
    if (!textarea) return;

    const toolbar = this.shadowRoot?.querySelector<EditorToolbar>('editor-toolbar');
    if (!toolbar) return;

    this.coordinator = new EditorToolbarCoordinator(textarea, toolbar);
  }

  /** Returns the current editor content. */
  getContent(): string {
    return this.content;
  }

  _checkSelection(): void {
    const textarea = this.shadowRoot?.querySelector('textarea');
    if (!textarea) return;
    this._hasSelection = textarea.selectionStart !== textarea.selectionEnd;
  }

  _onInput(e: Event): void {
    if (!(e.target instanceof HTMLTextAreaElement)) return;
    this.content = e.target.value;
    this.saveQueue?.contentChanged(this.content);
  }

  _onKeyup(): void {
    // Catch coordinator-initiated changes (formatting operations dispatch keyup)
    const textarea = this.shadowRoot?.querySelector('textarea');
    if (textarea && textarea.value !== this.content) {
      this.content = textarea.value;
      this.saveQueue?.contentChanged(this.content);
    }
    this._checkSelection();
  }

  _onKeydown(e: KeyboardEvent): void {
    if (e.key === 'Tab') {
      e.preventDefault();
      if (!(e.target instanceof HTMLTextAreaElement)) return;
      const textarea = e.target;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const value = textarea.value;

      textarea.value = value.substring(0, start) + '\t' + value.substring(end);
      textarea.selectionStart = textarea.selectionEnd = start + 1;

      this.content = textarea.value;
      this.saveQueue?.contentChanged(this.content);
    }
  }

  _onExitRequested(): void {
    this.dispatchEvent(new CustomEvent('exit-requested', { bubbles: true, composed: true }));
  }

  _onFileUploaded(e: Event): void {
    if (!(e instanceof CustomEvent)) return;
    const detail: unknown = e.detail;
    if (!isFileUploadedDetail(detail)) return;
    const textarea = this.shadowRoot?.querySelector('textarea');
    if (!textarea) return;

    const cursorPos = textarea.selectionStart;
    const cursorEnd = textarea.selectionEnd;
    const value = textarea.value;
    const textBefore = value.substring(0, cursorPos);
    const textAfter = value.substring(Math.max(cursorPos, cursorEnd));
    const prefix = detail.isImage ? '!' : '';
    const markdownLink = `${prefix}[${detail.filename}](${detail.uploadUrl})`;

    textarea.value = textBefore + markdownLink + textAfter;
    textarea.selectionStart = cursorPos;
    textarea.selectionEnd = cursorPos + markdownLink.length;

    this.content = textarea.value;
    this.saveQueue?.contentChanged(this.content);
  }

  private get statusText(): string {
    switch (this.saveStatus) {
      case 'editing':
        return 'Editing';
      case 'saving':
        return 'Saving';
      case 'saved':
        return 'Saved';
      case 'error':
        return 'Error';
      default:
        return '';
    }
  }

  _dismissSaveError(): void {
    this.error = null;
    this.saveStatus = 'idle';
  }

  override render() {
    if (this.loading) {
      return html`
        ${sharedStyles}
        <div class="loading-overlay">
          <i class="fa-solid fa-spinner fa-spin"></i>&nbsp;Loading...
        </div>
      `;
    }

    if (this.error && this.saveStatus !== 'error') {
      return html`
        ${sharedStyles}
        <error-display .augmentedError=${this.error}></error-display>
      `;
    }

    return html`
      ${sharedStyles}
      <div class="editor-container">
        <editor-toolbar ?has-selection=${this._hasSelection} ?hide-exit=${!this.autoSave} @exit-requested=${this._onExitRequested}></editor-toolbar>
        <div class="status-bar ${this.saveStatus !== 'idle' && this.saveStatus !== 'error' ? 'visible' : ''}">
          <span class="status-indicator ${this.saveStatus}">
            ${this.saveStatus === 'saving'
              ? html`<i class="fa-solid fa-spinner fa-spin"></i>`
              : nothing}
            ${this.statusText}
          </span>
        </div>
        ${this.error && this.saveStatus === 'error'
          ? html`
            <error-display
              .augmentedError=${this.error}
              .action=${{ label: 'Dismiss', onClick: () => this._dismissSaveError() }}
            ></error-display>
          `
          : nothing}
        <div class="editor-area">
          <file-drop-zone
            ?allow-uploads=${this.allowUploads}
            max-upload-mb="${this.maxUploadMb}"
            @file-uploaded=${this._onFileUploaded}
          >
            <textarea
              placeholder="Use markdown here."
              autocapitalize="none"
              .value=${live(this.content)}
              @input=${this._onInput}
              @keyup=${this._onKeyup}
              @keydown=${this._onKeydown}
              @select=${this._checkSelection}
              @mouseup=${this._checkSelection}
            ></textarea>
          </file-drop-zone>
        </div>
      </div>
    `;
  }
}

customElements.define('wiki-editor', WikiEditor);

declare global {
  interface HTMLElementTagNameMap {
    'wiki-editor': WikiEditor;
  }
}
