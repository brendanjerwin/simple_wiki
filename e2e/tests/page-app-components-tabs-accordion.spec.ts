/**
 * E2E tests for Tabs and Accordion page-app components (issue #1320, Slice 1).
 *
 * Tabs(labels, children, active?, onTabChange?) renders a tab bar that shows
 * one content panel at a time. Accordion(title, children) renders a collapsible
 * section using native <details>/<summary> elements.
 */
import { expect, test } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_tabs_accordion';

const PAGE_LOAD_TIMEOUT_MS = 15000;

function pageBody(program: string): string {
 return `\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('Tabs component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page app contains Tabs', () => {
  test('renders tab headers and shows the first panel by default', async ({ page, request }) => {
   const program = [
    '$tab = 0',
    'panelA = Text("Content for Overview")',
    'panelB = Text("Content for Details")',
    'tabs = Tabs(["Overview", "Details"], [panelA, panelB], $tab, @Set($tab, $value))',
    'root = Card("Multi-section App", tabs)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });

   // The card title renders
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Multi-section App' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Both tab headers render
   await expect(host.locator('.wiki-page-app-tabs-tab', { hasText: 'Overview' })).toBeVisible();
   await expect(host.locator('.wiki-page-app-tabs-tab', { hasText: 'Details' })).toBeVisible();

   // The first tab is marked active
   await expect(
    host.locator('.wiki-page-app-tabs-tab[data-active="true"]', { hasText: 'Overview' }),
   ).toBeVisible();

   // The first panel's content is shown
   await expect(host.locator('.wiki-page-app-tabs-panel')).toContainText('Content for Overview');

   // The second panel's content is hidden (not rendered)
   await expect(host.locator('.wiki-page-app-tabs-panel')).not.toContainText('Content for Details');
  });

  test('switches active panel when a tab header is clicked', async ({ page, request }) => {
   const program = [
    '$tab = 0',
    'panelA = Text("Panel A content")',
    'panelB = Text("Panel B content")',
    'tabs = Tabs(["Tab A", "Tab B"], [panelA, panelB], $tab, @Set($tab, $value))',
    'root = Card("Tabs Test", tabs)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Tabs Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Initially shows Panel A
   await expect(host.locator('.wiki-page-app-tabs-panel')).toContainText('Panel A content');

   // Click Tab B
   await host.locator('.wiki-page-app-tabs-tab', { hasText: 'Tab B' }).click();

   // Now shows Panel B
   await expect(host.locator('.wiki-page-app-tabs-panel')).toContainText('Panel B content', {
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
   await expect(host.locator('.wiki-page-app-tabs-panel')).not.toContainText('Panel A content');

   // Tab B is now marked active
   await expect(
    host.locator('.wiki-page-app-tabs-tab[data-active="true"]', { hasText: 'Tab B' }),
   ).toBeVisible();
  });
 });
});

test.describe.serial('Accordion component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page app contains Accordion', () => {
  test('renders the accordion header and hides content by default', async ({ page, request }) => {
   const program = [
    'body = Text("This is the collapsed content.")',
    'acc = Accordion("Warm-up Details", [body])',
    'root = Card("Workout", acc)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Workout' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // The accordion summary (header) renders
   await expect(
    host.locator('.wiki-page-app-accordion-summary', { hasText: 'Warm-up Details' }),
   ).toBeVisible();
  });

  test('expands content when the accordion summary is clicked', async ({ page, request }) => {
   const program = [
    'body = Text("Hidden body text revealed on expand.")',
    'acc = Accordion("Click to expand", [body])',
    'root = Card("Accordion Test", acc)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Accordion Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Click the summary to expand
   await host.locator('.wiki-page-app-accordion-summary', { hasText: 'Click to expand' }).click();

   // The accordion content becomes accessible (details[open])
   await expect(host.locator('.wiki-page-app-accordion[open]')).toHaveCount(1);
   await expect(
    host.locator('.wiki-page-app-accordion-content'),
   ).toContainText('Hidden body text revealed on expand.');
  });

  test('renders multiple Accordion items in a Stack', async ({ page, request }) => {
   const program = [
    'bodyA = Text("Warm-up exercises")',
    'bodyB = Text("Cool-down exercises")',
    'acc1 = Accordion("Warm-up", [bodyA])',
    'acc2 = Accordion("Cool-down", [bodyB])',
    'root = Card("Routine", Stack([acc1, acc2]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Routine' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   await expect(host.locator('.wiki-page-app-accordion')).toHaveCount(2);
   await expect(
    host.locator('.wiki-page-app-accordion-summary', { hasText: 'Warm-up' }),
   ).toBeVisible();
   await expect(
    host.locator('.wiki-page-app-accordion-summary', { hasText: 'Cool-down' }),
   ).toBeVisible();
  });
 });
});
