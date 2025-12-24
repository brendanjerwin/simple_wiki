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

  describe('when result has inventory context without title', () => {
    beforeEach(async () => {
      el.open = true;
      el.results = [{
        identifier: 'screwdriver',
        title: 'Screwdriver',
        fragment: 'A useful tool',
        highlights: [],
        inventoryContext: {
          isInventoryRelated: true,
          path: [{
            identifier: 'toolbox',
            title: ''
          }]
        }
      }] as unknown as WikiSearchResultsElement['results'];
      await el.updateComplete;
    });

    it('should render the Found In link', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn).to.exist;
    });

    it('should display "In:" text', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn?.textContent).to.contain('In:');
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

  describe('when result has inventory context with title', () => {
    beforeEach(async () => {
      el.open = true;
      el.results = [{
        identifier: 'screwdriver',
        title: 'Screwdriver',
        fragment: 'A useful tool',
        highlights: [],
        inventoryContext: {
          isInventoryRelated: true,
          path: [{
            identifier: 'toolbox',
            title: 'My Toolbox'
          }]
        }
      }] as unknown as WikiSearchResultsElement['results'];
      await el.updateComplete;
    });

    it('should render the Found In link', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn).to.exist;
    });

    it('should display "In:" text', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn?.textContent).to.contain('In:');
    });

    it('should link to the container', () => {
      const link = el.shadowRoot?.querySelector('.found-in a') as HTMLAnchorElement;
      expect(link?.getAttribute('href')).to.equal('/toolbox');
    });

    it('should display container title in link', () => {
      const link = el.shadowRoot?.querySelector('.found-in a');
      expect(link?.textContent).to.equal('My Toolbox');
    });
  });

  describe('when result has inventory context with nested path', () => {
    beforeEach(async () => {
      el.open = true;
      el.results = [{
        identifier: 'screwdriver',
        title: 'Screwdriver',
        fragment: 'A useful tool',
        highlights: [],
        inventoryContext: {
          isInventoryRelated: true,
          path: [
            { identifier: 'house', title: 'My House', depth: 0 },
            { identifier: 'garage', title: 'Main Garage', depth: 1 },
            { identifier: 'toolbox', title: 'My Toolbox', depth: 2 }
          ]
        }
      }] as unknown as WikiSearchResultsElement['results'];
      await el.updateComplete;
    });

    it('should render the Found In section', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn).to.exist;
    });

    it('should display all path elements', () => {
      const links = el.shadowRoot?.querySelectorAll('.found-in a');
      expect(links).to.have.lengthOf(3);
    });

    it('should display path elements in order', () => {
      const links = el.shadowRoot?.querySelectorAll('.found-in a');
      expect(links?.[0]?.textContent).to.equal('My House');
      expect(links?.[1]?.textContent).to.equal('Main Garage');
      expect(links?.[2]?.textContent).to.equal('My Toolbox');
    });

    it('should link each path element correctly', () => {
      const links = el.shadowRoot?.querySelectorAll('.found-in a') as NodeListOf<HTMLAnchorElement>;
      expect(links[0]?.getAttribute('href')).to.equal('/house');
      expect(links[1]?.getAttribute('href')).to.equal('/garage');
      expect(links[2]?.getAttribute('href')).to.equal('/toolbox');
    });

    it('should display separators between path elements', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn?.textContent).to.match(/My House\s*›\s*Main Garage\s*›\s*My Toolbox/);
    });
  });

  describe('when result has inventory context with path but no titles', () => {
    beforeEach(async () => {
      el.open = true;
      el.results = [{
        identifier: 'screwdriver',
        title: 'Screwdriver',
        fragment: 'A useful tool',
        highlights: [],
        inventoryContext: {
          isInventoryRelated: true,
          path: [
            { identifier: 'garage', title: '', depth: 0 },
            { identifier: 'toolbox', title: '', depth: 1 }
          ]
        }
      }] as unknown as WikiSearchResultsElement['results'];
      await el.updateComplete;
    });

    it('should fall back to identifiers when titles are empty', () => {
      const links = el.shadowRoot?.querySelectorAll('.found-in a');
      expect(links?.[0]?.textContent).to.equal('garage');
      expect(links?.[1]?.textContent).to.equal('toolbox');
    });
  });

  describe('when result has inventory context with mixed titles and identifiers', () => {
    beforeEach(async () => {
      el.open = true;
      el.results = [{
        identifier: 'power_drill',
        title: 'Cordless Power Drill',
        fragment: '18V cordless drill',
        highlights: [],
        inventoryContext: {
          isInventoryRelated: true,
          path: [
            { identifier: 'house', title: 'My House', depth: 0 },
            { identifier: 'workshop_shed', title: '', depth: 1 },
            { identifier: 'red_case', title: 'Red Tool Case', depth: 2 }
          ]
        }
      }] as unknown as WikiSearchResultsElement['results'];
      await el.updateComplete;
    });

    it('should display path with mixed titles and identifiers', () => {
      const links = el.shadowRoot?.querySelectorAll('.found-in a');
      expect(links).to.have.lengthOf(3);
      expect(links?.[0]?.textContent).to.equal('My House');
      expect(links?.[1]?.textContent).to.equal('workshop_shed');
      expect(links?.[2]?.textContent).to.equal('Red Tool Case');
    });

    it('should link all path elements correctly', () => {
      const links = el.shadowRoot?.querySelectorAll('.found-in a') as NodeListOf<HTMLAnchorElement>;
      expect(links[0]?.getAttribute('href')).to.equal('/house');
      expect(links[1]?.getAttribute('href')).to.equal('/workshop_shed');
      expect(links[2]?.getAttribute('href')).to.equal('/red_case');
    });

    it('should display separators between all elements', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn?.textContent).to.match(/My House\s*›\s*workshop_shed\s*›\s*Red Tool Case/);
    });
  });

  describe('when result has long container path (more than 4 levels)', () => {
    beforeEach(async () => {
      el.open = true;
      el.results = [{
        identifier: 'screwdriver',
        title: 'Screwdriver',
        fragment: 'A useful tool',
        highlights: [],
        inventoryContext: {
          isInventoryRelated: true,
          path: [
            { identifier: 'building', title: 'Main Building', depth: 0 },
            { identifier: 'floor2', title: 'Second Floor', depth: 1 },
            { identifier: 'storage_room', title: 'Storage Room', depth: 2 },
            { identifier: 'shelf_a', title: 'Shelf A', depth: 3 },
            { identifier: 'big_box', title: 'Big Box', depth: 4 },
            { identifier: 'small_box', title: 'Small Box', depth: 5 }
          ]
        }
      }] as unknown as WikiSearchResultsElement['results'];
      await el.updateComplete;
    });

    it('should truncate path and show ellipsis', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn).to.exist;
      
      // Should have ellipsis
      const ellipsis = foundIn?.querySelector('.path-ellipsis');
      expect(ellipsis).to.exist;
      expect(ellipsis?.textContent).to.equal('...');
    });

    it('should show last 3 items after ellipsis (total 4 visible)', () => {
      const links = el.shadowRoot?.querySelectorAll('.found-in a');
      expect(links).to.have.lengthOf(3); // Last 3 items
      
      // Should show the deepest (most useful) items
      expect(links?.[0]?.textContent).to.equal('Shelf A');
      expect(links?.[1]?.textContent).to.equal('Big Box');
      expect(links?.[2]?.textContent).to.equal('Small Box');
    });

    it('should not show the early items', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn?.textContent).to.not.contain('Main Building');
      expect(foundIn?.textContent).to.not.contain('Second Floor');
      expect(foundIn?.textContent).to.not.contain('Storage Room');
    });

    it('should maintain correct order with ellipsis first', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      const content = foundIn?.textContent || '';
      
      // Ellipsis should come before the visible items
      const ellipsisIndex = content.indexOf('...');
      const shelfIndex = content.indexOf('Shelf A');
      expect(ellipsisIndex).to.be.lessThan(shelfIndex);
    });
  });

  describe('when result has exactly 4 container levels', () => {
    beforeEach(async () => {
      el.open = true;
      el.results = [{
        identifier: 'item',
        title: 'Item',
        fragment: 'Some item',
        highlights: [],
        inventoryContext: {
          isInventoryRelated: true,
          path: [
            { identifier: 'level1', title: 'Level 1', depth: 0 },
            { identifier: 'level2', title: 'Level 2', depth: 1 },
            { identifier: 'level3', title: 'Level 3', depth: 2 },
            { identifier: 'level4', title: 'Level 4', depth: 3 }
          ]
        }
      }] as unknown as WikiSearchResultsElement['results'];
      await el.updateComplete;
    });

    it('should not show ellipsis when path is exactly 4 items', () => {
      const ellipsis = el.shadowRoot?.querySelector('.path-ellipsis');
      expect(ellipsis).to.not.exist;
    });

    it('should show all 4 items', () => {
      const links = el.shadowRoot?.querySelectorAll('.found-in a');
      expect(links).to.have.lengthOf(4);
    });
  });

  describe('when result has unsorted path', () => {
    beforeEach(async () => {
      el.open = true;
      el.results = [{
        identifier: 'item',
        title: 'Item',
        fragment: 'Some item',
        highlights: [],
        inventoryContext: {
          isInventoryRelated: true,
          path: [
            { identifier: 'b', title: 'B', depth: 1 },
            { identifier: 'c', title: 'C', depth: 2 },
            { identifier: 'a', title: 'A', depth: 0 }
          ]
        }
      }] as unknown as WikiSearchResultsElement['results'];
      await el.updateComplete;
    });

    it('should sort path by depth before rendering', () => {
      const links = el.shadowRoot?.querySelectorAll('.found-in a');
      expect(links?.[0]?.textContent).to.equal('A');
      expect(links?.[1]?.textContent).to.equal('B');
      expect(links?.[2]?.textContent).to.equal('C');
    });
  });

  describe('when result is not inventory related', () => {
    beforeEach(async () => {
      el.open = true;
      el.results = [{
        identifier: 'regular_page',
        title: 'Regular Page',
        fragment: 'Some content',
        highlights: [],
        inventoryContext: {
          isInventoryRelated: false,
          path: []
        }
      }] as unknown as WikiSearchResultsElement['results'];
      await el.updateComplete;
    });

    it('should not render the Found In section', () => {
      const foundIn = el.shadowRoot?.querySelector('.found-in');
      expect(foundIn).to.not.exist;
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