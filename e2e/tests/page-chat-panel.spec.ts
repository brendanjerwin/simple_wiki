import { test, expect } from '@playwright/test';

// Timeouts
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PANEL_INTERACTION_TIMEOUT_MS = 5000;
const NETWORK_SETTLE_WAIT_MS = 1000;
const STREAM_REQUEST_TIMEOUT_MS = 5000;

test.describe('Chat Panel', () => {
  test.setTimeout(60000);

  test.describe('Panel open and close', () => {

    test('should render the chat FAB button on page load', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // The FAB button is present in the shadow DOM (disabled when Claude not connected)
      const fab = page.locator('page-chat-panel .fab');
      await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should open the chat panel when FAB is clicked', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const fab = page.locator('page-chat-panel .fab');
      await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await fab.click();

      // Panel should now have the 'open' class
      await expect(page.locator('page-chat-panel .panel.open')).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should hide the FAB when panel is open', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const fab = page.locator('page-chat-panel .fab');
      await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await fab.click();

      // FAB should no longer be in the DOM when panel is open
      await expect(fab).not.toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should close the chat panel when close button is clicked', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Open the panel first
      const fab = page.locator('page-chat-panel .fab');
      await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await fab.click();
      await expect(page.locator('page-chat-panel .panel.open')).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });

      // Close it via the close button
      const closeBtn = page.locator('page-chat-panel .close-button');
      await closeBtn.click();

      // Panel should no longer have the 'open' class
      await expect(page.locator('page-chat-panel .panel.open')).not.toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should restore panel open state from localStorage', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Open the panel and let it save to localStorage
      const fab = page.locator('page-chat-panel .fab');
      await fab.click();
      await expect(page.locator('page-chat-panel .panel.open')).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });

      // Reload the page — panel should restore as open due to localStorage
      await page.reload();
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await expect(page.locator('page-chat-panel .panel.open')).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

  });

  test.describe('Persona name display', () => {

    test('should show empty panel title when no persona is configured', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Open the panel
      const fab = page.locator('page-chat-panel .fab');
      await fab.click();

      // Panel title should be empty (no persona configured)
      const panelTitle = page.locator('page-chat-panel .panel-title');
      await expect(panelTitle).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(panelTitle).toHaveText('');
    });

    test('should show "Open chat" aria-label when no persona is configured', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const fab = page.locator('page-chat-panel .fab');
      await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(fab).toHaveAttribute('aria-label', 'Open chat');
    });

    test('should display persona name in panel title when persona is configured', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Configure the persona via JavaScript (simulating server-side config)
      await page.evaluate(() => {
        const el = document.querySelector('page-chat-panel');
        if (el) (el as HTMLElement & { persona: string }).persona = 'Aria';
      });

      // Open the panel
      const fab = page.locator('page-chat-panel .fab');
      await fab.click();

      // Panel title should show the persona name
      const panelTitle = page.locator('page-chat-panel .panel-title');
      await expect(panelTitle).toHaveText('Aria', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should include persona name in FAB aria-label when persona is configured', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Configure the persona via JavaScript
      await page.evaluate(() => {
        const el = document.querySelector('page-chat-panel');
        if (el) (el as HTMLElement & { persona: string }).persona = 'Aria';
      });

      // Wait for Lit to re-render with the updated persona
      const fab = page.locator('page-chat-panel .fab');
      await expect(fab).toHaveAttribute('aria-label', 'Chat with Aria', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should include persona name in disconnected banner when persona is configured', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Configure the persona via JavaScript
      await page.evaluate(() => {
        const el = document.querySelector('page-chat-panel');
        if (el) (el as HTMLElement & { persona: string }).persona = 'Aria';
      });

      // Open the panel (Claude not connected — disconnected banner shows persona name)
      const fab = page.locator('page-chat-panel .fab');
      await fab.click();

      const disconnectedBanner = page.locator('page-chat-panel .status-banner.disconnected');
      await expect(disconnectedBanner).toContainText('Aria is not connected', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

  });

  test.describe('Chat stream deferral', () => {

    test('should NOT make SubscribeChat requests on page load (panel closed)', async ({ page }) => {
      const subscribeChatRequestUrls: string[] = [];

      // Listen for SubscribeChat requests before navigating
      page.on('request', (request) => {
        if (request.url().includes('/api.v1.ChatService/SubscribeChat')) {
          subscribeChatRequestUrls.push(request.url());
        }
      });

      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Wait to allow any initial requests to be captured
      await page.waitForTimeout(NETWORK_SETTLE_WAIT_MS);

      expect(subscribeChatRequestUrls).toHaveLength(0);
    });

    test('should make a SubscribeChat request when panel is opened for the first time', async ({ page }) => {
      const subscribeChatRequestUrls: string[] = [];

      // Listen for SubscribeChat requests before navigating
      page.on('request', (request) => {
        if (request.url().includes('/api.v1.ChatService/SubscribeChat')) {
          subscribeChatRequestUrls.push(request.url());
        }
      });

      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await page.waitForTimeout(NETWORK_SETTLE_WAIT_MS);

      // Confirm no requests have been made yet
      expect(subscribeChatRequestUrls).toHaveLength(0);

      // Open the panel — this should trigger startStream()
      const requestPromise = page.waitForRequest(
        (req) => req.url().includes('/api.v1.ChatService/SubscribeChat'),
        { timeout: STREAM_REQUEST_TIMEOUT_MS },
      );
      const fab = page.locator('page-chat-panel .fab');
      await fab.click();

      // Wait for the SubscribeChat request to be made
      await requestPromise;

      expect(subscribeChatRequestUrls.length).toBeGreaterThan(0);
    });

  });

  test.describe('Basic message send/receive flow', () => {

    test('should show the disconnected status banner when Claude is not available', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Open the panel
      const fab = page.locator('page-chat-panel .fab');
      await fab.click();

      // Should show disconnected banner (Claude not configured in E2E server)
      const disconnectedBanner = page.locator('page-chat-panel .status-banner.disconnected');
      await expect(disconnectedBanner).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should disable textarea and send button when Claude is not connected', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Open the panel
      const fab = page.locator('page-chat-panel .fab');
      await fab.click();

      // Textarea and send button should be disabled
      await expect(page.locator('page-chat-panel textarea')).toBeDisabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(page.locator('page-chat-panel .send-button')).toBeDisabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should enable textarea and send button when Claude is connected', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Open the panel
      const fab = page.locator('page-chat-panel .fab');
      await fab.click();
      await expect(page.locator('page-chat-panel .panel.open')).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });

      // Simulate Claude becoming connected
      await page.evaluate(() => {
        const el = document.querySelector('page-chat-panel');
        if (el) (el as HTMLElement & { claudeConnected: boolean }).claudeConnected = true;
      });

      // Textarea and send button should now be enabled
      await expect(page.locator('page-chat-panel textarea')).toBeEnabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(page.locator('page-chat-panel .send-button')).toBeEnabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should allow typing in the message textarea when Claude is connected', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Open the panel
      const fab = page.locator('page-chat-panel .fab');
      await fab.click();
      await expect(page.locator('page-chat-panel .panel.open')).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });

      // Simulate Claude becoming connected
      await page.evaluate(() => {
        const el = document.querySelector('page-chat-panel');
        if (el) (el as HTMLElement & { claudeConnected: boolean }).claudeConnected = true;
      });

      // Type a message in the textarea
      const textarea = page.locator('page-chat-panel textarea');
      await expect(textarea).toBeEnabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await textarea.fill('Hello, how can you help me?');

      await expect(textarea).toHaveValue('Hello, how can you help me?');
    });

    test('should attempt to send a message when send button is clicked', async ({ page }) => {
      const sendMessageRequests: string[] = [];

      page.on('request', (request) => {
        if (request.url().includes('/api.v1.ChatService/SendMessage')) {
          sendMessageRequests.push(request.url());
        }
      });

      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Open the panel
      const fab = page.locator('page-chat-panel .fab');
      await fab.click();
      await expect(page.locator('page-chat-panel .panel.open')).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });

      // Simulate Claude becoming connected
      await page.evaluate(() => {
        const el = document.querySelector('page-chat-panel');
        if (el) (el as HTMLElement & { claudeConnected: boolean }).claudeConnected = true;
      });

      const textarea = page.locator('page-chat-panel textarea');
      await expect(textarea).toBeEnabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await textarea.fill('Hello!');

      // Wait for the SendMessage request when the button is clicked
      const requestPromise = page.waitForRequest(
        (req) => req.url().includes('/api.v1.ChatService/SendMessage'),
        { timeout: STREAM_REQUEST_TIMEOUT_MS },
      );

      const sendBtn = page.locator('page-chat-panel .send-button');
      await sendBtn.click();

      await requestPromise;

      expect(sendMessageRequests.length).toBeGreaterThan(0);
    });

    test('should show empty state message when no messages exist', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Open the panel
      const fab = page.locator('page-chat-panel .fab');
      await fab.click();

      // Should show the empty state message
      const emptyState = page.locator('page-chat-panel .empty-state');
      await expect(emptyState).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(emptyState).toContainText('Send a message');
    });

  });

});
