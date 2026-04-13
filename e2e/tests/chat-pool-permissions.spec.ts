import { test, expect, type Page } from '@playwright/test';

// Timeouts
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PANEL_INTERACTION_TIMEOUT_MS = 5000;

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
  value: unknown,
): Promise<void> {
  await page.evaluate(
    ([p, v]) => {
      return customElements.whenDefined('page-chat-panel').then(() => {
        const el = document.querySelector('page-chat-panel');
        if (el) (el as HTMLElement & Record<string, unknown>)[p] = v;
      });
    },
    [prop, value] as [string, unknown],
  );
}

/**
 * Set the messages array on page-chat-panel, handling BigInt sequence fields
 * which cannot cross the Playwright serialization boundary.
 */
async function setChatPanelMessages(
  page: Page,
  messages: Array<{
    id: string;
    sender: number;
    content: string;
    renderedHtml: string;
    senderName: string;
    replyToId: string;
    edited: boolean;
    sequence: number;
    toolCalls: Array<{ toolCallId: string; title: string; status: string }>;
  }>,
): Promise<void> {
  await page.evaluate(
    (msgs) => {
      return customElements.whenDefined('page-chat-panel').then(() => {
        const el = document.querySelector('page-chat-panel');
        if (!el) return;
        const typedEl = el as HTMLElement & Record<string, unknown>;
        // Convert sequence numbers to BigInt inside the browser context
        typedEl['messages'] = msgs.map((m) => ({
          ...m,
          timestamp: new Date(),
          reactions: [],
          sequence: BigInt(m.sequence),
        }));
      });
    },
    messages,
  );
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

test.describe('chat-pool: Chat Pool and Permissions', () => {
  test.setTimeout(60000);

  test.describe('Chat panel status states', () => {
    test.beforeEach(async ({ page }) => {
      await navigateAndWait(page);
    });

    test('should show FAB as disabled when neither pool nor agent is connected', async ({ page }) => {
      const fab = page.locator('page-chat-panel .fab');
      await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(fab).toHaveClass(/disabled/, { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should show FAB without disabled class when poolConnected is true', async ({ page }) => {
      await setChatPanelProperty(page, 'poolConnected', true);
      await waitForUpdate(page);

      const fab = page.locator('page-chat-panel .fab');
      await expect(fab).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(fab).not.toHaveClass(/disabled/, { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should show "Send a message to start" banner when pool is connected but agent is not', async ({ page }) => {
      await setChatPanelProperty(page, 'poolConnected', true);
      await waitForUpdate(page);

      await openChatPanel(page);

      const banner = page.locator('page-chat-panel .status-banner');
      await expect(banner).toContainText('Send a message to start', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should enable textarea when poolConnected is true', async ({ page }) => {
      await setChatPanelProperty(page, 'poolConnected', true);
      await waitForUpdate(page);

      await openChatPanel(page);

      await expect(page.locator('page-chat-panel textarea')).toBeEnabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should enable send button when poolConnected is true', async ({ page }) => {
      await setChatPanelProperty(page, 'poolConnected', true);
      await waitForUpdate(page);

      await openChatPanel(page);

      await expect(page.locator('page-chat-panel .send-button')).toBeEnabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should enable textarea when agentConnected is true', async ({ page }) => {
      await openChatPanel(page);
      await setChatPanelProperty(page, 'agentConnected', true);
      await waitForUpdate(page);

      await expect(page.locator('page-chat-panel textarea')).toBeEnabled({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should show "Starting assistant..." banner when agentStarting is true', async ({ page }) => {
      await setChatPanelProperty(page, 'agentStarting', true);
      await waitForUpdate(page);

      await openChatPanel(page);

      const banner = page.locator('page-chat-panel .status-banner.reconnecting');
      await expect(banner).toContainText('Starting assistant', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });
  });

  test.describe('Permission prompt rendering', () => {
    test.beforeEach(async ({ page }) => {
      await navigateAndWait(page);
      await openChatPanel(page);
    });

    test('should render permission prompt when pendingPermission is set', async ({ page }) => {
      await setChatPanelProperty(page, 'pendingPermission', {
        requestId: 'perm-1',
        title: 'File Access',
        description: 'Allow reading /etc/config?',
        options: [
          { optionId: 'allow', label: 'Allow', description: 'Grant read access' },
          { optionId: 'allow-once', label: 'Allow Once', description: 'Grant one-time access' },
        ],
      });
      await waitForUpdate(page);

      const prompt = page.locator('page-chat-panel .permission-prompt');
      await expect(prompt).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should show permission title text', async ({ page }) => {
      await setChatPanelProperty(page, 'pendingPermission', {
        requestId: 'perm-2',
        title: 'File Access',
        description: 'Allow reading /etc/config?',
        options: [
          { optionId: 'allow', label: 'Allow', description: 'Grant read access' },
        ],
      });
      await waitForUpdate(page);

      const titleEl = page.locator('page-chat-panel .permission-title');
      await expect(titleEl).toContainText('Permission requested', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should show permission description with title and description text', async ({ page }) => {
      await setChatPanelProperty(page, 'pendingPermission', {
        requestId: 'perm-3',
        title: 'File Access',
        description: 'Allow reading /etc/config?',
        options: [
          { optionId: 'allow', label: 'Allow', description: 'Grant read access' },
        ],
      });
      await waitForUpdate(page);

      const descEl = page.locator('page-chat-panel .permission-description');
      await expect(descEl).toContainText('File Access', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(descEl).toContainText('Allow reading /etc/config?', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should render option buttons for each permission option', async ({ page }) => {
      await setChatPanelProperty(page, 'pendingPermission', {
        requestId: 'perm-4',
        title: 'Tool Use',
        description: 'Run shell command?',
        options: [
          { optionId: 'allow', label: 'Allow', description: 'Grant access' },
          { optionId: 'allow-once', label: 'Allow Once', description: 'One-time access' },
        ],
      });
      await waitForUpdate(page);

      const optionBtns = page.locator('page-chat-panel .permission-btn:not(.cancel)');
      await expect(optionBtns).toHaveCount(2, { timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(optionBtns.nth(0)).toHaveText('Allow');
      await expect(optionBtns.nth(1)).toHaveText('Allow Once');
    });

    test('should render Deny button', async ({ page }) => {
      await setChatPanelProperty(page, 'pendingPermission', {
        requestId: 'perm-5',
        title: 'Tool Use',
        description: 'Run shell command?',
        options: [
          { optionId: 'allow', label: 'Allow', description: 'Grant access' },
        ],
      });
      await waitForUpdate(page);

      const denyBtn = page.locator('page-chat-panel .permission-btn.cancel');
      await expect(denyBtn).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(denyBtn).toHaveText('Deny');
    });

    test('should clear permission prompt when an option button is clicked', async ({ page }) => {
      // Intercept the RespondToPermission gRPC call so the click handler completes
      await page.route('**/*RespondToPermission*', (route) => {
        void route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.alloc(0),
        });
      });

      await setChatPanelProperty(page, 'pendingPermission', {
        requestId: 'perm-6',
        title: 'Tool Use',
        description: 'Run shell command?',
        options: [
          { optionId: 'allow', label: 'Allow', description: 'Grant access' },
        ],
      });
      await waitForUpdate(page);

      const optionBtn = page.locator('page-chat-panel .permission-btn:not(.cancel)').first();
      await optionBtn.click();

      await expect(page.locator('page-chat-panel .permission-prompt')).not.toBeAttached({
        timeout: PANEL_INTERACTION_TIMEOUT_MS,
      });
    });

    test('should clear permission prompt when Deny button is clicked', async ({ page }) => {
      await page.route('**/*RespondToPermission*', (route) => {
        void route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.alloc(0),
        });
      });

      await setChatPanelProperty(page, 'pendingPermission', {
        requestId: 'perm-7',
        title: 'Tool Use',
        description: 'Run shell command?',
        options: [
          { optionId: 'allow', label: 'Allow', description: 'Grant access' },
        ],
      });
      await waitForUpdate(page);

      const denyBtn = page.locator('page-chat-panel .permission-btn.cancel');
      await denyBtn.click();

      await expect(page.locator('page-chat-panel .permission-prompt')).not.toBeAttached({
        timeout: PANEL_INTERACTION_TIMEOUT_MS,
      });
    });

    test('should send selectedOptionId when an option is clicked', async ({ page }) => {
      let capturedBody: ArrayBuffer | null = null;

      await page.route('**/*RespondToPermission*', (route) => {
        capturedBody = route.request().postDataBuffer() ?? null;
        void route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.alloc(0),
        });
      });

      await setChatPanelProperty(page, 'pendingPermission', {
        requestId: 'perm-8',
        title: 'Tool Use',
        description: 'Run shell command?',
        options: [
          { optionId: 'allow', label: 'Allow', description: 'Grant access' },
        ],
      });
      await waitForUpdate(page);

      const optionBtn = page.locator('page-chat-panel .permission-btn:not(.cancel)').first();

      const requestPromise = page.waitForRequest(
        (req) => req.url().includes('RespondToPermission'),
        { timeout: PANEL_INTERACTION_TIMEOUT_MS },
      );

      await optionBtn.click();
      await requestPromise;

      expect(capturedBody).not.toBeNull();
    });

    test('should send empty selectedOptionId when Deny is clicked', async ({ page }) => {
      let requestMade = false;

      await page.route('**/*RespondToPermission*', (route) => {
        requestMade = true;
        void route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.alloc(0),
        });
      });

      await setChatPanelProperty(page, 'pendingPermission', {
        requestId: 'perm-9',
        title: 'Tool Use',
        description: 'Run shell command?',
        options: [
          { optionId: 'allow', label: 'Allow', description: 'Grant access' },
        ],
      });
      await waitForUpdate(page);

      const denyBtn = page.locator('page-chat-panel .permission-btn.cancel');

      const requestPromise = page.waitForRequest(
        (req) => req.url().includes('RespondToPermission'),
        { timeout: PANEL_INTERACTION_TIMEOUT_MS },
      );

      await denyBtn.click();
      await requestPromise;

      expect(requestMade).toBe(true);
    });
  });

  test.describe('Tool call pills', () => {
    test.beforeEach(async ({ page }) => {
      await navigateAndWait(page);
      await openChatPanel(page);
    });

    test('should render tool call pills on a message with tool calls', async ({ page }) => {
      await setChatPanelProperty(page, 'agentConnected', true);
      await setChatPanelMessages(page, [
        {
          id: 'msg-1',
          sender: 1, // Sender.ASSISTANT
          content: 'Running a tool...',
          renderedHtml: '<p>Running a tool...</p>',
          senderName: 'Bot',
          replyToId: '',
          edited: false,
          sequence: 1,
          toolCalls: [
            { toolCallId: 'tc-1', title: 'read_file', status: 'running' },
            { toolCallId: 'tc-2', title: 'write_file', status: 'complete' },
          ],
        },
      ]);
      await waitForUpdate(page);

      const pills = page.locator('page-chat-panel chat-message-bubble .tool-call-pill');
      await expect(pills).toHaveCount(2, { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should show hourglass icon for running tool calls', async ({ page }) => {
      await setChatPanelProperty(page, 'agentConnected', true);
      await setChatPanelMessages(page, [
        {
          id: 'msg-2',
          sender: 1,
          content: 'Running...',
          renderedHtml: '<p>Running...</p>',
          senderName: 'Bot',
          replyToId: '',
          edited: false,
          sequence: 1,
          toolCalls: [
            { toolCallId: 'tc-1', title: 'read_file', status: 'running' },
          ],
        },
      ]);
      await waitForUpdate(page);

      const statusIcon = page.locator('page-chat-panel chat-message-bubble .tool-call-pill .status-icon');
      await expect(statusIcon).toContainText('\u23F3', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should show check mark icon for complete tool calls', async ({ page }) => {
      await setChatPanelProperty(page, 'agentConnected', true);
      await setChatPanelMessages(page, [
        {
          id: 'msg-3',
          sender: 1,
          content: 'Done.',
          renderedHtml: '<p>Done.</p>',
          senderName: 'Bot',
          replyToId: '',
          edited: false,
          sequence: 1,
          toolCalls: [
            { toolCallId: 'tc-1', title: 'read_file', status: 'complete' },
          ],
        },
      ]);
      await waitForUpdate(page);

      const statusIcon = page.locator('page-chat-panel chat-message-bubble .tool-call-pill .status-icon');
      await expect(statusIcon).toContainText('\u2705', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should show cross mark icon for error tool calls', async ({ page }) => {
      await setChatPanelProperty(page, 'agentConnected', true);
      await setChatPanelMessages(page, [
        {
          id: 'msg-4',
          sender: 1,
          content: 'Error occurred.',
          renderedHtml: '<p>Error occurred.</p>',
          senderName: 'Bot',
          replyToId: '',
          edited: false,
          sequence: 1,
          toolCalls: [
            { toolCallId: 'tc-1', title: 'write_file', status: 'error' },
          ],
        },
      ]);
      await waitForUpdate(page);

      const statusIcon = page.locator('page-chat-panel chat-message-bubble .tool-call-pill .status-icon');
      await expect(statusIcon).toContainText('\u274C', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should display tool call title in the pill', async ({ page }) => {
      await setChatPanelProperty(page, 'agentConnected', true);
      await setChatPanelMessages(page, [
        {
          id: 'msg-5',
          sender: 1,
          content: 'Working...',
          renderedHtml: '<p>Working...</p>',
          senderName: 'Bot',
          replyToId: '',
          edited: false,
          sequence: 1,
          toolCalls: [
            { toolCallId: 'tc-1', title: 'search_code', status: 'running' },
          ],
        },
      ]);
      await waitForUpdate(page);

      const pill = page.locator('page-chat-panel chat-message-bubble .tool-call-pill');
      await expect(pill).toContainText('search_code', { timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });
  });

  test.describe('Streaming edits', () => {
    test.beforeEach(async ({ page }) => {
      await navigateAndWait(page);
      await openChatPanel(page);
      await setChatPanelProperty(page, 'agentConnected', true);
    });

    test('should not show "(edited)" indicator when message edited flag is false', async ({ page }) => {
      await setChatPanelMessages(page, [
        {
          id: 'msg-stream-1',
          sender: 1,
          content: 'Streaming content...',
          renderedHtml: '<p>Streaming content...</p>',
          senderName: 'Bot',
          replyToId: '',
          edited: false,
          sequence: 1,
          toolCalls: [],
        },
      ]);
      await waitForUpdate(page);

      const editedIndicator = page.locator('page-chat-panel chat-message-bubble .edited-indicator');
      await expect(editedIndicator).not.toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
    });

    test('should show "(edited)" indicator when message edited flag is true', async ({ page }) => {
      await setChatPanelMessages(page, [
        {
          id: 'msg-stream-2',
          sender: 1,
          content: 'Final content after edit.',
          renderedHtml: '<p>Final content after edit.</p>',
          senderName: 'Bot',
          replyToId: '',
          edited: true,
          sequence: 1,
          toolCalls: [],
        },
      ]);
      await waitForUpdate(page);

      const editedIndicator = page.locator('page-chat-panel chat-message-bubble .edited-indicator');
      await expect(editedIndicator).toBeAttached({ timeout: PANEL_INTERACTION_TIMEOUT_MS });
      await expect(editedIndicator).toContainText('(edited)');
    });
  });
});
