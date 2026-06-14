import { LitElement, html, css, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { unsafeHTML } from 'lit/directives/unsafe-html.js';
import { colorCSS, typographyCSS } from './shared-styles.js';
import { Sender } from '../gen/api/v1/chat_pb.js';
import type { ToolCallState, PlanEntryState } from './page-chat-panel.js';

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
        margin-top: 6px;
        display: flex;
        flex-wrap: wrap;
        align-items: flex-start;
        gap: 4px;
      }

      /* Compact completed/failed tool call pill */
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
        align-self: flex-start;
      }

      .tool-call-pill .status-icon {
        font-size: 0.7rem;
      }

      /* Keep finished pills compact: truncate the label, full text on hover. */
      .tool-call-pill-label {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        max-width: 40ch;
      }

      /* Live expanded tool call row — takes its own full-width line */
      .tool-call-live {
        flex: 1 1 100%;
        display: flex;
        flex-direction: column;
        gap: 2px;
        padding: 4px 8px;
        border-radius: 6px;
        background: rgba(255, 255, 255, 0.06);
        border: 1px solid var(--color-border-subtle);
        font-size: 0.75rem;
        color: var(--color-text-muted);
      }

      .tool-call-live-header {
        display: flex;
        align-items: center;
        gap: 6px;
      }

      .tool-call-live-header .status-icon {
        font-size: 0.8rem;
      }

      .tool-call-live-title {
        flex: 1;
        font-weight: 500;
        color: var(--color-text-primary);
      }

      .tool-call-elapsed {
        font-size: 0.7rem;
        color: var(--color-text-muted);
        font-variant-numeric: tabular-nums;
      }

      .tool-call-detail {
        font-size: 0.7rem;
        color: var(--color-text-muted);
        max-height: 4.2em;
        overflow: auto;
        white-space: pre-wrap;
        word-break: break-all;
        font-family: monospace;
        line-height: 1.4;
      }

      /* Plan block */
      .plan-block {
        margin-top: 6px;
        padding: 6px 10px;
        border-radius: 6px;
        background: rgba(255, 255, 255, 0.04);
        border: 1px solid var(--color-border-subtle);
        font-size: 0.75rem;
      }

      .plan-header {
        font-size: 0.7rem;
        font-weight: 600;
        color: var(--color-text-muted);
        text-transform: uppercase;
        letter-spacing: 0.05em;
        margin-bottom: 4px;
      }

      .plan-entry {
        display: flex;
        align-items: flex-start;
        gap: 6px;
        padding: 2px 0;
        color: var(--color-text-muted);
        line-height: 1.4;
      }

      .plan-entry.in-progress {
        color: var(--color-text-primary);
        font-weight: 500;
      }

      .plan-entry.completed {
        opacity: 0.6;
      }

      .plan-entry-icon {
        flex-shrink: 0;
        margin-top: 1px;
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

  @property({ attribute: false })
  declare plan: PlanEntryState[];

  @state()
  private declare _nowMs: number;

  private _elapsedTimerId: ReturnType<typeof setInterval> | null = null;

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
    this.plan = [];
    this._nowMs = Date.now();
  }

  private _hasLiveToolCall(): boolean {
    return this.toolCalls.some((tc) => tc.status === 'pending' || tc.status === 'in_progress');
  }

  override connectedCallback() {
    super.connectedCallback();
    // Restart the timer if we are re-attached to the DOM with live tool calls
    // already set (a reconnect may not trigger updated()). _startElapsedTimer is
    // a no-op when there are no live tool calls or the timer is already running.
    this._startElapsedTimer();
  }

  override disconnectedCallback() {
    super.disconnectedCallback();
    this._stopElapsedTimer();
  }

  private _startElapsedTimer() {
    if (this._elapsedTimerId !== null) return;
    if (!this._hasLiveToolCall()) return;
    this._elapsedTimerId = setInterval(() => {
      this._nowMs = Date.now();
    }, 1000);
  }

  private _stopElapsedTimer() {
    if (this._elapsedTimerId !== null) {
      clearInterval(this._elapsedTimerId);
      this._elapsedTimerId = null;
    }
  }

  override updated() {
    // Start or stop the timer based on whether there are live tool calls
    if (this._hasLiveToolCall()) {
      this._startElapsedTimer();
    } else {
      this._stopElapsedTimer();
    }
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
        ${this._renderPlan()}
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
      case 'pending': return '•';   // bullet — neutral dot
      case 'in_progress': return '⏳'; // hourglass
      case 'completed': return '✅';  // check mark
      case 'failed': return '❌';     // cross mark
      default: return '•';           // bullet
    }
  }

  private _kindGlyph(kind: string): string {
    switch (kind) {
      case 'read': return '📄';
      case 'edit': return '✏️';
      case 'delete': return '🗑️';
      case 'move': return '↔️';
      case 'search': return '🔍';
      case 'execute': return '⚡';
      case 'think': return '💭';
      case 'fetch': return '🌐';
      case 'switch_mode': return '🔀';
      default: return '';
    }
  }

  private _formatElapsedMs(elapsedMs: number): string {
    const elapsedSec = Math.floor(elapsedMs / 1000);
    if (elapsedSec < 60) {
      return `${elapsedSec}s`;
    }
    const elapsedMin = Math.floor(elapsedSec / 60);
    const remainingSec = elapsedSec % 60;
    return `${elapsedMin}m${remainingSec}s`;
  }

  private _renderToolCalls() {
    if (this.toolCalls.length === 0) return nothing;

    return html`
      <div class="tool-calls">
        ${this.toolCalls.map((tc) => this._renderToolCall(tc))}
      </div>
    `;
  }

  private _renderToolCall(tc: ToolCallState) {
    const isLive = tc.status === 'pending' || tc.status === 'in_progress';

    if (isLive) {
      // startedAtMs is absent for historical tool calls (not replayed with a
      // timestamp); only show elapsed when we have a sane, finite value so we
      // never render "NaNs".
      const elapsedMs = this._nowMs - tc.startedAtMs;
      const showElapsed = Number.isFinite(elapsedMs) && elapsedMs >= 0;
      const kindGlyph = this._kindGlyph(tc.kind);
      return html`
        <div class="tool-call-live">
          <div class="tool-call-live-header">
            <span class="status-icon">${this._toolCallStatusIcon(tc.status)}</span>
            ${kindGlyph ? html`<span class="kind-glyph">${kindGlyph}</span>` : nothing}
            <span class="tool-call-live-title">${tc.title}</span>
            ${showElapsed
              ? html`<span class="tool-call-elapsed">${this._formatElapsedMs(elapsedMs)}</span>`
              : nothing}
          </div>
          ${tc.detail
            ? html`<div class="tool-call-detail">${tc.detail}</div>`
            : nothing}
        </div>
      `;
    }

    // Completed or failed: compact pill. Include the detail (the specifics —
    // e.g. the command/query/path) since the title alone is often a generic
    // category like "mcp". The pill label is ellipsis-truncated; the full label
    // is preserved in the title attribute for hover/accessibility.
    const kindGlyph = this._kindGlyph(tc.kind);
    const label = tc.detail ? `${tc.title}: ${tc.detail}` : tc.title;
    return html`
      <span class="tool-call-pill" title="${label}">
        <span class="status-icon">${this._toolCallStatusIcon(tc.status)}</span>
        ${kindGlyph ? html`<span class="kind-glyph">${kindGlyph}</span>` : nothing}
        <span class="tool-call-pill-label">${label}</span>
      </span>
    `;
  }

  private _planEntryIcon(status: string): string {
    switch (status) {
      case 'completed': return '☑';
      case 'in_progress': return '🔄';
      default: return '☐';
    }
  }

  private _renderPlan() {
    // A message legitimately may have no plan; tolerate an absent value rather
    // than dereferencing undefined (e.g. message state set without a plan field).
    if (!this.plan || this.plan.length === 0) return nothing;

    return html`
      <div class="plan-block">
        <div class="plan-header">Plan</div>
        ${this.plan.map(
          (entry) => html`
            <div class="plan-entry ${entry.status}">
              <span class="plan-entry-icon">${this._planEntryIcon(entry.status)}</span>
              <span class="plan-entry-content">${entry.content}</span>
            </div>
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
