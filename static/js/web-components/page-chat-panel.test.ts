import { expect, fixture, html } from '@open-wc/testing';
import { stub, spy, useFakeTimers, type SinonStub, type SinonSpy, type SinonFakeTimers } from 'sinon';
import { ConnectError, Code } from '@connectrpc/connect';
import { PageChatPanel } from './page-chat-panel.js';
import type { ChatMessageState } from './page-chat-panel.js';
import { Sender } from '../gen/api/v1/chat_pb.js';
import { resetForTesting } from './drawer-coordinator.js';

// Stub localStorage before tests
let localStorageStub: { getItem: SinonStub; setItem: SinonStub; removeItem: SinonStub };
let fetchStub: SinonStub;
let startStreamStub: SinonStub;
let pollChatStatusStub: SinonStub;

describe('PageChatPanel', () => {
  beforeEach(() => {
    localStorageStub = {
      getItem: stub(localStorage, 'getItem'),
      setItem: stub(localStorage, 'setItem'),
      removeItem: stub(localStorage, 'removeItem'),
    };
    // Stub fetch to prevent real network calls (e.g., from sendMessage)
    fetchStub = stub(window, 'fetch');
    fetchStub.resolves(new Response(new Uint8Array(0), { status: 200 }));
    // Stub startStream and pollChatStatus on the prototype to prevent background
    // gRPC streaming calls and reconnect loops that crash the browser during tests.
    // These are private methods accessed via prototype for testing purposes.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- stubbing private methods for testing
    startStreamStub = stub(PageChatPanel.prototype as any, 'startStream').resolves();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- stubbing private methods for testing
    pollChatStatusStub = stub(PageChatPanel.prototype as any, 'pollChatStatus').resolves();
  });

  afterEach(() => {
    localStorageStub.getItem.restore();
    localStorageStub.setItem.restore();
    localStorageStub.removeItem.restore();
    fetchStub.restore();
    startStreamStub.restore();
    pollChatStatusStub.restore();
    resetForTesting();
  });

  describe('when Claude is not connected', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
    });

    it('should render the FAB with disabled class', () => {
      const fab = el.shadowRoot!.querySelector('.fab');
      expect(fab).to.exist;
      expect(fab!.classList.contains('disabled')).to.equal(true);
    });

    it('should have aria-disabled on the FAB', () => {
      const fab = el.shadowRoot!.querySelector('.fab');
      expect(fab!.getAttribute('aria-disabled')).to.equal('true');
    });

    it('should show disconnected banner when panel is open', async () => {
      el.drawerOpen = true;
      await el.updateComplete;
      const banner = el.shadowRoot!.querySelector('.status-banner.disconnected');
      expect(banner).to.exist;
      expect(banner!.textContent).to.contain('TestPersona is not connected');
    });

    it('should disable the textarea when panel is open', async () => {
      el.drawerOpen = true;
      await el.updateComplete;
      const textarea = el.shadowRoot!.querySelector('textarea');
      expect(textarea!.disabled).to.equal(true);
    });

    it('should disable the send button when panel is open', async () => {
      el.drawerOpen = true;
      await el.updateComplete;
      const btn = el.shadowRoot!.querySelector('.send-button') as HTMLButtonElement;
      expect(btn.disabled).to.equal(true);
    });
  });

  describe('FAB rendering', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
      el.claudeConnected = true;
      await el.updateComplete;
    });

    it('should render the FAB button', () => {
      const fab = el.shadowRoot!.querySelector('.fab');
      expect(fab).to.exist;
    });

    it('should have correct aria-label', () => {
      const fab = el.shadowRoot!.querySelector('.fab');
      expect(fab!.getAttribute('aria-label')).to.equal('Chat with TestPersona');
    });

    it('should show robot icon when panel is closed', () => {
      const icon = el.shadowRoot!.querySelector('.fab i');
      expect(icon!.classList.contains('fa-robot')).to.equal(true);
    });
  });

  describe('when FAB is clicked', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
      el.claudeConnected = true;
      await el.updateComplete;
      const fab = el.shadowRoot!.querySelector('.fab') as HTMLElement;
      fab.click();
      await el.updateComplete;
    });

    it('should open the panel', () => {
      const panel = el.shadowRoot!.querySelector('.panel');
      expect(panel!.classList.contains('open')).to.equal(true);
    });

    it('should hide the FAB', () => {
      const fab = el.shadowRoot!.querySelector('.fab');
      expect(fab).to.equal(null);
    });

    it('should save state to localStorage', () => {
      expect(localStorageStub.setItem.calledWith('chat-panel-open', 'true')).to.equal(true);
    });
  });

  describe('when FAB is clicked then close button is clicked', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
      el.claudeConnected = true;
      await el.updateComplete;
      const fab = el.shadowRoot!.querySelector('.fab') as HTMLElement;
      fab.click();
      await el.updateComplete;
      const closeBtn = el.shadowRoot!.querySelector('.close-button') as HTMLElement;
      closeBtn.click();
      await el.updateComplete;
    });

    it('should close the panel', () => {
      const panel = el.shadowRoot!.querySelector('.panel');
      expect(panel!.classList.contains('open')).to.equal(false);
    });

    it('should save closed state to localStorage', () => {
      expect(localStorageStub.setItem.calledWith('chat-panel-open', 'false')).to.equal(true);
    });
  });

  describe('when localStorage has panel open', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
    });

    it('should restore the open state', () => {
      expect(el.drawerOpen).to.equal(true);
    });

    it('should show the panel as open', () => {
      const panel = el.shadowRoot!.querySelector('.panel');
      expect(panel!.classList.contains('open')).to.equal(true);
    });
  });

  describe('panel accessibility', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
    });

    it('should have role="log" on messages container', () => {
      const container = el.shadowRoot!.querySelector('.messages-container');
      expect(container!.getAttribute('role')).to.equal('log');
    });

    it('should have aria-live="polite" on messages container', () => {
      const container = el.shadowRoot!.querySelector('.messages-container');
      expect(container!.getAttribute('aria-live')).to.equal('polite');
    });

    it('should have aria-label on messages container', () => {
      const container = el.shadowRoot!.querySelector('.messages-container');
      expect(container!.getAttribute('aria-label')).to.equal('Chat messages');
    });
  });

  describe('text input', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
    });

    it('should have a textarea', () => {
      const textarea = el.shadowRoot!.querySelector('textarea');
      expect(textarea).to.exist;
    });

    it('should have maxlength of 2000', () => {
      const textarea = el.shadowRoot!.querySelector('textarea');
      expect(textarea!.getAttribute('maxlength')).to.equal('2000');
    });

    it('should have a send button', () => {
      const btn = el.shadowRoot!.querySelector('.send-button');
      expect(btn).to.exist;
    });
  });

  describe('when Enter key is pressed', () => {
    let el: PageChatPanel;
    let sendSpy: SinonSpy;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
      // We can't easily spy on the private sendMessage method, but we can test
      // that Enter doesn't insert a newline (default prevented)
      const textarea = el.shadowRoot!.querySelector('textarea')!;
      textarea.value = 'test message';
      const event = new KeyboardEvent('keydown', {
        key: 'Enter',
        cancelable: true,
        bubbles: true,
      });
      sendSpy = spy(event, 'preventDefault');
      textarea.dispatchEvent(event);
    });

    it('should prevent default (no newline)', () => {
      expect(sendSpy.calledOnce).to.equal(true);
    });
  });

  describe('when Shift+Enter is pressed', () => {
    let el: PageChatPanel;
    let preventDefaultCalled: boolean;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
      const textarea = el.shadowRoot!.querySelector('textarea')!;
      textarea.value = 'test message';
      const event = new KeyboardEvent('keydown', {
        key: 'Enter',
        shiftKey: true,
        cancelable: true,
        bubbles: true,
      });
      preventDefaultCalled = false;
      event.preventDefault = () => {
        preventDefaultCalled = true;
      };
      textarea.dispatchEvent(event);
    });

    it('should not prevent default (allows newline)', () => {
      expect(preventDefaultCalled).to.equal(false);
    });
  });

  describe('empty state', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
    });

    it('should show empty state message when no messages', () => {
      const empty = el.shadowRoot!.querySelector('.empty-state');
      expect(empty).to.exist;
      expect(empty!.textContent).to.contain('Send a message');
    });
  });

  describe('visibility change handling', () => {
    let addEventListenerSpy: SinonSpy;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      addEventListenerSpy = spy(document, 'addEventListener');
      await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
    });

    afterEach(() => {
      addEventListenerSpy.restore();
    });

    it('should add visibilitychange listener on connect', () => {
      const visibilityCalls = addEventListenerSpy.getCalls().filter(
        (call) => call.args[0] === 'visibilitychange',
      );
      expect(visibilityCalls.length).to.be.greaterThan(0);
    });
  });

  describe('error banner', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
      el.claudeConnected = true;
      el.streamState = 'disconnected';
      el.error = new Error('Connection lost');
      await el.updateComplete;
    });

    it('should display the error banner', () => {
      const banner = el.shadowRoot!.querySelector('.status-banner.disconnected');
      expect(banner).to.exist;
    });

    it('should show the error message', () => {
      const banner = el.shadowRoot!.querySelector('.status-banner.disconnected');
      expect(banner!.textContent).to.contain('Connection lost');
    });
  });

  describe('thinking indicator', () => {
    describe('when waiting for assistant', () => {
      let el: PageChatPanel;

      beforeEach(async () => {
        localStorageStub.getItem.returns('true');
        el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
        el.waitingForAssistant = true;
        await el.updateComplete;
      });

      it('should show the thinking indicator', () => {
        const indicator = el.shadowRoot!.querySelector('.thinking-indicator');
        expect(indicator).to.exist;
      });

      it('should contain thinking text', () => {
        const indicator = el.shadowRoot!.querySelector('.thinking-indicator');
        expect(indicator!.textContent).to.contain('TestPersona is thinking');
      });
    });

    describe('when not waiting for assistant', () => {
      let el: PageChatPanel;

      beforeEach(async () => {
        localStorageStub.getItem.returns('true');
        el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
        el.waitingForAssistant = false;
        await el.updateComplete;
      });

      it('should not show the thinking indicator', () => {
        const indicator = el.shadowRoot!.querySelector('.thinking-indicator');
        expect(indicator).to.equal(null);
      });
    });
  });

  describe('when Claude is connected', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
      el.claudeConnected = true;
      await el.updateComplete;
    });

    it('should enable the textarea', () => {
      const textarea = el.shadowRoot!.querySelector('textarea');
      expect(textarea!.disabled).to.equal(false);
    });

    it('should enable the send button', () => {
      const btn = el.shadowRoot!.querySelector('.send-button') as HTMLButtonElement;
      expect(btn.disabled).to.equal(false);
    });
  });

  describe('inert attribute on panel', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
    });

    describe('when panel is closed', () => {
      it('should have inert attribute', () => {
        const panel = el.shadowRoot!.querySelector('.panel');
        expect(panel!.hasAttribute('inert')).to.equal(true);
      });
    });

    describe('when panel is open', () => {
      beforeEach(async () => {
        el.drawerOpen = true;
        await el.updateComplete;
      });

      it('should not have inert attribute', () => {
        const panel = el.shadowRoot!.querySelector('.panel');
        expect(panel!.hasAttribute('inert')).to.equal(false);
      });
    });
  });

  describe('reconnecting state', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
      el.streamState = 'reconnecting';
      await el.updateComplete;
    });

    it('should show the reconnecting banner', () => {
      const banner = el.shadowRoot!.querySelector('.status-banner.reconnecting');
      expect(banner).to.exist;
      expect(banner!.textContent).to.contain('Reconnecting');
    });
  });

  describe('close button', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
      const closeBtn = el.shadowRoot!.querySelector('.close-button') as HTMLElement;
      closeBtn.click();
      await el.updateComplete;
    });

    it('should close the panel', () => {
      expect(el.drawerOpen).to.equal(false);
    });
  });

  describe('addReaction()', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);

      // Seed a message with one reaction group directly into state
      const msgState: ChatMessageState = {
        id: 'msg-1',
        sender: Sender.USER,
        content: 'Hello',
        renderedHtml: '',
        timestamp: new Date(),
        senderName: 'User',
        replyToId: '',
        reactions: [{ emoji: '👍', reactors: ['alice'], count: 1 }],
        edited: false,
        sequence: 0n,
      };
      (el as unknown as { messagesById: Map<string, ChatMessageState> }).messagesById.set('msg-1', msgState);
      el.messages = [msgState];
      await el.updateComplete;
    });

    describe('when adding a reaction with a new emoji', () => {
      beforeEach(async () => {
        (el as unknown as { addReaction(id: string, emoji: string, reactor: string): void }).addReaction('msg-1', '❤️', 'bob');
        await el.updateComplete;
      });

      it('should add a new reaction group', () => {
        const msg = el.messages[0]!;
        const group = msg.reactions.find((r) => r.emoji === '❤️');
        expect(group).to.not.equal(undefined);
        expect(group!.count).to.equal(1);
        expect(group!.reactors).to.include('bob');
      });

      it('should preserve the existing reaction group', () => {
        const msg = el.messages[0]!;
        const group = msg.reactions.find((r) => r.emoji === '👍');
        expect(group).to.not.equal(undefined);
      });
    });

    describe('when adding a reaction with an existing emoji', () => {
      let reactionsBefore: typeof el.messages[0]['reactions'];

      beforeEach(async () => {
        reactionsBefore = el.messages[0]!.reactions;
        (el as unknown as { addReaction(id: string, emoji: string, reactor: string): void }).addReaction('msg-1', '👍', 'bob');
        await el.updateComplete;
      });

      it('should update the reactor count', () => {
        const group = el.messages[0]!.reactions.find((r) => r.emoji === '👍');
        expect(group!.count).to.equal(2);
        expect(group!.reactors).to.include('bob');
      });

      it('should create a new reactions array reference for Lit reactivity', () => {
        expect(el.messages[0]!.reactions).to.not.equal(reactionsBefore);
      });
    });

    describe('when adding a reaction to a non-existent message', () => {
      it('should not throw', () => {
        expect(() => {
          (el as unknown as { addReaction(id: string, emoji: string, reactor: string): void }).addReaction('no-such-id', '👍', 'bob');
        }).to.not.throw();
      });
    });
  });

  describe('addMessage()', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
    });

    describe('when adding a new user message', () => {
      beforeEach(async () => {
        await (el as unknown as { addMessage(msg: object): Promise<void> }).addMessage({
          id: 'msg-1',
          sender: Sender.USER,
          content: 'Hello',
          senderName: 'User',
          replyToId: '',
          reactions: [],
          sequence: 0n,
          timestamp: null,
        });
        await el.updateComplete;
      });

      it('should add the message to messages array', () => {
        expect(el.messages).to.have.length(1);
        expect(el.messages[0]!.id).to.equal('msg-1');
      });

      it('should store the message in messagesById', () => {
        const stored = (el as unknown as { messagesById: Map<string, ChatMessageState> }).messagesById.get('msg-1');
        expect(stored).to.not.equal(undefined);
      });
    });

    describe('when adding an assistant message with no senderName', () => {
      beforeEach(async () => {
        await (el as unknown as { addMessage(msg: object): Promise<void> }).addMessage({
          id: 'msg-assistant-1',
          sender: Sender.ASSISTANT,
          content: 'Hello from the assistant',
          senderName: '',
          replyToId: '',
          reactions: [],
          sequence: 0n,
          timestamp: null,
        });
        await el.updateComplete;
      });

      it('should substitute the persona name as senderName', () => {
        expect(el.messages[0]!.senderName).to.equal('TestPersona');
      });

      it('should render the persona name in the chat bubble', () => {
        const bubble = el.shadowRoot!.querySelector('chat-message-bubble');
        expect(bubble).to.exist;
        const senderDiv = bubble!.shadowRoot!.querySelector('.sender-name');
        expect(senderDiv).to.exist;
        expect(senderDiv!.textContent).to.equal('TestPersona');
      });
    });

    describe('when adding an assistant message with an explicit senderName', () => {
      beforeEach(async () => {
        await (el as unknown as { addMessage(msg: object): Promise<void> }).addMessage({
          id: 'msg-assistant-2',
          sender: Sender.ASSISTANT,
          content: 'Hello from a named assistant',
          senderName: 'CustomBot',
          replyToId: '',
          reactions: [],
          sequence: 0n,
          timestamp: null,
        });
        await el.updateComplete;
      });

      it('should keep the explicit senderName', () => {
        expect(el.messages[0]!.senderName).to.equal('CustomBot');
      });
    });

    describe('when the same message is added twice (replay deduplication)', () => {
      const msg = {
        id: 'msg-dup',
        sender: Sender.USER,
        content: 'Original',
        senderName: 'User',
        replyToId: '',
        reactions: [],
        sequence: 0n,
        timestamp: null,
      };

      beforeEach(async () => {
        await (el as unknown as { addMessage(msg: object): Promise<void> }).addMessage(msg);
        await (el as unknown as { addMessage(msg: object): Promise<void> }).addMessage({ ...msg, content: 'Replayed' });
        await el.updateComplete;
      });

      it('should not add a duplicate entry', () => {
        expect(el.messages).to.have.length(1);
      });

      it('should update content in place', () => {
        expect(el.messages[0]!.content).to.equal('Replayed');
      });
    });
  });

  describe('editMessage()', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);

      const msgState: ChatMessageState = {
        id: 'edit-msg',
        sender: Sender.USER,
        content: 'Original content',
        renderedHtml: '',
        timestamp: new Date(),
        senderName: 'User',
        replyToId: '',
        reactions: [],
        edited: false,
        sequence: 0n,
      };
      (el as unknown as { messagesById: Map<string, ChatMessageState> }).messagesById.set('edit-msg', msgState);
      el.messages = [msgState];
      await el.updateComplete;
    });

    describe('when editing an existing message', () => {
      beforeEach(async () => {
        await (el as unknown as { editMessage(id: string, newContent: string): Promise<void> }).editMessage('edit-msg', 'Updated content');
        await el.updateComplete;
      });

      it('should update the message content', () => {
        expect(el.messages[0]!.content).to.equal('Updated content');
      });

      it('should mark the message as edited', () => {
        expect(el.messages[0]!.edited).to.equal(true);
      });
    });

    describe('when editing a non-existent message', () => {
      it('should not throw', async () => {
        let threw = false;
        try {
          await (el as unknown as { editMessage(id: string, newContent: string): Promise<void> }).editMessage('no-such-id', 'New content');
        } catch {
          threw = true;
        }
        expect(threw).to.equal(false);
      });
    });
  });

  describe('error clearing after reconnect', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
      el.claudeConnected = true;
      el.streamState = 'disconnected';
      el.error = new Error('Connection lost');
      await el.updateComplete;
    });

    describe('when the stream reconnects and error is cleared', () => {
      beforeEach(async () => {
        el.streamState = 'connected';
        el.error = null;
        await el.updateComplete;
      });

      it('should not show the error banner', () => {
        const banner = el.shadowRoot!.querySelector('.status-banner.disconnected');
        expect(banner).to.equal(null);
      });

      it('should have null error state', () => {
        expect(el.error).to.equal(null);
      });
    });
  });

  describe('Ctrl+Space global keyboard shortcut', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page" persona="TestPersona"></page-chat-panel>`);
      el.claudeConnected = true;
      await el.updateComplete;
    });

    describe('when Ctrl+Space is pressed', () => {
      let initialState: boolean;

      beforeEach(async () => {
        initialState = el.drawerOpen;
        const event = new KeyboardEvent('keydown', {
          key: ' ',
          code: 'Space',
          ctrlKey: true,
          cancelable: true,
          bubbles: true,
        });
        document.dispatchEvent(event);
        await el.updateComplete;
      });

      it('should toggle the panel open state', () => {
        expect(el.drawerOpen).to.not.equal(initialState);
      });
    });
  });
});

// Separate describe block for testing stream iteration methods directly.
// Does NOT stub startStream on the prototype — creates fixtures without a `page`
// attribute so connectedCallback doesn't auto-invoke startStream.
describe('PageChatPanel stream methods', () => {
  let el: PageChatPanel;
  let localStorageGetItemStub: SinonStub;
  let localStorageSetItemStub: SinonStub;
  let localStorageRemoveItemStub: SinonStub;
  let fetchStubInner: SinonStub;
  let pollChatStatusInnerStub: SinonStub;

  beforeEach(async () => {
    localStorageGetItemStub = stub(localStorage, 'getItem').returns(null);
    localStorageSetItemStub = stub(localStorage, 'setItem');
    localStorageRemoveItemStub = stub(localStorage, 'removeItem');
    fetchStubInner = stub(window, 'fetch').resolves(new Response(new Uint8Array(0), { status: 200 }));
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- stubbing private method for testing
    pollChatStatusInnerStub = stub(PageChatPanel.prototype as any, 'pollChatStatus').resolves();
    // No page attribute → connectedCallback skips startStream
    el = await fixture(html`<page-chat-panel></page-chat-panel>`);
  });

  afterEach(() => {
    localStorageGetItemStub.restore();
    localStorageSetItemStub.restore();
    localStorageRemoveItemStub.restore();
    fetchStubInner.restore();
    pollChatStatusInnerStub.restore();
    resetForTesting();
  });

  describe('runStreamIteration()', () => {

    describe('when subscribeChat yields events and completes', () => {
      let onResetDelayCallCount: number;

      beforeEach(async () => {
        onResetDelayCallCount = 0;
        const controller = new AbortController();
        // Do NOT set el.page via the property — that would trigger updated() → startStream()
        el.error = new Error('old error');
        el.streamState = 'reconnecting';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
        (el as any).chatClient = {
          subscribeChat: async function* () {
            yield { event: { case: undefined } };
            yield { event: { case: undefined } };
          },
        };

        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).runStreamIteration(controller.signal, () => { onResetDelayCallCount++; });
        await el.updateComplete;
      });

      it('should set streamState to connected', () => {
        expect(el.streamState).to.equal('connected');
      });

      it('should clear the error', () => {
        expect(el.error).to.equal(null);
      });

      it('should call onResetDelay for each event', () => {
        expect(onResetDelayCallCount).to.equal(2);
      });
    });

    describe('when subscribeChat completes immediately with no events', () => {

      beforeEach(async () => {
        const controller = new AbortController();
        // Do NOT set el.page via the property — that would trigger updated() → startStream()
        el.error = new Error('stale error');
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
        (el as any).chatClient = {
          subscribeChat: async function* () { /* empty */ },
        };

        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).runStreamIteration(controller.signal, () => { /* noop */ });
        await el.updateComplete;
      });

      it('should set streamState to connected', () => {
        expect(el.streamState).to.equal('connected');
      });

      it('should clear the error', () => {
        expect(el.error).to.equal(null);
      });
    });
  });

  describe('prepareStreamReconnect()', () => {

    describe('when called with an already-aborted signal', () => {
      let returnedDelay: number;
      const testError = new Error('stream error');

      beforeEach(async () => {
        const controller = new AbortController();
        controller.abort(); // Pre-abort so waitForReconnectDelay resolves immediately
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        returnedDelay = await (el as any).prepareStreamReconnect(testError, controller.signal, 1000);
        await el.updateComplete;
      });

      it('should set streamState to reconnecting', () => {
        expect(el.streamState).to.equal('reconnecting');
      });

      it('should set the error from the caught error', () => {
        expect(el.error).to.equal(testError);
      });

      it('should return the doubled delay', () => {
        expect(returnedDelay).to.equal(2000);
      });
    });

    describe('when delay would exceed the maximum', () => {
      let returnedDelay: number;

      beforeEach(async () => {
        const controller = new AbortController();
        controller.abort();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        returnedDelay = await (el as any).prepareStreamReconnect(new Error('err'), controller.signal, 20000);
      });

      it('should cap the delay at 30000ms', () => {
        expect(returnedDelay).to.equal(30000);
      });
    });

    describe('when the error is not an Error instance', () => {

      beforeEach(async () => {
        const controller = new AbortController();
        controller.abort();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).prepareStreamReconnect('string error', controller.signal, 1000);
        await el.updateComplete;
      });

      it('should wrap the value in an Error object', () => {
        expect(el.error).to.be.instanceof(Error);
        expect(el.error!.message).to.equal('string error');
      });
    });
  });

  describe('waitForReconnectDelay()', () => {
    let clock: SinonFakeTimers;

    beforeEach(() => {
      clock = useFakeTimers();
    });

    afterEach(() => {
      clock.restore();
    });

    describe('when signal is already aborted', () => {
      let resolved: boolean;

      beforeEach(async () => {
        resolved = false;
        const controller = new AbortController();
        controller.abort();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        (el as any).waitForReconnectDelay(controller.signal, 5000).then(() => {
          resolved = true;
        });
        await clock.tickAsync(0);
      });

      it('should resolve immediately without waiting', () => {
        expect(resolved).to.equal(true);
      });
    });

    describe('when the delay has not yet elapsed', () => {
      let resolved: boolean;

      beforeEach(async () => {
        resolved = false;
        const controller = new AbortController();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        (el as any).waitForReconnectDelay(controller.signal, 2000).then(() => {
          resolved = true;
        });
        await clock.tickAsync(1999);
      });

      it('should not resolve before the delay elapses', () => {
        expect(resolved).to.equal(false);
      });

      describe('after the delay elapses', () => {

        beforeEach(async () => {
          await clock.tickAsync(1);
        });

        it('should resolve', () => {
          expect(resolved).to.equal(true);
        });
      });
    });

    describe('when signal is aborted mid-delay', () => {
      let resolved: boolean;

      beforeEach(async () => {
        resolved = false;
        const controller = new AbortController();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        (el as any).waitForReconnectDelay(controller.signal, 5000).then(() => {
          resolved = true;
        });
        await clock.tickAsync(1000);
        controller.abort();
        await clock.tickAsync(0);
      });

      it('should resolve early when signal is aborted', () => {
        expect(resolved).to.equal(true);
      });
    });
  });

  describe('startStream()', () => {
    // Note: We do NOT set el.page via the property in these tests — doing so would trigger
    // Lit's updated() lifecycle which auto-calls startStream() and interferes with our test.
    // The mock subscribeChat ignores the page value in the request, so el.page can be empty.

    describe('when subscribeChat completes cleanly with no events', () => {

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
        (el as any).chatClient = {
          subscribeChat: async function* () { /* empty — clean end */ },
        };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).startStream();
        await el.updateComplete;
      });

      it('should end in disconnected state', () => {
        expect(el.streamState).to.equal('disconnected');
      });
    });

    describe('when subscribeChat throws an AbortError', () => {

      beforeEach(async () => {
        const abortError = new Error('The operation was aborted');
        abortError.name = 'AbortError';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
        (el as any).chatClient = {
          subscribeChat: async function* () {
            yield* [];
            throw abortError;
          },
        };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).startStream();
        await el.updateComplete;
      });

      it('should exit without reconnecting and end in disconnected state', () => {
        expect(el.streamState).to.equal('disconnected');
      });
    });

    describe('when subscribeChat throws a non-abort error', () => {

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
        (el as any).chatClient = {
          subscribeChat: async function* () {
            yield* [];
            throw new Error('network error');
          },
        };
        // Stub prepareStreamReconnect to abort the stream immediately and prevent real delays
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- stubbing private method for testing
        stub(el as any, 'prepareStreamReconnect').callsFake(async () => {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private field for testing
          (el as any).streamSubscription?.abort();
          return 2000;
        });
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).startStream();
        await el.updateComplete;
      });

      it('should attempt reconnect and end in disconnected state', () => {
        expect(el.streamState).to.equal('disconnected');
      });
    });
  });

  describe('stopStream()', () => {

    describe('when there is an active stream subscription', () => {
      let abortSpy: SinonSpy;

      beforeEach(() => {
        const controller = new AbortController();
        abortSpy = spy(controller, 'abort');
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private field for testing
        (el as any).streamSubscription = controller;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        (el as any).stopStream();
      });

      it('should abort the subscription', () => {
        expect(abortSpy.callCount).to.equal(1);
      });

      it('should set streamState to disconnected', () => {
        expect(el.streamState).to.equal('disconnected');
      });

      it('should clear streamSubscription', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private field for testing
        expect((el as any).streamSubscription).to.equal(undefined);
      });
    });

    describe('when there is no active stream subscription', () => {

      it('should not throw', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        expect(() => (el as any).stopStream()).to.not.throw();
      });
    });
  });

  describe('handleChatEvent()', () => {

    describe('when event is newMessage', () => {

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).handleChatEvent({
          event: {
            case: 'newMessage',
            value: {
              id: 'evt-msg-1',
              sender: Sender.USER,
              content: 'Hello from event',
              senderName: 'User',
              replyToId: '',
              reactions: [],
              sequence: 0n,
              timestamp: null,
            },
          },
        });
        await el.updateComplete;
      });

      it('should add the message', () => {
        expect(el.messages).to.have.length(1);
        expect(el.messages[0]!.id).to.equal('evt-msg-1');
      });
    });

    describe('when event is edit', () => {

      beforeEach(async () => {
        const msgState: ChatMessageState = {
          id: 'edit-evt-msg',
          sender: Sender.USER,
          content: 'Original',
          renderedHtml: '',
          timestamp: new Date(),
          senderName: 'User',
          replyToId: '',
          reactions: [],
          edited: false,
          sequence: 0n,
        };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private field for testing
        (el as any).messagesById.set('edit-evt-msg', msgState);
        el.messages = [msgState];

        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).handleChatEvent({
          event: {
            case: 'edit',
            value: { messageId: 'edit-evt-msg', newContent: 'Edited content' },
          },
        });
        await el.updateComplete;
      });

      it('should update the message content', () => {
        expect(el.messages[0]!.content).to.equal('Edited content');
      });

      it('should mark the message as edited', () => {
        expect(el.messages[0]!.edited).to.equal(true);
      });
    });

    describe('when event is reaction', () => {

      beforeEach(async () => {
        const msgState: ChatMessageState = {
          id: 'reaction-evt-msg',
          sender: Sender.USER,
          content: 'React to me',
          renderedHtml: '',
          timestamp: new Date(),
          senderName: 'User',
          replyToId: '',
          reactions: [],
          edited: false,
          sequence: 0n,
        };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private field for testing
        (el as any).messagesById.set('reaction-evt-msg', msgState);
        el.messages = [msgState];

        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).handleChatEvent({
          event: {
            case: 'reaction',
            value: { messageId: 'reaction-evt-msg', emoji: '👍', reactor: 'alice' },
          },
        });
        await el.updateComplete;
      });

      it('should add the reaction', () => {
        const reactions = el.messages[0]!.reactions;
        expect(reactions).to.have.length(1);
        expect(reactions[0]!.emoji).to.equal('👍');
      });
    });

    describe('when event case is undefined', () => {

      it('should not throw', async () => {
        let threw = false;
        try {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
          await (el as any).handleChatEvent({ event: { case: undefined } });
        } catch {
          threw = true;
        }
        expect(threw).to.equal(false);
      });
    });
  });

  describe('handleVisibilityChange()', () => {
    let startStreamSpy: SinonStub;

    beforeEach(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- stubbing private method for testing
      startStreamSpy = stub(el as any, 'startStream').resolves();
    });

    describe('when document becomes visible and stream is not connected and page is set', () => {

      beforeEach(() => {
        el.page = 'test-page';
        el.streamState = 'disconnected';
        Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        (el as any).handleVisibilityChange();
      });

      afterEach(() => {
        Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
      });

      it('should call startStream', () => {
        expect(startStreamSpy.callCount).to.equal(1);
      });
    });

    describe('when document becomes visible but stream is already connected', () => {

      beforeEach(() => {
        el.page = 'test-page';
        el.streamState = 'connected';
        Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        (el as any).handleVisibilityChange();
      });

      it('should not call startStream', () => {
        expect(startStreamSpy.callCount).to.equal(0);
      });
    });

    describe('when document is hidden', () => {

      beforeEach(() => {
        el.page = 'test-page';
        el.streamState = 'disconnected';
        Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true });
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        (el as any).handleVisibilityChange();
      });

      afterEach(() => {
        Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
      });

      it('should not call startStream', () => {
        expect(startStreamSpy.callCount).to.equal(0);
      });
    });

    describe('when page is not set', () => {

      beforeEach(() => {
        el.page = '';
        el.streamState = 'disconnected';
        Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        (el as any).handleVisibilityChange();
      });

      it('should not call startStream', () => {
        expect(startStreamSpy.callCount).to.equal(0);
      });
    });
  });
});

// Separate describe block for pollChatStatus and sendMessage.
// Does NOT stub pollChatStatus on the prototype so we can test the real implementation.
describe('PageChatPanel pollChatStatus and sendMessage', () => {
  let el: PageChatPanel;
  let localStorageGetItemStub: SinonStub;
  let localStorageSetItemStub: SinonStub;
  let localStorageRemoveItemStub: SinonStub;
  let fetchStubInner: SinonStub;
  let startStreamInnerStub: SinonStub;

  beforeEach(async () => {
    localStorageGetItemStub = stub(localStorage, 'getItem').returns(null);
    localStorageSetItemStub = stub(localStorage, 'setItem');
    localStorageRemoveItemStub = stub(localStorage, 'removeItem');
    fetchStubInner = stub(window, 'fetch').resolves(new Response(new Uint8Array(0), { status: 200 }));
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- stubbing private method for testing
    startStreamInnerStub = stub(PageChatPanel.prototype as any, 'startStream').resolves();
    // No page attribute → connectedCallback skips startStream
    el = await fixture(html`<page-chat-panel></page-chat-panel>`);
    // Replace chatClient with a safe mock to prevent real network calls
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
    (el as any).chatClient = {
      getChatStatus: stub().resolves({ connected: false }),
      sendMessage: stub().resolves(),
      subscribeChat: async function* () { /* empty */ },
    };
  });

  afterEach(() => {
    localStorageGetItemStub.restore();
    localStorageSetItemStub.restore();
    localStorageRemoveItemStub.restore();
    fetchStubInner.restore();
    startStreamInnerStub.restore();
    resetForTesting();
  });

  describe('pollChatStatus()', () => {

    describe('when getChatStatus returns connected=true', () => {

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
        (el as any).chatClient = {
          getChatStatus: stub().resolves({ connected: true }),
        };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).pollChatStatus();
        await el.updateComplete;
      });

      it('should set claudeConnected to true', () => {
        expect(el.claudeConnected).to.equal(true);
      });
    });

    describe('when getChatStatus returns connected=false', () => {

      beforeEach(async () => {
        el.claudeConnected = true;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
        (el as any).chatClient = {
          getChatStatus: stub().resolves({ connected: false }),
        };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).pollChatStatus();
        await el.updateComplete;
      });

      it('should set claudeConnected to false', () => {
        expect(el.claudeConnected).to.equal(false);
      });
    });

    describe('when getChatStatus throws', () => {

      beforeEach(async () => {
        el.claudeConnected = true;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
        (el as any).chatClient = {
          getChatStatus: stub().rejects(new Error('network error')),
        };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).pollChatStatus();
        await el.updateComplete;
      });

      it('should set claudeConnected to false', () => {
        expect(el.claudeConnected).to.equal(false);
      });
    });

    describe('when Claude just connected while panel is open', () => {
      let focusInputSpy: SinonSpy;

      beforeEach(async () => {
        el.claudeConnected = false;
        el.drawerOpen = true;
        await el.updateComplete;
        focusInputSpy = spy(el as unknown as { focusInput: () => void }, 'focusInput');
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
        (el as any).chatClient = {
          getChatStatus: stub().resolves({ connected: true }),
        };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).pollChatStatus();
        await el.updateComplete;
      });

      it('should call focusInput', () => {
        expect(focusInputSpy.called).to.equal(true);
      });
    });
  });

  describe('sendMessage()', () => {

    describe('when textarea is empty', () => {
      let sendMessageStub: SinonStub;

      beforeEach(async () => {
        el.page = 'test-page';
        el.claudeConnected = true;
        el.drawerOpen = true;
        await el.updateComplete;
        sendMessageStub = stub().resolves();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
        (el as any).chatClient = { sendMessage: sendMessageStub };
        const textarea = el.shadowRoot!.querySelector('textarea')!;
        textarea.value = '';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).sendMessage();
      });

      it('should not call chatClient.sendMessage', () => {
        expect(sendMessageStub.callCount).to.equal(0);
      });
    });

    describe('when message is sent successfully', () => {
      let sendMessageStub: SinonStub;

      beforeEach(async () => {
        el.page = 'test-page';
        el.claudeConnected = true;
        el.drawerOpen = true;
        await el.updateComplete;
        sendMessageStub = stub().resolves();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
        (el as any).chatClient = { sendMessage: sendMessageStub };
        const textarea = el.shadowRoot!.querySelector('textarea')!;
        textarea.value = 'Hello!';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).sendMessage();
        await el.updateComplete;
      });

      it('should call chatClient.sendMessage', () => {
        expect(sendMessageStub.callCount).to.equal(1);
      });

      it('should clear the textarea', () => {
        const textarea = el.shadowRoot!.querySelector('textarea')!;
        expect(textarea.value).to.equal('');
      });
    });

    describe('when sendMessage throws a ConnectError Unavailable', () => {

      beforeEach(async () => {
        el.page = 'test-page';
        el.claudeConnected = true;
        el.drawerOpen = true;
        await el.updateComplete;
        const connectError = new ConnectError('service unavailable', Code.Unavailable);
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
        (el as any).chatClient = { sendMessage: stub().rejects(connectError) };
        const textarea = el.shadowRoot!.querySelector('textarea')!;
        textarea.value = 'Hello!';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).sendMessage();
        await el.updateComplete;
      });

      it('should set the error', () => {
        expect(el.error).to.exist;
        expect(el.error).to.be.instanceof(ConnectError);
      });

      it('should reset waitingForAssistant', () => {
        expect(el.waitingForAssistant).to.equal(false);
      });
    });

    describe('when sendMessage throws a generic Error', () => {

      beforeEach(async () => {
        el.page = 'test-page';
        el.claudeConnected = true;
        el.drawerOpen = true;
        await el.updateComplete;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- replacing private chatClient for testing
        (el as any).chatClient = { sendMessage: stub().rejects(new Error('generic network error')) };
        const textarea = el.shadowRoot!.querySelector('textarea')!;
        textarea.value = 'Hello!';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- calling private method for testing
        await (el as any).sendMessage();
        await el.updateComplete;
      });

      it('should set error with the original message', () => {
        expect(el.error).to.exist;
        expect(el.error!.message).to.equal('generic network error');
      });

      it('should reset waitingForAssistant', () => {
        expect(el.waitingForAssistant).to.equal(false);
      });
    });
  });
});
