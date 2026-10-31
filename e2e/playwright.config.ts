import { defineConfig, devices } from '@playwright/test';

/**
 * Get Chromium executable path, preferring environment variable like web-test-runner
 */
const getChromiumPath = (): string | undefined => {
 const chromiumBin = process.env.CHROMIUM_BIN;
 if (chromiumBin) {
  console.log('Playwright Config - Using CHROMIUM_BIN:', chromiumBin);
  return chromiumBin;
 }

 // Fallback to system chromium path
 const fallbackPath = '/usr/bin/chromium';
 console.log('Playwright Config - Using fallback path:', fallbackPath);
 return fallbackPath;
};

/**
 * E2E server port. Defaults to 8090 (chosen to avoid conflicts with the
 * default dev server on 8050); parallel workers in a shared run can claim
 * their own slot with `SW_E2E_PORT=8094`. The TLS port defaults to HTTP
 * port + 10002 and can be claimed independently via SW_E2E_TLS_PORT when
 * the derived default collides with unrelated listeners.
 */
const WEB_SERVER_PORT = Number(process.env.SW_E2E_PORT ?? 8090);
const WEB_SERVER_TLS_PORT = Number(process.env.SW_E2E_TLS_PORT ?? WEB_SERVER_PORT + 10002);

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
 testDir: './tests',
 /* Run tests in files in parallel */
 fullyParallel: true,
 /* Fail the build on CI if you accidentally left test.only in the source code. */
 forbidOnly: !!process.env.CI,
 /* Retry on CI only */
 retries: process.env.CI ? 2 : 1,
 /* Opt out of parallel tests on CI. */
 workers: 1,
 /* Reporter to use. See https://playwright.dev/docs/test-reporters */
 reporter: 'list',
 /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
 use: {
  /* Base URL to use in actions like `await page.goto('/')`. */
  baseURL: `http://localhost:${WEB_SERVER_PORT}`,

  /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
  trace: 'on-first-retry',

  /* Take screenshot on failure */
  screenshot: 'only-on-failure',

  /* Disable video recording to avoid FFmpeg dependency */
  video: 'off',

  /* Use headless mode for CI/container environments, but allow override via environment variable */
  headless: process.env.PLAYWRIGHT_HEADED !== 'true',
 },

 /* Configure web server - this replaces the complex shell script.
 * HTTP port defaults to 8090 and TLS to HTTP + 10002; both are overridable (see below). */
 webServer: {
  command: `cd .. && ./simple_wiki-linux-amd64 --port ${WEB_SERVER_PORT} --tls-port ${WEB_SERVER_TLS_PORT} --data e2e/test-data --debug --chat-persona "" --trust-test-identity-headers`,
  url: `http://localhost:${WEB_SERVER_PORT}`,
  // Dev convenience: reuse an already-listening server (dev + manual
  // e2e-wiki-server sessions). In CI, never reuse — a stale/wrong
  // binary must fail loudly rather than produce a false green.
  reuseExistingServer: !process.env.CI,
  stdout: 'pipe',
  stderr: 'pipe',
  timeout: 30 * 1000,
 },

 /* Global setup and teardown for test data */
 globalSetup: './global-setup.ts',
 globalTeardown: './global-teardown.ts',

 /* Configure projects for major browsers */
 projects: [
  {
   name: 'chromium',
   use: {
    ...devices['Desktop Chrome'],
    /* Use system browser with proper path from environment */
    launchOptions: {
     executablePath: getChromiumPath(),
     args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
      '--disable-background-timer-throttling',
      '--disable-backgrounding-occluded-windows',
      '--disable-renderer-backgrounding',
      '--disable-extensions',
      '--disable-plugins',
      '--disable-gpu'
     ]
    }
   },
  },
 ],
});
