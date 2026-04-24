import { test, expect, type Page } from '@playwright/test';

// Timeouts
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PANEL_INTERACTION_TIMEOUT_MS = 5000;
const SCROLL_SETTLE_TIMEOUT_MS = 2000;

type MessageSpec = {
  id: string;
  sender: number;
  content: string;
  renderedHtml: string;
  senderName: string;
  replyToId: string;
  edited: boolean;
  sequence: number;
  toolCalls: Array<{ toolCallId: string; title: string; status: string }>;
};

/** Navigate to the home page and wait for the app to be ready. */
async function navigateAndWait(page: Page): Promise<void> {
  await page.goto('/home/view');
  await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
}

/** Click the FAB and wait for the panel to open. */
async function openChatPanel(page: Page) {
  const fab = page.locator('page-chat-panel .fab');
  const openPanel = page.locator('page-chat-panel .panel.open');

  await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
  await fab.click();
  await expect(openPanel).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });

  return { fab, openPanel };
}

/** Wait for Lit to complete its update cycle on page-chat-panel. */
async function waitForUpdate(page: Page): Promise<void> {
  await page.evaluate(() => {
    return customElements.whenDefined('page-chat-panel').then(() => {
      const el = document.querySelector('page-chat-panel');
      if (el && 'updateComplete' in el) {
        return (el as HTMLElement & { updateComplete: Promise<boolean> }).updateComplete;
      }
    });
  });
}

/** Get scroll position info from the messages container (in shadow DOM). */
async function getScrollInfo(
  page: Page,
): Promise<{ scrollTop: number; scrollHeight: number; clientHeight: number } | null> {
  return page.evaluate(() => {
    const host = document.querySelector('page-chat-panel');
    if (!host?.shadowRoot) return null;
    const container = host.shadowRoot.querySelector('.messages-container') as HTMLElement | null;
    if (!container) return null;
    return {
      scrollTop: container.scrollTop,
      scrollHeight: container.scrollHeight,
      clientHeight: container.clientHeight,
    };
  });
}

/**
 * Inject messages into the component, populating both `messages` (for rendering)
 * and the private `messagesById` map (needed for tool call updates to find messages).
 * TypeScript `private` is a compile-time concept only — these members are accessible at runtime.
 */
async function injectMessagesWithMap(page: Page, messages: MessageSpec[]): Promise<void> {
  await page.evaluate((msgs) => {
    return customElements.whenDefined('page-chat-panel').then(() => {
      const el = document.querySelector('page-chat-panel');
      if (!el) return;

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const typedEl = el as any;

      const processedMsgs = msgs.map((m) => ({
        ...m,
        timestamp: new Date(),
        reactions: [],
        sequence: BigInt(m.sequence),
      }));

      // Set the public messages array to trigger Lit re-render
      typedEl.messages = processedMsgs;

      // Populate the private messagesById map so updateToolCall() can find messages.
      // We mutate the existing Map rather than reassigning (avoids potential issues
      // with the readonly TypeScript annotation, which is not enforced at runtime).
      typedEl.messagesById.clear();
      for (const m of processedMsgs) {
        typedEl.messagesById.set(m.id, m);
      }
    });
  }, messages);
}

/**
 * Call the component's private `updateToolCall` method to simulate a tool call chat event.
 * This exercises the same scroll logic that runs during a real stream event.
 */
async function triggerToolCallUpdate(
  page: Page,
  messageId: string,
  toolCallId: string,
  title: string,
  status: string,
): Promise<void> {
  await page.evaluate(
    ([msgId, tcId, t, s]) => {
      return customElements.whenDefined('page-chat-panel').then(() => {
        const el = document.querySelector('page-chat-panel');
        if (!el) return Promise.resolve();
        // TypeScript private methods are accessible at runtime in compiled JS
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        return (el as any).updateToolCall(msgId, tcId, t, s);
      });
    },
    [messageId, toolCallId, title, status] as [string, string, string, string],
  );
}

/**
 * Simulate the user manually scrolling up in the messages container.
 * Sets scrollTop to 0 and dispatches a scroll event so the component's
 * _handleScroll handler runs and sets userHasScrolled = true.
 */
async function simulateUserScrollUp(page: Page): Promise<void> {
  await page.evaluate(() => {
    const host = document.querySelector('page-chat-panel');
    if (!host?.shadowRoot) return;
    const container = host.shadowRoot.querySelector('.messages-container') as HTMLElement | null;
    if (!container) return;

    // Scroll to top — when content overflows, this causes _handleScroll to set userHasScrolled = true
    container.scrollTop = 0;
    // Dispatch scroll event explicitly to ensure the Lit handler fires even if scrollTop was already 0
    container.dispatchEvent(new Event('scroll'));
  });
}

/** Generate N messages with enough content to make the messages container scrollable. */
function makeManyMessages(count = 20): MessageSpec[] {
  return Array.from({ length: count }, (_, i) => ({
    id: `scroll-msg-${i}`,
    sender: i % 2 === 0 ? 0 : 1, // alternate USER (0) / ASSISTANT (1)
    content: `Message ${i + 1}: This message has enough text to take up vertical space so the container becomes scrollable when many messages are loaded.`,
    renderedHtml: `<p>Message ${i + 1}: This message has enough text to take up vertical space so the container becomes scrollable when many messages are loaded.</p>`,
    senderName: i % 2 === 0 ? 'User' : 'Bot',
    replyToId: '',
    edited: false,
    sequence: i + 1,
    toolCalls: [] as Array<{ toolCallId: string; title: string; status: string }>,
  }));
}

test.describe('Chat Panel Scroll Behavior', () => {
  test.setTimeout(60000);

  test.describe('Auto-scroll on tool call updates', () => {
    test.beforeEach(async ({ page }) => {
      await navigateAndWait(page);
      await openChatPanel(page);
    });

    test('should scroll to bottom when a tool call update arrives and user has not scrolled', async ({
      page,
    }) => {
      // Inject enough messages to make the container overflow
      const messages = makeManyMessages(20);
      await injectMessagesWithMap(page, messages);
      await waitForUpdate(page);

      // Confirm the container is actually scrollable
      const before = await getScrollInfo(page);
      expect(before).not.toBeNull();
      expect(before!.scrollHeight).toBeGreaterThan(before!.clientHeight);

      // By default, userHasScrolled is false. Trigger a tool call update on the first message.
      // The component should call scrollToBottom() since userHasScrolled is false.
      await triggerToolCallUpdate(page, 'scroll-msg-0', 'tc-new', 'read_file', 'running');

      // After the update, the container should be scrolled to the bottom
      await expect
        .poll(
          async () => {
            const info = await getScrollInfo(page);
            if (!info) return false;
            const { scrollTop, scrollHeight, clientHeight } = info;
            // Allow 1px tolerance for sub-pixel rounding
            return scrollTop + clientHeight >= scrollHeight - 1;
          },
          { timeout: SCROLL_SETTLE_TIMEOUT_MS },
        )
        .toBe(true);
    });
  });

  test.describe('No auto-scroll when user has scrolled up', () => {
    test.beforeEach(async ({ page }) => {
      await navigateAndWait(page);
      await openChatPanel(page);
    });

    test('should preserve scroll position when user has manually scrolled up', async ({ page }) => {
      // Inject enough messages to make the container overflow
      const messages = makeManyMessages(20);
      await injectMessagesWithMap(page, messages);
      await waitForUpdate(page);

      // Confirm overflow
      const before = await getScrollInfo(page);
      expect(before).not.toBeNull();
      expect(before!.scrollHeight).toBeGreaterThan(before!.clientHeight);

      // Simulate the user scrolling up — sets userHasScrolled = true inside the component
      await simulateUserScrollUp(page);

      // Give Lit a chance to process the scroll event
      await waitForUpdate(page);

      // Scroll position should now be at the top
      const afterUserScroll = await getScrollInfo(page);
      expect(afterUserScroll).not.toBeNull();
      expect(afterUserScroll!.scrollTop).toBe(0);

      // Trigger a tool call update — the component should NOT auto-scroll
      // because userHasScrolled is true
      await triggerToolCallUpdate(page, 'scroll-msg-0', 'tc-preserve', 'write_file', 'running');
      await waitForUpdate(page);

      // Scroll position should remain near the top (not jump to bottom)
      const afterToolCall = await getScrollInfo(page);
      expect(afterToolCall).not.toBeNull();
      const { scrollTop, scrollHeight, clientHeight } = afterToolCall!;
      // Still not at the bottom — user's scroll position is preserved
      expect(scrollTop + clientHeight).toBeLessThan(scrollHeight - 10);
    });
  });
});
