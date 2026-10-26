/**
 * E2E: bare-app URL mode and per-page-app PWA manifest (issue #1324).
 *
 * /<page>/bare renders only the page app — no wiki chrome, no prose —
 * and the bare shell links a per-page manifest whose start_url is the
 * bare URL so the app installs as its own standalone surface.
 */
import { expect, test, type APIRequestContext } from '@playwright/test';
import { callAPI, counterProgram, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_bare';
const PAGE_LOAD_TIMEOUT_MS = 15000;

test.describe.serial('Bare app mode', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await setupPage(
   request,
   TEST_PAGE,
   `Prose that must not appear in bare mode.\n\n\`\`\`wiki-page-app\n${counterProgram('BareCounter', '$n')}\n\`\`\`\n`,
  );
 });

 test('/bare renders the app without wiki chrome or prose', async ({ page }) => {
  const response = await page.goto(`/${TEST_PAGE}/bare`);
  expect(response?.status()).toBe(200);

  const host = page.locator('wiki-page-app-host');
  await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });

  // No wiki chrome anywhere in the document.
  await expect(page.locator('#menu')).toHaveCount(0);
  await expect(page.locator('wiki-search')).toHaveCount(0);
  await expect(page.locator('page-chat-panel')).toHaveCount(0);
  await expect(page.locator('system-info')).toHaveCount(0);
  await expect(page.locator('page-auto-refresh')).toHaveCount(0);

  // No prose rendering either.
  await expect(page.getByText('Prose that must not appear in bare mode.')).toHaveCount(0);

  // Install affordance present in the shell.
  await expect(page.locator('wiki-pwa-install[bare]')).toHaveCount(1);
 });

 test('/bare is served with Cache-Control: no-store', async ({ request }) => {
  const response = await request.get(`/${TEST_PAGE}/bare`);
  expect(response.status()).toBe(200);
  expect(response.headers()['cache-control']).toBe('no-store');
 });

 test('/bare on a page without a page app is a 404', async ({ request }) => {
  await setupPage(request, 'e2e_page_app_no_app', 'Just plain prose.\n');
  const response = await request.get('/e2e_page_app_no_app/bare');
  expect(response.status()).toBe(404);
  const manifestless = await request.get('/manifest.json?page=e2e_page_app_no_app');
  expect(manifestless.status()).toBe(200); // manifest is name/start_url only; installability is gated by the shell
 });

 test('/bare on an unknown page is a 404', async ({ request }) => {
  const response = await request.get('/e2e_page_app_never_exist/bare');
  expect(response.status()).toBe(404);
 });

 test('manifest.json for the page has the bare start_url and standalone display', async ({
  request,
 }) => {
  const response = await request.get(`/manifest.json?page=${TEST_PAGE}`);
  expect(response.status()).toBe(200);
  const manifest = (await response.json()) as {
   name: string;
   start_url: string;
   scope: string;
   display: string;
  };
  expect(manifest.name).toBe(TEST_PAGE);
  expect(manifest.start_url).toBe(`/${TEST_PAGE}/bare`);
  expect(manifest.scope).toBe(`/${TEST_PAGE}/bare/`);
  expect(manifest.display).toBe('standalone');
 });

 test('the bare shell links its own manifest, not the site-wide one', async ({ page }) => {
  await page.goto(`/${TEST_PAGE}/bare`);
  const manifestHref = await page
   .locator('link[rel="manifest"]')
   .getAttribute('href', { timeout: PAGE_LOAD_TIMEOUT_MS });
  expect(manifestHref).toBe(`/manifest.json?page=${TEST_PAGE}`);
 });
});