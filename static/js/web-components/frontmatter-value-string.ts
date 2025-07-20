import { html, css, LitElement } from 'lit';
import { inputCSS } from './shared-styles.js';

export class FrontmatterValueString extends LitElement {
  static override styles = [
    inputCSS,
    css`
      :host {
        display: block;
      }

      .value-input {
        /* Uses .input-base from shared styles */
      }
    `
  ];

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
        class="value-input input-base"
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