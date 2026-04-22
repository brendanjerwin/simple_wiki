import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { sharedStyles, dialogStyles } from './shared-styles.js';
import { InventoryItemCreatorMover } from './inventory-item-creator-mover.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import { SearchService, SearchContentRequestSchema, type SearchResult } from '../gen/api/v1/search_pb.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import './error-display.js';
import type { TitleChangeEventDetail, IdentifierChangeEventDetail } from './event-types.js';
import './automagic-identifier-input.js';
import type { AutomagicIdentifierInput, GenerateIdentifierResult } from './automagic-identifier-input.js';

/**
 * InventoryAddItemDialog - Modal dialog for adding new inventory items
 *
 * Title-first workflow: user enters a title, identifier is auto-generated
 * via server call (automagic mode). User can click the sparkle button to
 * switch to manual mode for editing the identifier. Also includes Description
 * field and inline search results to help find existing items.
 */
export class InventoryAddItemDialog extends LitElement {
  static override readonly styles = dialogStyles(css`
    dialog {
      padding: 0;
      border: none;
      border-radius: 8px;
      background: var(--color-surface-elevated);
      max-width: 500px;
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
        max-height: 100%;
        border-radius: 0;
        margin: 0;
      }
    }

    .content {
      padding: 20px;
      overflow-y: auto;
      flex: 1;
    }

    .search-results {
      margin-top: 16px;
      border: 1px solid var(--color-border-subtle);
      border-radius: 4px;
      max-height: 200px;
      overflow-y: auto;
    }

    .search-results-header {
      padding: 8px 12px;
      background: var(--color-surface-sunken);
      border-bottom: 1px solid var(--color-border-subtle);
      font-size: 12px;
      font-weight: 500;
      color: var(--color-text-secondary);
    }

    .search-result-item {
      display: block;
      padding: 10px 12px;
      border-bottom: 1px solid var(--color-border-subtle);
      text-decoration: none;
      color: inherit;
      cursor: pointer;
      transition: background-color 0.15s;
    }

    .search-result-item:last-child {
      border-bottom: none;
    }

    .search-result-item:hover {
      background: var(--color-surface-sunken);
    }

    .search-result-title {
      font-weight: 500;
      color: var(--color-text-primary);
      margin-bottom: 2px;
    }

    .search-result-container {
      font-size: 12px;
      color: var(--color-text-secondary);
    }

    .search-result-container a {
      color: var(--color-text-link);
    }

    .form-group textarea {
      min-height: 50px;
    }

    .footer {
      display: flex;
      gap: 12px;
      padding: 16px 20px;
      border-top: 1px solid var(--color-border-subtle);
      justify-content: flex-end;
    }
  `);

  @property({ type: Boolean, reflect: true })
  declare open: boolean;

  @property({ type: String })
  declare container: string;

  @property({ type: String })
  declare itemTitle: string;

  @property({ type: String })
  declare itemIdentifier: string;

  @property({ type: String })
  declare description: string;

  @state()
  declare isUnique: boolean;

  @state()
  declare loading: boolean;

  @state()
  declare error: AugmentedError | null;

  @state()
  declare searchResults: SearchResult[];

  @state()
  declare searchLoading: boolean;

  private _searchDebounceTimer?: ReturnType<typeof setTimeout>;
  private readonly _debounceTimeoutMs = 300;
  private readonly searchClient = createClient(SearchService, getGrpcWebTransport());
  private readonly inventoryItemCreatorMover = new InventoryItemCreatorMover();
  private _previouslyFocusedElement: Element | null = null;

  constructor() {
    super();
    this.open = false;
    this.container = '';
    this.itemTitle = '';
    this.itemIdentifier = '';
    this.description = '';
    this.isUnique = true;
    this.loading = false;
    this.error = null;
    this.searchResults = [];
    this.searchLoading = false;
    this._previouslyFocusedElement = null;
  }

  override updated(changedProperties: Map<PropertyKey, unknown>): void {
    super.updated(changedProperties);
    if (changedProperties.has('open')) {
      const dialog = this.shadowRoot?.querySelector('dialog');
      if (!dialog) return;
      if (this.open && !dialog.open && this.isConnected) {
        this._previouslyFocusedElement = document.activeElement;
        dialog.showModal();
      } else if (!this.open && dialog.open) {
        dialog.close();
        if (this._previouslyFocusedElement instanceof HTMLElement) {
          this._previouslyFocusedElement.focus();
        }
        this._previouslyFocusedElement = null;
      }
    }
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this._clearSearchDebounceTimer();
  }

  private _clearSearchDebounceTimer(): void {
    if (this._searchDebounceTimer) {
      clearTimeout(this._searchDebounceTimer);
      delete this._searchDebounceTimer;
    }
  }

  private readonly _handleDialogCancel = (event: Event): void => {
    event.preventDefault();
    this._handleCancel();
  };

  private readonly _handleDialogClick = (event: MouseEvent): void => {
    if (event.target === event.currentTarget) {
      this._handleCancel();
    }
  };

  public openDialog(container: string): void {
    this.container = container;
    this.itemTitle = '';
    this.itemIdentifier = '';
    this.description = '';
    this.isUnique = true;
    this.error = null;
    this.loading = false;
    this.searchResults = [];
    this.searchLoading = false;
    this.open = true;

    // Reset and focus the automagic identifier input after render
    this.updateComplete.then(() => {
      const identifierInput = this.shadowRoot?.querySelector<AutomagicIdentifierInput>('automagic-identifier-input');
      identifierInput?.reset();
      identifierInput?.focusTitleInput();
    });
  }

  public close(): void {
    this.open = false;
    this._clearSearchDebounceTimer();
    this.itemTitle = '';
    this.itemIdentifier = '';
    this.description = '';
    this.isUnique = true;
    this.error = null;
    this.loading = false;
    this.searchResults = [];
    this.searchLoading = false;
  }

  /**
   * Adapter function to call InventoryItemCreatorMover.generateIdentifier
   * in the format expected by AutomagicIdentifierInput.
   */
  private readonly _generateIdentifier = async (text: string): Promise<GenerateIdentifierResult> => {
    const result = await this.inventoryItemCreatorMover.generateIdentifier(text);
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

  private readonly _handleTitleChange = (event: CustomEvent<TitleChangeEventDetail>): void => {
    this.itemTitle = event.detail.title;

    // Debounce search
    if (this._searchDebounceTimer) {
      clearTimeout(this._searchDebounceTimer);
    }

    const title = this.itemTitle.trim();
    if (!title) {
      this.searchResults = [];
      return;
    }

    this._searchDebounceTimer = setTimeout(() => {
      this._performSearch(title);
    }, this._debounceTimeoutMs);
  };

  private readonly _handleIdentifierChange = (event: CustomEvent<IdentifierChangeEventDetail>): void => {
    this.itemIdentifier = event.detail.identifier;
    this.isUnique = event.detail.isUnique;
  };

  private readonly _handleDescriptionInput = (event: Event): void => {
    if (!(event.target instanceof HTMLTextAreaElement)) {
      return;
    }
    const input = event.target;
    this.description = input.value;
  };

  private async _performSearch(query: string): Promise<void> {
    if (!query) {
      this.searchResults = [];
      return;
    }

    this.searchLoading = true;

    try {
      const request = create(SearchContentRequestSchema, {
        query,
        frontmatterKeyIncludeFilters: ['inventory.container'],
        frontmatterKeyExcludeFilters: ['inventory.is_container'],
        frontmatterKeysToReturnInResults: ['inventory.container'],
      });

      const response = await this.searchClient.searchContent(request);
      this.searchResults = response.results;
    } catch (err) {
      this.searchResults = [];
      this.error = AugmentErrorService.augmentError(err, 'search containers');
    } finally {
      this.searchLoading = false;
    }
  }

  private readonly _handleCancel = (): void => {
    this.close();
  };

  private get canSubmit(): boolean {
    return (
      this.itemTitle.trim().length > 0 &&
      this.itemIdentifier.trim().length > 0 &&
      this.isUnique &&
      !this.loading
    );
  }

  private readonly _handleSubmit = async (): Promise<void> => {
    if (!this.canSubmit) return;

    this.loading = true;
    this.error = null;

    const result = await this.inventoryItemCreatorMover.addItem(
      this.container,
      this.itemIdentifier.trim(),
      this.itemTitle.trim(),
      this.description.trim() || undefined
    );

    this.loading = false;

    if (result.success) {
      this.inventoryItemCreatorMover.showSuccess(
        result.summary || `Added ${this.itemTitle} to ${this.container}`,
        () => globalThis.location.reload()
      );
      this.close();
    } else {
      if (!result.error) {
        throw new Error('InventoryItemCreatorMover.addItem returned success=false without an error');
      }
      this.error = AugmentErrorService.augmentError(result.error, 'create item');
    }
  };

  private _renderSearchResults() {
    if (this.searchResults.length === 0 && !this.searchLoading) {
      return nothing;
    }

    const resultCountSuffix = this.searchResults.length === 1 ? '' : 's';

    return html`
      <div class="search-results">
        <div class="search-results-header">
          ${this.searchLoading
            ? 'Searching...'
            : `${this.searchResults.length} similar item${resultCountSuffix} found`}
        </div>
        ${this.searchResults.map(
          result => html`
            <a class="search-result-item" href="/${result.identifier}">
              <div class="search-result-title">${result.title || result.identifier}</div>
              ${result.frontmatter?.['inventory.container']
                ? html`<div class="search-result-container">Found In: ${result.frontmatter['inventory.container']}</div>`
                : ''}
            </a>
          `
        )}
      </div>
    `;
  }

  override render() {
    return html`
      ${sharedStyles}
      <dialog
        aria-labelledby="add-item-dialog-title"
        @cancel="${this._handleDialogCancel}"
        @click="${this._handleDialogClick}"
      >
        <div class="dialog-header system-font">
          <h2 id="add-item-dialog-title" class="dialog-title">Add Item to: ${this.container}</h2>
        </div>

        <div class="content">
          ${this.error
            ? html`<error-display
                .augmentedError=${this.error}
                .action=${{ label: 'Dismiss', onClick: () => { this.error = null; } }}
              ></error-display>`
            : ''}

          <automagic-identifier-input
            .title=${this.itemTitle}
            .identifier=${this.itemIdentifier}
            .generateIdentifier=${this._generateIdentifier}
            .disabled=${this.loading}
            titlePlaceholder="e.g., Phillips Head Screwdriver"
            titleHelpText="Human-readable name for the item (required)"
            @title-change=${this._handleTitleChange}
            @identifier-change=${this._handleIdentifierChange}
          ></automagic-identifier-input>

          <div class="form-group">
            <label for="description">Description (optional)</label>
            <textarea
              id="description"
              name="description"
              .value=${this.description}
              @input=${this._handleDescriptionInput}
              placeholder="Optional description of the item"
              ?disabled=${this.loading}
            ></textarea>
          </div>

          ${this._renderSearchResults()}
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
            ?disabled=${!this.canSubmit}
          >
            ${this.loading ? 'Adding...' : 'Add Item'}
          </button>
        </div>
      </dialog>
    `;
  }
}

customElements.define('inventory-add-item-dialog', InventoryAddItemDialog);

declare global {
  interface HTMLElementTagNameMap {
    'inventory-add-item-dialog': InventoryAddItemDialog;
  }
}
