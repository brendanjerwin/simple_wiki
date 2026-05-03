import { test, expect, type Page } from '@playwright/test';
import { COMPONENT_LOAD_TIMEOUT_MS } from './constants.js';

// Timeouts
const SAVE_TIMEOUT_MS = 10000;
const BLOG_LOAD_TIMEOUT_MS = 10000;
const DIALOG_APPEAR_TIMEOUT_MS = 5000;

// Unique blog page for this suite (avoids conflicts with blog-features.spec.ts)
const BLOG_PAGE = 'e2e_blog_new_post_a11y';

/**
 * Opens the blog-new-post-dialog by clicking the "New Post" button.
 * Waits until the dialog host has the `open` attribute.
 */
async function openDialog(page: Page): Promise<void> {
  const newPostButton = page.locator('wiki-blog button', { hasText: 'New Post' });
  await expect(newPostButton).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });
  await newPostButton.click();
  await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).toBeAttached({
    timeout: DIALOG_APPEAR_TIMEOUT_MS,
  });
}

test.describe('blog-new-post-dialog accessibility', () => {
  test.setTimeout(90000);

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    const textarea = page.locator('wiki-editor textarea');

    await page.goto(`/${BLOG_PAGE}/edit`);
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await textarea.fill(`+++
identifier = "${BLOG_PAGE}"
title = "E2E Blog A11y Test"
+++

{{ Blog "${BLOG_PAGE}" 10 }}`);
    await textarea.press('Space');
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
      timeout: SAVE_TIMEOUT_MS,
    });

    await ctx.close();
  });

  test.afterAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    const textarea = page.locator('wiki-editor textarea');

    try {
      await page.goto(`/${BLOG_PAGE}/edit`);
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await textarea.fill(`+++\nidentifier = "${BLOG_PAGE}"\n+++`);
      await textarea.press('Space');
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
        timeout: SAVE_TIMEOUT_MS,
      });
    } catch (_) {
      // Best-effort cleanup
    } finally {
      await ctx.close();
    }
  });

  test.beforeEach(async ({ page }) => {
    await page.goto(`/${BLOG_PAGE}/view`);
    await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
  });

  test.describe('ARIA attributes', () => {
    test('native <dialog> element has aria-labelledby set', async ({ page }) => {
      await openDialog(page);

      const nativeDialog = page.locator('wiki-blog blog-new-post-dialog dialog');
      await expect(nativeDialog).toHaveAttribute('aria-labelledby', 'blog-new-post-dialog-title');
    });

    test('aria-labelledby references an element with text "New Blog Post"', async ({ page }) => {
      await openDialog(page);

      const titleEl = page.locator('wiki-blog blog-new-post-dialog #blog-new-post-dialog-title');
      await expect(titleEl).toBeVisible();
      await expect(titleEl).toContainText('New Blog Post');
    });
  });

  test.describe('Escape key', () => {
    test('closes the dialog via the native cancel event', async ({ page }) => {
      await openDialog(page);

      await page.keyboard.press('Escape');

      await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).not.toBeAttached({
        timeout: DIALOG_APPEAR_TIMEOUT_MS,
      });
    });
  });

  test.describe('focus management', () => {
    test('title-input receives focus when the dialog opens', async ({ page }) => {
      await openDialog(page);

      // blog-new-post-dialog is inside wiki-blog's shadow root.
      // Traverse: document -> wiki-blog (shadow host) -> blog-new-post-dialog (shadow host)
      // The mixin schedules focus via rAF, so poll until the active element settles.
      await expect
        .poll(
          () =>
            page.evaluate(() => {
              const blog = document.querySelector('wiki-blog');
              const dialogHost = blog?.shadowRoot?.querySelector('blog-new-post-dialog');
              const active = dialogHost?.shadowRoot?.activeElement;
              return active?.tagName?.toLowerCase() ?? null;
            }),
          { timeout: DIALOG_APPEAR_TIMEOUT_MS },
        )
        .toBe('title-input');
    });

    test('focus returns to the wiki-blog host after closing via Cancel', async ({ page }) => {
      await openDialog(page);

      const cancelButton = page.locator('wiki-blog blog-new-post-dialog .btn-cancel');
      await cancelButton.click();

      await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).not.toBeAttached({
        timeout: DIALOG_APPEAR_TIMEOUT_MS,
      });

      // NativeDialogMixin restores focus to document.activeElement captured at open time.
      // When the "New Post" button (inside wiki-blog's shadow root) is clicked, the browser
      // sets document.activeElement to wiki-blog (the outermost shadow host).
      await expect
        .poll(
          () =>
            page.evaluate(() => {
              return document.activeElement?.tagName?.toLowerCase() ?? null;
            }),
          { timeout: DIALOG_APPEAR_TIMEOUT_MS },
        )
        .toBe('wiki-blog');
    });

    test('focus returns to the wiki-blog host after closing via Escape', async ({ page }) => {
      await openDialog(page);

      await page.keyboard.press('Escape');

      await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).not.toBeAttached({
        timeout: DIALOG_APPEAR_TIMEOUT_MS,
      });

      await expect
        .poll(
          () =>
            page.evaluate(() => {
              return document.activeElement?.tagName?.toLowerCase() ?? null;
            }),
          { timeout: DIALOG_APPEAR_TIMEOUT_MS },
        )
        .toBe('wiki-blog');
    });
  });

  test.describe('focus trap', () => {
    test('Tab key keeps focus within the dialog while it is open', async ({ page }) => {
      await openDialog(page);

      // Tab through several focusable elements; native showModal() traps focus
      for (let i = 0; i < 6; i++) {
        await page.keyboard.press('Tab');
      }

      await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).toBeAttached();
    });

    test('Shift+Tab keeps focus within the dialog while it is open', async ({ page }) => {
      await openDialog(page);

      for (let i = 0; i < 4; i++) {
        await page.keyboard.press('Shift+Tab');
      }

      await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).toBeAttached();
    });
  });

  test.describe('keyboard activation', () => {
    test('Enter activates the Cancel button when it is focused', async ({ page }) => {
      await openDialog(page);

      const cancelButton = page.locator('wiki-blog blog-new-post-dialog .btn-cancel');
      await cancelButton.focus();
      await page.keyboard.press('Enter');

      await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).not.toBeAttached({
        timeout: DIALOG_APPEAR_TIMEOUT_MS,
      });
    });

    test('Enter activates the close (×) button when it is focused', async ({ page }) => {
      await openDialog(page);

      const closeBtn = page.locator('wiki-blog blog-new-post-dialog .close-btn');
      await closeBtn.focus();
      await page.keyboard.press('Enter');

      await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).not.toBeAttached({
        timeout: DIALOG_APPEAR_TIMEOUT_MS,
      });
    });
  });
});
