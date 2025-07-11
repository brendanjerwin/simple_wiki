// Test setup file
import { beforeEach, afterEach } from 'vitest';

// Clean up the DOM after each test
afterEach(() => {
  document.body.innerHTML = '';
  // Clear all event listeners from window
  const newWindow = window.constructor.prototype;
  for (const key in newWindow) {
    if (key.startsWith('on')) {
      window[key] = null;
    }
  }
});