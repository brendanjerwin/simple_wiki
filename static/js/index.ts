// Main entry point for web components
import './web-components/wiki-search.js';
import './web-components/system-info.js';
import './web-components/frontmatter-editor-dialog.js';
import './web-components/confirmation-dialog.js';
import './web-components/toast-message.js';
import './web-components/kernel-panic.js'; // Import to register the component
import './web-components/inventory-add-item-dialog.js';
import './web-components/inventory-move-item-dialog.js';
import { showStoredToast } from './web-components/toast-message.js';
import { setupGlobalErrorHandler } from './web-components/global-error-handler.js';
import { pageDeleteService } from './web-components/page-deletion-service.js';
import { inventoryActionService } from './web-components/inventory-action-service.js';

// Set up global error handling to catch unhandled errors
setupGlobalErrorHandler();

// Make services available globally for simple_wiki.js
declare global {
  interface Window {
    pageDeleteService: PageDeletionService;
    inventoryActionService: typeof inventoryActionService;
  }
}

(window as unknown as { pageDeleteService: typeof pageDeleteService }).pageDeleteService = pageDeleteService;
(window as unknown as { inventoryActionService: typeof inventoryActionService }).inventoryActionService = inventoryActionService;

// Show any stored toast messages after page load
document.addEventListener('DOMContentLoaded', () => {
  showStoredToast();
});