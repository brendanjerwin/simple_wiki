import { html, css, LitElement, nothing } from 'lit';
import { TailscaleIdentity } from '../gen/api/v1/system_info_pb.js';
import { foundationCSS } from './shared-styles.js';

export class SystemInfoIdentity extends LitElement {
  static override styles = [
    foundationCSS,
    css`
      :host {
        display: block;
        font-size: 11px;
        line-height: 1.2;
      }

      .identity-info {
        display: flex;
        flex-direction: row;
        align-items: center;
        gap: 8px;
        border-top: 1px solid #404040;
        padding-top: 4px;
        margin-top: 2px;
      }

      .identity-row {
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
        color: #7bed9f;
        font-size: 10px;
      }

      .tailscale-icon {
        width: 12px;
        height: 12px;
        opacity: 0.8;
      }
    `];

  static override properties = {
    identity: { type: Object },
  };

  declare identity?: TailscaleIdentity;

  constructor() {
    super();
  }

  override render() {
    // Don't render anything if no identity
    if (!this.identity || !this.identity.loginName) {
      return nothing;
    }

    return html`
      <div class="identity-info">
        <div class="identity-row">
          <span class="label">User:</span>
          <span class="value">${this.identity.displayName || this.identity.loginName}</span>
        </div>
        ${this.identity.nodeName ? html`
          <div class="identity-row">
            <span class="label">Node:</span>
            <span class="value">${this.identity.nodeName}</span>
          </div>
        ` : nothing}
      </div>
    `;
  }
}

customElements.define('system-info-identity', SystemInfoIdentity);

declare global {
  interface HTMLElementTagNameMap {
    'system-info-identity': SystemInfoIdentity;
  }
}
