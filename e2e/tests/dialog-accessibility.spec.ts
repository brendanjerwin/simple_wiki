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
 * Opens the ConfirmationDialog via JavaScript evaluation.
 * The component is already registered via the JS bundle.
 */
async function openConfirmationDialog(page: Page): Promise<void> {
  await page.evaluate(() => {
    let dialog = document.querySelector('confirmation-dialog');
    if (!dialog) {
      dialog = document.createElement('confirmation-dialog');
      document.body.appendChild(dialog);
    }
    (dialog as any).openDialog({
      message: 'Test Confirmation',
      description: 'Accessibility test dialog.',
      confirmText: 'Confirm',
      cancelText: 'Cancel',
      confirmVariant: 'primary',
    });
  });
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
        await expect(page.locator('confirmation-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        const ariaLabelledBy = await page.evaluate(() => {
          const host = document.querySelector('confirmation-dialog');
          const dlg = host?.shadowRoot?.querySelector('dialog');
          return dlg?.getAttribute('aria-labelledby') ?? null;
        });

        expect(ariaLabelledBy).toBeTruthy();
      });

      test('aria-labelledby references an element containing the dialog message', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        await expect(page.locator('confirmation-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        const labelText = await page.evaluate(() => {
          const host = document.querySelector('confirmation-dialog');
          const dlg = host?.shadowRoot?.querySelector('dialog');
          const labelId = dlg?.getAttribute('aria-labelledby');
          if (!labelId) return null;
          const labelEl = host?.shadowRoot?.getElementById(labelId);
          return labelEl?.textContent?.trim() ?? null;
        });

        expect(labelText).toBe('Test Confirmation');
      });

      test('uses a native dialog element providing implicit role="dialog"', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        await expect(page.locator('confirmation-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        const tagName = await page.evaluate(() => {
          const host = document.querySelector('confirmation-dialog');
          const dlg = host?.shadowRoot?.querySelector('dialog');
          return dlg?.tagName.toLowerCase() ?? null;
        });

        // The native <dialog> element has implicit role="dialog" per the HTML spec
        expect(tagName).toBe('dialog');
      });
    });

    test.describe('when Escape key is pressed', () => {
      test('closes the dialog', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        await expect(page.locator('confirmation-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.keyboard.press('Escape');

        await expect(page.locator('confirmation-dialog dialog')).not.toBeVisible();
      });
    });

    test.describe('when dialog backdrop is clicked', () => {
      test('closes the dialog', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        const dialogEl = page.locator('confirmation-dialog dialog');
        await expect(dialogEl).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Click the <dialog> element near its edge (outside the dialog-box content)
        // to simulate a backdrop click. Position { x: 5, y: 5 } targets the top-left
        // corner of the <dialog> element, which is guaranteed to be outside the
        // centered .dialog-box panel.
        await dialogEl.click({ position: { x: 5, y: 5 } });

        await expect(dialogEl).not.toBeVisible();
      });
    });

    test.describe('focus management', () => {
      test('sets focus inside the dialog when opened', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        await expect(page.locator('confirmation-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Focus should move inside the dialog's shadow root after showModal()
        const focusedIsInsideDialog = await page.evaluate(() => {
          const host = document.querySelector('confirmation-dialog');
          if (!host?.shadowRoot) return false;
          const dlg = host.shadowRoot.querySelector('dialog');
          const focused = host.shadowRoot.activeElement;
          return dlg !== null && focused !== null && dlg.contains(focused);
        });

        expect(focusedIsInsideDialog).toBe(true);
      });

      test('restores focus to previously focused element when closed via Cancel button', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await addFocusedTriggerButton(page);
        await openConfirmationDialog(page);
        await expect(page.locator('confirmation-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.locator('confirmation-dialog dialog .button-cancel').click();
        await expect(page.locator('confirmation-dialog dialog')).not.toBeVisible();

        await expect(page.locator('#e2e-trigger-button')).toBeFocused();
      });

      test('restores focus to previously focused element when closed via Escape key', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await addFocusedTriggerButton(page);
        await openConfirmationDialog(page);
        await expect(page.locator('confirmation-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.keyboard.press('Escape');
        await expect(page.locator('confirmation-dialog dialog')).not.toBeVisible();

        await expect(page.locator('#e2e-trigger-button')).toBeFocused();
      });

      test('traps focus within dialog when Tab key is pressed', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        await expect(page.locator('confirmation-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Tab through dialog elements multiple times — native <dialog> with showModal()
        // provides built-in focus trapping so focus should never leave the dialog.
        for (let i = 0; i < 5; i++) {
          await page.keyboard.press('Tab');

          const focusedIsInsideDialog = await page.evaluate(() => {
            const host = document.querySelector('confirmation-dialog');
            if (!host?.shadowRoot) return false;
            const dlg = host.shadowRoot.querySelector('dialog');
            const focused = host.shadowRoot.activeElement;
            return dlg !== null && focused !== null && dlg.contains(focused);
          });

          expect(focusedIsInsideDialog).toBe(true);
        }

        // Dialog should still be visible — Tab does not close it
        await expect(page.locator('confirmation-dialog dialog')).toBeVisible();
      });
    });

    test.describe('keyboard navigation', () => {
      test('moves focus between Cancel and Confirm buttons when Tab is pressed', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openConfirmationDialog(page);
        await expect(page.locator('confirmation-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        const cancelButton = page.locator('confirmation-dialog dialog .button-cancel');
        const confirmButton = page.locator('confirmation-dialog dialog .button-primary');

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
        await expect(page.locator('confirmation-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.locator('confirmation-dialog dialog .button-cancel').focus();
        await page.keyboard.press('Enter');

        await expect(page.locator('confirmation-dialog dialog')).not.toBeVisible();
      });
    });
  });

  test.describe('FrontmatterEditorDialog', () => {
    test.describe('ARIA attributes', () => {
      test('dialog element has aria-labelledby attribute', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openFrontmatterEditorDialog(page);
        await expect(page.locator('frontmatter-editor-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        const ariaLabelledBy = await page.evaluate(() => {
          const host = document.querySelector('frontmatter-editor-dialog');
          const dlg = host?.shadowRoot?.querySelector('dialog');
          return dlg?.getAttribute('aria-labelledby') ?? null;
        });

        expect(ariaLabelledBy).toBeTruthy();
      });

      test('aria-labelledby references the dialog title heading', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openFrontmatterEditorDialog(page);
        await expect(page.locator('frontmatter-editor-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        const labelText = await page.evaluate(() => {
          const host = document.querySelector('frontmatter-editor-dialog');
          const dlg = host?.shadowRoot?.querySelector('dialog');
          const labelId = dlg?.getAttribute('aria-labelledby');
          if (!labelId) return null;
          const labelEl = host?.shadowRoot?.getElementById(labelId);
          return labelEl?.textContent?.trim() ?? null;
        });

        expect(labelText).toContain('Edit Frontmatter');
      });

      test('uses a native dialog element providing implicit role="dialog"', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openFrontmatterEditorDialog(page);
        await expect(page.locator('frontmatter-editor-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        const tagName = await page.evaluate(() => {
          const host = document.querySelector('frontmatter-editor-dialog');
          const dlg = host?.shadowRoot?.querySelector('dialog');
          return dlg?.tagName.toLowerCase() ?? null;
        });

        // The native <dialog> element has implicit role="dialog" per the HTML spec
        expect(tagName).toBe('dialog');
      });
    });

    test.describe('when Escape key is pressed', () => {
      test('closes the dialog', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openFrontmatterEditorDialog(page);
        await expect(page.locator('frontmatter-editor-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.keyboard.press('Escape');

        await expect(page.locator('frontmatter-editor-dialog dialog')).not.toBeVisible();
      });
    });

    test.describe('focus management', () => {
      test('restores focus to previously focused element when closed via Cancel button', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await addFocusedTriggerButton(page);
        await openFrontmatterEditorDialog(page);
        await expect(page.locator('frontmatter-editor-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.locator('frontmatter-editor-dialog dialog .button-secondary').click();
        await expect(page.locator('frontmatter-editor-dialog dialog')).not.toBeVisible();

        await expect(page.locator('#e2e-trigger-button')).toBeFocused();
      });

      test('restores focus to previously focused element when closed via Escape key', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await addFocusedTriggerButton(page);
        await openFrontmatterEditorDialog(page);
        await expect(page.locator('frontmatter-editor-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.keyboard.press('Escape');
        await expect(page.locator('frontmatter-editor-dialog dialog')).not.toBeVisible();

        await expect(page.locator('#e2e-trigger-button')).toBeFocused();
      });

      test('traps focus within dialog when Tab key is pressed', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openFrontmatterEditorDialog(page);
        await expect(page.locator('frontmatter-editor-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        // Tab through dialog elements multiple times — native <dialog> with showModal()
        // provides built-in focus trapping so focus should never leave the dialog.
        for (let i = 0; i < 5; i++) {
          await page.keyboard.press('Tab');

          const focusedIsInsideDialog = await page.evaluate(() => {
            const host = document.querySelector('frontmatter-editor-dialog');
            if (!host?.shadowRoot) return false;
            const dlg = host.shadowRoot.querySelector('dialog');
            const focused = host.shadowRoot.activeElement;
            return dlg !== null && focused !== null && dlg.contains(focused);
          });

          expect(focusedIsInsideDialog).toBe(true);
        }

        // Dialog should still be visible — Tab does not close it
        await expect(page.locator('frontmatter-editor-dialog dialog')).toBeVisible();
      });
    });

    test.describe('keyboard navigation', () => {
      test('can activate Cancel button with Enter key', async ({ page }) => {
        await page.goto('/home/edit');
        await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await openFrontmatterEditorDialog(page);
        await expect(page.locator('frontmatter-editor-dialog dialog')).toBeVisible({ timeout: DIALOG_TIMEOUT_MS });

        await page.locator('frontmatter-editor-dialog dialog .button-secondary').focus();
        await page.keyboard.press('Enter');

        await expect(page.locator('frontmatter-editor-dialog dialog')).not.toBeVisible();
      });
    });
  });
});
