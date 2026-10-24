import { test, expect, type Page } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers.js';

// E2E: the page app↔agent channel (issue #1272 W3).
// @ToAssistant in a program delivers a chat message on the page via
// ChatService.SendMessage — asserted against the REAL observable: the
// page-chat-panel (present on every /view) renders the page's chat
// history once its drawer is opened (the panel subscribes to the chat
// stream only when opened — messages = [] before that). The agent-reply→
// hot-swap half rides the same SetPageApp→Watch path covered by
// page-app-render-lang's hot-swap test; this spec pins the chat DOOR
// end-to-end through the real transport.

const TEST_PAGE = 'e2e_page app_toassistant';

const PROGRAM = [
 '$n = 0',
 'ask = Button("ask the assistant", @ToAssistant("please bump the counter"))',
 'root = Card("Counter", Stack([ask, Text("count: " + $n)]))',
 '',
].join('\n');

async function openChatPanel(page: Page): Promise<void> {
 const fab = page.locator('page-chat-panel .fab');
 await fab.click({ timeout: 15_000 });
 await expect(page.locator('page-chat-panel .panel.open')).toBeAttached({ timeout: 15_000 });
}

test.describe.serial('page app-lang-toassistant', () => {
 test.beforeEach(async ({ request }) => {
  await setupPage(request, TEST_PAGE, 'Prose about the counter app.\n\n```wiki-page-app\n' + PROGRAM + '```\n');
 });

 test.afterEach(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE }).catch(() => { });
 });

 test('a ToAssistant button press issues ChatService SendMessage on the page', async ({ page }) => {
  await page.goto(`/${TEST_PAGE}/view`);

  // Open the chat drawer first (the panel subscribes to the chat stream
  // only when opened) — keeps the flow in the real user path.
  await openChatPanel(page);

  // Wait for the page app to render its button inside the host shadow DOM.
  await expect
   .poll(async () =>
    page.evaluate(() => {
     const el = document.querySelector('wiki-page-app-host');
     return el?.shadowRoot?.querySelector('button.wiki-page-app-button')?.textContent ?? '';
    }),
    { timeout: 15_000 }
   ).toContain('ask the assistant');

  // The deterministic observable: the SendMessage RPC the host issues
  // (a server echo depends on the pool daemon, which e2e doesn't run).
  const sentPromise = page.waitForRequest(
   (r) => r.url().includes('/api.v1.ChatService/SendMessage'),
   { timeout: 15_000 },
  );
  await page.evaluate(() => {
   const el = document.querySelector('wiki-page-app-host');
   const btn = el?.shadowRoot?.querySelector<HTMLButtonElement>('button.wiki-page-app-button');
   btn?.click();
  });
  const sent = await sentPromise;
  // Connect-JSON POST bodies may be framed streaming (length-prefixed);
  // assert containment rather than postDataJSON().
  const raw = sent.postData() ?? '';
  expect(raw).toContain(TEST_PAGE);
  expect(raw).toContain('please bump the counter');
 });
});