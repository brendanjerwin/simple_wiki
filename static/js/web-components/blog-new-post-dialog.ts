import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import type { JsonObject } from '@bufbuild/protobuf';
import { sharedStyles, dialogStyles } from './shared-styles.js';
import { PageCreator } from './page-creator.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import type { WikiEditor } from './wiki-editor.js';
import './wiki-editor.js';
import './error-display.js';
import type { TitleInput } from './title-input.js';
import './title-input.js';

/**
 * BlogNewPostDialog - Modal dialog for creating new blog posts.
 *
 * Collects title, date, and body content, then creates a page
 * with the appropriate blog frontmatter.
 *
 * @fires post-created - Dispatched when a blog post is successfully created.
 */
export class BlogNewPostDialog extends LitElement {
  static override readonly styles = dialogStyles(css`
    :host {
      position: fixed;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      z-index: var(--z-modal);
      display: none;
    }

    :host([open]) {
      display: flex;
      align-items: center;
      justify-content: center;
      animation: fadeIn 0.2s ease-out;
    }

    .backdrop {
      position: fixed;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background: rgba(0, 0, 0, 0.5);
    }

    .dialog {
      background: white;
      max-width: 700px;
      width: 90%;
      max-height: 90vh;
      display: flex;
      flex-direction: column;
      position: relative;
      z-index: 1;
      animation: slideIn 0.2s ease-out;
      border-radius: 8px;
    }

    .header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 16px 20px;
      border-bottom: 1px solid #eee;
    }

    .header h2 {
      margin: 0;
      font-size: 1.2em;
    }

    .close-btn {
      background: none;
      border: none;
      font-size: 1.2em;
      cursor: pointer;
      color: #666;
      padding: 4px 8px;
    }

    .content {
      padding: 20px;
      overflow-y: auto;
      flex: 1;
    }

    .form-group {
      margin-bottom: 12px;
    }

    .form-row {
      display: flex;
      gap: 12px;
      margin-bottom: 12px;
    }

    .form-row .form-group {
      margin-bottom: 0;
    }

    .form-row .form-group:first-child {
      flex: 0 0 auto;
    }

    .form-row .form-group:last-child {
      flex: 1;
    }

    .form-group label {
      display: block;
      font-weight: 600;
      margin-bottom: 4px;
      font-size: 0.9em;
    }

    .form-group input,
    .form-group title-input {
      width: 100%;
      font-size: 1em;
      box-sizing: border-box;
    }

    .form-group input {
      padding: 8px;
      border: 1px solid #ddd;
      border-radius: 4px;
    }

    .editor-container {
      border: 1px solid #ddd;
      border-radius: 4px;
      height: 250px;
      overflow: hidden;
    }

    .editor-container wiki-editor {
      height: 100%;
    }

    .footer {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
      padding: 16px 20px;
      border-top: 1px solid #eee;
    }

    .btn {
      padding: 8px 16px;
      border-radius: 4px;
      border: 1px solid #ddd;
      cursor: pointer;
      font-size: 0.9em;
    }

    .btn-primary {
      background: #337ab7;
      color: white;
      border-color: #337ab7;
    }

    .btn-primary:disabled {
      opacity: 0.6;
      cursor: not-allowed;
    }

    .btn-cancel {
      background: white;
    }

    .summary-toggle {
      background: none;
      border: none;
      cursor: pointer;
      font-size: 0.9em;
      font-weight: 600;
      color: #555;
      padding: 0;
      margin-bottom: 4px;
      display: flex;
      align-items: center;
      gap: 6px;
    }

    .summary-toggle i {
      font-size: 0.75em;
      width: 10px;
    }

    #post-summary {
      width: 100%;
      font-size: 0.9em;
      padding: 8px;
      border: 1px solid #ddd;
      border-radius: 4px;
      box-sizing: border-box;
      resize: vertical;
      font-family: inherit;
      margin-top: 4px;
    }

    .identifier-preview {
      font-size: 0.8em;
      color: #999;
      font-family: "Lucida Console", Monaco, monospace;
      margin-top: 4px;
    }

    @keyframes fadeIn {
      from { opacity: 0; }
      to { opacity: 1; }
    }

    @keyframes slideIn {
      from { transform: translateY(-20px); opacity: 0; }
      to { transform: translateY(0); opacity: 1; }
    }
  `);

  @property({ type: String, attribute: 'blog-id' })
  declare blogId: string;

  @property({ type: Boolean, reflect: true })
  declare open: boolean;

  @state()
  declare title: string;

  @state()
  declare date: string;

  @state()
  declare creating: boolean;

  @state()
  declare error: AugmentedError | null;

  @state()
  declare summaryExpanded: boolean;

  @state()
  declare summary: string;

  @state()
  declare subtitle: string;

  private readonly pageCreator = new PageCreator();

  private _keydownController: AbortController | undefined = undefined;

  private _pendingFocusRaf: number | undefined = undefined;

  private get identifierPreview(): string {
    if (!this.blogId || !this.date || !this.title.trim()) return '';
    const slug = this.title.trim()
      .toLowerCase()
      .replaceAll(/[^a-z0-9\s-]/g, '')
      .replaceAll(/\s+/g, '-')
      .replaceAll(/-+/g, '-');
    return `${this.blogId}-${this.date}-${slug}`;
  }

  constructor() {
    super();
    this.blogId = '';
    this.open = false;
    this.title = '';
    this.date = new Date().toISOString().slice(0, 10);
    this.creating = false;
    this.error = null;
    this.summaryExpanded = false;
    this.summary = '';
    this.subtitle = '';
  }

  private _onTitleInput(): void {
    const titleInput = this.shadowRoot?.querySelector<TitleInput>('title-input');
    if (titleInput) {
      this.title = titleInput.value;
    }
  }

  private _toggleSummary(): void {
    this.summaryExpanded = !this.summaryExpanded;
  }

  private _onSummaryInput(e: Event): void {
    if (e.target instanceof HTMLTextAreaElement) {
      this.summary = e.target.value;
    }
  }

  private _onSubtitleInput(e: Event): void {
    if (e.target instanceof HTMLInputElement) {
      this.subtitle = e.target.value;
    }
  }

  private _onDateInput(e: Event): void {
    if (e.target instanceof HTMLInputElement) {
      this.date = e.target.value;
    }
  }

  override updated(changed: Map<string, unknown>): void {
    if (changed.has('open') && this.open) {
      // Cancel any pending focus RAF before starting a new one to prevent
      // race conditions or focusing elements in a closed dialog.
      if (this._pendingFocusRaf !== undefined) {
        cancelAnimationFrame(this._pendingFocusRaf);
      }
      // Focus the title input when dialog opens
      this._pendingFocusRaf = requestAnimationFrame(() => {
        this._pendingFocusRaf = undefined;
        if (this.open) {
          this.shadowRoot?.querySelector<HTMLElement>('title-input')?.focus();
        }
      });
    }
  }

  override connectedCallback(): void {
    super.connectedCallback();
    this._keydownController = new AbortController();
    document.addEventListener('keydown', (e: KeyboardEvent) => {
      if (e.key === 'Escape' && this.open) {
        this._close();
      }
    }, { signal: this._keydownController.signal });
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._pendingFocusRaf !== undefined) {
      cancelAnimationFrame(this._pendingFocusRaf);
      this._pendingFocusRaf = undefined;
    }
    this._keydownController?.abort();
    this._keydownController = undefined;
  }

  private _close(): void {
    this.open = false;
    this.title = '';
    this.date = new Date().toISOString().slice(0, 10);
    this.error = null;
    this.creating = false;
    this.summaryExpanded = false;
    this.summary = '';
    this.subtitle = '';
  }

  private async _submit(): Promise<void> {
    if (!this.title.trim() || !this.date) return;

    this.creating = true;
    this.error = null;

    try {
      // Generate identifier: blog-id-date-title-slug
      const slugText = `${this.blogId}-${this.date}-${this.title}`;
      const idResult = await this.pageCreator.generateIdentifier(slugText, true);
      if (idResult.error) {
        this.error = AugmentErrorService.augmentError(idResult.error, 'generate identifier');
        return;
      }

      // Get editor content
      const editor = this.shadowRoot?.querySelector<WikiEditor>('wiki-editor');
      const bodyContent = editor?.getContent() || '';

      // Build frontmatter
      const blogMeta: JsonObject = {
        identifier: this.blogId,
        'published-date': this.date,
      };
      if (this.subtitle.trim()) {
        blogMeta['subtitle'] = this.subtitle.trim();
      }
      if (this.summary.trim()) {
        blogMeta['summary_markdown'] = this.summary.trim();
      }
      const frontmatter: JsonObject = {
        title: this.title,
        blog: blogMeta,
      };

      // Create the page
      const result = await this.pageCreator.createPage(
        idResult.identifier,
        bodyContent,
        undefined,
        frontmatter
      );

      if (result.error) {
        this.error = AugmentErrorService.augmentError(result.error, 'create blog post');
        return;
      }

      this.pageCreator.showSuccess(`Blog post "${this.title}" created!`);
      this.dispatchEvent(new CustomEvent('post-created', {
        bubbles: true,
        composed: true,
        detail: { identifier: idResult.identifier, title: this.title },
      }));
      this._close();
    } catch (err) {
      this.error = AugmentErrorService.augmentError(err, 'create blog post');
    } finally {
      this.creating = false;
    }
  }

  override render() {
    if (!this.open) return nothing;

    return html`
      ${sharedStyles}
      <div class="backdrop" @click=${this._close}></div>
      <div class="dialog">
        <div class="header">
          <h2>New Blog Post</h2>
          <button class="close-btn" aria-label="Close" @click=${this._close}>
            <i class="fa-solid fa-xmark"></i>
          </button>
        </div>
        <div class="content">
          ${this.error
            ? html`<error-display .augmentedError="${this.error}"></error-display>`
            : nothing}
          <div class="form-group">
            <label @click=${() => this.shadowRoot?.querySelector<HTMLElement>('title-input')?.focus()}>Title</label>
            <title-input
              id="post-title"
              .value=${this.title}
              @input=${this._onTitleInput}
              placeholder="Enter post title"
              ?disabled=${this.creating}
            ></title-input>
          </div>
          <div class="form-row">
            <div class="form-group">
              <label for="post-date">Date</label>
              <input
                id="post-date"
                type="date"
                .value=${this.date}
                @input=${this._onDateInput}
                ?disabled=${this.creating}
              />
            </div>
            <div class="form-group">
              <label for="post-subtitle">Subtitle</label>
              <input
                id="post-subtitle"
                type="text"
                placeholder="Optional subtitle"
                .value=${this.subtitle}
                @input=${this._onSubtitleInput}
                ?disabled=${this.creating}
              />
            </div>
          </div>
          ${this.identifierPreview
            ? html`<div class="identifier-preview">${this.identifierPreview}</div>`
            : nothing}
          <div class="form-group">
            <button class="summary-toggle" @click=${this._toggleSummary} type="button">
              <i class="fa-solid fa-chevron-${this.summaryExpanded ? 'down' : 'right'}"></i>
              Summary
            </button>
            ${this.summaryExpanded
              ? html`<textarea
                  id="post-summary"
                  rows="3"
                  placeholder="Optional summary shown in the blog list"
                  .value=${this.summary}
                  @input=${this._onSummaryInput}
                  ?disabled=${this.creating}
                ></textarea>`
              : nothing}
          </div>
          <div class="form-group">
            <label>Body</label>
            <div class="editor-container">
              <wiki-editor
                initial-content=""
                .autoSave=${false}
                compact
              ></wiki-editor>
            </div>
          </div>
        </div>
        <div class="footer">
          <button class="btn btn-cancel" @click=${this._close} ?disabled=${this.creating}>
            Cancel
          </button>
          <button
            class="btn btn-primary"
            @click=${this._submit}
            ?disabled=${this.creating || !this.title.trim() || !this.date}
          >
            ${this.creating ? html`<i class="fa-solid fa-spinner fa-spin"></i> Creating...` : 'Create Post'}
          </button>
        </div>
      </div>
    `;
  }
}

customElements.define('blog-new-post-dialog', BlogNewPostDialog);

declare global {
  interface HTMLElementTagNameMap {
    'blog-new-post-dialog': BlogNewPostDialog;
  }
}
