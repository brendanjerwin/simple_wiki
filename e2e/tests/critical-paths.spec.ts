import { test, expect } from '@playwright/test';
import { frontMatterStringMatcher } from './helpers/frontmatter.js';

// Test data
const TEST_PAGE_NAME = 'E2ETestPage';

// Constants
const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;

// Helper functions
function formatTimestamp(): string {
  return new Date().toISOString().slice(0, 19);
}

// Increase test timeout to handle slow environments (Tailscale WhoIs lookups
// add latency to every HTTP request, making static asset loading slow)
test.describe('Simple Wiki E2E Critical Paths', () => {
  test.setTimeout(60000);

  test('should load the home page editing interface successfully', async ({ page }) => {
    // Go to the home page edit mode
    await page.goto('/home/edit');

    // Verify the page loads and we're in edit mode
    await expect(page).toHaveTitle('home');

    // Should have the edit interface elements — textarea and file-drop-zone
    // are inside the wiki-editor web component's shadow DOM.
    // The wiki-editor starts in loading state and renders the textarea
    // only after the gRPC ReadPage call completes.
    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await expect(page.locator('wiki-editor file-drop-zone')).toBeVisible();

    // Should have some default content with frontmatter
    const content = await textarea.inputValue();
    expect(content).toMatch(frontMatterStringMatcher('identifier', 'home'));
  });

  test('should create and edit a new page', async ({ page }) => {
    // Navigate directly to a new test page edit mode
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);

    // Should show the edit interface — textarea is inside wiki-editor's shadow DOM
    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Add content to the new page
    const testPageContent = `+++
identifier = "${TEST_PAGE_NAME.toLowerCase()}"
title = "E2E Test Page"
+++

# E2E Test Page

This is a test page created by the E2E test suite.

- Feature 1: Basic editing
- Feature 2: Auto-saving
- Feature 3: Navigation

Created at: ${formatTimestamp()}`;

    // Fill the content and trigger keyup event to start auto-save
    await textarea.fill(testPageContent);
    await textarea.press('Space'); // Trigger keyup event

    // Wait for save to complete — the wiki-editor component shows save
    // status in a .status-indicator span inside its shadow DOM
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });

    // Go to view mode to verify the page was created and content rendered
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);

    // The page should show the content (even if h1 shows identifier, body should have content)
    await expect(page.locator('body')).toContainText('E2E test suite', { timeout: PAGE_LOAD_TIMEOUT_MS });
    await expect(page.locator('body')).toContainText('Feature 1: Basic editing');
  });

  test('should show page list and allow navigation', async ({ page }) => {
    // Navigate to the page list
    await page.goto('/ls');

    // Should show a list of pages - the h1 shows "ls" not "List"
    await expect(page.locator('h1')).toContainText('ls', { timeout: PAGE_LOAD_TIMEOUT_MS });

    // Should have some pages listed (at least home)
    const pageLinks = page.locator('a[href*="/view"]');
    await expect(pageLinks.first()).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    // Find and click the home page link
    const homeLink = page.locator('a[href="/home/view"]');
    await homeLink.click();

    // Should navigate to the home page view
    expect(page.url()).toContain('/home/view');
  });

  test('should persist content across edit sessions', async ({ page }) => {
    // Go to test page edit mode
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);

    // Wait for the wiki-editor to finish loading before interacting
    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Create content with a unique timestamp
    const timestamp = formatTimestamp();
    const testContent = `+++
identifier = "${TEST_PAGE_NAME.toLowerCase()}"
+++

# Persistence Test

Content saved at: ${timestamp}

This tests that content persists across page reloads.`;

    // Fill and save the content
    await textarea.fill(testContent);
    await textarea.press('Space'); // Trigger keyup event

    // Wait for save to complete
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });

    // Navigate away and back to verify persistence
    await page.goto('/home/edit');
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);

    // Wait for the wiki-editor to finish loading on the second visit.
    // The component makes a fresh gRPC ReadPage call each time it mounts.
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Content should be preserved
    const savedContent = await textarea.inputValue();
    expect(savedContent).toContain(timestamp);
    expect(savedContent).toContain('Persistence Test');
  });

  test('should handle navigation between pages via URL', async ({ page }) => {
    // Navigating 5 pages with slow static asset loading (WhoIs lookups add
    // latency on every HTTP request) can exceed the default 60s test timeout.
    test.setTimeout(120000);

    // Test direct navigation to different page types.
    // Each entry has a URL and an element to wait for that proves the page loaded.
    // Edit pages use wiki-editor (body has overflow:hidden which Playwright
    // treats as invisible, so we cannot check body or article visibility).
    const testPages: Array<{ url: string; selector: string }> = [
      { url: '/home/edit', selector: 'wiki-editor' },
      { url: '/home/view', selector: '#rendered' },
      { url: '/ls', selector: '#rendered' },
      { url: `/${TEST_PAGE_NAME.toLowerCase()}/edit`, selector: 'wiki-editor' },
      { url: `/${TEST_PAGE_NAME.toLowerCase()}/view`, selector: '#rendered' },
    ];

    for (const { url, selector } of testPages) {
      await page.goto(url, { waitUntil: 'domcontentloaded' });

      // Verify the page loaded by checking for a key element
      await expect(page.locator(selector)).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    }
  });

  // Cleanup test - ensure we clean up test data
  test('should clean up test pages', async ({ page }) => {
    // Clean up the test page by setting minimal content
    try {
      await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Set minimal content to essentially "delete" the page
      await textarea.fill(`+++
identifier = "${TEST_PAGE_NAME.toLowerCase()}"
+++`);
      await textarea.press('Space'); // Trigger keyup event

      // Wait for save to complete
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });

      console.log(`Cleaned up test page: ${TEST_PAGE_NAME}`);
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      console.log(`Test page cleanup skipped: ${errorMessage}`);
    }

    // Reset home page to clean state
    await page.goto('/home/edit');
    const homeTextarea = page.locator('wiki-editor textarea');
    await expect(homeTextarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await homeTextarea.fill(`+++
identifier = "home"
+++

# Home

Welcome to your wiki!`);
    await homeTextarea.press('Space'); // Trigger keyup event

    // Wait for save to complete
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });
  });
});
