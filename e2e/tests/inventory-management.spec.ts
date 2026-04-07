import { test, expect, APIRequestContext } from '@playwright/test';

// Tests for inventory management features:
// 1. Creating an inventory item via the add-item dialog (UI)
// 2. Creating an inventory item via the CreateInventoryItem gRPC API
// 3. Listing container contents (ListContainerContents API)
// 4. Searching for item locations (FindItemLocation API)
// 5. Moving an inventory item between containers (UI + API verification)

// Timeouts
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const SAVE_TIMEOUT_MS = 10000;
const DIALOG_APPEAR_TIMEOUT_MS = 5000;
const MENU_APPEAR_TIMEOUT_MS = 5000;

// Per-run identifier suffix to prevent collisions between parallel CI workers/branches.
// Uses GITHUB_RUN_ID (a numeric CI run identifier) when available; falls back to the
// last 8 digits of Date.now() for local runs. Keeping the suffix purely numeric avoids
// case-transition complexity inside MungeIdentifier (no strcase.SnakeCase ambiguity).
// The replace guard is a safety net in case GITHUB_RUN_ID ever contains non-digits.
const TEST_RUN_SUFFIX = (
  process.env.GITHUB_RUN_ID ??
  String(Date.now()).slice(-8)
).replace(/[^0-9]/g, '');

// Test page identifiers — suffixed with TEST_RUN_SUFFIX to avoid cross-run collisions
const CONTAINER_A = `e2einvcontainera${TEST_RUN_SUFFIX}`;
const CONTAINER_A_TITLE = `E2E Inventory Container A ${TEST_RUN_SUFFIX}`;
const CONTAINER_B = `e2einvcontainerb${TEST_RUN_SUFFIX}`;
const CONTAINER_B_TITLE = `E2E Inventory Container B ${TEST_RUN_SUFFIX}`;
const API_ITEM = `e2einvapiitem${TEST_RUN_SUFFIX}`;
// UI_ITEM_TITLE is typed into the add-item dialog.  With a purely numeric suffix the
// identifier derivation is predictable:
// 'E2E Test Screwdriver 12345' → toTitleCase → 'E2e Test Screwdriver 12345'
//   → MungeIdentifier → 'e2e_test_screwdriver_12345'
// The JS regex below mirrors that conversion exactly for numeric suffixes.
// IMPORTANT: this regex must stay in sync with wikiidentifiers.MungeIdentifier in Go.
// If MungeIdentifier's behavior changes, update this derivation accordingly.
const UI_ITEM_TITLE = `E2E Test Screwdriver ${TEST_RUN_SUFFIX}`;
const UI_ITEM_IDENTIFIER = UI_ITEM_TITLE
  .toLowerCase()
  .replace(/[^a-z0-9]+/g, '_')
  .replace(/^_+|_+$/g, '');

async function callPageAPI(
  request: APIRequestContext,
  method: string,
  body: Record<string, unknown>,
) {
  return request.post(`/api.v1.PageManagementService/${method}`, {
    headers: { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1' },
    data: body,
  });
}

async function callInventoryAPI(
  request: APIRequestContext,
  method: string,
  body: Record<string, unknown>,
) {
  return request.post(`/api.v1.InventoryManagementService/${method}`, {
    headers: { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1' },
    data: body,
  });
}

test.describe('Inventory Management', () => {
  // Serial mode: tests share state (item in container, then moved)
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test.beforeAll(async ({ browser }) => {
    // Create two container pages with inventory frontmatter.
    // is_container = true is set explicitly so the move-item search index
    // finds them immediately without waiting for the normalization job.
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    try {
      for (const [identifier, title] of [
        [CONTAINER_A, CONTAINER_A_TITLE],
        [CONTAINER_B, CONTAINER_B_TITLE],
      ] as [string, string][]) {
        await page.goto(`/${identifier}/edit`);
        const textarea = page.locator('wiki-editor textarea');
        await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
        await textarea.fill(`+++
identifier = "${identifier}"
title = "${title}"

[inventory]
items = []
is_container = true
+++

# ${title}

This is an E2E test inventory container.`);
        await textarea.press('Space');
        await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });
      }
    } finally {
      await ctx.close();
    }
  });

  test.afterAll(async ({ request }) => {
    // Delete all test pages so their identifiers become available again on retries.
    // Stripping frontmatter leaves the page file on disk, so GenerateIdentifier still
    // treats the identifier as taken (isUnique=false). Calling DeletePage removes the
    // file entirely, freeing the identifier for subsequent runs.
    //
    // Note: callPageAPI returns a Response object rather than throwing on non-2xx status,
    // so we must check resp.ok() explicitly to detect failures.
    for (const identifier of [CONTAINER_A, CONTAINER_B, API_ITEM, UI_ITEM_IDENTIFIER]) {
      try {
        const resp = await callPageAPI(request, 'DeletePage', { pageName: identifier });
        if (!resp.ok()) {
          // Log but do not fail — cleanup is best-effort
          console.warn(`[afterAll] DeletePage(${identifier}) returned HTTP ${resp.status()}`);
        }
      } catch (err) {
        // Network-level errors should not fail the suite
        console.warn(`[afterAll] DeletePage(${identifier}) threw: ${String(err)}`);
      }
    }
  });

  test.describe('Creating an inventory item via the add-item dialog', () => {
    test('should open the add-item dialog on an inventory container page', async ({ page }) => {
      await page.goto(`/${CONTAINER_A}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // initInventoryMenu injects the submenu asynchronously after checking frontmatter
      await expect(page.locator('#inventory-add-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      // Open the submenu (CSS-hidden) then click "Add Item Here"
      await page.locator('.tools-menu').hover();
      await page.locator('#inventory-submenu-trigger').click();
      await page.locator('#inventory-add-item').click();

      // The dialog web component should receive the `open` attribute
      await expect(page.locator('inventory-add-item-dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
    });

    test('should create a new item when a title is entered and Add Item is clicked', async ({ page }) => {
      await page.goto(`/${CONTAINER_A}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-add-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      // Open the add-item dialog
      await page.locator('.tools-menu').hover();
      await page.locator('#inventory-submenu-trigger').click();
      await page.locator('#inventory-add-item').click();

      const dialog = page.locator('inventory-add-item-dialog[open]');
      await expect(dialog).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

      // The title input lives in deeply nested shadow DOM:
      //   inventory-add-item-dialog > automagic-identifier-input > title-input > input
      // Playwright's chained locators auto-pierce shadow boundaries at each step.
      const titleInput = dialog
        .locator('automagic-identifier-input')
        .locator('title-input')
        .locator('input');
      await expect(titleInput).toBeVisible({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      await titleInput.fill(UI_ITEM_TITLE);

      // The "Add Item" button is disabled until the identifier is auto-generated
      // (an async gRPC call to GenerateIdentifier). Wait for it to become enabled.
      const addButton = dialog.locator('button', { hasText: 'Add Item' });
      await expect(addButton).toBeEnabled({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

      // Submit the form and wait for the page to reload.
      // On success, the component calls window.location.reload(), which keeps the same URL.
      // waitForURL can resolve immediately because the page is already on that URL, so we
      // use waitForNavigation to explicitly wait for the next navigation event (the reload).
      // Using Promise.all ensures the navigation listener is registered before the click.
      await Promise.all([
        page.waitForNavigation({ waitUntil: 'load', timeout: SAVE_TIMEOUT_MS }),
        addButton.click(),
      ]);
      await expect(page).toHaveURL(`**/${CONTAINER_A}/view`);
    });
  });

  test.describe('Creating an inventory item via the gRPC API', () => {
    test('should create an item with CreateInventoryItem', async ({ request }) => {
      const resp = await callInventoryAPI(request, 'CreateInventoryItem', {
        itemIdentifier: API_ITEM,
        container: CONTAINER_A,
        title: 'E2E API Test Item',
      });

      expect(resp.status()).toBe(200);
      const body = await resp.json() as {
        success: boolean;
        itemIdentifier: string;
        summary: string;
      };
      expect(body.success).toBe(true);
      expect(body.itemIdentifier).toBe(API_ITEM);
      expect(body.summary).toBeTruthy();
    });

    test('should return success=false when the item already exists', async ({ request }) => {
      const resp = await callInventoryAPI(request, 'CreateInventoryItem', {
        itemIdentifier: API_ITEM,
        container: CONTAINER_A,
        title: 'Duplicate Item',
      });

      expect(resp.status()).toBe(200);
      const body = await resp.json() as { success: boolean; error: string };
      expect(body.success).toBe(false);
      expect(body.error).toBeTruthy();
    });
  });

  test.describe('Listing container contents', () => {
    test('should list items in a container with ListContainerContents', async ({ request }) => {
      const resp = await callInventoryAPI(request, 'ListContainerContents', {
        containerIdentifier: CONTAINER_A,
      });

      expect(resp.status()).toBe(200);
      const body = await resp.json() as {
        containerIdentifier: string;
        items: Array<{ identifier: string }>;
        totalCount: number;
        summary: string;
      };
      expect(body.containerIdentifier).toBe(CONTAINER_A);
      expect(body.items).toBeDefined();
      expect(body.items.some(item => item.identifier === API_ITEM)).toBe(true);
      expect(body.totalCount).toBeGreaterThan(0);
      expect(body.summary).toBeTruthy();
    });

    test('should return an empty list for a container with no items', async ({ request }) => {
      const resp = await callInventoryAPI(request, 'ListContainerContents', {
        containerIdentifier: CONTAINER_B,
      });

      expect(resp.status()).toBe(200);
      const body = await resp.json() as {
        items: Array<{ identifier: string }>;
        totalCount: number;
      };
      // CONTAINER_B has no items at this point in the serial run
      expect(body.items.some(item => item.identifier === API_ITEM)).toBe(false);
      expect(body.totalCount).toBe(0);
    });
  });

  test.describe('Searching for item locations (FindItemLocation)', () => {
    test('should find the container for a known item', async ({ request }) => {
      const resp = await callInventoryAPI(request, 'FindItemLocation', {
        itemIdentifier: API_ITEM,
      });

      expect(resp.status()).toBe(200);
      const body = await resp.json() as {
        itemIdentifier: string;
        found: boolean;
        locations: Array<{ container: string }>;
        summary: string;
      };
      expect(body.found).toBe(true);
      expect(body.itemIdentifier).toBe(API_ITEM);
      expect(body.locations.length).toBeGreaterThan(0);
      expect(body.locations[0].container).toBe(CONTAINER_A);
      expect(body.summary).toBeTruthy();
    });

    test('should return found=false for a non-existent item', async ({ request }) => {
      const resp = await callInventoryAPI(request, 'FindItemLocation', {
        itemIdentifier: 'e2enonexistentitemxyz999',
      });

      expect(resp.status()).toBe(200);
      const body = await resp.json() as { found: boolean };
      expect(body.found).toBe(false);
    });

    test('should include the full hierarchy path when includeHierarchy is true', async ({ request }) => {
      const resp = await callInventoryAPI(request, 'FindItemLocation', {
        itemIdentifier: API_ITEM,
        includeHierarchy: true,
      });

      expect(resp.status()).toBe(200);
      const body = await resp.json() as {
        found: boolean;
        locations: Array<{ container: string; path: string[] }>;
      };
      expect(body.found).toBe(true);
      expect(body.locations.length).toBeGreaterThan(0);
      // With hierarchy, the path array should be populated
      expect(body.locations[0].path).toBeDefined();
    });
  });

  test.describe('Moving an inventory item between containers', () => {
    test('should move an item via the move-item dialog', async ({ page }) => {
      // Navigate to the item page (created via API in previous describe block)
      await page.goto(`/${API_ITEM}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Items in a container should show "Move This Item" in the inventory menu
      await expect(page.locator('#inventory-move-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      // Open the submenu and click "Move This Item"
      await page.locator('.tools-menu').hover();
      await page.locator('#inventory-submenu-trigger').click();
      await page.locator('#inventory-move-item').click();

      // The move-item dialog should open
      const dialog = page.locator('inventory-move-item-dialog[open]');
      await expect(dialog).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

      // The search input lives in the dialog's shadow DOM
      const searchInput = dialog.locator('input[name="searchQuery"]');
      await expect(searchInput).toBeVisible({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      // Fill the search input and wait for results to appear.
      // The component debounces input; waiting for the move-to-button to become visible
      // is a deterministic signal that covers both the debounce delay and the API response.
      await searchInput.fill(CONTAINER_B_TITLE);

      // Click the "Move To" button on the first matching container
      const moveToButton = dialog.locator('.move-to-button', { hasText: 'Move To' });
      await expect(moveToButton.first()).toBeVisible({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

      // Submit the move and wait for the page to reload.
      // On success, the component calls window.location.reload(), which keeps the same URL.
      // waitForURL can resolve immediately because the page is already on that URL, so we
      // use waitForNavigation to explicitly wait for the next navigation event (the reload).
      await Promise.all([
        page.waitForNavigation({ waitUntil: 'load', timeout: SAVE_TIMEOUT_MS }),
        moveToButton.first().click(),
      ]);
      await expect(page).toHaveURL(`**/${API_ITEM}/view`);
    });

    test('should reflect the move in FindItemLocation after the dialog move', async ({ request }) => {
      const resp = await callInventoryAPI(request, 'FindItemLocation', {
        itemIdentifier: API_ITEM,
      });

      expect(resp.status()).toBe(200);
      const body = await resp.json() as {
        found: boolean;
        locations: Array<{ container: string }>;
      };
      expect(body.found).toBe(true);
      expect(body.locations.length).toBeGreaterThan(0);
      expect(body.locations[0].container).toBe(CONTAINER_B);
    });

    test('should move an item via the MoveInventoryItem gRPC API', async ({ request }) => {
      // Move the item back to CONTAINER_A via the API to verify the API itself
      const resp = await callInventoryAPI(request, 'MoveInventoryItem', {
        itemIdentifier: API_ITEM,
        newContainer: CONTAINER_A,
      });

      expect(resp.status()).toBe(200);
      const body = await resp.json() as {
        success: boolean;
        previousContainer: string;
        newContainer: string;
        summary: string;
      };
      expect(body.success).toBe(true);
      expect(body.previousContainer).toBe(CONTAINER_B);
      expect(body.newContainer).toBe(CONTAINER_A);
      expect(body.summary).toBeTruthy();
    });

    test('should reflect the API move in ListContainerContents', async ({ request }) => {
      const resp = await callInventoryAPI(request, 'ListContainerContents', {
        containerIdentifier: CONTAINER_A,
      });

      expect(resp.status()).toBe(200);
      const body = await resp.json() as {
        items: Array<{ identifier: string }>;
      };
      expect(body.items.some(item => item.identifier === API_ITEM)).toBe(true);
    });
  });
});
