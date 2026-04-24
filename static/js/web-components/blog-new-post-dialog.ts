import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import type { JsonObject } from '@bufbuild/protobuf';
import { sharedStyles, dialogStyles } from './shared-styles.js';
import { PageCreator } from './page-creator.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import { NativeDialogMixin } from './native-dialog-mixin.js';
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
export class BlogNewPostDialog extends NativeDialogMixin(LitElement) {
  static override readonly styles = dialogStyles(css`
    :host {
      display: block;
    }

    dialog {
      padding: 0;
      border: none;
      border-radius: 8px;
      background: var(--color-surface-elevated, white);
      max-width: 700px;
      width: 90%;
      max-height: 90vh;
      flex-direction: column;
      box-shadow: 0 10px 25px rgba(0, 0, 0, 0.3);
      animation: slideIn 0.2s ease-out;
      overflow: hidden;
    }

    dialog[open] {
      display: flex;
    }

    dialog::backdrop {
      background: rgba(0, 0, 0, 0.5);
      animation: fadeIn 0.2s ease-out;
    }

    @media (max-width: 768px) {
      dialog {
        width: 100%;
        max-width: none;
        max-height: none;
        border-radius: 0;
        margin: 0;
      }
    }

    .header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 16px 20px;
      border-bottom: 1px solid var(--color-border-subtle);
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
      color: var(--color-text-secondary);
      padding: 4px 8px;
    }

    .content {
      padding: 20px;
      overflow-y: auto;
      flex: 1;
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

    .form-group input,
    .form-group title-input {
      width: 100%;
      font-size: 1em;
      box-sizing: border-box;
    }

    .form-group input {
      padding: 8px;
      border: 1px solid var(--color-border-default);
      border-radius: 4px;
    }

    .editor-container {
      border: 1px solid var(--color-border-default);
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
      border-top: 1px solid var(--color-border-subtle);
    }

    .btn {
      padding: 8px 16px;
      border-radius: 4px;
      border: 1px solid var(--color-border-default);
      cursor: pointer;
      font-size: 0.9em;
    }

    .btn-primary {
      background: var(--color-action-confirm);
      color: white;
      border-color: var(--color-action-confirm);
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
      color: var(--color-text-secondary);
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
      border: 1px solid var(--color-border-default);
      border-radius: 4px;
      box-sizing: border-box;
      resize: vertical;
      font-family: inherit;
      margin-top: 4px;
    }

    .identifier-preview {
      font-size: 0.8em;
      color: var(--color-text-muted);
      font-family: "Lucida Console", Monaco, monospace;
      margin-top: 4px;
    }
  `);

  @property({ type: String, attribute: 'blog-id' })
  declare blogId: string;

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

  override updated(changed: Map<PropertyKey, unknown>): void {
    super.updated(changed);
    if (changed.has('open')) {
      if (this.open) {
        if (this._pendingFocusRaf !== undefined) {
          cancelAnimationFrame(this._pendingFocusRaf);
        }
        this._pendingFocusRaf = requestAnimationFrame(() => {
          this._pendingFocusRaf = undefined;
          if (this.open) {
            this.shadowRoot?.querySelector<HTMLElement>('title-input')?.focus();
          }
        });
      } else {
        if (this._pendingFocusRaf !== undefined) {
          cancelAnimationFrame(this._pendingFocusRaf);
          this._pendingFocusRaf = undefined;
        }
      }
    }
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._pendingFocusRaf !== undefined) {
      cancelAnimationFrame(this._pendingFocusRaf);
      this._pendingFocusRaf = undefined;
    }
  }

  protected _closeDialog(): void {
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
      this._closeDialog();
    } catch (err) {
      this.error = AugmentErrorService.augmentError(err, 'create blog post');
    } finally {
      this.creating = false;
    }
  }

  override render() {
    return html`
      ${sharedStyles}
      <dialog
        aria-labelledby="blog-new-post-dialog-title"
        @cancel=${this._handleDialogCancel}
        @click=${this._handleDialogClick}
      >
        <div class="header">
          <h2 id="blog-new-post-dialog-title">New Blog Post</h2>
          <button class="close-btn" aria-label="Close" @click=${this._closeDialog}>
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
          <button class="btn btn-cancel" @click=${this._closeDialog} ?disabled=${this.creating}>
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
      </dialog>
    `;
  }
}

customElements.define('blog-new-post-dialog', BlogNewPostDialog);

declare global {
  interface HTMLElementTagNameMap {
    'blog-new-post-dialog': BlogNewPostDialog;
  }
}
