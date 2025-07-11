import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { csrFixture, cleanupFixtures } from '@lit-labs/testing/fixtures.js';
import { html } from 'lit';
import { WikiSearch } from '../web-components/wiki-search.js';

describe('WikiSearch', () => {
  let wikiSearch;
  
  beforeEach(() => {
    // Setup fresh component for each test
    wikiSearch = new WikiSearch();
  });
  
  afterEach(() => {
    cleanupFixtures();
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
      document.body.appendChild(wikiSearch);
    });
    
    afterEach(() => {
      if (wikiSearch.parentNode) {
        wikiSearch.parentNode.removeChild(wikiSearch);
      }
      addEventListenerSpy.mockRestore();
    });

    it('should add keydown event listener', () => {
      expect(addEventListenerSpy).toHaveBeenCalledWith('keydown', wikiSearch._handleKeydown);
    });
  });

  describe('when component is disconnected from DOM', () => {
    let removeEventListenerSpy;
    
    beforeEach(() => {
      removeEventListenerSpy = vi.spyOn(window, 'removeEventListener');
      document.body.appendChild(wikiSearch);
      document.body.removeChild(wikiSearch);
    });
    
    afterEach(() => {
      removeEventListenerSpy.mockRestore();
    });

    it('should remove keydown event listener', () => {
      expect(removeEventListenerSpy).toHaveBeenCalledWith('keydown', wikiSearch._handleKeydown);
    });
  });

  describe('when keydown event is triggered', () => {
    let mockInput;
    let mockEvent;
    
    beforeEach(() => {
      // Ensure component is connected and has shadowRoot
      document.body.appendChild(wikiSearch);
      
      // Create mock input element
      mockInput = document.createElement('input');
      mockInput.type = 'search';
      mockInput.focus = vi.fn();
      
      // Mock querySelector to return our mock input
      vi.spyOn(wikiSearch.shadowRoot, 'querySelector').mockReturnValue(mockInput);
    });
    
    afterEach(() => {
      // Clean up
      if (wikiSearch.parentNode) {
        wikiSearch.parentNode.removeChild(wikiSearch);
      }
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
        document.body.appendChild(wikiSearch);
        document.body.removeChild(wikiSearch);
      }
    });
    
    afterEach(() => {
      addEventListenerSpy.mockRestore();
      removeEventListenerSpy.mockRestore();
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