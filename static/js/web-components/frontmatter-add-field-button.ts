import { html, css, LitElement } from 'lit';
import { property, state } from 'lit/decorators.js';
import { buttonCSS, foundationCSS, menuCSS } from './shared-styles.js';

export class FrontmatterAddFieldButton extends LitElement {
  static override styles = [
    foundationCSS,
    buttonCSS,
    menuCSS,
    css`
      :host {
        position: relative;
        display: inline-block;
      }
    `
  ];

  @state()
  declare open: boolean;

  @property({ type: Boolean })
  declare disabled: boolean;

  constructor() {
    super();
    this.open = false;
    this.disabled = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('click', this._handleClickOutside);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('click', this._handleClickOutside);
  }

  private _handleClickOutside = (event: Event): void => {
    if (!this.open) return;

    if (event.target instanceof Node && !this.contains(event.target)) {
      this.open = false;
    }
  };

  private _handleToggleDropdown = (event: Event): void => {
    event.stopPropagation();
    if (!this.disabled) {
      this.open = !this.open;
    }
  };

  private _handleAddField = (): void => {
    this._dispatchAddEvent('field');
    this.open = false;
  };

  private _handleAddArray = (): void => {
    this._dispatchAddEvent('array');
    this.open = false;
  };

  private _handleAddSection = (): void => {
    this._dispatchAddEvent('section');
    this.open = false;
  };

  private _dispatchAddEvent(type: 'field' | 'array' | 'section'): void {
    this.dispatchEvent(new CustomEvent('add-field', {
      detail: { type },
      bubbles: true,
    }));
  }

  override render() {
    return html`
      <button 
        class="button-base button-primary button-small button-dropdown border-radius-small" 
        ?disabled="${this.disabled}"
        @click="${this._handleToggleDropdown}"
      >
        Add Field
        <span class="dropdown-arrow ${this.open ? 'open' : ''}">▼</span>
      </button>
      ${this.open ? html`
        <div class="dropdown-menu border-radius">
          <button class="dropdown-item" @click="${this._handleAddField}">Add Field</button>
          <button class="dropdown-item" @click="${this._handleAddArray}">Add Array</button>
          <button class="dropdown-item" @click="${this._handleAddSection}">Add Section</button>
        </div>
      ` : ''}
    `;
  }
}

customElements.define('frontmatter-add-field-button', FrontmatterAddFieldButton);

declare global {
  interface HTMLElementTagNameMap {
    'frontmatter-add-field-button': FrontmatterAddFieldButton;
  }
}