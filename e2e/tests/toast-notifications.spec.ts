import { test, expect, type Page } from '@playwright/test';

// Timeouts
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const TOAST_APPEAR_TIMEOUT_MS = 5000;
const TOAST_DISMISS_TIMEOUT_MS = 8000;

// Storage keys must match those in toast-message.ts
const STORAGE_KEYS = {
  MESSAGE: 'toast-message',
  TYPE: 'toast-type',
  TIMEOUT: 'toast-timeout',
} as const;

/**
 * Set sessionStorage to simulate the state left by showToastAfter() before a page redirect.
 * This exercises the showStoredToast() path called from index.ts on page load.
 */
async function seedSessionStorageToast(
  page: Page,
  message: string,
  type: string,
  timeoutSeconds: number,
): Promise<void> {
  await page.evaluate(
    ([keys, msg, t, secs]) => {
      sessionStorage.setItem(keys[0], msg);
      sessionStorage.setItem(keys[1], t);
      sessionStorage.setItem(keys[2], secs);
    },
    [
      [STORAGE_KEYS.MESSAGE, STORAGE_KEYS.TYPE, STORAGE_KEYS.TIMEOUT],
      message,
      type,
      timeoutSeconds.toString(),
    ] as const,
  );
}

/**
 * Inject a toast-message element directly into the page via page.evaluate().
 * Mirrors the showToast() utility in toast-message.ts.
 */
async function injectToast(
  page: Page,
  message: string,
  type: string,
  timeoutSeconds: number,
  autoClose: boolean,
): Promise<void> {
  await page.evaluate(
    ([msg, t, secs, auto]) => {
      const toast = document.createElement('toast-message') as HTMLElement & {
        message: string;
        type: string;
        timeoutSeconds: number;
        autoClose: boolean;
        visible: boolean;
        show(): void;
      };
      toast.message = msg;
      toast.type = t;
      toast.timeoutSeconds = Number(secs);
      toast.autoClose = Boolean(auto);
      toast.visible = false;
      document.body.appendChild(toast);
      requestAnimationFrame(() => {
        toast.show();
      });
    },
    [message, type, timeoutSeconds, autoClose] as const,
  );
}

test.describe('Toast notification system', () => {
  test.setTimeout(60000);

  test.describe('when a success toast is queued in sessionStorage (post-operation notification)', () => {
    // This path is exercised after frontmatter saves and page deletions:
    //   showToastAfter() stores the message in sessionStorage,
    //   then index.ts calls showStoredToast() on the next page load.

    test.beforeEach(async ({ page }) => {
      // Navigate to any page so sessionStorage is available on the origin
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      // Simulate the state left by showToastAfter() before its callback redirects
      await seedSessionStorageToast(page, 'Frontmatter saved successfully!', 'success', 5);

      // Reload to trigger showStoredToast() from index.ts
      await page.reload();
      await expect(page.locator('#rendered')).toBeAttached({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });
    });

    test('toast element is present in the DOM', async ({ page }) => {
      await expect(page.locator('toast-message')).toBeAttached({
        timeout: TOAST_APPEAR_TIMEOUT_MS,
      });
    });

    test('toast becomes visible', async ({ page }) => {
      await expect(page.locator('toast-message[visible]')).toBeAttached({
        timeout: TOAST_APPEAR_TIMEOUT_MS,
      });
    });

    test('toast displays the correct message', async ({ page }) => {
      await expect(page.locator('toast-message .message')).toContainText(
        'Frontmatter saved successfully!',
        { timeout: TOAST_APPEAR_TIMEOUT_MS },
      );
    });

    test('sessionStorage is cleared after toast is shown', async ({ page }) => {
      await expect(page.locator('toast-message[visible]')).toBeAttached({
        timeout: TOAST_APPEAR_TIMEOUT_MS,
      });

      const storedMessage = await page.evaluate(() =>
        sessionStorage.getItem('toast-message'),
      );
      expect(storedMessage).toBeNull();
    });
  });

  test.describe('when an error toast is shown directly', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      await injectToast(page, 'Something went wrong', 'error', 10, false);
    });

    test('toast element is present in the DOM', async ({ page }) => {
      await expect(page.locator('toast-message')).toBeAttached({
        timeout: TOAST_APPEAR_TIMEOUT_MS,
      });
    });

    test('toast becomes visible', async ({ page }) => {
      await expect(page.locator('toast-message[visible]')).toBeAttached({
        timeout: TOAST_APPEAR_TIMEOUT_MS,
      });
    });

    test('toast displays the error message', async ({ page }) => {
      await expect(page.locator('toast-message .message')).toContainText(
        'Something went wrong',
        { timeout: TOAST_APPEAR_TIMEOUT_MS },
      );
    });
  });

  test.describe('when a toast with auto-close is shown', () => {
    // Use a 1-second timeout so the test does not wait too long
    const AUTO_CLOSE_TIMEOUT_SECONDS = 1;

    test.beforeEach(async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      await injectToast(page, 'Auto-close toast', 'info', AUTO_CLOSE_TIMEOUT_SECONDS, true);

      // Confirm the toast is visible before asserting it disappears
      await expect(page.locator('toast-message[visible]')).toBeAttached({
        timeout: TOAST_APPEAR_TIMEOUT_MS,
      });
    });

    test('toast is removed from the DOM after the timeout elapses', async ({ page }) => {
      // The component hides itself after timeoutSeconds, then removes itself from the DOM
      // after a 300 ms CSS transition. Allow a generous window here.
      await expect(page.locator('toast-message')).not.toBeAttached({
        timeout: TOAST_DISMISS_TIMEOUT_MS,
      });
    });
  });

  test.describe('when the close button is clicked', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      await injectToast(page, 'Click to close me', 'warning', 30, false);
      await expect(page.locator('toast-message[visible]')).toBeAttached({
        timeout: TOAST_APPEAR_TIMEOUT_MS,
      });
    });

    test('toast is removed from the DOM', async ({ page }) => {
      await page.locator('toast-message .close-button').click();
      await expect(page.locator('toast-message')).not.toBeAttached({
        timeout: TOAST_DISMISS_TIMEOUT_MS,
      });
    });
  });

  test.describe('when the toast body is clicked', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      await injectToast(page, 'Click body to dismiss', 'success', 30, false);
      await expect(page.locator('toast-message[visible]')).toBeAttached({
        timeout: TOAST_APPEAR_TIMEOUT_MS,
      });
    });

    test('toast is removed from the DOM', async ({ page }) => {
      // Click on the message text (not the close button) to trigger the body click handler
      await page.locator('toast-message .message').click();
      await expect(page.locator('toast-message')).not.toBeAttached({
        timeout: TOAST_DISMISS_TIMEOUT_MS,
      });
    });
  });

  test.describe('when multiple toasts are shown simultaneously', () => {
    const TOAST_COUNT = 3;

    test.beforeEach(async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      // Inject several toasts in quick succession
      await injectToast(page, 'Toast one', 'success', 30, false);
      await injectToast(page, 'Toast two', 'warning', 30, false);
      await injectToast(page, 'Toast three', 'error', 30, false);

      // Wait until all three are visible
      await expect(page.locator('toast-message[visible]')).toHaveCount(TOAST_COUNT, {
        timeout: TOAST_APPEAR_TIMEOUT_MS,
      });
    });

    test('all toasts are present in the DOM', async ({ page }) => {
      await expect(page.locator('toast-message')).toHaveCount(TOAST_COUNT);
    });

    test('each toast displays its own message', async ({ page }) => {
      await expect(page.locator('toast-message .message').nth(0)).toContainText('Toast one');
      await expect(page.locator('toast-message .message').nth(1)).toContainText('Toast two');
      await expect(page.locator('toast-message .message').nth(2)).toContainText('Toast three');
    });
  });
});
