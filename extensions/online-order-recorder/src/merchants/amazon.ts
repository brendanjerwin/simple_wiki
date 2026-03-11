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

  const headerItems = card.querySelectorAll('.order-header__header-list > li');
  let orderDate = '';
  let totalCents = 0;

  for (const li of headerItems) {
    const label = li.querySelector('.a-color-secondary')?.textContent?.trim().toLowerCase() ?? '';

    if (label === 'order placed') {
      const dateEl = li.querySelector('.a-text-bold');
      if (dateEl) {
        orderDate = parseAmazonDate(dateEl.textContent?.trim() ?? '');
      }
    } else if (label === 'total') {
      const totalEl = li.querySelector('.a-text-bold');
      if (totalEl) {
        totalCents = parsePriceCents(totalEl.textContent?.trim() ?? '');
      }
    }
  }

  const statusEl = card.querySelector('.delivery-box__primary-text');
  const deliveryStatus = statusEl?.textContent?.trim() ?? '';

  const itemBoxes = card.querySelectorAll('.item-box');
  const items: OrderItem[] = [];

  for (const itemBox of itemBoxes) {
    const titleEl = itemBox.querySelector('.yohtmlc-product-title a');
    const name = titleEl?.textContent?.trim() ?? '';

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
      console.debug('[Simple Wiki Companion] Amazon: parsed order', order.orderNumber, '-', order.items.length, 'items');
      orders.push(order);
    } else {
      console.debug('[Simple Wiki Companion] Amazon: failed to parse an order card');
    }
  }

  return orders;
}
