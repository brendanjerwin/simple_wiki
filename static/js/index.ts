// Main entry point for web components
import './web-components/wiki-search.js';
import './web-components/system-info.js';
import './web-components/frontmatter-editor-dialog.js';
import './web-components/confirmation-dialog.js';
import './web-components/toast-message.js';
import './web-components/kernel-panic.js'; // Import to register the component
import './web-components/inventory-add-item-dialog.js';
import './web-components/inventory-move-item-dialog.js';
import './web-components/wiki-image.js';
import './web-components/page-import-dialog.js';
import './web-components/insert-new-page-dialog.js';
import './web-components/wiki-checklist.js';
import './web-components/wiki-blog.js';
import './web-components/blog-new-post-dialog.js';
import './web-components/file-drop-zone.js';
import './web-components/wiki-editor.js';
import './web-components/wiki-table.js';
import { showStoredToast } from './web-components/toast-message.js';
import { setupGlobalErrorHandler } from './web-components/global-error-handler.js';
import { pageDeleteService, type PageDeleter } from './web-components/page-deletion-service.js';

// Set up global error handling to catch unhandled errors
setupGlobalErrorHandler();

// Make services available globally for simple_wiki.js
declare global {
  interface Window {
    pageDeleteService: PageDeleter;
    simple_wiki?: {
      pageName?: string;
      debounceMS?: number;
      lastFetch?: number;
    };
  }
}

// Make pageDeleteService available globally
window.pageDeleteService = pageDeleteService;

// Show any stored toast messages after page load
document.addEventListener('DOMContentLoaded', () => {
  showStoredToast();

  // Handle editor exit button (toolbar is inside wiki-editor shadow DOM)
  const wikiEditor = document.querySelector('wiki-editor');
  if (wikiEditor) {
    wikiEditor.addEventListener('exit-requested', () => {
      const pageName = window.simple_wiki?.pageName;
      if (pageName) {
        window.location.href = `/${pageName}/view`;
      } else {
        window.location.href = '/';
      }
    });
  }
});