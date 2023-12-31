import { html, css, LitElement, unsafeHTML } from '/static/js/lit-all.min.js';

export class MarkdownViewer extends LitElement {
    static get styles() {
        return css`
            :host {
            display: block;
            }
        article.markdown-body {
            padding: .35em;
            width: 100%;
            height: 710px;
            box-sizing: border-box;
            overflow: auto !important;
        }
        `;
    }

    static get properties() {
        return {
        markdown: { type: String },
        };
    }

    render() {
        return html`
        <link href="/static/css/fontawesome.min.css" rel="stylesheet">
        <link href="/static/css/solid.min.css" rel="stylesheet"> 
        <link rel="stylesheet" type="text/css" href="/static/css/github-markdown.css">
        <article class="markdown-body">
            ${unsafeHTML(this.markdown)}
        </article>
        `;
    }
}

customElements.define('markdown-viewer', MarkdownViewer);