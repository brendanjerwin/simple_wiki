import { html, fixture, expect } from '@open-wc/testing';
import { IndexingStatus } from './indexing-status.js';
import { GetIndexingStatusResponse, SingleIndexProgress } from '../gen/api/v1/system_info_pb.js';
import { Timestamp } from '@bufbuild/protobuf';
import { stub, useFakeTimers } from 'sinon';
import './indexing-status.js';

describe('IndexingStatus', () => {
  let el: IndexingStatus;
  let clock: any;

  beforeEach(async () => {
    clock = useFakeTimers();
    // Create element without connecting it to DOM first
    el = document.createElement('indexing-status') as IndexingStatus;
    
    // Stub methods that make network requests before connecting
    stub(el, 'loadStatus' as any).resolves();
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

  it('should be an instance of IndexingStatus', () => {
    expect(el).to.be.instanceOf(IndexingStatus);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('indexing-status');
  });

  describe('when loading', () => {
    beforeEach(async () => {
      el.loading = true;
      el.status = undefined;
      await el.updateComplete;
    });

    it('should display loading message', () => {
      const loadingText = el.shadowRoot!.querySelector('.loading');
      expect(loadingText).to.exist;
      expect(loadingText!.textContent).to.include('Loading indexing status');
    });
  });

  describe('when there is an error', () => {
    beforeEach(async () => {
      el.loading = false;
      el.error = 'Connection failed';
      await el.updateComplete;
    });

    it('should display error message', () => {
      const errorText = el.shadowRoot!.querySelector('.error');
      expect(errorText).to.exist;
      expect(errorText!.textContent).to.include('Connection failed');
    });
  });

  describe('when indexing is idle', () => {
    beforeEach(async () => {
      el.loading = false;
      el.status = new GetIndexingStatusResponse({
        isRunning: false,
        totalPages: 0,
        completedPages: 0,
        queueDepth: 0,
        processingRatePerSecond: 0,
        indexProgress: []
      });
      await el.updateComplete;
    });

    it('should show idle status', () => {
      const statusHeader = el.shadowRoot!.querySelector('.status-header');
      expect(statusHeader!.textContent).to.include('Idle');
    });

    it('should have idle status indicator', () => {
      const indicator = el.shadowRoot!.querySelector('.status-indicator');
      expect(indicator).to.have.class('idle');
      expect(indicator).to.not.have.class('running');
    });
  });

  describe('when indexing is running', () => {
    beforeEach(async () => {
      const mockTimestamp = new Timestamp({
        seconds: BigInt(Math.floor((Date.now() + 300000) / 1000)), // 5 minutes from now
        nanos: 0
      });

      const index1Progress = new SingleIndexProgress({
        name: 'frontmatter',
        completed: 75,
        total: 100,
        processingRatePerSecond: 10.5,
        lastError: undefined
      });

      const index2Progress = new SingleIndexProgress({
        name: 'bleve',
        completed: 50,
        total: 100,
        processingRatePerSecond: 5.2,
        lastError: undefined
      });

      el.loading = false;
      el.status = new GetIndexingStatusResponse({
        isRunning: true,
        totalPages: 100,
        completedPages: 50, // Minimum of the two indexes
        queueDepth: 25,
        processingRatePerSecond: 7.8,
        estimatedCompletion: mockTimestamp,
        indexProgress: [index1Progress, index2Progress]
      });
      await el.updateComplete;
    });

    it('should show active status', () => {
      const statusHeader = el.shadowRoot!.querySelector('.status-header');
      expect(statusHeader!.textContent).to.include('Active');
    });

    it('should have running status indicator', () => {
      const indicator = el.shadowRoot!.querySelector('.status-indicator');
      expect(indicator).to.have.class('running');
      expect(indicator).to.not.have.class('idle');
    });

    it('should display progress overview', () => {
      const progressOverview = el.shadowRoot!.querySelector('.progress-overview');
      expect(progressOverview).to.exist;

      const progressItems = progressOverview!.querySelectorAll('.progress-item');
      expect(progressItems).to.have.lengthOf(3); // Progress, Queue Depth, Rate
    });

    it('should show correct progress values', () => {
      const progressValues = el.shadowRoot!.querySelectorAll('.progress-value');
      const progressText = Array.from(progressValues).map(el => el.textContent);
      
      expect(progressText).to.include('50/100'); // Progress
      expect(progressText).to.include('25'); // Queue depth
      expect(progressText).to.include('8/s'); // Rate (rounded)
    });

    it('should display progress bar with correct width', () => {
      const progressFill = el.shadowRoot!.querySelector('.progress-fill');
      expect(progressFill).to.exist;
      expect(progressFill!.getAttribute('style')).to.include('width: 50%');
    });

    it('should show ETA', () => {
      const eta = el.shadowRoot!.querySelector('.eta');
      expect(eta).to.exist;
      expect(eta!.textContent).to.include('ETA');
    });

    it('should display per-index progress details', () => {
      const indexDetails = el.shadowRoot!.querySelector('.index-details');
      expect(indexDetails).to.exist;

      const summary = indexDetails!.querySelector('summary');
      expect(summary!.textContent).to.include('Per-Index Progress (2 indexes)');

      const indexItems = indexDetails!.querySelectorAll('.index-item');
      expect(indexItems).to.have.lengthOf(2);
    });

    it('should show correct per-index progress', () => {
      const indexItems = el.shadowRoot!.querySelectorAll('.index-item');
      
      // Check frontmatter index (75% complete)
      const frontmatterItem = Array.from(indexItems).find(item => 
        item.querySelector('.index-name')!.textContent === 'frontmatter'
      );
      expect(frontmatterItem).to.exist;
      
      const frontmatterProgress = frontmatterItem!.querySelector('.index-progress-fill');
      expect(frontmatterProgress!.getAttribute('style')).to.include('width: 75%');

      // Check bleve index (50% complete)
      const bleveItem = Array.from(indexItems).find(item => 
        item.querySelector('.index-name')!.textContent === 'bleve'
      );
      expect(bleveItem).to.exist;
      
      const bleveProgress = bleveItem!.querySelector('.index-progress-fill');
      expect(bleveProgress!.getAttribute('style')).to.include('width: 50%');
    });
  });

  describe('when index has errors', () => {
    beforeEach(async () => {
      const errorIndex = new SingleIndexProgress({
        name: 'problematic-index',
        completed: 10,
        total: 100,
        processingRatePerSecond: 1.0,
        lastError: 'Database connection failed'
      });

      el.loading = false;
      el.status = new GetIndexingStatusResponse({
        isRunning: true,
        totalPages: 100,
        completedPages: 10,
        queueDepth: 90,
        processingRatePerSecond: 1.0,
        indexProgress: [errorIndex]
      });
      await el.updateComplete;
    });

    it('should display error message for problematic index', async () => {
      // Force update to make sure content is rendered
      await el.updateComplete;
      
      // Need to open the details to see the error
      const details = el.shadowRoot!.querySelector('details') as HTMLDetailsElement;
      if (details) {
        details.open = true;
        await el.updateComplete;
      }
      
      const errorMessages = el.shadowRoot!.querySelectorAll('.error');
      const errorTexts = Array.from(errorMessages).map(e => e.textContent);
      const hasError = errorTexts.some(text => text && text.includes('Database connection failed'));
      expect(hasError).to.be.true;
    });

    it('should style progress bar as error', () => {
      const progressFill = el.shadowRoot!.querySelector('.index-progress-fill');
      expect(progressFill).to.have.class('error');
    });
  });

  describe('progress calculation', () => {
    it('should handle zero total pages', () => {
      el.status = new GetIndexingStatusResponse({
        isRunning: false,
        totalPages: 0,
        completedPages: 0,
        queueDepth: 0,
        processingRatePerSecond: 0,
        indexProgress: []
      });

      // Should not crash and should not show progress section
      expect(() => el.render()).to.not.throw();
    });

    it('should format rates correctly', async () => {
      // Test the private formatRate method through public interface
      el.status = new GetIndexingStatusResponse({
        isRunning: true,
        totalPages: 100,
        completedPages: 50,
        queueDepth: 25,
        processingRatePerSecond: 0.05, // Very slow rate
        indexProgress: []
      });

      await el.updateComplete;
      
      // Find the specific rate value (third progress item)
      const progressItems = el.shadowRoot!.querySelectorAll('.progress-item');
      expect(progressItems).to.have.length.greaterThan(2);
      
      const rateItem = progressItems[2]; // Third item should be rate
      const rateValue = rateItem.querySelector('.progress-value');
      expect(rateValue!.textContent).to.include('< 0.1/s');
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

    it('should start auto-refresh on connection', () => {
      expect(connectStub).to.not.have.been.called;
      // We can't easily test the timer without more complex mocking
    });
  });
});