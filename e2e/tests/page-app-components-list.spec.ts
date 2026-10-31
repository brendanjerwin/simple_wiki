/**
 * E2E tests for the List page-app component (issue #1320).
 *
 * List(items, ordered?) renders a real bulleted or numbered list: `items`
 * is an array of plain strings (or Text children, whose text lands inside
 * the bullets) and the optional second positional `ordered` switches the
 * semantic wrapper from `<ul>` to `<ol>`. Entries that are neither strings
 * nor component children drop at render time. Non-interactive.
 */
import { expect, test } from '@playwright/test';
import { setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_list';
const PAGE_LOAD_TIMEOUT_MS = 15000;

function listProgram(): string {
 return [
  'bulleted = List(["2 leg quarters", "1 tbsp smoked paprika", "3 cloves garlic"])',
  'ordered = List(["Season the chicken", "Sear skin-side down 6 min", "Roast at 425F for 20 min"], true)',
  'textItems = List([Text("low sodium"), "salt", Text("freshly ground pepper")])',
  'garbage = List([4, "flour"])',
  'root = Card("Paprika chicken", Stack([',
  '  SectionHeader("Ingredients", [Text("serves 4", "caption")]),',
  '  bulleted,',
  '  SectionHeader("Steps", [Text("about 30 min", "caption")]),',
  '  ordered,',
  '  textItems,',
  '  garbage,',
  ']))',
 ].join('\n');
}

function pageBody(program: string): string {
 return `Prose.\n\n\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('List component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await setupPage(request, TEST_PAGE, pageBody(listProgram()));
 });

 test.afterAll(async ({ request }) => {
  await setupPage(request, TEST_PAGE, 'Prose.\n');
 });

 test('renders string items as li children of a bulleted list by default', async ({ page }) => {
  await page.goto(`/${TEST_PAGE}/view`);

  const host = page.locator('wiki-page-app-host');
  await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
  await expect(host.locator('.wiki-page-app-card-title', { hasText: 'Paprika chicken' })).toBeVisible({
   timeout: PAGE_LOAD_TIMEOUT_MS,
  });

  const bulleted = host.locator('.wiki-page-app-list').first();
  await expect(bulleted).toBeVisible();
  expect(await bulleted.evaluate((el) => `${el.tagName.toLowerCase()}`)).toBe('ul');
  const items = bulleted.locator(':scope > li');
  await expect(items).toHaveCount(3);
  await expect(items).toHaveText([
   '2 leg quarters',
   '1 tbsp smoked paprika',
   '3 cloves garlic',
  ]);
 });

 test('renders an ol wrapper with numbered items when ordered is true', async ({ page }) => {
  await page.goto(`/${TEST_PAGE}/view`);

  const host = page.locator('wiki-page-app-host');
  await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
  await expect(host.locator('.wiki-page-app-list').first()).toBeVisible({
   timeout: PAGE_LOAD_TIMEOUT_MS,
  });

  const ordered = host.locator('.wiki-page-app-list').nth(1);
  expect(await ordered.evaluate((el) => `${el.tagName.toLowerCase()}`)).toBe('ol');
  const items = ordered.locator(':scope > li');
  await expect(items).toHaveCount(3);
  await expect(items).toHaveText([
   'Season the chicken',
   'Sear skin-side down 6 min',
   'Roast at 425F for 20 min',
  ]);
 });

 test('renders Text children inside the bullet text alongside strings', async ({ page }) => {
  await page.goto(`/${TEST_PAGE}/view`);

  const host = page.locator('wiki-page-app-host');
  await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
  await expect(host.locator('.wiki-page-app-list').first()).toBeVisible({
   timeout: PAGE_LOAD_TIMEOUT_MS,
  });

  const textItems = host.locator('.wiki-page-app-list').nth(2);
  const items = textItems.locator(':scope > li');
  await expect(items).toHaveCount(3);
  await expect(items).toHaveText([
   'low sodium',
   'salt',
   'freshly ground pepper',
  ]);
 });

 test('drops entries that are neither strings nor component children', async ({ page }) => {
  await page.goto(`/${TEST_PAGE}/view`);

  const host = page.locator('wiki-page-app-host');
  await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
  await expect(host.locator('.wiki-page-app-list').first()).toBeVisible({
   timeout: PAGE_LOAD_TIMEOUT_MS,
  });

  const garbage = host.locator('.wiki-page-app-list').nth(3);
  const items = garbage.locator(':scope > li');
  await expect(items).toHaveCount(1);
  await expect(items).toHaveText(['flour']);
 });
});
