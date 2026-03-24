import { expect, fixture, html } from '@open-wc/testing';
import { stub, spy, type SinonStub, type SinonSpy } from 'sinon';
import './page-chat-panel.js';
import type { PageChatPanel } from './page-chat-panel.js';

// Stub localStorage before tests
let localStorageStub: { getItem: SinonStub; setItem: SinonStub; removeItem: SinonStub };

describe('PageChatPanel', () => {
  beforeEach(() => {
    localStorageStub = {
      getItem: stub(localStorage, 'getItem'),
      setItem: stub(localStorage, 'setItem'),
      removeItem: stub(localStorage, 'removeItem'),
    };
  });

  afterEach(() => {
    localStorageStub.getItem.restore();
    localStorageStub.setItem.restore();
    localStorageStub.removeItem.restore();
  });

  describe('FAB rendering', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
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
      const fab = el.shadowRoot!.querySelector('.fab') as HTMLElement;
      fab.click();
      await el.updateComplete;
    });

    it('should open the panel', () => {
      const panel = el.shadowRoot!.querySelector('.panel');
      expect(panel!.classList.contains('open')).to.be.true;
    });

    it('should add active class to FAB', () => {
      const fab = el.shadowRoot!.querySelector('.fab');
      expect(fab!.classList.contains('active')).to.be.true;
    });

    it('should show close icon instead of robot', () => {
      const icon = el.shadowRoot!.querySelector('.fab i');
      expect(icon!.classList.contains('fa-xmark')).to.be.true;
    });

    it('should save state to localStorage', () => {
      expect(localStorageStub.setItem.calledWith('chat-panel-open', 'true')).to.be.true;
    });
  });

  describe('when FAB is clicked twice', () => {
    let el: PageChatPanel;

    beforeEach(async () => {
      localStorageStub.getItem.returns(null);
      el = await fixture(html`<page-chat-panel page="test-page"></page-chat-panel>`);
      const fab = el.shadowRoot!.querySelector('.fab') as HTMLElement;
      fab.click();
      await el.updateComplete;
      fab.click();
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
