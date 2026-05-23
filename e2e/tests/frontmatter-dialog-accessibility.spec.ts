import { test, expect, type APIRequestContext } from '@playwright/test';
import { COMPONENT_LOAD_TIMEOUT_MS } from './constants.js';

const TEST_RUN_SUFFIX = [
  process.env.GITHUB_RUN_ID ?? String(Date.now()).slice(-8),
  process.env.TEST_WORKER_INDEX ?? String(process.pid),
].join('_').replace(/[^A-Za-z0-9_]/g, '');

const TEST_PAGE_NAME = `E2EFrontmatterDialogA11y_${TEST_RUN_SUFFIX}`;
const TEST_PAGE_IDENTIFIER = TEST_PAGE_NAME.toLowerCase();
const DIALOG_TIMEOUT_MS = 5000;

async function callPageAPI(
  request: APIRequestContext,
  method: string,
  body: Record<string, unknown>,
) {
  return request.post(`/api.v1.PageManagementService/${method}`, {
    headers: { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1' },
    data: body,
  });
}

async function setupTestPage(request: APIRequestContext): Promise<void> {
  const contentMarkdown = `+++
identifier = "${TEST_PAGE_IDENTIFIER}"
title = "Frontmatter Dialog Accessibility"
+++

# Frontmatter Dialog Accessibility`;

  const createResp = await callPageAPI(request, 'CreatePage', {
    pageName: TEST_PAGE_IDENTIFIER,
    contentMarkdown,
  });
  if (createResp.ok()) {
    const body = await createResp.json() as { success: boolean };
    if (body.success) return;
  }

  const resetResp = await callPageAPI(request, 'UpdatePageContent', {
    pageName: TEST_PAGE_IDENTIFIER,
    newContentMarkdown: contentMarkdown,
  });
  expect(resetResp.ok()).toBeTruthy();
}

async function openFrontmatterDialog(page: import('@playwright/test').Page): Promise<void> {
  await page.locator('wiki-editor textarea').focus();
  await page.evaluate((pageName) => {
    const dialog = document.querySelector('frontmatter-editor-dialog') as
      | (HTMLElement & { openDialog?: (page: string) => void })
      | null;
    dialog?.openDialog?.(pageName);
  }, TEST_PAGE_IDENTIFIER);
  await expect(page.locator('frontmatter-editor-dialog dialog')).toBeVisible({
    timeout: DIALOG_TIMEOUT_MS,
  });
  await expect(page.locator('frontmatter-editor-dialog .loading')).not.toBeVisible({
    timeout: COMPONENT_LOAD_TIMEOUT_MS,
  });
}

test.describe('FrontmatterEditorDialog accessibility', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test.beforeAll(async ({ request }) => {
    await setupTestPage(request);
  });

  test.afterAll(async ({ request }) => {
    try {
      const resp = await callPageAPI(request, 'DeletePage', { pageName: TEST_PAGE_IDENTIFIER });
      if (!resp.ok()) {
        console.warn(`[afterAll] DeletePage(${TEST_PAGE_IDENTIFIER}) returned HTTP ${resp.status()}`);
      }
    } catch (err) {
      console.warn(`[afterAll] DeletePage(${TEST_PAGE_IDENTIFIER}) threw: ${String(err)}`);
    }
  });

  test.beforeEach(async ({ page }) => {
    await page.goto(`/${TEST_PAGE_IDENTIFIER}/edit`);
    await expect(page.locator('wiki-editor textarea')).toBeVisible({
      timeout: COMPONENT_LOAD_TIMEOUT_MS,
    });

    await openFrontmatterDialog(page);
  });

  test.describe('ARIA attributes', () => {
    test.describe('when the dialog is open', () => {
      test('should use a native dialog element', async ({ page }) => {
        const tagName = await page.evaluate(() =>
          document
            .querySelector('frontmatter-editor-dialog')
            ?.shadowRoot?.querySelector('dialog')
            ?.tagName.toLowerCase()
        );
        expect(tagName).toBe('dialog');
      });

      test('should label the dialog with the title element', async ({ page }) => {
        const labelledBy = await page.evaluate(() =>
          document
            .querySelector('frontmatter-editor-dialog')
            ?.shadowRoot?.querySelector('dialog')
            ?.getAttribute('aria-labelledby')
        );
        expect(labelledBy).toBe('frontmatter-dialog-title');
      });

      test('should expose the dialog title text', async ({ page }) => {
        const titleText = await page.evaluate(() =>
          document
            .querySelector('frontmatter-editor-dialog')
            ?.shadowRoot?.getElementById('frontmatter-dialog-title')
            ?.textContent?.trim()
        );
        expect(titleText).toBe('Edit Frontmatter');
      });
    });
  });

  test.describe('keyboard dismissal', () => {
    test.describe('when Escape is pressed', () => {
      test.beforeEach(async ({ page }) => {
        await page.keyboard.press('Escape');
      });

      test('should close the dialog', async ({ page }) => {
        await expect(page.locator('frontmatter-editor-dialog dialog')).not.toBeVisible({
          timeout: DIALOG_TIMEOUT_MS,
        });
      });
    });
  });

  test.describe('backdrop click', () => {
    test.describe('when the backdrop area is clicked', () => {
      test.beforeEach(async ({ page }) => {
        await page.mouse.click(0, 0);
      });

      test('should close the dialog', async ({ page }) => {
        await expect(page.locator('frontmatter-editor-dialog dialog')).not.toBeVisible({
          timeout: DIALOG_TIMEOUT_MS,
        });
      });
    });
  });

  test.describe('focus management', () => {
    test.describe('when the dialog opens', () => {
      test('should move focus inside the dialog shadow root', async ({ page }) => {
        await expect.poll(
          () => page.evaluate(() => {
            const host = document.querySelector('frontmatter-editor-dialog');
            return host?.shadowRoot?.activeElement !== null;
          }),
          { timeout: DIALOG_TIMEOUT_MS },
        ).toBe(true);
      });
    });

    test.describe('when Cancel is clicked', () => {
      test.beforeEach(async ({ page }) => {
        await page.locator('frontmatter-editor-dialog button.button-secondary').click();
        await expect(page.locator('frontmatter-editor-dialog dialog')).not.toBeVisible({
          timeout: DIALOG_TIMEOUT_MS,
        });
      });

      test('should return focus to the editor textarea', async ({ page }) => {
        await expect(page.locator('wiki-editor textarea')).toBeFocused({
          timeout: DIALOG_TIMEOUT_MS,
        });
      });
    });
  });

  test.describe('focus trap', () => {
    test.describe('when Tab is pressed from the Cancel button', () => {
      test.beforeEach(async ({ page }) => {
        await page.locator('frontmatter-editor-dialog button.button-secondary').focus();
        await page.keyboard.press('Tab');
      });

      test('should move focus to the Save button', async ({ page }) => {
        await expect.poll(
          () => page.evaluate(() => {
            const host = document.querySelector('frontmatter-editor-dialog');
            return (host?.shadowRoot?.activeElement as HTMLElement | null)?.textContent?.trim();
          }),
          { timeout: DIALOG_TIMEOUT_MS },
        ).toBe('Save');
      });

      test('should keep the dialog open', async ({ page }) => {
        await expect(page.locator('frontmatter-editor-dialog dialog')).toBeVisible();
      });
    });

    test.describe('when Tab is pressed from the Save button', () => {
      test.beforeEach(async ({ page }) => {
        await page.locator('frontmatter-editor-dialog .footer button.button-primary').focus();
        await page.keyboard.press('Tab');
      });

      test('should wrap focus to the close button', async ({ page }) => {
        await expect.poll(
          () => page.evaluate(() => {
            const host = document.querySelector('frontmatter-editor-dialog');
            return (host?.shadowRoot?.activeElement as HTMLElement | null)?.getAttribute('aria-label');
          }),
          { timeout: DIALOG_TIMEOUT_MS },
        ).toBe('Close dialog');
      });
    });

    test.describe('when Shift+Tab is pressed from the close button', () => {
      test.beforeEach(async ({ page }) => {
        await page.locator('frontmatter-editor-dialog button.icon-button').focus();
        await page.keyboard.press('Shift+Tab');
      });

      test('should wrap focus to the Save button', async ({ page }) => {
        await expect.poll(
          () => page.evaluate(() => {
            const host = document.querySelector('frontmatter-editor-dialog');
            return (host?.shadowRoot?.activeElement as HTMLElement | null)?.textContent?.trim();
          }),
          { timeout: DIALOG_TIMEOUT_MS },
        ).toBe('Save');
      });
    });
  });
});
