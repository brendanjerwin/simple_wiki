import { html, css, LitElement } from '/static/js/lit-all.min.js';

export class WikiSearch extends LitElement {
    static styles = css`
    div#container {
        position: relative;
        display: inline-block;
        padding: 0;
        margin: 0;
        max-width: 100%;
    }
    form { 
        display: flex;
        justify-content: center;
        padding: 1px;
        width: 100%;
        max-width: 500px;
        box-sizing: border-box;
    }
    input[type="search"] {
        flex-grow: 1 1 auto;
        padding: 5px;
        border: none;
        border-radius: 5px 0 0 5px;
        outline: none;
        font-size: 16px;
        max-width: 100%;
    }
    button {
        padding: 5px 15px;
        border: none;
        background-color: #6c757d;
        color: white;
        cursor: pointer;
        border-radius: 0 5px 5px 0;
        font-size: 16px;
        transition: background-color 0.3s ease;
    }
    button:hover {
        background-color: #9da5ab;
    }
    `;

    static properties = {
        search_endpoint: { type: String },
    };

    constructor() {
        super();
    }

    render() {
        return html`
    <div id="container">
        <form action="/search" method="get">
            <input type="search" name="q" placeholder="Search..." required>
            <button type="submit">Search</button>
        </form>
    </div>
        `;
    }
}
customElements.define('wiki-search', WikiSearch);
