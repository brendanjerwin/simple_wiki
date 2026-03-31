import { defineConfig } from 'vitest/config';

const ciCoverage = Boolean(process.env['CI_COVERAGE']);

export default defineConfig({
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
    ...(ciCoverage && {
      coverage: {
        provider: 'v8',
        reporter: ['lcov', 'text'],
        reportsDirectory: './coverage',
        exclude: ['**/*.test.ts'],
      },
    }),
  },
});
