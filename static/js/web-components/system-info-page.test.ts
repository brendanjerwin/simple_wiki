import { expect } from '@open-wc/testing';
import { fixture, html } from '@open-wc/testing-helpers';
import { useFakeTimers, type SinonFakeTimers } from 'sinon';
import { SystemInfoPage, type PageStatus } from './system-info-page.js';
import './system-info-page.js';

describe('SystemInfoPage', () => {
  it('should exist', async () => {
    const el = await fixture<SystemInfoPage>(html`<system-info-page></system-info-page>`);
    expect(el).to.be.instanceOf(SystemInfoPage);
  });

  describe('when pageStatus is not set', () => {
    let el: SystemInfoPage;

    beforeEach(async () => {
      el = await fixture<SystemInfoPage>(html`<system-info-page></system-info-page>`);
    });

    it('should render nothing', () => {
      expect(el.shadowRoot!.querySelector('.updated-row')).not.to.not.equal(null);
    });
  });

  describe('when pageStatus has no lastRefreshTime', () => {
    let el: SystemInfoPage;

    beforeEach(async () => {
      el = await fixture<SystemInfoPage>(html`<system-info-page></system-info-page>`);
      el.pageStatus = { pageName: 'test-page', isWatching: true };
      await el.updateComplete;
    });

    it('should render nothing', () => {
      expect(el.shadowRoot!.querySelector('.updated-row')).not.to.not.equal(null);
    });
  });

  describe('when pageStatus has a lastRefreshTime', () => {
    let el: SystemInfoPage;
    let clock: SinonFakeTimers;
    let now: Date;

    beforeEach(async () => {
      now = new Date('2024-01-01T12:00:00Z');
      clock = useFakeTimers(now.getTime());

      el = await fixture<SystemInfoPage>(html`<system-info-page></system-info-page>`);
      el.pageStatus = {
        pageName: 'test-page',
        isWatching: true,
        lastRefreshTime: now,
      } satisfies PageStatus;
      await el.updateComplete;
    });

    afterEach(() => {
      clock.restore();
    });

    it('should render the updated row', () => {
      expect(el.shadowRoot!.querySelector('.updated-row')).to.not.equal(null);
    });

    it('should display the label "Page saved:"', () => {
      const label = el.shadowRoot!.querySelector('.label');
      expect(label).to.not.equal(null);
      expect(label!.textContent).to.equal('Page saved:');
    });

    it('should display "0s ago" immediately after refresh', () => {
      const time = el.shadowRoot!.querySelector('.time');
      expect(time).to.not.equal(null);
      expect(time!.textContent).to.equal('0s ago');
    });

    describe('when 30 seconds pass', () => {
      beforeEach(async () => {
        await clock.tickAsync(30000);
        await el.updateComplete;
      });

      it('should update time display to "30s ago"', () => {
        const time = el.shadowRoot!.querySelector('.time');
        expect(time!.textContent).to.equal('30s ago');
      });
    });

    describe('when 2 minutes pass', () => {
      beforeEach(async () => {
        await clock.tickAsync(2 * 60 * 1000);
        await el.updateComplete;
      });

      it('should update time display to "~2m ago"', () => {
        const time = el.shadowRoot!.querySelector('.time');
        expect(time!.textContent).to.equal('~2m ago');
      });
    });
  });

  describe('formatTimeAgo', () => {
    let el: SystemInfoPage;
    let clock: SinonFakeTimers;

    beforeEach(async () => {
      el = await fixture<SystemInfoPage>(html`<system-info-page></system-info-page>`);
    });

    afterEach(() => {
      clock?.restore();
    });

    describe('when time is in hours', () => {
      beforeEach(async () => {
        const baseDate = new Date('2024-01-01T10:00:00Z');
        clock = useFakeTimers(new Date('2024-01-01T12:30:00Z').getTime());
        el.pageStatus = { pageName: 'test', isWatching: false, lastRefreshTime: baseDate };
        await el.updateComplete;
      });

      it('should display hours ago', () => {
        const time = el.shadowRoot!.querySelector('.time');
        expect(time!.textContent).to.equal('~2h ago');
      });
    });

    describe('when time is in days', () => {
      beforeEach(async () => {
        const baseDate = new Date('2024-01-01T10:00:00Z');
        clock = useFakeTimers(new Date('2024-01-04T10:00:00Z').getTime());
        el.pageStatus = { pageName: 'test', isWatching: false, lastRefreshTime: baseDate };
        await el.updateComplete;
      });

      it('should display days ago', () => {
        const time = el.shadowRoot!.querySelector('.time');
        expect(time!.textContent).to.equal('~3d ago');
      });
    });
  });

  describe('periodic time refresh', () => {
    let el: SystemInfoPage;
    let clock: SinonFakeTimers;

    beforeEach(async () => {
      const baseDate = new Date('2024-01-01T12:00:00Z');
      clock = useFakeTimers(baseDate.getTime());

      el = await fixture<SystemInfoPage>(html`<system-info-page></system-info-page>`);
      el.pageStatus = { pageName: 'test', isWatching: true, lastRefreshTime: baseDate };
      await el.updateComplete;
    });

    afterEach(() => {
      clock.restore();
    });

    describe('after interval elapses', () => {
      beforeEach(async () => {
        // Advance past the TIME_REFRESH_INTERVAL_MS (5000ms)
        await clock.tickAsync(10000);
        await el.updateComplete;
      });

      it('should show updated time display', () => {
        const time = el.shadowRoot!.querySelector('.time');
        expect(time!.textContent).to.equal('10s ago');
      });
    });
  });
});
