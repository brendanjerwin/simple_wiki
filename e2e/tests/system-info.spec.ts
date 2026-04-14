import { test, expect, type Page } from '@playwright/test';
import { COMPONENT_LOAD_TIMEOUT_MS } from './constants.js';

// Timeouts
const PANEL_INTERACTION_TIMEOUT_MS = 5000;
const API_LOAD_TIMEOUT_MS = 10000;

/** Navigate to the home page view and wait for the rendered content to be attached. */
async function navigateAndWait(page: Page): Promise<void> {
  await page.goto('/home/view');
  await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
  await expect(page.locator('system-info')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
}

/** Click the INFO drawer tab to open the system-info panel and wait for it to open. */
async function openSystemInfoPanel(page: Page): Promise<void> {
  const drawerTab = page.locator('system-info .drawer-tab');
  await expect(drawerTab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
  await drawerTab.click();
  await expect(page.locator('system-info .system-panel.drawerOpen')).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
}

test.describe('System Info Panel', () => {
  test.setTimeout(60000);

  test.beforeEach(async ({ page }) => {
    await navigateAndWait(page);
  });

  test.describe('Panel structure', () => {
    test('system-info element is attached to the DOM on a view page', async ({ page }) => {
      await expect(page.locator('system-info')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    });

    test('INFO drawer tab is present and shows correct label', async ({ page }) => {
      const drawerTab = page.locator('system-info .drawer-tab');
      await expect(drawerTab).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(drawerTab).toHaveText('INFO');
    });

    test('opening the panel reveals panel content', async ({ page }) => {
      await openSystemInfoPanel(page);
      await expect(page.locator('system-info .system-panel')).toHaveAttribute('aria-expanded', 'true', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });
  });

  test.describe('Version display', () => {
    test('system-info-version sub-component is attached when panel is open', async ({ page }) => {
      await openSystemInfoPanel(page);
      const versionComp = page.locator('system-info system-info-version');
      await expect(versionComp).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    });

    test('version info shows "Server:" label after API data loads', async ({ page }) => {
      await openSystemInfoPanel(page);
      const label = page.locator('system-info system-info-version .label');
      await expect(label).toHaveText('Server:', { timeout: API_LOAD_TIMEOUT_MS });
    });

    test('version commit value is non-empty after API data loads', async ({ page }) => {
      await openSystemInfoPanel(page);
      const commitValue = page.locator('system-info system-info-version .value.commit');
      await expect(commitValue).toBeAttached({ timeout: API_LOAD_TIMEOUT_MS });
      await expect(commitValue).not.toBeEmpty({ timeout: API_LOAD_TIMEOUT_MS });
    });

    test('version commit value does not show placeholder "Loading..." text', async ({ page }) => {
      await openSystemInfoPanel(page);
      const versionInfo = page.locator('system-info system-info-version .version-info');
      await expect(versionInfo).toBeAttached({ timeout: API_LOAD_TIMEOUT_MS });
      // Wait until loading indicator is gone — loading state shows "Commit:" and "Built:" labels
      await expect(page.locator('system-info system-info-version .value.loading')).not.toBeAttached({ timeout: API_LOAD_TIMEOUT_MS });
    });
  });

  test.describe('Identity section', () => {
    test('system-info-identity sub-component is attached when panel is open', async ({ page }) => {
      await openSystemInfoPanel(page);
      const identityComp = page.locator('system-info system-info-identity');
      await expect(identityComp).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    });

    test('identity section renders without an error-display when no Tailscale identity is configured', async ({ page }) => {
      await openSystemInfoPanel(page);
      // The component returns `nothing` when no identity is configured — no error-display should appear.
      const identityComp = page.locator('system-info system-info-identity');
      await expect(identityComp).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(identityComp.locator('error-display')).not.toBeAttached({ timeout: API_LOAD_TIMEOUT_MS });
    });
  });

  test.describe('Jobs section', () => {
    test('system-info-jobs sub-component is attached when panel is open', async ({ page }) => {
      await openSystemInfoPanel(page);
      const jobsComp = page.locator('system-info system-info-jobs');
      await expect(jobsComp).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    });

    test('jobs section renders without an error-display', async ({ page }) => {
      await openSystemInfoPanel(page);
      // Verify the jobs component is not showing an error — if error-display is attached,
      // the jobs component is in an error state which should not happen.
      const jobsComp = page.locator('system-info system-info-jobs');
      await expect(jobsComp).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(jobsComp.locator('error-display')).not.toBeAttached({ timeout: API_LOAD_TIMEOUT_MS });
    });
  });

  test.describe('Accessibility', () => {
    test('panel container has role="button"', async ({ page }) => {
      const panel = page.locator('system-info .system-panel');
      await expect(panel).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(panel).toHaveAttribute('role', 'button');
    });

    test('panel container has descriptive aria-label', async ({ page }) => {
      const panel = page.locator('system-info .system-panel');
      await expect(panel).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(panel).toHaveAttribute('aria-label', 'System information panel');
    });

    test('panel container is keyboard-focusable via tabindex', async ({ page }) => {
      const panel = page.locator('system-info .system-panel');
      await expect(panel).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(panel).toHaveAttribute('tabindex', '0');
    });

    test('aria-expanded is "false" when panel is closed', async ({ page }) => {
      const panel = page.locator('system-info .system-panel');
      await expect(panel).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(panel).toHaveAttribute('aria-expanded', 'false');
    });

    test('aria-expanded becomes "true" after panel is opened by click', async ({ page }) => {
      await openSystemInfoPanel(page);
      const panel = page.locator('system-info .system-panel');
      await expect(panel).toHaveAttribute('aria-expanded', 'true');
    });

    test('panel can be opened with the Enter key', async ({ page }) => {
      const panel = page.locator('system-info .system-panel');
      await expect(panel).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await panel.focus();
      await panel.press('Enter');
      await expect(panel).toHaveAttribute('aria-expanded', 'true', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('panel can be opened with the Space key', async ({ page }) => {
      const panel = page.locator('system-info .system-panel');
      await expect(panel).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await panel.focus();
      await panel.press('Space');
      await expect(panel).toHaveAttribute('aria-expanded', 'true', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('clicking outside the panel closes it', async ({ page }) => {
      await openSystemInfoPanel(page);
      const panel = page.locator('system-info .system-panel');
      await expect(panel).toHaveAttribute('aria-expanded', 'true');

      // Click somewhere outside the system-info element
      await page.locator('body').click({ position: { x: 10, y: 10 } });

      await expect(panel).toHaveAttribute('aria-expanded', 'false', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });
  });
});
