import { html, css, LitElement } from 'lit';

export class FrontmatterValueString extends LitElement {
  static override styles = css`
    :host {
      display: block;
    }

    .value-input {
      width: 100%;
      padding: 8px 12px;
      border: none;
      border-left: 1px solid #ddd;
      border-radius: 4px;
      font-size: 14px;
      font-family: inherit;
      box-sizing: border-box;
    }

    .value-input:focus {
      outline: none;
      border-left-color: #007bff;
      box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.1);
    }

    .value-input:disabled {
      background-color: #f8f9fa;
      color: #6c757d;
      cursor: not-allowed;
    }
  `;

  static override properties = {
    value: { type: String },
    placeholder: { type: String },
    disabled: { type: Boolean },
  };

  declare value: string;
  declare placeholder: string;
  declare disabled: boolean;

  constructor() {
    super();
    this.value = '';
    this.placeholder = '';
    this.disabled = false;
  }

  private _handleValueInput = (event: Event): void => {
    const target = event.target as HTMLInputElement;
    const newValue = target.value;
    const oldValue = this.value;

    // Don't update if the value hasn't actually changed
    if (newValue === oldValue) {
      return;
    }

    // Update the value property
    this.value = newValue;

    // Dispatch custom event with old and new values
    this.dispatchEvent(new CustomEvent('value-change', {
      detail: {
        oldValue,
        newValue,
      },
      bubbles: true,
    }));
  };

  override render() {
    return html`
      <input 
        type="text" 
        class="value-input"
        .value="${this.value}" 
        .placeholder="${this.placeholder}"
        .disabled="${this.disabled}"
        @blur="${this._handleValueInput}"
      />
    `;
  }
}

customElements.define('frontmatter-value-string', FrontmatterValueString);

declare global {
  interface HTMLElementTagNameMap {
    'frontmatter-value-string': FrontmatterValueString;
  }
}