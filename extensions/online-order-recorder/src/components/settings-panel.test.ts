/** @vitest-environment jsdom */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

// Set up browser mock on globalThis BEFORE importing the component module,
// since the @customElement decorator registers the element at import time.
const mockGet = vi.fn<(keys?: string | string[]) => Promise<Record<string, unknown>>>();
const mockSet = vi.fn<(items: Record<string, unknown>) => Promise<void>>();
const mockRemove = vi.fn<(keys: string | string[]) => Promise<void>>();
const mockAddListener = vi.fn();
const mockRemoveListener = vi.fn();

(globalThis as Record<string, unknown>)['browser'] = {
  storage: {
    local: {
      get: mockGet,
      set: mockSet,
      remove: mockRemove,
    },
    onChanged: {
      addListener: mockAddListener,
      removeListener: mockRemoveListener,
    },
  },
};

// Now import the component (registers the custom element once)
import { SettingsPanel } from './settings-panel.js';

function createSettingsPanel(): SettingsPanel {
  const el = document.createElement('settings-panel') as SettingsPanel;
  document.body.appendChild(el);
  return el;
}

describe('SettingsPanel', () => {
  let el: SettingsPanel;

  afterEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
  });

  describe('constructor defaults', () => {

    beforeEach(() => {
      mockGet.mockResolvedValue({});
      el = new SettingsPanel();
    });

    it('should default wikiUrl to http://localhost:8050', () => {
      expect(el.wikiUrl).to.equal('http://localhost:8050');
    });

    it('should default showSaved to false', () => {
      expect(el.showSaved).toBeFalsy();
    });

    it('should default isManuallySet to false', () => {
      expect(el.isManuallySet).toBeFalsy();
    });

    it('should default showAutoDetected to false', () => {
      expect(el.showAutoDetected).toBeFalsy();
    });
  });

  describe('connectedCallback', () => {

    beforeEach(async () => {
      vi.clearAllMocks();
      mockGet.mockResolvedValue({});
      el = createSettingsPanel();
      await el.updateComplete;
    });

    it('should call storage.local.get to load settings', () => {
      expect(mockGet).toHaveBeenCalledWith(['wikiUrl', 'wikiUrlManuallySet']);
    });

    it('should add a storage change listener', () => {
      expect(mockAddListener).toHaveBeenCalledOnce();
    });

    it('should pass a function as the storage change listener', () => {
      expect(typeof mockAddListener.mock.calls[0]![0]).to.equal('function');
    });
  });

  describe('disconnectedCallback', () => {

    beforeEach(async () => {
      vi.clearAllMocks();
      mockGet.mockResolvedValue({});
      el = createSettingsPanel();
      await el.updateComplete;
      el.remove();
    });

    it('should remove the storage change listener', () => {
      expect(mockRemoveListener).toHaveBeenCalledOnce();
    });

    it('should remove the same listener that was added', () => {
      const addedListener = mockAddListener.mock.calls[0]![0] as unknown;
      const removedListener = mockRemoveListener.mock.calls[0]![0] as unknown;
      expect(addedListener).to.equal(removedListener);
    });
  });

  describe('_loadSettings', () => {

    describe('when storage has a wikiUrl and wikiUrlManuallySet', () => {

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({
          wikiUrl: 'http://custom-wiki:9090',
          wikiUrlManuallySet: true,
        });
        el = createSettingsPanel();
        await el.updateComplete;
        // Wait for the async _loadSettings to complete
        await vi.waitFor(() => {
          expect(el.wikiUrl).to.equal('http://custom-wiki:9090');
        });
      });

      it('should set wikiUrl from storage', () => {
        expect(el.wikiUrl).to.equal('http://custom-wiki:9090');
      });

      it('should set isManuallySet to true', () => {
        expect(el.isManuallySet).toBeTruthy();
      });
    });

    describe('when storage has no wikiUrl', () => {

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({});
        el = createSettingsPanel();
        await el.updateComplete;
        // Allow async _loadSettings to settle
        await new Promise<void>(resolve => { setTimeout(resolve, 0); });
      });

      it('should keep the default wikiUrl', () => {
        expect(el.wikiUrl).to.equal('http://localhost:8050');
      });

      it('should set isManuallySet to false', () => {
        expect(el.isManuallySet).toBeFalsy();
      });
    });

    describe('when storage has wikiUrlManuallySet as false', () => {

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({
          wikiUrl: 'http://some-wiki:8050',
          wikiUrlManuallySet: false,
        });
        el = createSettingsPanel();
        await el.updateComplete;
        await vi.waitFor(() => {
          expect(el.wikiUrl).to.equal('http://some-wiki:8050');
        });
      });

      it('should set isManuallySet to false since the value is not true', () => {
        expect(el.isManuallySet).toBeFalsy();
      });
    });
  });

  describe('_handleSave', () => {

    beforeEach(async () => {
      vi.useFakeTimers();
      vi.clearAllMocks();
      mockGet.mockResolvedValue({});
      mockSet.mockResolvedValue(undefined);
      el = createSettingsPanel();
      await el.updateComplete;
      el.wikiUrl = 'http://my-wiki:3000';
      await el.updateComplete;

      // Click the Save button
      const saveButton = el.shadowRoot!.querySelector('button')!;
      saveButton.click();
      // Allow the async _handleSave to settle
      await vi.waitFor(() => {
        expect(mockSet).toHaveBeenCalled();
      });
      await el.updateComplete;
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it('should write the wikiUrl to storage', () => {
      expect(mockSet).toHaveBeenCalledWith(
        expect.objectContaining({ wikiUrl: 'http://my-wiki:3000' }),
      );
    });

    it('should write wikiUrlManuallySet as true to storage', () => {
      expect(mockSet).toHaveBeenCalledWith(
        expect.objectContaining({ wikiUrlManuallySet: true }),
      );
    });

    it('should set isManuallySet to true', () => {
      expect(el.isManuallySet).toBeTruthy();
    });

    it('should set showSaved to true', () => {
      expect(el.showSaved).toBeTruthy();
    });

    describe('when 2 seconds have passed', () => {

      beforeEach(async () => {
        await vi.advanceTimersByTimeAsync(2000);
        await el.updateComplete;
      });

      it('should clear showSaved', () => {
        expect(el.showSaved).toBeFalsy();
      });
    });
  });

  describe('_handleResetToAutoDetect', () => {

    beforeEach(async () => {
      vi.clearAllMocks();
      mockGet.mockResolvedValue({ wikiUrl: 'http://custom:8050', wikiUrlManuallySet: true });
      mockRemove.mockResolvedValue(undefined);
      el = createSettingsPanel();
      await el.updateComplete;
      await vi.waitFor(() => {
        expect(el.isManuallySet).toBeTruthy();
      });
      await el.updateComplete;

      // The reset button should be rendered since isManuallySet is true
      const resetButton = el.shadowRoot!.querySelector('.reset-link')!;
      (resetButton as HTMLButtonElement).click();
      await vi.waitFor(() => {
        expect(mockRemove).toHaveBeenCalled();
      });
      await el.updateComplete;
    });

    it('should call storage.local.remove with wikiUrlManuallySet', () => {
      expect(mockRemove).toHaveBeenCalledWith('wikiUrlManuallySet');
    });

    it('should set isManuallySet to false', () => {
      expect(el.isManuallySet).toBeFalsy();
    });
  });

  describe('_handleStorageChanged', () => {

    describe('when areaName is not local', () => {
      let storageListener: (changes: Record<string, { newValue?: unknown }>, areaName: string) => void;

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({});
        el = createSettingsPanel();
        await el.updateComplete;
        storageListener = mockAddListener.mock.calls[0]![0] as typeof storageListener;

        storageListener({ wikiUrl: { newValue: 'http://ignored:8050' } }, 'sync');
        await el.updateComplete;
      });

      it('should not update wikiUrl', () => {
        expect(el.wikiUrl).to.equal('http://localhost:8050');
      });
    });

    describe('when areaName is local and wikiUrl changes', () => {
      let storageListener: (changes: Record<string, { newValue?: unknown }>, areaName: string) => void;

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({});
        el = createSettingsPanel();
        await el.updateComplete;
        storageListener = mockAddListener.mock.calls[0]![0] as typeof storageListener;

        storageListener({ wikiUrl: { newValue: 'http://new-wiki:8050' } }, 'local');
        await el.updateComplete;
      });

      it('should update wikiUrl', () => {
        expect(el.wikiUrl).to.equal('http://new-wiki:8050');
      });
    });

    describe('when wikiUrl changes and isManuallySet is false', () => {
      let storageListener: (changes: Record<string, { newValue?: unknown }>, areaName: string) => void;

      beforeEach(async () => {
        vi.useFakeTimers();
        vi.clearAllMocks();
        mockGet.mockResolvedValue({});
        el = createSettingsPanel();
        await el.updateComplete;
        storageListener = mockAddListener.mock.calls[0]![0] as typeof storageListener;

        storageListener({ wikiUrl: { newValue: 'http://auto-wiki:8050' } }, 'local');
        await el.updateComplete;
      });

      afterEach(() => {
        vi.useRealTimers();
      });

      it('should set showAutoDetected to true', () => {
        expect(el.showAutoDetected).toBeTruthy();
      });

      describe('when 3 seconds have passed', () => {

        beforeEach(async () => {
          await vi.advanceTimersByTimeAsync(3000);
          await el.updateComplete;
        });

        it('should clear showAutoDetected', () => {
          expect(el.showAutoDetected).toBeFalsy();
        });
      });
    });

    describe('when wikiUrl changes and isManuallySet is true', () => {
      let storageListener: (changes: Record<string, { newValue?: unknown }>, areaName: string) => void;

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({ wikiUrl: 'http://manual:8050', wikiUrlManuallySet: true });
        el = createSettingsPanel();
        await el.updateComplete;
        await vi.waitFor(() => {
          expect(el.isManuallySet).toBeTruthy();
        });
        storageListener = mockAddListener.mock.calls[0]![0] as typeof storageListener;

        storageListener({ wikiUrl: { newValue: 'http://updated:8050' } }, 'local');
        await el.updateComplete;
      });

      it('should update wikiUrl', () => {
        expect(el.wikiUrl).to.equal('http://updated:8050');
      });

      it('should not set showAutoDetected', () => {
        expect(el.showAutoDetected).toBeFalsy();
      });
    });

    describe('when wikiUrlManuallySet changes', () => {
      let storageListener: (changes: Record<string, { newValue?: unknown }>, areaName: string) => void;

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({});
        el = createSettingsPanel();
        await el.updateComplete;
        storageListener = mockAddListener.mock.calls[0]![0] as typeof storageListener;

        storageListener({ wikiUrlManuallySet: { newValue: true } }, 'local');
        await el.updateComplete;
      });

      it('should update isManuallySet', () => {
        expect(el.isManuallySet).toBeTruthy();
      });
    });
  });

  describe('_handleInput', () => {

    beforeEach(async () => {
      vi.clearAllMocks();
      mockGet.mockResolvedValue({});
      el = createSettingsPanel();
      await el.updateComplete;

      const input = el.shadowRoot!.querySelector('input')!;
      input.value = 'http://typed-url:8080';
      input.dispatchEvent(new Event('input'));
      await el.updateComplete;
    });

    it('should update wikiUrl from input value', () => {
      expect(el.wikiUrl).to.equal('http://typed-url:8080');
    });
  });

  describe('render', () => {

    describe('when showSaved is true', () => {

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({});
        el = createSettingsPanel();
        await el.updateComplete;
        el.showSaved = true;
        await el.updateComplete;
      });

      it('should render the saved status message', () => {
        const savedMsg = el.shadowRoot!.querySelector('.status-msg.saved');
        expect(savedMsg).to.not.be.null;
        expect(savedMsg!.textContent).to.equal('Saved (manual)');
      });
    });

    describe('when showSaved is false', () => {

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({});
        el = createSettingsPanel();
        await el.updateComplete;
      });

      it('should not render the saved status message', () => {
        const savedMsg = el.shadowRoot!.querySelector('.status-msg.saved');
        expect(savedMsg).to.be.null;
      });
    });

    describe('when showAutoDetected is true', () => {

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({});
        el = createSettingsPanel();
        await el.updateComplete;
        el.showAutoDetected = true;
        await el.updateComplete;
      });

      it('should render the auto-detected status message', () => {
        const autoMsg = el.shadowRoot!.querySelector('.status-msg.auto-detected');
        expect(autoMsg).to.not.be.null;
        expect(autoMsg!.textContent).to.equal('Auto-detected from wiki');
      });
    });

    describe('when showAutoDetected is false', () => {

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({});
        el = createSettingsPanel();
        await el.updateComplete;
      });

      it('should not render the auto-detected status message', () => {
        const autoMsg = el.shadowRoot!.querySelector('.status-msg.auto-detected');
        expect(autoMsg).to.be.null;
      });
    });

    describe('when isManuallySet is true', () => {

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({ wikiUrl: 'http://test:8050', wikiUrlManuallySet: true });
        el = createSettingsPanel();
        await el.updateComplete;
        await vi.waitFor(() => {
          expect(el.isManuallySet).toBeTruthy();
        });
        await el.updateComplete;
      });

      it('should render the reset-to-auto-detect button', () => {
        const resetBtn = el.shadowRoot!.querySelector('.reset-link');
        expect(resetBtn).to.not.be.null;
        expect(resetBtn!.textContent).to.equal('Reset to auto-detect');
      });
    });

    describe('when isManuallySet is false', () => {

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({});
        el = createSettingsPanel();
        await el.updateComplete;
      });

      it('should not render the reset-to-auto-detect button', () => {
        const resetBtn = el.shadowRoot!.querySelector('.reset-link');
        expect(resetBtn).to.be.null;
      });
    });

    describe('when component is rendered with defaults', () => {

      beforeEach(async () => {
        vi.clearAllMocks();
        mockGet.mockResolvedValue({});
        el = createSettingsPanel();
        await el.updateComplete;
      });

      it('should render a label with text Wiki URL:', () => {
        const label = el.shadowRoot!.querySelector('label');
        expect(label).to.not.be.null;
        expect(label!.textContent).to.equal('Wiki URL:');
      });

      it('should render an input with the default URL', () => {
        const input = el.shadowRoot!.querySelector('input');
        expect(input).to.not.be.null;
        expect(input!.value).to.equal('http://localhost:8050');
      });

      it('should render a Save button', () => {
        const button = el.shadowRoot!.querySelector('button');
        expect(button).to.not.be.null;
        expect(button!.textContent).to.equal('Save');
      });
    });
  });
});
