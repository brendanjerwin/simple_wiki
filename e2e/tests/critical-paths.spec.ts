import { test, expect } from '@playwright/test';

// Test data
const TEST_PAGE_NAME = 'E2ETestPage';

// Constants
const SAVE_TIMEOUT_MS = 10000;

// Helper functions
function formatTimestamp(): string {
  return new Date().toISOString().slice(0, 19);
}

// Helper function to match frontmatter fields with flexible quote handling
function frontMatterStringMatcher(key: string, value: string): RegExp {
  // Matches: key = "value" or key = 'value'
  const escapedKey = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const escapedValue = value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return new RegExp(`${escapedKey}\\s*=\\s*['"]${escapedValue}['"]`);
}

test.describe('Simple Wiki E2E Critical Paths', () => {
  
  test('should load the home page editing interface successfully', async ({ page }) => {
    // Go to the home page edit mode
    await page.goto('/home/edit');
    
    // Verify the page loads and we're in edit mode
    await expect(page).toHaveTitle('home');
    
    // Should have the edit interface elements
    await expect(page.locator('#userInput')).toBeVisible();
    await expect(page.locator('form')).toBeVisible();
    
    // Should have some default content with frontmatter
    const textarea = page.locator('#userInput');
    const content = await textarea.inputValue();
    expect(content).toMatch(frontMatterStringMatcher('identifier', 'home'));
  });

  test('should create and edit a new page', async ({ page }) => {
    // Navigate directly to a new test page edit mode
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    
    // Should show the edit interface
    await expect(page.locator('#userInput')).toBeVisible();
    
    // Add content to the new page
    const textarea = page.locator('#userInput');
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
    
    // Wait for save to complete - look for save button to show success
    await expect(page.locator('#saveEditButton')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });
    
    // Go to view mode to verify the page was created and content rendered
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);
    
    // The page should show the content (even if h1 shows identifier, body should have content)
    await expect(page.locator('body')).toContainText('E2E test suite');
    await expect(page.locator('body')).toContainText('Feature 1: Basic editing');
  });

  test('should show page list and allow navigation', async ({ page }) => {
    // Navigate to the page list
    await page.goto('/ls');
    
    // Should show a list of pages - the h1 shows "ls" not "List"
    await expect(page.locator('h1')).toContainText('ls');
    
    // Should have some pages listed (at least home)
    const pageLinks = page.locator('a[href*="/view"]');
    await expect(pageLinks.first()).toBeVisible();
    
    // Find and click the home page link
    const homeLink = page.locator('a[href="/home/view"]');
    await homeLink.click();
    
    // Should navigate to the home page view
    await expect(page.url()).toContain('/home/view');
  });

  test('should persist content across edit sessions', async ({ page }) => {
    // Go to test page edit mode  
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    
    const textarea = page.locator('#userInput');
    
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
    await expect(page.locator('#saveEditButton')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });
    
    // Navigate away and back to verify persistence
    await page.goto('/home/edit');
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    
    // Content should be preserved
    const savedContent = await textarea.inputValue();
    expect(savedContent).toContain(timestamp);
    expect(savedContent).toContain('Persistence Test');
  });

  test('should handle navigation between pages via URL', async ({ page }) => {
    // Test direct navigation to different page types
    const testPages = [
      '/home/edit',
      '/home/view', 
      '/ls',
      `/${TEST_PAGE_NAME.toLowerCase()}/edit`,
      `/${TEST_PAGE_NAME.toLowerCase()}/view`
    ];
    
    for (const url of testPages) {
      await page.goto(url);
      
      // Should not show any error pages
      await expect(page.locator('body')).not.toContainText('404');
      await expect(page.locator('body')).not.toContainText('Error');
      
      // Should have loaded something (basic smoke test)
      await expect(page.locator('body')).toBeVisible();
    }
  });

  // Cleanup test - ensure we clean up test data
  test('should clean up test pages', async ({ page }) => {
    // Clean up the test page by setting minimal content
    try {
      await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
      const textarea = page.locator('#userInput');
      
      // Set minimal content to essentially "delete" the page
      await textarea.fill(`+++
identifier = "${TEST_PAGE_NAME.toLowerCase()}"
+++`);
      await textarea.press('Space'); // Trigger keyup event
      
      // Wait for save to complete
      await expect(page.locator('#saveEditButton')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });
      
      console.log(`Cleaned up test page: ${TEST_PAGE_NAME}`);
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      console.log(`Test page cleanup skipped: ${errorMessage}`);
    }
    
    // Reset home page to clean state  
    await page.goto('/home/edit');
    const homeTextarea = page.locator('#userInput');
    await homeTextarea.fill(`+++
identifier = "home"
+++

# Home

Welcome to your wiki!`);
    await homeTextarea.press('Space'); // Trigger keyup event
    
    // Wait for save to complete
    await expect(page.locator('#saveEditButton')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });
  });
});