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

  describe('when dispatching page-status-changed events', () => {
    let receivedEvents: CustomEvent[];

    beforeEach(async () => {
      receivedEvents = [];

      const handler = (e: Event) => {
        receivedEvents.push(e as CustomEvent);
      };
      document.addEventListener('page-status-changed', handler);

      await fixture<PageAutoRefresh>(
        html`<page-auto-refresh page-name="my-page"></page-auto-refresh>`,
      );

      // Allow the stream to start and dispatch events
      await new Promise(resolve => setTimeout(resolve, 200));

      document.removeEventListener('page-status-changed', handler);
    });

    it('should dispatch at least one event', () => {
      expect(receivedEvents.length).to.be.greaterThan(0);
    });

    it('should dispatch event with pageName from attribute', () => {
      expect(receivedEvents[0]!.detail.pageName).to.equal('my-page');
    });
  });
});
