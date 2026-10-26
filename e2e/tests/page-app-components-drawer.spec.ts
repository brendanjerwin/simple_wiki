/**
 * E2E tests for the Drawer page-app component (issue #1320, Slice 5).
 *
 * Drawer(open, placement?, children, onClose?) renders a slide-in panel from
 * one edge of the viewport when `open` is true. Clicking the backdrop
 * dispatches `onClose`. Supports placements: "right" (default), "left", "bottom".
 */
import { expect, test } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_drawer';

const PAGE_LOAD_TIMEOUT_MS = 15000;

function pageBody(program: string): string {
 return `\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('Drawer component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page app contains Drawer', () => {
  test('does not render the drawer panel when open=false', async ({ page, request }) => {
   const program = [
    '$open = false',
    'content = Text("Drawer content")',
    'drawer = Drawer($open, "right", [content], @Set($open, false))',
    'root = Card("App", Stack([Button("Open drawer", @Set($open, true)), drawer]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'App' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Drawer backdrop should not be in DOM when closed
   await expect(host.locator('.wiki-page-app-drawer-backdrop')).toHaveCount(0);
  });

  test('renders the drawer panel when triggered open', async ({ page, request }) => {
   const program = [
    '$open = false',
    'details = Stack([Text("Detail content", "title"), Text("Exercise: squats 3×12")])',
    'drawer = Drawer($open, "right", [details], @Set($open, false))',
    'root = Card("Routine", Stack([Button("View details", @Set($open, true)), drawer]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Routine' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Click to open
   await host.locator('.wiki-page-app-button', { hasText: 'View details' }).click();

   // Drawer renders with content
   await expect(host.locator('.wiki-page-app-drawer-backdrop')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
   await expect(host.locator('.wiki-page-app-drawer[data-placement="right"]')).toBeVisible();
   await expect(host.locator('.wiki-page-app-drawer-body')).toContainText('Exercise: squats 3×12');
  });

  test('closes when the backdrop is clicked', async ({ page, request }) => {
   const program = [
    '$open = false',
    'content = Text("Drawer body")',
    'drawer = Drawer($open, "right", [content], @Set($open, false))',
    'root = Card("App", Stack([Button("Open", @Set($open, true)), drawer]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'App' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Open the drawer
   await host.locator('.wiki-page-app-button', { hasText: 'Open' }).click();
   await expect(host.locator('.wiki-page-app-drawer-backdrop')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   // Click the backdrop (not the drawer itself) to close
   await host.locator('.wiki-page-app-drawer-backdrop').click({ position: { x: 10, y: 10 } });

   // Drawer should close
   await expect(host.locator('.wiki-page-app-drawer-backdrop')).toHaveCount(0, {
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });
 });
});
