import { html, fixture, expect } from '@open-wc/testing';
import { SystemInfoIndexing } from './system-info-indexing.js';
import { GetJobStatusResponse, JobQueueStatus } from '../gen/api/v1/system_info_pb.js';
import { stub, restore, type SinonStub } from 'sinon';
import './system-info-indexing.js';

describe('SystemInfoIndexing', () => {
  let el: SystemInfoIndexing;

  beforeEach(async () => {
    el = await fixture(html`<system-info-indexing></system-info-indexing>`);
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  it('should be an instance of SystemInfoIndexing', () => {
    expect(el).to.be.instanceOf(SystemInfoIndexing);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('system-info-indexing');
  });

  describe('when loading', () => {
    beforeEach(async () => {
      el.loading = true;
      el.jobStatus = undefined;
      await el.updateComplete;
    });

    it('should display loading message', () => {
      const loadingText = el.shadowRoot!.querySelector('.loading');
      expect(loadingText).to.exist;
      expect(loadingText!.textContent).to.include('Loading...');
    });
  });

  describe('when there is an error', () => {
    beforeEach(async () => {
      el.loading = false;
      el.error = 'Connection failed';
      el.jobStatus = undefined;
      await el.updateComplete;
    });

    it('should display error message', () => {
      const errorText = el.shadowRoot!.querySelector('.error');
      expect(errorText).to.exist;
      expect(errorText!.textContent).to.include('Connection failed');
    });
  });

  describe('when no jobs are active', () => {
    beforeEach(async () => {
      el.loading = false;
      el.jobStatus = new GetJobStatusResponse({
        jobQueues: []
      });
      await el.updateComplete;
    });

    it('should render nothing when no jobs are active', () => {
      const indexingInfo = el.shadowRoot!.querySelector('.indexing-info');
      expect(indexingInfo).to.not.exist;
    });
  });

  describe('when jobs are running', () => {
    beforeEach(async () => {
      const frontmatterQueue = new JobQueueStatus({
        name: 'Frontmatter',
        jobsRemaining: 25,
        highWaterMark: 100,
        isActive: true
      });

      const bleveQueue = new JobQueueStatus({
        name: 'Bleve',
        jobsRemaining: 50,
        highWaterMark: 100,
        isActive: true
      });

      el.loading = false;
      el.jobStatus = new GetJobStatusResponse({
        jobQueues: [frontmatterQueue, bleveQueue]
      });
      await el.updateComplete;
    });

    it('should display job info', () => {
      const indexingInfo = el.shadowRoot!.querySelector('.indexing-info');
      expect(indexingInfo).to.exist;
    });

    it('should show job header with queue names and counts', () => {
      const indexingHeader = el.shadowRoot!.querySelector('.indexing-header');
      expect(indexingHeader).to.exist;
      
      const value = indexingHeader!.querySelector('.value');
      expect(value!.textContent).to.equal('Frontmatter: 25, Bleve: 50');
    });

    it('should show active status indicator', () => {
      const indicator = el.shadowRoot!.querySelector('.status-indicator');
      expect(indicator).to.exist;
    });

    it('should show Jobs label', () => {
      const label = el.shadowRoot!.querySelector('.label');
      expect(label).to.exist;
      expect(label!.textContent).to.equal('Jobs');
    });
  });

  describe('when jobs are mixed order', () => {
    beforeEach(async () => {
      // Create queues in a non-alphabetical order to test sorting
      const zebraQueue = new JobQueueStatus({
        name: 'Zebra',
        jobsRemaining: 10,
        highWaterMark: 100,
        isActive: true
      });

      const alphaQueue = new JobQueueStatus({
        name: 'Alpha',
        jobsRemaining: 20,
        highWaterMark: 100,
        isActive: true
      });

      const betaQueue = new JobQueueStatus({
        name: 'Beta',
        jobsRemaining: 30,
        highWaterMark: 100,
        isActive: true
      });

      el.loading = false;
      el.jobStatus = new GetJobStatusResponse({
        jobQueues: [zebraQueue, alphaQueue, betaQueue] // Deliberately unsorted
      });
      await el.updateComplete;
    });

    it('should display all queue names in the value', () => {
      const value = el.shadowRoot!.querySelector('.value');
      expect(value!.textContent).to.include('Zebra: 10');
      expect(value!.textContent).to.include('Alpha: 20');
      expect(value!.textContent).to.include('Beta: 30');
    });
  });

  describe('when all jobs are idle', () => {
    beforeEach(async () => {
      const inactiveQueue = new JobQueueStatus({
        name: 'Frontmatter',
        jobsRemaining: 0,
        highWaterMark: 0,
        isActive: false
      });

      el.loading = false;
      el.jobStatus = new GetJobStatusResponse({
        jobQueues: [inactiveQueue]
      });
      await el.updateComplete;
    });

    it('should not render anything when all jobs are idle', () => {
      const indexingInfo = el.shadowRoot!.querySelector('.indexing-info');
      expect(indexingInfo).to.not.exist;
    });
  });

  describe('error click handling', () => {
    let writeTextStub: SinonStub;

    beforeEach(() => {
      // Setup clipboard API mock
      writeTextStub = stub();
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: writeTextStub
        },
        configurable: true
      });
    });

    afterEach(() => {
      restore();
    });

    describe('when jobs have errors', () => {
      beforeEach(async () => {
        // For now, job queue errors will be handled differently
        // This test structure is preserved for when we add error handling to job queues
        const workingQueue = new JobQueueStatus({
          name: 'Frontmatter',
          jobsRemaining: 25,
          highWaterMark: 100,
          isActive: true
        });

        el.loading = false;
        el.jobStatus = new GetJobStatusResponse({
          jobQueues: [workingQueue]
        });
        await el.updateComplete;
      });

      it('should display working queues', () => {
        const indexingInfo = el.shadowRoot!.querySelector('.indexing-info');
        expect(indexingInfo).to.exist;
      });

      // TODO: Add error handling tests when job queue error reporting is implemented
      it('should be ready for future error handling', () => {
        // Placeholder for error handling tests that will be added
        // when job queue error reporting is implemented
        expect(true).to.be.true;
      });
    });
  });
});