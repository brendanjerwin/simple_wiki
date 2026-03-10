import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';

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

    .saved-msg {
      font-size: 11px;
      color: #2e7d32;
      margin-top: 4px;
    }
  `;

  @property()
  declare wikiUrl: string;

  @state()
  declare showSaved: boolean;

  constructor() {
    super();
    this.wikiUrl = 'http://localhost:8050';
    this.showSaved = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    this.loadUrl();
  }

  private async loadUrl(): Promise<void> {
    const stored = await browser.storage.local.get('wikiUrl');
    if (stored['wikiUrl']) {
      this.wikiUrl = stored['wikiUrl'] as string;
    }
  }

  private async handleSave(): Promise<void> {
    await browser.storage.local.set({ wikiUrl: this.wikiUrl });
    this.showSaved = true;
    setTimeout(() => { this.showSaved = false; }, 2000);
  }

  private handleInput(e: Event): void {
    this.wikiUrl = (e.target as HTMLInputElement).value;
  }

  override render() {
    return html`
      <div class="settings-row">
        <label>Wiki URL:</label>
        <input
          type="url"
          .value=${this.wikiUrl}
          @input=${this.handleInput}
          placeholder="http://localhost:8050"
        />
        <button @click=${this.handleSave}>Save</button>
      </div>
      ${this.showSaved ? html`<div class="saved-msg">Saved</div>` : ''}
    `;
  }
}
