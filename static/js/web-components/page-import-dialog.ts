import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  PageImportService,
  ParseCSVPreviewRequestSchema,
  StartPageImportJobRequestSchema,
  ArrayOpType,
  type PageImportRecord,
} from '../gen/api/v1/page_import_pb.js';
import {
  SystemInfoService,
  StreamJobStatusRequestSchema,
  type GetJobStatusResponse,
} from '../gen/api/v1/system_info_pb.js';
import {
  sharedStyles,
  foundationCSS,
  dialogCSS,
  responsiveCSS,
  buttonCSS,
} from './shared-styles.js';
import './error-display.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import { flattenFrontmatter } from '../page-import/flatten-frontmatter.js';

type DialogState = 'upload' | 'validating' | 'preview' | 'importing' | 'complete';

interface ImportStats {
  total: number;
  errors: number;
  updates: number;
  creates: number;
}

/**
 * PageImportDialog - Modal dialog for importing pages from CSV
 *
 * Provides a multi-step workflow:
 * 1. Upload: File upload via drag-drop or file picker
 * 2. Validating: Loading state while parsing CSV
 * 3. Preview: Review records with navigation and error filtering
 * 4. Importing: Loading state while import job runs
 * 5. Complete: Summary with link to report page
 */
export class PageImportDialog extends LitElement {
  private static readonly PAGE_IMPORT_QUEUE_NAME = 'PageImportJob';

  static override styles = [
    foundationCSS,
    dialogCSS,
    responsiveCSS,
    buttonCSS,
    css`
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

      @keyframes fadeIn {
        from {
          opacity: 0;
        }
        to {
          opacity: 1;
        }
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
        max-height: 80vh;
        display: flex;
        flex-direction: column;
        position: relative;
        z-index: 1;
        animation: slideIn 0.2s ease-out;
        border-radius: 8px;
      }

      @keyframes slideIn {
        from {
          transform: translateY(-20px);
          opacity: 0;
        }
        to {
          transform: translateY(0);
          opacity: 1;
        }
      }

      /* Mobile-first responsive behavior */
      @media (max-width: 768px) {
        :host([open]) {
          align-items: stretch;
          justify-content: stretch;
        }

        .dialog {
          width: 100%;
          height: 100%;
          max-width: none;
          max-height: none;
          border-radius: 0;
          margin: 0;
        }
      }

      .content {
        flex: 1;
        padding: 20px;
        overflow-y: auto;
        min-height: 200px;
      }

      .footer {
        display: flex;
        gap: 12px;
        padding: 16px 20px;
        border-top: 1px solid #e0e0e0;
        justify-content: flex-end;
      }

      /* Upload State Styles */
      .drop-zone {
        border: 2px dashed #ccc;
        border-radius: 8px;
        padding: 40px;
        text-align: center;
        transition: all 0.2s ease;
        cursor: pointer;
      }

      .drop-zone.drag-over {
        border-color: #4a90d9;
        background: rgba(74, 144, 217, 0.05);
      }

      .drop-zone-icon {
        font-size: 48px;
        color: #999;
        margin-bottom: 16px;
      }

      .drop-zone-text {
        color: #666;
        margin-bottom: 8px;
      }

      .drop-zone-hint {
        font-size: 12px;
        color: #999;
      }

      /* Hide drag-drop zone on touch devices */
      @media (pointer: coarse) {
        .drop-zone {
          display: none;
        }
      }

      .file-input-wrapper {
        margin-top: 16px;
      }

      /* Hide the separate button on desktop (when drop zone is visible) */
      @media (pointer: fine) {
        .file-input-wrapper .button-base {
          display: none;
        }
      }

      .file-input {
        display: none;
      }

      /* Loading State Styles */
      .loading-container {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 40px;
        gap: 16px;
      }

      .loading-spinner {
        font-size: 32px;
        color: #4a90d9;
      }

      .loading-text {
        color: #666;
        font-size: 16px;
      }

      /* Preview State Styles */
      .summary-bar {
        display: flex;
        gap: 16px;
        padding: 12px 16px;
        background: #f8f9fa;
        border: 1px solid #e9ecef;
        border-radius: 4px;
        margin-bottom: 16px;
        flex-wrap: wrap;
      }

      .summary-item {
        font-size: 14px;
        color: #333;
      }

      .summary-item.errors {
        color: #dc3545;
        font-weight: 500;
      }

      .summary-item.creates {
        color: #28a745;
      }

      .summary-item.updates {
        color: #4a90d9;
      }

      .filter-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 16px;
        flex-wrap: wrap;
        gap: 12px;
      }

      .checkbox-label {
        display: flex;
        align-items: center;
        gap: 8px;
        cursor: pointer;
        font-size: 14px;
        color: #333;
      }

      .navigation {
        display: flex;
        align-items: center;
        gap: 12px;
      }

      .nav-info {
        font-size: 14px;
        color: #666;
        min-width: 120px;
        text-align: center;
      }

      .nav-button {
        padding: 6px 12px;
        font-size: 14px;
      }

      .record-panel {
        border: 1px solid #e9ecef;
        border-radius: 4px;
        overflow: hidden;
      }

      .record-header {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px 16px;
        background: #f8f9fa;
        border-bottom: 1px solid #e9ecef;
      }

      .record-identifier {
        font-weight: 600;
        font-size: 16px;
        color: #333;
      }

      .badge {
        font-size: 11px;
        font-weight: 600;
        padding: 2px 8px;
        border-radius: 12px;
        text-transform: uppercase;
      }

      .badge-new {
        background: #d4edda;
        color: #155724;
      }

      .badge-update {
        background: #cce5ff;
        color: #004085;
      }

      .record-body {
        padding: 16px;
      }

      .record-section {
        margin-bottom: 16px;
      }

      .record-section:last-child {
        margin-bottom: 0;
      }

      .section-title {
        font-size: 12px;
        font-weight: 600;
        color: #666;
        text-transform: uppercase;
        margin-bottom: 8px;
      }

      .field-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
      }

      .field-item {
        display: flex;
        align-items: flex-start;
        gap: 8px;
        font-size: 14px;
      }

      .field-key {
        font-weight: 500;
        color: #333;
        min-width: 120px;
      }

      .field-value {
        color: #666;
        word-break: break-word;
      }

      .field-delete {
        color: #dc3545;
        font-weight: 500;
      }

      .field-add {
        color: #28a745;
      }

      .field-remove {
        color: #dc3545;
      }

      .validation-errors {
        background: #fef2f2;
        border: 1px solid #fecaca;
        border-radius: 4px;
        padding: 12px;
      }

      .validation-error-item {
        color: #dc2626;
        font-size: 14px;
        margin-bottom: 4px;
      }

      .validation-error-item:last-child {
        margin-bottom: 0;
      }

      .warnings {
        background: #fffbeb;
        border: 1px solid #fcd34d;
        border-radius: 4px;
        padding: 12px;
      }

      .warning-item {
        color: #92400e;
        font-size: 14px;
        margin-bottom: 4px;
      }

      .warning-item:last-child {
        margin-bottom: 0;
      }

      /* Complete State Styles */
      .complete-container {
        text-align: center;
        padding: 40px 20px;
      }

      .complete-icon {
        font-size: 64px;
        color: #28a745;
        margin-bottom: 16px;
      }

      .complete-title {
        font-size: 24px;
        font-weight: 600;
        color: #333;
        margin-bottom: 8px;
      }

      .complete-message {
        font-size: 16px;
        color: #666;
        margin-bottom: 24px;
      }

      .report-link {
        display: inline-flex;
        align-items: center;
        gap: 8px;
        color: #4a90d9;
        text-decoration: none;
        font-size: 16px;
      }

      .report-link:hover {
        text-decoration: underline;
      }

      /* Parsing Errors */
      .parsing-errors {
        margin-bottom: 16px;
      }

      .parsing-error-title {
        font-weight: 600;
        color: #dc3545;
        margin-bottom: 8px;
      }
    `,
  ];

  @property({ type: Boolean, reflect: true })
  open = false;

  @state()
  private dialogState: DialogState = 'upload';

  @state()
  private file: File | null = null;

  @state()
  private records: PageImportRecord[] = [];

  @state()
  private currentRecordIndex = 0;

  @state()
  private showErrorsOnly = false;

  @state()
  private error: AugmentedError | null = null;

  @state()
  private stats: ImportStats = { total: 0, errors: 0, updates: 0, creates: 0 };

  @state()
  private importedCount = 0;

  @state()
  private dragOver = false;

  @state()
  private parsingErrors: string[] = [];

  @state()
  private importProgressMessage = '';

  private _pageImportClient: ReturnType<typeof createClient<typeof PageImportService>> | null =
    null;

  private _systemInfoClient: ReturnType<typeof createClient<typeof SystemInfoService>> | null =
    null;

  private _streamAbortController: AbortController | null = null;

  private get pageImportClient(): ReturnType<typeof createClient<typeof PageImportService>> {
    if (!this._pageImportClient) {
      this._pageImportClient = createClient(PageImportService, getGrpcWebTransport());
    }
    return this._pageImportClient;
  }

  private get systemInfoClient(): ReturnType<typeof createClient<typeof SystemInfoService>> {
    if (!this._systemInfoClient) {
      this._systemInfoClient = createClient(SystemInfoService, getGrpcWebTransport());
    }
    return this._systemInfoClient;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('keydown', this._handleKeydown);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('keydown', this._handleKeydown);
  }

  public _handleKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape' && this.open) {
      this.closeDialog();
    }
  };

  public openDialog(): void {
    this.open = true;
    this.resetState();
  }

  public closeDialog(): void {
    this._streamAbortController?.abort();
    this.open = false;
  }

  private resetState(): void {
    this.dialogState = 'upload';
    this.file = null;
    this.records = [];
    this.currentRecordIndex = 0;
    this.showErrorsOnly = false;
    this.error = null;
    this.stats = { total: 0, errors: 0, updates: 0, creates: 0 };
    this.importedCount = 0;
    this.dragOver = false;
    this.parsingErrors = [];
    this.importProgressMessage = '';
    this._streamAbortController = null;
  }

  private _handleBackdropClick = (): void => {
    this.closeDialog();
  };

  private _handleDialogClick = (event: Event): void => {
    event.stopPropagation();
  };

  private _handleDragOver = (event: DragEvent): void => {
    event.preventDefault();
    event.stopPropagation();
    this.dragOver = true;
  };

  private _handleDragLeave = (event: DragEvent): void => {
    event.preventDefault();
    event.stopPropagation();
    this.dragOver = false;
  };

  private _handleDrop = (event: DragEvent): void => {
    event.preventDefault();
    event.stopPropagation();
    this.dragOver = false;

    const files = event.dataTransfer?.files;
    const firstFile = files?.[0];
    if (firstFile) {
      this._processFile(firstFile);
    }
  };

  private _handleFileInputChange = (event: Event): void => {
    if (!(event.target instanceof HTMLInputElement)) {
      return;
    }
    const firstFile = event.target.files?.[0];
    if (firstFile) {
      this._processFile(firstFile);
    }
  };

  private _processFile(file: File): void {
    if (!file.name.toLowerCase().endsWith('.csv')) {
      this.error = AugmentErrorService.augmentError(
        new Error('Please select a CSV file'),
        'validating file'
      );
      return;
    }

    this.file = file;
    this.error = null;

    // Automatically trigger parsing
    this._handleParse();
  }

  private _handleSelectFileClick = (): void => {
    const input = this.shadowRoot?.querySelector<HTMLInputElement>('.file-input');
    input?.click();
  };

  private async _handleParse(): Promise<void> {
    if (!this.file) {
      return;
    }

    this.dialogState = 'validating';
    this.error = null;
    this.parsingErrors = [];

    try {
      const csvContent = await this.file.text();
      const request = create(ParseCSVPreviewRequestSchema, { csvContent });
      const response = await this.pageImportClient.parseCSVPreview(request);

      this.records = response.records;
      this.parsingErrors = response.parsingErrors;
      this.stats = {
        total: response.totalRecords,
        errors: response.errorCount,
        updates: response.updateCount,
        creates: response.createCount,
      };

      // Default to showing errors only if there are any
      this.showErrorsOnly = response.errorCount > 0;
      this.currentRecordIndex = 0;
      this.dialogState = 'preview';
    } catch (err) {
      this.error = AugmentErrorService.augmentError(err, 'parsing CSV');
      this.dialogState = 'upload';
    }
  }

  private async _handleImport(): Promise<void> {
    if (!this.file) {
      return;
    }

    this.dialogState = 'importing';
    this.error = null;
    this.importProgressMessage = 'Starting import...';

    try {
      const csvContent = await this.file.text();
      const request = create(StartPageImportJobRequestSchema, { csvContent });
      const response = await this.pageImportClient.startPageImportJob(request);

      if (!response.success) {
        throw new Error(response.error || 'Import failed');
      }

      this.importedCount = response.recordCount;
      this.importProgressMessage = `Importing ${this.importedCount} page${this.importedCount !== 1 ? 's' : ''}...`;

      // Wait for the job to complete by streaming job status
      await this._waitForJobCompletion();

      this.dialogState = 'complete';

      this.dispatchEvent(
        new CustomEvent('import-complete', {
          detail: { count: this.importedCount },
          bubbles: true,
          composed: true,
        })
      );
    } catch (err) {
      this.error = AugmentErrorService.augmentError(err, 'importing pages');
      this.dialogState = 'preview';
    }
  }

  /**
   * Waits for the import job to complete by streaming job status updates.
   * The stream automatically terminates when all queues are idle.
   * Stream errors are non-fatal - if progress tracking fails, the import still completes.
   */
  private async _waitForJobCompletion(): Promise<void> {
    this._streamAbortController = new AbortController();

    const request = create(StreamJobStatusRequestSchema, {
      updateIntervalMs: 500, // Poll every 500ms for responsive updates
    });

    try {
      // Stream job status until all queues are idle (stream auto-terminates)
      for await (const status of this.systemInfoClient.streamJobStatus(request, {
        signal: this._streamAbortController.signal,
      })) {
        this._updateProgressFromJobStatus(status);
      }
    } catch (err) {
      // AbortError is expected when dialog is closed - handle gracefully
      if (err instanceof Error && err.name === 'AbortError') {
        return;
      }
      // Other stream errors are non-fatal - log warning but let import complete
      console.warn('Progress stream error (import will still complete):', err);
      this.importProgressMessage = 'Completing import...';
    }
  }

  /**
   * Updates the progress message based on the current job queue status.
   * Each page import is now a separate job, so jobsRemaining represents pages remaining.
   */
  private _updateProgressFromJobStatus(status: GetJobStatusResponse): void {
    // Find the PageImportJob queue
    const importQueue = status.jobQueues.find(
      (q) => q.name === PageImportDialog.PAGE_IMPORT_QUEUE_NAME
    );

    // Queue not found - just return, stream will terminate when done
    if (!importQueue) {
      return;
    }

    // Queue is inactive - import is complete
    if (!importQueue.isActive) {
      this.importProgressMessage = 'Finalizing import...';
      return;
    }

    const total = importQueue.highWaterMark;
    const remaining = importQueue.jobsRemaining;
    const completed = total - remaining;

    // Zero highWaterMark means queue just started, show generic message
    if (total === 0) {
      this.importProgressMessage = 'Starting import...';
      return;
    }

    // Account for the report job at the end (total includes it)
    // Show "X of Y pages" where Y is total - 1 (excluding report job)
    const pageTotal = total - 1;
    const pagesCompleted = Math.min(completed, pageTotal);

    if (pagesCompleted >= pageTotal && remaining > 0) {
      // All page jobs done, report job running
      this.importProgressMessage = 'Generating import report...';
    } else {
      this.importProgressMessage = `Importing page ${pagesCompleted + 1} of ${pageTotal}...`;
    }
  }

  private get filteredRecords(): PageImportRecord[] {
    if (!this.showErrorsOnly) {
      return this.records;
    }
    return this.records.filter((r) => r.validationErrors.length > 0);
  }

  private get currentRecord(): PageImportRecord | null {
    const filtered = this.filteredRecords;
    if (filtered.length === 0 || this.currentRecordIndex >= filtered.length) {
      return null;
    }
    return filtered[this.currentRecordIndex] ?? null;
  }

  private get validRecordCount(): number {
    return this.stats.total - this.stats.errors;
  }

  private get canImport(): boolean {
    return this.validRecordCount > 0;
  }

  private _handlePrevRecord = (): void => {
    if (this.currentRecordIndex > 0) {
      this.currentRecordIndex--;
    }
  };

  private _handleNextRecord = (): void => {
    if (this.currentRecordIndex < this.filteredRecords.length - 1) {
      this.currentRecordIndex++;
    }
  };

  private _handleShowErrorsOnlyChange = (event: Event): void => {
    if (!(event.target instanceof HTMLInputElement)) {
      return;
    }
    this.showErrorsOnly = event.target.checked;
    this.currentRecordIndex = 0;
  };

  private _renderUploadState() {
    return html`
      <div
        class="drop-zone ${this.dragOver ? 'drag-over' : ''}"
        @dragover=${this._handleDragOver}
        @dragleave=${this._handleDragLeave}
        @drop=${this._handleDrop}
        @click=${this._handleSelectFileClick}
      >
        <div class="drop-zone-icon">
          <i class="fas fa-file-csv"></i>
        </div>
        <div class="drop-zone-text">Drag and drop your CSV file here</div>
        <div class="drop-zone-hint">or click to browse</div>
      </div>

      <div class="file-input-wrapper">
        <input
          type="file"
          class="file-input"
          accept=".csv"
          @change=${this._handleFileInputChange}
        />
        <button
          class="button-base button-secondary button-large border-radius-small"
          @click=${this._handleSelectFileClick}
        >
          <i class="fas fa-folder-open"></i> Select CSV File
        </button>
      </div>

      ${this.error
        ? html`<error-display .augmentedError=${this.error}></error-display>`
        : nothing}
    `;
  }

  private _renderLoadingState(message: string) {
    return html`
      <div class="loading-container">
        <div class="loading-spinner">
          <i class="fas fa-spinner fa-spin"></i>
        </div>
        <div class="loading-text">${message}</div>
      </div>
    `;
  }

  private _renderRecordDetail(record: PageImportRecord) {
    // Build unified field entries for sorting
    type FieldEntry = {
      key: string;
      render: () => ReturnType<typeof html>;
    };

    const fieldEntries: FieldEntry[] = [];

    // Add scalar fields from frontmatter
    if (record.frontmatter) {
      const flattened = flattenFrontmatter(record.frontmatter);
      for (const [key, value] of flattened) {
        fieldEntries.push({
          key,
          render: () => html`
            <div class="field-item">
              <span class="field-key">${key}:</span>
              <span class="field-value">${value}</span>
            </div>
          `,
        });
      }
    }

    // Add fields to delete (with DELETE badge)
    for (const field of record.fieldsToDelete) {
      fieldEntries.push({
        key: field,
        render: () => html`
          <div class="field-item">
            <span class="field-key">${field}:</span>
            <span class="field-value field-delete">DELETE</span>
          </div>
        `,
      });
    }

    // Add array operations (with [] suffix in display)
    for (const op of record.arrayOps) {
      const isAdd = op.operation === ArrayOpType.ENSURE_EXISTS;
      const displayKey = `${op.fieldPath}[]`;
      fieldEntries.push({
        key: displayKey,
        render: () => html`
          <div class="field-item">
            <span class="field-key">${displayKey}:</span>
            <span class="field-value ${isAdd ? 'field-add' : 'field-remove'}">
              ${isAdd ? '+ENSURE' : '-REMOVE'} "${op.value}"
            </span>
          </div>
        `,
      });
    }

    // Sort all fields alphabetically by key
    fieldEntries.sort((a, b) => a.key.localeCompare(b.key));

    return html`
      <div class="record-panel">
        <div class="record-header">
          <span class="record-identifier">${record.identifier}</span>
          <span class="badge ${record.pageExists ? 'badge-update' : 'badge-new'}">
            ${record.pageExists ? 'UPDATE' : 'NEW'}
          </span>
        </div>
        <div class="record-body">
          ${record.template
            ? html`
                <div class="record-section">
                  <div class="section-title">Template</div>
                  <div class="field-list">
                    <div class="field-item">
                      <span class="field-value">${record.template}</span>
                    </div>
                  </div>
                </div>
              `
            : nothing}
          ${fieldEntries.length > 0
            ? html`
                <div class="record-section">
                  <div class="section-title">Fields</div>
                  <div class="field-list">
                    ${fieldEntries.map((entry) => entry.render())}
                  </div>
                </div>
              `
            : nothing}
          ${record.warnings.length > 0
            ? html`
                <div class="record-section">
                  <div class="warnings">
                    ${record.warnings.map(
                      (warning) => html`
                        <div class="warning-item">
                          <i class="fas fa-exclamation-triangle"></i> ${warning}
                        </div>
                      `
                    )}
                  </div>
                </div>
              `
            : nothing}
          ${record.validationErrors.length > 0
            ? html`
                <div class="record-section">
                  <div class="validation-errors">
                    ${record.validationErrors.map(
                      (error) => html`
                        <div class="validation-error-item">
                          <i class="fas fa-times-circle"></i> ${error}
                        </div>
                      `
                    )}
                  </div>
                </div>
              `
            : nothing}
        </div>
      </div>
    `;
  }

  private _renderPreviewState() {
    const filtered = this.filteredRecords;
    const record = this.currentRecord;
    const label = this.showErrorsOnly ? 'Error' : 'Record';

    return html`
      ${this.parsingErrors.length > 0
        ? html`
            <div class="parsing-errors validation-errors">
              <div class="parsing-error-title">CSV Parsing Errors</div>
              ${this.parsingErrors.map(
                (error) => html`
                  <div class="validation-error-item">
                    <i class="fas fa-times-circle"></i> ${error}
                  </div>
                `
              )}
            </div>
          `
        : nothing}

      <div class="summary-bar">
        <span class="summary-item">${this.stats.total} total</span>
        <span class="summary-item creates">${this.stats.creates} new</span>
        <span class="summary-item updates">${this.stats.updates} update</span>
        ${this.stats.errors > 0
          ? html`<span class="summary-item errors">${this.stats.errors} err</span>`
          : nothing}
      </div>

      <div class="filter-row">
        ${this.stats.errors > 0
          ? html`
              <label class="checkbox-label">
                <input
                  type="checkbox"
                  .checked=${this.showErrorsOnly}
                  @change=${this._handleShowErrorsOnlyChange}
                />
                Show errors only
              </label>
            `
          : html`<div></div>`}

        ${filtered.length > 0
          ? html`
              <div class="navigation">
                <button
                  class="button-base button-secondary nav-button"
                  @click=${this._handlePrevRecord}
                  ?disabled=${this.currentRecordIndex === 0}
                >
                  <i class="fas fa-chevron-left"></i> Prev
                </button>
                <span class="nav-info"
                  >${label} ${this.currentRecordIndex + 1} of ${filtered.length}</span
                >
                <button
                  class="button-base button-secondary nav-button"
                  @click=${this._handleNextRecord}
                  ?disabled=${this.currentRecordIndex >= filtered.length - 1}
                >
                  Next <i class="fas fa-chevron-right"></i>
                </button>
              </div>
            `
          : nothing}
      </div>

      ${record
        ? this._renderRecordDetail(record)
        : html`
            <div class="loading-container">
              <div class="loading-text">
                ${this.showErrorsOnly ? 'No errors to display' : 'No records to display'}
              </div>
            </div>
          `}

      ${this.error
        ? html`<error-display .augmentedError=${this.error}></error-display>`
        : nothing}
    `;
  }

  private _renderCompleteState() {
    return html`
      <div class="complete-container">
        <div class="complete-icon">
          <i class="fas fa-check-circle"></i>
        </div>
        <div class="complete-title">Import Complete</div>
        <div class="complete-message">
          Successfully imported ${this.importedCount} page${this.importedCount !== 1 ? 's' : ''}.
        </div>
        <a href="/page_import_report" class="report-link">
          <i class="fas fa-file-alt"></i> View Import Report
        </a>
      </div>
    `;
  }

  private _renderContent() {
    switch (this.dialogState) {
      case 'upload':
        return this._renderUploadState();
      case 'validating':
        return this._renderLoadingState('Parsing CSV...');
      case 'preview':
        return this._renderPreviewState();
      case 'importing':
        return this._renderLoadingState(this.importProgressMessage || 'Importing pages...');
      case 'complete':
        return this._renderCompleteState();
    }
  }

  private _renderFooter() {
    switch (this.dialogState) {
      case 'upload':
        return html`
          <button
            class="button-base button-secondary button-large border-radius-small"
            @click=${this.closeDialog}
          >
            Cancel
          </button>
        `;
      case 'validating':
        return html`
          <button
            class="button-base button-secondary button-large border-radius-small"
            disabled
          >
            Cancel
          </button>
        `;
      case 'importing':
        return html`
          <button
            class="button-base button-secondary button-large border-radius-small"
            @click=${this.closeDialog}
          >
            Close
          </button>
        `;
      case 'preview':
        return html`
          <button
            class="button-base button-secondary button-large border-radius-small"
            @click=${this.closeDialog}
          >
            Cancel
          </button>
          <button
            class="button-base button-primary button-large border-radius-small"
            @click=${this._handleImport}
            ?disabled=${!this.canImport}
          >
            Import ${this.validRecordCount} page${this.validRecordCount !== 1 ? 's' : ''}
          </button>
        `;
      case 'complete':
        return html`
          <button
            class="button-base button-primary button-large border-radius-small"
            @click=${this.closeDialog}
          >
            Done
          </button>
        `;
    }
  }

  private _getDialogTitle(): string {
    switch (this.dialogState) {
      case 'upload':
        return 'Import Pages from CSV';
      case 'validating':
        return 'Validating CSV';
      case 'preview':
        return 'Preview Import';
      case 'importing':
        return 'Importing Pages';
      case 'complete':
        return 'Import Complete';
    }
  }

  override render() {
    return html`
      ${sharedStyles}
      <div class="backdrop" @click=${this._handleBackdropClick}></div>
      <div class="dialog system-font border-radius box-shadow" @click=${this._handleDialogClick}>
        <div class="dialog-header">
          <h2 class="dialog-title">${this._getDialogTitle()}</h2>
        </div>
        <div class="content">${this._renderContent()}</div>
        <div class="footer">${this._renderFooter()}</div>
      </div>
    `;
  }
}

customElements.define('page-import-dialog', PageImportDialog);

declare global {
  interface HTMLElementTagNameMap {
    'page-import-dialog': PageImportDialog;
  }
}
