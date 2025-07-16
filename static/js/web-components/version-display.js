import { html, css, LitElement } from 'lit';

// Constants for the gRPC-web endpoint
export const GRPC_WEB_ENDPOINT = '/api.v1.Version/GetVersion';

// Simple protobuf parser for the GetVersionResponse
function parseGetVersionResponse(buffer) {
  const view = new DataView(buffer);
  let offset = 0;
  
  // Skip gRPC-web frame header (5 bytes)
  offset += 5;
  
  const result = {
    version: '',
    commit: '',
    buildTime: null
  };
  
  // Parse protobuf message
  while (offset < buffer.byteLength - 5) { // -5 for trailer
    const tag = view.getUint8(offset++);
    const fieldNumber = tag >> 3;
    const wireType = tag & 0x07;
    
    if (wireType === 2) { // Length-delimited
      const length = view.getUint8(offset++);
      const value = new TextDecoder().decode(new Uint8Array(buffer, offset, length));
      offset += length;
      
      switch (fieldNumber) {
        case 1: // version
          result.version = value;
          break;
        case 2: // commit
          result.commit = value;
          break;
        case 3: // build_time (timestamp)
          // For now, skip timestamp parsing
          break;
      }
    }
  }
  
  return result;
}

export class VersionDisplay extends LitElement {
  // Simple protobuf parser for the GetVersionResponse
  static parseGetVersionResponse(buffer) {
    const view = new DataView(buffer);
    let offset = 0;
    
    // Skip gRPC-web frame header (5 bytes)
    offset += 5;
    
    const result = {
      version: '',
      commit: '',
      buildTime: null
    };
    
    // Parse protobuf message
    while (offset < buffer.byteLength - 5) { // -5 for trailer
      const tag = view.getUint8(offset++);
      const fieldNumber = tag >> 3;
      const wireType = tag & 0x07;
      
      if (wireType === 2) { // Length-delimited
        const length = view.getUint8(offset++);
        const value = new TextDecoder().decode(new Uint8Array(buffer, offset, length));
        offset += length;
        
        switch (fieldNumber) {
          case 1: // version
            result.version = value;
            break;
          case 2: // commit
            result.commit = value;
            break;
          case 3: // build_time (timestamp)
            // For now, skip timestamp parsing
            break;
        }
      }
    }
    
    return result;
  }

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
      // Create a properly formatted gRPC-web request
      const request = new Uint8Array([
        0x00, // gRPC-web frame header: uncompressed message
        0x00, 0x00, 0x00, 0x00 // message length: 0 (empty GetVersionRequest)
      ]);
      
      const response = await fetch(GRPC_WEB_ENDPOINT, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/grpc-web+proto',
          'Accept': 'application/grpc-web+proto'
        },
        body: request
      });
      
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      
      // Parse the gRPC-web response
      const data = await response.arrayBuffer();
      const result = VersionDisplay.parseGetVersionResponse(data);
      
      // Update the component with the response
      this.version = result.version;
      this.commit = result.commit;
      this.buildTime = result.buildTime ? new Date(result.buildTime).toISOString() : '';
      
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