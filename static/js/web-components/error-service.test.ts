import { expect } from '@open-wc/testing';
import { ConnectError, Code } from '@connectrpc/connect';
import { ErrorService } from './error-service.js';

describe('ErrorService', () => {
  describe('processError', () => {
    describe('when processing ConnectError instances', () => {
      it('should handle UNAVAILABLE errors', () => {
        const error = new ConnectError('Connection failed', Code.Unavailable);
        const result = ErrorService.processError(error, 'test operation');
        
        expect(result.message).to.equal('Unable to connect to server');
        expect(result.details).to.include('gRPC error: [unavailable] Connection failed');
        expect(result.icon).to.equal('network');
      });

      it('should handle NOT_FOUND errors', () => {
        const error = new ConnectError('Resource not found', Code.NotFound);
        const result = ErrorService.processError(error, 'load frontmatter');
        
        expect(result.message).to.equal('Page not found');
        expect(result.details).to.include('gRPC error: [not_found] Resource not found');
        expect(result.icon).to.equal('not-found');
      });

      it('should handle PERMISSION_DENIED errors', () => {
        const error = new ConnectError('Access denied', Code.PermissionDenied);
        const result = ErrorService.processError(error, 'save data');
        
        expect(result.message).to.equal('Access denied');
        expect(result.details).to.include('gRPC error: [permission_denied] Access denied');
        expect(result.icon).to.equal('permission');
      });

      it('should handle INVALID_ARGUMENT errors', () => {
        const error = new ConnectError('Invalid input', Code.InvalidArgument);
        const result = ErrorService.processError(error, 'validate data');
        
        expect(result.message).to.equal('Invalid request');
        expect(result.details).to.include('gRPC error: [invalid_argument] Invalid input');
        expect(result.icon).to.equal('validation');
      });

      it('should handle DEADLINE_EXCEEDED errors', () => {
        const error = new ConnectError('Timeout', Code.DeadlineExceeded);
        const result = ErrorService.processError(error, 'fetch data');
        
        expect(result.message).to.equal('Request timed out');
        expect(result.details).to.include('gRPC error: [deadline_exceeded] Timeout');
        expect(result.icon).to.equal('timeout');
      });

      it('should handle INTERNAL errors', () => {
        const error = new ConnectError('Server error', Code.Internal);
        const result = ErrorService.processError(error, 'process request');
        
        expect(result.message).to.equal('Server error');
        expect(result.details).to.include('gRPC error: [internal] Server error');
        expect(result.icon).to.equal('server');
      });

      it('should handle UNAUTHENTICATED errors', () => {
        const error = new ConnectError('Not authenticated', Code.Unauthenticated);
        const result = ErrorService.processError(error, 'access resource');
        
        expect(result.message).to.equal('Authentication required');
        expect(result.details).to.include('gRPC error: [unauthenticated] Not authenticated');
        expect(result.icon).to.equal('permission');
      });

      it('should handle unknown error codes', () => {
        const error = new ConnectError('Unknown error', Code.Unknown);
        const result = ErrorService.processError(error, 'perform action');
        
        expect(result.message).to.equal('Failed to perform action');
        expect(result.details).to.include('gRPC error: [unknown] Unknown error');
        expect(result.icon).to.equal('error');
      });
    });

    describe('when processing regular Error instances', () => {
      it('should handle Error objects with message and stack', () => {
        const error = new Error('Something went wrong');
        error.stack = 'Error: Something went wrong\n    at test.js:1:1';
        
        const result = ErrorService.processError(error, 'test operation');
        
        expect(result.message).to.equal('Something went wrong');
        expect(result.details).to.equal(error.stack);
        expect(result.icon).to.equal('error');
      });

      it('should handle Error objects without message', () => {
        const error = new Error('');
        
        const result = ErrorService.processError(error, 'test operation');
        
        expect(result.message).to.equal('Failed to test operation');
        expect(result.icon).to.equal('error');
      });

      it('should handle Error objects without stack', () => {
        const error = new Error('Test error');
        error.stack = undefined;
        
        const result = ErrorService.processError(error, 'test operation');
        
        expect(result.message).to.equal('Test error');
        expect(result.details).to.equal('Error: Test error');
        expect(result.icon).to.equal('error');
      });
    });

    describe('when processing non-Error objects', () => {
      it('should handle string errors', () => {
        const result = ErrorService.processError('String error message', 'test operation');
        
        expect(result.message).to.equal('Failed to test operation');
        expect(result.details).to.equal('String error message');
        expect(result.icon).to.equal('error');
      });

      it('should handle object errors', () => {
        const errorObj = { code: 500, message: 'Server error' };
        const result = ErrorService.processError(errorObj, 'test operation');
        
        expect(result.message).to.equal('Failed to test operation');
        expect(result.details).to.equal(JSON.stringify(errorObj));
        expect(result.icon).to.equal('error');
      });

      it('should handle null/undefined errors', () => {
        const result = ErrorService.processError(null, 'test operation');
        
        expect(result.message).to.equal('Failed to test operation');
        expect(result.details).to.equal('null');
        expect(result.icon).to.equal('error');
      });
    });

    describe('context-specific behavior', () => {
      it('should generate appropriate not found messages for different contexts', () => {
        const error = new ConnectError('Not found', Code.NotFound);
        
        const frontmatterResult = ErrorService.processError(error, 'load frontmatter');
        expect(frontmatterResult.message).to.equal('Page not found');
        
        const saveResult = ErrorService.processError(error, 'save frontmatter');
        expect(saveResult.message).to.equal('Cannot save to non-existent page');
        
        const genericResult = ErrorService.processError(error, 'fetch data');
        expect(genericResult.message).to.equal('Resource not found');
      });

      it('should include context in fallback error messages', () => {
        const result = ErrorService.processError('test error', 'custom operation');
        
        expect(result.message).to.equal('Failed to custom operation');
      });
    });
  });
});