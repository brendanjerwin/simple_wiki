import { expect, waitUntil } from '@open-wc/testing';
import sinon from 'sinon';
import './wiki-search.js';
import type { SearchResult } from '../gen/api/v1/search_pb.js';

const INVENTORY_ONLY_STORAGE_KEY = 'wiki-search-inventory-only';

interface WikiSearchElement extends HTMLElement {
  results: SearchResult[];
  noResults: boolean;
  loading: boolean;
  error: Error | null;
  inventoryOnly: boolean;
  totalUnfilteredCount: number;
  _handleKeydown: (event: KeyboardEvent) => void;
  handleFormSubmit: (event: Event) => Promise<void>;
  handleInventoryFilterChanged: (event: CustomEvent<{ inventoryOnly: boolean }>) => Promise<void>;
  performSearch: (query: string) => Promise<{ results: SearchResult[], totalUnfilteredCount: number }>;
  updateComplete: Promise<boolean>;
  shadowRoot: ShadowRoot;
}

describe('WikiSearch', () => {
  let el: WikiSearchElement;

  beforeEach(async () => {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- custom element matches interface
    el = document.createElement('wiki-search') as WikiSearchElement;
    document.body.appendChild(el);
    await el.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
    localStorage.removeItem(INVENTORY_ONLY_STORAGE_KEY);
    if (el.parentNode) {
      el.parentNode.removeChild(el);
    }
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  describe('when component is initialized', () => {
    it('should have the correct default properties', () => {
      expect(el.results).to.deep.equal([]);
      expect(el.noResults).to.equal(false);
      expect(el.loading).to.equal(false);
      expect(el.error).to.be.null;
      expect(el.inventoryOnly).to.equal(false);
    });

    it('should have a search input', () => {
      const searchInput = el.shadowRoot?.querySelector('input[type="search"]');
      expect(searchInput).to.exist;
    });

    it('should have a submit button', () => {
      const submitButton = el.shadowRoot?.querySelector('button[type="submit"]');
      expect(submitButton).to.exist;
    });

    it('should have the performSearch method', () => {
      expect(el.performSearch).to.be.a('function');
    });
  });

  describe('when Ctrl+K is pressed', () => {
    let searchInput: HTMLInputElement | null;
    let focusSpy: sinon.SinonSpy;
    let mockEvent: KeyboardEvent;

    beforeEach(async () => {
      await el.updateComplete;
      searchInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[type="search"]') ?? null;
      if (searchInput) {
        focusSpy = sinon.spy(searchInput, 'focus');
      }
      mockEvent = new KeyboardEvent('keydown', {
        ctrlKey: true,
        key: 'k',
        bubbles: true,
        cancelable: true,
      });

      el._handleKeydown(mockEvent);
    });

    it('should prevent default behavior', () => {
      expect(mockEvent.defaultPrevented).to.be.true;
    });

    it('should focus the search input', () => {
      expect(focusSpy).to.have.been.calledOnce;
    });
  });

  describe('when Cmd+K is pressed (Mac)', () => {
    let searchInput: HTMLInputElement | null;
    let focusSpy: sinon.SinonSpy;
    let mockEvent: KeyboardEvent;

    beforeEach(async () => {
      await el.updateComplete;
      searchInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[type="search"]') ?? null;
      if (searchInput) {
        focusSpy = sinon.spy(searchInput, 'focus');
      }
      mockEvent = new KeyboardEvent('keydown', {
        metaKey: true,
        key: 'k',
        bubbles: true,
        cancelable: true,
      });

      el._handleKeydown(mockEvent);
    });

    it('should prevent default behavior', () => {
      expect(mockEvent.defaultPrevented).to.be.true;
    });

    it('should focus the search input', () => {
      expect(focusSpy).to.have.been.calledOnce;
    });
  });

  describe('when wrong key combination is pressed', () => {
    let searchInput: HTMLInputElement | null;
    let focusSpy: sinon.SinonSpy;
    let mockEvent: KeyboardEvent;

    beforeEach(async () => {
      await el.updateComplete;
      searchInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[type="search"]') ?? null;
      if (searchInput) {
        focusSpy = sinon.spy(searchInput, 'focus');
      }
      mockEvent = new KeyboardEvent('keydown', {
        ctrlKey: true,
        key: 'j',
        bubbles: true,
        cancelable: true,
      });

      el._handleKeydown(mockEvent);
    });

    it('should not prevent default behavior', () => {
      expect(mockEvent.defaultPrevented).to.be.false;
    });

    it('should not focus the search input', () => {
      expect(focusSpy).to.not.have.been.called;
    });
  });

  describe('when component is connected to DOM', () => {
    let addEventListenerSpy: sinon.SinonSpy;

    beforeEach(async () => {
      addEventListenerSpy = sinon.spy(window, 'addEventListener');
      // Re-create the element to trigger connectedCallback
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- custom element matches interface
      el = document.createElement('wiki-search') as WikiSearchElement;
      document.body.appendChild(el);
      await el.updateComplete;
    });

    it('should add keydown event listener', () => {
      expect(addEventListenerSpy).to.have.been.calledWith('keydown', el._handleKeydown);
    });
  });

  describe('when component is disconnected from DOM', () => {
    let removeEventListenerSpy: sinon.SinonSpy;

    beforeEach(async () => {
      removeEventListenerSpy = sinon.spy(window, 'removeEventListener');
      // Re-create and then remove the element to trigger disconnectedCallback
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- custom element matches interface
      el = document.createElement('wiki-search') as WikiSearchElement;
      document.body.appendChild(el);
      await el.updateComplete;
      el.remove();
      // Wait for the next microtask to ensure disconnectedCallback runs
      await el.updateComplete;
    });

    it('should remove keydown event listener', () => {
      expect(removeEventListenerSpy).to.have.been.calledWith('keydown', el._handleKeydown);
    });
  });

  describe('when searching', () => {
    let form: HTMLFormElement | null;
    let searchInput: HTMLInputElement | null;

    beforeEach(async () => {
      await el.updateComplete;
      form = el.shadowRoot?.querySelector<HTMLFormElement>('form') ?? null;
      searchInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[type="search"]') ?? null;
    });

    describe('when form is submitted', () => {
      beforeEach(() => {
        if (searchInput) searchInput.value = 'test query';

        const submitEvent = new Event('submit', { bubbles: true, cancelable: true });
        form?.dispatchEvent(submitEvent);
      });

      it('should prevent default form submission', () => {
        const submitEvent = new Event('submit', { bubbles: true, cancelable: true });
        const preventDefaultSpy = sinon.spy(submitEvent, 'preventDefault');

        form?.dispatchEvent(submitEvent);
        expect(preventDefaultSpy).to.have.been.calledOnce;
      });

      it('should set loading state during search', () => {
        expect(el.loading).to.be.true;
      });
    });

    describe('when search input is empty', () => {
      beforeEach(() => {
        if (searchInput) searchInput.value = '';
        const submitEvent = new Event('submit', { bubbles: true, cancelable: true });
        form?.dispatchEvent(submitEvent);
      });

      it('should not perform search', () => {
        expect(el.loading).to.be.false;
      });
    });

    describe('when search fails', () => {
      let stubPerformSearch: sinon.SinonStub;

      beforeEach(async () => {
        stubPerformSearch = sinon.stub(el, 'performSearch');
        stubPerformSearch.rejects(new Error('Network error'));

        if (searchInput) searchInput.value = 'fail';
        const submitEvent = new Event('submit', { bubbles: true, cancelable: true });
        form?.dispatchEvent(submitEvent);

        await waitUntil(() => el.error?.message === 'Network error', 'Error should be set');
        await el.updateComplete;
      });

      afterEach(() => {
        stubPerformSearch.restore();
      });

      it('should set error property', () => {
        expect(el.error?.message).to.equal('Network error');
      });

      it('should display error message', () => {
        const errorDiv = el.shadowRoot?.querySelector('.error');
        expect(errorDiv).to.exist;
        expect(errorDiv?.textContent).to.equal('Network error');
      });

      it('should reset totalUnfilteredCount to 0', () => {
        expect(el.totalUnfilteredCount).to.equal(0);
      });
    });

    describe('when search fails with stale totalUnfilteredCount', () => {
      let stubPerformSearch: sinon.SinonStub;

      beforeEach(async () => {
        stubPerformSearch = sinon.stub(el, 'performSearch');

        // First search succeeds with a totalUnfilteredCount
        stubPerformSearch.resolves({ results: [{ identifier: 'test', title: 'Test' }], totalUnfilteredCount: 10 });
        if (searchInput) searchInput.value = 'success';
        let submitEvent = new Event('submit', { bubbles: true, cancelable: true });
        form?.dispatchEvent(submitEvent);
        await waitUntil(() => !el.loading, 'First search should complete');
        await el.updateComplete;

        // Verify the totalUnfilteredCount is set
        expect(el.totalUnfilteredCount).to.equal(10);

        // Second search fails
        stubPerformSearch.rejects(new Error('Network error'));
        if (searchInput) searchInput.value = 'fail';
        submitEvent = new Event('submit', { bubbles: true, cancelable: true });
        form?.dispatchEvent(submitEvent);
        await waitUntil(() => el.error?.message === 'Network error', 'Error should be set');
        await el.updateComplete;
      });

      afterEach(() => {
        stubPerformSearch.restore();
      });

      it('should reset totalUnfilteredCount to 0 even if it had a previous value', () => {
        expect(el.totalUnfilteredCount).to.equal(0);
      });

      it('should clear results', () => {
        expect(el.results).to.deep.equal([]);
      });

      it('should set error property', () => {
        expect(el.error?.message).to.equal('Network error');
      });
    });
  });

  describe('when inventory-filter-changed event is received', () => {
    let stubPerformSearch: sinon.SinonStub;
    let form: HTMLFormElement | null;
    let searchInput: HTMLInputElement | null;

    beforeEach(async () => {
      await el.updateComplete;
      form = el.shadowRoot?.querySelector<HTMLFormElement>('form') ?? null;
      searchInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[type="search"]') ?? null;

      // Stub performSearch to return mock results
      stubPerformSearch = sinon.stub(el, 'performSearch');
      stubPerformSearch.resolves({ results: [], totalUnfilteredCount: 0 });
    });

    afterEach(() => {
      stubPerformSearch.restore();
    });

    describe('when there is a previous search query', () => {
      beforeEach(async () => {
        // Perform an initial search
        if (searchInput) searchInput.value = 'test query';
        const submitEvent = new Event('submit', { bubbles: true, cancelable: true });
        form?.dispatchEvent(submitEvent);
        await waitUntil(() => !el.loading, 'Loading should complete');

        // Reset the stub to track the re-search
        stubPerformSearch.resetHistory();

        // Trigger the inventory filter change
        const event = new CustomEvent('inventory-filter-changed', {
          detail: { inventoryOnly: true },
          bubbles: true,
          composed: true
        });
        await el.handleInventoryFilterChanged(event);
        await el.updateComplete;
      });

      it('should set inventoryOnly to true', () => {
        expect(el.inventoryOnly).to.equal(true);
      });

      it('should re-run the search', () => {
        expect(stubPerformSearch).to.have.been.calledWith('test query');
      });
    });

    describe('when there is no previous search query', () => {
      beforeEach(async () => {
        // Don't perform an initial search - just trigger the filter change
        const event = new CustomEvent('inventory-filter-changed', {
          detail: { inventoryOnly: true },
          bubbles: true,
          composed: true
        });
        await el.handleInventoryFilterChanged(event);
        await el.updateComplete;
      });

      it('should set inventoryOnly to true', () => {
        expect(el.inventoryOnly).to.equal(true);
      });

      it('should not perform a search', () => {
        expect(stubPerformSearch).to.not.have.been.called;
      });
    });

    describe('when search fails during inventory filter change', () => {
      beforeEach(async () => {
        // Perform an initial search with some results
        stubPerformSearch.resolves({ results: [{ identifier: 'test', title: 'Test' }], totalUnfilteredCount: 5 });
        if (searchInput) searchInput.value = 'test query';
        const submitEvent = new Event('submit', { bubbles: true, cancelable: true });
        form?.dispatchEvent(submitEvent);
        await waitUntil(() => !el.loading, 'Loading should complete');

        // Set up the stub to fail for the next call
        stubPerformSearch.rejects(new Error('Network error during filter change'));

        // Trigger the inventory filter change
        const event = new CustomEvent('inventory-filter-changed', {
          detail: { inventoryOnly: true },
          bubbles: true,
          composed: true
        });
        await el.handleInventoryFilterChanged(event);
        await waitUntil(() => el.error?.message === 'Network error during filter change', 'Error should be set');
        await el.updateComplete;
      });

      it('should set error property', () => {
        expect(el.error?.message).to.equal('Network error during filter change');
      });

      it('should clear results', () => {
        expect(el.results).to.deep.equal([]);
      });

      it('should reset totalUnfilteredCount to 0', () => {
        expect(el.totalUnfilteredCount).to.equal(0);
      });
    });
  });

  describe('localStorage persistence', () => {
    describe('when localStorage has inventoryOnly set to true', () => {
      let newEl: WikiSearchElement;

      beforeEach(async () => {
        localStorage.setItem(INVENTORY_ONLY_STORAGE_KEY, 'true');
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- custom element matches interface
        newEl = document.createElement('wiki-search') as WikiSearchElement;
        document.body.appendChild(newEl);
        await newEl.updateComplete;
      });

      afterEach(() => {
        if (newEl.parentNode) {
          newEl.parentNode.removeChild(newEl);
        }
      });

      it('should load inventoryOnly as true', () => {
        expect(newEl.inventoryOnly).to.equal(true);
      });
    });

    describe('when localStorage has inventoryOnly set to false', () => {
      let newEl: WikiSearchElement;

      beforeEach(async () => {
        localStorage.setItem(INVENTORY_ONLY_STORAGE_KEY, 'false');
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- custom element matches interface
        newEl = document.createElement('wiki-search') as WikiSearchElement;
        document.body.appendChild(newEl);
        await newEl.updateComplete;
      });

      afterEach(() => {
        if (newEl.parentNode) {
          newEl.parentNode.removeChild(newEl);
        }
      });

      it('should load inventoryOnly as false', () => {
        expect(newEl.inventoryOnly).to.equal(false);
      });
    });

    describe('when localStorage has no inventoryOnly value', () => {
      let newEl: WikiSearchElement;

      beforeEach(async () => {
        localStorage.removeItem(INVENTORY_ONLY_STORAGE_KEY);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- custom element matches interface
        newEl = document.createElement('wiki-search') as WikiSearchElement;
        document.body.appendChild(newEl);
        await newEl.updateComplete;
      });

      afterEach(() => {
        if (newEl.parentNode) {
          newEl.parentNode.removeChild(newEl);
        }
      });

      it('should default inventoryOnly to false', () => {
        expect(newEl.inventoryOnly).to.equal(false);
      });
    });

    describe('when inventory filter is changed to true', () => {
      beforeEach(async () => {
        localStorage.removeItem(INVENTORY_ONLY_STORAGE_KEY);

        const event = new CustomEvent('inventory-filter-changed', {
          detail: { inventoryOnly: true },
          bubbles: true,
          composed: true
        });
        await el.handleInventoryFilterChanged(event);
        await el.updateComplete;
      });

      it('should save true to localStorage', () => {
        expect(localStorage.getItem(INVENTORY_ONLY_STORAGE_KEY)).to.equal('true');
      });
    });

    describe('when inventory filter is changed to false', () => {
      beforeEach(async () => {
        localStorage.setItem(INVENTORY_ONLY_STORAGE_KEY, 'true');

        const event = new CustomEvent('inventory-filter-changed', {
          detail: { inventoryOnly: false },
          bubbles: true,
          composed: true
        });
        await el.handleInventoryFilterChanged(event);
        await el.updateComplete;
      });

      it('should save false to localStorage', () => {
        expect(localStorage.getItem(INVENTORY_ONLY_STORAGE_KEY)).to.equal('false');
      });
    });
  });
});