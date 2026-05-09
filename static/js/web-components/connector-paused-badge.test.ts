import { expect, fixture, html } from '@open-wc/testing';
import sinon from 'sinon';
import './connector-paused-badge.js';
import type { ConnectorPausedBadge } from './connector-paused-badge.js';

function timeout(ms: number, message: string): Promise<never> {
  return new Promise<never>((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

describe('ConnectorPausedBadge', () => {
  let el: ConnectorPausedBadge;
  let clock: sinon.SinonFakeTimers;

  // Anchor "now" so date math is stable. Mid-afternoon UTC is fine.
  const now = new Date('2026-05-02T14:00:00Z');

  beforeEach(() => {
    clock = sinon.useFakeTimers(now.getTime());
  });

  afterEach(() => {
    if (el && el.parentNode) el.remove();
    clock.restore();
    sinon.restore();
  });

  // ---------------------------------------------------------- existence

  describe('when constructed', () => {
    beforeEach(async () => {
      el = (await fixture(
        html`<connector-paused-badge
          .connectorKind=${'google_keep' as const}
          .pausedAt=${new Date(now.getTime() - 30 * 60 * 1000)}
          .subscriptionTitle=${'shopping'}
        ></connector-paused-badge>`,
      )) as ConnectorPausedBadge;
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should exist', () => {
      expect(el).to.be.instanceOf(HTMLElement);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName).to.equal('CONNECTOR-PAUSED-BADGE');
    });

    it('should render a click-target button', () => {
      const btn = el.shadowRoot?.querySelector('button');
      expect(btn).to.exist;
    });

    it('should announce the paused state in copy', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Sync paused');
    });

    it('should invite the user to reconnect', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('reconnect');
    });
  });

  // ---------------------------------------------------------- urgency: muted (<1h)

  describe('when paused for less than an hour', () => {
    beforeEach(async () => {
      el = (await fixture(
        html`<connector-paused-badge
          .connectorKind=${'google_keep' as const}
          .pausedAt=${new Date(now.getTime() - 30 * 60 * 1000)}
          .subscriptionTitle=${'list'}
        ></connector-paused-badge>`,
      )) as ConnectorPausedBadge;
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should mark the host with the muted urgency tier', () => {
      expect(el.getAttribute('urgency')).to.equal('muted');
    });

    it('should render a relative time ("ago") in the copy', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('ago');
    });
  });

  // ---------------------------------------------------------- urgency: warning (1h-24h)

  describe('when paused for between 1 hour and 24 hours', () => {
    beforeEach(async () => {
      el = (await fixture(
        html`<connector-paused-badge
          .connectorKind=${'google_tasks' as const}
          .pausedAt=${new Date(now.getTime() - 2 * 60 * 60 * 1000)}
          .subscriptionTitle=${'list'}
        ></connector-paused-badge>`,
      )) as ConnectorPausedBadge;
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should mark the host with the warning urgency tier', () => {
      expect(el.getAttribute('urgency')).to.equal('warning');
    });

    it('should render a relative time in the copy', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('ago');
    });
  });

  // ---------------------------------------------------------- urgency: danger (>=24h)

  describe('when paused for 24 hours or more', () => {
    beforeEach(async () => {
      el = (await fixture(
        html`<connector-paused-badge
          .connectorKind=${'google_tasks' as const}
          .pausedAt=${new Date(now.getTime() - 48 * 60 * 60 * 1000)}
          .subscriptionTitle=${'list'}
        ></connector-paused-badge>`,
      )) as ConnectorPausedBadge;
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should mark the host with the danger urgency tier', () => {
      expect(el.getAttribute('urgency')).to.equal('danger');
    });

    it('should render an absolute time in the copy', () => {
      // Long pauses → absolute date for at-a-glance disambiguation.
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.not.include('ago');
    });
  });

  // ---------------------------------------------------------- click → request-reconnect

  describe('when the badge is clicked', () => {
    let dispatched: CustomEvent | null;

    beforeEach(async () => {
      el = (await fixture(
        html`<connector-paused-badge
          .connectorKind=${'google_tasks' as const}
          .pausedAt=${new Date(now.getTime() - 30 * 60 * 1000)}
          .subscriptionTitle=${'shopping'}
        ></connector-paused-badge>`,
      )) as ConnectorPausedBadge;
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      dispatched = null;
      el.addEventListener('request-reconnect', (e: Event) => {
        dispatched = e as CustomEvent;
      });

      const btn = el.shadowRoot?.querySelector('button') as HTMLButtonElement;
      btn.click();
    });

    it('should dispatch a request-reconnect event', () => {
      expect(dispatched).to.not.equal(null);
    });

    it('should include the connectorKind in the event detail', () => {
      const detail = (dispatched as unknown as CustomEvent).detail as { connectorKind: string };
      expect(detail.connectorKind).to.equal('google_tasks');
    });

    it('should bubble the event so ancestors can handle it', () => {
      expect((dispatched as unknown as CustomEvent).bubbles).to.equal(true);
    });

    it('should make the event composed so it crosses shadow boundaries', () => {
      expect((dispatched as unknown as CustomEvent).composed).to.equal(true);
    });

    // Production fix 2026-05-09: clicking the paused badge from a
    // checklist page (where <profile-paused-banner> is not mounted)
    // previously did nothing — the request-reconnect event bubbled
    // up unhandled. Fix: cancellable event + navigate to /profile
    // when no listener calls preventDefault.
    it('should make the event cancelable so listeners can opt out of the navigate fallback', () => {
      expect((dispatched as unknown as CustomEvent).cancelable).to.equal(true);
    });
  });

  describe('when the badge is clicked AND no listener calls preventDefault', () => {
    let navigatedTo: string | null;

    beforeEach(async () => {
      el = (await fixture(
        html`<connector-paused-badge
          .connectorKind=${'google_tasks' as const}
          .pausedAt=${new Date(now.getTime() - 30 * 60 * 1000)}
          .subscriptionTitle=${'shopping'}
        ></connector-paused-badge>`,
      )) as ConnectorPausedBadge;
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      navigatedTo = null;
      el.navigate = (url: string) => {
        navigatedTo = url;
      };

      const btn = el.shadowRoot?.querySelector('button') as HTMLButtonElement;
      btn.click();
    });

    it('should navigate to /profile (fallback for pages without profile-paused-banner)', () => {
      expect(navigatedTo).to.equal('/profile');
    });
  });

  describe('when the badge is clicked AND a listener calls preventDefault', () => {
    let navigatedTo: string | null;

    beforeEach(async () => {
      el = (await fixture(
        html`<connector-paused-badge
          .connectorKind=${'google_tasks' as const}
          .pausedAt=${new Date(now.getTime() - 30 * 60 * 1000)}
          .subscriptionTitle=${'shopping'}
        ></connector-paused-badge>`,
      )) as ConnectorPausedBadge;
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      navigatedTo = null;
      el.navigate = (url: string) => {
        navigatedTo = url;
      };
      el.addEventListener('request-reconnect', (e: Event) => {
        e.preventDefault();
      });

      const btn = el.shadowRoot?.querySelector('button') as HTMLButtonElement;
      btn.click();
    });

    it('should NOT navigate (the in-page listener handled it)', () => {
      expect(navigatedTo).to.equal(null);
    });
  });
});
