import { html, fixture, expect } from '@open-wc/testing';
import sinon, { type SinonStub } from 'sinon';
import type { MenuPopover } from './menu-popover.js';
import './menu-popover.js';

interface TriggerRect {
  top: number;
  bottom: number;
  left: number;
  right: number;
}

const viewportWidth = (): number => window.innerWidth;
const viewportHeight = (): number => window.innerHeight;

/** Near the bottom-right corner of whatever the runner's viewport is. */
function nearBottomRight(): TriggerRect {
  const w = viewportWidth();
  const h = viewportHeight();
  return { top: h - 40, bottom: h - 10, left: w - 60, right: w - 10 };
}

/** Near the top-left with ample room below/right — the default direction. */
function defaultPosition(): TriggerRect {
  return { top: 100, bottom: 130, left: 100, right: 200 };
}

async function createPopover(
  triggerRect: TriggerRect = defaultPosition(),
): Promise<{ popover: MenuPopover; stub: SinonStub }> {
  const container = await fixture<HTMLDivElement>(html`
    <div>
      <button class="trigger">trigger</button>
      <menu-popover>
        <button class="option">Option 1</button>
        <button class="option">Option 2</button>
      </menu-popover>
    </div>
  `);
  const anchor = container.querySelector<HTMLElement>('.trigger');
  const popover = container.querySelector<MenuPopover>('menu-popover');
  if (!anchor || !popover) throw new Error('fixture did not build');
  popover.anchor = anchor;
  const stub = sinon.stub(anchor, 'getBoundingClientRect');
  stub.returns(new DOMRect(triggerRect.left, triggerRect.top, triggerRect.right - triggerRect.left, triggerRect.bottom - triggerRect.top));
  await popover.updateComplete;
  return { popover, stub };
}

/** Creates and opens a popover; placement is synchronous in updated(). */
async function open(triggerRect?: TriggerRect): Promise<{ popover: MenuPopover; stub: SinonStub }> {
  const built = await createPopover(triggerRect);
  built.popover.open = true;
  await built.popover.updateComplete;
  return built;
}

function surfaceOf(popover: MenuPopover): HTMLElement {
  const surface = popover.shadowRoot?.querySelector<HTMLElement>('.menu-surface');
  if (!surface) throw new Error('menu surface not rendered');
  return surface;
}

function moveAnchor(stub: SinonStub, rect: TriggerRect): void {
  stub.resetBehavior();
  stub.returns(new DOMRect(rect.left, rect.top, rect.right - rect.left, rect.bottom - rect.top));
}

function closeRequestsOf(popover: MenuPopover): { count: () => number } {
  let count = 0;
  popover.addEventListener('menu-popover-close-request', () => {
    count += 1;
  });
  return { count: () => count };
}

describe('MenuPopover', () => {
  describe('when opened with room below and to the right', () => {
    let built!: { popover: MenuPopover; stub: SinonStub };

    beforeEach(async () => {
      built = await open();
    });

    it('should render the slotted options in the surface slot', () => {
      const slot = surfaceOf(built.popover).querySelector('slot');
      expect(slot?.assignedElements().length).to.equal(2);
    });

    it('should mark the surface placed', () => {
      expect(surfaceOf(built.popover).hasAttribute('data-placed')).to.equal(true);
    });

    it('should place the menu below the trigger within the viewport', () => {
      const style = surfaceOf(built.popover).style;
      const rect = surfaceOf(built.popover).getBoundingClientRect();
      expect(style.top).to.not.equal('');
      expect(style.bottom).to.equal('');
      expect(rect.top).to.be.at.least(130); // anchor bottom + gap
      expect(rect.right).to.be.at.most(viewportWidth());
    });
  });

  describe('when the trigger sits near the viewport bottom', () => {
    it('should flip the menu above the trigger', async () => {
      const h = viewportHeight();
      const { popover } = await open({ top: h - 40, bottom: h - 10, left: 100, right: 200 });
      const style = surfaceOf(popover).style;
      expect(style.bottom).to.not.equal('');
      expect(style.top).to.equal('');
    });
  });

  describe('when the trigger sits near the viewport right edge', () => {
    it('should mirror the menu to grow leftward', async () => {
      const w = viewportWidth();
      const { popover } = await open({ top: 100, bottom: 130, left: w - 30, right: w - 10 });
      const rect = surfaceOf(popover).getBoundingClientRect();
      // The 220px-min menu cannot fit left of a near-edge anchor, so the
      // clamp pulls its left edge back inside the viewport: the menu grows
      // leftward and stays fully within the window.
      expect(rect.left).to.be.at.most(viewportWidth() - 220);
    });
  });

  describe('when the menu content is taller than the available space', () => {
    it('should cap the surface to the available viewport space', async () => {
      const { popover } = await open();
      const tall = document.createElement('div');
      tall.style.height = '3000px';
      tall.textContent = 'tall';
      popover.appendChild(tall);
      popover.place();
      await popover.updateComplete;

      // maxHeight must cap to available viewport space, not content height.
      const maxHeightPx = Number.parseInt(surfaceOf(popover).style.maxHeight, 10);
      expect(maxHeightPx).to.be.below(viewportHeight());
      expect(maxHeightPx).to.be.above(0);
    });
  });
  describe('when the anchor is missing', () => {
    it('should leave the surface hidden', async () => {
      const popover = await fixture<MenuPopover>(html`
        <menu-popover><div>content</div></menu-popover>
      `);
      popover.open = true;
      await popover.updateComplete;
      expect(surfaceOf(popover).hasAttribute('data-placed')).to.equal(false);
    });
  });
  describe('when outside click lands', () => {
    it('should dispatch a close-request', async () => {
      const { popover } = await open();
      const counter = closeRequestsOf(popover);
      document.body.click();
      expect(counter.count()).to.equal(1);
    });
  });

  describe('when Escape is pressed', () => {
    it('should dispatch a close-request', async () => {
      const { popover } = await open();
      const counter = closeRequestsOf(popover);
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      expect(counter.count()).to.equal(1);
    });
  });

  describe('when placement changes while open', () => {
    it('should re-place and keep the surface inside the viewport', async () => {
      const { popover, stub } = await open(nearBottomRight());
      popover.placement = 'bottom-start';
      popover.place();
      await popover.updateComplete;
      const surface = surfaceOf(popover);
      expect(surface.hasAttribute('data-placed')).to.equal(true);
      const rect = surface.getBoundingClientRect();
      expect(rect.right).to.be.at.most(viewportWidth() + 1);
      expect(rect.left).to.be.at.least(0);
      stub.restore();
    });
  });

  describe('when the window resizes while open', () => {
    it('should re-place the surface inside the new viewport bounds', async () => {
      const { popover, stub } = await open(defaultPosition());
      expect(surfaceOf(popover).style.top).to.not.equal('');
      // The anchor moves to the right edge (as if the window narrowed);
      // the resize-triggered re-place must mirror the menu leftward.
      moveAnchor(stub, nearBottomRight());
      window.dispatchEvent(new Event('resize'));
      await popover.updateComplete;
      const surface = surfaceOf(popover);
      expect(surface.hasAttribute('data-placed')).to.equal(true);
      const rect2 = surface.getBoundingClientRect();
      expect(rect2.right).to.be.at.most(viewportWidth() + 1);
      expect(rect2.left).to.be.at.least(0);
      stub.restore();
    });
  });

  describe('when placed against a near-edge trigger', () => {
    it('should keep the surface within the viewport bounds', async () => {
      const { popover, stub } = await open(nearBottomRight());
      const rect = surfaceOf(popover).getBoundingClientRect();
      expect(rect.left).to.be.at.least(0);
      expect(rect.right).to.be.at.most(viewportWidth());
      expect(rect.top).to.be.at.least(0);
      expect(rect.bottom).to.be.at.most(viewportHeight());
      stub.restore();
    });
  });

  describe('when the surface is placed', () => {
    it('should make the surface visible (not visibility:hidden)', async () => {
      const { popover, stub } = await open();
      const surface = surfaceOf(popover);
      expect(surface.hasAttribute('data-placed')).to.equal(true);
      expect(getComputedStyle(surface).visibility).to.equal('visible');
      stub.restore();
    });
  });

  describe('when the anchor itself is clicked while open', () => {
    it('should not dispatch a close-request', async () => {
      const container = await fixture<HTMLDivElement>(html`
        <div>
          <button class="trigger">trigger</button>
          <menu-popover><button class="option">A</button></menu-popover>
        </div>
      `);
      const anchor = container.querySelector<HTMLElement>('.trigger')!;
      const popover = container.querySelector<MenuPopover>('menu-popover')!;
      popover.anchor = anchor;
      await popover.updateComplete;
      popover.open = true;
      await popover.updateComplete;

      const counter = closeRequestsOf(popover);
      anchor.click();
      expect(counter.count()).to.equal(0);
    });
  });

  describe('when the anchor is detached while open', () => {
    it('should fall back to host-rect placement and stay visible', async () => {
      const { popover, stub } = await open();
      const anchor = popover.anchor!;
      anchor.parentElement?.removeChild(anchor);
      popover.place();
      await popover.updateComplete;
      const surface = surfaceOf(popover);
      expect(surface.hasAttribute('data-placed')).to.equal(true);
      stub.restore();
    });
  });
});