import { html, fixture, expect } from '@open-wc/testing';
import sinon from 'sinon';
import { InventoryMoveItemDialog } from './inventory-move-item-dialog.js';
import './inventory-move-item-dialog.js';

describe('InventoryMoveItemDialog', () => {
  let el: InventoryMoveItemDialog;

  function timeout(ms: number, message: string): Promise<never> {
    return new Promise((_, reject) =>
      setTimeout(() => reject(new Error(message)), ms)
    );
  }

  beforeEach(async () => {
    el = await Promise.race([
      fixture(html`<inventory-move-item-dialog></inventory-move-item-dialog>`),
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

  it('should be an instance of InventoryMoveItemDialog', () => {
    expect(el).to.be.instanceOf(InventoryMoveItemDialog);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('inventory-move-item-dialog');
  });

  describe('when component is initialized', () => {
    it('should not be open by default', () => {
      expect(el.open).to.be.false;
    });

    it('should have empty itemIdentifier by default', () => {
      expect(el.itemIdentifier).to.equal('');
    });

    it('should have empty currentContainer by default', () => {
      expect(el.currentContainer).to.equal('');
    });

    it('should have empty searchQuery by default', () => {
      expect(el.searchQuery).to.equal('');
    });

    it('should have empty searchResults by default', () => {
      expect(el.searchResults).to.deep.equal([]);
    });

    it('should have searchLoading false by default', () => {
      expect(el.searchLoading).to.be.false;
    });

    it('should have movingTo null by default', () => {
      expect(el.movingTo).to.be.null;
    });

    it('should have no error by default', () => {
      expect(el.error).to.be.undefined;
    });
  });

  describe('openDialog', () => {
    describe('when called with item and current container', () => {
      beforeEach(() => {
        el.openDialog('screwdriver', 'drawer_kitchen');
      });

      it('should set open to true', () => {
        expect(el.open).to.be.true;
      });

      it('should set itemIdentifier', () => {
        expect(el.itemIdentifier).to.equal('screwdriver');
      });

      it('should set currentContainer', () => {
        expect(el.currentContainer).to.equal('drawer_kitchen');
      });

      it('should clear searchQuery', () => {
        expect(el.searchQuery).to.equal('');
      });

      it('should clear searchResults', () => {
        expect(el.searchResults).to.deep.equal([]);
      });

      it('should set movingTo to null', () => {
        expect(el.movingTo).to.be.null;
      });

      it('should clear error', () => {
        expect(el.error).to.be.undefined;
      });
    });
  });

  describe('close', () => {
    beforeEach(() => {
      el.openDialog('screwdriver', 'drawer_kitchen');
      el.searchQuery = 'toolbox';
      el.close();
    });

    it('should set open to false', () => {
      expect(el.open).to.be.false;
    });

    it('should clear searchQuery', () => {
      expect(el.searchQuery).to.equal('');
    });

    it('should clear searchResults', () => {
      expect(el.searchResults).to.deep.equal([]);
    });

    it('should set movingTo to null', () => {
      expect(el.movingTo).to.be.null;
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
        el.openDialog('screwdriver', 'drawer_kitchen');
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
        el.openDialog('screwdriver', 'drawer_kitchen');
        await el.updateComplete;
      });

      it('should have open attribute', () => {
        expect(el.hasAttribute('open')).to.be.true;
      });

      it('should render dialog title', () => {
        const title = el.shadowRoot?.querySelector('.dialog-title');
        expect(title?.textContent).to.contain('Move Item');
      });

      it('should render item identifier as readonly', () => {
        const itemInput = el.shadowRoot?.querySelector('input[name="itemIdentifier"]') as HTMLInputElement;
        expect(itemInput?.readOnly).to.be.true;
        expect(itemInput?.value).to.equal('screwdriver');
      });

      it('should render current container as readonly', () => {
        const containerInput = el.shadowRoot?.querySelector('input[name="currentContainer"]') as HTMLInputElement;
        expect(containerInput?.readOnly).to.be.true;
        expect(containerInput?.value).to.equal('drawer_kitchen');
      });

      it('should render search query field', () => {
        const searchInput = el.shadowRoot?.querySelector('input[name="searchQuery"]');
        expect(searchInput).to.exist;
      });

      it('should render cancel button', () => {
        const cancelBtn = el.shadowRoot?.querySelector('.button-secondary');
        expect(cancelBtn?.textContent).to.contain('Cancel');
      });

      it('should render footer hint', () => {
        const footerHint = el.shadowRoot?.querySelector('.footer-hint');
        expect(footerHint?.textContent).to.contain('Select a destination');
      });
    });

    describe('when dialog is closed', () => {
      it('should not have open attribute', () => {
        expect(el.hasAttribute('open')).to.be.false;
      });
    });

    describe('when error is present', () => {
      beforeEach(async () => {
        el.openDialog('screwdriver', 'drawer_kitchen');
        el.error = 'Container not found';
        await el.updateComplete;
      });

      it('should display error message', () => {
        const errorDiv = el.shadowRoot?.querySelector('.error-message');
        expect(errorDiv?.textContent).to.contain('Container not found');
      });
    });

    describe('when searchLoading is true', () => {
      beforeEach(async () => {
        el.openDialog('screwdriver', 'drawer_kitchen');
        el.searchQuery = 'toolbox';
        el.searchLoading = true;
        await el.updateComplete;
      });

      it('should show loading message', () => {
        const resultsHeader = el.shadowRoot?.querySelector('.search-results-header');
        expect(resultsHeader?.textContent).to.contain('Searching');
      });
    });

    describe('when searchQuery has value but no results', () => {
      beforeEach(async () => {
        el.openDialog('screwdriver', 'drawer_kitchen');
        el.searchQuery = 'nonexistent';
        el.searchResults = [];
        el.searchLoading = false;
        await el.updateComplete;
      });

      it('should show no results message', () => {
        const noResults = el.shadowRoot?.querySelector('.no-results');
        expect(noResults?.textContent).to.contain('No containers found');
      });
    });

    describe('when search results exist', () => {
      beforeEach(async () => {
        el.openDialog('screwdriver', 'drawer_kitchen');
        el.searchQuery = 'toolbox';
        el.searchResults = [
          {
            identifier: 'toolbox_garage',
            title: 'Garage Toolbox',
            fragment: '',
            highlights: [],
            frontmatter: { 'inventory.container': 'garage' },
          } as unknown as import('../gen/api/v1/search_pb.js').SearchResult,
          {
            identifier: 'toolbox_shed',
            title: 'Shed Toolbox',
            fragment: '',
            highlights: [],
            frontmatter: {},
          } as unknown as import('../gen/api/v1/search_pb.js').SearchResult,
        ];
        await el.updateComplete;
      });

      it('should display search results header', () => {
        const resultsHeader = el.shadowRoot?.querySelector('.search-results-header');
        expect(resultsHeader?.textContent).to.contain('2 containers found');
      });

      it('should display result items', () => {
        const resultItems = el.shadowRoot?.querySelectorAll('.search-result-item');
        expect(resultItems?.length).to.equal(2);
      });

      it('should display result title', () => {
        const resultTitle = el.shadowRoot?.querySelector('.result-title');
        expect(resultTitle?.textContent).to.equal('Garage Toolbox');
      });

      it('should display result container if present', () => {
        const resultContainer = el.shadowRoot?.querySelector('.result-container');
        expect(resultContainer?.textContent).to.contain('garage');
      });

      it('should render Move To buttons', () => {
        const moveButtons = el.shadowRoot?.querySelectorAll('.move-to-button');
        expect(moveButtons?.length).to.equal(2);
        expect(moveButtons?.[0]?.textContent).to.contain('Move To');
      });
    });

    describe('when movingTo is set', () => {
      beforeEach(async () => {
        el.openDialog('screwdriver', 'drawer_kitchen');
        el.searchQuery = 'toolbox';
        el.searchResults = [
          {
            identifier: 'toolbox_garage',
            title: 'Garage Toolbox',
            fragment: '',
            highlights: [],
            frontmatter: {},
          } as unknown as import('../gen/api/v1/search_pb.js').SearchResult,
        ];
        el.movingTo = 'toolbox_garage';
        await el.updateComplete;
      });

      it('should show Moving... on active button', () => {
        const moveButton = el.shadowRoot?.querySelector('.move-to-button');
        expect(moveButton?.textContent).to.contain('Moving...');
      });

      it('should disable all Move To buttons', () => {
        const moveButton = el.shadowRoot?.querySelector('.move-to-button') as HTMLButtonElement;
        expect(moveButton?.disabled).to.be.true;
      });

      it('should disable search input', () => {
        const searchInput = el.shadowRoot?.querySelector('input[name="searchQuery"]') as HTMLInputElement;
        expect(searchInput?.disabled).to.be.true;
      });

      it('should disable cancel button', () => {
        const cancelBtn = el.shadowRoot?.querySelector('.button-secondary') as HTMLButtonElement;
        expect(cancelBtn?.disabled).to.be.true;
      });
    });
  });
});
