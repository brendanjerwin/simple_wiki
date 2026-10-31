/**
 * E2E tests for the Card page-app component's backgroundImage prop (issue
 * #1320, Slice 5 imagery).
 *
 * Card(title, children, variant?, backgroundImage?) renders a validated URL as
 * a cover-fit CSS backdrop behind the card. Only fully-qualified https:// URLs
 * or wiki /uploads/ URLs are accepted; http://, javascript:, data:, and
 * protocol-relative URLs are silently dropped (no backdrop, no data-bg-image).
 */
import { expect, test } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_card_image';

const PAGE_LOAD_TIMEOUT_MS = 15000;

function pageBody(program: string): string {
 return `\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('Card background image component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page app carries a Card backgroundImage', () => {
  test('renders a validated https URL as the card backdrop', async ({ page, request }) => {
   const program = [
    'root = Card("Hero", [t], undefined, "https://example.com/hero.jpg")',
    't = Text("backdrop behind me")',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Hero' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Card is the program root and lives inside the host's shadow tree.
   // Assert the validated URL on the style attribute — it is the renderer's
   // contract and robust across rerenders (computed url() values get
   // absolutized by the CSSOM, which would break the wiki-relative case).
   const card = host.locator('.wiki-page-app-card');
   await expect(card).toHaveAttribute('data-bg-image', 'true');
   await expect(card).toHaveAttribute('style', /url\("https:\/\/example\.com\/hero\.jpg"\)/);
  });

  test('renders a wiki /uploads/ URL as the card backdrop', async ({ page, request }) => {
   const program = [
    'root = Card("Photo", [t], undefined, "/uploads/sha256-abc123?filename=photo.png")',
    't = Text("uploaded backdrop")',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Photo' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Same attribute-level contract; CSSOM would absolutize the relative URL.
   const card = host.locator('.wiki-page-app-card');
   await expect(card).toHaveAttribute('data-bg-image', 'true');
   await expect(card).toHaveAttribute('style', /url\("\/uploads\/sha256-abc123\?filename=photo\.png"\)/);
  });

  test('drops unsafe scheme URLs instead of rendering a backdrop', async ({ page, request }) => {
   const program = [
    'root = Card("Unsafe", [t], undefined, "javascript:alert(1)")',
    't = Text("still readable")',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Unsafe' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // The card renders normally: no backdrop marker, no background-image layer.
   const card = host.locator('.wiki-page-app-card');
   await expect(card).not.toHaveAttribute('data-bg-image');
   // The base card paints a color-only shorthand background, so the computed
   // background-image resolves to none when no validated URL was emitted.
   const backgroundImage = await card.evaluate((el) => getComputedStyle(el).backgroundImage);
   expect(backgroundImage).toBe('none');
  });

  test('renders a plain card without any backdrop layer when backgroundImage is omitted', async ({ page, request }) => {
   const program = [
    'root = Card("Plain", [t])',
    't = Text("no backdrop")',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'Plain' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const card = host.locator('.wiki-page-app-card');
   await expect(card).not.toHaveAttribute('data-bg-image');
   const backgroundImage = await card.evaluate((el) => getComputedStyle(el).backgroundImage);
   expect(backgroundImage).toBe('none');
  });
 });
});
