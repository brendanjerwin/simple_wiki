import { test, expect } from '@playwright/test';

// Tests rely on Playwright's shadow DOM auto-piercing for locators.
// Playwright automatically pierces open shadow roots when evaluating CSS selectors,
// so selectors like `#e2e-test-cib .button-trigger` work without explicit shadow DOM traversal.
// Focus assertions use locator.evaluate() since Playwright's toBeFocused() checks
// document.activeElement, which points to the shadow host rather than the focused element
// within an open shadow root.

// Extended window type used in page.evaluate() callbacks to store event result promises
type E2EWindow = typeof window & {
  __e2eCancelledPromise?: Promise<boolean>;
  __e2eConfirmedPromise?: Promise<boolean>;
};

// Timeouts
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const POPUP_APPEAR_TIMEOUT_MS = 3000;
const EVENT_TIMEOUT_MS = 3000;

/**
 * E2E tests for confirmation-interlock-button keyboard navigation and ARIA attributes.
 *
 * These tests inject the component directly into the wiki edit page (where all custom
 * elements are registered) and verify its behavior in a real browser environment.
 */
test.describe('confirmation-interlock-button', () => {
  test.setTimeout(60000);

  test.beforeEach(async ({ page }) => {
    // Navigate to the edit page so all custom elements are registered
    await page.goto('/home/edit');
    await expect(page.locator('wiki-editor textarea')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

    // Inject the component into the DOM for isolated testing
    await page.evaluate(() => {
      const el = document.createElement('confirmation-interlock-button') as HTMLElement & {
        label: string;
        confirmLabel: string;
        yesLabel: string;
        noLabel: string;
        disarmTimeoutMs: number;
      };
      el.id = 'e2e-test-cib';
      // Set properties directly (not via setAttribute) so Lit's camelCase property names
      // are used instead of the lowercased attribute names that the browser would produce.
      el.label = 'Delete';
      el.confirmLabel = 'Are you sure?';
      el.yesLabel = 'Yes';
      el.noLabel = 'No';
      // Disable auto-disarm so popup stays open for testing
      el.disarmTimeoutMs = 0;
      el.style.position = 'fixed';
      el.style.top = '100px';
      el.style.left = '200px';
      el.style.zIndex = '99999';
      document.body.appendChild(el);
    });

    // Wait for component to render
    await expect(page.locator('#e2e-test-cib')).toBeAttached({ timeout: POPUP_APPEAR_TIMEOUT_MS });
  });

  test.afterEach(async ({ page }) => {
    await page.evaluate(() => {
      document.getElementById('e2e-test-cib')?.remove();
    });
  });

  test.describe('trigger button ARIA attributes', () => {
    test('should have aria-haspopup="dialog" on the trigger button', async ({ page }) => {
      const triggerButton = page.locator('#e2e-test-cib .button-trigger');
      await expect(triggerButton).toHaveAttribute('aria-haspopup', 'dialog');
    });

    test('should have aria-expanded="false" when not armed', async ({ page }) => {
      const triggerButton = page.locator('#e2e-test-cib .button-trigger');
      await expect(triggerButton).toHaveAttribute('aria-expanded', 'false');
    });

    test('should have aria-expanded="true" when armed', async ({ page }) => {
      await page.locator('#e2e-test-cib .button-trigger').click();

      await expect(page.locator('#e2e-test-cib .button-trigger')).toHaveAttribute(
        'aria-expanded',
        'true',
        { timeout: POPUP_APPEAR_TIMEOUT_MS },
      );
    });

    test('should have aria-expanded="false" after disarming via No button', async ({ page }) => {
      await page.locator('#e2e-test-cib .button-trigger').click();
      await expect(page.locator('#e2e-test-cib .button-trigger')).toHaveAttribute(
        'aria-expanded',
        'true',
        { timeout: POPUP_APPEAR_TIMEOUT_MS },
      );

      await page.locator('#e2e-test-cib .button-no').click();
      await expect(page.locator('#e2e-test-cib .button-trigger')).toHaveAttribute(
        'aria-expanded',
        'false',
        { timeout: POPUP_APPEAR_TIMEOUT_MS },
      );
    });

    test('should have aria-expanded="false" after disarming via Escape', async ({ page }) => {
      await page.locator('#e2e-test-cib .button-trigger').click();
      await expect(page.locator('#e2e-test-cib .button-trigger')).toHaveAttribute(
        'aria-expanded',
        'true',
        { timeout: POPUP_APPEAR_TIMEOUT_MS },
      );

      await page.keyboard.press('Escape');
      await expect(page.locator('#e2e-test-cib .button-trigger')).toHaveAttribute(
        'aria-expanded',
        'false',
        { timeout: POPUP_APPEAR_TIMEOUT_MS },
      );
    });
  });

  test.describe('confirm popup ARIA attributes', () => {
    test.beforeEach(async ({ page }) => {
      await page.locator('#e2e-test-cib .button-trigger').click();
      await expect(page.locator('#e2e-test-cib .confirm-popup')).toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });
    });

    test('should have role="alertdialog" on the popup', async ({ page }) => {
      await expect(page.locator('#e2e-test-cib .confirm-popup')).toHaveAttribute(
        'role',
        'alertdialog',
      );
    });

    test('should have aria-modal="true" on the popup', async ({ page }) => {
      await expect(page.locator('#e2e-test-cib .confirm-popup')).toHaveAttribute(
        'aria-modal',
        'true',
      );
    });

    test('should have aria-labelledby pointing to confirm-label element', async ({ page }) => {
      await expect(page.locator('#e2e-test-cib .confirm-popup')).toHaveAttribute(
        'aria-labelledby',
        'confirm-label',
      );
    });

    test('confirm-label element should exist and contain the confirmation text', async ({
      page,
    }) => {
      await expect(page.locator('#e2e-test-cib #confirm-label')).toContainText('Are you sure?');
    });
  });

  test.describe('screen reader labels', () => {
    test('trigger button should display the configured label text', async ({ page }) => {
      await expect(page.locator('#e2e-test-cib .button-trigger')).toContainText('Delete');
    });

    test('Yes button should display the configured yesLabel text', async ({ page }) => {
      await page.locator('#e2e-test-cib .button-trigger').click();
      await expect(page.locator('#e2e-test-cib .button-yes')).toContainText('Yes', {
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });
    });

    test('No button should display the configured noLabel text', async ({ page }) => {
      await page.locator('#e2e-test-cib .button-trigger').click();
      await expect(page.locator('#e2e-test-cib .button-no')).toContainText('No', {
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });
    });

    test('confirm label should describe the destructive action', async ({ page }) => {
      await page.locator('#e2e-test-cib .button-trigger').click();
      await expect(page.locator('#e2e-test-cib #confirm-label')).toContainText('Are you sure?', {
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });
    });
  });

  test.describe('keyboard navigation flow', () => {
    test.beforeEach(async ({ page }) => {
      await page.locator('#e2e-test-cib .button-trigger').click();
      await expect(page.locator('#e2e-test-cib .confirm-popup')).toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });
    });

    test('should close the popup when Escape is pressed', async ({ page }) => {
      await page.keyboard.press('Escape');
      await expect(page.locator('#e2e-test-cib .confirm-popup')).not.toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });
    });

    test('should dispatch cancelled event when Escape is pressed', async ({ page }) => {
      // Register listener before triggering to ensure deterministic attachment
      await page.evaluate(({ timeoutMs }: { timeoutMs: number }) => {
        const target = document.getElementById('e2e-test-cib');
        (window as E2EWindow).__e2eCancelledPromise = new Promise<boolean>((resolve) => {
          const timeoutId = setTimeout(() => resolve(false), timeoutMs);
          target?.addEventListener('cancelled', () => {
            clearTimeout(timeoutId);
            resolve(true);
          }, { once: true });
        });
      }, { timeoutMs: EVENT_TIMEOUT_MS });

      await page.keyboard.press('Escape');

      const wasCancelled = await page.evaluate(() =>
        (window as E2EWindow).__e2eCancelledPromise
      );
      expect(wasCancelled).toBe(true);
    });

    test('should close the popup when Yes button is clicked', async ({ page }) => {
      await page.locator('#e2e-test-cib .button-yes').click();
      await expect(page.locator('#e2e-test-cib .confirm-popup')).not.toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });
    });

    test('should dispatch confirmed event when Yes button is clicked', async ({ page }) => {
      // Register listener before triggering to ensure deterministic attachment
      await page.evaluate(({ timeoutMs }: { timeoutMs: number }) => {
        const target = document.getElementById('e2e-test-cib');
        (window as E2EWindow).__e2eConfirmedPromise = new Promise<boolean>((resolve) => {
          const timeoutId = setTimeout(() => resolve(false), timeoutMs);
          target?.addEventListener('confirmed', () => {
            clearTimeout(timeoutId);
            resolve(true);
          }, { once: true });
        });
      }, { timeoutMs: EVENT_TIMEOUT_MS });

      await page.locator('#e2e-test-cib .button-yes').click();

      const wasConfirmed = await page.evaluate(() =>
        (window as E2EWindow).__e2eConfirmedPromise
      );
      expect(wasConfirmed).toBe(true);
    });

    test('should close the popup when No button is clicked', async ({ page }) => {
      await page.locator('#e2e-test-cib .button-no').click();
      await expect(page.locator('#e2e-test-cib .confirm-popup')).not.toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });
    });

    test('should dispatch cancelled event when No button is clicked', async ({ page }) => {
      // Register listener before triggering to ensure deterministic attachment
      await page.evaluate(({ timeoutMs }: { timeoutMs: number }) => {
        const target = document.getElementById('e2e-test-cib');
        (window as E2EWindow).__e2eCancelledPromise = new Promise<boolean>((resolve) => {
          const timeoutId = setTimeout(() => resolve(false), timeoutMs);
          target?.addEventListener('cancelled', () => {
            clearTimeout(timeoutId);
            resolve(true);
          }, { once: true });
        });
      }, { timeoutMs: EVENT_TIMEOUT_MS });

      await page.locator('#e2e-test-cib .button-no').click();

      const wasCancelled = await page.evaluate(() =>
        (window as E2EWindow).__e2eCancelledPromise
      );
      expect(wasCancelled).toBe(true);
    });

    test('should re-arm when trigger button is clicked after disarming', async ({ page }) => {
      // Disarm via No button
      await page.locator('#e2e-test-cib .button-no').click();
      await expect(page.locator('#e2e-test-cib .confirm-popup')).not.toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });

      // Re-arm by clicking trigger
      await page.locator('#e2e-test-cib .button-trigger').click();
      await expect(page.locator('#e2e-test-cib .confirm-popup')).toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });
    });

    test('should close the popup when the backdrop is clicked', async ({ page }) => {
      // Dispatch pointerdown on the backdrop — the component uses @pointerdown for dismissal.
      // Use bubbles+composed to match real pointer interaction behaviour.
      await page.locator('#e2e-test-cib .confirm-backdrop').dispatchEvent('pointerdown', {
        bubbles: true,
        composed: true,
      });
      await expect(page.locator('#e2e-test-cib .confirm-popup')).not.toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });
    });

    test('should dispatch confirmed event when Enter is pressed on a non-button popup element', async ({
      page,
    }) => {
      // Make the popup div focusable and focus it to exercise the non-button Enter path.
      // The popup keydown handler intercepts Enter only when e.target is not a button;
      // buttons handle Enter natively via their click event.
      const popup = page.locator('#e2e-test-cib .confirm-popup');
      await popup.evaluate((element) => {
        element.tabIndex = -1;
      });
      await popup.focus();

      // Register listener before pressing Enter to ensure deterministic attachment
      await page.evaluate(({ timeoutMs }: { timeoutMs: number }) => {
        const target = document.getElementById('e2e-test-cib');
        (window as E2EWindow).__e2eConfirmedPromise = new Promise<boolean>((resolve) => {
          const timeoutId = setTimeout(() => resolve(false), timeoutMs);
          target?.addEventListener('confirmed', () => {
            clearTimeout(timeoutId);
            resolve(true);
          }, { once: true });
        });
      }, { timeoutMs: EVENT_TIMEOUT_MS });

      await page.keyboard.press('Enter');

      const wasConfirmed = await page.evaluate(() =>
        (window as E2EWindow).__e2eConfirmedPromise
      );
      expect(wasConfirmed).toBe(true);
      await expect(page.locator('#e2e-test-cib .confirm-popup')).not.toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });
    });
  });

  test.describe('focus management', () => {
    test('should move focus to the Yes button when armed', async ({ page }) => {
      await page.locator('#e2e-test-cib .button-trigger').click();
      await expect(page.locator('#e2e-test-cib .confirm-popup')).toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });

      // After arming, the Yes button should receive focus.
      // Focus is applied inside updateComplete.then(), so poll until it lands.
      await expect.poll(
        () => page.locator('#e2e-test-cib .button-yes').evaluate(
          el => (el.getRootNode() as ShadowRoot).activeElement === el
        ),
        { timeout: POPUP_APPEAR_TIMEOUT_MS },
      ).toBe(true);
    });

    test('should return focus to the trigger button when disarmed via No button', async ({
      page,
    }) => {
      // Focus the trigger button before arming so it is captured as the return target
      await page.locator('#e2e-test-cib .button-trigger').focus();

      await page.locator('#e2e-test-cib .button-trigger').click();
      await expect(page.locator('#e2e-test-cib .confirm-popup')).toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });

      await page.locator('#e2e-test-cib .button-no').click();
      await expect(page.locator('#e2e-test-cib .confirm-popup')).not.toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });

      // Focus returns inside updateComplete.then(), so poll until it lands.
      await expect.poll(
        () => page.locator('#e2e-test-cib .button-trigger').evaluate(
          el => (el.getRootNode() as ShadowRoot).activeElement === el
        ),
        { timeout: POPUP_APPEAR_TIMEOUT_MS },
      ).toBe(true);
    });

    test('should return focus to the trigger button when disarmed via Escape', async ({
      page,
    }) => {
      // Focus the trigger button before arming
      await page.locator('#e2e-test-cib .button-trigger').focus();

      await page.locator('#e2e-test-cib .button-trigger').click();
      await expect(page.locator('#e2e-test-cib .confirm-popup')).toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });

      await page.keyboard.press('Escape');
      await expect(page.locator('#e2e-test-cib .confirm-popup')).not.toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });

      // Focus returns inside updateComplete.then(), so poll until it lands.
      await expect.poll(
        () => page.locator('#e2e-test-cib .button-trigger').evaluate(
          el => (el.getRootNode() as ShadowRoot).activeElement === el
        ),
        { timeout: POPUP_APPEAR_TIMEOUT_MS },
      ).toBe(true);
    });
  });

  test.describe('focus trap', () => {
    test.beforeEach(async ({ page }) => {
      await page.locator('#e2e-test-cib .button-trigger').click();
      await expect(page.locator('#e2e-test-cib .confirm-popup')).toBeAttached({
        timeout: POPUP_APPEAR_TIMEOUT_MS,
      });
    });

    test('should wrap Tab from the last focusable element back to the first (Yes button)', async ({
      page,
    }) => {
      // Focus the No button (last focusable element in the popup)
      await page.locator('#e2e-test-cib .button-no').focus();

      // Press Tab — the focus trap should wrap to the Yes button
      await page.keyboard.press('Tab');

      const yesFocused = await page.locator('#e2e-test-cib .button-yes').evaluate(
        el => (el.getRootNode() as ShadowRoot).activeElement === el
      );
      expect(yesFocused).toBe(true);
    });

    test('should wrap Shift+Tab from the first focusable element back to the last (No button)', async ({
      page,
    }) => {
      // The Yes button should already be focused after arming (from beforeEach).
      // Focus is applied inside updateComplete.then(), so poll until it lands.
      await expect.poll(
        () => page.locator('#e2e-test-cib .button-yes').evaluate(
          el => (el.getRootNode() as ShadowRoot).activeElement === el
        ),
        { timeout: POPUP_APPEAR_TIMEOUT_MS },
      ).toBe(true);

      // Press Shift+Tab — the focus trap should wrap to the No button
      await page.keyboard.press('Shift+Tab');

      const noFocused = await page.locator('#e2e-test-cib .button-no').evaluate(
        el => (el.getRootNode() as ShadowRoot).activeElement === el
      );
      expect(noFocused).toBe(true);
    });
  });
});
