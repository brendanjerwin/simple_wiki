import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { fixture, html } from '@lit-labs/testing';
import { WikiSearch } from './wiki-search.js';

describe('WikiSearch', () => {
  let wikiSearch;
  
  beforeEach(async () => {
    // Setup fresh component for each test
    wikiSearch = new WikiSearch();
  });
  
  afterEach(() => {
    // Clean up any DOM elements
    if (wikiSearch && wikiSearch.parentNode) {
      wikiSearch.parentNode.removeChild(wikiSearch);
    }
  });

  it('should exist', () => {
    expect(WikiSearch).toBeDefined();
  });

  describe('constructor', () => {
    it('should bind the keydown handler', () => {
      expect(wikiSearch._handleKeydown).toBeDefined();
      expect(typeof wikiSearch._handleKeydown).toBe('function');
    });

    it('should initialize with default properties', () => {
      expect(wikiSearch.resultArrayPath).toBe('results');
      expect(wikiSearch.results).toEqual([]);
    });
  });

  describe('when component is connected to DOM', () => {
    let addEventListenerSpy;
    
    beforeEach(() => {
      addEventListenerSpy = vi.spyOn(window, 'addEventListener');
      // Use a more controlled approach to connect the component
      wikiSearch.connectedCallback();
    });
    
    afterEach(() => {
      vi.restoreAllMocks();
    });

    it('should add keydown event listener', () => {
      expect(addEventListenerSpy).toHaveBeenCalledWith('keydown', wikiSearch._handleKeydown);
    });
  });

  describe('when component is disconnected from DOM', () => {
    let removeEventListenerSpy;
    
    beforeEach(() => {
      removeEventListenerSpy = vi.spyOn(window, 'removeEventListener');
      // First connect then disconnect
      wikiSearch.connectedCallback();
      wikiSearch.disconnectedCallback();
    });
    
    afterEach(() => {
      vi.restoreAllMocks();
    });

    it('should remove keydown event listener', () => {
      expect(removeEventListenerSpy).toHaveBeenCalledWith('keydown', wikiSearch._handleKeydown);
    });
  });

  describe('when keydown event is triggered', () => {
    let mockInput;
    let mockEvent;
    
    beforeEach(() => {
      // Create mock input element
      mockInput = document.createElement('input');
      mockInput.type = 'search';
      mockInput.focus = vi.fn();
      
      // Mock the querySelector method directly
      vi.spyOn(wikiSearch, 'shadowRoot', 'get').mockReturnValue({
        querySelector: vi.fn().mockReturnValue(mockInput)
      });
    });
    
    afterEach(() => {
      vi.restoreAllMocks();
    });

    describe('when Ctrl+K is pressed', () => {
      beforeEach(() => {
        mockEvent = {
          ctrlKey: true,
          key: 'k',
          preventDefault: vi.fn()
        };
        
        wikiSearch._handleKeydown(mockEvent);
      });

      it('should prevent default behavior', () => {
        expect(mockEvent.preventDefault).toHaveBeenCalled();
      });

      it('should focus the search input', () => {
        expect(mockInput.focus).toHaveBeenCalled();
      });
    });

    describe('when Cmd+K is pressed (Mac)', () => {
      beforeEach(() => {
        mockEvent = {
          metaKey: true,
          key: 'k',
          preventDefault: vi.fn()
        };
        
        wikiSearch._handleKeydown(mockEvent);
      });

      it('should prevent default behavior', () => {
        expect(mockEvent.preventDefault).toHaveBeenCalled();
      });

      it('should focus the search input', () => {
        expect(mockInput.focus).toHaveBeenCalled();
      });
    });

    describe('when wrong key combination is pressed', () => {
      beforeEach(() => {
        mockEvent = {
          ctrlKey: true,
          key: 'j',
          preventDefault: vi.fn()
        };
        
        wikiSearch._handleKeydown(mockEvent);
      });

      it('should not prevent default behavior', () => {
        expect(mockEvent.preventDefault).not.toHaveBeenCalled();
      });

      it('should not focus the search input', () => {
        expect(mockInput.focus).not.toHaveBeenCalled();
      });
    });
  });

  describe('when component undergoes multiple connect/disconnect cycles', () => {
    let addEventListenerSpy;
    let removeEventListenerSpy;
    
    beforeEach(() => {
      addEventListenerSpy = vi.spyOn(window, 'addEventListener');
      removeEventListenerSpy = vi.spyOn(window, 'removeEventListener');
      
      // Test multiple connect/disconnect cycles
      for (let i = 0; i < 3; i++) {
        wikiSearch.connectedCallback();
        wikiSearch.disconnectedCallback();
      }
    });
    
    afterEach(() => {
      vi.restoreAllMocks();
    });

    it('should add event listener for each connect', () => {
      expect(addEventListenerSpy).toHaveBeenCalledTimes(3);
    });

    it('should remove event listener for each disconnect', () => {
      expect(removeEventListenerSpy).toHaveBeenCalledTimes(3);
    });

    it('should use the same bound function for all event listeners', () => {
      const addCalls = addEventListenerSpy.mock.calls.filter(call => call[0] === 'keydown');
      const removeCalls = removeEventListenerSpy.mock.calls.filter(call => call[0] === 'keydown');
      
      expect(addCalls.length).toBe(3);
      expect(removeCalls.length).toBe(3);
      
      // Verify it's the same function reference
      const firstHandler = addCalls[0][1];
      expect(addCalls.every(call => call[1] === firstHandler)).toBe(true);
      expect(removeCalls.every(call => call[1] === firstHandler)).toBe(true);
    });
  });
});