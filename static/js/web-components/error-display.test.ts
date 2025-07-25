import { html, fixture, expect, assert } from '@open-wc/testing';
import sinon from 'sinon';
import { ErrorDisplay } from './error-display.js';

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
      expect(el.message).to.equal('');
      expect(el.details).to.be.undefined;
      expect(el.icon).to.be.undefined;
    });

    it('should not render details when no details provided', () => {
      const detailsElement = el.shadowRoot?.querySelector('.error-details');
      expect(detailsElement).to.not.exist;
    });

    it('should render with default icon when no icon is provided', () => {
      const iconElement = el.shadowRoot?.querySelector('.error-icon');
      expect(iconElement?.textContent).to.equal('⚠️');
    });
  });

  describe('when message is provided', () => {
    beforeEach(async () => {
      el.message = 'Test error message';
      await el.updateComplete;
    });

    it('should display the message', () => {
      const messageElement = el.shadowRoot?.querySelector('.error-message');
      expect(messageElement?.textContent).to.equal('Test error message');
    });
  });

  describe('when custom icon is provided', () => {
    beforeEach(async () => {
      el.icon = '🚨';
      await el.updateComplete;
    });

    it('should display the custom icon', () => {
      const iconElement = el.shadowRoot?.querySelector('.error-icon');
      expect(iconElement?.textContent).to.equal('🚨');
    });
  });

  describe('when details are provided', () => {
    beforeEach(async () => {
      el.message = 'Error occurred';
      el.details = 'Detailed error information here';
      await el.updateComplete;
    });

    it('should render expand button', () => {
      const expandButton = el.shadowRoot?.querySelector('.expand-button');
      expect(expandButton).to.exist;
    });

    it('should render details container', () => {
      const detailsContainer = el.shadowRoot?.querySelector('.error-details');
      expect(detailsContainer).to.exist;
    });

    it('should have details hidden by default', () => {
      const detailsContainer = el.shadowRoot?.querySelector('.error-details');
      expect(detailsContainer?.getAttribute('aria-hidden')).to.equal('true');
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
        const detailsContainer = el.shadowRoot?.querySelector('.error-details');
        expect(detailsContainer?.getAttribute('aria-hidden')).to.equal('false');
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
        expect(detailsContent?.textContent).to.equal('Detailed error information here');
      });

      describe('when expand button is clicked again', () => {
        beforeEach(async () => {
          const expandButton = el.shadowRoot?.querySelector('.expand-button') as HTMLButtonElement;
          expandButton.click();
          await el.updateComplete;
        });

        it('should collapse the details', () => {
          const detailsContainer = el.shadowRoot?.querySelector('.error-details');
          expect(detailsContainer?.getAttribute('aria-hidden')).to.equal('true');
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
        const event = new KeyboardEvent('keydown', { key: 'Enter' });
        expandButton.dispatchEvent(event);
        await el.updateComplete;
      });

      it('should expand the details', () => {
        const detailsContainer = el.shadowRoot?.querySelector('.error-details');
        expect(detailsContainer?.getAttribute('aria-hidden')).to.equal('false');
      });
    });

    describe('when Space key is pressed on expand button', () => {
      it('should expand the details', async () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button') as HTMLButtonElement;
        const event = new KeyboardEvent('keydown', { key: ' ' });
        const preventDefaultSpy = sinon.spy(event, 'preventDefault');
        expandButton.dispatchEvent(event);
        await el.updateComplete;
        
        expect(preventDefaultSpy).to.have.been.called;
        const detailsContainer = el.shadowRoot?.querySelector('.error-details');
        expect(detailsContainer?.getAttribute('aria-hidden')).to.equal('false');
      });
    });

    describe('when other key is pressed on expand button', () => {
      it('should not expand the details', async () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button') as HTMLButtonElement;
        const event = new KeyboardEvent('keydown', { key: 'Tab' });
        expandButton.dispatchEvent(event);
        await el.updateComplete;

        const detailsContainer = el.shadowRoot?.querySelector('.error-details');
        expect(detailsContainer?.getAttribute('aria-hidden')).to.equal('true');
      });
    });
  });

  describe('when no details are provided', () => {
    beforeEach(async () => {
      el.message = 'Simple error message';
      await el.updateComplete;
    });

    it('should not render expand button', () => {
      const expandButton = el.shadowRoot?.querySelector('.expand-button');
      expect(expandButton).to.not.exist;
    });

    it('should not render details container', () => {
      const detailsContainer = el.shadowRoot?.querySelector('.error-details');
      expect(detailsContainer).to.not.exist;
    });
  });

  describe('when details are empty string', () => {
    beforeEach(async () => {
      el.message = 'Error message';
      el.details = '';
      await el.updateComplete;
    });

    it('should not render expand button', () => {
      const expandButton = el.shadowRoot?.querySelector('.expand-button');
      expect(expandButton).to.not.exist;
    });
  });

  describe('when details are whitespace only', () => {
    beforeEach(async () => {
      el.message = 'Error message';
      el.details = '   \n\t  ';
      await el.updateComplete;
    });

    it('should not render expand button', () => {
      const expandButton = el.shadowRoot?.querySelector('.expand-button');
      expect(expandButton).to.not.exist;
    });
  });

  describe('accessibility features', () => {
    beforeEach(async () => {
      el.message = 'Error occurred';
      el.details = 'Detailed error information';
      await el.updateComplete;
    });

    it('should have proper ARIA attributes on expand button', () => {
      const expandButton = el.shadowRoot?.querySelector('.expand-button');
      expect(expandButton?.getAttribute('aria-expanded')).to.equal('false');
      expect(expandButton?.getAttribute('aria-controls')).to.equal('error-details');
    });

    it('should have proper ARIA attributes on details container', () => {
      const detailsContainer = el.shadowRoot?.querySelector('.error-details');
      expect(detailsContainer?.getAttribute('aria-hidden')).to.equal('true');
      expect(detailsContainer?.getAttribute('id')).to.equal('error-details');
    });

    it('should have aria-hidden on icon', () => {
      const icon = el.shadowRoot?.querySelector('.error-icon');
      expect(icon?.getAttribute('aria-hidden')).to.equal('true');
    });

    it('should have aria-hidden on expand icon', () => {
      const expandIcon = el.shadowRoot?.querySelector('.expand-icon');
      expect(expandIcon?.getAttribute('aria-hidden')).to.equal('true');
    });
  });
});