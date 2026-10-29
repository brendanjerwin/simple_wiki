/**
 * E2E tests for the NumberInput page-app component (issue #1320, Slice 4).
 *
 * NumberInput(label?, min, max, step, value, onChange) renders a native
 * number input bound to a numeric $variable. It commits on change (not per
 * keystroke), dispatches the coerced numeric $value, and re-renders live
 * from the reactive $variable so @Set/@Reset flow back into the field.
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

   // input[type=number] with fill() dispatches a change event on commit.
   await num.fill('42');

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
