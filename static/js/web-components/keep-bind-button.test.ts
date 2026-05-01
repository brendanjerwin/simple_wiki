import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import './keep-bind-button.js';
import type { KeepBindButton } from './keep-bind-button.js';
import {
  GetChecklistBindingStateResponseSchema,
  ChecklistBindingStateSchema,
  BindingStateSchema,
  ListNotesResponseSchema,
  KeepNoteSummarySchema,
  BindChecklistResponseSchema,
  UnbindChecklistResponseSchema,
} from '../gen/api/v1/keep_connector_pb.js';

// Access private client via type cast.
interface KeepBindButtonClient {
  getChecklistBindingState: sinon.SinonStub;
  listNotes: sinon.SinonStub;
  bindChecklist: sinon.SinonStub;
  unbindChecklist: sinon.SinonStub;
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

function makeUnboundState(): typeof GetChecklistBindingStateResponseSchema.$typeName {
  const state = create(ChecklistBindingStateSchema, { connectorConfigured: true });
  return create(GetChecklistBindingStateResponseSchema, { state }) as never;
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
      const state = create(ChecklistBindingStateSchema, { connectorConfigured: false });
      sinon
        .stub(clientOf(el), 'getChecklistBindingState')
        .resolves(create(GetChecklistBindingStateResponseSchema, { state }));
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
      el = document.createElement('keep-bind-button') as KeepBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      sinon
        .stub(clientOf(el), 'getChecklistBindingState')
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

  describe('when connector is configured but no binding exists', () => {
    beforeEach(async () => {
      el = document.createElement('keep-bind-button') as KeepBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const state = create(ChecklistBindingStateSchema, { connectorConfigured: true });
      sinon
        .stub(clientOf(el), 'getChecklistBindingState')
        .resolves(create(GetChecklistBindingStateResponseSchema, { state }));
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

  describe('when a binding exists', () => {
    beforeEach(async () => {
      el = document.createElement('keep-bind-button') as KeepBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');
      const binding = create(BindingStateSchema, {
        page: 'Board',
        listName: 'todo',
        keepNoteId: 'note-1',
        keepNoteTitle: 'My Keep note',
      });
      const state = create(ChecklistBindingStateSchema, {
        connectorConfigured: true,
        currentBinding: binding,
      });
      sinon
        .stub(clientOf(el), 'getChecklistBindingState')
        .resolves(create(GetChecklistBindingStateResponseSchema, { state }));
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
    let listNotesStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('keep-bind-button') as KeepBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');
      const state = create(ChecklistBindingStateSchema, { connectorConfigured: true });
      sinon
        .stub(clientOf(el), 'getChecklistBindingState')
        .resolves(create(GetChecklistBindingStateResponseSchema, { state }));

      const note = create(KeepNoteSummarySchema, { keepNoteId: 'n1', title: 'Sprint notes' });
      listNotesStub = sinon
        .stub(clientOf(el), 'listNotes')
        .resolves(create(ListNotesResponseSchema, { notes: [note] }));

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      // Click the bind trigger button
      const btn = el.shadowRoot?.querySelector('button.bind-trigger') as HTMLButtonElement;
      btn.click();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should call listNotes', () => {
      expect(listNotesStub.calledOnce).to.be.true;
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

  // ------------------------------------------------------------------ bind action

  describe('when Bind is confirmed after picking a note', () => {
    let bindStub: sinon.SinonStub;
    let getStateStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('keep-bind-button') as KeepBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'sprint');

      const unboundState = create(ChecklistBindingStateSchema, { connectorConfigured: true });

      // After bind, refresh returns bound state
      const boundBinding = create(BindingStateSchema, {
        page: 'Board',
        listName: 'sprint',
        keepNoteId: 'n1',
        keepNoteTitle: 'Sprint notes',
      });
      const boundState = create(ChecklistBindingStateSchema, {
        connectorConfigured: true,
        currentBinding: boundBinding,
      });
      getStateStub = sinon.stub(clientOf(el), 'getChecklistBindingState');
      getStateStub.onFirstCall().resolves(create(GetChecklistBindingStateResponseSchema, { state: unboundState }));
      getStateStub.onSecondCall().resolves(create(GetChecklistBindingStateResponseSchema, { state: boundState }));

      sinon
        .stub(clientOf(el), 'listNotes')
        .resolves(create(ListNotesResponseSchema, { notes: [] }));

      bindStub = sinon
        .stub(clientOf(el), 'bindChecklist')
        .resolves(create(BindChecklistResponseSchema, {}));

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

    it('should call bindChecklist', () => {
      expect(bindStub.calledOnce).to.be.true;
    });

    it('should refresh and show bound phase', () => {
      const badge = el.shadowRoot?.querySelector('.sync-badge');
      expect(badge).to.exist;
    });
  });

  // ------------------------------------------------------------------ unbind action

  describe('when handleUnbind is invoked', () => {
    let unbindStub: sinon.SinonStub;

    beforeEach(async () => {
      el = document.createElement('keep-bind-button') as KeepBindButton;
      el.setAttribute('page', 'Board');
      el.setAttribute('list-name', 'todo');

      const binding = create(BindingStateSchema, {
        page: 'Board',
        listName: 'todo',
        keepNoteId: 'note-1',
      });
      const boundState = create(ChecklistBindingStateSchema, {
        connectorConfigured: true,
        currentBinding: binding,
      });
      const unboundState = create(ChecklistBindingStateSchema, { connectorConfigured: true });

      const getStateStub = sinon.stub(clientOf(el), 'getChecklistBindingState');
      getStateStub.onFirstCall().resolves(create(GetChecklistBindingStateResponseSchema, { state: boundState }));
      getStateStub.onSecondCall().resolves(create(GetChecklistBindingStateResponseSchema, { state: unboundState }));

      unbindStub = sinon
        .stub(clientOf(el), 'unbindChecklist')
        .resolves(create(UnbindChecklistResponseSchema, {}));

      document.body.appendChild(el);
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);

      // Call private handleUnbind directly (bypasses interlock confirmation)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-type-assertion
      await (el as any).handleUnbind();
      await Promise.race([el.updateComplete, timeout(3000, 'updateComplete timed out')]);
    });

    it('should call unbindChecklist', () => {
      expect(unbindStub.calledOnce).to.be.true;
    });

    it('should transition back to unbound phase', () => {
      const badge = el.shadowRoot?.querySelector('.sync-badge');
      expect(badge).to.not.exist;
      const btn = el.shadowRoot?.querySelector('button.bind-trigger');
      expect(btn).to.exist;
    });
  });
});
