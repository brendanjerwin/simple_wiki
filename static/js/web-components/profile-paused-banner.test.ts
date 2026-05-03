import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import { create } from '@bufbuild/protobuf';
import './profile-paused-banner.js';
import type { ProfilePausedBanner } from './profile-paused-banner.js';
import {
  ConnectorKind,
  ConnectorStateSchema,
  GetStateResponseSchema,
  SubscriptionStateSchema,
} from '../gen/api/v1/connector_service_pb.js';

interface BannerClient {
  getState: sinon.SinonStub;
}

function clientOf(el: ProfilePausedBanner): BannerClient {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion, @typescript-eslint/no-explicit-any
  return (el as any).client as BannerClient;
}

function timeout(ms: number, message: string): Promise<never> {
  return new Promise<never>((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

// pausedSub builds a minimal SubscriptionState marked paused, suitable
// for stubbing GetState responses. Most fields aren't load-bearing for
// banner-rendering tests; we keep the shape minimal so the test reads
// as "this connector has a paused sub" rather than "this connector has
// a sub with twenty fields, one of which happens to be paused=true".
function pausedSub(
  kind: ConnectorKind,
  page = 'shopping_lists.this_week',
  listName = 'shopping',
) {
  return create(SubscriptionStateSchema, {
    page,
    listName,
    paused: true,
    connectorKind: kind,
    remoteListTitle: 'Shopping',
  });
}

function activeSub(
  kind: ConnectorKind,
  page = 'shopping_lists.this_week',
  listName = 'shopping',
) {
  return create(SubscriptionStateSchema, {
    page,
    listName,
    paused: false,
    connectorKind: kind,
    remoteListTitle: 'Shopping',
  });
}

function stateResponse(
  kind: ConnectorKind,
  subscriptions: ReturnType<typeof create>[],
) {
  return create(GetStateResponseSchema, {
    state: create(ConnectorStateSchema, {
      configured: true,
      email: 'user@example.com',
      connectorKind: kind,
      // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-explicit-any
      subscriptions: subscriptions as any,
    }),
  });
}

// stubBoth returns an object exposing both stubs so tests can adjust
// per-kind responses independently of each other.
function stubBoth(
  el: ProfilePausedBanner,
  keepResponse: ReturnType<typeof create>,
  tasksResponse: ReturnType<typeof create>,
): sinon.SinonStub {
  const client = clientOf(el);
  const stub = sinon.stub(client, 'getState');
  stub
    .withArgs(sinon.match((req: { connectorKind: ConnectorKind }) => req.connectorKind === ConnectorKind.GOOGLE_KEEP))
    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-explicit-any
    .resolves(keepResponse as any);
  stub
    .withArgs(sinon.match((req: { connectorKind: ConnectorKind }) => req.connectorKind === ConnectorKind.GOOGLE_TASKS))
    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-explicit-any
    .resolves(tasksResponse as any);
  return stub;
}

describe('ProfilePausedBanner', () => {
  let el: ProfilePausedBanner;

  afterEach(() => {
    if (el && el.parentNode) el.remove();
    sinon.restore();
  });

  // ------------------------------------------------------------------ existence

  describe('when constructed and both connectors return active subscriptions', () => {
    beforeEach(async () => {
      el = document.createElement('profile-paused-banner') as ProfilePausedBanner;
      stubBoth(
        el,
        stateResponse(ConnectorKind.GOOGLE_KEEP, [activeSub(ConnectorKind.GOOGLE_KEEP)]),
        stateResponse(ConnectorKind.GOOGLE_TASKS, [activeSub(ConnectorKind.GOOGLE_TASKS)]),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      // One more flush so the post-fetch render lands.
      await el.updateComplete;
    });

    it('should exist', () => {
      expect(el).to.be.instanceOf(HTMLElement);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName).to.equal('PROFILE-PAUSED-BANNER');
    });

    it('should render no banners', () => {
      const banners = el.shadowRoot?.querySelectorAll('.banner');
      expect(banners?.length ?? 0).to.equal(0);
    });
  });

  // ------------------------------------------------------------------ keep paused

  describe('when only the Keep connector has paused subscriptions', () => {
    beforeEach(async () => {
      el = document.createElement('profile-paused-banner') as ProfilePausedBanner;
      stubBoth(
        el,
        stateResponse(ConnectorKind.GOOGLE_KEEP, [pausedSub(ConnectorKind.GOOGLE_KEEP)]),
        stateResponse(ConnectorKind.GOOGLE_TASKS, [activeSub(ConnectorKind.GOOGLE_TASKS)]),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await el.updateComplete;
    });

    it('should render exactly one banner', () => {
      const banners = el.shadowRoot?.querySelectorAll('.banner');
      expect(banners?.length).to.equal(1);
    });

    it('should render the Google Keep banner', () => {
      const banner = el.shadowRoot?.querySelector('.banner');
      expect(banner?.getAttribute('data-connector-kind')).to.equal('google_keep');
    });

    it('should mention Google Keep in the copy', () => {
      const banner = el.shadowRoot?.querySelector('.banner');
      expect(banner?.textContent ?? '').to.include('Google Keep');
    });

    it('should expose a Reconnect button', () => {
      const btn = el.shadowRoot?.querySelector('button.reconnect');
      expect(btn).to.exist;
    });
  });

  // ------------------------------------------------------------------ tasks paused

  describe('when only the Tasks connector has paused subscriptions', () => {
    beforeEach(async () => {
      el = document.createElement('profile-paused-banner') as ProfilePausedBanner;
      stubBoth(
        el,
        stateResponse(ConnectorKind.GOOGLE_KEEP, [activeSub(ConnectorKind.GOOGLE_KEEP)]),
        stateResponse(ConnectorKind.GOOGLE_TASKS, [pausedSub(ConnectorKind.GOOGLE_TASKS)]),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await el.updateComplete;
    });

    it('should render exactly one banner', () => {
      const banners = el.shadowRoot?.querySelectorAll('.banner');
      expect(banners?.length).to.equal(1);
    });

    it('should render the Google Tasks banner', () => {
      const banner = el.shadowRoot?.querySelector('.banner');
      expect(banner?.getAttribute('data-connector-kind')).to.equal('google_tasks');
    });

    it('should mention Google Tasks in the copy', () => {
      const banner = el.shadowRoot?.querySelector('.banner');
      expect(banner?.textContent ?? '').to.include('Google Tasks');
    });
  });

  // ------------------------------------------------------------------ both paused → stacked

  describe('when both connectors have paused subscriptions', () => {
    beforeEach(async () => {
      el = document.createElement('profile-paused-banner') as ProfilePausedBanner;
      stubBoth(
        el,
        stateResponse(ConnectorKind.GOOGLE_KEEP, [pausedSub(ConnectorKind.GOOGLE_KEEP)]),
        stateResponse(ConnectorKind.GOOGLE_TASKS, [pausedSub(ConnectorKind.GOOGLE_TASKS)]),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await el.updateComplete;
    });

    it('should render two banners stacked', () => {
      const banners = el.shadowRoot?.querySelectorAll('.banner');
      expect(banners?.length).to.equal(2);
    });

    it('should render one banner per connector kind', () => {
      const slugs = Array.from(el.shadowRoot?.querySelectorAll('.banner') ?? [])
        .map((b) => b.getAttribute('data-connector-kind'))
        .sort();
      expect(slugs).to.deep.equal(['google_keep', 'google_tasks']);
    });
  });

  // ------------------------------------------------------------------ multiple paused subs in one kind → single banner

  describe('when one connector has multiple paused subscriptions', () => {
    beforeEach(async () => {
      el = document.createElement('profile-paused-banner') as ProfilePausedBanner;
      stubBoth(
        el,
        stateResponse(ConnectorKind.GOOGLE_KEEP, [
          pausedSub(ConnectorKind.GOOGLE_KEEP, 'shopping', 'a'),
          pausedSub(ConnectorKind.GOOGLE_KEEP, 'shopping', 'b'),
        ]),
        stateResponse(ConnectorKind.GOOGLE_TASKS, []),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await el.updateComplete;
    });

    it('should render only one banner for that connector', () => {
      const banners = el.shadowRoot?.querySelectorAll('.banner');
      expect(banners?.length).to.equal(1);
    });
  });

  // ------------------------------------------------------------------ unconfigured connector

  describe('when one connector returns no subscriptions at all', () => {
    beforeEach(async () => {
      el = document.createElement('profile-paused-banner') as ProfilePausedBanner;
      stubBoth(
        el,
        stateResponse(ConnectorKind.GOOGLE_KEEP, []),
        stateResponse(ConnectorKind.GOOGLE_TASKS, [pausedSub(ConnectorKind.GOOGLE_TASKS)]),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await el.updateComplete;
    });

    it('should render only the connector that is paused', () => {
      const banners = el.shadowRoot?.querySelectorAll('.banner');
      expect(banners?.length).to.equal(1);
      expect(banners?.[0]?.getAttribute('data-connector-kind')).to.equal('google_tasks');
    });
  });

  // ------------------------------------------------------------------ partial GetState failure

  describe('when one connector RPC rejects but the other succeeds with paused subs', () => {
    beforeEach(async () => {
      el = document.createElement('profile-paused-banner') as ProfilePausedBanner;
      const client = clientOf(el);
      const stub = sinon.stub(client, 'getState');
      stub
        .withArgs(
          sinon.match(
            (req: { connectorKind: ConnectorKind }) => req.connectorKind === ConnectorKind.GOOGLE_KEEP,
          ),
        )
        .rejects(new Error('keep transport failure'));
      stub
        .withArgs(
          sinon.match(
            (req: { connectorKind: ConnectorKind }) => req.connectorKind === ConnectorKind.GOOGLE_TASKS,
          ),
        )
        // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-explicit-any
        .resolves(stateResponse(ConnectorKind.GOOGLE_TASKS, [pausedSub(ConnectorKind.GOOGLE_TASKS)]) as any);
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await el.updateComplete;
    });

    it('should still render the Tasks banner', () => {
      const banners = el.shadowRoot?.querySelectorAll('.banner');
      expect(banners?.length).to.equal(1);
      expect(banners?.[0]?.getAttribute('data-connector-kind')).to.equal('google_tasks');
    });
  });

  // ------------------------------------------------------------------ banner click → scroll-and-focus when target exists

  describe('when the Reconnect button is clicked and a connect component is on the page', () => {
    let scrollSpy: sinon.SinonStub;
    let focusSpy: sinon.SinonStub;
    let navigateSpy: sinon.SinonSpy;
    let connectStub: HTMLElement;

    beforeEach(async () => {
      // Stub scrollIntoView and focus on the prototype before mounting
      // — the real <google-tasks-connect> custom element fires its own
      // RPC on connectedCallback which we don't care about for routing
      // tests. We pre-stub its client too so the unhandled rejection
      // doesn't surface.
      scrollSpy = sinon.stub(HTMLElement.prototype, 'scrollIntoView');
      focusSpy = sinon.stub(HTMLElement.prototype, 'focus');

      // Stub global fetch first so the real <google-tasks-connect>'s
      // connectedCallback RPC can't reach the network (it'd hang or
      // throw). We don't care about its lifecycle for routing tests.
      sinon.stub(window, 'fetch').rejects(new Error('test: no network'));
      connectStub = document.createElement('google-tasks-connect');
      document.body.appendChild(connectStub);

      el = document.createElement('profile-paused-banner') as ProfilePausedBanner;
      navigateSpy = sinon.spy();
      el.navigate = navigateSpy;
      stubBoth(
        el,
        stateResponse(ConnectorKind.GOOGLE_KEEP, []),
        stateResponse(ConnectorKind.GOOGLE_TASKS, [pausedSub(ConnectorKind.GOOGLE_TASKS)]),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await el.updateComplete;

      const btn = el.shadowRoot?.querySelector('button.reconnect') as HTMLButtonElement;
      btn.click();
    });

    afterEach(() => {
      connectStub.remove();
    });

    it('should scroll the connect component into view', () => {
      expect(scrollSpy.called).to.equal(true);
    });

    it('should focus the connect component', () => {
      expect(focusSpy.called).to.equal(true);
    });

    it('should not navigate away', () => {
      expect(navigateSpy.called).to.equal(false);
    });
  });

  // ------------------------------------------------------------------ banner click → navigate when no connect component

  describe('when the Reconnect button is clicked and no connect component is on the page', () => {
    let navigateSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el = document.createElement('profile-paused-banner') as ProfilePausedBanner;
      navigateSpy = sinon.spy();
      el.navigate = navigateSpy;
      stubBoth(
        el,
        stateResponse(ConnectorKind.GOOGLE_KEEP, []),
        stateResponse(ConnectorKind.GOOGLE_TASKS, [pausedSub(ConnectorKind.GOOGLE_TASKS)]),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await el.updateComplete;

      const btn = el.shadowRoot?.querySelector('button.reconnect') as HTMLButtonElement;
      btn.click();
    });

    it('should navigate to /profile', () => {
      expect(navigateSpy.calledWith('/profile')).to.equal(true);
    });
  });

  // ------------------------------------------------------------------ global request-reconnect listener: tasks slug

  describe('when a request-reconnect event fires from elsewhere on the page with the google_tasks slug and no connect component is on the page', () => {
    let navigateSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el = document.createElement('profile-paused-banner') as ProfilePausedBanner;
      navigateSpy = sinon.spy();
      el.navigate = navigateSpy;
      stubBoth(
        el,
        stateResponse(ConnectorKind.GOOGLE_KEEP, []),
        stateResponse(ConnectorKind.GOOGLE_TASKS, []),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await el.updateComplete;

      // Dispatch the event from the document — simulates a badge on
      // some other page bubbling up.
      document.dispatchEvent(
        new CustomEvent('request-reconnect', {
          detail: { connectorKind: 'google_tasks' },
          bubbles: true,
          composed: true,
        }),
      );
    });

    it('should navigate to /profile', () => {
      expect(navigateSpy.calledWith('/profile')).to.equal(true);
    });
  });

  describe('when a request-reconnect event fires with the google_keep slug and no connect component is on the page', () => {
    let navigateSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el = document.createElement('profile-paused-banner') as ProfilePausedBanner;
      navigateSpy = sinon.spy();
      el.navigate = navigateSpy;
      stubBoth(
        el,
        stateResponse(ConnectorKind.GOOGLE_KEEP, []),
        stateResponse(ConnectorKind.GOOGLE_TASKS, []),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await el.updateComplete;

      document.dispatchEvent(
        new CustomEvent('request-reconnect', {
          detail: { connectorKind: 'google_keep' },
          bubbles: true,
          composed: true,
        }),
      );
    });

    it('should navigate to /profile', () => {
      expect(navigateSpy.calledWith('/profile')).to.equal(true);
    });
  });

  describe('when a request-reconnect event fires with an unrecognised slug', () => {
    let navigateSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el = document.createElement('profile-paused-banner') as ProfilePausedBanner;
      navigateSpy = sinon.spy();
      el.navigate = navigateSpy;
      stubBoth(
        el,
        stateResponse(ConnectorKind.GOOGLE_KEEP, []),
        stateResponse(ConnectorKind.GOOGLE_TASKS, []),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await el.updateComplete;

      document.dispatchEvent(
        new CustomEvent('request-reconnect', {
          detail: { connectorKind: 'icloud_reminders_someday' },
          bubbles: true,
          composed: true,
        }),
      );
    });

    it('should not navigate', () => {
      expect(navigateSpy.called).to.equal(false);
    });
  });

  // ------------------------------------------------------------------ listener wiring

  describe('when the component is removed from the DOM', () => {
    let removeSpy: sinon.SinonSpy;

    beforeEach(async () => {
      removeSpy = sinon.spy(document, 'removeEventListener');
      el = document.createElement('profile-paused-banner') as ProfilePausedBanner;
      stubBoth(
        el,
        stateResponse(ConnectorKind.GOOGLE_KEEP, []),
        stateResponse(ConnectorKind.GOOGLE_TASKS, []),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await el.updateComplete;
      el.remove();
    });

    it('should remove its request-reconnect listener', () => {
      expect(removeSpy.calledWith('request-reconnect')).to.equal(true);
    });
  });
});
