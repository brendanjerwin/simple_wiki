import { html, fixture, expect, assert } from '@open-wc/testing';
import { ErrorDisplay } from './error-display.js';
import { AugmentedError, ErrorKind } from './augment-error-service.js';

describe('ErrorDisplay', () => {
  it('should exist', async () => {
    const el = await fixture(html`<error-display></error-display>`);
    assert.instanceOf(el, ErrorDisplay);
  });

  it('should have the correct tag name', async () => {
    const el = await fixture(html`<error-display></error-display>`);
    expect(el.tagName).to.equal('ERROR-DISPLAY');
  });

  it('should display error message', async () => {
    const el = await fixture(html`<error-display></error-display>`);
    
    const originalError = new Error('Test error message');
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      'error'
    );
    el.augmentedError = augmentedError;
    await el.updateComplete;

    const messageElement = el.shadowRoot?.querySelector('.error-message');
    expect(messageElement?.textContent?.trim()).to.equal('Test error message');
  });

  it('should display icon', async () => {
    const el = await fixture(html`<error-display></error-display>`);
    
    const originalError = new Error('Test error message');
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      'error'
    );
    el.augmentedError = augmentedError;
    await el.updateComplete;

    const iconElement = el.shadowRoot?.querySelector('.error-icon');
    expect(iconElement?.textContent).to.equal('❌');
  });

  it('should show expand button when stack exists', async () => {
    const el = await fixture(html`<error-display></error-display>`);
    
    const originalError = new Error('Test error message');
    originalError.stack = 'Error: Test error message\n    at Object.<anonymous> (test.js:1:1)';
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      'error'
    );
    el.augmentedError = augmentedError;
    await el.updateComplete;

    const expandButton = el.shadowRoot?.querySelector('.expand-button');
    expect(expandButton).to.exist;
  });

  it('should expand when button clicked', async () => {
    const el = await fixture(html`<error-display></error-display>`);
    
    const originalError = new Error('Test error message');
    originalError.stack = 'Error: Test error message\n    at Object.<anonymous> (test.js:1:1)';
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      'error'
    );
    el.augmentedError = augmentedError;
    await el.updateComplete;

    const expandButton = el.shadowRoot?.querySelector('.expand-button') as HTMLButtonElement;
    expandButton.click();
    await el.updateComplete;

    const detailsElement = el.shadowRoot?.querySelector('.error-details');
    expect(detailsElement?.getAttribute('aria-hidden')).to.equal('false');
  });

  it('should display structured message with goal', async () => {
    const el = await fixture(html`<error-display></error-display>`);
    
    const originalError = new Error('Connection refused');
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.NETWORK,
      'network',
      'saving document'
    );
    el.augmentedError = augmentedError;
    await el.updateComplete;

    const goalElement = el.shadowRoot?.querySelector('.error-goal');
    const detailElement = el.shadowRoot?.querySelector('.error-detail');
    
    expect(goalElement?.textContent?.trim()).to.equal('Error while saving document:');
    expect(detailElement?.textContent?.trim()).to.equal('Connection refused');
  });
});