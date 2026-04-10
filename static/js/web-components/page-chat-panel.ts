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
  CancelAgentPromptRequestSchema,
  Sender,
  type ChatMessage,
  type ChatEvent,
  type Reaction,
} from '../gen/api/v1/chat_pb.js';
import { ChatMarkdownRenderer } from './chat-markdown-renderer.js';
import { sharedStyles, colorCSS, typographyCSS, zIndexCSS, forceDarkTokensCSS } from './shared-styles.js';
import { DrawerMixin } from './drawer-mixin.js';
import { registerAmbientCTA, type AmbientCTA } from './drawer-coordinator.js';
import type { ReactionGroup, ScrollToMessageEventDetail } from './chat-message-bubble.js';
import './chat-message-bubble.js';

const STORAGE_KEY = 'chat-panel-open';
const MAX_INPUT_LENGTH = 2000;
const INITIAL_RECONNECT_DELAY_MS = 1000;
const MAX_RECONNECT_DELAY_MS = 30000;
const STATUS_POLL_INTERVAL_MS = 15000;
const STATUS_POLL_STARTING_MS = 3000;

export interface ToolCallState {
  toolCallId: string;
  title: string;
  status: string;
}

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
  toolCalls: ToolCallState[];
}

declare global {
  interface HTMLElementTagNameMap {
    'page-chat-panel': PageChatPanel;
  }
}

export class PageChatPanel extends DrawerMixin(LitElement) implements AmbientCTA {
  override readonly drawerId = 'page-chat';

  static override readonly styles = [
    colorCSS,
    typographyCSS,
    zIndexCSS,
    forceDarkTokensCSS,
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
        background: var(--color-editor-surface);
        border: 1px solid var(--color-editor-border);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
        z-index: var(--z-ambient);
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--color-text-muted);
        font-size: 1.2rem;
        transition: all 0.2s ease;
      }

      .fab:hover:not(.disabled) {
        color: var(--color-text-primary);
        box-shadow: 0 6px 16px rgba(0, 0, 0, 0.4);
      }

      .fab.disabled {
        opacity: 0.4;
        cursor: not-allowed;
      }

      .panel {
        position: fixed;
        top: 48px;
        right: 0;
        bottom: 0;
        width: 350px;
        background: var(--color-editor-surface);
        border-left: 1px solid var(--color-editor-border);
        display: flex;
        flex-direction: column;
        z-index: var(--z-drawer);
        transform: translateX(100%);
        transition: transform 0.3s ease;
      }

      .panel.open {
        transform: translateX(0);
      }

      .panel-header {
        padding: 12px 16px;
        border-bottom: 1px solid var(--color-editor-border);
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
        color: var(--color-text-muted);
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
        display: flex;
        align-items: center;
        gap: 6px;
      }

      .stop-button {
        margin-left: auto;
        background: rgba(220, 53, 69, 0.15);
        border: 1px solid rgba(220, 53, 69, 0.4);
        border-radius: 4px;
        color: var(--color-error);
        padding: 2px 8px;
        cursor: pointer;
        font-size: 0.75rem;
        flex-shrink: 0;
      }

      .stop-button:hover {
        background: rgba(220, 53, 69, 0.3);
      }

      .thinking-dots {
        display: inline-flex;
        gap: 3px;
      }

      .thinking-dots span {
        width: 6px;
        height: 6px;
        border-radius: 50%;
        background: var(--color-text-muted);
        animation: thinking-bounce 1.4s ease-in-out infinite;
      }

      .thinking-dots span:nth-child(2) {
        animation-delay: 0.2s;
      }

      .thinking-dots span:nth-child(3) {
        animation-delay: 0.4s;
      }

      @keyframes thinking-bounce {
        0%, 80%, 100% {
          opacity: 0.3;
          transform: scale(0.8);
        }
        40% {
          opacity: 1;
          transform: scale(1);
        }
      }

      .input-area {
        padding: 8px 12px;
        border-top: 1px solid var(--color-editor-border);
        display: flex;
        gap: 8px;
        align-items: flex-end;
        flex-shrink: 0;
      }

      .input-area textarea {
        flex: 1;
        min-width: 0;
        width: 100%;
        resize: none;
        border: 1px solid var(--color-editor-border);
        border-radius: 6px;
        background: rgba(255, 255, 255, 0.05);
        color: var(--color-text-primary);
        padding: 12px;
        font-size: 0.9rem;
        font-family: inherit;
        min-height: 44px;
        max-height: 120px;
        outline: none;
        box-sizing: border-box;
        -webkit-appearance: none;
        -webkit-tap-highlight-color: transparent;
        touch-action: manipulation;
      }

      .input-area textarea:focus {
        border-color: var(--color-text-muted);
      }

      .send-button {
        background: var(--color-chat-user-bg);
        border: none;
        border-radius: 6px;
        color: var(--color-chat-user-text);
        padding: 8px 12px;
        cursor: pointer;
        font-size: 0.85rem;
        flex-shrink: 0;
      }

      .send-button:hover:not(:disabled) {
        background: var(--color-chat-user-bg-hover);
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
          top: 0;
          bottom: 0;
          left: 0;
          right: 0;
          width: 100%;
          height: 100dvh;
          max-height: none;
          border-left: none;
          border-radius: 0;
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

  @property({ type: String })
  declare persona: string;

  @state()
  declare _fabVisible: boolean;

  @state()
  declare messages: ChatMessageState[];

  @state()
  declare streamState: 'connected' | 'reconnecting' | 'disconnected';

  @state()
  declare waitingForAssistant: boolean;

  @state()
  declare error: Error | null;

  @state()
  declare agentConnected: boolean;

  @state()
  declare poolConnected: boolean;

  @state()
  declare agentStarting: boolean;

  private pendingMessage: string | null = null;
  private readonly messagesById = new Map<string, ChatMessageState>();
  private streamSubscription: AbortController | undefined;
  private statusPollTimer: ReturnType<typeof setInterval> | undefined;
  private readonly chatClient: Client<typeof ChatService>;
  private readonly markdownRenderer: ChatMarkdownRenderer;
  private readonly _handleVisibilityChange: () => void;
  private readonly _handleViewportResize: () => void;
  private readonly _handleGlobalKeydown: (e: KeyboardEvent) => void;
  private userHasScrolled = false;

  private _restoreOpen = false;
  private _panelEverOpened = false;

  constructor() {
    super();
    this.page = '';
    this.persona = '';
    this._fabVisible = true;
    this.messages = [];
    this.streamState = 'disconnected';
    this.waitingForAssistant = false;
    this.error = null;
    this.agentConnected = false;
    this.poolConnected = false;
    this.agentStarting = false;
    this.chatClient = createClient(ChatService, getGrpcWebTransport());
    this.markdownRenderer = new ChatMarkdownRenderer();
    this._handleVisibilityChange = this.handleVisibilityChange.bind(this);
    this._handleViewportResize = this.handleViewportResize.bind(this);
    this._handleGlobalKeydown = this.handleGlobalKeydown.bind(this);

    // Read localStorage flag; actual open deferred to connectedCallback
    try {
      this._restoreOpen = localStorage.getItem(STORAGE_KEY) === 'true';
    } catch {
      // localStorage unavailable
    }
  }

  private _ctaCleanup: (() => void) | undefined;

  override connectedCallback() {
    super.connectedCallback();
    this._ctaCleanup = registerAmbientCTA(this);
    document.addEventListener('visibilitychange', this._handleVisibilityChange);
    document.addEventListener('keydown', this._handleGlobalKeydown);
    window.visualViewport?.addEventListener('resize', this._handleViewportResize);
    if (this._restoreOpen) {
      this.openDrawer();
      this._restoreOpen = false;
    }
    this.pollChatStatus();
    this.statusPollTimer = setInterval(() => this.pollChatStatus(), STATUS_POLL_INTERVAL_MS);
  }

  override disconnectedCallback() {
    super.disconnectedCallback();
    this._ctaCleanup?.();
    this._ctaCleanup = undefined;
    document.removeEventListener('visibilitychange', this._handleVisibilityChange);
    document.removeEventListener('keydown', this._handleGlobalKeydown);
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
      const oldPage = changedProperties.get('page');
      if (oldPage !== undefined && oldPage !== this.page) {
        // Navigating between pages or setting the page after initial render:
        // reset messages and restart the stream if the panel has ever been opened.
        // On true initial render (oldPage === undefined), openDrawer() handles starting
        // the stream when the panel is restored open; otherwise the stream starts the
        // first time the panel is opened.
        this.stopStream();
        this.messages = [];
        this.messagesById.clear();
        if (this.page && this._panelEverOpened) {
          this.startStream();
        }
      }
    }
  }

  private get _emptyStateMessage(): string {
    return this.persona
      ? 'Send a message to start chatting with ' + this.persona + ' about this page.'
      : 'Send a message to start chatting about this page.';
  }

  private get _thinkingText(): string {
    return this.persona ? this.persona + ' is thinking' : 'Thinking...';
  }

  private get _chatAvailable(): boolean {
    return this.poolConnected || this.agentConnected;
  }

  private _renderFab() {
    if (!this._fabVisible || this.drawerOpen) return nothing;
    const fabClass = this._chatAvailable ? 'fab' : 'fab disabled';
    return html`
      <button
        class="${fabClass}"
        @click=${this.toggleDrawer}
        aria-label=${this.persona ? 'Chat with ' + this.persona : 'Open chat'}
      >
        <i class="fa-solid fa-robot"></i>
      </button>
    `;
  }

  private _renderDisconnectedBanner() {
    if (this.agentStarting) {
      return html`<div class="status-banner reconnecting">Starting assistant...</div>`;
    }
    if (!this._chatAvailable) {
      const disconnectedText = this.persona ? this.persona + ' is not connected' : 'Not connected';
      return html`<div class="status-banner disconnected">${disconnectedText}</div>`;
    }
    if (this.poolConnected && !this.agentConnected && !this.agentStarting) {
      return html`<div class="status-banner">Send a message to start</div>`;
    }
    if (this.streamState === 'disconnected' && this.error) {
      return html`<div class="status-banner disconnected">${this.error.message}</div>`;
    }
    return nothing;
  }

  override render() {
    const notConnectedText = this.persona ? this.persona + ' is not connected' : 'Not connected';
    const inputEnabled = this._chatAvailable;
    const placeholder = inputEnabled ? 'Type a message...' : notConnectedText;
    return html`
      ${sharedStyles}
      ${this._renderFab()}

      <div class="panel force-dark ${this.drawerOpen ? 'open' : ''}" ?inert=${!this.drawerOpen}>
        <div class="panel-header">
          <span class="panel-title">${this.persona}</span>
          <button class="close-button" @click=${this.closeDrawer} aria-label="Close chat">
            <i class="fa-solid fa-xmark"></i>
          </button>
        </div>

        ${this.streamState === 'reconnecting'
          ? html`<div class="status-banner reconnecting">Reconnecting...</div>`
          : nothing}
        ${this._renderDisconnectedBanner()}

        <div
          class="messages-container"
          role="log"
          aria-live="polite"
          aria-label="Chat messages"
          @scroll=${this._handleScroll}
        >
          ${this.messages.length === 0
            ? html`<div class="empty-state">
                ${this._emptyStateMessage}
              </div>`
            : this.messages.map(
                (msg) => html`
                  <chat-message-bubble
                    message-id=${msg.id}
                    .sender=${msg.sender}
                    .senderName=${msg.senderName}
                    .content=${msg.content}
                    .renderedHtml=${msg.renderedHtml}
                    ?edited=${msg.edited}
                    reply-to-id=${msg.replyToId}
                    .reactions=${msg.reactions}
                    .toolCalls=${msg.toolCalls}
                    @scroll-to-message=${this._handleScrollToMessage}
                  ></chat-message-bubble>
                `,
              )}
        </div>

        ${this.waitingForAssistant
          ? html`<div class="thinking-indicator">
              <span class="thinking-dots">
                <span></span><span></span><span></span>
              </span>
              ${this._thinkingText}
              <button class="stop-button" @click=${this._handleStopClick} aria-label="Stop">
                Stop
              </button>
            </div>`
          : nothing}

        <div class="input-area" @click=${this._focusTextarea}>
          <textarea
            placeholder="${placeholder}"
            maxlength="${MAX_INPUT_LENGTH}"
            rows="2"
            enterkeyhint="send"
            ?disabled=${!inputEnabled}
            @keydown=${this._handleKeydown}
          ></textarea>
          <button
            class="send-button"
            @click=${this._handleSendClick}
            ?disabled=${!inputEnabled}
            aria-label="Send message"
          >
            Send
          </button>
        </div>
      </div>
    `;
  }

  override openDrawer(): void {
    super.openDrawer();
    this.resizePanel();
    try { localStorage.setItem(STORAGE_KEY, 'true'); } catch { /* */ }
    if (!this._panelEverOpened) {
      this._panelEverOpened = true;
      if (this.page) {
        this.startStream();
      }
    }
    this.updateComplete.then(() => {
      this.scrollToBottom();
      this.focusInput();
    });
  }

  override closeDrawer(): void {
    super.closeDrawer();
    try { localStorage.setItem(STORAGE_KEY, 'false'); } catch { /* */ }
  }

  setAmbientVisible(visible: boolean): void {
    this._fabVisible = visible;
  }

  private _handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      this.sendMessage();
    }
  }

  private _focusTextarea(e: Event) {
    if (e.target instanceof HTMLButtonElement) return; // Don't steal focus from send button
    this.focusInput();
  }

  private _handleSendClick() {
    this.sendMessage();
  }

  private async _handleStopClick() {
    try {
      const request = create(CancelAgentPromptRequestSchema, {
        page: this.page,
      });
      await this.chatClient.cancelAgentPrompt(request);
      this.waitingForAssistant = false;
    } catch (err) {
      this.error = err instanceof Error ? err : new Error(String(err));
    }
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
      this.pendingMessage = null;
    } catch (err) {
      if (err instanceof ConnectError && err.code === Code.Unavailable) {
        // Instance may be starting — queue message for auto-retry
        this.pendingMessage = content;
        this.error = null;
      } else {
        this.error = err instanceof Error ? err : new Error(String(err));
      }
      this.waitingForAssistant = false;
    }

    this.focusInput();
  }

  private handleGlobalKeydown(e: KeyboardEvent): void {
    if (e.ctrlKey && e.code === 'Space') {
      e.preventDefault();
      this.toggleDrawer();
    }
  }

  private async pollChatStatus(): Promise<void> {
    try {
      const request = create(GetChatStatusRequestSchema, {
        page: this.page,
      });
      const response = await this.chatClient.getChatStatus(request);
      const wasConnected = this.agentConnected;
      const wasStarting = this.agentStarting;
      this.agentConnected = response.connected;
      this.poolConnected = response.poolConnected;
      this.agentStarting = response.starting;

      // Adjust poll rate based on state
      this.adjustPollRate();

      // If agent just connected and we have a pending message, auto-send it
      if (!wasConnected && response.connected && this.pendingMessage) {
        const content = this.pendingMessage;
        this.pendingMessage = null;
        this.waitingForAssistant = true;
        try {
          const sendRequest = create(SendChatMessageRequestSchema, {
            page: this.page,
            content,
          });
          await this.chatClient.sendMessage(sendRequest);
        } catch (err) {
          this.pendingMessage = content;
          this.error = err instanceof Error ? err : new Error(String(err));
          this.waitingForAssistant = false;
        }
      }

      // If agent just connected and panel was open, re-focus input
      if (!wasConnected && response.connected && this.drawerOpen) {
        this.focusInput();
      }

      // Clear starting state message when no longer starting
      if (wasStarting && !response.starting) {
        this.error = null;
      }
    } catch {
      this.agentConnected = false;
      this.poolConnected = false;
      this.agentStarting = false;
    }
  }

  private adjustPollRate(): void {
    const desiredIntervalMs = this.agentStarting ? STATUS_POLL_STARTING_MS : STATUS_POLL_INTERVAL_MS;

    // Only restart the timer if the interval needs to change
    if (this.statusPollTimer) {
      clearInterval(this.statusPollTimer);
    }
    this.statusPollTimer = setInterval(() => this.pollChatStatus(), desiredIntervalMs);
  }

  private resizePanel(): void {
    const panel = this.shadowRoot?.querySelector<HTMLElement>('.panel');
    if (!panel) return;

    const viewport = window.visualViewport;
    if (viewport) {
      panel.style.height = `${viewport.height}px`;
      panel.style.top = `${viewport.offsetTop}px`;
    }
  }

  private handleViewportResize(): void {
    if (!window.visualViewport) return;
    this.resizePanel();
  }

  private handleVisibilityChange(): void {
    if (
      document.visibilityState === 'visible' &&
      this.page &&
      this._panelEverOpened &&
      this.streamState !== 'connected'
    ) {
      this.startStream();
    }
  }

  private async waitForReconnectDelay(signal: AbortSignal, delayMs: number): Promise<void> {
    if (signal.aborted) return;
    await new Promise<void>((resolve) => {
      const timer = setTimeout(resolve, delayMs);
      signal.addEventListener(
        'abort',
        () => {
          clearTimeout(timer);
          resolve();
        },
        { once: true },
      );
    });
  }

  private async startStream(): Promise<void> {
    this.stopStream();
    this.streamSubscription = new AbortController();
    const signal = this.streamSubscription.signal;
    let reconnectDelayMs = INITIAL_RECONNECT_DELAY_MS;

    while (!signal.aborted) {
      try {
        await this.runStreamIteration(signal, () => { reconnectDelayMs = INITIAL_RECONNECT_DELAY_MS; });
        break; // Stream ended cleanly
      } catch (err) {
        if (isAbortError(err) || signal.aborted) break;
        reconnectDelayMs = await this.prepareStreamReconnect(err, signal, reconnectDelayMs);
      }
    }

    if (this.streamState !== 'disconnected') {
      this.streamState = 'disconnected';
    }
  }

  private async runStreamIteration(
    signal: AbortSignal,
    onResetDelay: () => void,
  ): Promise<void> {
    const request = create(SubscribeChatRequestSchema, { page: this.page });
    this.streamState = 'connected';
    this.error = null;

    for await (const event of this.chatClient.subscribeChat(request, { signal })) {
      await this.handleChatEvent(event);
      onResetDelay();
    }
  }

  private async prepareStreamReconnect(
    err: unknown,
    signal: AbortSignal,
    reconnectDelayMs: number,
  ): Promise<number> {
    this.streamState = 'reconnecting';
    this.error = err instanceof Error ? err : new Error(String(err));
    await this.waitForReconnectDelay(signal, reconnectDelayMs);
    return Math.min(reconnectDelayMs * 2, MAX_RECONNECT_DELAY_MS);
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
        await this.editMessage(event.event.value.messageId, event.event.value.newContent, event.event.value.streaming);
        break;
      case 'toolCall':
        this.updateToolCall(
          event.event.value.messageId,
          event.event.value.toolCallId,
          event.event.value.title,
          event.event.value.status,
        );
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

    const timestamp = msg.timestamp ? new Date(Number(msg.timestamp.seconds) * 1000) : new Date();
    const fallbackSenderName = msg.sender === Sender.ASSISTANT ? this.persona : '';
    const senderName = msg.senderName || fallbackSenderName;

    const msgState: ChatMessageState = {
      id: msg.id,
      sender: msg.sender,
      content: msg.content,
      renderedHtml,
      timestamp,
      senderName,
      replyToId: msg.replyToId,
      reactions: groupReactions(msg.reactions),
      edited: false,
      sequence: msg.sequence,
      toolCalls: [],
    };

    const existing = this.messagesById.get(msg.id);
    if (existing) {
      // Update in-place for duplicate (replay) messages
      Object.assign(existing, msgState);
      this.messages = [...this.messages]; // trigger re-render
    } else {
      this.messagesById.set(msg.id, msgState);
      this.messages = [...this.messages, msgState];
    }

    await this.updateComplete;
    if (!this.userHasScrolled) {
      this.scrollToBottom();
    }
  }

  private async editMessage(messageId: string, newContent: string, streaming = false): Promise<void> {
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
    if (!streaming) {
      msg.edited = true;
    }
    this.messages = [...this.messages]; // trigger re-render

    // Auto-scroll during streaming if user hasn't scrolled up
    if (streaming && !this.userHasScrolled) {
      await this.updateComplete;
      this.scrollToBottom();
    }
  }

  private updateToolCall(messageId: string, toolCallId: string, title: string, status: string): void {
    const msg = this.messagesById.get(messageId);
    if (!msg) return;

    const existing = msg.toolCalls.find((tc) => tc.toolCallId === toolCallId);
    if (existing) {
      existing.title = title;
      existing.status = status;
      msg.toolCalls = [...msg.toolCalls];
    } else {
      msg.toolCalls = [...msg.toolCalls, { toolCallId, title, status }];
    }
    this.messages = [...this.messages];
  }

  private addReaction(messageId: string, emoji: string, reactor: string): void {
    const msg = this.messagesById.get(messageId);
    if (!msg) return;

    const existing = msg.reactions.find((r) => r.emoji === emoji);
    if (existing) {
      existing.reactors.push(reactor);
      existing.count = existing.reactors.length;
      msg.reactions = [...msg.reactions]; // new reference so Lit detects the change
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

function isAbortError(err: unknown): boolean {
  return err instanceof Error && err.name === 'AbortError';
}

function groupReactions(reactions: Reaction[]): ReactionGroup[] {
  const groups = new Map<string, string[]>();
  for (const r of reactions) {
    const existing = groups.get(r.emoji);
    if (existing) {
      existing.push(r.reactor);
    } else {
      groups.set(r.emoji, [r.reactor]);
    }
  }
  return Array.from(groups.entries()).map(([emoji, reactors]) => ({
    emoji,
    reactors,
    count: reactors.length,
  }));
}

customElements.define('page-chat-panel', PageChatPanel);
