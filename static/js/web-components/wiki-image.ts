import { html, css, LitElement, TemplateResult } from 'lit';
import { property } from 'lit/decorators.js';

/**
 * WikiImage - Displays images from wiki content with click-to-open behavior.
 *
 * @element wiki-image
 *
 * @property {string} src - The image source URL
 * @property {string} alt - Alternative text for the image
 * @property {string} title - Optional title/tooltip for the image
 */
export class WikiImage extends LitElement {
  static override styles = css`
    :host {
      display: block;
      text-align: center;
      margin: 1.5em 0;
    }

    img {
      max-width: 80%;
      max-height: 70vh;
      object-fit: contain;
      border-radius: 8px;
      box-shadow: 2px 2px 5px rgba(0, 0, 0, 0.15);
      cursor: pointer;
      transition: box-shadow 0.2s ease;
    }

    img:hover {
      box-shadow: 2px 2px 8px rgba(0, 0, 0, 0.25);
    }
  `;

  @property({ type: String })
  src = '';

  @property({ type: String })
  alt = '';

  @property({ type: String })
  title?: string;

  private _handleClick(): void {
    window.open(this.src, '_blank');
  }

  override render(): TemplateResult {
    return html`
      <img
        src="${this.src}"
        alt="${this.alt}"
        title="${this.title || ''}"
        @click="${this._handleClick}"
      />
    `;
  }
}

customElements.define('wiki-image', WikiImage);

declare global {
  interface HTMLElementTagNameMap {
    'wiki-image': WikiImage;
  }
}
