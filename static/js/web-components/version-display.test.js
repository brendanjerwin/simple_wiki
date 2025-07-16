import { html, fixture, expect } from '@open-wc/testing';
import { VersionDisplay } from './version-display.js';
import sinon from 'sinon';

describe('VersionDisplay', () => {
  let el;
  let clientStub;

  beforeEach(() => {
    // Mock the Connect client
    clientStub = {
      getVersion: sinon.stub()
    };
    
    // Mock the createClient function
    const mockModule = {
      createClient: sinon.stub().returns(clientStub)
    };
    
    // Patch the import to return our mock
    sinon.stub(VersionDisplay.prototype, 'client').value(clientStub);
  });

  afterEach(() => {
    sinon.restore();
  });

  describe('when component is created', () => {
    beforeEach(async () => {
      // Mock client to prevent immediate execution
      clientStub.getVersion.returns(new Promise(() => {})); // Never resolves
      
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
      // Mock successful response
      const mockResponse = {
        version: '1.2.3',
        commit: 'abc123',
        buildTime: { toDate: () => new Date('2023-01-01T12:00:00Z') }
      };
      clientStub.getVersion.resolves(mockResponse);

      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
      
      // Wait for async operations to complete
      await new Promise(resolve => setTimeout(resolve, 10));
      await el.updateComplete;
    });

    it('should call client.getVersion with correct request', () => {
      expect(clientStub.getVersion).to.have.been.calledOnce;
      // Verify the request is a GetVersionRequest instance
      const request = clientStub.getVersion.firstCall.args[0];
      expect(request.constructor.name).to.equal('GetVersionRequest');
    });

    it('should set version data from response', () => {
      expect(el.version).to.equal('1.2.3');
      expect(el.commit).to.equal('abc123');
      expect(el.buildTime).to.equal('1/1/2023, 12:00:00 PM');
      expect(el.loading).to.be.false;
      expect(el.error).to.equal('');
    });

    it('should display version information', () => {
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
      clientStub.getVersion.rejects(error);

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
      expect(el.shadowRoot.innerHTML.trim()).to.equal('');
    });
  });

  describe('when component has no data and not loading', () => {
    beforeEach(async () => {
      // Mock empty response
      clientStub.getVersion.resolves({
        version: '',
        commit: '',
        buildTime: null
      });

      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
      
      // Wait for async operations to complete
      await new Promise(resolve => setTimeout(resolve, 10));
      await el.updateComplete;
    });

    it('should not display anything when there is no data', () => {
      expect(el.shadowRoot.innerHTML.trim()).to.equal('');
    });
  });

  describe('when component is loading', () => {
    beforeEach(async () => {
      // Mock pending response
      clientStub.getVersion.returns(new Promise(() => {})); // Never resolves
      
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
      // Mock successful response
      const mockResponse = {
        version: '1.2.3',
        commit: 'abc123',
        buildTime: { toDate: () => new Date('2023-01-01T12:00:00Z') }
      };
      clientStub.getVersion.resolves(mockResponse);

      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
      
      // Wait for async operations to complete
      await new Promise(resolve => setTimeout(resolve, 10));
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
      // Mock realistic response
      const mockResponse = {
        version: 'v2.1.0',
        commit: 'f4b3a2c1',
        buildTime: { toDate: () => new Date('2023-12-01T10:30:00Z') }
      };
      clientStub.getVersion.resolves(mockResponse);

      el = await fixture(html`<version-display></version-display>`);
      await el.updateComplete;
      
      // Wait for async operations to complete
      await new Promise(resolve => setTimeout(resolve, 10));
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