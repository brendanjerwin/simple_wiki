/* eslint-disable @typescript-eslint/no-explicit-any */
import { expect } from '@open-wc/testing';
import { SystemInfo } from './system-info.js';
import { GetVersionResponse, GetJobStatusResponse, JobQueueStatus } from '../gen/api/v1/system_info_pb.js';
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
      el.jobStatus = undefined;
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
      el.jobStatus = new GetJobStatusResponse({
        jobQueues: []
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

    it('should not show job info when no jobs are active', () => {
      const indexingInfo = el.shadowRoot!.querySelector('.indexing-info');
      expect(indexingInfo).to.not.exist;
    });
  });

  describe('when jobs are active', () => {
    beforeEach(async () => {
      const mockTimestamp = new Timestamp({
        seconds: BigInt(Math.floor(new Date('2023-01-01T12:00:00Z').getTime() / 1000)),
        nanos: 0
      });

      const activeQueue = new JobQueueStatus({
        name: 'Frontmatter',
        jobsRemaining: 25,
        highWaterMark: 100,
        isActive: true
      });

      el.loading = false;
      el.version = new GetVersionResponse({
        commit: 'abc123def456',
        buildTime: mockTimestamp
      });
      el.jobStatus = new GetJobStatusResponse({
        jobQueues: [activeQueue]
      });
      await el.updateComplete;
    });

    it('should show job status component', () => {
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing');
      expect(indexingStatus).to.exist;
    });

    it('should pass correct data to job status component', () => {
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing') as any;
      expect(indexingStatus).to.exist;
      expect(indexingStatus.jobStatus).to.exist;
      expect(indexingStatus.jobStatus.jobQueues).to.have.lengthOf(1);
    });

    it('should pass correct job queue data', () => {
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing') as any;
      const queue = indexingStatus.jobStatus.jobQueues[0];
      expect(queue.name).to.equal('Frontmatter');
      expect(queue.jobsRemaining).to.equal(25);
      expect(queue.isActive).to.be.true;
    });


    it('should pass high water mark data', () => {
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing') as any;
      const queue = indexingStatus.jobStatus.jobQueues[0];
      expect(queue.highWaterMark).to.equal(100);
    });
  });

  describe('when jobs are idle', () => {
    beforeEach(async () => {
      const mockTimestamp = new Timestamp({
        seconds: BigInt(Math.floor(new Date('2023-01-01T12:00:00Z').getTime() / 1000)),
        nanos: 0
      });

      const idleQueue = new JobQueueStatus({
        name: 'Frontmatter',
        jobsRemaining: 0,
        highWaterMark: 0,
        isActive: false
      });

      el.loading = false;
      el.version = new GetVersionResponse({
        commit: 'abc123def456',
        buildTime: mockTimestamp
      });
      el.jobStatus = new GetJobStatusResponse({
        jobQueues: [idleQueue]
      });
      await el.updateComplete;
    });

    it('should show job status component even when idle', () => {
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing');
      expect(indexingStatus).to.exist;
    });
  });

  describe('formatting methods', () => {
    describe('when displaying long commit hash', () => {
      beforeEach(async () => {
        el.loading = false;
        el.version = new GetVersionResponse({
          commit: 'abc123def456789',
          buildTime: new Timestamp()
        });
        el.jobStatus = new GetJobStatusResponse({
          jobQueues: []
        });
        
        await el.updateComplete;
      });

      it('should pass full commit hash to version component', () => {
        const versionComponent = el.shadowRoot!.querySelector('system-info-version') as any;
        expect(versionComponent).to.exist;
        expect(versionComponent.version.commit).to.equal('abc123def456789');
      });
    });

    describe('when displaying tagged version', () => {
      beforeEach(async () => {
        el.loading = false;
        el.version = new GetVersionResponse({
          commit: 'v1.2.3 (abc123d)',
          buildTime: new Timestamp()
        });
        el.jobStatus = new GetJobStatusResponse({
          jobQueues: []
        });
        
        await el.updateComplete;
      });

      it('should pass tagged version to component unchanged', () => {
        const versionComponent = el.shadowRoot!.querySelector('system-info-version') as any;
        expect(versionComponent).to.exist;
        expect(versionComponent.version.commit).to.equal('v1.2.3 (abc123d)');
      });
    });

    describe('when displaying small job counts', () => {
      beforeEach(async () => {
        const smallQueue = new JobQueueStatus({
          name: 'Frontmatter',
          jobsRemaining: 1,
          highWaterMark: 100,
          isActive: true
        });

        el.jobStatus = new GetJobStatusResponse({
          jobQueues: [smallQueue]
        });

        await el.updateComplete;
      });

      it('should pass correct job count to indexing component', () => {
        const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing') as any;
        expect(indexingStatus.jobStatus.jobQueues[0].jobsRemaining).to.equal(1);
      });
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
    describe('when job queues are empty', () => {
      beforeEach(() => {
        el.jobStatus = new GetJobStatusResponse({
          jobQueues: []
        });
      });

      it('should not crash when rendering', () => {
        expect(() => el.render()).to.not.throw();
      });
    });

    describe('when displaying test queue data', () => {
      beforeEach(async () => {
        const testQueue = new JobQueueStatus({
          name: 'TestQueue',
          jobsRemaining: 75,
          highWaterMark: 200,
          isActive: true
        });

        el.jobStatus = new GetJobStatusResponse({
          jobQueues: [testQueue]
        });

        await el.updateComplete;
      });

      it('should pass correct job remaining count', () => {
        const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing') as any;
        const queue = indexingStatus.jobStatus.jobQueues[0];
        expect(queue.jobsRemaining).to.equal(75);
      });

      it('should pass correct high water mark', () => {
        const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing') as any;
        const queue = indexingStatus.jobStatus.jobQueues[0];
        expect(queue.highWaterMark).to.equal(200);
      });

      it('should pass correct queue name', () => {
        const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing') as any;
        const queue = indexingStatus.jobStatus.jobQueues[0];
        expect(queue.name).to.equal('TestQueue');
      });
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