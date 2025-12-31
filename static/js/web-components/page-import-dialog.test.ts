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
      beforeEach(async () => {
        el.openDialog();
        await el.updateComplete;
      });

      it('should set open to true', () => {
        expect(el.open).to.be.true;
      });

      it('should have open attribute', () => {
        expect(el.hasAttribute('open')).to.be.true;
      });
    });

    describe('when called after previous usage', () => {
      beforeEach(async () => {
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).dialogState = 'preview';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).file = new File(['test'], 'test.csv', { type: 'text/csv' });
        await el.updateComplete;
        el.closeDialog();
        el.openDialog();
        await el.updateComplete;
      });

      it('should reset state to upload', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).dialogState).to.equal('upload');
      });

      it('should clear any previous file', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).file).to.be.null;
      });

      it('should clear any previous error', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).error).to.be.null;
      });
    });
  });

  describe('closeDialog', () => {
    describe('when called', () => {
      beforeEach(async () => {
        el.openDialog();
        await el.updateComplete;
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
      beforeEach(async () => {
        el.openDialog();
        await el.updateComplete;
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      });

      it('should close the dialog', () => {
        expect(el.open).to.be.false;
      });
    });

    describe('when escape key is pressed while closed', () => {
      beforeEach(() => {
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      });

      it('should not close the dialog', () => {
        expect(el.open).to.be.false;
      });
    });

    describe('when other key is pressed while open', () => {
      beforeEach(async () => {
        el.openDialog();
        await el.updateComplete;
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
      });

      it('should not close the dialog', () => {
        expect(el.open).to.be.true;
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
        expect(backdrop).to.exist;
        backdrop!.click();
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
        expect(dialog).to.exist;
        dialog!.click();
      });

      it('should not close the dialog', () => {
        expect(closeDialogSpy).to.not.have.been.called;
      });
    });
  });

  describe('file selection', () => {
    describe('when a CSV file is selected', () => {
      let csvFile: File;

      beforeEach(async () => {
        el.openDialog();
        await el.updateComplete;

        csvFile = new File(['identifier,title\ntest,Test Page'], 'import.csv', { type: 'text/csv' });

        const fileInput = el.shadowRoot?.querySelector<HTMLInputElement>('.file-input');
        if (fileInput) {
          const dt = new DataTransfer();
          dt.items.add(csvFile);
          fileInput.files = dt.files;
          fileInput.dispatchEvent(new Event('change'));
        }
        await el.updateComplete;
      });

      it('should store the file', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).file).to.equal(csvFile);
      });

      it('should automatically transition to validating state', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).dialogState).to.equal('validating');
      });
    });

    describe('when a non-CSV file is selected', () => {
      beforeEach(async () => {
        el.openDialog();
        await el.updateComplete;

        const txtFile = new File(['some text'], 'document.txt', { type: 'text/plain' });

        const fileInput = el.shadowRoot?.querySelector<HTMLInputElement>('.file-input');
        if (fileInput) {
          const dt = new DataTransfer();
          dt.items.add(txtFile);
          fileInput.files = dt.files;
          fileInput.dispatchEvent(new Event('change'));
        }
        await el.updateComplete;
      });

      it('should not store the file', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).file).to.be.null;
      });

      it('should show error', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).error).to.exist;
      });

      it('should have correct error message', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).error?.message).to.contain('CSV');
      });

      it('should remain in upload state', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).dialogState).to.equal('upload');
      });
    });

  });

  describe('dialog title', () => {
    describe('when in upload state', () => {
      beforeEach(async () => {
        el.openDialog();
        await el.updateComplete;
      });

      it('should show upload title', () => {
        const title = el.shadowRoot?.querySelector('.dialog-title');
        expect(title?.textContent).to.equal('Import Pages from CSV');
      });
    });

    describe('when in preview state', () => {
      beforeEach(async () => {
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).dialogState = 'preview';
        await el.updateComplete;
      });

      it('should show preview title', () => {
        const title = el.shadowRoot?.querySelector('.dialog-title');
        expect(title?.textContent).to.equal('Preview Import');
      });
    });

    describe('when in complete state', () => {
      beforeEach(async () => {
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).dialogState = 'complete';
        await el.updateComplete;
      });

      it('should show complete title', () => {
        const title = el.shadowRoot?.querySelector('.dialog-title');
        expect(title?.textContent).to.equal('Import Complete');
      });
    });
  });

  describe('when in validating state', () => {
    beforeEach(async () => {
      el.openDialog();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (el as any).dialogState = 'validating';
      await el.updateComplete;
    });

    it('should show loading spinner', () => {
      const spinner = el.shadowRoot?.querySelector('.loading-spinner');
      expect(spinner).to.exist;
    });

    it('should show parsing message', () => {
      const loadingText = el.shadowRoot?.querySelector('.loading-text');
      expect(loadingText?.textContent).to.contain('Parsing CSV');
    });

    it('should disable cancel button', () => {
      const cancelBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.footer .button-secondary');
      expect(cancelBtn?.disabled).to.be.true;
    });
  });

  describe('when in importing state', () => {
    beforeEach(async () => {
      el.openDialog();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (el as any).dialogState = 'importing';
      await el.updateComplete;
    });

    it('should show loading spinner', () => {
      const spinner = el.shadowRoot?.querySelector('.loading-spinner');
      expect(spinner).to.exist;
    });

    it('should show importing message', () => {
      const loadingText = el.shadowRoot?.querySelector('.loading-text');
      expect(loadingText?.textContent).to.contain('Importing pages');
    });

    it('should show Close button that is enabled', () => {
      const closeBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.footer .button-secondary');
      expect(closeBtn?.textContent?.trim()).to.equal('Close');
      expect(closeBtn?.disabled).to.be.false;
    });
  });

  describe('import button in preview state', () => {
    describe('when there are validation errors', () => {
      beforeEach(async () => {
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).dialogState = 'preview';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).stats = { total: 5, errors: 2, updates: 2, creates: 1 };
        await el.updateComplete;
      });

      it('should disable the import button', () => {
        const importBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.footer .button-primary');
        expect(importBtn?.disabled).to.be.true;
      });
    });

    describe('when there are no validation errors', () => {
      beforeEach(async () => {
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).dialogState = 'preview';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).stats = { total: 5, errors: 0, updates: 2, creates: 3 };
        await el.updateComplete;
      });

      it('should enable the import button', () => {
        const importBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.footer .button-primary');
        expect(importBtn?.disabled).to.be.false;
      });
    });
  });

  describe('cancel button in upload state', () => {
    describe('when cancel button is clicked', () => {
      let closeDialogSpy: sinon.SinonSpy;

      beforeEach(async () => {
        closeDialogSpy = sinon.spy(el, 'closeDialog');
        el.openDialog();
        await el.updateComplete;
        // Use .footer selector to avoid matching the "Select CSV File" button
        const cancelBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.footer .button-secondary');
        expect(cancelBtn).to.exist;
        cancelBtn!.click();
      });

      it('should close the dialog', () => {
        expect(closeDialogSpy).to.have.been.calledOnce;
      });
    });
  });

  describe('event listener lifecycle', () => {
    let lifecycleEl: PageImportDialog;

    afterEach(() => {
      if (lifecycleEl && lifecycleEl.parentNode) {
        lifecycleEl.remove();
      }
    });

    describe('when component is connected', () => {
      let addEventListenerSpy: sinon.SinonSpy;

      beforeEach(async () => {
        addEventListenerSpy = sinon.spy(document, 'addEventListener');
        lifecycleEl = await fixture(html`<page-import-dialog></page-import-dialog>`);
        await lifecycleEl.updateComplete;
      });

      it('should add keydown event listener', () => {
        expect(addEventListenerSpy).to.have.been.calledWith('keydown', lifecycleEl._handleKeydown);
      });
    });

    describe('when component is disconnected', () => {
      let removeEventListenerSpy: sinon.SinonSpy;

      beforeEach(async () => {
        removeEventListenerSpy = sinon.spy(document, 'removeEventListener');
        lifecycleEl = await fixture(html`<page-import-dialog></page-import-dialog>`);
        await lifecycleEl.updateComplete;
        lifecycleEl.remove();
      });

      it('should remove keydown event listener', () => {
        expect(removeEventListenerSpy).to.have.been.calledWith('keydown', lifecycleEl._handleKeydown);
      });
    });
  });
});
