import { test, expect } from '@playwright/test';

// Timeouts
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const SAVE_TIMEOUT_MS = 10000;
const MENU_APPEAR_TIMEOUT_MS = 5000;

test.describe('Menu Functionality (migrated from simple_wiki.js)', () => {
  test.setTimeout(60000);

  test.describe('Page Import Menu', () => {
    test('should show the Import Pages menu item on a view page', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // The page import trigger should be injected by initPageImportMenu()
      // It lives inside .pure-menu-children (CSS-hidden dropdown), so check DOM attachment not CSS visibility
      await expect(page.locator('#page-import-trigger')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });
    });

    test('should show the FontAwesome import icon next to the label', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const trigger = page.locator('#page-import-trigger');
      // Item lives inside .pure-menu-children (CSS-hidden dropdown), so check DOM attachment not CSS visibility
      await expect(trigger).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });
      await expect(trigger.locator('i.fa-solid.fa-file-import')).toBeAttached();
    });

    test('should open the import dialog when Import Pages is clicked', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Item lives inside .pure-menu-children (CSS-hidden dropdown); confirm it's attached before interacting
      await expect(page.locator('#page-import-trigger')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });
      // Hover over the tools-menu to open the CSS dropdown, then click the import trigger
      await page.locator('.tools-menu').hover();
      await page.locator('#page-import-trigger').click();

      // The page-import-dialog should become open/visible
      await expect(page.locator('page-import-dialog')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });
    });
  });

  test.describe('Inventory Menu', () => {
    const INVENTORY_PAGE = 'e2einventorycontainer';

    test.beforeAll(async ({ browser }) => {
      // Create a page with inventory container frontmatter so the inventory menu appears
      const ctx = await browser.newContext();
      const page = await ctx.newPage();
      await page.goto(`/${INVENTORY_PAGE}/edit`);

      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await textarea.fill(`+++
identifier = "${INVENTORY_PAGE}"
title = "E2E Inventory Container"

[inventory]
items = []
+++

# E2E Inventory Container

This page is used by E2E tests for the inventory menu.`);
      await textarea.press('Space');
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });

      await ctx.close();
    });

    test.afterAll(async ({ browser }) => {
      // Clean up the test page
      const ctx = await browser.newContext();
      const page = await ctx.newPage();
      try {
        await page.goto(`/${INVENTORY_PAGE}/edit`);
        const textarea = page.locator('wiki-editor textarea');
        await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
        await textarea.fill(`+++\nidentifier = "${INVENTORY_PAGE}"\n+++`);
        await textarea.press('Space');
        await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });
      } catch (_) {
        // Best effort cleanup
      } finally {
        await ctx.close();
      }
    });

    test('should show the Inventory submenu on an inventory container page', async ({ page }) => {
      await page.goto(`/${INVENTORY_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // initInventoryMenu fetches and injects the submenu asynchronously
      await expect(page.locator('#inventory-submenu')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });
    });

    test('should show the Add Item Here option for a container page', async ({ page }) => {
      await page.goto(`/${INVENTORY_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-submenu')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await expect(page.locator('#inventory-add-item')).toBeAttached();
    });

    test('should not show Move This Item on a container-only page', async ({ page }) => {
      await page.goto(`/${INVENTORY_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-submenu')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await expect(page.locator('#inventory-move-item')).not.toBeAttached();
    });

    test('submenu trigger should have aria-expanded=false initially', async ({ page }) => {
      await page.goto(`/${INVENTORY_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-submenu-trigger')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      await expect(page.locator('#inventory-submenu-trigger')).toHaveAttribute('aria-expanded', 'false');
    });

    test('submenu trigger should toggle aria-expanded on click', async ({ page }) => {
      await page.goto(`/${INVENTORY_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('#inventory-submenu-trigger')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      const trigger = page.locator('#inventory-submenu-trigger');
      // Hover over the tools-menu to open the CSS dropdown, making the trigger clickable
      await page.locator('.tools-menu').hover();
      await trigger.click();
      await expect(trigger).toHaveAttribute('aria-expanded', 'true');

      await trigger.click();
      await expect(trigger).toHaveAttribute('aria-expanded', 'false');
    });

    test('should not show inventory submenu on a non-inventory page', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      // Give the async menu init time to complete
      await page.waitForTimeout(1000);

      await expect(page.locator('#inventory-submenu')).not.toBeAttached();
    });
  });

  test.describe('Print Label Menu', () => {
    const LABEL_PRINTER_PAGE = 'e2elabelprinter';
    const CONTENT_PAGE = 'e2eprinttest';

    test.beforeAll(async ({ browser }) => {
      const ctx = await browser.newContext();
      const page = await ctx.newPage();

      // Create a label printer page
      await page.goto(`/${LABEL_PRINTER_PAGE}/edit`);
      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await textarea.fill(`+++
identifier = "${LABEL_PRINTER_PAGE}"
title = "E2E Test Label Printer"

[label_printer]
type = "zebra"
+++

# E2E Test Label Printer`);
      await textarea.press('Space');
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });

      // Create a content page to print from
      await page.goto(`/${CONTENT_PAGE}/edit`);
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await textarea.fill(`+++
identifier = "${CONTENT_PAGE}"
title = "E2E Print Test Page"
+++

# E2E Print Test Page`);
      await textarea.press('Space');
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });

      await ctx.close();
    });

    test.afterAll(async ({ browser }) => {
      const ctx = await browser.newContext();
      const page = await ctx.newPage();
      try {
        for (const pageName of [LABEL_PRINTER_PAGE, CONTENT_PAGE]) {
          await page.goto(`/${pageName}/edit`);
          const textarea = page.locator('wiki-editor textarea');
          await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
          await textarea.fill(`+++\nidentifier = "${pageName}"\n+++`);
          await textarea.press('Space');
          await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });
        }
      } catch (_) {
        // Best effort cleanup
      } finally {
        await ctx.close();
      }
    });

    test('should show a Print menu item when label printer pages exist', async ({ page }) => {
      await page.goto(`/${CONTENT_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // initPrintMenu fetches label printers and injects menu items
      const printLink = page.locator('.pure-menu-link', { hasText: /Print.*E2E Test Label Printer/i });
      await expect(printLink).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });
    });
  });
});
