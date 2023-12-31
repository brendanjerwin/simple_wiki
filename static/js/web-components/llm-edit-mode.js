import { html, css, LitElement } from '/static/js/lit-all.min.js';
import { ModalInteraction } from './modal-interaction.js';
import { LlmEditExperience } from './llm-edit-experience.js';

export class LlmEditMode extends LitElement {
    static styles = css`
        :host([enabled]) modal-interaction {
            display: block;
        }
    `;
   
    static get properties() {
        return {
            enabled: { type: Boolean, reflect: true, attribute: 'enabled' },
            pageIdentifier: { type: String }
        };
    }

    connectedCallback() {
        super.connectedCallback();
        window.addEventListener('llm-edit-mode-invoked', this.handleModeInvoked.bind(this));
        this.addEventListener('modal-closed', this.handleModalClosed.bind(this));
    }

    disconnectedCallback() {
        super.disconnectedCallback();
        window.removeEventListener('llm-edit-mode-invoked', this.handleModeInvoked.bind(this));
        this.removeEventListener('modal-closed', this.handleModalClosed.bind(this));
    }

    handleModeInvoked(e) {
        this.pageIdentifier = e.detail.pageIdentifier;
        this.enabled = true;
    }

    handleModalClosed() {
        this.enabled = false;
        this.dispatchEvent(new CustomEvent('llm-edit-mode-exited', {composed: true, bubbles: true }));
    }

    render() {
    return html`
        ${this.enabled ? html`
        <modal-interaction id="modalInteraction" title="LLM Edit" fa-icon="robot">
            <llm-edit-experience pageIdentifier="${this.pageIdentifier}"></llm-edit-experience>
        </modal-interaction>
        ` : ''}
    `;}
}

customElements.define('llm-edit-mode', LlmEditMode);