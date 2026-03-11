import { LitElement, html, css, nothing } from 'lit';
import { customElement, state } from 'lit/decorators.js';

@customElement('settings-panel')
export class SettingsPanel extends LitElement {
  static override styles = css`
    :host {
      display: block;
      margin-bottom: 12px;
    }

    .settings-row {
      display: flex;
      gap: 8px;
      align-items: center;
    }

    label {
      font-size: 12px;
      color: #555;
      white-space: nowrap;
    }

    input {
      flex: 1;
      padding: 4px 8px;
      border: 1px solid #ccc;
      border-radius: 4px;
      font-size: 13px;
    }

    button {
      padding: 4px 12px;
      border: 1px solid #ccc;
      border-radius: 4px;
      cursor: pointer;
      font-size: 12px;
      background: #fff;
    }

    button:hover {
      background: #f5f5f5;
    }

    .status-msg {
      font-size: 11px;
      margin-top: 4px;
    }

    .status-msg.saved {
      color: #2e7d32;
    }

    .status-msg.auto-detected {
      color: #1565c0;
    }

    .reset-link {
      font-size: 11px;
      color: #1565c0;
      cursor: pointer;
      text-decoration: underline;
      background: none;
      border: none;
      padding: 0;
      margin-top: 4px;
    }
  `;

  @state()
  declare wikiUrl: string;

  @state()
  declare showSaved: boolean;

  @state()
  declare isManuallySet: boolean;

  @state()
  declare showAutoDetected: boolean;

  private _storageListener = this._handleStorageChanged.bind(this);

  constructor() {
    super();
    this.wikiUrl = 'http://localhost:8050';
    this.showSaved = false;
    this.isManuallySet = false;
    this.showAutoDetected = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    this._loadSettings();
    browser.storage.onChanged.addListener(this._storageListener);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    browser.storage.onChanged.removeListener(this._storageListener);
  }

  private _handleStorageChanged(changes: Record<string, browser.storage.StorageChange>, areaName: string): void {
    if (areaName !== 'local') return;

    if (changes['wikiUrl']?.newValue !== undefined) {
      this.wikiUrl = changes['wikiUrl'].newValue as string;
      if (!this.isManuallySet) {
        this.showAutoDetected = true;
        setTimeout(() => { this.showAutoDetected = false; }, 3000);
      }
    }

    if (changes['wikiUrlManuallySet']?.newValue !== undefined) {
      this.isManuallySet = changes['wikiUrlManuallySet'].newValue as boolean;
    }
  }

  private async _loadSettings(): Promise<void> {
    const stored = await browser.storage.local.get(['wikiUrl', 'wikiUrlManuallySet']);
    if (stored['wikiUrl']) {
      this.wikiUrl = stored['wikiUrl'] as string;
    }
    this.isManuallySet = stored['wikiUrlManuallySet'] === true;
  }

  private async _handleSave(): Promise<void> {
    await browser.storage.local.set({ wikiUrl: this.wikiUrl, wikiUrlManuallySet: true });
    this.isManuallySet = true;
    this.showSaved = true;
    setTimeout(() => { this.showSaved = false; }, 2000);
  }

  private async _handleResetToAutoDetect(): Promise<void> {
    await browser.storage.local.remove('wikiUrlManuallySet');
    this.isManuallySet = false;
  }

  private _handleInput(e: Event): void {
    this.wikiUrl = (e.target as HTMLInputElement).value;
  }

  override render() {
    return html`
      <div class="settings-row">
        <label>Wiki URL:</label>
        <input
          type="url"
          .value=${this.wikiUrl}
          @input=${this._handleInput}
          placeholder="http://localhost:8050"
        />
        <button @click=${this._handleSave}>Save</button>
      </div>
      ${this.showSaved ? html`<div class="status-msg saved">Saved (manual)</div>` : nothing}
      ${this.showAutoDetected ? html`<div class="status-msg auto-detected">Auto-detected from wiki</div>` : nothing}
      ${this.isManuallySet ? html`<button class="reset-link" @click=${this._handleResetToAutoDetect}>Reset to auto-detect</button>` : nothing}
    `;
  }
}
