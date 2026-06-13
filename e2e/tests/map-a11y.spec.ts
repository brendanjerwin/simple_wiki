import { test, expect } from '@playwright/test';
import { clearMapPage, seedMapPage } from './helpers/map-page';

const TEST_PAGE_NAME = 'e2e_map_a11y_test';
const TEST_MAP_NAME = 'yard';

test.describe('Map Accessibility E2E Tests', () => {
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

  test('should expose the map component with an accessible label', async ({ page }) => {
    await page.goto(`/${TEST_PAGE_NAME}/view`);
    await expect(page.locator('#rendered')).toBeVisible({ timeout: 15000 });

    const canvas = page.locator('wiki-map #map-canvas');
    await expect(canvas).toBeVisible({ timeout: 15000 });
    await expect(canvas).toHaveAttribute('aria-label', 'yard');
  });
});
