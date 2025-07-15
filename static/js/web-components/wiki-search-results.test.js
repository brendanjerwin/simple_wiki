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
      expect(el.open).to.equal(false);
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
    
    it('should add click event listener', () => {
      expect(addEventListenerSpy).to.have.been.calledWith('click', el._boundHandleClickOutside);
    });
  });

  describe('when component is disconnected from DOM', () => {
    let removeEventListenerSpy;
    
    beforeEach(async () => {
      removeEventListenerSpy = sinon.spy(document, 'removeEventListener');
      // Re-create and then remove the element to trigger disconnectedCallback
      el = await fixture(html`<wiki-search-results></wiki-search-results>`);
      await el.updateComplete;
      el.remove();
      // Wait for the next microtask to ensure disconnectedCallback runs
      await el.updateComplete;
    });
    
    it('should remove click event listener', () => {
      expect(removeEventListenerSpy).to.have.been.calledWith('click', el._boundHandleClickOutside);
    });
  });

  describe('when click event is triggered outside', () => {
    let closeSpy;
    
    beforeEach(() => {
      closeSpy = sinon.spy(el, 'close');
      el.open = true;
    });

    describe('when click is outside the popover', () => {
      beforeEach(() => {
        // Mock composedPath to return empty path (simulating click outside)
        const mockEvent = {
          composedPath: () => []
        };
        el.handleClickOutside(mockEvent);
      });

      it('should call close method', () => {
        expect(closeSpy).to.have.been.calledOnce;
      });
    });

    describe('when click is inside the popover', () => {
      beforeEach(() => {
        // Mock composedPath to include the popover element
        const popover = el.shadowRoot.querySelector('.popover');
        const mockEvent = {
          composedPath: () => [popover]
        };
        el.handleClickOutside(mockEvent);
      });

      it('should not call close method', () => {
        expect(closeSpy).to.not.have.been.called;
      });
    });
  });

  describe('when close method is called', () => {
    let dispatchEventSpy;
    
    beforeEach(() => {
      dispatchEventSpy = sinon.spy(el, 'dispatchEvent');
      el.close();
    });

    it('should dispatch search-results-closed event', () => {
      expect(dispatchEventSpy).to.have.been.calledOnce;
      const event = dispatchEventSpy.getCall(0).args[0];
      expect(event.type).to.equal('search-results-closed');
      expect(event.bubbles).to.be.true;
      expect(event.composed).to.be.true;
    });
  });
});