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
      fixture(html`<inventory-add-item-dialog></inventory-add-item-dialog>`),
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

    it('should not be loading by default', () => {
      expect(el.loading).to.be.false;
    });

    it('should have no error by default', () => {
      expect(el.error).to.be.undefined;
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

      it('should clear error', () => {
        expect(el.error).to.be.undefined;
      });
    });
  });

  describe('close', () => {
    beforeEach(() => {
      el.openDialog('drawer_kitchen');
      el.itemIdentifier = 'screwdriver';
      el.itemTitle = 'Phillips Screwdriver';
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

    it('should clear error', () => {
      expect(el.error).to.be.undefined;
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

      it('should render dialog title', () => {
        const title = el.shadowRoot?.querySelector('.dialog-title');
        expect(title?.textContent).to.contain('Add Item');
      });

      it('should render container field as readonly', () => {
        const containerInput = el.shadowRoot?.querySelector('input[name="container"]') as HTMLInputElement;
        expect(containerInput?.readOnly).to.be.true;
        expect(containerInput?.value).to.equal('drawer_kitchen');
      });

      it('should render item identifier field', () => {
        const itemInput = el.shadowRoot?.querySelector('input[name="itemIdentifier"]');
        expect(itemInput).to.exist;
      });

      it('should render title field', () => {
        const titleInput = el.shadowRoot?.querySelector('input[name="title"]');
        expect(titleInput).to.exist;
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
        const addBtn = el.shadowRoot?.querySelector('.button-primary') as HTMLButtonElement;
        expect(addBtn?.disabled).to.be.true;
      });
    });

    describe('when error is present', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.error = 'Item already exists';
        await el.updateComplete;
      });

      it('should display error message', () => {
        const errorDiv = el.shadowRoot?.querySelector('.error-message');
        expect(errorDiv?.textContent).to.contain('Item already exists');
      });
    });
  });

  describe('form validation', () => {
    describe('when item identifier is empty', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.itemIdentifier = '';
        await el.updateComplete;
      });

      it('should disable the add button', () => {
        const addBtn = el.shadowRoot?.querySelector('.button-primary') as HTMLButtonElement;
        expect(addBtn?.disabled).to.be.true;
      });
    });

    describe('when item identifier has value', () => {
      beforeEach(async () => {
        el.openDialog('drawer_kitchen');
        el.itemIdentifier = 'screwdriver';
        await el.updateComplete;
      });

      it('should enable the add button', () => {
        const addBtn = el.shadowRoot?.querySelector('.button-primary') as HTMLButtonElement;
        expect(addBtn?.disabled).to.be.false;
      });
    });
  });
});
