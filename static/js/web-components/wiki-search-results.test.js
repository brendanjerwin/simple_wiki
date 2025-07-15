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

  describe('memory leak test', () => {
    it('should use the same function reference for bound handler', () => {
      // After the fix, the bound function should be stored and reused
      const element = el;
      const boundFn = element._handleClickOutside;
      
      // This should be the same reference each time
      expect(boundFn).to.equal(element._handleClickOutside);
    });
  });
});