/**
 * E2E tests for Switch and Select page-app components (issue #1320, Slice 2).
 *
 * Switch(label?, checked, onChange) renders a boolean toggle bound to a
 * $variable. Select(label?, value, onChange, options) renders a dropdown
 * (existed before; this spec adds e2e coverage for it too).
 */
import { expect, test } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_switch_select';

const PAGE_LOAD_TIMEOUT_MS = 15000;

function pageBody(program: string): string {
 return `\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('Switch component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page app contains Switch', () => {
  test('renders the switch with label in unchecked state', async ({ page, request }) => {
   const program = [
    '$struggled = false',
    'sw = Switch("Struggled", $struggled, @Set($struggled, $value))',
    'root = Card("Leg Hold", sw)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Leg Hold' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // The switch label renders
   await expect(host.locator('.wiki-page-app-switch-label', { hasText: 'Struggled' })).toBeVisible();

   // The checkbox is unchecked (initial state = false)
   await expect(host.locator('.wiki-page-app-switch-input')).not.toBeChecked();
  });

  test('toggles checked state when the switch is clicked', async ({ page, request }) => {
   const program = [
    '$struggled = false',
    'sw = Switch("Struggled", $struggled, @Set($struggled, $value))',
    'status = Text("struggled: " + $struggled)',
    'root = Card("Toggle Test", Stack([sw, status]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Toggle Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Initially unchecked
   await expect(host.locator('.wiki-page-app-switch-input')).not.toBeChecked();

   // Click to toggle on
   await host.locator('.wiki-page-app-switch-input').click();

   // Switch is now checked and state updates
   await expect(host.locator('.wiki-page-app-switch-input')).toBeChecked({ timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-text', { hasText: 'struggled: true' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });

  test('renders switch with initial checked=true', async ({ page, request }) => {
   const program = [
    '$active = true',
    'sw = Switch("Active", $active, @Set($active, $value))',
    'root = Card("Switch Init Test", sw)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Switch Init Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   await expect(host.locator('.wiki-page-app-switch-input')).toBeChecked();
  });
 });
});

test.describe.serial('Select component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page app contains Select', () => {
  test('renders select with options and a label', async ({ page, request }) => {
   const program = [
    '$routine = "strength"',
    'picker = Select("Routine type", $routine, @Set($routine, $value), ["strength", "mobility", "cardio"])',
    'root = Card("Workout Picker", picker)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Workout Picker' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Label renders
   await expect(host.locator('.wiki-page-app-field-label', { hasText: 'Routine type' })).toBeVisible();

   // The select element renders with options
   const select = host.locator('.wiki-page-app-select');
   await expect(select).toBeVisible();
   await expect(select.locator('option')).toHaveCount(3);
  });

  test('updates bound $variable when a new option is selected', async ({ page, request }) => {
   const program = [
    '$routine = "strength"',
    'picker = Select("Routine", $routine, @Set($routine, $value), ["strength", "mobility", "cardio"])',
    'label = Text("selected: " + $routine)',
    'root = Card("Select Test", Stack([picker, label]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Select Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Initially shows "strength"
   await expect(host.locator('.wiki-page-app-text', { hasText: 'selected: strength' })).toBeVisible();

   // Select "mobility"
   await host.locator('.wiki-page-app-select').selectOption('mobility');

   // State updates
   await expect(host.locator('.wiki-page-app-text', { hasText: 'selected: mobility' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });
 });
});
