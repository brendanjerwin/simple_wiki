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

    it('should have empty newContainer by default', () => {
      expect(el.newContainer).to.equal('');
    });

    it('should not be loading by default', () => {
      expect(el.loading).to.be.false;
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

      it('should clear newContainer', () => {
        expect(el.newContainer).to.equal('');
      });

      it('should clear error', () => {
        expect(el.error).to.be.undefined;
      });
    });
  });

  describe('close', () => {
    beforeEach(() => {
      el.openDialog('screwdriver', 'drawer_kitchen');
      el.newContainer = 'toolbox_garage';
      el.close();
    });

    it('should set open to false', () => {
      expect(el.open).to.be.false;
    });

    it('should clear newContainer', () => {
      expect(el.newContainer).to.equal('');
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

      it('should render new container field', () => {
        const newContainerInput = el.shadowRoot?.querySelector('input[name="newContainer"]');
        expect(newContainerInput).to.exist;
      });

      it('should render cancel button', () => {
        const cancelBtn = el.shadowRoot?.querySelector('.button-secondary');
        expect(cancelBtn?.textContent).to.contain('Cancel');
      });

      it('should render move button', () => {
        const moveBtn = el.shadowRoot?.querySelector('.button-primary');
        expect(moveBtn?.textContent).to.contain('Move');
      });
    });

    describe('when dialog is closed', () => {
      it('should not have open attribute', () => {
        expect(el.hasAttribute('open')).to.be.false;
      });
    });

    describe('when loading', () => {
      beforeEach(async () => {
        el.openDialog('screwdriver', 'drawer_kitchen');
        el.newContainer = 'toolbox_garage';
        el.loading = true;
        await el.updateComplete;
      });

      it('should disable the move button', () => {
        const moveBtn = el.shadowRoot?.querySelector('.button-primary') as HTMLButtonElement;
        expect(moveBtn?.disabled).to.be.true;
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
  });

  describe('form validation', () => {
    describe('when new container is empty', () => {
      beforeEach(async () => {
        el.openDialog('screwdriver', 'drawer_kitchen');
        el.newContainer = '';
        await el.updateComplete;
      });

      it('should disable the move button', () => {
        const moveBtn = el.shadowRoot?.querySelector('.button-primary') as HTMLButtonElement;
        expect(moveBtn?.disabled).to.be.true;
      });
    });

    describe('when new container has value', () => {
      beforeEach(async () => {
        el.openDialog('screwdriver', 'drawer_kitchen');
        el.newContainer = 'toolbox_garage';
        await el.updateComplete;
      });

      it('should enable the move button', () => {
        const moveBtn = el.shadowRoot?.querySelector('.button-primary') as HTMLButtonElement;
        expect(moveBtn?.disabled).to.be.false;
      });
    });

    describe('when new container is same as current', () => {
      beforeEach(async () => {
        el.openDialog('screwdriver', 'drawer_kitchen');
        el.newContainer = 'drawer_kitchen';
        await el.updateComplete;
      });

      it('should disable the move button', () => {
        const moveBtn = el.shadowRoot?.querySelector('.button-primary') as HTMLButtonElement;
        expect(moveBtn?.disabled).to.be.true;
      });
    });
  });
});
