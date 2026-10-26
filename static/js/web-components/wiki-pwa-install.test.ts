/**
 * Manifest-only install (docs/proj-pwa.md: no service worker).
 * Network-free by construction: beforeinstallprompt and matchMedia are
 * sinon stubs. The Installed confirmation runs on the injectable
 * confirmationMs property (0 = immediate hide), never on real timers.
 */
import { expect, fixture, html } from '@open-wc/testing';
import sinon, { type SinonStub } from 'sinon';
// Side-effect import: runs customElements.define before any fixture mounts.
import './wiki-pwa-install.js';
import type { WikiPwaInstall, BeforeInstallPromptEvent } from './wiki-pwa-install.js';

const PAGE = 'recipes';

const ownServiceWorkerDescriptor = Object.getOwnPropertyDescriptor(navigator, 'serviceWorker');

function stubMatchMedia(standalone: boolean): void {
 sinon.stub(window, 'matchMedia').returns({
  matches: standalone,
  addEventListener: sinon.stub(),
  removeEventListener: sinon.stub(),
 } as unknown as MediaQueryList);
}

function restoreServiceWorker(): void {
 if (ownServiceWorkerDescriptor !== undefined) {
  Object.defineProperty(navigator, 'serviceWorker', ownServiceWorkerDescriptor);
  return;
 }
 delete (navigator as unknown as Record<string, unknown>)['serviceWorker'];
}

function stubServiceWorker(register: SinonStub): void {
 Object.defineProperty(navigator, 'serviceWorker', {
  value: { register },
  configurable: true,
  writable: true,
 });
}

type PromptEventHarness = {
 event: BeforeInstallPromptEvent;
 prompt: SinonStub;
};

function createPromptEvent(outcome: 'accepted' | 'dismissed' = 'accepted'): PromptEventHarness {
 // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- browser install-prompt event type is not in lib.dom
 const event = new Event('beforeinstallprompt', { cancelable: true }) as BeforeInstallPromptEvent;
 const prompt = sinon.stub().resolves();
 event.prompt = prompt;
 event.userChoice = Promise.resolve({ outcome, platform: 'test' });
 return { event, prompt };
}

/** Mount the element with stubbed platform APIs; returns after it settles. */
async function mountPwaInstall(options: {
 bare: boolean;
 standalone?: boolean;
 register?: SinonStub;
 confirmationMs?: number;
}): Promise<WikiPwaInstall> {
 stubMatchMedia(options.standalone ?? false);
 stubServiceWorker(options.register ?? sinon.stub().resolves({}));
 const mounted = await fixture<WikiPwaInstall>(html`
    <wiki-pwa-install
      .bare=${options.bare}
      .confirmationMs=${options.confirmationMs ?? 3000}
      page=${PAGE}
    ></wiki-pwa-install>
  `);
 await settle(mounted);
 return mounted;
}

/**
 * Drain async handler chains and Lit updates: a single updateComplete plus a
 * microtask turn. The earlier 10-iteration updateComplete loop hung WTR
 * sessions outright; one drain is enough for these sync-driven handlers.
 */
async function settle(el: WikiPwaInstall): Promise<void> {
 await el.updateComplete;
 await Promise.resolve();
}

function findButton(root: ShadowRoot | null, selector: string): HTMLButtonElement {
 const button = root?.querySelector<HTMLButtonElement>(selector);
 if (!button) throw new Error(`expected a ${selector} in the shadow root`);
 return button;
}

function findLink(root: ShadowRoot | null, selector: string): HTMLAnchorElement {
 const link = root?.querySelector<HTMLAnchorElement>(selector);
 if (!link) throw new Error(`expected a ${selector} in the shadow root`);
 return link;
}

function labelOf(element: Element): string {
 return element.textContent?.trim() ?? '';
}

function shadowText(el: WikiPwaInstall): string {
 return el.shadowRoot?.textContent ?? '';
}

describe('WikiPwaInstall', () => {
 let el: WikiPwaInstall;

 afterEach(() => {
  el?.remove();
  sinon.restore();
  restoreServiceWorker();
 });

 describe('when an install prompt event fires in bare mode', () => {
  let promptEvent: BeforeInstallPromptEvent;

  beforeEach(async () => {
   el = await mountPwaInstall({ bare: true });
   promptEvent = createPromptEvent().event;
   window.dispatchEvent(promptEvent);
   await settle(el);
  });

  it('renders an Install button', () => {
   expect(labelOf(findButton(el.shadowRoot, '.install-button'))).to.equal('Install');
  });

  it('suppresses the browser install infobar', () => {
   expect(promptEvent.defaultPrevented).to.be.true;
  });
 });

 describe('when the Install button is clicked and the browser accepts', () => {
  let promptStub: SinonStub;

  beforeEach(async () => {
   el = await mountPwaInstall({ bare: true });
   const harness = createPromptEvent('accepted');
   promptStub = harness.prompt;
   window.dispatchEvent(harness.event);
   await settle(el);
   findButton(el.shadowRoot, '.install-button').click();
   await settle(el);
  });

  it('calls prompt() on the captured event', () => {
   expect(promptStub).to.have.been.calledOnce;
  });

  it('shows the Installed confirmation', () => {
   expect(shadowText(el)).to.include('Installed');
  });
 });

 describe('when the Install button is clicked and the browser dismisses', () => {
  beforeEach(async () => {
   el = await mountPwaInstall({ bare: true });
   const harness = createPromptEvent('dismissed');
   window.dispatchEvent(harness.event);
   await settle(el);
   findButton(el.shadowRoot, '.install-button').click();
   await settle(el);
  });

  it('returns to the Add to Home Screen fallback', () => {
   expect(labelOf(findButton(el.shadowRoot, '.a2hs-button'))).to.equal('Add to Home Screen');
  });
 });

 describe('when the browser reports the app as installed', () => {
  beforeEach(async () => {
   el = await mountPwaInstall({ bare: true });
   window.dispatchEvent(new Event('appinstalled'));
   await settle(el);
  });

  it('shows the Installed confirmation', () => {
   expect(shadowText(el)).to.include('Installed');
  });
 });

 describe('when the brief Installed confirmation elapses', () => {
  beforeEach(async () => {
   el = await mountPwaInstall({ bare: true, confirmationMs: 0 });
   window.dispatchEvent(new Event('appinstalled'));
   await settle(el);
  });

  it('hides itself', () => {
   expect(shadowText(el).trim()).to.equal('');
  });
 });

 describe('when no install prompt event fires in bare mode', () => {
  beforeEach(async () => {
   el = await mountPwaInstall({ bare: true });
  });

  it('renders the Add to Home Screen fallback', () => {
   expect(labelOf(findButton(el.shadowRoot, '.a2hs-button'))).to.equal('Add to Home Screen');
  });

  it('keeps the per-platform instructions hidden until toggled', () => {
   expect(shadowText(el)).to.not.include('Share');
  });

  describe('when the Add to Home Screen button is toggled', () => {
   beforeEach(async () => {
    findButton(el.shadowRoot, '.a2hs-button').click();
    await settle(el);
   });

   it('shows the iOS Safari Share instruction', () => {
    expect(shadowText(el)).to.include('Share');
   });

   it('shows the Android/desktop Chrome Install app instruction', () => {
    expect(shadowText(el)).to.include('Install app');
   });
  });
 });

 describe('when display-mode is standalone in bare mode', () => {
  beforeEach(async () => {
   el = await mountPwaInstall({ bare: true, standalone: true });
  });

  it('renders nothing because the app is already installed', () => {
   expect(shadowText(el).trim()).to.equal('');
  });
 });

 describe('when in wiki chrome mode without the bare attribute', () => {
  beforeEach(async () => {
   el = await mountPwaInstall({ bare: false });
  });

  it('renders an Open App link', () => {
   expect(findLink(el.shadowRoot, '.open-app-link')).to.exist;
  });

  it('points the Open App link at the bare URL', () => {
   expect(findLink(el.shadowRoot, '.open-app-link').getAttribute('href')).to.equal(`/${PAGE}/bare`);
  });

  it('still renders the install affordance', () => {
   expect(labelOf(findButton(el.shadowRoot, '.a2hs-button'))).to.equal('Add to Home Screen');
  });
 });
});
