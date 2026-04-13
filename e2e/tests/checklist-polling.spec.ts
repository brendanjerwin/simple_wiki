import { test, expect } from '@playwright/test';

const TEST_PAGE_NAME = 'e2echecklistpollingtest';
const TEST_LIST_NAME = 'polling_test_list';

const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;

test.describe('Checklist Polling Optimization E2E Tests', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(90000);

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    // Create the test page with a checklist component
    await page.goto(`/${TEST_PAGE_NAME}/edit`);
    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const content = `+++
identifier = "${TEST_PAGE_NAME}"
title = "Checklist Polling Test Page"
+++

# Checklist Polling Test Page

<wiki-checklist list-name="${TEST_LIST_NAME}" page="${TEST_PAGE_NAME}"></wiki-checklist>`;

    await textarea.fill(content);
    await textarea.press('Space');
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
      timeout: SAVE_TIMEOUT_MS,
    });

    // Navigate to view page and add test items so they exist for all tests
    await page.goto(`/${TEST_PAGE_NAME}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const checklist = page.locator('wiki-checklist');
    await expect(checklist.locator('.loading')).not.toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    const addInput = checklist.locator('.add-text-input');
    const addButton = checklist.locator('.add-btn');

    for (const itemText of ['Poll test item 1', 'Poll test item 2', 'Poll test item 3']) {
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

  test.describe('tab visibility - polling pause', () => {
    test('should not poll while the tab is hidden', async ({ page }) => {
      // Install fake clock BEFORE navigation so the component's setInterval uses the fake clock.
      // This lets us advance time past the 10-second poll interval without actually waiting.
      await page.clock.install();

      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist).toBeAttached();
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      // Track GetFrontmatter calls made after the component has finished its initial load.
      // These represent poll-cycle requests, not the initial data fetch.
      let pollCallCount = 0;
      await page.route('**/api.v1.Frontmatter/GetFrontmatter', route => {
        pollCallCount++;
        return route.continue();
      });

      // Simulate the tab becoming hidden (e.g. user switches to another tab)
      await page.evaluate(() => {
        Object.defineProperty(document, 'hidden', {
          value: true,
          configurable: true,
          writable: true,
        });
        document.dispatchEvent(new Event('visibilitychange'));
      });

      // Advance the fake clock past the 10-second poll interval (plus buffer),
      // firing any due timers — runFor fires all callbacks within the time range
      await page.clock.runFor(12000);

      // The poll timer fired but document.hidden was true, so fetchData must NOT have been called
      expect(pollCallCount).toBe(0);
    });

    test('should fetch immediately when the tab becomes visible again', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist).toBeAttached();
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      // Simulate the tab going hidden first so we have a clear starting point.
      // Set both document.hidden and document.visibilityState for a complete simulation.
      await page.evaluate(() => {
        Object.defineProperty(document, 'hidden', {
          value: true,
          configurable: true,
          writable: true,
        });
        Object.defineProperty(document, 'visibilityState', {
          value: 'hidden',
          configurable: true,
          writable: true,
        });
        document.dispatchEvent(new Event('visibilitychange'));
      });

      // Install the route interceptor after initial load so we only count
      // the fetch triggered by the visibility restore, not the initial load.
      let visibilityRestoreFetchCount = 0;
      await page.route('**/api.v1.Frontmatter/GetFrontmatter', route => {
        visibilityRestoreFetchCount++;
        return route.continue();
      });

      // Simulate the tab becoming visible again — the _handleVisibilityChange handler
      // must call fetchData() immediately when document.hidden becomes false.
      // Set both document.hidden and document.visibilityState for a complete simulation.
      await page.evaluate(() => {
        Object.defineProperty(document, 'hidden', {
          value: false,
          configurable: true,
          writable: true,
        });
        Object.defineProperty(document, 'visibilityState', {
          value: 'visible',
          configurable: true,
          writable: true,
        });
        document.dispatchEvent(new Event('visibilitychange'));
      });

      // The fetch triggered by visibility restore should happen promptly
      await expect(async () => {
        expect(visibilityRestoreFetchCount).toBeGreaterThan(0);
      }).toPass({ timeout: 5000 });
    });
  });

  test.describe('save state - prevents concurrent saves', () => {
    test('should disable checkboxes while a save is in progress', async ({ page }) => {
      // Delay MergeFrontmatter to hold the component in saving=true long enough to assert on
      await page.route('**/api.v1.Frontmatter/MergeFrontmatter', async route => {
        await new Promise<void>(resolve => setTimeout(resolve, 2000));
        await route.continue();
      });

      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist).toBeAttached();
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      // Click first checkbox to start a save operation
      const firstCheckbox = checklist.locator('.item-checkbox').first();
      await firstCheckbox.click();

      // The saving indicator should be visible while the save is in flight
      await expect(checklist.locator('.saving-indicator')).toBeVisible();

      // All checkboxes must be disabled (preventing rapid-fire saves)
      const checkboxes = checklist.locator('.item-checkbox');
      const checkboxCount = await checkboxes.count();
      for (let i = 0; i < checkboxCount; i++) {
        await expect(checkboxes.nth(i)).toBeDisabled();
      }
    });

    test('should re-enable checkboxes after a save completes', async ({ page }) => {
      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist).toBeAttached();
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      // Toggle a checkbox to trigger a save
      const firstCheckbox = checklist.locator('.item-checkbox').first();
      await firstCheckbox.click();

      // Wait for the save to complete (saving indicator disappears)
      await expect(checklist.locator('.saving-indicator')).not.toBeVisible({
        timeout: SAVE_TIMEOUT_MS,
      });

      // Checkboxes must be re-enabled once the save is done
      const checkboxes = checklist.locator('.item-checkbox');
      const checkboxCount = await checkboxes.count();
      for (let i = 0; i < checkboxCount; i++) {
        await expect(checkboxes.nth(i)).not.toBeDisabled();
      }
    });

    test('should produce exactly one MergeFrontmatter request per checkbox toggle', async ({ page }) => {
      let mergeCallCount = 0;
      await page.route('**/api.v1.Frontmatter/MergeFrontmatter', async route => {
        mergeCallCount++;
        await route.continue();
      });

      await page.goto(`/${TEST_PAGE_NAME}/view`);
      await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const checklist = page.locator('wiki-checklist');
      await expect(checklist).toBeAttached();
      await expect(checklist.locator('.loading')).not.toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      const countBeforeToggle = mergeCallCount;

      // Toggle the first checkbox
      const firstCheckbox = checklist.locator('.item-checkbox').first();
      await firstCheckbox.click();

      // Wait for the save to complete
      await expect(checklist.locator('.saving-indicator')).not.toBeVisible({
        timeout: SAVE_TIMEOUT_MS,
      });

      // Exactly one MergeFrontmatter request must have been made for this single toggle —
      // debouncing via the saving=true disabled state prevents any extra concurrent requests
      expect(mergeCallCount - countBeforeToggle).toBe(1);
    });
  });
});
