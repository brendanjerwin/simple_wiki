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
import { EditorContextMenuCoordinator } from '../services/editor-context-menu-coordinator.js';
import type { EditorContextMenu } from './editor-context-menu.js';
import type { EditorToolbar } from './editor-toolbar.js';
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
 * and EditorContextMenuCoordinator for formatting.
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
        textarea {
          padding-left: 15%;
          padding-right: 15%;
        }
      }

      @media (min-width: 100em) {
        textarea {
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

  @state()
  declare loading: boolean;

  @state()
  declare saveStatus: SaveStatus;

  @state()
  declare error: AugmentedError | null;

  @state()
  declare content: string;

  /** Stored for future optimistic concurrency control */
  versionHash = '';
  private saveQueue: EditorSaveQueue | null = null;
  private coordinator: EditorContextMenuCoordinator | null = null;

  readonly client = createClient(PageManagementService, getGrpcWebTransport());

  constructor() {
    super();
    this.page = '';
    this.allowUploads = false;
    this.maxUploadMb = 10;
    this.debounceMs = 750;
    this.loading = true;
    this.saveStatus = 'idle';
    this.error = null;
    this.content = '';
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
      await this.updateComplete;
      this.setupSaveQueue();
      this.setupCoordinator();
      this.focusTextarea();
    });
  }

  private teardown(): void {
    this.saveQueue?.destroy();
    this.saveQueue = null;
    this.coordinator?.detach();
    this.coordinator = null;
  }

  private async loadContent(): Promise<void> {
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
        this.saveStatus = status;
        if (status === 'error' && err) {
          this.error = AugmentErrorService.augmentError(err, 'save page');
        } else if (status !== 'error') {
          this.error = null;
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

    const menu = document.querySelector<EditorContextMenu>('editor-context-menu');
    const toolbar = document.querySelector<EditorToolbar>('editor-toolbar');

    if (menu) {
      this.coordinator = new EditorContextMenuCoordinator(
        textarea,
        menu,
        undefined,
        undefined,
        toolbar ?? null
      );
    }
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
        <div class="status-bar ${this.saveStatus !== 'idle' ? 'visible' : ''}">
          <span class="status-indicator ${this.saveStatus}">
            ${this.saveStatus === 'saving'
              ? html`<i class="fa-solid fa-spinner fa-spin"></i>`
              : nothing}
            ${this.statusText}
          </span>
          ${this.error && this.saveStatus === 'error'
            ? html`<span class="status-error">&nbsp;— ${this.error.message}</span>`
            : nothing}
        </div>
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
