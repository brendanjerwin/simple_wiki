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

describe('parseOrders', () => {
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

  describe('when a card is missing the order ID element', () => {
    let result: Order[];

    beforeEach(() => {
      const html = `
        <div class="order-card js-order-card">
          <div class="a-box a-color-offset-background order-header">
            <ul class="order-header__header-list">
              <li>
                <span class="a-color-secondary">Order placed</span>
                <span class="a-text-bold">March 5, 2026</span>
              </li>
              <li>
                <span class="a-color-secondary">Total</span>
                <span class="a-text-bold">$10.00</span>
              </li>
            </ul>
          </div>
          <div class="delivery-box">
            <span class="delivery-box__primary-text">Delivered</span>
            <div class="item-box">
              <div class="yohtmlc-product-title">Widget</div>
            </div>
          </div>
        </div>`;
      const dom = new JSDOM(html);
      result = parseOrders(dom.window.document);
    });

    it('should skip the card and return empty array', () => {
      expect(result).to.have.length(0);
    });
  });

  describe('when a card has an order ID that does not match the order number pattern', () => {
    let result: Order[];

    beforeEach(() => {
      const html = `
        <div class="order-card js-order-card">
          <div class="a-box a-color-offset-background order-header">
            <ul class="order-header__header-list">
              <li>
                <span class="a-color-secondary">Order placed</span>
                <span class="a-text-bold">March 5, 2026</span>
              </li>
              <li>
                <span class="a-color-secondary">Total</span>
                <span class="a-text-bold">$10.00</span>
              </li>
              <li>
                <div class="yohtmlc-order-id">
                  <span dir="ltr">NOT-A-VALID-ORDER-NUMBER</span>
                </div>
              </li>
            </ul>
          </div>
          <div class="delivery-box">
            <span class="delivery-box__primary-text">Delivered</span>
            <div class="item-box">
              <div class="yohtmlc-product-title">Widget</div>
            </div>
          </div>
        </div>`;
      const dom = new JSDOM(html);
      result = parseOrders(dom.window.document);
    });

    it('should skip the card and return empty array', () => {
      expect(result).to.have.length(0);
    });
  });

  describe('when a card has no items', () => {
    let result: Order[];

    beforeEach(() => {
      const html = `
        <div class="order-card js-order-card">
          <div class="a-box a-color-offset-background order-header">
            <ul class="order-header__header-list">
              <li>
                <span class="a-color-secondary">Order placed</span>
                <span class="a-text-bold">March 5, 2026</span>
              </li>
              <li>
                <span class="a-color-secondary">Total</span>
                <span class="a-text-bold">$10.00</span>
              </li>
              <li>
                <div class="yohtmlc-order-id">
                  <span dir="ltr">111-2345678-9012345</span>
                </div>
              </li>
            </ul>
          </div>
          <div class="delivery-box">
            <span class="delivery-box__primary-text">Delivered</span>
          </div>
        </div>`;
      const dom = new JSDOM(html);
      result = parseOrders(dom.window.document);
    });

    it('should skip the card and return empty array', () => {
      expect(result).to.have.length(0);
    });
  });

  describe('when a card has an invalid date format', () => {
    let order: Order;

    beforeEach(() => {
      const html = `
        <div class="order-card js-order-card">
          <div class="a-box a-color-offset-background order-header">
            <ul class="order-header__header-list">
              <li>
                <span class="a-color-secondary">Order placed</span>
                <span class="a-text-bold">not a date</span>
              </li>
              <li>
                <span class="a-color-secondary">Total</span>
                <span class="a-text-bold">$10.00</span>
              </li>
              <li>
                <div class="yohtmlc-order-id">
                  <span dir="ltr">111-2345678-9012345</span>
                </div>
              </li>
            </ul>
          </div>
          <div class="delivery-box">
            <span class="delivery-box__primary-text">Delivered</span>
            <div class="item-box">
              <div class="yohtmlc-product-title">Widget</div>
            </div>
          </div>
        </div>`;
      const dom = new JSDOM(html);
      const orders = parseOrders(dom.window.document);
      order = orders[0]!;
    });

    it('should parse the card successfully', () => {
      expect(order).to.exist;
    });

    it('should return an empty string for the order date', () => {
      expect(order.orderDate).to.equal('');
    });
  });

  describe('when a card has an unrecognized month name', () => {
    let order: Order;

    beforeEach(() => {
      const html = `
        <div class="order-card js-order-card">
          <div class="a-box a-color-offset-background order-header">
            <ul class="order-header__header-list">
              <li>
                <span class="a-color-secondary">Order placed</span>
                <span class="a-text-bold">Julember 5, 2026</span>
              </li>
              <li>
                <span class="a-color-secondary">Total</span>
                <span class="a-text-bold">$10.00</span>
              </li>
              <li>
                <div class="yohtmlc-order-id">
                  <span dir="ltr">111-2345678-9012345</span>
                </div>
              </li>
            </ul>
          </div>
          <div class="delivery-box">
            <span class="delivery-box__primary-text">Delivered</span>
            <div class="item-box">
              <div class="yohtmlc-product-title">Widget</div>
            </div>
          </div>
        </div>`;
      const dom = new JSDOM(html);
      const orders = parseOrders(dom.window.document);
      order = orders[0]!;
    });

    it('should parse the card successfully', () => {
      expect(order).to.exist;
    });

    it('should return an empty string for the order date', () => {
      expect(order.orderDate).to.equal('');
    });
  });

  describe('when a card has an invalid price format', () => {
    let order: Order;

    beforeEach(() => {
      const html = `
        <div class="order-card js-order-card">
          <div class="a-box a-color-offset-background order-header">
            <ul class="order-header__header-list">
              <li>
                <span class="a-color-secondary">Order placed</span>
                <span class="a-text-bold">March 5, 2026</span>
              </li>
              <li>
                <span class="a-color-secondary">Total</span>
                <span class="a-text-bold">not a price</span>
              </li>
              <li>
                <div class="yohtmlc-order-id">
                  <span dir="ltr">111-2345678-9012345</span>
                </div>
              </li>
            </ul>
          </div>
          <div class="delivery-box">
            <span class="delivery-box__primary-text">Delivered</span>
            <div class="item-box">
              <div class="yohtmlc-product-title">Widget</div>
            </div>
          </div>
        </div>`;
      const dom = new JSDOM(html);
      const orders = parseOrders(dom.window.document);
      order = orders[0]!;
    });

    it('should parse the card successfully', () => {
      expect(order).to.exist;
    });

    it('should return 0 for totalCents', () => {
      expect(order.totalCents).to.equal(0);
    });
  });

  describe('when a card has a price with comma-separated thousands', () => {
    let order: Order;

    beforeEach(() => {
      const html = `
        <div class="order-card js-order-card">
          <div class="a-box a-color-offset-background order-header">
            <ul class="order-header__header-list">
              <li>
                <span class="a-color-secondary">Order placed</span>
                <span class="a-text-bold">March 5, 2026</span>
              </li>
              <li>
                <span class="a-color-secondary">Total</span>
                <span class="a-text-bold">$1,234.56</span>
              </li>
              <li>
                <div class="yohtmlc-order-id">
                  <span dir="ltr">111-2345678-9012345</span>
                </div>
              </li>
            </ul>
          </div>
          <div class="delivery-box">
            <span class="delivery-box__primary-text">Delivered</span>
            <div class="item-box">
              <div class="yohtmlc-product-title">Expensive Item</div>
            </div>
          </div>
        </div>`;
      const dom = new JSDOM(html);
      const orders = parseOrders(dom.window.document);
      order = orders[0]!;
    });

    it('should parse the price correctly', () => {
      expect(order.totalCents).to.equal(123456);
    });
  });
});
