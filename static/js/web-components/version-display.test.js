import { html, fixture, expect } from '@open-wc/testing';
import { VersionDisplay } from './version-display.js';
import sinon from 'sinon';

describe('VersionDisplay', () => {
  let el;
  let fetchStub;

  beforeEach(() => {
    // Mock the fetch API
    fetchStub = sinon.stub(window, 'fetch');
  });

  afterEach(() => {
    sinon.restore();
  });

  describe('when component is created', () => {
    beforeEach(async () => {
      // Mock fetch to prevent immediate execution
      fetchStub.returns(Promise.reject(new Error('Network error')));
      
      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
      
      // Wait for async operations to complete
      await new Promise(resolve => setTimeout(resolve, 10));
      await el.updateComplete;
    });

    it('should exist', () => {
      expect(el).to.be.instanceOf(VersionDisplay);
    });

    it('should have initial empty state after failed fetch', () => {
      expect(el.loading).to.be.false;
      expect(el.version).to.equal('');
      expect(el.commit).to.equal('');
      expect(el.buildTime).to.equal('');
      expect(el.error).to.equal('Network error');
    });

    it('should be positioned fixed at bottom right', () => {
      const styles = getComputedStyle(el);
      expect(styles.position).to.equal('fixed');
      expect(styles.bottom).to.equal('5px');
      expect(styles.right).to.equal('5px');
    });
  });

  describe('when component is connected to DOM', () => {
    let fetchVersionSpy;

    beforeEach(async () => {
      fetchVersionSpy = sinon.spy(VersionDisplay.prototype, 'fetchVersion');
      fetchStub.returns(Promise.reject(new Error('Network error')));
      
      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
    });

    afterEach(() => {
      fetchVersionSpy.restore();
    });

    it('should call fetchVersion on connectedCallback', () => {
      expect(fetchVersionSpy).to.have.been.calledOnce;
    });
  });

  describe('when fetchVersion is called successfully', () => {
    beforeEach(async () => {
      // Mock successful response
      const mockResponse = {
        ok: true,
        status: 200,
        statusText: 'OK',
        arrayBuffer: () => Promise.resolve(new ArrayBuffer(0))
      };
      fetchStub.resolves(mockResponse);
      
      el = await fixture(html`<version-display></version-display>`);
      
      // Manually set version data to simulate successful parsing
      el.version = '1.2.3';
      el.commit = 'abc123';
      el.buildTime = '1/1/2023, 12:00:00 PM';
      el.loading = false;
      el.error = '';
      
      await el.updateComplete;
    });

    it('should call fetch with correct parameters', () => {
      expect(fetchStub).to.have.been.calledOnce;
      expect(fetchStub.firstCall.args[0]).to.equal('/api.v1.Version/GetVersion');
      expect(fetchStub.firstCall.args[1].method).to.equal('POST');
      expect(fetchStub.firstCall.args[1].headers['Content-Type']).to.equal('application/grpc-web+proto');
    });

    it('should display version information when manually set', () => {
      const versionPanel = el.shadowRoot.querySelector('.version-panel');
      expect(versionPanel).to.exist;
      expect(versionPanel.textContent).to.include('1.2.3');
      expect(versionPanel.textContent).to.include('abc123');
    });
  });

  describe('when fetchVersion fails', () => {
    beforeEach(async () => {
      // Mock failed response
      const error = new Error('Network error');
      fetchStub.rejects(error);

      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
      
      // Wait for async operations to complete
      await new Promise(resolve => setTimeout(resolve, 10));
      await el.updateComplete;
    });

    it('should handle errors gracefully', () => {
      expect(el.version).to.equal('');
      expect(el.commit).to.equal('');
      expect(el.buildTime).to.equal('');
      expect(el.loading).to.be.false;
      expect(el.error).to.equal('Network error');
    });

    it('should not display anything when there is an error', () => {
      // Check that no visible elements are rendered
      const versionPanel = el.shadowRoot.querySelector('.version-panel');
      expect(versionPanel).to.be.null;
    });
  });

  describe('when component has no data and not loading', () => {
    beforeEach(async () => {
      // Mock successful response but no data
      const mockResponse = {
        ok: true,
        status: 200,
        statusText: 'OK',
        arrayBuffer: () => Promise.resolve(new ArrayBuffer(0))
      };
      fetchStub.resolves(mockResponse);

      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
      
      // Wait for async operations to complete
      await new Promise(resolve => setTimeout(resolve, 10));
      await el.updateComplete;
    });

    it('should not display anything when there is no data', () => {
      // Check that no visible elements are rendered
      const versionPanel = el.shadowRoot.querySelector('.version-panel');
      expect(versionPanel).to.be.null;
    });
  });

  describe('when component is loading', () => {
    beforeEach(async () => {
      // Mock pending response
      fetchStub.returns(new Promise(() => {})); // Never resolves
      
      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
    });

    it('should display loading state', () => {
      const versionPanel = el.shadowRoot.querySelector('.version-panel');
      expect(versionPanel).to.exist;
      expect(versionPanel.classList.contains('loading')).to.be.true;
    });

    it('should show loading indicators', () => {
      const values = el.shadowRoot.querySelectorAll('.value');
      values.forEach(value => {
        expect(value.textContent).to.equal('...');
      });
    });
  });

  describe('styling', () => {
    beforeEach(async () => {
      // Mock successful response and manually set data
      const mockResponse = {
        ok: true,
        status: 200,
        statusText: 'OK',
        arrayBuffer: () => Promise.resolve(new ArrayBuffer(0))
      };
      fetchStub.resolves(mockResponse);

      el = await fixture(html`<version-display></version-display>`);
      
      // Manually set version data to simulate successful parsing
      el.version = '1.2.3';
      el.commit = 'abc123';
      el.buildTime = '1/1/2023, 12:00:00 PM';
      el.loading = false;
      el.error = '';
      
      await el.updateComplete;
    });

    it('should have correct font family', () => {
      const styles = getComputedStyle(el);
      expect(styles.fontFamily).to.include('monospace');
    });

    it('should have correct z-index', () => {
      const styles = getComputedStyle(el);
      expect(styles.zIndex).to.equal('1000');
    });

    it('should have version panel with correct styling', () => {
      const versionPanel = el.shadowRoot.querySelector('.version-panel');
      const styles = getComputedStyle(versionPanel);
      expect(styles.borderRadius).to.equal('3px');
      expect(styles.backdropFilter).to.equal('blur(3px)');
    });

    it('should have version items with correct layout', () => {
      const versionInfo = el.shadowRoot.querySelector('.version-info');
      const styles = getComputedStyle(versionInfo);
      expect(styles.display).to.equal('flex');
      expect(styles.gap).to.equal('12px');
    });
  });

  describe('real-world integration', () => {
    beforeEach(async () => {
      // Mock successful response and manually set realistic data
      const mockResponse = {
        ok: true,
        status: 200,
        statusText: 'OK',
        arrayBuffer: () => Promise.resolve(new ArrayBuffer(0))
      };
      fetchStub.resolves(mockResponse);

      el = await fixture(html`<version-display></version-display>`);
      
      // Manually set version data to simulate successful parsing
      el.version = 'v2.1.0';
      el.commit = 'f4b3a2c1';
      el.buildTime = '12/1/2023, 10:30:00 AM';
      el.loading = false;
      el.error = '';
      
      await el.updateComplete;
    });

    it('should display complete version information', () => {
      const versionPanel = el.shadowRoot.querySelector('.version-panel');
      const text = versionPanel.textContent;
      
      expect(text).to.include('v2.1.0');
      expect(text).to.include('f4b3a2c1');
      expect(text).to.include('12/1/2023');
    });

    it('should have all expected labels', () => {
      const labels = el.shadowRoot.querySelectorAll('.label');
      expect(labels).to.have.length(3);
      expect(labels[0].textContent).to.equal('v');
      expect(labels[1].textContent).to.equal('@');
      expect(labels[2].textContent).to.equal('built');
    });

    it('should have corresponding values', () => {
      const values = el.shadowRoot.querySelectorAll('.value');
      expect(values).to.have.length(3);
      expect(values[0].textContent).to.equal('v2.1.0');
      expect(values[1].textContent).to.equal('f4b3a2c1');
      expect(values[2].textContent).to.include('12/1/2023');
    });
  });
});