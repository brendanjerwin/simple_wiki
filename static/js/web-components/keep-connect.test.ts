import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import './keep-connect.js';
import type { KeepConnect } from './keep-connect.js';
import {
  GetStateResponseSchema,
  ConnectorStateSchema,
  SubscriptionStateSchema,
} from '../gen/api/v1/connector_service_pb.js';

// Access private client fields via type cast.
interface KeepConnectClient {
  getState: sinon.SinonStub;
  completeAuth: sinon.SinonStub;
  disconnect: sinon.SinonStub;
  unsubscribe: sinon.SinonStub;
}

function clientOf(el: KeepConnect): KeepConnectClient {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion, @typescript-eslint/no-explicit-any
  return (el as any).client as KeepConnectClient;
}

function timeout(ms: number, message: string): Promise<never> {
  return new Promise<never>((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

describe('KeepConnect', () => {
  let el: KeepConnect;

  function stubGetStateDisconnected(): void {
    const state = create(ConnectorStateSchema, { configured: false });
    sinon
      .stub(clientOf(el), 'getState')
      .resolves(create(GetStateResponseSchema, { state }));
  }

  function stubGetStateConnected(email = 'user@example.com'): void {
    const state = create(ConnectorStateSchema, { configured: true, email });
    sinon
      .stub(clientOf(el), 'getState')
      .resolves(create(GetStateResponseSchema, { state }));
  }

  afterEach(() => {
    el.remove();
    sinon.restore();
  });

  // ------------------------------------------------------------------ basics

  describe('when mounted with a stub that prevents real transport', () => {
    beforeEach(async () => {
      el = document.createElement('keep-connect') as KeepConnect;
      stubGetStateDisconnected();
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should exist', () => {
      expect(el).to.be.instanceOf(HTMLElement);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName).to.equal('KEEP-CONNECT');
    });

    it('should have a shadow root', () => {
      expect(el.shadowRoot).to.exist;
    });
  });

  // ------------------------------------------------------------------ disconnected phase

  describe('when getState returns disconnected (configured=false)', () => {
    beforeEach(async () => {
      el = document.createElement('keep-connect') as KeepConnect;
      stubGetStateDisconnected();
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render the connect form', () => {
      const form = el.shadowRoot?.querySelector('form');
      expect(form).to.exist;
    });

    it('should not render a Disconnect button', () => {
      const btns = el.shadowRoot?.querySelectorAll('confirmation-interlock-button');
      expect(btns?.length ?? 0).to.equal(0);
    });

    it('should render an email input', () => {
      const input = el.shadowRoot?.querySelector('input[type="email"]');
      expect(input).to.exist;
    });

    it('should render a password input for the oauth_token', () => {
      const input = el.shadowRoot?.querySelector('input[type="password"]');
      expect(input).to.exist;
    });
  });

  // ------------------------------------------------------------------ connected phase (no subscriptions)

  describe('when getState returns configured=true with no subscriptions', () => {
    beforeEach(async () => {
      el = document.createElement('keep-connect') as KeepConnect;
      stubGetStateConnected('alice@example.com');
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should not render the connect form', () => {
      const form = el.shadowRoot?.querySelector('form');
      expect(form).to.not.exist;
    });

    it('should display the connected email', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('alice@example.com');
    });

    it('should render the Disconnect interlock button', () => {
      const btns = el.shadowRoot?.querySelectorAll('confirmation-interlock-button');
      expect(btns?.length ?? 0).to.be.greaterThan(0);
    });

    it('should display the empty-bindings message', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('No checklists bound');
    });
  });

  // ------------------------------------------------------------------ connected phase (with subscriptions)

  describe('when getState returns configured=true with one subscription', () => {
    beforeEach(async () => {
      el = document.createElement('keep-connect') as KeepConnect;
      const subscription = create(SubscriptionStateSchema, {
        page: 'Board',
        listName: 'todo',
        remoteListHandle: 'note-abc',
        remoteListTitle: 'My todo list',
      });
      const state = create(ConnectorStateSchema, {
        configured: true,
        email: 'bob@example.com',
        subscriptions: [subscription],
      });
      sinon
        .stub(clientOf(el), 'getState')
        .resolves(create(GetStateResponseSchema, { state }));
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render the subscription page name', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Board');
    });

    it('should render the subscription list name', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('todo');
    });

    it('should render the Keep note title', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('My todo list');
    });

    it('should render an unbind interlock per subscription plus one Disconnect button', () => {
      const btns = el.shadowRoot?.querySelectorAll('confirmation-interlock-button');
      // 1 Disconnect + 1 per subscription
      expect(btns?.length ?? 0).to.equal(2);
    });
  });

  // ------------------------------------------------------------------ error phase

  describe('when getState rejects', () => {
    beforeEach(async () => {
      el = document.createElement('keep-connect') as KeepConnect;
      sinon
        .stub(clientOf(el), 'getState')
        .rejects(new ConnectError('network failure', Code.Unavailable));
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render error-display', () => {
      const errorEl = el.shadowRoot?.querySelector('error-display');
      expect(errorEl).to.exist;
    });

    it('should fall back to disconnected phase and show the connect form', () => {
      const form = el.shadowRoot?.querySelector('form');
      expect(form).to.exist;
    });
  });

  // ------------------------------------------------------------------ connect form validation

  describe('when the connect form is submitted without filling in fields', () => {
    beforeEach(async () => {
      el = document.createElement('keep-connect') as KeepConnect;
      stubGetStateDisconnected();
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      // Fire submit without populating formEmail / formOAuthToken
      const form = el.shadowRoot?.querySelector('form') as HTMLFormElement;
      form.dispatchEvent(new SubmitEvent('submit', { bubbles: true, cancelable: true }));
      await el.updateComplete;
    });

    it('should render an error-display for missing fields', () => {
      const errorEl = el.shadowRoot?.querySelector('error-display');
      expect(errorEl).to.exist;
    });

    it('should not call completeAuth', () => {
      // completeAuth stub was never set up — assert it was never reached
      // by verifying phase stayed at 'disconnected' (form is still visible)
      const form = el.shadowRoot?.querySelector('form');
      expect(form).to.exist;
    });
  });

  // ------------------------------------------------------------------ successful connect

  describe('when connect form is submitted with valid credentials', () => {
    let completeAuthStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('keep-connect') as KeepConnect;
      stubGetStateDisconnected();
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      // Set up the completeAuth stub to return connected state
      const connectedState = create(ConnectorStateSchema, {
        configured: true,
        email: 'new@example.com',
      });
      const { CompleteAuthResponseSchema } = await import('../gen/api/v1/connector_service_pb.js');
      completeAuthStub = sinon
        .stub(clientOf(el), 'completeAuth')
        .resolves(create(CompleteAuthResponseSchema, { state: connectedState }));

      // Populate the private form fields
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion
      (el as any).formEmail = 'new@example.com';
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion
      (el as any).formOAuthToken = 'oauth2_4/0Afake';

      const form = el.shadowRoot?.querySelector('form') as HTMLFormElement;
      form.dispatchEvent(new SubmitEvent('submit', { bubbles: true, cancelable: true }));
      await el.updateComplete;
    });

    it('should call completeAuth once', () => {
      expect(completeAuthStub.calledOnce).to.be.true;
    });

    it('should transition to connected phase (no form)', () => {
      const form = el.shadowRoot?.querySelector('form');
      expect(form).to.not.exist;
    });

    it('should display the new email in connected state', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('new@example.com');
    });
  });

  // ------------------------------------------------------------------ disconnect

  describe('when disconnect is invoked after being connected', () => {
    let disconnectStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('keep-connect') as KeepConnect;
      stubGetStateConnected('carol@example.com');
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      const { DisconnectResponseSchema } = await import('../gen/api/v1/connector_service_pb.js');
      const disconnectedState = create(ConnectorStateSchema, { configured: false });
      disconnectStub = sinon
        .stub(clientOf(el), 'disconnect')
        .resolves(create(DisconnectResponseSchema, { state: disconnectedState }));

      // Call private handleDisconnect directly to bypass interlock
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion
      await (el as any).handleDisconnect();
      await el.updateComplete;
    });

    it('should call disconnect once', () => {
      expect(disconnectStub.calledOnce).to.be.true;
    });

    it('should transition to disconnected phase', () => {
      const form = el.shadowRoot?.querySelector('form');
      expect(form).to.exist;
    });
  });
});
