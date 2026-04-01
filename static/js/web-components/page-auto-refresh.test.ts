import { expect } from '@open-wc/testing';
import { fixture, html } from '@open-wc/testing-helpers';
import sinon, { type SinonStub, type SinonSpy } from 'sinon';
import { create } from '@bufbuild/protobuf';
import { TimestampSchema } from '@bufbuild/protobuf/wkt';
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

    beforeEach(() => {
      // Create without appending to DOM — prevents connectedCallback from calling startWatching
      el = document.createElement('page-auto-refresh') as PageAutoRefresh;
      el.setAttribute('page-name', 'test-page');
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
      expect(el.pageName).to.equal(undefined);
    });
  });

  describe('refreshPageContent', () => {
    describe('when fetch throws a network error', () => {
      let el: PageAutoRefresh;
      let errorEvent: CustomEvent | undefined;

      beforeEach(async () => {
        fetchStub.rejects(new Error('Network error'));

        // Stub startWatching before connecting to prevent gRPC reconnect loop from
        // racing with refreshPageContent and capturing the page-watch-error event first
        el = document.createElement('page-auto-refresh') as PageAutoRefresh;
        sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
        el.setAttribute('page-name', 'my-page');
        document.body.appendChild(el);
        await el.updateComplete;

        errorEvent = undefined;
        const errorReceived = new Promise<void>((resolve, reject) => {
          const timeout = setTimeout(() => reject(new Error('page-watch-error event timed out')), 3000);
          document.addEventListener('page-watch-error', (e: Event) => {
            clearTimeout(timeout);
            errorEvent = e as CustomEvent;
            resolve();
          }, { once: true });
        });

        // Call private method directly to test isolation from stream reconnect logic
        const refreshPromise = (el as unknown as { refreshPageContent: () => Promise<void> }).refreshPageContent();
        // refreshPageContent dispatches page-watch-error and re-throws — swallow the throw
        await Promise.all([
          refreshPromise.catch(() => { /* expected */ }),
          errorReceived,
        ]);
      });

      afterEach(() => {
        el.remove();
        sinon.restore();
      });

      it('should dispatch page-watch-error', () => {
        expect(errorEvent).to.not.equal(null);
      });

      it('should include the error in the event detail', () => {
        expect(errorEvent!.detail.error).to.be.instanceOf(Error);
      });
    });

    describe('when fetch returns non-ok response', () => {
      let el: PageAutoRefresh;
      let errorEvents: CustomEvent[];
      let didThrow: boolean;

      beforeEach(async () => {
        fetchStub.resolves(new Response('Not Found', { status: 404, statusText: 'Not Found' }));

        el = document.createElement('page-auto-refresh') as PageAutoRefresh;
        sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
        document.body.appendChild(el);
        await el.updateComplete;

        errorEvents = [];
        document.addEventListener('page-watch-error', (e: Event) => {
          errorEvents.push(e as CustomEvent);
        });

        didThrow = false;
        await (el as unknown as { refreshPageContent: () => Promise<void> })
          .refreshPageContent()
          .catch(() => { didThrow = true; });
      });

      afterEach(() => {
        el.remove();
        sinon.restore();
      });

      it('should dispatch page-watch-error event', () => {
        expect(errorEvents.length).to.equal(1);
      });

      it('should include error message in event detail', () => {
        expect((errorEvents[0]!.detail.error as Error).message).to.include('Not Found');
      });

      it('should not throw', () => {
        expect(didThrow).to.equal(false);
      });
    });

    describe('when fetch succeeds with rendered div', () => {
      let el: PageAutoRefresh;
      let renderedDiv: HTMLDivElement;
      let statusEvents: CustomEvent[];
      let scrollToSpy: SinonSpy;

      beforeEach(async () => {
        const mockHtml = '<html><body><div id="rendered"><p>Updated Content</p></div></body></html>';
        fetchStub.resolves(new Response(mockHtml, { status: 200 }));

        renderedDiv = document.createElement('div');
        renderedDiv.id = 'rendered';
        renderedDiv.innerHTML = '<p>Old Content</p>';
        document.body.appendChild(renderedDiv);

        scrollToSpy = sinon.spy(window, 'scrollTo');

        el = document.createElement('page-auto-refresh') as PageAutoRefresh;
        sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
        document.body.appendChild(el);
        await el.updateComplete;

        statusEvents = [];
        document.addEventListener('page-status-changed', (e: Event) => {
          statusEvents.push(e as CustomEvent);
        });

        await (el as unknown as { refreshPageContent: () => Promise<void> }).refreshPageContent();
      });

      afterEach(() => {
        el.remove();
        renderedDiv.remove();
        scrollToSpy.restore();
        sinon.restore();
      });

      it('should update page content', () => {
        expect(renderedDiv.innerHTML).to.include('Updated Content');
      });

      it('should restore scroll position', () => {
        expect(scrollToSpy).to.have.callCount(1);
      });

      it('should dispatch page-status-changed event', () => {
        expect(statusEvents.length).to.be.greaterThan(0);
      });
    });

    describe('when fetch succeeds but rendered div not in new content', () => {
      let el: PageAutoRefresh;
      let renderedDiv: HTMLDivElement;
      let statusEvents: CustomEvent[];

      beforeEach(async () => {
        const mockHtml = '<html><body><p>No rendered div here</p></body></html>';
        fetchStub.resolves(new Response(mockHtml, { status: 200 }));

        renderedDiv = document.createElement('div');
        renderedDiv.id = 'rendered';
        renderedDiv.innerHTML = '<p>Original Content</p>';
        document.body.appendChild(renderedDiv);

        el = document.createElement('page-auto-refresh') as PageAutoRefresh;
        sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
        document.body.appendChild(el);
        await el.updateComplete;

        statusEvents = [];
        document.addEventListener('page-status-changed', (e: Event) => {
          statusEvents.push(e as CustomEvent);
        });

        await (el as unknown as { refreshPageContent: () => Promise<void> }).refreshPageContent();
      });

      afterEach(() => {
        el.remove();
        renderedDiv.remove();
        sinon.restore();
      });

      it('should not update page content', () => {
        expect(renderedDiv.innerHTML).to.include('Original Content');
      });

      it('should not dispatch page-status-changed', () => {
        expect(statusEvents.length).to.equal(0);
      });
    });
  });

  describe('when dispatching page-status-changed events', () => {
    let receivedEvents: CustomEvent[];
    let el: PageAutoRefresh;

    beforeEach(async () => {
      receivedEvents = [];

      el = document.createElement('page-auto-refresh') as PageAutoRefresh;
      const privateEl = el as unknown as {
        isWatching: boolean;
        dispatchPageStatusEvent: () => void;
        startWatching: () => void;
      };

      // Stub startWatching before connecting to DOM to prevent real gRPC calls.
      // The fake simulates the initial status dispatch that startWatching performs.
      sinon.stub(privateEl, 'startWatching')
        .callsFake(() => {
          privateEl.isWatching = true;
          privateEl.dispatchPageStatusEvent();
        });

      const firstEventReceived = new Promise<void>((resolve, reject) => {
        const timeout = setTimeout(() => reject(new Error('page-status-changed event timed out')), 3000);
        const handler = (e: Event) => {
          clearTimeout(timeout);
          receivedEvents.push(e as CustomEvent);
          document.removeEventListener('page-status-changed', handler);
          resolve();
        };
        document.addEventListener('page-status-changed', handler);
      });

      // Append first (without pageName), then set pageName to trigger the updated() lifecycle path
      document.body.appendChild(el);
      await el.updateComplete;

      el.pageName = 'my-page';
      await el.updateComplete;

      await firstEventReceived;
    });

    afterEach(() => {
      el.remove();
      sinon.restore();
    });

    it('should dispatch at least one event', () => {
      expect(receivedEvents.length).to.be.greaterThan(0);
    });

    it('should dispatch event with pageName from attribute', () => {
      expect(receivedEvents[0]!.detail.pageName).to.equal('my-page');
    });

    it('should dispatch event with isWatching true', () => {
      expect(receivedEvents[0]!.detail.isWatching).to.equal(true);
    });
  });

  describe('connectedCallback', () => {
    describe('when element is connected to DOM', () => {
      let el: PageAutoRefresh;
      let addEventListenerSpy: SinonSpy;

      beforeEach(async () => {
        el = document.createElement('page-auto-refresh') as PageAutoRefresh;
        sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
        addEventListenerSpy = sinon.spy(document, 'addEventListener');

        document.body.appendChild(el);
        await el.updateComplete;
      });

      afterEach(() => {
        el.remove();
        addEventListenerSpy.restore();
        sinon.restore();
      });

      it('should add visibilitychange event listener', () => {
        expect(addEventListenerSpy).to.have.been.calledWith(
          'visibilitychange',
          (el as unknown as { _handleVisibilityChange: () => void })._handleVisibilityChange,
        );
      });
    });
  });

  describe('disconnectedCallback', () => {
    describe('when element is disconnected from DOM', () => {
      let el: PageAutoRefresh;
      let removeEventListenerSpy: SinonSpy;

      beforeEach(async () => {
        el = document.createElement('page-auto-refresh') as PageAutoRefresh;
        sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
        document.body.appendChild(el);
        await el.updateComplete;

        removeEventListenerSpy = sinon.spy(document, 'removeEventListener');
        el.remove();
      });

      afterEach(() => {
        removeEventListenerSpy.restore();
        sinon.restore();
      });

      it('should remove visibilitychange event listener', () => {
        expect(removeEventListenerSpy).to.have.been.calledWith(
          'visibilitychange',
          (el as unknown as { _handleVisibilityChange: () => void })._handleVisibilityChange,
        );
      });
    });
  });

  describe('when pageName property is updated', () => {
    let el: PageAutoRefresh;
    let startWatchingStub: SinonStub;
    let stopWatchingStub: SinonStub;

    beforeEach(async () => {
      el = document.createElement('page-auto-refresh') as PageAutoRefresh;
      startWatchingStub = sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
      stopWatchingStub = sinon.stub(el as unknown as { stopWatching: () => void }, 'stopWatching');

      document.body.appendChild(el);
      await el.updateComplete;

      startWatchingStub.resetHistory();
      stopWatchingStub.resetHistory();

      el.pageName = 'new-page';
      await el.updateComplete;
    });

    afterEach(() => {
      el.remove();
      sinon.restore();
    });

    it('should call stopWatching', () => {
      expect(stopWatchingStub).to.have.callCount(1);
    });

    it('should call startWatching', () => {
      expect(startWatchingStub).to.have.callCount(1);
    });

    describe('when pageName is subsequently cleared', () => {
      beforeEach(async () => {
        startWatchingStub.resetHistory();
        stopWatchingStub.resetHistory();

        el.pageName = undefined as unknown as string;
        await el.updateComplete;
      });

      it('should call stopWatching', () => {
        expect(stopWatchingStub).to.have.callCount(1);
      });

      it('should not call startWatching', () => {
        expect(startWatchingStub).to.have.callCount(0);
      });
    });
  });

  describe('handleVisibilityChange', () => {
    let el: PageAutoRefresh;
    let startWatchingStub: SinonStub;

    beforeEach(async () => {
      el = document.createElement('page-auto-refresh') as PageAutoRefresh;
      startWatchingStub = sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
      document.body.appendChild(el);
      await el.updateComplete;

      el.pageName = 'test-page';
      await el.updateComplete;

      startWatchingStub.resetHistory();
    });

    afterEach(() => {
      el.remove();
      sinon.restore();
    });

    describe('when tab becomes visible and not currently watching', () => {
      beforeEach(() => {
        (el as unknown as { isWatching: boolean }).isWatching = false;
        // In the Chromium test runner, document.visibilityState is 'visible'
        (el as unknown as { handleVisibilityChange: () => void }).handleVisibilityChange();
      });

      it('should call startWatching', () => {
        expect(startWatchingStub).to.have.callCount(1);
      });
    });

    describe('when tab is visible but already watching', () => {
      beforeEach(() => {
        (el as unknown as { isWatching: boolean }).isWatching = true;
        (el as unknown as { handleVisibilityChange: () => void }).handleVisibilityChange();
      });

      it('should not call startWatching', () => {
        expect(startWatchingStub).to.have.callCount(0);
      });
    });
  });

  describe('stopWatching', () => {
    describe('when subscription exists', () => {
      let el: PageAutoRefresh;
      let statusEvents: CustomEvent[];
      let controller: AbortController;

      beforeEach(async () => {
        el = document.createElement('page-auto-refresh') as PageAutoRefresh;
        sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
        document.body.appendChild(el);
        await el.updateComplete;

        controller = new AbortController();
        (el as unknown as { streamSubscription: AbortController }).streamSubscription = controller;
        (el as unknown as { isWatching: boolean }).isWatching = true;

        statusEvents = [];
        document.addEventListener('page-status-changed', (e: Event) => {
          statusEvents.push(e as CustomEvent);
        });

        (el as unknown as { stopWatching: () => void }).stopWatching();
      });

      afterEach(() => {
        el.remove();
        sinon.restore();
      });

      it('should abort the stream subscription', () => {
        expect(controller.signal.aborted).to.equal(true);
      });

      it('should clear streamSubscription', () => {
        expect((el as unknown as { streamSubscription: AbortController | undefined }).streamSubscription).to.equal(undefined);
      });

      it('should set isWatching to false', () => {
        expect((el as unknown as { isWatching: boolean }).isWatching).to.equal(false);
      });

      it('should dispatch page-status-changed event', () => {
        expect(statusEvents.length).to.be.greaterThan(0);
      });
    });

    describe('when no subscription exists', () => {
      let el: PageAutoRefresh;
      let statusEvents: CustomEvent[];

      beforeEach(async () => {
        el = document.createElement('page-auto-refresh') as PageAutoRefresh;
        sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
        document.body.appendChild(el);
        await el.updateComplete;

        statusEvents = [];
        document.addEventListener('page-status-changed', (e: Event) => {
          statusEvents.push(e as CustomEvent);
        });

        (el as unknown as { stopWatching: () => void }).stopWatching();
      });

      afterEach(() => {
        el.remove();
        sinon.restore();
      });

      it('should not dispatch page-status-changed event', () => {
        expect(statusEvents.length).to.equal(0);
      });
    });
  });

  describe('_dispatchWatchError', () => {
    let el: PageAutoRefresh;
    let errorEvents: CustomEvent[];
    let testError: Error;

    beforeEach(async () => {
      el = document.createElement('page-auto-refresh') as PageAutoRefresh;
      sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
      document.body.appendChild(el);
      await el.updateComplete;

      testError = new Error('Stream failed');
      errorEvents = [];
      document.addEventListener('page-watch-error', (e: Event) => {
        errorEvents.push(e as CustomEvent);
      });

      (el as unknown as { _dispatchWatchError: (err: unknown) => void })._dispatchWatchError(testError);
    });

    afterEach(() => {
      el.remove();
      sinon.restore();
    });

    it('should dispatch page-watch-error event', () => {
      expect(errorEvents.length).to.equal(1);
    });

    it('should include the error in event detail', () => {
      expect(errorEvents[0]!.detail.error).to.equal(testError);
    });
  });

  describe('_handleWatchResponse', () => {
    let el: PageAutoRefresh;

    beforeEach(async () => {
      el = document.createElement('page-auto-refresh') as PageAutoRefresh;
      sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
      document.body.appendChild(el);
      await el.updateComplete;
    });

    afterEach(() => {
      el.remove();
      sinon.restore();
    });

    describe('when response has lastModified', () => {
      beforeEach(async () => {
        const mockTimestamp = create(TimestampSchema, {
          seconds: BigInt(Math.floor(new Date('2024-01-01T12:00:00Z').getTime() / 1000)),
          nanos: 0,
        });

        await (el as unknown as { _handleWatchResponse: (r: unknown) => Promise<void> })
          ._handleWatchResponse({ versionHash: 'abc', lastModified: mockTimestamp });
      });

      it('should update lastRefreshTime', () => {
        expect((el as unknown as { lastRefreshTime: Date | undefined }).lastRefreshTime).to.be.instanceOf(Date);
      });
    });

    describe('when response has no lastModified', () => {
      beforeEach(async () => {
        await (el as unknown as { _handleWatchResponse: (r: unknown) => Promise<void> })
          ._handleWatchResponse({ versionHash: 'abc' });
      });

      it('should not set lastRefreshTime', () => {
        expect((el as unknown as { lastRefreshTime: Date | undefined }).lastRefreshTime).to.equal(undefined);
      });
    });

    describe('when currentHash is not set (first response)', () => {
      let handleFirstResponseStub: SinonStub;

      beforeEach(async () => {
        handleFirstResponseStub = sinon.stub(
          el as unknown as { _handleFirstResponse: (r: unknown) => void },
          '_handleFirstResponse',
        );

        await (el as unknown as { _handleWatchResponse: (r: unknown) => Promise<void> })
          ._handleWatchResponse({ versionHash: 'abc' });
      });

      it('should call _handleFirstResponse', () => {
        expect(handleFirstResponseStub).to.have.callCount(1);
      });
    });

    describe('when hash matches current hash', () => {
      let handleHashChangedStub: SinonStub;

      beforeEach(async () => {
        (el as unknown as { currentHash: string }).currentHash = 'same-hash';
        handleHashChangedStub = sinon.stub(
          el as unknown as { _handleHashChanged: (r: unknown) => Promise<void> },
          '_handleHashChanged',
        ).resolves();

        await (el as unknown as { _handleWatchResponse: (r: unknown) => Promise<void> })
          ._handleWatchResponse({ versionHash: 'same-hash' });
      });

      it('should not call _handleHashChanged', () => {
        expect(handleHashChangedStub).to.have.callCount(0);
      });
    });

    describe('when hash has changed', () => {
      let handleHashChangedStub: SinonStub;

      beforeEach(async () => {
        (el as unknown as { currentHash: string }).currentHash = 'old-hash';
        handleHashChangedStub = sinon.stub(
          el as unknown as { _handleHashChanged: (r: unknown) => Promise<void> },
          '_handleHashChanged',
        ).resolves();

        await (el as unknown as { _handleWatchResponse: (r: unknown) => Promise<void> })
          ._handleWatchResponse({ versionHash: 'new-hash' });
      });

      it('should call _handleHashChanged', () => {
        expect(handleHashChangedStub).to.have.callCount(1);
      });
    });
  });

  describe('_handleFirstResponse', () => {
    let el: PageAutoRefresh;
    let statusEvents: CustomEvent[];

    beforeEach(async () => {
      el = document.createElement('page-auto-refresh') as PageAutoRefresh;
      sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
      document.body.appendChild(el);
      await el.updateComplete;

      statusEvents = [];
      document.addEventListener('page-status-changed', (e: Event) => {
        statusEvents.push(e as CustomEvent);
      });

      (el as unknown as { _handleFirstResponse: (r: unknown) => void })
        ._handleFirstResponse({ versionHash: 'initial-hash' });
    });

    afterEach(() => {
      el.remove();
      sinon.restore();
    });

    it('should set currentHash', () => {
      expect((el as unknown as { currentHash: string }).currentHash).to.equal('initial-hash');
    });

    it('should set dataset versionHash', () => {
      expect(el.dataset['versionHash']).to.equal('initial-hash');
    });

    it('should dispatch page-status-changed event', () => {
      expect(statusEvents.length).to.be.greaterThan(0);
    });
  });

  describe('_handleHashChanged', () => {
    let el: PageAutoRefresh;

    beforeEach(async () => {
      el = document.createElement('page-auto-refresh') as PageAutoRefresh;
      sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
      document.body.appendChild(el);
      await el.updateComplete;

      (el as unknown as { currentHash: string }).currentHash = 'old-hash';
    });

    afterEach(() => {
      el.remove();
      sinon.restore();
    });

    describe('when refreshPageContent succeeds', () => {
      beforeEach(async () => {
        sinon.stub(
          el as unknown as { refreshPageContent: () => Promise<void> },
          'refreshPageContent',
        ).resolves();

        await (el as unknown as { _handleHashChanged: (r: unknown) => Promise<void> })
          ._handleHashChanged({ versionHash: 'new-hash' });
      });

      it('should update currentHash to new hash', () => {
        expect((el as unknown as { currentHash: string }).currentHash).to.equal('new-hash');
      });
    });

    describe('when refreshPageContent throws', () => {
      beforeEach(async () => {
        sinon.stub(
          el as unknown as { refreshPageContent: () => Promise<void> },
          'refreshPageContent',
        ).rejects(new Error('Fetch failed'));

        await (el as unknown as { _handleHashChanged: (r: unknown) => Promise<void> })
          ._handleHashChanged({ versionHash: 'new-hash' });
      });

      it('should not update currentHash', () => {
        expect((el as unknown as { currentHash: string }).currentHash).to.equal('old-hash');
      });
    });
  });

  describe('_waitForReconnect', () => {
    let el: PageAutoRefresh;

    beforeEach(async () => {
      el = document.createElement('page-auto-refresh') as PageAutoRefresh;
      sinon.stub(el as unknown as { startWatching: () => void }, 'startWatching');
      document.body.appendChild(el);
      await el.updateComplete;
    });

    afterEach(() => {
      el.remove();
      sinon.restore();
    });

    describe('when waiting for delay', () => {
      let resolved: boolean;

      beforeEach(async () => {
        resolved = false;
        const controller = new AbortController();
        // Use a very short real delay (10ms) to avoid fake timers which interfere
        // with web-test-runner's WebSocket keepalive mechanism and cause browser disconnects.
        await (el as unknown as { _waitForReconnect: (s: AbortSignal, ms: number) => Promise<void> })
          ._waitForReconnect(controller.signal, 10)
          .then(() => { resolved = true; });
      });

      it('should resolve after the delay', () => {
        expect(resolved).to.equal(true);
      });
    });

    describe('when aborted before delay expires', () => {
      let resolved: boolean;

      beforeEach(async () => {
        resolved = false;
        const controller = new AbortController();
        const promise = (el as unknown as { _waitForReconnect: (s: AbortSignal, ms: number) => Promise<void> })
          ._waitForReconnect(controller.signal, 60000)
          .then(() => { resolved = true; });

        // Abort immediately — the promise should resolve without waiting for the timeout
        controller.abort();
        await promise;
      });

      it('should resolve early when aborted', () => {
        expect(resolved).to.equal(true);
      });
    });
  });
});
