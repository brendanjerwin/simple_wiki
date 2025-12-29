import { html, fixture, expect, assert } from '@open-wc/testing';
import { stub } from 'sinon';
import { SystemInfoVersion } from './system-info-version.js';
import { GetVersionResponseSchema } from '../gen/api/v1/system_info_pb.js';
import { create } from '@bufbuild/protobuf';
import { TimestampSchema } from '@bufbuild/protobuf/wkt';
import './system-info-version.js';

function timeout(ms: number, message: string) {
  return new Promise((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

describe('SystemInfoVersion', () => {
  let el: SystemInfoVersion;
  let fetchStub: ReturnType<typeof stub>;

  beforeEach(async () => {
    // Prevent any potential network calls that could cause hanging
    fetchStub = stub(window, 'fetch');
    fetchStub.resolves(new Response('{}'));
    
    el = await Promise.race([
      fixture(html`<system-info-version></system-info-version>`),
      timeout(5000, "SystemInfoVersion fixture timed out"),
    ]);
  });

  afterEach(() => {
    if (fetchStub) {
      fetchStub.restore();
    }
  });

  it('should exist', () => {
    assert.instanceOf(el, SystemInfoVersion);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName).to.equal('SYSTEM-INFO-VERSION');
  });

  it('should have initial loading state as false', () => {
    expect(el.loading).to.be.false;
  });

  it('should have undefined version initially', () => {
    expect(el.version).to.be.undefined;
  });

  it('should have null error initially', () => {
    expect(el.error).to.be.null;
  });

  describe('when in loading state without version data', () => {
    beforeEach(async () => {
      el.loading = true;
      el.version = undefined;
      el.error = null;
      await Promise.race([
        el.updateComplete,
        timeout(5000, "Component update timed out"),
      ]);
    });

    it('should display loading message for commit', () => {
      const loadingElements = el.shadowRoot?.querySelectorAll('.loading');
      expect(loadingElements).to.have.length(2);
      
      const commitRow = el.shadowRoot?.querySelector('.version-row:first-child');
      const loadingSpan = commitRow?.querySelector('.loading');
      expect(loadingSpan?.textContent).to.equal('Loading...');
    });

    it('should display loading message for build time', () => {
      const buildTimeRow = el.shadowRoot?.querySelector('.version-row:last-child');
      const loadingSpan = buildTimeRow?.querySelector('.loading');
      expect(loadingSpan?.textContent).to.equal('Loading...');
    });

    it('should display commit label', () => {
      const commitLabel = el.shadowRoot?.querySelector('.version-row:first-child .label');
      expect(commitLabel?.textContent).to.equal('Commit:');
    });

    it('should display built label', () => {
      const builtLabel = el.shadowRoot?.querySelector('.version-row:last-child .label');
      expect(builtLabel?.textContent).to.equal('Built:');
    });
  });

  describe('when in error state without version data', () => {
    beforeEach(async () => {
      el.loading = false;
      el.error = new Error('Failed to load version info');
      el.version = undefined;

      await Promise.race([
        el.updateComplete,
        timeout(5000, "Component update timed out"),
      ]);
    });

    it('should display error message', () => {
      const errorElement = el.shadowRoot?.querySelector('.error');
      expect(errorElement?.textContent).to.equal('Failed to load version info');
    });

    it('should not display version rows', () => {
      const versionRows = el.shadowRoot?.querySelectorAll('.version-row');
      expect(versionRows).to.have.length(0);
    });
  });

  describe('when displaying version information', () => {
    describe('when version has complete data', () => {
      beforeEach(async () => {
  
        const mockTimestamp = create(TimestampSchema, {
          seconds: BigInt(Math.floor(new Date('2023-06-15T14:30:00Z').getTime() / 1000)),
          nanos: 0
        });

        el.loading = false;
        el.error = undefined;
        el.version = create(GetVersionResponseSchema, {
          commit: 'abcdef1234567890',
          buildTime: mockTimestamp
        });

  
        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should display truncated commit hash', () => {
        const commitValue = el.shadowRoot?.querySelector('.version-row:first-child .value');
        expect(commitValue?.textContent).to.equal('abcdef1');
      });

      it('should display formatted build time', () => {
        const buildTimeValue = el.shadowRoot?.querySelector('.version-row:last-child .value');
        expect(buildTimeValue?.textContent).to.contain('Jun 15, 2023');
      });

      it('should not display loading state', () => {
        const loadingElements = el.shadowRoot?.querySelectorAll('.loading');
        expect(loadingElements).to.have.length(0);
      });

      it('should not display error state', () => {
        const errorElement = el.shadowRoot?.querySelector('.error');
        expect(errorElement).to.not.exist;
      });
    });

    describe('when version has tagged commit', () => {
      beforeEach(async () => {
  
        const mockTimestamp = create(TimestampSchema, {
          seconds: BigInt(Math.floor(new Date('2023-06-15T14:30:00Z').getTime() / 1000)),
          nanos: 0
        });

        el.loading = false;
        el.error = undefined;
        el.version = create(GetVersionResponseSchema, {
          commit: 'v1.2.3 (abcdef1)',
          buildTime: mockTimestamp
        });

  
        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should not truncate tagged version', () => {
        const commitValue = el.shadowRoot?.querySelector('.version-row:first-child .value');
        expect(commitValue?.textContent).to.equal('v1.2.3 (abcdef1)');
      });
    });

    describe('when version has empty commit', () => {
      beforeEach(async () => {
  
        const mockTimestamp = create(TimestampSchema, {
          seconds: BigInt(Math.floor(new Date('2023-06-15T14:30:00Z').getTime() / 1000)),
          nanos: 0
        });

        el.loading = false;
        el.error = undefined;
        el.version = create(GetVersionResponseSchema, {
          commit: '',
          buildTime: mockTimestamp
        });

  
        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should display empty commit value', () => {
        const commitValue = el.shadowRoot?.querySelector('.version-row:first-child .value');
        expect(commitValue?.textContent).to.equal('');
      });
    });

    describe('when version has no buildTime', () => {
      beforeEach(async () => {
  
        el.loading = false;
        el.error = undefined;
        el.version = create(GetVersionResponseSchema, {
          commit: 'abcdef1234567890',
          buildTime: undefined
        });

  
        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should display empty build time value', () => {
        const buildTimeValue = el.shadowRoot?.querySelector('.version-row:last-child .value');
        expect(buildTimeValue?.textContent).to.equal('');
      });
    });
  });

  describe('formatCommit method', () => {
    describe('when commit is a long hash', () => {
      let result: string;

      beforeEach(() => {
  
        const longCommit = 'abcdefghijklmnopqrstuvwxyz123456789';

  
        result = el['formatCommit'](longCommit);
      });

      it('should truncate to 7 characters', () => {
        expect(result).to.equal('abcdefg');
      });
    });

    describe('when commit is exactly 7 characters', () => {
      let result: string;

      beforeEach(() => {
  
        const sevenCharCommit = 'abc1234';

  
        result = el['formatCommit'](sevenCharCommit);
      });

      it('should not truncate', () => {
        expect(result).to.equal('abc1234');
      });
    });

    describe('when commit is shorter than 7 characters', () => {
      let result: string;

      beforeEach(() => {
  
        const shortCommit = 'abc12';

  
        result = el['formatCommit'](shortCommit);
      });

      it('should not truncate', () => {
        expect(result).to.equal('abc12');
      });
    });

    describe('when commit contains parentheses', () => {
      describe('when commit is a simple tagged version', () => {
        let result: string;

        beforeEach(() => {
    
          const taggedCommit = 'v1.2.3 (abc1234)';

    
          result = el['formatCommit'](taggedCommit);
        });

        it('should not truncate tagged version', () => {
          expect(result).to.equal('v1.2.3 (abc1234)');
        });
      });

      describe('when commit is a complex tagged version', () => {
        let result: string;

        beforeEach(() => {
    
          const complexTaggedCommit = 'v10.20.30-beta.1 (abcdefghijklmnop)';

    
          result = el['formatCommit'](complexTaggedCommit);
        });

        it('should not truncate complex tagged version', () => {
          expect(result).to.equal('v10.20.30-beta.1 (abcdefghijklmnop)');
        });
      });

      describe('when commit has only opening parenthesis', () => {
        let result: string;

        beforeEach(() => {
    
          const incompleteParens = 'v1.2.3 (abc1234567890';

    
          result = el['formatCommit'](incompleteParens);
        });

        it('should truncate because it lacks closing parenthesis', () => {
          expect(result).to.equal('v1.2.3 ');
        });
      });

      describe('when commit has only closing parenthesis', () => {
        let result: string;

        beforeEach(() => {
    
          const incompleteParens = 'v1.2.3 abc1234567890)';

    
          result = el['formatCommit'](incompleteParens);
        });

        it('should truncate because it lacks opening parenthesis', () => {
          expect(result).to.equal('v1.2.3 ');
        });
      });
    });

    describe('when commit is empty string', () => {
      let result: string;

      beforeEach(() => {
  
        const emptyCommit = '';

  
        result = el['formatCommit'](emptyCommit);
      });

      it('should return empty string', () => {
        expect(result).to.equal('');
      });
    });
  });

  describe('formatTimestamp method', () => {
    describe('when timestamp represents a valid date', () => {
      let result: string;

      beforeEach(() => {
  
        const timestamp = create(TimestampSchema, {
          seconds: BigInt(Math.floor(new Date('2023-12-25T15:45:30Z').getTime() / 1000)),
          nanos: 0
        });

  
        result = el['formatTimestamp'](timestamp);
      });

      it('should format with month, day, year', () => {
        expect(result).to.contain('Dec 25, 2023');
      });

      it('should format with time', () => {
        // Time will be in local timezone, so we just check that time is present
        expect(result).to.match(/\d{1,2}:\d{2} [AP]M/);
      });
    });

    describe('when timestamp represents beginning of epoch', () => {
      let result: string;

      beforeEach(() => {
  
        const timestamp = create(TimestampSchema, {
          seconds: BigInt(0),
          nanos: 0
        });

  
        result = el['formatTimestamp'](timestamp);
      });

      it('should format epoch date correctly', () => {
        // Epoch date might show as Dec 31, 1969 in some timezones
        expect(result).to.match(/(Dec 31, 1969|Jan 1, 1970)/);
      });
    });

    describe('when timestamp has nanoseconds', () => {
      let result: string;

      beforeEach(() => {
  
        const timestamp = create(TimestampSchema, {
          seconds: BigInt(Math.floor(new Date('2023-06-15T14:30:00Z').getTime() / 1000)),
          nanos: 500000000 // 0.5 seconds
        });

  
        result = el['formatTimestamp'](timestamp);
      });

      it('should ignore nanoseconds and format correctly', () => {
        expect(result).to.contain('Jun 15, 2023');
        // Time will be in local timezone, so we just check that time is present
        expect(result).to.match(/\d{1,2}:\d{2} [AP]M/);
      });
    });
  });

  describe('component structure', () => {
    beforeEach(async () => {

      const mockTimestamp = create(TimestampSchema, {
        seconds: BigInt(Math.floor(new Date('2023-06-15T14:30:00Z').getTime() / 1000)),
        nanos: 0
      });

      el.loading = false;
      el.error = undefined;
      el.version = create(GetVersionResponseSchema, {
        commit: 'abcdef1234567890',
        buildTime: mockTimestamp
      });
      await Promise.race([
        el.updateComplete,
        timeout(5000, "Component update timed out"),
      ]);
    });

    it('should have version-info container', () => {
      const versionInfo = el.shadowRoot?.querySelector('.version-info');
      expect(versionInfo).to.exist;
    });

    it('should have two version-row elements', () => {
      const versionRows = el.shadowRoot?.querySelectorAll('.version-row');
      expect(versionRows).to.have.length(2);
    });

    it('should have labels with correct classes', () => {
      const labels = el.shadowRoot?.querySelectorAll('.label');
      expect(labels).to.have.length(2);
      labels?.forEach(label => {
        expect(label.classList.contains('label')).to.be.true;
      });
    });

    it('should have values with correct classes', () => {
      const values = el.shadowRoot?.querySelectorAll('.value');
      expect(values).to.have.length(2);
      values?.forEach(value => {
        expect(value.classList.contains('value')).to.be.true;
      });
    });

    it('should have commit value with commit class', () => {
      const commitValue = el.shadowRoot?.querySelector('.version-row:first-child .value');
      expect(commitValue?.classList.contains('commit')).to.be.true;
    });
  });

  describe('error handling edge cases', () => {
    describe('when loading is true but version exists', () => {
      beforeEach(async () => {
  
        const mockTimestamp = create(TimestampSchema, {
          seconds: BigInt(Math.floor(new Date('2023-06-15T14:30:00Z').getTime() / 1000)),
          nanos: 0
        });

        el.loading = true;
        el.error = undefined;
        el.version = create(GetVersionResponseSchema, {
          commit: 'abcdef1234567890',
          buildTime: mockTimestamp
        });

  
        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should display version information instead of loading', () => {
        const commitValue = el.shadowRoot?.querySelector('.version-row:first-child .value');
        expect(commitValue?.textContent).to.equal('abcdef1');
        
        const loadingElements = el.shadowRoot?.querySelectorAll('.loading');
        expect(loadingElements).to.have.length(0);
      });
    });

    describe('when error exists but version also exists', () => {
      beforeEach(async () => {
  
        const mockTimestamp = create(TimestampSchema, {
          seconds: BigInt(Math.floor(new Date('2023-06-15T14:30:00Z').getTime() / 1000)),
          nanos: 0
        });

        el.loading = false;
        el.error = new Error('Some error');
        el.version = create(GetVersionResponseSchema, {
          commit: 'abcdef1234567890',
          buildTime: mockTimestamp
        });

  
        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should display version information instead of error', () => {
        const commitValue = el.shadowRoot?.querySelector('.version-row:first-child .value');
        expect(commitValue?.textContent).to.equal('abcdef1');
        
        const errorElement = el.shadowRoot?.querySelector('.error');
        expect(errorElement).to.not.exist;
      });
    });
  });
});