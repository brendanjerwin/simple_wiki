const { test, expect } = require('@playwright/test');

// Test data
const TEST_PAGE_NAME = 'E2ETestPage';

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
    expect(content).toContain('identifier = "home"');
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

Created at: ${new Date().toISOString().slice(0, 19)}`;

    await textarea.fill(testPageContent);
    await page.waitForTimeout(1000);
    
    // Go to view mode to verify the page was created and content rendered
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);
    
    // The page should show the content (even if h1 shows identifier, body should have content)
    await expect(page.locator('body')).toContainText('E2E test suite');
    await expect(page.locator('body')).toContainText('Feature 1: Basic editing');
  });

  test('should show page list and allow navigation', async ({ page }) => {
    // Navigate to the page list
    await page.goto('/ls');
    
    // Should show a list of pages
    await expect(page.locator('h1')).toContainText('List');
    
    // Should have some pages listed (at least home)
    const pageLinks = page.locator('a[href*="/view"]');
    await expect(pageLinks).toHaveCount({ gte: 1 });
    
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
    const timestamp = new Date().toISOString().slice(0, 19);
    const testContent = `+++
identifier = "${TEST_PAGE_NAME.toLowerCase()}"
+++

# Persistence Test

Content saved at: ${timestamp}

This tests that content persists across page reloads.`;
    
    // Fill and save the content
    await textarea.fill(testContent);
    await page.waitForTimeout(1000); // Wait for auto-save
    
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
      await page.waitForTimeout(1000);
      
      console.log(`Cleaned up test page: ${TEST_PAGE_NAME}`);
    } catch (error) {
      console.log(`Test page cleanup skipped: ${error.message}`);
    }
    
    // Reset home page to clean state  
    await page.goto('/home/edit');
    const homeTextarea = page.locator('#userInput');
    await homeTextarea.fill(`+++
identifier = "home"
+++

# Home

Welcome to your wiki!`);
    await page.waitForTimeout(1000);
  });
});