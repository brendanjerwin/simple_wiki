import { test, expect, type Page } from '@playwright/test';

// Z-index layer values from ADR-0008 (shared-styles.ts)
const Z_LAYERS = {
  ambient: 100,
  drawer: 200,
  popover: 300,
  modal: 400,
  notification: 500,
  blocker: 600,
} as const;

const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const MENU_APPEAR_TIMEOUT_MS = 5000;
const ANIMATION_SETTLE_MS = 400;

/**
 * Reads z-index CSS custom property token values from the system-info element.
 * system-info includes zIndexCSS which defines all 6 tokens on :host, making
 * them accessible via getComputedStyle from outside the shadow DOM.
 */
async function readTokenValues(page: Page) {
  return page.evaluate(() => {
    const el = document.querySelector('system-info');
    if (!el) throw new Error('system-info not found — is this a view page?');
    const style = getComputedStyle(el);
    return {
      ambient: parseInt(style.getPropertyValue('--z-ambient').trim(), 10),
      drawer: parseInt(style.getPropertyValue('--z-drawer').trim(), 10),
      popover: parseInt(style.getPropertyValue('--z-popover').trim(), 10),
      modal: parseInt(style.getPropertyValue('--z-modal').trim(), 10),
      notification: parseInt(style.getPropertyValue('--z-notification').trim(), 10),
      blocker: parseInt(style.getPropertyValue('--z-blocker').trim(), 10),
    };
  });
}

/** Opens the page-import-dialog via the tools menu trigger. */
async function openPageImportDialog(page: Page): Promise<void> {
  await expect(page.locator('#page-import-trigger')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });
  await page.locator('.tools-menu').hover();
  await page.locator('#page-import-trigger').click();
  await expect(page.locator('page-import-dialog')).toHaveAttribute('open', { timeout: MENU_APPEAR_TIMEOUT_MS });
}

/** Injects a toast-message into the page and calls show() on the next animation frame. */
async function injectToast(page: Page, message: string): Promise<void> {
  await page.evaluate((msg: string) => {
    interface ToastElement extends HTMLElement {
      message: string;
      type: string;
      timeoutSeconds: number;
      autoClose: boolean;
      visible: boolean;
      show(): void;
    }
    const toast = document.createElement('toast-message') as ToastElement;
    toast.message = msg;
    toast.type = 'info';
    toast.timeoutSeconds = 30;
    toast.autoClose = false;
    toast.visible = false;
    document.body.appendChild(toast);
    requestAnimationFrame(() => { toast.show(); });
  }, message);
  await expect(page.locator('toast-message')).toBeAttached();
}

test.describe('Z-Index Token System (ADR-0008)', () => {
  test.setTimeout(60000);

  test.describe('CSS custom property token values', () => {
    test('all z-index tokens should be defined with correct numeric values', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('system-info')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      const tokens = await readTokenValues(page);

      expect(tokens.ambient).toBe(Z_LAYERS.ambient);
      expect(tokens.drawer).toBe(Z_LAYERS.drawer);
      expect(tokens.popover).toBe(Z_LAYERS.popover);
      expect(tokens.modal).toBe(Z_LAYERS.modal);
      expect(tokens.notification).toBe(Z_LAYERS.notification);
      expect(tokens.blocker).toBe(Z_LAYERS.blocker);
    });

    test('layer tokens should be strictly ordered from lowest to highest', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('system-info')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      const tokens = await readTokenValues(page);

      expect(tokens.ambient).toBeLessThan(tokens.drawer);
      expect(tokens.drawer).toBeLessThan(tokens.popover);
      expect(tokens.popover).toBeLessThan(tokens.modal);
      expect(tokens.modal).toBeLessThan(tokens.notification);
      expect(tokens.notification).toBeLessThan(tokens.blocker);
    });

    test('all six layer tokens should have distinct z-index values', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('system-info')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      const tokens = await readTokenValues(page);
      const values = Object.values(tokens);
      const uniqueValues = new Set(values);

      expect(uniqueValues.size).toBe(values.length);
    });
  });

  test.describe('drawer layer (z-index: 200)', () => {
    test('system-info should render at the drawer tier', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('system-info')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      const zIndex = await page.evaluate(() => {
        const el = document.querySelector('system-info');
        if (!el) throw new Error('system-info not found');
        return parseInt(getComputedStyle(el).zIndex, 10);
      });

      expect(zIndex).toBe(Z_LAYERS.drawer);
    });
  });

  test.describe('popover layer (z-index: 300)', () => {
    test('search results popover should render at the popover tier', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-search')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      // wiki-search-results lives inside wiki-search's shadow DOM.
      // It uses --z-popover on its internal .popover element.
      const zIndex = await page.evaluate(() => {
        const searchEl = document.querySelector('wiki-search');
        if (!searchEl?.shadowRoot) return null;
        const resultsEl = searchEl.shadowRoot.querySelector('wiki-search-results');
        if (!resultsEl?.shadowRoot) return null;
        const popover = resultsEl.shadowRoot.querySelector('.popover');
        if (!popover) return null;
        return parseInt(getComputedStyle(popover).zIndex, 10);
      });

      expect(zIndex).toBe(Z_LAYERS.popover);
    });

    test('popover layer should appear below the modal layer', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('system-info')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });

      const tokens = await readTokenValues(page);

      expect(tokens.popover).toBeLessThan(tokens.modal);
    });
  });

  test.describe('modal layer (z-index: 400)', () => {
    test('page-import-dialog should render at the modal tier when open', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await openPageImportDialog(page);

      const zIndex = await page.evaluate(() => {
        const el = document.querySelector('page-import-dialog');
        if (!el) throw new Error('page-import-dialog not found');
        return parseInt(getComputedStyle(el).zIndex, 10);
      });

      expect(zIndex).toBe(Z_LAYERS.modal);
    });

    test('an open dialog should be the topmost element at the viewport center', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await openPageImportDialog(page);
      // Allow dialog fade-in animation to complete before checking layout
      await page.waitForTimeout(ANIMATION_SETTLE_MS);

      // The dialog backdrop covers the full viewport; at center, the dialog should be topmost
      const topmostTag = await page.evaluate(() => {
        function getTopmost(x: number, y: number): string {
          const el = document.elementFromPoint(x, y);
          if (!el) return 'none';
          const root = el.getRootNode();
          if (root instanceof ShadowRoot) return root.host.tagName.toLowerCase();
          let current: Element | null = el;
          while (current && current !== document.body) {
            const tag = current.tagName.toLowerCase();
            if (tag.includes('-')) return tag;
            current = current.parentElement;
          }
          return el.tagName.toLowerCase();
        }
        return getTopmost(window.innerWidth / 2, window.innerHeight / 2);
      });

      expect(topmostTag).toBe('page-import-dialog');
    });
  });

  test.describe('notification layer (z-index: 500)', () => {
    test('toast-message should render at the notification tier', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await injectToast(page, 'Z-index notification tier test');

      const zIndex = await page.evaluate(() => {
        const el = document.querySelector('toast-message');
        if (!el) throw new Error('toast-message not found');
        return parseInt(getComputedStyle(el).zIndex, 10);
      });

      expect(zIndex).toBe(Z_LAYERS.notification);
    });

    test('toast should have a higher z-index than an open modal dialog', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await openPageImportDialog(page);
      await injectToast(page, 'Notification above dialog');

      const { toastZ, dialogZ } = await page.evaluate(() => {
        const toast = document.querySelector('toast-message');
        const dialog = document.querySelector('page-import-dialog');
        if (!toast || !dialog) throw new Error('toast-message or page-import-dialog not found');
        return {
          toastZ: parseInt(getComputedStyle(toast).zIndex, 10),
          dialogZ: parseInt(getComputedStyle(dialog).zIndex, 10),
        };
      });

      expect(toastZ).toBeGreaterThan(dialogZ);
    });

    test('toast should be the topmost element at its screen position even when a modal is open', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Open dialog first — its backdrop covers the full viewport at z-index 400
      await openPageImportDialog(page);

      // Inject toast — it slides in at z-index 500, above the dialog
      await injectToast(page, 'Toast visible above dialog');

      // Wait for toast slide-in animation (300ms) and dialog animation to fully settle
      await page.waitForTimeout(ANIMATION_SETTLE_MS);

      // At the toast's rendered position, toast-message should be the topmost element
      const topmostAtToastPosition = await page.evaluate(() => {
        function getTopmost(x: number, y: number): string {
          const el = document.elementFromPoint(x, y);
          if (!el) return 'none';
          const root = el.getRootNode();
          if (root instanceof ShadowRoot) return root.host.tagName.toLowerCase();
          let current: Element | null = el;
          while (current && current !== document.body) {
            const tag = current.tagName.toLowerCase();
            if (tag.includes('-')) return tag;
            current = current.parentElement;
          }
          return el.tagName.toLowerCase();
        }

        const toastEl = document.querySelector('toast-message') as HTMLElement;
        if (!toastEl) return 'no-toast';
        const rect = toastEl.getBoundingClientRect();
        if (rect.width === 0 || rect.height === 0) return 'toast-not-rendered';
        return getTopmost(rect.left + rect.width / 2, rect.top + rect.height / 2);
      });

      expect(topmostAtToastPosition).toBe('toast-message');
    });
  });

  test.describe('no z-index conflicts between overlapping components', () => {
    test('drawer and modal should have distinct z-index values when both are present', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await openPageImportDialog(page);

      const { drawerZ, modalZ } = await page.evaluate(() => {
        const drawer = document.querySelector('system-info');
        const modal = document.querySelector('page-import-dialog');
        if (!drawer || !modal) throw new Error('system-info or page-import-dialog not found');
        return {
          drawerZ: parseInt(getComputedStyle(drawer).zIndex, 10),
          modalZ: parseInt(getComputedStyle(modal).zIndex, 10),
        };
      });

      expect(drawerZ).not.toBe(modalZ);
      expect(modalZ).toBeGreaterThan(drawerZ);
    });

    test('notification and modal should have distinct z-index values when both are present', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await openPageImportDialog(page);
      await injectToast(page, 'Conflict check notification');

      const { notificationZ, modalZ } = await page.evaluate(() => {
        const notification = document.querySelector('toast-message');
        const modal = document.querySelector('page-import-dialog');
        if (!notification || !modal) throw new Error('toast-message or page-import-dialog not found');
        return {
          notificationZ: parseInt(getComputedStyle(notification).zIndex, 10),
          modalZ: parseInt(getComputedStyle(modal).zIndex, 10),
        };
      });

      expect(notificationZ).not.toBe(modalZ);
      expect(notificationZ).toBeGreaterThan(modalZ);
    });
  });
});
