import { chromeLauncher } from '@web/test-runner-chrome';
import { esbuildPlugin } from '@web/dev-server-esbuild';

const chromiumPath = process.env.CHROMIUM_BIN;
console.log('WTR Config - CHROMIUM_BIN:', chromiumPath);

export default {
  files: './web-components/**/*.test.ts',
  base: '/static/js/',
  nodeResolve: true,
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
      timeout: '5000', // 5 seconds
    },
  },
  testsFinishTimeout: 180000,
};