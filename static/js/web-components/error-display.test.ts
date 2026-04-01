import { html, fixture, expect } from '@open-wc/testing';
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
    expect(el).to.be.instanceOf(ErrorDisplay);
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
    expect(expandButton).to.not.equal(null);
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
        expect(actionButton).to.equal(null);
      });

      it('should not display actions container', () => {
        const actionsContainer = el.shadowRoot?.querySelector('.error-actions');
        expect(actionsContainer).to.equal(null);
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
        expect(actionButton).to.not.equal(null);
      });

      it('should display action label', () => {
        const actionButton = el.shadowRoot?.querySelector('.action-button');
        expect(actionButton?.textContent?.trim()).to.equal('Try Again');
      });

      it('should display actions container', () => {
        const actionsContainer = el.shadowRoot?.querySelector('.error-actions');
        expect(actionsContainer).to.not.equal(null);
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
          expect(onClickStub.callCount).to.equal(1);
        });
      });
    });
  });

  describe('_renderDetails', () => {
    describe('when error has no stack', () => {
      beforeEach(async () => {
        const originalError = new Error('Test error');
        delete originalError.stack;
        el.augmentedError = new AugmentedError(originalError, ErrorKind.ERROR, 'error');

        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should not display expand button', () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button');
        expect(expandButton).to.equal(null);
      });

      it('should not display details section', () => {
        const detailsSection = el.shadowRoot?.querySelector('.error-details');
        expect(detailsSection).to.equal(null);
      });
    });

    describe('when error has stack', () => {
      beforeEach(async () => {
        const originalError = new Error('Test error');
        originalError.stack = 'Error: Test error\n    at Object.<anonymous> (test.js:1:1)';
        el.augmentedError = new AugmentedError(originalError, ErrorKind.ERROR, 'error');

        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should display expand button', () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button');
        expect(expandButton).to.not.equal(null);
      });

      it('should show "Show details" label initially', () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button');
        expect(expandButton?.textContent?.trim()).to.include('Show details');
      });

      it('should have details hidden initially', () => {
        const detailsSection = el.shadowRoot?.querySelector('.error-details');
        expect(detailsSection?.getAttribute('aria-hidden')).to.equal('true');
      });

      it('should set aria-expanded to false on button initially', () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button');
        expect(expandButton?.getAttribute('aria-expanded')).to.equal('false');
      });

      describe('when expand button is clicked', () => {
        beforeEach(async () => {
          const expandButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.expand-button');
          expandButton?.click();

          await Promise.race([
            el.updateComplete,
            timeout(5000, "Component update after click timed out"),
          ]);
        });

        it('should show details', () => {
          const detailsSection = el.shadowRoot?.querySelector('.error-details');
          expect(detailsSection?.getAttribute('aria-hidden')).to.equal('false');
        });

        it('should change label to "Hide details"', () => {
          const expandButton = el.shadowRoot?.querySelector('.expand-button');
          expect(expandButton?.textContent?.trim()).to.include('Hide details');
        });

        it('should set aria-expanded to true on button', () => {
          const expandButton = el.shadowRoot?.querySelector('.expand-button');
          expect(expandButton?.getAttribute('aria-expanded')).to.equal('true');
        });

        it('should apply expanded class to expand icon', () => {
          const expandIcon = el.shadowRoot?.querySelector('.expand-icon');
          expect(expandIcon?.classList.contains('expanded')).to.equal(true);
        });

        describe('when expand button is clicked again', () => {
          beforeEach(async () => {
            const expandButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.expand-button');
            expandButton?.click();

            await Promise.race([
              el.updateComplete,
              timeout(5000, "Component update after second click timed out"),
            ]);
          });

          it('should hide details again', () => {
            const detailsSection = el.shadowRoot?.querySelector('.error-details');
            expect(detailsSection?.getAttribute('aria-hidden')).to.equal('true');
          });

          it('should revert label to "Show details"', () => {
            const expandButton = el.shadowRoot?.querySelector('.expand-button');
            expect(expandButton?.textContent?.trim()).to.include('Show details');
          });
        });
      });

      describe('when Enter key is pressed on expand button', () => {
        beforeEach(async () => {
          const expandButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.expand-button');
          expandButton?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));

          await Promise.race([
            el.updateComplete,
            timeout(5000, "Component update timed out"),
          ]);
        });

        it('should expand details', () => {
          const detailsSection = el.shadowRoot?.querySelector('.error-details');
          expect(detailsSection?.getAttribute('aria-hidden')).to.equal('false');
        });
      });

      describe('when Space key is pressed on expand button', () => {
        beforeEach(async () => {
          const expandButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.expand-button');
          expandButton?.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }));

          await Promise.race([
            el.updateComplete,
            timeout(5000, "Component update timed out"),
          ]);
        });

        it('should expand details', () => {
          const detailsSection = el.shadowRoot?.querySelector('.error-details');
          expect(detailsSection?.getAttribute('aria-hidden')).to.equal('false');
        });
      });
    });
  });
});