import { Order } from './merchants/types.js';
import { readPage, updatePageContent, createPage } from './wiki-client.js';
import { formatOrderRow, isDuplicate, appendRowsToTable } from './table-formatter.js';

const WIKI_PAGE = 'online_orders';
const DEFAULT_WIKI_URL = 'http://localhost:8050';

let pendingOrders: Order[] = [];

interface OrdersDetectedMessage {
  type: 'ORDERS_DETECTED';
  orders: Order[];
}

interface SaveOrdersMessage {
  type: 'SAVE_ORDERS';
  orders: Order[];
}

interface GetPendingMessage {
  type: 'GET_PENDING';
}

interface DismissMessage {
  type: 'DISMISS';
}

type ExtensionMessage = OrdersDetectedMessage | SaveOrdersMessage | GetPendingMessage | DismissMessage;

async function getWikiUrl(): Promise<string> {
  const stored = await browser.storage.local.get('wikiUrl');
  return (stored['wikiUrl'] as string | undefined) ?? DEFAULT_WIKI_URL;
}

async function saveOrdersToWiki(orders: Order[]): Promise<{ savedCount: number; skippedCount: number }> {
  const wikiUrl = await getWikiUrl();
  let contentMarkdown: string;
  let versionHash: string;
  let pageExists = true;

  try {
    const page = await readPage(wikiUrl, WIKI_PAGE);
    contentMarkdown = page.contentMarkdown;
    versionHash = page.versionHash;
  } catch {
    contentMarkdown = '';
    versionHash = '';
    pageExists = false;
  }

  const newRows: string[] = [];
  let skippedCount = 0;

  for (const order of orders) {
    if (isDuplicate(contentMarkdown, order.orderNumber)) {
      skippedCount++;
      continue;
    }
    newRows.push(formatOrderRow(order));
  }

  if (newRows.length === 0) {
    return { savedCount: 0, skippedCount };
  }

  const updatedMarkdown = appendRowsToTable(contentMarkdown, newRows);

  if (pageExists) {
    await updatePageContent(wikiUrl, WIKI_PAGE, updatedMarkdown, versionHash);
  } else {
    await createPage(wikiUrl, WIKI_PAGE, updatedMarkdown);
  }

  return { savedCount: newRows.length, skippedCount };
}

browser.runtime.onMessage.addListener((
  message: unknown,
  _sender: browser.runtime.MessageSender,
  sendResponse: (response: unknown) => void
): true | undefined => {
  const msg = message as ExtensionMessage;

  switch (msg.type) {
    case 'ORDERS_DETECTED':
      pendingOrders = msg.orders;
      browser.browserAction.setBadgeText({ text: String(pendingOrders.length) });
      browser.browserAction.setBadgeBackgroundColor({ color: '#43a047' });
      return undefined;

    case 'GET_PENDING':
      sendResponse({ orders: pendingOrders });
      return undefined;

    case 'SAVE_ORDERS':
      saveOrdersToWiki(msg.orders)
        .then(result => sendResponse({ success: true, ...result }))
        .catch(err => sendResponse({
          success: false,
          error: err instanceof Error ? err.message : String(err),
        }));
      return true;

    case 'DISMISS':
      pendingOrders = [];
      browser.browserAction.setBadgeText({ text: '' });
      return undefined;

    default:
      return undefined;
  }
});
