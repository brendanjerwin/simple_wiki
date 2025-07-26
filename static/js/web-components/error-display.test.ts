import { html, fixture, expect, assert } from '@open-wc/testing';
import { ErrorDisplay } from './error-display.js';
import { AugmentedError, ErrorKind } from './augment-error-service.js';

describe('ErrorDisplay', () => {
  let el: ErrorDisplay;

  beforeEach(async () => {
    el = await fixture(html`<error-display></error-display>`);
  });

  it('should exist', () => {
    assert.instanceOf(el, ErrorDisplay);
  });

  it('should be an instance of ErrorDisplay', () => {
    expect(el).to.be.instanceOf(ErrorDisplay);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName).to.equal('ERROR-DISPLAY');
  });

  describe('when component is initialized', () => {
    it('should have default properties', () => {
      expect(el.augmentedError).to.be.undefined;
    });

    it('should not render anything when no augmentedError provided', () => {
      const headerElement = el.shadowRoot?.querySelector('.error-header');
      expect(headerElement).to.not.exist;
    });
  });

  describe('when augmentedError is provided', () => {
    let augmentedError: AugmentedError;

    beforeEach(async () => {
      const originalError = new Error('Test error message');
      augmentedError = new AugmentedError(
        originalError,
        ErrorKind.ERROR,
        'error'
      );
      el.augmentedError = augmentedError;
      await el.updateComplete;
    });

    it('should display the message', () => {
      const messageElement = el.shadowRoot?.querySelector('.error-message');
      expect(messageElement?.textContent?.trim()).to.equal('Test error message');
    });

    it('should display the icon', () => {
      const iconElement = el.shadowRoot?.querySelector('.error-icon');
      expect(iconElement?.textContent).to.equal('❌');
    });

    it('should not render details section when no details provided', () => {
      const detailsElement = el.shadowRoot?.querySelector('.error-details');
      expect(detailsElement).to.not.exist;
    });
  });

  describe('when augmentedError with details is provided', () => {
    let augmentedError: AugmentedError;

    beforeEach(async () => {
      const originalError = new Error('Test error message');
      originalError.stack = 'Error: Test error message\n    at Object.<anonymous> (test.js:1:1)';
      augmentedError = new AugmentedError(
        originalError,
        ErrorKind.ERROR,
        'error'
      );
      el.augmentedError = augmentedError;
      await el.updateComplete;
    });

    it('should render expand button', () => {
      const expandButton = el.shadowRoot?.querySelector('.expand-button');
      expect(expandButton).to.exist;
    });

    it('should render details container', () => {
      const detailsElement = el.shadowRoot?.querySelector('.error-details');
      expect(detailsElement).to.exist;
    });

    it('should have details hidden by default', () => {
      const detailsElement = el.shadowRoot?.querySelector('.error-details');
      expect(detailsElement?.getAttribute('aria-hidden')).to.equal('true');
    });

    it('should show "Show details" text initially', () => {
      const expandButton = el.shadowRoot?.querySelector('.expand-button');
      expect(expandButton?.textContent?.trim()).to.include('Show details');
    });

    describe('when expand button is clicked', () => {
      beforeEach(async () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button') as HTMLButtonElement;
        expandButton.click();
        await el.updateComplete;
      });

      it('should expand the details', () => {
        const detailsElement = el.shadowRoot?.querySelector('.error-details');
        expect(detailsElement?.getAttribute('aria-hidden')).to.equal('false');
      });

      it('should change button text to "Hide details"', () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button');
        expect(expandButton?.textContent?.trim()).to.include('Hide details');
      });

      it('should set aria-expanded to true', () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button');
        expect(expandButton?.getAttribute('aria-expanded')).to.equal('true');
      });

      it('should add expanded class to icon', () => {
        const expandIcon = el.shadowRoot?.querySelector('.expand-icon');
        expect(expandIcon?.classList.contains('expanded')).to.be.true;
      });

      it('should display the details content', () => {
        const detailsContent = el.shadowRoot?.querySelector('.error-details-content');
        expect(detailsContent?.textContent).to.equal('Error: Test error message\n    at Object.<anonymous> (test.js:1:1)');
      });

      describe('when expand button is clicked again', () => {
        beforeEach(async () => {
          const expandButton = el.shadowRoot?.querySelector('.expand-button') as HTMLButtonElement;
          expandButton.click();
          await el.updateComplete;
        });

        it('should collapse the details', () => {
          const detailsElement = el.shadowRoot?.querySelector('.error-details');
          expect(detailsElement?.getAttribute('aria-hidden')).to.equal('true');
        });

        it('should change button text back to "Show details"', () => {
          const expandButton = el.shadowRoot?.querySelector('.expand-button');
          expect(expandButton?.textContent?.trim()).to.include('Show details');
        });

        it('should set aria-expanded to false', () => {
          const expandButton = el.shadowRoot?.querySelector('.expand-button');
          expect(expandButton?.getAttribute('aria-expanded')).to.equal('false');
        });

        it('should remove expanded class from icon', () => {
          const expandIcon = el.shadowRoot?.querySelector('.expand-icon');
          expect(expandIcon?.classList.contains('expanded')).to.be.false;
        });
      });
    });

    describe('when Enter key is pressed on expand button', () => {
      beforeEach(async () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button') as HTMLButtonElement;
        expandButton.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
        await el.updateComplete;
      });

      it('should expand the details', () => {
        const detailsElement = el.shadowRoot?.querySelector('.error-details');
        expect(detailsElement?.getAttribute('aria-hidden')).to.equal('false');
      });
    });

    describe('when Space key is pressed on expand button', () => {
      beforeEach(async () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button') as HTMLButtonElement;
        expandButton.dispatchEvent(new KeyboardEvent('keydown', { key: ' ' }));
        await el.updateComplete;
      });

      it('should expand the details', () => {
        const detailsElement = el.shadowRoot?.querySelector('.error-details');
        expect(detailsElement?.getAttribute('aria-hidden')).to.equal('false');
      });
    });

    describe('when other key is pressed on expand button', () => {
      beforeEach(async () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button') as HTMLButtonElement;
        expandButton.dispatchEvent(new KeyboardEvent('keydown', { key: 'a' }));
        await el.updateComplete;
      });

      it('should not expand the details', () => {
        const detailsElement = el.shadowRoot?.querySelector('.error-details');
        expect(detailsElement?.getAttribute('aria-hidden')).to.equal('true');
      });
    });
  });

  describe('when augmentedError has empty details', () => {
    beforeEach(async () => {
      const originalError = new Error('Test error message');
      originalError.stack = '';
      const augmentedError = new AugmentedError(
        originalError,
        ErrorKind.ERROR,
        'error'
      );
      el.augmentedError = augmentedError;
      await el.updateComplete;
    });

    it('should not render expand button', () => {
      const expandButton = el.shadowRoot?.querySelector('.expand-button');
      expect(expandButton).to.not.exist;
    });

    it('should not render details container', () => {
      const detailsElement = el.shadowRoot?.querySelector('.error-details');
      expect(detailsElement).to.not.exist;
    });
  });

  describe('when augmentedError has whitespace-only details', () => {
    beforeEach(async () => {
      const originalError = new Error('Test error message');
      originalError.stack = '   \n\t   ';
      const augmentedError = new AugmentedError(
        originalError,
        ErrorKind.ERROR,
        'error'
      );
      el.augmentedError = augmentedError;
      await el.updateComplete;
    });

    it('should not render expand button', () => {
      const expandButton = el.shadowRoot?.querySelector('.expand-button');
      expect(expandButton).to.not.exist;
    });
  });

  describe('accessibility features', () => {
    beforeEach(async () => {
      const originalError = new Error('Test error message');
      originalError.stack = 'Error: Test error message\n    at Object.<anonymous> (test.js:1:1)';
      const augmentedError = new AugmentedError(
        originalError,
        ErrorKind.ERROR,
        'error'
      );
      el.augmentedError = augmentedError;
      await el.updateComplete;
    });

    it('should have proper ARIA attributes on expand button', () => {
      const expandButton = el.shadowRoot?.querySelector('.expand-button');
      expect(expandButton?.getAttribute('aria-expanded')).to.equal('false');
      expect(expandButton?.getAttribute('aria-controls')).to.equal('error-details');
    });

    it('should have proper ARIA attributes on details container', () => {
      const detailsElement = el.shadowRoot?.querySelector('.error-details');
      expect(detailsElement?.getAttribute('aria-hidden')).to.equal('true');
      expect(detailsElement?.getAttribute('id')).to.equal('error-details');
    });

    it('should have aria-hidden on icon', () => {
      const iconElement = el.shadowRoot?.querySelector('.error-icon');
      expect(iconElement?.getAttribute('aria-hidden')).to.equal('true');
    });

    it('should have aria-hidden on expand icon', () => {
      const expandIcon = el.shadowRoot?.querySelector('.expand-icon');
      expect(expandIcon?.getAttribute('aria-hidden')).to.equal('true');
    });
  });

  describe('custom icon support', () => {
    it('should display custom emoji icon', async () => {
      const originalError = new Error('Custom error');
      const augmentedError = new AugmentedError(
        originalError,
        ErrorKind.ERROR,
        '🎯'
      );
      el.augmentedError = augmentedError;
      await el.updateComplete;

      const iconElement = el.shadowRoot?.querySelector('.error-icon');
      expect(iconElement?.textContent).to.equal('🎯');
    });

    it('should display standard icon', async () => {
      const originalError = new Error('Network error');
      const augmentedError = new AugmentedError(
        originalError,
        ErrorKind.NETWORK,
        'network'
      );
      el.augmentedError = augmentedError;
      await el.updateComplete;

      const iconElement = el.shadowRoot?.querySelector('.error-icon');
      expect(iconElement?.textContent).to.equal('🌐');
    });
  });

  describe('when failedGoalDescription is provided', () => {
    it('should display "Error while {goal}: {message}"', async () => {
      const originalError = new Error('Connection refused');
      const augmentedError = new AugmentedError(
        originalError,
        ErrorKind.NETWORK,
        'network',
        'saving document'
      );
      el.augmentedError = augmentedError;
      await el.updateComplete;

      const messageElement = el.shadowRoot?.querySelector('.error-message');
      expect(messageElement?.textContent?.trim()).to.equal('Error while saving document: Connection refused');
    });

    it('should display just the message when no failedGoalDescription', async () => {
      const originalError = new Error('Connection refused');
      const augmentedError = new AugmentedError(
        originalError,
        ErrorKind.NETWORK,
        'network'
      );
      el.augmentedError = augmentedError;
      await el.updateComplete;

      const messageElement = el.shadowRoot?.querySelector('.error-message');
      expect(messageElement?.textContent?.trim()).to.equal('Connection refused');
    });
  });
});