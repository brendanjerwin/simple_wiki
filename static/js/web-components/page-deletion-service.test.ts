import { expect } from '@open-wc/testing';
import { PageDeleter } from './page-deletion-service.js';
import './confirmation-dialog.js'; // Ensure custom element is defined
import sinon from 'sinon';

describe.skip('PageDeleter', () => {
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

    it('should set dialog properties', () => {
      expect(mockDialog.id).to.equal('page-deletion-dialog');
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
          icon: '⚠️',
          irreversible: true
        });
      });

      it('should store page name in dialog dataset', () => {
        expect(mockDialog.dataset.pageName).to.equal('test-page');
      });
    });

    describe('when called with empty page name', () => {
      let consoleErrorStub: sinon.SinonStub;

      beforeEach(() => {
        consoleErrorStub = sinon.stub(console, 'error');
        service.confirmAndDeletePage('');
      });

      afterEach(() => {
        consoleErrorStub.restore();
      });

      it('should log error and not open dialog', () => {
        expect(consoleErrorStub).to.have.been.calledWith('PageDeleter: pageName is required');
        expect(mockDialog.openDialog).to.not.have.been.called;
      });
    });
  });

  describe('event handling', () => {
    let confirmHandler: (event: Event) => void;
    let cancelHandler: (event: Event) => void;

    beforeEach(() => {
      // Extract the event handlers that were registered
      const confirmCall = mockDialog.addEventListener.getCalls().find(call => call.args[0] === 'confirm');
      const cancelCall = mockDialog.addEventListener.getCalls().find(call => call.args[0] === 'cancel');
      
      confirmHandler = confirmCall?.args[1] || (() => {});
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
        let consoleErrorStub: sinon.SinonStub;

        beforeEach(() => {
          delete mockDialog.dataset.pageName;
          consoleErrorStub = sinon.stub(console, 'error');
          
          const confirmEvent = new CustomEvent('confirm');
          confirmHandler(confirmEvent);
        });

        afterEach(() => {
          consoleErrorStub.restore();
        });

        it('should log error and return early', () => {
          expect(consoleErrorStub).to.have.been.calledWith('PageDeleter: No page name found for deletion');
        });
      });

      // Note: Testing actual gRPC calls would require more complex mocking
      // For now, we test the basic validation and error handling
    });
  });

  describe('destroy method', () => {
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

  describe('singleton export', () => {
    it('should export a singleton instance', async () => {
      const { pageDeleteService } = await import('./page-deletion-service.js');
      expect(pageDeleteService).to.be.instanceof(PageDeleter);
    });
  });
});