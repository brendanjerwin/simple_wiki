/**
 * E2E tests for the Skeleton page-app component (issue #1320).
 *
 * Skeleton(kind?, loading?) is a pure-CSS shimmer placeholder: a span with
 * .wiki-page-app-skeleton whose data-kind picks the shape ("text" line by
 * default, "title" shorter block, "card" block, or "circle" 32px circular
 * placeholder matching Avatar md). The optional second positional
 * `loading` binds a $boolean (Query-friendly): while it is false the
 * block disappears so real content takes its place. Specs pin
 * the shape/aria-hidden DOM, the design-system animation wiring, the
 * prefers-reduced-motion fallback, and the @Set-driven $loading gate.
 */
import { expect, test } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_skeleton';
const PAGE_LOAD_TIMEOUT_MS = 15000;

const SKELETON_PROGRAM = [
 'textSkeleton = Skeleton()',
 'lineSkeleton = Skeleton("text")',
 'titleSkeleton = Skeleton("title")',
 'circleSkeleton = Skeleton("circle")',
 'cardSkeleton = Skeleton("card")',
 '$loading = true',
 'gateSkeleton = Skeleton(undefined, $loading)',
 'doneButton = Button("content ready", @Set($loading, false))',
 'refetchButton = Button("refetch", @Set($loading, true))',
 'root = Card("Skeleton test", Stack([Stack([titleSkeleton, circleSkeleton, textSkeleton, lineSkeleton, gateSkeleton], "horizontal"), cardSkeleton, doneButton, refetchButton]))',
].join('\n');

test.describe.serial('Skeleton component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  await setupPage(
   request,
   TEST_PAGE,
   `Prose.\n\n\`\`\`wiki-page-app\n${SKELETON_PROGRAM}\n\`\`\`\n`,
  );
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page renders skeleton placeholders', () => {
  test('renders each shape kind as an aria-hidden span with its data-kind', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

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
     width: Math.round(el.getBoundingClientRect().width),
    }));
   });
   expect(shapes).toHaveLength(6);
   expect(shapes.filter((s) => s.kind === 'text')).toHaveLength(3);
   expect(shapes.filter((s) => s.kind === 'title')).toHaveLength(1);
   expect(shapes.filter((s) => s.kind === 'circle')).toHaveLength(1);
   expect(shapes.filter((s) => s.kind === 'card')).toHaveLength(1);
   for (const shape of shapes) {
    expect(shape.hidden, JSON.stringify(shape)).toBe('true');
   }
   // Geometry guards: the circle must hold 32px inside the horizontal
   // row (flex:none — no squash against the full-width text sibling),
   // and the title block stays narrower than the full-width card.
   const circle = shapes.find((s) => s.kind === 'circle');
   expect(circle?.width, JSON.stringify(circle)).toBe(32);
   const title = shapes.find((s) => s.kind === 'title');
   const card = shapes.find((s) => s.kind === 'card');
   expect(title?.width, JSON.stringify(title)).toBeLessThan(Number(card?.width ?? 0));
  });

  test('animates the shimmer through the design-system CSS', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Skeleton test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   const styles = await page.evaluate(() => {
    const host = document.querySelector('wiki-page-app-host');
    const skeletons = Array.from(
     host?.shadowRoot?.querySelectorAll('.wiki-page-app-skeleton') ?? [],
    );
    return skeletons.map((el) => getComputedStyle(el));
   });
   expect(styles.length).toBeGreaterThan(0);
   for (const style of styles) {
    expect(style.animationName).toBe('wiki-page-app-skeleton-shimmer');
    // animationName alone can pass while the paint resolves to none —
    // pin the sweep gradient so a broken color-mix/var fallback fails
    // loudly instead of rendering a guaranteed flat grey block.
    expect(style.backgroundImage).toContain('linear-gradient');
   }
  });

  test('stands still under prefers-reduced-motion instead of shimmering', async ({ page }) => {
   await page.emulateMedia({ reducedMotion: 'reduce' });
   await page.goto(`/${TEST_PAGE}/view`);

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
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Skeleton test' })).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   // The gated skeleton renders while $loading is true…
   await expect(host.locator('.wiki-page-app-skeleton')).toHaveCount(6);
   // Wait for the state flush the @Set step produces before counting:
   // this separates "the action plan ran" from "the host repainted",
   // so a flaky here reads as an unpainted re-render, not a lost click.
   const readyFlush = page.waitForResponse(
    (resp) => resp.url().includes('PageAppStateService/MergePageAppState'),
   );
   // …vanishes once the action sets $loading = false…
   await host.getByRole('button', { name: 'content ready' }).click();
   await readyFlush;
   await expect(host.locator('.wiki-page-app-skeleton')).toHaveCount(5, { timeout: 10000 });
   // …and re-arms when the gate flips back to true (the Query contract).
   const refetchFlush = page.waitForResponse(
    (resp) => resp.url().includes('PageAppStateService/MergePageAppState'),
   );
   await host.getByRole('button', { name: 'refetch' }).click();
   await refetchFlush;
   await expect(host.locator('.wiki-page-app-skeleton')).toHaveCount(6, { timeout: 10000 });
  });
 });
});
