import { html, fixture, expect, assert } from '@open-wc/testing';
import { stub, type SinonStub } from 'sinon';
import { ErrorDisplay, type ErrorAction } from './error-display.js';
import { AugmentedError, ErrorKind } from './augment-error-service.js';

function timeout(ms: number, message: string): Promise<never> {
  return new Promise((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

describe('ErrorDisplay', () => {
  let el: ErrorDisplay;
  let fetchStub: ReturnType<typeof stub>;

  beforeEach(async () => {
    // Prevent any potential network calls that could cause hanging
    fetchStub = stub(window, 'fetch');
    fetchStub.resolves(new Response('{}'));
    
    el = await Promise.race([
      fixture<ErrorDisplay>(html`<error-display></error-display>`),
      timeout(5000, "ErrorDisplay fixture timed out"),
    ]);
  });

  afterEach(() => {
    if (fetchStub) {
      fetchStub.restore();
    }
  });

  it('should exist', () => {
    assert.instanceOf(el, ErrorDisplay);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName).to.equal('ERROR-DISPLAY');
  });

  it('should display error message', async () => {
    const originalError = new Error('Test error message');
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      'error'
    );
    el.augmentedError = augmentedError;
    await Promise.race([
      el.updateComplete,
      timeout(5000, "Component update timed out"),
    ]);

    const messageElement = el.shadowRoot?.querySelector('.error-message');
    expect(messageElement?.textContent?.trim()).to.equal('Test error message');
  });

  it('should display icon', async () => {
    const originalError = new Error('Test error message');
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      'error'
    );
    el.augmentedError = augmentedError;
    await Promise.race([
      el.updateComplete,
      timeout(5000, "Component update timed out"),
    ]);

    const iconElement = el.shadowRoot?.querySelector('.error-icon');
    expect(iconElement?.textContent).to.equal('❌');
  });

  it('should show expand button when stack exists', async () => {
    const originalError = new Error('Test error message');
    originalError.stack = 'Error: Test error message\n    at Object.<anonymous> (test.js:1:1)';
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      'error'
    );
    el.augmentedError = augmentedError;
    await Promise.race([
      el.updateComplete,
      timeout(5000, "Component update timed out"),
    ]);

    const expandButton = el.shadowRoot?.querySelector('.expand-button');
    expect(expandButton).to.exist;
  });

  it('should expand when button clicked', async () => {
    const originalError = new Error('Test error message');
    originalError.stack = 'Error: Test error message\n    at Object.<anonymous> (test.js:1:1)';
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      'error'
    );
    el.augmentedError = augmentedError;
    await Promise.race([
      el.updateComplete,
      timeout(5000, "Component update timed out"),
    ]);

    const expandButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.expand-button');
    expandButton?.click();
    await Promise.race([
      el.updateComplete,
      timeout(5000, "Component update after click timed out"),
    ]);

    const detailsElement = el.shadowRoot?.querySelector('.error-details');
    expect(detailsElement?.getAttribute('aria-hidden')).to.equal('false');
  });

  it('should display structured message with goal', async () => {
    const originalError = new Error('Connection refused');
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.NETWORK,
      'network',
      'saving document'
    );
    el.augmentedError = augmentedError;
    await Promise.race([
      el.updateComplete,
      timeout(5000, "Component update timed out"),
    ]);

    const goalElement = el.shadowRoot?.querySelector('.error-goal');
    const detailElement = el.shadowRoot?.querySelector('.error-detail');
    
    expect(goalElement?.textContent?.trim()).to.equal('Error while saving document:');
    expect(detailElement?.textContent?.trim()).to.equal('Connection refused');
  });

  describe('action button', () => {
    describe('when action is not provided', () => {
      beforeEach(async () => {
        const originalError = new Error('Test error');
        el.augmentedError = new AugmentedError(originalError, ErrorKind.ERROR, 'error');
        delete el.action;
        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should not display action button', () => {
        const actionButton = el.shadowRoot?.querySelector('.action-button');
        expect(actionButton).to.not.exist;
      });

      it('should not display actions container', () => {
        const actionsContainer = el.shadowRoot?.querySelector('.error-actions');
        expect(actionsContainer).to.not.exist;
      });
    });

    describe('when action is provided', () => {
      let onClickStub: SinonStub;
      let action: ErrorAction;

      beforeEach(async () => {
        onClickStub = stub();
        action = {
          label: 'Try Again',
          onClick: onClickStub
        };

        const originalError = new Error('Test error');
        el.augmentedError = new AugmentedError(originalError, ErrorKind.ERROR, 'error');
        el.action = action;
        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should display action button', () => {
        const actionButton = el.shadowRoot?.querySelector('.action-button');
        expect(actionButton).to.exist;
      });

      it('should display action label', () => {
        const actionButton = el.shadowRoot?.querySelector('.action-button');
        expect(actionButton?.textContent?.trim()).to.equal('Try Again');
      });

      it('should display actions container', () => {
        const actionsContainer = el.shadowRoot?.querySelector('.error-actions');
        expect(actionsContainer).to.exist;
      });

      describe('when action button is clicked', () => {
        beforeEach(async () => {
          const actionButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.action-button');
          actionButton?.click();
          await Promise.race([
            el.updateComplete,
            timeout(5000, "Component update timed out"),
          ]);
        });

        it('should call onClick callback', () => {
          expect(onClickStub).to.have.been.calledOnce;
        });
      });
    });
  });
});