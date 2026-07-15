import { test, expect } from '@playwright/test';
import * as path from 'path';
import * as fs from 'fs';
import {
  clearMapPage,
  createMockGpxFile,
  createMockGeoJsonFile,
  seedMapPageWithTracks,
} from './helpers/map-page';

const TEST_PAGE = 'e2e_gps_tracks_test';
const TEST_MAP = 'backyard';
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;

test.describe('GPS Track & Leaflet Tag Control E2E Tests', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  const testGpxPath = path.join(__dirname, '..', 'test-data', 'test_route.gpx');
  const testGeoJsonPath = path.join(__dirname, '..', 'test-data', 'test_route.geojson');
  const largeGpxPath = path.join(__dirname, '..', 'test-data', 'large_route.gpx');

  test.beforeAll(async () => {
    // Write temporary test GPX/GeoJSON files
    createMockGpxFile(testGpxPath, [[41.1, -72.2], [41.11, -72.21], [41.12, -72.22]]);
    createMockGeoJsonFile(testGeoJsonPath, [[41.1, -72.2], [41.11, -72.21], [41.12, -72.22]]);
    
    // Create large GPX file (e.g. 1000 points)
    const largePoints: [number, number][] = [];
    for (let i = 0; i < 1000; i++) {
      largePoints.push([41.1 + i * 0.0001, -72.2 - i * 0.0001]);
    }
    createMockGpxFile(largeGpxPath, largePoints);
  });

  test.afterAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    try {
      await clearMapPage(page, TEST_PAGE);
    } finally {
      await ctx.close();
    }

    // Cleanup files
    if (fs.existsSync(testGpxPath)) fs.unlinkSync(testGpxPath);
    if (fs.existsSync(testGeoJsonPath)) fs.unlinkSync(testGeoJsonPath);
    if (fs.existsSync(largeGpxPath)) fs.unlinkSync(largeGpxPath);
  });

  test.describe('F1: GPS Track Upload', () => {
    test.beforeEach(async ({ page }) => {
      // Seed initial empty map
      await seedMapPageWithTracks(page, TEST_PAGE, TEST_MAP, '');
    });

    test('should upload valid GPX track via widget and display success', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      const map = page.locator('wiki-map');
      await expect(map.locator('.leaflet-container')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Click map to reveal tools panel (mirroring wiki-image hover/tap reveal)
      await map.locator('.leaflet-container').click();
      const toolsPanel = map.locator('.tools-panel');
      await expect(toolsPanel).toBeVisible();

      const uploadBtn = toolsPanel.locator('button[aria-label="Add GPS track"]');
      await expect(uploadBtn).toBeVisible();

      // Trigger file upload dialog
      const fileChooserPromise = page.waitForEvent('filechooser');
      await uploadBtn.click();
      const fileChooser = await fileChooserPromise;
      await fileChooser.setFiles(testGpxPath);

      // Supply label and tags in the popover dialog
      const popover = map.locator('.upload-popover');
      await expect(popover).toBeVisible();
      await popover.locator('input[name="label"]').fill('GPX Hike Trail');
      await popover.locator('input[name="tags"]').fill('hiking,scenic');
      await popover.locator('button[type="submit"]').click();

      // Verify success notification and rendering
      const toast = page.locator('toast-message');
      await expect(toast).toContainText('uploaded successfully', { timeout: 10000 });
      await expect(map.locator('path.leaflet-interactive')).toHaveCount(1, { timeout: COMPONENT_LOAD_TIMEOUT_MS });
    });

    test('should reject unsupported file types (non-track file)', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      const map = page.locator('wiki-map');
      await map.locator('.leaflet-container').click();

      // Create a dummy image text file
      const dummyImgPath = path.join(__dirname, '..', 'test-data', 'dummy.png');
      fs.writeFileSync(dummyImgPath, 'not-an-image');

      const fileChooserPromise = page.waitForEvent('filechooser');
      await map.locator('button[aria-label="Add GPS track"]').click();
      const fileChooser = await fileChooserPromise;
      await fileChooser.setFiles(dummyImgPath);

      // Expect UI error
      const popover = map.locator('.upload-popover');
      await expect(popover.locator('.error-message')).toContainText('Unsupported file type');

      fs.unlinkSync(dummyImgPath);
    });
  });

  test.describe('F2: GPS Track Rendering & F3: GPS Track Download', () => {
    const trackUid = 'track-abc1234';

    test.beforeEach(async ({ page }) => {
      // Seed map with a track element metadata
      const tracksMetadata = `
[agent.maps.${TEST_MAP}.tracks.${trackUid}]
label = "Mt Marcy Trail"
file_hash = "mockhash123"
filename = "marcy.gpx"
format = "GPX"
color = "#10b981"
tags = ["hiking", "mountain"]
created_at = "2026-06-12T20:00:00Z"
updated_at = "2026-06-12T20:00:00Z"
created_by = "e2e"
automated = true
`;
      await seedMapPageWithTracks(page, TEST_PAGE, TEST_MAP, tracksMetadata);
    });

    test('should render seeded track and allow opening details popup and download', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      const map = page.locator('wiki-map');
      await expect(map.locator('.leaflet-container')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Check polyline rendering
      const polyline = map.locator('path.leaflet-interactive').first();
      await expect(polyline).toBeAttached();

      // Click polyline to open Leaflet Popup
      await polyline.click({ force: true });
      const popup = map.locator('.leaflet-popup-content');
      await expect(popup).toBeVisible();

      // Popup details
      await expect(popup).toContainText('Mt Marcy Trail');
      await expect(popup.locator('a.download-track-link')).toBeVisible();

      // Assert download url parameter formats
      const href = await popup.locator('a.download-track-link').getAttribute('href');
      expect(href).toContain('/uploads/mockhash123');
      expect(href).toContain('filename=marcy.gpx');
    });
  });

  test.describe('F4: Leaflet Tag Control', () => {
    test.beforeEach(async ({ page }) => {
      const tracksMetadata = `
[agent.maps.${TEST_MAP}.tracks.track-t1]
label = "Scenic Path"
file_hash = "hash-t1"
filename = "scenic.gpx"
format = "GPX"
tags = ["scenic", "easy"]

[agent.maps.${TEST_MAP}.tracks.track-t2]
label = "Difficult Climb"
file_hash = "hash-t2"
filename = "climb.gpx"
format = "GPX"
tags = ["difficult"]
`;
      const extraElements = `
[[maps.${TEST_MAP}.markers]]
uid = "marker-m1"
label = "Camp site"
lat = 41.101
lon = -72.201
tags = ["easy"]
`;
      await seedMapPageWithTracks(page, TEST_PAGE, TEST_MAP, tracksMetadata, extraElements);
    });

    test('should filter map overlays dynamically based on tag toggles', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      const map = page.locator('wiki-map');
      await expect(map.locator('.leaflet-container')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Initially all layers should be visible (2 polylines + 1 marker = 3 interactives)
      await expect(map.locator('path.leaflet-interactive')).toHaveCount(2);
      await expect(map.locator('.wiki-map-marker')).toHaveCount(1);

      // Open layer/tag control panel
      const tagControl = map.locator('.leaflet-control-layers');
      await expect(tagControl).toBeVisible();
      await tagControl.hover(); // expands the control if collapse is on

      // Uncheck "difficult"
      const difficultCheckbox = tagControl.locator('input[type="checkbox"][value="difficult"]');
      await difficultCheckbox.uncheck();

      // Now "Difficult Climb" polyline should be hidden (so 1 polyline left)
      await expect(map.locator('path.leaflet-interactive')).toHaveCount(1);

      // Uncheck "easy"
      const easyCheckbox = tagControl.locator('input[type="checkbox"][value="easy"]');
      await easyCheckbox.uncheck();

      // Now easy marker is hidden, easy track "Scenic Path" is still shown because it also has "scenic" (OR semantics!)
      await expect(map.locator('.wiki-map-marker')).toHaveCount(0);
      await expect(map.locator('path.leaflet-interactive')).toHaveCount(1);

      // Uncheck "scenic"
      const scenicCheckbox = tagControl.locator('input[type="checkbox"][value="scenic"]');
      await scenicCheckbox.uncheck();

      // Now all overlays are hidden
      await expect(map.locator('path.leaflet-interactive')).toHaveCount(0);
    });
  });
});
