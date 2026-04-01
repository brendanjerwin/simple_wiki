import { html, fixture, expect } from '@open-wc/testing';
import { stub } from 'sinon';
import { create } from '@bufbuild/protobuf';
import { SystemInfoIdentity } from './system-info-identity.js';
import { TailscaleIdentitySchema } from '../gen/api/v1/system_info_pb.js';
import './system-info-identity.js';

function timeout(ms: number, message: string) {
  return new Promise((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

describe('SystemInfoIdentity', () => {
  let el: SystemInfoIdentity;
  let fetchStub: ReturnType<typeof stub>;

  beforeEach(async () => {
    // Prevent any potential network calls that could cause hanging
    fetchStub = stub(window, 'fetch');
    fetchStub.resolves(new Response('{}'));

    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- fixture returns unknown when using Promise.race
    el = await Promise.race([
      fixture<SystemInfoIdentity>(html`<system-info-identity></system-info-identity>`),
      timeout(5000, "SystemInfoIdentity fixture timed out"),
    ]) as SystemInfoIdentity;
  });

  afterEach(() => {
    if (fetchStub) {
      fetchStub.restore();
    }
  });

  it('should exist', () => {
    expect(el).to.be.instanceOf(SystemInfoIdentity);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName).to.equal('SYSTEM-INFO-IDENTITY');
  });

  it('should have undefined identity initially', () => {
    expect(el.identity).to.equal(undefined);
  });

  describe('when no identity is provided', () => {
    beforeEach(async () => {
      delete el.identity;
      await Promise.race([
        el.updateComplete,
        timeout(5000, "Component update timed out"),
      ]);
    });

    it('should render nothing', () => {
      // The component should render nothing (empty shadow root content)
      const content = el.shadowRoot?.querySelector('.identity-info');
      expect(content).to.equal(null);
    });
  });

  describe('when identity has empty loginName', () => {
    beforeEach(async () => {
      el.identity = create(TailscaleIdentitySchema, {
        loginName: '',
        displayName: '',
        nodeName: ''
      });
      await Promise.race([
        el.updateComplete,
        timeout(5000, "Component update timed out"),
      ]);
    });

    it('should render nothing', () => {
      const content = el.shadowRoot?.querySelector('.identity-info');
      expect(content).to.equal(null);
    });
  });

  describe('when identity has login name only', () => {
    beforeEach(async () => {
      el.identity = create(TailscaleIdentitySchema, {
        loginName: 'user@example.com',
        displayName: '',
        nodeName: ''
      });
      await Promise.race([
        el.updateComplete,
        timeout(5000, "Component update timed out"),
      ]);
    });

    it('should display identity info container', () => {
      const container = el.shadowRoot?.querySelector('.identity-info');
      expect(container).to.not.equal(null);
    });

    it('should display User label', () => {
      const label = el.shadowRoot?.querySelector('.identity-row .label');
      expect(label?.textContent).to.equal('User:');
    });

    it('should display login name as user value', () => {
      const value = el.shadowRoot?.querySelector('.identity-row .value');
      expect(value?.textContent).to.equal('user@example.com');
    });

    it('should not display node row', () => {
      const identityRows = el.shadowRoot?.querySelectorAll('.identity-row');
      expect(identityRows).to.have.length(1);
    });
  });

  describe('when identity has display name', () => {
    beforeEach(async () => {
      el.identity = create(TailscaleIdentitySchema, {
        loginName: 'user@example.com',
        displayName: 'Test User',
        nodeName: ''
      });
      await Promise.race([
        el.updateComplete,
        timeout(5000, "Component update timed out"),
      ]);
    });

    it('should display display name instead of login name', () => {
      const value = el.shadowRoot?.querySelector('.identity-row .value');
      expect(value?.textContent).to.equal('Test User');
    });
  });

  describe('when identity has node name', () => {
    beforeEach(async () => {
      el.identity = create(TailscaleIdentitySchema, {
        loginName: 'user@example.com',
        displayName: 'Test User',
        nodeName: 'my-laptop'
      });
      await Promise.race([
        el.updateComplete,
        timeout(5000, "Component update timed out"),
      ]);
    });

    it('should display one identity row', () => {
      const identityRows = el.shadowRoot?.querySelectorAll('.identity-row');
      expect(identityRows).to.have.length(1);
    });

    it('should include node name in the row', () => {
      const values = el.shadowRoot?.querySelectorAll('.value');
      const nodeValue = Array.from(values ?? []).find(v => v.textContent?.includes('my-laptop'));
      expect(nodeValue).to.not.equal(null);
    });
  });

  describe('when identity has all fields', () => {
    beforeEach(async () => {
      el.identity = create(TailscaleIdentitySchema, {
        loginName: 'user@example.com',
        displayName: 'Test User',
        nodeName: 'my-laptop'
      });
      await Promise.race([
        el.updateComplete,
        timeout(5000, "Component update timed out"),
      ]);
    });

    it('should display display name in user row', () => {
      const userValue = el.shadowRoot?.querySelector('.identity-row:first-child .value');
      expect(userValue?.textContent).to.equal('Test User');
    });

    it('should include node name', () => {
      const row = el.shadowRoot?.querySelector('.identity-row');
      expect(row?.textContent).to.contain('my-laptop');
    });
  });

  describe('component structure', () => {
    beforeEach(async () => {
      el.identity = create(TailscaleIdentitySchema, {
        loginName: 'user@example.com',
        displayName: 'Test User',
        nodeName: 'my-laptop'
      });
      await Promise.race([
        el.updateComplete,
        timeout(5000, "Component update timed out"),
      ]);
    });

    it('should have identity-info container with correct structure', () => {
      const container = el.shadowRoot?.querySelector('.identity-info');
      expect(container).to.not.equal(null);
    });

    it('should have one identity-row element', () => {
      const rows = el.shadowRoot?.querySelectorAll('.identity-row');
      expect(rows).to.have.length(1);
    });

    it('should have a label', () => {
      const labels = el.shadowRoot?.querySelectorAll('.label');
      expect(labels).to.have.length(1);
    });

    it('should have value elements', () => {
      const values = el.shadowRoot?.querySelectorAll('.value');
      expect(values!.length).to.be.greaterThan(0);
    });
  });
});
