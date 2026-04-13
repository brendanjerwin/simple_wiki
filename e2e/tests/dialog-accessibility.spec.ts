import { test, expect, type Page } from '@playwright/test';
import { COMPONENT_LOAD_TIMEOUT_MS } from './constants.js';

// Timeouts (local — not shared across spec files)
const DIALOG_TIMEOUT_MS = 10000;

/**
 * Adds a focusable trigger button to the page and focuses it.
 * Used to test focus restoration after dialog close.
 */
async function addFocusedTriggerButton(page: Page): Promise<void> {
  await page.evaluate(() => {
    const existing = document.getElementById('e2e-trigger-button');
    if (!existing) {
      const btn = document.createElement('button');
      btn.id = 'e2e-trigger-button';
      btn.textContent = 'Trigger';
      document.body.prepend(btn);
    }
    (document.getElementById('e2e-trigger-button') as HTMLButtonElement).focus();
  });
}

/**
 * Opens the ConfirmationDialog via the real #erasePage user flow.
 *
 * pageDeleteService pre-creates <confirmation-dialog id="page-deletion-dialog">
 * at module load time, so showModal() fires on an already-rendered element —
 * no Lit initialization timing issues. Polls via waitForFunction until the
 * native <dialog> is actually open and has layout (handles browser paint delay).
 */
async function openConfirmationDialog(page: Page): Promise<void> {
  await page.locator('.tools-menu').hover();
  await page.locator('#erasePage').click();
  await page.waitForFunction(
    () => {
      const host = document.querySelector('confirmation-dialog');
      const dlg = host?.shadowRoot?.querySelector('dialog');
      return dlg?.open === true && (dlg?.offsetHeight ?? 0) > 0;
    },
    undefined,
    { timeout: DIALOG_TIMEOUT_MS }
  );
}

/**
 * Opens the FrontmatterEditorDialog via JavaScript evaluation.
 * The element is already in the DOM at id="frontmatter-dialog".
 */
async function openFrontmatterEditorDialog(page: Page): Promise<void> {
  await page.evaluate(() => {
    const dialog = document.querySelector('frontmatter-editor-dialog') as any;
    dialog?.openDialog('home');
  });
}

test.describe('Dialog Accessibility E2E Tests', () => {
  test.setTimeout(60000);

  test.describe('ConfirmationDialog', () => {
    test.describe('ARIA attributes', () => {
      test('dialog element has aria-labelledby attribute', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        // Playwright's chained locators automatically pierce shadow DOM
        const dlg = page.locator('confirmation-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        const ariaLabelledBy = await dlg.getAttribute('aria-labelledby');
        expect(ariaLabelledBy).toBeTruthy();
      });

      test('aria-labelledby references an element containing the dialog message', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        const dlg = page.locator('confirmation-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        const ariaLabelledBy = await dlg.getAttribute('aria-labelledby');
        expect(ariaLabelledBy).toBeTruthy();

        // Use chained locator to pierce shadow DOM and find the labelled element by ID
        const labelText = await page.locator('confirmation-dialog').locator(`[id="${ariaLabelledBy}"]`).textContent();
        // The real message from pageDeleteService.confirmAndDeletePage()
        expect(labelText?.trim()).toBe('Are you sure you want to delete this page?');
      });

      test('uses a native dialog element providing implicit role="dialog"', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        const dlg = page.locator('confirmation-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Evaluate on the locator-resolved element (Playwright already pierced shadow DOM)
        const tagName = await dlg.evaluate(el => el.tagName.toLowerCase());
        // The native <dialog> element has implicit role="dialog" per the HTML spec
        expect(tagName).toBe('dialog');
      });
    });

    test.describe('when Escape key is pressed', () => {
      test('closes the dialog', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        const dlg = page.locator('confirmation-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.keyboard.press('Escape');

        await expect(dlg).not.toBeVisible();
      });
    });

    test.describe('when dialog backdrop is clicked', () => {
      test('closes the dialog', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        const dlg = page.locator('confirmation-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Click the <dialog> element near its edge (outside the dialog-box content)
        // to simulate a backdrop click. Position { x: 5, y: 5 } targets the top-left
        // corner of the <dialog> element, which is guaranteed to be outside the
        // centered .dialog-box panel.
        await dlg.click({ position: { x: 5, y: 5 } });

        await expect(dlg).not.toBeVisible();
      });
    });

    test.describe('focus management', () => {
      test('sets focus inside the dialog when opened', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        await expect(page.locator('confirmation-dialog').locator('dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // When focus is inside a shadow DOM, document.activeElement returns the shadow host —
        // not the element within the shadow tree. This is the correct cross-browser check.
        const focusedIsInsideDialog = await page.evaluate(() => {
          return document.activeElement?.tagName.toLowerCase() === 'confirmation-dialog';
        });

        expect(focusedIsInsideDialog).toBe(true);
      });

      test('restores focus to the trigger element when closed via Cancel button', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        // The hover+click on #erasePage moves focus there; dialog captures it and
        // restores focus to #erasePage after close.
        await openConfirmationDialog(page);
        const dlg = page.locator('confirmation-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.locator('confirmation-dialog').locator('.button-cancel').click();
        await expect(dlg).not.toBeVisible();

        await expect(page.locator('#erasePage')).toBeFocused();
      });

      test('restores focus to the trigger element when closed via Escape key', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        // The hover+click on #erasePage moves focus there; dialog captures it and
        // restores focus to #erasePage after close.
        await openConfirmationDialog(page);
        const dlg = page.locator('confirmation-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.keyboard.press('Escape');
        await expect(dlg).not.toBeVisible();

        await expect(page.locator('#erasePage')).toBeFocused();
      });

      test('traps focus within dialog when Tab key is pressed', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        await expect(page.locator('confirmation-dialog').locator('dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Tab through dialog elements multiple times — native <dialog> with showModal()
        // provides built-in focus trapping so focus should never leave the dialog.
        for (let i = 0; i < 5; i++) {
          await page.keyboard.press('Tab');

          // When focus is inside a shadow DOM, document.activeElement returns the shadow host
          const focusedIsInsideDialog = await page.evaluate(() => {
            return document.activeElement?.tagName.toLowerCase() === 'confirmation-dialog';
          });

          expect(focusedIsInsideDialog).toBe(true);
        }

        // Dialog should still be visible — Tab does not close it
        await expect(page.locator('confirmation-dialog').locator('dialog')).toBeVisible();
      });
    });

    test.describe('keyboard navigation', () => {
      test('moves focus between Cancel and Confirm buttons when Tab is pressed', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        await expect(page.locator('confirmation-dialog').locator('dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Playwright's chained locators pierce shadow DOM to find buttons inside the dialog
        const cancelButton = page.locator('confirmation-dialog').locator('.button-cancel');
        // confirmVariant is 'danger' from pageDeleteService, so the confirm button has class button-danger
        const confirmButton = page.locator('confirmation-dialog').locator('.button-danger');

        await cancelButton.focus();
        await expect(cancelButton).toBeFocused();

        // Tab moves from Cancel to Confirm
        await page.keyboard.press('Tab');
        await expect(confirmButton).toBeFocused();

        // Tab wraps back to Cancel (native dialog focus cycle)
        await page.keyboard.press('Tab');
        await expect(cancelButton).toBeFocused();
      });

      test('can activate Cancel button with Enter key', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        const dlg = page.locator('confirmation-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.locator('confirmation-dialog').locator('.button-cancel').focus();
        await page.keyboard.press('Enter');

        await expect(dlg).not.toBeVisible();
      });
    });
  });

  test.describe('FrontmatterEditorDialog', () => {
    test.describe('ARIA attributes', () => {
      test('dialog element has aria-labelledby attribute', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openFrontmatterEditorDialog(page);
        // Playwright's chained locators automatically pierce shadow DOM
        const dlg = page.locator('frontmatter-editor-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        const ariaLabelledBy = await dlg.getAttribute('aria-labelledby');
        expect(ariaLabelledBy).toBeTruthy();
      });

      test('aria-labelledby references the dialog title heading', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openFrontmatterEditorDialog(page);
        const dlg = page.locator('frontmatter-editor-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        const ariaLabelledBy = await dlg.getAttribute('aria-labelledby');
        expect(ariaLabelledBy).toBeTruthy();

        // Use chained locator to pierce shadow DOM and find the labelled element by ID
        const labelText = await page.locator('frontmatter-editor-dialog').locator(`[id="${ariaLabelledBy}"]`).textContent();
        expect(labelText?.trim()).toContain('Edit Frontmatter');
      });

      test('uses a native dialog element providing implicit role="dialog"', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openFrontmatterEditorDialog(page);
        const dlg = page.locator('frontmatter-editor-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Evaluate on the locator-resolved element (Playwright already pierced shadow DOM)
        const tagName = await dlg.evaluate(el => el.tagName.toLowerCase());
        // The native <dialog> element has implicit role="dialog" per the HTML spec
        expect(tagName).toBe('dialog');
      });
    });

    test.describe('when Escape key is pressed', () => {
      test('closes the dialog', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openFrontmatterEditorDialog(page);
        const dlg = page.locator('frontmatter-editor-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.keyboard.press('Escape');

        await expect(dlg).not.toBeVisible();
      });
    });

    test.describe('focus management', () => {
      test('restores focus to previously focused element when closed via Cancel button', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await addFocusedTriggerButton(page);
        await openFrontmatterEditorDialog(page);
        const dlg = page.locator('frontmatter-editor-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.locator('frontmatter-editor-dialog').locator('.button-secondary').click();
        await expect(dlg).not.toBeVisible();

        await expect(page.locator('#e2e-trigger-button')).toBeFocused();
      });

      test('restores focus to previously focused element when closed via Escape key', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await addFocusedTriggerButton(page);
        await openFrontmatterEditorDialog(page);
        const dlg = page.locator('frontmatter-editor-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.keyboard.press('Escape');
        await expect(dlg).not.toBeVisible();

        await expect(page.locator('#e2e-trigger-button')).toBeFocused();
      });

      test('traps focus within dialog when Tab key is pressed', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openFrontmatterEditorDialog(page);
        await expect(page.locator('frontmatter-editor-dialog').locator('dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Tab through dialog elements multiple times — native <dialog> with showModal()
        // provides built-in focus trapping so focus should never leave the dialog.
        for (let i = 0; i < 5; i++) {
          await page.keyboard.press('Tab');

          // When focus is inside a shadow DOM, document.activeElement returns the shadow host
          const focusedIsInsideDialog = await page.evaluate(() => {
            return document.activeElement?.tagName.toLowerCase() === 'frontmatter-editor-dialog';
          });

          expect(focusedIsInsideDialog).toBe(true);
        }

        // Dialog should still be visible — Tab does not close it
        await expect(page.locator('frontmatter-editor-dialog').locator('dialog')).toBeVisible();
      });
    });

    test.describe('keyboard navigation', () => {
      test('can activate Cancel button with Enter key', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openFrontmatterEditorDialog(page);
        const dlg = page.locator('frontmatter-editor-dialog').locator('dialog');
        await expect(dlg).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.locator('frontmatter-editor-dialog').locator('.button-secondary').focus();
        await page.keyboard.press('Enter');

        await expect(dlg).not.toBeVisible();
      });
    });
  });
});
