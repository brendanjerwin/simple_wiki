import { test, expect } from '@playwright/test';
import { clearMapPage, seedMapPage } from './helpers/map-page';

const TEST_PAGE_NAME = 'e2e_map_test';
const TEST_MAP_NAME = 'yard';

const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;

test.describe('Map E2E Tests', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    try {
      await seedMapPage(page, TEST_PAGE_NAME, TEST_MAP_NAME);
    } finally {
      await ctx.close();
    }
  });

  test.afterAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    try {
      await clearMapPage(page, TEST_PAGE_NAME);
    } finally {
      await ctx.close();
    }
  });

  test('should render the Map macro as a wiki-map component', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const map = page.locator('wiki-map');
    await expect(map).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await expect(map).toHaveAttribute('name', TEST_MAP_NAME);
    await expect(map).toHaveAttribute('page', TEST_PAGE_NAME);
  });

  test('should render visible Leaflet map content', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const map = page.locator('wiki-map');
    await expect(map.locator('.leaflet-container')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await expect(map.locator('.wiki-map-marker')).toHaveCount(1, { timeout: COMPONENT_LOAD_TIMEOUT_MS });
  });
});
