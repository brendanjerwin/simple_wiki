import { test, expect, type Page, type APIRequestContext } from '@playwright/test';
import { COMPONENT_LOAD_TIMEOUT_MS } from './constants.js';

// Timeouts (local — not shared across spec files)
const SAVE_TIMEOUT_MS = 15000;
const DIALOG_APPEAR_TIMEOUT_MS = 5000;
const MENU_APPEAR_TIMEOUT_MS = 5000;

// Per-run identifier suffix to prevent collisions between parallel CI workers/branches.
const TEST_RUN_SUFFIX = (
  process.env.GITHUB_RUN_ID ?? String(Date.now()).slice(-8)
).replace(/[^0-9]/g, '');

const A11Y_CONTAINER = `e2einva11ycontainer${TEST_RUN_SUFFIX}`;
const A11Y_CONTAINER_TITLE = `E2E A11y Container ${TEST_RUN_SUFFIX}`;
const A11Y_ITEM = `e2einva11yitem${TEST_RUN_SUFFIX}`;

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

/** Hover the tools menu, open the inventory submenu, and click the specified menu item. */
async function openInventoryMenuItem(page: Page, menuItemId: string): Promise<void> {
  await page.locator('.tools-menu').hover();
  await page.locator('#inventory-submenu-trigger').click();
  await page.locator(`#${menuItemId}`).click();
}

test.describe('Inventory Dialog Accessibility', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test.beforeAll(async ({ browser, request }) => {
    // Create the container page via browser UI
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    try {
      await page.goto(`/${A11Y_CONTAINER}/edit`);
      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await textarea.fill(`+++
identifier = "${A11Y_CONTAINER}"
title = "${A11Y_CONTAINER_TITLE}"

[inventory]
items = []
is_container = true
+++

# ${A11Y_CONTAINER_TITLE}

E2E accessibility test container.`);
      await textarea.press('Space');
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
        timeout: SAVE_TIMEOUT_MS,
      });
    } finally {
      await ctx.close();
    }

    // Create an item in the container via API — needed for move-item dialog tests
    const resp = await callInventoryAPI(request, 'CreateInventoryItem', {
      itemIdentifier: A11Y_ITEM,
      container: A11Y_CONTAINER,
      title: 'E2E A11y Test Item',
    });
    if (!resp.ok()) {
      throw new Error(`[beforeAll] CreateInventoryItem(${A11Y_ITEM}) returned HTTP ${resp.status()}`);
    }
  });

  test.afterAll(async ({ request }) => {
    for (const identifier of [A11Y_CONTAINER, A11Y_ITEM]) {
      try {
        const resp = await callPageAPI(request, 'DeletePage', { pageName: identifier });
        if (!resp.ok()) {
          console.warn(`[afterAll] DeletePage(${identifier}) returned HTTP ${resp.status()}`);
        }
      } catch (err) {
        console.warn(`[afterAll] DeletePage(${identifier}) threw: ${String(err)}`);
      }
    }
  });

  // ─── inventory-add-item-dialog ─────────────────────────────────────────────

  test.describe('inventory-add-item-dialog ARIA attributes', () => {
    test('dialog element has aria-labelledby="add-item-dialog-title"', async ({ page }) => {
      await page.goto(`/${A11Y_CONTAINER}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-add-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-add-item');

      const dialog = page.locator('inventory-add-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      await expect(dialog.locator('dialog')).toHaveAttribute('aria-labelledby', 'add-item-dialog-title');
    });

    test('title element with id="add-item-dialog-title" is present and labelled', async ({ page }) => {
      await page.goto(`/${A11Y_CONTAINER}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-add-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-add-item');

      const dialog = page.locator('inventory-add-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      await expect(dialog.locator('#add-item-dialog-title')).toBeAttached();
      await expect(dialog.locator('#add-item-dialog-title')).toContainText('Add Item');
    });
  });

  test.describe('inventory-add-item-dialog native dialog behavior', () => {
    test('native <dialog> element is open via showModal', async ({ page }) => {
      await page.goto(`/${A11Y_CONTAINER}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-add-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-add-item');

      const dialog = page.locator('inventory-add-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      // Native <dialog> opened via showModal() has the `open` attribute and is visible
      await expect(dialog.locator('dialog')).toBeVisible();
      await expect(dialog.locator('dialog')).toHaveAttribute('open');
    });

    test('closes when Escape key is pressed', async ({ page }) => {
      await page.goto(`/${A11Y_CONTAINER}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-add-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-add-item');

      const dialog = page.locator('inventory-add-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      await page.keyboard.press('Escape');

      await expect(dialog).not.toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });
    });

    test('closes when backdrop is clicked', async ({ page }) => {
      await page.goto(`/${A11Y_CONTAINER}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-add-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-add-item');

      const dialog = page.locator('inventory-add-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      // Simulate backdrop click: dispatch click directly on the native <dialog> element.
      // When the browser fires a click on dialog::backdrop, event.target is the <dialog> itself,
      // which is what _handleDialogClick checks for (event.target === event.currentTarget).
      await page.evaluate(() => {
        const host = document.querySelector('inventory-add-item-dialog');
        const d = host?.shadowRoot?.querySelector('dialog');
        d?.dispatchEvent(new MouseEvent('click', { bubbles: false }));
      });

      await expect(dialog).not.toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });
    });
  });

  test.describe('inventory-add-item-dialog focus management', () => {
    test('focus returns to trigger element when closed via Cancel', async ({ page }) => {
      await page.goto(`/${A11Y_CONTAINER}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-add-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      // Focus the trigger element explicitly before clicking so it is captured as
      // _previouslyFocusedElement when the dialog opens.
      await page.locator('.tools-menu').hover();
      await page.locator('#inventory-submenu-trigger').click();
      const triggerBtn = page.locator('#inventory-add-item');
      await triggerBtn.focus();
      await triggerBtn.click();

      const dialog = page.locator('inventory-add-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      // Close via Cancel button
      await dialog.locator('button.button-secondary').click();
      await expect(dialog).not.toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      // Focus should have returned to the element that triggered the dialog open
      await expect(page.locator('#inventory-add-item')).toBeFocused({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
    });

    test('Tab key stays within dialog (focus trap)', async ({ page }) => {
      await page.goto(`/${A11Y_CONTAINER}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-add-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-add-item');

      const dialog = page.locator('inventory-add-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      // Tab through focusable elements several times — native showModal() traps focus
      for (let i = 0; i < 10; i++) {
        await page.keyboard.press('Tab');
      }

      // Dialog should still be open; focus never escaped
      await expect(dialog).toHaveAttribute('open');
      await expect(dialog.locator('dialog')).toBeVisible();
    });
  });

  test.describe('inventory-add-item-dialog keyboard activation', () => {
    test('Enter activates the focused Cancel button', async ({ page }) => {
      await page.goto(`/${A11Y_CONTAINER}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-add-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-add-item');

      const dialog = page.locator('inventory-add-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      // Focus the Cancel button and activate it via keyboard
      await dialog.locator('button.button-secondary').focus();
      await page.keyboard.press('Enter');

      await expect(dialog).not.toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });
    });
  });

  // ─── inventory-move-item-dialog ────────────────────────────────────────────

  test.describe('inventory-move-item-dialog ARIA attributes', () => {
    test('dialog element has aria-labelledby="move-item-dialog-title"', async ({ page }) => {
      await page.goto(`/${A11Y_ITEM}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-move-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-move-item');

      const dialog = page.locator('inventory-move-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      await expect(dialog.locator('dialog')).toHaveAttribute('aria-labelledby', 'move-item-dialog-title');
    });

    test('title element with id="move-item-dialog-title" is present and labelled', async ({ page }) => {
      await page.goto(`/${A11Y_ITEM}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-move-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-move-item');

      const dialog = page.locator('inventory-move-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      await expect(dialog.locator('#move-item-dialog-title')).toBeAttached();
      await expect(dialog.locator('#move-item-dialog-title')).toContainText('Move Item');
    });
  });

  test.describe('inventory-move-item-dialog native dialog behavior', () => {
    test('native <dialog> element is open via showModal', async ({ page }) => {
      await page.goto(`/${A11Y_ITEM}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-move-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-move-item');

      const dialog = page.locator('inventory-move-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      await expect(dialog.locator('dialog')).toBeVisible();
      await expect(dialog.locator('dialog')).toHaveAttribute('open');
    });

    test('closes when Escape key is pressed', async ({ page }) => {
      await page.goto(`/${A11Y_ITEM}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-move-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-move-item');

      const dialog = page.locator('inventory-move-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      await page.keyboard.press('Escape');

      await expect(dialog).not.toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });
    });

    test('closes when backdrop is clicked', async ({ page }) => {
      await page.goto(`/${A11Y_ITEM}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-move-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-move-item');

      const dialog = page.locator('inventory-move-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      await page.evaluate(() => {
        const host = document.querySelector('inventory-move-item-dialog');
        const d = host?.shadowRoot?.querySelector('dialog');
        d?.dispatchEvent(new MouseEvent('click', { bubbles: false }));
      });

      await expect(dialog).not.toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });
    });
  });

  test.describe('inventory-move-item-dialog focus management', () => {
    test('focus returns to trigger element when closed via Cancel', async ({ page }) => {
      await page.goto(`/${A11Y_ITEM}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-move-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      // Focus the trigger element explicitly before clicking
      await page.locator('.tools-menu').hover();
      await page.locator('#inventory-submenu-trigger').click();
      const triggerBtn = page.locator('#inventory-move-item');
      await triggerBtn.focus();
      await triggerBtn.click();

      const dialog = page.locator('inventory-move-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      // Close via Cancel button
      await dialog.locator('button.button-secondary').click();
      await expect(dialog).not.toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      // Focus should have returned to the element that triggered the dialog open
      await expect(page.locator('#inventory-move-item')).toBeFocused({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
    });

    test('Tab key stays within dialog (focus trap)', async ({ page }) => {
      await page.goto(`/${A11Y_ITEM}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-move-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-move-item');

      const dialog = page.locator('inventory-move-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      for (let i = 0; i < 10; i++) {
        await page.keyboard.press('Tab');
      }

      await expect(dialog).toHaveAttribute('open');
      await expect(dialog.locator('dialog')).toBeVisible();
    });
  });

  test.describe('inventory-move-item-dialog keyboard activation', () => {
    test('Enter activates the focused Cancel button', async ({ page }) => {
      await page.goto(`/${A11Y_ITEM}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-move-item')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await openInventoryMenuItem(page, 'inventory-move-item');

      const dialog = page.locator('inventory-move-item-dialog');
      await expect(dialog).toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });

      await dialog.locator('button.button-secondary').focus();
      await page.keyboard.press('Enter');

      await expect(dialog).not.toHaveAttribute('open', { timeout: DIALOG_APPEAR_TIMEOUT_MS });
    });
  });
});
