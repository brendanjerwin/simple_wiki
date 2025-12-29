import { html, fixture, expect } from '@open-wc/testing';
import sinon from 'sinon';
import { InventoryAddItemDialog } from './inventory-add-item-dialog.js';
import './inventory-add-item-dialog.js';

describe('InventoryAddItemDialog', () => {
  let el: InventoryAddItemDialog;

  function timeout(ms: number, message: string): Promise<never> {
    return new Promise((_, reject) =>
      setTimeout(() => reject(new Error(message)), ms)
    );
  }

  beforeEach(async () => {
    el = await Promise.race([
      fixture<InventoryAddItemDialog>(html`<inventory-add-item-dialog></inventory-add-item-dialog>`),
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

  it('should be an instance of InventoryAddItemDialog', () => {
    expect(el).to.be.instanceOf(InventoryAddItemDialog);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('inventory-add-item-dialog');
  });

  describe('when component is initialized', () => {
    it('should not be open by default', () => {
      expect(el.open).to.be.false;
    });

    it('should have empty container by default', () => {
      expect(el.container).to.equal('');
    });

    it('should have empty itemIdentifier by default', () => {
      expect(el.itemIdentifier).to.equal('');
    });

    it('should have empty title by default', () => {
      expect(el.itemTitle).to.equal('');
    });

    it('should have empty description by default', () => {
      expect(el.description).to.equal('');
    });

    it('should have automagicMode enabled by default', () => {
      expect(el.automagicMode).to.be.true;
    });

    it('should not be loading by default', () => {
      expect(el.loading).to.be.false;
    });

    it('should have no error by default', () => {
      expect(el.error).to.be.null;
    });

    it('should have isUnique true by default', () => {
      expect(el.isUnique).to.be.true;
    });

    it('should have no existingPage by default', () => {
      expect(el.existingPage).to.be.undefined;
    });

    it('should have empty searchResults by default', () => {
      expect(el.searchResults).to.deep.equal([]);
    });
  });

  describe('openDialog', () => {
    describe('when called with a container', () => {
      beforeEach(() => {
        el.openDialog('drawer_kitchen');
      });

      it('should set open to true', () => {
        expect(el.open).to.be.true;
      });

      it('should set container', () => {
        expect(el.container).to.equal('drawer_kitchen');
      });

      it('should clear itemIdentifier', () => {
        expect(el.itemIdentifier).to.equal('');
      });

      it('should clear title', () => {
        expect(el.itemTitle).to.equal('');
      });

      it('should clear description', () => {
        expect(el.description).to.equal('');
      });

      it('should set automagicMode to true', () => {
        expect(el.automagicMode).to.be.true;
      });

      it('should clear error', () => {
        expect(el.error).to.be.null;
      });

      it('should reset isUnique to true', () => {
        expect(el.isUnique).to.be.true;
      });

      it('should clear existingPage', () => {
        expect(el.existingPage).to.be.undefined;
      });

      it('should clear searchResults', () => {
        expect(el.searchResults).to.deep.equal([]);
      });
    });
  });

  describe('close', () => {
    beforeEach(() => {
      el.openDialog('drawer_kitchen');
      el.itemIdentifier = 'screwdriver';
      el.itemTitle = 'Phillips Screwdriver';
      el.description = 'A yellow-handled screwdriver';
      el.automagicMode = false;
      el.close();
    });

    it('should set open to false', () => {
      expect(el.open).to.be.false;
    });

    it('should clear itemIdentifier', () => {
      expect(el.itemIdentifier).to.equal('');
    });

    it('should clear title', () => {
      expect(el.itemTitle).to.equal('');
    });

    it('should clear description', () => {
      expect(el.description).to.equal('');
    });

    it('should clear error', () => {
      expect(el.error).to.be.null;
    });

    it('should reset isUnique to true', () => {
      expect(el.isUnique).to.be.true;
    });

    it('should clear existingPage', () => {
      expect(el.existingPage).to.be.undefined;
    });

    it('should clear searchResults', () => {
      expect(el.searchResults).to.deep.equal([]);
    });
  });

  describe('keyboard handling', () => {
    describe('when escape key is pressed while open', () => {
      let closeSpy: sinon.SinonSpy;

      beforeEach(() => {
        closeSpy = sinon.spy(el, 'close');
        el.openDialog('drawer_kitchen');
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
        el.openDialog('drawer_kitchen');
        await el.updateComplete;
      });

      it('should have open attribute', () => {
        expect(el.hasAttribute('open')).to.be.true;
      });

      it('should render dialog title with container', () => {
        const title = el.shadowRoot?.querySelector('.dialog-title');
        expect(title?.textContent).to.contain('Add Item to: drawer_kitchen');
      });

      it('should render title field', () => {
        const titleInput = el.shadowRoot?.querySelector('input[name="title"]');
        expect(titleInput).to.exist;
      });

      it('should render item identifier field', () => {
        const itemInput = el.shadowRoot?.querySelector('input[name="itemIdentifier"]');
        expect(itemInput).to.exist;
      });

      it('should render description field', () => {
        const descriptionInput = el.shadowRoot?.querySelector('textarea[name="description"]');
        expect(descriptionInput).to.exist;
      });

      it('should render automagic button', () => {
        const automagicBtn = el.shadowRoot?.querySelector('.automagic-button');
        expect(automagicBtn).to.exist;
      });

      it('should render cancel button', () => {
        const cancelBtn = el.shadowRoot?.querySelector('.button-secondary');
        expect(cancelBtn?.textContent).to.contain('Cancel');
      });

      it('should render add button', () => {
        const addBtn = el.shadowRoot?.querySelector('.button-primary');
        expect(addBtn?.textContent).to.contain('Add');
      });
    });

    describe('when dialog is closed', () => {
      it('should not have open attribute', () => {
        expect(el.hasAttribute('open')).to.be.false;
      });
    });

    describe('when loading', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.loading = true;
        await el.updateComplete;
      });

      it('should disable the add button', () => {
        const addBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-primary');
        expect(addBtn?.disabled).to.be.true;
      });
    });

    describe('when error is present', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.error = new Error('Item already exists');
        await el.updateComplete;
      });

      it('should store error as Error object', () => {
        expect(el.error).to.be.instanceOf(Error);
      });

      it('should preserve error message', () => {
        expect(el.error?.message).to.equal('Item already exists');
      });

      it('should display error message in UI', () => {
        const errorDiv = el.shadowRoot?.querySelector('.error-message');
        expect(errorDiv?.textContent).to.contain('Item already exists');
      });
    });

    describe('when identifier conflict exists', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.itemTitle = 'Screwdriver';
        el.itemIdentifier = 'screwdriver';
        el.isUnique = false;
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock test data
        el.existingPage = {
          identifier: 'screwdriver',
          title: 'Phillips Screwdriver',
          container: 'toolbox_garage',
        } as import('../gen/api/v1/page_management_pb.js').ExistingPageInfo;
        await el.updateComplete;
      });

      it('should display conflict warning', () => {
        const warningDiv = el.shadowRoot?.querySelector('.conflict-warning');
        expect(warningDiv).to.exist;
        expect(warningDiv?.textContent).to.contain('Identifier already exists');
      });

      it('should include link to existing page', () => {
        const warningDiv = el.shadowRoot?.querySelector('.conflict-warning');
        const link = warningDiv?.querySelector('a');
        expect(link?.getAttribute('href')).to.equal('/screwdriver');
      });

      it('should disable add button', () => {
        const addBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-primary');
        expect(addBtn?.disabled).to.be.true;
      });
    });

    describe('when search results exist', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.searchResults = [
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock test data
          {
            identifier: 'hammer',
            title: 'Claw Hammer',
            fragment: 'A useful tool',
            highlights: [],
            frontmatter: { 'inventory.container': 'toolbox_garage' },
          } as unknown as import('../gen/api/v1/search_pb.js').SearchResult,
        ];
        await el.updateComplete;
      });

      it('should display search results section', () => {
        const resultsDiv = el.shadowRoot?.querySelector('.search-results');
        expect(resultsDiv).to.exist;
      });

      it('should display search result item', () => {
        const resultItem = el.shadowRoot?.querySelector('.search-result-item');
        expect(resultItem).to.exist;
      });

      it('should display result title', () => {
        const titleDiv = el.shadowRoot?.querySelector('.search-result-title');
        expect(titleDiv?.textContent).to.equal('Claw Hammer');
      });

      it('should display result container', () => {
        const containerDiv = el.shadowRoot?.querySelector('.search-result-container');
        expect(containerDiv?.textContent).to.contain('toolbox_garage');
      });
    });

    describe('when in automagic mode', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.automagicMode = true;
        await el.updateComplete;
      });

      it('should show sparkle icon', () => {
        const icon = el.shadowRoot?.querySelector('.automagic-button i');
        expect(icon?.classList.contains('fa-wand-magic-sparkles')).to.be.true;
      });

      it('should have automagic class on button', () => {
        const btn = el.shadowRoot?.querySelector('.automagic-button');
        expect(btn?.classList.contains('automagic')).to.be.true;
      });

      it('should make identifier field readonly', () => {
        const input = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="itemIdentifier"]');
        expect(input?.readOnly).to.be.true;
      });

      it('should set identifier tabindex to -1', () => {
        const input = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="itemIdentifier"]');
        expect(input?.tabIndex).to.equal(-1);
      });
    });

    describe('when in manual mode', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.automagicMode = false;
        await el.updateComplete;
      });

      it('should show pen icon', () => {
        const icon = el.shadowRoot?.querySelector('.automagic-button i');
        expect(icon?.classList.contains('fa-pen')).to.be.true;
      });

      it('should have manual class on button', () => {
        const btn = el.shadowRoot?.querySelector('.automagic-button');
        expect(btn?.classList.contains('manual')).to.be.true;
      });

      it('should not make identifier field readonly', () => {
        const input = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="itemIdentifier"]');
        expect(input?.readOnly).to.be.false;
      });

      it('should set identifier tabindex to 0', () => {
        const input = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="itemIdentifier"]');
        expect(input?.tabIndex).to.equal(0);
      });
    });
  });

  describe('backdrop click handling', () => {
    describe('when backdrop is clicked', () => {
      let closeSpy: sinon.SinonSpy;

      beforeEach(async () => {
        closeSpy = sinon.spy(el, 'close');
        el.openDialog('drawer_kitchen');
        await el.updateComplete;
        const backdrop = el.shadowRoot?.querySelector<HTMLElement>('.backdrop');
        backdrop?.click();
      });

      it('should close the dialog', () => {
        expect(closeSpy).to.have.been.calledOnce;
      });
    });
  });

  describe('dialog click handling', () => {
    describe('when dialog content is clicked', () => {
      let closeSpy: sinon.SinonSpy;

      beforeEach(async () => {
        closeSpy = sinon.spy(el, 'close');
        el.openDialog('drawer_kitchen');
        await el.updateComplete;
        const dialog = el.shadowRoot?.querySelector<HTMLElement>('.dialog');
        dialog?.click();
      });

      it('should not close the dialog', () => {
        expect(closeSpy).to.not.have.been.called;
      });
    });
  });

  describe('cancel button', () => {
    describe('when cancel button is clicked', () => {
      let closeSpy: sinon.SinonSpy;

      beforeEach(async () => {
        closeSpy = sinon.spy(el, 'close');
        el.openDialog('drawer_kitchen');
        await el.updateComplete;
        const cancelBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-secondary');
        cancelBtn?.click();
      });

      it('should close the dialog', () => {
        expect(closeSpy).to.have.been.calledOnce;
      });
    });
  });

  describe('title input handling', () => {
    describe('when title is typed', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        await el.updateComplete;
        const titleInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
        if (titleInput) {
          titleInput.value = 'Test Item';
          titleInput.dispatchEvent(new Event('input'));
        }
      });

      it('should update itemTitle property', () => {
        expect(el.itemTitle).to.equal('Test Item');
      });
    });

    describe('when input event target is not an HTMLInputElement', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.itemTitle = 'original';
        await el.updateComplete;
        // Create an event with a non-input target
        const event = new Event('input');
        Object.defineProperty(event, 'target', { value: document.createElement('div') });
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        (el as unknown as { _handleTitleInput: (e: Event) => void })._handleTitleInput(event);
      });

      it('should not update itemTitle property', () => {
        expect(el.itemTitle).to.equal('original');
      });
    });
  });

  describe('description input handling', () => {
    describe('when description is typed', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        await el.updateComplete;
        const descInput = el.shadowRoot?.querySelector<HTMLTextAreaElement>('textarea[name="description"]');
        if (descInput) {
          descInput.value = 'Test description';
          descInput.dispatchEvent(new Event('input'));
        }
      });

      it('should update description property', () => {
        expect(el.description).to.equal('Test description');
      });
    });

    describe('when input event target is not an HTMLTextAreaElement', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.description = 'original';
        await el.updateComplete;
        // Create an event with a non-textarea target
        const event = new Event('input');
        Object.defineProperty(event, 'target', { value: document.createElement('div') });
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        (el as unknown as { _handleDescriptionInput: (e: Event) => void })._handleDescriptionInput(event);
      });

      it('should not update description property', () => {
        expect(el.description).to.equal('original');
      });
    });
  });

  describe('identifier input handling', () => {
    describe('when in manual mode and identifier is typed', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.automagicMode = false;
        await el.updateComplete;
        const identifierInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="itemIdentifier"]');
        if (identifierInput) {
          identifierInput.value = 'custom_identifier';
          identifierInput.dispatchEvent(new Event('input'));
        }
      });

      it('should update itemIdentifier property', () => {
        expect(el.itemIdentifier).to.equal('custom_identifier');
      });
    });

    describe('when in automagic mode and identifier input is triggered', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.automagicMode = true;
        el.itemIdentifier = 'original_value';
        await el.updateComplete;
        const identifierInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="itemIdentifier"]');
        if (identifierInput) {
          identifierInput.value = 'attempted_change';
          identifierInput.dispatchEvent(new Event('input'));
        }
      });

      it('should not update itemIdentifier property', () => {
        expect(el.itemIdentifier).to.equal('original_value');
      });
    });

    describe('when input event target is not an HTMLInputElement', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.automagicMode = false;
        el.itemIdentifier = 'original';
        await el.updateComplete;
        // Create an event with a non-input target
        const event = new Event('input');
        Object.defineProperty(event, 'target', { value: document.createElement('div') });
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        (el as unknown as { _handleIdentifierInput: (e: Event) => void })._handleIdentifierInput(event);
      });

      it('should not update itemIdentifier property', () => {
        expect(el.itemIdentifier).to.equal('original');
      });
    });
  });

  describe('automagic toggle', () => {
    describe('when toggle is clicked to switch to manual mode', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.automagicMode = true;
        await el.updateComplete;
        const toggleBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.automagic-button');
        toggleBtn?.click();
      });

      it('should switch to manual mode', () => {
        expect(el.automagicMode).to.be.false;
      });
    });

    describe('when toggle is clicked to switch back to automagic mode', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.automagicMode = false;
        await el.updateComplete;
        const toggleBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.automagic-button');
        toggleBtn?.click();
      });

      it('should switch to automagic mode', () => {
        expect(el.automagicMode).to.be.true;
      });
    });
  });

  describe('event listener lifecycle', () => {
    describe('when component is connected', () => {
      let addEventListenerSpy: sinon.SinonSpy;

      beforeEach(async () => {
        addEventListenerSpy = sinon.spy(document, 'addEventListener');
        el = await fixture(html`<inventory-add-item-dialog></inventory-add-item-dialog>`);
        await el.updateComplete;
      });

      it('should add keydown event listener', () => {
        expect(addEventListenerSpy).to.have.been.calledWith('keydown', el._handleKeydown);
      });
    });

    describe('when component is disconnected', () => {
      let removeEventListenerSpy: sinon.SinonSpy;

      beforeEach(async () => {
        removeEventListenerSpy = sinon.spy(document, 'removeEventListener');
        el = await fixture(html`<inventory-add-item-dialog></inventory-add-item-dialog>`);
        await el.updateComplete;
        el.remove();
      });

      it('should remove keydown event listener', () => {
        expect(removeEventListenerSpy).to.have.been.calledWith('keydown', el._handleKeydown);
      });
    });
  });

  describe('search results rendering edge cases', () => {
    describe('when search is loading', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.searchLoading = true;
        el.searchResults = [];
        await el.updateComplete;
      });

      it('should display loading text', () => {
        const header = el.shadowRoot?.querySelector('.search-results-header');
        expect(header?.textContent).to.contain('Searching...');
      });
    });

    describe('when exactly one result is found', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.searchResults = [
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock test data
          {
            identifier: 'item1',
            title: 'Item One',
            fragment: '',
            highlights: [],
            frontmatter: {},
          } as unknown as import('../gen/api/v1/search_pb.js').SearchResult,
        ];
        el.searchLoading = false;
        await el.updateComplete;
      });

      it('should display singular text', () => {
        const header = el.shadowRoot?.querySelector('.search-results-header');
        expect(header?.textContent).to.contain('1 similar item found');
      });
    });

    describe('when search result has no container', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.searchResults = [
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock test data
          {
            identifier: 'item1',
            title: 'Item One',
            fragment: '',
            highlights: [],
            frontmatter: {},
          } as unknown as import('../gen/api/v1/search_pb.js').SearchResult,
        ];
        await el.updateComplete;
      });

      it('should not display container info', () => {
        const containerDiv = el.shadowRoot?.querySelector('.search-result-container');
        expect(containerDiv).to.not.exist;
      });
    });

    describe('when search result uses identifier as title', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.searchResults = [
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock test data
          {
            identifier: 'item_identifier',
            title: '',
            fragment: '',
            highlights: [],
            frontmatter: {},
          } as unknown as import('../gen/api/v1/search_pb.js').SearchResult,
        ];
        await el.updateComplete;
      });

      it('should display identifier as title', () => {
        const titleDiv = el.shadowRoot?.querySelector('.search-result-title');
        expect(titleDiv?.textContent).to.equal('item_identifier');
      });
    });
  });

  describe('conflict warning edge cases', () => {
    describe('when existingPage has no title', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.isUnique = false;
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock test data
        el.existingPage = {
          identifier: 'test_item',
          title: '',
          container: '',
        } as import('../gen/api/v1/page_management_pb.js').ExistingPageInfo;
        await el.updateComplete;
      });

      it('should display identifier as link text', () => {
        const warningDiv = el.shadowRoot?.querySelector('.conflict-warning');
        const link = warningDiv?.querySelector('a');
        expect(link?.textContent).to.equal('test_item');
      });
    });

    describe('when existingPage has no container', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.isUnique = false;
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock test data
        el.existingPage = {
          identifier: 'test_item',
          title: 'Test Item',
          container: '',
        } as import('../gen/api/v1/page_management_pb.js').ExistingPageInfo;
        await el.updateComplete;
      });

      it('should not display container info in warning', () => {
        const warningDiv = el.shadowRoot?.querySelector('.conflict-warning');
        expect(warningDiv?.textContent).to.not.contain('Found In');
      });
    });
  });

  describe('form validation', () => {
    describe('when title is empty', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.itemTitle = '';
        el.itemIdentifier = 'screwdriver';
        await el.updateComplete;
      });

      it('should disable the add button', () => {
        const addBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-primary');
        expect(addBtn?.disabled).to.be.true;
      });
    });

    describe('when identifier is empty', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.itemTitle = 'Screwdriver';
        el.itemIdentifier = '';
        await el.updateComplete;
      });

      it('should disable the add button', () => {
        const addBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-primary');
        expect(addBtn?.disabled).to.be.true;
      });
    });

    describe('when identifier is not unique', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.itemTitle = 'Screwdriver';
        el.itemIdentifier = 'screwdriver';
        el.isUnique = false;
        await el.updateComplete;
      });

      it('should disable the add button', () => {
        const addBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-primary');
        expect(addBtn?.disabled).to.be.true;
      });
    });

    describe('when all fields are valid', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.itemTitle = 'Phillips Screwdriver';
        el.itemIdentifier = 'phillips_screwdriver';
        el.isUnique = true;
        await el.updateComplete;
      });

      it('should enable the add button', () => {
        const addBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-primary');
        expect(addBtn?.disabled).to.be.false;
      });
    });
  });
});
