import { html, fixture, expect } from '@open-wc/testing';
import sinon from 'sinon';
import { PageImportDialog } from './page-import-dialog.js';
import './page-import-dialog.js';

function timeout(ms: number, message: string): Promise<never> {
  return new Promise((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

describe('PageImportDialog', () => {
  let el: PageImportDialog;

  beforeEach(async () => {
    // Stub fetch to prevent any network calls
    sinon.stub(window, 'fetch').resolves(new Response('{}'));

    el = await Promise.race([
      fixture<PageImportDialog>(html`<page-import-dialog></page-import-dialog>`),
      timeout(5000, 'Component fixture timed out'),
    ]);
    await el.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
    if (el) {
      el.remove();
    }
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  it('should be an instance of PageImportDialog', () => {
    expect(el).to.be.instanceOf(PageImportDialog);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('page-import-dialog');
  });

  describe('when component is initialized', () => {
    it('should not be open by default', () => {
      expect(el.open).to.be.false;
    });

    it('should not have open attribute by default', () => {
      expect(el.hasAttribute('open')).to.be.false;
    });
  });

  describe('openDialog', () => {
    describe('when called', () => {
      beforeEach(() => {
        el.openDialog();
      });

      it('should set open to true', () => {
        expect(el.open).to.be.true;
      });
    });

    describe('when called after previous usage', () => {
      beforeEach(async () => {
        // Simulate previous usage with state
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access -- accessing private state for testing
        (el as any).dialogState = 'preview';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access -- accessing private state for testing
        (el as any).file = new File(['test'], 'test.csv', { type: 'text/csv' });
        await el.updateComplete;

        el.closeDialog();
        el.openDialog();
        await el.updateComplete;
      });

      it('should reset state to upload', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access -- accessing private state for testing
        expect((el as any).dialogState).to.equal('upload');
      });

      it('should clear any previous file', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access -- accessing private state for testing
        expect((el as any).file).to.be.null;
      });

      it('should clear any previous error', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access -- accessing private state for testing
        expect((el as any).error).to.be.null;
      });
    });
  });

  describe('closeDialog', () => {
    describe('when called', () => {
      beforeEach(() => {
        el.openDialog();
        el.closeDialog();
      });

      it('should set open to false', () => {
        expect(el.open).to.be.false;
      });

      it('should remove open attribute', () => {
        expect(el.hasAttribute('open')).to.be.false;
      });
    });
  });

  describe('keyboard handling', () => {
    describe('when escape key is pressed while open', () => {
      let closeDialogSpy: sinon.SinonSpy;

      beforeEach(() => {
        closeDialogSpy = sinon.spy(el, 'closeDialog');
        el.openDialog();
        el._handleKeydown(new KeyboardEvent('keydown', { key: 'Escape' }));
      });

      it('should close the dialog', () => {
        expect(closeDialogSpy).to.have.been.calledOnce;
      });
    });

    describe('when escape key is pressed while closed', () => {
      let closeDialogSpy: sinon.SinonSpy;

      beforeEach(() => {
        closeDialogSpy = sinon.spy(el, 'closeDialog');
        el._handleKeydown(new KeyboardEvent('keydown', { key: 'Escape' }));
      });

      it('should not close the dialog', () => {
        expect(closeDialogSpy).to.not.have.been.called;
      });
    });

    describe('when other key is pressed while open', () => {
      let closeDialogSpy: sinon.SinonSpy;

      beforeEach(() => {
        closeDialogSpy = sinon.spy(el, 'closeDialog');
        el.openDialog();
        el._handleKeydown(new KeyboardEvent('keydown', { key: 'Enter' }));
      });

      it('should not close the dialog', () => {
        expect(closeDialogSpy).to.not.have.been.called;
      });
    });
  });

  describe('event listener lifecycle', () => {
    describe('when component is connected', () => {
      let addEventListenerSpy: sinon.SinonSpy;

      beforeEach(async () => {
        addEventListenerSpy = sinon.spy(document, 'addEventListener');
        el = await fixture(html`<page-import-dialog></page-import-dialog>`);
        await el.updateComplete;
      });

      it('should add keydown event listener', () => {
        expect(addEventListenerSpy).to.have.been.calledWith('keydown', el._handleKeydown);
      });
    });

    describe('when component is disconnected', () => {
      let removeEventListenerSpy: sinon.SinonSpy;

      beforeEach(async () => {
        removeEventListenerSpy = sinon.spy(document, 'removeEventListener');
        el = await fixture(html`<page-import-dialog></page-import-dialog>`);
        await el.updateComplete;
        el.remove();
      });

      it('should remove keydown event listener', () => {
        expect(removeEventListenerSpy).to.have.been.calledWith('keydown', el._handleKeydown);
      });
    });
  });

  describe('backdrop click handling', () => {
    describe('when backdrop is clicked', () => {
      let closeDialogSpy: sinon.SinonSpy;

      beforeEach(async () => {
        closeDialogSpy = sinon.spy(el, 'closeDialog');
        el.openDialog();
        await el.updateComplete;
        const backdrop = el.shadowRoot?.querySelector<HTMLElement>('.backdrop');
        backdrop?.click();
      });

      it('should close the dialog', () => {
        expect(closeDialogSpy).to.have.been.calledOnce;
      });
    });
  });

  describe('dialog click handling', () => {
    describe('when dialog content is clicked', () => {
      let closeDialogSpy: sinon.SinonSpy;

      beforeEach(async () => {
        closeDialogSpy = sinon.spy(el, 'closeDialog');
        el.openDialog();
        await el.updateComplete;
        const dialog = el.shadowRoot?.querySelector<HTMLElement>('.dialog');
        dialog?.click();
      });

      it('should not close the dialog', () => {
        expect(closeDialogSpy).to.not.have.been.called;
      });
    });
  });
});
