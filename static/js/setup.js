// Test setup file
import { afterEach } from 'vitest';

// Clean up the DOM after each test
afterEach(() => {
  document.body.innerHTML = '';
  // Clear all event listeners from window
  const proto = window.constructor?.prototype || {};
  for (const key in proto) {
    if (key.startsWith('on')) {
      window[key] = null;
    }
  }
});