/**
 * E2E tests for the Checkbox page-app component (issue #1320).
 *
 * Checkbox(label?, checked, onChange) renders a native boolean checkbox
 * bound to a $variable, following the Switch component pattern.
 */
import { expect, test } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_checkbox_radio';

const PAGE_LOAD_TIMEOUT_MS = 15000;

function pageBody(program: string): string {
 return `\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('Checkbox component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page app contains Checkbox', () => {
  test('renders the checkbox with label in unchecked state', async ({ page, request }) => {
   const program = [
    '$done = false',
    'cb = Checkbox("Done", $done, @Set($done, $value))',
    'root = Card("Task", cb)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Task' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // The checkbox label renders
   await expect(host.locator('.wiki-page-app-checkbox-label', { hasText: 'Done' })).toBeVisible();

   // The checkbox is unchecked (initial state = false)
   await expect(host.locator('.wiki-page-app-checkbox-input')).not.toBeChecked();
  });

  test('toggles checked state when the checkbox is clicked', async ({ page, request }) => {
   const program = [
    '$done = false',
    'cb = Checkbox("Done", $done, @Set($done, $value))',
    'status = Text("done: " + $done)',
    'root = Card("Toggle Test", Stack([cb, status]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Toggle Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Initially unchecked
   await expect(host.locator('.wiki-page-app-checkbox-input')).not.toBeChecked();

   // Click to toggle on
   await host.locator('.wiki-page-app-checkbox-input').click();

   // Checkbox is now checked and state updates
   await expect(host.locator('.wiki-page-app-checkbox-input')).toBeChecked({ timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-text', { hasText: 'done: true' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });

  test('renders checkbox with initial checked=true', async ({ page, request }) => {
   const program = [
    '$done = true',
    'cb = Checkbox("Done", $done, @Set($done, $value))',
    'root = Card("Precheck Test", cb)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Precheck Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   await expect(host.locator('.wiki-page-app-checkbox-input')).toBeChecked();
  });

  test('unchecks a pre-checked checkbox and updates state', async ({ page, request }) => {
   const program = [
    '$done = true',
    'cb = Checkbox("Done", $done, @Set($done, $value))',
    'status = Text("done: " + $done)',
    'root = Card("Uncheck Test", Stack([cb, status]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Uncheck Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Initially checked
   await expect(host.locator('.wiki-page-app-checkbox-input')).toBeChecked();

   // Click to toggle off
   await host.locator('.wiki-page-app-checkbox-input').click();

   // Checkbox is now unchecked and state updates
   await expect(host.locator('.wiki-page-app-checkbox-input')).not.toBeChecked({ timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-text', { hasText: 'done: false' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });
 });
});
