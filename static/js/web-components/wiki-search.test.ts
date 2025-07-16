import { html, fixture, expect } from '@open-wc/testing';
import sinon from 'sinon';
import { WikiSearch } from './wiki-search.js';

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

  describe('constructor', () => {

    it('should bind the keydown handler', () => {
      expect(el._handleKeydown).to.be.a('function');
    });

    it('should initialize with default properties', () => {
      expect(el.resultArrayPath).to.equal('results');
      expect(el.results).to.deep.equal([]);
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

  describe('when keydown event is triggered', () => {
    let searchInput: HTMLInputElement;
    let focusSpy: sinon.SinonSpy;
    let mockEvent: KeyboardEvent;
    
    beforeEach(async () => {
      // Wait for component to be fully rendered
      await el.updateComplete;
      searchInput = el.shadowRoot?.querySelector('input[type="search"]') as HTMLInputElement;
      if (searchInput) {
        focusSpy = sinon.spy(searchInput, 'focus');
      }
    });

    describe('when Ctrl+K is pressed', () => {
      beforeEach(() => {
        if (searchInput) {
          mockEvent = new KeyboardEvent('keydown', {
            ctrlKey: true,
            key: 'k',
            bubbles: true,
            cancelable: true,
          });
          window.dispatchEvent(mockEvent);
        }
      });

      it('should prevent default behavior', () => {
        if (searchInput && mockEvent) {
          expect(mockEvent.defaultPrevented).to.be.true;
        } else {
          expect.fail('Search input not found or event not created');
        }
      });

      it('should focus the search input', () => {
        if (searchInput && focusSpy) {
          expect(focusSpy).to.have.been.calledOnce;
        } else {
          expect.fail('Search input not found or focus spy not created');
        }
      });
    });

    describe('when Cmd+K is pressed (Mac)', () => {
      beforeEach(() => {
        if (searchInput) {
          mockEvent = new KeyboardEvent('keydown', {
            metaKey: true,
            key: 'k',
            bubbles: true,
            cancelable: true,
          });
          window.dispatchEvent(mockEvent);
        }
      });

      it('should prevent default behavior', () => {
        if (searchInput && mockEvent) {
          expect(mockEvent.defaultPrevented).to.be.true;
        } else {
          expect.fail('Search input not found or event not created');
        }
      });

      it('should focus the search input', () => {
        if (searchInput && focusSpy) {
          expect(focusSpy).to.have.been.calledOnce;
        } else {
          expect.fail('Search input not found or focus spy not created');
        }
      });
    });

    describe('when wrong key combination is pressed', () => {
      beforeEach(() => {
        if (searchInput) {
          mockEvent = new KeyboardEvent('keydown', {
            ctrlKey: true,
            key: 'j',
            bubbles: true,
            cancelable: true,
          });
          window.dispatchEvent(mockEvent);
        }
      });

      it('should not prevent default behavior', () => {
        if (searchInput && mockEvent) {
          expect(mockEvent.defaultPrevented).to.be.false;
        } else {
          expect.fail('Search input not found or event not created');
        }
      });

      it('should not focus the search input', () => {
        if (searchInput && focusSpy) {
          expect(focusSpy).to.not.have.been.called;
        } else {
          expect.fail('Search input not found or focus spy not created');
        }
      });
    });
  });
});