import { LitElement, html, css, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient, ConnectError, Code, type Client } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  ChatService,
  GetChatStatusRequestSchema,
  SendChatMessageRequestSchema,
  SubscribeChatRequestSchema,
  Sender,
  type ChatMessage,
  type ChatEvent,
} from '../gen/api/v1/chat_pb.js';
import { ChatMarkdownRenderer } from './chat-markdown-renderer.js';
import { sharedStyles, colorCSS, typographyCSS } from './shared-styles.js';
import type { ReactionGroup, ScrollToMessageEventDetail } from './chat-message-bubble.js';
import './chat-message-bubble.js';

const STORAGE_KEY = 'chat-panel-open';
const MAX_INPUT_LENGTH = 2000;
const INITIAL_RECONNECT_DELAY_MS = 1000;
const MAX_RECONNECT_DELAY_MS = 30000;
const STATUS_POLL_INTERVAL_MS = 15000;

export interface ChatMessageState {
  id: string;
  sender: Sender;
  content: string;
  renderedHtml: string;
  timestamp: Date;
  senderName: string;
  replyToId: string;
  reactions: ReactionGroup[];
  edited: boolean;
  sequence: bigint;
}

declare global {
  interface HTMLElementTagNameMap {
    'page-chat-panel': PageChatPanel;
  }
}

export class PageChatPanel extends LitElement {
  static override styles = [
    colorCSS,
    typographyCSS,
    css`
      :host {
        display: block;
      }

      .fab {
        position: fixed;
        bottom: 70px;
        right: 16px;
        width: 48px;
        height: 48px;
        border-radius: 50%;
        background: var(--color-background-primary);
        border: 1px solid var(--color-border-primary);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
        z-index: 999;
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--color-text-muted);
        font-size: 1.2rem;
        transition: all 0.2s ease;
      }

      .fab:hover {
        color: var(--color-text-primary);
        box-shadow: 0 6px 16px rgba(0, 0, 0, 0.4);
      }

      .panel {
        position: fixed;
        top: 0;
        right: 0;
        bottom: 0;
        width: 350px;
        background: var(--color-background-primary);
        border-left: 1px solid var(--color-border-primary);
        display: flex;
        flex-direction: column;
        z-index: 998;
        transform: translateX(100%);
        transition: transform 0.3s ease;
      }

      .panel.open {
        transform: translateX(0);
      }

      .panel-header {
        padding: 12px 16px;
        border-bottom: 1px solid var(--color-border-primary);
        display: flex;
        align-items: center;
        justify-content: space-between;
        flex-shrink: 0;
      }

      .panel-title {
        font-size: 0.9rem;
        font-weight: 600;
        color: var(--color-text-primary);
      }

      .close-button {
        background: none;
        border: none;
        color: var(--color-text-muted);
        cursor: pointer;
        font-size: 1.1rem;
        padding: 4px;
      }

      .close-button:hover {
        color: var(--color-text-primary);
      }

      .messages-container {
        flex: 1;
        overflow-y: auto;
        padding: 12px;
        display: flex;
        flex-direction: column;
      }

      .status-banner {
        padding: 8px 12px;
        font-size: 0.8rem;
        text-align: center;
        flex-shrink: 0;
      }

      .status-banner.reconnecting {
        background: rgba(255, 193, 7, 0.15);
        color: var(--color-warning);
      }

      .status-banner.disconnected {
        background: rgba(220, 53, 69, 0.15);
        color: var(--color-error);
      }

      .thinking-indicator {
        padding: 8px 12px;
        font-size: 0.8rem;
        color: var(--color-text-muted);
        font-style: italic;
      }

      .input-area {
        padding: 8px 12px;
        border-top: 1px solid var(--color-border-primary);
        display: flex;
        gap: 8px;
        align-items: flex-end;
        flex-shrink: 0;
      }

      .input-area textarea {
        flex: 1;
        resize: none;
        border: 1px solid var(--color-border-primary);
        border-radius: 6px;
        background: rgba(255, 255, 255, 0.05);
        color: var(--color-text-primary);
        padding: 8px;
        font-size: 0.85rem;
        font-family: inherit;
        min-height: 36px;
        max-height: 120px;
        outline: none;
      }

      .input-area textarea:focus {
        border-color: var(--color-text-muted);
      }

      .send-button {
        background: #3a5a8c;
        border: none;
        border-radius: 6px;
        color: white;
        padding: 8px 12px;
        cursor: pointer;
        font-size: 0.85rem;
        flex-shrink: 0;
      }

      .send-button:hover:not(:disabled) {
        background: #4a6a9c;
      }

      .send-button:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }

      .empty-state {
        flex: 1;
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--color-text-muted);
        font-size: 0.85rem;
        text-align: center;
        padding: 24px;
      }

      @media (max-width: 768px) {
        .panel {
          top: auto;
          bottom: 0;
          left: 0;
          right: 0;
          width: 100%;
          height: 60dvh;
          max-height: var(--chat-panel-height, 60dvh);
          border-left: none;
          border-top: 1px solid var(--color-border-primary);
          border-radius: 12px 12px 0 0;
          transform: translateY(100%);
        }

        .panel.open {
          transform: translateY(0);
        }
      }
    `,
  ];

  @property({ type: String })
  declare page: string;

  @state()
  declare panelOpen: boolean;

  @state()
  declare messages: ChatMessageState[];

  @state()
  declare streamState: 'connected' | 'reconnecting' | 'disconnected';

  @state()
  declare waitingForAssistant: boolean;

  @state()
  declare error: Error | null;

  @state()
  declare claudeConnected: boolean;

  private messagesById = new Map<string, ChatMessageState>();
  private streamSubscription: AbortController | undefined;
  private statusPollTimer: ReturnType<typeof setInterval> | undefined;
  private chatClient: Client<typeof ChatService>;
  private markdownRenderer: ChatMarkdownRenderer;
  private _handleVisibilityChange: () => void;
  private _handleViewportResize: () => void;
  private userHasScrolled = false;

  constructor() {
    super();
    this.page = '';
    this.panelOpen = false;
    this.messages = [];
    this.streamState = 'disconnected';
    this.waitingForAssistant = false;
    this.error = null;
    this.claudeConnected = false;
    this.chatClient = createClient(ChatService, getGrpcWebTransport());
    this.markdownRenderer = new ChatMarkdownRenderer();
    this._handleVisibilityChange = this.handleVisibilityChange.bind(this);
    this._handleViewportResize = this.handleViewportResize.bind(this);

    // Restore panel state from localStorage
    try {
      this.panelOpen = localStorage.getItem(STORAGE_KEY) === 'true';
    } catch {
      // localStorage unavailable
    }
  }

  override connectedCallback() {
    super.connectedCallback();
    document.addEventListener('visibilitychange', this._handleVisibilityChange);
    window.visualViewport?.addEventListener('resize', this._handleViewportResize);
    if (this.page) {
      this.startStream();
    }
    this.pollChatStatus();
    this.statusPollTimer = setInterval(() => this.pollChatStatus(), STATUS_POLL_INTERVAL_MS);
  }

  override disconnectedCallback() {
    super.disconnectedCallback();
    document.removeEventListener('visibilitychange', this._handleVisibilityChange);
    window.visualViewport?.removeEventListener('resize', this._handleViewportResize);
    this.stopStream();
    if (this.statusPollTimer) {
      clearInterval(this.statusPollTimer);
      this.statusPollTimer = undefined;
    }
  }

  override updated(changedProperties: Map<string, unknown>) {
    super.updated(changedProperties);

    if (changedProperties.has('page')) {
      this.stopStream();
      this.messages = [];
      this.messagesById.clear();
      if (this.page) {
        this.startStream();
      }
    }
  }

  override render() {
    return html`
      ${sharedStyles}
      ${this.panelOpen || !this.claudeConnected ? nothing : html`
        <button
          class="fab"
          @click=${this.togglePanel}
          aria-label="Chat with Claude"
        >
          <i class="fa-solid fa-robot"></i>
        </button>
      `}

      <div class="panel ${this.panelOpen ? 'open' : ''}">
        <div class="panel-header">
          <span class="panel-title">Claude</span>
          <button class="close-button" @click=${this.togglePanel} aria-label="Close chat">
            <i class="fa-solid fa-xmark"></i>
          </button>
        </div>

        ${this.streamState === 'reconnecting'
          ? html`<div class="status-banner reconnecting">Reconnecting...</div>`
          : nothing}
        ${!this.claudeConnected
          ? html`<div class="status-banner disconnected">Claude is not connected</div>`
          : this.streamState === 'disconnected' && this.error
            ? html`<div class="status-banner disconnected">${this.error.message}</div>`
            : nothing}

        <div
          class="messages-container"
          role="log"
          aria-live="polite"
          aria-label="Chat messages"
          @scroll=${this._handleScroll}
        >
          ${this.messages.length === 0
            ? html`<div class="empty-state">
                Send a message to start chatting with Claude about this page.
              </div>`
            : this.messages.map(
                (msg) => html`
                  <chat-message-bubble
                    message-id=${msg.id}
                    .sender=${msg.sender}
                    sender-name=${msg.senderName}
                    content=${msg.content}
                    rendered-html=${msg.renderedHtml}
                    ?edited=${msg.edited}
                    reply-to-id=${msg.replyToId}
                    .reactions=${msg.reactions}
                    @scroll-to-message=${this._handleScrollToMessage}
                  ></chat-message-bubble>
                `,
              )}
        </div>

        ${this.waitingForAssistant
          ? html`<div class="thinking-indicator">Claude is thinking...</div>`
          : nothing}

        <div class="input-area">
          <textarea
            placeholder="${this.claudeConnected ? 'Type a message...' : 'Claude is not connected'}"
            maxlength="${MAX_INPUT_LENGTH}"
            rows="1"
            ?disabled=${!this.claudeConnected}
            @keydown=${this._handleKeydown}
          ></textarea>
          <button
            class="send-button"
            @click=${this._handleSendClick}
            ?disabled=${!this.claudeConnected}
            aria-label="Send message"
          >
            Send
          </button>
        </div>
      </div>
    `;
  }

  private togglePanel() {
    this.panelOpen = !this.panelOpen;
    try {
      localStorage.setItem(STORAGE_KEY, String(this.panelOpen));
    } catch {
      // localStorage unavailable
    }
    if (this.panelOpen) {
      this.updateComplete.then(() => {
        this.scrollToBottom();
        this.focusInput();
      });
    }
  }

  private _handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      this.sendMessage();
    }
  }

  private _handleSendClick() {
    this.sendMessage();
  }

  private _handleScroll(e: Event) {
    if (!(e.target instanceof HTMLElement)) return;
    const container = e.target;
    const threshold = 50;
    this.userHasScrolled =
      container.scrollTop + container.clientHeight < container.scrollHeight - threshold;
  }

  private _handleScrollToMessage(e: CustomEvent<ScrollToMessageEventDetail>) {
    const target = this.shadowRoot?.querySelector(
      `chat-message-bubble[message-id="${CSS.escape(e.detail.messageId)}"]`,
    );
    if (target) {
      target.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  }

  private async sendMessage() {
    const textarea = this.shadowRoot?.querySelector('textarea');
    if (!textarea) return;

    const content = textarea.value.trim();
    if (!content) return;

    textarea.value = '';
    this.waitingForAssistant = true;

    try {
      const request = create(SendChatMessageRequestSchema, {
        page: this.page,
        content,
      });
      await this.chatClient.sendMessage(request);
    } catch (err) {
      if (err instanceof ConnectError && err.code === Code.Unavailable) {
        this.error = err;
      } else {
        this.error = err instanceof Error ? err : new Error(String(err));
      }
      this.waitingForAssistant = false;
    }

    this.focusInput();
  }

  private async pollChatStatus(): Promise<void> {
    try {
      const request = create(GetChatStatusRequestSchema, {});
      const response = await this.chatClient.getChatStatus(request);
      const wasConnected = this.claudeConnected;
      this.claudeConnected = response.connected;

      // If Claude just connected and panel was open, re-focus input
      if (!wasConnected && response.connected && this.panelOpen) {
        this.focusInput();
      }
    } catch {
      this.claudeConnected = false;
    }
  }

  private handleViewportResize(): void {
    const viewport = window.visualViewport;
    if (!viewport) return;

    // When the keyboard opens, visualViewport.height shrinks.
    // Set a CSS custom property so the panel fits above the keyboard.
    const panel = this.shadowRoot?.querySelector('.panel');
    if (panel instanceof HTMLElement) {
      const availableHeight = viewport.height;
      panel.style.setProperty('--chat-panel-height', `${availableHeight * 0.9}px`);
      panel.style.bottom = `${window.innerHeight - viewport.height - viewport.offsetTop}px`;
    }
  }

  private handleVisibilityChange(): void {
    if (
      document.visibilityState === 'visible' &&
      this.page &&
      this.streamState !== 'connected'
    ) {
      this.startStream();
    }
  }

  private async startStream(): Promise<void> {
    this.stopStream();
    this.streamSubscription = new AbortController();
    const signal = this.streamSubscription.signal;
    let reconnectDelayMs = INITIAL_RECONNECT_DELAY_MS;

    while (!signal.aborted) {
      try {
        const request = create(SubscribeChatRequestSchema, { page: this.page });
        this.streamState = 'connected';
        this.error = null;

        for await (const event of this.chatClient.subscribeChat(request, { signal })) {
          await this.handleChatEvent(event);
          reconnectDelayMs = INITIAL_RECONNECT_DELAY_MS;
        }

        // Stream ended cleanly
        break;
      } catch (err) {
        if (err instanceof Error && err.name === 'AbortError') break;
        if (signal.aborted) break;

        this.streamState = 'reconnecting';
        this.error = err instanceof Error ? err : new Error(String(err));

        await new Promise<void>((resolve) => {
          const timer = setTimeout(resolve, reconnectDelayMs);
          signal.addEventListener(
            'abort',
            () => {
              clearTimeout(timer);
              resolve();
            },
            { once: true },
          );
        });

        reconnectDelayMs = Math.min(reconnectDelayMs * 2, MAX_RECONNECT_DELAY_MS);
      }
    }

    if (this.streamState !== 'disconnected') {
      this.streamState = 'disconnected';
    }
  }

  private stopStream(): void {
    if (this.streamSubscription) {
      this.streamSubscription.abort();
      this.streamSubscription = undefined;
      this.streamState = 'disconnected';
    }
  }

  private async handleChatEvent(event: ChatEvent): Promise<void> {
    switch (event.event.case) {
      case 'newMessage':
        await this.addMessage(event.event.value);
        break;
      case 'edit':
        await this.editMessage(event.event.value.messageId, event.event.value.newContent);
        break;
      case 'reaction':
        this.addReaction(
          event.event.value.messageId,
          event.event.value.emoji,
          event.event.value.reactor,
        );
        break;
    }
  }

  private async addMessage(msg: ChatMessage): Promise<void> {
    let renderedHtml = '';
    if (msg.sender === Sender.ASSISTANT) {
      try {
        renderedHtml = await this.markdownRenderer.renderMarkdown(msg.content, this.page);
      } catch {
        // Fall back to raw content on render failure
      }
      this.waitingForAssistant = false;
    }

    const msgState: ChatMessageState = {
      id: msg.id,
      sender: msg.sender,
      content: msg.content,
      renderedHtml,
      timestamp: msg.timestamp ? new Date(Number(msg.timestamp.seconds) * 1000) : new Date(),
      senderName: msg.senderName,
      replyToId: msg.replyToId,
      reactions: [],
      edited: false,
      sequence: msg.sequence,
    };

    this.messagesById.set(msg.id, msgState);
    this.messages = [...this.messages, msgState];

    await this.updateComplete;
    if (!this.userHasScrolled) {
      this.scrollToBottom();
    }
  }

  private async editMessage(messageId: string, newContent: string): Promise<void> {
    const msg = this.messagesById.get(messageId);
    if (!msg) return;

    let renderedHtml = '';
    if (msg.sender === Sender.ASSISTANT) {
      try {
        renderedHtml = await this.markdownRenderer.renderMarkdown(newContent, this.page);
      } catch {
        // Fall back to raw content
      }
    }

    msg.content = newContent;
    msg.renderedHtml = renderedHtml;
    msg.edited = true;
    this.messages = [...this.messages]; // trigger re-render
  }

  private addReaction(messageId: string, emoji: string, reactor: string): void {
    const msg = this.messagesById.get(messageId);
    if (!msg) return;

    const existing = msg.reactions.find((r) => r.emoji === emoji);
    if (existing) {
      existing.reactors.push(reactor);
      existing.count = existing.reactors.length;
    } else {
      msg.reactions = [...msg.reactions, { emoji, reactors: [reactor], count: 1 }];
    }
    this.messages = [...this.messages]; // trigger re-render
  }

  private scrollToBottom(): void {
    const container = this.shadowRoot?.querySelector('.messages-container');
    if (container) {
      container.scrollTop = container.scrollHeight;
    }
  }

  private focusInput(): void {
    const textarea = this.shadowRoot?.querySelector('textarea');
    textarea?.focus();
  }
}

customElements.define('page-chat-panel', PageChatPanel);
