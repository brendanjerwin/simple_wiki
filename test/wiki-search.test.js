import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { WikiSearch } from '../static/js/web-components/wiki-search.js';

// Mock the lit-all.min.js import since it's not available in test environment
vi.mock('/static/vendor/js/lit-all.min.js', () => {
  // Create a minimal mock of LitElement
  class MockLitElement extends HTMLElement {
    static properties = {};
    static styles = {};
    
    constructor() {
      super();
      this.attachShadow({ mode: 'open' });
    }
    
    connectedCallback() {
      this.render();
    }
    
    disconnectedCallback() {
      // Override in implementations
    }
    
    render() {
      return '';
    }
    
    // Simple template literal handler
    static html(strings, ...values) {
      return strings.reduce((result, string, i) => {
        return result + string + (values[i] || '');
      }, '');
    }
    
    static css(strings, ...values) {
      return strings.reduce((result, string, i) => {
        return result + string + (values[i] || '');
      }, '');
    }
  }
  
  const html = MockLitElement.html;
  const css = MockLitElement.css;
  
  return {
    html,
    css,
    LitElement: MockLitElement
  };
});

describe('WikiSearch', () => {
  let wikiSearch;
  let mockEventListener;
  
  beforeEach(() => {
    // Clear any existing custom elements
    if (customElements.get('wiki-search')) {
      // Can't undefine, so we'll work with the existing one
    } else {
      customElements.define('wiki-search', WikiSearch);
    }
    
    // Create a new instance
    wikiSearch = new WikiSearch();
    
    // Mock addEventListener and removeEventListener
    mockEventListener = vi.fn();
    const originalAddEventListener = window.addEventListener;
    const originalRemoveEventListener = window.removeEventListener;
    
    // Track event listeners
    const eventListeners = new Map();
    
    window.addEventListener = vi.fn((type, listener, options) => {
      eventListeners.set(type, listener);
      return originalAddEventListener.call(window, type, listener, options);
    });
    
    window.removeEventListener = vi.fn((type, listener, options) => {
      eventListeners.delete(type);
      return originalRemoveEventListener.call(window, type, listener, options);
    });
    
    // Store references for cleanup
    wikiSearch._originalAddEventListener = originalAddEventListener;
    wikiSearch._originalRemoveEventListener = originalRemoveEventListener;
    wikiSearch._eventListeners = eventListeners;
  });
  
  afterEach(() => {
    // Clean up event listeners
    if (wikiSearch._eventListeners) {
      wikiSearch._eventListeners.clear();
    }
    
    // Restore original methods
    if (wikiSearch._originalAddEventListener) {
      window.addEventListener = wikiSearch._originalAddEventListener;
    }
    if (wikiSearch._originalRemoveEventListener) {
      window.removeEventListener = wikiSearch._originalRemoveEventListener;
    }
    
    // Remove element from DOM
    if (wikiSearch && wikiSearch.parentNode) {
      wikiSearch.parentNode.removeChild(wikiSearch);
    }
  });
  
  it('should create an instance', () => {
    expect(wikiSearch).toBeInstanceOf(WikiSearch);
  });
  
  it('should bind the keydown handler in constructor', () => {
    expect(wikiSearch._handleKeydown).toBeDefined();
    expect(typeof wikiSearch._handleKeydown).toBe('function');
  });
  
  it('should add event listener when connected', () => {
    // Mock shadowRoot for the component
    wikiSearch.shadowRoot.innerHTML = '<input type="search" />';
    
    // Append to DOM to trigger connectedCallback
    document.body.appendChild(wikiSearch);
    
    // Verify that addEventListener was called
    expect(window.addEventListener).toHaveBeenCalledWith('keydown', wikiSearch._handleKeydown);
  });
  
  it('should remove event listener when disconnected', () => {
    // Mock shadowRoot for the component
    wikiSearch.shadowRoot.innerHTML = '<input type="search" />';
    
    // Append to DOM to trigger connectedCallback
    document.body.appendChild(wikiSearch);
    
    // Remove from DOM to trigger disconnectedCallback
    document.body.removeChild(wikiSearch);
    
    // Verify that removeEventListener was called
    expect(window.removeEventListener).toHaveBeenCalledWith('keydown', wikiSearch._handleKeydown);
  });
  
  it('should handle keydown event correctly', () => {
    // Mock shadowRoot with search input
    const mockInput = document.createElement('input');
    mockInput.type = 'search';
    mockInput.focus = vi.fn();
    
    wikiSearch.shadowRoot.appendChild(mockInput);
    vi.spyOn(wikiSearch.shadowRoot, 'querySelector').mockReturnValue(mockInput);
    
    // Create mock event
    const mockEvent = {
      ctrlKey: true,
      key: 'k',
      preventDefault: vi.fn()
    };
    
    // Call the handler
    wikiSearch._handleKeydown(mockEvent);
    
    // Verify preventDefault was called and input was focused
    expect(mockEvent.preventDefault).toHaveBeenCalled();
    expect(mockInput.focus).toHaveBeenCalled();
  });
  
  it('should handle keydown event with metaKey (Mac)', () => {
    // Mock shadowRoot with search input
    const mockInput = document.createElement('input');
    mockInput.type = 'search';
    mockInput.focus = vi.fn();
    
    wikiSearch.shadowRoot.appendChild(mockInput);
    vi.spyOn(wikiSearch.shadowRoot, 'querySelector').mockReturnValue(mockInput);
    
    // Create mock event with metaKey (Mac)
    const mockEvent = {
      metaKey: true,
      key: 'k',
      preventDefault: vi.fn()
    };
    
    // Call the handler
    wikiSearch._handleKeydown(mockEvent);
    
    // Verify preventDefault was called and input was focused
    expect(mockEvent.preventDefault).toHaveBeenCalled();
    expect(mockInput.focus).toHaveBeenCalled();
  });
  
  it('should not handle keydown event for wrong key combination', () => {
    // Mock shadowRoot with search input
    const mockInput = document.createElement('input');
    mockInput.type = 'search';
    mockInput.focus = vi.fn();
    
    wikiSearch.shadowRoot.appendChild(mockInput);
    vi.spyOn(wikiSearch.shadowRoot, 'querySelector').mockReturnValue(mockInput);
    
    // Create mock event with wrong key
    const mockEvent = {
      ctrlKey: true,
      key: 'j',
      preventDefault: vi.fn()
    };
    
    // Call the handler
    wikiSearch._handleKeydown(mockEvent);
    
    // Verify preventDefault was NOT called and input was NOT focused
    expect(mockEvent.preventDefault).not.toHaveBeenCalled();
    expect(mockInput.focus).not.toHaveBeenCalled();
  });
  
  it('should prevent memory leak by properly managing event listeners', () => {
    // Mock shadowRoot for the component
    wikiSearch.shadowRoot.innerHTML = '<input type="search" />';
    
    // Test multiple connect/disconnect cycles
    for (let i = 0; i < 3; i++) {
      // Connect
      document.body.appendChild(wikiSearch);
      expect(window.addEventListener).toHaveBeenCalledWith('keydown', wikiSearch._handleKeydown);
      
      // Disconnect
      document.body.removeChild(wikiSearch);
      expect(window.removeEventListener).toHaveBeenCalledWith('keydown', wikiSearch._handleKeydown);
    }
    
    // Verify the same handler function is used (bound in constructor)
    const addCalls = window.addEventListener.mock.calls.filter(call => call[0] === 'keydown');
    const removeCalls = window.removeEventListener.mock.calls.filter(call => call[0] === 'keydown');
    
    // All calls should use the same bound function
    expect(addCalls.length).toBe(3);
    expect(removeCalls.length).toBe(3);
    
    // Verify it's the same function reference
    const firstHandler = addCalls[0][1];
    expect(addCalls.every(call => call[1] === firstHandler)).toBe(true);
    expect(removeCalls.every(call => call[1] === firstHandler)).toBe(true);
  });
});