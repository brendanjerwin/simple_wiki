import { defineConfig, devices } from '@playwright/test';
import path from 'path';

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
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  testDir: './tests',
  /* Run tests in files in parallel */
  fullyParallel: false,
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
    baseURL: 'http://localhost:8051',

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',
    
    /* Take screenshot on failure */
    screenshot: 'only-on-failure',
    
    /* Disable video recording to avoid FFmpeg dependency */
    video: 'off',
    
    /* Use headless mode for CI/container environments */
    headless: true,
  },

  /* Configure web server - this replaces the complex shell script */
  webServer: {
    command: 'cd .. && ./simple_wiki-linux-amd64 --port 8051 --data e2e/test-data --debug',
    url: 'http://localhost:8051',
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