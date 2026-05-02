import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import './connector-subscribe-button.js';
import type { ConnectorSubscribeButton } from './connector-subscribe-button.js';
import {
  ConnectorKind,
  GetChecklistSubscriptionStateResponseSchema,
  ChecklistSubscriptionStateSchema,
  SubscriptionStateSchema,
  ListRemoteListsResponseSchema,
  RemoteListSummarySchema,
  SubscribeResponseSchema,
  UnsubscribeResponseSchema,
  GetStateResponseSchema,
  ConnectorStateSchema,
} from '../gen/api/v1/connector_service_pb.js';

interface SubscribeButtonClient {
  getChecklistSubscriptionState: sinon.SinonStub;
  getState: sinon.SinonStub;
  listRemoteLists: sinon.SinonStub;
  subscribe: sinon.SinonStub;
  unsubscribe: sinon.SinonStub;
}

function clientOf(el: ConnectorSubscribeButton): SubscribeButtonClient {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion, @typescript-eslint/no-explicit-any
  return (el as any).client as SubscribeButtonClient;
}

function timeout(ms: number, message: string): Promise<never> {
  return new Promise<never>((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

function stubAllRpcs(el: ConnectorSubscribeButton): SubscribeButtonClient {
  const client = clientOf(el);
  // Default-stub every RPC so unrelated calls don't hang. Tests override
  // specific methods by re-stubbing after this is called.
  sinon.stub(client, 'getChecklistSubscriptionState');
  sinon.stub(client, 'getState');
  sinon.stub(client, 'listRemoteLists');
  sinon.stub(client, 'subscribe');
  sinon.stub(client, 'unsubscribe');
  return client;
}

function configuredState(currentSubscription?: ReturnType<typeof create>) {
  return create(ChecklistSubscriptionStateSchema, {
    connectorConfigured: true,
    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-explicit-any
    currentSubscription: currentSubscription as any,
  });
}

function authedKindResponse(kind: ConnectorKind, email = 'a@b.test') {
  return create(GetStateResponseSchema, {
    state: create(ConnectorStateSchema, {
      configured: true,
      email,
      connectorKind: kind,
    }),
  });
}

function unauthedKindResponse(kind: ConnectorKind) {
  return create(GetStateResponseSchema, {
    state: create(ConnectorStateSchema, {
      configured: false,
      connectorKind: kind,
    }),
  });
}

describe('ConnectorSubscribeButton', () => {
  let el: ConnectorSubscribeButton;

  afterEach(() => {
    if (el && el.parentNode) el.remove();
    sinon.restore();
  });

  // ------------------------------------------------------------------ no attributes → hidden

  describe('when page and list-name attributes are not set', () => {
    beforeEach(async () => {
      el = document.createElement('connector-subscribe-button') as ConnectorSubscribeButton;
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should exist', () => {
      expect(el).to.be.instanceOf(HTMLElement);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName).to.equal('CONNECTOR-SUBSCRIBE-BUTTON');
    });

    it('should render nothing (phase=hidden)', () => {
      const root = el.shadowRoot;
      const content = root?.innerHTML.trim() ?? '';
      expect(content).to.not.include('<button');
      expect(content).to.not.include('<span');
    });
  });

  // ------------------------------------------------------------------ no connectors configured → hidden

  describe('when no connector is configured', () => {
    beforeEach(async () => {
      el = document.createElement('connector-subscribe-button') as ConnectorSubscribeButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const client = stubAllRpcs(el);
      const state = create(ChecklistSubscriptionStateSchema, { connectorConfigured: false });
      client.getChecklistSubscriptionState.resolves(
        create(GetChecklistSubscriptionStateResponseSchema, { state }),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render nothing (phase=hidden)', () => {
      const content = el.shadowRoot?.innerHTML.trim() ?? '';
      expect(content).to.not.include('<button');
      expect(content).to.not.include('Subscribe');
    });
  });

  // ------------------------------------------------------------------ getChecklistSubscriptionState errors → hidden

  describe('when getChecklistSubscriptionState rejects', () => {
    beforeEach(async () => {
      el = document.createElement('connector-subscribe-button') as ConnectorSubscribeButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const client = stubAllRpcs(el);
      client.getChecklistSubscriptionState.rejects(
        new ConnectError('unavailable', Code.Unavailable),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render nothing (fail-closed behavior)', () => {
      const content = el.shadowRoot?.innerHTML.trim() ?? '';
      expect(content).to.not.include('<button');
    });
  });

  // ------------------------------------------------------------------ unsubscribed state

  describe('when a connector is configured but no subscription exists', () => {
    beforeEach(async () => {
      el = document.createElement('connector-subscribe-button') as ConnectorSubscribeButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const client = stubAllRpcs(el);
      client.getChecklistSubscriptionState.resolves(
        create(GetChecklistSubscriptionStateResponseSchema, { state: configuredState() }),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render the Subscribe trigger button', () => {
      const btn = el.shadowRoot?.querySelector('button.subscribe-trigger');
      expect(btn).to.exist;
    });

    it('should not render an unsubscribe interlock', () => {
      const interlock = el.shadowRoot?.querySelector('confirmation-interlock-button');
      expect(interlock).to.not.exist;
    });

    it('should not render the sync badge', () => {
      const badge = el.shadowRoot?.querySelector('.sync-badge');
      expect(badge).to.not.exist;
    });

    it('should not include the word "connector" in user-facing copy', () => {
      const text = el.shadowRoot?.textContent?.toLowerCase() ?? '';
      expect(text).to.not.include('connector');
    });
  });

  // ------------------------------------------------------------------ subscribed (Keep) state

  describe('when subscribed to a Google Keep note', () => {
    beforeEach(async () => {
      el = document.createElement('connector-subscribe-button') as ConnectorSubscribeButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const client = stubAllRpcs(el);
      const subscription = create(SubscriptionStateSchema, {
        page: 'Board',
        listName: 'todo',
        remoteListHandle: 'note-1',
        remoteListTitle: 'My Keep note',
        connectorKind: ConnectorKind.GOOGLE_KEEP,
      });
      client.getChecklistSubscriptionState.resolves(
        create(GetChecklistSubscriptionStateResponseSchema, {
          state: configuredState(subscription),
        }),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render the sync badge', () => {
      const badge = el.shadowRoot?.querySelector('.sync-badge');
      expect(badge).to.exist;
    });

    it('should display "Synced with Google Keep note <title>"', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Synced with Google Keep note');
      expect(text).to.include('My Keep note');
    });

    it('should render the unsubscribe interlock', () => {
      const interlock = el.shadowRoot?.querySelector('confirmation-interlock-button');
      expect(interlock).to.exist;
    });

    it('should not render the Subscribe trigger button', () => {
      const btn = el.shadowRoot?.querySelector('button.subscribe-trigger');
      expect(btn).to.not.exist;
    });

    it('should not render a paused badge when sync is healthy', () => {
      const badge = el.shadowRoot?.querySelector('connector-paused-badge');
      expect(badge).to.not.exist;
    });
  });

  // ------------------------------------------------------------------ subscribed (Tasks) state

  describe('when subscribed to a Google Tasks list', () => {
    beforeEach(async () => {
      el = document.createElement('connector-subscribe-button') as ConnectorSubscribeButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const client = stubAllRpcs(el);
      const subscription = create(SubscriptionStateSchema, {
        page: 'Board',
        listName: 'todo',
        remoteListHandle: 'tl-1',
        remoteListTitle: 'My Tasks list',
        connectorKind: ConnectorKind.GOOGLE_TASKS,
      });
      client.getChecklistSubscriptionState.resolves(
        create(GetChecklistSubscriptionStateResponseSchema, {
          state: configuredState(subscription),
        }),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should display "Synced with Google Tasks list <title>"', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Synced with Google Tasks list');
      expect(text).to.include('My Tasks list');
    });
  });

  // ------------------------------------------------------------------ subscribed-and-paused state

  describe('when the subscription is paused', () => {
    beforeEach(async () => {
      el = document.createElement('connector-subscribe-button') as ConnectorSubscribeButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const client = stubAllRpcs(el);
      const subscription = create(SubscriptionStateSchema, {
        page: 'Board',
        listName: 'todo',
        remoteListHandle: 'tl-1',
        remoteListTitle: 'My Tasks list',
        connectorKind: ConnectorKind.GOOGLE_TASKS,
        paused: true,
      });
      client.getChecklistSubscriptionState.resolves(
        create(GetChecklistSubscriptionStateResponseSchema, {
          state: configuredState(subscription),
        }),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render the connector-paused-badge', () => {
      const badge = el.shadowRoot?.querySelector('connector-paused-badge');
      expect(badge).to.exist;
    });

    it('should not render the synced badge', () => {
      const badge = el.shadowRoot?.querySelector('.sync-badge');
      expect(badge).to.not.exist;
    });
  });

  // ------------------------------------------------------------------ default-to-authed: only Keep

  describe('when only Google Keep is authenticated and Subscribe is clicked', () => {
    let listRemoteListsStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('connector-subscribe-button') as ConnectorSubscribeButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');
      const client = stubAllRpcs(el);
      client.getChecklistSubscriptionState.resolves(
        create(GetChecklistSubscriptionStateResponseSchema, { state: configuredState() }),
      );
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_KEEP }))
        .resolves(authedKindResponse(ConnectorKind.GOOGLE_KEEP));
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_TASKS }))
        .resolves(unauthedKindResponse(ConnectorKind.GOOGLE_TASKS));

      const note = create(RemoteListSummarySchema, {
        remoteListHandle: 'n1',
        title: 'Sprint notes',
      });
      listRemoteListsStub = client.listRemoteLists.resolves(
        create(ListRemoteListsResponseSchema, { lists: [note] }),
      );

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      const btn = el.shadowRoot?.querySelector('button.subscribe-trigger') as HTMLButtonElement;
      btn.click();
      // Two ticks: connector-pick decision (resolves GetState) + list fetch.
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should skip the connector picker and go straight to the list picker', () => {
      const select = el.shadowRoot?.querySelector('select');
      expect(select).to.exist;
    });

    it('should request lists from Google Keep, not Tasks', () => {
      expect(listRemoteListsStub.calledOnce).to.be.true;
      const call = listRemoteListsStub.firstCall.args[0] as { connectorKind: ConnectorKind };
      expect(call.connectorKind).to.equal(ConnectorKind.GOOGLE_KEEP);
    });

    it('should populate the list picker with fetched options', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Sprint notes');
    });
  });

  // ------------------------------------------------------------------ default-to-authed: only Tasks

  describe('when only Google Tasks is authenticated and Subscribe is clicked', () => {
    let listRemoteListsStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('connector-subscribe-button') as ConnectorSubscribeButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');
      const client = stubAllRpcs(el);
      client.getChecklistSubscriptionState.resolves(
        create(GetChecklistSubscriptionStateResponseSchema, { state: configuredState() }),
      );
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_KEEP }))
        .resolves(unauthedKindResponse(ConnectorKind.GOOGLE_KEEP));
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_TASKS }))
        .resolves(authedKindResponse(ConnectorKind.GOOGLE_TASKS));

      const list = create(RemoteListSummarySchema, {
        remoteListHandle: 'tl-1',
        title: 'Errands',
      });
      listRemoteListsStub = client.listRemoteLists.resolves(
        create(ListRemoteListsResponseSchema, { lists: [list] }),
      );

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      const btn = el.shadowRoot?.querySelector('button.subscribe-trigger') as HTMLButtonElement;
      btn.click();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should skip the connector picker and go straight to the list picker', () => {
      const select = el.shadowRoot?.querySelector('select');
      expect(select).to.exist;
    });

    it('should request lists from Google Tasks, not Keep', () => {
      expect(listRemoteListsStub.calledOnce).to.be.true;
      const call = listRemoteListsStub.firstCall.args[0] as { connectorKind: ConnectorKind };
      expect(call.connectorKind).to.equal(ConnectorKind.GOOGLE_TASKS);
    });
  });

  // ------------------------------------------------------------------ multi-authed: both Keep + Tasks

  describe('when both connectors are authenticated and Subscribe is clicked', () => {
    beforeEach(async () => {
      el = document.createElement('connector-subscribe-button') as ConnectorSubscribeButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');
      const client = stubAllRpcs(el);
      client.getChecklistSubscriptionState.resolves(
        create(GetChecklistSubscriptionStateResponseSchema, { state: configuredState() }),
      );
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_KEEP }))
        .resolves(authedKindResponse(ConnectorKind.GOOGLE_KEEP));
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_TASKS }))
        .resolves(authedKindResponse(ConnectorKind.GOOGLE_TASKS));

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      const btn = el.shadowRoot?.querySelector('button.subscribe-trigger') as HTMLButtonElement;
      btn.click();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should show the connector picker first', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Google Keep');
      expect(text).to.include('Google Tasks');
    });

    it('should not request remote lists yet', () => {
      const client = clientOf(el);
      expect(client.listRemoteLists.called).to.be.false;
    });
  });

  // ------------------------------------------------------------------ multi-authed → user picks Keep → list picker

  describe('when both connectors are authenticated and the user picks Google Keep', () => {
    let listRemoteListsStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('connector-subscribe-button') as ConnectorSubscribeButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');
      const client = stubAllRpcs(el);
      client.getChecklistSubscriptionState.resolves(
        create(GetChecklistSubscriptionStateResponseSchema, { state: configuredState() }),
      );
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_KEEP }))
        .resolves(authedKindResponse(ConnectorKind.GOOGLE_KEEP));
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_TASKS }))
        .resolves(authedKindResponse(ConnectorKind.GOOGLE_TASKS));
      listRemoteListsStub = client.listRemoteLists.resolves(
        create(ListRemoteListsResponseSchema, { lists: [] }),
      );

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      const btn = el.shadowRoot?.querySelector('button.subscribe-trigger') as HTMLButtonElement;
      btn.click();
      // handleSubscribeClick awaits Promise.all of two GetState calls
      // before setting phase = 'connector-pick'. Pump microtasks until
      // the connector-pick phase actually renders.
      for (let i = 0; i < 10; i += 1) {
        await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
        if (el.shadowRoot?.querySelector('button.connector-choice')) break;
      }

      const choices = Array.from(
        el.shadowRoot?.querySelectorAll('button.connector-choice') ?? [],
      ) as HTMLButtonElement[];
      const keepBtn = choices.find((b) => b.textContent?.includes('Google Keep'));
      keepBtn?.click();
      // openListPicker is async — pump again until the list-pick renders.
      for (let i = 0; i < 10; i += 1) {
        await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
        if (el.shadowRoot?.querySelector('select')) break;
      }
    });

    it('should call listRemoteLists with GOOGLE_KEEP', () => {
      expect(listRemoteListsStub.calledOnce).to.be.true;
      const call = listRemoteListsStub.firstCall.args[0] as { connectorKind: ConnectorKind };
      expect(call.connectorKind).to.equal(ConnectorKind.GOOGLE_KEEP);
    });

    it('should display the list picker', () => {
      const select = el.shadowRoot?.querySelector('select');
      expect(select).to.exist;
    });
  });

  // ------------------------------------------------------------------ subscribe action (Keep)

  describe('when Subscribe is confirmed after picking a Keep note', () => {
    let subscribeStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('connector-subscribe-button') as ConnectorSubscribeButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');

      const client = stubAllRpcs(el);
      const unboundState = configuredState();
      const boundSubscription = create(SubscriptionStateSchema, {
        page: 'Board',
        listName: 'sprint',
        remoteListHandle: 'n1',
        remoteListTitle: 'Sprint notes',
        connectorKind: ConnectorKind.GOOGLE_KEEP,
      });
      const boundState = configuredState(boundSubscription);
      client.getChecklistSubscriptionState.onFirstCall().resolves(
        create(GetChecklistSubscriptionStateResponseSchema, { state: unboundState }),
      );
      client.getChecklistSubscriptionState.onSecondCall().resolves(
        create(GetChecklistSubscriptionStateResponseSchema, { state: boundState }),
      );
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_KEEP }))
        .resolves(authedKindResponse(ConnectorKind.GOOGLE_KEEP));
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_TASKS }))
        .resolves(unauthedKindResponse(ConnectorKind.GOOGLE_TASKS));
      client.listRemoteLists.resolves(create(ListRemoteListsResponseSchema, { lists: [] }));
      subscribeStub = client.subscribe.resolves(create(SubscribeResponseSchema, {}));

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      const triggerBtn = el.shadowRoot?.querySelector('button.subscribe-trigger') as HTMLButtonElement;
      triggerBtn.click();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      const btns = Array.from(
        el.shadowRoot?.querySelectorAll('button.subscribe-trigger') ?? [],
      ) as HTMLButtonElement[];
      const subscribeBtn = btns.find((b) => b.textContent?.trim() === 'Subscribe');
      subscribeBtn?.click();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should call subscribe', () => {
      expect(subscribeStub.calledOnce).to.be.true;
    });

    it('should pass the correct connectorKind to subscribe', () => {
      const call = subscribeStub.firstCall.args[0] as { connectorKind: ConnectorKind };
      expect(call.connectorKind).to.equal(ConnectorKind.GOOGLE_KEEP);
    });

    it('should refresh and show the synced badge', () => {
      const badge = el.shadowRoot?.querySelector('.sync-badge');
      expect(badge).to.exist;
    });
  });

  // ------------------------------------------------------------------ unsubscribe action

  describe('when handleUnsubscribe is invoked', () => {
    let unsubscribeStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('connector-subscribe-button') as ConnectorSubscribeButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');

      const client = stubAllRpcs(el);
      const subscription = create(SubscriptionStateSchema, {
        page: 'Board',
        listName: 'todo',
        remoteListHandle: 'note-1',
        connectorKind: ConnectorKind.GOOGLE_KEEP,
      });
      const boundState = configuredState(subscription);
      const unboundState = configuredState();
      client.getChecklistSubscriptionState.onFirstCall().resolves(
        create(GetChecklistSubscriptionStateResponseSchema, { state: boundState }),
      );
      client.getChecklistSubscriptionState.onSecondCall().resolves(
        create(GetChecklistSubscriptionStateResponseSchema, { state: unboundState }),
      );
      unsubscribeStub = client.unsubscribe.resolves(create(UnsubscribeResponseSchema, {}));

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      // Call private handler directly (bypasses interlock confirmation)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion
      await (el as any).handleUnsubscribe();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should call unsubscribe', () => {
      expect(unsubscribeStub.calledOnce).to.be.true;
    });

    it('should pass the connectorKind from the current subscription', () => {
      const call = unsubscribeStub.firstCall.args[0] as { connectorKind: ConnectorKind };
      expect(call.connectorKind).to.equal(ConnectorKind.GOOGLE_KEEP);
    });

    it('should transition back to unsubscribed phase', () => {
      const badge = el.shadowRoot?.querySelector('.sync-badge');
      expect(badge).to.not.exist;
      const btn = el.shadowRoot?.querySelector('button.subscribe-trigger');
      expect(btn).to.exist;
    });
  });
});
