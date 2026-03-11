import { Order, OrderItem } from './types.js';

const ORDER_NUMBER_PATTERN = /\d{3}-\d{7}-\d{7}/;

const MONTH_MAP: Record<string, string> = {
  'January': '01',
  'February': '02',
  'March': '03',
  'April': '04',
  'May': '05',
  'June': '06',
  'July': '07',
  'August': '08',
  'September': '09',
  'October': '10',
  'November': '11',
  'December': '12',
};

function parseAmazonDate(dateText: string): string {
  const match = dateText.trim().match(/^(\w+)\s+(\d{1,2}),\s+(\d{4})$/);
  if (!match) {
    return '';
  }

  const monthName = match[1]!;
  const day = match[2]!;
  const year = match[3]!;
  const month = MONTH_MAP[monthName];

  if (!month) {
    return '';
  }

  return `${year}-${month}-${day.padStart(2, '0')}`;
}

function parsePriceCents(priceText: string): number {
  const match = priceText.trim().match(/\$(\d+(?:,\d{3})*)\.(\d{2})/);
  if (!match) {
    return 0;
  }

  const dollars = parseInt(match[1]!.replace(/,/g, ''), 10);
  const cents = parseInt(match[2]!, 10);
  return dollars * 100 + cents;
}

// Finds a header value by its label text (e.g. "Order placed", "Total").
// Amazon nests these in .order-header__header-list-item elements with a
// label span (.a-text-caps) and a value span (.aok-break-word or .a-text-bold).
function findHeaderValue(card: Element, labelText: string): string {
  const headerItems = card.querySelectorAll('.order-header__header-list-item, .order-header__header-list > li');

  for (const item of headerItems) {
    const label = item.querySelector('.a-color-secondary')?.textContent?.trim().toLowerCase() ?? '';
    if (label.includes(labelText.toLowerCase())) {
      // Try new Amazon DOM first (.aok-break-word), then old (.a-text-bold)
      const valueEl = item.querySelector('.aok-break-word') ?? item.querySelector('.a-text-bold');
      return valueEl?.textContent?.trim() ?? '';
    }
  }

  return '';
}

function parseOrderCard(card: Element): Order | null {
  const orderIdEl = card.querySelector('.yohtmlc-order-id span[dir="ltr"]');
  if (!orderIdEl) {
    return null;
  }

  const orderIdText = orderIdEl.textContent?.trim() ?? '';
  const orderNumberMatch = orderIdText.match(ORDER_NUMBER_PATTERN);
  if (!orderNumberMatch) {
    return null;
  }
  const orderNumber = orderNumberMatch[0];

  const orderDate = parseAmazonDate(findHeaderValue(card, 'order placed'));
  const totalCents = parsePriceCents(findHeaderValue(card, 'total'));

  console.debug('[Simple Wiki Companion] Amazon: order', orderNumber, 'date:', orderDate, 'total:', totalCents);

  const statusEl = card.querySelector('.delivery-box__primary-text');
  const deliveryStatus = statusEl?.textContent?.trim() ?? '';

  const itemBoxes = card.querySelectorAll('.item-box');
  const items: OrderItem[] = [];

  for (const itemBox of itemBoxes) {
    const titleEl = itemBox.querySelector('.yohtmlc-product-title a') ?? itemBox.querySelector('.yohtmlc-product-title');
    const name = titleEl?.textContent?.trim() ?? '';

    // Individual item prices aren't shown on the order list page,
    // so we use 0 for per-item price. The order total is captured above.
    const priceEl = itemBox.querySelector('.a-price .a-offscreen');
    const priceCents = parsePriceCents(priceEl?.textContent?.trim() ?? '');

    if (name) {
      items.push({ name, priceCents, quantity: 1 });
    }
  }

  if (items.length === 0) {
    return null;
  }

  return {
    merchant: 'Amazon',
    orderNumber,
    orderDate,
    items,
    totalCents,
    deliveryStatus,
  };
}

export function parseOrders(doc: Document): Order[] {
  const orderCards = doc.querySelectorAll('.order-card.js-order-card');
  console.debug('[Simple Wiki Companion] Amazon: found', orderCards.length, 'order cards');
  const orders: Order[] = [];

  for (const card of orderCards) {
    const order = parseOrderCard(card);
    if (order) {
      console.debug('[Simple Wiki Companion] Amazon: parsed order', order.orderNumber, '-', order.items.length, 'items, total:', order.totalCents);
      orders.push(order);
    } else {
      console.debug('[Simple Wiki Companion] Amazon: failed to parse an order card');
    }
  }

  return orders;
}
