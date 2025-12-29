import { html, css, LitElement } from 'lit';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import { SystemInfoService, GetVersionRequestSchema, GetJobStatusRequestSchema, StreamJobStatusRequestSchema, type GetVersionResponse, type GetJobStatusResponse } from '../gen/api/v1/system_info_pb.js';
import { foundationCSS } from './shared-styles.js';
import './system-info-identity.js';
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
      }

      .system-panel {
        display: flex;
        flex-direction: row;
        position: relative;
        max-width: 400px;
        transform: translateX(calc(100% - 24px));
        transition: transform 0.3s ease;
        cursor: pointer;
        outline: none;
      }

      .system-panel:focus-visible {
        outline: 2px solid #4a9eff;
        outline-offset: 2px;
      }

      .system-panel.expanded {
        transform: translateX(0);
      }

      .drawer-tab {
        width: 24px;
        background: #2d2d2d;
        border: 1px solid #404040;
        border-right: none;
        border-radius: 4px 0 0 4px;
        display: flex;
        align-items: center;
        justify-content: center;
        writing-mode: vertical-rl;
        text-orientation: mixed;
        padding: 6px 2px;
        font-size: 9px;
        font-weight: 600;
        color: #888;
        letter-spacing: 0.5px;
        box-shadow: -2px 0 3px rgba(0, 0, 0, 0.2);
        flex-shrink: 0;
        opacity: 0.3;
        transition: opacity 0.3s ease, color 0.3s ease;
      }

      .system-panel:hover .drawer-tab,
      .system-panel:focus-visible .drawer-tab {
        opacity: 0.6;
        color: #aaa;
      }

      .system-panel.expanded .drawer-tab {
        opacity: 0.9;
        color: #ccc;
      }

      .panel-content {
        background: #2d2d2d;
        border: 1px solid #404040;
        border-radius: 0 4px 4px 0;
        padding: 4px 8px;
        box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
        opacity: 0.2;
        transition: opacity 0.3s ease;
      }

      .system-panel.expanded .panel-content {
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
    jobStatus: { state: true },
    loading: { state: true },
    error: { state: true },
    expanded: { state: true },
  };

  declare version?: GetVersionResponse;
  declare jobStatus?: GetJobStatusResponse;
  declare loading: boolean;
  declare error: Error | null;
  declare expanded: boolean;
  private debounceTimer?: ReturnType<typeof setTimeout>;
  private refreshTimer?: ReturnType<typeof setInterval>;
  private streamSubscription?: AbortController;
  private _handleClickOutside: (event: MouseEvent) => void;

  private client = createClient(SystemInfoService, getGrpcWebTransport());

  constructor() {
    super();
    this.loading = true;
    this.error = null;
    this.expanded = false;
    this._handleClickOutside = this.handleClickOutside.bind(this);
  }

  override connectedCallback(): void {
    super.connectedCallback();
    this.loadSystemInfo();
    document.addEventListener('click', this._handleClickOutside);
  }

  override firstUpdated(): void {
    // Add hover event listener to the overlay after the component is first rendered
    const overlay = this.shadowRoot?.querySelector('.hover-overlay');
    if (overlay) {
      overlay.addEventListener('mouseenter', this.handleMouseEnter);
    }
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this.stopJobStream();
    this.stopAutoRefresh();
    // Clean up debounce timer
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
      delete this.debounceTimer;
    }
    // Remove click-outside listener
    document.removeEventListener('click', this._handleClickOutside);
    
    // Remove hover event listener
    const overlay = this.shadowRoot?.querySelector('.hover-overlay');
    if (overlay) {
      overlay.removeEventListener('mouseenter', this.handleMouseEnter);
    }
  }

  private handlePanelClick = (event: Event): void => {
    // Stop propagation to prevent click-outside from firing
    event.stopPropagation();
    // Toggle expansion state
    this.expanded = !this.expanded;
  };

  private handlePanelKeydown = (event: KeyboardEvent): void => {
    // Support keyboard activation with Enter or Space
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      event.stopPropagation();
      this.expanded = !this.expanded;
    }
  };

  private handleClickOutside(event: MouseEvent): void {
    // Close when an expanded panel receives a click outside this component
    const path = event.composedPath();
    if (this.expanded && !path.includes(this)) {
      this.expanded = false;
    }
  }

  private handleMouseEnter = (): void => {
    // Clear any existing debounce timer
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
    }

    // Set a new debounce timer
    this.debounceTimer = setTimeout(() => {
      // If we're streaming, just reload version info, otherwise reload everything
      if (this.streamSubscription) {
        this.reloadVersionOnly();
      } else {
        this.loadSystemInfo();
      }
    }, SystemInfo.DEBOUNCE_DELAY);
  }

  private async reloadVersionOnly(): Promise<void> {
    try {
      this.version = await this.client.getVersion(create(GetVersionRequestSchema, {}));
    } catch (err) {
      this.error = err instanceof Error ? err : new Error(String(err));
    } finally {
      this.requestUpdate();
    }
  }

  private startAutoRefresh(): void {
    this.stopAutoRefresh();
    
    // Use different refresh intervals based on whether jobs are running
    const hasActiveJobs = this.jobStatus?.jobQueues.some(queue => queue.isActive);
    const interval = hasActiveJobs ? 
      SystemInfo.REFRESH_INTERVAL : 
      SystemInfo.IDLE_REFRESH_INTERVAL;
    
    this.refreshTimer = setInterval(() => {
      this.loadSystemInfo();
    }, interval);
  }

  private stopAutoRefresh(): void {
    if (this.refreshTimer) {
      clearInterval(this.refreshTimer);
      delete this.refreshTimer;
    }
  }

  private async loadSystemInfo(): Promise<void> {
    try {
      this.error = null;
      
      // Load version (always use unary call for this)
      this.version = await this.client.getVersion(create(GetVersionRequestSchema, {}));
      
      // Load initial job status
      this.jobStatus = await this.client.getJobStatus(create(GetJobStatusRequestSchema, {}));
      
      // Use streaming if any jobs are active, otherwise use polling
      const hasActiveJobs = this.jobStatus.jobQueues.some(queue => queue.isActive);
      if (hasActiveJobs) {
        this.startJobStream();
      } else {
        this.startAutoRefresh();
      }
    } catch (err) {
      this.error = err instanceof Error ? err : new Error(String(err));
      // Fallback to polling on error
      this.startAutoRefresh();
    } finally {
      this.loading = false;
      this.requestUpdate();
    }
  }

  private async startJobStream(): Promise<void> {
    this.stopJobStream();
    this.stopAutoRefresh();
    
    this.streamSubscription = new AbortController();
    
    try {
      const request = create(StreamJobStatusRequestSchema, {
        updateIntervalMs: 1000 // 1 second updates
      });
      
      for await (const response of this.client.streamJobStatus(request, {
        signal: this.streamSubscription.signal
      })) {
        this.jobStatus = response;
        this.requestUpdate();
        
        // Stop streaming when all jobs complete
        const hasActiveJobs = response.jobQueues.some(queue => queue.isActive);
        if (!hasActiveJobs) {
          this.stopJobStream();
          this.startAutoRefresh(); // Switch to polling for idle state
          break;
        }
      }
    } catch (err) {
      const isAbortError = err instanceof Error && err.name === 'AbortError';
      if (!isAbortError) {
        this.error = err instanceof Error ? err : new Error(String(err));
        // Fallback to polling
        this.startAutoRefresh();
      }
    }
  }

  private stopJobStream(): void {
    if (this.streamSubscription) {
      this.streamSubscription.abort();
      delete this.streamSubscription;
    }
  }



  override render() {
    return html`
      <div 
        class="system-panel ${this.expanded ? 'expanded' : ''}" 
        role="button"
        tabindex="0"
        aria-expanded="${this.expanded}"
        aria-label="System information panel"
        @click="${this.handlePanelClick}"
        @keydown="${this.handlePanelKeydown}">
        ${this.expanded ? html`<div class="hover-overlay"></div>` : ''}
        <div class="drawer-tab">INFO</div>
        <div class="panel-content system-font">
          <div class="system-content">
            <!-- Version Info (Always Present) -->
            <system-info-version
              .version="${this.version}"
              .loading="${this.loading}"
              .error="${this.error}"></system-info-version>

            <!-- Tailscale Identity (if available) -->
            <system-info-identity
              .identity="${this.version?.tailscaleIdentity}"></system-info-identity>

            <!-- Job Status Component -->
            <system-info-indexing
              .jobStatus="${this.jobStatus}"
              .loading="${this.loading}"
              .error="${this.error}"></system-info-indexing>
          </div>
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