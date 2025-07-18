import { html, css, LitElement } from 'lit';

export class FrontmatterAddFieldButton extends LitElement {
  static override styles = css`
      :host {
        position: relative;
        display: inline-block;
      }

      .dropdown-button {
        padding: 8px 16px;
        font-size: 14px;
        border: 1px solid #28a745;
        border-radius: 4px;
        cursor: pointer;
        background: #28a745;
        color: white;
        display: flex;
        align-items: center;
        gap: 8px;
        font-weight: 500;
        transition: all 0.2s ease;
      }

      .dropdown-button:hover {
        background: #218838;
        border-color: #218838;
        transform: translateY(-1px);
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
      }

      .dropdown-button:active {
        transform: translateY(0);
      }

      .dropdown-menu {
        position: absolute;
        top: 100%;
        left: 0;
        background: white;
        border: 1px solid #ddd;
        border-radius: 4px;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
        z-index: 1000;
        min-width: 150px;
        margin-top: 4px;
      }

      .dropdown-item {
        padding: 10px 16px;
        cursor: pointer;
        border: none;
        background: none;
        width: 100%;
        text-align: left;
        font-size: 14px;
        color: #333;
        transition: background-color 0.2s ease;
      }

      .dropdown-item:hover {
        background: #f8f9fa;
      }

      .dropdown-item:first-child {
        border-radius: 4px 4px 0 0;
      }

      .dropdown-item:last-child {
        border-radius: 0 0 4px 4px;
      }

      .dropdown-arrow {
        transition: transform 0.2s ease;
      }

      .dropdown-arrow.open {
        transform: rotate(180deg);
      }
    `;

  static override properties = {
    open: { type: Boolean, state: true },
    disabled: { type: Boolean },
  };

  declare open: boolean;
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
    
    if (!this.contains(event.target as Node)) {
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
        class="dropdown-button" 
        ?disabled="${this.disabled}"
        @click="${this._handleToggleDropdown}"
      >
        Add Field
        <span class="dropdown-arrow ${this.open ? 'open' : ''}">▼</span>
      </button>
      ${this.open ? html`
        <div class="dropdown-menu">
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