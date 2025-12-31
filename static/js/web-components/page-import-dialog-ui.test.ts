import { html, fixture, expect } from '@open-wc/testing';
import sinon from 'sinon';
import { PageImportDialog } from './page-import-dialog.js';
import './page-import-dialog.js';

function timeout(ms: number, message: string): Promise<never> {
  return new Promise((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

// File selection and UI state tests for PageImportDialog
// Split from main test file to avoid test framework resource exhaustion
describe('PageImportDialog UI', () => {
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
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access -- accessing private state for testing
        expect((el as any).file).to.equal(csvFile);
      });

      it('should display file name', () => {
        const fileName = el.shadowRoot?.querySelector('.file-info-name');
        expect(fileName?.textContent).to.equal('import.csv');
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
  });

  describe('validating state', () => {
    beforeEach(async () => {
      el.openDialog();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access -- accessing private state for testing
      (el as any).dialogState = 'validating';
      await el.updateComplete;
    });

    it('should show loading spinner', () => {
      const spinner = el.shadowRoot?.querySelector('.loading-spinner');
      expect(spinner).to.exist;
    });
  });
});
