import { expect } from '@esm-bundle/chai';
import { stub, type SinonStub } from 'sinon';
import {
  registerDrawer,
  registerAmbientCTA,
  notifyDrawerOpened,
  notifyDrawerClosed,
  resetForTesting,
  type DrawerParticipant,
  type AmbientCTA,
} from './drawer-coordinator.js';

function createMockDrawer(id: string): DrawerParticipant & { closeDrawer: SinonStub } {
  return { drawerId: id, closeDrawer: stub() };
}

function createMockCTA(): AmbientCTA & { setAmbientVisible: SinonStub } {
  return { setAmbientVisible: stub() };
}

describe('DrawerCoordinator', () => {
  afterEach(() => {
    resetForTesting();
  });

  describe('when a drawer opens', () => {
    let drawerA: ReturnType<typeof createMockDrawer>;
    let drawerB: ReturnType<typeof createMockDrawer>;

    beforeEach(() => {
      drawerA = createMockDrawer('a');
      drawerB = createMockDrawer('b');
      registerDrawer(drawerA);
      registerDrawer(drawerB);
      notifyDrawerOpened('a');
    });

    it('should close all other drawers', () => {
      expect(drawerB.closeDrawer.calledOnce).to.equal(true);
    });

    it('should not close the opening drawer', () => {
      expect(drawerA.closeDrawer.called).to.equal(false);
    });
  });

  describe('when a drawer opens and CTAs are registered', () => {
    let cta1: ReturnType<typeof createMockCTA>;
    let cta2: ReturnType<typeof createMockCTA>;

    beforeEach(() => {
      cta1 = createMockCTA();
      cta2 = createMockCTA();
      registerDrawer(createMockDrawer('a'));
      registerAmbientCTA(cta1);
      registerAmbientCTA(cta2);
      notifyDrawerOpened('a');
    });

    it('should hide all CTAs', () => {
      expect(cta1.setAmbientVisible.calledWith(false)).to.equal(true);
    });

    it('should hide the second CTA too', () => {
      expect(cta2.setAmbientVisible.calledWith(false)).to.equal(true);
    });
  });

  describe('when all drawers close', () => {
    let cta: ReturnType<typeof createMockCTA>;

    beforeEach(() => {
      cta = createMockCTA();
      registerDrawer(createMockDrawer('a'));
      registerAmbientCTA(cta);
      notifyDrawerOpened('a');
      cta.setAmbientVisible.resetHistory();
      notifyDrawerClosed('a');
    });

    it('should show all CTAs', () => {
      expect(cta.setAmbientVisible.calledWith(true)).to.equal(true);
    });
  });

  describe('when one drawer closes but another is still open', () => {
    let cta: ReturnType<typeof createMockCTA>;

    beforeEach(() => {
      cta = createMockCTA();
      registerDrawer(createMockDrawer('a'));
      registerDrawer(createMockDrawer('b'));
      registerAmbientCTA(cta);
      notifyDrawerOpened('a');
      notifyDrawerOpened('b');
      cta.setAmbientVisible.resetHistory();
      notifyDrawerClosed('a');
    });

    it('should not show CTAs', () => {
      expect(cta.setAmbientVisible.called).to.equal(false);
    });
  });

  describe('when a drawer is deregistered', () => {
    let drawer: ReturnType<typeof createMockDrawer>;

    beforeEach(() => {
      drawer = createMockDrawer('a');
      const deregister = registerDrawer(drawer);
      deregister();
      notifyDrawerOpened('b');
    });

    it('should not receive closeDrawer calls', () => {
      expect(drawer.closeDrawer.called).to.equal(false);
    });
  });

  describe('when a CTA is deregistered', () => {
    let cta: ReturnType<typeof createMockCTA>;

    beforeEach(() => {
      cta = createMockCTA();
      registerDrawer(createMockDrawer('a'));
      const deregister = registerAmbientCTA(cta);
      deregister();
      notifyDrawerOpened('a');
    });

    it('should not receive setAmbientVisible calls', () => {
      expect(cta.setAmbientVisible.called).to.equal(false);
    });
  });

  describe('mutual exclusion', () => {
    let drawerA: ReturnType<typeof createMockDrawer>;
    let drawerB: ReturnType<typeof createMockDrawer>;

    beforeEach(() => {
      drawerA = createMockDrawer('a');
      drawerB = createMockDrawer('b');
      registerDrawer(drawerA);
      registerDrawer(drawerB);
      notifyDrawerOpened('a');
      drawerB.closeDrawer.resetHistory();
      notifyDrawerOpened('b');
    });

    it('should close drawer A when drawer B opens', () => {
      expect(drawerA.closeDrawer.calledOnce).to.equal(true);
    });

    it('should not close the newly opening drawer B', () => {
      expect(drawerB.closeDrawer.called).to.equal(false);
    });
  });
});
