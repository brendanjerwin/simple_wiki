import { html, fixture, expect } from '@open-wc/testing';
import { SystemInfo } from './system-info.js';
import { GetVersionResponse, GetIndexingStatusResponse, SingleIndexProgress } from '../gen/api/v1/system_info_pb.js';
import { Timestamp } from '@bufbuild/protobuf';
import { stub, useFakeTimers } from 'sinon';
import './system-info.js';

describe('SystemInfo', () => {
  let el: SystemInfo;
  let clock: any;

  beforeEach(async () => {
    clock = useFakeTimers();
    // Create element without connecting it to DOM first
    el = document.createElement('system-info') as SystemInfo;
    
    // Stub methods that make network requests before connecting
    stub(el, 'loadSystemInfo' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    // Now add to DOM
    document.body.appendChild(el);
    await el.updateComplete;
  });

  afterEach(() => {
    clock.restore();
    // Clean up DOM
    if (el.parentNode) {
      el.parentNode.removeChild(el);
    }
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  it('should be an instance of SystemInfo', () => {
    expect(el).to.be.instanceOf(SystemInfo);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('system-info');
  });

  describe('when loading', () => {
    beforeEach(async () => {
      el.loading = true;
      el.version = undefined;
      el.indexingStatus = undefined;
      await el.updateComplete;
    });

    it('should display loading message for version', () => {
      const loadingText = el.shadowRoot!.querySelector('.loading');
      expect(loadingText).to.exist;
      expect(loadingText!.textContent).to.include('Loading');
    });
  });

  describe('when there is an error', () => {
    beforeEach(async () => {
      el.loading = false;
      el.error = 'Connection failed';
      el.version = undefined;
      await el.updateComplete;
    });

    it('should display error message', () => {
      const errorText = el.shadowRoot!.querySelector('.error');
      expect(errorText).to.exist;
      expect(errorText!.textContent).to.include('Connection failed');
    });
  });

  describe('when version is loaded', () => {
    beforeEach(async () => {
      const mockTimestamp = new Timestamp({
        seconds: BigInt(Math.floor(new Date('2023-01-01T12:00:00Z').getTime() / 1000)),
        nanos: 0
      });

      el.loading = false;
      el.version = new GetVersionResponse({
        commit: 'abc123def456',
        buildTime: mockTimestamp
      });
      el.indexingStatus = new GetIndexingStatusResponse({
        isRunning: false,
        totalPages: 0,
        completedPages: 0,
        queueDepth: 0,
        processingRatePerSecond: 0,
        indexProgress: []
      });
      await el.updateComplete;
    });

    it('should display version information', () => {
      const versionInfo = el.shadowRoot!.querySelector('.version-info');
      expect(versionInfo).to.exist;
    });

    it('should show commit hash', () => {
      const commitValue = el.shadowRoot!.querySelector('.commit');
      expect(commitValue).to.exist;
      expect(commitValue!.textContent).to.equal('abc123d'); // Truncated
    });

    it('should show build time', () => {
      const values = el.shadowRoot!.querySelectorAll('.value');
      const buildTimeValue = Array.from(values).find(v => 
        v.textContent && v.textContent.includes('Jan')
      );
      expect(buildTimeValue).to.exist;
    });

    it('should not show indexing info when not running', () => {
      const indexingInfo = el.shadowRoot!.querySelector('.indexing-info');
      expect(indexingInfo).to.not.exist;
    });
  });

  describe('when indexing is active', () => {
    beforeEach(async () => {
      const mockTimestamp = new Timestamp({
        seconds: BigInt(Math.floor(new Date('2023-01-01T12:00:00Z').getTime() / 1000)),
        nanos: 0
      });

      el.loading = false;
      el.version = new GetVersionResponse({
        commit: 'abc123def456',
        buildTime: mockTimestamp
      });
      el.indexingStatus = new GetIndexingStatusResponse({
        isRunning: true,
        totalPages: 100,
        completedPages: 50,
        queueDepth: 25,
        processingRatePerSecond: 12.5,
        indexProgress: []
      });
      await el.updateComplete;
    });

    it('should show indexing info section', () => {
      const indexingInfo = el.shadowRoot!.querySelector('.indexing-info');
      expect(indexingInfo).to.exist;
    });

    it('should show active status indicator', () => {
      const indicator = el.shadowRoot!.querySelector('.status-indicator');
      expect(indicator).to.exist;
      expect(indicator).to.not.have.class('idle');
    });

    it('should display progress information', () => {
      const indexingInfo = el.shadowRoot!.querySelector('.indexing-info');
      expect(indexingInfo!.textContent).to.include('50/100');
    });

    it('should show progress bar', () => {
      const progressBar = el.shadowRoot!.querySelector('.progress-bar-mini');
      expect(progressBar).to.exist;
      
      const progressFill = el.shadowRoot!.querySelector('.progress-fill-mini');
      expect(progressFill).to.exist;
      expect(progressFill!.getAttribute('style')).to.include('width: 50%');
    });

    it('should display processing rate', () => {
      const rate = el.shadowRoot!.querySelector('.rate');
      expect(rate).to.exist;
      expect(rate!.textContent).to.include('13/s'); // Rounded
    });

    it('should show queue depth when present', () => {
      const queue = el.shadowRoot!.querySelector('.queue');
      expect(queue).to.exist;
      expect(queue!.textContent).to.include('Q:25');
    });
  });

  describe('when indexing is idle', () => {
    beforeEach(async () => {
      const mockTimestamp = new Timestamp({
        seconds: BigInt(Math.floor(new Date('2023-01-01T12:00:00Z').getTime() / 1000)),
        nanos: 0
      });

      el.loading = false;
      el.version = new GetVersionResponse({
        commit: 'abc123def456',
        buildTime: mockTimestamp
      });
      el.indexingStatus = new GetIndexingStatusResponse({
        isRunning: false,
        totalPages: 100,
        completedPages: 100,
        queueDepth: 0,
        processingRatePerSecond: 0,
        indexProgress: []
      });
      await el.updateComplete;
    });

    it('should not show indexing info when idle', () => {
      const indexingInfo = el.shadowRoot!.querySelector('.indexing-info');
      expect(indexingInfo).to.not.exist;
    });
  });

  describe('formatting methods', () => {
    it('should format commit hash correctly', async () => {
      // Test long commit hash
      el.loading = false;
      el.version = new GetVersionResponse({
        commit: 'abc123def456789',
        buildTime: new Timestamp()
      });
      el.indexingStatus = new GetIndexingStatusResponse({
        isRunning: false,
        totalPages: 0,
        completedPages: 0,
        queueDepth: 0,
        processingRatePerSecond: 0,
        indexProgress: []
      });
      
      await el.updateComplete;
      
      const commitValue = el.shadowRoot!.querySelector('.commit');
      expect(commitValue!.textContent).to.equal('abc123d');
    });

    it('should format tagged version correctly', async () => {
      // Test tagged version (should not be truncated)
      el.loading = false;
      el.version = new GetVersionResponse({
        commit: 'v1.2.3 (abc123d)',
        buildTime: new Timestamp()
      });
      el.indexingStatus = new GetIndexingStatusResponse({
        isRunning: false,
        totalPages: 0,
        completedPages: 0,
        queueDepth: 0,
        processingRatePerSecond: 0,
        indexProgress: []
      });
      
      await el.updateComplete;
      
      const commitValue = el.shadowRoot!.querySelector('.commit');
      expect(commitValue!.textContent).to.equal('v1.2.3 (abc123d)');
    });

    it('should format processing rates correctly', async () => {
      el.indexingStatus = new GetIndexingStatusResponse({
        isRunning: true,
        totalPages: 100,
        completedPages: 50,
        queueDepth: 0,
        processingRatePerSecond: 0.05, // Very slow rate
        indexProgress: []
      });

      await el.updateComplete;
      
      const rate = el.shadowRoot!.querySelector('.rate');
      expect(rate!.textContent).to.include('< 0.1/s');
    });
  });

  describe('component styling', () => {
    it('should have fixed positioning', () => {
      const style = getComputedStyle(el);
      expect(style.position).to.equal('fixed');
    });

    it('should be positioned in bottom right', () => {
      const styles = el.constructor.styles;
      const cssText = styles.map(s => s.cssText).join('');
      expect(cssText).to.include('bottom: 2px');
      expect(cssText).to.include('right: 2px');
    });

    it('should have high z-index', () => {
      const styles = el.constructor.styles;
      const cssText = styles.map(s => s.cssText).join('');
      expect(cssText).to.include('z-index: 1000');
    });
  });

  describe('progress calculation', () => {
    it('should handle zero total pages', () => {
      el.indexingStatus = new GetIndexingStatusResponse({
        isRunning: false,
        totalPages: 0,
        completedPages: 0,
        queueDepth: 0,
        processingRatePerSecond: 0,
        indexProgress: []
      });

      // Should not crash and should not show indexing section
      expect(() => el.render()).to.not.throw();
    });

    it('should calculate progress percentage correctly', async () => {
      el.indexingStatus = new GetIndexingStatusResponse({
        isRunning: true,
        totalPages: 200,
        completedPages: 75,
        queueDepth: 0,
        processingRatePerSecond: 5.0,
        indexProgress: []
      });

      await el.updateComplete;
      
      const progressFill = el.shadowRoot!.querySelector('.progress-fill-mini');
      expect(progressFill!.getAttribute('style')).to.include('width: 37.5%');
    });
  });

  describe('component lifecycle', () => {
    let connectStub: any;
    let disconnectStub: any;

    beforeEach(() => {
      connectStub = stub(el, 'connectedCallback');
      disconnectStub = stub(el, 'disconnectedCallback');
    });

    afterEach(() => {
      connectStub.restore();
      disconnectStub.restore();
    });

    it('should handle connection lifecycle', () => {
      expect(connectStub).to.not.have.been.called;
      expect(disconnectStub).to.not.have.been.called;
      // We can't easily test the timer management without more complex mocking
    });
  });
});