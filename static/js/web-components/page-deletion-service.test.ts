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
    remove: sinon.SinonSpy;
    dataset: { pageName?: string };
    id: string;
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
      remove: sinon.spy(),
      dataset: {},
      id: ''
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
      remove: sinon.SinonSpy;
      dataset: Record<string, string>;
    };

    beforeEach(() => {
      sinon.restore(); // Clear previous stubs

      existingDialog = {
        openDialog: sinon.spy(),
        addEventListener: sinon.spy(),
        removeEventListener: sinon.spy(),
        remove: sinon.spy(),
        dataset: {}
      };
      
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock dialog for testing
      sinon.stub(document, 'querySelector').returns(existingDialog as unknown as Element);
      service = new PageDeleter();
    });

    it('should register event listeners on the existing dialog', () => {
      expect(existingDialog.addEventListener).to.have.been.calledTwice;
      expect(existingDialog.addEventListener).to.have.been.calledWith('confirm', sinon.match.func);
      expect(existingDialog.addEventListener).to.have.been.calledWith('cancel', sinon.match.func);
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
        beforeEach(async () => {
          delete mockDialog.dataset.pageName;

          // Extract the confirm handler that was registered and call it
          const confirmCall = mockDialog.addEventListener.getCalls().find(call => call.args[0] === 'confirm');
          const handler: unknown = confirmCall?.args[1];
          const confirmHandler = typeof handler === 'function' ? (handler as () => void) : undefined;
          if (confirmHandler) {
            confirmHandler();
            // Allow microtasks to flush so the async handleConfirm completes
            await Promise.resolve();
          }
        });

        it('should show error in dialog', () => {
          expect(mockDialog.showError).to.have.been.called;
        });

        it('should show error with correct message', () => {
          const errorArg: unknown = mockDialog.showError.firstCall?.args[0];
          expect(errorArg).to.be.instanceOf(Error);
          expect((errorArg as Error).message).to.include('No page name found for deletion');
        });
      });

      // Note: Testing actual gRPC calls would require more complex mocking
      // The redirect (location.href) and reload() calls use browser navigation APIs
      // that cannot be mocked without actually navigating the test page.
      // We test the synchronous pre-await logic (setLoading) here.

      describe('when page name is stored and gRPC call is initiated', () => {
        beforeEach(() => {
          mockDialog.dataset.pageName = 'test-page';

          // Stub the private gRPC client's deletePage to return a never-resolving promise
          // so we can test the synchronous setup (setLoading) without triggering navigation
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
          const client = (service as unknown as { client: { deletePage: (req: unknown) => Promise<unknown> } }).client;
          sinon.stub(client, 'deletePage').returns(new Promise(() => { /* never resolves */ }));

          // Extract the confirm handler and trigger it synchronously
          const confirmCall = mockDialog.addEventListener.getCalls().find(call => call.args[0] === 'confirm');
          const handler: unknown = confirmCall?.args[1];
          const confirmHandler = typeof handler === 'function' ? (handler as () => void) : undefined;
          confirmHandler?.();
        });

        it('should set loading state before making the gRPC call', () => {
          expect(mockDialog.setLoading).to.have.been.calledWith(true);
        });
      });
    });
  });

  describe('destroy method', () => {
    describe('when destroy is called', () => {
      beforeEach(() => {
        service.destroy();
      });

      it('should remove confirm event listener', () => {
        expect(mockDialog.removeEventListener).to.have.been.calledWith('confirm', sinon.match.func);
      });

      it('should remove cancel event listener', () => {
        expect(mockDialog.removeEventListener).to.have.been.calledWith('cancel', sinon.match.func);
      });

      it('should remove dialog from DOM', () => {
        expect(mockDialog.remove).to.have.been.called;
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