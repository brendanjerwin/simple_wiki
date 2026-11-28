import { showKernelPanic } from './kernel-panic.js';
import { AugmentErrorService } from './augment-error-service.js';
import {
 attributeErrorEvent,
 attributeRejection,
 logForeignError,
} from './error-attribution.js';

// Store references to the handlers so we can remove them later
let errorHandler: ((event: ErrorEvent) => void) | null = null;
let rejectionHandler: ((event: PromiseRejectionEvent) => void) | null = null;

/**
 * Set up global error handlers to catch unhandled errors and promise rejections.
 *
 * The Kernel Panic screen is reserved for errors that originate in the wiki's
 * own code. Errors from foreign code injected into the page (browser extension
 * content scripts, injected helpers, `webkit-masked-url://` scripts) are logged
 * and left alone - the page stays alive and the browser keeps its own console
 * reporting for them. Attribution is evidence-based and defaults to ours, so
 * the gate can never swallow one of our own failures (issue #1501).
 */
export function setupGlobalErrorHandler(): void {
 // Handle unhandled JavaScript errors
 errorHandler = (event: ErrorEvent): void => {
  const attribution = attributeErrorEvent({
   filename: event.filename,
   error: event.error,
  });

  if (attribution.origin === 'foreign') {
   logForeignError({
    handler: 'error',
    attribution,
    reason: event.error,
    isTrusted: event.isTrusted,
    hasPromise: false,
    handlerEntryStack: undefined,
   });
   return;
  }

  const error = event.error || new Error(event.message || 'Unknown error');
  const augmentedError = AugmentErrorService.augmentError(error);
  showKernelPanic(augmentedError);
 };

 // Handle unhandled promise rejections
 rejectionHandler = (event: PromiseRejectionEvent): void => {
  // Captured before anything else: for non-Error reasons this is the only
  // evidence of which script dispatched the event.
  const handlerEntryStack = new Error().stack;
  const attribution = attributeRejection({
   reason: event.reason,
   handlerEntryStack,
  });

  if (attribution.origin === 'foreign') {
   logForeignError({
    handler: 'unhandledrejection',
    attribution,
    reason: event.reason,
    isTrusted: event.isTrusted,
    hasPromise: event.promise instanceof Promise,
    handlerEntryStack,
   });
   return;
  }

  // Only our own fatal errors suppress the browser's default reporting:
  // the Kernel Panic screen is the report for them. Foreign errors above
  // keep the browser console entry the browser would have written.
  event.preventDefault();

  const augmentedError = AugmentErrorService.augmentError(event.reason);
  showKernelPanic(augmentedError);
 };

 // Register the handlers
 globalThis.addEventListener('error', errorHandler);
 globalThis.addEventListener('unhandledrejection', rejectionHandler);
}

/**
 * Remove the global error handlers. Useful for testing or when the application
 * is being torn down.
 */
export function teardownGlobalErrorHandler(): void {
 if (errorHandler) {
  globalThis.removeEventListener('error', errorHandler);
  errorHandler = null;
 }

 if (rejectionHandler) {
  globalThis.removeEventListener('unhandledrejection', rejectionHandler);
  rejectionHandler = null;
 }
}
