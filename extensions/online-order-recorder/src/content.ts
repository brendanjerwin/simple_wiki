import { parseOrders } from './merchants/amazon.js';
import { Order } from './merchants/types.js';

console.debug('[Simple Wiki Companion] Content script running on', globalThis.location.href);

function detectAndParse(): Order[] {
  const hostname = globalThis.location.hostname;

  if (hostname.endsWith('.amazon.com') || hostname === 'amazon.com') {
    console.debug('[Simple Wiki Companion] Amazon page detected, parsing orders...');
    const orders = parseOrders(document);
    console.debug('[Simple Wiki Companion] Found', orders.length, 'orders');
    return orders;
  }

  return [];
}

const orders = detectAndParse();

if (orders.length > 0) {
  console.debug('[Simple Wiki Companion] Sending', orders.length, 'orders to background script');
  void browser.runtime.sendMessage({
    type: 'ORDERS_DETECTED',
    orders,
  });
}
