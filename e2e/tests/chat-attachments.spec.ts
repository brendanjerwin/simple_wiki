import { test, expect, type Page } from '@playwright/test';
import { COMPONENT_LOAD_TIMEOUT_MS } from './constants.js';

const PANEL_INTERACTION_TIMEOUT_MS = 5000;

/** Navigate to the home page and wait for the app to be ready. */
async function navigateAndWait(page: Page): Promise<void> {
  await page.goto('/home/view');
  await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
}

/** Open the chat panel by clicking the FAB. */
async function openChatPanel(page: Page) {
  const fab = page.locator('page-chat-panel .fab');
  const openPanel = page.locator('page-chat-panel .panel.open');
  await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
  await fab.click();
  await expect(openPanel).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
  return { openPanel };
}

/** Intercept the upload request and return a fake attachment response. */
async function interceptAttachmentUpload(page: Page, attachmentId = 'test-attach-id') {
  await page.route('/api/chat-attachments', async (route) => {
    const request = route.request();
    const body = request.postData();
    const filename = body ? extractFilenameFromFormData(body) : 'file.png';
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({
        id: attachmentId,
        url: `/api/chat-attachments/${attachmentId}`,
        filename: filename || 'pasted-image.png',
        mimeType: 'image/png',
      }),
    });
  });
}

/** Rudimentary extraction of filename from multipart form body string. */
function extractFilenameFromFormData(body: string): string | null {
  const match = /filename="([^"]+)"/.exec(body);
  return match ? match[1] : null;
}

test.describe('Chat Attachments', () => {
  test.setTimeout(60000);

  test.describe('+ button and attach menu', () => {
    test.beforeEach(async ({ page }) => {
      await navigateAndWait(page);
      await openChatPanel(page);
    });

    test('should render the attach (+) button', async ({ page }) => {
      const attachButton = page.locator('page-chat-panel .attach-button');
      await expect(attachButton).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should open the attach menu when + button is clicked', async ({ page }) => {
      const attachButton = page.locator('page-chat-panel .attach-button');
      await attachButton.click();
      const menu = page.locator('page-chat-panel .attach-menu');
      await expect(menu).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should show upload file, photo library, and camera options in the menu', async ({ page }) => {
      const attachButton = page.locator('page-chat-panel .attach-button');
      await attachButton.click();

      const menuItems = page.locator('page-chat-panel .attach-menu-item');
      await expect(menuItems).toHaveCount(3, { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should close the attach menu when clicking outside', async ({ page }) => {
      const attachButton = page.locator('page-chat-panel .attach-button');
      await attachButton.click();
      await expect(page.locator('page-chat-panel .attach-menu')).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });

      // Click outside the menu (on the messages area)
      await page.locator('page-chat-panel .messages-container').click();
      await expect(page.locator('page-chat-panel .attach-menu')).not.toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });
  });

  test.describe('camera input attributes', () => {
    test.beforeEach(async ({ page }) => {
      await navigateAndWait(page);
      await openChatPanel(page);
    });

    test('camera input should have capture="environment" attribute', async ({ page }) => {
      const cameraInput = page.locator('page-chat-panel .attach-input-camera');
      await expect(cameraInput).toHaveAttribute('capture', 'environment', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('camera input should accept images', async ({ page }) => {
      const cameraInput = page.locator('page-chat-panel .attach-input-camera');
      await expect(cameraInput).toHaveAttribute('accept', 'image/*', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('photo library input should accept images without capture attribute', async ({ page }) => {
      const imageInput = page.locator('page-chat-panel .attach-input-image');
      await expect(imageInput).toHaveAttribute('accept', 'image/*', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(imageInput).not.toHaveAttribute('capture', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('file input should accept all file types', async ({ page }) => {
      const fileInput = page.locator('page-chat-panel .attach-input-file');
      await expect(fileInput).toHaveAttribute('accept', '*/*', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });
  });

  test.describe('paste image attach flow', () => {
    test.beforeEach(async ({ page }) => {
      await interceptAttachmentUpload(page, 'pasted-id-123');
      await navigateAndWait(page);
      await openChatPanel(page);
    });

    test('should show an attachment chip after pasting an image', async ({ page }) => {
      const textarea = page.locator('page-chat-panel textarea');
      await textarea.focus();

      // Simulate paste of a PNG file via the clipboard API
      await page.evaluate(async () => {
        const data = new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10]); // minimal PNG header
        const blob = new Blob([data], { type: 'image/png' });
        const file = new File([blob], 'pasted-image.png', { type: 'image/png' });

        const dt = new DataTransfer();
        dt.items.add(file);

        const chatPanel = document.querySelector('page-chat-panel');
        const shadowTextarea = chatPanel?.shadowRoot?.querySelector('textarea');
        if (!shadowTextarea) throw new Error('textarea not found');

        const event = new ClipboardEvent('paste', {
          bubbles: true,
          cancelable: true,
          clipboardData: dt,
        });
        shadowTextarea.dispatchEvent(event);
      });

      // The attachment chip should appear
      const chip = page.locator('page-chat-panel .attachment-chip');
      await expect(chip).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should display the filename in the attachment chip', async ({ page }) => {
      const textarea = page.locator('page-chat-panel textarea');
      await textarea.focus();

      await page.evaluate(async () => {
        const data = new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10]);
        const blob = new Blob([data], { type: 'image/png' });
        const file = new File([blob], 'screenshot.png', { type: 'image/png' });

        const dt = new DataTransfer();
        dt.items.add(file);

        const chatPanel = document.querySelector('page-chat-panel');
        const shadowTextarea = chatPanel?.shadowRoot?.querySelector('textarea');
        if (!shadowTextarea) throw new Error('textarea not found');

        const event = new ClipboardEvent('paste', {
          bubbles: true,
          cancelable: true,
          clipboardData: dt,
        });
        shadowTextarea.dispatchEvent(event);
      });

      const chipName = page.locator('page-chat-panel .chip-name');
      await expect(chipName).toContainText('screenshot', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should allow removing an attachment chip via the × button', async ({ page }) => {
      const textarea = page.locator('page-chat-panel textarea');
      await textarea.focus();

      await page.evaluate(async () => {
        const data = new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10]);
        const blob = new Blob([data], { type: 'image/png' });
        const file = new File([blob], 'to-remove.png', { type: 'image/png' });

        const dt = new DataTransfer();
        dt.items.add(file);

        const chatPanel = document.querySelector('page-chat-panel');
        const shadowTextarea = chatPanel?.shadowRoot?.querySelector('textarea');
        if (!shadowTextarea) throw new Error('textarea not found');

        const event = new ClipboardEvent('paste', {
          bubbles: true,
          cancelable: true,
          clipboardData: dt,
        });
        shadowTextarea.dispatchEvent(event);
      });

      const chip = page.locator('page-chat-panel .attachment-chip');
      await expect(chip).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });

      const removeButton = page.locator('page-chat-panel .chip-remove');
      await removeButton.click();

      await expect(chip).not.toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });
  });

  test.describe('drag-and-drop attach flow', () => {
    test.beforeEach(async ({ page }) => {
      await interceptAttachmentUpload(page, 'dragged-id-456');
      await navigateAndWait(page);
      await openChatPanel(page);
    });

    test('should show drag-over state when dragging a file over the input area', async ({ page }) => {
      const inputArea = page.locator('page-chat-panel .input-area');

      await page.evaluate(() => {
        const chatPanel = document.querySelector('page-chat-panel');
        const area = chatPanel?.shadowRoot?.querySelector('.input-area');
        if (!area) throw new Error('input-area not found');

        const dt = new DataTransfer();
        const event = new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer: dt });
        area.dispatchEvent(event);
      });

      await expect(inputArea).toHaveClass(/drag-over/, { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should clear drag-over state on dragleave', async ({ page }) => {
      await page.evaluate(() => {
        const chatPanel = document.querySelector('page-chat-panel');
        const area = chatPanel?.shadowRoot?.querySelector('.input-area');
        if (!area) throw new Error('input-area not found');

        const dt = new DataTransfer();
        area.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer: dt }));
        area.dispatchEvent(new DragEvent('dragleave', { bubbles: true, cancelable: true, dataTransfer: dt }));
      });

      const inputArea = page.locator('page-chat-panel .input-area');
      await expect(inputArea).not.toHaveClass(/drag-over/, { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should show an attachment chip after dropping an image file', async ({ page }) => {
      await page.evaluate(async () => {
        const data = new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10]);
        const blob = new Blob([data], { type: 'image/png' });
        const file = new File([blob], 'dropped-photo.png', { type: 'image/png' });

        const chatPanel = document.querySelector('page-chat-panel');
        const area = chatPanel?.shadowRoot?.querySelector('.input-area');
        if (!area) throw new Error('input-area not found');

        const dt = new DataTransfer();
        dt.items.add(file);

        area.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer: dt }));
        area.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: dt }));
      });

      const chip = page.locator('page-chat-panel .attachment-chip');
      await expect(chip).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should display the dropped filename in the attachment chip', async ({ page }) => {
      await page.evaluate(async () => {
        const data = new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10]);
        const blob = new Blob([data], { type: 'image/png' });
        const file = new File([blob], 'diagram.png', { type: 'image/png' });

        const chatPanel = document.querySelector('page-chat-panel');
        const area = chatPanel?.shadowRoot?.querySelector('.input-area');
        if (!area) throw new Error('input-area not found');

        const dt = new DataTransfer();
        dt.items.add(file);

        area.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: dt }));
      });

      const chipName = page.locator('page-chat-panel .chip-name');
      await expect(chipName).toContainText('diagram', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });
  });
});
