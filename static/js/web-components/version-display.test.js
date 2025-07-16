import { html, fixture, expect } from '@open-wc/testing';
import { VersionDisplay } from './version-display.js';
import sinon from 'sinon';

describe('VersionDisplay', () => {
  let el;
  let fetchStub;

  beforeEach(() => {
    // Mock fetch globally
    fetchStub = sinon.stub(window, 'fetch');
    
    // Default successful response
    const mockResponse = {
      ok: true,
      arrayBuffer: () => Promise.resolve(new ArrayBuffer(20)) // Mock binary response
    };
    fetchStub.resolves(mockResponse);
  });

  afterEach(() => {
    fetchStub.restore();
  });

  describe('when component is created', () => {
    beforeEach(async () => {
      // Mock fetch to prevent immediate execution
      const pendingPromise = new Promise(() => {}); // Never resolves
      fetchStub.returns(pendingPromise);
      
      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
    });

    it('should exist', () => {
      expect(el).to.be.instanceOf(VersionDisplay);
    });

    it('should have initial loading state', () => {
      expect(el.loading).to.be.true; // Should be loading when created
      expect(el.version).to.equal('');
      expect(el.commit).to.equal('');
      expect(el.buildTime).to.equal('');
      expect(el.error).to.equal('');
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
      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
      
      // Wait for async operations to complete
      await new Promise(resolve => setTimeout(resolve, 10));
      await el.updateComplete;
    });

    it('should make gRPC-web request to correct endpoint', () => {
      expect(fetchStub).to.have.been.calledWith(
        `${window.location.origin}/api.v1.Version/GetVersion`,
        sinon.match({
          method: 'POST',
          headers: {
            'Content-Type': 'application/grpc-web+proto',
            'Accept': 'application/grpc-web+proto',
          }
        })
      );
    });

    it('should display decoded version information', () => {
      expect(el.version).to.equal('dev');
      expect(el.commit).to.equal('local-dev');
      expect(el.buildTime).to.be.a('string');
      expect(el.loading).to.be.false;
      expect(el.error).to.equal(''); // No error when successful
    });

    it('should render version information in horizontal layout', () => {
      const panel = el.shadowRoot.querySelector('.version-panel');
      expect(panel).to.exist;
      expect(panel.classList.contains('loading')).to.be.false;
      
      const versionInfo = panel.querySelector('.version-info');
      expect(versionInfo).to.exist;
      
      const values = versionInfo.querySelectorAll('.value');
      expect(values).to.have.length(3);
      expect(values[0].textContent).to.equal('dev');
      expect(values[1].textContent).to.equal('local-dev');
      expect(values[2].textContent).to.not.equal('...');
    });

    it('should display proper labels in horizontal layout', () => {
      const labels = el.shadowRoot.querySelectorAll('.label');
      expect(labels).to.have.length(3);
      expect(labels[0].textContent).to.equal('v');
      expect(labels[1].textContent).to.equal('@');
      expect(labels[2].textContent).to.equal('built');
    });
  });

  describe('when fetch fails', () => {
    beforeEach(async () => {
      fetchStub.rejects(new Error('Network error'));
      
      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
      
      // Wait for async operations to complete
      await new Promise(resolve => setTimeout(resolve, 10));
      await el.updateComplete;
    });

    it('should be blank when fetch fails', () => {
      expect(el.version).to.equal('');
      expect(el.commit).to.equal('');
      expect(el.buildTime).to.equal('');
      expect(el.error).to.equal('Network error');
      expect(el.loading).to.be.false;
    });

    it('should not render anything when error occurs', () => {
      const panel = el.shadowRoot.querySelector('.version-panel');
      expect(panel).to.not.exist;
    });
  });

  describe('when HTTP response is not ok', () => {
    beforeEach(async () => {
      fetchStub.resolves({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error'
      });
      
      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
      
      // Wait for async operations to complete
      await new Promise(resolve => setTimeout(resolve, 10));
      await el.updateComplete;
    });

    it('should be blank when HTTP response is not ok', () => {
      expect(el.version).to.equal('');
      expect(el.commit).to.equal('');
      expect(el.buildTime).to.equal('');
      expect(el.error).to.contain('HTTP 500');
      expect(el.loading).to.be.false;
    });
  });

  describe('when loading', () => {
    beforeEach(async () => {
      // Mock fetch to return a pending promise to simulate loading
      const pendingPromise = new Promise(() => {}); // Never resolves
      fetchStub.returns(pendingPromise);
      
      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
    });

    it('should display loading panel', () => {
      const panel = el.shadowRoot.querySelector('.version-panel');
      expect(panel).to.exist;
      expect(panel.classList.contains('loading')).to.be.true;
    });

    it('should display loading text', () => {
      const values = el.shadowRoot.querySelectorAll('.value');
      expect(values).to.have.length(3);
      expect(values[0].textContent).to.equal('...');
      expect(values[1].textContent).to.equal('...');
      expect(values[2].textContent).to.equal('...');
    });
  });

  describe('gRPC-web message encoding/decoding', () => {
    beforeEach(async () => {
      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
    });

    it('should encode empty message correctly', () => {
      const encoded = el.encodeGrpcWebMessage({});
      expect(encoded).to.be.instanceOf(Uint8Array);
      expect(encoded.length).to.equal(5); // gRPC-web frame header
    });

    it('should decode message and return mock data', () => {
      const buffer = new ArrayBuffer(20);
      const decoded = el.decodeGrpcWebMessage(buffer);
      
      expect(decoded).to.have.property('version');
      expect(decoded).to.have.property('commit');
      expect(decoded).to.have.property('buildTime');
      expect(decoded.version).to.equal('dev');
      expect(decoded.commit).to.equal('local-dev');
    });
  });

  describe('styling', () => {
    beforeEach(async () => {
      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
    });

    it('should have more transparent background', () => {
      const panel = el.shadowRoot.querySelector('.version-panel');
      const styles = getComputedStyle(panel);
      expect(styles.backgroundColor).to.equal('rgba(0, 0, 0, 0.2)');
    });

    it('should have monospace font', () => {
      const styles = getComputedStyle(el);
      expect(styles.fontFamily).to.contain('monospace');
    });

    it('should have high z-index', () => {
      const styles = getComputedStyle(el);
      expect(styles.zIndex).to.equal('1000');
    });
  });
});