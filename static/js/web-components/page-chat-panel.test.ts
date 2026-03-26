import { expect, fixture, html } from '@open-wc/testing';
import { stub, spy, type SinonStub, type SinonSpy } from 'sinon';
import './page-chat-panel.js';
import type { PageChatPanel, ChatMessageState } from './page-chat-panel.js';
import { Sender } from '../gen/api/v1/chat_pb.js';
import { resetForTesting } from './drawer-coordinator.js';

// Stub localStorage before tests
let localStorageStub: { getItem: SinonStub; setItem: SinonStub; removeItem: SinonStub };
let fetchStub: SinonStub;

describe('PageChatPanel', () => {
  beforeEach(() => {
    localStorageStub = {
      getItem: stub(localStorage, 'getItem'),
      setItem: stub(localStorage, 'setItem'),
      removeItem: stub(localStorage, 'removeItem'),
    };
    // Stub fetch to prevent real network calls from pollChatStatus
    fetchStub = stub(window, 'fetch');
    fetchStub.resolves(new Response(new Uint8Array(0), { status: 200 }));
  });

  afterEach(() => {
    localStorageStub.getItem.restore();
    localStorageStub.setItem.restore();
    localStorageStub.removeItem.restore();
    fetchStub.restore();
    resetForTesting();
  });

  describe('when Claude is not connected', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
    });

    it('should render the FAB with disabled class', () => {
      const fab = el.shadowRoot!.querySelector('.fab');
      expect(fab).to.not.be.null;
      expect(fab!.classList.contains('disabled')).to.be.true;
    });

    it('should have aria-disabled on the FAB', () => {
      const fab = el.shadowRoot!.querySelector('.fab');
      expect(fab!.getAttribute('aria-disabled')).to.equal('true');
    });

    it('should show disconnected banner when panel is open', async () => {
      el.drawerOpen = true;
      await el.updateComplete;
      const banner = el.shadowRoot!.querySelector('.status-banner.disconnected');
      expect(banner).to.not.be.null;
      expect(banner!.textContent).to.contain('Dorium is not connected');
    });

    it('should disable the textarea when panel is open', async () => {
      el.drawerOpen = true;
      await el.updateComplete;
      const textarea = el.shadowRoot!.querySelector('textarea');
      expect(textarea!.disabled).to.be.true;
    });

    it('should disable the send button when panel is open', async () => {
      el.drawerOpen = true;
      await el.updateComplete;
      const btn = el.shadowRoot!.querySelector('.send-button') as HTMLButtonElement;
      expect(btn.disabled).to.be.true;
    });
  });

  describe('FAB rendering', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
      el.claudeConnected = true;
      await el.updateComplete;
    });

    it('should render the FAB button', () => {
      const fab = el.shadowRoot!.querySelector('.fab');
      expect(fab).to.not.be.null;
    });

    it('should have correct aria-label', () => {
      const fab = el.shadowRoot!.querySelector('.fab');
      expect(fab!.getAttribute('aria-label')).to.equal('Chat with Dorium');
    });

    it('should show robot icon when panel is closed', () => {
      const icon = el.shadowRoot!.querySelector('.fab i');
      expect(icon!.classList.contains('fa-robot')).to.be.true;
    });
  });

  describe('when FAB is clicked', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
      el.claudeConnected = true;
      await el.updateComplete;
      const fab = el.shadowRoot!.querySelector('.fab') as HTMLElement;
      fab.click();
      await el.updateComplete;
    });

    it('should open the panel', () => {
      const panel = el.shadowRoot!.querySelector('.panel');
      expect(panel!.classList.contains('open')).to.be.true;
    });

    it('should hide the FAB', () => {
      const fab = el.shadowRoot!.querySelector('.fab');
      expect(fab).to.be.null;
    });

    it('should save state to localStorage', () => {
      expect(localStorageStub.setItem.calledWith('chat-panel-open', 'true')).to.be.true;
    });
  });

  describe('when FAB is clicked then close button is clicked', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
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
      expect(panel!.classList.contains('open')).to.be.false;
    });

    it('should save closed state to localStorage', () => {
      expect(localStorageStub.setItem.calledWith('chat-panel-open', 'false')).to.be.true;
    });
  });

  describe('when localStorage has panel open', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
    });

    it('should restore the open state', () => {
      expect(el.drawerOpen).to.be.true;
    });

    it('should show the panel as open', () => {
      const panel = el.shadowRoot!.querySelector('.panel');
      expect(panel!.classList.contains('open')).to.be.true;
    });
  });

  describe('panel accessibility', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
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
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
    });

    it('should have a textarea', () => {
      const textarea = el.shadowRoot!.querySelector('textarea');
      expect(textarea).to.not.be.null;
    });

    it('should have maxlength of 2000', () => {
      const textarea = el.shadowRoot!.querySelector('textarea');
      expect(textarea!.getAttribute('maxlength')).to.equal('2000');
    });

    it('should have a send button', () => {
      const btn = el.shadowRoot!.querySelector('.send-button');
      expect(btn).to.not.be.null;
    });
  });

  describe('when Enter key is pressed', () => {
    let el: PageChatPanel;
    let sendSpy: SinonSpy;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
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
      expect(sendSpy.calledOnce).to.be.true;
    });
  });

  describe('when Shift+Enter is pressed', () => {
    let el: PageChatPanel;
    let preventDefaultCalled: boolean;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
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
      expect(preventDefaultCalled).to.be.false;
    });
  });

  describe('empty state', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
    });

    it('should show empty state message when no messages', () => {
      const empty = el.shadowRoot!.querySelector('.empty-state');
      expect(empty).to.not.be.null;
      expect(empty!.textContent).to.contain('Send a message');
    });
  });

  describe('visibility change handling', () => {
    let addEventListenerSpy: SinonSpy;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      addEventListenerSpy = spy(document, 'addEventListener');
      await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
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
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
      el.claudeConnected = true;
      el.streamState = 'disconnected';
      el.error = new Error('Connection lost');
      await el.updateComplete;
    });

    it('should display the error banner', () => {
      const banner = el.shadowRoot!.querySelector('.status-banner.disconnected');
      expect(banner).to.not.be.null;
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
        el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
        el.waitingForAssistant = true;
        await el.updateComplete;
      });

      it('should show the thinking indicator', () => {
        const indicator = el.shadowRoot!.querySelector('.thinking-indicator');
        expect(indicator).to.not.be.null;
      });

      it('should contain thinking text', () => {
        const indicator = el.shadowRoot!.querySelector('.thinking-indicator');
        expect(indicator!.textContent).to.contain('Dorium is thinking');
      });
    });

    describe('when not waiting for assistant', () => {
      let el: PageChatPanel;

      beforeEach(async () => {
        localStorageStub.getItem.returns('true');
        el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
        el.waitingForAssistant = false;
        await el.updateComplete;
      });

      it('should not show the thinking indicator', () => {
        const indicator = el.shadowRoot!.querySelector('.thinking-indicator');
        expect(indicator).to.be.null;
      });
    });
  });

  describe('when Claude is connected', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
      el.claudeConnected = true;
      await el.updateComplete;
    });

    it('should enable the textarea', () => {
      const textarea = el.shadowRoot!.querySelector('textarea');
      expect(textarea!.disabled).to.be.false;
    });

    it('should enable the send button', () => {
      const btn = el.shadowRoot!.querySelector('.send-button') as HTMLButtonElement;
      expect(btn.disabled).to.be.false;
    });
  });

  describe('inert attribute on panel', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
    });

    describe('when panel is closed', () => {
      it('should have inert attribute', () => {
        const panel = el.shadowRoot!.querySelector('.panel');
        expect(panel!.hasAttribute('inert')).to.be.true;
      });
    });

    describe('when panel is open', () => {
      beforeEach(async () => {
        el.drawerOpen = true;
        await el.updateComplete;
      });

      it('should not have inert attribute', () => {
        const panel = el.shadowRoot!.querySelector('.panel');
        expect(panel!.hasAttribute('inert')).to.be.false;
      });
    });
  });

  describe('reconnecting state', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
      el.streamState = 'reconnecting';
      await el.updateComplete;
    });

    it('should show the reconnecting banner', () => {
      const banner = el.shadowRoot!.querySelector('.status-banner.reconnecting');
      expect(banner).to.not.be.null;
      expect(banner!.textContent).to.contain('Reconnecting');
    });
  });

  describe('close button', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns('true');
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
      const closeBtn = el.shadowRoot!.querySelector('.close-button') as HTMLElement;
      closeBtn.click();
      await el.updateComplete;
    });

    it('should close the panel', () => {
      expect(el.drawerOpen).to.be.false;
    });
  });

  describe('addReaction()', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);

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
        expect(group).to.not.be.undefined;
        expect(group!.count).to.equal(1);
        expect(group!.reactors).to.include('bob');
      });

      it('should preserve the existing reaction group', () => {
        const msg = el.messages[0]!;
        const group = msg.reactions.find((r) => r.emoji === '👍');
        expect(group).to.not.be.undefined;
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
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
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
        expect(stored).to.not.be.undefined;
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
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);

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
        expect(el.messages[0]!.edited).to.be.true;
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
        expect(threw).to.be.false;
      });
    });
  });

  describe('Ctrl+Space global keyboard shortcut', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
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
