import { expect } from '@open-wc/testing';
import { ConnectError, Code } from '@connectrpc/connect';
import {
  AugmentErrorService,
  AugmentedError,
  ErrorKind,
  coerceThirdPartyError
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
      });

      describe('when Error has empty message', () => {
        let error: Error;
        let augmented: AugmentedError;

        beforeEach(() => {
          error = new Error('');
          augmented = AugmentErrorService.augmentError(error);
        });

        it('should preserve empty message', () => {
          expect(augmented.message).to.equal('');
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
          augmented = new AugmentedError(new Error('Test'), ErrorKind.WARNING, 'warning');
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

        it('should use object stringification', () => {
          expect(augmented.message).to.equal('{"code":500,"message":"Server error"}');
        });

        it('should set errorKind to ERROR', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.ERROR);
        });
      });

      describe('when error is an object with circular reference', () => {
        let circularObj: Record<string, unknown>;
        let augmented: AugmentedError;

        beforeEach(() => {
          circularObj = { name: 'CircularError' };
          circularObj['self'] = circularObj;  // Create circular reference
          augmented = AugmentErrorService.augmentError(circularObj);
        });

        it('should fall back to Object.prototype.toString when JSON.stringify fails', () => {
          expect(augmented.message).to.equal('[object Object]');
        });

        it('should set errorKind to ERROR', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.ERROR);
        });
      });

      describe('when error is a function', () => {
        it('should use toString() to include function name in message', () => {
          const fn = function namedFn() { return 42; };
          const augmented = AugmentErrorService.augmentError(fn);
          expect(augmented.message).to.include('namedFn');
        });

        it('should set errorKind to ERROR', () => {
          const augmented = AugmentErrorService.augmentError(() => 0);
          expect(augmented.errorKind).to.equal(ErrorKind.ERROR);
        });
      });

      describe('when error is a bigint', () => {
        let augmented: AugmentedError;

        beforeEach(() => {
          augmented = AugmentErrorService.augmentError(BigInt(42));
        });

        it('should use the numeric string representation', () => {
          expect(augmented.message).to.equal('42');
        });

        it('should set errorKind to ERROR', () => {
          expect(augmented.errorKind).to.equal(ErrorKind.ERROR);
        });
      });

      describe('when error is null or undefined', () => {
        it('should use fallback message for null', () => {
          const augmented = AugmentErrorService.augmentError(null);
          expect(augmented.message).to.equal('An unknown error occurred');
        });

        it('should use fallback message for undefined', () => {
          const augmented = AugmentErrorService.augmentError(undefined);
          expect(augmented.message).to.equal('An unknown error occurred');
        });
      });

      describe('when error is a DOMException-like object (name + message, not instanceof Error)', () => {
        let augmented: AugmentedError;

        beforeEach(() => {
          // Simulate a cross-realm DOMException whose instanceof check fails in this realm.
          // Real DOMExceptions stringify to {} because their properties are non-enumerable.
          const domExceptionLike = { name: 'AbortError', message: 'The user aborted a request.', code: 20 };
          augmented = AugmentErrorService.augmentError(domExceptionLike);
        });

        it('should include AbortError in the message', () => {
          expect(augmented.message).to.include('AbortError');
        });

        it('should not produce {} as the message', () => {
          expect(augmented.message).not.to.equal('{}');
        });
      });

      describe('when error is an Error-like object with name, message, and stack but not instanceof Error', () => {
        let augmented: AugmentedError;
        const fakeStack = 'SomeError: oops\n    at someFunction (file.js:10:5)';

        beforeEach(() => {
          // The shape reported in #1501: looks exactly like an Error but instanceof is false.
          const errorLike = {
            name: 'SomeError',
            message: 'oops',
            stack: fakeStack,
          };
          augmented = AugmentErrorService.augmentError(errorLike);
        });

        it('should include the original message text', () => {
          expect(augmented.message).to.include('oops');
        });

        it('should preserve the stack from the Error-like object', () => {
          expect(augmented.stack).to.equal(fakeStack);
        });
      });

      describe('when error is an object whose toString throws', () => {
        let augmented: AugmentedError;

        beforeEach(() => {
          // toJSON throw forces JSON.stringify to fail; Object.prototype.toString is used safely.
          const dangerousObj = {
            toJSON() { throw new Error('toJSON deliberately threw'); },
            toString() { throw new Error('toString deliberately threw'); },
          };
          augmented = AugmentErrorService.augmentError(dangerousObj);
        });

        it('should not throw', () => {
          expect(augmented).to.be.instanceof(AugmentedError);
        });

        it('should fall back to Object.prototype.toString', () => {
          expect(augmented.message).to.equal('[object Object]');
        });
      });

      describe('when error is a plain empty object', () => {
        let augmented: AugmentedError;

        beforeEach(() => {
          augmented = AugmentErrorService.augmentError({});
        });

        it('should not produce the literal string {}', () => {
          expect(augmented.message).not.to.equal('{}');
        });

        it('should fall back to Object.prototype.toString', () => {
          expect(augmented.message).to.equal('[object Object]');
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
    const originalError = new Error('Test');
    const augmented = new AugmentedError(originalError, ErrorKind.ERROR, 'error');
    
    expect(augmented).to.be.instanceof(Error);
    expect(augmented).to.be.instanceof(AugmentedError);
  });

  it('should have correct properties', () => {
    const originalError = new Error('Test message');
    const augmented = new AugmentedError(
      originalError, 
      ErrorKind.WARNING, 
      'warning',
      'loading data'
    );

    expect(augmented.message).to.equal('Test message');
    expect(augmented.errorKind).to.equal(ErrorKind.WARNING);
    expect(augmented.icon).to.equal('warning');
    expect(augmented.failedGoalDescription).to.equal('loading data');
    expect(augmented.originalError).to.equal(originalError);
    expect(augmented.name).to.equal('Error');
  });

  describe('when the source Error has a stack', () => {
    let originalError: Error;
    let originalStack: string | undefined;
    let augmented: AugmentedError;

    beforeEach(() => {
      // Arrange
      originalError = new Error('Original');
      originalStack = originalError.stack;
      
      // Act
      augmented = new AugmentedError(
        originalError,
        ErrorKind.ERROR,
        'error'
      );
    });

    it('should preserve the stack', () => {
      expect(augmented.stack).to.equal(originalStack);
    });
  });

  describe('delegating to original error properties', () => {
    describe('when original error is TypeError', () => {
      let typeError: TypeError;
      let augmented: AugmentedError;

      beforeEach(() => {
        // Arrange
        typeError = new TypeError('Type error');

        // Act
        augmented = new AugmentedError(
          typeError,
          ErrorKind.ERROR,
          'error'
        );
      });

      it('should delegate name', () => {
        expect(augmented.name).to.equal('TypeError');
      });
    });

    describe('when original error has cause', () => {
      let causeError: Error;
      let originalError: Error;
      let augmented: AugmentedError;

      beforeEach(() => {
        // Arrange
        causeError = new Error('Root cause');
        originalError = new Error('Error with cause', { cause: causeError });

        // Act
        augmented = new AugmentedError(
          originalError,
          ErrorKind.ERROR,
          'error'
        );
      });

      it('should delegate cause', () => {
        expect(augmented.cause).to.equal(causeError);
      });
    });

    describe('when original error has no cause', () => {
      let originalError: Error;
      let augmented: AugmentedError;

      beforeEach(() => {
        // Arrange
        originalError = new Error('No cause');

        // Act
        augmented = new AugmentedError(
          originalError,
          ErrorKind.ERROR,
          'error'
        );
      });

      it('should return undefined for cause', () => {
        expect(augmented.cause).to.be.undefined;
      });
    });
  });
});

describe('coerceThirdPartyError', () => {
  describe('when error is an Error instance', () => {
    let originalError: Error;
    let result: Error;

    beforeEach(() => {
      originalError = new Error('Original error');
      result = coerceThirdPartyError(originalError, 'operation failed');
    });

    it('should return the same Error instance', () => {
      expect(result).to.equal(originalError);
    });
  });

  describe('when error is a string', () => {
    let result: Error;

    beforeEach(() => {
      result = coerceThirdPartyError('string error message', 'operation failed');
    });

    it('should return Error with string as message', () => {
      expect(result).to.be.instanceof(Error);
      expect(result.message).to.equal('string error message');
    });
  });

  describe('when error is a non-null object', () => {
    let result: Error;

    beforeEach(() => {
      result = coerceThirdPartyError({ code: 500 }, 'operation failed');
    });

    it('should return Error with JSON stringified object', () => {
      expect(result).to.be.instanceof(Error);
      expect(result.message).to.equal('{"code":500}');
    });
  });

  describe('when error is a number', () => {
    let result: Error;

    beforeEach(() => {
      result = coerceThirdPartyError(42, 'operation failed');
    });

    it('should return Error with number stringified', () => {
      expect(result).to.be.instanceof(Error);
      expect(result.message).to.equal('42');
    });
  });

  describe('when error is null', () => {
    let result: Error;

    beforeEach(() => {
      result = coerceThirdPartyError(null, 'operation failed');
    });

    it('should return Error with context message', () => {
      expect(result).to.be.instanceof(Error);
      expect(result.message).to.equal('operation failed');
    });
  });

  describe('when error is undefined', () => {
    let result: Error;

    beforeEach(() => {
      result = coerceThirdPartyError(undefined, 'operation failed');
    });

    it('should return Error with context message', () => {
      expect(result).to.be.instanceof(Error);
      expect(result.message).to.equal('operation failed');
    });
  });
});

