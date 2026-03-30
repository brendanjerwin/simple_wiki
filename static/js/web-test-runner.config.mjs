import { chromeLauncher } from '@web/test-runner-chrome';
import { esbuildPlugin } from '@web/dev-server-esbuild';
import { summaryReporter } from '@web/test-runner';

const chromiumPath = process.env.CHROMIUM_BIN;
const ciCoverage = Boolean(process.env.CI_COVERAGE);
console.log('WTR Config - CHROMIUM_BIN:', chromiumPath);
console.log('WTR Config - Coverage enabled:', ciCoverage);

export default {
  files: ['./{web-components,services}/**/*.test.ts'],
  // Run test files sequentially: editor-toolbar-coordinator.test.ts calls openDialog()
  // which makes real gRPC network requests that interfere with concurrently running
  // Chrome test pages, causing flaky failures.
  concurrency: 1,
  base: '/static/js/',
  nodeResolve: true,
  reporters: [summaryReporter()],
  browsers: [
    chromeLauncher({
      executablePath: chromiumPath,
    }),
  ],
  plugins: [
    esbuildPlugin({
      ts: true,
      target: 'es2020',
      tsconfig: './tsconfig.json',
    }),
  ],
  coverage: ciCoverage,
  coverageConfig: {
    report: true,
    reportDir: 'coverage',
    reporters: ['lcov', 'json'],
    exclude: ['node_modules/**'],
  },
  testFramework: {
    config: {
      timeout: '10000', // 10 seconds
    },
  },
  testsFinishTimeout: 540000, // 9 minutes (allows coverage collection to complete within the 10-minute CI timeout),
  filterBrowserLogs(log) {
    // This is the full message that Lit logs to the console.
    const litDevModeMessage = 'Lit is in dev mode. Not recommended for production! See https://lit.dev/msg/dev-mode for more information.';
    return !log.args.some(
      arg =>
        typeof arg === 'string' && arg === litDevModeMessage,
    );
  },
};
