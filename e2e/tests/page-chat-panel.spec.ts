import { test, expect, type Page } from '@playwright/test';

// Timeouts
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PANEL_INTERACTION_TIMEOUT_MS = 5000;
const REQUEST_TIMEOUT_MS = 5000;

/** Navigate to the home page and wait for the app to be ready. */
async function navigateAndWait(page: Page): Promise<void> {
  await page.goto('/home/view');
  await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
}

/** Click the FAB and wait for the panel to open. Returns the open panel locator. */
async function openChatPanel(page: Page) {
  const fab = page.locator('page-chat-panel .fab');
  const openPanel = page.locator('page-chat-panel .panel.open');

  await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
  await fab.click();
  await expect(openPanel).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });

  return { fab, openPanel };
}

/** Set a property on page-chat-panel, waiting for the element to be upgraded first. */
async function setChatPanelProperty(
  page: Page,
  prop: string,
  value: string | boolean,
): Promise<void> {
  await page.evaluate(
    ([p, v]) => {
      return customElements.whenDefined('page-chat-panel').then(() => {
        const el = document.querySelector('page-chat-panel');
        if (el) (el as HTMLElement & Record<string, unknown>)[p] = v;
      });
    },
    [prop, value] as [string, string | boolean],
  );
}

test.describe('Chat Panel', () => {
  test.setTimeout(60000);

  test.describe('Panel open and close', () => {
    test.beforeEach(async ({ page }) => {
      await navigateAndWait(page);
    });

    test('should render the chat FAB button on page load', async ({ page }) => {
      // The FAB button is present in the shadow DOM (shows disabled style when assistant not connected)
      const fab = page.locator('page-chat-panel .fab');
      await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should open the chat panel when FAB is clicked', async ({ page }) => {
      await openChatPanel(page);
    });

    test('should hide the FAB when panel is open', async ({ page }) => {
      const { fab } = await openChatPanel(page);

      // FAB should be hidden (but still in DOM) when panel is open, for focus management
      await expect(fab).toBeHidden({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should close the chat panel when close button is clicked', async ({ page }) => {
      const { openPanel } = await openChatPanel(page);

      // Close it via the close button
      const closeBtn = page.locator('page-chat-panel .close-button');
      await closeBtn.click();

      // Panel should no longer have the 'open' class
      await expect(openPanel).not.toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should restore panel open state from localStorage', async ({ page }) => {
      // Open the panel and wait for localStorage to be persisted
      await openChatPanel(page);

      await expect
        .poll(
          () => page.evaluate(() => localStorage.getItem('chat-panel-open')),
          { timeout: PANEL_INTERACTION_TIMEOUT_MS },
        )
        .toBe('true');

      // Reload the page — panel should restore as open due to localStorage
      await page.reload();
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await expect(page.locator('page-chat-panel .panel.open')).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });
  });

  test.describe('Persona name display', () => {
    test.beforeEach(async ({ page }) => {
      await navigateAndWait(page);
    });

    test('should show empty panel title when no persona is configured', async ({ page }) => {
      await openChatPanel(page);

      // Panel title should be empty (no persona configured)
      const panelTitle = page.locator('page-chat-panel .panel-title');
      await expect(panelTitle).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(panelTitle).toHaveText(/^\s*$/);
    });

    test('should show "Open chat" aria-label when no persona is configured', async ({ page }) => {
      const fab = page.locator('page-chat-panel .fab');
      await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(fab).toHaveAttribute('aria-label', 'Open chat');
    });

    test('should display persona name in panel title when persona is configured', async ({ page }) => {
      await setChatPanelProperty(page, 'persona', 'Aria');

      await openChatPanel(page);

      // Panel title should show the persona name
      const panelTitle = page.locator('page-chat-panel .panel-title');
      await expect(panelTitle).toHaveText('Aria', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should include persona name in FAB aria-label when persona is configured', async ({ page }) => {
      await setChatPanelProperty(page, 'persona', 'Aria');

      // Wait for Lit to re-render with the updated persona
      const fab = page.locator('page-chat-panel .fab');
      await expect(fab).toHaveAttribute('aria-label', 'Chat with Aria', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should include persona name in disconnected banner when persona is configured', async ({ page }) => {
      await setChatPanelProperty(page, 'persona', 'Aria');

      // Open the panel (assistant not connected — disconnected banner shows persona name)
      await openChatPanel(page);

      const disconnectedBanner = page.locator('page-chat-panel .status-banner.disconnected');
      await expect(disconnectedBanner).toContainText('Aria is not connected', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });
  });

  test.describe('Chat stream deferral', () => {

    test('should NOT make SubscribeChat requests on page load (panel closed)', async ({ page }) => {
      // Install route handler before navigating to detect any unexpected SubscribeChat calls
      let subscribeChatCallCount = 0;
      await page.route('**/*SubscribeChat*', (route) => {
        subscribeChatCallCount++;
        void route.continue().catch(() => {});
      });

      await navigateAndWait(page);

      // Poll briefly to ensure no requests have been made
      await expect
        .poll(() => subscribeChatCallCount, { timeout: 2000 })
        .toBe(0);
    });

    test('should make a SubscribeChat request when panel is opened for the first time', async ({ page }) => {
      await navigateAndWait(page);

      // Confirm no SubscribeChat requests before opening
      let subscribeChatCallCount = 0;
      page.on('request', (request) => {
        if (request.url().includes('SubscribeChat')) subscribeChatCallCount++;
      });

      // Opening the panel should trigger startStream()
      const requestPromise = page.waitForRequest(
        (req) => req.url().includes('SubscribeChat'),
        { timeout: REQUEST_TIMEOUT_MS },
      );
      await openChatPanel(page);

      // Wait for the SubscribeChat request to be made
      await requestPromise;

      expect(subscribeChatCallCount).toBeGreaterThan(0);
    });
  });

  test.describe('Basic message send/receive flow', () => {
    test.beforeEach(async ({ page }) => {
      await navigateAndWait(page);
    });

    test('should show the disconnected status banner when assistant is not available', async ({ page }) => {
      await openChatPanel(page);

      // Should show disconnected banner (assistant not configured in E2E server)
      const disconnectedBanner = page.locator('page-chat-panel .status-banner.disconnected');
      await expect(disconnectedBanner).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should disable textarea and send button when assistant is not connected', async ({ page }) => {
      await openChatPanel(page);

      // Textarea and send button should be disabled
      await expect(page.locator('page-chat-panel textarea')).toBeDisabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(page.locator('page-chat-panel .send-button')).toBeDisabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should enable textarea and send button when assistant is connected', async ({ page }) => {
      await openChatPanel(page);

      // Simulate assistant becoming connected
      await setChatPanelProperty(page, 'agentConnected', true);

      // Textarea and send button should now be enabled
      await expect(page.locator('page-chat-panel textarea')).toBeEnabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(page.locator('page-chat-panel .send-button')).toBeEnabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should allow typing in the message textarea when assistant is connected', async ({ page }) => {
      await openChatPanel(page);

      // Simulate assistant becoming connected
      await setChatPanelProperty(page, 'agentConnected', true);

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

      await openChatPanel(page);

      // Simulate assistant becoming connected
      await setChatPanelProperty(page, 'agentConnected', true);

      const textarea = page.locator('page-chat-panel textarea');
      await expect(textarea).toBeEnabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await textarea.fill('Hello!');

      // Wait for the SendMessage request when the button is clicked
      const requestPromise = page.waitForRequest(
        (req) => req.url().includes('/api.v1.ChatService/SendMessage'),
        { timeout: REQUEST_TIMEOUT_MS },
      );

      const sendBtn = page.locator('page-chat-panel .send-button');
      await sendBtn.click();

      await requestPromise;

      expect(sendMessageRequests.length).toBeGreaterThan(0);
    });

    test('should show empty state message when no messages exist', async ({ page }) => {
      await openChatPanel(page);

      // Should show the empty state message
      const emptyState = page.locator('page-chat-panel .empty-state');
      await expect(emptyState).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(emptyState).toContainText('Send a message');
    });
  });
});

