import { test, expect, type Page } from '@playwright/test';
import { COMPONENT_LOAD_TIMEOUT_MS } from './constants.js';

// Helpers

async function gotoEditPage(page: Page): Promise<void> {
  await page.goto('/home/edit');
  await expect(page.locator('wiki-editor editor-toolbar')).toBeAttached({
    timeout: COMPONENT_LOAD_TIMEOUT_MS,
  });
}

async function waitForEditorReady(page: Page): Promise<void> {
  await expect(page.locator('wiki-editor textarea')).toBeVisible({
    timeout: COMPONENT_LOAD_TIMEOUT_MS,
  });
}

test.describe('editor-toolbar', () => {
  test.setTimeout(60000);

  test.describe('toolbar button rendering', () => {
    test.beforeEach(async ({ page }) => {
      await gotoEditPage(page);
    });

    test('bold button is rendered', async ({ page }) => {
      await expect(page.locator('wiki-editor editor-toolbar [data-action="bold"]')).toBeAttached();
    });

    test('italic button is rendered', async ({ page }) => {
      await expect(page.locator('wiki-editor editor-toolbar [data-action="italic"]')).toBeAttached();
    });

    test('link button is rendered', async ({ page }) => {
      await expect(page.locator('wiki-editor editor-toolbar [data-action="link"]')).toBeAttached();
    });

    test('upload image button is rendered', async ({ page }) => {
      await expect(
        page.locator('wiki-editor editor-toolbar [data-action="upload-image"]'),
      ).toBeAttached();
    });

    test('new page button is rendered', async ({ page }) => {
      await expect(
        page.locator('wiki-editor editor-toolbar [data-action="new-page"]'),
      ).toBeAttached();
    });

    test('exit/done button is rendered', async ({ page }) => {
      await expect(page.locator('wiki-editor editor-toolbar [data-action="exit"]')).toBeAttached();
    });
  });

  test.describe('when no text is selected', () => {
    test.beforeEach(async ({ page }) => {
      await gotoEditPage(page);
    });

    test('bold button is disabled', async ({ page }) => {
      await expect(
        page.locator('wiki-editor editor-toolbar [data-action="bold"]'),
      ).toBeDisabled();
    });

    test('italic button is disabled', async ({ page }) => {
      await expect(
        page.locator('wiki-editor editor-toolbar [data-action="italic"]'),
      ).toBeDisabled();
    });

    test('link button is disabled', async ({ page }) => {
      await expect(
        page.locator('wiki-editor editor-toolbar [data-action="link"]'),
      ).toBeDisabled();
    });
  });

  test.describe('when has-selection is enabled on the toolbar', () => {
    test.beforeEach(async ({ page }) => {
      await gotoEditPage(page);
      await page.evaluate(() => {
        const wikiEditor = document.querySelector('wiki-editor');
        const toolbar = wikiEditor?.shadowRoot?.querySelector('editor-toolbar');
        toolbar?.setAttribute('has-selection', '');
      });
    });

    test('bold button is enabled', async ({ page }) => {
      await expect(
        page.locator('wiki-editor editor-toolbar [data-action="bold"]'),
      ).toBeEnabled();
    });

    test('italic button is enabled', async ({ page }) => {
      await expect(
        page.locator('wiki-editor editor-toolbar [data-action="italic"]'),
      ).toBeEnabled();
    });

    test('link button is enabled', async ({ page }) => {
      await expect(
        page.locator('wiki-editor editor-toolbar [data-action="link"]'),
      ).toBeEnabled();
    });

    test.describe('when the bold button is clicked', () => {
      let eventReceived: Promise<boolean>;

      test.beforeEach(async ({ page }) => {
        eventReceived = page.evaluate(
          () =>
            new Promise<boolean>((resolve) => {
              document.addEventListener('format-bold-requested', () => resolve(true), {
                once: true,
              });
              setTimeout(() => resolve(false), 3000);
            }),
        );
        await page.locator('wiki-editor editor-toolbar [data-action="bold"]').click();
      });

      test('dispatches format-bold-requested event', async () => {
        expect(await eventReceived).toBe(true);
      });
    });

    test.describe('when the italic button is clicked', () => {
      let eventReceived: Promise<boolean>;

      test.beforeEach(async ({ page }) => {
        eventReceived = page.evaluate(
          () =>
            new Promise<boolean>((resolve) => {
              document.addEventListener('format-italic-requested', () => resolve(true), {
                once: true,
              });
              setTimeout(() => resolve(false), 3000);
            }),
        );
        await page.locator('wiki-editor editor-toolbar [data-action="italic"]').click();
      });

      test('dispatches format-italic-requested event', async () => {
        expect(await eventReceived).toBe(true);
      });
    });

    test.describe('when the link button is clicked', () => {
      let eventReceived: Promise<boolean>;

      test.beforeEach(async ({ page }) => {
        eventReceived = page.evaluate(
          () =>
            new Promise<boolean>((resolve) => {
              document.addEventListener('insert-link-requested', () => resolve(true), {
                once: true,
              });
              setTimeout(() => resolve(false), 3000);
            }),
        );
        await page.locator('wiki-editor editor-toolbar [data-action="link"]').click();
      });

      test('dispatches insert-link-requested event', async () => {
        expect(await eventReceived).toBe(true);
      });
    });
  });

  test.describe('upload dropdown', () => {
    test.beforeEach(async ({ page }) => {
      await gotoEditPage(page);
    });

    test.describe('when the dropdown toggle is clicked', () => {
      test.beforeEach(async ({ page }) => {
        await page
          .locator('wiki-editor editor-toolbar [title="More upload options"]')
          .click();
      });

      test('upload dropdown menu is visible', async ({ page }) => {
        await expect(
          page.locator('wiki-editor editor-toolbar .upload-dropdown-menu'),
        ).toBeVisible();
      });

      test('upload file option is present in the menu', async ({ page }) => {
        await expect(
          page.locator('wiki-editor editor-toolbar [data-action="upload-file"]'),
        ).toBeVisible();
      });
    });

    test.describe('when the dropdown toggle is clicked twice', () => {
      test.beforeEach(async ({ page }) => {
        const toggle = page.locator(
          'wiki-editor editor-toolbar [title="More upload options"]',
        );
        await toggle.click();
        // Wait for menu to appear before clicking again
        await expect(
          page.locator('wiki-editor editor-toolbar .upload-dropdown-menu'),
        ).toBeVisible();
        await toggle.click();
      });

      test('upload dropdown menu is dismissed', async ({ page }) => {
        await expect(
          page.locator('wiki-editor editor-toolbar .upload-dropdown-menu'),
        ).not.toBeAttached();
      });
    });

    test.describe('when clicking outside the dropdown after it is open', () => {
      test.beforeEach(async ({ page }) => {
        await page
          .locator('wiki-editor editor-toolbar [title="More upload options"]')
          .click();
        await expect(
          page.locator('wiki-editor editor-toolbar .upload-dropdown-menu'),
        ).toBeVisible();
        // Click outside the toolbar — use the textarea which is always below the toolbar
        await page.locator('wiki-editor textarea').click();
      });

      test('upload dropdown menu is closed', async ({ page }) => {
        await expect(
          page.locator('wiki-editor editor-toolbar .upload-dropdown-menu'),
        ).not.toBeAttached();
      });
    });
  });
});

test.describe('file-drop-zone', () => {
  test.setTimeout(60000);

  test.describe('drop overlay on drag', () => {
    test.describe('when uploads are enabled and a file is dragged over the drop zone', () => {
      test.beforeEach(async ({ page }) => {
        await gotoEditPage(page);
        await waitForEditorReady(page);

        await page.evaluate(() => {
          const wikiEditor = document.querySelector('wiki-editor');
          const fileDropZone = wikiEditor?.shadowRoot?.querySelector('file-drop-zone') as
            | (HTMLElement & { allowUploads: boolean })
            | null;
          if (!fileDropZone) return;

          fileDropZone.allowUploads = true;

          // Dispatch dragenter on the inner .drop-zone element so the component's
          // event handler fires and sets dragging = true.
          const dropZoneEl = fileDropZone.shadowRoot?.querySelector('.drop-zone');
          dropZoneEl?.dispatchEvent(new DragEvent('dragenter', { bubbles: true, cancelable: true }));
        });
      });

      test('drop overlay is visible', async ({ page }) => {
        await expect(
          page.locator('wiki-editor file-drop-zone .drop-overlay'),
        ).toBeVisible();
      });
    });

    test.describe('when uploads are disabled and a file is dragged over the drop zone', () => {
      test.beforeEach(async ({ page }) => {
        await gotoEditPage(page);
        await waitForEditorReady(page);

        // Explicitly disable uploads then dispatch drag (server config may have uploads enabled)
        await page.evaluate(() => {
          const wikiEditor = document.querySelector('wiki-editor');
          const fileDropZone = wikiEditor?.shadowRoot?.querySelector('file-drop-zone') as
            | (HTMLElement & { allowUploads: boolean })
            | null;
          if (!fileDropZone) return;
          fileDropZone.allowUploads = false;
          const dropZoneEl = fileDropZone.shadowRoot?.querySelector('.drop-zone');
          dropZoneEl?.dispatchEvent(new DragEvent('dragenter', { bubbles: true, cancelable: true }));
        });
      });

      test('drop overlay is not shown', async ({ page }) => {
        await expect(
          page.locator('wiki-editor file-drop-zone .drop-overlay'),
        ).not.toBeAttached();
      });
    });
  });

  test.describe('upload error display', () => {
    test.describe('when a file exceeds the maximum upload size', () => {
      test.beforeEach(async ({ page }) => {
        await gotoEditPage(page);
        await waitForEditorReady(page);

        // Lower the max size to 0 so any non-empty file triggers the validation error,
        // then call _uploadFile directly to exercise the validation path.
        await page.evaluate(async () => {
          const wikiEditor = document.querySelector('wiki-editor');
          const fileDropZone = wikiEditor?.shadowRoot?.querySelector('file-drop-zone') as
            | (HTMLElement & {
                allowUploads: boolean;
                maxUploadMb: number;
                _uploadFile: (file: File) => Promise<void>;
              })
            | null;
          if (!fileDropZone) return;

          fileDropZone.allowUploads = true;
          fileDropZone.maxUploadMb = 0; // 0 MB limit — any file is too large
          const file = new File(['test content'], 'test-image.png', { type: 'image/png' });
          await fileDropZone._uploadFile(file);
        });
      });

      test('error-display component is visible', async ({ page }) => {
        await expect(
          page.locator('wiki-editor file-drop-zone error-display'),
        ).toBeVisible();
      });
    });

    test.describe('when the error dismiss action is clicked', () => {
      test.beforeEach(async ({ page }) => {
        await gotoEditPage(page);
        await waitForEditorReady(page);

        // Trigger the validation error
        await page.evaluate(async () => {
          const wikiEditor = document.querySelector('wiki-editor');
          const fileDropZone = wikiEditor?.shadowRoot?.querySelector('file-drop-zone') as
            | (HTMLElement & {
                allowUploads: boolean;
                maxUploadMb: number;
                _uploadFile: (file: File) => Promise<void>;
              })
            | null;
          if (!fileDropZone) return;

          fileDropZone.allowUploads = true;
          fileDropZone.maxUploadMb = 0;
          const file = new File(['test content'], 'test-image.png', { type: 'image/png' });
          await fileDropZone._uploadFile(file);
        });

        await expect(page.locator('wiki-editor file-drop-zone error-display')).toBeVisible({ timeout: 5000 });

        // Click the Dismiss button using page.evaluate() with explicit shadow DOM traversal.
        // The button lives 3 shadow roots deep (wiki-editor > file-drop-zone > error-display)
        // and Playwright's chained locators don't reliably pierce that many levels.
        await page.evaluate(() => {
          const wikiEditor = document.querySelector('wiki-editor');
          const fileDropZone = wikiEditor?.shadowRoot?.querySelector('file-drop-zone');
          const errorDisplay = fileDropZone?.shadowRoot?.querySelector('error-display');
          const actionButton = errorDisplay?.shadowRoot?.querySelector('.action-button') as HTMLButtonElement | null;
          actionButton?.click();
        });
      });

      test('error-display component is removed', async ({ page }) => {
        await expect(
          page.locator('wiki-editor file-drop-zone error-display'),
        ).not.toBeAttached();
      });
    });
  });
});
