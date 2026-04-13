import { test, expect } from '@playwright/test';

// Timeouts
const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;

const TEST_PAGE = 'e2e-collapsible-heading-a11y-test';

const TEST_CONTENT = `+++
identifier = "${TEST_PAGE}"
title = "Collapsible Heading A11y E2E Test"
+++

#^ Accessibility Test Section

Content under accessibility test section.

##^ Nested Accessibility Section

Nested section content.
`;

test.describe('Collapsible Heading Accessibility Attributes', () => {
  // Run serially: tests share a single test page; beforeAll creates it.
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    await page.goto(`/${TEST_PAGE}/edit`);
    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await textarea.fill(TEST_CONTENT);
    await textarea.press('Space');
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });

    await ctx.close();
  });

  test.afterAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    try {
      await page.goto(`/${TEST_PAGE}/edit`);
      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await textarea.fill(`+++\nidentifier = "${TEST_PAGE}"\n+++`);
      await textarea.press('Space');
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });
    } catch (e) {
      // Ignore cleanup failures — test data is non-critical, but log for debugging.
      console.warn('Collapsible heading a11y E2E test cleanup failed:', e);
    }

    await ctx.close();
  });

  test.beforeEach(async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
  });

  test('toggle button has an aria-label starting with "Toggle"', async ({ page }) => {
    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    await expect(toggle).toHaveAttribute('aria-label', /^Toggle /);
  });

  test('aria-label includes the heading text', async ({ page }) => {
    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    await expect(toggle).toHaveAttribute('aria-label', 'Toggle Accessibility Test Section');
  });

  test('toggle button has an aria-controls attribute', async ({ page }) => {
    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    await expect(toggle).toHaveAttribute('aria-controls', /.+/);
  });

  test('aria-controls references an element that exists in the DOM', async ({ page }) => {
    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    const controlsId = await toggle.getAttribute('aria-controls');
    expect(controlsId).toBeTruthy();

    const controlledElement = firstCollapsible.locator(`#${controlsId}`);
    await expect(controlledElement).toBeAttached();
  });

  test('aria-controls value matches the id of the content element', async ({ page }) => {
    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    const content = firstCollapsible.locator('.ch-content').first();

    const controlsId = await toggle.getAttribute('aria-controls');
    const contentId = await content.getAttribute('id');

    expect(controlsId).toBeTruthy();
    expect(contentId).toBeTruthy();
    expect(controlsId).toEqual(contentId);
  });

  test('toggle button has aria-expanded set to false when collapsed', async ({ page }) => {
    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  });

  test('aria-expanded updates to true after expanding', async ({ page }) => {
    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    await toggle.click();

    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });

  test('aria-expanded returns to false after collapsing again', async ({ page }) => {
    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    await toggle.click();
    await toggle.click();

    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  });

  test('Enter key activates the toggle and updates aria-expanded', async ({ page }) => {
    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    await toggle.focus();
    await toggle.press('Enter');

    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });

  test('Space key activates the toggle and updates aria-expanded', async ({ page }) => {
    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    await toggle.focus();
    await toggle.press('Space');

    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });

  test('each collapsible heading has a unique aria-controls id', async ({ page }) => {
    const collapsibles = page.locator('collapsible-heading');
    await expect(collapsibles.first()).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Expand outer section to reveal the nested collapsible heading
    await collapsibles.first().locator('.ch-toggle').first().click();
    await expect(collapsibles.nth(1)).toBeAttached();

    const firstControlsId = await collapsibles.first().locator('.ch-toggle').first().getAttribute('aria-controls');
    const secondControlsId = await collapsibles.nth(1).locator('.ch-toggle').first().getAttribute('aria-controls');

    expect(firstControlsId).toBeTruthy();
    expect(secondControlsId).toBeTruthy();
    expect(firstControlsId).not.toEqual(secondControlsId);
  });

  test('nested heading aria-label includes its own heading text', async ({ page }) => {
    const collapsibles = page.locator('collapsible-heading');
    await expect(collapsibles.first()).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Expand outer section to reveal the nested collapsible heading
    await collapsibles.first().locator('.ch-toggle').first().click();
    await expect(collapsibles.nth(1)).toBeAttached();

    const nestedToggle = collapsibles.nth(1).locator('.ch-toggle').first();
    await expect(nestedToggle).toHaveAttribute('aria-label', 'Toggle Nested Accessibility Section');
  });
});
