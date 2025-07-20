import { html, fixture, expect, assert } from '@open-wc/testing';
import sinon from 'sinon';
import { ToastMessage, showToast, showStoredToast, storeToastForRefresh } from './toast-message.js';

describe('ToastMessage', () => {
  let el: ToastMessage;

  beforeEach(async () => {
    el = await fixture(html`<toast-message></toast-message>`);
  });

  it('should exist', () => {
    assert.instanceOf(el, ToastMessage);
  });

  it('should have default properties', () => {
    expect(el.message).to.equal('');
    expect(el.type).to.equal('info');
    expect(el.visible).to.be.false;
    expect(el.timeout).to.equal(5000);
    expect(el.autoClose).to.be.true;
  });

  describe('when showing a message', () => {
    beforeEach(async () => {
      el.message = 'Test message';
      el.type = 'success';
      await el.updateComplete;
    });

    it('should display the message', () => {
      const messageElement = el.shadowRoot?.querySelector('.message');
      expect(messageElement?.textContent).to.equal('Test message');
    });

    it('should have correct type class', () => {
      const toastElement = el.shadowRoot?.querySelector('.toast');
      expect(toastElement?.classList.contains('success')).to.be.true;
    });

    it('should show correct icon for success type', () => {
      const iconElement = el.shadowRoot?.querySelector('.icon');
      expect(iconElement?.textContent?.trim()).to.equal('✓');
    });
  });

  describe('when showing different message types', () => {
    it('should show error icon for error type', async () => {
      el.type = 'error';
      await el.updateComplete;
      const iconElement = el.shadowRoot?.querySelector('.icon');
      expect(iconElement?.textContent?.trim()).to.equal('✕');
    });

    it('should show warning icon for warning type', async () => {
      el.type = 'warning';
      await el.updateComplete;
      const iconElement = el.shadowRoot?.querySelector('.icon');
      expect(iconElement?.textContent?.trim()).to.equal('⚠');
    });

    it('should show info icon for info type', async () => {
      el.type = 'info';
      await el.updateComplete;
      const iconElement = el.shadowRoot?.querySelector('.icon');
      expect(iconElement?.textContent?.trim()).to.equal('ℹ');
    });
  });

  describe('when show() is called', () => {
    let clock: sinon.SinonFakeTimers;

    beforeEach(() => {
      clock = sinon.useFakeTimers();
      el.message = 'Test message';
      el.timeout = 1000;
    });

    afterEach(() => {
      clock.restore();
    });

    it('should set visible to true', () => {
      el.show();
      expect(el.visible).to.be.true;
    });

    it('should auto-hide after timeout when autoClose is true', () => {
      el.autoClose = true;
      el.show();
      
      expect(el.visible).to.be.true;
      
      clock.tick(1000);
      
      expect(el.visible).to.be.false;
    });

    it('should not auto-hide when autoClose is false', () => {
      el.autoClose = false;
      el.show();
      
      expect(el.visible).to.be.true;
      
      clock.tick(1000);
      
      expect(el.visible).to.be.true;
    });

    it('should not auto-hide when timeout is 0', () => {
      el.timeout = 0;
      el.show();
      
      expect(el.visible).to.be.true;
      
      clock.tick(5000);
      
      expect(el.visible).to.be.true;
    });
  });

  describe('when hide() is called', () => {
    beforeEach(() => {
      el.visible = true;
    });

    it('should set visible to false', () => {
      el.hide();
      expect(el.visible).to.be.false;
    });
  });

  describe('when close button is clicked', () => {
    beforeEach(async () => {
      el.visible = true;
      await el.updateComplete;
    });

    it('should hide the toast', async () => {
      const closeButton = el.shadowRoot?.querySelector('.close-button') as HTMLButtonElement;
      expect(closeButton).to.exist;
      
      closeButton.click();
      await el.updateComplete;
      
      expect(el.visible).to.be.false;
    });
  });
});

describe('showToast utility function', () => {
  beforeEach(() => {
    // Clean up any existing toast elements
    document.querySelectorAll('toast-message').forEach(el => el.remove());
  });

  afterEach(() => {
    // Clean up after each test
    document.querySelectorAll('toast-message').forEach(el => el.remove());
  });

  it('should create and show a toast message', async () => {
    const toast = showToast('Test message', 'success');
    
    expect(toast).to.be.instanceOf(ToastMessage);
    expect(toast.message).to.equal('Test message');
    expect(toast.type).to.equal('success');
    expect(document.body.contains(toast)).to.be.true;
  });

  it('should use default type if not specified', () => {
    const toast = showToast('Test message');
    expect(toast.type).to.equal('info');
  });

  it('should use custom timeout', () => {
    const toast = showToast('Test message', 'info', 3000);
    expect(toast.timeout).to.equal(3000);
  });
});

describe('sessionStorage toast functions', () => {
  beforeEach(() => {
    // Clean up sessionStorage and DOM
    sessionStorage.clear();
    document.querySelectorAll('toast-message').forEach(el => el.remove());
  });

  afterEach(() => {
    sessionStorage.clear();
    document.querySelectorAll('toast-message').forEach(el => el.remove());
  });

  describe('storeToastForRefresh', () => {
    it('should store message and type in sessionStorage', () => {
      storeToastForRefresh('Success message', 'success');
      
      expect(sessionStorage.getItem('toast-message')).to.equal('Success message');
      expect(sessionStorage.getItem('toast-type')).to.equal('success');
    });

    it('should use default type if not specified', () => {
      storeToastForRefresh('Success message');
      
      expect(sessionStorage.getItem('toast-message')).to.equal('Success message');
      expect(sessionStorage.getItem('toast-type')).to.equal('success');
    });
  });

  describe('showStoredToast', () => {
    it('should show toast from sessionStorage and clear it', async () => {
      sessionStorage.setItem('toast-message', 'Stored message');
      sessionStorage.setItem('toast-type', 'warning');
      
      showStoredToast();
      
      // Should clear from sessionStorage
      expect(sessionStorage.getItem('toast-message')).to.be.null;
      expect(sessionStorage.getItem('toast-type')).to.be.null;
      
      // Should create toast element
      const toast = document.querySelector('toast-message') as ToastMessage;
      expect(toast).to.exist;
      expect(toast.message).to.equal('Stored message');
      expect(toast.type).to.equal('warning');
    });

    it('should do nothing if no stored message exists', () => {
      showStoredToast();
      
      const toast = document.querySelector('toast-message');
      expect(toast).to.be.null;
    });

    it('should use default type if stored type is invalid', () => {
      sessionStorage.setItem('toast-message', 'Stored message');
      // Don't set toast-type, should default to 'info'
      
      showStoredToast();
      
      const toast = document.querySelector('toast-message') as ToastMessage;
      expect(toast).to.exist;
      expect(toast.type).to.equal('info');
    });
  });
});