import { test, expect, type Page } from '@playwright/test';
import { COMPONENT_LOAD_TIMEOUT_MS } from './constants.js';

// Timeouts
const SAVE_TIMEOUT_MS = 10000;
const TOOLS_INTERACTION_TIMEOUT_MS = 2000;

// Test page identifier
const TEST_PAGE = 'e2e_wiki_image_test';

// Use a known static image served by the wiki itself — no external network required.
const TEST_IMAGE_SRC = '/static/img/favicon/favicon-32x32.png';
const TEST_IMAGE_ALT = 'E2E test image';

const TEST_PAGE_MARKDOWN = `+++
identifier = "${TEST_PAGE}"
title = "Wiki Image E2E Test Page"
+++

# Wiki Image Test

This page is used for E2E testing of the wiki-image component.

![${TEST_IMAGE_ALT}](${TEST_IMAGE_SRC})

Some text after the image.`;

/** Create/reset the test page via the editor UI. */
async function setupTestPage(page: Page): Promise<void> {
  await page.goto(`/${TEST_PAGE}/edit`);
  const textarea = page.locator('wiki-editor textarea');
  await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
  await textarea.fill(TEST_PAGE_MARKDOWN);
  await textarea.press('Space');
  await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
    timeout: SAVE_TIMEOUT_MS,
  });
}

/** Navigate to the view page and wait for rendered content to be attached. */
async function navigateToViewPage(page: Page): Promise<void> {
  await page.goto(`/${TEST_PAGE}/view`);
  await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
  await expect(page.locator('wiki-image')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
}

test.describe('wiki-image component', () => {
  test.setTimeout(60000);
  test.describe.configure({ mode: 'serial' });

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await setupTestPage(page);
    await ctx.close();
  });

  test.describe('Image rendering', () => {
    test.beforeEach(async ({ page }) => {
      await navigateToViewPage(page);
    });

    test('wiki-image element is attached to the DOM', async ({ page }) => {
      await expect(page.locator('wiki-image')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    });

    test('img element within wiki-image renders with correct src attribute', async ({ page }) => {
      const img = page.locator('wiki-image img');
      await expect(img).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(img).toHaveAttribute('src', TEST_IMAGE_SRC);
    });

    test('img element within wiki-image renders with correct alt attribute', async ({ page }) => {
      const img = page.locator('wiki-image img');
      await expect(img).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(img).toHaveAttribute('alt', TEST_IMAGE_ALT);
    });

    test('image-container element is rendered inside wiki-image', async ({ page }) => {
      const container = page.locator('wiki-image .image-container');
      await expect(container).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    });
  });

  test.describe('Tools overlay', () => {
    test.beforeEach(async ({ page }) => {
      await navigateToViewPage(page);
    });

    test('tools panel is present in the wiki-image shadow DOM', async ({ page }) => {
      const toolsPanel = page.locator('wiki-image .tools-panel');
      await expect(toolsPanel).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    });

    test('tools panel contains "Open in new tab" button', async ({ page }) => {
      const btn = page.locator('wiki-image button[aria-label="Open in new tab"]');
      await expect(btn).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    });

    test('tools panel contains "Download" button', async ({ page }) => {
      const btn = page.locator('wiki-image button[aria-label="Download"]');
      await expect(btn).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    });

    test('clicking on image sets the tools-open attribute', async ({ page }) => {
      const wikiImage = page.locator('wiki-image');
      await page.locator('wiki-image img').click();
      await expect(wikiImage).toHaveAttribute('tools-open', '', {
        timeout: TOOLS_INTERACTION_TIMEOUT_MS,
      });
    });

    test('clicking outside removes the tools-open attribute', async ({ page }) => {
      const wikiImage = page.locator('wiki-image');
      await page.locator('wiki-image img').click();
      await expect(wikiImage).toHaveAttribute('tools-open', '', {
        timeout: TOOLS_INTERACTION_TIMEOUT_MS,
      });

      await page.evaluate(() => {
        document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      });

      await expect(wikiImage).not.toHaveAttribute('tools-open', {
        timeout: TOOLS_INTERACTION_TIMEOUT_MS,
      });
    });

    test('"Open in new tab" button triggers navigation to image URL in new page', async ({
      page,
      context,
    }) => {
      const wikiImage = page.locator('wiki-image');
      await page.locator('wiki-image img').click();
      await expect(wikiImage).toHaveAttribute('tools-open', '', {
        timeout: TOOLS_INTERACTION_TIMEOUT_MS,
      });

      const [newPage] = await Promise.all([
        context.waitForEvent('page'),
        page.locator('wiki-image button[aria-label="Open in new tab"]').click(),
      ]);

      await expect(newPage).toHaveURL(new RegExp(TEST_IMAGE_SRC.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
      await newPage.close();
    });
  });

  test.describe('Accessibility', () => {
    test.beforeEach(async ({ page }) => {
      await navigateToViewPage(page);
    });

    test('img element has non-empty alt text', async ({ page }) => {
      const img = page.locator('wiki-image img');
      await expect(img).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      const alt = await img.getAttribute('alt');
      expect(alt).toBeTruthy();
      expect((alt as string).trim()).not.toBe('');
    });

    test('"Open in new tab" button has aria-label', async ({ page }) => {
      const btn = page.locator('wiki-image button[aria-label="Open in new tab"]');
      await expect(btn).toHaveAttribute('aria-label', 'Open in new tab');
    });

    test('"Download" button has aria-label', async ({ page }) => {
      const btn = page.locator('wiki-image button[aria-label="Download"]');
      await expect(btn).toHaveAttribute('aria-label', 'Download');
    });

    test('tools panel has role="toolbar"', async ({ page }) => {
      const toolsPanel = page.locator('wiki-image .tools-panel');
      await expect(toolsPanel).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(toolsPanel).toHaveAttribute('role', 'toolbar');
    });

    test('tools panel has aria-label "Image tools"', async ({ page }) => {
      const toolsPanel = page.locator('wiki-image .tools-panel');
      await expect(toolsPanel).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(toolsPanel).toHaveAttribute('aria-label', 'Image tools');
    });

    test('img element has tabindex="0" making it keyboard-focusable', async ({ page }) => {
      const img = page.locator('wiki-image img');
      await expect(img).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(img).toHaveAttribute('tabindex', '0');
    });

    test('pressing Enter on image sets tools-open attribute', async ({ page }) => {
      const img = page.locator('wiki-image img');
      const wikiImage = page.locator('wiki-image');
      await expect(img).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await img.focus();
      await img.press('Enter');
      await expect(wikiImage).toHaveAttribute('tools-open', '', {
        timeout: TOOLS_INTERACTION_TIMEOUT_MS,
      });
    });

    test('pressing Space on image sets tools-open attribute', async ({ page }) => {
      const img = page.locator('wiki-image img');
      const wikiImage = page.locator('wiki-image');
      await expect(img).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await img.focus();
      await img.press('Space');
      await expect(wikiImage).toHaveAttribute('tools-open', '', {
        timeout: TOOLS_INTERACTION_TIMEOUT_MS,
      });
    });
  });

  test.describe('Mobile behavior (toolsOpen toggle)', () => {
    test.beforeEach(async ({ page }) => {
      await navigateToViewPage(page);
    });

    test('clicking image sets tools-open attribute', async ({ page }) => {
      const wikiImage = page.locator('wiki-image');
      await expect(wikiImage).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await page.locator('wiki-image img').click();
      await expect(wikiImage).toHaveAttribute('tools-open', '', {
        timeout: TOOLS_INTERACTION_TIMEOUT_MS,
      });
    });

    test('tools-open attribute is removed when clicking outside', async ({ page }) => {
      const wikiImage = page.locator('wiki-image');
      await page.locator('wiki-image img').click();
      await expect(wikiImage).toHaveAttribute('tools-open', '', {
        timeout: TOOLS_INTERACTION_TIMEOUT_MS,
      });

      await page.evaluate(() => {
        document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      });

      await expect(wikiImage).not.toHaveAttribute('tools-open', {
        timeout: TOOLS_INTERACTION_TIMEOUT_MS,
      });
    });

    test('tools-open attribute remains set on second click on image', async ({ page }) => {
      // Component uses click-inside to open; close-bar X closes on mobile.
      // A second click on the image should keep tools open (not toggle off).
      const wikiImage = page.locator('wiki-image');
      await page.locator('wiki-image img').click();
      await expect(wikiImage).toHaveAttribute('tools-open', '', {
        timeout: TOOLS_INTERACTION_TIMEOUT_MS,
      });

      // Second click on image — tools should remain open
      await wikiImage.locator('img').click();
      await expect(wikiImage).toHaveAttribute('tools-open', '', {
        timeout: TOOLS_INTERACTION_TIMEOUT_MS,
      });
    });
  });
});
