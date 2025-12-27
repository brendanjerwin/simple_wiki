import { html, fixture, expect, assert } from '@open-wc/testing';
import { stub } from 'sinon';
import { SystemInfoIdentity } from './system-info-identity.js';
import { TailscaleIdentity } from '../gen/api/v1/system_info_pb.js';
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

    el = await Promise.race([
      fixture(html`<system-info-identity></system-info-identity>`),
      timeout(5000, "SystemInfoIdentity fixture timed out"),
    ]);
  });

  afterEach(() => {
    if (fetchStub) {
      fetchStub.restore();
    }
  });

  it('should exist', () => {
    assert.instanceOf(el, SystemInfoIdentity);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName).to.equal('SYSTEM-INFO-IDENTITY');
  });

  it('should have undefined identity initially', () => {
    expect(el.identity).to.be.undefined;
  });

  describe('when no identity is provided', () => {
    beforeEach(async () => {
      el.identity = undefined;
      await Promise.race([
        el.updateComplete,
        timeout(5000, "Component update timed out"),
      ]);
    });

    it('should render nothing', () => {
      // The component should render nothing (empty shadow root content)
      const content = el.shadowRoot?.querySelector('.identity-info');
      expect(content).to.not.exist;
    });
  });

  describe('when identity has empty loginName', () => {
    beforeEach(async () => {
      el.identity = new TailscaleIdentity({
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
      expect(content).to.not.exist;
    });
  });

  describe('when identity has login name only', () => {
    beforeEach(async () => {
      el.identity = new TailscaleIdentity({
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
      expect(container).to.exist;
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
      el.identity = new TailscaleIdentity({
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
      el.identity = new TailscaleIdentity({
        loginName: 'user@example.com',
        displayName: 'Test User',
        nodeName: 'my-laptop'
      });
      await Promise.race([
        el.updateComplete,
        timeout(5000, "Component update timed out"),
      ]);
    });

    it('should display two identity rows', () => {
      const identityRows = el.shadowRoot?.querySelectorAll('.identity-row');
      expect(identityRows).to.have.length(2);
    });

    it('should display Node label', () => {
      const labels = el.shadowRoot?.querySelectorAll('.label');
      expect(labels?.[1]?.textContent).to.equal('Node:');
    });

    it('should display node name', () => {
      const values = el.shadowRoot?.querySelectorAll('.value');
      expect(values?.[1]?.textContent).to.equal('my-laptop');
    });
  });

  describe('when identity has all fields', () => {
    beforeEach(async () => {
      el.identity = new TailscaleIdentity({
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

    it('should display node name in node row', () => {
      const nodeValue = el.shadowRoot?.querySelector('.identity-row:last-child .value');
      expect(nodeValue?.textContent).to.equal('my-laptop');
    });
  });

  describe('component structure', () => {
    beforeEach(async () => {
      el.identity = new TailscaleIdentity({
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
      expect(container).to.exist;
    });

    it('should have identity-row elements', () => {
      const rows = el.shadowRoot?.querySelectorAll('.identity-row');
      expect(rows).to.have.length(2);
    });

    it('should have labels with label class', () => {
      const labels = el.shadowRoot?.querySelectorAll('.label');
      expect(labels).to.have.length(2);
    });

    it('should have values with value class', () => {
      const values = el.shadowRoot?.querySelectorAll('.value');
      expect(values).to.have.length(2);
    });
  });
});
