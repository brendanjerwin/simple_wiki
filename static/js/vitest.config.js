import { defineConfig } from 'vitest/config';
import { fileURLToPath } from 'url';
import { dirname, resolve } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

export default defineConfig({
  test: {
    environment: 'happy-dom',
    globals: true,
    setupFiles: [resolve(__dirname, './setup.js')],
    coverage: {
      reporter: ['text', 'html', 'lcov'],
      reportsDirectory: resolve(__dirname, './coverage')
    }
  },
  resolve: {
    alias: {
      '/static/vendor/js/lit-all.min.js': resolve(__dirname, '../vendor/js/lit-all.min.js')
    }
  },
  server: {
    fs: {
      allow: ['..']
    }
  }
});