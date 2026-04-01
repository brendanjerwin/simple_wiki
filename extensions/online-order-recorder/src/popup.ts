import './components/settings-panel.js';
import { Order } from './merchants/types.js';
import { OrderList } from './components/order-list.js';

const statusEl = document.getElementById('status');
const orderListEl = document.querySelector('order-list') as OrderList | null;

function setStatus(text: string, type: 'error' | 'success' | ''): void {
  if (!statusEl) {
    return;
  }
  statusEl.textContent = text;
  statusEl.className = type;
}

async function loadPendingOrders(): Promise<void> {
  const response = await browser.runtime.sendMessage({ type: 'GET_PENDING' }) as { orders: Order[] };
  if (orderListEl && response.orders) {
    orderListEl.orders = response.orders;
  }
}

document.addEventListener('orders-selected', async (e: Event) => {
  const detail = (e as CustomEvent<{ orders: Order[] }>).detail;
  if (orderListEl) {
    orderListEl.saving = true;
  }
  setStatus('Saving orders...', '');

  const response = await browser.runtime.sendMessage({
    type: 'SAVE_ORDERS',
    orders: detail.orders,
  }) as { success: boolean; savedCount?: number; skippedCount?: number; error?: string };

  if (orderListEl) {
    orderListEl.saving = false;
  }

  if (response.success) {
    const parts: string[] = [];
    if (response.savedCount) {
      parts.push(`${response.savedCount} saved`);
    }
    if (response.skippedCount) {
      parts.push(`${response.skippedCount} duplicates skipped`);
    }
    setStatus(parts.join(', ') || 'Done', 'success');
    await browser.runtime.sendMessage({ type: 'DISMISS' });
    if (orderListEl) {
      orderListEl.orders = [];
    }
  } else {
    setStatus(response.error ?? 'Save failed', 'error');
  }
});

document.addEventListener('orders-dismissed', async () => {
  await browser.runtime.sendMessage({ type: 'DISMISS' });
  if (orderListEl) {
    orderListEl.orders = [];
  }
  setStatus('Dismissed', '');
});

loadPendingOrders();
