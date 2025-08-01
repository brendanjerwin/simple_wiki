import { html, css, LitElement } from 'lit';
import { createClient } from '@connectrpc/connect';
import { getGrpcWebTransport } from './grpc-transport.js';
import { SystemInfoService } from '../gen/api/v1/system_info_connect.js';
import { GetIndexingStatusRequest, GetIndexingStatusResponse } from '../gen/api/v1/system_info_pb.js';
import { Timestamp } from '@bufbuild/protobuf';
import { foundationCSS } from './shared-styles.js';

export class IndexingStatus extends LitElement {
  static readonly REFRESH_INTERVAL = 2000; // Refresh every 2 seconds when indexing is running
  static readonly IDLE_REFRESH_INTERVAL = 10000; // Refresh every 10 seconds when idle

  static override styles = [
    foundationCSS,
    css`
      :host {
        display: block;
        font-size: 13px;
        line-height: 1.4;
      }

      .indexing-panel {
        background: #f8f9fa;
        border: 1px solid #e9ecef;
        border-radius: 6px;
        padding: 12px;
        margin: 8px 0;
      }

      .dark-theme .indexing-panel {
        background: #2d2d2d;
        border-color: #404040;
        color: #e0e0e0;
      }

      .status-header {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 8px;
        font-weight: 600;
      }

      .status-indicator {
        width: 10px;
        height: 10px;
        border-radius: 50%;
        background: #6c757d;
      }

      .status-indicator.running {
        background: #28a745;
        animation: pulse 2s infinite;
      }

      .status-indicator.idle {
        background: #6c757d;
      }

      @keyframes pulse {
        0% { opacity: 1; }
        50% { opacity: 0.5; }
        100% { opacity: 1; }
      }

      .progress-overview {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
        gap: 8px;
        margin-bottom: 12px;
      }

      .progress-item {
        display: flex;
        flex-direction: column;
        gap: 2px;
      }

      .progress-label {
        font-size: 11px;
        color: #6c757d;
        text-transform: uppercase;
        letter-spacing: 0.5px;
      }

      .dark-theme .progress-label {
        color: #adb5bd;
      }

      .progress-value {
        font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
        font-weight: 600;
        color: #495057;
      }

      .dark-theme .progress-value {
        color: #e0e0e0;
      }

      .progress-bar {
        width: 100%;
        height: 6px;
        background: #e9ecef;
        border-radius: 3px;
        overflow: hidden;
        margin: 4px 0;
      }

      .dark-theme .progress-bar {
        background: #404040;
      }

      .progress-fill {
        height: 100%;
        background: #007bff;
        transition: width 0.3s ease;
        border-radius: 3px;
      }

      .progress-fill.complete {
        background: #28a745;
      }

      .index-details {
        margin-top: 12px;
      }

      .index-details summary {
        cursor: pointer;
        font-weight: 500;
        color: #007bff;
        margin-bottom: 8px;
      }

      .dark-theme .index-details summary {
        color: #66b3ff;
      }

      .index-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
        padding-left: 16px;
      }

      .index-item {
        display: grid;
        grid-template-columns: 100px 1fr auto;
        align-items: center;
        gap: 8px;
        padding: 4px 0;
        border-bottom: 1px solid #e9ecef;
      }

      .dark-theme .index-item {
        border-bottom-color: #404040;
      }

      .index-name {
        font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
        font-size: 11px;
        color: #6c757d;
      }

      .dark-theme .index-name {
        color: #adb5bd;
      }

      .index-progress {
        display: flex;
        align-items: center;
        gap: 8px;
        flex: 1;
      }

      .index-progress-bar {
        flex: 1;
        height: 4px;
        background: #e9ecef;
        border-radius: 2px;
        overflow: hidden;
      }

      .dark-theme .index-progress-bar {
        background: #404040;
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
        font-size: 10px;
        color: #6c757d;
        font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
      }

      .dark-theme .index-stats {
        color: #adb5bd;
      }

      .error {
        color: #dc3545;
        font-size: 11px;
        margin-top: 4px;
      }

      .dark-theme .error {
        color: #ff6b6b;
      }

      .loading {
        color: #6c757d;
        font-style: italic;
      }

      .dark-theme .loading {
        color: #adb5bd;
      }

      .eta {
        font-size: 11px;
        color: #6c757d;
        margin-top: 4px;
      }

      .dark-theme .eta {
        color: #adb5bd;
      }
    `];

  static override properties = {
    status: { state: true },
    loading: { state: true },
    error: { state: true },
  };

  declare status?: GetIndexingStatusResponse;
  declare loading: boolean;
  declare error?: string;
  private refreshTimer?: ReturnType<typeof setInterval>;

  private client = createClient(SystemInfoService, getGrpcWebTransport());

  constructor() {
    super();
    this.loading = true;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    this.loadStatus();
    this.startAutoRefresh();
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this.stopAutoRefresh();
  }

  private startAutoRefresh(): void {
    this.stopAutoRefresh();
    
    // Use different refresh intervals based on whether indexing is running
    const interval = this.status?.isRunning ? 
      IndexingStatus.REFRESH_INTERVAL : 
      IndexingStatus.IDLE_REFRESH_INTERVAL;
    
    this.refreshTimer = setInterval(() => {
      this.loadStatus();
    }, interval);
  }

  private stopAutoRefresh(): void {
    if (this.refreshTimer) {
      clearInterval(this.refreshTimer);
      this.refreshTimer = undefined;
    }
  }

  private async loadStatus(): Promise<void> {
    try {
      this.error = undefined;
      
      const response = await this.client.getIndexingStatus(new GetIndexingStatusRequest());
      this.status = response;
      
      // Adjust refresh interval based on running status
      if (this.refreshTimer) {
        const currentInterval = this.status.isRunning ? 
          IndexingStatus.REFRESH_INTERVAL : 
          IndexingStatus.IDLE_REFRESH_INTERVAL;
        this.stopAutoRefresh();
        this.refreshTimer = setInterval(() => {
          this.loadStatus();
        }, currentInterval);
      }
    } catch (err) {
      this.error = err instanceof Error ? err.message : 'Failed to load indexing status';
    } finally {
      this.loading = false;
      this.requestUpdate();
    }
  }

  private formatTimestamp(timestamp: Timestamp): string {
    const date = timestamp.toDate();
    const now = new Date();
    const diffMs = date.getTime() - now.getTime();
    
    if (diffMs < 0) {
      return 'Overdue';
    }
    
    const diffSeconds = Math.floor(diffMs / 1000);
    const diffMinutes = Math.floor(diffSeconds / 60);
    const diffHours = Math.floor(diffMinutes / 60);
    
    if (diffHours > 0) {
      return `~${diffHours}h ${diffMinutes % 60}m`;
    } else if (diffMinutes > 0) {
      return `~${diffMinutes}m`;
    } else {
      return `~${diffSeconds}s`;
    }
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
    if (this.loading && !this.status) {
      return html`
        <div class="indexing-panel">
          <div class="loading">Loading indexing status...</div>
        </div>
      `;
    }

    if (this.error) {
      return html`
        <div class="indexing-panel">
          <div class="error">Error: ${this.error}</div>
        </div>
      `;
    }

    if (!this.status) {
      return html`
        <div class="indexing-panel">
          <div class="loading">No indexing status available</div>
        </div>
      `;
    }

    const overallProgress = this.calculateProgress(this.status.completedPages, this.status.totalPages);
    const isComplete = overallProgress === 100;

    return html`
      <div class="indexing-panel">
        <div class="status-header">
          <div class="status-indicator ${this.status.isRunning ? 'running' : 'idle'}"></div>
          <span>Indexing ${this.status.isRunning ? 'Active' : 'Idle'}</span>
        </div>

        ${this.status.totalPages > 0 ? html`
          <div class="progress-overview">
            <div class="progress-item">
              <div class="progress-label">Progress</div>
              <div class="progress-value">${this.status.completedPages}/${this.status.totalPages}</div>
            </div>
            <div class="progress-item">
              <div class="progress-label">Queue Depth</div>
              <div class="progress-value">${this.status.queueDepth}</div>
            </div>
            <div class="progress-item">
              <div class="progress-label">Rate</div>
              <div class="progress-value">${this.formatRate(this.status.processingRatePerSecond)}</div>
            </div>
          </div>

          <div class="progress-bar">
            <div class="progress-fill ${isComplete ? 'complete' : ''}" 
                 style="width: ${overallProgress}%"></div>
          </div>

          ${this.status.estimatedCompletion && this.status.isRunning ? html`
            <div class="eta">ETA: ${this.formatTimestamp(this.status.estimatedCompletion)}</div>
          ` : ''}
        ` : ''}

        ${this.status.indexProgress && this.status.indexProgress.length > 0 ? html`
          <details class="index-details">
            <summary>Per-Index Progress (${this.status.indexProgress.length} indexes)</summary>
            <div class="index-list">
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
                      <div class="index-stats">${index.completed}/${index.total}</div>
                    </div>
                    <div class="index-stats">${this.formatRate(index.processingRatePerSecond)}</div>
                  </div>
                  ${hasError ? html`
                    <div class="error">Error: ${index.lastError}</div>
                  ` : ''}
                `;
              })}
            </div>
          </details>
        ` : ''}
      </div>
    `;
  }
}

customElements.define('indexing-status', IndexingStatus);

declare global {
  interface HTMLElementTagNameMap {
    'indexing-status': IndexingStatus;
  }
}