import { html, css, LitElement } from 'lit';
import { createClient } from '@connectrpc/connect';
import { getGrpcWebTransport } from './grpc-transport.js';
import { SystemInfoService } from '../gen/api/v1/system_info_connect.js';
import { GetVersionRequest, GetVersionResponse, GetIndexingStatusRequest, GetIndexingStatusResponse } from '../gen/api/v1/system_info_pb.js';
import { Timestamp } from '@bufbuild/protobuf';
import { foundationCSS } from './shared-styles.js';

export class SystemInfo extends LitElement {
  static readonly DEBOUNCE_DELAY = 300;
  static readonly REFRESH_INTERVAL = 2000; // 2 seconds when indexing active
  static readonly IDLE_REFRESH_INTERVAL = 10000; // 10 seconds when idle

  static override styles = [
    foundationCSS,
    css`
      :host {
        position: fixed;
        bottom: 2px;
        right: 2px;
        z-index: 1000;
        font-size: 11px;
        line-height: 1.2;
        transition: opacity 0.3s ease;
      }

      .system-panel {
        background: #2d2d2d;
        border: 1px solid #404040;
        border-radius: 4px;
        padding: 4px 8px;
        opacity: 0.2;
        transition: opacity 0.3s ease;
        box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
        position: relative;
        max-width: 400px;
      }

      .system-panel:hover {
        opacity: 0.9;
      }

      .hover-overlay {
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        z-index: 1;
        background: transparent;
        cursor: pointer;
      }

      .system-content {
        display: flex;
        flex-direction: column;
        gap: 4px;
        color: white;
        white-space: nowrap;
        position: relative;
        z-index: 2;
        pointer-events: none;
      }

      .version-info {
        display: flex;
        flex-direction: row;
        align-items: center;
        gap: 12px;
      }

      .version-row {
        display: flex;
        align-items: center;
        white-space: nowrap;
      }

      .indexing-info {
        display: flex;
        flex-direction: column;
        gap: 2px;
        border-top: 1px solid #404040;
        padding-top: 4px;
        margin-top: 2px;
      }

      .indexing-header {
        display: flex;
        align-items: center;
        gap: 6px;
      }

      .status-indicator {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background: #28a745;
        animation: pulse 2s infinite;
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

      .indexing-stats {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 10px;
      }

      .progress-compact {
        display: flex;
        align-items: center;
        gap: 4px;
      }

      .progress-bar-mini {
        width: 40px;
        height: 3px;
        background: #404040;
        border-radius: 2px;
        overflow: hidden;
      }

      .progress-fill-mini {
        height: 100%;
        background: #28a745;
        transition: width 0.3s ease;
        border-radius: 2px;
      }

      .label {
        font-weight: bold;
        color: white;
        margin-right: 4px;
      }

      .value {
        font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
        color: #ccc;
        font-size: 10px;
      }

      .commit {
        max-width: 120px;
        overflow: hidden;
        text-overflow: ellipsis;
      }

      .error {
        color: #ff6b6b;
      }

      .loading {
        color: #ccc;
      }

      .rate {
        color: #adb5bd;
      }

      .queue {
        color: #ffc107;
      }
    `];

  static override properties = {
    version: { state: true },
    indexingStatus: { state: true },
    loading: { state: true },
    error: { state: true },
  };

  declare version?: GetVersionResponse;
  declare indexingStatus?: GetIndexingStatusResponse;
  declare loading: boolean;
  declare error?: string;
  private debounceTimer?: ReturnType<typeof setTimeout>;
  private refreshTimer?: ReturnType<typeof setInterval>;

  private client = createClient(SystemInfoService, getGrpcWebTransport());

  constructor() {
    super();
    this.loading = true;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    this.loadSystemInfo();
    this.startAutoRefresh();
  }

  override firstUpdated(): void {
    // Add hover event listener to the overlay after the component is first rendered
    const overlay = this.shadowRoot?.querySelector('.hover-overlay');
    if (overlay) {
      overlay.addEventListener('mouseenter', this.handleMouseEnter.bind(this));
    }
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this.stopAutoRefresh();
    // Clean up debounce timer
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
      this.debounceTimer = undefined;
    }
  }

  private handleMouseEnter(): void {
    // Clear any existing debounce timer
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
    }

    // Set a new debounce timer
    this.debounceTimer = setTimeout(() => {
      this.loadSystemInfo();
    }, SystemInfo.DEBOUNCE_DELAY);
  }

  private startAutoRefresh(): void {
    this.stopAutoRefresh();
    
    // Use different refresh intervals based on whether indexing is running
    const interval = this.indexingStatus?.isRunning ? 
      SystemInfo.REFRESH_INTERVAL : 
      SystemInfo.IDLE_REFRESH_INTERVAL;
    
    this.refreshTimer = setInterval(() => {
      this.loadSystemInfo();
    }, interval);
  }

  private stopAutoRefresh(): void {
    if (this.refreshTimer) {
      clearInterval(this.refreshTimer);
      this.refreshTimer = undefined;
    }
  }

  private async loadSystemInfo(): Promise<void> {
    try {
      this.error = undefined;
      
      // Load both version and indexing status in parallel
      const [versionResponse, indexingResponse] = await Promise.all([
        this.client.getVersion(new GetVersionRequest()),
        this.client.getIndexingStatus(new GetIndexingStatusRequest())
      ]);
      
      this.version = versionResponse;
      this.indexingStatus = indexingResponse;
      
      // Adjust refresh interval based on indexing status
      if (this.refreshTimer) {
        const currentInterval = this.indexingStatus.isRunning ? 
          SystemInfo.REFRESH_INTERVAL : 
          SystemInfo.IDLE_REFRESH_INTERVAL;
        this.stopAutoRefresh();
        this.refreshTimer = setInterval(() => {
          this.loadSystemInfo();
        }, currentInterval);
      }
    } catch (err) {
      this.error = err instanceof Error ? err.message : 'Failed to load system info';
    } finally {
      this.loading = false;
      this.requestUpdate();
    }
  }

  private formatTimestamp(timestamp: Timestamp): string {
    const date = timestamp.toDate();
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  private formatCommit(commit: string): string {
    // If commit contains parentheses, it's likely a tagged version like "v1.2.3 (abc1234)"
    if (commit.includes('(') && commit.includes(')')) {
      return commit;
    }
    
    // For plain commit hashes, truncate to 7 characters
    return commit.length > 7 ? commit.substring(0, 7) : commit;
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
    return html`
      <div class="system-panel system-font">
        <div class="hover-overlay"></div>
        <div class="system-content">
          <!-- Version Info (Always Present) -->
          <div class="version-info">
            ${this.loading && !this.version ? html`
              <div class="version-row">
                <span class="label">Commit:</span>
                <span class="value loading">Loading...</span>
              </div>
              <div class="version-row">
                <span class="label">Built:</span>
                <span class="value loading">Loading...</span>
              </div>
            ` : this.error && !this.version ? html`
              <div class="error">${this.error}</div>
            ` : html`
              <div class="version-row">
                <span class="label">Commit:</span>
                <span class="value commit">${this.formatCommit(this.version?.commit || '')}</span>
              </div>
              <div class="version-row">
                <span class="label">Built:</span>
                <span class="value">${this.version?.buildTime ? this.formatTimestamp(this.version.buildTime) : ''}</span>
              </div>
            `}
          </div>

          <!-- Indexing Info (Only When Active) -->
          ${this.indexingStatus?.isRunning ? html`
            <div class="indexing-info">
              <div class="indexing-header">
                <div class="status-indicator"></div>
                <span class="label">Indexing</span>
                <span class="value">${this.indexingStatus.completedPages}/${this.indexingStatus.totalPages}</span>
              </div>
              <div class="indexing-stats">
                <div class="progress-compact">
                  <div class="progress-bar-mini">
                    <div class="progress-fill-mini" 
                         style="width: ${this.calculateProgress(this.indexingStatus.completedPages, this.indexingStatus.totalPages)}%"></div>
                  </div>
                </div>
                <span class="rate">${this.formatRate(this.indexingStatus.processingRatePerSecond)}</span>
                ${this.indexingStatus.queueDepth > 0 ? html`
                  <span class="queue">Q:${this.indexingStatus.queueDepth}</span>
                ` : ''}
              </div>
            </div>
          ` : ''}
        </div>
      </div>
    `;
  }
}

customElements.define('system-info', SystemInfo);

declare global {
  interface HTMLElementTagNameMap {
    'system-info': SystemInfo;
  }
}