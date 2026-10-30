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

test.describe.serial('Radio component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page app contains Radio', () => {
  test('renders the radio group with all options and initial selection', async ({ page, request }) => {
   const program = [
    '$size = "md"',
    'r = Radio("Size", ["sm", "md", "lg"], $size, @Set($size, $value))',
    'root = Card("Picker", r)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Picker' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // The group label renders
   await expect(host.locator('.wiki-page-app-radio-group-label', { hasText: 'Size' })).toBeVisible();

   // All three options render as radio inputs
   const inputs = host.locator('.wiki-page-app-radio-input');
   await expect(inputs).toHaveCount(3);

   // Only the initial selection is checked
   await expect(inputs.nth(1)).toBeChecked();
   await expect(inputs.nth(0)).not.toBeChecked();
   await expect(inputs.nth(2)).not.toBeChecked();
  });

  test('updates selected state when a different option is clicked', async ({ page, request }) => {
   const program = [
    '$size = "sm"',
    'r = Radio("Size", ["sm", "md", "lg"], $size, @Set($size, $value))',
    'status = Text("picked: " + $size)',
    'root = Card("Switch Test", Stack([r, status]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Switch Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const inputs = host.locator('.wiki-page-app-radio-input');

   // Initially the first option is selected
   await expect(inputs.nth(0)).toBeChecked();
   await expect(host.locator('.wiki-page-app-text', { hasText: 'picked: sm' })).toBeVisible();

   // Click the third option
   await inputs.nth(2).click();

   // Selection moved and state updated
   await expect(inputs.nth(2)).toBeChecked({ timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(inputs.nth(0)).not.toBeChecked();
   await expect(host.locator('.wiki-page-app-text', { hasText: 'picked: lg' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });
 });
});
