import { html, fixture, expect } from '@open-wc/testing';
import './confirmation-dialog.js';
import { ConfirmationDialog } from './confirmation-dialog.js';
import { AugmentErrorService } from './augment-error-service.js';
import sinon from 'sinon';

describe('ConfirmationDialog', () => {
  let el: ConfirmationDialog;

  beforeEach(async () => {
    el = await fixture(html`<confirmation-dialog></confirmation-dialog>`);
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  it('should be hidden by default', () => {
    expect(el.hasAttribute('open')).to.be.false;
  });

  describe('when opening the dialog', () => {
    beforeEach(() => {
      el.openDialog({
        message: 'Test confirmation message',
        confirmText: 'Test Confirm',
        cancelText: 'Test Cancel'
      });
    });

    it('should become visible', () => {
      expect(el.hasAttribute('open')).to.be.true;
    });

    it('should display the configured message', async () => {
      await el.updateComplete;
      const messageEl = el.shadowRoot?.querySelector('.dialog-message');
      expect(messageEl?.textContent?.trim()).to.equal('Test confirmation message');
    });

    it('should display custom button text', async () => {
      await el.updateComplete;
      const confirmBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-danger');
      const cancelBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-cancel');

      expect(confirmBtn?.textContent?.trim()).to.equal('Test Confirm');
      expect(cancelBtn?.textContent?.trim()).to.equal('Test Cancel');
    });

    it('should show default warning icon', async () => {
      await el.updateComplete;
      const iconEl = el.shadowRoot?.querySelector('.dialog-icon');
      expect(iconEl?.textContent?.trim()).to.equal('⚠️');
    });
  });

  describe('when configuring with different variants', () => {
    describe('when using primary variant', () => {
      beforeEach(async () => {
        el.openDialog({
          message: 'Primary action',
          confirmVariant: 'primary'
        });
        await el.updateComplete;
      });

      it('should apply primary button styling', () => {
        const confirmBtn = el.shadowRoot?.querySelector('.button-primary');
        expect(confirmBtn).to.exist;
      });
    });

    describe('when using warning variant', () => {
      beforeEach(async () => {
        el.openDialog({
          message: 'Warning action',
          confirmVariant: 'warning'
        });
        await el.updateComplete;
      });

      it('should apply warning button styling', () => {
        const confirmBtn = el.shadowRoot?.querySelector('.button-warning');
        expect(confirmBtn).to.exist;
      });
    });

    describe('when using danger variant', () => {
      beforeEach(async () => {
        el.openDialog({
          message: 'Danger action',
          confirmVariant: 'danger'
        });
        await el.updateComplete;
      });

      it('should apply danger button styling', () => {
        const confirmBtn = el.shadowRoot?.querySelector('.button-danger');
        expect(confirmBtn).to.exist;
      });
    });
  });

  describe('when configured with description', () => {
    beforeEach(async () => {
      el.openDialog({
        message: 'Test message',
        description: 'Additional details'
      });
      await el.updateComplete;
    });

    it('should display the description', () => {
      const descEl = el.shadowRoot?.querySelector('.dialog-description');
      expect(descEl?.textContent?.trim()).to.equal('Additional details');
    });
  });

  describe('when configured as irreversible', () => {
    beforeEach(async () => {
      el.openDialog({
        message: 'Test message',
        irreversible: true
      });
      await el.updateComplete;
    });

    it('should show irreversible warning', () => {
      const irreversibleEl = el.shadowRoot?.querySelector('.dialog-description.irreversible');
      expect(irreversibleEl?.textContent?.trim()).to.equal('This action cannot be undone.');
    });
  });

  describe('when configured with custom icon', () => {
    beforeEach(async () => {
      el.openDialog({
        message: 'Test message',
        icon: '💾'
      });
      await el.updateComplete;
    });

    it('should display the custom icon', () => {
      const iconEl = el.shadowRoot?.querySelector('.dialog-icon');
      expect(iconEl?.textContent?.trim()).to.equal('💾');
    });
  });

  describe('event handling', () => {
    let confirmSpy: sinon.SinonSpy;
    let cancelSpy: sinon.SinonSpy;

    beforeEach(() => {
      confirmSpy = sinon.spy();
      cancelSpy = sinon.spy();
      el.addEventListener('confirm', confirmSpy);
      el.addEventListener('cancel', cancelSpy);
      
      el.openDialog({
        message: 'Test message'
      });
    });

    describe('when clicking confirm button', () => {
      beforeEach(async () => {
        await el.updateComplete;
        const confirmBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-danger');
        confirmBtn?.click();
      });

      it('should dispatch confirm event', () => {
        expect(confirmSpy).to.have.been.calledOnce;
      });

      it('should include config in event detail', () => {
        expect(confirmSpy.firstCall.args[0].detail.config.message).to.equal('Test message');
      });
    });

    describe('when clicking cancel button', () => {
      beforeEach(async () => {
        await el.updateComplete;
        const cancelBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-cancel');
        cancelBtn?.click();
      });

      it('should dispatch cancel event', () => {
        expect(cancelSpy).to.have.been.calledOnce;
      });

      it('should close the dialog', () => {
        expect(el.hasAttribute('open')).to.be.false;
      });
    });

    describe('when clicking overlay', () => {
      beforeEach(async () => {
        await el.updateComplete;
        const overlay = el.shadowRoot?.querySelector<HTMLElement>('.overlay');
        overlay?.click();
      });

      it('should dispatch cancel event', () => {
        expect(cancelSpy).to.have.been.calledOnce;
      });

      it('should close the dialog', () => {
        expect(el.hasAttribute('open')).to.be.false;
      });
    });
  });

  describe('loading state', () => {
    beforeEach(async () => {
      el.openDialog({
        message: 'Test message'
      });
      await el.updateComplete;
    });

    describe('when setting loading state', () => {
      beforeEach(async () => {
        el.setLoading(true);
        await el.updateComplete;
      });

      it('should disable confirm button', () => {
        const confirmBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-danger');
        expect(confirmBtn?.disabled).to.be.true;
      });

      it('should disable cancel button', () => {
        const cancelBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-cancel');
        expect(cancelBtn?.disabled).to.be.true;
      });

      it('should show loading text', () => {
        const confirmBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-danger');
        expect(confirmBtn?.textContent?.trim()).to.equal('Processing...');
      });
    });
  });

  describe('error handling', () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let testError: any;

    beforeEach(async () => {
      el.openDialog({
        message: 'Test message'
      });
      
      const mockError = new Error('Test error');
      testError = AugmentErrorService.augmentError(mockError, 'test operation');
      el.showError(testError);
      await el.updateComplete;
    });

    it('should display error component', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.exist;
    });

    it('should not be in loading state', () => {
      const confirmBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-danger');
      expect(confirmBtn?.disabled).to.be.false;
    });
  });

  describe('when closing the dialog', () => {
    beforeEach(() => {
      el.openDialog({
        message: 'Test message'
      });
      el.setLoading(true);
      el.showError(AugmentErrorService.augmentError(new Error('test'), 'test'));
      el.closeDialog();
    });

    it('should hide the dialog', () => {
      expect(el.hasAttribute('open')).to.be.false;
    });

    it('should reset loading state', () => {
      // Re-open to check internal state is reset
      el.openDialog({ message: 'New message' });
      expect(el.shadowRoot?.querySelector('.button-danger[disabled]')).to.not.exist;
    });

    it('should clear errors', async () => {
      // Re-open to check internal state is reset
      el.openDialog({ message: 'New message' });
      await el.updateComplete;
      expect(el.shadowRoot?.querySelector('error-display')).to.not.exist;
    });
  });

  describe('when buttons are disabled during loading', () => {
    let confirmSpy: sinon.SinonSpy;
    let cancelSpy: sinon.SinonSpy;

    beforeEach(async () => {
      confirmSpy = sinon.spy();
      cancelSpy = sinon.spy();
      el.addEventListener('confirm', confirmSpy);
      el.addEventListener('cancel', cancelSpy);
      
      el.openDialog({
        message: 'Test message'
      });
      el.setLoading(true);
      await el.updateComplete;
    });

    describe('when trying to click confirm button while loading', () => {
      beforeEach(() => {
        const confirmBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-danger');
        confirmBtn?.click();
      });

      it('should not dispatch confirm event', () => {
        expect(confirmSpy).to.not.have.been.called;
      });
    });

    describe('when trying to click cancel button while loading', () => {
      beforeEach(() => {
        const cancelBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-cancel');
        cancelBtn?.click();
      });

      it('should not dispatch cancel event', () => {
        expect(cancelSpy).to.not.have.been.called;
      });
    });
  });
});