import { test, expect } from '@playwright/test';
import { COMPONENT_LOAD_TIMEOUT_MS } from './constants.js';

// Playwright automatically pierces open shadow roots when evaluating CSS selectors,
// so selectors like `frontmatter-editor-dialog .button-secondary` work without explicit
// shadow DOM traversal. Focus assertions use page.evaluate() since Playwright's
// toBeFocused() checks document.activeElement, which points to the shadow host rather
// than the focused element within an open shadow root.

const DIALOG_APPEAR_TIMEOUT_MS = 5000;
const FRONTMATTER_LOAD_TIMEOUT_MS = 10000;

test.describe('frontmatter-editor-dialog', () => {
  test.setTimeout(60000);

  test.beforeEach(async ({ page }) => {
    await page.goto('/home/edit');
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
  });

  // ────────────────────────────────────────────────────────────
  // ARIA attributes
  // ────────────────────────────────────────────────────────────

  test.describe('ARIA attributes', () => {

    test.describe('when the dialog is opened', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(() => {
          const dialog = document.querySelector('frontmatter-editor-dialog') as any;
          dialog?.openDialog('home');
        });

        await expect(page.locator('frontmatter-editor-dialog dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test('should have aria-labelledby set on the native dialog element', async ({ page }) => {
        const ariaLabelledBy = await page.evaluate(() =>
          document.querySelector('frontmatter-editor-dialog')?.shadowRoot?.querySelector('dialog')?.getAttribute('aria-labelledby')
        );
        expect(ariaLabelledBy).toBe('frontmatter-dialog-title');
      });

      test('should have the labelled element contain "Edit Frontmatter"', async ({ page }) => {
        const labelText = await page.evaluate(() => {
          const host = document.querySelector('frontmatter-editor-dialog');
          const labelId = host?.shadowRoot?.querySelector('dialog')?.getAttribute('aria-labelledby');
          if (!labelId) return null;
          return host?.shadowRoot?.getElementById(labelId)?.textContent?.trim();
        });
        expect(labelText).toContain('Edit Frontmatter');
      });

      test('should use a native <dialog> element (implicit role="dialog")', async ({ page }) => {
        const tagName = await page.evaluate(() =>
          document.querySelector('frontmatter-editor-dialog')?.shadowRoot?.querySelector('dialog')?.tagName.toLowerCase()
        );
        expect(tagName).toBe('dialog');
      });

    });

  });

  // ────────────────────────────────────────────────────────────
  // Escape key
  // ────────────────────────────────────────────────────────────

  test.describe('Escape key', () => {

    test.describe('when Escape is pressed while the dialog is open', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(() => {
          const dialog = document.querySelector('frontmatter-editor-dialog') as any;
          dialog?.openDialog('home');
        });

        await expect(page.locator('frontmatter-editor-dialog dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
        await page.keyboard.press('Escape');
      });

      test('should close the dialog', async ({ page }) => {
        await expect(page.locator('frontmatter-editor-dialog[open]')).not.toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

    });

  });

  // ────────────────────────────────────────────────────────────
  // Focus management
  // ────────────────────────────────────────────────────────────

  test.describe('focus management', () => {

    test.describe('when the dialog is closed via the Cancel button', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(() => {
          const btn = document.createElement('button');
          btn.id = 'e2e-prev-focus-fed';
          btn.setAttribute('style', 'position:absolute;left:-9999px');
          document.body.appendChild(btn);
          btn.focus();
        });

        await page.evaluate(() => {
          const dialog = document.querySelector('frontmatter-editor-dialog') as any;
          dialog?.openDialog('home');
        });

        await expect(page.locator('frontmatter-editor-dialog dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
        await page.locator('frontmatter-editor-dialog .button-secondary').click();
        await expect(page.locator('frontmatter-editor-dialog[open]')).not.toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test.afterEach(async ({ page }) => {
        await page.evaluate(() => document.getElementById('e2e-prev-focus-fed')?.remove());
      });

      test('should return focus to the previously focused element', async ({ page }) => {
        await expect.poll(
          () => page.evaluate(() => document.activeElement === document.getElementById('e2e-prev-focus-fed')),
          { timeout: DIALOG_APPEAR_TIMEOUT_MS },
        ).toBe(true);
      });

    });

    test.describe('when the dialog is closed via the Escape key', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(() => {
          const btn = document.createElement('button');
          btn.id = 'e2e-prev-focus-fed-esc';
          btn.setAttribute('style', 'position:absolute;left:-9999px');
          document.body.appendChild(btn);
          btn.focus();
        });

        await page.evaluate(() => {
          const dialog = document.querySelector('frontmatter-editor-dialog') as any;
          dialog?.openDialog('home');
        });

        await expect(page.locator('frontmatter-editor-dialog dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
        await page.keyboard.press('Escape');
        await expect(page.locator('frontmatter-editor-dialog[open]')).not.toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

      test.afterEach(async ({ page }) => {
        await page.evaluate(() => document.getElementById('e2e-prev-focus-fed-esc')?.remove());
      });

      test('should return focus to the previously focused element', async ({ page }) => {
        await expect.poll(
          () => page.evaluate(() => document.activeElement === document.getElementById('e2e-prev-focus-fed-esc')),
          { timeout: DIALOG_APPEAR_TIMEOUT_MS },
        ).toBe(true);
      });

    });

  });

  // ────────────────────────────────────────────────────────────
  // Focus trap
  // ────────────────────────────────────────────────────────────

  test.describe('focus trap', () => {

    test.describe('when the dialog is open and frontmatter has loaded', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(() => {
          const dialog = document.querySelector('frontmatter-editor-dialog') as any;
          dialog?.openDialog('home');
        });

        await expect(page.locator('frontmatter-editor-dialog dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

        // Wait for the Save button to be enabled (frontmatter loaded, not loading/saving)
        await expect(page.locator('frontmatter-editor-dialog .button-primary')).toBeEnabled({ timeout: FRONTMATTER_LOAD_TIMEOUT_MS });
      });

      test.describe('when Tab is pressed on the last focusable element', () => {
        test.beforeEach(async ({ page }) => {
          // Focus the Save button (last focusable element in the dialog)
          await page.locator('frontmatter-editor-dialog .button-primary').focus();
          await page.keyboard.press('Tab');
        });

        test('should keep the dialog open', async ({ page }) => {
          await expect(page.locator('frontmatter-editor-dialog dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
        });

        test('should wrap focus to the first focusable element', async ({ page }) => {
          await expect.poll(
            () => page.evaluate(() => {
              const host = document.querySelector('frontmatter-editor-dialog');
              return host?.shadowRoot?.activeElement !== null;
            }),
            { timeout: DIALOG_APPEAR_TIMEOUT_MS },
          ).toBe(true);
        });

      });

      test.describe('when Shift+Tab is pressed on the first focusable element', () => {
        test.beforeEach(async ({ page }) => {
          // Focus the first focusable element (close/icon button in the header)
          await page.locator('frontmatter-editor-dialog .icon-button').focus();
          await page.keyboard.press('Shift+Tab');
        });

        test('should keep the dialog open', async ({ page }) => {
          await expect(page.locator('frontmatter-editor-dialog dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
        });

        test('should wrap focus to the last focusable element', async ({ page }) => {
          const activeElementText = await page.evaluate(() => {
            const host = document.querySelector('frontmatter-editor-dialog');
            return (host?.shadowRoot?.activeElement as HTMLElement)?.textContent?.trim();
          });
          expect(activeElementText).toBe('Save');
        });

      });

    });

  });

  // ────────────────────────────────────────────────────────────
  // Keyboard activation
  // ────────────────────────────────────────────────────────────

  test.describe('keyboard activation', () => {

    test.describe('when the Cancel button is focused and Enter is pressed', () => {
      test.beforeEach(async ({ page }) => {
        await page.evaluate(() => {
          const dialog = document.querySelector('frontmatter-editor-dialog') as any;
          dialog?.openDialog('home');
        });

        await expect(page.locator('frontmatter-editor-dialog dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
        await page.locator('frontmatter-editor-dialog .button-secondary').focus();
        await page.keyboard.press('Enter');
      });

      test('should close the dialog', async ({ page }) => {
        await expect(page.locator('frontmatter-editor-dialog[open]')).not.toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
      });

    });

  });

});
