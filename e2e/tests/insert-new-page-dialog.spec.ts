import { test, expect, type Page } from '@playwright/test';
import { COMPONENT_LOAD_TIMEOUT_MS, IDENTIFIER_GENERATE_TIMEOUT_MS } from './constants.js';

// Selectors
const FOOTER_PRIMARY_BUTTON = '.footer button.button-primary';

// Timeouts (local — not shared across spec files)
const DIALOG_TIMEOUT_MS = 10000;
const SAVE_TIMEOUT_MS = 10000;

// Unique prefix for all pages created in this test suite (used for cleanup)
const TEST_PAGE_PREFIX = 'e2e_insert_page';

/**
 * Opens the InsertNewPageDialog via JavaScript evaluation.
 * Creates the element if it doesn't exist yet, then calls openDialog().
 * Note: Does NOT wire up the page-created event listener from EditorToolbarCoordinator.
 * Use the toolbar button click instead when testing page creation.
 */
async function openInsertNewPageDialog(page: Page): Promise<void> {
  await page.evaluate(() => {
    let dialog = document.querySelector('insert-new-page-dialog');
    if (!dialog) {
      dialog = document.createElement('insert-new-page-dialog');
      document.body.appendChild(dialog);
    }
    (dialog as any).openDialog();
  });
}

test.describe('InsertNewPageDialog E2E Tests', () => {
  test.setTimeout(60000);

  test.afterAll(async ({ browser }) => {
    // Clean up any pages created during this test suite
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    try {
      await page.goto('/ls');
      await expect(page.locator('h1')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const pageLinks = await page.locator(`a[href*="/${TEST_PAGE_PREFIX}"]`).all();
      for (const link of pageLinks) {
        const href = await link.getAttribute('href');
        if (!href) continue;
        const identifier = href.replace('/view', '').replace(/^\//, '');

        await page.goto(`/${identifier}/edit`);
        const textarea = page.locator('wiki-editor textarea');
        await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
        await textarea.fill(`+++\nidentifier = "${identifier}"\n+++`);
        await textarea.press('Space');
        await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
          timeout: SAVE_TIMEOUT_MS,
        });
      }
    } catch (_) {
      // Best effort cleanup
    } finally {
      await ctx.close();
    }
  });

  test('should open dialog when clicking toolbar new page button', async ({ page }) => {
    await page.goto('/home/edit');

    const textarea = page.locator('wiki-editor textarea');
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Click the "Create & Link New Page" toolbar button
    const newPageButton = page.locator('wiki-editor editor-toolbar button[data-action="new-page"]');
    await expect(newPageButton).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await newPageButton.click();

    // Dialog should be created and attached to the DOM
    const dialog = page.locator('insert-new-page-dialog');
    await expect(dialog).toBeAttached({ timeout: DIALOG_TIMEOUT_MS });

    // Dialog panel should become visible
    await expect(dialog.locator('.dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    // Verify dialog header text
    await expect(dialog.locator('.dialog-title')).toContainText('Insert New Page');
  });

  test('should display dialog form elements when open', async ({ page }) => {
    await page.goto('/home/edit');
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await openInsertNewPageDialog(page);

    const dialog = page.locator('insert-new-page-dialog');
    await expect(dialog.locator('.dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    // Should render the automagic identifier input component
    await expect(dialog.locator('automagic-identifier-input')).toBeAttached();

    // Should render the template selector
    await expect(dialog.locator('select[name="template"]')).toBeAttached();

    // Should render Cancel and Create Page buttons
    await expect(dialog.locator('button.button-secondary')).toContainText('Cancel');
    await expect(dialog.locator(FOOTER_PRIMARY_BUTTON)).toContainText('Create Page');
  });

  test('should close dialog when clicking Cancel button', async ({ page }) => {
    await page.goto('/home/edit');
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await openInsertNewPageDialog(page);

    const dialog = page.locator('insert-new-page-dialog');
    await expect(dialog.locator('.dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    // Click the Cancel button
    await dialog.locator('button.button-secondary').click();

    // Dialog should close
    await expect(dialog.locator('.dialog')).not.toBeVisible();
  });

  test('should close dialog when clicking backdrop', async ({ page }) => {
    await page.goto('/home/edit');
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await openInsertNewPageDialog(page);

    const dialog = page.locator('insert-new-page-dialog');
    await expect(dialog.locator('.dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    // Click the backdrop element (behind the dialog panel)
    // Use position: { x: 10, y: 10 } to click near the top-left corner, which is
    // guaranteed to be outside the centered dialog panel.
    await dialog.locator('.backdrop').click({ position: { x: 10, y: 10 } });

    // Dialog should close
    await expect(dialog.locator('.dialog')).not.toBeVisible();
  });

  test('should close dialog when pressing Escape key', async ({ page }) => {
    await page.goto('/home/edit');
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await openInsertNewPageDialog(page);

    const dialog = page.locator('insert-new-page-dialog');
    await expect(dialog.locator('.dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    // Press Escape key — the component listens for keydown on document
    await page.keyboard.press('Escape');

    // Dialog should close
    await expect(dialog.locator('.dialog')).not.toBeVisible();
  });

  test('should disable Create Page button when identifier is empty', async ({ page }) => {
    await page.goto('/home/edit');
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await openInsertNewPageDialog(page);

    const dialog = page.locator('insert-new-page-dialog');
    await expect(dialog.locator('.dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    // Create Page button should be disabled when no identifier is set
    await expect(dialog.locator(FOOTER_PRIMARY_BUTTON)).toBeDisabled();
  });

  test('should enable Create Page button after entering a title', async ({ page }) => {
    const timestamp = Date.now();

    await page.goto('/home/edit');
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await openInsertNewPageDialog(page);

    const dialog = page.locator('insert-new-page-dialog');
    await expect(dialog.locator('.dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    // Type a unique title to trigger automagic identifier generation (debounce + gRPC call)
    const titleInput = dialog.locator('automagic-identifier-input title-input input');
    await titleInput.fill(`E2E Test Title ${timestamp}`);

    // Wait for the identifier to be generated and Create Page button to become enabled
    await expect(dialog.locator(FOOTER_PRIMARY_BUTTON)).toBeEnabled({
      timeout: IDENTIFIER_GENERATE_TIMEOUT_MS,
    });
  });

  test('should create a new page and insert markdown link in editor', async ({ page }) => {
    const uniqueIdentifier = `${TEST_PAGE_PREFIX}_${Date.now()}`;

    await page.goto('/home/edit');

    const editorTextarea = page.locator('wiki-editor textarea');
    await expect(editorTextarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Use toolbar button to open dialog so EditorToolbarCoordinator wires up
    // the page-created event listener that inserts the markdown link
    const newPageButton = page.locator('wiki-editor editor-toolbar button[data-action="new-page"]');
    await newPageButton.click();

    const dialog = page.locator('insert-new-page-dialog');
    await expect(dialog.locator('.dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    // Set identifier directly via evaluate to bypass gRPC debounce timing
    await page.evaluate((id) => {
      const dialogEl = document.querySelector('insert-new-page-dialog');
      if (dialogEl) {
        (dialogEl as any).pageIdentifier = id;
        (dialogEl as any).pageTitle = 'E2E Insert Test Page';
        (dialogEl as any).isUnique = true;
      }
    }, uniqueIdentifier);

    // Wait for Create Page button to become enabled after Lit re-render
    const createButton = dialog.locator(FOOTER_PRIMARY_BUTTON);
    await expect(createButton).toBeEnabled({ timeout: DIALOG_TIMEOUT_MS });

    // Click Create Page — this triggers the gRPC CreatePage call
    await createButton.click();

    // Dialog should close after successful page creation
    await expect(dialog.locator('.dialog')).not.toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    // The markdown link should be inserted into the editor textarea
    const editorContent = await editorTextarea.inputValue();
    expect(editorContent).toContain(`/${uniqueIdentifier}`);
  });

  test('should not trigger unhandled promise rejections when opening dialog', async ({ page }) => {
    const unhandledRejections: string[] = [];

    // Listen for unhandled rejections — PR #776 fixed this exact bug
    page.on('pageerror', (error) => {
      unhandledRejections.push(error.message);
    });

    await page.goto('/home/edit');
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Open the dialog (this was the source of the unhandled rejection fixed in PR #776)
    await openInsertNewPageDialog(page);

    const dialog = page.locator('insert-new-page-dialog');
    await expect(dialog.locator('.dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    // Wait for the template-loading async operation to complete
    await page.waitForTimeout(2000);

    // No unhandled promise rejections should have occurred
    expect(unhandledRejections).toHaveLength(0);
  });

  test('should support Tab key navigation within the dialog', async ({ page }) => {
    await page.goto('/home/edit');
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await openInsertNewPageDialog(page);

    const dialog = page.locator('insert-new-page-dialog');
    await expect(dialog.locator('.dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    // Focus the title input to start Tab navigation from a known position
    const titleInput = dialog.locator('automagic-identifier-input title-input input');
    await titleInput.click();
    await expect(titleInput).toBeFocused();

    // Tab should move focus within the dialog without closing it
    await page.keyboard.press('Tab');
    await expect(dialog.locator('.dialog')).toBeVisible();

    // Dialog should remain open after tabbing
    await expect(dialog).toHaveAttribute('open');
  });

  test('should reset form state when dialog is reopened', async ({ page }) => {
    await page.goto('/home/edit');
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Open the dialog and enter a title
    await openInsertNewPageDialog(page);
    const dialog = page.locator('insert-new-page-dialog');
    await expect(dialog.locator('.dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    const titleInput = dialog.locator('automagic-identifier-input title-input input');
    await titleInput.fill('Some Title To Clear');

    // Close the dialog via Cancel
    await dialog.locator('button.button-secondary').click();
    await expect(dialog.locator('.dialog')).not.toBeVisible();

    // Reopen the dialog
    await openInsertNewPageDialog(page);
    await expect(dialog.locator('.dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

    // Title input should be cleared after reset
    await expect(titleInput).toHaveValue('');

    // Create Page button should be disabled (no identifier after reset)
    await expect(dialog.locator(FOOTER_PRIMARY_BUTTON)).toBeDisabled();
  });
});
