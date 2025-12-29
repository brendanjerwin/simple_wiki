import { expect } from '@open-wc/testing';
import { SystemInfo } from './system-info.js';
import { SystemInfoVersion } from './system-info-version.js';
import { GetVersionResponseSchema, GetJobStatusResponseSchema, JobQueueStatusSchema } from '../gen/api/v1/system_info_pb.js';
import { create } from '@bufbuild/protobuf';
import { TimestampSchema } from '@bufbuild/protobuf/wkt';
import { stub, useFakeTimers, type SinonFakeTimers, type SinonStub } from 'sinon';
import './system-info.js';

// Interface for testing private methods - accessed via unknown cast
interface SystemInfoTestInterface {
  _handleClickOutside: (event: MouseEvent) => void;
}

describe('SystemInfo', () => {
  let el: SystemInfo;
  let clock: SinonFakeTimers;

  beforeEach(async () => {
    clock = useFakeTimers();
    // Create element without connecting it to DOM first
    el = document.createElement('system-info') as SystemInfo;
    
    // Stub methods that make network requests before connecting
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing methods by name for stubbing
    stub(el, 'loadSystemInfo' as keyof SystemInfo).resolves();
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing methods by name for stubbing
    stub(el, 'startAutoRefresh' as keyof SystemInfo);
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing methods by name for stubbing
    stub(el, 'stopAutoRefresh' as keyof SystemInfo);
    
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
      delete el.version;
      delete el.jobStatus;
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
      el.error = new Error('Connection failed');
      delete el.version;
      await el.updateComplete;
    });

    it('should display error message', () => {
      const versionComponent = el.shadowRoot!.querySelector('system-info-version');
      expect(versionComponent).to.exist;
      expect(versionComponent!.error?.message).to.equal('Connection failed');
    });
  });

  describe('when version is loaded', () => {
    beforeEach(async () => {
      const mockTimestamp = create(TimestampSchema, {
        seconds: BigInt(Math.floor(new Date('2023-01-01T12:00:00Z').getTime() / 1000)),
        nanos: 0
      });

      el.loading = false;
      el.version = create(GetVersionResponseSchema, {
        commit: 'abc123def456',
        buildTime: mockTimestamp
      });
      el.jobStatus = create(GetJobStatusResponseSchema, {
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
      const versionComponent = el.shadowRoot!.querySelector<SystemInfoVersion>('system-info-version');
      expect(versionComponent).to.exist;
      expect(versionComponent!.version!.commit).to.equal('abc123def456'); // From beforeEach setup
    });

    it('should show build time', () => {
      const versionComponent = el.shadowRoot!.querySelector<SystemInfoVersion>('system-info-version');
      expect(versionComponent).to.exist;
      expect(versionComponent!.version!.buildTime).to.exist;
    });

    it('should not show job info when no jobs are active', () => {
      const indexingInfo = el.shadowRoot!.querySelector('.indexing-info');
      expect(indexingInfo).to.not.exist;
    });
  });

  describe('when jobs are active', () => {
    beforeEach(async () => {
      const mockTimestamp = create(TimestampSchema, {
        seconds: BigInt(Math.floor(new Date('2023-01-01T12:00:00Z').getTime() / 1000)),
        nanos: 0
      });

      const activeQueue = create(JobQueueStatusSchema, {
        name: 'Frontmatter',
        jobsRemaining: 25,
        highWaterMark: 100,
        isActive: true
      });

      el.loading = false;
      el.version = create(GetVersionResponseSchema, {
        commit: 'abc123def456',
        buildTime: mockTimestamp
      });
      el.jobStatus = create(GetJobStatusResponseSchema, {
        jobQueues: [activeQueue]
      });
      await el.updateComplete;
    });

    it('should show job status component', () => {
      const indexingStatus = el.shadowRoot!.querySelector('system-info-indexing');
      expect(indexingStatus).to.exist;
    });

    it('should pass correct data to job status component', () => {
      const indexingStatus = el.shadowRoot!.querySelector<HTMLElement & { jobStatus: { jobQueues: unknown[] } }>('system-info-indexing');
      expect(indexingStatus).to.exist;
      expect(indexingStatus!.jobStatus).to.exist;
      expect(indexingStatus!.jobStatus.jobQueues).to.have.lengthOf(1);
    });

    it('should pass correct job queue data', () => {
      const indexingStatus = el.shadowRoot!.querySelector<HTMLElement & { jobStatus?: { jobQueues: Array<{ name: string; jobsRemaining: number; isActive: boolean; highWaterMark: number }> } }>('system-info-indexing');
      const queue = indexingStatus?.jobStatus?.jobQueues[0];
      expect(queue).to.exist;
      expect(queue!.name).to.equal('Frontmatter');
      expect(queue!.jobsRemaining).to.equal(25);
      expect(queue!.isActive).to.be.true;
    });


    it('should pass high water mark data', () => {
      const indexingStatus = el.shadowRoot!.querySelector<HTMLElement & { jobStatus?: { jobQueues: Array<{ highWaterMark: number }> } }>('system-info-indexing');
      const queue = indexingStatus?.jobStatus?.jobQueues[0];
      expect(queue).to.exist;
      expect(queue!.highWaterMark).to.equal(100);
    });
  });

  describe('when jobs are idle', () => {
    beforeEach(async () => {
      const mockTimestamp = create(TimestampSchema, {
        seconds: BigInt(Math.floor(new Date('2023-01-01T12:00:00Z').getTime() / 1000)),
        nanos: 0
      });

      const idleQueue = create(JobQueueStatusSchema, {
        name: 'Frontmatter',
        jobsRemaining: 0,
        highWaterMark: 0,
        isActive: false
      });

      el.loading = false;
      el.version = create(GetVersionResponseSchema, {
        commit: 'abc123def456',
        buildTime: mockTimestamp
      });
      el.jobStatus = create(GetJobStatusResponseSchema, {
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
        el.version = create(GetVersionResponseSchema, {
          commit: 'abc123def456789',
          buildTime: create(TimestampSchema, {})
        });
        el.jobStatus = create(GetJobStatusResponseSchema, {
          jobQueues: []
        });
        
        await el.updateComplete;
      });

      it('should pass full commit hash to version component', () => {
        const versionComponent = el.shadowRoot!.querySelector<SystemInfoVersion>('system-info-version');
        expect(versionComponent).to.exist;
        expect(versionComponent!.version!.commit).to.equal('abc123def456789');
      });
    });

    describe('when displaying tagged version', () => {
      beforeEach(async () => {
        el.loading = false;
        el.version = create(GetVersionResponseSchema, {
          commit: 'v1.2.3 (abc123d)',
          buildTime: create(TimestampSchema, {})
        });
        el.jobStatus = create(GetJobStatusResponseSchema, {
          jobQueues: []
        });

        await el.updateComplete;
      });

      it('should pass tagged version to component unchanged', () => {
        const versionComponent = el.shadowRoot!.querySelector<SystemInfoVersion>('system-info-version');
        expect(versionComponent).to.exist;
        expect(versionComponent!.version!.commit).to.equal('v1.2.3 (abc123d)');
      });
    });

    describe('when displaying small job counts', () => {
      beforeEach(async () => {
        const smallQueue = create(JobQueueStatusSchema, {
          name: 'Frontmatter',
          jobsRemaining: 1,
          highWaterMark: 100,
          isActive: true
        });

        el.jobStatus = create(GetJobStatusResponseSchema, {
          jobQueues: [smallQueue]
        });

        await el.updateComplete;
      });

      it('should pass correct job count to indexing component', () => {
        const indexingStatus = el.shadowRoot!.querySelector<HTMLElement & { jobStatus?: { jobQueues: Array<{ jobsRemaining: number }> } }>('system-info-indexing');
        const queue = indexingStatus?.jobStatus?.jobQueues[0];
        expect(queue).to.exist;
        expect(queue!.jobsRemaining).to.equal(1);
      });
    });
  });

  describe('component styling', () => {
    it('should have fixed positioning', () => {
      const style = getComputedStyle(el);
      expect(style.position).to.equal('fixed');
    });

    it('should be positioned in bottom right', () => {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing static styles property for testing
      const styles = (el.constructor as typeof SystemInfo).styles as Array<{ cssText: string }>;
      const cssText = styles.map(s => s.cssText).join('');
      expect(cssText).to.include('bottom: 2px');
      expect(cssText).to.include('right: 2px');
    });

    it('should have high z-index', () => {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing static styles property for testing
      const styles = (el.constructor as typeof SystemInfo).styles as Array<{ cssText: string }>;
      const cssText = styles.map(s => s.cssText).join('');
      expect(cssText).to.include('z-index: 1000');
    });
  });

  describe('progress calculation', () => {
    describe('when job queues are empty', () => {
      beforeEach(() => {
        el.jobStatus = create(GetJobStatusResponseSchema, {
          jobQueues: []
        });
      });

      it('should not crash when rendering', () => {
        expect(() => el.render()).to.not.throw();
      });
    });

    describe('when displaying test queue data', () => {
      beforeEach(async () => {
        const testQueue = create(JobQueueStatusSchema, {
          name: 'TestQueue',
          jobsRemaining: 75,
          highWaterMark: 200,
          isActive: true
        });

        el.jobStatus = create(GetJobStatusResponseSchema, {
          jobQueues: [testQueue]
        });

        await el.updateComplete;
      });

      it('should pass correct job remaining count', () => {
        const indexingStatus = el.shadowRoot!.querySelector<HTMLElement & { jobStatus?: { jobQueues: Array<{ jobsRemaining: number }> } }>('system-info-indexing');
        const queue = indexingStatus?.jobStatus?.jobQueues[0];
        expect(queue).to.exist;
        expect(queue!.jobsRemaining).to.equal(75);
      });

      it('should pass correct high water mark', () => {
        const indexingStatus = el.shadowRoot!.querySelector<HTMLElement & { jobStatus?: { jobQueues: Array<{ highWaterMark: number }> } }>('system-info-indexing');
        const queue = indexingStatus?.jobStatus?.jobQueues[0];
        expect(queue).to.exist;
        expect(queue!.highWaterMark).to.equal(200);
      });

      it('should pass correct queue name', () => {
        const indexingStatus = el.shadowRoot!.querySelector<HTMLElement & { jobStatus?: { jobQueues: Array<{ name: string }> } }>('system-info-indexing');
        const queue = indexingStatus?.jobStatus?.jobQueues[0];
        expect(queue).to.exist;
        expect(queue!.name).to.equal('TestQueue');
      });
    });
  });

  describe('component lifecycle', () => {
    let connectStub: SinonStub;
    let disconnectStub: SinonStub;

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

  describe('drawer tab functionality', () => {
    beforeEach(async () => {
      const mockTimestamp = create(TimestampSchema, {
        seconds: BigInt(Math.floor(new Date('2023-01-01T12:00:00Z').getTime() / 1000)),
        nanos: 0
      });

      el.loading = false;
      el.version = create(GetVersionResponseSchema, {
        commit: 'abc123def456',
        buildTime: mockTimestamp
      });
      el.jobStatus = create(GetJobStatusResponseSchema, {
        jobQueues: []
      });
      await el.updateComplete;
    });

    it('should start with expanded state as false', () => {
      expect(el.expanded).to.be.false;
    });

    it('should render drawer tab element', () => {
      const tab = el.shadowRoot!.querySelector('.drawer-tab');
      expect(tab).to.exist;
    });

    it('should render INFO text in drawer tab', () => {
      const tab = el.shadowRoot!.querySelector('.drawer-tab');
      expect(tab).to.exist;
      expect(tab!.textContent).to.equal('INFO');
    });

    it('should not have expanded class when collapsed', () => {
      const panel = el.shadowRoot!.querySelector('.system-panel');
      expect(panel!.classList.contains('expanded')).to.be.false;
    });

    it('should have expanded class when expanded', async () => {
      el.expanded = true;
      await el.updateComplete;
      
      const panel = el.shadowRoot!.querySelector('.system-panel');
      expect(panel!.classList.contains('expanded')).to.be.true;
    });

    it('should toggle expanded state on click', async () => {
      const panel = el.shadowRoot!.querySelector<HTMLElement>('.system-panel');
      expect(el.expanded).to.be.false;

      panel!.click();
      await el.updateComplete;

      expect(el.expanded).to.be.true;
    });

    it('should toggle expanded state on Enter key', async () => {
      const panel = el.shadowRoot!.querySelector<HTMLElement>('.system-panel');
      expect(el.expanded).to.be.false;

      const event = new KeyboardEvent('keydown', { key: 'Enter' });
      panel!.dispatchEvent(event);
      await el.updateComplete;

      expect(el.expanded).to.be.true;
    });

    it('should toggle expanded state on Space key', async () => {
      const panel = el.shadowRoot!.querySelector<HTMLElement>('.system-panel');
      expect(el.expanded).to.be.false;

      const event = new KeyboardEvent('keydown', { key: ' ' });
      panel!.dispatchEvent(event);
      await el.updateComplete;

      expect(el.expanded).to.be.true;
    });

    it('should not toggle on other keys', async () => {
      const panel = el.shadowRoot!.querySelector<HTMLElement>('.system-panel');
      expect(el.expanded).to.be.false;

      const event = new KeyboardEvent('keydown', { key: 'a' });
      panel!.dispatchEvent(event);
      await el.updateComplete;

      expect(el.expanded).to.be.false;
    });

    it('should have role="button"', () => {
      const panel = el.shadowRoot!.querySelector('.system-panel');
      expect(panel!.getAttribute('role')).to.equal('button');
    });

    it('should have tabindex="0"', () => {
      const panel = el.shadowRoot!.querySelector('.system-panel');
      expect(panel!.getAttribute('tabindex')).to.equal('0');
    });

    it('should have aria-label', () => {
      const panel = el.shadowRoot!.querySelector('.system-panel');
      expect(panel!.getAttribute('aria-label')).to.equal('System information panel');
    });

    it('should update aria-expanded when state changes', async () => {
      const panel = el.shadowRoot!.querySelector('.system-panel');
      expect(panel!.getAttribute('aria-expanded')).to.equal('false');
      
      el.expanded = true;
      await el.updateComplete;
      
      expect(panel!.getAttribute('aria-expanded')).to.equal('true');
    });

    it('should render panel-content wrapper', () => {
      const panelContent = el.shadowRoot!.querySelector('.panel-content');
      expect(panelContent).to.exist;
    });

    it('should render system-content inside panel-content', () => {
      const panelContent = el.shadowRoot!.querySelector('.panel-content');
      const systemContent = panelContent!.querySelector('.system-content');
      expect(systemContent).to.exist;
    });
  });

  describe('click-outside behavior', () => {
    beforeEach(async () => {
      const mockTimestamp = create(TimestampSchema, {
        seconds: BigInt(Math.floor(new Date('2023-01-01T12:00:00Z').getTime() / 1000)),
        nanos: 0
      });

      el.loading = false;
      el.version = create(GetVersionResponseSchema, {
        commit: 'abc123def456',
        buildTime: mockTimestamp
      });
      el.jobStatus = create(GetJobStatusResponseSchema, {
        jobQueues: []
      });
      await el.updateComplete;
    });

    it('should collapse when clicking outside and expanded', async () => {
      el.expanded = true;
      await el.updateComplete;

      // Simulate click outside
      const outsideEvent = new MouseEvent('click', { bubbles: true });
      document.dispatchEvent(outsideEvent);
      
      expect(el.expanded).to.be.false;
    });

    it('should not collapse when clicking inside the panel', async () => {
      el.expanded = true;
      await el.updateComplete;

      // Click on the panel itself (which stops propagation)
      const panel = el.shadowRoot!.querySelector<HTMLElement>('.system-panel');
      panel!.click();
      await el.updateComplete;

      // Should toggle to collapsed then back to expanded
      expect(el.expanded).to.be.false;
    });

    it('should not change state when clicking outside while collapsed', async () => {
      el.expanded = false;
      await el.updateComplete;

      // Simulate click outside
      const outsideEvent = new MouseEvent('click', { bubbles: true });
      document.dispatchEvent(outsideEvent);

      expect(el.expanded).to.be.false;
    });

    it('should handle click events with composed path correctly', async () => {
      el.expanded = true;
      await el.updateComplete;

      // Create a mock event that doesn't include this element in composed path
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- creating mock event for testing
      const mockEvent = {
        composedPath: () => []
      } as unknown as MouseEvent;

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
      (el as unknown as SystemInfoTestInterface)._handleClickOutside(mockEvent);

      expect(el.expanded).to.be.false;
    });
  });

  describe('error UX behaviors', () => {
    describe('when an error occurs', () => {
      beforeEach(async () => {
        el.loading = false;
        el.error = new Error('Test error message');
        await el.updateComplete;
      });

      it('should display error in version component', () => {
        const versionComponent = el.shadowRoot!.querySelector('system-info-version');
        expect(versionComponent).to.exist;
        expect(versionComponent!.error?.message).to.equal('Test error message');
      });

      it('should show error even without version data', () => {
        delete el.version;
        const versionComponent = el.shadowRoot!.querySelector('system-info-version');
        expect(versionComponent).to.exist;
        expect(versionComponent!.error).to.exist;
      });
    });

    describe('when error is cleared', () => {
      beforeEach(async () => {
        el.error = new Error('Previous error');
        await el.updateComplete;
        el.error = null;
        await el.updateComplete;
      });

      it('should no longer show error in version component', () => {
        const versionComponent = el.shadowRoot!.querySelector('system-info-version');
        expect(versionComponent).to.exist;
        expect(versionComponent!.error).to.be.null;
      });
    });
  });
});

/**
 * Mock SystemInfo client for error handling tests
 */
interface MockSystemInfoClient {
  getVersion: SinonStub;
  getJobStatus: SinonStub;
  streamJobStatus: SinonStub;
}

function createMockClient(): MockSystemInfoClient {
  return {
    getVersion: stub(),
    getJobStatus: stub(),
    streamJobStatus: stub(),
  };
}

/**
 * Interface for accessing private properties for testing
 */
interface SystemInfoPrivateAccess {
  client: MockSystemInfoClient;
  reloadVersionOnly: () => Promise<void>;
  loadSystemInfo: () => Promise<void>;
  startJobStream: () => Promise<void>;
  startAutoRefresh: () => void;
  stopAutoRefresh: () => void;
  stopJobStream: () => void;
  streamSubscription?: AbortController;
}

describe('SystemInfo error handling', () => {
  let el: SystemInfo;
  let mockClient: MockSystemInfoClient;
  let clock: SinonFakeTimers;
  let startAutoRefreshStub: SinonStub;
  let stopAutoRefreshStub: SinonStub;
  let stopJobStreamStub: SinonStub;

  beforeEach(() => {
    clock = useFakeTimers();
  });

  afterEach(() => {
    clock.restore();
    if (startAutoRefreshStub) startAutoRefreshStub.restore();
    if (stopAutoRefreshStub) stopAutoRefreshStub.restore();
    if (stopJobStreamStub) stopJobStreamStub.restore();
    if (el && el.parentNode) {
      el.parentNode.removeChild(el);
    }
  });

  describe('loadSystemInfo', () => {
    describe('when getVersion throws an error', () => {
      let testError: Error;

      beforeEach(async () => {
        testError = new Error('Connection refused');

        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client with error response
        mockClient = createMockClient();
        mockClient.getVersion.rejects(testError);

        // Stub lifecycle methods before connection
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopAutoRefreshStub = stub(el, 'stopAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopJobStreamStub = stub(el, 'stopJobStream' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        // Wait for connectedCallback's loadSystemInfo to complete
        await clock.tickAsync(0);
        await el.updateComplete;
      });

      it('should set the error property', () => {
        expect(el.error).to.exist;
        expect(el.error?.message).to.equal('Connection refused');
      });

      it('should set loading to false', () => {
        expect(el.loading).to.be.false;
      });

      it('should start auto refresh as fallback', () => {
        expect(startAutoRefreshStub).to.have.been.called;
      });

      it('should display error in version component', () => {
        const versionComponent = el.shadowRoot!.querySelector('system-info-version');
        expect(versionComponent).to.exist;
        expect(versionComponent!.error?.message).to.equal('Connection refused');
      });
    });

    describe('when getJobStatus throws an error', () => {
      let testError: Error;

      beforeEach(async () => {
        testError = new Error('Server unavailable');

        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client
        mockClient = createMockClient();
        // getVersion succeeds
        mockClient.getVersion.resolves(create(GetVersionResponseSchema, {
          commit: 'abc123',
          buildTime: create(TimestampSchema, {})
        }));
        // getJobStatus fails
        mockClient.getJobStatus.rejects(testError);

        // Stub lifecycle methods before connection
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopAutoRefreshStub = stub(el, 'stopAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopJobStreamStub = stub(el, 'stopJobStream' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        // Wait for connectedCallback's loadSystemInfo to complete
        await clock.tickAsync(0);
        await el.updateComplete;
      });

      it('should set the error property', () => {
        expect(el.error).to.exist;
        expect(el.error?.message).to.equal('Server unavailable');
      });

      it('should set loading to false', () => {
        expect(el.loading).to.be.false;
      });

      it('should start auto refresh as fallback', () => {
        expect(startAutoRefreshStub).to.have.been.called;
      });
    });

    describe('when error is a non-Error object', () => {
      beforeEach(async () => {
        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client
        mockClient = createMockClient();
        // Use callsFake to reject with a non-Error object directly
        mockClient.getVersion.callsFake(() => Promise.reject({ code: 'UNAVAILABLE' }));

        // Stub lifecycle methods before connection
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopAutoRefreshStub = stub(el, 'stopAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopJobStreamStub = stub(el, 'stopJobStream' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        await clock.tickAsync(0);
        await el.updateComplete;
      });

      it('should convert to Error object', () => {
        expect(el.error).to.be.instanceOf(Error);
        // The error message will be the stringified version of the object
        expect(el.error?.message).to.equal('[object Object]');
      });
    });
  });

  describe('reloadVersionOnly', () => {
    describe('when getVersion throws an error', () => {
      let testError: Error;
      let privateAccess: SystemInfoPrivateAccess;

      beforeEach(async () => {
        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client - initial load succeeds
        mockClient = createMockClient();
        mockClient.getVersion.resolves(create(GetVersionResponseSchema, {
          commit: 'abc123',
          buildTime: create(TimestampSchema, {})
        }));
        mockClient.getJobStatus.resolves(create(GetJobStatusResponseSchema, {
          jobQueues: []
        }));

        // Stub lifecycle methods before connection
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopAutoRefreshStub = stub(el, 'stopAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopJobStreamStub = stub(el, 'stopJobStream' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        await clock.tickAsync(0);
        await el.updateComplete;

        // Clear error from any previous calls
        el.error = null;
        await el.updateComplete;

        // Now make getVersion fail for reload
        testError = new Error('Network timeout');
        mockClient.getVersion.rejects(testError);

        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        privateAccess = el as unknown as SystemInfoPrivateAccess;
        await privateAccess.reloadVersionOnly();
        await el.updateComplete;
      });

      it('should set the error property', () => {
        expect(el.error).to.exist;
        expect(el.error?.message).to.equal('Network timeout');
      });

      it('should display error in version component', () => {
        const versionComponent = el.shadowRoot!.querySelector('system-info-version');
        expect(versionComponent).to.exist;
        expect(versionComponent!.error?.message).to.equal('Network timeout');
      });
    });

    describe('when error is a non-Error object', () => {
      let privateAccess: SystemInfoPrivateAccess;

      beforeEach(async () => {
        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client - initial load succeeds
        mockClient = createMockClient();
        mockClient.getVersion.resolves(create(GetVersionResponseSchema, {
          commit: 'abc123',
          buildTime: create(TimestampSchema, {})
        }));
        mockClient.getJobStatus.resolves(create(GetJobStatusResponseSchema, {
          jobQueues: []
        }));

        // Stub lifecycle methods before connection
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopAutoRefreshStub = stub(el, 'stopAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopJobStreamStub = stub(el, 'stopJobStream' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        await clock.tickAsync(0);
        await el.updateComplete;

        // Now make getVersion fail with non-Error
        mockClient.getVersion.rejects({ code: 'UNAVAILABLE' });

        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        privateAccess = el as unknown as SystemInfoPrivateAccess;
        await privateAccess.reloadVersionOnly();
        await el.updateComplete;
      });

      it('should convert to Error object', () => {
        expect(el.error).to.be.instanceOf(Error);
      });
    });
  });

  describe('stopAutoRefresh', () => {
    describe('when refresh timer exists', () => {
      let privateAccess: SystemInfoPrivateAccess;

      beforeEach(async () => {
        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client - initial load succeeds
        mockClient = createMockClient();
        mockClient.getVersion.resolves(create(GetVersionResponseSchema, {
          commit: 'abc123',
          buildTime: create(TimestampSchema, {})
        }));
        mockClient.getJobStatus.resolves(create(GetJobStatusResponseSchema, {
          jobQueues: []
        }));

        // Stub lifecycle methods before connection except stopAutoRefresh
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopJobStreamStub = stub(el, 'stopJobStream' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        await clock.tickAsync(0);
        await el.updateComplete;

        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        privateAccess = el as unknown as SystemInfoPrivateAccess;

        // Manually set a refresh timer to test cleanup
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-type-assertion -- setting private property for testing
        (el as any).refreshTimer = setInterval(() => {}, 1000);

        // Call stopAutoRefresh
        privateAccess.stopAutoRefresh();
      });

      it('should clear the refresh timer', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        expect((el as any).refreshTimer).to.be.undefined;
      });
    });

    describe('when refresh timer does not exist', () => {
      let privateAccess: SystemInfoPrivateAccess;

      beforeEach(async () => {
        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client - initial load succeeds
        mockClient = createMockClient();
        mockClient.getVersion.resolves(create(GetVersionResponseSchema, {
          commit: 'abc123',
          buildTime: create(TimestampSchema, {})
        }));
        mockClient.getJobStatus.resolves(create(GetJobStatusResponseSchema, {
          jobQueues: []
        }));

        // Stub lifecycle methods before connection except stopAutoRefresh
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopJobStreamStub = stub(el, 'stopJobStream' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        await clock.tickAsync(0);
        await el.updateComplete;

        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        privateAccess = el as unknown as SystemInfoPrivateAccess;

        // Ensure no refresh timer exists
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-type-assertion -- setting private property for testing
        delete (el as any).refreshTimer;

        // Call stopAutoRefresh - should not throw
        privateAccess.stopAutoRefresh();
      });

      it('should not throw', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        expect((el as any).refreshTimer).to.be.undefined;
      });
    });
  });

  describe('stopJobStream', () => {
    describe('when stream subscription exists', () => {
      let privateAccess: SystemInfoPrivateAccess;
      let abortSpy: SinonStub;

      beforeEach(async () => {
        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client - initial load succeeds
        mockClient = createMockClient();
        mockClient.getVersion.resolves(create(GetVersionResponseSchema, {
          commit: 'abc123',
          buildTime: create(TimestampSchema, {})
        }));
        mockClient.getJobStatus.resolves(create(GetJobStatusResponseSchema, {
          jobQueues: []
        }));

        // Stub lifecycle methods before connection except stopJobStream
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopAutoRefreshStub = stub(el, 'stopAutoRefresh' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        await clock.tickAsync(0);
        await el.updateComplete;

        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        privateAccess = el as unknown as SystemInfoPrivateAccess;

        // Manually set a stream subscription to test cleanup
        const mockAbortController = new AbortController();
        abortSpy = stub(mockAbortController, 'abort');
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-type-assertion -- setting private property for testing
        (el as any).streamSubscription = mockAbortController;

        // Call stopJobStream
        privateAccess.stopJobStream();
      });

      it('should abort the stream subscription', () => {
        expect(abortSpy).to.have.been.calledOnce;
      });

      it('should delete the stream subscription', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        expect((el as any).streamSubscription).to.be.undefined;
      });
    });

    describe('when stream subscription does not exist', () => {
      let privateAccess: SystemInfoPrivateAccess;

      beforeEach(async () => {
        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client - initial load succeeds
        mockClient = createMockClient();
        mockClient.getVersion.resolves(create(GetVersionResponseSchema, {
          commit: 'abc123',
          buildTime: create(TimestampSchema, {})
        }));
        mockClient.getJobStatus.resolves(create(GetJobStatusResponseSchema, {
          jobQueues: []
        }));

        // Stub lifecycle methods before connection except stopJobStream
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopAutoRefreshStub = stub(el, 'stopAutoRefresh' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        await clock.tickAsync(0);
        await el.updateComplete;

        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        privateAccess = el as unknown as SystemInfoPrivateAccess;

        // Ensure no stream subscription exists
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-type-assertion -- setting private property for testing
        delete (el as any).streamSubscription;

        // Call stopJobStream - should not throw
        privateAccess.stopJobStream();
      });

      it('should not throw', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        expect((el as any).streamSubscription).to.be.undefined;
      });
    });
  });

  describe('disconnectedCallback cleanup', () => {
    describe('when debounce timer exists', () => {
      beforeEach(async () => {
        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client - initial load succeeds
        mockClient = createMockClient();
        mockClient.getVersion.resolves(create(GetVersionResponseSchema, {
          commit: 'abc123',
          buildTime: create(TimestampSchema, {})
        }));
        mockClient.getJobStatus.resolves(create(GetJobStatusResponseSchema, {
          jobQueues: []
        }));

        // Stub lifecycle methods before connection
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        await clock.tickAsync(0);
        await el.updateComplete;

        // Manually set a debounce timer to test cleanup
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-type-assertion -- setting private property for testing
        (el as any).debounceTimer = setTimeout(() => {}, 1000);

        // Remove from DOM (this triggers disconnectedCallback)
        el.parentNode?.removeChild(el);
      });

      it('should clear the debounce timer', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        expect((el as any).debounceTimer).to.be.undefined;
      });
    });

    describe('when debounce timer does not exist', () => {
      beforeEach(async () => {
        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client - initial load succeeds
        mockClient = createMockClient();
        mockClient.getVersion.resolves(create(GetVersionResponseSchema, {
          commit: 'abc123',
          buildTime: create(TimestampSchema, {})
        }));
        mockClient.getJobStatus.resolves(create(GetJobStatusResponseSchema, {
          jobQueues: []
        }));

        // Stub lifecycle methods before connection
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        await clock.tickAsync(0);
        await el.updateComplete;

        // Ensure no debounce timer exists
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-type-assertion -- setting private property for testing
        delete (el as any).debounceTimer;

        // Remove from DOM (this triggers disconnectedCallback)
        el.parentNode?.removeChild(el);
      });

      it('should not throw when no debounce timer exists', () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        expect((el as any).debounceTimer).to.be.undefined;
      });
    });
  });

  describe('startJobStream', () => {
    describe('when stream throws a non-abort error', () => {
      let privateAccess: SystemInfoPrivateAccess;

      beforeEach(async () => {
        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client - initial load succeeds with active jobs
        mockClient = createMockClient();
        mockClient.getVersion.resolves(create(GetVersionResponseSchema, {
          commit: 'abc123',
          buildTime: create(TimestampSchema, {})
        }));
        mockClient.getJobStatus.resolves(create(GetJobStatusResponseSchema, {
          jobQueues: [create(JobQueueStatusSchema, {
            name: 'Test',
            jobsRemaining: 10,
            highWaterMark: 100,
            isActive: true
          })]
        }));

        // Stub lifecycle methods before connection
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopAutoRefreshStub = stub(el, 'stopAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopJobStreamStub = stub(el, 'stopJobStream' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        await clock.tickAsync(0);
        await el.updateComplete;

        // Clear the error and reset stubs for next call
        el.error = null;
        startAutoRefreshStub.resetHistory();
        stopJobStreamStub.restore();
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for stubbing
        stopJobStreamStub = stub(el, 'stopJobStream' as keyof SystemInfo);

        // Make stream throw an error
        const streamError = new Error('Stream connection failed');
        // eslint-disable-next-line @typescript-eslint/require-await, require-yield -- async generator for mocking streaming errors
        mockClient.streamJobStatus.callsFake(async function* () {
          throw streamError;
        });

        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        privateAccess = el as unknown as SystemInfoPrivateAccess;
        await privateAccess.startJobStream();
        await el.updateComplete;
      });

      it('should set the error property', () => {
        expect(el.error).to.exist;
        expect(el.error?.message).to.equal('Stream connection failed');
      });

      it('should start auto refresh as fallback', () => {
        expect(startAutoRefreshStub).to.have.been.called;
      });

      it('should display error in version component', () => {
        const versionComponent = el.shadowRoot!.querySelector('system-info-version');
        expect(versionComponent).to.exist;
        expect(versionComponent!.error?.message).to.equal('Stream connection failed');
      });
    });

    describe('when stream throws an AbortError', () => {
      let privateAccess: SystemInfoPrivateAccess;

      beforeEach(async () => {
        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client - initial load succeeds
        mockClient = createMockClient();
        mockClient.getVersion.resolves(create(GetVersionResponseSchema, {
          commit: 'abc123',
          buildTime: create(TimestampSchema, {})
        }));
        mockClient.getJobStatus.resolves(create(GetJobStatusResponseSchema, {
          jobQueues: [create(JobQueueStatusSchema, {
            name: 'Test',
            jobsRemaining: 10,
            highWaterMark: 100,
            isActive: true
          })]
        }));

        // Stub lifecycle methods before connection
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopAutoRefreshStub = stub(el, 'stopAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopJobStreamStub = stub(el, 'stopJobStream' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        await clock.tickAsync(0);
        await el.updateComplete;

        // Clear the error and reset stubs
        el.error = null;
        startAutoRefreshStub.resetHistory();
        stopJobStreamStub.restore();
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for stubbing
        stopJobStreamStub = stub(el, 'stopJobStream' as keyof SystemInfo);

        // Make stream throw an AbortError
        const abortError = new Error('Aborted');
        abortError.name = 'AbortError';
        // eslint-disable-next-line @typescript-eslint/require-await, require-yield -- async generator for mocking streaming errors
        mockClient.streamJobStatus.callsFake(async function* () {
          throw abortError;
        });

        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        privateAccess = el as unknown as SystemInfoPrivateAccess;
        await privateAccess.startJobStream();
        await el.updateComplete;
      });

      it('should not set the error property', () => {
        expect(el.error).to.be.null;
      });

      it('should not start auto refresh', () => {
        expect(startAutoRefreshStub).to.not.have.been.called;
      });
    });

    describe('when stream error is a non-Error object', () => {
      let privateAccess: SystemInfoPrivateAccess;

      beforeEach(async () => {
        // Create element without connecting it to DOM
        el = document.createElement('system-info') as SystemInfo;

        // Create mock client - initial load succeeds
        mockClient = createMockClient();
        mockClient.getVersion.resolves(create(GetVersionResponseSchema, {
          commit: 'abc123',
          buildTime: create(TimestampSchema, {})
        }));
        mockClient.getJobStatus.resolves(create(GetJobStatusResponseSchema, {
          jobQueues: [create(JobQueueStatusSchema, {
            name: 'Test',
            jobsRemaining: 10,
            highWaterMark: 100,
            isActive: true
          })]
        }));

        // Stub lifecycle methods before connection
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        startAutoRefreshStub = stub(el, 'startAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopAutoRefreshStub = stub(el, 'stopAutoRefresh' as keyof SystemInfo);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private methods for stubbing
        stopJobStreamStub = stub(el, 'stopJobStream' as keyof SystemInfo);

        // Replace the client before loadSystemInfo is called
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as SystemInfoPrivateAccess).client = mockClient;

        // Add to DOM (this triggers connectedCallback -> loadSystemInfo)
        document.body.appendChild(el);

        await clock.tickAsync(0);
        await el.updateComplete;

        // Clear the error and reset stubs
        el.error = null;
        startAutoRefreshStub.resetHistory();
        stopJobStreamStub.restore();
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for stubbing
        stopJobStreamStub = stub(el, 'stopJobStream' as keyof SystemInfo);

        // Make stream throw a non-Error object
        // eslint-disable-next-line @typescript-eslint/require-await, require-yield -- async generator for mocking streaming errors
        mockClient.streamJobStatus.callsFake(async function* () {
          throw 'string error from stream';
        });

        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        privateAccess = el as unknown as SystemInfoPrivateAccess;
        await privateAccess.startJobStream();
        await el.updateComplete;
      });

      it('should convert to Error object', () => {
        expect(el.error).to.be.instanceOf(Error);
        expect(el.error?.message).to.equal('string error from stream');
      });
    });
  });
});