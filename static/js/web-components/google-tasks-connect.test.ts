import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import './google-tasks-connect.js';
import type { GoogleTasksConnect } from './google-tasks-connect.js';
import {
  GetStateResponseSchema,
  ConnectorStateSchema,
  BeginAuthResponseSchema,
  DisconnectResponseSchema,
  BindingStateSchema,
  UnbindResponseSchema,
} from '../gen/api/v1/connector_service_pb.js';

interface GoogleTasksConnectClient {
  getState: sinon.SinonStub;
  beginAuth: sinon.SinonStub;
  disconnect: sinon.SinonStub;
  unbind: sinon.SinonStub;
}

function clientOf(el: GoogleTasksConnect): GoogleTasksConnectClient {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion, @typescript-eslint/no-explicit-any
  return (el as any).client as GoogleTasksConnectClient;
}

function timeout(ms: number, message: string): Promise<never> {
  return new Promise<never>((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

describe('GoogleTasksConnect', () => {
  let el: GoogleTasksConnect;

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
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      stubGetStateDisconnected();
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should exist', () => {
      expect(el).to.be.instanceOf(HTMLElement);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName).to.equal('GOOGLE-TASKS-CONNECT');
    });

    it('should have a shadow root', () => {
      expect(el.shadowRoot).to.exist;
    });
  });

  // ------------------------------------------------------------------ disconnected phase

  describe('when getState returns disconnected (configured=false)', () => {
    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      stubGetStateDisconnected();
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render an enabled Connect button', () => {
      const btn = el.shadowRoot?.querySelector('button');
      expect(btn).to.exist;
      expect(btn?.disabled).to.be.false;
    });

    it('should not render a Disconnect interlock button', () => {
      const btns = el.shadowRoot?.querySelectorAll('confirmation-interlock-button');
      expect(btns?.length ?? 0).to.equal(0);
    });

    it('should mention Google Tasks in the body copy', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Google Tasks');
    });
  });

  // ------------------------------------------------------------------ connected phase

  describe('when getState returns configured=true with an email', () => {
    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      stubGetStateConnected('alice@example.com');
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should not render the Connect button', () => {
      // In the connected phase the only button is the Disconnect
      // interlock — there should be no plain <button> for Connect.
      const btn = el.shadowRoot?.querySelector('button');
      expect(btn).to.not.exist;
    });

    it('should display the connected email', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('alice@example.com');
    });

    it('should render a Disconnect interlock button', () => {
      const btns = el.shadowRoot?.querySelectorAll('confirmation-interlock-button');
      expect(btns?.length ?? 0).to.equal(1);
    });
  });

  // ------------------------------------------------------------------ connected phase (legacy empty-email state)

  describe('when getState returns configured=true with an empty email (legacy state)', () => {
    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      stubGetStateConnected('');
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should NOT render the half-rendered "Connected as ." with empty value', () => {
      const text = el.shadowRoot?.textContent ?? '';
      // Invariant: the literal "Connected as ." (period after empty email)
      // is the bug we are fixing; it must never appear.
      expect(text).to.not.match(/Connected as\s*\./);
    });

    it('should surface the gap with a reconnect prompt', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text.toLowerCase()).to.include('email is missing');
    });

    it('should still render the Disconnect interlock so the user can recover', () => {
      const btns = el.shadowRoot?.querySelectorAll('confirmation-interlock-button');
      expect(btns?.length ?? 0).to.equal(1);
    });
  });

  // ------------------------------------------------------------------ connected phase (no subscriptions)

  describe('when getState returns configured=true with no subscriptions', () => {
    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      stubGetStateConnected('alice@example.com');
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should display the empty-bindings message', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('No checklists bound');
    });
  });

  // ------------------------------------------------------------------ connected phase (with subscriptions)

  describe('when getState returns configured=true with one subscription', () => {
    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      const subscription = create(BindingStateSchema, {
        page: 'Board',
        listName: 'todo',
        remoteListHandle: 'tasklist-abc',
        remoteListTitle: 'My todo list',
      });
      const state = create(ConnectorStateSchema, {
        configured: true,
        email: 'bob@example.com',
        bindings: [subscription],
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

    it('should render the Tasks list title', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('My todo list');
    });

    it('should use the "Bound to ... Tasks list" vocabulary', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Tasks list');
    });

    it('should render an unbind interlock per subscription plus one Disconnect button', () => {
      const btns = el.shadowRoot?.querySelectorAll('confirmation-interlock-button');
      // 1 Disconnect + 1 per subscription
      expect(btns?.length ?? 0).to.equal(2);
    });
  });

  // ------------------------------------------------------------------ paused subscription → reconnect banner CTA

  describe('when getState returns configured=true with a paused subscription', () => {
    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      const subscription = create(BindingStateSchema, {
        page: 'shopping',
        listName: 'this_week',
        remoteListHandle: 'tasklist-xyz',
        remoteListTitle: 'This Week',
        paused: true,
      });
      const state = create(ConnectorStateSchema, {
        configured: true,
        email: 'paused@example.com',
        bindings: [subscription],
      });
      sinon
        .stub(clientOf(el), 'getState')
        .resolves(create(GetStateResponseSchema, { state }));
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render a reconnect-banner', () => {
      const banner = el.shadowRoot?.querySelector('.reconnect-banner');
      expect(banner).to.not.be.null;
    });

    it('should explain *why* reconnection is needed in the banner copy', () => {
      const text = el.shadowRoot?.querySelector('.reconnect-banner')?.textContent ?? '';
      expect(text).to.match(/refresh|revoked|authorization|expired/i);
    });

    it('should render a prominent Reconnect button inside the banner', () => {
      const btn = el.shadowRoot?.querySelector('.reconnect-banner button.reconnect-btn');
      expect(btn?.textContent ?? '').to.include('Reconnect');
    });

    it('should mention bindings will resume automatically (auto-resume on reconnect)', () => {
      const text = el.shadowRoot?.querySelector('.reconnect-banner')?.textContent ?? '';
      expect(text).to.match(/automatic|resume/i);
    });
  });

  describe('when getState returns configured=true with no paused subscriptions', () => {
    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      const subscription = create(BindingStateSchema, {
        page: 'shopping',
        listName: 'weekly',
        remoteListHandle: 'tasklist-abc',
        remoteListTitle: 'Weekly',
        paused: false,
      });
      const state = create(ConnectorStateSchema, {
        configured: true,
        email: 'healthy@example.com',
        bindings: [subscription],
      });
      sinon
        .stub(clientOf(el), 'getState')
        .resolves(create(GetStateResponseSchema, { state }));
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should NOT render a reconnect-banner (no paused subscriptions)', () => {
      const banner = el.shadowRoot?.querySelector('.reconnect-banner');
      expect(banner).to.be.null;
    });
  });

  describe('when the Reconnect button in the paused banner is clicked', () => {
    let beginAuthStub: sinon.SinonStub;
    let redirectSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      const subscription = create(BindingStateSchema, {
        page: 'shopping',
        listName: 'this_week',
        remoteListHandle: 'tasklist-xyz',
        remoteListTitle: 'This Week',
        paused: true,
      });
      const state = create(ConnectorStateSchema, {
        configured: true,
        email: 'paused@example.com',
        bindings: [subscription],
      });
      sinon
        .stub(clientOf(el), 'getState')
        .resolves(create(GetStateResponseSchema, { state }));
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      beginAuthStub = sinon
        .stub(clientOf(el), 'beginAuth')
        .resolves(
          create(BeginAuthResponseSchema, {
            authorizationUrl: 'https://accounts.google.com/o/oauth2/auth?client_id=fake',
            state: 'opaque-state-token',
          }),
        );

      redirectSpy = sinon.spy();
      el.redirect = redirectSpy;

      const btn = el.shadowRoot?.querySelector(
        '.reconnect-banner button.reconnect-btn',
      ) as HTMLButtonElement | null;
      btn?.click();
      await el.updateComplete;
      await el.updateComplete;
    });

    it('should call beginAuth once', () => {
      expect(beginAuthStub.calledOnce).to.be.true;
    });

    it('should redirect to the OAuth authorization URL', () => {
      expect(redirectSpy.calledOnce).to.be.true;
      expect(redirectSpy.firstCall.args[0]).to.equal(
        'https://accounts.google.com/o/oauth2/auth?client_id=fake',
      );
    });
  });

  // ------------------------------------------------------------------ unsubscribe

  describe('when handleUnbind is invoked for a subscription', () => {
    let unbindStub: sinon.SinonStub;
    let getStateStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      const subscription = create(BindingStateSchema, {
        page: 'Board',
        listName: 'todo',
        remoteListHandle: 'tasklist-abc',
        remoteListTitle: 'My todo list',
      });
      const state = create(ConnectorStateSchema, {
        configured: true,
        email: 'bob@example.com',
        bindings: [subscription],
      });
      getStateStub = sinon
        .stub(clientOf(el), 'getState')
        .resolves(create(GetStateResponseSchema, { state }));
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      unbindStub = sinon
        .stub(clientOf(el), 'unbind')
        .resolves(create(UnbindResponseSchema, {}));

      // Call private handleUnbind directly to bypass the interlock.
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion
      await (el as any).handleUnbind(subscription);
      await el.updateComplete;
    });

    it('should call unsubscribe once', () => {
      expect(unbindStub.calledOnce).to.be.true;
    });

    it('should call unsubscribe with the page and listName', () => {
      const arg = unbindStub.firstCall.args[0] as { page: string; listName: string };
      expect(arg.page).to.equal('Board');
      expect(arg.listName).to.equal('todo');
    });

    it('should refresh state after unsubscribe', () => {
      // initial mount + post-unsubscribe refresh
      expect(getStateStub.callCount).to.equal(2);
    });
  });

  // ------------------------------------------------------------------ unconfigured phase (server signals via FailedPrecondition)

  describe('when Connect is clicked but BeginAuth returns FailedPrecondition', () => {
    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      stubGetStateDisconnected();
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      sinon
        .stub(clientOf(el), 'beginAuth')
        .rejects(
          new ConnectError(
            "google tasks integration is not configured by this wiki's operator",
            Code.FailedPrecondition,
          ),
        );

      const btn = el.shadowRoot?.querySelector('button') as HTMLButtonElement;
      btn.click();
      // Allow the rejected promise + state update to flush.
      await el.updateComplete;
      await el.updateComplete;
    });

    it('should render a disabled Connect button', () => {
      const btn = el.shadowRoot?.querySelector('button') as HTMLButtonElement | null;
      expect(btn).to.exist;
      expect(btn?.disabled).to.be.true;
    });

    it('should point the user at the help page', () => {
      const link = el.shadowRoot?.querySelector('a[href="/help_google_tasks/view"]');
      expect(link).to.exist;
    });

    it('should not render an error-display (unconfigured is a known state, not an error)', () => {
      const errorEl = el.shadowRoot?.querySelector('error-display');
      expect(errorEl).to.not.exist;
    });
  });

  // ------------------------------------------------------------------ connect happy path (BeginAuth returns auth URL)

  describe('when Connect is clicked and BeginAuth returns an authorization URL', () => {
    let beginAuthStub: sinon.SinonStub;
    let redirectSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      stubGetStateDisconnected();
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      beginAuthStub = sinon
        .stub(clientOf(el), 'beginAuth')
        .resolves(
          create(BeginAuthResponseSchema, {
            authorizationUrl: 'https://accounts.google.com/o/oauth2/auth?client_id=fake',
            state: 'opaque-state-token',
          }),
        );

      // Replace the redirect seam so we don't actually navigate the test runner.
      redirectSpy = sinon.spy();
      el.redirect = redirectSpy;

      const btn = el.shadowRoot?.querySelector('button') as HTMLButtonElement;
      btn.click();
      await el.updateComplete;
      await el.updateComplete;
    });

    it('should call beginAuth once', () => {
      expect(beginAuthStub.calledOnce).to.be.true;
    });

    it('should redirect to the returned authorization URL', () => {
      expect(redirectSpy.calledOnce).to.be.true;
      expect(redirectSpy.firstCall.args[0]).to.equal(
        'https://accounts.google.com/o/oauth2/auth?client_id=fake',
      );
    });
  });

  // ------------------------------------------------------------------ connect error path (non-FailedPrecondition)

  describe('when BeginAuth fails with a non-precondition error', () => {
    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      stubGetStateDisconnected();
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      sinon
        .stub(clientOf(el), 'beginAuth')
        .rejects(new ConnectError('network failure', Code.Unavailable));

      const btn = el.shadowRoot?.querySelector('button') as HTMLButtonElement;
      btn.click();
      await el.updateComplete;
      await el.updateComplete;
    });

    it('should render error-display', () => {
      const errorEl = el.shadowRoot?.querySelector('error-display');
      expect(errorEl).to.exist;
    });

    it('should fall back to the disconnected phase so the user can retry', () => {
      const btn = el.shadowRoot?.querySelector('button') as HTMLButtonElement | null;
      expect(btn).to.exist;
      expect(btn?.disabled).to.be.false;
    });
  });

  // ------------------------------------------------------------------ disconnect

  describe('when handleDisconnect is invoked from the connected phase', () => {
    let disconnectStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
      stubGetStateConnected('carol@example.com');
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      const disconnectedState = create(ConnectorStateSchema, { configured: false });
      disconnectStub = sinon
        .stub(clientOf(el), 'disconnect')
        .resolves(create(DisconnectResponseSchema, { state: disconnectedState }));

      // Call private handleDisconnect directly to bypass the interlock.
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion
      await (el as any).handleDisconnect();
      await el.updateComplete;
    });

    it('should call disconnect once', () => {
      expect(disconnectStub.calledOnce).to.be.true;
    });

    it('should transition to the disconnected phase (Connect button visible)', () => {
      const btn = el.shadowRoot?.querySelector('button') as HTMLButtonElement | null;
      expect(btn).to.exist;
      expect(btn?.disabled).to.be.false;
    });
  });

  // ------------------------------------------------------------------ getState error

  describe('when getState rejects', () => {
    beforeEach(async () => {
      el = document.createElement('google-tasks-connect') as GoogleTasksConnect;
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

    it('should fall back to the disconnected phase', () => {
      const btn = el.shadowRoot?.querySelector('button') as HTMLButtonElement | null;
      expect(btn).to.exist;
      expect(btn?.disabled).to.be.false;
    });
  });
});
