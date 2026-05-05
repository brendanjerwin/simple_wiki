import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import './connector-bind-button.js';
import type { ConnectorBindButton } from './connector-bind-button.js';
import {
  ConnectorKind,
  GetChecklistBindingStateResponseSchema,
  ChecklistBindingStateSchema,
  BindingStateSchema,
  ListRemoteListsResponseSchema,
  RemoteListSummarySchema,
  BindResponseSchema,
  UnbindResponseSchema,
  GetStateResponseSchema,
  ConnectorStateSchema,
} from '../gen/api/v1/connector_service_pb.js';

interface BindButtonClient {
  getChecklistBindingState: sinon.SinonStub;
  getState: sinon.SinonStub;
  listRemoteLists: sinon.SinonStub;
  bind: sinon.SinonStub;
  unbind: sinon.SinonStub;
}

function clientOf(el: ConnectorBindButton): BindButtonClient {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion, @typescript-eslint/no-explicit-any
  return (el as any).client as BindButtonClient;
}

function timeout(ms: number, message: string): Promise<never> {
  return new Promise<never>((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

function stubAllRpcs(el: ConnectorBindButton): BindButtonClient {
  const client = clientOf(el);
  // Default-stub every RPC so unrelated calls don't hang. Tests override
  // specific methods by re-stubbing after this is called.
  sinon.stub(client, 'getChecklistBindingState');
  sinon.stub(client, 'getState');
  sinon.stub(client, 'listRemoteLists');
  sinon.stub(client, 'bind');
  sinon.stub(client, 'unbind');
  return client;
}

function configuredState(currentBinding?: ReturnType<typeof create>) {
  return create(ChecklistBindingStateSchema, {
    connectorConfigured: true,
    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-explicit-any
    currentBinding: currentBinding as any,
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

describe('ConnectorBindButton', () => {
  let el: ConnectorBindButton;

  afterEach(() => {
    if (el && el.parentNode) el.remove();
    sinon.restore();
  });

  // ------------------------------------------------------------------ no attributes → hidden

  describe('when page and list-name attributes are not set', () => {
    beforeEach(async () => {
      el = document.createElement('connector-bind-button') as ConnectorBindButton;
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should exist', () => {
      expect(el).to.be.instanceOf(HTMLElement);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName).to.equal('CONNECTOR-BIND-BUTTON');
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
      el = document.createElement('connector-bind-button') as ConnectorBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const client = stubAllRpcs(el);
      const state = create(ChecklistBindingStateSchema, { connectorConfigured: false });
      client.getChecklistBindingState.resolves(
        create(GetChecklistBindingStateResponseSchema, { state }),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render nothing (phase=hidden)', () => {
      const content = el.shadowRoot?.innerHTML.trim() ?? '';
      expect(content).to.not.include('<button');
      expect(content).to.not.include('Bind');
    });
  });

  // ------------------------------------------------------------------ getChecklistBindingState errors → hidden

  describe('when getChecklistBindingState rejects', () => {
    beforeEach(async () => {
      el = document.createElement('connector-bind-button') as ConnectorBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const client = stubAllRpcs(el);
      client.getChecklistBindingState.rejects(
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
      el = document.createElement('connector-bind-button') as ConnectorBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const client = stubAllRpcs(el);
      client.getChecklistBindingState.resolves(
        create(GetChecklistBindingStateResponseSchema, { state: configuredState() }),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render the Bind trigger button', () => {
      const btn = el.shadowRoot?.querySelector('button.bind-trigger');
      expect(btn).to.exist;
    });

    it('should not render an unbind interlock', () => {
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
      el = document.createElement('connector-bind-button') as ConnectorBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const client = stubAllRpcs(el);
      const subscription = create(BindingStateSchema, {
        page: 'Board',
        listName: 'todo',
        remoteListHandle: 'note-1',
        remoteListTitle: 'My Keep note',
        connectorKind: ConnectorKind.GOOGLE_KEEP,
      });
      client.getChecklistBindingState.resolves(
        create(GetChecklistBindingStateResponseSchema, {
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

    it('should display "Bound to Google Keep note <title>"', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Bound to Google Keep note');
      expect(text).to.include('My Keep note');
    });

    it('should render the unbind interlock', () => {
      const interlock = el.shadowRoot?.querySelector('confirmation-interlock-button');
      expect(interlock).to.exist;
    });

    it('should not render the Bind trigger button', () => {
      const btn = el.shadowRoot?.querySelector('button.bind-trigger');
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
      el = document.createElement('connector-bind-button') as ConnectorBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const client = stubAllRpcs(el);
      const subscription = create(BindingStateSchema, {
        page: 'Board',
        listName: 'todo',
        remoteListHandle: 'tl-1',
        remoteListTitle: 'My Tasks list',
        connectorKind: ConnectorKind.GOOGLE_TASKS,
      });
      client.getChecklistBindingState.resolves(
        create(GetChecklistBindingStateResponseSchema, {
          state: configuredState(subscription),
        }),
      );
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should display "Bound to Google Tasks list <title>"', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Bound to Google Tasks list');
      expect(text).to.include('My Tasks list');
    });
  });

  // ------------------------------------------------------------------ subscribed-and-paused state

  describe('when the subscription is paused', () => {
    beforeEach(async () => {
      el = document.createElement('connector-bind-button') as ConnectorBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const client = stubAllRpcs(el);
      const subscription = create(BindingStateSchema, {
        page: 'Board',
        listName: 'todo',
        remoteListHandle: 'tl-1',
        remoteListTitle: 'My Tasks list',
        connectorKind: ConnectorKind.GOOGLE_TASKS,
        paused: true,
      });
      client.getChecklistBindingState.resolves(
        create(GetChecklistBindingStateResponseSchema, {
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

  describe('when only Google Keep is authenticated and Bind is clicked', () => {
    let listRemoteListsStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('connector-bind-button') as ConnectorBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');
      const client = stubAllRpcs(el);
      client.getChecklistBindingState.resolves(
        create(GetChecklistBindingStateResponseSchema, { state: configuredState() }),
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

      const btn = el.shadowRoot?.querySelector('button.bind-trigger') as HTMLButtonElement;
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

  describe('when only Google Tasks is authenticated and Bind is clicked', () => {
    let listRemoteListsStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('connector-bind-button') as ConnectorBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');
      const client = stubAllRpcs(el);
      client.getChecklistBindingState.resolves(
        create(GetChecklistBindingStateResponseSchema, { state: configuredState() }),
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

      const btn = el.shadowRoot?.querySelector('button.bind-trigger') as HTMLButtonElement;
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

  describe('when both connectors are authenticated and Bind is clicked', () => {
    beforeEach(async () => {
      el = document.createElement('connector-bind-button') as ConnectorBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');
      const client = stubAllRpcs(el);
      client.getChecklistBindingState.resolves(
        create(GetChecklistBindingStateResponseSchema, { state: configuredState() }),
      );
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_KEEP }))
        .resolves(authedKindResponse(ConnectorKind.GOOGLE_KEEP));
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_TASKS }))
        .resolves(authedKindResponse(ConnectorKind.GOOGLE_TASKS));

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      const btn = el.shadowRoot?.querySelector('button.bind-trigger') as HTMLButtonElement;
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
      el = document.createElement('connector-bind-button') as ConnectorBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');
      const client = stubAllRpcs(el);
      client.getChecklistBindingState.resolves(
        create(GetChecklistBindingStateResponseSchema, { state: configuredState() }),
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

      const btn = el.shadowRoot?.querySelector('button.bind-trigger') as HTMLButtonElement;
      btn.click();
      // handleBindClick awaits Promise.all of two GetState calls
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

  describe('when Bind is confirmed after picking a Keep note', () => {
    let bindStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('connector-bind-button') as ConnectorBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');

      const client = stubAllRpcs(el);
      const unboundState = configuredState();
      const boundBinding = create(BindingStateSchema, {
        page: 'Board',
        listName: 'sprint',
        remoteListHandle: 'n1',
        remoteListTitle: 'Sprint notes',
        connectorKind: ConnectorKind.GOOGLE_KEEP,
      });
      const boundState = configuredState(boundBinding);
      client.getChecklistBindingState.onFirstCall().resolves(
        create(GetChecklistBindingStateResponseSchema, { state: unboundState }),
      );
      client.getChecklistBindingState.onSecondCall().resolves(
        create(GetChecklistBindingStateResponseSchema, { state: boundState }),
      );
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_KEEP }))
        .resolves(authedKindResponse(ConnectorKind.GOOGLE_KEEP));
      client.getState
        .withArgs(sinon.match({ connectorKind: ConnectorKind.GOOGLE_TASKS }))
        .resolves(unauthedKindResponse(ConnectorKind.GOOGLE_TASKS));
      client.listRemoteLists.resolves(create(ListRemoteListsResponseSchema, { lists: [] }));
      bindStub = client.bind.resolves(create(BindResponseSchema, {}));

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      const triggerBtn = el.shadowRoot?.querySelector('button.bind-trigger') as HTMLButtonElement;
      triggerBtn.click();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      const btns = Array.from(
        el.shadowRoot?.querySelectorAll('button.bind-trigger') ?? [],
      ) as HTMLButtonElement[];
      const bindBtn = btns.find((b) => b.textContent?.trim() === 'Bind');
      bindBtn?.click();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should call subscribe', () => {
      expect(bindStub.calledOnce).to.be.true;
    });

    it('should pass the correct connectorKind to subscribe', () => {
      const call = bindStub.firstCall.args[0] as { connectorKind: ConnectorKind };
      expect(call.connectorKind).to.equal(ConnectorKind.GOOGLE_KEEP);
    });

    it('should refresh and show the synced badge', () => {
      const badge = el.shadowRoot?.querySelector('.sync-badge');
      expect(badge).to.exist;
    });
  });

  // ------------------------------------------------------------------ unsubscribe action

  describe('when handleUnbind is invoked', () => {
    let unbindStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('connector-bind-button') as ConnectorBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');

      const client = stubAllRpcs(el);
      const subscription = create(BindingStateSchema, {
        page: 'Board',
        listName: 'todo',
        remoteListHandle: 'note-1',
        connectorKind: ConnectorKind.GOOGLE_KEEP,
      });
      const boundState = configuredState(subscription);
      const unboundState = configuredState();
      client.getChecklistBindingState.onFirstCall().resolves(
        create(GetChecklistBindingStateResponseSchema, { state: boundState }),
      );
      client.getChecklistBindingState.onSecondCall().resolves(
        create(GetChecklistBindingStateResponseSchema, { state: unboundState }),
      );
      unbindStub = client.unbind.resolves(create(UnbindResponseSchema, {}));

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      // Call private handler directly (bypasses interlock confirmation)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion
      await (el as any).handleUnbind();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should call unsubscribe', () => {
      expect(unbindStub.calledOnce).to.be.true;
    });

    it('should pass the connectorKind from the current subscription', () => {
      const call = unbindStub.firstCall.args[0] as { connectorKind: ConnectorKind };
      expect(call.connectorKind).to.equal(ConnectorKind.GOOGLE_KEEP);
    });

    it('should transition back to unsubscribed phase', () => {
      const badge = el.shadowRoot?.querySelector('.sync-badge');
      expect(badge).to.not.exist;
      const btn = el.shadowRoot?.querySelector('button.bind-trigger');
      expect(btn).to.exist;
    });
  });
});
