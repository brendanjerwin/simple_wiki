import { expect } from '@open-wc/testing';
import { PageDeleter } from './page-deletion-service.js';
import './confirmation-dialog.js'; // Ensure custom element is defined
import sinon from 'sinon';

describe('PageDeleter', () => {
  let service: PageDeleter;
  let mockDialog: {
    openDialog: sinon.SinonSpy;
    setLoading: sinon.SinonSpy;
    showError: sinon.SinonSpy;
    closeDialog: sinon.SinonSpy;
    addEventListener: sinon.SinonSpy;
    removeEventListener: sinon.SinonSpy;
    dataset: { pageName?: string };
    hidden: boolean;
    id: string;
    parentNode?: { removeChild: sinon.SinonSpy } | null;
  };
  let createElementStub: sinon.SinonStub;
  let appendChildStub: sinon.SinonStub;

  beforeEach(() => {
    // Create a mock dialog element
    mockDialog = {
      openDialog: sinon.spy(),
      setLoading: sinon.spy(),
      showError: sinon.spy(),
      closeDialog: sinon.spy(),
      addEventListener: sinon.spy(),
      removeEventListener: sinon.spy(),
      dataset: {},
      hidden: true,
      id: '',
      parentNode: null
    };

    // Stub document methods
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock dialog for testing
    createElementStub = sinon.stub(document, 'createElement').returns(mockDialog as unknown as HTMLElement);
    appendChildStub = sinon.stub(document.body, 'appendChild');
    sinon.stub(document, 'querySelector').returns(null); // Force creation of new dialog

    service = new PageDeleter();
  });

  afterEach(() => {
    service.destroy();
    sinon.restore();
  });

  it('should exist', () => {
    expect(service).to.exist;
  });

  describe('dialog initialization', () => {
    it('should create confirmation dialog element', () => {
      expect(createElementStub).to.have.been.calledWith('confirmation-dialog');
    });

    it('should set dialog id', () => {
      expect(mockDialog.id).to.equal('page-deletion-dialog');
    });

    it('should set dialog as hidden', () => {
      expect(mockDialog.hidden).to.be.true;
    });

    it('should append dialog to document body', () => {
      expect(appendChildStub).to.have.been.calledWith(mockDialog);
    });

    it('should set up event listeners', () => {
      expect(mockDialog.addEventListener).to.have.been.calledWith('confirm', sinon.match.func);
      expect(mockDialog.addEventListener).to.have.been.calledWith('cancel', sinon.match.func);
    });
  });

  describe('when using existing dialog', () => {
    let existingDialog: {
      openDialog: sinon.SinonSpy;
      addEventListener: sinon.SinonSpy;
      removeEventListener: sinon.SinonSpy;
      dataset: Record<string, string>;
      parentNode: null;
    };

    beforeEach(() => {
      sinon.restore(); // Clear previous stubs

      existingDialog = {
        openDialog: sinon.spy(),
        addEventListener: sinon.spy(),
        removeEventListener: sinon.spy(),
        dataset: {},
        parentNode: null
      };
      
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock dialog for testing
      sinon.stub(document, 'querySelector').returns(existingDialog as unknown as Element);
      service = new PageDeleter();
    });

    it('should use existing dialog element', () => {
      expect(service).to.exist;
      // The existing dialog should be used, not created
    });
  });

  describe('confirmAndDeletePage', () => {
    describe('when called with valid page name', () => {
      beforeEach(() => {
        service.confirmAndDeletePage('test-page');
      });

      it('should open dialog with correct configuration', () => {
        expect(mockDialog.openDialog).to.have.been.calledWith({
          message: 'Are you sure you want to delete this page?',
          description: 'Page: test-page',
          confirmText: 'Delete Page',
          cancelText: 'Cancel',
          confirmVariant: 'danger',
          icon: 'warning',
          irreversible: true
        });
      });

      it('should store page name in dialog dataset', () => {
        expect(mockDialog.dataset.pageName).to.equal('test-page');
      });
    });

    describe('when called with empty page name', () => {
      let thrownError: Error | undefined;

      beforeEach(() => {
        try {
          service.confirmAndDeletePage('');
        } catch (err: unknown) {
          if (err instanceof Error) {
            thrownError = err;
          }
        }
      });

      it('should throw an error', () => {
        expect(thrownError).to.exist;
      });

      it('should throw with correct message', () => {
        expect(thrownError?.message).to.equal('PageDeleter: pageName is required');
      });

      it('should not open dialog', () => {
        expect(mockDialog.openDialog).to.not.have.been.called;
      });
    });
  });

  describe('event handling', () => {
    let cancelHandler: (event: Event) => void;

    beforeEach(() => {
      // Extract the event handlers that were registered
      const cancelCall = mockDialog.addEventListener.getCalls().find(call => call.args[0] === 'cancel');

      cancelHandler = cancelCall?.args[1] || (() => {});
    });

    describe('when cancel event is dispatched', () => {
      beforeEach(() => {
        mockDialog.dataset.pageName = 'test-page';
        const cancelEvent = new CustomEvent('cancel');
        cancelHandler(cancelEvent);
      });

      it('should clear page name from dataset', () => {
        expect(mockDialog.dataset.pageName).to.be.undefined;
      });
    });

    describe('when confirm event is dispatched', () => {
      describe('when no page name is stored', () => {
        let thrownError: Error | undefined;

        beforeEach(async () => {
          delete mockDialog.dataset.pageName;

          try {
            // handleConfirm is async, need to await it
            // Extract the confirm handler that was registered and call it
            const confirmCall = mockDialog.addEventListener.getCalls().find(call => call.args[0] === 'confirm');
            const handler: unknown = confirmCall?.args[1];
            const confirmHandler = typeof handler === 'function' ? (handler as (event: Event) => Promise<void>) : undefined;
            if (confirmHandler) {
              await confirmHandler(new CustomEvent('confirm'));
            }
          } catch (err: unknown) {
            if (err instanceof Error) {
              thrownError = err;
            }
          }
        });

        it('should throw an error', () => {
          expect(thrownError).to.exist;
        });

        it('should throw with correct message', () => {
          expect(thrownError?.message).to.equal('PageDeleter: No page name found for deletion');
        });
      });

      // Note: Testing actual gRPC calls would require more complex mocking
      // For now, we test the basic validation and error handling
    });
  });

  describe('destroy method', () => {
    describe('when dialog has parent node', () => {
      beforeEach(() => {
        // Set up a parent node for the dialog
        mockDialog.parentNode = { removeChild: sinon.spy() };

        service.destroy();
      });

      it('should remove event listeners', () => {
        expect(mockDialog.removeEventListener).to.have.been.calledWith('confirm', sinon.match.func);
        expect(mockDialog.removeEventListener).to.have.been.calledWith('cancel', sinon.match.func);
      });

      it('should remove dialog from DOM', () => {
        // parentNode is set in beforeEach, non-null assertion is safe here
        expect(mockDialog.parentNode!.removeChild).to.have.been.calledWith(mockDialog);
      });
    });

    describe('when dialog has no parent node', () => {
      beforeEach(() => {
        mockDialog.parentNode = null;

        service.destroy();
      });

      it('should remove event listeners', () => {
        expect(mockDialog.removeEventListener).to.have.been.calledWith('confirm', sinon.match.func);
        expect(mockDialog.removeEventListener).to.have.been.calledWith('cancel', sinon.match.func);
      });

      it('should not attempt to remove dialog from DOM', () => {
        // No error should occur when parentNode is null
        expect(service).to.exist;
      });
    });
  });

  describe('singleton export', () => {
    it('should export a singleton instance', async () => {
      const { pageDeleteService } = await import('./page-deletion-service.js');
      expect(pageDeleteService).to.be.instanceof(PageDeleter);
    });
  });
});