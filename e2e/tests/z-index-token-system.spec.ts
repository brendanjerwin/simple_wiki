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

/**
 * Reads z-index CSS custom property token values from the system-info element.
 * system-info includes zIndexCSS which defines all 6 tokens on :host, making
 * them accessible via getComputedStyle from outside the shadow DOM.
 */
async function readTokenValues(page: Page): Promise<Record<keyof typeof Z_LAYERS, number>> {
  const names = Object.keys(Z_LAYERS) as Array<keyof typeof Z_LAYERS>;
  const result = await page.evaluate((tokenNames) => {
    const el = document.querySelector('system-info');
    if (!el) throw new Error('system-info not found — is this a view page?');
    const style = getComputedStyle(el);
    return Object.fromEntries(
      tokenNames.map(name => [name, parseInt(style.getPropertyValue(`--z-${name}`).trim(), 10)])
    );
  }, names);
  return result as Record<keyof typeof Z_LAYERS, number>;
}

/** Opens the page-import-dialog via the tools menu trigger. */
async function openPageImportDialog(page: Page): Promise<void> {
  await expect(page.locator('#page-import-trigger')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });
  await page.locator('.tools-menu').hover();
  await page.locator('#page-import-trigger').click();
  await expect(page.locator('page-import-dialog')).toHaveAttribute('open', '', { timeout: MENU_APPEAR_TIMEOUT_MS });
}

/**
 * Opens the search results popover by typing a query into the wiki-search input
 * and waiting for the wiki-search-results element to gain its open attribute.
 */
async function openSearchPopover(page: Page): Promise<void> {
  const searchInput = page.locator('wiki-search input[type="search"]');
  await expect(searchInput).toBeVisible({ timeout: MENU_APPEAR_TIMEOUT_MS });
  await searchInput.fill('test');
  await searchInput.press('Enter');
  await expect(page.locator('wiki-search-results')).toHaveAttribute('open', '', { timeout: MENU_APPEAR_TIMEOUT_MS });
}

/** Injects a toast-message into the page and waits for it to be fully slid into view. */
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
  // Wait for the visible attribute (set by show()), then wait for the slide-in
  // animation to complete so that getBoundingClientRect() reflects the final position.
  await expect(page.locator('toast-message')).toHaveAttribute('visible', '', { timeout: MENU_APPEAR_TIMEOUT_MS });
  // The toast slides in from the right: animation is complete once the right edge
  // is within the viewport (i.e. no longer translated off-screen by translateX(100%)).
  await page.waitForFunction(() => {
    const toast = document.querySelector('toast-message') as HTMLElement;
    if (!toast) return false;
    const rect = toast.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0 && rect.right <= window.innerWidth + 1;
  }, { timeout: MENU_APPEAR_TIMEOUT_MS });
}

/**
 * Returns the custom element tag name of the topmost element at (x, y),
 * or 'none' if neither the hit element nor any ancestor is a custom element.
 * If the hit element lives inside a shadow root, the search starts from the host.
 */
async function getTopmostCustomElementTagAt(page: Page, x: number, y: number): Promise<string> {
  return page.evaluate(([px, py]: [number, number]) => {
    const el = document.elementFromPoint(px, py);
    if (!el) return 'none';
    const root = el.getRootNode();
    let current: Element | null = root instanceof ShadowRoot ? root.host : el;
    while (current && current !== document.body) {
      const tag = current.tagName.toLowerCase();
      if (tag.includes('-')) return tag;
      current = current.parentElement;
    }
    return 'none';
  }, [x, y] as [number, number]);
}

test.describe('Z-Index Token System (ADR-0008)', () => {
  test.setTimeout(60000);

  test.beforeEach(async ({ page }) => {
    await page.goto('/home/view');
    await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
  });

  test.describe('CSS custom property token values', () => {
    let tokens: Record<keyof typeof Z_LAYERS, number>;

    test.beforeEach(async ({ page }) => {
      await expect(page.locator('system-info')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });
      tokens = await readTokenValues(page);
    });

    test('ambient token should equal 100', async () => {
      expect(tokens.ambient).toBe(Z_LAYERS.ambient);
    });

    test('drawer token should equal 200', async () => {
      expect(tokens.drawer).toBe(Z_LAYERS.drawer);
    });

    test('popover token should equal 300', async () => {
      expect(tokens.popover).toBe(Z_LAYERS.popover);
    });

    test('modal token should equal 400', async () => {
      expect(tokens.modal).toBe(Z_LAYERS.modal);
    });

    test('notification token should equal 500', async () => {
      expect(tokens.notification).toBe(Z_LAYERS.notification);
    });

    test('blocker token should equal 600', async () => {
      expect(tokens.blocker).toBe(Z_LAYERS.blocker);
    });

    test('all six layer tokens should have distinct z-index values', async () => {
      const values = Object.values(tokens);
      expect(new Set(values).size).toBe(values.length);
    });

    test('tokens should be ordered from lowest to highest by layer', async () => {
      expect(tokens.ambient).toBeLessThan(tokens.drawer);
      expect(tokens.drawer).toBeLessThan(tokens.popover);
      expect(tokens.popover).toBeLessThan(tokens.modal);
      expect(tokens.modal).toBeLessThan(tokens.notification);
      expect(tokens.notification).toBeLessThan(tokens.blocker);
    });
  });

  test.describe('drawer layer (z-index: 200)', () => {
    test.describe('when system-info is present on the page', () => {
      let drawerZIndex: number;

      test.beforeEach(async ({ page }) => {
        await expect(page.locator('system-info')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });
        drawerZIndex = await page.evaluate(() => {
          const el = document.querySelector('system-info');
          if (!el) throw new Error('system-info not found');
          return parseInt(getComputedStyle(el).zIndex, 10);
        });
      });

      test('system-info should render at the drawer tier', async () => {
        expect(drawerZIndex).toBe(Z_LAYERS.drawer);
      });
    });
  });

  test.describe('popover layer (z-index: 300)', () => {
    test.describe('when search popover is open', () => {
      let popoverZIndex: number;

      test.beforeEach(async ({ page }) => {
        await expect(page.locator('wiki-search')).toBeAttached({ timeout: MENU_APPEAR_TIMEOUT_MS });
        await openSearchPopover(page);

        // wiki-search-results lives inside wiki-search's shadow DOM.
        // It uses --z-popover on its internal .popover element.
        popoverZIndex = await page.evaluate(() => {
          const searchEl = document.querySelector('wiki-search');
          if (!searchEl) throw new Error('wiki-search not found');
          if (!searchEl.shadowRoot) throw new Error('wiki-search shadowRoot not found');
          const resultsEl = searchEl.shadowRoot.querySelector('wiki-search-results');
          if (!resultsEl) throw new Error('wiki-search-results not found inside wiki-search shadowRoot');
          if (!resultsEl.shadowRoot) throw new Error('wiki-search-results shadowRoot not found');
          const popover = resultsEl.shadowRoot.querySelector('.popover');
          if (!popover) throw new Error('.popover not found inside wiki-search-results shadowRoot');
          return parseInt(getComputedStyle(popover).zIndex, 10);
        });
      });

      test('search results popover should render at the popover tier', async () => {
        expect(popoverZIndex).toBe(Z_LAYERS.popover);
      });
    });
  });

  test.describe('modal layer (z-index: 400)', () => {
    test.describe('when page-import-dialog is open', () => {
      let dialogZIndex: number;
      let topmostTagAtCenter: string;

      test.beforeEach(async ({ page }) => {
        await openPageImportDialog(page);
        await expect(page.locator('page-import-dialog')).toBeVisible({ timeout: MENU_APPEAR_TIMEOUT_MS });

        dialogZIndex = await page.evaluate(() => {
          const el = document.querySelector('page-import-dialog');
          if (!el) throw new Error('page-import-dialog not found');
          return parseInt(getComputedStyle(el).zIndex, 10);
        });

        // Use the Playwright viewport size to compute center coordinates
        const viewport = page.viewportSize();
        if (!viewport) throw new Error('page.viewportSize() returned null — no viewport configured');
        const centerX = viewport.width / 2;
        const centerY = viewport.height / 2;

        // The dialog backdrop covers the full viewport; at center the dialog should be topmost
        topmostTagAtCenter = await getTopmostCustomElementTagAt(page, centerX, centerY);
      });

      test('page-import-dialog should render at the modal tier', async () => {
        expect(dialogZIndex).toBe(Z_LAYERS.modal);
      });

      test('dialog should be the topmost element at the viewport center', async () => {
        expect(topmostTagAtCenter).toBe('page-import-dialog');
      });
    });
  });

  test.describe('notification layer (z-index: 500)', () => {
    test.describe('when a toast is shown', () => {
      let toastZIndex: number;

      test.beforeEach(async ({ page }) => {
        await injectToast(page, 'Z-index notification tier test');
        toastZIndex = await page.evaluate(() => {
          const el = document.querySelector('toast-message');
          if (!el) throw new Error('toast-message not found');
          return parseInt(getComputedStyle(el).zIndex, 10);
        });
      });

      test('toast-message should render at the notification tier', async () => {
        expect(toastZIndex).toBe(Z_LAYERS.notification);
      });
    });

    test.describe('when a toast is shown while a modal is open', () => {
      let toastZIndex: number;
      let dialogZIndex: number;
      let topmostTagAtToast: string;

      test.beforeEach(async ({ page }) => {
        // Open dialog first — its backdrop covers the full viewport at z-index 400
        await openPageImportDialog(page);

        // Inject toast — it slides in at z-index 500, above the dialog;
        // injectToast waits for the slide-in animation to complete
        await injectToast(page, 'Notification above dialog');

        const zIndexes = await page.evaluate(() => {
          const toast = document.querySelector('toast-message');
          const dialog = document.querySelector('page-import-dialog');
          if (!toast || !dialog) throw new Error('toast-message or page-import-dialog not found');
          return {
            toastZ: parseInt(getComputedStyle(toast).zIndex, 10),
            dialogZ: parseInt(getComputedStyle(dialog).zIndex, 10),
          };
        });
        toastZIndex = zIndexes.toastZ;
        dialogZIndex = zIndexes.dialogZ;

        const toastRect = await page.locator('toast-message').boundingBox();
        if (!toastRect) throw new Error('toast-message has no bounding box');
        topmostTagAtToast = await getTopmostCustomElementTagAt(
          page,
          toastRect.x + toastRect.width / 2,
          toastRect.y + toastRect.height / 2,
        );
      });

      test('toast should have a higher z-index than the open modal dialog', async () => {
        expect(toastZIndex).toBeGreaterThan(dialogZIndex);
      });

      test('toast should be the topmost element at its screen position', async () => {
        expect(topmostTagAtToast).toBe('toast-message');
      });
    });
  });

  test.describe('no z-index conflicts between overlapping components', () => {
    test.describe('when drawer and modal are both present', () => {
      let drawerZIndex: number;
      let modalZIndex: number;

      test.beforeEach(async ({ page }) => {
        await openPageImportDialog(page);

        const zIndexes = await page.evaluate(() => {
          const drawer = document.querySelector('system-info');
          const modal = document.querySelector('page-import-dialog');
          if (!drawer || !modal) throw new Error('system-info or page-import-dialog not found');
          return {
            drawerZ: parseInt(getComputedStyle(drawer).zIndex, 10),
            modalZ: parseInt(getComputedStyle(modal).zIndex, 10),
          };
        });
        drawerZIndex = zIndexes.drawerZ;
        modalZIndex = zIndexes.modalZ;
      });

      test('drawer and modal should have distinct z-index values', async () => {
        expect(drawerZIndex).not.toBe(modalZIndex);
      });

      test('modal should be above the drawer', async () => {
        expect(modalZIndex).toBeGreaterThan(drawerZIndex);
      });
    });

    test.describe('when notification and modal are both present', () => {
      let notificationZIndex: number;
      let modalZIndex: number;

      test.beforeEach(async ({ page }) => {
        await openPageImportDialog(page);
        await injectToast(page, 'Conflict check notification');

        const zIndexes = await page.evaluate(() => {
          const notification = document.querySelector('toast-message');
          const modal = document.querySelector('page-import-dialog');
          if (!notification || !modal) throw new Error('toast-message or page-import-dialog not found');
          return {
            notificationZ: parseInt(getComputedStyle(notification).zIndex, 10),
            modalZ: parseInt(getComputedStyle(modal).zIndex, 10),
          };
        });
        notificationZIndex = zIndexes.notificationZ;
        modalZIndex = zIndexes.modalZ;
      });

      test('notification and modal should have distinct z-index values', async () => {
        expect(notificationZIndex).not.toBe(modalZIndex);
      });

      test('notification should be above the modal', async () => {
        expect(notificationZIndex).toBeGreaterThan(modalZIndex);
      });
    });
  });
});
