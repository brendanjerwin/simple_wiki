import { expect, fixture, html } from '@open-wc/testing';
import sinon from 'sinon';
import type { SinonStub } from 'sinon';
import './collapsible-heading.js';
import type { CollapsibleHeading } from './collapsible-heading.js';

afterEach(() => {
  sinon.restore();
});

describe('CollapsibleHeading', () => {
  it('should exist', async () => {
    const el = await fixture<CollapsibleHeading>(html`
      <collapsible-heading heading-level="1">
        <h1 slot="heading" id="test">Test</h1>
        <p>Content</p>
      </collapsible-heading>
    `);
    expect(el).to.exist;
  });

  describe('when rendered with content', () => {
    let el: CollapsibleHeading;

    beforeEach(async () => {
      sinon.stub(Storage.prototype, 'getItem').returns(null);
      el = await fixture<CollapsibleHeading>(html`
        <collapsible-heading heading-level="1">
          <h1 slot="heading" id="test-section">Test Heading</h1>
          <p>Section content here.</p>
        </collapsible-heading>
      `);
    });

    it('should render a toggle button', () => {
      const button = el.shadowRoot?.querySelector('.ch-toggle');
      expect(button).to.exist;
    });

    it('should render the heading slot', () => {
      const slot = el.shadowRoot?.querySelector('slot[name="heading"]');
      expect(slot).to.exist;
    });

    it('should render the content slot', () => {
      const slot = el.shadowRoot?.querySelector('slot:not([name])');
      expect(slot).to.exist;
    });
  });

  describe('when initially rendered without saved state', () => {
    let el: CollapsibleHeading;

    beforeEach(async () => {
      sinon.stub(Storage.prototype, 'getItem').returns(null);
      el = await fixture<CollapsibleHeading>(html`
        <collapsible-heading heading-level="1">
          <h1 slot="heading" id="test-section">Test Heading</h1>
          <p>Section content here.</p>
        </collapsible-heading>
      `);
    });

    it('should be collapsed by default', () => {
      expect(el.collapsed).to.equal(true);
    });

    it('should show collapsed indicator in toggle button', () => {
      const button = el.shadowRoot?.querySelector('.ch-toggle');
      expect(button?.textContent?.trim()).to.equal('▶');
    });

    it('should have section content hidden', () => {
      const content = el.shadowRoot?.querySelector('.ch-content');
      expect(content?.hasAttribute('hidden')).to.equal(true);
    });
  });

  describe('when saved state is expanded', () => {
    let el: CollapsibleHeading;

    beforeEach(async () => {
      sinon.stub(Storage.prototype, 'getItem').returns('expanded');
      el = await fixture<CollapsibleHeading>(html`
        <collapsible-heading heading-level="1">
          <h1 slot="heading" id="test-section">Test Heading</h1>
          <p>Section content here.</p>
        </collapsible-heading>
      `);
    });

    it('should be expanded', () => {
      expect(el.collapsed).to.equal(false);
    });

    it('should show expanded indicator in toggle button', () => {
      const button = el.shadowRoot?.querySelector('.ch-toggle');
      expect(button?.textContent?.trim()).to.equal('▼');
    });

    it('should have section content visible', () => {
      const content = el.shadowRoot?.querySelector('.ch-content');
      expect(content?.hasAttribute('hidden')).to.equal(false);
    });
  });

  describe('when toggle button is clicked while collapsed', () => {
    let el: CollapsibleHeading;
    let localStorageSetStub: SinonStub;

    beforeEach(async () => {
      sinon.stub(Storage.prototype, 'getItem').returns(null);
      localStorageSetStub = sinon.stub(Storage.prototype, 'setItem');
      el = await fixture<CollapsibleHeading>(html`
        <collapsible-heading heading-level="1">
          <h1 slot="heading" id="test-section">Test Heading</h1>
          <p>Section content here.</p>
        </collapsible-heading>
      `);
      const button = el.shadowRoot?.querySelector<HTMLButtonElement>('.ch-toggle');
      button?.click();
      await el.updateComplete;
    });

    it('should become expanded', () => {
      expect(el.collapsed).to.equal(false);
    });

    it('should show expanded indicator', () => {
      const button = el.shadowRoot?.querySelector('.ch-toggle');
      expect(button?.textContent?.trim()).to.equal('▼');
    });

    it('should save expanded state to localStorage', () => {
      expect(localStorageSetStub).to.have.been.calledWith(
        sinon.match.string,
        'expanded',
      );
    });

    it('should show section content', () => {
      const content = el.shadowRoot?.querySelector('.ch-content');
      expect(content?.hasAttribute('hidden')).to.equal(false);
    });
  });

  describe('when toggle button is clicked while expanded', () => {
    let el: CollapsibleHeading;
    let localStorageSetStub: SinonStub;

    beforeEach(async () => {
      sinon.stub(Storage.prototype, 'getItem').returns('expanded');
      localStorageSetStub = sinon.stub(Storage.prototype, 'setItem');
      el = await fixture<CollapsibleHeading>(html`
        <collapsible-heading heading-level="1">
          <h1 slot="heading" id="test-section">Test Heading</h1>
          <p>Section content here.</p>
        </collapsible-heading>
      `);
      const button = el.shadowRoot?.querySelector<HTMLButtonElement>('.ch-toggle');
      button?.click();
      await el.updateComplete;
    });

    it('should become collapsed', () => {
      expect(el.collapsed).to.equal(true);
    });

    it('should show collapsed indicator', () => {
      const button = el.shadowRoot?.querySelector('.ch-toggle');
      expect(button?.textContent?.trim()).to.equal('▶');
    });

    it('should save collapsed state to localStorage', () => {
      expect(localStorageSetStub).to.have.been.calledWith(
        sinon.match.string,
        'collapsed',
      );
    });

    it('should hide section content', () => {
      const content = el.shadowRoot?.querySelector('.ch-content');
      expect(content?.hasAttribute('hidden')).to.equal(true);
    });
  });

  describe('when the heading has an id', () => {
    let el: CollapsibleHeading;
    let localStorageSetStub: SinonStub;

    beforeEach(async () => {
      sinon.stub(Storage.prototype, 'getItem').returns(null);
      localStorageSetStub = sinon.stub(Storage.prototype, 'setItem');
      el = await fixture<CollapsibleHeading>(html`
        <collapsible-heading heading-level="1">
          <h1 slot="heading" id="departments-reference">Departments Reference</h1>
          <p>Content</p>
        </collapsible-heading>
      `);
    });

    it('should use the heading id in the storage key', () => {
      const button = el.shadowRoot?.querySelector<HTMLButtonElement>('.ch-toggle');
      button?.click();
      const storedKey = localStorageSetStub.firstCall.args[0] as string;
      expect(storedKey).to.include('departments-reference');
    });
  });

  describe('keyboard accessibility', () => {
    let el: CollapsibleHeading;

    beforeEach(async () => {
      sinon.stub(Storage.prototype, 'getItem').returns(null);
      el = await fixture<CollapsibleHeading>(html`
        <collapsible-heading heading-level="1">
          <h1 slot="heading" id="test">Test</h1>
          <p>Content</p>
        </collapsible-heading>
      `);
    });

    it('should have a toggle button element', () => {
      const button = el.shadowRoot?.querySelector('.ch-toggle');
      expect(button?.tagName).to.equal('BUTTON');
    });

    it('should set aria-expanded to false when collapsed', () => {
      const button = el.shadowRoot?.querySelector('.ch-toggle');
      expect(button?.getAttribute('aria-expanded')).to.equal('false');
    });
  });

  describe('aria-label accessibility', () => {
    let el: CollapsibleHeading;

    beforeEach(async () => {
      sinon.stub(Storage.prototype, 'getItem').returns(null);
      el = await fixture<CollapsibleHeading>(html`
        <collapsible-heading heading-level="2">
          <h2 slot="heading" id="my-section">My Section</h2>
          <p>Content</p>
        </collapsible-heading>
      `);
    });

    it('should have aria-label on the toggle button derived from heading text', () => {
      const button = el.shadowRoot?.querySelector('.ch-toggle');
      expect(button?.getAttribute('aria-label')).to.equal('Toggle My Section');
    });

    it('should have an id on the content div', () => {
      const content = el.shadowRoot?.querySelector('.ch-content');
      expect(content?.id).to.not.be.empty;
    });

    it('should have aria-controls on the toggle button matching the content div id', () => {
      const button = el.shadowRoot?.querySelector('.ch-toggle');
      const content = el.shadowRoot?.querySelector('.ch-content');
      expect(button?.getAttribute('aria-controls')).to.equal(content?.id);
    });
  });

  describe('aria-label accessibility when heading has no text', () => {
    let el: CollapsibleHeading;

    beforeEach(async () => {
      sinon.stub(Storage.prototype, 'getItem').returns(null);
      el = await fixture<CollapsibleHeading>(html`
        <collapsible-heading heading-level="1">
          <h1 slot="heading" id="empty-heading"></h1>
          <p>Content</p>
        </collapsible-heading>
      `);
    });

    it('should fall back to "Toggle section" aria-label', () => {
      const button = el.shadowRoot?.querySelector('.ch-toggle');
      expect(button?.getAttribute('aria-label')).to.equal('Toggle section');
    });
  });

  describe('when the heading has no id', () => {
    let el: CollapsibleHeading;
    let localStorageSetStub: SinonStub;
    let getItemStub: SinonStub;

    beforeEach(async () => {
      getItemStub = sinon.stub(Storage.prototype, 'getItem').returns(null);
      localStorageSetStub = sinon.stub(Storage.prototype, 'setItem');
      el = await fixture<CollapsibleHeading>(html`
        <collapsible-heading heading-level="1">
          <h1 slot="heading">No ID Here</h1>
          <p>Content</p>
        </collapsible-heading>
      `);
    });

    it('should not call localStorage.setItem on toggle', async () => {
      const button = el.shadowRoot?.querySelector<HTMLButtonElement>('.ch-toggle');
      button?.click();
      await el.updateComplete;
      expect(localStorageSetStub.callCount).to.equal(0);
    });

    it('should not call localStorage.getItem for state loading', () => {
      expect(getItemStub.callCount).to.equal(0);
    });
  });
});
