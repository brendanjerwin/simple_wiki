import { html, css, LitElement } from 'lit';
import './frontmatter-value-string.js';
import './frontmatter-value-array.js';
import './frontmatter-value-section.js';

export class FrontmatterValue extends LitElement {
  static override styles = css`
    :host {
      display: block;
    }

    .empty-value-message {
      text-align: center;
      color: #666;
      font-style: italic;
      padding: 16px;
      border: 1px dashed #ddd;
      border-radius: 4px;
    }
  `;

  static override properties = {
    value: { type: Object },
    disabled: { type: Boolean },
    placeholder: { type: String },
  };

  declare value: unknown;
  declare disabled: boolean;
  declare placeholder: string;

  constructor() {
    super();
    this.value = null;
    this.disabled = false;
    this.placeholder = '';
  }

  private _handleValueChange = (event: CustomEvent): void => {
    const oldValue = this.value;
    let newValue: unknown;

    // Stop the event from bubbling further up since we'll re-dispatch it
    event.stopPropagation();

    // Extract the new value based on the event type
    if (event.type === 'value-change') {
      newValue = event.detail.newValue;
    } else if (event.type === 'array-change') {
      newValue = event.detail.newArray;
    } else if (event.type === 'section-change') {
      newValue = event.detail.newFields;
    } else {
      return; // Unknown event type
    }

    // Update our value
    this.value = newValue;

    // Debug logging for data structure changes
    console.log('[FrontmatterValue] Value delegated change:', {
      eventType: event.type,
      oldValue,
      newValue,
      valueType: Array.isArray(newValue) ? 'array' : typeof newValue
    });

    // Dispatch our own value-change event
    this.dispatchEvent(new CustomEvent('value-change', {
      detail: {
        oldValue,
        newValue,
      },
      bubbles: true,
    }));
  };

  private renderValueComponent() {
    // Handle null/undefined values
    if (this.value === null || this.value === undefined) {
      return html`
        <div class="empty-value-message">No value to display</div>
      `;
    }

    // Handle arrays
    if (Array.isArray(this.value)) {
      return html`
        <frontmatter-value-array
          .values="${this.value}"
          .disabled="${this.disabled}"
          .placeholder="${this.placeholder}"
          @array-change="${this._handleValueChange}"
        ></frontmatter-value-array>
      `;
    }

    // Handle objects (sections)
    if (typeof this.value === 'object') {
      return html`
        <frontmatter-value-section
          .fields="${this.value}"
          .disabled="${this.disabled}"
          @section-change="${this._handleValueChange}"
        ></frontmatter-value-section>
      `;
    }

    // Handle primitive types (string, number, boolean) as strings
    return html`
      <frontmatter-value-string
        .value="${String(this.value)}"
        .disabled="${this.disabled}"
        .placeholder="${this.placeholder}"
        @value-change="${this._handleValueChange}"
      ></frontmatter-value-string>
    `;
  }

  override render() {
    return this.renderValueComponent();
  }
}

customElements.define('frontmatter-value', FrontmatterValue);

declare global {
  interface HTMLElementTagNameMap {
    'frontmatter-value': FrontmatterValue;
  }
}