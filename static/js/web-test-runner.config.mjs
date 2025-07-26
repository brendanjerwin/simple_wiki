import { chromeLauncher } from '@web/test-runner-chrome';
import { esbuildPlugin } from '@web/dev-server-esbuild';
import { summaryReporter } from '@web/test-runner';

const chromiumPath = process.env.CHROMIUM_BIN;
console.log('WTR Config - CHROMIUM_BIN:', chromiumPath);

export default {
  files: './web-components/**/*.test.ts',
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
  coverage: true,
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
  testsFinishTimeout: 600000, // 10 minutes,
  filterBrowserLogs(log) {
    // This is the full message that Lit logs to the console.
    const litDevModeMessage = 'Lit is in dev mode. Not recommended for production! See https://lit.dev/msg/dev-mode for more information.';
    return !log.args.some(
      arg =>
        typeof arg === 'string' && arg === litDevModeMessage,
    );
  },
};
