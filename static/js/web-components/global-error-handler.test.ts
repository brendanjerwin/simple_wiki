import { expect } from '@open-wc/testing';
import { stub, restore, match, type SinonStub } from 'sinon';
import { setupGlobalErrorHandler, teardownGlobalErrorHandler } from './global-error-handler.js';
import type { KernelPanic } from './kernel-panic.js';

describe('Global Error Handler', () => {
  let addEventListenerStub: SinonStub;
  let removeEventListenerStub: SinonStub;

  beforeEach(() => {
    // Stub window event listeners
    addEventListenerStub = stub(window, 'addEventListener');
    removeEventListenerStub = stub(window, 'removeEventListener');
  });

  afterEach(() => {
    teardownGlobalErrorHandler();
    restore();
    document.querySelectorAll('kernel-panic').forEach(el => el.remove());
  });

  describe('setupGlobalErrorHandler', () => {
    beforeEach(() => {
      setupGlobalErrorHandler();
    });

    it('should register error event listener', () => {
      expect(addEventListenerStub).to.have.been.calledWith('error', match.func);
    });

    it('should register unhandledrejection event listener', () => {
      expect(addEventListenerStub).to.have.been.calledWith('unhandledrejection', match.func);
    });
  });

  describe('teardownGlobalErrorHandler', () => {
    beforeEach(() => {
      setupGlobalErrorHandler();
      teardownGlobalErrorHandler();
    });

    it('should remove error event listener', () => {
      expect(removeEventListenerStub).to.have.been.calledWith('error', match.func);
    });

    it('should remove unhandledrejection event listener', () => {
      expect(removeEventListenerStub).to.have.been.calledWith('unhandledrejection', match.func);
    });
  });

  describe('when global error occurs', () => {
    let errorHandler: (event: ErrorEvent) => void;

    beforeEach(() => {
      setupGlobalErrorHandler();
      // Get the error handler that was registered
      const errorCall = addEventListenerStub.getCalls().find(call => call.args[0] === 'error');
      errorHandler = errorCall!.args[1];
    });

    it('should register error handler', () => {
      expect(errorHandler).to.be.a('function');
    });

    describe('when handling error events with error object', () => {
      let mockError: Error;
      let mockErrorEvent: ErrorEvent;
      let panicEl: KernelPanic | null;

      beforeEach(() => {
        mockError = new Error('Test error message');
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock event for testing
        mockErrorEvent = {
          error: mockError,
          message: 'Test error message',
          filename: 'test.js',
          lineno: 42,
          colno: 10
        } as ErrorEvent;

        errorHandler(mockErrorEvent);
        panicEl = document.querySelector('kernel-panic');
      });

      it('should show kernel panic', () => {
        expect(panicEl).to.exist;
      });

      it('should display the error message', () => {
        expect(panicEl?.augmentedError?.message).to.equal('Test error message');
      });
    });

    describe('when handling errors without error object', () => {
      let mockErrorEvent: ErrorEvent;
      let panicEl: KernelPanic | null;

      beforeEach(() => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock event for testing
        mockErrorEvent = {
          error: null,
          message: 'Script error',
          filename: 'unknown',
          lineno: 0,
          colno: 0
        } as ErrorEvent;

        errorHandler(mockErrorEvent);
        panicEl = document.querySelector('kernel-panic');
      });

      it('should show kernel panic', () => {
        expect(panicEl).to.exist;
      });

      it('should display the fallback error message', () => {
        expect(panicEl?.augmentedError?.message).to.equal('Script error');
      });
    });
  });

  describe('when unhandled promise rejection occurs', () => {
    let rejectionHandler: (event: PromiseRejectionEvent) => void;

    beforeEach(() => {
      setupGlobalErrorHandler();
      // Get the rejection handler that was registered
      const rejectionCall = addEventListenerStub.getCalls().find(call => call.args[0] === 'unhandledrejection');
      rejectionHandler = rejectionCall!.args[1];
    });

    it('should register rejection handler', () => {
      expect(rejectionHandler).to.be.a('function');
    });

    describe('when handling rejection events', () => {
      let mockError: Error;
      let preventDefaultStub: SinonStub;
      let mockRejectionEvent: PromiseRejectionEvent;
      let panicEl: KernelPanic | null;

      beforeEach(() => {
        mockError = new Error('Promise rejection error');
        preventDefaultStub = stub();
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock event for testing
        mockRejectionEvent = {
          reason: mockError,
          preventDefault: preventDefaultStub
        } as unknown as PromiseRejectionEvent;

        rejectionHandler(mockRejectionEvent);
        panicEl = document.querySelector('kernel-panic');
      });

      it('should show kernel panic', () => {
        expect(panicEl).to.exist;
      });

      it('should display the promise rejection error message', () => {
        expect(panicEl?.augmentedError?.message).to.equal('Promise rejection error');
      });

      it('should prevent default', () => {
        expect(preventDefaultStub).to.have.been.calledOnce;
      });
    });

    describe('when rejection reason is a non-Error value', () => {
      let preventDefaultStub: SinonStub;
      let panicEl: KernelPanic | null;

      beforeEach(() => {
        preventDefaultStub = stub();
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock event for testing
        const mockRejectionEvent = {
          reason: 'String rejection reason',
          preventDefault: preventDefaultStub
        } as unknown as PromiseRejectionEvent;

        rejectionHandler(mockRejectionEvent);
        panicEl = document.querySelector('kernel-panic');
      });

      it('should prevent default', () => {
        expect(preventDefaultStub).to.have.been.calledOnce;
      });

      it('should show kernel panic', () => {
        expect(panicEl).to.exist;
      });
    });
  });
});