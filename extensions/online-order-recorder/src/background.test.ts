import { describe, it, expect, beforeEach, vi } from 'vitest';

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

// We test handleWikiUrlDetected indirectly through the message listener
// Import triggers the module which calls browser.runtime.onMessage.addListener
await import('./background.js');

// Grab the listener that was registered
const messageListener = mockBrowser.runtime.onMessage.addListener.mock.calls[0]![0] as (
  message: unknown,
  sender: unknown,
  sendResponse: (response: unknown) => void
) => true | undefined;

describe('handleWikiUrlDetected (via message listener)', () => {
  beforeEach(() => {
    storageData = {};
    vi.clearAllMocks();
    // Re-assign get/set since we cleared mocks
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
  });

  describe('when no URL is stored', () => {
    beforeEach(async () => {
      messageListener(
        { type: 'WIKI_URL_DETECTED', wikiUrl: 'http://wiki.local:8050' },
        {},
        vi.fn()
      );
      // Let the async handler complete
      await vi.waitFor(() => {
        expect(mockBrowser.storage.local.set).toHaveBeenCalled();
      });
    });

    it('should save the detected URL', () => {
      expect(storageData['wikiUrl']).to.equal('http://wiki.local:8050');
    });
  });

  describe('when the same URL is already stored', () => {
    beforeEach(async () => {
      storageData = { wikiUrl: 'http://wiki.local:8050' };
      messageListener(
        { type: 'WIKI_URL_DETECTED', wikiUrl: 'http://wiki.local:8050' },
        {},
        vi.fn()
      );
      // Give async handler time to potentially run
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should not call set again', () => {
      expect(mockBrowser.storage.local.set).not.toHaveBeenCalled();
    });
  });

  describe('when a different URL is stored but not manually set', () => {
    beforeEach(async () => {
      storageData = { wikiUrl: 'http://old-wiki:8050' };
      messageListener(
        { type: 'WIKI_URL_DETECTED', wikiUrl: 'http://new-wiki:8050' },
        {},
        vi.fn()
      );
      await vi.waitFor(() => {
        expect(mockBrowser.storage.local.set).toHaveBeenCalled();
      });
    });

    it('should update to the new URL', () => {
      expect(storageData['wikiUrl']).to.equal('http://new-wiki:8050');
    });
  });

  describe('when wikiUrlManuallySet is true', () => {
    beforeEach(async () => {
      storageData = { wikiUrl: 'http://manual-wiki:8050', wikiUrlManuallySet: true };
      messageListener(
        { type: 'WIKI_URL_DETECTED', wikiUrl: 'http://auto-wiki:8050' },
        {},
        vi.fn()
      );
      // Give async handler time to potentially run
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should not update the URL', () => {
      expect(storageData['wikiUrl']).to.equal('http://manual-wiki:8050');
    });

    it('should not call set', () => {
      expect(mockBrowser.storage.local.set).not.toHaveBeenCalled();
    });
  });
});
