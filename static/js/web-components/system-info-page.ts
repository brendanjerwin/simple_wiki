import { html, css, LitElement } from 'lit';
import { property } from 'lit/decorators.js';

export interface PageStatus {
  pageName: string;
  versionHash?: string;
  lastRefreshTime?: Date;
  isWatching: boolean;
}

export class SystemInfoPage extends LitElement {
  static override styles = css`
    :host {
      display: block;
      color: #aaa;
      font-size: 10px;
    }

    .page-info {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }

    .page-row {
      display: flex;
      align-items: center;
      gap: 4px;
    }

    .page-label {
      color: #666;
      min-width: 45px;
    }

    .page-value {
      color: #ccc;
      font-weight: 500;
    }

    .watching-indicator {
      color: #4a9eff;
      font-size: 8px;
    }

    .hash {
      font-family: monospace;
      font-size: 9px;
      color: #888;
    }

    .time {
      color: #888;
    }
  `;

  @property({ type: Object })
  declare pageStatus?: PageStatus;

  private formatTime(date: Date): string {
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffSec = Math.floor(diffMs / 1000);

    if (diffSec < 60) {
      return `${diffSec}s ago`;
    }

    const diffMin = Math.floor(diffSec / 60);
    if (diffMin < 60) {
      return `${diffMin}m ago`;
    }

    const diffHour = Math.floor(diffMin / 60);
    return `${diffHour}h ago`;
  }

  override render() {
    if (!this.pageStatus) {
      return html``;
    }

    const { pageName, versionHash, lastRefreshTime, isWatching } = this.pageStatus;

    return html`
      <div class="page-info">
        <div class="page-row">
          <span class="page-label">Page:</span>
          <span class="page-value">${pageName}</span>
          ${isWatching ? html`<span class="watching-indicator">●</span>` : ''}
        </div>
        ${versionHash ? html`
          <div class="page-row">
            <span class="page-label">Hash:</span>
            <span class="hash">${versionHash.substring(0, 8)}...</span>
          </div>
        ` : ''}
        ${lastRefreshTime ? html`
          <div class="page-row">
            <span class="page-label">Updated:</span>
            <span class="time">${this.formatTime(lastRefreshTime)}</span>
          </div>
        ` : ''}
      </div>
    `;
  }
}

customElements.define('system-info-page', SystemInfoPage);

declare global {
  interface HTMLElementTagNameMap {
    'system-info-page': SystemInfoPage;
  }
}
