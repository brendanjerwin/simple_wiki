import { Order } from './merchants/types.js';

const TABLE_HEADER = '| Date | Merchant | Order # | Items | Prices | Total | Status |';
const TABLE_SEPARATOR = '|------|----------|---------|-------|--------|-------|--------|';

export function formatCentsAsDollars(cents: number): string {
  const dollars = Math.floor(cents / 100);
  const remainderCents = cents % 100;
  return `$${dollars}.${String(remainderCents).padStart(2, '0')}`;
}

export function formatOrderRow(order: Order): string {
  const itemNames = order.items
    .map(item => item.quantity > 1 ? `${item.name} x${item.quantity}` : item.name)
    .join('; ');

  const itemPrices = order.items
    .map(item => formatCentsAsDollars(item.priceCents))
    .join('; ');

  const total = formatCentsAsDollars(order.totalCents);

  return `| ${order.orderDate} | ${order.merchant} | ${order.orderNumber} | ${itemNames} | ${itemPrices} | ${total} | ${order.deliveryStatus} |`;
}

export function isDuplicate(markdown: string, orderNumber: string): boolean {
  return markdown.includes(orderNumber);
}

export function appendRowsToTable(existingMarkdown: string, newRows: string[]): string {
  if (newRows.length === 0) {
    return existingMarkdown;
  }

  const trimmed = existingMarkdown.trim();

  if (trimmed === '') {
    return [TABLE_HEADER, TABLE_SEPARATOR, ...newRows].join('\n') + '\n';
  }

  if (trimmed.includes(TABLE_HEADER)) {
    const trailing = existingMarkdown.endsWith('\n') ? '' : '\n';
    return existingMarkdown + trailing + newRows.join('\n') + '\n';
  }

  const separator = trimmed.length > 0 ? '\n\n' : '';
  return existingMarkdown + separator + [TABLE_HEADER, TABLE_SEPARATOR, ...newRows].join('\n') + '\n';
}
