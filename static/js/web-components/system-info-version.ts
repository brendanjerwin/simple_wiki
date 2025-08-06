import { html, css, LitElement } from 'lit';
import { GetVersionResponse } from '../gen/api/v1/system_info_pb.js';
import { Timestamp } from '@bufbuild/protobuf';
import { foundationCSS } from './shared-styles.js';

export class SystemInfoVersion extends LitElement {
  static override styles = [
    foundationCSS,
    css`
      :host {
        display: block;
        font-size: 11px;
        line-height: 1.2;
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
    `];

  static override properties = {
    version: { type: Object },
    loading: { type: Boolean },
    error: { type: String },
  };

  declare version?: GetVersionResponse;
  declare loading: boolean;
  declare error?: string;

  constructor() {
    super();
    this.loading = false;
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

  override render() {
    return html`
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
    `;
  }
}

customElements.define('system-info-version', SystemInfoVersion);

declare global {
  interface HTMLElementTagNameMap {
    'system-info-version': SystemInfoVersion;
  }
}