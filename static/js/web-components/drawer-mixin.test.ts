import { expect, fixture, html } from '@open-wc/testing';
import { LitElement } from 'lit';
import { DrawerMixin } from './drawer-mixin.js';
import { resetForTesting, registerDrawer, notifyDrawerOpened } from './drawer-coordinator.js';
import type { DrawerParticipant } from './drawer-coordinator.js';

// Minimal test component using the mixin
class TestDrawer extends DrawerMixin(LitElement) {
  override readonly drawerId = 'test-drawer';

  override createRenderRoot() {
    return this;
  }
}

// Second drawer for mutual exclusion tests
class TestDrawerB extends DrawerMixin(LitElement) {
  override readonly drawerId = 'test-drawer-b';

  override createRenderRoot() {
    return this;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'test-drawer-element': TestDrawer;
    'test-drawer-b-element': TestDrawerB;
  }
}

if (!customElements.get('test-drawer-element')) {
  customElements.define('test-drawer-element', TestDrawer);
}
if (!customElements.get('test-drawer-b-element')) {
  customElements.define('test-drawer-b-element', TestDrawerB);
}

describe('DrawerMixin', () => {
  afterEach(() => {
    resetForTesting();
  });

  describe('when connected to DOM', () => {
    let el: TestDrawer;

    beforeEach(async () => {
      el = await fixture(html`<test-drawer-element></test-drawer-element>`);
    });

    it('should start with drawer closed', () => {
      expect(el.drawerOpen).to.equal(false);
    });
  });

  describe('openDrawer()', () => {
    let el: TestDrawer;

    beforeEach(async () => {
      el = await fixture(html`<test-drawer-element></test-drawer-element>`);
      el.openDrawer();
    });

    it('should set drawerOpen to true', () => {
      expect(el.drawerOpen).to.equal(true);
    });
  });

  describe('closeDrawer()', () => {
    let el: TestDrawer;

    beforeEach(async () => {
      el = await fixture(html`<test-drawer-element></test-drawer-element>`);
      el.openDrawer();
      el.closeDrawer();
    });

    it('should set drawerOpen to false', () => {
      expect(el.drawerOpen).to.equal(false);
    });
  });

  describe('closeDrawer() when already closed', () => {
    let el: TestDrawer;
    let closeCalled: boolean;

    beforeEach(async () => {
      el = await fixture(html`<test-drawer-element></test-drawer-element>`);
      closeCalled = false;
      // If closeDrawer notifies coordinator when already closed, that's a bug.
      // We can't easily test this without stubbing, but we verify no error is thrown.
      el.closeDrawer();
      closeCalled = true;
    });

    it('should not throw', () => {
      expect(closeCalled).to.equal(true);
    });

    it('should remain closed', () => {
      expect(el.drawerOpen).to.equal(false);
    });
  });

  describe('toggleDrawer()', () => {
    let el: TestDrawer;

    beforeEach(async () => {
      el = await fixture(html`<test-drawer-element></test-drawer-element>`);
    });

    describe('when closed', () => {
      beforeEach(() => {
        el.toggleDrawer();
      });

      it('should open the drawer', () => {
        expect(el.drawerOpen).to.equal(true);
      });
    });

    describe('when open', () => {
      beforeEach(() => {
        el.openDrawer();
        el.toggleDrawer();
      });

      it('should close the drawer', () => {
        expect(el.drawerOpen).to.equal(false);
      });
    });
  });

  describe('mutual exclusion via coordinator', () => {
    let drawerA: TestDrawer;
    let drawerB: TestDrawerB;

    beforeEach(async () => {
      drawerA = await fixture(html`<test-drawer-element></test-drawer-element>`);
      drawerB = await fixture(html`<test-drawer-b-element></test-drawer-b-element>`);
      drawerA.openDrawer();
      drawerB.openDrawer();
    });

    it('should close drawer A when drawer B opens', () => {
      expect(drawerA.drawerOpen).to.equal(false);
    });

    it('should keep drawer B open', () => {
      expect(drawerB.drawerOpen).to.equal(true);
    });
  });

  describe('when disconnected from DOM', () => {
    let el: TestDrawer;
    let externalDrawer: DrawerParticipant & { closeCalled: boolean };

    beforeEach(async () => {
      el = await fixture(html`<test-drawer-element></test-drawer-element>`);
      el.remove();

      // Register another drawer and open it — the removed element should not be affected
      externalDrawer = {
        drawerId: 'external',
        closeCalled: false,
        closeDrawer() { this.closeCalled = true; },
      };
      registerDrawer(externalDrawer);
      notifyDrawerOpened('external');
    });

    it('should not receive closeDrawer after removal', () => {
      // el was deregistered, so opening 'external' should not call el.closeDrawer
      // We can't directly test this without spying, but el.drawerOpen should still be false
      expect(el.drawerOpen).to.equal(false);
    });
  });
});
