import { html, css, LitElement, nothing } from 'lit';
import { property } from 'lit/decorators.js';
import { foundationCSS } from './shared-styles.js';

export interface PageStatus {
  pageName: string;
  versionHash?: string;
  lastRefreshTime?: Date;
  isWatching: boolean;
}

export class SystemInfoPage extends LitElement {
  static override styles = [
    foundationCSS,
    css`
      :host {
        display: block;
        font-size: 11px;
        line-height: 1.2;
      }

      .updated-row {
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
    `];

  @property({ type: Object })
  declare pageStatus?: PageStatus;

  private formatTimeAgo(date: Date): string {
    const nowMs = Date.now();
    const diffSec = Math.floor((nowMs - date.getTime()) / 1000);

    if (diffSec < 0) {
      return 'just now';
    }

    if (diffSec < 60) {
      return `${diffSec}s ago`;
    }

    const diffMin = Math.floor(diffSec / 60);
    if (diffMin < 60) {
      return `~${diffMin}m ago`;
    }

    const diffHour = Math.floor(diffMin / 60);
    if (diffHour < 24) {
      return `~${diffHour}h ago`;
    }

    const diffDay = Math.floor(diffHour / 24);
    if (diffDay < 30) {
      return `~${diffDay}d ago`;
    }

    const diffMonth = Math.floor(diffDay / 30);
    if (diffMonth < 12) {
      return `~${diffMonth}mo ago`;
    }

    const diffYear = Math.floor(diffDay / 365);
    return `~${diffYear}y ago`;
  }

  override render() {
    if (!this.pageStatus?.lastRefreshTime) {
      return nothing;
    }

    return html`
      <div class="updated-row">
        <span class="label">Page saved:</span>
        <span class="value">${this.formatTimeAgo(this.pageStatus.lastRefreshTime)}</span>
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
