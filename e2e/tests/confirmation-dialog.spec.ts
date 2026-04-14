import { test, expect } from '@playwright/test';
import { COMPONENT_LOAD_TIMEOUT_MS } from './constants.js';

// Playwright automatically pierces open shadow roots when evaluating CSS selectors,
// so selectors like `#e2e-test-cd .button-cancel` work without explicit shadow DOM traversal.
// Focus assertions use page.evaluate() since Playwright's toBeFocused() checks
// document.activeElement, which points to the shadow host rather than the focused element
// within an open shadow root.

// Extended window type for event promise storage
type E2EWindow = typeof window & {
  __e2eConfirmPromise?: Promise<boolean>;
  __e2eCancelPromise?: Promise<boolean>;
};

// Timeouts
const DIALOG_APPEAR_TIMEOUT_MS = 3000;
const EVENT_TIMEOUT_MS = 3000;

const DEFAULT_CONFIG = {
  message: 'Are you sure you want to delete?',
  description: 'This will permanently remove the item.',
  confirmText: 'Delete',
  cancelText: 'Cancel',
  confirmVariant: 'danger' as const,
  icon: 'warning',
};

/**
 * E2E tests for confirmation-dialog component.
 *
 * The component is injected directly into the wiki edit page (where all custom
 * elements are registered) so it runs in a real browser with the full app context.
 */
test.describe('confirmation-dialog', () => {
  test.setTimeout(60000);

  test.beforeEach(async ({ page }) => {
    await page.goto('/home/edit');
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    await page.evaluate(() => {
      const el = document.createElement('confirmation-dialog');
      el.id = 'e2e-test-cd';
      document.body.appendChild(el);
    });

    await expect(page.locator('#e2e-test-cd')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
  });

  test.afterEach(async ({ page }) => {
    await page.evaluate(() => {
      const el = document.getElementById('e2e-test-cd') as (HTMLElement & { closeDialog?: () => void }) | null;
      el?.closeDialog?.();
      el?.remove();
    });
  });

  // ────────────────────────────────────────────────────────────
  // Dialog lifecycle
  // ────────────────────────────────────────────────────────────

  test.describe('dialog lifecycle', () => {

    test.describe('when the dialog is opened', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate((cfg) => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog(cfg);
        }, DEFAULT_CONFIG);

        await expect(page.locator('#e2e-test-cd[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test('should display the configured message', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd .dialog-message')).toContainText('Are you sure you want to delete?');
      });

      test('should display the configured description', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd .dialog-description')).toContainText('This will permanently remove the item.');
      });

      test('should display the confirm button with the configured text', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd .button-danger')).toContainText('Delete');
      });

      test('should display the cancel button with the configured text', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd .button-cancel')).toContainText('Cancel');
      });

    });

    test.describe('when cancel is clicked', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate((cfg) => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog(cfg);
        }, DEFAULT_CONFIG);

        await expect(page.locator('#e2e-test-cd[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
        await page.locator('#e2e-test-cd .button-cancel').click();
      });

      test('should close the dialog', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd[open]')).not.toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

    });

    test.describe('when confirm is clicked', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(({ cfg, timeoutMs }: { cfg: typeof DEFAULT_CONFIG; timeoutMs: number }) => {
          const target = document.getElementById('e2e-test-cd');
          (window as E2EWindow).__e2eConfirmPromise = new Promise<boolean>((resolve) => {
            const timeoutId = setTimeout(() => resolve(false), timeoutMs);
            target?.addEventListener('confirm', () => {
              clearTimeout(timeoutId);
              resolve(true);
            }, { once: true });
          });
          (target as HTMLElement & { openDialog: (c: unknown) => void }).openDialog(cfg);
        }, { cfg: DEFAULT_CONFIG, timeoutMs: EVENT_TIMEOUT_MS });

        await expect(page.locator('#e2e-test-cd[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
        await page.locator('#e2e-test-cd .button-danger').click();
      });

      test('should dispatch the confirm event', async ({ page }) => {
        const wasConfirmed = await page.evaluate(() => (window as E2EWindow).__e2eConfirmPromise);
        expect(wasConfirmed).toBe(true);
      });

    });

    test.describe('when cancel event listener is attached and cancel is clicked', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(({ cfg, timeoutMs }: { cfg: typeof DEFAULT_CONFIG; timeoutMs: number }) => {
          const target = document.getElementById('e2e-test-cd');
          (window as E2EWindow).__e2eCancelPromise = new Promise<boolean>((resolve) => {
            const timeoutId = setTimeout(() => resolve(false), timeoutMs);
            target?.addEventListener('cancel', () => {
              clearTimeout(timeoutId);
              resolve(true);
            }, { once: true });
          });
          (target as HTMLElement & { openDialog: (c: unknown) => void }).openDialog(cfg);
        }, { cfg: DEFAULT_CONFIG, timeoutMs: EVENT_TIMEOUT_MS });

        await expect(page.locator('#e2e-test-cd[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
        await page.locator('#e2e-test-cd .button-cancel').click();
      });

      test('should dispatch the cancel event', async ({ page }) => {
        const wasCancelled = await page.evaluate(() => (window as E2EWindow).__e2eCancelPromise);
        expect(wasCancelled).toBe(true);
      });

    });

  });

  // ────────────────────────────────────────────────────────────
  // Native dialog behavior
  // ────────────────────────────────────────────────────────────

  test.describe('native dialog behavior', () => {

    test.describe('when the dialog is opened', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate((cfg) => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog(cfg);
        }, DEFAULT_CONFIG);

        await expect(page.locator('#e2e-test-cd dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test('should use a native <dialog> element', async ({ page }) => {
        const tagName = await page.evaluate(() =>
          document.getElementById('e2e-test-cd')?.shadowRoot?.querySelector('dialog')?.tagName.toLowerCase()
        );
        expect(tagName).toBe('dialog');
      });

      test('should have the native dialog in its open state', async ({ page }) => {
        const isOpen = await page.evaluate(() => {
          const dialogEl = document.getElementById('e2e-test-cd')?.shadowRoot?.querySelector('dialog') as HTMLDialogElement | null;
          return dialogEl?.open ?? false;
        });
        expect(isOpen).toBe(true);
      });

    });

    test.describe('when Escape is pressed', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate((cfg) => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog(cfg);
        }, DEFAULT_CONFIG);

        await expect(page.locator('#e2e-test-cd dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
        await page.keyboard.press('Escape');
      });

      test('should close the dialog', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd[open]')).not.toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

    });

    test.describe('when the backdrop is clicked', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate((cfg) => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog(cfg);
        }, DEFAULT_CONFIG);

        await expect(page.locator('#e2e-test-cd dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

        // Dispatch a click directly on the <dialog> element (target === currentTarget),
        // which is how the backdrop click handler detects a click outside the dialog box.
        await page.evaluate(() => {
          const dialogEl = document.getElementById('e2e-test-cd')?.shadowRoot?.querySelector('dialog');
          dialogEl?.click();
        });
      });

      test('should close the dialog', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd[open]')).not.toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

    });

  });

  // ────────────────────────────────────────────────────────────
  // Accessibility
  // ────────────────────────────────────────────────────────────

  test.describe('accessibility', () => {

    test.describe('when the dialog is opened', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate((cfg) => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog(cfg);
        }, DEFAULT_CONFIG);

        await expect(page.locator('#e2e-test-cd dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test('should use a native <dialog> element providing implicit ARIA role dialog', async ({ page }) => {
        // Native <dialog> elements have an implicit ARIA role of "dialog" per the spec;
        // there is no need to set role="dialog" explicitly.
        const tagName = await page.evaluate(() =>
          document.getElementById('e2e-test-cd')?.shadowRoot?.querySelector('dialog')?.tagName.toLowerCase()
        );
        expect(tagName).toBe('dialog');
      });

      test('should move focus into the dialog shadow root', async ({ page }) => {
        // After showModal() the browser moves focus inside the dialog; since the dialog
        // lives in a shadow root, document.activeElement points to the host while
        // shadowRoot.activeElement points to the focused button inside.
        await expect.poll(
          () => page.evaluate(() => {
            const host = document.getElementById('e2e-test-cd');
            return host?.shadowRoot?.activeElement !== null;
          }),
          { timeout: DIALOG_APPEAR_TIMEOUT_MS },
        ).toBe(true);
      });

    });

    test.describe('when the dialog is closed after being opened', () => {
      test.beforeEach(async ({ page }) => {
        // Make the host element focusable and focus it so the component captures it
        // as _previouslyFocusedElement for focus restoration.
        await page.evaluate(() => {
          const host = document.getElementById('e2e-test-cd') as HTMLElement;
          host.tabIndex = -1;
          host.focus();
        });

        await page.evaluate((cfg) => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog(cfg);
        }, DEFAULT_CONFIG);

        await expect(page.locator('#e2e-test-cd dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
        await page.locator('#e2e-test-cd .button-cancel').click();
        await expect(page.locator('#e2e-test-cd[open]')).not.toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test('should return focus to the previously focused element', async ({ page }) => {
        await expect.poll(
          () => page.evaluate(() => document.activeElement === document.getElementById('e2e-test-cd')),
          { timeout: DIALOG_APPEAR_TIMEOUT_MS },
        ).toBe(true);
      });

    });

    test.describe('focus trap', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate((cfg) => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog(cfg);
        }, DEFAULT_CONFIG);

        await expect(page.locator('#e2e-test-cd dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

        // Wait for initial focus to land inside the dialog
        await expect.poll(
          () => page.evaluate(() => {
            const host = document.getElementById('e2e-test-cd');
            return host?.shadowRoot?.activeElement !== null;
          }),
          { timeout: DIALOG_APPEAR_TIMEOUT_MS },
        ).toBe(true);
      });

      test('should keep focus within the dialog when Tab is pressed from the last button', async ({ page }) => {
        // Focus the confirm button (last focusable element)
        await page.locator('#e2e-test-cd .button-danger').focus();

        // Tab should wrap focus back inside the dialog (native <dialog> focus trap)
        await page.keyboard.press('Tab');

        const focusStillInDialog = await page.evaluate(() => {
          const host = document.getElementById('e2e-test-cd');
          return host?.shadowRoot?.activeElement !== null;
        });
        expect(focusStillInDialog).toBe(true);
      });

      test('should keep focus within the dialog when Shift+Tab is pressed from the first button', async ({ page }) => {
        // Focus the cancel button (first focusable element)
        await page.locator('#e2e-test-cd .button-cancel').focus();

        // Shift+Tab should wrap focus back inside the dialog
        await page.keyboard.press('Shift+Tab');

        const focusStillInDialog = await page.evaluate(() => {
          const host = document.getElementById('e2e-test-cd');
          return host?.shadowRoot?.activeElement !== null;
        });
        expect(focusStillInDialog).toBe(true);
      });

    });

  });

  // ────────────────────────────────────────────────────────────
  // Error states
  // ────────────────────────────────────────────────────────────

  test.describe('error states', () => {

    test.describe('when showError is called with an augmented error', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate((cfg) => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog(cfg);
        }, DEFAULT_CONFIG);

        await expect(page.locator('#e2e-test-cd[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

        // Pass a plain object that satisfies the AugmentedError shape accessed by error-display
        await page.evaluate(() => {
          const el = document.getElementById('e2e-test-cd') as HTMLElement & { showError: (e: unknown) => void };
          el.showError({
            message: 'Network error occurred',
            icon: 'network',
            stack: 'Error: Network error occurred',
            name: 'Error',
            errorKind: 'network',
            originalError: new Error('Network error occurred'),
            failedGoalDescription: 'deleting page',
            cause: undefined,
          });
        });
      });

      test('should render the error-display component within the dialog', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd error-display')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

    });

    test.describe('when setLoading is called with true', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate((cfg) => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog(cfg);
        }, DEFAULT_CONFIG);

        await expect(page.locator('#e2e-test-cd[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

        await page.evaluate(() => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { setLoading: (l: boolean) => void }).setLoading(true);
        });
      });

      test('should show the loading text on the confirm button', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd .button-danger')).toContainText('Processing...', { timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test('should disable the cancel button', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd .button-cancel')).toBeDisabled({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test('should disable the confirm button', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd .button-danger')).toBeDisabled({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

    });

  });

  // ────────────────────────────────────────────────────────────
  // Icon system
  // ────────────────────────────────────────────────────────────

  test.describe('icon system', () => {

    test.describe('when configured with the warning icon', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(() => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog({
            message: 'Warning action',
            icon: 'warning',
            confirmVariant: 'danger',
          });
        });

        await expect(page.locator('#e2e-test-cd[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test('should render the warning emoji', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd .dialog-icon')).toContainText('⚠️');
      });

    });

    test.describe('when configured with the error icon', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(() => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog({
            message: 'Critical action',
            icon: 'error',
            confirmVariant: 'primary',
          });
        });

        await expect(page.locator('#e2e-test-cd[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test('should render the error emoji', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd .dialog-icon')).toContainText('❌');
      });

    });

    test.describe('when configured with the network icon', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(() => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog({
            message: 'Network action',
            icon: 'network',
            confirmVariant: 'warning',
          });
        });

        await expect(page.locator('#e2e-test-cd[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test('should render the network emoji', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd .dialog-icon')).toContainText('🌐');
      });

    });

    test.describe('when irreversible is true', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(() => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog({
            message: 'Delete page',
            irreversible: true,
            confirmVariant: 'danger',
          });
        });

        await expect(page.locator('#e2e-test-cd[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test('should display the irreversible warning message', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd .irreversible')).toContainText('This action cannot be undone.');
      });

    });

    test.describe('when irreversible is false', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(() => {
          (document.getElementById('e2e-test-cd') as HTMLElement & { openDialog: (c: unknown) => void }).openDialog({
            message: 'Reversible action',
            irreversible: false,
            confirmVariant: 'primary',
          });
        });

        await expect(page.locator('#e2e-test-cd[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test('should not display the irreversible warning message', async ({ page }) => {
        await expect(page.locator('#e2e-test-cd .irreversible')).not.toBeAttached();
      });

    });

  });

});
