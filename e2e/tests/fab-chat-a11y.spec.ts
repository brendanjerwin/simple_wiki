import { test, expect, type Page } from '@playwright/test';

// Timeouts
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PANEL_INTERACTION_TIMEOUT_MS = 5000;

/** Navigate to the home page and wait for the app to be ready. */
async function navigateAndWait(page: Page): Promise<void> {
  await page.goto('/home/view');
  await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
}

/** Click the FAB and wait for the panel to open. Returns FAB and open panel locators. */
async function openChatPanel(page: Page) {
  const fab = page.locator('page-chat-panel .fab');
  const openPanel = page.locator('page-chat-panel .panel.open');

  await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
  await fab.click();
  await expect(openPanel).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });

  return { fab, openPanel };
}

test.describe('FAB Chat Button Accessibility', () => {
  test.setTimeout(60000);

  test.beforeEach(async ({ page }) => {
    await navigateAndWait(page);
  });

  test('FAB has aria-controls="chat-panel"', async ({ page }) => {
    const fab = page.locator('page-chat-panel .fab');
    await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    await expect(fab).toHaveAttribute('aria-controls', 'chat-panel');
  });

  test('panel element has id="chat-panel"', async ({ page }) => {
    const panel = page.locator('page-chat-panel #chat-panel');
    await expect(panel).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
  });

  test('FAB has aria-expanded="false" when panel is closed', async ({ page }) => {
    const fab = page.locator('page-chat-panel .fab');
    await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    await expect(fab).toHaveAttribute('aria-expanded', 'false');
  });

  test('FAB has aria-expanded="true" when panel is open', async ({ page }) => {
    const { fab } = await openChatPanel(page);
    await expect(fab).toHaveAttribute('aria-expanded', 'true', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
  });

  test('FAB has hidden attribute while panel is open', async ({ page }) => {
    const { fab } = await openChatPanel(page);
    await expect(fab).toHaveAttribute('hidden', '', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
  });

  test('focus returns to FAB after closing the panel', async ({ page }) => {
    const { openPanel } = await openChatPanel(page);

    const closeBtn = page.locator('page-chat-panel .close-button');
    await closeBtn.click();
    await expect(openPanel).not.toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });

    // Focus must return to the FAB in the shadow DOM after panel closes
    await expect
      .poll(
        () =>
          page.evaluate(() => {
            const host = document.querySelector('page-chat-panel');
            if (!host?.shadowRoot) return false;
            const fab = host.shadowRoot.querySelector('.fab');
            return host.shadowRoot.activeElement === fab;
          }),
        { timeout: PANEL_INTERACTION_TIMEOUT_MS },
      )
      .toBe(true);
  });
});
