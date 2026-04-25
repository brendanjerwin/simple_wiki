import { html, fixture, expect, waitUntil } from '@open-wc/testing';
import sinon from 'sinon';
import { displayTitle } from './wiki-hashtag.js';
import type { SearchResult } from '../gen/api/v1/search_pb.js';

interface WikiHashtagElement extends HTMLElement {
  tag: string;
  performSearch: (query: string) => Promise<SearchResult[]>;
  updateComplete: Promise<boolean>;
  shadowRoot: ShadowRoot;
}

describe('displayTitle', () => {
  it('returns the frontmatter title when one is set', () => {
    expect(displayTitle({ identifier: 'p', title: 'Real Title' })).to.equal('Real Title');
  });

  it('humanizes the identifier when title is empty', () => {
    expect(displayTitle({ identifier: 'home_lab', title: '' })).to.equal('Home Lab');
  });

  it('humanizes the identifier when title equals identifier', () => {
    expect(displayTitle({ identifier: 'home_lab', title: 'home_lab' })).to.equal('Home Lab');
  });

  it('handles hyphens, mixed separators, and capitalization', () => {
    expect(displayTitle({ identifier: 'engineering-blog', title: '' })).to.equal('Engineering Blog');
    expect(displayTitle({ identifier: 'foo_bar-baz', title: '' })).to.equal('Foo Bar Baz');
  });

  it('trims whitespace from titles before comparing to identifier', () => {
    expect(displayTitle({ identifier: 'p', title: '  ' })).to.equal('P');
  });
});

describe('WikiHashtag', () => {
  let el: WikiHashtagElement;
  let performSearchStub: sinon.SinonStub;

  beforeEach(async () => {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- custom element matches interface
    el = (await fixture(
      html`<wiki-hashtag tag="groceries">#Groceries</wiki-hashtag>`,
    )) as WikiHashtagElement;
    performSearchStub = sinon.stub(el, 'performSearch');
    // Default: a single result so most click-to-open paths render a list.
    performSearchStub.resolves([
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- minimal mock for tests
      { identifier: 'tagged-page', title: 'Tagged Page' } as SearchResult,
    ]);
  });

  afterEach(() => {
    sinon.restore();
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  describe('when rendered', () => {
    let anchor: HTMLAnchorElement | null;

    beforeEach(() => {
      anchor = el.shadowRoot.querySelector<HTMLAnchorElement>('a.hashtag-pill');
    });

    it('should render an anchor with the hashtag-pill class', () => {
      expect(anchor).to.exist;
    });

    it('should set href to "#" so right-click/no-JS does not navigate to a non-existent route', () => {
      expect(anchor?.getAttribute('href')).to.equal('#');
    });

    it('should slot the display text from light DOM', () => {
      // Display text is provided as the slotted child so case is preserved.
      expect(el.textContent?.trim()).to.equal('#Groceries');
    });

    it('should not render the bubble while closed', () => {
      const bubble = el.shadowRoot.querySelector('.bubble');
      expect(bubble).to.be.null;
    });
  });

  describe('when the anchor is clicked without modifier keys', () => {
    let clickEvent: MouseEvent;
    let anchor: HTMLAnchorElement;

    beforeEach(async () => {
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      clickEvent = new MouseEvent('click', {
        bubbles: true,
        cancelable: true,
        composed: true,
      });
      anchor.dispatchEvent(clickEvent);
      await waitUntil(
        () => el.shadowRoot.querySelector('.bubble-list') !== null,
        'Bubble list should appear after fetch resolves',
      );
    });

    it('should preventDefault on the click so the browser does not navigate', () => {
      expect(clickEvent.defaultPrevented).to.equal(true);
    });

    it('should call performSearch with the #-prefixed tag', () => {
      expect(performSearchStub).to.have.been.calledWith('#groceries');
    });

    it('should open the popover bubble', () => {
      const bubble = el.shadowRoot.querySelector('.bubble');
      expect(bubble).to.exist;
    });

    it('should render a list of result links', () => {
      const links = el.shadowRoot.querySelectorAll('.bubble-list a');
      expect(links.length).to.equal(1);
      expect(links[0]?.getAttribute('href')).to.equal('/tagged-page');
      expect(links[0]?.textContent?.trim()).to.equal('Tagged Page');
    });
  });

  describe('when results have no real title (server fell back to identifier)', () => {
    beforeEach(async () => {
      performSearchStub.reset();
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- minimal mock for tests
      performSearchStub.resolves([
        { identifier: 'home_lab', title: 'home_lab' } as SearchResult,
        { identifier: 'engineering-blog', title: '' } as SearchResult,
      ]);

      const anchor = el.shadowRoot.querySelector('a.hashtag-pill') as HTMLAnchorElement;
      anchor.click();
      await waitUntil(() => performSearchStub.called);
      await el.updateComplete;
    });

    it('should humanize the identifier instead of showing the raw form', () => {
      const links = el.shadowRoot.querySelectorAll('.bubble-list a');
      expect(links.length).to.equal(2);
      expect(links[0]?.textContent?.trim()).to.equal('Home Lab');
      expect(links[1]?.textContent?.trim()).to.equal('Engineering Blog');
    });
  });

  describe('when the anchor is clicked a second time', () => {
    let anchor: HTMLAnchorElement;

    beforeEach(async () => {
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      anchor.dispatchEvent(
        new MouseEvent('click', { bubbles: true, cancelable: true, composed: true }),
      );
      await waitUntil(
        () => el.shadowRoot.querySelector('.bubble') !== null,
        'Bubble should be open after first click',
      );

      anchor.dispatchEvent(
        new MouseEvent('click', { bubbles: true, cancelable: true, composed: true }),
      );
      await el.updateComplete;
    });

    it('should close the popover', () => {
      const bubble = el.shadowRoot.querySelector('.bubble');
      expect(bubble).to.be.null;
    });
  });

  describe('when the search is in flight', () => {
    let anchor: HTMLAnchorElement;
    let resolveSearch: (results: SearchResult[]) => void;

    beforeEach(async () => {
      performSearchStub.callsFake(
        () =>
          new Promise<SearchResult[]>((resolve) => {
            resolveSearch = resolve;
          }),
      );
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      anchor.dispatchEvent(
        new MouseEvent('click', { bubbles: true, cancelable: true, composed: true }),
      );
      await waitUntil(
        () => el.shadowRoot.querySelector('.bubble-loading') !== null,
        'Loading state should appear before fetch resolves',
      );
    });

    afterEach(() => {
      // Make sure we don't leave a dangling promise.
      resolveSearch?.([]);
    });

    it('should render the loading indicator', () => {
      const loading = el.shadowRoot.querySelector('.bubble-loading');
      expect(loading).to.exist;
    });

    it('should render a spinner inside the loading state', () => {
      const spinner = el.shadowRoot.querySelector('.bubble-loading .spinner');
      expect(spinner).to.exist;
    });
  });

  describe('when the search returns no results', () => {
    let anchor: HTMLAnchorElement;

    beforeEach(async () => {
      performSearchStub.resolves([]);
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      anchor.dispatchEvent(
        new MouseEvent('click', { bubbles: true, cancelable: true, composed: true }),
      );
      await waitUntil(
        () => el.shadowRoot.querySelector('.bubble-empty') !== null,
        'Empty state should appear',
      );
    });

    it('should render the empty state', () => {
      const empty = el.shadowRoot.querySelector('.bubble-empty');
      expect(empty?.textContent?.trim()).to.equal('No pages tagged #groceries.');
    });

    it('should not render a result list', () => {
      const list = el.shadowRoot.querySelector('.bubble-list');
      expect(list).to.be.null;
    });
  });

  describe('when the search throws', () => {
    let anchor: HTMLAnchorElement;

    beforeEach(async () => {
      performSearchStub.rejects(new Error('boom'));
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      anchor.dispatchEvent(
        new MouseEvent('click', { bubbles: true, cancelable: true, composed: true }),
      );
      await waitUntil(
        () => el.shadowRoot.querySelector('error-display') !== null,
        'error-display should be rendered',
      );
    });

    it('should render an error-display in the bubble', () => {
      const errorDisplay = el.shadowRoot.querySelector('error-display');
      expect(errorDisplay).to.exist;
    });
  });

  describe('when an outside click happens while open', () => {
    let anchor: HTMLAnchorElement;
    let outside: HTMLDivElement;

    beforeEach(async () => {
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      anchor.dispatchEvent(
        new MouseEvent('click', { bubbles: true, cancelable: true, composed: true }),
      );
      await waitUntil(
        () => el.shadowRoot.querySelector('.bubble') !== null,
        'Bubble should be open',
      );

      outside = document.createElement('div');
      document.body.appendChild(outside);
      outside.dispatchEvent(
        new MouseEvent('click', { bubbles: true, cancelable: true, composed: true }),
      );
      await el.updateComplete;
    });

    afterEach(() => {
      outside.remove();
    });

    it('should close the popover', () => {
      const bubble = el.shadowRoot.querySelector('.bubble');
      expect(bubble).to.be.null;
    });
  });

  describe('when Escape is pressed while open', () => {
    let anchor: HTMLAnchorElement;

    beforeEach(async () => {
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      anchor.dispatchEvent(
        new MouseEvent('click', { bubbles: true, cancelable: true, composed: true }),
      );
      await waitUntil(
        () => el.shadowRoot.querySelector('.bubble') !== null,
        'Bubble should be open',
      );

      document.dispatchEvent(
        new KeyboardEvent('keydown', {
          key: 'Escape',
          bubbles: true,
          cancelable: true,
        }),
      );
      await el.updateComplete;
    });

    it('should close the popover', () => {
      const bubble = el.shadowRoot.querySelector('.bubble');
      expect(bubble).to.be.null;
    });
  });

  // Modifier-key clicks (ctrl/meta/shift) used to fall through to native
  // browser navigation, but the inert `#` href made that pointless and the
  // synthetic-click test runner kept tripping page reloads. Now clicks
  // always open the popover regardless of modifiers — verify that.
  describe('when the anchor is ctrl-clicked', () => {
    let clickEvent: MouseEvent;
    let anchor: HTMLAnchorElement;

    beforeEach(async () => {
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      clickEvent = new MouseEvent('click', {
        bubbles: true,
        cancelable: true,
        composed: true,
        ctrlKey: true,
      });
      anchor.dispatchEvent(clickEvent);
      await waitUntil(
        () => el.shadowRoot.querySelector('.bubble') !== null,
        'Popover should still open on ctrl-click',
      );
    });

    it('should preventDefault so the browser does not navigate', () => {
      expect(clickEvent.defaultPrevented).to.equal(true);
    });

    it('should call performSearch (popover wins over modifier-fallback)', () => {
      expect(performSearchStub).to.have.been.calledWith('#groceries');
    });
  });

  describe('when a contextmenu event fires on the anchor', () => {
    // Right-click should open the browser's context menu, not trigger search.
    let contextEvent: MouseEvent;
    let anchor: HTMLAnchorElement;

    beforeEach(async () => {
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      contextEvent = new MouseEvent('contextmenu', {
        bubbles: true,
        cancelable: true,
        composed: true,
      });
      anchor.dispatchEvent(contextEvent);
      await el.updateComplete;
    });

    it('should not preventDefault on the context menu event', () => {
      expect(contextEvent.defaultPrevented).to.equal(false);
    });

    it('should not call performSearch', () => {
      expect(performSearchStub).to.not.have.been.called;
    });
  });

  describe('when tag is updated after construction', () => {
    let anchor: HTMLAnchorElement;

    beforeEach(async () => {
      el.tag = 'urgent';
      await el.updateComplete;
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
    });

    it('should keep the inert `#` href regardless of tag', () => {
      expect(anchor.getAttribute('href')).to.equal('#');
    });

    describe('and then clicked', () => {
      beforeEach(async () => {
        anchor.dispatchEvent(
          new MouseEvent('click', { bubbles: true, cancelable: true, composed: true }),
        );
        await waitUntil(
          () => el.shadowRoot.querySelector('.bubble') !== null,
          'Bubble should open',
        );
      });

      it('should call performSearch with the new tag', () => {
        expect(performSearchStub).to.have.been.calledWith('#urgent');
      });
    });
  });

  describe('when a second wiki-hashtag opens its popover while the first is open', () => {
    let firstEl: WikiHashtagElement;
    let secondEl: WikiHashtagElement;

    beforeEach(async () => {
      const container = await fixture<HTMLElement>(html`
        <div>
          <wiki-hashtag tag="alpha">#alpha</wiki-hashtag>
          <wiki-hashtag tag="beta">#beta</wiki-hashtag>
        </div>
      `);
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- custom element matches interface
      firstEl = container.children[0] as WikiHashtagElement;
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- custom element matches interface
      secondEl = container.children[1] as WikiHashtagElement;

      sinon.stub(firstEl, 'performSearch').resolves([]);
      sinon.stub(secondEl, 'performSearch').resolves([]);

      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      const firstAnchor = firstEl.shadowRoot.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      firstAnchor.dispatchEvent(
        new MouseEvent('click', { bubbles: true, cancelable: true, composed: true }),
      );
      await waitUntil(
        () => firstEl.shadowRoot.querySelector('.bubble') !== null,
        'First popover should open',
      );

      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      const secondAnchor = secondEl.shadowRoot.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      secondAnchor.dispatchEvent(
        new MouseEvent('click', { bubbles: true, cancelable: true, composed: true }),
      );
      await waitUntil(
        () => secondEl.shadowRoot.querySelector('.bubble') !== null,
        'Second popover should open',
      );
    });

    it('should close the first popover', () => {
      expect(firstEl.shadowRoot.querySelector('.bubble')).to.be.null;
    });

    it('should keep the second popover open', () => {
      expect(secondEl.shadowRoot.querySelector('.bubble')).to.exist;
    });
  });
});
