/**
 * E2E tests for the Badge component (issue #1320).
 *
 * Badge(label, variant?) is a non-interactive inline pill: a span with
 * .wiki-page-app-badge and an optional data-variant attribute reusing the
 * Button emphasis palette plus "accent". We verify the rendered DOM
 * structure, variant attributes, label text, and SectionHeader composition.
 * (Icon/Css components join this spec file as they land in this phase.)
 * Icon coverage added here: DOM shape, aria-hidden, inline SVG contract,
 * and Stack-row composition.
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

test.describe.serial('Icon component', () => {
 test.setTimeout(90000);

 const ICON_PROGRAM = [
  'checkIcon = Icon("check")',
  'closeIcon = Icon("close")',
  'infoIcon = Icon("info")',
  'warnIcon = Icon("warn")',
  'errorIcon = Icon("error")',
  'chevronLeftIcon = Icon("chevron-left")',
  'chevronRightIcon = Icon("chevron-right")',
  'arrowLeftIcon = Icon("arrow-left")',
  'arrowRightIcon = Icon("arrow-right")',
  'playIcon = Icon("play")',
  'pauseIcon = Icon("pause")',
  'calendarIcon = Icon("calendar")',
  'clockIcon = Icon("clock")',
  'userIcon = Icon("user")',
  'imageIcon = Icon("image")',
  'doneRow = Stack([checkIcon, Text("complete")], undefined, "sm")',
  'root = Card("Icon test", Stack([checkIcon, closeIcon, infoIcon, warnIcon, errorIcon, chevronLeftIcon, chevronRightIcon, arrowLeftIcon, arrowRightIcon, playIcon, pauseIcon, calendarIcon, clockIcon, userIcon, imageIcon, doneRow]))',
 ].join('\n');

 test.describe('when the page renders icons from the curated registry', () => {
  test.beforeEach(async ({ request }) => {
   await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
   await setupPage(
    request,
    TEST_PAGE,
    `Prose.\n\n\`\`\`wiki-page-app\n${ICON_PROGRAM}\n\`\`\`\n`,
   );
  });

  test.afterAll(async ({ request }) => {
   await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
   await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  });

  test('renders every curated icon as an aria-hidden span with inline SVG and no external references', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const { icons, externalRefs } = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const spans = Array.from(host?.shadowRoot?.querySelectorAll('span.wiki-page-app-icon') ?? []);
    let external = false;
    for (const span of spans) {
     const svg = span.querySelector('svg');
     if (!svg) continue;
     const markup = span.innerHTML;
     if (/https?:|url\(/i.test(markup)) external = true;
    }
    return {
     icons: spans.map((span) => {
      const svg = span.querySelector('svg');
      return {
       icon: span.getAttribute('data-icon'),
       hidden: span.getAttribute('aria-hidden'),
       hasSvg: svg !== null,
       viewBox: svg?.getAttribute('viewBox') ?? null,
       stroke: svg?.getAttribute('stroke') ?? null,
       shapes: svg?.querySelectorAll('path,circle,rect,line,polygon').length ?? 0,
      };
     }),
     externalRefs: external,
    };
   });
   expect(externalRefs).toBe(false);
   const expectedIcons = [
    'check', 'close', 'info', 'warn', 'error', 'chevron-left',
    'chevron-right', 'arrow-left', 'arrow-right', 'play', 'pause',
    'calendar', 'clock', 'user', 'image',
   ];
   // 14 top-level icons in program order, plus `check` again inside the
   // composed Stack row.
   expect(icons.length).toBe(expectedIcons.length + 1);
   expect(icons.slice(0, expectedIcons.length).map((i) => i.icon)).toEqual(expectedIcons);
   expect(icons[expectedIcons.length].icon).toBe('check');
   for (const icon of icons) {
    expect(icon.hidden, icon.icon).toBe('true');
    expect(icon.hasSvg, icon.icon).toBe(true);
    expect(icon.viewBox, icon.icon).toBe('0 0 24 24');
    expect(icon.stroke, icon.icon).toBe('currentColor');
    expect(icon.shapes, icon.icon).toBeGreaterThan(0);
   }
  });

  test('composes an icon into a Stack row with its label text', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(host.locator('.wiki-page-app-card-title')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const row = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const stacks = Array.from(host?.shadowRoot?.querySelectorAll('.wiki-page-app-stack') ?? []);
    const rowStack = stacks.find(
     (s) =>
      s.parentElement?.classList.contains('wiki-page-app-stack') &&
      s.querySelector('span.wiki-page-app-icon') !== null,
    ) ?? null;
    if (!rowStack) return null;
    const span = rowStack.querySelector('span.wiki-page-app-icon')!;
    const rect = span.getBoundingClientRect();
    const svg = span.querySelector('svg');
    return {
     iconStroke: svg?.getAttribute('stroke') ?? null,
     text: rowStack.textContent?.trim().replace(/\s+/g, ' ') ?? null,
     paths: svg?.querySelectorAll('path,circle,rect,line,polygon').length ?? 0,
     widthPx: rect.width,
     heightPx: rect.height,
    };
   });
   expect(row).not.toBeNull();
   expect(row!.iconStroke).toBe('currentColor');
   expect(row!.text).toContain('complete');
   expect(row!.paths).toBeGreaterThan(0);
   expect(row!.widthPx).toBeGreaterThan(0);
   expect(row!.widthPx).toBeCloseTo(row!.heightPx, 0);
  });
 });
});

test.describe.serial('Icon component — unknown name', () => {
 test.setTimeout(90000);
 const BAD_ICON_PROGRAM = [
  'okIcon = Icon("check")',
  'badIcon = Icon("sparkly")',
  'root = Card("Icon test", Stack([okIcon, badIcon, Text("still renders")]))',
 ].join('\n');

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  await setupPage(
   request,
   TEST_PAGE,
   `Prose.\n\n\`\`\`wiki-page-app\n${BAD_ICON_PROGRAM}\n\`\`\`\n`,
  );
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test('surfaces the parse error and drops only the bad statement', async ({ page }) => {
  await page.goto(`/${TEST_PAGE}/view`);

  const host = page.locator('wiki-page-app-host');
  await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
  await expect(host.locator('.wiki-page-app-card-title')).toBeVisible({
   timeout: PAGE_LOAD_TIMEOUT_MS,
  });

  const { errorText, icons, stackText } = await page.evaluate(() => {
   const host = document.querySelector('wiki-page-app-host');
   const errorPanel =
    host?.shadowRoot?.querySelector('.wiki-page-app-parse-errors') ?? null;
   const icons = Array.from(
    host?.shadowRoot?.querySelectorAll('span.wiki-page-app-icon') ?? [],
   ).map((span) => span.getAttribute('data-icon'));
   return {
    errorText: errorPanel?.textContent?.trim() ?? null,
    icons,
    stackText:
     host?.shadowRoot?.querySelector('.wiki-page-app-stack')?.textContent?.trim()
     ?? null,
   };
  });
  expect(errorText).not.toBeNull();
  expect(errorText).toContain('sparkly');
  expect(icons).toEqual(['check']);
  expect(stackText).toContain('still renders');
 });
});
