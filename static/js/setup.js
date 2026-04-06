// Test setup file
import { afterEach } from 'vitest';

// Clean up the DOM after each test
afterEach(() => {
  document.body.innerHTML = '';
  // Clear all event listeners from window
  const proto = globalThis.constructor?.prototype || {};
  for (const key in proto) {
    if (key.startsWith('on')) {
      globalThis[key] = null;
    }
  }
});