import { html, fixture, expect } from '@open-wc/testing';
import { VersionDisplay } from './version-display.js';
import './version-display.js';

describe('VersionDisplay', () => {
  let el: VersionDisplay;

  beforeEach(async () => {
    el = await fixture(html`<version-display></version-display>`);
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  it('should be an instance of VersionDisplay', () => {
    expect(el).to.be.instanceOf(VersionDisplay);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('version-display');
  });

  describe('when component is rendered', () => {
    beforeEach(async () => {
      await el.updateComplete;
    });

    it('should display version panel', () => {
      const panel = el.shadowRoot?.querySelector('.version-panel');
      expect(panel).to.exist;
    });

    it('should show loading state or error state initially', () => {
      const loading = el.shadowRoot?.querySelector('.loading');
      const error = el.shadowRoot?.querySelector('.error');
      // Should have either loading or error state
      expect(loading || error).to.exist;
    });
  });

  describe('when positioned', () => {
    it('should have fixed position styling', () => {
      const styles = getComputedStyle(el);
      expect(styles.position).to.equal('fixed');
    });
  });
});