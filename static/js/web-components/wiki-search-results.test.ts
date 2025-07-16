import { html, fixture, expect } from '@open-wc/testing';
import sinon from 'sinon';
import './wiki-search-results.js';

describe('WikiSearchResults', () => {
  let el: any;

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

  describe('when component is initialized', () => {
    it('should have default properties', () => {
      expect(el.results).to.deep.equal([]);
      expect(el.open).to.equal(false);
    });

    it('should have the click handler bound', () => {
      expect(el._handleClickOutside).to.be.a('function');
    });
  });

  describe('when component is connected to DOM', () => {
    let addEventListenerSpy: sinon.SinonSpy;
    
    beforeEach(async () => {
      addEventListenerSpy = sinon.spy(document, 'addEventListener');
      // Re-create the element to trigger connectedCallback
      el = await fixture(html`<wiki-search-results></wiki-search-results>`);
      await el.updateComplete;
    });
    
    it('should add click event listener', () => {
      expect(addEventListenerSpy).to.have.been.calledWith('click', el._handleClickOutside);
    });
  });

  describe('when component is disconnected from DOM', () => {
    let removeEventListenerSpy: sinon.SinonSpy;
    
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
      expect(removeEventListenerSpy).to.have.been.calledWith('click', el._handleClickOutside);
    });
  });

  describe('when clicking outside the popover', () => {
    let closeSpy: sinon.SinonSpy;
    let mockEvent: any;
    
    beforeEach(async () => {
      el.open = true;
      el.results = [{ Identifier: 'test', Title: 'Test', FragmentHTML: 'Test content' }];
      await el.updateComplete;
      closeSpy = sinon.spy(el, 'close');
      
      // Simulate clicking outside (event won't include the popover)
      mockEvent = {
        composedPath: () => []
      };
      el._handleClickOutside(mockEvent);
    });

    it('should close the popover', () => {
      expect(closeSpy).to.have.been.calledOnce;
    });
  });

  describe('when clicking inside the popover', () => {
    let closeSpy: sinon.SinonSpy;
    let mockEvent: any;
    
    beforeEach(async () => {
      el.open = true;
      el.results = [{ Identifier: 'test', Title: 'Test', FragmentHTML: 'Test content' }];
      await el.updateComplete;
      closeSpy = sinon.spy(el, 'close');
      
      // Simulate clicking inside (event includes the popover)
      const mockPopover = el.shadowRoot.querySelector('.popover');
      mockEvent = {
        composedPath: () => [mockPopover]
      };
      el._handleClickOutside(mockEvent);
    });

    it('should not close the popover', () => {
      expect(closeSpy).to.not.have.been.called;
    });
  });
});