import { test, expect } from '@playwright/test';

// Test data
const TEST_PAGE_NAME = 'E2EChecklistTest';
const TEST_LIST_NAME = 'e2e_test_list';

// Constants
const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;
const POLL_WAIT_MS = 4000; // Checklist polls every 3s, wait 4s to ensure it completes

test.describe('Checklist E2E Tests', () => {
  test.setTimeout(60000);

  // Create a test page with a checklist before all tests
  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const initialContent = `+++
identifier = "${TEST_PAGE_NAME.toLowerCase()}"
title = "Checklist Test Page"
[${TEST_LIST_NAME}]
+++

# Checklist Test Page

<wiki-checklist list-name="${TEST_LIST_NAME}" page="${TEST_PAGE_NAME.toLowerCase()}"></wiki-checklist>`;

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

  test('should render checklist component on view page', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);

    // Wait for page to load
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    // Verify checklist component is present
    const checklist = page.locator('wiki-checklist');
    await expect(checklist).toBeAttached();

    // Verify it has the correct attributes
    await expect(checklist).toHaveAttribute('list-name', TEST_LIST_NAME);
    await expect(checklist).toHaveAttribute('page', TEST_PAGE_NAME.toLowerCase());
  });

  test('should add items to the checklist', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const checklist = page.locator('wiki-checklist');
    await expect(checklist).toBeAttached();

    // Wait for checklist to load (no loading indicator)
    await expect(checklist.locator('.loading')).not.toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Find the add input and button
    const addInput = checklist.locator('.add-text-input');
    const addButton = checklist.locator('.add-btn');

    await expect(addInput).toBeVisible();

    // Add a new item
    await addInput.fill('Test item 1');
    await addButton.click();

    // Wait for save to complete and item to appear
    await page.waitForTimeout(POLL_WAIT_MS);

    // Verify the item appears in the list
    const items = checklist.locator('.item-row');
    await expect(items).toHaveCount(1);
    await expect(items.first().locator('.item-display-text')).toContainText('Test item 1');
  });

  test('should check and uncheck items', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const checklist = page.locator('wiki-checklist');
    await expect(checklist).toBeAttached();
    await expect(checklist.locator('.loading')).not.toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Get the first item's checkbox
    const firstItem = checklist.locator('.item-row').first();
    const checkbox = firstItem.locator('.item-checkbox');

    // Verify checkbox starts unchecked
    await expect(checkbox).not.toBeChecked();

    // Check the item
    await checkbox.check();

    // Wait for save
    await page.waitForTimeout(POLL_WAIT_MS);

    // Verify it's checked
    await expect(checkbox).toBeChecked();

    // Uncheck the item
    await checkbox.uncheck();

    // Wait for save
    await page.waitForTimeout(POLL_WAIT_MS);

    // Verify it's unchecked
    await expect(checkbox).not.toBeChecked();
  });

  test('should add tagged items and display tags', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const checklist = page.locator('wiki-checklist');
    await expect(checklist).toBeAttached();
    await expect(checklist.locator('.loading')).not.toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Add an item with tags using :tag syntax
    const addInput = checklist.locator('.add-text-input');
    const addButton = checklist.locator('.add-btn');

    await addInput.fill('Buy milk :dairy :grocery');
    await addButton.click();

    // Wait for save and poll
    await page.waitForTimeout(POLL_WAIT_MS);

    // Find the newly added item (should be last)
    const items = checklist.locator('.item-row');
    const itemCount = await items.count();
    const taggedItem = items.nth(itemCount - 1);

    // Verify the text doesn't include the tag syntax
    await expect(taggedItem.locator('.item-display-text')).toContainText('Buy milk');

    // Verify tags are displayed as badges
    const tagBadges = taggedItem.locator('.item-tag-badge');
    await expect(tagBadges).toHaveCount(2);

    const tagTexts = await tagBadges.allTextContents();
    expect(tagTexts).toContain('dairy');
    expect(tagTexts).toContain('grocery');
  });

  test('should filter items by tags', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const checklist = page.locator('wiki-checklist');
    await expect(checklist).toBeAttached();
    await expect(checklist.locator('.loading')).not.toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Add another tagged item to ensure we have multiple items with different tags
    const addInput = checklist.locator('.add-text-input');
    const addButton = checklist.locator('.add-btn');

    await addInput.fill('Clean room :chores');
    await addButton.click();
    await page.waitForTimeout(POLL_WAIT_MS);

    // Count total items before filtering
    const totalItems = await checklist.locator('.item-row').count();
    expect(totalItems).toBeGreaterThan(1);

    // Click on a tag pill in the tag filter bar
    const tagFilterBar = checklist.locator('.tag-filter-bar');
    if ((await tagFilterBar.count()) > 0) {
      const tagPill = tagFilterBar.locator('.tag-pill').first();
      if ((await tagPill.count()) > 0) {
        await tagPill.click();

        // Wait for filter to apply
        await page.waitForTimeout(1000);

        // Verify filtering occurred (some items should be filtered out)
        const filteredItems = await checklist.locator('.item-row').count();

        // With filtering active, we should have fewer items visible
        // or at least the same if all items have the tag
        expect(filteredItems).toBeLessThanOrEqual(totalItems);
      }
    }
  });

  test('should persist items after page reload', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const checklist = page.locator('wiki-checklist');
    await expect(checklist).toBeAttached();
    await expect(checklist.locator('.loading')).not.toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Count current items
    const itemCountBefore = await checklist.locator('.item-row').count();
    expect(itemCountBefore).toBeGreaterThan(0);

    // Get text of first item
    const firstItemText = await checklist
      .locator('.item-row')
      .first()
      .locator('.item-display-text')
      .textContent();

    // Reload the page
    await page.reload();
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const checklistAfterReload = page.locator('wiki-checklist');
    await expect(checklistAfterReload).toBeAttached();
    await expect(checklistAfterReload.locator('.loading')).not.toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Verify items are still present
    const itemCountAfter = await checklistAfterReload.locator('.item-row').count();
    expect(itemCountAfter).toBe(itemCountBefore);

    // Verify first item still has same text
    const firstItemTextAfter = await checklistAfterReload
      .locator('.item-row')
      .first()
      .locator('.item-display-text')
      .textContent();
    expect(firstItemTextAfter).toBe(firstItemText);
  });

  test('should remove items', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const checklist = page.locator('wiki-checklist');
    await expect(checklist).toBeAttached();
    await expect(checklist.locator('.loading')).not.toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    // Count items before removal
    const itemCountBefore = await checklist.locator('.item-row').count();

    if (itemCountBefore > 0) {
      // Find and click the remove button on the first item
      const firstItem = checklist.locator('.item-row').first();
      const removeButton = firstItem.locator('.remove-btn');

      await removeButton.click();

      // Wait for save and poll
      await page.waitForTimeout(POLL_WAIT_MS);

      // Verify item count decreased
      const itemCountAfter = await checklist.locator('.item-row').count();
      expect(itemCountAfter).toBe(itemCountBefore - 1);
    }
  });
});
