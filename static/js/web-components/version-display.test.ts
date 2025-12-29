import { html, fixture, expect } from '@open-wc/testing';
import { VersionDisplay } from './version-display.js';
import { GetVersionResponseSchema } from '../gen/api/v1/system_info_pb.js';
import { create } from '@bufbuild/protobuf';
import { TimestampSchema } from '@bufbuild/protobuf/wkt';
import './version-display.js';

describe('VersionDisplay', () => {
  let el: VersionDisplay;

  beforeEach(async () => {
    el = await fixture(html`<version-display></version-display>`);
    // Set initial state manually to avoid network requests
    el.loading = false;
    el.error = undefined;
    
    // Create a proper mock timestamp
    const mockTimestamp = create(TimestampSchema, {
      seconds: BigInt(Math.floor(new Date('2023-01-01T12:00:00Z').getTime() / 1000)),
      nanos: 0
    });
    
    el.version = create(GetVersionResponseSchema, {
      commit: 'abc123def456',
      buildTime: mockTimestamp
    });
    
    await el.updateComplete;
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
      // Make sure the component has rendered with version data
      el.loading = false;
      el.error = undefined;
      el.version = {
        commit: 'thisiscommithash',
        buildTime: { toDate: () => new Date('2023-01-01T12:00:00Z') }
      } as unknown;
      await el.updateComplete;
    });

    it('should display version panel', () => {
      const panel = el.shadowRoot?.querySelector('.version-panel');
      expect(panel).to.exist;
    });

    it('should display hover overlay', () => {
      const overlay = el.shadowRoot?.querySelector('.hover-overlay');
      expect(overlay).to.exist;
    });

    it('should display version information', () => {
      const versionRow = el.shadowRoot?.querySelector('.version-row');
      expect(versionRow).to.exist;
      expect(versionRow?.textContent).to.contain('thisisc');
    });
  });

  describe('when positioned', () => {
    it('should have fixed position styling', () => {
      const styles = getComputedStyle(el);
      expect(styles.position).to.equal('fixed');
    });
  });

  describe('when in error state', () => {
    beforeEach(async () => {
      el.loading = false;
      el.error = 'Network error';
      el.version = undefined;
      await el.updateComplete;
    });

    it('should show error state', () => {
      const error = el.shadowRoot?.querySelector('.error');
      expect(error).to.exist;
      expect(error?.textContent).to.contain('Network error');
    });

    it('should not display version information', () => {
      const versionRow = el.shadowRoot?.querySelector('.version-row');
      expect(versionRow).to.not.exist;
    });
  });

  describe('when in loading state', () => {
    beforeEach(async () => {
      el.loading = true;
      el.error = undefined;
      el.version = undefined;
      await el.updateComplete;
    });

    it('should show loading state', () => {
      const loadingElements = el.shadowRoot?.querySelectorAll('.loading');
      expect(loadingElements).to.have.length(2);
      loadingElements?.forEach(element => {
        expect(element.textContent).to.contain('Loading...');
      });
    });

    it('should not show error state', () => {
      const error = el.shadowRoot?.querySelector('.error');
      expect(error).to.not.exist;
    });

    it('should show version row structure', () => {
      const versionRows = el.shadowRoot?.querySelectorAll('.version-row');
      expect(versionRows).to.have.length(2);

      // Check that the labels are present
      const labels = el.shadowRoot?.querySelectorAll('.label');
      expect(labels).to.have.length(2);
      expect(labels?.[0]?.textContent).to.contain('Commit:');
      expect(labels?.[1]?.textContent).to.contain('Built:');
    });
  });

  describe('when formatting commit hash', () => {
    it('should truncate long commit hashes', () => {
      const longCommit = 'abcdefghijklmnopqrstuvwxyz123456789';
      const result = el['formatCommit'](longCommit);
      expect(result).to.equal('abcdefg');
    });

    it('should not truncate short commit hashes', () => {
      const shortCommit = 'abc123';
      const result = el['formatCommit'](shortCommit);
      expect(result).to.equal('abc123');
    });

    it('should not truncate tagged versions', () => {
      const taggedVersion = 'v1.2.3 (abc1234)';
      const result = el['formatCommit'](taggedVersion);
      expect(result).to.equal('v1.2.3 (abc1234)');
    });

    it('should not truncate longer tagged versions', () => {
      const taggedVersion = 'v10.20.30-beta.1 (abcdef123456)';
      const result = el['formatCommit'](taggedVersion);
      expect(result).to.equal('v10.20.30-beta.1 (abcdef123456)');
    });
  });

  describe('when formatting timestamp', () => {
    it('should format valid timestamp', () => {
      const mockTimestamp = create(TimestampSchema, {
        seconds: BigInt(Math.floor(new Date('2023-01-01T12:00:00Z').getTime() / 1000)),
        nanos: 0
      });
      const result = el['formatTimestamp'](mockTimestamp);
      expect(result).to.contain('Jan 1, 2023');
    });
  });
});
