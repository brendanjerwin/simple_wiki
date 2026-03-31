import { describe, it, expect, beforeEach, vi } from 'vitest';

// Mock wiki-client module (hoisted by vitest before any imports)
const mockReadPage = vi.fn();
const mockUpdatePageContent = vi.fn();
const mockCreatePage = vi.fn();

vi.mock('./wiki-client.js', () => ({
  readPage: mockReadPage,
  updatePageContent: mockUpdatePageContent,
  createPage: mockCreatePage,
}));

// Mock browser storage
let storageData: Record<string, unknown> = {};

const mockBrowser = {
  storage: {
    local: {
      get: vi.fn(async (keys?: string | string[]) => {
        if (!keys) return { ...storageData };
        const keyList = Array.isArray(keys) ? keys : [keys];
        const result: Record<string, unknown> = {};
        for (const key of keyList) {
          if (key in storageData) {
            result[key] = storageData[key];
          }
        }
        return result;
      }),
      set: vi.fn(async (items: Record<string, unknown>) => {
        Object.assign(storageData, items);
      }),
      remove: vi.fn(async (keys: string | string[]) => {
        const keyList = Array.isArray(keys) ? keys : [keys];
        for (const key of keyList) {
          delete storageData[key];
        }
      }),
    },
    onChanged: {
      addListener: vi.fn(),
      removeListener: vi.fn(),
    },
  },
  runtime: {
    onMessage: {
      addListener: vi.fn(),
    },
    sendMessage: vi.fn(),
  },
  browserAction: {
    setBadgeText: vi.fn(),
    setBadgeBackgroundColor: vi.fn(),
  },
};

// Assign mock to global before importing
(globalThis as Record<string, unknown>)['browser'] = mockBrowser;

// Import triggers the module which calls browser.runtime.onMessage.addListener
await import('./background.js');

// Grab the listener that was registered
const messageListener = mockBrowser.runtime.onMessage.addListener.mock.calls[0]![0] as (
  message: unknown,
  sender: unknown,
  sendResponse: (response: unknown) => void
) => true | undefined;

function resetStorageMocks(): void {
  mockBrowser.storage.local.get.mockImplementation(async (keys?: string | string[]) => {
    if (!keys) return { ...storageData };
    const keyList = Array.isArray(keys) ? keys : [keys];
    const result: Record<string, unknown> = {};
    for (const key of keyList) {
      if (key in storageData) {
        result[key] = storageData[key];
      }
    }
    return result;
  });
  mockBrowser.storage.local.set.mockImplementation(async (items: Record<string, unknown>) => {
    Object.assign(storageData, items);
  });
}

const sampleOrders = [
  {
    merchant: 'Amazon',
    orderNumber: '111-1234567-8901234',
    orderDate: '2026-03-01',
    totalCents: 2999,
    deliveryStatus: 'Delivered',
    items: [{ name: 'Widget', quantity: 1, priceCents: 2999 }],
  },
  {
    merchant: 'Amazon',
    orderNumber: '222-2345678-9012345',
    orderDate: '2026-03-05',
    totalCents: 1599,
    deliveryStatus: 'Arriving',
    items: [{ name: 'Gadget', quantity: 2, priceCents: 799 }],
  },
];

describe('handleWikiUrlDetected (via message listener)', () => {
  beforeEach(() => {
    storageData = {};
    vi.clearAllMocks();
    resetStorageMocks();
  });

  describe('when no URL is stored', () => {
    beforeEach(async () => {
      messageListener(
        { type: 'WIKI_URL_DETECTED', wikiUrl: 'https://wiki.local:8050' },
        {},
        vi.fn()
      );
      await vi.waitFor(() => {
        expect(mockBrowser.storage.local.set).toHaveBeenCalled();
      });
    });

    it('should save the detected URL', () => {
      expect(storageData['wikiUrl']).to.equal('https://wiki.local:8050');
    });
  });

  describe('when the same URL is already stored', () => {
    beforeEach(async () => {
      storageData = { wikiUrl: 'https://wiki.local:8050' };
      messageListener(
        { type: 'WIKI_URL_DETECTED', wikiUrl: 'https://wiki.local:8050' },
        {},
        vi.fn()
      );
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should not call set again', () => {
      expect(mockBrowser.storage.local.set).not.toHaveBeenCalled();
    });
  });

  describe('when a different URL is stored but not manually set', () => {
    beforeEach(async () => {
      storageData = { wikiUrl: 'https://old-wiki:8050' };
      messageListener(
        { type: 'WIKI_URL_DETECTED', wikiUrl: 'https://new-wiki:8050' },
        {},
        vi.fn()
      );
      await vi.waitFor(() => {
        expect(mockBrowser.storage.local.set).toHaveBeenCalled();
      });
    });

    it('should update to the new URL', () => {
      expect(storageData['wikiUrl']).to.equal('https://new-wiki:8050');
    });
  });

  describe('when wikiUrlManuallySet is true', () => {
    beforeEach(async () => {
      storageData = { wikiUrl: 'https://manual-wiki:8050', wikiUrlManuallySet: true };
      messageListener(
        { type: 'WIKI_URL_DETECTED', wikiUrl: 'https://auto-wiki:8050' },
        {},
        vi.fn()
      );
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should not update the URL', () => {
      expect(storageData['wikiUrl']).to.equal('https://manual-wiki:8050');
    });

    it('should not call set', () => {
      expect(mockBrowser.storage.local.set).not.toHaveBeenCalled();
    });
  });
});

describe('ORDERS_DETECTED handler', () => {
  beforeEach(() => {
    storageData = {};
    vi.clearAllMocks();
    resetStorageMocks();
  });

  describe('when orders are detected', () => {
    beforeEach(() => {
      messageListener(
        { type: 'ORDERS_DETECTED', orders: sampleOrders },
        {},
        vi.fn()
      );
    });

    it('should set badge text to order count', () => {
      expect(mockBrowser.browserAction.setBadgeText).toHaveBeenCalledWith({ text: '2' });
    });

    it('should set badge background to green', () => {
      expect(mockBrowser.browserAction.setBadgeBackgroundColor).toHaveBeenCalledWith({ color: '#43a047' });
    });
  });
});

describe('GET_PENDING handler', () => {
  beforeEach(() => {
    storageData = {};
    vi.clearAllMocks();
    resetStorageMocks();
  });

  describe('when orders were previously detected', () => {
    let sendResponse: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      // First detect orders
      messageListener(
        { type: 'ORDERS_DETECTED', orders: sampleOrders },
        {},
        vi.fn()
      );
      vi.clearAllMocks();

      // Then request pending
      sendResponse = vi.fn();
      messageListener({ type: 'GET_PENDING' }, {}, sendResponse);
    });

    it('should respond with pending orders', () => {
      expect(sendResponse).toHaveBeenCalledWith({ orders: sampleOrders });
    });
  });

  describe('when no orders were detected', () => {
    let sendResponse: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      // Dismiss any existing orders first
      messageListener({ type: 'DISMISS' }, {}, vi.fn());
      vi.clearAllMocks();

      sendResponse = vi.fn();
      messageListener({ type: 'GET_PENDING' }, {}, sendResponse);
    });

    it('should respond with empty array', () => {
      expect(sendResponse).toHaveBeenCalledWith({ orders: [] });
    });
  });
});

describe('DISMISS handler', () => {
  beforeEach(() => {
    storageData = {};
    vi.clearAllMocks();
    resetStorageMocks();
  });

  describe('when dismissing pending orders', () => {
    beforeEach(() => {
      // First detect orders
      messageListener(
        { type: 'ORDERS_DETECTED', orders: sampleOrders },
        {},
        vi.fn()
      );
      vi.clearAllMocks();

      // Then dismiss
      messageListener({ type: 'DISMISS' }, {}, vi.fn());
    });

    it('should clear badge text', () => {
      expect(mockBrowser.browserAction.setBadgeText).toHaveBeenCalledWith({ text: '' });
    });

    it('should clear pending orders', () => {
      const sendResponse = vi.fn();
      messageListener({ type: 'GET_PENDING' }, {}, sendResponse);
      expect(sendResponse).toHaveBeenCalledWith({ orders: [] });
    });
  });
});

describe('SAVE_ORDERS handler', () => {
  beforeEach(() => {
    storageData = { wikiUrl: 'https://wiki.test' };
    vi.clearAllMocks();
    resetStorageMocks();
  });

  describe('when saving to a new page (page does not exist)', () => {
    let sendResponse: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
      mockReadPage.mockRejectedValue(new Error('not found'));
      mockCreatePage.mockResolvedValue({});

      sendResponse = vi.fn();
      messageListener(
        { type: 'SAVE_ORDERS', orders: sampleOrders },
        {},
        sendResponse
      );

      await vi.waitFor(() => {
        expect(sendResponse).toHaveBeenCalled();
      });
    });

    it('should respond with success', () => {
      expect(sendResponse).toHaveBeenCalledWith(
        expect.objectContaining({ success: true })
      );
    });

    it('should report saved count', () => {
      expect(sendResponse).toHaveBeenCalledWith(
        expect.objectContaining({ savedCount: 2 })
      );
    });

    it('should report zero skipped', () => {
      expect(sendResponse).toHaveBeenCalledWith(
        expect.objectContaining({ skippedCount: 0 })
      );
    });

    it('should call createPage', () => {
      expect(mockCreatePage).toHaveBeenCalledOnce();
    });

    it('should not call updatePageContent', () => {
      expect(mockUpdatePageContent).not.toHaveBeenCalled();
    });
  });

  describe('when saving to an existing page', () => {
    let sendResponse: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
      mockReadPage.mockResolvedValue({
        contentMarkdown: '| Date | Merchant | Order # | Items | Prices | Total | Status |\n|------|----------|---------|-------|--------|-------|--------|\n',
        versionHash: 'hash123',
      });
      mockUpdatePageContent.mockResolvedValue({});

      sendResponse = vi.fn();
      messageListener(
        { type: 'SAVE_ORDERS', orders: sampleOrders },
        {},
        sendResponse
      );

      await vi.waitFor(() => {
        expect(sendResponse).toHaveBeenCalled();
      });
    });

    it('should respond with success', () => {
      expect(sendResponse).toHaveBeenCalledWith(
        expect.objectContaining({ success: true })
      );
    });

    it('should call updatePageContent', () => {
      expect(mockUpdatePageContent).toHaveBeenCalledOnce();
    });

    it('should not call createPage', () => {
      expect(mockCreatePage).not.toHaveBeenCalled();
    });

    it('should pass version hash for optimistic locking', () => {
      expect(mockUpdatePageContent).toHaveBeenCalledWith(
        'https://wiki.test',
        'online_orders',
        expect.any(String),
        'hash123'
      );
    });
  });

  describe('when all orders are duplicates', () => {
    let sendResponse: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
      mockReadPage.mockResolvedValue({
        contentMarkdown: '| stuff | 111-1234567-8901234 | 222-2345678-9012345 |',
        versionHash: 'hash456',
      });

      sendResponse = vi.fn();
      messageListener(
        { type: 'SAVE_ORDERS', orders: sampleOrders },
        {},
        sendResponse
      );

      await vi.waitFor(() => {
        expect(sendResponse).toHaveBeenCalled();
      });
    });

    it('should respond with success', () => {
      expect(sendResponse).toHaveBeenCalledWith(
        expect.objectContaining({ success: true })
      );
    });

    it('should report zero saved', () => {
      expect(sendResponse).toHaveBeenCalledWith(
        expect.objectContaining({ savedCount: 0 })
      );
    });

    it('should report all skipped', () => {
      expect(sendResponse).toHaveBeenCalledWith(
        expect.objectContaining({ skippedCount: 2 })
      );
    });

    it('should not call updatePageContent', () => {
      expect(mockUpdatePageContent).not.toHaveBeenCalled();
    });

    it('should not call createPage', () => {
      expect(mockCreatePage).not.toHaveBeenCalled();
    });
  });

  describe('when some orders are duplicates', () => {
    let sendResponse: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
      mockReadPage.mockResolvedValue({
        contentMarkdown: '| stuff | 111-1234567-8901234 |',
        versionHash: 'hash789',
      });
      mockUpdatePageContent.mockResolvedValue({});

      sendResponse = vi.fn();
      messageListener(
        { type: 'SAVE_ORDERS', orders: sampleOrders },
        {},
        sendResponse
      );

      await vi.waitFor(() => {
        expect(sendResponse).toHaveBeenCalled();
      });
    });

    it('should report one saved', () => {
      expect(sendResponse).toHaveBeenCalledWith(
        expect.objectContaining({ savedCount: 1 })
      );
    });

    it('should report one skipped', () => {
      expect(sendResponse).toHaveBeenCalledWith(
        expect.objectContaining({ skippedCount: 1 })
      );
    });
  });

  describe('when wiki client throws during save', () => {
    let sendResponse: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
      mockReadPage.mockResolvedValue({
        contentMarkdown: '',
        versionHash: 'hash000',
      });
      mockUpdatePageContent.mockRejectedValue(new Error('version mismatch'));

      sendResponse = vi.fn();
      messageListener(
        { type: 'SAVE_ORDERS', orders: sampleOrders },
        {},
        sendResponse
      );

      await vi.waitFor(() => {
        expect(sendResponse).toHaveBeenCalled();
      });
    });

    it('should respond with failure', () => {
      expect(sendResponse).toHaveBeenCalledWith(
        expect.objectContaining({ success: false })
      );
    });

    it('should include error message', () => {
      expect(sendResponse).toHaveBeenCalledWith(
        expect.objectContaining({ error: 'version mismatch' })
      );
    });
  });

  describe('when wiki URL is not configured', () => {
    let sendResponse: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
      storageData = {};
      mockReadPage.mockRejectedValue(new Error('not found'));
      mockCreatePage.mockResolvedValue({});

      sendResponse = vi.fn();
      messageListener(
        { type: 'SAVE_ORDERS', orders: sampleOrders },
        {},
        sendResponse
      );

      await vi.waitFor(() => {
        expect(sendResponse).toHaveBeenCalled();
      });
    });

    it('should use default wiki URL', () => {
      expect(mockReadPage).toHaveBeenCalledWith(
        'http://localhost:8050',
        'online_orders'
      );
    });
  });
});
