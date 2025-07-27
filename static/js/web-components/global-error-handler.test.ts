import { expect } from '@open-wc/testing';
import { stub, restore, match } from 'sinon';
import { setupGlobalErrorHandler, teardownGlobalErrorHandler } from './global-error-handler.js';

describe('Global Error Handler', () => {
  let addEventListenerStub: sinon.SinonStub;
  let removeEventListenerStub: sinon.SinonStub;

  beforeEach(() => {
    // Stub window event listeners
    addEventListenerStub = stub(window, 'addEventListener');
    removeEventListenerStub = stub(window, 'removeEventListener');
  });

  afterEach(() => {
    restore();
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

      beforeEach(() => {
        mockError = new Error('Test error message');
        mockErrorEvent = {
          error: mockError,
          message: 'Test error message',
          filename: 'test.js',
          lineno: 42,
          colno: 10
        } as ErrorEvent;

        errorHandler(mockErrorEvent);
      });

      it('should not throw', () => {
        // If we get here, errorHandler didn't throw
        expect(true).to.be.true;
      });
    });

    describe('when handling errors without error object', () => {
      let mockErrorEvent: ErrorEvent;

      beforeEach(() => {
        mockErrorEvent = {
          error: null,
          message: 'Script error',
          filename: 'unknown',
          lineno: 0,
          colno: 0
        } as ErrorEvent;

        errorHandler(mockErrorEvent);
      });

      it('should not throw', () => {
        // If we get here, errorHandler didn't throw
        expect(true).to.be.true;
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

    beforeEach(() => {
      mockError = new Error('Promise rejection error');
      preventDefaultStub = stub();
      mockRejectionEvent = {
        reason: mockError,
        preventDefault: preventDefaultStub
      } as unknown as PromiseRejectionEvent;

      rejectionHandler(mockRejectionEvent);
    });

    it('should not throw', () => {
      // If we get here, rejectionHandler didn't throw
      expect(true).to.be.true;
    });

    it('should prevent default', () => {
      expect(preventDefaultStub).to.have.been.calledOnce;
    });
  });

    it('should handle non-Error rejection reasons', () => {
      const preventDefaultStub = stub();
      const mockRejectionEvent = {
        reason: 'String rejection reason',
        preventDefault: preventDefaultStub
      } as unknown as PromiseRejectionEvent;

      expect(() => rejectionHandler(mockRejectionEvent)).to.not.throw();
      expect(preventDefaultStub).to.have.been.calledOnce;
    });
  });
});