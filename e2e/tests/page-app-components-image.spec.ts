/**
 * E2E tests for the Image page-app component (issue #1320, PLAN-1320 B7
 * imagery).
 *
 * Image(src, alt?, width?, height?, fit?) renders a content <img> from a
 * validated URL: a FileStorageService upload URL (`/uploads/<hash>?...`) or a
 * fully-qualified https:// URL. http://, javascript:, data:, and
 * protocol-relative URLs never fetch — they render an eager data-broken
 * fallback instead. alt governs decorative vs informative semantics
 * (aria-hidden), fit maps onto object-fit via data-fit, and a real upload
 * decodes without data-broken.
 */
import { expect, test, type APIRequestContext } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_image';
const PAGE_LOAD_TIMEOUT_MS = 15000;

// A 1×1 transparent PNG so the image <img> actually decodes.
const PNG_BASE64 =
 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';

async function uploadImage(request: APIRequestContext): Promise<string> {
 const resp = await callAPI(request, 'FileStorageService', 'UploadFile', {
  content: PNG_BASE64,
  filename: 'image-spec.png',
 });
 const json = (resp.json ?? {}) as { uploadUrl?: string; upload_url?: string };
 const uploadUrl = json.uploadUrl ?? json.upload_url;
 expect(uploadUrl, `UploadFile response: ${resp.text}`).toBeTruthy();
 return uploadUrl as string;
}

function imageProgram(uploadUrl: string): string {
 return [
  `uploadImage = Image("${uploadUrl}", "uploaded hero", 240, 240, "cover")`,
  'httpsImage = Image("https://example.com/hero.png", "remote hero", 320, 180, "contain")',
  'decorative = Image("/uploads/sha256-AAAA?filename=ribbon.png")',
  'brokenUpload = Image("/uploads/sha256-notarealhash", "missing")',
  'unsafe = Image("javascript:alert(1)", "must never fetch")',
  'root = Card("Image test", Stack([uploadImage, httpsImage, decorative, brokenUpload, unsafe]))',
 ].join('\n');
}

function pageBody(program: string): string {
 return `Prose.\n\n\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('Image component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  const uploadUrl = await uploadImage(request);
  await setupPage(request, TEST_PAGE, pageBody(imageProgram(uploadUrl)));
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page renders images in one card', () => {
  test('renders a FileStorageService upload image as an <img> without data-broken', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Image test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const wrapper = host.locator('.wiki-page-app-image', { has: page.locator('img[alt="uploaded hero"]') });
   await expect(wrapper).toHaveCount(1);
   await expect(wrapper).not.toHaveAttribute('data-broken');
   const src = await wrapper.locator('img.wiki-page-app-image-img').getAttribute('src');
   expect(src).toContain('/uploads/');
  });

  test('renders a fully-qualified https image src', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Image test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const wrapper = host.locator('.wiki-page-app-image', { has: page.locator('img[alt="remote hero"]') });
   await expect(wrapper).toHaveCount(1);
   await expect(
    wrapper.locator('img.wiki-page-app-image-img'),
   ).toHaveAttribute('src', 'https://example.com/hero.png');
  });

  test('stamps data-fit and native width/height for explicit dimensions', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Image test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const coverImage = host.locator('.wiki-page-app-image', { has: page.locator('img[alt="uploaded hero"]') });
   await expect(coverImage).toHaveAttribute('data-fit', 'cover');
   await expect(coverImage.locator('img.wiki-page-app-image-img')).toHaveAttribute('width', '240');
   await expect(coverImage.locator('img.wiki-page-app-image-img')).toHaveAttribute('height', '240');

   const containImage = host.locator('.wiki-page-app-image', { has: page.locator('img[alt="remote hero"]') });
   await expect(containImage).toHaveAttribute('data-fit', 'contain');
  });

  test('treats an empty alt as a decorative aria-hidden image', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Image test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const decorative = host.locator('.wiki-page-app-image', { has: page.locator('img[alt=""]') });
   await expect(decorative).toHaveCount(1);
   await expect(decorative).toHaveAttribute('aria-hidden', 'true');
  });

  test('falls back and stamps data-broken when an upload URL fails to load', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Image test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const broken = host.locator('.wiki-page-app-image', { has: page.locator('img[alt="missing"]') });
   await expect(broken).toHaveAttribute('data-broken', 'true', { timeout: PAGE_LOAD_TIMEOUT_MS });
   // The variant glyph fallback shows underneath the hidden <img>.
   await expect(broken.locator('.wiki-page-app-image-fallback')).toBeVisible();
  });

  test('drops unsafe scheme srcs without fetching and renders the eager broken fallback', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Image test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const unsafe = host.locator('.wiki-page-app-image', { has: page.locator('img[alt="must never fetch"]') });
   await expect(unsafe).toHaveAttribute('data-broken', 'true', { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(unsafe.locator('img')).toHaveCount(0);
   // No <img> anywhere fetched the javascript: URL (the fallback never
   // fetches; the literal program text inside the fence is not a fetch).
   await expect(host.locator('img[src^="javascript:"]')).toHaveCount(0);
  });
 });
});
