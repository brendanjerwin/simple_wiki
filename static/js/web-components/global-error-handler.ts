import { showKernelPanic } from './kernel-panic.js';

// Store references to the handlers so we can remove them later
let errorHandler: ((event: ErrorEvent) => void) | null = null;
let rejectionHandler: ((event: PromiseRejectionEvent) => void) | null = null;

/**
 * Set up global error handlers to catch unhandled errors and promise rejections.
 * These will display a kernel panic screen for any errors that bubble up without
 * being handled by application code.
 */
export function setupGlobalErrorHandler(): void {
  // Handle unhandled JavaScript errors
  errorHandler = (event: ErrorEvent): void => {
    const error = event.error || new Error(event.message || 'Unknown error');
    showKernelPanic('An unhandled error has occurred', error);
  };

  // Handle unhandled promise rejections
  rejectionHandler = (event: PromiseRejectionEvent): void => {
    event.preventDefault(); // Prevent the default browser handling
    
    // Convert the rejection reason to an Error if it's not already one
    const error = event.reason instanceof Error 
      ? event.reason 
      : new Error(String(event.reason));
    
    showKernelPanic('An unhandled promise rejection has occurred', error);
  };

  // Register the handlers
  window.addEventListener('error', errorHandler);
  window.addEventListener('unhandledrejection', rejectionHandler);
}

/**
 * Remove the global error handlers. Useful for testing or when the application
 * is being torn down.
 */
export function teardownGlobalErrorHandler(): void {
  if (errorHandler) {
    window.removeEventListener('error', errorHandler);
    errorHandler = null;
  }
  
  if (rejectionHandler) {
    window.removeEventListener('unhandledrejection', rejectionHandler);
    rejectionHandler = null;
  }
}