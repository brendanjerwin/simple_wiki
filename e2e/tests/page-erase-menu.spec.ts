import { test, expect } from '@playwright/test';

// Test page name (lowercase per wiki convention)
const TEST_PAGE = 'e2eerasemenutestpage';

// Timeouts
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const SAVE_TIMEOUT_MS = 10000;
const DIALOG_APPEAR_TIMEOUT_MS = 5000;
const NAVIGATION_TIMEOUT_MS = 10000;

test.describe('Page Erase Menu', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test.beforeAll(async ({ browser }) => {
    // Create a test page to be erased
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    await page.goto(`/${TEST_PAGE}/edit`);
    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await textarea.fill(`+++
identifier = "${TEST_PAGE}"
title = "E2E Erase Menu Test Page"
+++

# E2E Erase Menu Test Page

This page is used by E2E tests for the page erase menu confirmation flow.`);
    await textarea.press('Space');

    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
      timeout: SAVE_TIMEOUT_MS,
    });

    await ctx.close();
  });

  test('should show a confirmation dialog when Erase is clicked', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Open the tools menu by hovering, then click Erase
    await page.locator('.tools-menu').hover();
    await page.locator('#erasePage').click();

    // The confirmation dialog should appear (page-deletion-service appends it with this id)
    await expect(page.locator('#page-deletion-dialog[open]')).toBeAttached({
      timeout: DIALOG_APPEAR_TIMEOUT_MS,
    });
  });

  test('should display the page name and warning message in the dialog', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await page.locator('.tools-menu').hover();
    await page.locator('#erasePage').click();

    await expect(page.locator('#page-deletion-dialog[open]')).toBeAttached({
      timeout: DIALOG_APPEAR_TIMEOUT_MS,
    });

    await expect(page.locator('#page-deletion-dialog .dialog-message')).toContainText(
      'Are you sure you want to delete this page?',
    );
    await expect(page.locator('#page-deletion-dialog .dialog-description')).toContainText(
      TEST_PAGE,
    );
  });

  test('should close the dialog and remain on the page when Cancel is clicked', async ({
    page,
  }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Open the dialog
    await page.locator('.tools-menu').hover();
    await page.locator('#erasePage').click();
    await expect(page.locator('#page-deletion-dialog[open]')).toBeAttached({
      timeout: DIALOG_APPEAR_TIMEOUT_MS,
    });

    // Click Cancel
    await page.locator('#page-deletion-dialog .button-cancel').click();

    // Dialog should close
    await expect(page.locator('#page-deletion-dialog[open]')).not.toBeAttached({
      timeout: DIALOG_APPEAR_TIMEOUT_MS,
    });

    // Should remain on the test page (not navigated away)
    await expect(page).toHaveURL(new RegExp(`/${TEST_PAGE}/view`));
  });

  test('should navigate to / after confirming deletion', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Open the dialog
    await page.locator('.tools-menu').hover();
    await page.locator('#erasePage').click();
    await expect(page.locator('#page-deletion-dialog[open]')).toBeAttached({
      timeout: DIALOG_APPEAR_TIMEOUT_MS,
    });

    // Click the danger (Delete) button to confirm deletion
    await page.locator('#page-deletion-dialog .button-danger').click();

    // Should navigate to home page after successful deletion.
    // The server redirects '/' → '/{defaultPage}/view', so the final URL is '/home/view'.
    await expect(page).toHaveURL('/home/view', { timeout: NAVIGATION_TIMEOUT_MS });
  });
});
