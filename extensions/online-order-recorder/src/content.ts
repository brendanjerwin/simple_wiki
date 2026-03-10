import { parseOrders } from './merchants/amazon.js';
import { Order } from './merchants/types.js';

function detectAndParse(): Order[] {
  const hostname = window.location.hostname;

  if (hostname.endsWith('.amazon.com') || hostname === 'amazon.com') {
    return parseOrders(document);
  }

  return [];
}

const orders = detectAndParse();

if (orders.length > 0) {
  browser.runtime.sendMessage({
    type: 'ORDERS_DETECTED',
    orders,
  });
}
