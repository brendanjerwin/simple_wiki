/**
 * E2E tests for the NumberInput, Textarea, and DatePicker page-app
 * components (issue #1320, Slice 4 + P3).
 *
 * NumberInput(label?, min, max, step, value, onChange) renders a native
 * number input bound to a numeric $variable. It commits on change (not per
 * keystroke), dispatches the coerced numeric $value, and re-renders live
 * from the reactive $variable so @Set/@Reset flow back into the field.
 *
 * Textarea(label?, placeholder?, rows?, value, onChange) renders a multiline
 * text field bound to a string $variable. It commits on change (blur/Enter,
 * not per keystroke), dispatches the committed string $value, and re-renders
 * live from the reactive $variable. Defaults when omitted: rows 4.
 *
 * DatePicker(label?, value, onChange) renders a native date input bound to
 * an ISO (yyyy-mm-dd) $variable. It commits on change (the browser's date
 * picker), dispatches the picked ISO $value, re-renders live from the
 * reactive $variable, and renders empty for missing/invalid values.
 */
import { expect, test } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_number_input';

const PAGE_LOAD_TIMEOUT_MS = 15000;

function pageBody(program: string): string {
 return `\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('NumberInput component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page app contains NumberInput', () => {
  test('renders the number input with label and aria-relevant attributes', async ({ page, request }) => {
   const program = [
    '$reps = 10',
    'num = NumberInput("Reps", 1, 100, 5, $reps, @Set($reps, $value))',
    'root = Card("Strength Log", num)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Strength Log' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // The label renders
   await expect(host.locator('.wiki-page-app-field-label', { hasText: 'Reps' })).toBeVisible();

   // Number input attributes reflect min/max/step and the hydrated value
   const num = host.locator('input.wiki-page-app-number-input');
   await expect(num).toBeVisible();
   await expect(num).toHaveAttribute('type', 'number');
   await expect(num).toHaveAttribute('min', '1');
   await expect(num).toHaveAttribute('max', '100');
   await expect(num).toHaveAttribute('step', '5');
   await expect(num).toHaveValue('10');
  });

  test('commits the coerced numeric value on change and updates state', async ({ page, request }) => {
   const program = [
    '$reps = 10',
    'num = NumberInput("Reps", undefined, undefined, undefined, $reps, @Set($reps, $value))',
    'readout = Text("logged: " + $reps)',
    'root = Card("Commit Test", Stack([num, readout]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Commit Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const num = host.locator('input.wiki-page-app-number-input');
   await expect(num).toHaveValue('10');

   // fill() does not commit reliably here; drive the change event directly
   // (same pattern as the Slider spec).
   await num.evaluate((el) => {
    el.value = '42';
    el.dispatchEvent(new Event('change', { bubbles: true }));
   });
   await num.press('Tab');

   await expect(host.locator('.wiki-page-app-text', { hasText: 'logged: 42' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });

  test('applies default step and unrestricted min/max when omitted', async ({ page, request }) => {
   const program = [
    '$reps = 10',
    'num = NumberInput("Reps", undefined, undefined, undefined, $reps, @Set($reps, $value))',
    'root = Card("Default Test", num)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Default Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const num = host.locator('input.wiki-page-app-number-input');
   await expect(num).toHaveAttribute('step', '1');
   await expect(num).not.toHaveAttribute('min', /./);
   await expect(num).not.toHaveAttribute('max', /./);
   await expect(num).toHaveValue('10');
  });
 });
});

test.describe.serial('Textarea component', () => {
 test.setTimeout(90000);

 const TEXTAREA_TEST_PAGE = 'e2e_page_app_textarea';

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEXTAREA_TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEXTAREA_TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEXTAREA_TEST_PAGE });
 });

 test.describe('when a page app contains Textarea', () => {
  test('renders the textarea with rows and hydrated $variable value', async ({ page, request }) => {
   const program = [
    '$notes = "draft text"',
    'ta = Textarea("Session notes", "how did it feel?", 6, $notes, @Set($notes, $value))',
    'root = Card("Training Journal", ta)',
   ].join('\n');
   await setupPage(request, TEXTAREA_TEST_PAGE, pageBody(program));

   await page.goto(`/${TEXTAREA_TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Training Journal' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   await expect(host.locator('.wiki-page-app-field-label', { hasText: 'Session notes' })).toBeVisible();

   const ta = host.locator('textarea.wiki-page-app-textarea');
   await expect(ta).toBeVisible();
   await expect(ta).toHaveAttribute('rows', '6');
   await expect(ta).toHaveAttribute('placeholder', 'how did it feel?');
   await expect(ta).toHaveValue('draft text');
  });

  test('commits the string value on change and updates state', async ({ page, request }) => {
   const program = [
    '$notes = ""',
    'ta = Textarea("Session notes", undefined, undefined, $notes, @Set($notes, $value))',
    'readout = Text("you wrote: " + $notes)',
    'root = Card("Commit Test", Stack([ta, readout]))',
   ].join('\n');
   await setupPage(request, TEXTAREA_TEST_PAGE, pageBody(program));

   await page.goto(`/${TEXTAREA_TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Commit Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const ta = host.locator('textarea.wiki-page-app-textarea');
   await expect(ta).toHaveValue('');

   // fill() does not commit reliably here; drive the change event directly
   // (same pattern as the Slider spec).
   await ta.evaluate((el) => {
    el.value = 'felt strong today';
    el.dispatchEvent(new Event('change', { bubbles: true }));
   });

   await expect(host.locator('.wiki-page-app-text', { hasText: 'you wrote: felt strong today' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });

  test('applies default rows 4 when omitted', async ({ page, request }) => {
   const program = [
    '$notes = ""',
    'ta = Textarea("Session notes", undefined, undefined, $notes, @Set($notes, $value))',
    'root = Card("Default Rows Test", ta)',
   ].join('\n');
   await setupPage(request, TEXTAREA_TEST_PAGE, pageBody(program));

   await page.goto(`/${TEXTAREA_TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Default Rows Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const ta = host.locator('textarea.wiki-page-app-textarea');
   await expect(ta).toHaveAttribute('rows', '4');
  });
 });
});

test.describe.serial('DatePicker component', () => {
 test.setTimeout(90000);

 const DATEPICKER_TEST_PAGE = 'e2e_page_app_date_picker';

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: DATEPICKER_TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: DATEPICKER_TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: DATEPICKER_TEST_PAGE });
 });

 test.describe('when a page app contains DatePicker', () => {
  test('renders the date input with the hydrated ISO $variable value', async ({ page, request }) => {
   const program = [
    '$due = "2026-01-15"',
    'due = DatePicker("Due date", $due, @Set($due, $value))',
    'root = Card("Sprint Board", due)',
   ].join('\n');
   await setupPage(request, DATEPICKER_TEST_PAGE, pageBody(program));

   await page.goto(`/${DATEPICKER_TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Sprint Board' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   await expect(host.locator('.wiki-page-app-field-label', { hasText: 'Due date' })).toBeVisible();

   const picker = host.locator('input.wiki-page-app-datepicker[type="date"]');
   await expect(picker).toBeVisible();
   await expect(picker).toHaveValue('2026-01-15');
  });

  test('commits the picked ISO date on change and updates state', async ({ page, request }) => {
   const program = [
    '$due = ""',
    'due = DatePicker("Due date", $due, @Set($due, $value))',
    'readout = Text("ends: " + $due)',
    'root = Card("Commit Test", Stack([due, readout]))',
   ].join('\n');
   await setupPage(request, DATEPICKER_TEST_PAGE, pageBody(program));

   await page.goto(`/${DATEPICKER_TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Commit Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const picker = host.locator('input.wiki-page-app-datepicker[type="date"]');
   await expect(picker).toHaveValue('');

   // fill() does not commit reliably for these controls; drive the change
   // event directly (same pattern as the Textarea spec).
   await picker.evaluate((el) => {
    el.value = '2026-03-02';
    el.dispatchEvent(new Event('change', { bubbles: true }));
   });

   await expect(host.locator('.wiki-page-app-text', { hasText: 'ends: 2026-03-02' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });

  test('renders empty for a missing or invalid $variable value', async ({ page, request }) => {
   const program = [
    '$bogus = "not-a-date"',
    'd1 = DatePicker("Invalid", $bogus, @Set($bogus, $value))',
    'd2 = DatePicker("Empty", $empty)',
    'root = Card("Degrade Test", Stack([d1, d2]))',
   ].join('\n');
   await setupPage(request, DATEPICKER_TEST_PAGE, pageBody(program));

   await page.goto(`/${DATEPICKER_TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Degrade Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const pickers = host.locator('input.wiki-page-app-datepicker[type="date"]');
   await expect(pickers).toHaveCount(2);
   await expect(pickers.nth(0)).toHaveValue('');
   await expect(pickers.nth(1)).toHaveValue('');
  });
 });
});
