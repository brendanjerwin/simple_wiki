import { test, expect, type Page } from '@playwright/test';

// Test data
const TEST_PAGE = 'e2e-table-filter-sort-test';

// Constants
const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;

// Sort indicator unicode characters (must match wiki-table.ts _getSortIndicator)
const SORT_NEUTRAL = '\u21C5';    // ⇕  — no active sort
const SORT_ASCENDING = '\u2191';  // ↑  — sorted ascending
const SORT_DESCENDING = '\u2193'; // ↓  — sorted descending

const TEST_CONTENT = `+++
identifier = "${TEST_PAGE}"
title = "Table Filter Sort E2E Test"
+++

# Table Test

| Name | Category | Score |
|------|----------|-------|
| Alpha | Fruit | 10 |
| Beta | Vegetable | 20 |
| Gamma | Fruit | 30 |
| Delta | Vegetable | 40 |
| Epsilon | Fruit | 50 |
`;

// Returns a locator for the shadow-DOM table rows rendered by wiki-table.
// Uses .table-wrapper to avoid matching the hidden slotted source <table>.
function tableRows(page: Page) {
  return page.locator('wiki-table').locator('.table-wrapper tbody tr');
}

test.describe('Table Filtering and Sorting E2E Tests', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  // Create the test page once before all tests.
  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    await page.goto(`/${TEST_PAGE}/edit`);
    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await textarea.fill(TEST_CONTENT);
    await textarea.press('Space');
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
      timeout: SAVE_TIMEOUT_MS,
    });

    await ctx.close();
  });

  // Remove the test page content after all tests.
  test.afterAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    try {
      await page.goto(`/${TEST_PAGE}/edit`);
      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await textarea.fill(`+++\nidentifier = "${TEST_PAGE}"\n+++`);
      await textarea.press('Space');
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
        timeout: SAVE_TIMEOUT_MS,
      });
    } catch (e) {
      console.warn('Table filter/sort E2E test cleanup failed:', e);
    } finally {
      await ctx.close();
    }
  });

  // Before each test: clear persisted table state from localStorage and navigate
  // to a clean view of the test page so each test starts from a known baseline.
  test.beforeEach(async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await page.evaluate(() => {
      const keys = Object.keys(localStorage).filter(k => k.startsWith('wiki-table-state:'));
      for (const k of keys) localStorage.removeItem(k);
    });
    await page.reload();
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    await expect(page.locator('wiki-table').locator('.table-wrapper')).toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });
  });

  // ═══════════════════════════════════════════════════════════════════════════
  // Sorting
  // ═══════════════════════════════════════════════════════════════════════════

  test('should show neutral sort indicator initially', async ({ page }) => {
    const sortArrows = page.locator('wiki-table').locator('.sort-arrows').first();
    await expect(sortArrows).toContainText(SORT_NEUTRAL);
  });

  test('should sort ascending on first sort-arrow click', async ({ page }) => {
    const sortArrows = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortArrows.click();

    await expect(sortArrows).toContainText(SORT_ASCENDING);
    // Alphabetically first and last names after ascending sort
    await expect(tableRows(page).first().locator('td').first()).toContainText('Alpha');
    await expect(tableRows(page).last().locator('td').first()).toContainText('Gamma');
  });

  test('should sort descending on second sort-arrow click', async ({ page }) => {
    const sortArrows = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortArrows.click(); // → ascending
    await sortArrows.click(); // → descending

    await expect(sortArrows).toContainText(SORT_DESCENDING);
    await expect(tableRows(page).first().locator('td').first()).toContainText('Gamma');
    await expect(tableRows(page).last().locator('td').first()).toContainText('Alpha');
  });

  test('should remove sort on third sort-arrow click', async ({ page }) => {
    const sortArrows = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortArrows.click(); // → ascending
    await sortArrows.click(); // → descending
    await sortArrows.click(); // → none

    await expect(sortArrows).toContainText(SORT_NEUTRAL);
  });

  test('should display sorted CSS class and aria-sort on the active column', async ({ page }) => {
    const firstTh = page.locator('wiki-table').locator('.table-wrapper th').first();
    await expect(firstTh).not.toHaveClass(/sorted/);
    await expect(firstTh).toHaveAttribute('aria-sort', 'none');

    const sortArrows = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortArrows.click(); // → ascending

    await expect(firstTh).toHaveClass(/sorted/);
    await expect(firstTh).toHaveAttribute('aria-sort', 'ascending');

    await sortArrows.click(); // → descending
    await expect(firstTh).toHaveAttribute('aria-sort', 'descending');
  });

  // ═══════════════════════════════════════════════════════════════════════════
  // Filtering
  // ═══════════════════════════════════════════════════════════════════════════

  test('should open filter popover when clicking a column header', async ({ page }) => {
    const categoryHeader = page.locator('wiki-table').locator('.header-main').nth(1);
    await categoryHeader.click();

    const popover = page.locator('wiki-table').locator('table-filter-popover');
    await expect(popover.locator('.popover-header')).toBeVisible();
    await expect(popover.locator('.checkbox-list')).toBeVisible();
  });

  test('should filter rows by unchecking a value in the checkbox filter', async ({ page }) => {
    const categoryHeader = page.locator('wiki-table').locator('.header-main').nth(1);
    await categoryHeader.click();

    const popover = page.locator('wiki-table').locator('table-filter-popover');
    const vegetableCheckbox = popover
      .locator('.checkbox-item')
      .filter({ hasText: 'Vegetable' })
      .locator('input[type="checkbox"]');
    await vegetableCheckbox.uncheck();
    await popover.locator('[aria-label="Apply"]').click();

    // Status bar: "3 of 5 rows"
    await expect(page.locator('wiki-table').locator('.row-count')).toContainText('3 of 5 rows');
    await expect(tableRows(page)).toHaveCount(3);
  });

  test('should show filter dot indicator on a filtered column', async ({ page }) => {
    const categoryHeader = page.locator('wiki-table').locator('.header-main').nth(1);
    await categoryHeader.click();

    const popover = page.locator('wiki-table').locator('table-filter-popover');
    const vegetableCheckbox = popover
      .locator('.checkbox-item')
      .filter({ hasText: 'Vegetable' })
      .locator('input[type="checkbox"]');
    await vegetableCheckbox.uncheck();
    await popover.locator('[aria-label="Apply"]').click();

    const filterDot = page.locator('wiki-table').locator('.header-main').nth(1).locator('.filter-dot');
    await expect(filterDot).toBeVisible();
  });

  test('should restore all rows after clearing the active filter', async ({ page }) => {
    // Apply a filter first
    const categoryHeader = page.locator('wiki-table').locator('.header-main').nth(1);
    await categoryHeader.click();

    const popover = page.locator('wiki-table').locator('table-filter-popover');
    const vegetableCheckbox = popover
      .locator('.checkbox-item')
      .filter({ hasText: 'Vegetable' })
      .locator('input[type="checkbox"]');
    await vegetableCheckbox.uncheck();
    await popover.locator('[aria-label="Apply"]').click();

    await expect(tableRows(page)).toHaveCount(3);

    // Clear via the status-bar "clear" pill
    await page.locator('wiki-table').locator('.tag-filter-clear').click();

    await expect(page.locator('wiki-table').locator('.row-count')).toContainText('5 rows');
    await expect(tableRows(page)).toHaveCount(5);
  });

  test('should apply filters on multiple columns simultaneously', async ({ page }) => {
    // Locator for the (single active) popover; re-evaluated lazily on each access.
    const popover = page.locator('wiki-table').locator('table-filter-popover');

    // Step 1: filter Category — exclude Vegetable → 3 Fruit rows remain
    const categoryHeader = page.locator('wiki-table').locator('.header-main').nth(1);
    await categoryHeader.click();

    const vegetableCheckbox = popover
      .locator('.checkbox-item')
      .filter({ hasText: 'Vegetable' })
      .locator('input[type="checkbox"]');
    await vegetableCheckbox.uncheck();
    await popover.locator('[aria-label="Apply"]').click();

    await expect(tableRows(page)).toHaveCount(3);

    // Step 2: filter Name — exclude Gamma and Epsilon → only Alpha remains
    const nameHeader = page.locator('wiki-table').locator('.header-main').first();
    await nameHeader.click();

    await expect(popover.locator('.checkbox-list')).toBeVisible();

    const gammaCheckbox = popover
      .locator('.checkbox-item')
      .filter({ hasText: 'Gamma' })
      .locator('input[type="checkbox"]');
    await gammaCheckbox.uncheck();

    const epsilonCheckbox = popover
      .locator('.checkbox-item')
      .filter({ hasText: 'Epsilon' })
      .locator('input[type="checkbox"]');
    await epsilonCheckbox.uncheck();

    await popover.locator('[aria-label="Apply"]').click();

    // Both filters active: only Alpha (Fruit) survives
    await expect(tableRows(page)).toHaveCount(1);
    await expect(page.locator('wiki-table').locator('.row-count')).toContainText('1 of 5 rows');
  });

  // ═══════════════════════════════════════════════════════════════════════════
  // State Persistence
  // ═══════════════════════════════════════════════════════════════════════════

  test('should persist sort after navigating away and returning', async ({ page }) => {
    const sortArrows = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortArrows.click();
    await expect(sortArrows).toContainText(SORT_ASCENDING);

    // Navigate away (to the edit view) then back to the rendered view
    await page.goto(`/${TEST_PAGE}/edit`);
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    await expect(page.locator('wiki-table').locator('.table-wrapper')).toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Ascending sort should be restored from localStorage
    const restoredSortArrows = page.locator('wiki-table').locator('.sort-arrows').first();
    await expect(restoredSortArrows).toContainText(SORT_ASCENDING);
  });

  test('should persist filter after navigating away and returning', async ({ page }) => {
    // Apply a filter
    const categoryHeader = page.locator('wiki-table').locator('.header-main').nth(1);
    await categoryHeader.click();

    const popover = page.locator('wiki-table').locator('table-filter-popover');
    const vegetableCheckbox = popover
      .locator('.checkbox-item')
      .filter({ hasText: 'Vegetable' })
      .locator('input[type="checkbox"]');
    await vegetableCheckbox.uncheck();
    await popover.locator('[aria-label="Apply"]').click();

    await expect(tableRows(page)).toHaveCount(3);

    // Navigate away then back
    await page.goto(`/${TEST_PAGE}/edit`);
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    await expect(page.locator('wiki-table').locator('.table-wrapper')).toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Filter should be restored — only 3 Fruit rows visible
    await expect(tableRows(page)).toHaveCount(3);
    await expect(page.locator('wiki-table').locator('.row-count')).toContainText('3 of 5 rows');
  });

  // ═══════════════════════════════════════════════════════════════════════════
  // Keyboard Accessibility
  // ═══════════════════════════════════════════════════════════════════════════

  test('should allow keyboard activation of sort and OK in filter popover', async ({ page }) => {
    const categoryHeader = page.locator('wiki-table').locator('.header-main').nth(1);
    await categoryHeader.click();

    const popover = page.locator('wiki-table').locator('table-filter-popover');
    await expect(popover.locator('.popover-header')).toBeVisible();

    // Focus and activate the "Sort ascending" pill via keyboard
    const sortAscButton = popover.locator('[aria-label="Sort ascending"]');
    await sortAscButton.focus();
    await expect(sortAscButton).toBeFocused();
    await page.keyboard.press('Enter');

    // Focus and activate "Apply" (OK) via keyboard
    const okButton = popover.locator('[aria-label="Apply"]');
    await okButton.focus();
    await expect(okButton).toBeFocused();
    await page.keyboard.press('Enter');

    // Popover closes and sort is applied on the Category column (index 1)
    await expect(popover.locator('.popover-header')).not.toBeVisible();
    const sortArrows = page.locator('wiki-table').locator('.sort-arrows').nth(1);
    await expect(sortArrows).toContainText(SORT_ASCENDING);
  });

  test('should allow keyboard activation of Cancel in filter popover', async ({ page }) => {
    const categoryHeader = page.locator('wiki-table').locator('.header-main').nth(1);
    await categoryHeader.click();

    const popover = page.locator('wiki-table').locator('table-filter-popover');
    await expect(popover.locator('.popover-header')).toBeVisible();

    // Focus and press Enter on Cancel — popover should close without changes
    const cancelButton = popover.locator('[aria-label="Cancel"]');
    await cancelButton.focus();
    await expect(cancelButton).toBeFocused();
    await page.keyboard.press('Enter');

    await expect(popover.locator('.popover-header')).not.toBeVisible();
    // No filter was applied — all 5 rows remain
    await expect(tableRows(page)).toHaveCount(5);
  });
});
