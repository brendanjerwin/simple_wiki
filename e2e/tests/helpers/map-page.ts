import { expect, type Page } from '@playwright/test';

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
