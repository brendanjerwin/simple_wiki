import { html, css, LitElement } from 'lit';
import { GetVersionRequest } from '../gen/api/v1/version_pb_simple.js';

export class VersionDisplay extends LitElement {
  static styles = css`
    :host {
      position: fixed;
      bottom: 5px;
      right: 5px;
      z-index: 1000;
      font-family: monospace;
      font-size: 11px;
      line-height: 1.2;
    }

    .version-panel {
      background-color: rgba(0, 0, 0, 0.2);
      color: white;
      padding: 4px 8px;
      border-radius: 3px;
      backdrop-filter: blur(3px);
      transition: background-color 0.3s ease, opacity 0.3s ease;
      white-space: nowrap;
    }

    .version-panel:hover {
      background-color: rgba(0, 0, 0, 0.6);
    }

    .version-panel.loading {
      opacity: 0.5;
    }

    .version-info {
      display: flex;
      gap: 12px;
      align-items: center;
    }

    .version-item {
      display: flex;
      align-items: center;
      gap: 4px;
    }

    .label {
      font-weight: normal;
      color: #ccc;
      font-size: 10px;
    }

    .value {
      color: #fff;
      font-size: 11px;
    }
  `;

  static properties = {
    version: { type: String },
    commit: { type: String },
    buildTime: { type: String },
    loading: { type: Boolean },
    error: { type: String },
  };

  constructor() {
    super();
    this.version = '';
    this.commit = '';
    this.buildTime = '';
    this.loading = false;
    this.error = '';
  }

  connectedCallback() {
    super.connectedCallback();
    this.fetchVersion();
  }

  async fetchVersion() {
    this.loading = true;
    this.error = '';
    
    try {
      const request = new GetVersionRequest();
      const response = await fetch('/api.v1.Version/GetVersion', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/grpc-web+proto',
        },
        body: new Uint8Array(0), // Empty request body
      });
      
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      
      // For now, this will fail and show blank (as intended)
      // In a real implementation, we'd parse the grpc-web response
      throw new Error('gRPC-web endpoint not available');
    } catch (error) {
      console.error('Failed to fetch version:', error);
      // Don't show fallback data - leave blank if not working
      this.version = '';
      this.commit = '';
      this.buildTime = '';
      this.error = error.message;
    } finally {
      this.loading = false;
    }
  }

  render() {
    // If there's an error or no data, don't show anything
    if (this.error || (!this.version && !this.commit && !this.buildTime && !this.loading)) {
      return html``;
    }

    return html`
      <div class="version-panel ${this.loading ? 'loading' : ''}">
        <div class="version-info">
          <div class="version-item">
            <span class="label">v</span>
            <span class="value">${this.version || '...'}</span>
          </div>
          <div class="version-item">
            <span class="label">@</span>
            <span class="value">${this.commit || '...'}</span>
          </div>
          <div class="version-item">
            <span class="label">built</span>
            <span class="value">${this.buildTime || '...'}</span>
          </div>
        </div>
      </div>
    `;
  }
}

customElements.define('version-display', VersionDisplay);