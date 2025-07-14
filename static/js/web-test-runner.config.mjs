import { fromRollup } from '@web/dev-server-rollup';
import rollupCommonjs from '@rollup/plugin-commonjs';
import { playwrightLauncher } from '@web/test-runner-playwright';
import { execSync } from 'child_process';

const commonjs = fromRollup(rollupCommonjs);

function getChromiumPath() {
  try {
    return execSync('which chromium').toString().trim();
  } catch (error) {
    console.error('Could not find chromium executable:', error.message);
    return undefined;
  }
}

const chromiumPath = getChromiumPath();

export default {
  files: './web-components/**/*.test.js',
  base: '/static/js/',
  nodeResolve: true,
  plugins: [
    commonjs(),
  ],
  testFramework: { type: 'mocha' },
  browsers: [
    playwrightLauncher({
      product: 'chromium',
      launchOptions: {
        executablePath: chromiumPath,
      },
    }),
  ],
};