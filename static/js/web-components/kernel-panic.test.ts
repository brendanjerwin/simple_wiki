import { html, fixture, expect } from '@open-wc/testing';
import { KernelPanic, showKernelPanic } from './kernel-panic.js';
import type { ProcessedError } from './error-service.js';
import { ErrorService } from './error-service.js';
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

    it('should have empty message by default', () => {
      expect(el.message).to.equal('');
    });

    it('should have no error by default', () => {
      expect(el.error).to.be.null;
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

  describe('when message is provided', () => {
    beforeEach(async () => {
      el = await fixture(html`<kernel-panic message="Something went wrong"></kernel-panic>`);
    });

    it('should display the message in error display component', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.exist;
      expect(errorDisplay?.message).to.equal('Something went wrong');
    });
  });

  describe('when error is provided', () => {
    beforeEach(async () => {
      const testError = new Error('Test error message');
      testError.stack = 'Error: Test error message\n    at test:1:1';
      el = await fixture(html`<kernel-panic .error="${testError}"></kernel-panic>`);
    });

    it('should display error in error display component', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.exist;
    });

    it('should include error details in the error display component', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.exist;
      // The error details should be set as a property
      expect(errorDisplay?.details).to.include('Test error message');
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

  describe('when processedError is provided', () => {
    beforeEach(async () => {
      const processedError: ProcessedError = {
        message: 'System failure detected',
        details: 'Stack trace: Error at line 42\nCaused by memory leak',
        icon: 'error'
      };
      el = await fixture(html`<kernel-panic .processedError="${processedError}"></kernel-panic>`);
    });

    it('should have processedError property', () => {
      expect(el.processedError).to.exist;
      expect(el.processedError.message).to.equal('System failure detected');
    });

    it('should display the error using error-display component', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.exist;
      expect(errorDisplay?.message).to.equal('System failure detected');
      expect(errorDisplay?.details).to.equal('Stack trace: Error at line 42\nCaused by memory leak');
      expect(errorDisplay?.icon).to.equal('error');
    });
  });

  describe('when both legacy properties and processedError are provided', () => {
    beforeEach(async () => {
      const processedError: ProcessedError = {
        message: 'Processed error message',
        details: 'Processed error details',
        icon: 'warning'
      };
      el = await fixture(html`<kernel-panic 
        message="Legacy message"
        .error="${new Error('Legacy error')}"
        .processedError="${processedError}"></kernel-panic>`);
    });

    it('should prefer processedError over legacy properties', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.exist;
      expect(errorDisplay?.message).to.equal('Processed error message');
      expect(errorDisplay?.details).to.equal('Processed error details');
      expect(errorDisplay?.icon).to.equal('warning');
    });
  });
});

describe('showKernelPanic function', () => {
  let processErrorSpy: sinon.SinonSpy;

  beforeEach(() => {
    // Clean up any existing kernel-panic elements
    document.querySelectorAll('kernel-panic').forEach(el => el.remove());
    
    // Spy on ErrorService.processError
    processErrorSpy = sinon.spy(ErrorService, 'processError');
  });

  afterEach(() => {
    sinon.restore();
    // Clean up created elements
    document.querySelectorAll('kernel-panic').forEach(el => el.remove());
  });

  describe('when called with error', () => {
    beforeEach(() => {
      const testError = new Error('Test system error');
      showKernelPanic('Critical system failure', testError);
    });

    it('should call ErrorService.processError with the error', () => {
      expect(processErrorSpy).to.have.been.calledOnce;
      expect(processErrorSpy).to.have.been.calledWith(
        sinon.match.instanceOf(Error).and(sinon.match.has('message', 'Test system error')),
        'system operation'
      );
    });

    it('should create a kernel-panic element in document body', () => {
      const kernelPanicElement = document.querySelector('kernel-panic');
      expect(kernelPanicElement).to.exist;
    });

    it('should set processedError on the created element', () => {
      const kernelPanicElement = document.querySelector('kernel-panic') as HTMLElement & { processedError: ProcessedError };
      expect(kernelPanicElement.processedError).to.exist;
      expect(kernelPanicElement.processedError.message).to.equal('Critical system failure');
    });
  });
});