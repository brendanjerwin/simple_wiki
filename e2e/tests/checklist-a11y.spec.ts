import { test, expect } from '@playwright/test';

const TEST_PAGE_NAME = 'e2echecklistaccesstest';
const TEST_LIST_NAME = 'a11y_test_list';

const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;

test.describe('Checklist Accessibility E2E Tests', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(120000);

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    await page.goto(`/${TEST_PAGE_NAME}/edit`);
    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const content = `+++
identifier = "${TEST_PAGE_NAME}"
title = "Checklist A11y Test Page"
[${TEST_LIST_NAME}]
+++

# Checklist A11y Test Page

<wiki-checklist list-name="${TEST_LIST_NAME}" page="${TEST_PAGE_NAME}"></wiki-checklist>`;

    await textarea.fill(content);
    await textarea.press('Space');
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
      timeout: SAVE_TIMEOUT_MS,
    });

    // Navigate to view page and add test items with tags
    await page.goto(`/${TEST_PAGE_NAME}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const checklist = page.locator('wiki-checklist');
    await expect(checklist.locator('.loading')).not.toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    const addInput = checklist.locator('.add-text-input');
    const addButton = checklist.locator('.add-btn');

    for (const itemText of ['First item #alpha', 'Second item #beta', 'Third item #alpha']) {
      await addInput.fill(itemText);
      await addButton.click();
      await expect(checklist.locator('.saving-indicator')).not.toBeVisible({
        timeout: SAVE_TIMEOUT_MS,
      });
    }

    await ctx.close();
  });

  test.afterAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    try {
      await page.goto(`/${TEST_PAGE_NAME}/edit`);
      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await textarea.fill(`+++\nidentifier = "${TEST_PAGE_NAME}"\n+++`);
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

  test.describe('ARIA attributes', () => {
    test('checkboxes should have aria-label matching item text', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      const firstRow = checklist.locator('.item-row').first();
      const displayText = await firstRow.locator('.item-display-text').textContent();
      const firstCheckbox = firstRow.locator('.item-checkbox');

      await expect(firstCheckbox).toHaveAttribute('aria-label', displayText?.trim() ?? '');
    });

    test('drag handles should have a descriptive aria-label mentioning arrow keys', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      const firstHandle = checklist.locator('.drag-handle').first();
      const ariaLabel = await firstHandle.getAttribute('aria-label');

      expect(ariaLabel).toBeTruthy();
      expect(ariaLabel?.toLowerCase()).toContain('arrow keys');
    });

    test('drag handles should have role=button', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      const firstHandle = checklist.locator('.drag-handle').first();
      await expect(firstHandle).toHaveAttribute('role', 'button');
    });

    test('tag filter pills should have aria-pressed=false by default', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      const tagPill = checklist.locator('.tag-pill').first();
      await expect(tagPill).toHaveAttribute('aria-pressed', 'false');
    });

    test('tag filter pills should have aria-label with filter description', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      const tagPill = checklist.locator('.tag-pill').first();
      const ariaLabel = await tagPill.getAttribute('aria-label');

      expect(ariaLabel).toBeTruthy();
      expect(ariaLabel?.toLowerCase()).toContain('filter by');
    });

    test('status live region should be present with role=status and aria-live=polite', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      // The sr-only live region should always be present in the DOM
      const liveRegion = checklist.locator('[role="status"][aria-live="polite"]');
      await expect(liveRegion).toBeAttached();
      await expect(liveRegion).toHaveAttribute('aria-atomic', 'true');
    });

    test('loading indicator should have role=status and aria-live=polite', async ({ page }) => {
      // Navigate before loading completes — the loading indicator may appear briefly
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');

      // The loading div, while present, must have correct ARIA attributes
      // It may disappear quickly; check it if visible or verify attributes on the element selector
      const loadingEl = checklist.locator('.loading');
      const count = await loadingEl.count();
      if (count > 0 && await loadingEl.isVisible()) {
        await expect(loadingEl).toHaveAttribute('role', 'status');
        await expect(loadingEl).toHaveAttribute('aria-live', 'polite');
      }

      // Regardless, wait for load to complete
      await expect(loadingEl).not.toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    });

    test('error wrapper should have role=alert when error is displayed', async ({ page }) => {
      // Intercept ChecklistService.ListItems to force an error. Per
      // ADR-0010 the web component now talks to the dedicated service,
      // not the generic Frontmatter API.
      await page.route('**/api.v1.ChecklistService/ListItems', route => {
        return route.fulfill({ status: 500, body: 'Internal Server Error' });
      });

      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      const errorWrapper = checklist.locator('.error-wrapper');

      await expect(errorWrapper).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(errorWrapper).toHaveAttribute('role', 'alert');
    });
  });

  test.describe('keyboard interactions', () => {
    test('should check a checkbox via keyboard Space', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      const firstCheckbox = checklist.locator('.item-checkbox').first();
      await firstCheckbox.focus();
      await expect(firstCheckbox).not.toBeChecked();

      await firstCheckbox.press('Space');
      await expect(firstCheckbox).toBeChecked();

      // Restore state — uncheck
      await firstCheckbox.press('Space');
      await expect(firstCheckbox).not.toBeChecked();
    });

    test('should move an item down via ArrowDown on its drag handle', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      const firstRowText = await checklist
        .locator('.item-row')
        .first()
        .locator('.item-display-text')
        .textContent();

      const firstHandle = checklist.locator('.drag-handle').first();
      await firstHandle.focus();
      await firstHandle.press('ArrowDown');

      await expect(checklist.locator('.saving-indicator')).not.toBeVisible({
        timeout: SAVE_TIMEOUT_MS,
      });

      // First item should now be second
      const newSecondRowText = await checklist
        .locator('.item-row')
        .nth(1)
        .locator('.item-display-text')
        .textContent();
      expect(newSecondRowText?.trim()).toBe(firstRowText?.trim());

      // Restore: move it back up from its new position (index 1)
      const secondHandle = checklist.locator('.drag-handle').nth(1);
      await secondHandle.focus();
      await secondHandle.press('ArrowUp');
      await expect(checklist.locator('.saving-indicator')).not.toBeVisible({
        timeout: SAVE_TIMEOUT_MS,
      });
    });

    test('should move an item up via ArrowUp on its drag handle', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      const secondRowText = await checklist
        .locator('.item-row')
        .nth(1)
        .locator('.item-display-text')
        .textContent();

      const secondHandle = checklist.locator('.drag-handle').nth(1);
      await secondHandle.focus();
      await secondHandle.press('ArrowUp');

      await expect(checklist.locator('.saving-indicator')).not.toBeVisible({
        timeout: SAVE_TIMEOUT_MS,
      });

      // Second item should now be first
      const newFirstRowText = await checklist
        .locator('.item-row')
        .first()
        .locator('.item-display-text')
        .textContent();
      expect(newFirstRowText?.trim()).toBe(secondRowText?.trim());

      // Restore: move it back down
      const firstHandle = checklist.locator('.drag-handle').first();
      await firstHandle.focus();
      await firstHandle.press('ArrowDown');
      await expect(checklist.locator('.saving-indicator')).not.toBeVisible({
        timeout: SAVE_TIMEOUT_MS,
      });
    });

    test('should toggle tag filter with keyboard activation', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      const tagPill = checklist.locator('.tag-pill').first();
      await tagPill.focus();

      // Activate with Enter — native <button> responds to Enter
      await tagPill.press('Enter');
      await expect(tagPill).toHaveAttribute('aria-pressed', 'true');

      // Deactivate with Space
      await tagPill.press('Space');
      await expect(tagPill).toHaveAttribute('aria-pressed', 'false');
    });

    test('aria-pressed on tag filter pill should update when clicked', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      const tagPill = checklist.locator('.tag-pill').first();
      await expect(tagPill).toHaveAttribute('aria-pressed', 'false');

      await tagPill.click();
      await expect(tagPill).toHaveAttribute('aria-pressed', 'true');

      // Reset
      await tagPill.click();
      await expect(tagPill).toHaveAttribute('aria-pressed', 'false');
    });
  });

  test.describe('focus management', () => {
    test('clicking item display text should move focus to edit input', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      const firstDisplayText = checklist.locator('.item-display-text').first();
      await firstDisplayText.click();

      const editInput = checklist.locator('.item-text');
      await expect(editInput).toBeFocused();

      // Blur to restore state
      await editInput.press('Escape');
    });

    test('pressing Enter in edit mode should return focus to display text', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      const firstDisplayText = checklist.locator('.item-display-text').first();
      await firstDisplayText.click();

      const editInput = checklist.locator('.item-text');
      await expect(editInput).toBeFocused();

      await editInput.press('Enter');

      // After Enter, focus should return to the item's display text
      await expect(firstDisplayText).toBeFocused();
    });
  });
});
