import { chromeLauncher } from '@web/test-runner-chrome';

const chromiumPath = process.env.CHROMIUM_BIN;
console.log('WTR Config - CHROMIUM_BIN:', chromiumPath);

export default {
  files: './web-components/**/*.test.js',
  base: '/static/js/',
  nodeResolve: true,
  browsers: [
    chromeLauncher({
      executablePath: chromiumPath,
    }),
  ],
};