import { expect, fixture, html } from '@open-wc/testing';
import { stub, spy, type SinonStub, type SinonSpy } from 'sinon';
import './page-chat-panel.js';
import type { PageChatPanel } from './page-chat-panel.js';

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
      el.panelOpen = true;
      await el.updateComplete;
      const banner = el.shadowRoot!.querySelector('.status-banner.disconnected');
      expect(banner).to.not.be.null;
      expect(banner!.textContent).to.contain('Claude is not connected');
    });

    it('should disable the textarea when panel is open', async () => {
      el.panelOpen = true;
      await el.updateComplete;
      const textarea = el.shadowRoot!.querySelector('textarea');
      expect(textarea!.disabled).to.be.true;
    });

    it('should disable the send button when panel is open', async () => {
      el.panelOpen = true;
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
      expect(fab!.getAttribute('aria-label')).to.equal('Chat with Claude');
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
      expect(el.panelOpen).to.be.true;
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
        expect(indicator!.textContent).to.contain('Claude is thinking');
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
        el.panelOpen = true;
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
      expect(el.panelOpen).to.be.false;
    });
  });
});
