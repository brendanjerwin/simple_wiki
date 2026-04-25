import { html, fixture, expect } from '@open-wc/testing';
import sinon from 'sinon';
import './wiki-hashtag.js';

interface WikiHashtagElement extends HTMLElement {
  tag: string;
  updateComplete: Promise<boolean>;
}

interface SearchOpenDetail {
  query: string;
}

describe('WikiHashtag', () => {
  let el: WikiHashtagElement;

  beforeEach(async () => {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- custom element matches interface
    el = (await fixture(
      html`<wiki-hashtag tag="groceries">#Groceries</wiki-hashtag>`,
    )) as WikiHashtagElement;
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
      anchor = el.shadowRoot?.querySelector<HTMLAnchorElement>('a.hashtag-pill') ?? null;
    });

    it('should render an anchor with the hashtag-pill class', () => {
      expect(anchor).to.exist;
    });

    it('should set href to the search fallback URL with the encoded tag', () => {
      // Fallback href is what makes "open in new tab" / no-JS still useful.
      expect(anchor?.getAttribute('href')).to.equal('/?q=%23groceries');
    });

    it('should slot the display text from light DOM', () => {
      // Display text is provided as the slotted child so case is preserved.
      expect(el.textContent?.trim()).to.equal('#Groceries');
    });
  });

  describe('when the anchor is clicked without modifier keys', () => {
    let dispatched: CustomEvent<SearchOpenDetail> | null;
    let clickEvent: MouseEvent;
    let anchor: HTMLAnchorElement;

    beforeEach(() => {
      dispatched = null;
      globalThis.addEventListener(
        'wiki-search-open',
        (e: Event) => {
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- known custom event
          dispatched = e as CustomEvent<SearchOpenDetail>;
        },
        { once: true },
      );

      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot!.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      clickEvent = new MouseEvent('click', {
        bubbles: true,
        cancelable: true,
        composed: true,
      });
      anchor.dispatchEvent(clickEvent);
    });

    it('should dispatch a wiki-search-open event', () => {
      expect(dispatched).to.exist;
    });

    it('should include the #-prefixed tag as the query', () => {
      expect(dispatched?.detail.query).to.equal('#groceries');
    });

    it('should bubble through the document', () => {
      expect(dispatched?.bubbles).to.equal(true);
    });

    it('should be composed so it crosses shadow boundaries', () => {
      expect(dispatched?.composed).to.equal(true);
    });

    it('should preventDefault on the click so the browser does not navigate', () => {
      expect(clickEvent.defaultPrevented).to.equal(true);
    });
  });

  describe('when the anchor is ctrl-clicked', () => {
    let dispatched: Event | null;
    let clickEvent: MouseEvent;
    let anchor: HTMLAnchorElement;

    beforeEach(() => {
      dispatched = null;
      globalThis.addEventListener(
        'wiki-search-open',
        (e: Event) => {
          dispatched = e;
        },
        { once: true },
      );

      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot!.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      clickEvent = new MouseEvent('click', {
        bubbles: true,
        cancelable: true,
        composed: true,
        ctrlKey: true,
      });
      anchor.dispatchEvent(clickEvent);
    });

    it('should not dispatch a wiki-search-open event', () => {
      expect(dispatched).to.be.null;
    });

    it('should not preventDefault so the browser can open in a new tab via the href', () => {
      expect(clickEvent.defaultPrevented).to.equal(false);
    });
  });

  describe('when the anchor is meta-clicked', () => {
    // Cmd-click on macOS is meta-click — same "open in new tab" intent.
    let dispatched: Event | null;
    let clickEvent: MouseEvent;
    let anchor: HTMLAnchorElement;

    beforeEach(() => {
      dispatched = null;
      globalThis.addEventListener(
        'wiki-search-open',
        (e: Event) => {
          dispatched = e;
        },
        { once: true },
      );

      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot!.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      clickEvent = new MouseEvent('click', {
        bubbles: true,
        cancelable: true,
        composed: true,
        metaKey: true,
      });
      anchor.dispatchEvent(clickEvent);
    });

    it('should not dispatch a wiki-search-open event', () => {
      expect(dispatched).to.be.null;
    });

    it('should not preventDefault', () => {
      expect(clickEvent.defaultPrevented).to.equal(false);
    });
  });

  describe('when the anchor is shift-clicked', () => {
    // Shift-click traditionally opens in a new window — preserve that.
    let dispatched: Event | null;
    let clickEvent: MouseEvent;
    let anchor: HTMLAnchorElement;

    beforeEach(() => {
      dispatched = null;
      globalThis.addEventListener(
        'wiki-search-open',
        (e: Event) => {
          dispatched = e;
        },
        { once: true },
      );

      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot!.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      clickEvent = new MouseEvent('click', {
        bubbles: true,
        cancelable: true,
        composed: true,
        shiftKey: true,
      });
      anchor.dispatchEvent(clickEvent);
    });

    it('should not dispatch a wiki-search-open event', () => {
      expect(dispatched).to.be.null;
    });

    it('should not preventDefault', () => {
      expect(clickEvent.defaultPrevented).to.equal(false);
    });
  });

  describe('when a contextmenu event fires on the anchor', () => {
    // Right-click should open the browser's context menu, not trigger search.
    let dispatched: Event | null;
    let contextEvent: MouseEvent;
    let anchor: HTMLAnchorElement;

    beforeEach(() => {
      dispatched = null;
      globalThis.addEventListener(
        'wiki-search-open',
        (e: Event) => {
          dispatched = e;
        },
        { once: true },
      );

      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot!.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
      contextEvent = new MouseEvent('contextmenu', {
        bubbles: true,
        cancelable: true,
        composed: true,
      });
      anchor.dispatchEvent(contextEvent);
    });

    it('should not dispatch a wiki-search-open event', () => {
      expect(dispatched).to.be.null;
    });

    it('should not preventDefault on the context menu event', () => {
      expect(contextEvent.defaultPrevented).to.equal(false);
    });
  });

  describe('when tag is updated after construction', () => {
    let anchor: HTMLAnchorElement;

    beforeEach(async () => {
      el.tag = 'urgent';
      await el.updateComplete;
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- rendered above
      anchor = el.shadowRoot!.querySelector<HTMLAnchorElement>('a.hashtag-pill')!;
    });

    it('should update the fallback href to the new tag', () => {
      expect(anchor.getAttribute('href')).to.equal('/?q=%23urgent');
    });

    describe('and then clicked', () => {
      let dispatched: CustomEvent<SearchOpenDetail> | null;

      beforeEach(() => {
        dispatched = null;
        globalThis.addEventListener(
          'wiki-search-open',
          (e: Event) => {
            // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- known custom event
            dispatched = e as CustomEvent<SearchOpenDetail>;
          },
          { once: true },
        );
        anchor.dispatchEvent(
          new MouseEvent('click', { bubbles: true, cancelable: true, composed: true }),
        );
      });

      it('should dispatch the new tag in the query', () => {
        expect(dispatched?.detail.query).to.equal('#urgent');
      });
    });
  });
});
