import { html, fixture, expect } from '@open-wc/testing';
import sinon from 'sinon';
import { InventoryFindItemDialog } from './inventory-find-item-dialog.js';
import './inventory-find-item-dialog.js';

describe('InventoryFindItemDialog', () => {
  let el: InventoryFindItemDialog;

  function timeout(ms: number, message: string): Promise<never> {
    return new Promise((_, reject) =>
      setTimeout(() => reject(new Error(message)), ms)
    );
  }

  beforeEach(async () => {
    el = await Promise.race([
      fixture(html`<inventory-find-item-dialog></inventory-find-item-dialog>`),
      timeout(5000, 'Component fixture timed out'),
    ]);
    await el.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
    if (el) {
      el.remove();
    }
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  it('should be an instance of InventoryFindItemDialog', () => {
    expect(el).to.be.instanceOf(InventoryFindItemDialog);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('inventory-find-item-dialog');
  });

  describe('when component is initialized', () => {
    it('should not be open by default', () => {
      expect(el.open).to.be.false;
    });

    it('should have empty searchQuery by default', () => {
      expect(el.searchQuery).to.equal('');
    });

    it('should not be loading by default', () => {
      expect(el.loading).to.be.false;
    });

    it('should have no error by default', () => {
      expect(el.error).to.be.undefined;
    });

    it('should have no results by default', () => {
      expect(el.results).to.be.undefined;
    });
  });

  describe('openDialog', () => {
    describe('when called', () => {
      beforeEach(() => {
        el.openDialog();
      });

      it('should set open to true', () => {
        expect(el.open).to.be.true;
      });

      it('should clear searchQuery', () => {
        expect(el.searchQuery).to.equal('');
      });

      it('should clear results', () => {
        expect(el.results).to.be.undefined;
      });

      it('should clear error', () => {
        expect(el.error).to.be.undefined;
      });
    });

    describe('when called with pre-filled query', () => {
      beforeEach(() => {
        el.openDialog('screwdriver');
      });

      it('should set searchQuery', () => {
        expect(el.searchQuery).to.equal('screwdriver');
      });
    });
  });

  describe('close', () => {
    beforeEach(() => {
      el.openDialog();
      el.searchQuery = 'screwdriver';
      el.results = { found: true, locations: [] };
      el.close();
    });

    it('should set open to false', () => {
      expect(el.open).to.be.false;
    });

    it('should clear searchQuery', () => {
      expect(el.searchQuery).to.equal('');
    });

    it('should clear results', () => {
      expect(el.results).to.be.undefined;
    });

    it('should clear error', () => {
      expect(el.error).to.be.undefined;
    });
  });

  describe('keyboard handling', () => {
    describe('when escape key is pressed while open', () => {
      let closeSpy: sinon.SinonSpy;

      beforeEach(() => {
        closeSpy = sinon.spy(el, 'close');
        el.openDialog();
        el._handleKeydown(new KeyboardEvent('keydown', { key: 'Escape' }));
      });

      it('should close the dialog', () => {
        expect(closeSpy).to.have.been.calledOnce;
      });
    });

    describe('when escape key is pressed while closed', () => {
      let closeSpy: sinon.SinonSpy;

      beforeEach(() => {
        closeSpy = sinon.spy(el, 'close');
        el._handleKeydown(new KeyboardEvent('keydown', { key: 'Escape' }));
      });

      it('should not close the dialog', () => {
        expect(closeSpy).to.not.have.been.called;
      });
    });
  });

  describe('rendering', () => {
    describe('when dialog is open', () => {
      beforeEach(async () => {
        el.openDialog();
        await el.updateComplete;
      });

      it('should have open attribute', () => {
        expect(el.hasAttribute('open')).to.be.true;
      });

      it('should render dialog title', () => {
        const title = el.shadowRoot?.querySelector('.header h2');
        expect(title?.textContent).to.contain('Find Item');
      });

      it('should render search input field', () => {
        const searchInput = el.shadowRoot?.querySelector('input[name="searchQuery"]');
        expect(searchInput).to.exist;
      });

      it('should render search button', () => {
        const searchBtn = el.shadowRoot?.querySelector('button.primary');
        expect(searchBtn?.textContent).to.contain('Search');
      });

      it('should render close button', () => {
        const closeBtn = el.shadowRoot?.querySelector('button.secondary');
        expect(closeBtn?.textContent).to.contain('Close');
      });
    });

    describe('when dialog is closed', () => {
      it('should not have open attribute', () => {
        expect(el.hasAttribute('open')).to.be.false;
      });
    });

    describe('when loading', () => {
      beforeEach(async () => {
        el.openDialog();
        el.searchQuery = 'screwdriver';
        el.loading = true;
        await el.updateComplete;
      });

      it('should disable the search button', () => {
        const searchBtn = el.shadowRoot?.querySelector('button.primary') as HTMLButtonElement;
        expect(searchBtn?.disabled).to.be.true;
      });
    });

    describe('when error is present', () => {
      beforeEach(async () => {
        el.openDialog();
        el.error = 'Search failed';
        await el.updateComplete;
      });

      it('should display error message', () => {
        const errorDiv = el.shadowRoot?.querySelector('.error-message');
        expect(errorDiv?.textContent).to.contain('Search failed');
      });
    });

    describe('when results show item found', () => {
      beforeEach(async () => {
        el.openDialog();
        el.results = {
          found: true,
          locations: [
            { container: 'drawer_kitchen', path: ['house', 'kitchen', 'drawer_kitchen'] },
          ],
          summary: 'Found in drawer_kitchen',
        };
        await el.updateComplete;
      });

      it('should display results section', () => {
        const resultsSection = el.shadowRoot?.querySelector('.results');
        expect(resultsSection).to.exist;
      });

      it('should display location with link', () => {
        const locationLink = el.shadowRoot?.querySelector('.location-link');
        expect(locationLink).to.exist;
      });
    });

    describe('when results show item not found', () => {
      beforeEach(async () => {
        el.openDialog();
        el.results = {
          found: false,
          locations: [],
          summary: 'Item not found',
        };
        await el.updateComplete;
      });

      it('should display not found message', () => {
        const notFound = el.shadowRoot?.querySelector('.not-found');
        expect(notFound).to.exist;
      });
    });

    describe('when item found in multiple locations', () => {
      beforeEach(async () => {
        el.openDialog();
        el.results = {
          found: true,
          locations: [
            { container: 'drawer_kitchen', path: ['kitchen', 'drawer_kitchen'] },
            { container: 'toolbox_garage', path: ['garage', 'toolbox_garage'] },
          ],
          summary: 'Found in 2 locations',
        };
        await el.updateComplete;
      });

      it('should display anomaly warning', () => {
        const warning = el.shadowRoot?.querySelector('.anomaly-warning');
        expect(warning).to.exist;
      });

      it('should display all locations', () => {
        const locations = el.shadowRoot?.querySelectorAll('.location-item');
        expect(locations?.length).to.equal(2);
      });
    });
  });

  describe('form validation', () => {
    describe('when search query is empty', () => {
      beforeEach(async () => {
        el.openDialog();
        el.searchQuery = '';
        await el.updateComplete;
      });

      it('should disable the search button', () => {
        const searchBtn = el.shadowRoot?.querySelector('button.primary') as HTMLButtonElement;
        expect(searchBtn?.disabled).to.be.true;
      });
    });

    describe('when search query has value', () => {
      beforeEach(async () => {
        el.openDialog();
        el.searchQuery = 'screwdriver';
        await el.updateComplete;
      });

      it('should enable the search button', () => {
        const searchBtn = el.shadowRoot?.querySelector('button.primary') as HTMLButtonElement;
        expect(searchBtn?.disabled).to.be.false;
      });
    });
  });
});
