import { html, css, LitElement } from 'lit';
import { createClient } from '@connectrpc/connect';
import { getGrpcWebTransport } from './grpc-transport.js';
import { SystemInfoService } from '../gen/api/v1/system_info_connect.js';
import { GetVersionRequest, GetVersionResponse, GetIndexingStatusRequest, GetIndexingStatusResponse } from '../gen/api/v1/system_info_pb.js';
import { foundationCSS } from './shared-styles.js';
import './system-info-indexing.js';
import './system-info-version.js';

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


      system-info-indexing {
        border-top: 1px solid #404040;
        padding-top: 4px;
        margin-top: 2px;
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



  override render() {
    return html`
      <div class="system-panel system-font">
        <div class="hover-overlay"></div>
        <div class="system-content">
          <!-- Version Info (Always Present) -->
          <system-info-version 
            .version="${this.version}"
            .loading="${this.loading}"
            .error="${this.error}"></system-info-version>

          <!-- Indexing Status Component -->
          <system-info-indexing 
            .status="${this.indexingStatus}"
            .loading="${this.loading}"
            .error="${this.error}"></system-info-indexing>
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