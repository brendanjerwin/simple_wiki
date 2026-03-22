import { expect } from '@open-wc/testing';
import { fixture, html } from '@open-wc/testing-helpers';
import sinon, { type SinonStub } from 'sinon';
import { PageAutoRefresh } from './page-auto-refresh.js';

describe('PageAutoRefresh', () => {
  let fetchStub: SinonStub;

  beforeEach(() => {
    fetchStub = sinon.stub(window, 'fetch');
    fetchStub.resolves(new Response('{}'));
  });

  afterEach(() => {
    fetchStub.restore();
  });

  it('should exist', async () => {
    const el = await fixture<PageAutoRefresh>(
      html`<page-auto-refresh></page-auto-refresh>`,
    );
    expect(el).to.be.instanceOf(PageAutoRefresh);
  });

  describe('page-name attribute mapping', () => {
    let el: PageAutoRefresh;

    beforeEach(async () => {
      el = await fixture<PageAutoRefresh>(
        html`<page-auto-refresh page-name="test-page"></page-auto-refresh>`,
      );
    });

    it('should map page-name attribute to pageName property', () => {
      expect(el.pageName).to.equal('test-page');
    });
  });

  describe('when page-name attribute is not set', () => {
    let el: PageAutoRefresh;

    beforeEach(async () => {
      el = await fixture<PageAutoRefresh>(
        html`<page-auto-refresh></page-auto-refresh>`,
      );
    });

    it('should not have pageName set', () => {
      expect(el.pageName).to.be.undefined;
    });
  });

  describe('refreshPageContent', () => {
    describe('when fetch throws a network error', () => {
      let el: PageAutoRefresh;
      let errorEvent: CustomEvent | undefined;

      beforeEach(async () => {
        fetchStub.rejects(new Error('Network error'));

        el = await fixture<PageAutoRefresh>(
          html`<page-auto-refresh page-name="my-page"></page-auto-refresh>`,
        );

        errorEvent = undefined;
        const errorReceived = new Promise<void>(resolve => {
          document.addEventListener('page-watch-error', (e: Event) => {
            errorEvent = e as CustomEvent;
            resolve();
          }, { once: true });
        });

        // Call private method directly to test isolation from stream reconnect logic
        const refreshPromise = (el as unknown as Record<string, () => Promise<void>>)['refreshPageContent']();
        // refreshPageContent dispatches page-watch-error and re-throws — swallow the throw
        await Promise.all([
          refreshPromise.catch(() => { /* expected */ }),
          errorReceived,
        ]);
      });

      afterEach(() => {
        el.remove();
      });

      it('should dispatch page-watch-error', () => {
        expect(errorEvent).to.exist;
      });

      it('should include the error in the event detail', () => {
        expect(errorEvent!.detail.error).to.be.instanceOf(Error);
      });
    });
  });

  describe('when dispatching page-status-changed events', () => {
    let receivedEvents: CustomEvent[];
    let el: PageAutoRefresh;

    beforeEach(async () => {
      receivedEvents = [];

      // Register listener before fixture so we catch synchronous dispatch during connectedCallback
      const firstEventReceived = new Promise<void>(resolve => {
        const handler = (e: Event) => {
          receivedEvents.push(e as CustomEvent);
          document.removeEventListener('page-status-changed', handler);
          resolve();
        };
        document.addEventListener('page-status-changed', handler);
      });

      el = await fixture<PageAutoRefresh>(
        html`<page-auto-refresh page-name="my-page"></page-auto-refresh>`,
      );

      // Wait for the first status event (dispatched synchronously during connectedCallback)
      await firstEventReceived;
    });

    afterEach(() => {
      // Ensure any additional event listeners are cleaned up when el disconnects
      el.remove();
    });

    it('should dispatch at least one event', () => {
      expect(receivedEvents.length).to.be.greaterThan(0);
    });

    it('should dispatch event with pageName from attribute', () => {
      expect(receivedEvents[0]!.detail.pageName).to.equal('my-page');
    });

    it('should dispatch event with isWatching true', () => {
      expect(receivedEvents[0]!.detail.isWatching).to.be.true;
    });
  });
});
