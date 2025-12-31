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

      it('should remain closed', () => {
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
        if (!fileInput) {
          throw new Error('File input not found - test setup failed');
        }
        const dt = new DataTransfer();
        dt.items.add(csvFile);
        fileInput.files = dt.files;
        fileInput.dispatchEvent(new Event('change'));
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
        if (!fileInput) {
          throw new Error('File input not found - test setup failed');
        }
        const dt = new DataTransfer();
        dt.items.add(txtFile);
        fileInput.files = dt.files;
        fileInput.dispatchEvent(new Event('change'));
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

    describe('when in importing state', () => {
      beforeEach(async () => {
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).dialogState = 'importing';
        await el.updateComplete;
      });

      it('should show importing title', () => {
        const title = el.shadowRoot?.querySelector('.dialog-title');
        expect(title?.textContent).to.equal('Importing Pages');
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
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (el as any).importedCount = 5;
      await el.updateComplete;
    });

    it('should show importing container', () => {
      const container = el.shadowRoot?.querySelector('.importing-container');
      expect(container).to.exist;
    });

    it('should show explainer text', () => {
      const explainer = el.shadowRoot?.querySelector('.importing-explainer');
      expect(explainer).to.exist;
      expect(explainer?.textContent).to.contain('import');
    });

    it('should show link to report page', () => {
      const reportLink = el.shadowRoot?.querySelector<HTMLAnchorElement>('.report-link');
      expect(reportLink).to.exist;
      expect(reportLink?.href).to.contain('page_import_report');
    });

    it('should show job status section', () => {
      const statusSection = el.shadowRoot?.querySelector('.job-status-section');
      expect(statusSection).to.exist;
    });

    it('should show Close button that is enabled', () => {
      const closeBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.footer .button-secondary');
      expect(closeBtn?.textContent?.trim()).to.equal('Close');
      expect(closeBtn?.disabled).to.be.false;
    });

    describe('when job queue status is available', () => {
      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).jobQueueStatus = {
          jobsRemaining: 3,
          highWaterMark: 6,
          isActive: true,
        };
        await el.updateComplete;
      });

      it('should show jobs remaining', () => {
        const content = el.shadowRoot?.querySelector('.job-status-section')?.textContent;
        expect(content).to.contain('3');
      });

      it('should show total jobs', () => {
        const content = el.shadowRoot?.querySelector('.job-status-section')?.textContent;
        expect(content).to.contain('6');
      });

      it('should show active status', () => {
        const statusValue = el.shadowRoot?.querySelector('.job-status-value.active');
        expect(statusValue?.textContent?.trim()).to.equal('Active');
      });
    });

    describe('when job queue status is not yet available', () => {
      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).jobQueueStatus = null;
        await el.updateComplete;
      });

      it('should show waiting message', () => {
        const waiting = el.shadowRoot?.querySelector('.job-status-waiting');
        expect(waiting).to.exist;
      });
    });

    describe('when streaming is disconnected', () => {
      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).streamingDisconnected = true;
        await el.updateComplete;
      });

      it('should show disconnected message', () => {
        const disconnected = el.shadowRoot?.querySelector('.job-status-disconnected');
        expect(disconnected).to.exist;
      });

      it('should indicate import continues in background', () => {
        const disconnected = el.shadowRoot?.querySelector('.job-status-disconnected');
        expect(disconnected?.textContent).to.contain('continues in background');
      });
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

  describe('drag and drop handling', () => {
    describe('when dragover event fires', () => {
      beforeEach(async () => {
        el.openDialog();
        await el.updateComplete;
        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        const event = new DragEvent('dragover', { bubbles: true, cancelable: true });
        dropZone?.dispatchEvent(event);
        await el.updateComplete;
      });

      it('should set dragOver to true', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).dragOver).to.be.true;
      });

      it('should add drag-over class to drop zone', () => {
        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        expect(dropZone?.classList.contains('drag-over')).to.be.true;
      });
    });

    describe('when dragleave event fires', () => {
      beforeEach(async () => {
        el.openDialog();
        await el.updateComplete;
        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        // First trigger dragover
        dropZone?.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true }));
        await el.updateComplete;
        // Then trigger dragleave
        dropZone?.dispatchEvent(new DragEvent('dragleave', { bubbles: true, cancelable: true }));
        await el.updateComplete;
      });

      it('should set dragOver to false', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).dragOver).to.be.false;
      });
    });

    describe('when a CSV file is dropped', () => {
      beforeEach(async () => {
        el.openDialog();
        await el.updateComplete;
        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        const csvFile = new File(['identifier,title\ntest,Test'], 'import.csv', { type: 'text/csv' });
        const dt = new DataTransfer();
        dt.items.add(csvFile);
        const dropEvent = new DragEvent('drop', {
          bubbles: true,
          cancelable: true,
          dataTransfer: dt,
        });
        dropZone?.dispatchEvent(dropEvent);
        await el.updateComplete;
      });

      it('should process the file', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).file).to.exist;
      });

      it('should transition to validating state', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).dialogState).to.equal('validating');
      });
    });

    describe('when a non-CSV file is dropped', () => {
      beforeEach(async () => {
        el.openDialog();
        await el.updateComplete;
        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        const txtFile = new File(['some text'], 'document.txt', { type: 'text/plain' });
        const dt = new DataTransfer();
        dt.items.add(txtFile);
        const dropEvent = new DragEvent('drop', {
          bubbles: true,
          cancelable: true,
          dataTransfer: dt,
        });
        dropZone?.dispatchEvent(dropEvent);
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
    });
  });

  describe('record navigation', () => {
    describe('when navigating through records', () => {
      beforeEach(async () => {
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).dialogState = 'preview';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).records = [
          { identifier: 'page1', pageExists: false, validationErrors: [], warnings: [], arrayOps: [], fieldsToDelete: [] },
          { identifier: 'page2', pageExists: true, validationErrors: [], warnings: [], arrayOps: [], fieldsToDelete: [] },
          { identifier: 'page3', pageExists: false, validationErrors: ['error'], warnings: [], arrayOps: [], fieldsToDelete: [] },
        ];
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).stats = { total: 3, errors: 1, updates: 1, creates: 1 };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).showErrorsOnly = false;
        await el.updateComplete;
      });

      it('should start at first record', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).currentRecordIndex).to.equal(0);
      });

      describe('when clicking next', () => {
        beforeEach(async () => {
          const nextBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.navigation button:last-child');
          nextBtn?.click();
          await el.updateComplete;
        });

        it('should move to next record', () => {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          expect((el as any).currentRecordIndex).to.equal(1);
        });
      });

      describe('when clicking prev after navigating forward', () => {
        beforeEach(async () => {
          // Move forward first
          const nextBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.navigation button:last-child');
          nextBtn?.click();
          await el.updateComplete;
          // Then move back
          const prevBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.navigation button:first-child');
          prevBtn?.click();
          await el.updateComplete;
        });

        it('should move back to previous record', () => {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          expect((el as any).currentRecordIndex).to.equal(0);
        });
      });

      describe('when at first record', () => {
        it('should disable prev button', () => {
          const prevBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.navigation button:first-child');
          expect(prevBtn?.disabled).to.be.true;
        });
      });

      describe('when at last record', () => {
        beforeEach(async () => {
          // Navigate to last record
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          (el as any).currentRecordIndex = 2;
          await el.updateComplete;
        });

        it('should disable next button', () => {
          const nextBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.navigation button:last-child');
          expect(nextBtn?.disabled).to.be.true;
        });
      });
    });
  });

  describe('showErrorsOnly filter', () => {
    beforeEach(async () => {
      el.openDialog();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (el as any).dialogState = 'preview';
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (el as any).records = [
        { identifier: 'page1', pageExists: false, validationErrors: [], warnings: [], arrayOps: [], fieldsToDelete: [] },
        { identifier: 'page2', pageExists: true, validationErrors: ['error1'], warnings: [], arrayOps: [], fieldsToDelete: [] },
        { identifier: 'page3', pageExists: false, validationErrors: [], warnings: [], arrayOps: [], fieldsToDelete: [] },
      ];
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (el as any).stats = { total: 3, errors: 1, updates: 1, creates: 1 };
      await el.updateComplete;
    });

    describe('when showErrorsOnly is false', () => {
      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).showErrorsOnly = false;
        await el.updateComplete;
      });

      it('should show all records in navigation', () => {
        const navInfo = el.shadowRoot?.querySelector('.nav-info');
        expect(navInfo?.textContent).to.contain('of 3');
      });
    });

    describe('when showErrorsOnly is true', () => {
      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).showErrorsOnly = true;
        await el.updateComplete;
      });

      it('should only show records with errors in navigation', () => {
        const navInfo = el.shadowRoot?.querySelector('.nav-info');
        expect(navInfo?.textContent).to.contain('of 1');
      });
    });

    describe('when checkbox is toggled', () => {
      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).showErrorsOnly = false;
        await el.updateComplete;
        const checkbox = el.shadowRoot?.querySelector<HTMLInputElement>('.checkbox-label input');
        if (checkbox) {
          checkbox.checked = true;
          checkbox.dispatchEvent(new Event('change'));
        }
        await el.updateComplete;
      });

      it('should update showErrorsOnly', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).showErrorsOnly).to.be.true;
      });

      it('should reset to first record', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        expect((el as any).currentRecordIndex).to.equal(0);
      });
    });
  });

  describe('preview state with records', () => {
    describe('when displaying a new page record', () => {
      beforeEach(async () => {
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).dialogState = 'preview';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).records = [
          {
            identifier: 'new_page',
            pageExists: false,
            template: 'inv_item',
            frontmatter: { title: 'New Page', inventory: { container: 'drawer' } },
            validationErrors: [],
            warnings: ['This is a warning'],
            arrayOps: [{ fieldPath: 'tags', operation: 0, value: 'tag1' }],
            fieldsToDelete: ['old_field'],
          },
        ];
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).stats = { total: 1, errors: 0, updates: 0, creates: 1 };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).showErrorsOnly = false;
        await el.updateComplete;
      });

      it('should show the record identifier', () => {
        const identifier = el.shadowRoot?.querySelector('.record-identifier');
        expect(identifier?.textContent).to.equal('new_page');
      });

      it('should show NEW badge for new pages', () => {
        const badge = el.shadowRoot?.querySelector('.badge');
        expect(badge?.textContent?.trim()).to.equal('NEW');
        expect(badge?.classList.contains('badge-new')).to.be.true;
      });

      it('should show template section', () => {
        const sectionTitle = el.shadowRoot?.querySelector('.section-title');
        expect(sectionTitle?.textContent).to.equal('Template');
      });

      it('should show warnings section', () => {
        const warning = el.shadowRoot?.querySelector('.warning-item');
        expect(warning).to.exist;
        expect(warning?.textContent).to.contain('This is a warning');
      });

      it('should show fields to delete', () => {
        const deleteField = el.shadowRoot?.querySelector('.field-delete');
        expect(deleteField).to.exist;
        expect(deleteField?.textContent).to.contain('DELETE');
      });
    });

    describe('when displaying an update page record', () => {
      beforeEach(async () => {
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).dialogState = 'preview';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).records = [
          {
            identifier: 'existing_page',
            pageExists: true,
            frontmatter: { title: 'Updated' },
            validationErrors: [],
            warnings: [],
            arrayOps: [],
            fieldsToDelete: [],
          },
        ];
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).stats = { total: 1, errors: 0, updates: 1, creates: 0 };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).showErrorsOnly = false;
        await el.updateComplete;
      });

      it('should show UPDATE badge for existing pages', () => {
        const badge = el.shadowRoot?.querySelector('.badge');
        expect(badge?.textContent?.trim()).to.equal('UPDATE');
        expect(badge?.classList.contains('badge-update')).to.be.true;
      });
    });

    describe('when displaying a record with validation errors', () => {
      beforeEach(async () => {
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).dialogState = 'preview';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).records = [
          {
            identifier: 'error_page',
            pageExists: false,
            frontmatter: {},
            validationErrors: ['Error 1', 'Error 2'],
            warnings: [],
            arrayOps: [],
            fieldsToDelete: [],
          },
        ];
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).stats = { total: 1, errors: 1, updates: 0, creates: 0 };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).showErrorsOnly = false;
        await el.updateComplete;
      });

      it('should show validation errors section', () => {
        const errorItems = el.shadowRoot?.querySelectorAll('.validation-error-item');
        expect(errorItems?.length).to.equal(2);
      });
    });

    describe('when displaying parsing errors', () => {
      beforeEach(async () => {
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).dialogState = 'preview';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).records = [];
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).parsingErrors = ['Line 5: Invalid CSV format'];
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).stats = { total: 0, errors: 0, updates: 0, creates: 0 };
        await el.updateComplete;
      });

      it('should show parsing errors section', () => {
        const parsingErrors = el.shadowRoot?.querySelector('.parsing-errors');
        expect(parsingErrors).to.exist;
      });

      it('should show parsing error content', () => {
        const errorItem = el.shadowRoot?.querySelector('.parsing-errors .validation-error-item');
        expect(errorItem?.textContent).to.contain('Line 5');
      });
    });

    describe('when no records to display', () => {
      beforeEach(async () => {
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).dialogState = 'preview';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).records = [];
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).stats = { total: 0, errors: 0, updates: 0, creates: 0 };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).showErrorsOnly = false;
        await el.updateComplete;
      });

      it('should show no records message', () => {
        const loadingText = el.shadowRoot?.querySelector('.loading-text');
        expect(loadingText?.textContent).to.contain('No records');
      });
    });

    describe('when no errors to display with showErrorsOnly', () => {
      beforeEach(async () => {
        el.openDialog();
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).dialogState = 'preview';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).records = [
          { identifier: 'page1', pageExists: false, validationErrors: [], warnings: [], arrayOps: [], fieldsToDelete: [] },
        ];
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).stats = { total: 1, errors: 0, updates: 0, creates: 1 };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (el as any).showErrorsOnly = true;
        await el.updateComplete;
      });

      it('should show no errors message', () => {
        const loadingText = el.shadowRoot?.querySelector('.loading-text');
        expect(loadingText?.textContent).to.contain('No errors');
      });
    });
  });

  describe('summary bar', () => {
    beforeEach(async () => {
      el.openDialog();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (el as any).dialogState = 'preview';
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (el as any).records = [];
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (el as any).stats = { total: 10, errors: 2, updates: 3, creates: 5 };
      await el.updateComplete;
    });

    it('should show total count', () => {
      const summaryBar = el.shadowRoot?.querySelector('.summary-bar');
      expect(summaryBar?.textContent).to.contain('10 total');
    });

    it('should show creates count', () => {
      const creates = el.shadowRoot?.querySelector('.summary-item.creates');
      expect(creates?.textContent).to.contain('5 new');
    });

    it('should show updates count', () => {
      const updates = el.shadowRoot?.querySelector('.summary-item.updates');
      expect(updates?.textContent).to.contain('3 update');
    });

    it('should show errors count', () => {
      const errors = el.shadowRoot?.querySelector('.summary-item.errors');
      expect(errors?.textContent).to.contain('2 err');
    });
  });

  describe('array operations display', () => {
    beforeEach(async () => {
      el.openDialog();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (el as any).dialogState = 'preview';
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (el as any).records = [
        {
          identifier: 'page_with_arrays',
          pageExists: false,
          frontmatter: {},
          validationErrors: [],
          warnings: [],
          arrayOps: [
            { fieldPath: 'tags', operation: 0, value: 'add_tag' },  // ENSURE_EXISTS = 0
            { fieldPath: 'labels', operation: 1, value: 'remove_label' },  // DELETE_VALUE = 1
          ],
          fieldsToDelete: [],
        },
      ];
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (el as any).stats = { total: 1, errors: 0, updates: 0, creates: 1 };
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (el as any).showErrorsOnly = false;
      await el.updateComplete;
    });

    it('should show add operation with +ENSURE prefix', () => {
      const addField = el.shadowRoot?.querySelector('.field-add');
      expect(addField?.textContent).to.contain('+ENSURE');
    });

    it('should show remove operation with -REMOVE prefix', () => {
      const removeField = el.shadowRoot?.querySelector('.field-remove');
      expect(removeField?.textContent).to.contain('-REMOVE');
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
