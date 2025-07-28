// Main entry point for web components
import './web-components/wiki-search.js';
import './web-components/version-display.js';
import './web-components/frontmatter-editor-dialog.js';
import './web-components/page-delete-dialog.js';
import './web-components/toast-message.js';
import './web-components/kernel-panic.js'; // Import to register the component
import { showStoredToast } from './web-components/toast-message.js';
import { setupGlobalErrorHandler } from './web-components/global-error-handler.js';

// Set up global error handling to catch unhandled errors
setupGlobalErrorHandler();

// Show any stored toast messages after page load
document.addEventListener('DOMContentLoaded', () => {
  showStoredToast();
});