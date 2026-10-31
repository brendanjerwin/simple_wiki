/**
 * E2E tests for the Avatar page-app component (issue #1320).
 *
 * Avatar(src?, initials?, size?) renders a circular avatar: `src` accepts a
 * FileStorageService upload URL (`/uploads/<hash>?filename=...`) or an
 * http(s) image URL; absent or failed images fall back to the `initials`
 * text (or a person glyph when there are none). This spec covers a REAL
 * uploaded image (no data-broken), an initials-only fallback, a broken-src
 * fallback (reveal via data-broken), and the explicit size tokens.
 * (Skeleton/List components join this spec file as they land in this phase.)
 */
import { expect, test, type APIRequestContext } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_avatar';
const PAGE_LOAD_TIMEOUT_MS = 15000;

// A 1×1 transparent PNG so the avatar <img> actually decodes.
const PNG_BASE64 =
 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';

async function uploadAvatarPng(request: APIRequestContext): Promise<string> {
 const resp = await callAPI(request, 'FileStorageService', 'UploadFile', {
  content: PNG_BASE64,
  filename: 'avatar.png',
 });
 const json = (resp.json ?? {}) as { uploadUrl?: string; upload_url?: string };
 const uploadUrl = json.uploadUrl ?? json.upload_url;
 expect(uploadUrl, `UploadFile response: ${resp.text}`).toBeTruthy();
 return uploadUrl as string;
}

function avatarProgram(uploadUrl: string): string {
 return [
  `imgAvatar = Avatar("${uploadUrl}", "BJ")`,
  'fallbackAvatar = Avatar(undefined, "TC")',
  'glyphAvatar = Avatar()',
  'brokenAvatar = Avatar("/uploads/sha256-notarealhash", "WX")',
  `tinyAvatar = Avatar("${uploadUrl}", "JJ", "sm")`,
  `largeAvatar = Avatar("${uploadUrl}", "JJ", "lg")`,
  'root = Card("Avatar test", Stack([imgAvatar, fallbackAvatar, glyphAvatar, brokenAvatar, Stack([tinyAvatar, largeAvatar], "horizontal", "sm")]))',
 ].join('\n');
}

function pageBody(program: string): string {
 return `Prose.\n\n\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('Avatar component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  const uploadUrl = await uploadAvatarPng(request);
  await setupPage(request, TEST_PAGE, pageBody(avatarProgram(uploadUrl)));
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page renders avatars in one card', () => {
  test('renders a FileStorageService upload avatar as an <img> without data-broken', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Avatar test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const imgAvatar = host.locator('.wiki-page-app-avatar-fallback', { hasText: 'BJ' })
    .locator('xpath=..');
   await expect(imgAvatar).toBeVisible();
   expect(await imgAvatar.getAttribute('data-broken')).toBe(null);
   const src = await imgAvatar.locator('img.wiki-page-app-avatar-img').getAttribute('src');
   expect(src).toContain('/uploads/');
  });

  test('renders the initials fallback with no <img> when src is absent', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Avatar test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const fallbackAvatar = host.locator('.wiki-page-app-avatar-fallback', { hasText: 'TC' })
    .locator('xpath=..');
   await expect(fallbackAvatar.locator('img')).toHaveCount(0);
   expect(await fallbackAvatar.getAttribute('data-broken')).toBe(null);
  });

  test('renders a person glyph when neither src nor initials is present', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Avatar test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const glyphAvatar = host.locator(
    '.wiki-page-app-avatar-fallback svg',
   ).locator('xpath=../..');
   await expect(glyphAvatar).toBeVisible();
   await expect(glyphAvatar.locator('img')).toHaveCount(0);
  });

  test('falls back to initials and stamps data-broken when the image fails to load', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Avatar test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const brokenAvatar = host.locator('.wiki-page-app-avatar-fallback', { hasText: 'WX' })
    .locator('xpath=..');
   await expect(brokenAvatar).toBeVisible();
   await expect(brokenAvatar).toHaveAttribute('data-broken', 'true', { timeout: PAGE_LOAD_TIMEOUT_MS });
   // The <img> element stays in the DOM but the CSS hides it once the
   // wrapper is data-broken; the initials fallback remains the visible
   // content either way.
   expect(await brokenAvatar.locator('.wiki-page-app-avatar-fallback').textContent()).toBe('WX');
  });

  test('marks data-size for the explicit sm and lg size tokens', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Avatar test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const tinyAvatar = host.locator('.wiki-page-app-avatar-fallback', { hasText: 'JJ' })
    .locator('xpath=..').nth(0);
   await expect(tinyAvatar).toHaveAttribute('data-size', 'sm');
   const largeAvatar = host.locator('.wiki-page-app-avatar-fallback', { hasText: 'JJ' })
    .locator('xpath=..').nth(1);
   await expect(largeAvatar).toHaveAttribute('data-size', 'lg');
  });
 });
});
