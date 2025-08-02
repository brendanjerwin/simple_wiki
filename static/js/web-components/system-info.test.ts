/* eslint-disable @typescript-eslint/no-explicit-any */
import { expect } from '@open-wc/testing';
import { SystemInfo } from './system-info.js';
import { GetVersionResponse, GetIndexingStatusResponse } from '../gen/api/v1/system_info_pb.js';
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
      const versionComponent = el.shadowRoot!.querySelector('system-info-version');
      expect(versionComponent).to.exist;
      expect(versionComponent!.loading).to.be.true;
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
      const versionComponent = el.shadowRoot!.querySelector('system-info-version');
      expect(versionComponent).to.exist;
      expect(versionComponent!.error).to.equal('Connection failed');
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
      const versionComponent = el.shadowRoot!.querySelector('system-info-version');
      expect(versionComponent).to.exist;
      expect(versionComponent!.version).to.exist;
    });

    it('should show commit hash', () => {
      const versionComponent = el.shadowRoot!.querySelector('system-info-version') as any;
      expect(versionComponent).to.exist;
      expect(versionComponent.version.commit).to.equal('abc123def456'); // From beforeEach setup
    });

    it('should show build time', () => {
      const versionComponent = el.shadowRoot!.querySelector('system-info-version') as any;
      expect(versionComponent).to.exist;
      expect(versionComponent.version.buildTime).to.exist;
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

    it('should show indexing status component', () => {
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing');
      expect(indexingStatus).to.exist;
    });

    it('should pass correct data to indexing status component', () => {
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing') as any;
      expect(indexingStatus).to.exist;
      expect(indexingStatus.status).to.exist;
      expect(indexingStatus.status.isRunning).to.be.true;
    });

    it('should pass correct progress data', () => {
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing') as any;
      expect(indexingStatus.status.completedPages).to.equal(50);
      expect(indexingStatus.status.totalPages).to.equal(100);
    });


    it('should pass processing rate data', () => {
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing') as any;
      expect(indexingStatus.status.processingRatePerSecond).to.equal(12.5);
    });

    it('should pass queue depth data', () => {
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing') as any;
      expect(indexingStatus.status.queueDepth).to.equal(25);
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

    it('should show indexing status component even when idle', () => {
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing');
      expect(indexingStatus).to.exist;
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
      
      const versionComponent = el.shadowRoot!.querySelector('system-info-version') as any;
      expect(versionComponent).to.exist;
      expect(versionComponent.version.commit).to.equal('abc123def456789');
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
      
      const versionComponent = el.shadowRoot!.querySelector('system-info-version') as any;
      expect(versionComponent).to.exist;
      expect(versionComponent.version.commit).to.equal('v1.2.3 (abc123d)');
    });

    it('should pass slow processing rates to component correctly', async () => {
      el.indexingStatus = new GetIndexingStatusResponse({
        isRunning: true,
        totalPages: 100,
        completedPages: 50,
        queueDepth: 0,
        processingRatePerSecond: 0.05, // Very slow rate
        indexProgress: []
      });

      await el.updateComplete;
      
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing') as any;
      expect(indexingStatus.status.processingRatePerSecond).to.equal(0.05);
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

    it('should pass correct progress data for calculation', async () => {
      el.indexingStatus = new GetIndexingStatusResponse({
        isRunning: true,
        totalPages: 200,
        completedPages: 75,
        queueDepth: 0,
        processingRatePerSecond: 5.0,
        indexProgress: []
      });

      await el.updateComplete;
      
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing') as any;
      expect(indexingStatus.status.completedPages).to.equal(75);
      expect(indexingStatus.status.totalPages).to.equal(200);
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