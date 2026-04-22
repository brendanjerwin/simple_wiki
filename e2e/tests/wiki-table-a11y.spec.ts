import { test, expect, type Page } from '@playwright/test';

// E2E tests for wiki-table column sort keyboard accessibility.
// These tests verify the improvements introduced in PR #942 — column header
// spans converted to keyboard-accessible buttons with aria-labels and scope
// attributes — are working and will catch regressions.

const TEST_PAGE = 'e2e-wiki-table-sort-a11y-test';

const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;

// Sort indicator unicode characters (must match wiki-table.ts _getSortIndicator)
const SORT_ASCENDING = '\u2191';  // ↑  — sorted ascending

const TEST_CONTENT = `+++
identifier = "${TEST_PAGE}"
title = "Wiki Table Sort Accessibility E2E Test"
+++

# Sort Accessibility Test

| Name | Category | Score |
|------|----------|-------|
| Alpha | Fruit | 10 |
| Beta | Vegetable | 20 |
| Gamma | Fruit | 30 |
| Delta | Vegetable | 40 |
| Epsilon | Fruit | 50 |
`;

function tableRows(page: Page) {
  return page.locator('wiki-table').locator('.table-wrapper tbody tr');
}

test.describe('Wiki Table Sort Keyboard Accessibility', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

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
      console.warn('Wiki table sort a11y E2E test cleanup failed:', e);
    } finally {
      await ctx.close();
    }
  });

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
  // Keyboard Sort
  // ═══════════════════════════════════════════════════════════════════════════

  test('should sort ascending when pressing Enter on a sort button', async ({ page }) => {
    const sortButton = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortButton.focus();
    await expect(sortButton).toBeFocused();
    await page.keyboard.press('Enter');

    await expect(sortButton).toContainText(SORT_ASCENDING);
    await expect(tableRows(page).first().locator('td').first()).toContainText('Alpha');
    await expect(tableRows(page).last().locator('td').first()).toContainText('Gamma');
  });

  test('should sort ascending when pressing Space on a sort button', async ({ page }) => {
    const sortButton = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortButton.focus();
    await expect(sortButton).toBeFocused();
    await page.keyboard.press('Space');

    await expect(sortButton).toContainText(SORT_ASCENDING);
    await expect(tableRows(page).first().locator('td').first()).toContainText('Alpha');
    await expect(tableRows(page).last().locator('td').first()).toContainText('Gamma');
  });

  // ═══════════════════════════════════════════════════════════════════════════
  // ARIA Attributes on Sort Buttons
  // ═══════════════════════════════════════════════════════════════════════════

  test('should have aria-label containing "Name" on the Name column sort button', async ({ page }) => {
    const ariaLabel = await page.evaluate(() => {
      const wikiTable = document.querySelector('wiki-table');
      const buttons = wikiTable?.shadowRoot?.querySelectorAll('.sort-arrows');
      return buttons?.[0]?.getAttribute('aria-label') ?? '';
    });

    expect(ariaLabel).toContain('Name');
  });

  test('should have aria-label containing "Category" on the Category column sort button', async ({ page }) => {
    const ariaLabel = await page.evaluate(() => {
      const wikiTable = document.querySelector('wiki-table');
      const buttons = wikiTable?.shadowRoot?.querySelectorAll('.sort-arrows');
      return buttons?.[1]?.getAttribute('aria-label') ?? '';
    });

    expect(ariaLabel).toContain('Category');
  });

  test('should have aria-label containing "Score" on the Score column sort button', async ({ page }) => {
    const ariaLabel = await page.evaluate(() => {
      const wikiTable = document.querySelector('wiki-table');
      const buttons = wikiTable?.shadowRoot?.querySelectorAll('.sort-arrows');
      return buttons?.[2]?.getAttribute('aria-label') ?? '';
    });

    expect(ariaLabel).toContain('Score');
  });

  // ═══════════════════════════════════════════════════════════════════════════
  // scope=col on th Elements
  // ═══════════════════════════════════════════════════════════════════════════

  test('should have scope="col" on all column header th elements', async ({ page }) => {
    const scopeAttrs = await page.evaluate(() => {
      const wikiTable = document.querySelector('wiki-table');
      const headers = wikiTable?.shadowRoot?.querySelectorAll('thead th');
      return Array.from(headers ?? []).map(th => th.getAttribute('scope') ?? '');
    });

    expect(scopeAttrs.length).toBeGreaterThan(0);
    for (const scope of scopeAttrs) {
      expect(scope).toBe('col');
    }
  });

  // ═══════════════════════════════════════════════════════════════════════════
  // Regression: Mouse Click Still Sorts
  // ═══════════════════════════════════════════════════════════════════════════

  test('should sort ascending when clicking the sort button with a mouse', async ({ page }) => {
    const sortButton = page.locator('wiki-table').locator('.sort-arrows').first();
    await sortButton.click();

    await expect(sortButton).toContainText(SORT_ASCENDING);
    await expect(tableRows(page).first().locator('td').first()).toContainText('Alpha');
    await expect(tableRows(page).last().locator('td').first()).toContainText('Gamma');
  });

  test('should cycle through sort states on repeated mouse clicks', async ({ page }) => {
    const SORT_NEUTRAL = '\u21C5';
    const SORT_DESCENDING = '\u2193';

    const sortButton = page.locator('wiki-table').locator('.sort-arrows').first();

    await sortButton.click(); // → ascending
    await expect(sortButton).toContainText(SORT_ASCENDING);

    await sortButton.click(); // → descending
    await expect(sortButton).toContainText(SORT_DESCENDING);

    await sortButton.click(); // → none
    await expect(sortButton).toContainText(SORT_NEUTRAL);
  });
});
