import { expect } from '@open-wc/testing';
import { ConnectError, Code } from '@connectrpc/connect';
import { 
  AugmentErrorService, 
  AugmentedError, 
  ErrorKind
} from './augment-error-service.js';

describe('AugmentErrorService', () => {
  describe('augmentError', () => {
    describe('when processing ConnectError instances', () => {
      describe('when the error is UNAVAILABLE', () => {
        let connectError: ConnectError;
        let augmented: AugmentedError;

        beforeEach(() => {
          connectError = new ConnectError('Service unavailable', Code.Unavailable);
          augmented = AugmentErrorService.augmentError(connectError);
        });

        it('should return AugmentedError instance', () => {
          expect(augmented).to.be.instanceof(AugmentedError);
        });

        it('should preserve original message', () => {
          expect(augmented.message).to.equal('[unavailable] Service unavailable');
        });

        it('should set errorKind to NETWORK', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.NETWORK);
        });

        it('should set icon to network', () => {
          expect(augmented.icon).to.equal('network');
        });

        it('should include gRPC error in details', () => {
          expect(augmented.details).to.include('gRPC error:');
        });
      });

      describe('when the error is NOT_FOUND', () => {
        let connectError: ConnectError;
        let augmented: AugmentedError;

        beforeEach(() => {
          connectError = new ConnectError('Resource not found', Code.NotFound);
          augmented = AugmentErrorService.augmentError(connectError);
        });

        it('should set errorKind to NOT_FOUND', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.NOT_FOUND);
        });

        it('should set icon to not-found', () => {
          expect(augmented.icon).to.equal('not-found');
        });
      });

      describe('when the error is PERMISSION_DENIED', () => {
        let connectError: ConnectError;
        let augmented: AugmentedError;

        beforeEach(() => {
          connectError = new ConnectError('Access denied', Code.PermissionDenied);
          augmented = AugmentErrorService.augmentError(connectError);
        });

        it('should set errorKind to PERMISSION', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.PERMISSION);
        });

        it('should set icon to permission', () => {
          expect(augmented.icon).to.equal('permission');
        });
      });

      describe('when the error is UNAUTHENTICATED', () => {
        let connectError: ConnectError;
        let augmented: AugmentedError;

        beforeEach(() => {
          connectError = new ConnectError('Authentication required', Code.Unauthenticated);
          augmented = AugmentErrorService.augmentError(connectError);
        });

        it('should set errorKind to PERMISSION', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.PERMISSION);
        });

        it('should set icon to permission', () => {
          expect(augmented.icon).to.equal('permission');
        });
      });

      describe('when the error is INVALID_ARGUMENT', () => {
        let connectError: ConnectError;
        let augmented: AugmentedError;

        beforeEach(() => {
          connectError = new ConnectError('Invalid input', Code.InvalidArgument);
          augmented = AugmentErrorService.augmentError(connectError);
        });

        it('should set errorKind to VALIDATION', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.VALIDATION);
        });

        it('should set icon to validation', () => {
          expect(augmented.icon).to.equal('validation');
        });
      });

      describe('when the error is DEADLINE_EXCEEDED', () => {
        let connectError: ConnectError;
        let augmented: AugmentedError;

        beforeEach(() => {
          connectError = new ConnectError('Timeout', Code.DeadlineExceeded);
          augmented = AugmentErrorService.augmentError(connectError);
        });

        it('should set errorKind to TIMEOUT', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.TIMEOUT);
        });

        it('should set icon to timeout', () => {
          expect(augmented.icon).to.equal('timeout');
        });
      });

      describe('when the error is INTERNAL', () => {
        let connectError: ConnectError;
        let augmented: AugmentedError;

        beforeEach(() => {
          connectError = new ConnectError('Internal error', Code.Internal);
          augmented = AugmentErrorService.augmentError(connectError);
        });

        it('should set errorKind to SERVER', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.SERVER);
        });

        it('should set icon to server', () => {
          expect(augmented.icon).to.equal('server');
        });
      });

      describe('when the error code is unknown', () => {
        let connectError: ConnectError;
        let augmented: AugmentedError;

        beforeEach(() => {
          connectError = new ConnectError('Unknown error', Code.Unknown);
          augmented = AugmentErrorService.augmentError(connectError);
        });

        it('should set errorKind to ERROR', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.ERROR);
        });

        it('should set icon to error', () => {
          expect(augmented.icon).to.equal('error');
        });
      });
    });

    describe('when processing regular Error instances', () => {
      describe('when Error has message and stack', () => {
        let error: Error;
        let augmented: AugmentedError;

        beforeEach(() => {
          error = new Error('Test error');
          augmented = AugmentErrorService.augmentError(error);
        });

        it('should return AugmentedError instance', () => {
          expect(augmented).to.be.instanceof(AugmentedError);
        });

        it('should preserve original message', () => {
          expect(augmented.message).to.equal('Test error');
        });

        it('should set errorKind to ERROR', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.ERROR);
        });

        it('should set icon to error', () => {
          expect(augmented.icon).to.equal('error');
        });

        it('should include error in details', () => {
          expect(augmented.details).to.include('Error: Test error');
        });
      });

      describe('when Error has empty message', () => {
        let error: Error;
        let augmented: AugmentedError;

        beforeEach(() => {
          error = new Error('');
          augmented = AugmentErrorService.augmentError(error);
        });

        it('should provide fallback message', () => {
          expect(augmented.message).to.equal('An error occurred');
        });

        it('should set errorKind to ERROR', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.ERROR);
        });
      });

      describe('when preserving stack trace', () => {
        let error: Error;
        let augmented: AugmentedError;

        beforeEach(() => {
          error = new Error('Test error');
          augmented = AugmentErrorService.augmentError(error);
        });

        it('should preserve original stack', () => {
          expect(augmented.stack).to.equal(error.stack);
        });
      });
    });

    describe('when processing AugmentedError instances', () => {
      describe('when augmented error is passed', () => {
        let augmented: AugmentedError;
        let result: AugmentedError;

        beforeEach(() => {
          augmented = new AugmentedError('Test', ErrorKind.WARNING, 'warning');
          result = AugmentErrorService.augmentError(augmented);
        });

        it('should return same instance without modification', () => {
          expect(result).to.equal(augmented);
        });
      });
    });

    describe('when processing non-Error objects', () => {
      describe('when error is a string', () => {
        let augmented: AugmentedError;

        beforeEach(() => {
          augmented = AugmentErrorService.augmentError('String error');
        });

        it('should use string as message', () => {
          expect(augmented.message).to.equal('String error');
        });

        it('should use string as details', () => {
          expect(augmented.details).to.equal('String error');
        });

        it('should set errorKind to ERROR', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.ERROR);
        });
      });

      describe('when error is an object', () => {
        let errorObj: { code: number; message: string };
        let augmented: AugmentedError;

        beforeEach(() => {
          errorObj = { code: 500, message: 'Server error' };
          augmented = AugmentErrorService.augmentError(errorObj);
        });

        it('should use fallback message', () => {
          expect(augmented.message).to.equal('An unknown error occurred');
        });

        it('should serialize object as details', () => {
          expect(augmented.details).to.equal(JSON.stringify(errorObj));
        });

        it('should set errorKind to ERROR', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.ERROR);
        });
      });
    });
  });

  describe('getIconString', () => {
    it('should return emoji for standard icons', () => {
      expect(AugmentErrorService.getIconString('warning')).to.equal('⚠️');
      expect(AugmentErrorService.getIconString('error')).to.equal('❌');
      expect(AugmentErrorService.getIconString('network')).to.equal('🌐');
    });

    it('should return custom icons as-is', () => {
      expect(AugmentErrorService.getIconString('🎯')).to.equal('🎯');
      expect(AugmentErrorService.getIconString('custom')).to.equal('custom');
    });
  });
});

describe('AugmentedError', () => {
  it('should extend Error', () => {
    const augmented = new AugmentedError('Test', ErrorKind.ERROR, 'error');
    
    expect(augmented).to.be.instanceof(Error);
    expect(augmented).to.be.instanceof(AugmentedError);
  });

  it('should have correct properties', () => {
    const augmented = new AugmentedError(
      'Test message', 
      ErrorKind.WARNING, 
      'warning',
      'Test details'
    );

    expect(augmented.message).to.equal('Test message');
    expect(augmented.errorKind).to.equal(ErrorKind.WARNING);
    expect(augmented.icon).to.equal('warning');
    expect(augmented.details).to.equal('Test details');
    expect(augmented.name).to.equal('AugmentedError');
  });

  it('should preserve original error stack when provided', () => {
    const originalError = new Error('Original');
    const originalStack = originalError.stack;
    
    const augmented = new AugmentedError(
      'New message',
      ErrorKind.ERROR,
      'error',
      undefined,
      originalError
    );

    expect(augmented.stack).to.equal(originalStack);
  });
});

describe('ErrorKind enum', () => {
  it('should have all expected values', () => {
    expect(ErrorKind.WARNING).to.equal('warning');
    expect(ErrorKind.ERROR).to.equal('error');
    expect(ErrorKind.NETWORK).to.equal('network');
    expect(ErrorKind.PERMISSION).to.equal('permission');
    expect(ErrorKind.TIMEOUT).to.equal('timeout');
    expect(ErrorKind.NOT_FOUND).to.equal('not-found');
    expect(ErrorKind.VALIDATION).to.equal('validation');
    expect(ErrorKind.SERVER).to.equal('server');
  });
});