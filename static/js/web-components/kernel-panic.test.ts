import { html, fixture, expect } from '@open-wc/testing';
import { KernelPanic } from './kernel-panic.js';

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

    it('should display the message', () => {
      const messageEl = el.shadowRoot?.querySelector('.message');
      expect(messageEl).to.exist;
      expect(messageEl?.textContent).to.equal('Something went wrong');
    });
  });

  describe('when error is provided', () => {
    beforeEach(async () => {
      const testError = new Error('Test error message');
      testError.stack = 'Error: Test error message\n    at test:1:1';
      el = await fixture(html`<kernel-panic .error="${testError}"></kernel-panic>`);
    });

    it('should display error details section', () => {
      const errorDetails = el.shadowRoot?.querySelector('.error-details');
      expect(errorDetails).to.exist;
    });

    it('should display error stack trace', () => {
      const errorStack = el.shadowRoot?.querySelector('.error-stack');
      expect(errorStack).to.exist;
      expect(errorStack?.textContent).to.include('Test error message');
    });
  });
});