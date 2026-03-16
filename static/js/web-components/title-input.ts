import { html, css, LitElement } from 'lit';
import { property } from 'lit/decorators.js';
import { live } from 'lit/directives/live.js';
import { dialogCSS } from './shared-styles.js';

const LOWERCASE_WORDS = new Set([
  'a', 'an', 'the', 'and', 'but', 'or', 'for', 'nor',
  'in', 'on', 'at', 'to', 'by', 'of', 'up', 'as', 'is',
]);

/**
 * Converts a string to title case.
 * Capitalizes the first letter of each word, except for common articles,
 * conjunctions, and prepositions (unless they are the first or last word).
 */
export function toTitleCase(text: string): string {
  const words = text.split(/(\s+)/);
  const contentWords = words.filter(w => w.trim().length > 0);
  const lastContentIndex = words.length - 1 - [...words].reverse().findIndex(w => w.trim().length > 0);

  return words.map((word, index) => {
    if (word.trim().length === 0) return word;

    const isFirst = index === words.findIndex(w => w.trim().length > 0);
    const isLast = index === lastContentIndex;
    const lower = word.toLowerCase();

    if (!isFirst && !isLast && LOWERCASE_WORDS.has(lower)) {
      return lower;
    }

    if (contentWords.length <= 1) {
      return word.charAt(0).toUpperCase() + word.slice(1);
    }

    return word.charAt(0).toUpperCase() + word.slice(1).toLowerCase();
  }).join('');
}

/**
 * TitleInput - A text input that automatically applies title casing and trims whitespace.
 *
 * @fires input - Standard input event, value is title-cased.
 */
export class TitleInput extends LitElement {
  static override shadowRootOptions: ShadowRootInit = {
    ...LitElement.shadowRootOptions,
    delegatesFocus: true,
  };

  static override styles = [
    dialogCSS,
    css`
      :host {
        display: block;
      }

      input {
        width: 100%;
        padding: 8px;
        border: 1px solid #ddd;
        border-radius: 4px;
        font-size: 1em;
        box-sizing: border-box;
      }
    `,
  ];

  @property({ type: String })
  declare value: string;

  @property({ type: String })
  declare placeholder: string;

  @property({ type: Boolean })
  declare disabled: boolean;

  @property({ type: String })
  declare name: string;

  @property({ type: String })
  declare id: string;

  constructor() {
    super();
    this.value = '';
    this.placeholder = 'Enter a title';
    this.disabled = false;
    this.name = '';
    this.id = '';
  }

  /** Focus the input element. */
  public focusInput(): void {
    const input = this.shadowRoot?.querySelector<HTMLInputElement>('input');
    input?.focus();
  }

  private _onInput(e: Event): void {
    if (!(e.target instanceof HTMLInputElement)) return;
    const input = e.target;

    const cursorPos = input.selectionStart ?? input.value.length;
    const titleCased = toTitleCase(input.value);
    this.value = titleCased;
    input.value = titleCased;

    // Restore cursor position after title-casing
    input.setSelectionRange(cursorPos, cursorPos);

    this.dispatchEvent(new Event('input', { bubbles: true, composed: true }));
  }

  private _onBlur(e: Event): void {
    if (!(e.target instanceof HTMLInputElement)) return;
    const trimmed = this.value.trim();
    if (trimmed !== this.value) {
      this.value = trimmed;
      e.target.value = trimmed;
      this.dispatchEvent(new Event('input', { bubbles: true, composed: true }));
    }
  }

  override render() {
    return html`
      <input
        type="text"
        .value=${live(this.value)}
        @input=${this._onInput}
        @blur=${this._onBlur}
        placeholder=${this.placeholder}
        ?disabled=${this.disabled}
        name=${this.name}
        id=${this.id}
      />
    `;
  }
}

customElements.define('title-input', TitleInput);

declare global {
  interface HTMLElementTagNameMap {
    'title-input': TitleInput;
  }
}
