/**
 * E2E tests for the Slider page-app component (issue #1320, Slice 3).
 *
 * Slider(label?, min, max, step, value, onChange) renders a native range
 * input bound to a numeric $variable. @Set($var, $value) flows the new
 * number back into state so it survives reloads and re-renders live.
 */
import { expect, test } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_slider';

const PAGE_LOAD_TIMEOUT_MS = 15000;

function pageBody(program: string): string {
 return `\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('Slider component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page app contains Slider', () => {
  test('renders the slider with label at the initial value', async ({ page, request }) => {
   const program = [
    '$sec = 30',
    'sl = Slider("Hold seconds", 5, 120, 5, $sec, @Set($sec, $value))',
    'root = Card("Leg Hold", sl)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Leg Hold' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // The label renders
   await expect(host.locator('.wiki-page-app-field-label', { hasText: 'Hold seconds' })).toBeVisible();

   // Range input attributes reflect min/max/step and the hydrated value
   const slider = host.locator('input.wiki-page-app-slider');
   await expect(slider).toBeVisible();
   await expect(slider).toHaveAttribute('type', 'range');
   await expect(slider).toHaveAttribute('min', '5');
   await expect(slider).toHaveAttribute('max', '120');
   await expect(slider).toHaveAttribute('step', '5');
   await expect(slider).toHaveValue('30');
   await expect(host.locator('.wiki-page-app-slider-value')).toHaveText('30');
  });

  test('updates state and re-renders when the slider is moved', async ({ page, request }) => {
   const program = [
    '$sec = 30',
    'sl = Slider("Hold seconds", 5, 120, 5, $sec, @Set($sec, $value))',
    'readout = Text("configured: " + $sec)',
    'root = Card("Move Test", Stack([sl, readout]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Move Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const slider = host.locator('input.wiki-page-app-slider');
   await expect(slider).toHaveValue('30');

   // input[type=range] rejects fill(); drive the change event directly.
   await slider.evaluate((el) => {
    el.value = '65';
    el.dispatchEvent(new Event('change', { bubbles: true }));
   });

   await expect(host.locator('.wiki-page-app-slider-value')).toHaveText('65', {
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
   await expect(host.locator('.wiki-page-app-text', { hasText: 'configured: 65' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });

  test('applies defaults when min, max, and step are omitted', async ({ page, request }) => {
   const program = [
    '$n = 50',
    'sl = Slider("Amount", undefined, undefined, undefined, $n, @Set($n, $value))',
    'root = Card("Default Test", sl)',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Default Test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const slider = host.locator('input.wiki-page-app-slider');
   await expect(slider).toHaveAttribute('min', '0');
   await expect(slider).toHaveAttribute('max', '100');
   await expect(slider).toHaveAttribute('step', '1');
   await expect(slider).toHaveValue('50');
  });
 });
});
