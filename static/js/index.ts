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
import { pageDeleteService } from './web-components/page-deletion-service.js';
import { initInventoryMenu } from './web-components/inventory-menu.js';
import { initPrintMenu } from './web-components/print-label.js';
import { initPageImportMenu } from './web-components/page-import-menu.js';
import type { FrontmatterEditorDialog } from './web-components/frontmatter-editor-dialog.js';

// Set up global error handling to catch unhandled errors
setupGlobalErrorHandler();

declare global {
  interface Window {
    simple_wiki?: {
      pageName?: string;
      debounceMS?: number;
      lastFetch?: number;
    };
  }
}

// Show any stored toast messages after page load
document.addEventListener('DOMContentLoaded', () => {
  showStoredToast();

  // Handle page deletion
  const erasePageEl = document.getElementById('erasePage');
  erasePageEl?.addEventListener('click', (e) => {
    e.preventDefault();
    const pageName = window.simple_wiki?.pageName;
    if (pageName) {
      pageDeleteService.confirmAndDeletePage(pageName);
    }
  });

  // Handle frontmatter editing
  const editFrontmatterEl = document.getElementById('editFrontmatter');
  editFrontmatterEl?.addEventListener('click', (e) => {
    e.preventDefault();
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- frontmatter-editor-dialog is registered in HTMLElementTagNameMap
    const dialog = document.getElementById('frontmatter-dialog') as FrontmatterEditorDialog | null;
    dialog?.openDialog(window.simple_wiki?.pageName ?? '');
  });

  // Initialize dynamic menu items
  initPrintMenu();
  initInventoryMenu();
  initPageImportMenu();

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