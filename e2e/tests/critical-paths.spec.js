const { test, expect } = require('@playwright/test');

// Test data
const TEST_PAGE_NAME = 'E2ETestPage';
const TEST_PAGE_CONTENT = `# E2E Test Page

This is a test page created by the E2E test suite.

## Features to test
- Basic editing
- Saving
- Navigation  
- Linking to other pages

Here's a link to [[AnotherTestPage]] that we'll create.`;

const ANOTHER_PAGE_CONTENT = `# Another Test Page

This page was created by following a link from the first test page.

Back to [[${TEST_PAGE_NAME}]]`;

test.describe('Simple Wiki E2E Critical Paths', () => {
  
  test.beforeEach(async ({ page }) => {
    // Go to the home page edit mode before each test
    await page.goto('/home/edit');
  });

  test('should load the home page editing interface successfully', async ({ page }) => {
    // Verify the page loads and we're in edit mode
    await expect(page).toHaveTitle('home');
    
    // Should have the edit interface elements
    await expect(page.locator('#userInput')).toBeVisible();
    await expect(page.locator('form')).toBeVisible();
    
    // Should have some default content
    const textarea = page.locator('#userInput');
    const content = await textarea.inputValue();
    expect(content).toContain('identifier = "home"');
  });

  test('should create a new page by editing home page and navigating to it', async ({ page }) => {
    // Step 1: Edit the home page to add a link to our test page
    const textarea = page.locator('#userInput');
    await textarea.click();
    
    // Clear existing content and add our test content with a link
    await textarea.fill(`+++
identifier = "home"
+++

# Home Page

Welcome to the wiki!

Check out the [[${TEST_PAGE_NAME}]] for testing.`);
    
    // Wait a moment for auto-save (debounce is 500ms by default)
    await page.waitForTimeout(600);
    
    // Step 2: Click on the link to navigate to the new page
    // First go to view mode to see the rendered link
    await page.goto('/home/view');
    await page.locator(`text=${TEST_PAGE_NAME}`).click();
    
    // Step 3: Should be on the new page, likely redirected to edit mode since it doesn't exist
    await expect(page.url()).toContain(TEST_PAGE_NAME.toLowerCase());
    
    // Step 4: Add content to the new page
    const newPageTextarea = page.locator('#userInput');
    await newPageTextarea.fill(`+++
identifier = "${TEST_PAGE_NAME}"
+++

${TEST_PAGE_CONTENT}`);
    
    // Wait for auto-save
    await page.waitForTimeout(600);
    
    // Step 5: Verify content was saved by going to view mode
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);
    await expect(page.locator('h1')).toContainText('E2E Test Page');
  });

  test('should navigate between linked pages', async ({ page }) => {
    // First, set up the test page if it doesn't exist
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    
    const textarea = page.locator('#userInput');
    await textarea.fill(`+++
identifier = "${TEST_PAGE_NAME}"
+++

${TEST_PAGE_CONTENT}`);
    await page.waitForTimeout(600);
    
    // Go to view mode and click on the link to AnotherTestPage
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);
    await page.locator('text=AnotherTestPage').click();
    
    // Should be on the new page in edit mode
    await expect(page.url()).toContain('anothertestpage');
    
    // Add content to this page
    await page.locator('#userInput').fill(`+++
identifier = "anothertestpage"
+++

${ANOTHER_PAGE_CONTENT}`);
    await page.waitForTimeout(600);
    
    // Navigate back using the link in view mode
    await page.goto('/anothertestpage/view');
    await page.locator(`text=${TEST_PAGE_NAME}`).click();
    
    // Should be back on the original test page
    await expect(page.url()).toContain(TEST_PAGE_NAME.toLowerCase());
  });

  test('should show page list and navigate to pages', async ({ page }) => {
    // Navigate to the page list
    await page.goto('/ls');
    
    // Should show a list of pages
    await expect(page.locator('h1')).toContainText('List');
    
    // Should have some pages listed (at least home)
    await expect(page.locator('a[href*="/view"]')).toHaveCount({ gte: 1 });
    
    // Click on the home page link and verify navigation
    await page.locator('a[href="/home/view"]').click();
    
    // Should navigate to the home page view
    await expect(page.url()).toContain('/home/view');
  });

  test('should handle editing and auto-save functionality', async ({ page }) => {
    // Go to our test page
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    
    const textarea = page.locator('#userInput');
    const testContent = `+++
identifier = "${TEST_PAGE_NAME}"
+++

# Auto-save Test

This content should be automatically saved.

Timestamp: ${new Date().toISOString()}`;
    
    // Clear and type new content
    await textarea.fill(testContent);
    
    // Wait for auto-save (default debounce is 500ms)
    await page.waitForTimeout(700);
    
    // Navigate away and back to verify save
    await page.goto('/home/edit');
    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
    
    // Content should be preserved
    await expect(textarea).toHaveValue(testContent);
  });

  // Cleanup test - runs last to clean up test data
  test('should clean up test pages', async ({ page }) => {
    // This test helps clean up by clearing test pages
    // Go to each test page and clear its content
    
    const pagesToClean = [TEST_PAGE_NAME.toLowerCase(), 'anothertestpage'];
    
    for (const pageName of pagesToClean) {
      try {
        await page.goto(`/${pageName}/edit`);
        const textarea = page.locator('#userInput');
        
        // Clear the content (but keep a minimal identifier)
        await textarea.fill(`+++
identifier = "${pageName}"
+++`);
        await page.waitForTimeout(600);
        
        console.log(`Cleaned up page: ${pageName}`);
      } catch (error) {
        // Page might not exist, which is fine
        console.log(`Page ${pageName} not found or already clean`);
      }
    }
    
    // Reset home page to a clean state
    await page.goto('/home/edit');
    const textarea = page.locator('#userInput');
    await textarea.fill(`+++
identifier = "home"
+++

# Home

Welcome to your wiki!`);
    await page.waitForTimeout(600);
  });
});