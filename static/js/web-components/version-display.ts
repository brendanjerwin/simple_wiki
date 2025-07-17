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
      bottom: 10px;
      right: 10px;
      z-index: 1000;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen',
        'Ubuntu', 'Cantarell', 'Fira Sans', 'Droid Sans', 'Helvetica Neue',
        sans-serif;
      font-size: 11px;
      line-height: 1.2;
      transition: opacity 0.3s ease;
    }

    .version-panel {
      background: rgba(0, 0, 0, 0.1);
      backdrop-filter: blur(10px);
      border: 1px solid rgba(255, 255, 255, 0.1);
      border-radius: 6px;
      padding: 8px 12px;
      opacity: 0.3;
      transition: opacity 0.3s ease;
      max-width: 300px;
      box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
    }

    .version-panel:hover {
      opacity: 0.9;
    }

    .version-info {
      display: flex;
      flex-direction: column;
      gap: 2px;
      color: #333;
    }

    .version-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      white-space: nowrap;
    }

    .label {
      font-weight: 500;
      color: #666;
      margin-right: 8px;
    }

    .value {
      font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
      color: #333;
      font-size: 10px;
    }

    .commit {
      max-width: 100px;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .error {
      color: #d32f2f;
    }

    .loading {
      color: #666;
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
        <div class="version-info">
          ${this.loading ? html`
            <div class="loading">Loading version...</div>
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