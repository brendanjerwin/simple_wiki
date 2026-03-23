import { test, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';

// Timeouts
const PAGE_LOAD_TIMEOUT_MS = 15000;

// The test data directory used by the server (matches playwright.config.ts webServer command)
const TEST_DATA_DIR = path.join(__dirname, '..', 'test-data');

// Name of the corrupted page file we create directly in the data directory
const CORRUPTED_PAGE_FILENAME = 'e2e-corrupted-page.md';

test.describe('Page List (/ls) Error Surfacing', () => {
  test.setTimeout(60000);

  test.beforeAll(() => {
    // Write a markdown file with invalid YAML frontmatter directly to the test data
    // directory. The YAML-to-TOML migration will fail to parse it, causing ReadPage
    // to return an error during directory listing — triggering the error banner and
    // error row in the /ls UI.
    const corruptedContent = `---
title: [this is invalid: yaml: syntax
---

This page has deliberately broken frontmatter to trigger a ReadPage error.
`;
    fs.mkdirSync(TEST_DATA_DIR, { recursive: true });
    fs.writeFileSync(path.join(TEST_DATA_DIR, CORRUPTED_PAGE_FILENAME), corruptedContent, 'utf8');
  });

  test.afterAll(() => {
    const filePath = path.join(TEST_DATA_DIR, CORRUPTED_PAGE_FILENAME);
    if (fs.existsSync(filePath)) {
      fs.rmSync(filePath);
    }
  });

  test.describe('when a page fails to load during directory listing', () => {
    test('should display the error banner at the top of the list', async ({ page }) => {
      await page.goto('/ls');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const errorBanner = page.locator('.directory-error-banner');
      await expect(errorBanner).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should mention the failing page in the error banner', async ({ page }) => {
      await page.goto('/ls');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const errorBanner = page.locator('.directory-error-banner');
      await expect(errorBanner).toContainText('e2e-corrupted-page', { timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should include a descriptive error message in the banner (not a silent failure)', async ({ page }) => {
      await page.goto('/ls');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const errorBanner = page.locator('.directory-error-banner');
      // The banner should contain more than just the page name — the error message itself
      await expect(errorBanner).not.toBeEmpty();
      const bannerText = await errorBanner.innerText();
      expect(bannerText.length).toBeGreaterThan(20);
    });

    test('should display an error row in the table for the failing page', async ({ page }) => {
      await page.goto('/ls');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const errorRow = page.locator('tr.directory-error-row');
      await expect(errorRow).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should name the failing page in the error row', async ({ page }) => {
      await page.goto('/ls');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

      const errorRow = page.locator('tr.directory-error-row');
      await expect(errorRow).toContainText('e2e-corrupted-page', { timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should still display the table with successfully loaded pages', async ({ page }) => {
      await page.goto('/ls');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

      // The table itself should still be rendered despite the error
      await expect(page.locator('table')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });
  });
});
