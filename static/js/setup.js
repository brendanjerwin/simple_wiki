// Test setup file
import { afterEach } from 'vitest';

// Clean up the DOM after each test
afterEach(() => {
  document.body.innerHTML = '';
  // Clear all event listeners from window
  const newWindow = globalThis.constructor.prototype;
  for (const key in newWindow) {
    if (key.startsWith('on')) {
      globalThis[key] = null;
    }
  }
});