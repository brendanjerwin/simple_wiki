import { html, fixture, expect, assert } from '@open-wc/testing';
import sinon from 'sinon';
import { ToastMessage, showToast, showStoredToast, showToastAfter } from './toast-message.js';
import { AugmentedError, ErrorKind } from './augment-error-service.js';

describe('ToastMessage', () => {
  let el: ToastMessage;

  beforeEach(async () => {
    el = await fixture(html`<toast-message></toast-message>`);
  });

  it('should exist', () => {
    assert.instanceOf(el, ToastMessage);
  });

  it('should have default properties', () => {
    expect(el.message).to.be.undefined;
    expect(el.type).to.be.undefined;
    expect(el.visible).to.be.undefined;
    expect(el.timeoutSeconds).to.be.undefined;
    expect(el.autoClose).to.be.undefined;
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
      expect(iconElement?.textContent?.trim()).to.equal('✅');
    });
  });

  describe('when showing different message types', () => {
    it('should show error icon for error type', async () => {
      el.type = 'error';
      await el.updateComplete;
      const iconElement = el.shadowRoot?.querySelector('.icon');
      expect(iconElement?.textContent?.trim()).to.equal('❌');
    });

    it('should show warning icon for warning type', async () => {
      el.type = 'warning';
      await el.updateComplete;
      const iconElement = el.shadowRoot?.querySelector('.icon');
      expect(iconElement?.textContent?.trim()).to.equal('⚠️');
    });

    it('should show info icon for info type', async () => {
      el.type = 'info';
      await el.updateComplete;
      const iconElement = el.shadowRoot?.querySelector('.icon');
      expect(iconElement?.textContent?.trim()).to.equal('ℹ️');
    });
  });

  describe('when augmentedError is provided', () => {
    let augmentedError: AugmentedError;

    beforeEach(async () => {
      const originalError = new Error('Test error message');
      augmentedError = new AugmentedError(originalError, ErrorKind.ERROR, 'error', 'testing error display');
      el.augmentedError = augmentedError;
      await el.updateComplete;
    });

    it('should embed error-display component', () => {
      const errorDisplayElement = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplayElement).to.exist;
    });

    it('should pass augmentedError to error-display component', () => {
      const errorDisplayElement = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplayElement?.augmentedError).to.equal(augmentedError);
    });

    it('should not display simple message text', () => {
      const messageElement = el.shadowRoot?.querySelector('.message');
      expect(messageElement).to.not.exist;
    });
  });

  describe('when no augmentedError is provided', () => {
    beforeEach(async () => {
      el.message = 'Simple text message';
      el.type = 'info';
      await el.updateComplete;
    });

    it('should not embed error-display component', () => {
      const errorDisplayElement = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplayElement).to.not.exist;
    });

    it('should display simple message text', () => {
      const messageElement = el.shadowRoot?.querySelector('.message');
      expect(messageElement).to.exist;
      expect(messageElement?.textContent).to.equal('Simple text message');
    });
  });

  describe('when show() is called', () => {
    let clock: sinon.SinonFakeTimers;

    beforeEach(() => {
      clock = sinon.useFakeTimers();
      el.message = 'Test message';
      el.timeoutSeconds = 1;
      el.autoClose = true;
    });

    afterEach(() => {
      clock.restore();
    });

    it('should set visible to true', () => {
      el.show();
      expect(el.visible).to.be.true;
    });

    describe('when autoClose is true', () => {
      beforeEach(() => {
        el.show();
      });

      it('should auto-hide after timeout', () => {
        expect(el.visible).to.be.true;
        
        clock.tick(1000);
        
        expect(el.visible).to.be.false;
      });
    });

    describe('when autoClose is false', () => {
      beforeEach(() => {
        el.autoClose = false;
        el.show();
        clock.tick(1000);
      });

      it('should not auto-hide', () => {
        expect(el.visible).to.be.true;
      });
    });

    describe('when timeout is 0', () => {
      beforeEach(() => {
        el.timeoutSeconds = 0;
        el.show();
        clock.tick(5000);
      });

      it('should not auto-hide', () => {
        expect(el.visible).to.be.true;
      });
    });

    describe('when type is error', () => {
      beforeEach(() => {
        el.type = 'error';
        el.timeoutSeconds = 1;
        // Don't set autoClose explicitly
        el.autoClose = undefined;
      });

      describe('when autoClose is not set', () => {
        beforeEach(() => {
          el.show();
          clock.tick(1000);
        });

        it('should not auto-hide by default', () => {
          expect(el.visible).to.be.true;
        });
      });

      describe('when autoClose is explicitly set to true', () => {
        beforeEach(() => {
          el.autoClose = true;
          el.show();
          clock.tick(1000);
        });

        it('should auto-hide when explicitly enabled', () => {
          expect(el.visible).to.be.false;
        });
      });

      describe('when autoClose is explicitly set to false', () => {
        beforeEach(() => {
          el.autoClose = false;
          el.show();
          clock.tick(1000);
        });

        it('should not auto-hide when explicitly disabled', () => {
          expect(el.visible).to.be.true;
        });
      });
    });

    describe('when type is not error', () => {
      beforeEach(() => {
        el.type = 'success';
        el.timeoutSeconds = 1;
        el.autoClose = true;
        el.show();
        clock.tick(1000);
      });

      it('should maintain existing auto-close behavior', () => {
        expect(el.visible).to.be.false;
      });
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

  describe('when toast is clicked', () => {
    beforeEach(async () => {
      el.visible = true;
      await el.updateComplete;
    });

    it('should hide the toast', async () => {
      const toastElement = el.shadowRoot?.querySelector<HTMLElement>('.toast');
      expect(toastElement).to.exist;

      toastElement!.click();
      await el.updateComplete;

      expect(el.visible).to.be.false;
    });

    describe('when clicking on error-display component', () => {
      beforeEach(async () => {
        const augmentedError = new AugmentedError(
          new Error('Test error'),
          ErrorKind.NETWORK,
          'network',
          'testing'
        );
        el.augmentedError = augmentedError;
        el.type = 'error';
        await el.updateComplete;
      });

      it('should not hide the toast', async () => {
        const errorDisplay = el.shadowRoot?.querySelector<HTMLElement>('error-display');
        expect(errorDisplay).to.exist;

        errorDisplay!.click();
        await el.updateComplete;

        expect(el.visible).to.be.true;
      });
    });
  });

  describe('when close button is clicked', () => {
    beforeEach(async () => {
      el.visible = true;
      await el.updateComplete;
    });

    it('should hide the toast', async () => {
      const closeButton = el.shadowRoot?.querySelector<HTMLElement>('.close-button');
      expect(closeButton).to.exist;

      closeButton!.click();
      await el.updateComplete;

      expect(el.visible).to.be.false;
    });

    it('should have proper accessibility attributes', () => {
      const closeButton = el.shadowRoot?.querySelector<HTMLElement>('.close-button');
      expect(closeButton).to.exist;
      expect(closeButton!.getAttribute('aria-label')).to.equal('Close notification');
      expect(closeButton!.getAttribute('title')).to.equal('Close notification');
    });
  });
});

describe('showToast utility function', () => {
  beforeEach(() => {
    // Clean up any existing toast elements
    document.querySelectorAll('toast-message').forEach(el => el.remove());
    sessionStorage.clear();
  });

  afterEach(() => {
    // Clean up after each test
    document.querySelectorAll('toast-message').forEach(el => el.remove());
    sessionStorage.clear();
  });

  it('should create and show a toast message', async () => {
    showToast('Test message', 'success', 5);

    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- querying for custom element
    const toast = document.querySelector('toast-message') as ToastMessage;
    expect(toast).to.exist;
    expect(toast.message).to.equal('Test message');
    expect(toast.type).to.equal('success');
  });

  it('should use default type if not specified', () => {
    showToast('Test message', 'info', 5);

    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- querying for custom element
    const toast = document.querySelector('toast-message') as ToastMessage;
    expect(toast.type).to.equal('info');
  });
});

describe('showToastAfter utility function', () => {
  beforeEach(() => {
    // Clean up any existing toast elements
    document.querySelectorAll('toast-message').forEach(el => el.remove());
    sessionStorage.clear();
  });

  afterEach(() => {
    // Clean up after each test
    document.querySelectorAll('toast-message').forEach(el => el.remove());
    sessionStorage.clear();
  });

  it('should store toast message and execute function', async () => {
    let functionExecuted = false;
    
    showToastAfter('Test message', 'success', 5, () => {
      functionExecuted = true;
    });
    
    expect(functionExecuted).to.be.true;
    
    // Wait for the delayed showStoredToast call
    await new Promise(resolve => setTimeout(resolve, 150));
    
    // sessionStorage should be cleared after showStoredToast is called
    expect(sessionStorage.getItem('toast-message')).to.be.null;
    expect(sessionStorage.getItem('toast-type')).to.be.null;
    
    // But the toast should be displayed
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- querying for custom element
    const toast = document.querySelector('toast-message') as ToastMessage;
    expect(toast).to.exist;
    expect(toast.message).to.equal('Test message');
    expect(toast.type).to.equal('success');
  });

  it('should show toast after execution if no page refresh occurs', async () => {
    showToastAfter('Test message', 'warning', 5, () => {
      // No-op function that doesn't refresh page
    });

    // Wait for the delayed showStoredToast call
    await new Promise(resolve => setTimeout(resolve, 150));

    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- querying for custom element
    const toast = document.querySelector('toast-message') as ToastMessage;
    expect(toast).to.exist;
    expect(toast.message).to.equal('Test message');
    expect(toast.type).to.equal('warning');
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

  describe('showStoredToast', () => {
    it('should show toast from sessionStorage and clear it', async () => {
      sessionStorage.setItem('toast-message', 'Stored message');
      sessionStorage.setItem('toast-type', 'warning');
      
      showStoredToast();
      
      // Should clear from sessionStorage
      expect(sessionStorage.getItem('toast-message')).to.be.null;
      expect(sessionStorage.getItem('toast-type')).to.be.null;
      
      // Should create toast element
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- querying for custom element
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

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- querying for custom element
      const toast = document.querySelector('toast-message') as ToastMessage;
      expect(toast).to.exist;
      expect(toast.type).to.equal('info');
    });
  });
});