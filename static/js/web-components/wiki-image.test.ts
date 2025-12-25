import { html, fixture, expect } from '@open-wc/testing';
import './wiki-image.js';
import type { WikiImage } from './wiki-image.js';
import sinon, { SinonStub } from 'sinon';

describe('WikiImage', () => {
  let el: WikiImage;

  it('should exist', async () => {
    el = await fixture(html`<wiki-image></wiki-image>`);
    expect(el).to.exist;
  });

  describe('when rendering with src and alt', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <wiki-image src="/uploads/test.jpg" alt="Test image"></wiki-image>
      `);
    });

    it('should render an img element', () => {
      const img = el.shadowRoot?.querySelector('img');
      expect(img).to.exist;
    });

    it('should set the src attribute', () => {
      const img = el.shadowRoot?.querySelector('img');
      expect(img?.getAttribute('src')).to.equal('/uploads/test.jpg');
    });

    it('should set the alt attribute', () => {
      const img = el.shadowRoot?.querySelector('img');
      expect(img?.alt).to.equal('Test image');
    });
  });

  describe('when rendering with title', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <wiki-image
          src="/uploads/test.jpg"
          alt="Test image"
          title="Click to view"
        ></wiki-image>
      `);
    });

    it('should set the title attribute', () => {
      const img = el.shadowRoot?.querySelector('img');
      expect(img?.title).to.equal('Click to view');
    });
  });

  describe('when clicking the image', () => {
    let windowOpenStub: SinonStub;

    beforeEach(async () => {
      windowOpenStub = sinon.stub(window, 'open');
      el = await fixture(html`
        <wiki-image src="/uploads/test.jpg" alt="Test"></wiki-image>
      `);
      const img = el.shadowRoot?.querySelector('img');
      img?.click();
    });

    afterEach(() => {
      windowOpenStub.restore();
    });

    it('should open the image in a new tab', () => {
      expect(windowOpenStub).to.have.been.calledWith('/uploads/test.jpg', '_blank');
    });
  });
});
