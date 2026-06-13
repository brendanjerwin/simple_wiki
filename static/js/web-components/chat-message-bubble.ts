import { LitElement, html, css, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
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

      .live-tool-calls {
        display: flex;
        flex-direction: column;
        gap: 6px;
        margin-top: 6px;
        max-height: 112px;
        overflow-y: auto;
        padding-right: 2px;
      }

      .live-tool-call {
        display: flex;
        align-items: flex-start;
        gap: 8px;
        padding: 8px 10px;
        border-radius: 8px;
        background: rgba(255, 255, 255, 0.06);
        border: 1px solid var(--color-border-subtle);
      }

      .live-tool-copy,
      .subagent-copy {
        min-width: 0;
        display: flex;
        flex-direction: column;
        gap: 2px;
      }

      .live-tool-status,
      .subagent-title {
        font-size: 0.75rem;
        font-weight: 600;
        color: var(--color-text-primary);
      }

      .live-tool-title,
      .subagent-preview {
        font-size: 0.75rem;
        color: var(--color-text-muted);
        line-height: 1.35;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }

      .subagent-cards {
        display: flex;
        flex-direction: column;
        gap: 8px;
        margin-top: 6px;
      }

      .subagent-card {
        padding: 10px 12px;
        border-radius: 10px;
        background: rgba(77, 171, 247, 0.12);
        border: 1px solid rgba(77, 171, 247, 0.35);
      }

      .subagent-header {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 6px;
      }

      .subagent-badge {
        font-size: 0.68rem;
        font-weight: 700;
        letter-spacing: 0.04em;
        text-transform: uppercase;
        color: var(--color-text-primary);
      }

      .subagent-meta {
        margin-left: auto;
        display: inline-flex;
        align-items: center;
        gap: 8px;
        font-size: 0.68rem;
        color: var(--color-text-muted);
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

  @state()
  declare elapsedNowMs: number;

  private elapsedTimer: ReturnType<typeof setInterval> | undefined;

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
    this.elapsedNowMs = Date.now();
  }

  override connectedCallback() {
    super.connectedCallback();
    this.syncElapsedTimer();
  }

  override disconnectedCallback() {
    this.clearElapsedTimer();
    super.disconnectedCallback();
  }

  override updated(changedProperties: Map<string, unknown>) {
    super.updated(changedProperties);
    if (changedProperties.has('toolCalls')) {
      this.syncElapsedTimer();
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
    switch (this._toolCallStatusKind(status)) {
      case 'running': return '\u23F3';
      case 'waiting': return '\u23F8';
      case 'complete': return '\u2705';
      case 'error': return '\u274C';
      default: return '\u2022';
    }
  }

  private _renderToolCalls() {
    if (this.toolCalls.length === 0) return nothing;

    const liveSubagentCalls = this.toolCalls.filter((toolCall) => this._isLiveToolCall(toolCall) && this._isSubagentCall(toolCall));
    const liveToolCalls = this.toolCalls.filter((toolCall) => this._isLiveToolCall(toolCall) && !this._isSubagentCall(toolCall));
    const compactToolCalls = this.toolCalls.filter((toolCall) => !this._isLiveToolCall(toolCall));

    return html`
      ${liveSubagentCalls.length > 0
        ? html`
            <div class="subagent-cards">
              ${liveSubagentCalls.map((toolCall) => this._renderSubagentCard(toolCall))}
            </div>
          `
        : nothing}
      ${liveToolCalls.length > 0
        ? html`
            <div class="live-tool-calls">
              ${liveToolCalls.map((toolCall) => this._renderLiveToolCall(toolCall))}
            </div>
          `
        : nothing}
      ${compactToolCalls.length > 0
        ? html`
            <div class="tool-calls">
              ${compactToolCalls.map(
                (toolCall) => html`
                  <span class="tool-call-pill" title=${this._toolCallTooltip(toolCall)}>
                    <span class="status-icon">${this._toolCallStatusIcon(toolCall.status)}</span>
                    ${this._compactToolCallLabel(toolCall)}
                  </span>
                `,
              )}
            </div>
          `
        : nothing}
    `;
  }

  private _renderLiveToolCall(toolCall: ToolCallState) {
    return html`
      <div class="live-tool-call" title=${this._toolCallTooltip(toolCall)}>
        <span class="status-icon">${this._toolCallStatusIcon(toolCall.status)}</span>
        <div class="live-tool-copy">
          <div class="live-tool-status">${this._liveToolStatusLabel(toolCall)}</div>
          <div class="live-tool-title">${toolCall.title}</div>
        </div>
      </div>
    `;
  }

  private _renderSubagentCard(toolCall: ToolCallState) {
    const parsedTitle = this._parseSubagentTitle(toolCall.title);
    return html`
      <div class="subagent-card" title=${this._toolCallTooltip(toolCall)}>
        <div class="subagent-header">
          <span class="subagent-badge">Sub-agent</span>
          <div class="subagent-meta">
            <span>${this._toolCallStatusLabel(toolCall.status)}</span>
            <span class="subagent-elapsed">${this._toolCallElapsed(toolCall)}</span>
          </div>
        </div>
        <div class="subagent-copy">
          <div class="subagent-title">${parsedTitle.label}</div>
          <div class="subagent-preview">${parsedTitle.preview}</div>
        </div>
      </div>
    `;
  }

  private _toolCallStatusKind(status: string): 'running' | 'waiting' | 'complete' | 'error' | 'unknown' {
    const normalizedStatus = status.trim().toLowerCase().replace(/[\s-]+/g, '_');
    switch (normalizedStatus) {
      case 'running':
      case 'in_progress':
        return 'running';
      case 'waiting':
      case 'queued':
      case 'pending':
        return 'waiting';
      case 'complete':
      case 'completed':
      case 'success':
        return 'complete';
      case 'error':
      case 'failed':
      case 'cancelled':
      case 'canceled':
      case 'timeout':
        return 'error';
      default:
        return 'unknown';
    }
  }

  private _toolCallStatusLabel(status: string): string {
    switch (this._toolCallStatusKind(status)) {
      case 'running':
        return 'Running';
      case 'waiting':
        return 'Waiting';
      case 'complete':
        return 'Completed';
      case 'error':
        return 'Error';
      default:
        return 'Queued';
    }
  }

  private _isLiveToolCall(toolCall: ToolCallState): boolean {
    const statusKind = this._toolCallStatusKind(toolCall.status);
    return statusKind === 'running' || statusKind === 'waiting';
  }

  private _isSubagentCall(toolCall: ToolCallState): boolean {
    const normalizedTitle = toolCall.title.trim().toLowerCase();
    if (normalizedTitle.includes('sub-agent') || normalizedTitle.includes('subagent')) {
      return true;
    }
    if (normalizedTitle.includes('/parallel') || normalizedTitle.includes('/chain') || normalizedTitle.includes('/run')) {
      return true;
    }
    if (/\b(explore|research|code-review|general-purpose|task|background)\s+agent\b/.test(normalizedTitle)) {
      return true;
    }
    return /\bagent\b/.test(normalizedTitle)
      && /\b(launch|dispatch|spawn|start|run)\b/.test(normalizedTitle);
  }

  private _liveToolStatusLabel(toolCall: ToolCallState): string {
    return `${this._toolCallStatusLabel(toolCall.status)} ${this._shortToolLabel(toolCall.title)}`;
  }

  private _shortToolLabel(title: string): string {
    const trimmedTitle = title.trim();
    const separatorMatch = trimmedTitle.match(/^(.+?)(?::| — | - )/);
    return separatorMatch?.[1]?.trim() || trimmedTitle;
  }

  private _compactToolCallLabel(toolCall: ToolCallState): string {
    if (this._isSubagentCall(toolCall)) {
      const parsedTitle = this._parseSubagentTitle(toolCall.title);
      if (parsedTitle.preview !== toolCall.title) {
        return `${parsedTitle.label}: ${parsedTitle.preview}`;
      }
      return parsedTitle.label;
    }
    return this._shortToolLabel(toolCall.title);
  }

  private _parseSubagentTitle(title: string): { label: string; preview: string } {
    const trimmedTitle = title.trim();
    const separatorIndex = trimmedTitle.indexOf(':');
    if (separatorIndex !== -1) {
      const prefix = trimmedTitle.slice(0, separatorIndex).trim();
      const suffix = trimmedTitle.slice(separatorIndex + 1).trim();
      const normalizedLabel = this._normalizeSubagentLabel(prefix);
      if (normalizedLabel) {
        return {
          label: normalizedLabel,
          preview: suffix || trimmedTitle,
        };
      }
    }

    const agentTypeMatch = trimmedTitle.match(/\b(explore|research|code-review|general-purpose|task|background)\s+agent\b/i);
    if (agentTypeMatch) {
      return {
        label: this._capitalizeWords(agentTypeMatch[0]),
        preview: trimmedTitle,
      };
    }

    return {
      label: 'Sub-agent',
      preview: trimmedTitle,
    };
  }

  private _normalizeSubagentLabel(titlePrefix: string): string | null {
    const cleanedPrefix = titlePrefix.replace(/\b(launch|dispatch|spawn|start|run)\b/gi, '').trim();
    if (/\bagent\b/i.test(cleanedPrefix)) {
      return this._capitalizeWords(cleanedPrefix.replace(/\bsubagent\b/i, 'sub-agent'));
    }
    const agentTypeMatch = cleanedPrefix.match(/\b(explore|research|code-review|general-purpose|task|background)\b/i);
    if (!agentTypeMatch) {
      return null;
    }
    return this._capitalizeWords(`${agentTypeMatch[1]} agent`);
  }

  private _capitalizeWords(value: string): string {
    return value.replace(/\b\w/g, (letter) => letter.toUpperCase());
  }

  private _toolCallTooltip(toolCall: ToolCallState): string {
    return `${this._toolCallStatusLabel(toolCall.status)}: ${toolCall.title}`;
  }

  private _toolCallElapsed(toolCall: ToolCallState): string {
    const startedAtMs = toolCall.startedAtMs ?? this.elapsedNowMs;
    return this._formatElapsed(this.elapsedNowMs - startedAtMs);
  }

  private _formatElapsed(elapsedMs: number): string {
    const safeElapsedMs = Math.max(0, elapsedMs);
    const elapsedSeconds = Math.floor(safeElapsedMs / 1000);
    const elapsedMinutes = Math.floor(elapsedSeconds / 60);
    const elapsedHours = Math.floor(elapsedMinutes / 60);
    if (elapsedHours >= 1) {
      return `${elapsedHours}h`;
    }
    if (elapsedMinutes >= 1) {
      return `${elapsedMinutes}m`;
    }
    return `${elapsedSeconds}s`;
  }

  private syncElapsedTimer() {
    this.elapsedNowMs = Date.now();
    if (!this.toolCalls.some((toolCall) => this._isLiveToolCall(toolCall))) {
      this.clearElapsedTimer();
      return;
    }
    if (this.elapsedTimer) {
      return;
    }
    this.elapsedTimer = setInterval(() => {
      this.elapsedNowMs = Date.now();
    }, 1000);
  }

  private clearElapsedTimer() {
    if (!this.elapsedTimer) {
      return;
    }
    clearInterval(this.elapsedTimer);
    this.elapsedTimer = undefined;
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
