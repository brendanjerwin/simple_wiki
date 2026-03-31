import { describe, it, expect, beforeEach } from 'vitest';
import { JSDOM } from 'jsdom';
import { readFileSync } from 'fs';
import { resolve } from 'path';
import { parseOrders } from './amazon.js';
import { Order } from './types.js';

const fixtureHtml = readFileSync(
  resolve(__dirname, '../../test-fixtures/amazon-order-history.html'),
  'utf-8'
);

function makeOrderCard(options: {
  orderId?: string;
  date?: string;
  price?: string;
  itemName?: string | null;
  includeOrderIdEl?: boolean;
} = {}): string {
  const {
    orderId = '111-2345678-9012345',
    date = 'March 5, 2026',
    price = '$45.52',
    itemName = 'Widget',
    includeOrderIdEl = true,
  } = options;

  const orderIdHtml = includeOrderIdEl
    ? `<li><div class="yohtmlc-order-id"><span dir="ltr">${orderId}</span></div></li>`
    : '';

  const itemHtml = itemName !== null
    ? `<div class="item-box"><div class="yohtmlc-product-title">${itemName}</div></div>`
    : '';

  return `
    <div class="order-card js-order-card">
      <ul class="order-header__header-list">
        <li>
          <span class="a-color-secondary">Order placed</span>
          <span class="a-text-bold">${date}</span>
        </li>
        <li>
          <span class="a-color-secondary">Total</span>
          <span class="a-text-bold">${price}</span>
        </li>
        ${orderIdHtml}
      </ul>
      <span class="delivery-box__primary-text">Delivered</span>
      ${itemHtml}
    </div>
  `;
}

function parseFromHtml(cardHtml: string): Order[] {
  const dom = new JSDOM(`<html><body>${cardHtml}</body></html>`);
  return parseOrders(dom.window.document);
}

describe('parseOrders', () => {

  describe('when given the Amazon fixture', () => {
    let orders: Order[];

    beforeEach(() => {
      const dom = new JSDOM(fixtureHtml);
      orders = parseOrders(dom.window.document);
    });

    describe('when given an Amazon order history page', () => {

      it('should return 3 orders', () => {
        expect(orders).to.have.length(3);
      });

      describe('each order', () => {

        it('should have merchant set to Amazon', () => {
          for (const order of orders) {
            expect(order.merchant).to.equal('Amazon');
          }
        });

        it('should have a valid order number format', () => {
          for (const order of orders) {
            expect(order.orderNumber).to.match(/^\d{3}-\d{7}-\d{7}$/);
          }
        });

        it('should have an ISO date string', () => {
          for (const order of orders) {
            expect(order.orderDate).to.match(/^\d{4}-\d{2}-\d{2}$/);
          }
        });

        it('should have at least one item', () => {
          for (const order of orders) {
            expect(order.items.length).to.be.greaterThan(0);
          }
        });

        it('should have a positive totalCents', () => {
          for (const order of orders) {
            expect(order.totalCents).to.be.greaterThan(0);
          }
        });

        it('should have a non-empty deliveryStatus', () => {
          for (const order of orders) {
            expect(order.deliveryStatus).to.be.a('string');
            expect(order.deliveryStatus.length).to.be.greaterThan(0);
          }
        });

      });

    });

    describe('when parsing order 1 (multi-item, arriving)', () => {
      let order: Order;

      beforeEach(() => {
        order = orders[0]!;
      });

      it('should have the correct order number', () => {
        expect(order.orderNumber).to.equal('111-2345678-9012345');
      });

      it('should have the correct date', () => {
        expect(order.orderDate).to.equal('2026-03-05');
      });

      it('should have 2 items', () => {
        expect(order.items).to.have.length(2);
      });

      it('should have the correct first item name', () => {
        expect(order.items[0]!.name).to.equal('Acme Wireless Bluetooth Headphones');
      });

      it('should have the correct first item price', () => {
        expect(order.items[0]!.priceCents).to.equal(2999);
      });

      it('should have the correct second item name', () => {
        expect(order.items[1]!.name).to.equal('USB-C Charging Cable 6ft');
      });

      it('should have the correct second item price', () => {
        expect(order.items[1]!.priceCents).to.equal(1553);
      });

      it('should have the correct total', () => {
        expect(order.totalCents).to.equal(4552);
      });

      it('should have the correct delivery status', () => {
        expect(order.deliveryStatus).to.equal('Arriving March 12 - March 16');
      });

    });

    describe('when parsing order 2 (single item, delivered)', () => {
      let order: Order;

      beforeEach(() => {
        order = orders[1]!;
      });

      it('should have the correct order number', () => {
        expect(order.orderNumber).to.equal('222-3456789-0123456');
      });

      it('should have the correct date', () => {
        expect(order.orderDate).to.equal('2026-03-01');
      });

      it('should have 1 item', () => {
        expect(order.items).to.have.length(1);
      });

      it('should have the correct item name', () => {
        expect(order.items[0]!.name).to.equal('Stainless Steel Water Bottle 32oz');
      });

      it('should have the correct total', () => {
        expect(order.totalCents).to.equal(1699);
      });

      it('should have the correct delivery status', () => {
        expect(order.deliveryStatus).to.equal('Delivered March 4');
      });

    });

    describe('when parsing order 3 (three items, cancelled)', () => {
      let order: Order;

      beforeEach(() => {
        order = orders[2]!;
      });

      it('should have the correct order number', () => {
        expect(order.orderNumber).to.equal('333-4567890-1234567');
      });

      it('should have the correct date', () => {
        expect(order.orderDate).to.equal('2026-02-28');
      });

      it('should have 3 items', () => {
        expect(order.items).to.have.length(3);
      });

      it('should have the correct total', () => {
        expect(order.totalCents).to.equal(8997);
      });

      it('should have the correct delivery status', () => {
        expect(order.deliveryStatus).to.equal('Cancelled');
      });

    });

  });

  describe('when given a non-Amazon page', () => {
    let nonAmazonOrders: Order[];

    beforeEach(() => {
      const dom = new JSDOM('<html><body><p>Not an Amazon page</p></body></html>');
      nonAmazonOrders = parseOrders(dom.window.document);
    });

    it('should return an empty array', () => {
      expect(nonAmazonOrders).to.have.length(0);
    });

  });

  describe('when given an order card without an order ID element', () => {
    let result: Order[];

    beforeEach(() => {
      result = parseFromHtml(makeOrderCard({ includeOrderIdEl: false }));
    });

    it('should exclude that order from results', () => {
      expect(result).to.have.length(0);
    });

  });

  describe('when given an order card with an invalid order number format', () => {
    let result: Order[];

    beforeEach(() => {
      result = parseFromHtml(makeOrderCard({ orderId: 'NOT-A-VALID-ORDER-NUMBER' }));
    });

    it('should exclude that order from results', () => {
      expect(result).to.have.length(0);
    });

  });

  describe('when given an order card with no items', () => {
    let result: Order[];

    beforeEach(() => {
      result = parseFromHtml(makeOrderCard({ itemName: null }));
    });

    it('should exclude that order from results', () => {
      expect(result).to.have.length(0);
    });

  });

  describe('when given an order card with an invalid date format', () => {
    let result: Order[];

    beforeEach(() => {
      result = parseFromHtml(makeOrderCard({ date: '2026/03/05' }));
    });

    it('should include the order', () => {
      expect(result).to.have.length(1);
    });

    it('should set orderDate to empty string', () => {
      expect(result[0]!.orderDate).to.equal('');
    });

  });

  describe('when given an order card with an unknown month name', () => {
    let result: Order[];

    beforeEach(() => {
      result = parseFromHtml(makeOrderCard({ date: 'Martian 5, 2026' }));
    });

    it('should include the order', () => {
      expect(result).to.have.length(1);
    });

    it('should set orderDate to empty string', () => {
      expect(result[0]!.orderDate).to.equal('');
    });

  });

  describe('when given an order card with an invalid price format', () => {
    let result: Order[];

    beforeEach(() => {
      result = parseFromHtml(makeOrderCard({ price: 'not-a-price' }));
    });

    it('should include the order', () => {
      expect(result).to.have.length(1);
    });

    it('should set totalCents to 0', () => {
      expect(result[0]!.totalCents).to.equal(0);
    });

  });

  describe('when given an order card with a comma-formatted price', () => {
    let result: Order[];

    beforeEach(() => {
      result = parseFromHtml(makeOrderCard({ price: '$1,234.56' }));
    });

    it('should include the order', () => {
      expect(result).to.have.length(1);
    });

    it('should parse the price correctly in cents', () => {
      expect(result[0]!.totalCents).to.equal(123456);
    });

  });

  describe('when given multiple cards where some are invalid', () => {
    let result: Order[];

    beforeEach(() => {
      const validCard = makeOrderCard({ orderId: '111-2345678-9012345' });
      const invalidCard = makeOrderCard({ includeOrderIdEl: false });
      const anotherValidCard = makeOrderCard({ orderId: '222-3456789-0123456' });
      result = parseFromHtml(validCard + invalidCard + anotherValidCard);
    });

    it('should only return the valid orders', () => {
      expect(result).to.have.length(2);
    });

    it('should have the correct order numbers', () => {
      expect(result[0]!.orderNumber).to.equal('111-2345678-9012345');
      expect(result[1]!.orderNumber).to.equal('222-3456789-0123456');
    });

  });

  describe('when given an order card with no date or total headers', () => {
    let result: Order[];

    beforeEach(() => {
      // Card has a 'ship to' header but no 'order placed' or 'total' headers,
      // exercising the findHeaderValue fallback (returns '' when no header matches).
      const cardHtml = `
        <div class="order-card js-order-card">
          <ul class="order-header__header-list">
            <li>
              <span class="a-color-secondary">Ship to</span>
              <span class="a-text-bold">Jane Doe</span>
            </li>
            <li>
              <div class="yohtmlc-order-id"><span dir="ltr">111-2345678-9012345</span></div>
            </li>
          </ul>
          <span class="delivery-box__primary-text">Delivered</span>
          <div class="item-box"><div class="yohtmlc-product-title">Widget</div></div>
        </div>
      `;
      result = parseFromHtml(cardHtml);
    });

    it('should include the order', () => {
      expect(result).to.have.length(1);
    });

    it('should set orderDate to empty string', () => {
      expect(result[0]!.orderDate).to.equal('');
    });

    it('should set totalCents to 0', () => {
      expect(result[0]!.totalCents).to.equal(0);
    });

  });

});
