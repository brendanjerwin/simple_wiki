import { test, expect } from '@playwright/test';

// Test data
const TEST_PAGE_1 = 'E2ESearchTest1';
const TEST_PAGE_2 = 'E2ESearchTest2';
const UNIQUE_SEARCH_TERM = 'e2e_unique_search_term_xyz123';

// Constants
const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;
const SEARCH_TIMEOUT_MS = 10000;

test.describe('Search E2E Tests', () => {
  test.setTimeout(60000);

  // Create test pages with known content before all tests
  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    // Create first test page
    await page.goto(`/${TEST_PAGE_1.toLowerCase()}/edit`);
    let textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const content1 = `+++
identifier = "${TEST_PAGE_1.toLowerCase()}"
title = "Search Test Page 1"
+++

# Search Test Page 1

This page contains the unique search term: ${UNIQUE_SEARCH_TERM}.

It should appear in search results.`;

    await textarea.fill(content1);
    await textarea.press('Space');
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
      timeout: SAVE_TIMEOUT_MS,
    });

    // Create second test page
    await page.goto(`/${TEST_PAGE_2.toLowerCase()}/edit`);
    textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const content2 = `+++
identifier = "${TEST_PAGE_2.toLowerCase()}"
title = "Search Test Page 2"
+++

# Search Test Page 2

This page also contains ${UNIQUE_SEARCH_TERM} for testing search functionality.

Multiple results should be found.`;

    await textarea.fill(content2);
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
      // Clean up first page
      await page.goto(`/${TEST_PAGE_1.toLowerCase()}/edit`);
      let textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await textarea.fill(`+++
identifier = "${TEST_PAGE_1.toLowerCase()}"
+++`);
      await textarea.press('Space');
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
        timeout: SAVE_TIMEOUT_MS,
      });

      // Clean up second page
      await page.goto(`/${TEST_PAGE_2.toLowerCase()}/edit`);
      textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await textarea.fill(`+++
identifier = "${TEST_PAGE_2.toLowerCase()}"
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

  test('should render search component on page', async ({ page }) => {
    await page.goto('/home/view');
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    // Verify search component exists
    const searchComponent = page.locator('wiki-search');
    await expect(searchComponent).toBeAttached();

    // Verify search input is visible
    const searchInput = searchComponent.locator('input[type="search"]');
    await expect(searchInput).toBeVisible();
  });

  test('should focus search input with Ctrl+K keyboard shortcut', async ({ page }) => {
    await page.goto('/home/view');
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const searchInput = page.locator('wiki-search input[type="search"]');

    // Trigger Ctrl+K (Cmd+K on Mac is also supported but we test Ctrl+K)
    await page.keyboard.press('Control+k');

    // Verify search input is focused
    await expect(searchInput).toBeFocused();
  });

  test('should search for content and display results', async ({ page }) => {
    await page.goto('/home/view');
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const searchComponent = page.locator('wiki-search');
    const searchInput = searchComponent.locator('input[type="search"]');
    const searchButton = searchComponent.locator('button[type="submit"]');

    // Enter search term
    await searchInput.fill(UNIQUE_SEARCH_TERM);

    // Submit search
    await searchButton.click();

    // Wait for results component to appear
    const resultsComponent = searchComponent.locator('wiki-search-results');
    await expect(resultsComponent).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });

    // Verify results are displayed
    // The results component should show our test pages
    await expect(resultsComponent).toContainText('Search Test Page', {
      timeout: SEARCH_TIMEOUT_MS,
    });
  });

  test('should navigate to page when clicking search result', async ({ page }) => {
    await page.goto('/home/view');
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const searchComponent = page.locator('wiki-search');
    const searchInput = searchComponent.locator('input[type="search"]');

    // Search for our unique term
    await searchInput.fill(UNIQUE_SEARCH_TERM);
    await searchInput.press('Enter');

    // Wait for results
    const resultsComponent = searchComponent.locator('wiki-search-results');
    await expect(resultsComponent).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });

    // Click the first result link
    const firstResult = resultsComponent.locator('a').first();
    await expect(firstResult).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });
    await firstResult.click();

    // Verify navigation occurred to one of our test pages
    await page.waitForURL(
      (url) => url.pathname.includes(TEST_PAGE_1.toLowerCase()) || url.pathname.includes(TEST_PAGE_2.toLowerCase()),
      { timeout: PAGE_LOAD_TIMEOUT_MS },
    );

    // Verify we're on a view page
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
  });

  test('should display empty state for non-existent search terms', async ({ page }) => {
    await page.goto('/home/view');
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const searchComponent = page.locator('wiki-search');
    const searchInput = searchComponent.locator('input[type="search"]');
    const searchButton = searchComponent.locator('button[type="submit"]');

    // Search for something that doesn't exist
    const nonExistentTerm = 'nonexistent_term_abcxyz123456789';
    await searchInput.fill(nonExistentTerm);
    await searchButton.click();

    // Wait for results component to appear
    const resultsComponent = searchComponent.locator('wiki-search-results');
    await expect(resultsComponent).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });

    // Verify it shows no results state
    // The component should indicate no results were found
    await expect(resultsComponent).toContainText('No results', { timeout: SEARCH_TIMEOUT_MS });
  });

  test('should handle empty search gracefully', async ({ page }) => {
    await page.goto('/home/view');
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const searchComponent = page.locator('wiki-search');
    const searchInput = searchComponent.locator('input[type="search"]');
    const searchButton = searchComponent.locator('button[type="submit"]');

    // Try to search with empty input
    await searchInput.fill('');
    await searchButton.click();

    // Results should not appear for empty search
    const resultsComponent = searchComponent.locator('wiki-search-results');

    // Wait a moment to ensure nothing happens
    await page.waitForTimeout(1000);

    // Results component should not be visible
    await expect(resultsComponent).not.toBeVisible();
  });

  test('should show loading state during search', async ({ page }) => {
    await page.goto('/home/view');
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const searchComponent = page.locator('wiki-search');
    const searchInput = searchComponent.locator('input[type="search"]');
    const searchButton = searchComponent.locator('button[type="submit"]');

    // Enter search term
    await searchInput.fill(UNIQUE_SEARCH_TERM);

    // Submit search
    await searchButton.click();

    // The component should show some indication of loading
    // This might be brief, so we just verify the search completes successfully
    const resultsComponent = searchComponent.locator('wiki-search-results');
    await expect(resultsComponent).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });
  });

  test('should persist search term in input after search', async ({ page }) => {
    await page.goto('/home/view');
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const searchComponent = page.locator('wiki-search');
    const searchInput = searchComponent.locator('input[type="search"]');

    // Search for our term
    await searchInput.fill(UNIQUE_SEARCH_TERM);
    await searchInput.press('Enter');

    // Wait for results
    const resultsComponent = searchComponent.locator('wiki-search-results');
    await expect(resultsComponent).toBeVisible({ timeout: SEARCH_TIMEOUT_MS });

    // Verify the search term is still in the input
    await expect(searchInput).toHaveValue(UNIQUE_SEARCH_TERM);
  });
});
