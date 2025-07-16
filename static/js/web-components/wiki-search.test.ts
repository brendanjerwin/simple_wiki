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

  describe('keyboard shortcuts', () => {
    let searchInput: HTMLInputElement;
    let focusSpy: sinon.SinonSpy;

    beforeEach(async () => {
      await el.updateComplete;
      searchInput = el.shadowRoot?.querySelector('input[type="search"]') as HTMLInputElement;
      focusSpy = sinon.spy(searchInput, 'focus');
    });

    it('should focus input when Ctrl+K is pressed', () => {
      const mockEvent = new KeyboardEvent('keydown', {
        ctrlKey: true,
        key: 'k',
        bubbles: true,
        cancelable: true,
      });
      
      // Call the handler directly instead of dispatching to window
      el._handleKeydown(mockEvent);
      
      expect(mockEvent.defaultPrevented).to.be.true;
      expect(focusSpy).to.have.been.calledOnce;
    });

    it('should focus input when Cmd+K is pressed (Mac)', () => {
      const mockEvent = new KeyboardEvent('keydown', {
        metaKey: true,
        key: 'k',
        bubbles: true,
        cancelable: true,
      });
      
      // Call the handler directly instead of dispatching to window
      el._handleKeydown(mockEvent);
      
      expect(mockEvent.defaultPrevented).to.be.true;
      expect(focusSpy).to.have.been.calledOnce;
    });

    it('should not focus input when wrong key combination is pressed', () => {
      const mockEvent = new KeyboardEvent('keydown', {
        ctrlKey: true,
        key: 'j',
        bubbles: true,
        cancelable: true,
      });
      
      // Call the handler directly instead of dispatching to window
      el._handleKeydown(mockEvent);
      
      expect(mockEvent.defaultPrevented).to.be.false;
      expect(focusSpy).to.not.have.been.called;
    });
  });

  describe('event listener wiring', () => {
    it('should add keydown event listener when connected', () => {
      const addEventListenerSpy = sinon.spy(window, 'addEventListener');
      
      // Create a new element to trigger connectedCallback
      const newEl = document.createElement('wiki-search');
      document.body.appendChild(newEl);
      
      expect(addEventListenerSpy).to.have.been.calledWith('keydown', sinon.match.func);
      
      // Clean up
      document.body.removeChild(newEl);
      addEventListenerSpy.restore();
    });

    it('should remove keydown event listener when disconnected', () => {
      const removeEventListenerSpy = sinon.spy(window, 'removeEventListener');
      
      // Create and remove an element to trigger disconnectedCallback
      const newEl = document.createElement('wiki-search');
      document.body.appendChild(newEl);
      document.body.removeChild(newEl);
      
      expect(removeEventListenerSpy).to.have.been.calledWith('keydown', sinon.match.func);
      
      removeEventListenerSpy.restore();
    });
  });
});