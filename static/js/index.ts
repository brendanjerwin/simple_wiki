// Main entry point for web components
import './web-components/wiki-search.js';
import './web-components/version-display.js';
import './web-components/frontmatter-editor-dialog.js';
import './web-components/confirmation-dialog.js';
import './web-components/toast-message.js';
import './web-components/kernel-panic.js'; // Import to register the component
import { showStoredToast } from './web-components/toast-message.js';
import { setupGlobalErrorHandler } from './web-components/global-error-handler.js';
import { pageDeleteService } from './web-components/page-deletion-service.js';

// Set up global error handling to catch unhandled errors
setupGlobalErrorHandler();

// Make page deletion service available globally for simple_wiki.js
declare global {
  interface Window {
    pageDeleteService: PageDeletionService;
  }
}

(window as unknown as { pageDeleteService: typeof pageDeleteService }).pageDeleteService = pageDeleteService;

// Show any stored toast messages after page load
document.addEventListener('DOMContentLoaded', () => {
  showStoredToast();
});