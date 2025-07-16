import { html, fixture, expect } from '@open-wc/testing';
import sinon from 'sinon';
import './wiki-search.js';

describe('WikiSearch', () => {
  let el: any;

  beforeEach(async () => {
    el = await fixture(html`<wiki-search></wiki-search>`);
    await el.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  describe('when component is initialized', () => {
    it('should have the correct default properties', () => {
      expect(el.resultArrayPath).to.equal('results');
      expect(el.results).to.deep.equal([]);
      expect(el.noResults).to.equal(false);
    });

    it('should have a search input', () => {
      const searchInput = el.shadowRoot?.querySelector('input[type="search"]');
      expect(searchInput).to.exist;
    });

    it('should have a submit button', () => {
      const submitButton = el.shadowRoot?.querySelector('button[type="submit"]');
      expect(submitButton).to.exist;
    });

    it('should have a search results component', () => {
      const searchResults = el.shadowRoot?.querySelector('wiki-search-results');
      expect(searchResults).to.exist;
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
      el = await fixture(html`<wiki-search></wiki-search>`);
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
      el = await fixture(html`<wiki-search></wiki-search>`);
      await el.updateComplete;
      el.remove();
      // Wait for the next microtask to ensure disconnectedCallback runs
      await el.updateComplete;
    });

    it('should remove keydown event listener', () => {
      expect(removeEventListenerSpy).to.have.been.calledWith('keydown', el._handleKeydown);
    });
  });
});