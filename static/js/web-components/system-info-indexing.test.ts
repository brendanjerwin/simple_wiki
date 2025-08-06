import { html, fixture, expect, assert } from '@open-wc/testing';
import { stub } from 'sinon';
import { SystemInfoIndexing } from './system-info-indexing.js';
import { GetJobStatusResponse, JobQueueStatus } from '../gen/api/v1/system_info_pb.js';

function timeout(ms: number, message: string) {
  return new Promise((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

describe('SystemInfoIndexing', () => {
  let el: SystemInfoIndexing;
  let fetchStub: ReturnType<typeof stub>;
  let writeTextStub: ReturnType<typeof stub>;

  beforeEach(async () => {
    // Prevent any potential network calls that could cause hanging
    fetchStub = stub(window, 'fetch');
    fetchStub.resolves(new Response('{}'));

    // Stub clipboard writeText for error click tests
    writeTextStub = stub(navigator.clipboard, 'writeText');
    writeTextStub.resolves();
    
    el = await Promise.race([
      fixture(html`<system-info-indexing></system-info-indexing>`),
      timeout(5000, "SystemInfoIndexing fixture timed out"),
    ]);
  });

  afterEach(() => {
    if (fetchStub) {
      fetchStub.restore();
    }
    if (writeTextStub) {
      writeTextStub.restore();
    }
  });

  it('should exist', () => {
    assert.instanceOf(el, SystemInfoIndexing);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName).to.equal('SYSTEM-INFO-INDEXING');
  });

  it('should have default property values', () => {
    expect(el.jobStatus).to.be.undefined;
    expect(el.loading).to.be.false;
    expect(el.error).to.be.undefined;
  });

  describe('when loading is true and no jobStatus', () => {
    beforeEach(async () => {
      el.loading = true;
      el.jobStatus = undefined;
      await el.updateComplete;
    });

    it('should display loading message', () => {
      const loadingElement = el.shadowRoot?.querySelector('.loading');
      expect(loadingElement).to.exist;
      expect(loadingElement?.textContent?.trim()).to.equal('Loading...');
    });

    it('should not display indexing info', () => {
      const indexingInfo = el.shadowRoot?.querySelector('.indexing-info');
      expect(indexingInfo).to.not.exist;
    });
  });

  describe('when error is set', () => {
    beforeEach(async () => {
      el.error = 'Test error message';
      await el.updateComplete;
    });

    it('should display error message', () => {
      const errorElement = el.shadowRoot?.querySelector('.error');
      expect(errorElement).to.exist;
      expect(errorElement?.textContent?.trim()).to.equal('Test error message');
    });

    it('should not display indexing info', () => {
      const indexingInfo = el.shadowRoot?.querySelector('.indexing-info');
      expect(indexingInfo).to.not.exist;
    });

    it('should not display loading message', () => {
      const loadingElement = el.shadowRoot?.querySelector('.loading');
      expect(loadingElement).to.not.exist;
    });
  });

  describe('when no jobStatus is provided', () => {
    beforeEach(async () => {
      el.loading = false;
      el.error = undefined;
      el.jobStatus = undefined;
      await el.updateComplete;
    });

    it('should display no data message', () => {
      const loadingElement = el.shadowRoot?.querySelector('.loading');
      expect(loadingElement).to.exist;
      expect(loadingElement?.textContent?.trim()).to.equal('No data');
    });

    it('should not display indexing info', () => {
      const indexingInfo = el.shadowRoot?.querySelector('.indexing-info');
      expect(indexingInfo).to.not.exist;
    });
  });

  describe('when jobStatus has no active queues', () => {
    beforeEach(async () => {
      const jobStatus = new GetJobStatusResponse();
      
      // Create inactive queues
      const inactiveQueue1 = new JobQueueStatus({
        name: 'Frontmatter',
        jobsRemaining: 0,
        highWaterMark: 5,
        isActive: false
      });
      
      const inactiveQueue2 = new JobQueueStatus({
        name: 'Bleve',
        jobsRemaining: 0,
        highWaterMark: 10,
        isActive: false
      });

      jobStatus.jobQueues = [inactiveQueue1, inactiveQueue2];
      
      el.jobStatus = jobStatus;
      el.loading = false;
      el.error = undefined;
      await el.updateComplete;
    });

    it('should render empty content', () => {
      // Component should render nothing when no active queues
      const indexingInfo = el.shadowRoot?.querySelector('.indexing-info');
      expect(indexingInfo).to.not.exist;
      
      const loadingElement = el.shadowRoot?.querySelector('.loading');
      expect(loadingElement).to.not.exist;
      
      const errorElement = el.shadowRoot?.querySelector('.error');
      expect(errorElement).to.not.exist;
    });

    it('should have empty shadowRoot content', () => {
      const content = el.shadowRoot?.textContent?.trim();
      expect(content).to.equal('');
    });
  });

  describe('when jobStatus has only active queues', () => {
    let activeQueue1: JobQueueStatus;
    let activeQueue2: JobQueueStatus;

    beforeEach(async () => {
      const jobStatus = new GetJobStatusResponse();
      
      activeQueue1 = new JobQueueStatus({
        name: 'Frontmatter',
        jobsRemaining: 5,
        highWaterMark: 10,
        isActive: true
      });
      
      activeQueue2 = new JobQueueStatus({
        name: 'Bleve',
        jobsRemaining: 3,
        highWaterMark: 8,
        isActive: true
      });

      jobStatus.jobQueues = [activeQueue1, activeQueue2];
      
      el.jobStatus = jobStatus;
      el.loading = false;
      el.error = undefined;
      await el.updateComplete;
    });

    it('should display indexing info section', () => {
      const indexingInfo = el.shadowRoot?.querySelector('.indexing-info');
      expect(indexingInfo).to.exist;
    });

    it('should display indexing header with status indicator', () => {
      const header = el.shadowRoot?.querySelector('.indexing-header');
      expect(header).to.exist;
      
      const statusIndicator = el.shadowRoot?.querySelector('.status-indicator');
      expect(statusIndicator).to.exist;
    });

    it('should display Jobs label', () => {
      const label = el.shadowRoot?.querySelector('.label');
      expect(label).to.exist;
      expect(label?.textContent?.trim()).to.equal('Jobs');
    });

    it('should display queue information in correct format', () => {
      const value = el.shadowRoot?.querySelector('.value');
      expect(value).to.exist;
      expect(value?.textContent?.trim()).to.equal('Frontmatter: 5, Bleve: 3');
    });

    it('should not display loading message', () => {
      const loadingElement = el.shadowRoot?.querySelector('.loading');
      expect(loadingElement).to.not.exist;
    });

    it('should not display error message', () => {
      const errorElement = el.shadowRoot?.querySelector('.error');
      expect(errorElement).to.not.exist;
    });
  });

  describe('when jobStatus has mixed active and inactive queues', () => {
    beforeEach(async () => {
      const jobStatus = new GetJobStatusResponse();
      
      const activeQueue = new JobQueueStatus({
        name: 'Frontmatter',
        jobsRemaining: 7,
        highWaterMark: 15,
        isActive: true
      });
      
      const inactiveQueue1 = new JobQueueStatus({
        name: 'Bleve',
        jobsRemaining: 0,
        highWaterMark: 5,
        isActive: false
      });
      
      const inactiveQueue2 = new JobQueueStatus({
        name: 'File Scan',
        jobsRemaining: 0,
        highWaterMark: 3,
        isActive: false
      });

      jobStatus.jobQueues = [activeQueue, inactiveQueue1, inactiveQueue2];
      
      el.jobStatus = jobStatus;
      el.loading = false;
      el.error = undefined;
      await el.updateComplete;
    });

    it('should only display active queues', () => {
      const value = el.shadowRoot?.querySelector('.value');
      expect(value).to.exist;
      expect(value?.textContent?.trim()).to.equal('Frontmatter: 7');
    });

    it('should display indexing info section', () => {
      const indexingInfo = el.shadowRoot?.querySelector('.indexing-info');
      expect(indexingInfo).to.exist;
    });
  });

  describe('when jobStatus has multiple active queues', () => {
    beforeEach(async () => {
      const jobStatus = new GetJobStatusResponse();
      
      const queue1 = new JobQueueStatus({
        name: 'Frontmatter',
        jobsRemaining: 12,
        highWaterMark: 20,
        isActive: true
      });
      
      const queue2 = new JobQueueStatus({
        name: 'Bleve',
        jobsRemaining: 8,
        highWaterMark: 15,
        isActive: true
      });
      
      const queue3 = new JobQueueStatus({
        name: 'File Scan',
        jobsRemaining: 3,
        highWaterMark: 10,
        isActive: true
      });

      jobStatus.jobQueues = [queue1, queue2, queue3];
      
      el.jobStatus = jobStatus;
      el.loading = false;
      el.error = undefined;
      await el.updateComplete;
    });

    it('should display all active queues separated by commas', () => {
      const value = el.shadowRoot?.querySelector('.value');
      expect(value).to.exist;
      expect(value?.textContent?.trim()).to.equal('Frontmatter: 12, Bleve: 8, File Scan: 3');
    });
  });

  describe('when jobStatus has a single active queue', () => {
    beforeEach(async () => {
      const jobStatus = new GetJobStatusResponse();
      
      const singleQueue = new JobQueueStatus({
        name: 'Bleve',
        jobsRemaining: 25,
        highWaterMark: 30,
        isActive: true
      });

      jobStatus.jobQueues = [singleQueue];
      
      el.jobStatus = jobStatus;
      el.loading = false;
      el.error = undefined;
      await el.updateComplete;
    });

    it('should display single queue without comma', () => {
      const value = el.shadowRoot?.querySelector('.value');
      expect(value).to.exist;
      expect(value?.textContent?.trim()).to.equal('Bleve: 25');
    });
  });

  describe('when loading is true but jobStatus exists', () => {
    beforeEach(async () => {
      const jobStatus = new GetJobStatusResponse();
      
      const activeQueue = new JobQueueStatus({
        name: 'Frontmatter',
        jobsRemaining: 5,
        highWaterMark: 10,
        isActive: true
      });

      jobStatus.jobQueues = [activeQueue];
      
      el.jobStatus = jobStatus;
      el.loading = true;
      el.error = undefined;
      await el.updateComplete;
    });

    it('should display queue information when jobStatus exists', () => {
      const indexingInfo = el.shadowRoot?.querySelector('.indexing-info');
      expect(indexingInfo).to.exist;
      
      const value = el.shadowRoot?.querySelector('.value');
      expect(value).to.exist;
      expect(value?.textContent?.trim()).to.equal('Frontmatter: 5');
    });

    it('should not display loading message when jobStatus exists', () => {
      const loadingElement = el.shadowRoot?.querySelector('.loading');
      expect(loadingElement).to.not.exist;
    });
  });

  describe('when error state is present', () => {
    beforeEach(async () => {
      el.error = 'Connection failed';
      await el.updateComplete;
    });

    it('should display error message', () => {
      const errorElement = el.shadowRoot?.querySelector('.error');
      expect(errorElement).to.exist;
      expect(errorElement?.textContent?.trim()).to.equal('Connection failed');
    });

    it('should have basic error styling', () => {
      const errorElement = el.shadowRoot?.querySelector('.error');
      expect(errorElement).to.exist;
      expect(errorElement?.classList.contains('error')).to.be.true;
    });

    it('should not have clickable styling by default', () => {
      const errorElement = el.shadowRoot?.querySelector('.error');
      expect(errorElement).to.exist;
      expect(errorElement?.classList.contains('clickable')).to.be.false;
    });
  });

  describe('formatRate method', () => {
    describe('when rate is very low', () => {
      let result: string;

      beforeEach(() => {
        const component = el as SystemInfoIndexing & { formatRate(rate: number): string };
        result = component.formatRate(0.05);
      });

      it('should return < 0.1/s format', () => {
        expect(result).to.equal('< 0.1/s');
      });
    });

    describe('when rate is less than 1 but above 0.1', () => {
      let result: string;

      beforeEach(() => {
        const component = el as SystemInfoIndexing & { formatRate(rate: number): string };
        result = component.formatRate(0.7);
      });

      it('should return decimal format', () => {
        expect(result).to.equal('0.7/s');
      });
    });

    describe('when rate is greater than or equal to 1', () => {
      let result: string;

      beforeEach(() => {
        const component = el as SystemInfoIndexing & { formatRate(rate: number): string };
        result = component.formatRate(2.8);
      });

      it('should return rounded integer format', () => {
        expect(result).to.equal('3/s');
      });
    });

    describe('when rate is exactly 0.1', () => {
      let result: string;

      beforeEach(() => {
        const component = el as SystemInfoIndexing & { formatRate(rate: number): string };
        result = component.formatRate(0.1);
      });

      it('should return decimal format', () => {
        expect(result).to.equal('0.1/s');
      });
    });

    describe('when rate is exactly 1', () => {
      let result: string;

      beforeEach(() => {
        const component = el as SystemInfoIndexing & { formatRate(rate: number): string };
        result = component.formatRate(1.0);
      });

      it('should return integer format', () => {
        expect(result).to.equal('1/s');
      });
    });
  });

  describe('calculateProgress method', () => {
    describe('when calculating normal progress', () => {
      let result: number;

      beforeEach(() => {
        const component = el as SystemInfoIndexing & { calculateProgress(completed: number, total: number): number };
        result = component.calculateProgress(50, 100);
      });

      it('should return correct percentage', () => {
        expect(result).to.equal(50);
      });
    });

    describe('when total is zero', () => {
      let result: number;

      beforeEach(() => {
        const component = el as SystemInfoIndexing & { calculateProgress(completed: number, total: number): number };
        result = component.calculateProgress(10, 0);
      });

      it('should return zero to avoid division by zero', () => {
        expect(result).to.equal(0);
      });
    });

    describe('when progress is complete', () => {
      let result: number;

      beforeEach(() => {
        const component = el as SystemInfoIndexing & { calculateProgress(completed: number, total: number): number };
        result = component.calculateProgress(100, 100);
      });

      it('should return 100 percent', () => {
        expect(result).to.equal(100);
      });
    });

    describe('when no progress has been made', () => {
      let result: number;

      beforeEach(() => {
        const component = el as SystemInfoIndexing & { calculateProgress(completed: number, total: number): number };
        result = component.calculateProgress(0, 100);
      });

      it('should return zero percent', () => {
        expect(result).to.equal(0);
      });
    });
  });

  describe('accessibility', () => {
    describe('when error is present', () => {
      beforeEach(async () => {
        el.error = 'Accessibility test error';
        await el.updateComplete;
      });

      it('should display error message in accessible format', () => {
        const errorElement = el.shadowRoot?.querySelector('.error') as HTMLElement;
        expect(errorElement).to.exist;
        expect(errorElement.textContent?.trim()).to.equal('Accessibility test error');
      });

      it('should have basic error element without interactive features', () => {
        const errorElement = el.shadowRoot?.querySelector('.error') as HTMLElement;
        expect(errorElement).to.exist;
        
        // Should not have interactive attributes since error clicking is not implemented
        expect(errorElement.tabIndex).to.not.equal(0);
        expect(errorElement.classList.contains('clickable')).to.be.false;
      });
    });

    describe('when indexing info is displayed', () => {
      beforeEach(async () => {
        const jobStatus = new GetJobStatusResponse();
        
        const activeQueue = new JobQueueStatus({
          name: 'Frontmatter',
          jobsRemaining: 5,
          highWaterMark: 10,
          isActive: true
        });

        jobStatus.jobQueues = [activeQueue];
        
        el.jobStatus = jobStatus;
        el.loading = false;
        el.error = undefined;
        await el.updateComplete;
      });

      it('should have semantic structure with proper labels', () => {
        const label = el.shadowRoot?.querySelector('.label');
        expect(label).to.exist;
        expect(label?.textContent?.trim()).to.equal('Jobs');
        
        const value = el.shadowRoot?.querySelector('.value');
        expect(value).to.exist;
        expect(value?.textContent?.trim()).to.include('Frontmatter: 5');
      });
    });
  });
});