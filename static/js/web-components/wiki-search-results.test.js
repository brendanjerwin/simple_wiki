import { html, fixture, expect } from '@open-wc/testing';
import sinon from 'sinon';
import './wiki-search-results.js';

describe('WikiSearchResults', () => {
  let el;

  beforeEach(async () => {
    el = await fixture(html`<wiki-search-results></wiki-search-results>`);
    await el.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  describe('constructor', () => {
    it('should initialize with default properties', () => {
      expect(el.results).to.deep.equal([]);
      expect(el.open).to.be.false;
    });

    it('should bind the handleClickOutside handler', () => {
      expect(el.handleClickOutside).to.be.a('function');
    });
  });

  describe('when component is connected to DOM', () => {
    let addEventListenerSpy;
    
    beforeEach(async () => {
      addEventListenerSpy = sinon.spy(document, 'addEventListener');
      // Re-create the element to trigger connectedCallback
      el = await fixture(html`<wiki-search-results></wiki-search-results>`);
      await el.updateComplete;
    });
    
    it('should add click event listener to document', () => {
      expect(addEventListenerSpy).to.have.been.calledWith('click', el.handleClickOutside);
    });
  });

  describe('when component is disconnected from DOM', () => {
    let removeEventListenerSpy;
    let addEventListenerSpy;
    let boundHandler;
    
    beforeEach(async () => {
      addEventListenerSpy = sinon.spy(document, 'addEventListener');
      removeEventListenerSpy = sinon.spy(document, 'removeEventListener');
      
      // Re-create and then remove the element to trigger disconnectedCallback
      el = await fixture(html`<wiki-search-results></wiki-search-results>`);
      await el.updateComplete;
      
      // Get the bound handler that was actually added
      boundHandler = addEventListenerSpy.getCall(0).args[1];
      
      // Remove the element to trigger disconnectedCallback
      el.remove();
      // Wait for the next microtask to ensure disconnectedCallback runs
      await el.updateComplete;
    });
    
    it('should remove click event listener from document', () => {
      expect(removeEventListenerSpy).to.have.been.calledWith('click', boundHandler);
    });
  });

  describe('handleClickOutside', () => {
    beforeEach(async () => {
      el.open = true;
      await el.updateComplete;
    });

    it('should close when clicking outside the popover', () => {
      const closeStub = sinon.stub(el, 'close');
      const mockEvent = {
        composedPath: () => []
      };
      
      el.handleClickOutside(mockEvent);
      
      expect(closeStub).to.have.been.calledOnce;
    });

    it('should not close when clicking inside the popover', () => {
      const closeStub = sinon.stub(el, 'close');
      const popover = el.shadowRoot.querySelector('.popover');
      const mockEvent = {
        composedPath: () => [popover]
      };
      
      el.handleClickOutside(mockEvent);
      
      expect(closeStub).to.not.have.been.called;
    });

    it('should not close when component is not open', () => {
      el.open = false;
      const closeStub = sinon.stub(el, 'close');
      const mockEvent = {
        composedPath: () => []
      };
      
      el.handleClickOutside(mockEvent);
      
      expect(closeStub).to.not.have.been.called;
    });
  });
});