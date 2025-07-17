import { html, css, LitElement } from 'lit';
import { createClient } from '@connectrpc/connect';
import { createGrpcWebTransport } from '@connectrpc/connect-web';
import { Version } from '../gen/api/v1/version_connect.js';
import { GetVersionRequest, GetVersionResponse } from '../gen/api/v1/version_pb.js';
import { Timestamp } from '@bufbuild/protobuf';

export class VersionDisplay extends LitElement {
  static override styles = css`
    :host {
      position: fixed;
      bottom: 2px;
      right: 2px;
      z-index: 1000;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen',
        'Ubuntu', 'Cantarell', 'Fira Sans', 'Droid Sans', 'Helvetica Neue',
        sans-serif;
      font-size: 11px;
      line-height: 1.2;
      transition: opacity 0.3s ease;
    }

    .version-panel {
      background: #2d2d2d;
      border: 1px solid #404040;
      border-radius: 4px;
      padding: 4px 8px;
      opacity: 0.2;
      transition: opacity 0.3s ease;
      box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
      position: relative;
    }

    .version-panel:hover {
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

    .version-info {
      display: flex;
      flex-direction: row;
      align-items: center;
      gap: 12px;
      color: white;
      white-space: nowrap;
    }

    .version-row {
      display: flex;
      align-items: center;
      white-space: nowrap;
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
      max-width: 70px;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .error {
      color: #ff6b6b;
    }

    .loading {
      color: #ccc;
    }
  `;

  static override properties = {
    version: { state: true },
    loading: { state: true },
    error: { state: true },
  };

  declare version?: GetVersionResponse;
  declare loading: boolean;
  declare error?: string;
  private debounceTimer?: number;

  private client = createClient(Version, createGrpcWebTransport({
    baseUrl: window.location.origin,
  }));

  constructor() {
    super();
    this.loading = true;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    this.loadVersion();
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
    
    // Set a new debounce timer (300ms delay)
    this.debounceTimer = setTimeout(() => {
      this.loadVersion();
    }, 300);
  }

  private async loadVersion(): Promise<void> {
    try {
      this.loading = true;
      this.error = undefined;
      this.requestUpdate();
      
      const response = await this.client.getVersion(new GetVersionRequest());
      this.version = response;
    } catch (err) {
      this.error = err instanceof Error ? err.message : 'Failed to load version';
    } finally {
      this.loading = false;
      this.requestUpdate();
    }
  }

  private formatTimestamp(timestamp?: Timestamp): string {
    if (!timestamp) return 'Unknown';
    
    try {
      const date = timestamp.toDate();
      return date.toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return 'Invalid date';
    }
  }

  private formatCommit(commit: string): string {
    return commit.length > 7 ? commit.substring(0, 7) : commit;
  }

  override render() {
    return html`
      <div class="version-panel">
        <div class="hover-overlay"></div>
        <div class="version-info">
          ${this.loading ? html`
            <div class="version-row">
              <span class="label">Version:</span>
              <span class="value loading">Loading...</span>
            </div>
            <div class="version-row">
              <span class="label">Commit:</span>
              <span class="value loading">Loading...</span>
            </div>
            <div class="version-row">
              <span class="label">Built:</span>
              <span class="value loading">Loading...</span>
            </div>
          ` : this.error ? html`
            <div class="error">${this.error}</div>
          ` : html`
            <div class="version-row">
              <span class="label">Version:</span>
              <span class="value">${this.version?.version || 'Unknown'}</span>
            </div>
            <div class="version-row">
              <span class="label">Commit:</span>
              <span class="value commit">${this.formatCommit(this.version?.commit || '')}</span>
            </div>
            <div class="version-row">
              <span class="label">Built:</span>
              <span class="value">${this.formatTimestamp(this.version?.buildTime)}</span>
            </div>
          `}
        </div>
      </div>
    `;
  }
}

customElements.define('version-display', VersionDisplay);

declare global {
  interface HTMLElementTagNameMap {
    'version-display': VersionDisplay;
  }
}