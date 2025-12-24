// Main entry point for web components
import './web-components/wiki-search.js';
import './web-components/system-info.js';
import './web-components/frontmatter-editor-dialog.js';
import './web-components/confirmation-dialog.js';
import './web-components/toast-message.js';
import './web-components/kernel-panic.js'; // Import to register the component
import './web-components/inventory-add-item-dialog.js';
import './web-components/inventory-move-item-dialog.js';
import './web-components/editor-context-menu.js';
import './web-components/editor-toolbar.js';
import { showStoredToast } from './web-components/toast-message.js';
import { setupGlobalErrorHandler } from './web-components/global-error-handler.js';
import { pageDeleteService } from './web-components/page-deletion-service.js';
import { EditorContextMenuCoordinator } from './services/editor-context-menu-coordinator.js';
import type { EditorContextMenu } from './web-components/editor-context-menu.js';
import type { EditorToolbar } from './web-components/editor-toolbar.js';

// Set up global error handling to catch unhandled errors
setupGlobalErrorHandler();

// Make services available globally for simple_wiki.js
declare global {
  interface Window {
    pageDeleteService: PageDeletionService;
    simple_wiki?: {
      pageName?: string;
      debounceMS?: number;
      lastFetch?: number;
    };
  }
}

(window as unknown as { pageDeleteService: typeof pageDeleteService }).pageDeleteService = pageDeleteService;

// Show any stored toast messages after page load
document.addEventListener('DOMContentLoaded', () => {
  showStoredToast();

  // Initialize editor context menu and toolbar on edit pages
  const textarea = document.getElementById('userInput') as HTMLTextAreaElement | null;
  const menu = document.getElementById('editor-context-menu') as EditorContextMenu | null;
  const toolbar = document.getElementById('editor-toolbar') as EditorToolbar | null;
  if (textarea && menu) {
    new EditorContextMenuCoordinator(textarea, menu, undefined, undefined, toolbar);
  }

  // Handle toolbar exit button
  if (toolbar) {
    toolbar.addEventListener('exit-requested', () => {
      const pageName = window.simple_wiki?.pageName;
      if (pageName) {
        window.location.href = `/${pageName}/view`;
      } else {
        window.location.href = '/';
      }
    });
  }
});