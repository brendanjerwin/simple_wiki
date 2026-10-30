/**
 * E2E tests for the Badge component (issue #1320).
 *
 * Badge(label, variant?) is a non-interactive inline pill: a span with
 * .wiki-page-app-badge and an optional data-variant attribute reusing the
 * Button emphasis palette plus "accent". We verify the rendered DOM
 * structure, variant attributes, label text, and SectionHeader composition.
 * (Icon/Css components join this spec file as they land in this phase.)
 */
import { expect, test } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_badge';
const PAGE_LOAD_TIMEOUT_MS = 15000;

const BADGE_PROGRAM = [
 'defaultBadge = Badge("4 sets")',
 'primaryBadge = Badge("level 3", "primary")',
 'secondaryBadge = Badge("queued", "secondary")',
 'ghostBadge = Badge("hover me", "ghost")',
 'dangerBadge = Badge("injured", "danger")',
 'accentBadge = Badge("3 done", "accent")',
 'badgedHeader = SectionHeader("Workout", [Badge("4 sets", "accent")])',
 'root = Card("Badge test", Stack([defaultBadge, primaryBadge, secondaryBadge, ghostBadge, dangerBadge, accentBadge, badgedHeader]))',
].join('\n');

test.describe.serial('Badge component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  await setupPage(
   request,
   TEST_PAGE,
   `Prose.\n\n\`\`\`wiki-page-app\n${BADGE_PROGRAM}\n\`\`\`\n`,
  );
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when the page renders Badges with and without variants', () => {
  test('renders the default Badge as a labeled span without data-variant', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const { text, ariaLabel } = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const badges = Array.from(
     host?.shadowRoot?.querySelectorAll('.wiki-page-app-badge') ?? [],
    );
    const badge = badges.find(
     (b) =>
      b.textContent?.trim() === '4 sets' &&
      b.getAttribute('data-variant') === null,
    ) ?? null;
    if (!badge) return { text: null, ariaLabel: null };
    return {
     text: badge.textContent?.trim() ?? null,
     ariaLabel: badge.getAttribute('aria-label'),
    };
   });
   expect(text).toBe('4 sets');
   expect(ariaLabel).toBe('4 sets');
  });

  test('sets data-variant and aria-label for every Badge variant', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const variants = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const badge = (text: string) =>
     Array.from(host?.shadowRoot?.querySelectorAll('.wiki-page-app-badge') ?? [])
      .find((b) => b.textContent?.trim() === text) ?? null;
    return {
     default: badge('4 sets')?.getAttribute('data-variant') ?? null,
     primary: badge('level 3')?.getAttribute('data-variant') ?? null,
     secondary: badge('queued')?.getAttribute('data-variant') ?? null,
     ghost: badge('hover me')?.getAttribute('data-variant') ?? null,
     danger: badge('injured')?.getAttribute('data-variant') ?? null,
     accent: badge('3 done')?.getAttribute('data-variant') ?? null,
    };
   });
   expect(variants.default).toBe(null);
   for (const variant of ['primary', 'secondary', 'ghost', 'danger', 'accent']) {
    expect(variants[variant], variant).toBe(variant);
   }
  });
 });

 test.describe('when a Badge is SectionHeader trailing content', () => {
  test('renders the Badge inside the section header row', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const composed = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const header = host?.shadowRoot?.querySelector('.wiki-page-app-section-header') ?? null;
    const badge = header?.querySelector('.wiki-page-app-badge') ?? null;
    if (!header || !badge) return null;
    return {
     headerTitle: header.querySelector('.wiki-page-app-section-header-title')?.textContent ?? null,
     badgeText: badge.textContent?.trim() ?? null,
     badgeVariant: badge.getAttribute('data-variant'),
     badgeInHeader: badge.closest('.wiki-page-app-section-header') === header,
    };
   });
   expect(composed).not.toBeNull();
   expect(composed!.headerTitle).toBe('Workout');
   expect(composed!.badgeText).toBe('4 sets');
   expect(composed!.badgeVariant).toBe('accent');
   expect(composed!.badgeInHeader).toBe(true);
  });
 });
});
