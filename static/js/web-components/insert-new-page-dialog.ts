import { html, css, LitElement, nothing } from 'lit';
import type { JsonObject } from '@bufbuild/protobuf';
import { sharedStyles, dialogStyles } from './shared-styles.js';
import { PageCreator } from './page-creator.js';
import type { TemplateInfo } from '../gen/api/v1/page_management_pb.js';
import type { AutomagicIdentifierInput, GenerateIdentifierResult } from './automagic-identifier-input.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import { SYSTEM_TEMPLATE_IDENTIFIERS } from './system-templates.js';
import type {
  TitleChangeEventDetail,
  IdentifierChangeEventDetail,
  SectionChangeEventDetail,
  PageCreatedEventDetail,
} from './event-types.js';
import './confirmation-interlock-button.js';
import './frontmatter-value-section.js';
import './error-display.js';

const NONE_TEMPLATE_VALUE = '';

/**
 * InsertNewPageDialog - Modal dialog for creating new wiki pages
 *
 * Features:
 * - Title input with auto-generated identifier (automagic mode)
 * - Template dropdown that populates frontmatter when selected
 * - Embedded frontmatter editor
 * - Dropdown locks when template selected or frontmatter edited
 * - Inline confirmation to clear frontmatter when changing template
 *
 * @fires page-created - Dispatched when page is created. Detail: { identifier, title, markdownLink }
 */
export class InsertNewPageDialog extends LitElement {
  static override styles = dialogStyles(css`
    :host {
      position: fixed;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      z-index: 9999;
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
      max-width: 600px;
      width: 90%;
      max-height: 90vh;
      display: flex;
      flex-direction: column;
      position: relative;
      z-index: 1;
      animation: slideIn 0.2s ease-out;
      border-radius: 8px;
    }

    .content {
      padding: 20px;
      overflow-y: auto;
      flex: 1;
    }

    .error-message {
      background: #fef2f2;
      border: 1px solid #fecaca;
      color: #dc2626;
      padding: 12px;
      border-radius: 4px;
      margin-bottom: 16px;
      font-size: 14px;
    }

    .template-row {
      display: flex;
      gap: 12px;
      align-items: center;
    }

    .template-row select {
      flex: 1;
      padding: 10px 12px;
      border: 1px solid #ddd;
      border-radius: 4px;
      font-size: 14px;
      font-family: inherit;
      background: white;
    }

    .template-row select:disabled {
      background: #f5f5f5;
      color: #666;
    }

    .template-note {
      font-size: 12px;
      color: #666;
      margin-top: 4px;
      font-style: italic;
    }

    .frontmatter-section {
      margin-top: 16px;
      border: 1px solid #e5e7eb;
      border-radius: 4px;
      padding: 12px;
      background: #f9fafb;
    }

    .frontmatter-header {
      font-size: 14px;
      font-weight: 500;
      color: #374151;
      margin-bottom: 8px;
    }

    .frontmatter-empty {
      color: #6b7280;
      font-style: italic;
      font-size: 13px;
      padding: 8px;
      text-align: center;
    }

    .footer {
      display: flex;
      gap: 12px;
      padding: 16px 20px;
      border-top: 1px solid #e0e0e0;
      justify-content: flex-end;
    }
  `);

  static override properties = {
    open: { type: Boolean, reflect: true },
    pageTitle: { type: String },
    pageIdentifier: { type: String },
    isUnique: { state: true },
    loading: { state: true },
    templatesLoading: { state: true },
    error: { state: true },
    templates: { state: true },
    selectedTemplate: { state: true },
    templateLocked: { state: true },
    frontmatter: { state: true },
    frontmatterDirty: { state: true },
  };

  declare open: boolean;
  declare pageTitle: string;
  declare pageIdentifier: string;
  declare isUnique: boolean;
  declare loading: boolean;
  declare templatesLoading: boolean;
  declare error: AugmentedError | null;
  declare templates: TemplateInfo[];
  declare selectedTemplate: string;
  declare templateLocked: boolean;
  declare frontmatter: JsonObject;
  declare frontmatterDirty: boolean;

  private pageCreator = new PageCreator();

  constructor() {
    super();
    this.open = false;
    this.pageTitle = '';
    this.pageIdentifier = '';
    this.isUnique = true;
    this.loading = false;
    this.templatesLoading = false;
    this.error = null;
    this.templates = [];
    this.selectedTemplate = NONE_TEMPLATE_VALUE;
    this.templateLocked = false;
    this.frontmatter = {};
    this.frontmatterDirty = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('keydown', this._handleKeydown);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('keydown', this._handleKeydown);
  }

  private _handleKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape' && this.open) {
      this.close();
    }
  };

  public async openDialog(): Promise<void> {
    this._resetState();
    this.open = true;
    await this._loadTemplates();

    // Focus the title input after render (use optional chaining for method in case element not upgraded)
    this.updateComplete.then(() => {
      const identifierInput = this.shadowRoot?.querySelector<AutomagicIdentifierInput>('automagic-identifier-input');
      identifierInput?.focusTitleInput?.();
    });
  }

  public close(): void {
    this.open = false;
    this._resetState();
  }

  private _resetState(): void {
    this.pageTitle = '';
    this.pageIdentifier = '';
    this.isUnique = true;
    this.loading = false;
    this.error = null;
    this.selectedTemplate = NONE_TEMPLATE_VALUE;
    this.templateLocked = false;
    this.frontmatter = {};
    this.frontmatterDirty = false;
  }

  private async _loadTemplates(): Promise<void> {
    this.templatesLoading = true;
    const result = await this.pageCreator.listTemplates([...SYSTEM_TEMPLATE_IDENTIFIERS]);
    this.templatesLoading = false;

    if (result.error) {
      this.error = AugmentErrorService.augmentError(result.error, 'load templates');
      this.templates = [];
    } else {
      this.templates = result.templates;
    }
  }

  private _handleBackdropClick = (): void => {
    this.close();
  };

  private _handleDialogClick = (event: Event): void => {
    event.stopPropagation();
  };

  /**
   * Adapter function to call PageCreator.generateIdentifier
   * in the format expected by AutomagicIdentifierInput.
   */
  private _generateIdentifier = async (text: string): Promise<GenerateIdentifierResult> => {
    const result = await this.pageCreator.generateIdentifier(text);
    const generateResult: GenerateIdentifierResult = {
      identifier: result.identifier,
      isUnique: result.isUnique,
    };
    if (result.existingPage) {
      generateResult.existingPage = result.existingPage;
    }
    if (result.error) {
      generateResult.error = result.error;
    }
    return generateResult;
  };

  private _handleTitleChange = (event: CustomEvent<TitleChangeEventDetail>): void => {
    this.pageTitle = event.detail.title;
  };

  private _handleIdentifierChange = (event: CustomEvent<IdentifierChangeEventDetail>): void => {
    this.pageIdentifier = event.detail.identifier;
    this.isUnique = event.detail.isUnique;
  };

  private _handleTemplateChange = async (event: Event): Promise<void> => {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
    const select = event.target as HTMLSelectElement;
    const newTemplate = select.value;

    this.selectedTemplate = newTemplate;

    if (newTemplate === NONE_TEMPLATE_VALUE) {
      // Cleared template selection - clear frontmatter
      this.frontmatter = {};
      this.templateLocked = false;
      this.frontmatterDirty = false;
    } else {
      // Template selected - load its frontmatter
      const success = await this._loadTemplateFrontmatter(newTemplate);
      if (success) {
        this.templateLocked = true;
      }
    }
  };

  private async _loadTemplateFrontmatter(templateIdentifier: string): Promise<boolean> {
    const result = await this.pageCreator.getTemplateFrontmatter(templateIdentifier);

    if (result.error) {
      this.error = AugmentErrorService.augmentError(result.error, 'load template frontmatter');
      this.frontmatter = {};
      // Unlock dropdown so user can select a different template
      this.templateLocked = false;
      this.selectedTemplate = '';
      return false;
    }

    // Filter out system keys - these are set automatically
    const systemKeys = ['template', 'identifier'];
    this.frontmatter = Object.fromEntries(
      Object.entries(result.frontmatter).filter(([key]) => !systemKeys.includes(key))
    );
    return true;
  }

  private _handleFrontmatterChange = (event: CustomEvent<SectionChangeEventDetail>): void => {
    this.frontmatter = event.detail.newFields;
    this.frontmatterDirty = true;

    // Lock the dropdown if frontmatter was edited (even with no template)
    if (!this.templateLocked && Object.keys(this.frontmatter).length > 0) {
      this.templateLocked = true;
    }
  };

  private _handleChangeTemplateConfirmed = (): void => {
    // Clear frontmatter and unlock dropdown
    this.frontmatter = {};
    this.frontmatterDirty = false;
    this.templateLocked = false;
    this.selectedTemplate = NONE_TEMPLATE_VALUE;
  };

  private _handleCancel = (): void => {
    this.close();
  };

  private get _canSubmit(): boolean {
    return (
      this.pageIdentifier.trim().length > 0 &&
      this.isUnique &&
      !this.loading
    );
  }

  private _handleSubmit = async (): Promise<void> => {
    if (!this._canSubmit) return;

    this.loading = true;
    this.error = null;

    const identifier = this.pageIdentifier.trim();
    const template = this.selectedTemplate || undefined;

    const result = await this.pageCreator.createPage(
      identifier,
      '', // Empty content - just creating the page
      template,
      this.frontmatter
    );

    this.loading = false;

    if (result.success) {
      const title = this.pageTitle.trim() || identifier;
      const markdownLink = `[${title}](/${identifier})`;

      this.dispatchEvent(
        new CustomEvent<PageCreatedEventDetail>('page-created', {
          detail: { identifier, title, markdownLink } satisfies PageCreatedEventDetail,
          bubbles: true,
          composed: true,
        })
      );

      this.pageCreator.showSuccess(`Created page: ${identifier}`);
      this.close();
    } else {
      // INVARIANT ASSERTION: If createPage returns success=false, it must provide an error.
      // This indicates a programming bug in PageCreator.createPage, not a runtime error.
      if (!result.error) {
        throw new Error('PageCreator.createPage returned success=false without an error');
      }
      this.error = AugmentErrorService.augmentError(result.error, 'create page');
    }
  };

  private _renderTemplateSelector() {
    const hasTemplates = this.templates.length > 0;
    const isDisabled = this.templatesLoading || this.templateLocked || !hasTemplates;

    return html`
      <div class="form-group">
        <label for="template">Template (optional)</label>
        <div class="template-row">
          <select
            id="template"
            name="template"
            .value=${this.selectedTemplate}
            @change=${this._handleTemplateChange}
            ?disabled=${isDisabled}
          >
            <option value="">
              ${this.templatesLoading
                ? 'Loading templates...'
                : !hasTemplates
                  ? '(none - no templates defined)'
                  : '(none)'}
            </option>
            ${this.templates.map(
              template => html`
                <option value="${template.identifier}">
                  ${template.title || template.identifier}
                </option>
              `
            )}
          </select>
          ${this.templateLocked
            ? html`
                <confirmation-interlock-button
                  label="Change"
                  confirmLabel="Clear frontmatter?"
                  yesLabel="Clear"
                  noLabel="Keep"
                  .disarmTimeoutMs=${0}
                  @confirmed=${this._handleChangeTemplateConfirmed}
                ></confirmation-interlock-button>
              `
            : nothing}
        </div>
        ${this.templateLocked && this.selectedTemplate
          ? html`<div class="template-note">Template applied. Click "Change" to select a different template.</div>`
          : nothing}
      </div>
    `;
  }

  private _renderFrontmatterEditor() {
    const isEmpty = Object.keys(this.frontmatter).length === 0;

    return html`
      <div class="frontmatter-section">
        <div class="frontmatter-header">
          Frontmatter ${this.selectedTemplate ? '(from template)' : ''}
        </div>
        ${isEmpty
          ? html`<div class="frontmatter-empty">No frontmatter fields. Use "Add Field" to add metadata.</div>`
          : nothing}
        <frontmatter-value-section
          .fields=${this.frontmatter}
          .isRoot=${true}
          @section-change=${this._handleFrontmatterChange}
        ></frontmatter-value-section>
      </div>
    `;
  }

  override render() {
    return html`
      ${sharedStyles}
      <div class="backdrop" @click=${this._handleBackdropClick}></div>
      <div class="dialog system-font border-radius box-shadow" @click=${this._handleDialogClick}>
        <div class="dialog-header">
          <h2 class="dialog-title">Insert New Page</h2>
        </div>

        <div class="content">
          ${this.error
            ? html`<error-display .augmentedError=${this.error}></error-display>`
            : nothing}

          <automagic-identifier-input
            .title=${this.pageTitle}
            .identifier=${this.pageIdentifier}
            .generateIdentifier=${this._generateIdentifier}
            .disabled=${this.loading}
            titlePlaceholder="e.g., My New Article"
            titleHelpText="Title is optional - used for link text and auto-generating identifier"
            @title-change=${this._handleTitleChange}
            @identifier-change=${this._handleIdentifierChange}
          ></automagic-identifier-input>

          ${this._renderTemplateSelector()}
          ${this._renderFrontmatterEditor()}
        </div>

        <div class="footer">
          <button
            class="button-base button-secondary button-large border-radius-small"
            @click=${this._handleCancel}
            ?disabled=${this.loading}
          >
            Cancel
          </button>
          <button
            class="button-base button-primary button-large border-radius-small"
            @click=${this._handleSubmit}
            ?disabled=${!this._canSubmit}
          >
            ${this.loading ? 'Creating...' : 'Create Page'}
          </button>
        </div>
      </div>
    `;
  }
}

customElements.define('insert-new-page-dialog', InsertNewPageDialog);

declare global {
  interface HTMLElementTagNameMap {
    'insert-new-page-dialog': InsertNewPageDialog;
  }
}
