import { html, fixture, expect } from '@open-wc/testing';
import sinon from 'sinon';
import './wiki-search-results.js';

interface WikiSearchResultsElement extends HTMLElement {
  results: Array<{
    Identifier: string;
    Title: string;
    FragmentHTML?: string;
  }>;
  open: boolean;
  inventoryOnly: boolean;
  _handleClickOutside: (event: Event) => void;
  close: () => void;
  updateComplete: Promise<boolean>;
  shadowRoot: ShadowRoot;
}

describe('WikiSearchResults', () => {
  let el: WikiSearchResultsElement;

  beforeEach(async () => {
    el = await fixture(html`<wiki-search-results></wiki-search-results>`) as WikiSearchResultsElement;
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
      expect(el.inventoryOnly).to.equal(false);
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
      el = await fixture(html`<wiki-search-results></wiki-search-results>`) as WikiSearchResultsElement;
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
      el = await fixture(html`<wiki-search-results></wiki-search-results>`) as WikiSearchResultsElement;
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
    let mockEvent: Event & { composedPath(): EventTarget[] };
    
    beforeEach(async () => {
      el.open = true;
      el.results = [{ Identifier: 'test', Title: 'Test', FragmentHTML: 'Test content' }];
      await el.updateComplete;
      closeSpy = sinon.spy(el, 'close');
      
      // Simulate clicking outside (event won't include the popover)
      mockEvent = {
        ...new Event('click'),
        composedPath: () => []
      } as Event & { composedPath(): EventTarget[] };
      el._handleClickOutside(mockEvent);
    });

    it('should close the popover', () => {
      expect(closeSpy).to.have.been.calledOnce;
    });
  });

  describe('when clicking inside the popover', () => {
    let closeSpy: sinon.SinonSpy;
    let mockEvent: Event & { composedPath(): EventTarget[] };

    beforeEach(async () => {
      el.open = true;
      el.results = [{ Identifier: 'test', Title: 'Test', FragmentHTML: 'Test content' }];
      await el.updateComplete;
      closeSpy = sinon.spy(el, 'close');

      // Simulate clicking inside (event includes the popover)
      const mockPopover = el.shadowRoot.querySelector('.popover');
      mockEvent = {
        ...new Event('click'),
        composedPath: () => [mockPopover]
      } as Event & { composedPath(): EventTarget[] };
      el._handleClickOutside(mockEvent);
    });

    it('should not close the popover', () => {
      expect(closeSpy).to.not.have.been.called;
    });
  });

  describe('when dialog is open', () => {
    beforeEach(async () => {
      el.open = true;
      el.results = [{ Identifier: 'test', Title: 'Test', FragmentHTML: 'Test content' }];
      await el.updateComplete;
    });

    it('should render the inventory checkbox', () => {
      const checkbox = el.shadowRoot?.querySelector('.inventory-filter input[type="checkbox"]');
      expect(checkbox).to.exist;
    });

    it('should render the filter divider', () => {
      const divider = el.shadowRoot?.querySelector('.filter-divider');
      expect(divider).to.exist;
    });
  });

  describe('when inventory checkbox is checked', () => {
    let checkbox: HTMLInputElement;
    let eventSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el.open = true;
      el.results = [{ Identifier: 'test', Title: 'Test', FragmentHTML: 'Test content' }];
      await el.updateComplete;

      eventSpy = sinon.spy();
      el.addEventListener('inventory-filter-changed', eventSpy);

      checkbox = el.shadowRoot?.querySelector('.inventory-filter input[type="checkbox"]') as HTMLInputElement;
      checkbox.checked = true;
      checkbox.dispatchEvent(new Event('change', { bubbles: true }));
      await el.updateComplete;
    });

    it('should set inventoryOnly to true', () => {
      expect(el.inventoryOnly).to.equal(true);
    });

    it('should emit inventory-filter-changed event', () => {
      expect(eventSpy).to.have.been.calledOnce;
    });

    it('should include inventoryOnly in event detail', () => {
      const event = eventSpy.getCall(0).args[0] as CustomEvent;
      expect(event.detail.inventoryOnly).to.equal(true);
    });
  });

  describe('when inventory checkbox is unchecked', () => {
    let checkbox: HTMLInputElement;
    let eventSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el.open = true;
      el.inventoryOnly = true;
      el.results = [{ Identifier: 'test', Title: 'Test', FragmentHTML: 'Test content' }];
      await el.updateComplete;

      eventSpy = sinon.spy();
      el.addEventListener('inventory-filter-changed', eventSpy);

      checkbox = el.shadowRoot?.querySelector('.inventory-filter input[type="checkbox"]') as HTMLInputElement;
      checkbox.checked = false;
      checkbox.dispatchEvent(new Event('change', { bubbles: true }));
      await el.updateComplete;
    });

    it('should set inventoryOnly to false', () => {
      expect(el.inventoryOnly).to.equal(false);
    });

    it('should emit inventory-filter-changed event', () => {
      expect(eventSpy).to.have.been.calledOnce;
    });

    it('should include inventoryOnly false in event detail', () => {
      const event = eventSpy.getCall(0).args[0] as CustomEvent;
      expect(event.detail.inventoryOnly).to.equal(false);
    });
  });

  describe('when result has inventory.container in frontmatter', () => {
    beforeEach(async () => {
      el.open = true;
      el.results = [{
        identifier: 'screwdriver',
        title: 'Screwdriver',
        fragment: 'A useful tool',
        highlights: [],
        frontmatter: { 'inventory.container': 'toolbox' }
      }] as unknown as WikiSearchResultsElement['results'];
      await el.updateComplete;
    });

    it('should render the Found In link', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn).to.exist;
    });

    it('should display "Found In:" text', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn?.textContent).to.contain('Found In:');
    });

    it('should link to the container', () => {
      const link = el.shadowRoot?.querySelector('.found-in a') as HTMLAnchorElement;
      expect(link?.getAttribute('href')).to.equal('/toolbox');
    });

    it('should display container identifier in link', () => {
      const link = el.shadowRoot?.querySelector('.found-in a');
      expect(link?.textContent).to.equal('toolbox');
    });
  });

  describe('when result has no frontmatter', () => {
    beforeEach(async () => {
      el.open = true;
      el.results = [{
        identifier: 'test_page',
        title: 'Test Page',
        fragment: 'Some content',
        highlights: []
      }] as unknown as WikiSearchResultsElement['results'];
      await el.updateComplete;
    });

    it('should not render the Found In section', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn).to.not.exist;
    });
  });

  describe('when result has frontmatter but no container', () => {
    beforeEach(async () => {
      el.open = true;
      el.results = [{
        identifier: 'test_page',
        title: 'Test Page',
        fragment: 'Some content',
        highlights: [],
        frontmatter: { 'some.other.key': 'value' }
      }] as unknown as WikiSearchResultsElement['results'];
      await el.updateComplete;
    });

    it('should not render the Found In section', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn).to.not.exist;
    });
  });
});