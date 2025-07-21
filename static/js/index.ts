// Main entry point for web components
import './web-components/wiki-search.js';
import './web-components/version-display.js';
import './web-components/frontmatter-editor-dialog.js';
import './web-components/toast-message.js';
import { showStoredToast } from './web-components/toast-message.js';

// Show any stored toast messages after page load
document.addEventListener('DOMContentLoaded', () => {
  showStoredToast();
});