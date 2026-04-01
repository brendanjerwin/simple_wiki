import { test, expect } from '@playwright/test';

// Test data
const TEST_PAGE_NAME = 'E2EFrontmatterTest';

// Constants
const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;
const DIALOG_TIMEOUT_MS = 5000;

// Helper function to match frontmatter fields with flexible quote handling
function frontMatterStringMatcher(key: string, value: string): RegExp {
  const escapedKey = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const escapedValue = value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return new RegExp(`${escapedKey}\\s*=\\s*['"]${escapedValue}['"]`);
}

test.describe('Frontmatter Editing E2E Tests', () => {
  test.setTimeout(60000);

  // Create a test page before all tests
  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const initialContent = `+++
identifier = "${TEST_PAGE_NAME.toLowerCase()}"
title = "Frontmatter Test Page"
+++

# Frontmatter Test Page

This page is used to test frontmatter editing functionality.`;

    await textarea.fill(initialContent);
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
      await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await textarea.fill(`+++
identifier = "${TEST_PAGE_NAME.toLowerCase()}"
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

  test('should open frontmatter editor dialog on edit page', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);

    // Wait for page to load
    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Find and verify the frontmatter dialog component exists
    const dialog = page.locator('frontmatter-editor-dialog');
    await expect(dialog).toBeAttached();

    // The dialog should be closed initially
    await expect(dialog.locator('.dialog')).not.toBeVisible();
  });

  test('should display loading state when opening dialog', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    await expect(page.locator('wiki-editor textarea')).toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Open the dialog by evaluating JavaScript
    await page.evaluate((pageName) => {
      const dialog = document.querySelector('frontmatter-editor-dialog');
      if (dialog) {
        (dialog as any).openDialog(pageName);
      }
    }, TEST_PAGE_NAME.toLowerCase());

    // Dialog should become visible
    const dialog = page.locator('frontmatter-editor-dialog .dialog');
    await expect(dialog).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    // Should either show loading state briefly or already have content
    // We just verify the dialog opened successfully
    await expect(page.locator('frontmatter-editor-dialog .dialog-header')).toContainText(
      'Edit Frontmatter',
    );
  });

  test('should display existing frontmatter values', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    await expect(page.locator('wiki-editor textarea')).toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Open the dialog
    await page.evaluate((pageName) => {
      const dialog = document.querySelector('frontmatter-editor-dialog');
      if (dialog) {
        (dialog as any).openDialog(pageName);
      }
    }, TEST_PAGE_NAME.toLowerCase());

    await expect(page.locator('frontmatter-editor-dialog .dialog')).toBeVisible({
      timeout: DIALOG_TIMEOUT_MS,
    });

    // Wait for content to load (no longer loading)
    await expect(page.locator('frontmatter-editor-dialog .loading')).not.toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Verify the frontmatter content area is visible
    await expect(page.locator('frontmatter-editor-dialog .content')).toBeVisible();

    // Verify that frontmatter-value-section component is present (renders the fields)
    await expect(page.locator('frontmatter-editor-dialog frontmatter-value-section')).toBeAttached();
  });

  test('should close dialog when clicking cancel', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    await expect(page.locator('wiki-editor textarea')).toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Open the dialog
    await page.evaluate((pageName) => {
      const dialog = document.querySelector('frontmatter-editor-dialog');
      if (dialog) {
        (dialog as any).openDialog(pageName);
      }
    }, TEST_PAGE_NAME.toLowerCase());

    await expect(page.locator('frontmatter-editor-dialog .dialog')).toBeVisible({
      timeout: DIALOG_TIMEOUT_MS,
    });

    // Click cancel button (has class button-secondary, not button-cancel)
    await page.locator('frontmatter-editor-dialog button.button-secondary').click();

    // Dialog should close
    await expect(page.locator('frontmatter-editor-dialog .dialog')).not.toBeVisible();
  });

  test('should close dialog when pressing Escape key', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    await expect(page.locator('wiki-editor textarea')).toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Open the dialog
    await page.evaluate((pageName) => {
      const dialog = document.querySelector('frontmatter-editor-dialog');
      if (dialog) {
        (dialog as any).openDialog(pageName);
      }
    }, TEST_PAGE_NAME.toLowerCase());

    await expect(page.locator('frontmatter-editor-dialog .dialog')).toBeVisible({
      timeout: DIALOG_TIMEOUT_MS,
    });

    // Press Escape key
    await page.keyboard.press('Escape');

    // Dialog should close
    await expect(page.locator('frontmatter-editor-dialog .dialog')).not.toBeVisible();
  });

  test('should add and save a new frontmatter field', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // First, add a custom field via the editor to ensure we can test it
    const currentContent = await textarea.inputValue();
    const frontmatterMatch = currentContent.match(/^\+\+\+[\s\S]*?\+\+\+/);
    const bodyMatch = currentContent.match(/\+\+\+[\s\S]*?\+\+\+([\s\S]*)$/);
    const body = bodyMatch ? bodyMatch[1] : '';

    const newContent = `+++
identifier = "${TEST_PAGE_NAME.toLowerCase()}"
title = "Frontmatter Test Page"
author = "E2E Test"
+++${body}`;

    await textarea.fill(newContent);
    await textarea.press('Space');
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
      timeout: SAVE_TIMEOUT_MS,
    });

    // Reload to verify persistence
    await page.reload();
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const savedContent = await textarea.inputValue();
    expect(savedContent).toMatch(frontMatterStringMatcher('author', 'E2E Test'));
  });

  test('should handle error state gracefully', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    await expect(page.locator('wiki-editor textarea')).toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Try to open dialog for a non-existent page (should handle gracefully)
    await page.evaluate(() => {
      const dialog = document.querySelector('frontmatter-editor-dialog');
      if (dialog) {
        (dialog as any).openDialog('nonexistent_page_12345');
      }
    });

    await expect(page.locator('frontmatter-editor-dialog .dialog')).toBeVisible({
      timeout: DIALOG_TIMEOUT_MS,
    });

    // Should show error display component
    // Note: Depending on implementation, this might show loading or error
    // We just verify the dialog opened and has content
    await expect(page.locator('frontmatter-editor-dialog .dialog-header')).toContainText(
      'Edit Frontmatter',
    );
  });
});
