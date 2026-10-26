/**
 * E2E tests for the Tooltip component (issue #1320).
 *
 * Tooltip("hint", [children]) is a pure-CSS wrapper component: it renders
 * a div with data-tooltip and title attributes, then shows a ::after
 * pseudo-element bubble on hover. We verify the DOM structure and that
 * children render inside the wrapper.
 */
import { expect, test } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_tooltip';
const PAGE_LOAD_TIMEOUT_MS = 15000;

const TOOLTIP_PROGRAM = [
 'tip = Tooltip("Hold for 30 seconds", [Button("start left")])',
 'root = Card("Tooltip test", Stack([tip]))',
].join('\n');

function pageBody(program: string): string {
 return `Prose.\n\n\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('Tooltip component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  await setupPage(request, TEST_PAGE, pageBody(TOOLTIP_PROGRAM));
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when the page renders a Tooltip wrapping a Button', () => {
  test('renders the tooltip wrapper with the correct data-tooltip attribute', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });

   // Wait for the card title to confirm the app rendered
   await expect(host.locator('.wiki-page-app-card-title')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   // The tooltip wrapper div must be present with the correct data-tooltip
   const tooltipAttr = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const wrap = host?.shadowRoot?.querySelector('.wiki-page-app-tooltip-wrap');
    return wrap?.getAttribute('data-tooltip') ?? null;
   });
   expect(tooltipAttr).toBe('Hold for 30 seconds');
  });

  test('renders the child Button inside the tooltip wrapper', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   // Button inside the tooltip wrapper
   const buttonText = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const wrap = host?.shadowRoot?.querySelector('.wiki-page-app-tooltip-wrap');
    const button = wrap?.querySelector('button.wiki-page-app-button');
    return button?.textContent?.trim() ?? null;
   });
   expect(buttonText).toBe('start left');
  });

  test('sets the title attribute for keyboard/screen-reader accessibility', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const titleAttr = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const wrap = host?.shadowRoot?.querySelector('.wiki-page-app-tooltip-wrap');
    return wrap?.getAttribute('title') ?? null;
   });
   expect(titleAttr).toBe('Hold for 30 seconds');
  });

  test('keeps the tooltip wrapper shrink-wrapped to its child inside the Stack flex column', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   // align-self: flex-start must shrink-wrap the wrap to the Button's
   // width; left: 50% on the ::after bubble then centers over the
   // trigger rather than over the full card width.
   const widths = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const wrap = host?.shadowRoot?.querySelector('.wiki-page-app-tooltip-wrap');
    const button = wrap?.querySelector('button.wiki-page-app-button');
    if (!wrap || !button) return null;
    return {
     wrapWidthPx: wrap.getBoundingClientRect().width,
     buttonWidthPx: button.getBoundingClientRect().width,
    };
   });
   expect(widths).not.toBeNull();
   expect(widths!.wrapWidthPx).toBeCloseTo(widths!.buttonWidthPx, 0);
  });
 });
});
