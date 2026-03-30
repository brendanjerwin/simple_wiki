import { describe, it, expect, beforeEach, vi } from 'vitest';
import { JSDOM } from 'jsdom';
import { Order } from './merchants/types.js';

vi.mock('./components/order-list.js', () => ({ OrderList: class {} }));
vi.mock('./components/settings-panel.js', () => ({ SettingsPanel: class {} }));

async function importPopup(): Promise<void> {
  await import('./popup.js');
  // Wait a microtask for the loadPendingOrders promise to settle
  await new Promise<void>(resolve => { setTimeout(resolve, 0); });
}

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

interface MockOrderListEl {
  orders: Order[];
  saving: boolean;
}

describe('popup', () => {
  let sendMessage: ReturnType<typeof vi.fn>;
  let orderListEl: MockOrderListEl;
  let dom: JSDOM;
  let statusEl: HTMLElement;

  beforeEach(async () => {
    vi.resetModules();

    vi.doMock('./components/order-list.js', () => ({ OrderList: class {} }));
    vi.doMock('./components/settings-panel.js', () => ({ SettingsPanel: class {} }));

    dom = new JSDOM('<!DOCTYPE html><html><body><div id="status"></div><order-list></order-list></body></html>');

    orderListEl = { orders: [], saving: false };

    // Patch globals so the popup module finds the DOM
    const origGetElementById = dom.window.document.getElementById.bind(dom.window.document);
    const origQuerySelector = dom.window.document.querySelector.bind(dom.window.document);

    (globalThis as Record<string, unknown>)['document'] = new Proxy(dom.window.document, {
      get(target, prop) {
        if (prop === 'getElementById') {
          return origGetElementById;
        }
        if (prop === 'querySelector') {
          return (selector: string) => {
            if (selector === 'order-list') return orderListEl as unknown as Element;
            return origQuerySelector(selector);
          };
        }
        if (prop === 'addEventListener') {
          return target.addEventListener.bind(target);
        }
        if (prop === 'dispatchEvent') {
          return target.dispatchEvent.bind(target);
        }
        const val = (target as unknown as Record<string | symbol, unknown>)[prop];
        if (typeof val === 'function') {
          return val.bind(target);
        }
        return val;
      },
    });

    (globalThis as Record<string, unknown>)['CustomEvent'] = dom.window.CustomEvent;
    (globalThis as Record<string, unknown>)['Event'] = dom.window.Event;

    sendMessage = vi.fn();
    (globalThis as Record<string, unknown>)['browser'] = {
      runtime: { sendMessage },
    };

    statusEl = dom.window.document.getElementById('status')!;
  });

  describe('loadPendingOrders', () => {

    describe('when module loads with pending orders', () => {
      beforeEach(async () => {
        const testOrders = [makeOrder()];
        sendMessage.mockResolvedValue({ orders: testOrders });

        await importPopup();
      });

      it('should send GET_PENDING message', () => {
        expect(sendMessage).toHaveBeenCalledWith({ type: 'GET_PENDING' });
      });

      it('should set orderListEl.orders from response', () => {
        expect(orderListEl.orders).toHaveLength(1);
        expect(orderListEl.orders[0]!.merchant).to.equal('TestMerchant');
      });
    });

    describe('when response has no orders property', () => {
      beforeEach(async () => {
        sendMessage.mockResolvedValue({});

        await importPopup();
      });

      it('should not update orderListEl.orders', () => {
        expect(orderListEl.orders).toHaveLength(0);
      });
    });
  });

  describe('orders-selected event', () => {

    describe('when save succeeds with saved and skipped counts', () => {
      beforeEach(async () => {
        sendMessage.mockResolvedValueOnce({ orders: [] });

        await importPopup();

        sendMessage.mockResolvedValueOnce({
          success: true,
          savedCount: 3,
          skippedCount: 1,
        });
        sendMessage.mockResolvedValueOnce(undefined);

        dom.window.document.dispatchEvent(
          new dom.window.CustomEvent('orders-selected', {
            detail: { orders: [makeOrder()] },
          }),
        );

        await new Promise<void>(resolve => { setTimeout(resolve, 10); });
      });

      it('should send SAVE_ORDERS message', () => {
        expect(sendMessage).toHaveBeenCalledWith(
          expect.objectContaining({ type: 'SAVE_ORDERS' }),
        );
      });

      it('should set status text with saved and skipped counts', () => {
        expect(statusEl.textContent).to.equal('3 saved, 1 duplicates skipped');
      });

      it('should set status className to success', () => {
        expect(statusEl.className).to.equal('success');
      });

      it('should send DISMISS message after successful save', () => {
        expect(sendMessage).toHaveBeenCalledWith({ type: 'DISMISS' });
      });

      it('should clear orderListEl.orders', () => {
        expect(orderListEl.orders).toHaveLength(0);
      });

      it('should set saving to false after completion', () => {
        expect(orderListEl.saving).to.equal(false);
      });
    });

    describe('when save succeeds with no counts', () => {
      beforeEach(async () => {
        sendMessage.mockResolvedValueOnce({ orders: [] });

        await importPopup();

        sendMessage.mockResolvedValueOnce({ success: true });
        sendMessage.mockResolvedValueOnce(undefined);

        dom.window.document.dispatchEvent(
          new dom.window.CustomEvent('orders-selected', {
            detail: { orders: [makeOrder()] },
          }),
        );

        await new Promise<void>(resolve => { setTimeout(resolve, 10); });
      });

      it('should set status text to Done', () => {
        expect(statusEl.textContent).to.equal('Done');
      });

      it('should set status className to success', () => {
        expect(statusEl.className).to.equal('success');
      });
    });

    describe('when save succeeds with only savedCount', () => {
      beforeEach(async () => {
        sendMessage.mockResolvedValueOnce({ orders: [] });

        await importPopup();

        sendMessage.mockResolvedValueOnce({ success: true, savedCount: 5 });
        sendMessage.mockResolvedValueOnce(undefined);

        dom.window.document.dispatchEvent(
          new dom.window.CustomEvent('orders-selected', {
            detail: { orders: [makeOrder()] },
          }),
        );

        await new Promise<void>(resolve => { setTimeout(resolve, 10); });
      });

      it('should set status text with only saved count', () => {
        expect(statusEl.textContent).to.equal('5 saved');
      });
    });

    describe('when save succeeds with only skippedCount', () => {
      beforeEach(async () => {
        sendMessage.mockResolvedValueOnce({ orders: [] });

        await importPopup();

        sendMessage.mockResolvedValueOnce({ success: true, skippedCount: 2 });
        sendMessage.mockResolvedValueOnce(undefined);

        dom.window.document.dispatchEvent(
          new dom.window.CustomEvent('orders-selected', {
            detail: { orders: [makeOrder()] },
          }),
        );

        await new Promise<void>(resolve => { setTimeout(resolve, 10); });
      });

      it('should set status text with only skipped count', () => {
        expect(statusEl.textContent).to.equal('2 duplicates skipped');
      });
    });

    describe('when save fails with error message', () => {
      beforeEach(async () => {
        sendMessage.mockResolvedValueOnce({ orders: [] });

        await importPopup();

        sendMessage.mockResolvedValueOnce({
          success: false,
          error: 'Wiki connection failed',
        });

        dom.window.document.dispatchEvent(
          new dom.window.CustomEvent('orders-selected', {
            detail: { orders: [makeOrder()] },
          }),
        );

        await new Promise<void>(resolve => { setTimeout(resolve, 10); });
      });

      it('should set status text to the error message', () => {
        expect(statusEl.textContent).to.equal('Wiki connection failed');
      });

      it('should set status className to error', () => {
        expect(statusEl.className).to.equal('error');
      });

      it('should set saving to false', () => {
        expect(orderListEl.saving).to.equal(false);
      });
    });

    describe('when save fails without error message', () => {
      beforeEach(async () => {
        sendMessage.mockResolvedValueOnce({ orders: [] });

        await importPopup();

        sendMessage.mockResolvedValueOnce({ success: false });

        dom.window.document.dispatchEvent(
          new dom.window.CustomEvent('orders-selected', {
            detail: { orders: [makeOrder()] },
          }),
        );

        await new Promise<void>(resolve => { setTimeout(resolve, 10); });
      });

      it('should set status text to Save failed', () => {
        expect(statusEl.textContent).to.equal('Save failed');
      });

      it('should set status className to error', () => {
        expect(statusEl.className).to.equal('error');
      });
    });

    describe('when saving is in progress', () => {
      let savingDuringRequest: boolean;
      let statusDuringRequest: string;

      beforeEach(async () => {
        sendMessage.mockResolvedValueOnce({ orders: [] });

        await importPopup();

        savingDuringRequest = false;
        statusDuringRequest = '';
        sendMessage.mockImplementationOnce(() => {
          savingDuringRequest = orderListEl.saving;
          statusDuringRequest = statusEl.textContent ?? '';
          return Promise.resolve({ success: true });
        });
        sendMessage.mockResolvedValueOnce(undefined);

        dom.window.document.dispatchEvent(
          new dom.window.CustomEvent('orders-selected', {
            detail: { orders: [makeOrder()] },
          }),
        );

        await new Promise<void>(resolve => { setTimeout(resolve, 10); });
      });

      it('should set saving to true while request is pending', () => {
        expect(savingDuringRequest).to.equal(true);
      });

      it('should set status to Saving orders... before sendMessage resolves', () => {
        expect(statusDuringRequest).to.equal('Saving orders...');
      });
    });
  });

  describe('orders-dismissed event', () => {

    describe('when orders are dismissed', () => {
      beforeEach(async () => {
        sendMessage.mockResolvedValueOnce({ orders: [makeOrder()] });

        await importPopup();

        sendMessage.mockResolvedValueOnce(undefined);

        dom.window.document.dispatchEvent(
          new dom.window.CustomEvent('orders-dismissed'),
        );

        await new Promise<void>(resolve => { setTimeout(resolve, 10); });
      });

      it('should send DISMISS message', () => {
        expect(sendMessage).toHaveBeenCalledWith({ type: 'DISMISS' });
      });

      it('should clear orderListEl.orders', () => {
        expect(orderListEl.orders).toHaveLength(0);
      });

      it('should set status text to Dismissed', () => {
        expect(statusEl.textContent).to.equal('Dismissed');
      });

      it('should set status className to empty string', () => {
        expect(statusEl.className).to.equal('');
      });
    });
  });
});
