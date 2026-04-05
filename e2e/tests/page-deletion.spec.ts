import { test, expect } from '@playwright/test';
import { frontMatterStringMatcher } from './helpers/frontmatter.js';

// Test data
const TEST_PAGE_FOR_DELETION = 'E2EDeletionTest';

// Constants
const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;

test.describe('Page Deletion E2E Tests', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test('should create a page for deletion testing', async ({ page }) => {
    // Navigate to the test page edit mode
    await page.goto(`/${TEST_PAGE_FOR_DELETION.toLowerCase()}/edit`);

    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Create the page with some content
    const content = `+++
identifier = "${TEST_PAGE_FOR_DELETION.toLowerCase()}"
title = "Page for Deletion Test"
+++

# Page for Deletion Test

This page will be deleted as part of the E2E test suite.

It contains some test content to verify deletion works correctly.`;

    await textarea.fill(content);
    await textarea.press('Space');

    // Wait for save to complete
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
      timeout: SAVE_TIMEOUT_MS,
    });

    // Verify the page was created by navigating to view mode
    await page.goto(`/${TEST_PAGE_FOR_DELETION.toLowerCase()}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
    await expect(page.locator('body')).toContainText('Page for Deletion Test');
  });

  test('should verify page appears in page list before deletion', async ({ page }) => {
    // Navigate to the page list
    await page.goto('/ls');

    await expect(page.locator('h1')).toContainText('ls', { timeout: PAGE_LOAD_TIMEOUT_MS });

    // Find the link to our test page
    const pageLink = page.locator(`a[href="/${TEST_PAGE_FOR_DELETION.toLowerCase()}/view"]`);
    await expect(pageLink).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
  });

  test('should delete the page by removing all content except frontmatter', async ({ page }) => {
    // Navigate to edit mode
    await page.goto(`/${TEST_PAGE_FOR_DELETION.toLowerCase()}/edit`);

    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Delete the page by setting minimal content (just identifier)
    const minimalContent = `+++
identifier = "${TEST_PAGE_FOR_DELETION.toLowerCase()}"
+++`;

    await textarea.fill(minimalContent);
    await textarea.press('Space');

    // Wait for save to complete
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
      timeout: SAVE_TIMEOUT_MS,
    });
  });

  test('should show minimal-content page in page list after deletion', async ({ page }) => {
    // Navigate to the page list
    await page.goto('/ls');

    await expect(page.locator('h1')).toContainText('ls', { timeout: PAGE_LOAD_TIMEOUT_MS });

    // Verify view links are present
    const pageLinks = page.locator('a[href*="/view"]');
    await expect(pageLinks.first()).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    // Get all page link texts
    const allLinks = await pageLinks.allTextContents();

    expect(allLinks.length).toBeGreaterThan(0); // At least home page should exist

    // The server lists all .md files regardless of content, so a minimal-content
    // page still appears in /ls after "deletion" (content-stripping).
    const hasDeletedPage = allLinks.some((text) =>
      text.toLowerCase().includes(TEST_PAGE_FOR_DELETION.toLowerCase()),
    );
    expect(hasDeletedPage).toBe(true);
  });

  test('should show minimal content when navigating to deleted page view', async ({ page }) => {
    // Navigate to the "deleted" page's view mode
    await page.goto(`/${TEST_PAGE_FOR_DELETION.toLowerCase()}/view`);

    // The page should load but show minimal content (empty body = zero-height #rendered,
    // so check it's in the DOM rather than visually present)
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    // Verify it doesn't show the original content
    await expect(page.locator('body')).not.toContainText('This page will be deleted');

    // The page might show empty content or just the title
    // We verify it loads without the original content
  });

  test('should allow editing the deleted page to restore it', async ({ page }) => {
    // Navigate to edit mode of the deleted page
    await page.goto(`/${TEST_PAGE_FOR_DELETION.toLowerCase()}/edit`);

    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Verify it shows minimal content (TOML may use single or double quotes)
    const currentContent = await textarea.inputValue();
    expect(currentContent).toMatch(frontMatterStringMatcher('identifier', TEST_PAGE_FOR_DELETION.toLowerCase()));

    // Restore the page with new content
    const restoredContent = `+++
identifier = "${TEST_PAGE_FOR_DELETION.toLowerCase()}"
title = "Restored Page"
+++

# Restored Page

This page was restored after deletion.`;

    await textarea.fill(restoredContent);
    await textarea.press('Space');

    // Wait for save to complete
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
      timeout: SAVE_TIMEOUT_MS,
    });

    // Verify restoration by viewing the page
    await page.goto(`/${TEST_PAGE_FOR_DELETION.toLowerCase()}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
    await expect(page.locator('body')).toContainText('Restored Page');
    await expect(page.locator('body')).toContainText('This page was restored after deletion');
  });

  test('should clean up the test page completely', async ({ page }) => {
    // Final cleanup - delete the page again
    await page.goto(`/${TEST_PAGE_FOR_DELETION.toLowerCase()}/edit`);

    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const minimalContent = `+++
identifier = "${TEST_PAGE_FOR_DELETION.toLowerCase()}"
+++`;

    await textarea.fill(minimalContent);
    await textarea.press('Space');

    // Wait for save to complete
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
      timeout: SAVE_TIMEOUT_MS,
    });
  });
});
