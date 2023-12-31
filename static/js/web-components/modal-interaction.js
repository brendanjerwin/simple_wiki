import { html, css, LitElement } from '/static/js/lit-all.min.js';

export class ModalInteraction extends LitElement {
    static styles = css`
        :host {
            display: none;
            position: fixed;
            z-index: 1;
            left: 0;
            top: 0;
            width: 100%;
            height: 100%;
            overflow: auto;
            background-color: rgba(0,0,0,0.4);
        }
        .modal-content #spinner {
            z-index: 10000;
            position: absolute;
            top: 0;
            right: 0;
            bottom: 0;
            left: 0;
            display: flex;
            justify-content: center;
            align-items: center;
            background-color: rgba(255, 255, 255, 0.8);
            border-radius: 10px;
        }
        .modal-content #spinner[hidden] {
            display: none !important;
        }
        .modal-content #spinner .fas {
            font-size: 5em;
            animation: spin 1s linear infinite;
        }

        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        .modal-content {
            border: 1px solid #888;
            display: flex;
            flex-direction: column;
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            width: 100%;
            max-height: 95%;
            max-width: 95%;
            border-radius: 10px;
            box-shadow: 0px 5px 15px rgba(0, 0, 0, 0.3);
            z-index: 9999;
            background-color: white;
        }
        .title-bar {
            display: flex;
            justify-content: space-between;
            align-items: center;
            border-top-right-radius: 10px;
            border-top-left-radius: 10px;
            background-color: #f8f8f8;
            padding: 10px;
            border-bottom: 1px solid #e8e8e8;
        }
        .title-bar h2 {
            font-size: 16px;
            margin: 0;
        }
    `;

    static get properties() {
        return {
            title: { type: String },
            icon: { type: String, attribute: 'fa-icon' },
            busy: { type: Boolean, reflect: true },
        };
    }

    constructor() {
        super();
        this.title = 'Modal Interaction';
    }

    randomSpinner() {
        const spinners = [
            'fa-spinner',
            'fa-gear',
            'fa-circle-notch',
            'fa-rotate-right',
            'fa-yin-yang',
            'fa-stroopwafel',
            'fa-hurricane',
            'fa-robot',
            'fa-atom',
            'fa-wrench'
        ];
        return spinners[Math.floor(Math.random() * spinners.length)];
    }

    render() {
        return html`
            <link href="/static/css/fontawesome.min.css" rel="stylesheet">
            <link href="/static/css/solid.min.css" rel="stylesheet">
            <div class="modal-content">
                <div id="spinner" .hidden="${!this.busy}"><i class="fas ${this.randomSpinner()}"></i></div>
                <div class="title-bar">
                    <h2><i class="fa-solid fa-${this.icon}"></i> ${this.title}</h2>
                </div>
                <slot></slot>
            </div>
        `;
    }

    firstUpdated() {
        this.addEventListener('modal-closed', () => this.style.display = 'none');
        this.addEventListener('modal-busy', this.handleModalBusy)
        this.addEventListener('modal-not-busy', this.handleModalNotBusy)
    }

    handleModalBusy() {
        this.busy = true;
    }

    handleModalNotBusy() {
        this.busy = false;
    }
}

customElements.define('modal-interaction', ModalInteraction);