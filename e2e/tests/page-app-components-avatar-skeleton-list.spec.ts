/**
 * E2E tests for the Avatar page-app component (issue #1320).
 *
 * Avatar(src?, initials?, size?) renders a circular avatar: `src` accepts a
 * FileStorageService upload URL (`/uploads/<hash>?filename=...`) or an
 * http(s) image URL; absent or failed images fall back to the `initials`
 * text (or a person glyph when there are none). This spec covers a REAL
 * uploaded image (no data-broken), an initials-only fallback, a broken-src
 * fallback (reveal via data-broken), the explicit size tokens, and the
 * Skeleton(kind?, loading?) shimmer placeholders (shape kinds, shimmer
 * CSS wiring, reduced-motion fallback, $loading gate).
 * (Skeleton has landed; List joins this spec file when it lands.)
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

/**
 * Skeleton(kind?, loading?) is a pure-CSS shimmer placeholder: a span
 * with .wiki-page-app-skeleton whose data-kind picks the shape ("text"
 * line by default, "avatar" 32px circle, "card" block). The optional
 * second positional `loading` binds a $boolean (Query-friendly): while
 * it is false the block disappears so real content takes its place. The
 * shimmer is design-system CSS — specs pin the animation wiring and the
 * prefers-reduced-motion fallback.
 */
const SKELETON_TEST_PAGE = 'e2e_page_app_skeleton';

const SKELETON_PROGRAM = [
 'textSkeleton = Skeleton()',
 'lineSkeleton = Skeleton("text")',
 'avatarSkeleton = Skeleton("avatar")',
 'cardSkeleton = Skeleton("card")',
 '$loading = true',
 'gateSkeleton = Skeleton(undefined, $loading)',
 'doneButton = Button("content ready", @Set($loading, false))',
 'refetchButton = Button("refetch", @Set($loading, true))',
 'root = Card("Skeleton test", Stack([Stack([avatarSkeleton, textSkeleton, lineSkeleton, gateSkeleton], "horizontal"), cardSkeleton, doneButton, refetchButton]))',
].join('\n');

test.describe.serial('Skeleton component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: SKELETON_TEST_PAGE });
  await setupPage(
   request,
   SKELETON_TEST_PAGE,
   `Prose.\n\n\`\`\`wiki-page-app\n${SKELETON_PROGRAM}\n\`\`\`\n`,
  );
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: SKELETON_TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: SKELETON_TEST_PAGE });
 });

 test.describe('when a page renders skeleton placeholders', () => {
  test('renders each shape kind as an aria-hidden span with its data-kind', async ({ page }) => {
   await page.goto(`/${SKELETON_TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Skeleton test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const shapes = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const skeletons = Array.from(
     host?.shadowRoot?.querySelectorAll('.wiki-page-app-skeleton') ?? [],
    );
    return skeletons.map((el) => ({
     kind: el.getAttribute('data-kind'),
     hidden: el.getAttribute('aria-hidden'),
    }));
   });
   expect(shapes).toHaveLength(5);
   expect(shapes.filter((s) => s.kind === 'text')).toHaveLength(3);
   expect(shapes.filter((s) => s.kind === 'avatar')).toHaveLength(1);
   expect(shapes.filter((s) => s.kind === 'card')).toHaveLength(1);
   for (const shape of shapes) {
    expect(shape.hidden, JSON.stringify(shape)).toBe('true');
   }
  });

  test('animates the shimmer through the design-system CSS', async ({ page }) => {
   await page.goto(`/${SKELETON_TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Skeleton test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const names = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const skeletons = Array.from(
     host?.shadowRoot?.querySelectorAll('.wiki-page-app-skeleton') ?? [],
    );
    return skeletons.map((el) => getComputedStyle(el).animationName);
   });
   expect(names.length).toBeGreaterThan(0);
   for (const name of names) {
    expect(name).toBe('wiki-page-app-skeleton-shimmer');
   }
  });

  test('stands still under prefers-reduced-motion instead of shimmering', async ({ page }) => {
   await page.emulateMedia({ reducedMotion: 'reduce' });
   await page.goto(`/${SKELETON_TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Skeleton test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const names = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const skeletons = Array.from(
     host?.shadowRoot?.querySelectorAll('.wiki-page-app-skeleton') ?? [],
    );
    return skeletons.map((el) => getComputedStyle(el).animationName);
   });
   expect(names.length).toBeGreaterThan(0);
   for (const name of names) {
    expect(name).toBe('none');
   }
  });

  test('disappears while a bound $loading is false and returns when true', async ({ page }) => {
   await page.goto(`/${SKELETON_TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Skeleton test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   // The gated skeleton renders while $loading is true…
   await expect(host.locator('.wiki-page-app-skeleton')).toHaveCount(5);
   // …vanishes once the action sets $loading = false…
   await host.getByRole('button', { name: 'content ready' }).click();
   await expect(host.locator('.wiki-page-app-skeleton')).toHaveCount(4);
   // …and re-arms when the gate flips back to true (the Query contract).
   await host.getByRole('button', { name: 'refetch' }).click();
   await expect(host.locator('.wiki-page-app-skeleton')).toHaveCount(5);
  });
 });
});
