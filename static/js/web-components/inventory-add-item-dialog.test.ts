import { html, fixture, expect } from '@open-wc/testing';
import sinon, { type SinonStub } from 'sinon';
import { InventoryAddItemDialog } from './inventory-add-item-dialog.js';
import type { InventoryItemCreatorMover } from './inventory-item-creator-mover.js';
import type { ExistingPageInfo } from '../gen/api/v1/page_management_pb.js';
import { AugmentedError, ErrorKind } from './augment-error-service.js';
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

  describe('async title input handling with debounce', () => {
    let clock: sinon.SinonFakeTimers;
    let generateIdentifierStub: SinonStub;
    let searchContentStub: SinonStub;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private property for testing
    let inventoryItemCreatorMover: any;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private property for testing
    let searchClient: any;

    beforeEach(async () => {
      clock = sinon.useFakeTimers();
      el = await fixture<InventoryAddItemDialog>(html`<inventory-add-item-dialog></inventory-add-item-dialog>`);
      await el.updateComplete;

      // Access private properties for stubbing
      // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
      inventoryItemCreatorMover = (el as any).inventoryItemCreatorMover as InventoryItemCreatorMover;
      // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
      searchClient = (el as any).searchClient;

      generateIdentifierStub = sinon.stub(inventoryItemCreatorMover, 'generateIdentifier');
      searchContentStub = sinon.stub(searchClient, 'searchContent');
    });

    afterEach(() => {
      clock.restore();
      sinon.restore();
    });

    describe('when title is cleared', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.itemTitle = 'Some Title';
        el.itemIdentifier = 'some_title';
        el.searchResults = [
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock test data
          { identifier: 'item1', title: 'Item', fragment: '', highlights: [], frontmatter: {} } as unknown as import('../gen/api/v1/search_pb.js').SearchResult,
        ];
        await el.updateComplete;

        // Trigger title input with empty value
        const titleInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
        if (titleInput) {
          titleInput.value = '';
          titleInput.dispatchEvent(new Event('input'));
        }

        // Advance past debounce
        await clock.tickAsync(350);
        await el.updateComplete;
      });

      it('should clear itemIdentifier', () => {
        expect(el.itemIdentifier).to.equal('');
      });

      it('should set isUnique to true', () => {
        expect(el.isUnique).to.be.true;
      });

      it('should clear existingPage', () => {
        expect(el.existingPage).to.be.undefined;
      });

      it('should clear searchResults', () => {
        expect(el.searchResults).to.deep.equal([]);
      });
    });

    describe('when title is entered in automagic mode and generateIdentifier succeeds', () => {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock test data
      const mockExistingPage = {
        identifier: 'test_item',
        title: 'Existing Test Item',
        container: 'drawer_existing',
      } as ExistingPageInfo;

      beforeEach(async () => {
        generateIdentifierStub.resolves({
          identifier: 'test_item',
          isUnique: false,
          existingPage: mockExistingPage,
        });
        searchContentStub.resolves({ results: [] });

        el.openDialog('drawer_kitchen');
        el.automagicMode = true;
        await el.updateComplete;

        // Trigger title input
        const titleInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
        if (titleInput) {
          titleInput.value = 'Test Item';
          titleInput.dispatchEvent(new Event('input'));
        }

        // Advance past debounce
        await clock.tickAsync(350);
        await el.updateComplete;
      });

      it('should call generateIdentifier with the title', () => {
        expect(generateIdentifierStub).to.have.been.calledWith('Test Item');
      });

      it('should set itemIdentifier from response', () => {
        expect(el.itemIdentifier).to.equal('test_item');
      });

      it('should set isUnique from response', () => {
        expect(el.isUnique).to.be.false;
      });

      it('should set existingPage from response', () => {
        expect(el.existingPage).to.deep.equal(mockExistingPage);
      });

      it('should clear automagicError', () => {
        expect(el.automagicError).to.be.null;
      });
    });

    describe('when title is entered in automagic mode and result has no existingPage', () => {
      beforeEach(async () => {
        generateIdentifierStub.resolves({
          identifier: 'unique_item',
          isUnique: true,
          existingPage: undefined,
        });
        searchContentStub.resolves({ results: [] });

        el.openDialog('drawer_kitchen');
        el.automagicMode = true;
        await el.updateComplete;

        // Trigger title input
        const titleInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
        if (titleInput) {
          titleInput.value = 'Unique Item';
          titleInput.dispatchEvent(new Event('input'));
        }

        // Advance past debounce
        await clock.tickAsync(350);
        await el.updateComplete;
      });

      it('should not set existingPage', () => {
        expect(el.existingPage).to.be.undefined;
      });

      it('should set isUnique to true', () => {
        expect(el.isUnique).to.be.true;
      });
    });

    describe('when title is entered in automagic mode and generateIdentifier fails', () => {
      let testError: Error;

      beforeEach(async () => {
        testError = new Error('Server unavailable');
        generateIdentifierStub.resolves({
          identifier: '',
          isUnique: true,
          error: testError,
        });
        searchContentStub.resolves({ results: [] });

        el.openDialog('drawer_kitchen');
        el.automagicMode = true;
        await el.updateComplete;

        // Trigger title input
        const titleInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
        if (titleInput) {
          titleInput.value = 'Test Item';
          titleInput.dispatchEvent(new Event('input'));
        }

        // Advance past debounce
        await clock.tickAsync(350);
        await el.updateComplete;
      });

      it('should set automagicError', () => {
        expect(el.automagicError).to.exist;
      });

      it('should have correct failedGoalDescription in automagicError', () => {
        expect(el.automagicError?.failedGoalDescription).to.equal('generating identifier');
      });
    });

    describe('when title is entered in manual mode', () => {
      beforeEach(async () => {
        searchContentStub.resolves({ results: [] });

        el.openDialog('drawer_kitchen');
        el.automagicMode = false;
        el.itemIdentifier = 'manual_id';
        await el.updateComplete;

        // Trigger title input
        const titleInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
        if (titleInput) {
          titleInput.value = 'Test Item';
          titleInput.dispatchEvent(new Event('input'));
        }

        // Advance past debounce
        await clock.tickAsync(350);
        await el.updateComplete;
      });

      it('should not call generateIdentifier', () => {
        expect(generateIdentifierStub).to.not.have.been.called;
      });

      it('should preserve manual identifier', () => {
        expect(el.itemIdentifier).to.equal('manual_id');
      });
    });

    describe('when search succeeds', () => {
      const mockResults = [
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock test data
        { identifier: 'item1', title: 'Similar Item', fragment: '', highlights: [], frontmatter: {} } as unknown as import('../gen/api/v1/search_pb.js').SearchResult,
      ];

      beforeEach(async () => {
        generateIdentifierStub.resolves({
          identifier: 'test_item',
          isUnique: true,
        });
        searchContentStub.resolves({ results: mockResults });

        el.openDialog('drawer_kitchen');
        el.automagicMode = true;
        await el.updateComplete;

        // Trigger title input
        const titleInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
        if (titleInput) {
          titleInput.value = 'Test';
          titleInput.dispatchEvent(new Event('input'));
        }

        // Advance past debounce
        await clock.tickAsync(350);
        await el.updateComplete;
      });

      it('should populate searchResults', () => {
        expect(el.searchResults).to.deep.equal(mockResults);
      });

      it('should set searchLoading to false', () => {
        expect(el.searchLoading).to.be.false;
      });
    });

    describe('when search fails', () => {
      beforeEach(async () => {
        generateIdentifierStub.resolves({
          identifier: 'test_item',
          isUnique: true,
        });
        searchContentStub.rejects(new Error('Search service unavailable'));

        el.openDialog('drawer_kitchen');
        el.automagicMode = true;
        await el.updateComplete;

        // Trigger title input
        const titleInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
        if (titleInput) {
          titleInput.value = 'Test';
          titleInput.dispatchEvent(new Event('input'));
        }

        // Advance past debounce
        await clock.tickAsync(350);
        await el.updateComplete;
      });

      it('should clear searchResults', () => {
        expect(el.searchResults).to.deep.equal([]);
      });

      it('should set error', () => {
        expect(el.error).to.be.instanceOf(Error);
      });

      it('should set searchLoading to false', () => {
        expect(el.searchLoading).to.be.false;
      });
    });
  });

  describe('async identifier input handling with debounce', () => {
    let clock: sinon.SinonFakeTimers;
    let generateIdentifierStub: SinonStub;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private property for testing
    let inventoryItemCreatorMover: any;

    beforeEach(async () => {
      clock = sinon.useFakeTimers();
      el = await fixture<InventoryAddItemDialog>(html`<inventory-add-item-dialog></inventory-add-item-dialog>`);
      await el.updateComplete;

      // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
      inventoryItemCreatorMover = (el as any).inventoryItemCreatorMover as InventoryItemCreatorMover;
      generateIdentifierStub = sinon.stub(inventoryItemCreatorMover, 'generateIdentifier');
    });

    afterEach(() => {
      clock.restore();
      sinon.restore();
    });

    describe('when identifier is cleared in manual mode', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.automagicMode = false;
        el.isUnique = false;
        el.itemIdentifier = '';  // Set identifier to empty to trigger the clear path
        await el.updateComplete;

        // Call the method directly to verify the clearing behavior
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        await (el as unknown as { _checkIdentifierAvailability: () => Promise<void> })._checkIdentifierAvailability();
        await el.updateComplete;
      });

      it('should set isUnique to true', () => {
        expect(el.isUnique).to.be.true;
      });

      it('should not have existingPage set', () => {
        expect(el.existingPage).to.be.undefined;
      });
    });

    describe('when identifier is entered in manual mode and check succeeds', () => {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock test data
      const mockExistingPage = {
        identifier: 'conflict_item',
        title: 'Conflicting Item',
        container: 'drawer_other',
      } as ExistingPageInfo;

      beforeEach(async () => {
        generateIdentifierStub.resolves({
          identifier: 'conflict_item',
          isUnique: false,
          existingPage: mockExistingPage,
        });

        el.openDialog('drawer_kitchen');
        el.automagicMode = false;
        await el.updateComplete;

        // Trigger identifier input
        const identifierInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="itemIdentifier"]');
        if (identifierInput) {
          identifierInput.value = 'conflict_item';
          identifierInput.dispatchEvent(new Event('input'));
        }

        // Advance past debounce
        await clock.tickAsync(350);
        await el.updateComplete;
      });

      it('should call generateIdentifier with the identifier', () => {
        expect(generateIdentifierStub).to.have.been.calledWith('conflict_item');
      });

      it('should set isUnique from response', () => {
        expect(el.isUnique).to.be.false;
      });

      it('should set existingPage from response', () => {
        expect(el.existingPage).to.deep.equal(mockExistingPage);
      });
    });

    describe('when identifier check returns no existingPage', () => {
      beforeEach(async () => {
        generateIdentifierStub.resolves({
          identifier: 'unique_item',
          isUnique: true,
          existingPage: undefined,
        });

        el.openDialog('drawer_kitchen');
        el.automagicMode = false;
        await el.updateComplete;

        // Trigger identifier input
        const identifierInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="itemIdentifier"]');
        if (identifierInput) {
          identifierInput.value = 'unique_item';
          identifierInput.dispatchEvent(new Event('input'));
        }

        // Advance past debounce
        await clock.tickAsync(350);
        await el.updateComplete;
      });

      it('should not set existingPage', () => {
        expect(el.existingPage).to.be.undefined;
      });
    });

    describe('when identifier check fails with error', () => {
      beforeEach(async () => {
        generateIdentifierStub.resolves({
          identifier: '',
          isUnique: true,
          error: new Error('Server error'),
        });

        el.openDialog('drawer_kitchen');
        el.automagicMode = false;
        el.isUnique = false;
        await el.updateComplete;

        // Trigger identifier input
        const identifierInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="itemIdentifier"]');
        if (identifierInput) {
          identifierInput.value = 'test_id';
          identifierInput.dispatchEvent(new Event('input'));
        }

        // Advance past debounce
        await clock.tickAsync(350);
        await el.updateComplete;
      });

      it('should not update isUnique on error', () => {
        // When error occurs, the method doesn't update state
        expect(el.isUnique).to.be.false;
      });
    });
  });

  describe('automagic toggle with regeneration', () => {
    let clock: sinon.SinonFakeTimers;
    let generateIdentifierStub: SinonStub;
    let searchContentStub: SinonStub;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private property for testing
    let inventoryItemCreatorMover: any;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private property for testing
    let searchClient: any;

    beforeEach(async () => {
      clock = sinon.useFakeTimers();
      el = await fixture<InventoryAddItemDialog>(html`<inventory-add-item-dialog></inventory-add-item-dialog>`);
      await el.updateComplete;

      // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
      inventoryItemCreatorMover = (el as any).inventoryItemCreatorMover as InventoryItemCreatorMover;
      // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
      searchClient = (el as any).searchClient;

      generateIdentifierStub = sinon.stub(inventoryItemCreatorMover, 'generateIdentifier');
      searchContentStub = sinon.stub(searchClient, 'searchContent');
    });

    afterEach(() => {
      clock.restore();
      sinon.restore();
    });

    describe('when switching to automagic mode with existing title', () => {
      beforeEach(async () => {
        generateIdentifierStub.resolves({
          identifier: 'regenerated_id',
          isUnique: true,
        });
        searchContentStub.resolves({ results: [] });

        el.openDialog('drawer_kitchen');
        el.automagicMode = false;
        el.itemTitle = 'Test Item';
        el.itemIdentifier = 'manual_id';
        await el.updateComplete;

        // Toggle back to automagic
        const toggleBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.automagic-button');
        toggleBtn?.click();

        // Allow async operations to complete
        await clock.tickAsync(50);
        await el.updateComplete;
      });

      it('should switch to automagic mode', () => {
        expect(el.automagicMode).to.be.true;
      });

      it('should call generateIdentifier to regenerate', () => {
        expect(generateIdentifierStub).to.have.been.calledWith('Test Item');
      });

      it('should update itemIdentifier from regeneration', () => {
        expect(el.itemIdentifier).to.equal('regenerated_id');
      });
    });

    describe('when switching to automagic mode with empty title', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.automagicMode = false;
        el.itemTitle = '';
        await el.updateComplete;

        // Toggle back to automagic
        const toggleBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.automagic-button');
        toggleBtn?.click();

        await clock.tickAsync(50);
        await el.updateComplete;
      });

      it('should switch to automagic mode', () => {
        expect(el.automagicMode).to.be.true;
      });

      it('should not call generateIdentifier', () => {
        expect(generateIdentifierStub).to.not.have.been.called;
      });
    });
  });

  describe('form submission', () => {
    let addItemStub: SinonStub;
    let showSuccessStub: SinonStub;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private property for testing
    let inventoryItemCreatorMover: any;

    beforeEach(async () => {
      el = await fixture<InventoryAddItemDialog>(html`<inventory-add-item-dialog></inventory-add-item-dialog>`);
      await el.updateComplete;

      // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
      inventoryItemCreatorMover = (el as any).inventoryItemCreatorMover as InventoryItemCreatorMover;
      addItemStub = sinon.stub(inventoryItemCreatorMover, 'addItem');
      showSuccessStub = sinon.stub(inventoryItemCreatorMover, 'showSuccess');
    });

    afterEach(() => {
      sinon.restore();
    });

    describe('when form cannot be submitted', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.itemTitle = '';
        el.itemIdentifier = '';
        await el.updateComplete;

        // Click submit button (should be disabled, but test the handler)
        const addBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-primary');
        addBtn?.click();
        await el.updateComplete;
      });

      it('should not call addItem', () => {
        expect(addItemStub).to.not.have.been.called;
      });
    });

    describe('when submission succeeds', () => {
      let closeSpy: sinon.SinonSpy;

      beforeEach(async () => {
        addItemStub.resolves({
          success: true,
          itemIdentifier: 'screwdriver',
          summary: 'Added Screwdriver to drawer_kitchen',
        });

        el.openDialog('drawer_kitchen');
        el.itemTitle = 'Screwdriver';
        el.itemIdentifier = 'screwdriver';
        el.description = 'A handy tool';
        el.isUnique = true;
        await el.updateComplete;

        closeSpy = sinon.spy(el, 'close');

        const addBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-primary');
        addBtn?.click();
        await el.updateComplete;
      });

      it('should call addItem with correct parameters', () => {
        expect(addItemStub).to.have.been.calledWith(
          'drawer_kitchen',
          'screwdriver',
          'Screwdriver',
          'A handy tool'
        );
      });

      it('should show success message', () => {
        expect(showSuccessStub).to.have.been.called;
      });

      it('should use response summary in success message', () => {
        expect(showSuccessStub.firstCall.args[0]).to.equal('Added Screwdriver to drawer_kitchen');
      });

      it('should close the dialog', () => {
        expect(closeSpy).to.have.been.calledOnce;
      });

      it('should set loading to false', () => {
        expect(el.loading).to.be.false;
      });
    });

    describe('when submission succeeds without custom summary', () => {
      beforeEach(async () => {
        addItemStub.resolves({
          success: true,
          itemIdentifier: 'screwdriver',
          summary: undefined,
        });

        el.openDialog('drawer_kitchen');
        el.itemTitle = 'Screwdriver';
        el.itemIdentifier = 'screwdriver';
        el.isUnique = true;
        await el.updateComplete;

        const addBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-primary');
        addBtn?.click();
        await el.updateComplete;
      });

      it('should use fallback success message', () => {
        expect(showSuccessStub.firstCall.args[0]).to.equal('Added Screwdriver to drawer_kitchen');
      });
    });

    describe('when submission succeeds without description', () => {
      beforeEach(async () => {
        addItemStub.resolves({
          success: true,
          itemIdentifier: 'screwdriver',
        });

        el.openDialog('drawer_kitchen');
        el.itemTitle = 'Screwdriver';
        el.itemIdentifier = 'screwdriver';
        el.description = '';
        el.isUnique = true;
        await el.updateComplete;

        const addBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-primary');
        addBtn?.click();
        await el.updateComplete;
      });

      it('should call addItem with undefined description', () => {
        expect(addItemStub).to.have.been.calledWith(
          'drawer_kitchen',
          'screwdriver',
          'Screwdriver',
          undefined
        );
      });
    });

    describe('when submission fails with error', () => {
      let testError: Error;

      beforeEach(async () => {
        testError = new Error('Item creation failed');
        addItemStub.resolves({
          success: false,
          error: testError,
        });

        el.openDialog('drawer_kitchen');
        el.itemTitle = 'Screwdriver';
        el.itemIdentifier = 'screwdriver';
        el.isUnique = true;
        await el.updateComplete;

        const addBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.button-primary');
        addBtn?.click();
        await el.updateComplete;
      });

      it('should set error', () => {
        expect(el.error).to.equal(testError);
      });

      it('should set loading to false', () => {
        expect(el.loading).to.be.false;
      });

      it('should not close the dialog', () => {
        expect(el.open).to.be.true;
      });
    });

    describe('when submission fails without error object', () => {
      let thrownError: Error | null;

      beforeEach(async () => {
        addItemStub.resolves({
          success: false,
          error: undefined,
        });

        el.openDialog('drawer_kitchen');
        el.itemTitle = 'Screwdriver';
        el.itemIdentifier = 'screwdriver';
        el.isUnique = true;
        await el.updateComplete;

        thrownError = null;

        // Wrap the submit call in a promise that catches the error
        try {
          // Access the private method directly to catch the thrown error
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
          await (el as unknown as { _handleSubmit: () => Promise<void> })._handleSubmit();
        } catch (err) {
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- coercing caught error for testing
          thrownError = err as Error;
        }
      });

      it('should throw an error', () => {
        expect(thrownError).to.be.instanceOf(Error);
      });

      it('should include descriptive error message', () => {
        expect(thrownError?.message).to.contain('success=false without an error');
      });
    });
  });

  describe('switch to manual mode from automagic error', () => {
    beforeEach(async () => {
      el = await fixture<InventoryAddItemDialog>(html`<inventory-add-item-dialog></inventory-add-item-dialog>`);
      await el.updateComplete;
    });

    afterEach(() => {
      sinon.restore();
    });

    describe('when _handleSwitchToManual is called', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.automagicMode = true;
        el.automagicError = new AugmentedError(
          new Error('Test error'),
          ErrorKind.SERVER,
          'server',
          'testing'
        );
        el.itemIdentifier = 'auto_generated';
        await el.updateComplete;

        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        (el as unknown as { _handleSwitchToManual: () => void })._handleSwitchToManual();
        await el.updateComplete;
      });

      it('should set automagicMode to false', () => {
        expect(el.automagicMode).to.be.false;
      });

      it('should clear automagicError', () => {
        expect(el.automagicError).to.be.null;
      });

      it('should clear itemIdentifier', () => {
        expect(el.itemIdentifier).to.equal('');
      });
    });
  });

  describe('debounce timer cleanup', () => {
    let clock: sinon.SinonFakeTimers;
    let generateIdentifierStub: SinonStub;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private property for testing
    let inventoryItemCreatorMover: any;

    beforeEach(async () => {
      clock = sinon.useFakeTimers();
      el = await fixture<InventoryAddItemDialog>(html`<inventory-add-item-dialog></inventory-add-item-dialog>`);
      await el.updateComplete;

      // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
      inventoryItemCreatorMover = (el as any).inventoryItemCreatorMover as InventoryItemCreatorMover;
      generateIdentifierStub = sinon.stub(inventoryItemCreatorMover, 'generateIdentifier');
      generateIdentifierStub.resolves({ identifier: 'test', isUnique: true });
    });

    afterEach(() => {
      clock.restore();
      sinon.restore();
    });

    describe('when component is disconnected with pending timers', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.automagicMode = false;
        await el.updateComplete;

        // Trigger identifier input to start a timer
        const identifierInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="itemIdentifier"]');
        if (identifierInput) {
          identifierInput.value = 'test';
          identifierInput.dispatchEvent(new Event('input'));
        }

        // Remove component before timer fires
        el.remove();
        await clock.tickAsync(350);
      });

      it('should not call generateIdentifier after disconnect', () => {
        expect(generateIdentifierStub).to.not.have.been.called;
      });
    });

    describe('when close is called with pending timers', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.automagicMode = false;
        await el.updateComplete;

        // Trigger identifier input to start a timer
        const identifierInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="itemIdentifier"]');
        if (identifierInput) {
          identifierInput.value = 'test';
          identifierInput.dispatchEvent(new Event('input'));
        }

        // Close before timer fires
        el.close();
        await clock.tickAsync(350);
      });

      it('should not call generateIdentifier after close', () => {
        expect(generateIdentifierStub).to.not.have.been.called;
      });
    });

    describe('when rapid title input occurs', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.automagicMode = true;
        await el.updateComplete;

        const titleInput = el.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
        if (titleInput) {
          // Rapid input - each should cancel the previous timer
          titleInput.value = 'a';
          titleInput.dispatchEvent(new Event('input'));
          await clock.tickAsync(100);

          titleInput.value = 'ab';
          titleInput.dispatchEvent(new Event('input'));
          await clock.tickAsync(100);

          titleInput.value = 'abc';
          titleInput.dispatchEvent(new Event('input'));
          await clock.tickAsync(350);
        }
      });

      it('should only call generateIdentifier once with final value', () => {
        expect(generateIdentifierStub).to.have.been.calledOnce;
        expect(generateIdentifierStub).to.have.been.calledWith('abc');
      });
    });
  });

  describe('search with empty query', () => {
    let searchContentStub: SinonStub;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- accessing private property for testing
    let searchClient: any;

    beforeEach(async () => {
      el = await fixture<InventoryAddItemDialog>(html`<inventory-add-item-dialog></inventory-add-item-dialog>`);
      await el.updateComplete;

      // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
      searchClient = (el as any).searchClient;
      searchContentStub = sinon.stub(searchClient, 'searchContent');
    });

    afterEach(() => {
      sinon.restore();
    });

    describe('when _performSearch is called with empty query', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.searchResults = [
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock test data
          { identifier: 'item1', title: 'Item', fragment: '', highlights: [], frontmatter: {} } as unknown as import('../gen/api/v1/search_pb.js').SearchResult,
        ];
        await el.updateComplete;

        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        await (el as unknown as { _performSearch: (query: string) => Promise<void> })._performSearch('');
        await el.updateComplete;
      });

      it('should clear searchResults', () => {
        expect(el.searchResults).to.deep.equal([]);
      });

      it('should not call searchContent', () => {
        expect(searchContentStub).to.not.have.been.called;
      });
    });
  });
});
