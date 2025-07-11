import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'happy-dom',
    globals: true,
    setupFiles: ['./test/setup.js'],
    coverage: {
      reporter: ['text', 'html', 'lcov'],
      reportsDirectory: './coverage'
    }
  },
  resolve: {
    alias: {
      '/static/vendor/js/lit-all.min.js': new URL('./static/vendor/js/lit-all.min.js', import.meta.url).pathname
    }
  }
});