/** @vitest-environment jsdom */

import { describe, it, expect, beforeEach } from 'vitest';
import { Order } from '../merchants/types.js';
import type { OrderList } from './order-list.js';

function makeOrder(overrides: Partial<Order> = {}): Order {
  return {
    merchant: 'TestMerchant',
    orderNumber: '123-456',
    orderDate: '2026-01-15',
    items: [{ name: 'Widget', priceCents: 999, quantity: 1 }],
    totalCents: 999,
    deliveryStatus: 'Delivered',
    ...overrides,
  };
}

async function createEl(): Promise<OrderList> {
  await import('./order-list.js');
  const el = document.createElement('order-list') as OrderList;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

function shadowRoot(el: OrderList): ShadowRoot {
  const root = el.shadowRoot;
  if (!root) {
    throw new Error('shadowRoot is null');
  }
  return root;
}

describe('OrderList', () => {

  let el: OrderList;

  beforeEach(async () => {
    document.body.innerHTML = '';
  });

  it('should exist', async () => {
    el = await createEl();
    expect(el).to.be.instanceOf(HTMLElement);
  });

  describe('when orders is empty', () => {

    beforeEach(async () => {
      el = await createEl();
    });

    it('should render the no-orders message', () => {
      const noOrders = shadowRoot(el).querySelector('.no-orders');
      expect(noOrders).to.not.be.null;
      expect(noOrders!.textContent).to.equal('No orders detected on this page.');
    });

    it('should not render any order items', () => {
      const items = shadowRoot(el).querySelectorAll('.order-item');
      expect(items.length).to.equal(0);
    });

    it('should not render action buttons', () => {
      const actions = shadowRoot(el).querySelector('.actions');
      expect(actions).to.be.null;
    });
  });

  describe('when orders are provided', () => {

    let orders: Order[];

    beforeEach(async () => {
      orders = [
        makeOrder({ merchant: 'Amazon', orderNumber: 'A-001', totalCents: 2999 }),
        makeOrder({ merchant: 'Target', orderNumber: 'T-002', totalCents: 5, deliveryStatus: '' }),
      ];
      el = await createEl();
      el.orders = orders;
      await el.updateComplete;
    });

    it('should render order items for each order', () => {
      const items = shadowRoot(el).querySelectorAll('.order-item');
      expect(items.length).to.equal(2);
    });

    it('should render the merchant name', () => {
      const merchants = shadowRoot(el).querySelectorAll('.order-merchant');
      expect(merchants[0]!.textContent).to.equal('Amazon');
      expect(merchants[1]!.textContent).to.equal('Target');
    });

    it('should render the order number', () => {
      const numbers = shadowRoot(el).querySelectorAll('.order-number');
      expect(numbers[0]!.textContent).to.equal('A-001');
      expect(numbers[1]!.textContent).to.equal('T-002');
    });

    it('should render item names in the item list', () => {
      const itemNames = shadowRoot(el).querySelectorAll('.item-name');
      expect(itemNames.length).to.equal(2);
      expect(itemNames[0]!.textContent).to.equal('Widget');
    });

    it('should not render the no-orders message', () => {
      const noOrders = shadowRoot(el).querySelector('.no-orders');
      expect(noOrders).to.be.null;
    });

    it('should render Save to Wiki button', () => {
      const buttons = shadowRoot(el).querySelectorAll('button');
      expect(buttons[0]!.textContent!.trim()).to.equal('Save to Wiki');
    });

    it('should render Dismiss button', () => {
      const buttons = shadowRoot(el).querySelectorAll('button');
      expect(buttons[1]!.textContent!.trim()).to.equal('Dismiss');
    });
  });

  describe('formatCentsDisplay via rendered output', () => {

    describe('when totalCents is 2999', () => {

      beforeEach(async () => {
        el = await createEl();
        el.orders = [makeOrder({ totalCents: 2999 })];
        await el.updateComplete;
      });

      it('should display $29.99', () => {
        const meta = shadowRoot(el).querySelector('.order-meta');
        expect(meta!.textContent).to.contain('$29.99');
      });
    });

    describe('when totalCents is 5', () => {

      beforeEach(async () => {
        el = await createEl();
        el.orders = [makeOrder({ totalCents: 5 })];
        await el.updateComplete;
      });

      it('should display $0.05', () => {
        const meta = shadowRoot(el).querySelector('.order-meta');
        expect(meta!.textContent).to.contain('$0.05');
      });
    });

    describe('when totalCents is 10000', () => {

      beforeEach(async () => {
        el = await createEl();
        el.orders = [makeOrder({ totalCents: 10000 })];
        await el.updateComplete;
      });

      it('should display $100.00', () => {
        const meta = shadowRoot(el).querySelector('.order-meta');
        expect(meta!.textContent).to.contain('$100.00');
      });
    });
  });

  describe('deliveryStatus in order-meta', () => {

    describe('when deliveryStatus is a non-empty string', () => {

      beforeEach(async () => {
        el = await createEl();
        el.orders = [makeOrder({ deliveryStatus: 'Shipped' })];
        await el.updateComplete;
      });

      it('should include the delivery status in the meta', () => {
        const meta = shadowRoot(el).querySelector('.order-meta');
        expect(meta!.textContent).to.contain('Shipped');
      });
    });

    describe('when deliveryStatus is an empty string', () => {

      beforeEach(async () => {
        el = await createEl();
        el.orders = [makeOrder({ deliveryStatus: '' })];
        await el.updateComplete;
      });

      it('should not include a trailing separator', () => {
        const meta = shadowRoot(el).querySelector('.order-meta');
        const text = meta!.textContent!;
        // Should end with the price, not have an extra middot after it
        expect(text).to.not.contain('Shipped');
      });
    });
  });

  describe('updated() auto-checks all orders', () => {

    describe('when orders are set', () => {

      beforeEach(async () => {
        el = await createEl();
        el.orders = [makeOrder(), makeOrder(), makeOrder()];
        await el.updateComplete;
      });

      it('should check all indices', () => {
        expect(el.checkedIndices.size).to.equal(3);
        expect(el.checkedIndices.has(0)).to.be.true;
        expect(el.checkedIndices.has(1)).to.be.true;
        expect(el.checkedIndices.has(2)).to.be.true;
      });

      it('should render all checkboxes as checked', () => {
        const checkboxes = shadowRoot(el).querySelectorAll<HTMLInputElement>('input[type="checkbox"]');
        expect(checkboxes.length).to.equal(3);
        for (const cb of checkboxes) {
          expect(cb.checked).to.be.true;
        }
      });
    });

    describe('when orders are replaced with a different set', () => {

      beforeEach(async () => {
        el = await createEl();
        el.orders = [makeOrder(), makeOrder(), makeOrder()];
        await el.updateComplete;

        // Replace with a smaller set
        el.orders = [makeOrder()];
        await el.updateComplete;
      });

      it('should reset checked indices to match new orders', () => {
        expect(el.checkedIndices.size).to.equal(1);
        expect(el.checkedIndices.has(0)).to.be.true;
      });
    });
  });

  describe('handleCheck', () => {

    describe('when unchecking an order', () => {

      beforeEach(async () => {
        el = await createEl();
        el.orders = [makeOrder(), makeOrder()];
        await el.updateComplete;

        // Uncheck the first checkbox
        const checkboxes = shadowRoot(el).querySelectorAll<HTMLInputElement>('input[type="checkbox"]');
        checkboxes[0]!.checked = false;
        checkboxes[0]!.dispatchEvent(new Event('change'));
        await el.updateComplete;
      });

      it('should remove the index from checkedIndices', () => {
        expect(el.checkedIndices.has(0)).to.be.false;
      });

      it('should keep other indices checked', () => {
        expect(el.checkedIndices.has(1)).to.be.true;
      });
    });

    describe('when re-checking an unchecked order', () => {

      beforeEach(async () => {
        el = await createEl();
        el.orders = [makeOrder(), makeOrder()];
        await el.updateComplete;

        // Uncheck first
        const checkboxes = shadowRoot(el).querySelectorAll<HTMLInputElement>('input[type="checkbox"]');
        checkboxes[0]!.checked = false;
        checkboxes[0]!.dispatchEvent(new Event('change'));
        await el.updateComplete;

        // Re-check
        const checkboxesAfter = shadowRoot(el).querySelectorAll<HTMLInputElement>('input[type="checkbox"]');
        checkboxesAfter[0]!.checked = true;
        checkboxesAfter[0]!.dispatchEvent(new Event('change'));
        await el.updateComplete;
      });

      it('should add the index back to checkedIndices', () => {
        expect(el.checkedIndices.has(0)).to.be.true;
      });

      it('should have all indices checked', () => {
        expect(el.checkedIndices.size).to.equal(2);
      });
    });
  });

  describe('handleSave', () => {

    describe('when all orders are checked', () => {

      let receivedEvent: CustomEvent | null;

      beforeEach(async () => {
        receivedEvent = null;
        el = await createEl();
        el.orders = [
          makeOrder({ merchant: 'Amazon' }),
          makeOrder({ merchant: 'Target' }),
        ];
        await el.updateComplete;
        // updated() sets checkedIndices which triggers a second render
        await el.updateComplete;

        el.addEventListener('orders-selected', ((e: Event) => {
          receivedEvent = e as CustomEvent;
        }) as EventListener);

        const saveButton = shadowRoot(el).querySelector<HTMLButtonElement>('button.primary');
        saveButton!.click();
      });

      it('should dispatch orders-selected event', () => {
        expect(receivedEvent).to.not.be.null;
      });

      it('should include all orders in event detail', () => {
        expect(receivedEvent!.detail.orders).to.have.length(2);
      });

      it('should include the correct order merchants', () => {
        expect(receivedEvent!.detail.orders[0].merchant).to.equal('Amazon');
        expect(receivedEvent!.detail.orders[1].merchant).to.equal('Target');
      });

      it('should dispatch a bubbling event', () => {
        expect(receivedEvent!.bubbles).to.be.true;
      });

      it('should dispatch a composed event', () => {
        expect(receivedEvent!.composed).to.be.true;
      });
    });

    describe('when some orders are unchecked', () => {

      let receivedEvent: CustomEvent | null;

      beforeEach(async () => {
        receivedEvent = null;
        el = await createEl();
        el.orders = [
          makeOrder({ merchant: 'Amazon' }),
          makeOrder({ merchant: 'Target' }),
          makeOrder({ merchant: 'Walmart' }),
        ];
        await el.updateComplete;

        // Uncheck the second order (index 1)
        const checkboxes = shadowRoot(el).querySelectorAll<HTMLInputElement>('input[type="checkbox"]');
        checkboxes[1]!.checked = false;
        checkboxes[1]!.dispatchEvent(new Event('change'));
        await el.updateComplete;

        el.addEventListener('orders-selected', ((e: Event) => {
          receivedEvent = e as CustomEvent;
        }) as EventListener);

        const saveButton = shadowRoot(el).querySelector<HTMLButtonElement>('button.primary');
        saveButton!.click();
      });

      it('should only include checked orders', () => {
        expect(receivedEvent!.detail.orders).to.have.length(2);
      });

      it('should include first and third orders', () => {
        expect(receivedEvent!.detail.orders[0].merchant).to.equal('Amazon');
        expect(receivedEvent!.detail.orders[1].merchant).to.equal('Walmart');
      });
    });
  });

  describe('handleDismiss', () => {

    describe('when dismiss button is clicked', () => {

      let receivedEvent: CustomEvent | null;

      beforeEach(async () => {
        receivedEvent = null;
        el = await createEl();
        el.orders = [makeOrder()];
        await el.updateComplete;

        el.addEventListener('orders-dismissed', ((e: Event) => {
          receivedEvent = e as CustomEvent;
        }) as EventListener);

        const buttons = shadowRoot(el).querySelectorAll<HTMLButtonElement>('button');
        buttons[1]!.click();
      });

      it('should dispatch orders-dismissed event', () => {
        expect(receivedEvent).to.not.be.null;
      });

      it('should dispatch a bubbling event', () => {
        expect(receivedEvent!.bubbles).to.be.true;
      });

      it('should dispatch a composed event', () => {
        expect(receivedEvent!.composed).to.be.true;
      });
    });
  });

  describe('saving state', () => {

    describe('when saving is true', () => {

      beforeEach(async () => {
        el = await createEl();
        el.orders = [makeOrder()];
        await el.updateComplete;

        el.saving = true;
        await el.updateComplete;
      });

      it('should disable the save button', () => {
        const saveButton = shadowRoot(el).querySelector<HTMLButtonElement>('button.primary');
        expect(saveButton!.disabled).to.be.true;
      });

      it('should show Saving... text on the save button', () => {
        const saveButton = shadowRoot(el).querySelector<HTMLButtonElement>('button.primary');
        expect(saveButton!.textContent!.trim()).to.equal('Saving...');
      });

      it('should disable the dismiss button', () => {
        const buttons = shadowRoot(el).querySelectorAll<HTMLButtonElement>('button');
        expect(buttons[1]!.disabled).to.be.true;
      });

      it('should disable all checkboxes', () => {
        const checkboxes = shadowRoot(el).querySelectorAll<HTMLInputElement>('input[type="checkbox"]');
        for (const cb of checkboxes) {
          expect(cb.disabled).to.be.true;
        }
      });
    });

    describe('when saving is false', () => {

      beforeEach(async () => {
        el = await createEl();
        el.orders = [makeOrder()];
        await el.updateComplete;
      });

      it('should not disable the save button', () => {
        const saveButton = shadowRoot(el).querySelector<HTMLButtonElement>('button.primary');
        expect(saveButton!.disabled).to.be.false;
      });

      it('should show Save to Wiki text on the save button', () => {
        const saveButton = shadowRoot(el).querySelector<HTMLButtonElement>('button.primary');
        expect(saveButton!.textContent!.trim()).to.equal('Save to Wiki');
      });

      it('should not disable the dismiss button', () => {
        const buttons = shadowRoot(el).querySelectorAll<HTMLButtonElement>('button');
        expect(buttons[1]!.disabled).to.be.false;
      });

      it('should not disable checkboxes', () => {
        const checkboxes = shadowRoot(el).querySelectorAll<HTMLInputElement>('input[type="checkbox"]');
        for (const cb of checkboxes) {
          expect(cb.disabled).to.be.false;
        }
      });
    });
  });

  describe('save button disabled state', () => {

    describe('when no orders are checked', () => {

      beforeEach(async () => {
        el = await createEl();
        el.orders = [makeOrder()];
        await el.updateComplete;

        // Uncheck the only order
        const checkboxes = shadowRoot(el).querySelectorAll<HTMLInputElement>('input[type="checkbox"]');
        checkboxes[0]!.checked = false;
        checkboxes[0]!.dispatchEvent(new Event('change'));
        await el.updateComplete;
      });

      it('should disable the save button', () => {
        const saveButton = shadowRoot(el).querySelector<HTMLButtonElement>('button.primary');
        expect(saveButton!.disabled).to.be.true;
      });
    });
  });
});
