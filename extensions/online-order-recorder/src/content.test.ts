import { describe, it, expect, beforeEach, vi } from 'vitest';
import type { Order } from './merchants/types.js';

const mockParseOrders = vi.fn<(doc: Document) => Order[]>();

vi.mock('./merchants/amazon.js', () => ({
  parseOrders: mockParseOrders,
}));

const mockSendMessage = vi.fn();

function setupGlobals(hostname: string): void {
  (globalThis as Record<string, unknown>)['browser'] = {
    runtime: {
      sendMessage: mockSendMessage,
    },
  };
  (globalThis as Record<string, unknown>)['window'] = {
    location: {
      hostname,
      href: `https://${hostname}/orders`,
    },
  };
  (globalThis as Record<string, unknown>)['location'] = {
    hostname,
    href: `https://${hostname}/orders`,
  };
  // content.ts also references bare `document` in parseOrders(document)
  // Since parseOrders is mocked, we just need it to exist
  if (!('document' in globalThis)) {
    (globalThis as Record<string, unknown>)['document'] = {};
  }
}

function makeFakeOrder(overrides: Partial<Order> = {}): Order {
  return {
    merchant: 'Amazon',
    orderNumber: '111-2345678-9012345',
    orderDate: '2026-03-05',
    items: [{ name: 'Widget', priceCents: 999, quantity: 1 }],
    totalCents: 999,
    deliveryStatus: 'Delivered',
    ...overrides,
  };
}

describe('content script', () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
  });

  describe('when on www.amazon.com', () => {
    describe('when parseOrders returns orders', () => {
      let fakeOrders: Order[];

      beforeEach(async () => {
        fakeOrders = [makeFakeOrder()];
        setupGlobals('www.amazon.com');
        mockParseOrders.mockReturnValue(fakeOrders);

        await import('./content.js');
      });

      it('should call parseOrders', () => {
        expect(mockParseOrders).toHaveBeenCalledOnce();
      });

      it('should send ORDERS_DETECTED message to background script', () => {
        expect(mockSendMessage).toHaveBeenCalledOnce();
        expect(mockSendMessage).toHaveBeenCalledWith({
          type: 'ORDERS_DETECTED',
          orders: fakeOrders,
        });
      });
    });

    describe('when parseOrders returns an empty array', () => {
      beforeEach(async () => {
        setupGlobals('www.amazon.com');
        mockParseOrders.mockReturnValue([]);

        await import('./content.js');
      });

      it('should call parseOrders', () => {
        expect(mockParseOrders).toHaveBeenCalledOnce();
      });

      it('should not send a message', () => {
        expect(mockSendMessage).not.toHaveBeenCalled();
      });
    });
  });

  describe('when on amazon.com (bare domain)', () => {
    beforeEach(async () => {
      setupGlobals('amazon.com');
      mockParseOrders.mockReturnValue([makeFakeOrder()]);

      await import('./content.js');
    });

    it('should call parseOrders', () => {
      expect(mockParseOrders).toHaveBeenCalledOnce();
    });

    it('should send ORDERS_DETECTED message', () => {
      expect(mockSendMessage).toHaveBeenCalledOnce();
    });
  });

  describe('when on smile.amazon.com (Amazon subdomain)', () => {
    beforeEach(async () => {
      setupGlobals('smile.amazon.com');
      mockParseOrders.mockReturnValue([makeFakeOrder()]);

      await import('./content.js');
    });

    it('should call parseOrders', () => {
      expect(mockParseOrders).toHaveBeenCalledOnce();
    });

    it('should send ORDERS_DETECTED message', () => {
      expect(mockSendMessage).toHaveBeenCalledOnce();
    });
  });

  describe('when on a non-Amazon domain', () => {
    beforeEach(async () => {
      setupGlobals('www.example.com');

      await import('./content.js');
    });

    it('should not call parseOrders', () => {
      expect(mockParseOrders).not.toHaveBeenCalled();
    });

    it('should not send a message', () => {
      expect(mockSendMessage).not.toHaveBeenCalled();
    });
  });

  describe('when on a domain that contains amazon but is not amazon.com', () => {
    beforeEach(async () => {
      setupGlobals('not-amazon.com');

      await import('./content.js');
    });

    it('should not call parseOrders', () => {
      expect(mockParseOrders).not.toHaveBeenCalled();
    });

    it('should not send a message', () => {
      expect(mockSendMessage).not.toHaveBeenCalled();
    });
  });

  describe('when parseOrders returns multiple orders', () => {
    let fakeOrders: Order[];

    beforeEach(async () => {
      fakeOrders = [
        makeFakeOrder({ orderNumber: '111-0000000-0000001' }),
        makeFakeOrder({ orderNumber: '111-0000000-0000002' }),
        makeFakeOrder({ orderNumber: '111-0000000-0000003' }),
      ];
      setupGlobals('www.amazon.com');
      mockParseOrders.mockReturnValue(fakeOrders);

      await import('./content.js');
    });

    it('should send all orders in the message', () => {
      expect(mockSendMessage).toHaveBeenCalledWith({
        type: 'ORDERS_DETECTED',
        orders: fakeOrders,
      });
    });
  });
});
