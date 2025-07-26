import { html, fixture, expect } from '@open-wc/testing';
import { KernelPanic, showKernelPanic } from './kernel-panic.js';
import { AugmentErrorService, AugmentedError, ErrorKind } from './augment-error-service.js';
import * as sinon from 'sinon';

describe('KernelPanic', () => {
  let el: KernelPanic;

  describe('should exist', () => {
    beforeEach(async () => {
      el = await fixture(html`<kernel-panic></kernel-panic>`);
    });

    it('should exist', () => {
      expect(el).to.exist;
    });

    it('should be an instance of KernelPanic', () => {
      expect(el).to.be.instanceof(KernelPanic);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName.toLowerCase()).to.equal('kernel-panic');
    });
  });

  describe('when component is initialized', () => {
    beforeEach(async () => {
      el = await fixture(html`<kernel-panic></kernel-panic>`);
    });

    it('should have no augmentedError by default', () => {
      expect(el.augmentedError).to.be.null;
    });

    it('should render header with skull and title', () => {
      const header = el.shadowRoot?.querySelector('.header');
      const skull = header?.querySelector('.skull');
      const title = header?.querySelector('.title');
      
      expect(header).to.exist;
      expect(skull?.textContent).to.equal('💀');
      expect(title?.textContent).to.equal('Kernel Panic');
    });

    it('should render instructions', () => {
      const instructions = el.shadowRoot?.querySelector('.instructions');
      expect(instructions).to.exist;
    });

    it('should render refresh button', () => {
      const button = el.shadowRoot?.querySelector('.refresh-button');
      expect(button).to.exist;
      expect(button?.textContent?.trim()).to.equal('Refresh Page');
    });
  });

  describe('when refresh button is clicked', () => {
    beforeEach(async () => {
      el = await fixture(html`<kernel-panic></kernel-panic>`);
    });

    it('should have a refresh handler method', () => {
      // Verify the component has the _handleRefresh method
      expect(typeof (el as KernelPanic & { _handleRefresh: () => void })._handleRefresh).to.equal('function');
    });

    it('should render button with click handler', () => {
      const button = el.shadowRoot?.querySelector('.refresh-button') as HTMLButtonElement;
      expect(button).to.exist;
      expect(button.textContent?.trim()).to.equal('Refresh Page');
      
      // Verify button is clickable
      expect(button.disabled).to.be.false;
    });
  });

  describe('when augmentedError is provided', () => {
    beforeEach(async () => {
      const augmentedError = new AugmentedError(
        'System failure detected',
        ErrorKind.ERROR,
        'error',
        'Stack trace: Error at line 42\nCaused by memory leak'
      );
      el = await fixture(html`<kernel-panic .augmentedError="${augmentedError}"></kernel-panic>`);
    });

    it('should have augmentedError property', () => {
      expect(el.augmentedError).to.exist;
      expect(el.augmentedError?.message).to.equal('System failure detected');
    });

    it('should display the error using error-display component', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.exist;
      expect(errorDisplay?.augmentedError).to.equal(el.augmentedError);
    });
  });
  
  describe('when no augmentedError is provided', () => {
    beforeEach(async () => {
      el = await fixture(html`<kernel-panic></kernel-panic>`);
    });

    it('should not render error-display component', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.not.exist;
    });
  });
});

describe('showKernelPanic function', () => {
  let augmentErrorSpy: sinon.SinonSpy;

  beforeEach(() => {
    // Clean up any existing kernel-panic elements
    document.querySelectorAll('kernel-panic').forEach(el => el.remove());
    
    // Spy on AugmentErrorService.augmentError
    augmentErrorSpy = sinon.spy(AugmentErrorService, 'augmentError');
  });

  afterEach(() => {
    sinon.restore();
    // Clean up created elements
    document.querySelectorAll('kernel-panic').forEach(el => el.remove());
  });

  describe('when called with error', () => {
    beforeEach(() => {
      const testError = new Error('Test system error');
      showKernelPanic(testError);
    });

    it('should call AugmentErrorService.augmentError with the error', () => {
      expect(augmentErrorSpy).to.have.been.calledOnce;
      expect(augmentErrorSpy).to.have.been.calledWith(
        sinon.match.instanceOf(Error).and(sinon.match.has('message', 'Test system error'))
      );
    });

    it('should create a kernel-panic element in document body', () => {
      const kernelPanicElement = document.querySelector('kernel-panic');
      expect(kernelPanicElement).to.exist;
    });

    it('should set augmentedError on the created element', () => {
      const kernelPanicElement = document.querySelector('kernel-panic') as HTMLElement & { augmentedError: AugmentedError };
      expect(kernelPanicElement.augmentedError).to.exist;
      expect(kernelPanicElement.augmentedError.message).to.equal('Test system error');
    });
  });
});