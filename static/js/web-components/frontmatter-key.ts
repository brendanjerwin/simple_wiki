import { html, css, LitElement } from 'lit';
import { property } from 'lit/decorators.js';

export class FrontmatterKey extends LitElement {
  static override styles = css`
    :host {
      display: inline-block;
    }

    .key-input {
      font-weight: 600;
      color: #333;
      background: transparent;
      border: none;
      border-left: 1px solid transparent;
      border-radius: 4px;
      padding: 4px 8px;
      font-size: 14px;
      font-family: inherit;
      box-sizing: border-box;
      cursor: pointer;
      transition: all 0.2s ease;
      text-decoration: underline;
      text-decoration-style: dashed;
      text-decoration-color: transparent;
    }

    .key-input:hover {
      background: #f8f9fa;
      border-left-color: #ddd;
      text-decoration-color: #999;
    }

    .key-input:focus {
      outline: none;
      border-left-color: #007bff;
      box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.1);
      background: white;
      cursor: text;
      text-decoration: none;
    }

    .key-display {
      font-weight: 600;
      color: #333;
      padding: 8px 12px;
      font-size: 14px;
      font-family: inherit;
      text-decoration: underline;
      text-decoration-style: solid;
      text-decoration-color: #333;
    }
  `;

  @property({ type: String })
  declare key: string;

  @property({ type: Boolean })
  declare editable: boolean;

  @property({ type: String })
  declare placeholder: string;

  constructor() {
    super();
    this.key = '';
    this.editable = false;
    this.placeholder = '';
  }

  private _handleKeyInput = (event: Event): void => {
    if (!(event.target instanceof HTMLInputElement)) {
      return;
    }
    const target = event.target;
    const newKey = target.value.trim();
    const oldKey = this.key;

    // Validation: don't allow empty or whitespace-only keys
    if (!newKey) {
      target.value = oldKey; // Revert the input
      return;
    }

    // Don't update if the key hasn't actually changed
    if (newKey === oldKey) {
      return;
    }

    // Update the key property
    this.key = newKey;

    // Dispatch custom event with old and new key values
    this.dispatchEvent(new CustomEvent('key-change', {
      detail: {
        oldKey,
        newKey,
      },
      bubbles: true,
    }));
  };

  override render() {
    if (this.editable) {
      return html`
        <input 
          type="text" 
          class="key-input"
          .value="${this.key}" 
          .placeholder="${this.placeholder}"
          @blur="${this._handleKeyInput}"
        />
      `;
    }

    return html`
      <span class="key-display">${this.key}</span>
    `;
  }
}

customElements.define('frontmatter-key', FrontmatterKey);

declare global {
  interface HTMLElementTagNameMap {
    'frontmatter-key': FrontmatterKey;
  }
}