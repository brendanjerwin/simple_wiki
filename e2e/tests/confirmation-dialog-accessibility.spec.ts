import { test, expect } from '@playwright/test';
import { COMPONENT_LOAD_TIMEOUT_MS } from './constants.js';

// Playwright automatically pierces open shadow roots when evaluating CSS selectors,
// so selectors like `#page-deletion-dialog .button-cancel` work without explicit
// shadow DOM traversal.

// Timeouts
const DIALOG_APPEAR_TIMEOUT_MS = 5000;

/**
 * E2E accessibility tests for ConfirmationDialog using the real page deletion flow.
 *
 * Opens the dialog via the tools-menu Erase button on /home/view so that the full
 * production code path (page-deletion-service → confirmation-dialog) is exercised.
 * All tests only click Cancel or press Escape — the home page is never actually deleted.
 */
test.describe('ConfirmationDialog accessibility (via erasePage flow)', () => {
  test.setTimeout(60000);

  test.beforeEach(async ({ page }) => {
    await page.goto('/home/view');
    await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await page.locator('.tools-menu').hover();
    await page.locator('#erasePage').click();

    await expect(page.locator('#page-deletion-dialog[open]')).toBeAttached({
      timeout: DIALOG_APPEAR_TIMEOUT_MS,
    });
  });

  test.afterEach(async ({ page }) => {
    // Close dialog if still open after a test (e.g. focus trap test that didn't close it)
    await page.evaluate(() => {
      const el = document.getElementById('page-deletion-dialog') as (HTMLElement & { closeDialog?: () => void }) | null;
      if (el?.getAttribute('open') !== null) {
        el?.closeDialog?.();
      }
    });
  });

  // ──────────────────────────────────────────────────────────────
  // ARIA attributes
  // ──────────────────────────────────────────────────────────────

  test.describe('ARIA attributes', () => {

    test.describe('when the dialog is open', () => {

      test('should use a native <dialog> element (implicit role="dialog")', async ({ page }) => {
        const tagName = await page.evaluate(() =>
          document.getElementById('page-deletion-dialog')?.shadowRoot?.querySelector('dialog')?.tagName.toLowerCase()
        );
        expect(tagName).toBe('dialog');
      });

      test('should have aria-labelledby set on the native dialog element', async ({ page }) => {
        const ariaLabelledby = await page.evaluate(() =>
          document.getElementById('page-deletion-dialog')?.shadowRoot?.querySelector('dialog')?.getAttribute('aria-labelledby')
        );
        expect(ariaLabelledby).toBe('confirmation-dialog-title');
      });

      test('should have a labelled element that contains the delete message', async ({ page }) => {
        const labelText = await page.evaluate(() => {
          const shadowRoot = document.getElementById('page-deletion-dialog')?.shadowRoot;
          return shadowRoot?.getElementById('confirmation-dialog-title')?.textContent?.trim();
        });
        expect(labelText).toContain('Are you sure you want to delete this page?');
      });

    });

  });

  // ──────────────────────────────────────────────────────────────
  // Keyboard dismissal
  // ──────────────────────────────────────────────────────────────

  test.describe('keyboard dismissal', () => {

    test.describe('when Escape is pressed', () => {
      test.beforeEach(async ({ page }) => {
        await page.keyboard.press('Escape');
      });

      test('should close the dialog', async ({ page }) => {
        await expect(page.locator('#page-deletion-dialog[open]')).not.toBeAttached({
          timeout: DIALOG_APPEAR_TIMEOUT_MS,
        });
      });
    });

  });

  // ──────────────────────────────────────────────────────────────
  // Backdrop click
  // ──────────────────────────────────────────────────────────────

  test.describe('backdrop click', () => {

    test.describe('when the backdrop area of the dialog is clicked', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(() => {
          const dialogEl = document.getElementById('page-deletion-dialog')?.shadowRoot?.querySelector('dialog');
          dialogEl?.click();
        });
      });

      test('should close the dialog', async ({ page }) => {
        await expect(page.locator('#page-deletion-dialog[open]')).not.toBeAttached({
          timeout: DIALOG_APPEAR_TIMEOUT_MS,
        });
      });
    });

  });

  // ──────────────────────────────────────────────────────────────
  // Focus management
  // ──────────────────────────────────────────────────────────────

  test.describe('focus management', () => {

    test.describe('when the dialog opens', () => {

      test('should move focus inside the dialog shadow root', async ({ page }) => {
        await expect.poll(
          () => page.evaluate(() => {
            const host = document.getElementById('page-deletion-dialog');
            return host?.shadowRoot?.activeElement !== null;
          }),
          { timeout: DIALOG_APPEAR_TIMEOUT_MS },
        ).toBe(true);
      });

    });

    test.describe('when Cancel button is clicked', () => {
      test.beforeEach(async ({ page }) => {
        await page.locator('#page-deletion-dialog .button-cancel').click();
        await expect(page.locator('#page-deletion-dialog[open]')).not.toBeAttached({
          timeout: DIALOG_APPEAR_TIMEOUT_MS,
        });
      });

      test('should return focus to the #erasePage button', async ({ page }) => {
        await expect.poll(
          () => page.evaluate(() => document.activeElement?.id === 'erasePage'),
          { timeout: DIALOG_APPEAR_TIMEOUT_MS },
        ).toBe(true);
      });
    });

    test.describe('when Escape is pressed', () => {
      test.beforeEach(async ({ page }) => {
        await page.keyboard.press('Escape');
        await expect(page.locator('#page-deletion-dialog[open]')).not.toBeAttached({
          timeout: DIALOG_APPEAR_TIMEOUT_MS,
        });
      });

      test('should return focus to the #erasePage button', async ({ page }) => {
        await expect.poll(
          () => page.evaluate(() => document.activeElement?.id === 'erasePage'),
          { timeout: DIALOG_APPEAR_TIMEOUT_MS },
        ).toBe(true);
      });
    });

  });

  // ──────────────────────────────────────────────────────────────
  // Focus trap
  // ──────────────────────────────────────────────────────────────

  test.describe('focus trap', () => {

    test.beforeEach(async ({ page }) => {
      // Wait for initial focus to land on the cancel button (autofocus)
      await expect.poll(
        () => page.evaluate(() => {
          const host = document.getElementById('page-deletion-dialog');
          return host?.shadowRoot?.activeElement !== null;
        }),
        { timeout: DIALOG_APPEAR_TIMEOUT_MS },
      ).toBe(true);
    });

    test.describe('when Tab is pressed from the Cancel button', () => {
      test.beforeEach(async ({ page }) => {
        // Focus the cancel button explicitly before tabbing
        await page.locator('#page-deletion-dialog .button-cancel').focus();
        await page.keyboard.press('Tab');
      });

      test('should move focus to the Confirm (danger) button', async ({ page }) => {
        const focusedClass = await page.evaluate(() => {
          const host = document.getElementById('page-deletion-dialog');
          return (host?.shadowRoot?.activeElement as HTMLElement | null)?.className ?? '';
        });
        expect(focusedClass).toContain('button-danger');
      });

      test('should keep the dialog open', async ({ page }) => {
        await expect(page.locator('#page-deletion-dialog[open]')).toBeAttached();
      });
    });

    test.describe('when Tab is pressed from the Confirm button', () => {
      test.beforeEach(async ({ page }) => {
        await page.locator('#page-deletion-dialog .button-danger').focus();
        await page.keyboard.press('Tab');
      });

      test('should wrap focus back to the Cancel button', async ({ page }) => {
        const focusedClass = await page.evaluate(() => {
          const host = document.getElementById('page-deletion-dialog');
          return (host?.shadowRoot?.activeElement as HTMLElement | null)?.className ?? '';
        });
        expect(focusedClass).toContain('button-cancel');
      });

      test('should keep the dialog open', async ({ page }) => {
        await expect(page.locator('#page-deletion-dialog[open]')).toBeAttached();
      });
    });

    test.describe('when Shift+Tab is pressed from the Cancel button', () => {
      test.beforeEach(async ({ page }) => {
        await page.locator('#page-deletion-dialog .button-cancel').focus();
        await page.keyboard.press('Shift+Tab');
      });

      test('should wrap focus to the Confirm (danger) button', async ({ page }) => {
        const focusedClass = await page.evaluate(() => {
          const host = document.getElementById('page-deletion-dialog');
          return (host?.shadowRoot?.activeElement as HTMLElement | null)?.className ?? '';
        });
        expect(focusedClass).toContain('button-danger');
      });

      test('should keep the dialog open', async ({ page }) => {
        await expect(page.locator('#page-deletion-dialog[open]')).toBeAttached();
      });
    });

  });

  // ──────────────────────────────────────────────────────────────
  // Keyboard activation
  // ──────────────────────────────────────────────────────────────

  test.describe('keyboard activation', () => {

    test.describe('when Enter is pressed while Cancel button is focused', () => {
      test.beforeEach(async ({ page }) => {
        await page.locator('#page-deletion-dialog .button-cancel').focus();
        await page.keyboard.press('Enter');
      });

      test('should close the dialog', async ({ page }) => {
        await expect(page.locator('#page-deletion-dialog[open]')).not.toBeAttached({
          timeout: DIALOG_APPEAR_TIMEOUT_MS,
        });
      });
    });

  });

});
