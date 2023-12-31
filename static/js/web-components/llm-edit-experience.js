import { html, css, LitElement, unsafeHTML } from '/static/js/lit-all.min.js';
import { MarkdownViewer } from '/static/js/web-components/markdown-viewer.js';

export class LlmEditExperience extends LitElement {

    static styles = css`
        :host {
            display: block;
            height: 100%;
            width: 100%;
            border-radius: 10px;
        }

        div.outer {
            display: flex;
            flex-direction: column;
            height: 100%;
            width: 100%;
        }

        div.content {
            width: 100%;
            display: flex;
            flex-direction: row;

            height: 710px; //Needs media query to make this responsive and still scroll
        }

        div.left {
            height: 100%;
            width: 30%;
            border-right: 1px solid #e8e8e8;

            display: flex;
            flex-direction: column;
        }

        div.right {
            height: 100%;
            width: 70%;
        }

        div#footer {
            display: flex;
            flex-direction: row;
            justify-content: flex-end;
            align-items: center;
            width: 100%;
            height: 50px;
            background-color: #f8f8f8;
            border-top: 1px solid #e8e8e8;
            border-bottom-right-radius: 10px;
            border-bottom-left-radius: 10px;
        }
        button#cancel {
            border: none;
            outline: none;
            background-color: transparent;
            cursor: pointer;
            font-size: 18px;
            padding: 0;
            margin: .35em;
            margin-right: 1.5em;
            transition: transform 0.3s ease;
        }
        button#cancel:hover {
            transform: scale(1.1);
        }
        button#cancel:active {  
            transform: scale(0.9);
        }
        button#save {
            border: none;
            outline: none;
            background-color: transparent;
            cursor: pointer;
            font-size: 18px;
            padding: 0;
            margin: .35em;
            margin-right: 1.5em;
            transition: transform 0.3s ease;
        }
        button#save:not(:disabled):hover {
            transform: scale(1.1);
        }
        button#save:not(:disabled):active {
            transform: scale(0.9);
        }

        ol#chat-history {
            flex-grow: 1;
            overflow-y: auto;
            padding: 10px;
            height: 100%;
        }

        div#chat-input {
            display: flex;
            flex-direction: row;
            align-items: center;
            border-top: 1px solid #e8e8e8;
            width: 100%;
        }
        textarea#chat-input-text {
            flex-grow: 1;
            resize: none;
            border: none;
            margin: .25em;
            outline: none;
            scroll-behavior: smooth;
            font-size: 16px;
            height: 3em;
            overflow: auto;
            scrollbar-width: none; /* For Firefox */
            -ms-overflow-style: none; /* For Internet Explorer and Edge */
        }

        textarea#chat-input-text::-webkit-scrollbar {
            width: 0px; /* For Chrome, Safari, and Opera */
        }

        button#chat-input-submit {
            border: none;
            outline: none;
            background-color: transparent;
            cursor: pointer;
            font-size: 18px;
            padding: 0;
            margin: .35em;
            transition: transform 0.3s ease;
        }

        button#chat-input-submit:hover {
            transform: scale(1.1);
        }

        button#chat-input-submit:active {
            transform: scale(0.9);
        }

        ol#chat-history li {
            list-style-type: none;
            margin-bottom: 10px;
            border-radius: 20px;
            border: 1px solid #333333;
        }

        ol#chat-history li.actor-user {
            background-color: #4682B4;
            margin-left: 20%; 
            padding: 10px; 
            color: #ffffff; 
            text-align: right;
        }
        
        ol#chat-history li.actor-robot {
            background-color: #808080; 
            margin-right: 20%; 
            padding: 10px; 
            color: #ffffff; 
            text-align: left;
        }

        ol#chat-history li.actor-bug {
            background-color: #8B0000; 
            padding: 10px;
            color: #ffffff;
            text-align: center;
        }
    `;

    static properties = {
        pageIdentifier: { type: String },
        chatHistory: { type: Array },
        currentInteraction: { type: Object },
        chatInputText: { type: String },
        currentNewContentHtml: { type: String },
    };

    constructor() {
        super();
        this.chatHistory = [
            { actor: 'robot', text: 'Hello! I\'m the LLM Edit Robot. I\'m here to help you edit this page. Just tell me what change you\'d like to make.'}
        ];
        this.currentInteraction = null;
        this.chatInputText = '';
    }

    handleChatInputTextChange(e) {
        this.chatInputText = e.target.value;
    }

    handleChatInputKeyDown(e) {
        if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
            this.handleChatSend();
            e.preventDefault(); // Prevents the default action (line break in this case)
        }
    }

    handleChatSend() {
        if (this.chatInputText) {
            var inputText = this.chatInputText;
            this.chatHistory = [...this.chatHistory, { actor: 'user', text: inputText }];
            this.chatInputText = '';

            this.sendChatMessage(inputText);
        }
    }

    handleCancelButtonClick() {
        if (this.currentInteraction) {
            var isSure = confirm('Are you sure you want to cancel this edit?');
            if (isSure) {
                this.emitModalClosedEvent();
            }
        } else {
            this.emitModalClosedEvent();
        }
    }

    emitModalClosedEvent() {
        this.dispatchEvent(new CustomEvent('modal-closed', {composed: true, bubbles: true }));
    }

    handleSaveButtonClick() {
        if (this.currentInteraction) {
            this.saveInteraction();
        }
    }

    emitBusyEvent() {
        this.dispatchEvent(new CustomEvent('modal-busy', {composed: true, bubbles: true }));
    }

    emitNotBusyEvent() {
        this.dispatchEvent(new CustomEvent('modal-not-busy', {composed: true, bubbles: true }));
    }

    sendChatMessage(message) {
        this.emitBusyEvent();
        if (!this.currentInteraction) {
            this.startLlmEdit(message);
        } else {
            this.continueInteraction(message);
        }
    }

    startLlmEdit(message) {
        this.emitBusyEvent();

        fetch('/api/llm/edit/start', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                page_identifier: this.pageIdentifier,
                edit_prompt: message
            })
        }).then(response => response.json())
          .then(editStartApiResponse => {
            this.emitNotBusyEvent();
            if (editStartApiResponse.success) {
                this.currentInteraction = editStartApiResponse.response;
                this.currentNewContentHtml = editStartApiResponse.response.new_content;
                this.chatHistory = [...this.chatHistory, { actor: 'robot', text: editStartApiResponse.response.response_to_user }];
            } else {
                this.chatHistory = [...this.chatHistory, { actor: 'bug', text: 'There was an error starting the LLM edit.' }];
            }
        });
    }

    newlinesToBr(text) {
        return text.replace(/(?:\r\n|\r|\n)/g, '<br>');
    }

    continueInteraction(message) {
        this.emitBusyEvent();

        fetch('/api/llm/edit/continue', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                interaction_id: this.currentInteraction.interaction_id,
                answer: message
            })
        }).then(response => response.json())
          .then(editContinueApiResponse => {
            this.emitNotBusyEvent();
            if (editContinueApiResponse.success) {
                this.currentInteraction = editContinueApiResponse.response;
                this.currentNewContentHtml = editContinueApiResponse.response.new_content;
                this.chatHistory = [...this.chatHistory, { actor: 'robot', text: editContinueApiResponse.response.response_to_user }];
            } else {
                this.chatHistory = [...this.chatHistory, { actor: 'bug', text: 'There was an error continuing the conversation.' }];
            }
        });
    }

    saveInteraction() {
        this.emitBusyEvent();

        fetch('/api/llm/edit/save', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                interaction_id: this.currentInteraction.interaction_id,
                page_identifier: this.pageIdentifier
            })
        }).then(response => response.json())
          .then(editSaveApiResponse => {
            this.emitNotBusyEvent();
            if (editSaveApiResponse.success) {
                this.emitModalClosedEvent();
            } else {
                this.chatHistory = [...this.chatHistory, { actor: 'bug', text: 'There was an error saving the interaction.' }];
            }
        });
    }

    render() {
        return html`
        <link href="/static/css/fontawesome.min.css" rel="stylesheet">
        <link href="/static/css/solid.min.css" rel="stylesheet">
        <div class="outer">
            <div class="content">
                <div class="left">
                    <ol id="chat-history">
                        ${this.chatHistory.map((item) => html`
                            <li class="actor-${item.actor}"><i class="fas fa-${item.actor}"></i> ${this.newlinesToBr(item.text)}</li>
                        `)}
                    </ol>
                    <div id="chat-input">
                        <textarea id="chat-input-text" placeholder="Type a message..." .value="${this.chatInputText}" @input="${this.handleChatInputTextChange}" @keydown="${this.handleChatInputKeyDown}"></textarea>
                        <button id="chat-input-submit" alt="Submit" @click="${this.handleChatSend}"><i class="fas fa-message"></i></button>
                    </div>
                </div>
                <div class="right">
                    <markdown-viewer .markdown="${this.currentNewContentHtml}"></markdown-viewer>
                </div>
            </div>
            <div id="footer">
                <button id="cancel" @click="${this.handleCancelButtonClick}"><i class="fas fa-ban"></i> Cancel</button>
                <button id="save" ?disabled="${!this.currentNewContentHtml}" @click="${this.handleSaveButtonClick}"><i class="fas fa-floppy-disk"></i> Save</button>
            </div>
        </div>
        `;
    }
}

customElements.define('llm-edit-experience', LlmEditExperience);