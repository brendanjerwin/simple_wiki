import { test, expect, type Browser, type Locator, type Page } from '@playwright/test';

// Test data — unique per run/worker to avoid parallel collision
const TEST_PAGE_NAME = `e2edarkmodestest-${process.pid}-${Date.now()}`;

// Constants
const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;
const DIALOG_TIMEOUT_MS = 5000;
// Search index is updated asynchronously; allow extra time for the indexer to process the new page.
const SEARCH_INDEX_UPDATE_TIMEOUT_MS = 30000;
const SEARCH_INDEX_RETRY_INTERVAL_MS = 2000;

// Design token values from default.css — Light Mode (:root defaults)
const LIGHT_SURFACE_PRIMARY = '#ffffff';
const LIGHT_SURFACE_SUNKEN = '#f8f9fa';
const LIGHT_TEXT_PRIMARY = '#333333';

// Design token values from default.css — Dark Mode (@media prefers-color-scheme: dark)
const DARK_SURFACE_PRIMARY = '#1e1e1e';
const DARK_SURFACE_ELEVATED = '#2d2d2d';
const DARK_SURFACE_SUNKEN = '#141414';
const DARK_TEXT_PRIMARY = '#e9ecef';

/**
 * Retries a search query until at least one result appears in .item_content.
 *
 * wiki-search-results uses a position:fixed popover, so its host element has zero height
 * and cannot be detected with toBeVisible(). We wait for .item_content directly since
 * that IS visible inside the popover. The search index is updated asynchronously after
 * saving a page, so retrying is necessary for freshly-created pages.
 */
async function searchUntilResultsVisible(
  searchInput: Locator,
  itemContentLocator: Locator,
  term: string,
): Promise<void> {
  const maxAttempts = Math.ceil(SEARCH_INDEX_UPDATE_TIMEOUT_MS / SEARCH_INDEX_RETRY_INTERVAL_MS);
  for (let i = 0; i < maxAttempts; i++) {
    await searchInput.fill(term);
    await searchInput.press('Enter');
    try {
      await itemContentLocator.waitFor({ state: 'visible', timeout: SEARCH_INDEX_RETRY_INTERVAL_MS });
      return;
    } catch (e) {
      // Timeout waiting for results — search index may not have updated yet; retry
      if (i < maxAttempts - 1) continue;
      throw e;
    }
  }
  throw new Error(
    `Search results for "${term}" not found within ${SEARCH_INDEX_UPDATE_TIMEOUT_MS}ms. ` +
    `Search index may not have updated yet.`,
  );
}

/**
 * Opens the test page in a fresh browser context with the given colorScheme, waits for
 * #rendered to be visible, then runs the callback. The context is always closed when done.
 */
async function withViewPage<T>(
  browser: Browser,
  colorScheme: 'light' | 'dark',
  fn: (page: Page) => Promise<T>,
): Promise<T> {
  const ctx = await browser.newContext({ colorScheme });
  const page = await ctx.newPage();
  try {
    await page.goto(`/${TEST_PAGE_NAME}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
    return await fn(page);
  } finally {
    await ctx.close();
  }
}

/** Reads a CSS custom property value from :root. */
async function getCssVar(page: Page, varName: string): Promise<string> {
  return page.evaluate(
    (name: string) => getComputedStyle(document.documentElement).getPropertyValue(name).trim(),
    varName,
  );
}

test.describe('Dark Mode E2E Tests', () => {
  test.setTimeout(90000);

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    try {
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
    } finally {
      await ctx.close();
    }
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
      await withViewPage(browser, 'light', async (page) => {
        expect(await getCssVar(page, '--color-surface-primary')).toBe(LIGHT_SURFACE_PRIMARY);
      });
    });

    test('should apply light text-primary token to :root', async ({ browser }) => {
      await withViewPage(browser, 'light', async (page) => {
        expect(await getCssVar(page, '--color-text-primary')).toBe(LIGHT_TEXT_PRIMARY);
      });
    });

    test('should use light surface token as body background color', async ({ browser }) => {
      await withViewPage(browser, 'light', async (page) => {
        // Light surface-primary #ffffff = rgb(255, 255, 255)
        await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(255, 255, 255)');
      });
    });

    test('should apply light surface-sunken token to navigation bar', async ({ browser }) => {
      await withViewPage(browser, 'light', async (page) => {
        // Light surface-sunken #f8f9fa = rgb(248, 249, 250)
        await expect(page.locator('div.pure-menu-horizontal')).toHaveCSS('background-color', 'rgb(248, 249, 250)');
      });
    });
  });

  test.describe('design token application in dark mode', () => {
    test('should apply dark surface-primary token to :root', async ({ browser }) => {
      await withViewPage(browser, 'dark', async (page) => {
        expect(await getCssVar(page, '--color-surface-primary')).toBe(DARK_SURFACE_PRIMARY);
      });
    });

    test('should apply dark text-primary token to :root', async ({ browser }) => {
      await withViewPage(browser, 'dark', async (page) => {
        expect(await getCssVar(page, '--color-text-primary')).toBe(DARK_TEXT_PRIMARY);
      });
    });

    test('should use dark surface token as body background color', async ({ browser }) => {
      await withViewPage(browser, 'dark', async (page) => {
        // Dark surface-primary #1e1e1e = rgb(30, 30, 30)
        await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(30, 30, 30)');
      });
    });

    test('should apply dark surface-sunken token to navigation bar', async ({ browser }) => {
      await withViewPage(browser, 'dark', async (page) => {
        // Dark surface-sunken #141414 = rgb(20, 20, 20)
        await expect(page.locator('div.pure-menu-horizontal')).toHaveCSS('background-color', 'rgb(20, 20, 20)');
      });
    });
  });

  test.describe('design token contrast between light and dark modes', () => {
    test('should use different surface-primary values in light and dark mode', async ({ browser }) => {
      const lightToken = await withViewPage(browser, 'light', (page) => getCssVar(page, '--color-surface-primary'));
      const darkToken = await withViewPage(browser, 'dark', (page) => getCssVar(page, '--color-surface-primary'));
      expect(lightToken).toBe(LIGHT_SURFACE_PRIMARY);
      expect(darkToken).toBe(DARK_SURFACE_PRIMARY);
      expect(lightToken).not.toBe(darkToken);
    });

    test('should use different text-primary values in light and dark mode', async ({ browser }) => {
      const lightToken = await withViewPage(browser, 'light', (page) => getCssVar(page, '--color-text-primary'));
      const darkToken = await withViewPage(browser, 'dark', (page) => getCssVar(page, '--color-text-primary'));
      expect(lightToken).toBe(LIGHT_TEXT_PRIMARY);
      expect(darkToken).toBe(DARK_TEXT_PRIMARY);
      expect(lightToken).not.toBe(darkToken);
    });

    test('should use different surface-sunken values in light and dark mode', async ({ browser }) => {
      const lightToken = await withViewPage(browser, 'light', (page) => getCssVar(page, '--color-surface-sunken'));
      const darkToken = await withViewPage(browser, 'dark', (page) => getCssVar(page, '--color-surface-sunken'));
      expect(lightToken).toBe(LIGHT_SURFACE_SUNKEN);
      expect(darkToken).toBe(DARK_SURFACE_SUNKEN);
      expect(lightToken).not.toBe(darkToken);
    });
  });

  test.describe('dark mode persistence', () => {
    test('should maintain dark mode tokens after page reload', async ({ browser }) => {
      await withViewPage(browser, 'dark', async (page) => {
        await page.reload();
        await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
        expect(await getCssVar(page, '--color-surface-primary')).toBe(DARK_SURFACE_PRIMARY);
      });
    });

    test('should maintain dark mode tokens when navigating from view to edit page', async ({ browser }) => {
      await withViewPage(browser, 'dark', async (page) => {
        const viewToken = await getCssVar(page, '--color-surface-primary');

        await page.goto(`/${TEST_PAGE_NAME}/edit`);
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        const editToken = await getCssVar(page, '--color-surface-primary');

        expect(viewToken).toBe(DARK_SURFACE_PRIMARY);
        expect(editToken).toBe(DARK_SURFACE_PRIMARY);
      });
    });
  });

  test.describe('dialog backgrounds in dark mode', () => {
    test('should render frontmatter dialog with dark elevated surface background in dark mode', async ({ browser }) => {
      await withViewPage(browser, 'dark', async (page) => {
        await page.locator('.tools-menu').hover();
        const editFrontmatterBtn = page.locator('#editFrontmatter');
        await expect(editFrontmatterBtn).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });
        await editFrontmatterBtn.click();

        const dialog = page.locator('frontmatter-editor-dialog').locator('dialog');
        await expect(dialog).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Verify dialog uses dark elevated surface token (CSS variable inherited from :root)
        // Dark surface-elevated #2d2d2d = rgb(45, 45, 45)
        await expect(dialog).toHaveCSS('background-color', 'rgb(45, 45, 45)');
      });
    });

    test('should render frontmatter dialog with light elevated surface background in light mode', async ({ browser }) => {
      await withViewPage(browser, 'light', async (page) => {
        await page.locator('.tools-menu').hover();
        const editFrontmatterBtn = page.locator('#editFrontmatter');
        await expect(editFrontmatterBtn).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });
        await editFrontmatterBtn.click();

        const dialog = page.locator('frontmatter-editor-dialog').locator('dialog');
        await expect(dialog).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Light surface-elevated #ffffff = rgb(255, 255, 255)
        await expect(dialog).toHaveCSS('background-color', 'rgb(255, 255, 255)');
      });
    });
  });

  test.describe('item_content contrast in search results', () => {
    test('should use dark elevated surface for item_content background in dark mode', async ({ browser }) => {
      await withViewPage(browser, 'dark', async (page) => {
        const searchComponent = page.locator('wiki-search#site-search');
        const searchInput = searchComponent.locator('input[type="search"]');
        await expect(searchInput).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        // wiki-search-results host has zero height (position:fixed popover), so wait for
        // .item_content directly (which IS visible inside the popover). Retry the
        // search to handle async index latency after page creation.
        const itemContent = searchComponent.locator('wiki-search-results .item_content').first();
        await searchUntilResultsVisible(searchInput, itemContent, 'e2edarkmode_unique_xyz');

        // Check computed background color of item_content (uses --color-surface-elevated)
        // Dark surface-elevated #2d2d2d = rgb(45, 45, 45)
        await expect(itemContent).toHaveCSS('background-color', 'rgb(45, 45, 45)');
      });
    });

    test('should use light elevated surface for item_content background in light mode', async ({ browser }) => {
      await withViewPage(browser, 'light', async (page) => {
        const searchComponent = page.locator('wiki-search#site-search');
        const searchInput = searchComponent.locator('input[type="search"]');
        await expect(searchInput).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        const itemContent = searchComponent.locator('wiki-search-results .item_content').first();
        await searchUntilResultsVisible(searchInput, itemContent, 'e2edarkmode_unique_xyz');

        // Light surface-elevated #ffffff = rgb(255, 255, 255)
        await expect(itemContent).toHaveCSS('background-color', 'rgb(255, 255, 255)');
      });
    });

    test('should show different item_content contrast between light and dark mode', async ({ browser }) => {
      const lightBg = await withViewPage(browser, 'light', async (page) => {
        const searchComponent = page.locator('wiki-search#site-search');
        const searchInput = searchComponent.locator('input[type="search"]');
        const itemContent = searchComponent.locator('wiki-search-results .item_content').first();
        await searchUntilResultsVisible(searchInput, itemContent, 'e2edarkmode_unique_xyz');
        return itemContent.evaluate((el) => getComputedStyle(el).backgroundColor);
      });

      const darkBg = await withViewPage(browser, 'dark', async (page) => {
        const searchComponent = page.locator('wiki-search#site-search');
        const searchInput = searchComponent.locator('input[type="search"]');
        const itemContent = searchComponent.locator('wiki-search-results .item_content').first();
        await searchUntilResultsVisible(searchInput, itemContent, 'e2edarkmode_unique_xyz');
        return itemContent.evaluate((el) => getComputedStyle(el).backgroundColor);
      });

      expect(lightBg).not.toBe(darkBg);
      expect(darkBg).toBe('rgb(45, 45, 45)');
      expect(lightBg).toBe('rgb(255, 255, 255)');
    });
  });

  test.describe('dark mode surface-elevated token', () => {
    test('should apply different surface-elevated token values in light and dark mode', async ({ browser }) => {
      const lightToken = await withViewPage(browser, 'light', (page) => getCssVar(page, '--color-surface-elevated'));
      const darkToken = await withViewPage(browser, 'dark', (page) => getCssVar(page, '--color-surface-elevated'));
      expect(darkToken).toBe(DARK_SURFACE_ELEVATED);
      expect(lightToken).not.toBe(darkToken);
    });
  });
});
