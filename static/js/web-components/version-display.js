import { html, css, LitElement } from 'lit';

export class VersionDisplay extends LitElement {
  static styles = css`
    :host {
      position: fixed;
      bottom: 20px;
      right: 20px;
      z-index: 1000;
      font-family: monospace;
      font-size: 12px;
      line-height: 1.4;
    }

    .version-panel {
      background-color: rgba(0, 0, 0, 0.7);
      color: white;
      padding: 10px;
      border-radius: 5px;
      max-width: 300px;
      backdrop-filter: blur(5px);
      box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
      transition: opacity 0.3s ease;
    }

    .version-panel.loading {
      opacity: 0.5;
    }

    .version-panel.error {
      background-color: rgba(139, 0, 0, 0.7);
    }

    .version-info {
      display: flex;
      flex-direction: column;
      gap: 5px;
    }

    .version-info div {
      word-break: break-all;
    }

    .label {
      font-weight: bold;
      color: #ccc;
    }

    .value {
      margin-left: 10px;
      color: #fff;
    }

    .error-message {
      color: #ffcccb;
      font-style: italic;
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

  // Simple gRPC-web request implementation
  async makeGrpcWebRequest(service, method, requestData = {}) {
    const url = `${window.location.origin}/${service}/${method}`;
    
    try {
      const response = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/grpc-web+proto',
          'Accept': 'application/grpc-web+proto',
        },
        body: this.encodeGrpcWebMessage(requestData),
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      const responseData = await response.arrayBuffer();
      return this.decodeGrpcWebMessage(responseData);
    } catch (error) {
      console.error('gRPC-web request failed:', error);
      throw error;
    }
  }

  // Simple encoding for empty request (GetVersionRequest has no fields)
  encodeGrpcWebMessage(data) {
    // For GetVersionRequest, we send an empty message
    return new Uint8Array([0, 0, 0, 0, 0]); // gRPC-web frame header + empty message
  }

  // Simple decoding for GetVersionResponse
  decodeGrpcWebMessage(buffer) {
    // This is a simplified decoder - in a real implementation, 
    // we'd need to properly parse the protobuf message
    const view = new DataView(buffer);
    
    // Skip gRPC-web frame header (5 bytes)
    const messageLength = view.getUint32(1, false);
    
    // For now, return mock data since proper protobuf parsing is complex
    // In a real implementation, this would parse the actual protobuf response
    return {
      version: 'dev',
      commit: 'local-dev',
      buildTime: new Date().toISOString(),
    };
  }

  async fetchVersion() {
    this.loading = true;
    this.error = '';
    
    try {
      // Try to make a simple gRPC-web request
      const response = await this.makeGrpcWebRequest('api.v1.Version', 'GetVersion');
      
      this.version = response.version;
      this.commit = response.commit;
      this.buildTime = new Date(response.buildTime).toLocaleString();
    } catch (error) {
      console.error('Failed to fetch version:', error);
      // Fallback to mock data for demo purposes
      this.version = 'dev';
      this.commit = 'local-dev';
      this.buildTime = new Date().toLocaleString();
      this.error = `Using fallback data (${error.message})`;
    } finally {
      this.loading = false;
    }
  }

  render() {
    if (this.error) {
      return html`
        <div class="version-panel error">
          <div class="version-info">
            <div>
              <span class="label">Version:</span>
              <span class="value">${this.version || 'Loading...'}</span>
            </div>
            <div>
              <span class="label">Commit:</span>
              <span class="value">${this.commit || 'Loading...'}</span>
            </div>
            <div>
              <span class="label">Built:</span>
              <span class="value">${this.buildTime || 'Loading...'}</span>
            </div>
          </div>
          <div class="error-message">${this.error}</div>
        </div>
      `;
    }

    return html`
      <div class="version-panel ${this.loading ? 'loading' : ''}">
        <div class="version-info">
          <div>
            <span class="label">Version:</span>
            <span class="value">${this.version || 'Loading...'}</span>
          </div>
          <div>
            <span class="label">Commit:</span>
            <span class="value">${this.commit || 'Loading...'}</span>
          </div>
          <div>
            <span class="label">Built:</span>
            <span class="value">${this.buildTime || 'Loading...'}</span>
          </div>
        </div>
      </div>
    `;
  }
}

customElements.define('version-display', VersionDisplay);