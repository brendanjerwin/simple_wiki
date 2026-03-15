import { parseOrders } from './merchants/amazon.js';
import { Order } from './merchants/types.js';

console.debug('[Simple Wiki Companion] Content script running on', window.location.href);

function detectAndParse(): Order[] {
  const hostname = window.location.hostname;

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
  browser.runtime.sendMessage({
    type: 'ORDERS_DETECTED',
    orders,
  });
}
