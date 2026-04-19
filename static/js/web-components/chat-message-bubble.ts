import { LitElement, html, css, nothing } from 'lit';
import { property } from 'lit/decorators.js';
import { unsafeHTML } from 'lit/directives/unsafe-html.js';
import { colorCSS, typographyCSS } from './shared-styles.js';
import { Sender } from '../gen/api/v1/chat_pb.js';
import type { ToolCallState } from './page-chat-panel.js';

export interface ReactionGroup {
  emoji: string;
  reactors: string[];
  count: number;
}

export interface ScrollToMessageEventDetail {
  messageId: string;
}

declare global {
  interface HTMLElementTagNameMap {
    'chat-message-bubble': ChatMessageBubble;
  }
}

export class ChatMessageBubble extends LitElement {
  static override readonly styles = [
    colorCSS,
    typographyCSS,
    css`
      :host {
        display: block;
        margin-bottom: 8px;
      }

      .bubble {
        padding: 8px 12px;
        border-radius: 8px;
        max-width: 90%;
        word-wrap: break-word;
        overflow-wrap: break-word;
      }

      .bubble.user {
        background: var(--color-chat-user-bg);
        margin-left: auto;
        border-bottom-right-radius: 2px;
        color: var(--color-chat-user-text);
      }

      .bubble.assistant {
        background: var(--color-surface-elevated);
        border: 1px solid var(--color-border-default);
        margin-right: auto;
        border-bottom-left-radius: 2px;
      }

      .sender-name {
        font-size: 0.75rem;
        color: var(--color-text-muted);
        margin-bottom: 2px;
      }

      .content {
        font-size: 0.875rem;
        line-height: 1.4;
        color: var(--color-text-primary);
      }

      .content p:first-child {
        margin-top: 0;
      }

      .content p:last-child {
        margin-bottom: 0;
      }

      .edited-indicator {
        font-size: 0.7rem;
        color: var(--color-text-muted);
        font-style: italic;
        margin-top: 2px;
      }

      .reply-link {
        font-size: 0.75rem;
        color: var(--color-text-muted);
        cursor: pointer;
        margin-bottom: 4px;
        display: flex;
        align-items: center;
        gap: 4px;
      }

      .reply-link:hover {
        color: var(--color-text-primary);
        text-decoration: underline;
      }

      .reactions {
        display: flex;
        flex-wrap: wrap;
        gap: 4px;
        margin-top: 4px;
      }

      .reaction-chip {
        display: inline-flex;
        align-items: center;
        gap: 2px;
        padding: 2px 6px;
        border-radius: 10px;
        background: rgba(255, 255, 255, 0.1);
        border: 1px solid var(--color-border-subtle);
        font-size: 0.8rem;
        cursor: default;
      }

      .reaction-count {
        font-size: 0.7rem;
        color: var(--color-text-muted);
      }

      .tool-calls {
        display: flex;
        flex-wrap: wrap;
        gap: 4px;
        margin-top: 6px;
      }

      .tool-call-pill {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        padding: 2px 8px;
        border-radius: 10px;
        background: rgba(255, 255, 255, 0.08);
        border: 1px solid var(--color-border-subtle);
        font-size: 0.75rem;
        color: var(--color-text-muted);
      }

      .tool-call-pill .status-icon {
        font-size: 0.7rem;
      }
    `,
  ];

  @property({ type: String, attribute: 'message-id' })
  declare messageId: string;

  @property({ type: Number })
  declare sender: Sender;

  @property({ type: String, attribute: 'sender-name' })
  declare senderName: string;

  @property({ type: String, attribute: 'rendered-html' })
  declare renderedHtml: string;

  @property({ type: String })
  declare content: string;

  @property({ type: Boolean })
  declare edited: boolean;

  @property({ type: String, attribute: 'reply-to-id' })
  declare replyToId: string;

  @property({ attribute: false })
  declare reactions: ReactionGroup[];

  @property({ attribute: false })
  declare toolCalls: ToolCallState[];

  constructor() {
    super();
    this.messageId = '';
    this.sender = Sender.SENDER_UNSPECIFIED;
    this.senderName = '';
    this.renderedHtml = '';
    this.content = '';
    this.edited = false;
    this.replyToId = '';
    this.reactions = [];
    this.toolCalls = [];
  }

  override render() {
    const isUser = this.sender === Sender.USER;
    const bubbleClass = isUser ? 'user' : 'assistant';

    return html`
      ${this.replyToId
        ? html`<div
            class="reply-link"
            role="link"
            tabindex="0"
            @click=${this._handleReplyClick}
            @keydown=${this._handleReplyKeydown}
          >
            ↩ Replying to a message
          </div>`
        : nothing}
      <div class="bubble ${bubbleClass}" data-message-id=${this.messageId}>
        ${this.senderName
          ? html`<div class="sender-name">${this.senderName}</div>`
          : nothing}
        <div class="content">
          ${this.renderedHtml
            ? unsafeHTML(this.renderedHtml)
            : html`${this.content}`}
        </div>
        ${this._renderToolCalls()}
        ${this.edited
          ? html`<div class="edited-indicator">(edited)</div>`
          : nothing}
      </div>
      ${this.reactions.length > 0
        ? html`<div class="reactions">
            ${this.reactions.map(
              (r) => html`
                <span class="reaction-chip">
                  <span>${r.emoji}</span>
                  ${r.count > 1
                    ? html`<span class="reaction-count">${r.count}</span>`
                    : nothing}
                </span>
              `,
            )}
          </div>`
        : nothing}
    `;
  }

  private _toolCallStatusIcon(status: string): string {
    switch (status) {
      case 'running': return '\u23F3';  // hourglass
      case 'complete': return '\u2705'; // check mark
      case 'error': return '\u274C';    // cross mark
      default: return '\u2022';         // bullet
    }
  }

  private _renderToolCalls() {
    if (this.toolCalls.length === 0) return nothing;

    return html`
      <div class="tool-calls">
        ${this.toolCalls.map(
          (tc) => html`
            <span class="tool-call-pill">
              <span class="status-icon">${this._toolCallStatusIcon(tc.status)}</span>
              ${tc.title}
            </span>
          `,
        )}
      </div>
    `;
  }

  private _handleReplyClick() {
    this.dispatchEvent(
      new CustomEvent('scroll-to-message', {
        detail: { messageId: this.replyToId },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _handleReplyKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      this._handleReplyClick();
    }
  }
}

customElements.define('chat-message-bubble', ChatMessageBubble);
