import { expect } from '@open-wc/testing';
import { ConnectError, Code } from '@connectrpc/connect';
import { 
  AugmentErrorService, 
  AugmentedError, 
  ErrorKind,
  type ErrorIcon
} from './augment-error-service.js';

describe('AugmentErrorService', () => {
  describe('augmentError', () => {
    describe('when processing ConnectError instances', () => {
      it('should handle UNAVAILABLE errors', () => {
        const connectError = new ConnectError('Service unavailable', Code.Unavailable);
        const augmented = AugmentErrorService.augmentError(connectError);

        expect(augmented).to.be.instanceof(AugmentedError);
        expect(augmented.message).to.equal('[unavailable] Service unavailable');
        expect(augmented.errorKind).to.equal(ErrorKind.NETWORK);
        expect(augmented.icon).to.equal('network');
        expect(augmented.details).to.include('gRPC error:');
      });

      it('should handle NOT_FOUND errors', () => {
        const connectError = new ConnectError('Resource not found', Code.NotFound);
        const augmented = AugmentErrorService.augmentError(connectError);

        expect(augmented.errorKind).to.equal(ErrorKind.NOT_FOUND);
        expect(augmented.icon).to.equal('not-found');
      });

      it('should handle PERMISSION_DENIED errors', () => {
        const connectError = new ConnectError('Access denied', Code.PermissionDenied);
        const augmented = AugmentErrorService.augmentError(connectError);

        expect(augmented.errorKind).to.equal(ErrorKind.PERMISSION);
        expect(augmented.icon).to.equal('permission');
      });

      it('should handle UNAUTHENTICATED errors', () => {
        const connectError = new ConnectError('Authentication required', Code.Unauthenticated);
        const augmented = AugmentErrorService.augmentError(connectError);

        expect(augmented.errorKind).to.equal(ErrorKind.PERMISSION);
        expect(augmented.icon).to.equal('permission');
      });

      it('should handle INVALID_ARGUMENT errors', () => {
        const connectError = new ConnectError('Invalid input', Code.InvalidArgument);
        const augmented = AugmentErrorService.augmentError(connectError);

        expect(augmented.errorKind).to.equal(ErrorKind.VALIDATION);
        expect(augmented.icon).to.equal('validation');
      });

      it('should handle DEADLINE_EXCEEDED errors', () => {
        const connectError = new ConnectError('Timeout', Code.DeadlineExceeded);
        const augmented = AugmentErrorService.augmentError(connectError);

        expect(augmented.errorKind).to.equal(ErrorKind.TIMEOUT);
        expect(augmented.icon).to.equal('timeout');
      });

      it('should handle INTERNAL errors', () => {
        const connectError = new ConnectError('Internal error', Code.Internal);
        const augmented = AugmentErrorService.augmentError(connectError);

        expect(augmented.errorKind).to.equal(ErrorKind.SERVER);
        expect(augmented.icon).to.equal('server');
      });

      it('should handle unknown error codes', () => {
        const connectError = new ConnectError('Unknown error', Code.Unknown);
        const augmented = AugmentErrorService.augmentError(connectError);

        expect(augmented.errorKind).to.equal(ErrorKind.ERROR);
        expect(augmented.icon).to.equal('error');
      });
    });

    describe('when processing regular Error instances', () => {
      it('should handle Error objects with message and stack', () => {
        const error = new Error('Test error');
        const augmented = AugmentErrorService.augmentError(error);

        expect(augmented).to.be.instanceof(AugmentedError);
        expect(augmented.message).to.equal('Test error');
        expect(augmented.errorKind).to.equal(ErrorKind.ERROR);
        expect(augmented.icon).to.equal('error');
        expect(augmented.details).to.include('Error: Test error');
      });

      it('should handle Error objects without message', () => {
        const error = new Error('');
        const augmented = AugmentErrorService.augmentError(error);

        expect(augmented.message).to.equal('An error occurred');
        expect(augmented.errorKind).to.equal(ErrorKind.ERROR);
      });

      it('should preserve original stack trace', () => {
        const error = new Error('Test error');
        const originalStack = error.stack;
        const augmented = AugmentErrorService.augmentError(error);

        expect(augmented.stack).to.equal(originalStack);
      });
    });

    describe('when processing AugmentedError instances', () => {
      it('should return the same instance without modification', () => {
        const augmented = new AugmentedError('Test', ErrorKind.WARNING, 'warning');
        const result = AugmentErrorService.augmentError(augmented);

        expect(result).to.equal(augmented);
      });
    });

    describe('when processing non-Error objects', () => {
      it('should handle string errors', () => {
        const augmented = AugmentErrorService.augmentError('String error');

        expect(augmented.message).to.equal('An unknown error occurred');
        expect(augmented.details).to.equal('String error');
        expect(augmented.errorKind).to.equal(ErrorKind.ERROR);
      });

      it('should handle object errors', () => {
        const errorObj = { code: 500, message: 'Server error' };
        const augmented = AugmentErrorService.augmentError(errorObj);

        expect(augmented.message).to.equal('An unknown error occurred');
        expect(augmented.details).to.equal(JSON.stringify(errorObj));
      });

      it('should use fallback message when provided', () => {
        const augmented = AugmentErrorService.augmentError('test', 'Custom fallback');

        expect(augmented.message).to.equal('Custom fallback');
        expect(augmented.details).to.equal('test');
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