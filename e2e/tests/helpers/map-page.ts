import { expect, type Page } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;

export async function seedMapPage(page: Page, pageName: string, mapName: string): Promise<void> {
  await page.goto(`/${pageName}/edit`);
  const textarea = page.locator('wiki-editor textarea');
  await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

  const content = `+++
identifier = "${pageName}"
title = "Map E2E Test"

[maps.${mapName}.view]
lat = 41.1
lon = -72.2
zoom = 16

[maps.${mapName}.style]
tile_layer_id = 1

[[maps.${mapName}.markers]]
uid = "marker-shed"
label = "Shed"
lat = 41.1
lon = -72.2
popup_markdown = "[[shed]]"
color = "#2563eb"

[agent.maps.${mapName}]
updated_at = "2026-06-12T20:00:00Z"
sync_token = 1

[agent.maps.${mapName}.markers.marker-shed]
created_at = "2026-06-12T20:00:00Z"
updated_at = "2026-06-12T20:00:00Z"
created_by = "e2e"
automated = true
sort_order = 1000
+++

# Map E2E Test

{{ Map "${mapName}" }}
`;

  await textarea.fill(content);
  await textarea.press('Space');
  await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
    timeout: SAVE_TIMEOUT_MS,
  });
}

export async function clearMapPage(page: Page, pageName: string): Promise<void> {
  await page.goto(`/${pageName}/edit`);
  const textarea = page.locator('wiki-editor textarea');
  await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
  await textarea.fill(`+++\nidentifier = "${pageName}"\n+++`);
  await textarea.press('Space');
  await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
    timeout: SAVE_TIMEOUT_MS,
  });
}

/**
 * Creates a mock GPX file with the given points list.
 */
export function createMockGpxFile(filePath: string, points: [number, number][]): void {
  const dir = path.dirname(filePath);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }

  const trkpts = points.map(pt => `<trkpt lat="${pt[0]}" lon="${pt[1]}"></trkpt>`).join('\n      ');
  const gpxContent = `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="Playwright Test" xmlns="http://www.topografix.com/GPX/1/1">
  <trk>
    <name>E2E Test Trail</name>
    <trkseg>
      ${trkpts}
    </trkseg>
  </trk>
</gpx>`;

  fs.writeFileSync(filePath, gpxContent, 'utf-8');
}

/**
 * Creates a mock GeoJSON file.
 */
export function createMockGeoJsonFile(filePath: string, coordinates: [number, number][]): void {
  const dir = path.dirname(filePath);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }

  const geoJsonContent = {
    type: 'FeatureCollection',
    features: [
      {
        type: 'Feature',
        properties: {
          name: 'E2E Test Trail GeoJSON',
        },
        geometry: {
          type: 'LineString',
          coordinates: coordinates.map(pt => [pt[1], pt[0]]), // GeoJSON is [lon, lat]
        },
      },
    ],
  };

  fs.writeFileSync(filePath, JSON.stringify(geoJsonContent, null, 2), 'utf-8');
}

/**
 * Seeds a map page with complex tracks, markers, and style parameters.
 */
export async function seedMapPageWithTracks(
  page: Page,
  pageName: string,
  mapName: string,
  tracksMetadata: string,
  extraElements: string = ''
): Promise<void> {
  await page.goto(`/${pageName}/edit`);
  const textarea = page.locator('wiki-editor textarea');
  await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

  const content = `+++
identifier = "${pageName}"
title = "Map Track E2E Test"

[maps.${mapName}.view]
lat = 41.1
lon = -72.2
zoom = 16

[maps.${mapName}.style]
tile_layer_id = 1
aspect_ratio = "3:2"

${extraElements}

[agent.maps.${mapName}]
updated_at = "2026-06-12T20:00:00Z"
sync_token = 2

${tracksMetadata}
+++

# Map Track E2E Test

{{ Map "${mapName}" }}
`;

  await textarea.fill(content);
  await textarea.press('Space');
  await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
    timeout: SAVE_TIMEOUT_MS,
  });
}
