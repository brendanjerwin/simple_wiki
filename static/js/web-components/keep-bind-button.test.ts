import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import './keep-bind-button.js';
import type { KeepBindButton } from './keep-bind-button.js';
import {
  GetChecklistSubscriptionStateResponseSchema,
  ChecklistSubscriptionStateSchema,
  SubscriptionStateSchema,
  ListRemoteListsResponseSchema,
  RemoteListSummarySchema,
  SubscribeResponseSchema,
  UnsubscribeResponseSchema,
} from '../gen/api/v1/connector_service_pb.js';

// Access private client via type cast.
interface KeepBindButtonClient {
  getChecklistSubscriptionState: sinon.SinonStub;
  listRemoteLists: sinon.SinonStub;
  subscribe: sinon.SinonStub;
  unsubscribe: sinon.SinonStub;
}

function clientOf(el: KeepBindButton): KeepBindButtonClient {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion, @typescript-eslint/no-explicit-any
  return (el as any).client as KeepBindButtonClient;
}

function timeout(ms: number, message: string): Promise<never> {
  return new Promise<never>((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

describe('KeepBindButton', () => {
  let el: KeepBindButton;

  afterEach(() => {
    el.remove();
    sinon.restore();
  });

  // ------------------------------------------------------------------ no attributes → hidden

  describe('when page and list-name attributes are not set', () => {
    beforeEach(async () => {
      // No fetch stub needed — component returns early with phase=hidden
      // when page/listName are empty.
      el = document.createElement('keep-bind-button') as KeepBindButton;
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should exist', () => {
      expect(el).to.be.instanceOf(HTMLElement);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName).to.equal('KEEP-BIND-BUTTON');
    });

    it('should render nothing (phase=hidden)', () => {
      // Shadow root should be empty when phase is hidden or loading
      const root = el.shadowRoot;
      const content = root?.innerHTML.trim() ?? '';
      // The component returns `nothing` for hidden/loading — no meaningful DOM
      expect(content).to.not.include('<button');
      expect(content).to.not.include('<span');
    });
  });

  // ------------------------------------------------------------------ connector not configured → hidden

  describe('when connector is not configured', () => {
    beforeEach(async () => {
      el = document.createElement('keep-bind-button') as KeepBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const state = create(ChecklistSubscriptionStateSchema, { connectorConfigured: false });
      sinon
        .stub(clientOf(el), 'getChecklistSubscriptionState')
        .resolves(create(GetChecklistSubscriptionStateResponseSchema, { state }));
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render nothing (phase=hidden)', () => {
      const content = el.shadowRoot?.innerHTML.trim() ?? '';
      expect(content).to.not.include('<button');
      expect(content).to.not.include('Bind');
    });
  });

  // ------------------------------------------------------------------ getChecklistSubscriptionState errors → hidden

  describe('when getChecklistSubscriptionState rejects', () => {
    beforeEach(async () => {
      el = document.createElement('keep-bind-button') as KeepBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      sinon
        .stub(clientOf(el), 'getChecklistSubscriptionState')
        .rejects(new ConnectError('unavailable', Code.Unavailable));
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render nothing (fail-closed behavior)', () => {
      const content = el.shadowRoot?.innerHTML.trim() ?? '';
      expect(content).to.not.include('<button');
    });
  });

  // ------------------------------------------------------------------ unbound state

  describe('when connector is configured but no subscription exists', () => {
    beforeEach(async () => {
      el = document.createElement('keep-bind-button') as KeepBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const state = create(ChecklistSubscriptionStateSchema, { connectorConfigured: true });
      sinon
        .stub(clientOf(el), 'getChecklistSubscriptionState')
        .resolves(create(GetChecklistSubscriptionStateResponseSchema, { state }));
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render the Bind to Keep trigger button', () => {
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
  });

  // ------------------------------------------------------------------ bound state

  describe('when a subscription exists', () => {
    beforeEach(async () => {
      el = document.createElement('keep-bind-button') as KeepBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const subscription = create(SubscriptionStateSchema, {
        page: 'Board',
        listName: 'todo',
        remoteListHandle: 'note-1',
        remoteListTitle: 'My Keep note',
      });
      const state = create(ChecklistSubscriptionStateSchema, {
        connectorConfigured: true,
        currentSubscription: subscription,
      });
      sinon
        .stub(clientOf(el), 'getChecklistSubscriptionState')
        .resolves(create(GetChecklistSubscriptionStateResponseSchema, { state }));
      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should render the sync badge', () => {
      const badge = el.shadowRoot?.querySelector('.sync-badge');
      expect(badge).to.exist;
    });

    it('should display the Keep note title in the badge', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('My Keep note');
    });

    it('should render the Unbind interlock button', () => {
      const interlock = el.shadowRoot?.querySelector('confirmation-interlock-button');
      expect(interlock).to.exist;
    });

    it('should not render the Bind trigger button', () => {
      const btn = el.shadowRoot?.querySelector('button.bind-trigger');
      expect(btn).to.not.exist;
    });
  });

  // ------------------------------------------------------------------ picker phase

  describe('when Bind to Keep button is clicked', () => {
    let listRemoteListsStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('keep-bind-button') as KeepBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');
      const state = create(ChecklistSubscriptionStateSchema, { connectorConfigured: true });
      sinon
        .stub(clientOf(el), 'getChecklistSubscriptionState')
        .resolves(create(GetChecklistSubscriptionStateResponseSchema, { state }));

      const note = create(RemoteListSummarySchema, { remoteListHandle: 'n1', title: 'Sprint notes' });
      listRemoteListsStub = sinon
        .stub(clientOf(el), 'listRemoteLists')
        .resolves(create(ListRemoteListsResponseSchema, { lists: [note] }));

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      // Click the bind trigger button
      const btn = el.shadowRoot?.querySelector('button.bind-trigger') as HTMLButtonElement;
      btn.click();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should call listRemoteLists', () => {
      expect(listRemoteListsStub.calledOnce).to.be.true;
    });

    it('should render the note picker select', () => {
      const select = el.shadowRoot?.querySelector('select');
      expect(select).to.exist;
    });

    it('should include the fetched note as an option', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Sprint notes');
    });

    it('should render a Bind confirm button', () => {
      const btns = Array.from(
        el.shadowRoot?.querySelectorAll('button.bind-trigger') ?? [],
      );
      const labels = btns.map((b) => b.textContent?.trim());
      expect(labels).to.include('Bind');
    });

    it('should render a Cancel button', () => {
      const btns = Array.from(
        el.shadowRoot?.querySelectorAll('button.bind-trigger') ?? [],
      );
      const labels = btns.map((b) => b.textContent?.trim());
      expect(labels).to.include('Cancel');
    });
  });

  // ------------------------------------------------------------------ subscribe action

  describe('when Bind is confirmed after picking a note', () => {
    let subscribeStub: sinon.SinonStub;
    let getStateStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('keep-bind-button') as KeepBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');

      const unboundState = create(ChecklistSubscriptionStateSchema, { connectorConfigured: true });

      // After subscribe, refresh returns bound state
      const boundSubscription = create(SubscriptionStateSchema, {
        page: 'Board',
        listName: 'sprint',
        remoteListHandle: 'n1',
        remoteListTitle: 'Sprint notes',
      });
      const boundState = create(ChecklistSubscriptionStateSchema, {
        connectorConfigured: true,
        currentSubscription: boundSubscription,
      });
      getStateStub = sinon.stub(clientOf(el), 'getChecklistSubscriptionState');
      getStateStub.onFirstCall().resolves(create(GetChecklistSubscriptionStateResponseSchema, { state: unboundState }));
      getStateStub.onSecondCall().resolves(create(GetChecklistSubscriptionStateResponseSchema, { state: boundState }));

      sinon
        .stub(clientOf(el), 'listRemoteLists')
        .resolves(create(ListRemoteListsResponseSchema, { lists: [] }));

      subscribeStub = sinon
        .stub(clientOf(el), 'subscribe')
        .resolves(create(SubscribeResponseSchema, {}));

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      // Open picker
      const triggerBtn = el.shadowRoot?.querySelector('button.bind-trigger') as HTMLButtonElement;
      triggerBtn.click();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      // Click Bind confirm button (first button in picker is Bind, second is Cancel)
      const btns = Array.from(
        el.shadowRoot?.querySelectorAll('button.bind-trigger') ?? [],
      ) as HTMLButtonElement[];
      const bindBtn = btns.find((b) => b.textContent?.trim() === 'Bind');
      bindBtn?.click();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should call subscribe', () => {
      expect(subscribeStub.calledOnce).to.be.true;
    });

    it('should refresh and show bound phase', () => {
      const badge = el.shadowRoot?.querySelector('.sync-badge');
      expect(badge).to.exist;
    });
  });

  // ------------------------------------------------------------------ unsubscribe action

  describe('when handleUnbind is invoked', () => {
    let unsubscribeStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('keep-bind-button') as KeepBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');

      const subscription = create(SubscriptionStateSchema, {
        page: 'Board',
        listName: 'todo',
        remoteListHandle: 'note-1',
      });
      const boundState = create(ChecklistSubscriptionStateSchema, {
        connectorConfigured: true,
        currentSubscription: subscription,
      });
      const unboundState = create(ChecklistSubscriptionStateSchema, { connectorConfigured: true });

      const getStateStub = sinon.stub(clientOf(el), 'getChecklistSubscriptionState');
      getStateStub.onFirstCall().resolves(create(GetChecklistSubscriptionStateResponseSchema, { state: boundState }));
      getStateStub.onSecondCall().resolves(create(GetChecklistSubscriptionStateResponseSchema, { state: unboundState }));

      unsubscribeStub = sinon
        .stub(clientOf(el), 'unsubscribe')
        .resolves(create(UnsubscribeResponseSchema, {}));

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      // Call private handleUnbind directly (bypasses interlock confirmation)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion
      await (el as any).handleUnbind();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should call unsubscribe', () => {
      expect(unsubscribeStub.calledOnce).to.be.true;
    });

    it('should transition back to unbound phase', () => {
      const badge = el.shadowRoot?.querySelector('.sync-badge');
      expect(badge).to.not.exist;
      const btn = el.shadowRoot?.querySelector('button.bind-trigger');
      expect(btn).to.exist;
    });
  });
});
