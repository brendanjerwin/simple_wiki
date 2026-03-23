import { test, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';

// Timeouts
const PAGE_LOAD_TIMEOUT_MS = 15000;

// The test data directory used by the server (matches playwright.config.ts webServer command)
const TEST_DATA_DIR = path.join(__dirname, '..', 'test-data');

// The human-readable identifier for the corrupted page (used in UI assertions)
const CORRUPTED_PAGE_IDENTIFIER = 'e2e-corrupted-page';

// Encode a string using standard base32 (RFC 4648), matching Go's base32.StdEncoding.EncodeToString.
// The wiki stores pages as base32(strings.ToLower(identifier)) + ".md"
function encodeBase32(str: string): string {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  const bytes = Buffer.from(str, 'utf8');
  let bits = 0;
  let value = 0;
  let output = '';

  for (const byte of bytes) {
    value = (value << 8) | byte;
    bits += 8;
    while (bits >= 5) {
      bits -= 5;
      output += alphabet[(value >>> bits) & 0x1f];
    }
  }

  if (bits > 0) {
    output += alphabet[(value << (5 - bits)) & 0x1f];
  }

  // Pad to multiple of 8 characters
  while (output.length % 8 !== 0) {
    output += '=';
  }

  return output;
}

// The on-disk filename the wiki will assign to this page identifier
const CORRUPTED_PAGE_FILENAME = encodeBase32(CORRUPTED_PAGE_IDENTIFIER.toLowerCase()) + '.md';

test.describe('Page List (/ls) Error Surfacing', () => {
  test.setTimeout(60000);

  test.beforeAll(() => {
    // Write a markdown file with invalid YAML frontmatter directly to the test data
    // directory, using the same base32-encoded filename scheme the wiki uses.
    // The YAML-to-TOML migration will fail to parse it, causing ReadPage
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
    fs.rmSync(path.join(TEST_DATA_DIR, CORRUPTED_PAGE_FILENAME), { force: true });
  });

  test.describe('when a page fails to load during directory listing', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/ls');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should display the error banner at the top of the list', async ({ page }) => {
      const errorBanner = page.locator('.directory-error-banner');
      await expect(errorBanner).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should mention the failing page in the error banner', async ({ page }) => {
      const errorBanner = page.locator('.directory-error-banner');
      await expect(errorBanner).toContainText(CORRUPTED_PAGE_IDENTIFIER, { timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should include a descriptive error message in the banner (not a silent failure)', async ({ page }) => {
      const errorBanner = page.locator('.directory-error-banner');
      await expect(errorBanner).not.toBeEmpty();
      const bannerText = await errorBanner.innerText();
      expect(bannerText.toLowerCase()).toMatch(/error|failed|invalid/);
    });

    test('should display an error row in the table for the failing page', async ({ page }) => {
      const errorRow = page.locator('tr.directory-error-row');
      await expect(errorRow).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should name the failing page in the error row', async ({ page }) => {
      const errorRow = page.locator('tr.directory-error-row');
      await expect(errorRow).toContainText(CORRUPTED_PAGE_IDENTIFIER, { timeout: PAGE_LOAD_TIMEOUT_MS });
    });

    test('should still display the table with successfully loaded pages', async ({ page }) => {
      // The table itself should still be rendered despite the error
      await expect(page.locator('table')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
    });
  });
});
