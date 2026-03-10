import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Order } from '../merchants/types.js';

function formatCentsDisplay(cents: number): string {
  const dollars = Math.floor(cents / 100);
  const remainderCents = cents % 100;
  return `$${dollars}.${String(remainderCents).padStart(2, '0')}`;
}

@customElement('order-list')
export class OrderList extends LitElement {
  static override styles = css`
    :host {
      display: block;
    }

    .order-item {
      display: flex;
      align-items: flex-start;
      gap: 8px;
      padding: 8px 0;
      border-bottom: 1px solid #eee;
    }

    .order-item:last-child {
      border-bottom: none;
    }

    .order-details {
      flex: 1;
    }

    .order-merchant {
      font-weight: 600;
      font-size: 13px;
    }

    .order-number {
      font-size: 11px;
      color: #666;
      font-family: monospace;
    }

    .order-meta {
      font-size: 12px;
      color: #555;
      margin-top: 2px;
    }

    .no-orders {
      color: #888;
      font-style: italic;
      padding: 16px 0;
      text-align: center;
    }

    .actions {
      margin-top: 12px;
      display: flex;
      gap: 8px;
    }

    button {
      padding: 6px 16px;
      border: 1px solid #ccc;
      border-radius: 4px;
      cursor: pointer;
      font-size: 13px;
      background: #fff;
    }

    button.primary {
      background: #43a047;
      color: white;
      border-color: #388e3c;
    }

    button:hover {
      opacity: 0.9;
    }

    button:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }
  `;

  @property({ attribute: false })
  declare orders: Order[];

  @state()
  declare checkedIndices: Set<number>;

  @state()
  declare saving: boolean;

  constructor() {
    super();
    this.orders = [];
    this.checkedIndices = new Set();
    this.saving = false;
  }

  override updated(changedProps: Map<string, unknown>): void {
    if (changedProps.has('orders')) {
      this.checkedIndices = new Set(this.orders.map((_, i) => i));
    }
  }

  private handleCheck(index: number, checked: boolean): void {
    const next = new Set(this.checkedIndices);
    if (checked) {
      next.add(index);
    } else {
      next.delete(index);
    }
    this.checkedIndices = next;
  }

  private handleSave(): void {
    const selectedOrders = this.orders.filter((_, i) => this.checkedIndices.has(i));
    this.dispatchEvent(new CustomEvent('orders-selected', {
      detail: { orders: selectedOrders },
      bubbles: true,
      composed: true,
    }));
  }

  private handleDismiss(): void {
    this.dispatchEvent(new CustomEvent('orders-dismissed', {
      bubbles: true,
      composed: true,
    }));
  }

  override render() {
    if (this.orders.length === 0) {
      return html`<div class="no-orders">No orders detected on this page.</div>`;
    }

    return html`
      ${this.orders.map((order, i) => html`
        <div class="order-item">
          <input
            type="checkbox"
            .checked=${this.checkedIndices.has(i)}
            @change=${(e: Event) => this.handleCheck(i, (e.target as HTMLInputElement).checked)}
            ?disabled=${this.saving}
          />
          <div class="order-details">
            <div class="order-merchant">${order.merchant}</div>
            <div class="order-number">${order.orderNumber}</div>
            <div class="order-meta">
              ${order.orderDate} &middot; ${order.items.length} item${order.items.length !== 1 ? 's' : ''} &middot; ${formatCentsDisplay(order.totalCents)}
            </div>
          </div>
        </div>
      `)}
      <div class="actions">
        <button class="primary" @click=${this.handleSave} ?disabled=${this.saving || this.checkedIndices.size === 0}>
          ${this.saving ? 'Saving...' : 'Save to Wiki'}
        </button>
        <button @click=${this.handleDismiss} ?disabled=${this.saving}>Dismiss</button>
      </div>
    `;
  }
}
