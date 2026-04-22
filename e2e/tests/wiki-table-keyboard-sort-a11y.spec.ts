import { test, expect, type Page } from '@playwright/test';

// Test data
const TEST_PAGE = 'e2e-wiki-table-keyboard-sort-a11y-test';

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
title = "Wiki Table Keyboard Sort A11y E2E Test"
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

test.describe('Wiki Table Keyboard Sort Accessibility E2E Tests', () => {
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
      console.warn('Wiki table keyboard sort a11y E2E test cleanup failed:', e);
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
  // WCAG Structural Attributes
  // ═══════════════════════════════════════════════════════════════════════════

  test('should have scope="col" on all th header elements', async ({ page }) => {
    const thElements = page.locator('wiki-table').locator('.table-wrapper th');
    const count = await thElements.count();
    expect(count).toBeGreaterThan(0);

    for (let i = 0; i < count; i++) {
      await expect(thElements.nth(i)).toHaveAttribute('scope', 'col');
    }
  });

  test('should have aria-label containing column name on each sort button', async ({ page }) => {
    const nameSortButton = page.locator('wiki-table').locator('.sort-arrows').nth(0);
    await expect(nameSortButton).toHaveAttribute('aria-label', 'Sort by Name');

    const categorySortButton = page.locator('wiki-table').locator('.sort-arrows').nth(1);
    await expect(categorySortButton).toHaveAttribute('aria-label', 'Sort by Category');

    const scoreSortButton = page.locator('wiki-table').locator('.sort-arrows').nth(2);
    await expect(scoreSortButton).toHaveAttribute('aria-label', 'Sort by Score');
  });

  // ═══════════════════════════════════════════════════════════════════════════
  // Keyboard Sort
  // ═══════════════════════════════════════════════════════════════════════════

  test('should be focusable — sort button receives focus via programmatic focus', async ({ page }) => {
    const sortButton = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortButton.focus();
    await expect(sortButton).toBeFocused();
  });

  test('should sort ascending when pressing Enter on focused sort button', async ({ page }) => {
    const sortButton = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortButton.focus();
    await expect(sortButton).toBeFocused();

    await page.keyboard.press('Enter');

    await expect(sortButton).toContainText(SORT_ASCENDING);
    await expect(tableRows(page).first().locator('td').first()).toContainText('Alpha');
    await expect(tableRows(page).last().locator('td').first()).toContainText('Gamma');
  });

  test('should sort ascending when pressing Space on focused sort button', async ({ page }) => {
    const sortButton = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortButton.focus();
    await expect(sortButton).toBeFocused();

    await page.keyboard.press('Space');

    await expect(sortButton).toContainText(SORT_ASCENDING);
    await expect(tableRows(page).first().locator('td').first()).toContainText('Alpha');
  });

  test('should cycle through ascending → descending → neutral with repeated Enter presses', async ({ page }) => {
    const sortButton = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortButton.focus();

    await page.keyboard.press('Enter');
    await expect(sortButton).toContainText(SORT_ASCENDING);

    await page.keyboard.press('Enter');
    await expect(sortButton).toContainText(SORT_DESCENDING);
    await expect(tableRows(page).first().locator('td').first()).toContainText('Gamma');
    await expect(tableRows(page).last().locator('td').first()).toContainText('Alpha');

    await page.keyboard.press('Enter');
    await expect(sortButton).toContainText(SORT_NEUTRAL);
  });

  test('should be reachable via Tab from the adjacent header button', async ({ page }) => {
    // Focus the header-main button (the clickable column label) and Tab once
    // to reach the sort-arrows button in the same header cell.
    const headerMainButton = page.locator('wiki-table').locator('.header-main').first();
    await headerMainButton.focus();
    await page.keyboard.press('Tab');

    const sortButton = page.locator('wiki-table').locator('.sort-arrows').first();
    await expect(sortButton).toBeFocused();
  });

  // ═══════════════════════════════════════════════════════════════════════════
  // Regression: Mouse Click Sort Still Works
  // ═══════════════════════════════════════════════════════════════════════════

  test('should sort ascending on first sort button click (no regression)', async ({ page }) => {
    const sortButton = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortButton.click();

    await expect(sortButton).toContainText(SORT_ASCENDING);
    await expect(tableRows(page).first().locator('td').first()).toContainText('Alpha');
    await expect(tableRows(page).last().locator('td').first()).toContainText('Gamma');
  });

  test('should sort descending on second sort button click (no regression)', async ({ page }) => {
    const sortButton = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortButton.click();
    await sortButton.click();

    await expect(sortButton).toContainText(SORT_DESCENDING);
    await expect(tableRows(page).first().locator('td').first()).toContainText('Gamma');
    await expect(tableRows(page).last().locator('td').first()).toContainText('Alpha');
  });

  test('should remove sort on third sort button click (no regression)', async ({ page }) => {
    const sortButton = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortButton.click();
    await sortButton.click();
    await sortButton.click();

    await expect(sortButton).toContainText(SORT_NEUTRAL);
  });
});
