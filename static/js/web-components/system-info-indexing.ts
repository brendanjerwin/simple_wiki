import { html, css, LitElement } from 'lit';
import { GetIndexingStatusResponse } from '../gen/api/v1/system_info_pb.js';
import { foundationCSS } from './shared-styles.js';

export class SystemInfoIndexing extends LitElement {

  static override styles = [
    foundationCSS,
    css`
      :host {
        display: block;
        font-size: 11px;
        line-height: 1.2;
      }

      .indexing-info {
        display: flex;
        flex-direction: column;
        gap: 4px;
      }

      .indexing-header {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: inherit;
      }

      .status-indicator {
        width: 6px;
        height: 6px;
        border-radius: 50%;
        background: #28a745;
        animation: pulse 2s infinite;
        flex-shrink: 0;
      }

      .status-indicator.idle {
        background: #6c757d;
        animation: none;
      }

      @keyframes pulse {
        0% { opacity: 1; }
        50% { opacity: 0.5; }
        100% { opacity: 1; }
      }

      .label {
        color: #adb5bd;
        font-weight: 500;
      }

      .value {
        color: #e9ecef;
        font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
        font-weight: 600;
      }

      .indexing-stats {
        display: flex;
        align-items: center;
        gap: 8px;
      }

      .progress-compact {
        flex: 1;
        min-width: 0;
      }

      .progress-bar-mini {
        width: 100%;
        height: 3px;
        background: rgba(255, 255, 255, 0.1);
        border-radius: 2px;
        overflow: hidden;
      }

      .progress-fill-mini {
        height: 100%;
        background: linear-gradient(90deg, #007bff, #28a745);
        transition: width 0.3s ease;
        border-radius: 2px;
      }

      .rate {
        color: #adb5bd;
        font-size: 10px;
        font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
        white-space: nowrap;
      }

      .queue {
        color: #ffc107;
        font-size: 10px;
        font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
        white-space: nowrap;
      }

      .per-index-progress {
        display: flex;
        flex-direction: column;
        gap: 2px;
        margin-top: 4px;
      }

      .index-item {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 10px;
      }

      .index-name {
        color: #6c757d;
        font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
        min-width: 50px;
        font-size: 9px;
      }

      .index-progress {
        flex: 1;
        min-width: 0;
      }

      .index-progress-bar {
        width: 100%;
        height: 2px;
        background: rgba(255, 255, 255, 0.1);
        border-radius: 1px;
        overflow: hidden;
      }

      .index-progress-fill {
        height: 100%;
        background: #007bff;
        transition: width 0.3s ease;
      }

      .index-progress-fill.complete {
        background: #28a745;
      }

      .index-progress-fill.error {
        background: #dc3545;
      }

      .index-stats {
        color: #6c757d;
        font-size: 9px;
        font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
        white-space: nowrap;
      }

      .error {
        color: #ff6b6b;
        font-size: 10px;
      }

      .loading {
        color: #adb5bd;
        font-style: italic;
      }

    `];

  static override properties = {
    status: { type: Object },
    loading: { type: Boolean },
    error: { type: String },
  };

  declare status?: GetIndexingStatusResponse;
  declare loading: boolean;
  declare error?: string;

  constructor() {
    super();
    this.loading = false;
  }

  private formatRate(rate: number): string {
    if (rate < 0.1) return '< 0.1/s';
    if (rate < 1) return `${rate.toFixed(1)}/s`;
    return `${Math.round(rate)}/s`;
  }

  private calculateProgress(completed: number, total: number): number {
    return total > 0 ? (completed / total) * 100 : 0;
  }

  override render() {
    // Handle loading and error states
    if (this.loading && !this.status) {
      return html`<div class="loading">Loading...</div>`;
    }

    if (this.error) {
      return html`<div class="error">${this.error}</div>`;
    }

    if (!this.status) {
      return html`<div class="loading">No data</div>`;
    }

    // Don't render anything when not running
    if (!this.status.isRunning) {
      return html``;
    }

    const overallProgress = this.calculateProgress(this.status.completedPages, this.status.totalPages);

    return html`
      <div class="indexing-info">
        <div class="indexing-header">
          <div class="status-indicator ${this.status.isRunning ? '' : 'idle'}"></div>
          <span class="label">Indexing</span>
          <span class="value">${this.status.completedPages}/${this.status.totalPages}</span>
        </div>
        <div class="indexing-stats">
          <div class="progress-compact">
            <div class="progress-bar-mini">
              <div class="progress-fill-mini" 
                   style="width: ${overallProgress}%"></div>
            </div>
          </div>
          <span class="rate">${this.formatRate(this.status.processingRatePerSecond)}</span>
          ${this.status.queueDepth > 0 ? html`
            <span class="queue">Q:${this.status.queueDepth}</span>
          ` : ''}
        </div>
        
        <!-- Per-index progress inline (no dropdown) -->
        ${this.status.indexProgress && this.status.indexProgress.length > 1 ? html`
          <div class="per-index-progress">
            ${this.status.indexProgress.map(index => {
              const indexProgress = this.calculateProgress(index.completed, index.total);
              const isIndexComplete = indexProgress === 100;
              const hasError = index.lastError && index.lastError.trim() !== '';
              
              return html`
                <div class="index-item">
                  <div class="index-name">${index.name}</div>
                  <div class="index-progress">
                    <div class="index-progress-bar">
                      <div class="index-progress-fill ${isIndexComplete ? 'complete' : hasError ? 'error' : ''}" 
                           style="width: ${indexProgress}%"></div>
                    </div>
                  </div>
                  <div class="index-stats">${this.formatRate(index.processingRatePerSecond)}</div>
                </div>
                ${hasError ? html`
                  <div class="error">Error: ${index.lastError}</div>
                ` : ''}
              `;
            })}
          </div>
        ` : ''}
      </div>
    `;
  }
}

customElements.define('system-info-indexing', SystemInfoIndexing);

declare global {
  interface HTMLElementTagNameMap {
    'system-info-indexing': SystemInfoIndexing;
  }
}