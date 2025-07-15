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

    it('should bind the click handler', () => {
      expect(el._handleClickOutside).to.be.a('function');
    });
  });

  describe('functionality verification', () => {
    it('should close when clicking outside', async () => {
      el.open = true;
      el.results = [{ Identifier: 'test', Title: 'Test', FragmentHTML: 'Test content' }];
      await el.updateComplete;

      // Mock the close method
      const closeSpy = sinon.spy(el, 'close');
      
      // Simulate clicking outside (event won't include the popover)
      const mockEvent = {
        composedPath: () => []
      };
      
      el._handleClickOutside(mockEvent);
      
      expect(closeSpy).to.have.been.calledOnce;
    });
    
    it('should not close when clicking inside popover', async () => {
      el.open = true;
      el.results = [{ Identifier: 'test', Title: 'Test', FragmentHTML: 'Test content' }];
      await el.updateComplete;

      // Mock the close method
      const closeSpy = sinon.spy(el, 'close');
      
      // Simulate clicking inside (event includes the popover)
      const mockPopover = el.shadowRoot.querySelector('.popover');
      const mockEvent = {
        composedPath: () => [mockPopover]
      };
      
      el._handleClickOutside(mockEvent);
      
      expect(closeSpy).to.not.have.been.called;
    });
  });
});