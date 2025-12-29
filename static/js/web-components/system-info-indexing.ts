import { html, css, LitElement } from 'lit';
import { GetJobStatusResponse } from '../gen/api/v1/system_info_pb.js';
import { foundationCSS } from './shared-styles.js';
import { showToast } from './toast-message.js';

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

      .error.clickable {
        cursor: pointer;
        transition: background-color 0.2s ease;
        border-radius: 4px;
        padding: 4px;
      }

      .error.clickable:hover {
        background-color: rgba(255, 107, 107, 0.1);
      }

      .error.clickable:focus {
        outline: 2px solid #ff6b6b;
        outline-offset: 2px;
      }

      .error.clickable:active {
        background-color: rgba(255, 107, 107, 0.2);
      }

      .loading {
        color: #adb5bd;
        font-style: italic;
      }

    `];

  static override properties = {
    jobStatus: { type: Object },
    loading: { type: Boolean },
    error: { type: String },
  };

  declare jobStatus?: GetJobStatusResponse;
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

  private _handleErrorClick = async (event: Event, errorText: string): Promise<void> => {
    event.stopPropagation();
    try {
      await navigator.clipboard.writeText(errorText);
      showToast('Error copied to clipboard', 'success', 3);
    } catch {
      showToast('Failed to copy error to clipboard', 'error', 5);
    }
  };

  private _handleErrorKeydown = async (event: KeyboardEvent, errorText: string): Promise<void> => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      await this._handleErrorClick(event, errorText);
    }
  };

  override render() {
    // Handle loading and error states
    if (this.loading && !this.jobStatus) {
      return html`<div class="loading">Loading...</div>`;
    }

    if (this.error) {
      return html`<div class="error">${this.error}</div>`;
    }

    if (!this.jobStatus) {
      return html`<div class="loading">No data</div>`;
    }

    // Filter active queues
    const activeQueues = this.jobStatus.jobQueues.filter(queue => queue.isActive);
    
    // Don't render anything when no active queues
    if (activeQueues.length === 0) {
      return html``;
    }

    return html`
      <div class="indexing-info">
        <div class="indexing-header">
          <div class="status-indicator"></div>
          <span class="label">Jobs</span>
          <span class="value">${activeQueues.map(queue => `${queue.name}: ${queue.jobsRemaining}`).join(', ')}</span>
        </div>
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