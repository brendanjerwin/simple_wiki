import { html, fixture, expect } from '@open-wc/testing';
import { SystemInfoIndexing } from './system-info-indexing.js';
import { GetIndexingStatusResponse, SingleIndexProgress } from '../gen/api/v1/system_info_pb.js';
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
      el.status = undefined;
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
      el.status = undefined;
      await el.updateComplete;
    });

    it('should display error message', () => {
      const errorText = el.shadowRoot!.querySelector('.error');
      expect(errorText).to.exist;
      expect(errorText!.textContent).to.include('Connection failed');
    });
  });

  describe('when indexing is not running', () => {
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

    it('should render nothing when not running', () => {
      const indexingInfo = el.shadowRoot!.querySelector('.indexing-info');
      expect(indexingInfo).to.not.exist;
    });
  });

  describe('when indexing is running', () => {
    beforeEach(async () => {
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
        completedPages: 50,
        queueDepth: 25,
        processingRatePerSecond: 7.8,
        indexProgress: [index1Progress, index2Progress]
      });
      await el.updateComplete;
    });

    it('should display indexing info', () => {
      const indexingInfo = el.shadowRoot!.querySelector('.indexing-info');
      expect(indexingInfo).to.exist;
    });

    it('should show indexing header with progress', () => {
      const indexingHeader = el.shadowRoot!.querySelector('.indexing-header');
      expect(indexingHeader).to.exist;
      
      const value = indexingHeader!.querySelector('.value');
      expect(value!.textContent).to.equal('50/100');
    });

    it('should show active status indicator', () => {
      const indicator = el.shadowRoot!.querySelector('.status-indicator');
      expect(indicator).to.exist;
      expect(indicator).to.not.have.class('idle');
    });

    it('should display progress bar', () => {
      const progressBar = el.shadowRoot!.querySelector('.progress-bar-mini');
      expect(progressBar).to.exist;
      
      const progressFill = progressBar!.querySelector('.progress-fill-mini');
      expect(progressFill).to.exist;
    });

    it('should show processing rate', () => {
      const rate = el.shadowRoot!.querySelector('.rate');
      expect(rate).to.exist;
      expect(rate!.textContent).to.equal('8/s');
    });

    it('should display queue depth when present', () => {
      const queue = el.shadowRoot!.querySelector('.queue');
      expect(queue).to.exist;
      expect(queue!.textContent).to.equal('Q:25');
    });

    it('should show per-index progress when multiple indexes exist', () => {
      const perIndexProgress = el.shadowRoot!.querySelector('.per-index-progress');
      expect(perIndexProgress).to.exist;
      
      const indexItems = perIndexProgress!.querySelectorAll('.index-item');
      expect(indexItems).to.have.lengthOf(2);
    });
  });

  describe('when indexing is idle', () => {
    beforeEach(async () => {
      el.loading = false;
      el.status = new GetIndexingStatusResponse({
        isRunning: false,
        totalPages: 100,
        completedPages: 100,
        queueDepth: 0,
        processingRatePerSecond: 0,
        indexProgress: []
      });
      await el.updateComplete;
    });

    it('should not render anything when idle', () => {
      const indexingInfo = el.shadowRoot!.querySelector('.indexing-info');
      expect(indexingInfo).to.not.exist;
    });
  });
});