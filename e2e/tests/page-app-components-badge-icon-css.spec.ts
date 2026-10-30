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
 'dangerBadge = Badge("injured", "danger")',
 'secondaryBadge = Badge("5 min", "secondary")',
 'badgedHeader = Badge("4 sets", "accent")',
 'header = SectionHeader("Workout", [badgedHeader])',
 'root = Card("Badge test", Stack([defaultBadge, dangerBadge, secondaryBadge, header]))',
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
  test('renders each Badge as a span with the expected label', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const labels = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const badges = Array.from(
     host?.shadowRoot?.querySelectorAll('.wiki-page-app-card-body .wiki-page-app-badge') ?? [],
    );
    // Skip the badge nested inside the SectionHeader; it lives below.
    const top = badges.filter((b) => b.closest('.wiki-page-app-section-header') === null);
    return top.map((b) => b.textContent?.trim() ?? null);
   });
   expect(labels).toEqual(['4 sets', 'injured', '5 min']);
  });

  test('sets data-variant only when a variant is passed', async ({ page }) => {
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
     danger: badge('injured')?.getAttribute('data-variant') ?? null,
     secondary: badge('5 min')?.getAttribute('data-variant') ?? null,
    };
   });
   // The default badge renders both bare and variant-marked spans with the
   // same text; assert per-testid-free semantics via ordered selection.
   expect(variants.danger).toBe('danger');
   expect(variants.secondary).toBe('secondary');
  });

  test('renders the default badge without a data-variant attribute', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const bareVariants = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    return Array.from(
     host?.shadowRoot?.querySelectorAll('.wiki-page-app-badge') ?? [],
    )
     .filter((b) => b.textContent?.trim() === '4 sets')
     .filter((b) => b.getAttribute('data-variant') === null)
     .length;
   });
   expect(bareVariants).toBe(1);
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
