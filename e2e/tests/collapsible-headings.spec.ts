import { test, expect } from '@playwright/test';

// Timeouts
const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;

const TEST_PAGE = 'e2e-collapsible-headings-test';

// Markdown content with collapsible and regular headings
const TEST_CONTENT = `+++
identifier = "${TEST_PAGE}"
title = "Collapsible Headings E2E Test"
+++

#^ Section One

Content under section one.

More content under section one.

##^ Nested Section

Content under nested section.

## Regular Subsection

Regular content here.

#^ Section Two

Content under section two.

# Regular Heading

Regular heading content.
`;

test.describe('Collapsible Headings', () => {
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
    } catch {
      // Ignore cleanup failures — test data is non-critical
    }

    await ctx.close();
  });

  test('heading with ^ marker renders with a toggle control', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Use .first() because the outer section's shadow DOM toggle and any nested
    // section toggles are both reachable from the same host element.
    const toggle = firstCollapsible.locator('.ch-toggle').first();
    await expect(toggle).toBeVisible();
  });

  test('collapsible heading is collapsed by default', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    const content = firstCollapsible.locator('.ch-content').first();

    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
    await expect(content).not.toBeVisible();
  });

  test('clicking toggle expands the content under the heading', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    const content = firstCollapsible.locator('.ch-content').first();

    // Initially collapsed
    await expect(content).not.toBeVisible();

    // Expand by clicking the toggle
    await toggle.click();

    await expect(content).toBeVisible();
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
    await expect(page.locator('#rendered')).toContainText('Content under section one');
  });

  test('clicking toggle again collapses the expanded content', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    const content = firstCollapsible.locator('.ch-content').first();

    // Expand
    await toggle.click();
    await expect(content).toBeVisible();

    // Collapse again
    await toggle.click();

    await expect(content).not.toBeVisible();
    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  });

  test('collapsed content has the hidden attribute for screen reader accessibility', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // The .ch-content div must carry the HTML `hidden` attribute when collapsed so
    // assistive technologies skip the concealed content entirely.
    const content = firstCollapsible.locator('.ch-content').first();
    await expect(content).toHaveAttribute('hidden', '');
  });

  test('toggle button is focusable via keyboard (Tab)', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    await toggle.focus();

    await expect(toggle).toBeFocused();
  });

  test('toggle can be activated with the Space key', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    const content = firstCollapsible.locator('.ch-content').first();

    await toggle.focus();
    await toggle.press('Space');

    await expect(content).toBeVisible();
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });

  test('toggle can be activated with the Enter key', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const toggle = firstCollapsible.locator('.ch-toggle').first();
    const content = firstCollapsible.locator('.ch-content').first();

    await toggle.focus();
    await toggle.press('Enter');

    await expect(content).toBeVisible();
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });

  test('expanded state persists across a page reload', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const firstCollapsible = page.locator('collapsible-heading').first();
    await expect(firstCollapsible).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Expand the first collapsible heading
    await firstCollapsible.locator('.ch-toggle').first().click();
    await expect(firstCollapsible.locator('.ch-content').first()).toBeVisible();

    // Reload within the same browser context (localStorage is preserved)
    await page.reload();
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
    await expect(page.locator('collapsible-heading').first()).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const afterReload = page.locator('collapsible-heading').first();
    await expect(afterReload.locator('.ch-toggle').first()).toHaveAttribute('aria-expanded', 'true');
    await expect(afterReload.locator('.ch-content').first()).toBeVisible();
  });

  test('nested collapsible headings collapse and expand independently', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    // Section One is the first top-level collapsible heading; expand it to reveal nested content
    const sectionOne = page.locator('collapsible-heading[heading-level="1"]').first();
    await expect(sectionOne).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await sectionOne.locator('.ch-toggle').first().click();
    await expect(sectionOne.locator('.ch-content').first()).toBeVisible();

    // The nested ##^ section should now be in the DOM and still collapsed
    const nestedSection = sectionOne.locator('collapsible-heading').first();
    await expect(nestedSection).toBeAttached();

    const nestedToggle = nestedSection.locator('.ch-toggle').first();
    const nestedContent = nestedSection.locator('.ch-content').first();

    await expect(nestedToggle).toHaveAttribute('aria-expanded', 'false');
    await expect(nestedContent).not.toBeVisible();

    // Expand the nested section independently
    await nestedToggle.click();

    await expect(nestedContent).toBeVisible();
    await expect(nestedToggle).toHaveAttribute('aria-expanded', 'true');
    // Check text on the component element itself — slotted content lives in the light DOM
    await expect(nestedSection).toContainText('Content under nested section');

    // The outer section should still be expanded
    await expect(sectionOne.locator('.ch-content').first()).toBeVisible();
  });

  test('multiple top-level collapsible headings expand and collapse independently', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const topLevel = page.locator('collapsible-heading[heading-level="1"]');
    await expect(topLevel.first()).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    const sectionCount = await topLevel.count();
    expect(sectionCount).toBeGreaterThanOrEqual(2);

    const sectionOne = topLevel.nth(0);
    const sectionTwo = topLevel.nth(1);

    // Expand only Section One
    await sectionOne.locator('.ch-toggle').first().click();
    await expect(sectionOne.locator('.ch-content').first()).toBeVisible();

    // Section Two must remain collapsed (independent state)
    await expect(sectionTwo.locator('.ch-content').first()).not.toBeVisible();
    await expect(sectionTwo.locator('.ch-toggle').first()).toHaveAttribute('aria-expanded', 'false');

    // Expand Section Two
    await sectionTwo.locator('.ch-toggle').first().click();
    await expect(sectionTwo.locator('.ch-content').first()).toBeVisible();
    // Check text on the component element itself — slotted content lives in the light DOM
    await expect(sectionTwo).toContainText('Content under section two');

    // Section One must still be expanded
    await expect(sectionOne.locator('.ch-content').first()).toBeVisible();
  });

  test('regular headings without ^ marker are not wrapped in collapsible-heading', async ({ page }) => {
    await page.goto(`/${TEST_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    // The "# Regular Heading" in the test content is a plain <h1> — not inside a
    // collapsible-heading component. Elements used as heading slots carry slot="heading";
    // regular headings do not have that attribute.
    const regularH1 = page.locator('h1:not([slot="heading"])');
    await expect(regularH1).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
    await expect(regularH1).toContainText('Regular Heading');

    // A regular heading element must NOT itself contain a toggle button
    await expect(regularH1.locator('.ch-toggle')).not.toBeAttached();
  });
});
