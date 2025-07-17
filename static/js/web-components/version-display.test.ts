import { html, fixture, expect } from '@open-wc/testing';
import { VersionDisplay } from './version-display.js';
import './version-display.js';
import * as sinon from 'sinon';

describe('VersionDisplay', () => {
  let el: VersionDisplay;
  let getVersionStub: sinon.SinonStub;
  const mockResponse = {
    version: '1.2.3',
    commit: 'abc123def456',
    buildTime: { toDate: () => new Date('2023-01-01T12:00:00Z') }
  };

  beforeEach(async () => {
    el = await fixture(html`<version-display></version-display>`);
    // Stub the gRPC client immediately to prevent network requests
    getVersionStub = sinon.stub(el['client'], 'getVersion').resolves(mockResponse);
  });

  afterEach(() => {
    if (getVersionStub) {
      getVersionStub.restore();
    }
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

    it('should display hover overlay', () => {
      const overlay = el.shadowRoot?.querySelector('.hover-overlay');
      expect(overlay).to.exist;
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

  describe('when gRPC response is successful', () => {
    beforeEach(async () => {
      getVersionStub.resolves(mockResponse);
      await el['loadVersion']();
      await el.updateComplete;
    });

    it('should not show loading state', () => {
      const loading = el.shadowRoot?.querySelector('.loading');
      expect(loading).to.not.exist;
    });

    it('should not show error state', () => {
      const error = el.shadowRoot?.querySelector('.error');
      expect(error).to.not.exist;
    });

    it('should display version information', () => {
      const versionRow = el.shadowRoot?.querySelector('.version-row');
      expect(versionRow).to.exist;
      expect(versionRow?.textContent).to.contain('1.2.3');
    });

    it('should display commit hash', () => {
      const commitElement = el.shadowRoot?.querySelector('.commit');
      expect(commitElement).to.exist;
      expect(commitElement?.textContent).to.contain('abc123d');
    });

    it('should display build time', () => {
      const versionInfo = el.shadowRoot?.querySelector('.version-info');
      expect(versionInfo?.textContent).to.contain('Jan 1, 2023');
    });
  });

  describe('when gRPC response fails', () => {
    const mockError = new Error('Network error');

    beforeEach(async () => {
      getVersionStub.rejects(mockError);
      await el['loadVersion']();
      await el.updateComplete;
    });

    it('should not show loading state', () => {
      const loading = el.shadowRoot?.querySelector('.loading');
      expect(loading).to.not.exist;
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

  describe('when gRPC response fails with unknown error', () => {
    beforeEach(async () => {
      getVersionStub.callsFake(() => Promise.reject('Unknown error'));
      await el['loadVersion']();
      await el.updateComplete;
    });

    it('should not show loading state', () => {
      const loading = el.shadowRoot?.querySelector('.loading');
      expect(loading).to.not.exist;
    });

    it('should show generic error message', () => {
      const error = el.shadowRoot?.querySelector('.error');
      expect(error).to.exist;
      expect(error?.textContent).to.contain('Failed to load version');
    });
  });

  describe('when loading state is active', () => {
    beforeEach(async () => {
      // Create a promise that won't resolve during the test
      const promise = new Promise(() => {
        // This promise never resolves, keeping the component in loading state
      });
      getVersionStub.returns(promise);
      el['loadVersion']();
      await el.updateComplete;
    });

    it('should show loading state', () => {
      const loading = el.shadowRoot?.querySelector('.loading');
      expect(loading).to.exist;
      expect(loading?.textContent).to.contain('Loading version...');
    });

    it('should not show error state', () => {
      const error = el.shadowRoot?.querySelector('.error');
      expect(error).to.not.exist;
    });

    it('should not show version information', () => {
      const versionRow = el.shadowRoot?.querySelector('.version-row');
      expect(versionRow).to.not.exist;
    });
  });

  describe('when mouse enters component', () => {
    let loadVersionSpy: sinon.SinonSpy;

    beforeEach(async () => {
      // Reset stub to default behavior
      getVersionStub.resolves(mockResponse);
      loadVersionSpy = sinon.spy(el, 'loadVersion' as keyof VersionDisplay);
      await el.updateComplete;
    });

    afterEach(() => {
      loadVersionSpy.restore();
    });

    it('should call loadVersion on mouseenter to overlay', () => {
      const overlay = el.shadowRoot?.querySelector('.hover-overlay');
      expect(overlay).to.exist;
      
      const mouseEnterEvent = new MouseEvent('mouseenter');
      overlay?.dispatchEvent(mouseEnterEvent);
      expect(loadVersionSpy).to.have.been.calledOnce;
    });
  });

  describe('when component is connected to DOM', () => {
    let loadVersionSpy: sinon.SinonSpy;

    beforeEach(async () => {
      loadVersionSpy = sinon.spy(VersionDisplay.prototype, 'loadVersion' as keyof VersionDisplay);
      // Re-create the element to trigger connectedCallback
      el = await fixture(html`<version-display></version-display>`);
      // Reset stub to default behavior
      getVersionStub.resolves(mockResponse);
      await el.updateComplete;
    });

    afterEach(() => {
      loadVersionSpy.restore();
    });

    it('should call loadVersion when connected', () => {
      expect(loadVersionSpy).to.have.been.called;
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
  });

  describe('when formatting timestamp', () => {
    it('should format valid timestamp', () => {
      const mockTimestamp = { toDate: () => new Date('2023-01-01T12:00:00Z') };
      const result = el['formatTimestamp'](mockTimestamp as { toDate: () => Date });
      expect(result).to.contain('Jan 1, 2023');
    });

    it('should handle undefined timestamp', () => {
      const result = el['formatTimestamp'](undefined);
      expect(result).to.equal('Unknown');
    });

    it('should handle invalid timestamp', () => {
      const mockTimestamp = { toDate: () => { throw new Error('Invalid date'); } };
      const result = el['formatTimestamp'](mockTimestamp as { toDate: () => Date });
      expect(result).to.equal('Invalid date');
    });
  });
});