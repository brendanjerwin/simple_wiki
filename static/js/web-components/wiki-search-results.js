import { html, css, LitElement } from '/static/js/lit-all.min.js';

class WikiSearchResults extends LitElement {
    static styles = css`
        :host {
            display: block;
            position: relative;
        }
        .popover {
            display: none;
            position: fixed;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            border-radius: 10px;
            box-shadow: 0px 5px 15px rgba(0, 0, 0, 0.3);
            z-index: 9999;
            background-color: white;
        }
        a {
            display: block;
            padding: 5px;
            margin: 5px;
            text-decoration: none;
            border-radius: 5px;
            transition: background-color 0.3s ease;
            cursor: pointer;
        }
        .popover:not(:hover) a:focus {
            outline: 2px solid #4d90fe;
        }
        a:hover {
            outline: 2px solid #4d90fe;
        }
        .title-bar {
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
        :host([open]) .popover {
            display: block;
        }
    `;

    static properties = {
        results: { type: Array },
        open: { type: Boolean, reflect: true }
    };

    constructor() {
        super();
        this.results = [];
        this.open = false;
    }

    connectedCallback() {
        super.connectedCallback();
        document.addEventListener('click', this.handleClickOutside.bind(this));
    }

    disconnectedCallback() {
        document.removeEventListener('click', this.handleClickOutside.bind(this));
        super.disconnectedCallback();
    }

    handleClickOutside(event) {
        const path = event.composedPath();
        if (this.open && !path.includes(this.shadowRoot.querySelector('.popover'))) {
            this.open = false;
            this.dispatchEvent(new CustomEvent('search-results-closed', {
                bubbles: true,
                composed: true
            }));
        }
    }

    handlePopoverClick(event) {
        // Stop the click event from bubbling up to the document
        event.stopPropagation();
    }

    updated(changedProperties) {
        if (changedProperties.has('results') && this.results.length > 0) {
            const firstLink = this.shadowRoot.querySelector('a');
            if (firstLink) {
                firstLink.focus();
            }
        }
    }

    render() {
        return html`
            <div class="popover" @click="${this.handlePopoverClick}">
                <div class="title-bar">
                    <h2>Search Results</h2>
                </div>
                ${this.results.map(result => html`<a href="/${result.Identifier}">${result.Title}</a>`)}
            </div>
        `;
    }
}

customElements.define('wiki-search-results', WikiSearchResults);