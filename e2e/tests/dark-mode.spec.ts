import { test, expect } from '@playwright/test';

// Test data — unique per run/worker to avoid parallel collision
const TEST_PAGE_NAME = `e2edarkmodestest-${process.pid}-${Date.now()}`;

// Constants
const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;
const SEARCH_TIMEOUT_MS = 10000;
const DIALOG_TIMEOUT_MS = 5000;

// Design token values from default.css — Light Mode (:root defaults)
const LIGHT_SURFACE_PRIMARY = '#ffffff';
const LIGHT_SURFACE_SUNKEN = '#f8f9fa';
const LIGHT_TEXT_PRIMARY = '#333333';

// Design token values from default.css — Dark Mode (@media prefers-color-scheme: dark)
const DARK_SURFACE_PRIMARY = '#1e1e1e';
const DARK_SURFACE_ELEVATED = '#2d2d2d';
const DARK_SURFACE_SUNKEN = '#141414';
const DARK_TEXT_PRIMARY = '#e9ecef';

test.describe('Dark Mode E2E Tests', () => {
  test.setTimeout(60000);

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    await page.goto(`/${TEST_PAGE_NAME}/edit`);
    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await textarea.fill(`+++
identifier = "${TEST_PAGE_NAME}"
title = "Dark Mode Test Page"
+++

# Dark Mode Test Page

This page contains content for testing dark mode E2E functionality.
The unique term e2edarkmode_unique_xyz helps search find this page.`);
    await textarea.press('Space');
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
      timeout: SAVE_TIMEOUT_MS,
    });

    await ctx.close();
  });

  test.afterAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    try {
      await page.goto(`/${TEST_PAGE_NAME}/edit`);
      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await textarea.fill(`+++
identifier = "${TEST_PAGE_NAME}"
+++`);
      await textarea.press('Space');
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
        timeout: SAVE_TIMEOUT_MS,
      });
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      console.log(`Cleanup failed: ${errorMessage}`);
    } finally {
      await ctx.close();
    }
  });

  test.describe('design token application in light mode', () => {
    test('should apply light surface-primary token to :root', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'light' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        const token = await page.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-surface-primary').trim()
        );

        expect(token).toBe(LIGHT_SURFACE_PRIMARY);
      } finally {
        await ctx.close();
      }
    });

    test('should apply light text-primary token to :root', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'light' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        const token = await page.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-text-primary').trim()
        );

        expect(token).toBe(LIGHT_TEXT_PRIMARY);
      } finally {
        await ctx.close();
      }
    });

    test('should use light surface token as body background color', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'light' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        // Light surface-primary #ffffff = rgb(255, 255, 255)
        await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(255, 255, 255)');
      } finally {
        await ctx.close();
      }
    });

    test('should apply light surface-sunken token to navigation bar', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'light' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        // Light surface-sunken #f8f9fa = rgb(248, 249, 250)
        await expect(page.locator('div.pure-menu-horizontal')).toHaveCSS('background-color', 'rgb(248, 249, 250)');
      } finally {
        await ctx.close();
      }
    });
  });

  test.describe('design token application in dark mode', () => {
    test('should apply dark surface-primary token to :root', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'dark' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        const token = await page.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-surface-primary').trim()
        );

        expect(token).toBe(DARK_SURFACE_PRIMARY);
      } finally {
        await ctx.close();
      }
    });

    test('should apply dark text-primary token to :root', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'dark' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        const token = await page.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-text-primary').trim()
        );

        expect(token).toBe(DARK_TEXT_PRIMARY);
      } finally {
        await ctx.close();
      }
    });

    test('should use dark surface token as body background color', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'dark' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        // Dark surface-primary #1e1e1e = rgb(30, 30, 30)
        await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(30, 30, 30)');
      } finally {
        await ctx.close();
      }
    });

    test('should apply dark surface-sunken token to navigation bar', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'dark' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        // Dark surface-sunken #141414 = rgb(20, 20, 20)
        await expect(page.locator('div.pure-menu-horizontal')).toHaveCSS('background-color', 'rgb(20, 20, 20)');
      } finally {
        await ctx.close();
      }
    });
  });

  test.describe('design token contrast between light and dark modes', () => {
    test('should use different surface-primary values in light and dark mode', async ({ browser }) => {
      let lightToken = '';
      let darkToken = '';

      const lightCtx = await browser.newContext({ colorScheme: 'light' });
      try {
        const lightPage = await lightCtx.newPage();
        await lightPage.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(lightPage.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
        lightToken = await lightPage.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-surface-primary').trim()
        );
      } finally {
        await lightCtx.close();
      }

      const darkCtx = await browser.newContext({ colorScheme: 'dark' });
      try {
        const darkPage = await darkCtx.newPage();
        await darkPage.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(darkPage.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
        darkToken = await darkPage.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-surface-primary').trim()
        );
      } finally {
        await darkCtx.close();
      }

      expect(lightToken).toBe(LIGHT_SURFACE_PRIMARY);
      expect(darkToken).toBe(DARK_SURFACE_PRIMARY);
      expect(lightToken).not.toBe(darkToken);
    });

    test('should use different text-primary values in light and dark mode', async ({ browser }) => {
      let lightToken = '';
      let darkToken = '';

      const lightCtx = await browser.newContext({ colorScheme: 'light' });
      try {
        const lightPage = await lightCtx.newPage();
        await lightPage.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(lightPage.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
        lightToken = await lightPage.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-text-primary').trim()
        );
      } finally {
        await lightCtx.close();
      }

      const darkCtx = await browser.newContext({ colorScheme: 'dark' });
      try {
        const darkPage = await darkCtx.newPage();
        await darkPage.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(darkPage.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
        darkToken = await darkPage.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-text-primary').trim()
        );
      } finally {
        await darkCtx.close();
      }

      expect(lightToken).toBe(LIGHT_TEXT_PRIMARY);
      expect(darkToken).toBe(DARK_TEXT_PRIMARY);
      expect(lightToken).not.toBe(darkToken);
    });

    test('should use different surface-sunken values in light and dark mode', async ({ browser }) => {
      let lightToken = '';
      let darkToken = '';

      const lightCtx = await browser.newContext({ colorScheme: 'light' });
      try {
        const lightPage = await lightCtx.newPage();
        await lightPage.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(lightPage.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
        lightToken = await lightPage.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-surface-sunken').trim()
        );
      } finally {
        await lightCtx.close();
      }

      const darkCtx = await browser.newContext({ colorScheme: 'dark' });
      try {
        const darkPage = await darkCtx.newPage();
        await darkPage.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(darkPage.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
        darkToken = await darkPage.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-surface-sunken').trim()
        );
      } finally {
        await darkCtx.close();
      }

      expect(lightToken).toBe(LIGHT_SURFACE_SUNKEN);
      expect(darkToken).toBe(DARK_SURFACE_SUNKEN);
      expect(lightToken).not.toBe(darkToken);
    });
  });

  test.describe('dark mode persistence', () => {
    test('should maintain dark mode tokens after page reload', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'dark' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        await page.reload();
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        const token = await page.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-surface-primary').trim()
        );

        expect(token).toBe(DARK_SURFACE_PRIMARY);
      } finally {
        await ctx.close();
      }
    });

    test('should maintain dark mode tokens when navigating from view to edit page', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'dark' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        const viewToken = await page.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-surface-primary').trim()
        );

        await page.goto(`/${TEST_PAGE_NAME}/edit`);
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        const editToken = await page.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-surface-primary').trim()
        );

        expect(viewToken).toBe(DARK_SURFACE_PRIMARY);
        expect(editToken).toBe(DARK_SURFACE_PRIMARY);
      } finally {
        await ctx.close();
      }
    });
  });

  test.describe('dialog backgrounds in dark mode', () => {
    test('should render frontmatter dialog with dark elevated surface background in dark mode', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'dark' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        await page.locator('.tools-menu').hover();
        await page.click('#editFrontmatter');

        const dialog = page.locator('frontmatter-editor-dialog').locator('.dialog');
        await expect(dialog).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Verify dialog uses dark elevated surface token (CSS variable inherited from :root)
        // Dark surface-elevated #2d2d2d = rgb(45, 45, 45)
        await expect(dialog).toHaveCSS('background-color', 'rgb(45, 45, 45)');
      } finally {
        await ctx.close();
      }
    });

    test('should render frontmatter dialog with light elevated surface background in light mode', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'light' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        await page.locator('.tools-menu').hover();
        await page.click('#editFrontmatter');

        const dialog = page.locator('frontmatter-editor-dialog').locator('.dialog');
        await expect(dialog).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Light surface-elevated #ffffff = rgb(255, 255, 255)
        await expect(dialog).toHaveCSS('background-color', 'rgb(255, 255, 255)');
      } finally {
        await ctx.close();
      }
    });
  });

  test.describe('item_content contrast in search results', () => {
    test('should use dark elevated surface for item_content background in dark mode', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'dark' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        const searchComponent = page.locator('wiki-search#site-search');
        const searchInput = searchComponent.locator('input[type="search"]');
        await expect(searchInput).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await searchInput.fill('e2edarkmode_unique_xyz');
        await searchInput.press('Enter');

        // Wait for results to appear
        const resultsComponent = searchComponent.locator('wiki-search-results');
        await expect(resultsComponent).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });

        // Verify item_content is visible in results
        const itemContent = resultsComponent.locator('.item_content').first();
        await expect(itemContent).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });

        // Check computed background color of item_content (uses --color-surface-elevated)
        // Dark surface-elevated #2d2d2d = rgb(45, 45, 45)
        await expect(itemContent).toHaveCSS('background-color', 'rgb(45, 45, 45)');
      } finally {
        await ctx.close();
      }
    });

    test('should use light elevated surface for item_content background in light mode', async ({ browser }) => {
      const ctx = await browser.newContext({ colorScheme: 'light' });
      const page = await ctx.newPage();

      try {
        await page.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        const searchComponent = page.locator('wiki-search#site-search');
        const searchInput = searchComponent.locator('input[type="search"]');
        await expect(searchInput).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await searchInput.fill('e2edarkmode_unique_xyz');
        await searchInput.press('Enter');

        const resultsComponent = searchComponent.locator('wiki-search-results');
        await expect(resultsComponent).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });

        const itemContent = resultsComponent.locator('.item_content').first();
        await expect(itemContent).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });

        // Light surface-elevated #ffffff = rgb(255, 255, 255)
        await expect(itemContent).toHaveCSS('background-color', 'rgb(255, 255, 255)');
      } finally {
        await ctx.close();
      }
    });

    test('should show different item_content contrast between light and dark mode', async ({ browser }) => {
      let lightBg = '';
      let darkBg = '';

      const lightCtx = await browser.newContext({ colorScheme: 'light' });
      try {
        const lightPage = await lightCtx.newPage();
        await lightPage.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(lightPage.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        const lightSearch = lightPage.locator('wiki-search#site-search');
        await lightSearch.locator('input[type="search"]').fill('e2edarkmode_unique_xyz');
        await lightSearch.locator('input[type="search"]').press('Enter');
        await expect(lightSearch.locator('wiki-search-results')).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });
        const lightItemContent = lightSearch.locator('wiki-search-results .item_content').first();
        await expect(lightItemContent).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });
        lightBg = await lightItemContent.evaluate(el => getComputedStyle(el).backgroundColor);
      } finally {
        await lightCtx.close();
      }

      const darkCtx = await browser.newContext({ colorScheme: 'dark' });
      try {
        const darkPage = await darkCtx.newPage();
        await darkPage.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(darkPage.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

        const darkSearch = darkPage.locator('wiki-search#site-search');
        await darkSearch.locator('input[type="search"]').fill('e2edarkmode_unique_xyz');
        await darkSearch.locator('input[type="search"]').press('Enter');
        await expect(darkSearch.locator('wiki-search-results')).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });
        const darkItemContent = darkSearch.locator('wiki-search-results .item_content').first();
        await expect(darkItemContent).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });
        darkBg = await darkItemContent.evaluate(el => getComputedStyle(el).backgroundColor);
      } finally {
        await darkCtx.close();
      }

      expect(lightBg).not.toBe(darkBg);
      expect(darkBg).toBe('rgb(45, 45, 45)');
      expect(lightBg).toBe('rgb(255, 255, 255)');
    });
  });

  test.describe('dark mode surface-elevated token', () => {
    test('should apply different surface-elevated token values in light and dark mode', async ({ browser }) => {
      let lightToken = '';
      let darkToken = '';

      const lightCtx = await browser.newContext({ colorScheme: 'light' });
      try {
        const lightPage = await lightCtx.newPage();
        await lightPage.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(lightPage.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
        lightToken = await lightPage.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-surface-elevated').trim()
        );
      } finally {
        await lightCtx.close();
      }

      const darkCtx = await browser.newContext({ colorScheme: 'dark' });
      try {
        const darkPage = await darkCtx.newPage();
        await darkPage.goto(`/${TEST_PAGE_NAME}/view`);
        await expect(darkPage.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
        darkToken = await darkPage.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--color-surface-elevated').trim()
        );
      } finally {
        await darkCtx.close();
      }

      expect(darkToken).toBe(DARK_SURFACE_ELEVATED);
      expect(lightToken).not.toBe(darkToken);
    });
  });
});
