import { expect, waitUntil } from '@open-wc/testing';
import sinon from 'sinon';
import './wiki-search.js';
import type { SearchResult } from '../gen/api/v1/search_pb.js';

interface WikiSearchElement extends HTMLElement {
  results: SearchResult[];
  noResults: boolean;
  loading: boolean;
  error?: string;
  _handleKeydown: (event: KeyboardEvent) => void;
  handleFormSubmit: (event: Event) => Promise<void>;
  performSearch: (query: string) => Promise<SearchResult[]>;
  updateComplete: Promise<boolean>;
  shadowRoot: ShadowRoot;
}

describe('WikiSearch', () => {
  let el: WikiSearchElement;

  beforeEach(async () => {
    el = document.createElement('wiki-search') as WikiSearchElement;
    document.body.appendChild(el);
    await el.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
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
      expect(el.error).to.be.undefined;
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
    let searchInput: HTMLInputElement;
    let focusSpy: sinon.SinonSpy;
    let mockEvent: KeyboardEvent;

    beforeEach(async () => {
      await el.updateComplete;
      searchInput = el.shadowRoot?.querySelector('input[type="search"]') as HTMLInputElement;
      focusSpy = sinon.spy(searchInput, 'focus');
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
    let searchInput: HTMLInputElement;
    let focusSpy: sinon.SinonSpy;
    let mockEvent: KeyboardEvent;

    beforeEach(async () => {
      await el.updateComplete;
      searchInput = el.shadowRoot?.querySelector('input[type="search"]') as HTMLInputElement;
      focusSpy = sinon.spy(searchInput, 'focus');
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
    let searchInput: HTMLInputElement;
    let focusSpy: sinon.SinonSpy;
    let mockEvent: KeyboardEvent;

    beforeEach(async () => {
      await el.updateComplete;
      searchInput = el.shadowRoot?.querySelector('input[type="search"]') as HTMLInputElement;
      focusSpy = sinon.spy(searchInput, 'focus');
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
    let form: HTMLFormElement;
    let searchInput: HTMLInputElement;

    beforeEach(async () => {
      await el.updateComplete;
      form = el.shadowRoot?.querySelector('form') as HTMLFormElement;
      searchInput = el.shadowRoot?.querySelector('input[type="search"]') as HTMLInputElement;
    });

    describe('when form is submitted', () => {
      beforeEach(() => {
        searchInput.value = 'test query';
        
        const submitEvent = new Event('submit', { bubbles: true, cancelable: true });
        form.dispatchEvent(submitEvent);
      });

      it('should prevent default form submission', () => {
        const submitEvent = new Event('submit', { bubbles: true, cancelable: true });
        const preventDefaultSpy = sinon.spy(submitEvent, 'preventDefault');
        
        form.dispatchEvent(submitEvent);
        expect(preventDefaultSpy).to.have.been.calledOnce;
      });

      it('should set loading state during search', () => {
        expect(el.loading).to.be.true;
      });
    });

    describe('when search input is empty', () => {
      beforeEach(() => {
        searchInput.value = '';
        const submitEvent = new Event('submit', { bubbles: true, cancelable: true });
        form.dispatchEvent(submitEvent);
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

        searchInput.value = 'fail';
        const submitEvent = new Event('submit', { bubbles: true, cancelable: true });
        form.dispatchEvent(submitEvent);

        await waitUntil(() => el.error === 'Network error', 'Error should be set');
        await el.updateComplete;
      });

      afterEach(() => {
        stubPerformSearch.restore();
      });

      it('should set error property', () => {
        expect(el.error).to.equal('Network error');
      });

      it('should display error message', () => {
        const errorDiv = el.shadowRoot?.querySelector('.error');
        expect(errorDiv).to.exist;
        expect(errorDiv?.textContent).to.equal('Network error');
      });
    });
  });
});