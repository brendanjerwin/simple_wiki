import { html, fixture, expect, waitUntil } from '@open-wc/testing';
import sinon from 'sinon';
import { create } from '@bufbuild/protobuf';
import { TimestampSchema } from '@bufbuild/protobuf/wkt';
import { AgentDetailsDialog } from './agent-details-dialog.js';
import {
  AgentScheduleSchema,
  ChatContextSchema,
  BackgroundActivityEntrySchema,
  ListSchedulesResponseSchema,
  GetChatContextResponseSchema,
  DeleteScheduleResponseSchema,
  ScheduleStatus,
} from '../gen/api/v1/agent_metadata_pb.js';
import { AugmentedError, ErrorKind } from './augment-error-service.js';
import './agent-details-dialog.js';

function timeout(ms: number, message: string): Promise<never> {
  return new Promise((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms)
  );
}

interface FakeAgentClient {
  listSchedules: sinon.SinonStub;
  getChatContext: sinon.SinonStub;
  deleteSchedule: sinon.SinonStub;
}

function buildFakeClient(): FakeAgentClient {
  return {
    listSchedules: sinon.stub(),
    getChatContext: sinon.stub(),
    deleteSchedule: sinon.stub(),
  };
}

function makeSchedule(overrides: Partial<{
  id: string;
  cron: string;
  prompt: string;
  lastStatus: ScheduleStatus;
}> = {}): ReturnType<typeof create<typeof AgentScheduleSchema>> {
  return create(AgentScheduleSchema, {
    id: overrides.id ?? 'sched-1',
    cron: overrides.cron ?? '0 0 9 * * *',
    prompt: overrides.prompt ?? 'Do the thing',
    maxTurns: 0,
    enabled: true,
    lastStatus: overrides.lastStatus ?? ScheduleStatus.OK,
    lastErrorMessage: '',
    lastDurationSeconds: 0,
  });
}

describe('AgentDetailsDialog', () => {
  let el: AgentDetailsDialog;

  afterEach(() => {
    sinon.restore();
    if (el) {
      el.remove();
    }
  });

  describe('basic existence', () => {
    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);
      await el.updateComplete;
    });

    it('should exist', () => {
      expect(el).to.exist;
    });

    it('should be an instance of AgentDetailsDialog', () => {
      expect(el).to.be.instanceOf(AgentDetailsDialog);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName.toLowerCase()).to.equal('agent-details-dialog');
    });

    it('should not be open by default', () => {
      expect(el.open).to.be.false;
    });

    it('should have empty page by default', () => {
      expect(el.page).to.equal('');
    });

    it('should not be loading by default', () => {
      expect(el.loading).to.be.false;
    });

    it('should have no error by default', () => {
      expect(el.augmentedError).to.equal(null);
    });

    it('should have empty schedules by default', () => {
      expect(el.schedules).to.deep.equal([]);
    });

    it('should have null chat context by default', () => {
      expect(el.chatContext).to.equal(null);
    });

    it('should render a native dialog element', () => {
      const dialog = el.shadowRoot?.querySelector('dialog');
      expect(dialog).to.exist;
    });

    it('should not have the dialog open by default', () => {
      const dialog = el.shadowRoot?.querySelector('dialog');
      expect(dialog?.open).to.be.false;
    });

    it('should have aria-labelledby pointing to the dialog title id', () => {
      const dialog = el.shadowRoot?.querySelector('dialog');
      expect(dialog?.getAttribute('aria-labelledby')).to.equal('agent-details-dialog-title');
    });

    it('should have an h2 with id agent-details-dialog-title', () => {
      const h2 = el.shadowRoot?.querySelector('h2#agent-details-dialog-title');
      expect(h2).to.exist;
    });
  });

  describe('when openDialog is called', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, { schedules: [] }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('my-page');
      await Promise.race([el.updateComplete, timeout(5000, 'updateComplete timed out')]);
    });

    it('should set the page property', () => {
      expect(el.page).to.equal('my-page');
    });

    it('should set open to true', () => {
      expect(el.open).to.be.true;
    });

    it('should open the native dialog', () => {
      const dialog = el.shadowRoot?.querySelector('dialog');
      expect(dialog?.open).to.be.true;
    });
  });

  describe('when openDialog triggers loading', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, { schedules: [] }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('p');
      await waitUntil(() => fakeClient.listSchedules.called && fakeClient.getChatContext.called, 'RPCs not called', { timeout: 1000 });
    });

    it('should call listSchedules once', () => {
      expect(fakeClient.listSchedules).to.have.been.calledOnce;
    });

    it('should call listSchedules with the requested page', () => {
      const callArgs = fakeClient.listSchedules.getCall(0).args[0];
      expect(callArgs.page).to.equal('p');
    });

    it('should call getChatContext once', () => {
      expect(fakeClient.getChatContext).to.have.been.calledOnce;
    });

    it('should call getChatContext with the requested page', () => {
      const callArgs = fakeClient.getChatContext.getCall(0).args[0];
      expect(callArgs.page).to.equal('p');
    });
  });

  describe('when load resolves with schedules', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, {
        schedules: [
          makeSchedule({ id: 'morning', cron: '0 0 9 * * *', lastStatus: ScheduleStatus.OK }),
          makeSchedule({ id: 'evening', cron: '0 0 18 * * *', lastStatus: ScheduleStatus.ERROR }),
        ],
      }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('p');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should render one row per schedule', () => {
      const rows = el.shadowRoot?.querySelectorAll('.schedule-row');
      expect(rows?.length).to.equal(2);
    });

    it('should render the schedule id for each row', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('morning');
      expect(text).to.include('evening');
    });

    it('should render the schedule cron for each row', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('0 0 9 * * *');
      expect(text).to.include('0 0 18 * * *');
    });

    it('should render a status badge for each row', () => {
      const badges = el.shadowRoot?.querySelectorAll('.schedule-row .status-badge');
      expect(badges?.length).to.equal(2);
    });

    it('should mark the OK schedule with the status-ok class', () => {
      const okRow = el.shadowRoot?.querySelector('[data-schedule-id="morning"] .status-badge');
      expect(okRow?.classList.contains('status-ok')).to.be.true;
    });

    it('should mark the error schedule with the status-error class', () => {
      const errorRow = el.shadowRoot?.querySelector('[data-schedule-id="evening"] .status-badge');
      expect(errorRow?.classList.contains('status-error')).to.be.true;
    });
  });

  describe('when load resolves with chat context', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      const updatedAt = create(TimestampSchema, {
        seconds: BigInt(Math.floor(new Date('2024-06-15T10:00:00Z').getTime() / 1000)),
        nanos: 0,
      });

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, { schedules: [] }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {
        chatContext: create(ChatContextSchema, {
          lastConversationSummary: 'Discussed the pastry order',
          userGoals: ['Track inventory', 'Plan weekend bake'],
          pendingItems: ['Order flour', 'Update price list'],
          keyContext: 'Bakery is closed Mondays',
          lastUpdated: updatedAt,
          backgroundActivity: [],
        }),
      }));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('p');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should render the last conversation summary', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Discussed the pastry order');
    });

    it('should render each user goal as a list item', () => {
      const items = el.shadowRoot?.querySelectorAll('.goal-list .chat-list-item');
      expect(items?.length).to.equal(2);
    });

    it('should include the user goal text', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Track inventory');
      expect(text).to.include('Plan weekend bake');
    });

    it('should render each pending item as a list item', () => {
      const items = el.shadowRoot?.querySelectorAll('.pending-list .chat-list-item');
      expect(items?.length).to.equal(2);
    });

    it('should include the pending item text', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Order flour');
      expect(text).to.include('Update price list');
    });

    it('should render a leading checkbox glyph for each pending item', () => {
      const glyphs = el.shadowRoot?.querySelectorAll('.pending-list .chat-list-item .fa-square');
      expect(glyphs?.length).to.equal(2);
    });

    it('should not render a checkbox glyph on user goals', () => {
      const glyphs = el.shadowRoot?.querySelectorAll('.goal-list .chat-list-item .fa-square');
      expect(glyphs?.length).to.equal(0);
    });

    it('should render the key context', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Bakery is closed Mondays');
    });

    it('should render the last updated timestamp in human-friendly form', () => {
      const lastUpdatedEl = el.shadowRoot?.querySelector('.chat-last-updated');
      expect(lastUpdatedEl?.textContent ?? '').to.include('2024');
    });
  });

  describe('when load resolves with background activity', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      const earlyTs = create(TimestampSchema, {
        seconds: BigInt(Math.floor(new Date('2024-06-10T08:00:00Z').getTime() / 1000)),
        nanos: 0,
      });
      const lateTs = create(TimestampSchema, {
        seconds: BigInt(Math.floor(new Date('2024-06-15T20:00:00Z').getTime() / 1000)),
        nanos: 0,
      });

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, { schedules: [] }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {
        chatContext: create(ChatContextSchema, {
          backgroundActivity: [
            create(BackgroundActivityEntrySchema, {
              timestamp: earlyTs,
              scheduleId: 'morning',
              status: ScheduleStatus.OK,
              summary: 'Older entry',
            }),
            create(BackgroundActivityEntrySchema, {
              timestamp: lateTs,
              scheduleId: 'evening',
              status: ScheduleStatus.ERROR,
              summary: 'Newer entry',
            }),
          ],
        }),
      }));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('p');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should render one row per activity entry', () => {
      const rows = el.shadowRoot?.querySelectorAll('.activity-row');
      expect(rows?.length).to.equal(2);
    });

    it('should render the newest entry first', () => {
      const rows = el.shadowRoot?.querySelectorAll('.activity-row');
      expect(rows?.[0]?.textContent).to.include('Newer entry');
      expect(rows?.[1]?.textContent).to.include('Older entry');
    });

    it('should include the schedule id for each entry', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('morning');
      expect(text).to.include('evening');
    });

    it('should include a status badge for each entry', () => {
      const badges = el.shadowRoot?.querySelectorAll('.activity-row .status-badge');
      expect(badges?.length).to.equal(2);
    });

    it('should include the summary text for each entry', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('Newer entry');
      expect(text).to.include('Older entry');
    });
  });

  describe('when schedules list is empty', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, { schedules: [] }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('p');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should show the "No schedules yet" message', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('No schedules yet');
    });
  });

  describe('when chat context is missing', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, { schedules: [] }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('p');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should show the "No chat memory recorded" message', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('No chat memory recorded');
    });

    it('should show the "No background activity" message', () => {
      const text = el.shadowRoot?.textContent ?? '';
      expect(text).to.include('No background activity');
    });
  });

  describe('when an RPC errors', () => {
    let fakeClient: FakeAgentClient;
    const failure = new Error('boom');

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.rejects(failure);
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('p');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should set augmentedError', () => {
      expect(el.augmentedError).to.be.instanceOf(AugmentedError);
    });

    it('should preserve the original error message', () => {
      expect(el.augmentedError?.message).to.equal('boom');
    });

    it('should render the error-display component', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.exist;
    });

    it('should not render the schedules list', () => {
      const list = el.shadowRoot?.querySelector('.schedule-list');
      expect(list).to.equal(null);
    });
  });

  describe('when loading state is observed during RPC', () => {
    let fakeClient: FakeAgentClient;
    let listResolve: (value: unknown) => void;
    let chatResolve: (value: unknown) => void;
    let loadingDuringRpc: boolean;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      const listPromise = new Promise((resolve) => { listResolve = resolve; });
      const chatPromise = new Promise((resolve) => { chatResolve = resolve; });

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.returns(listPromise);
      fakeClient.getChatContext.returns(chatPromise);
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('p');
      await el.updateComplete;
      loadingDuringRpc = el.loading;

      // Now resolve and let the test finish cleanly
      listResolve(create(ListSchedulesResponseSchema, { schedules: [] }));
      chatResolve(create(GetChatContextResponseSchema, {}));
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
    });

    it('should be loading while RPCs are in flight', () => {
      expect(loadingDuringRpc).to.be.true;
    });
  });

  describe('when loading is true', () => {
    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);
      el.loading = true;
      await el.updateComplete;
    });

    it('should render the loading indicator', () => {
      const loadingEl = el.shadowRoot?.querySelector('.loading');
      expect(loadingEl).to.exist;
    });

    it('should show loading text', () => {
      const loadingEl = el.shadowRoot?.querySelector('.loading');
      expect(loadingEl?.textContent?.trim()).to.include('Loading agent details...');
    });

    it('should not render the schedules list', () => {
      const list = el.shadowRoot?.querySelector('.schedule-list');
      expect(list).to.equal(null);
    });

    it('should not render the error-display', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.equal(null);
    });
  });

  describe('when delete schedule button is clicked once', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, {
        schedules: [
          makeSchedule({ id: 'keep-me' }),
          makeSchedule({ id: 'delete-me' }),
        ],
      }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      fakeClient.deleteSchedule.resolves(create(DeleteScheduleResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('my-page');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;

      const deleteButton = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="delete-me"] .delete-schedule-button'
      );
      deleteButton?.click();
      await el.updateComplete;
    });

    it('should not call deleteSchedule', () => {
      expect(fakeClient.deleteSchedule).to.not.have.been.called;
    });

    it('should arm the delete confirmation for that schedule', () => {
      expect(el.confirmingDeleteId).to.equal('delete-me');
    });

    it('should change the button label to a confirming label', () => {
      const button = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="delete-me"] .delete-schedule-button'
      );
      expect(button?.textContent?.trim()).to.equal('Confirm delete');
    });

    it('should mark the armed button with a confirming class', () => {
      const button = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="delete-me"] .delete-schedule-button'
      );
      expect(button?.classList.contains('confirming')).to.be.true;
    });

    it('should keep the row in local state', () => {
      const ids = el.schedules.map((s) => s.id);
      expect(ids).to.deep.equal(['keep-me', 'delete-me']);
    });

    it('should still render the row in the DOM', () => {
      const row = el.shadowRoot?.querySelector('[data-schedule-id="delete-me"]');
      expect(row).to.exist;
    });
  });

  describe('when delete schedule is confirmed by a second click', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, {
        schedules: [
          makeSchedule({ id: 'keep-me' }),
          makeSchedule({ id: 'delete-me' }),
        ],
      }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      fakeClient.deleteSchedule.resolves(create(DeleteScheduleResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('my-page');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;

      const firstClickButton = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="delete-me"] .delete-schedule-button'
      );
      firstClickButton?.click();
      await el.updateComplete;

      const confirmButton = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="delete-me"] .delete-schedule-button'
      );
      confirmButton?.click();
      await waitUntil(() => fakeClient.deleteSchedule.called, 'delete not called', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should call deleteSchedule once', () => {
      expect(fakeClient.deleteSchedule).to.have.been.calledOnce;
    });

    it('should call deleteSchedule with the page', () => {
      const args = fakeClient.deleteSchedule.getCall(0).args[0];
      expect(args.page).to.equal('my-page');
    });

    it('should call deleteSchedule with the targeted schedule id', () => {
      const args = fakeClient.deleteSchedule.getCall(0).args[0];
      expect(args.scheduleId).to.equal('delete-me');
    });

    it('should remove the deleted schedule from local state', () => {
      const ids = el.schedules.map((s) => s.id);
      expect(ids).to.deep.equal(['keep-me']);
    });

    it('should no longer render the deleted schedule row', () => {
      const deletedRow = el.shadowRoot?.querySelector('[data-schedule-id="delete-me"]');
      expect(deletedRow).to.equal(null);
    });

    it('should keep the remaining schedule row visible', () => {
      const remainingRow = el.shadowRoot?.querySelector('[data-schedule-id="keep-me"]');
      expect(remainingRow).to.exist;
    });

    it('should clear the armed confirmation state', () => {
      expect(el.confirmingDeleteId).to.equal(null);
    });
  });

  describe('when delete is armed on schedule A and then schedule B is clicked', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, {
        schedules: [
          makeSchedule({ id: 'sched-a' }),
          makeSchedule({ id: 'sched-b' }),
        ],
      }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      fakeClient.deleteSchedule.resolves(create(DeleteScheduleResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('my-page');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;

      const buttonA = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="sched-a"] .delete-schedule-button'
      );
      buttonA?.click();
      await el.updateComplete;

      const buttonB = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="sched-b"] .delete-schedule-button'
      );
      buttonB?.click();
      await el.updateComplete;
    });

    it('should not call deleteSchedule', () => {
      expect(fakeClient.deleteSchedule).to.not.have.been.called;
    });

    it('should move the armed confirmation to schedule B', () => {
      expect(el.confirmingDeleteId).to.equal('sched-b');
    });

    it('should not mark schedule A as confirming', () => {
      const buttonA = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="sched-a"] .delete-schedule-button'
      );
      expect(buttonA?.classList.contains('confirming')).to.be.false;
    });

    it('should mark schedule B as confirming', () => {
      const buttonB = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="sched-b"] .delete-schedule-button'
      );
      expect(buttonB?.classList.contains('confirming')).to.be.true;
    });
  });

  describe('when delete is armed and then Escape is pressed', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, {
        schedules: [makeSchedule({ id: 'armed-row' })],
      }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      fakeClient.deleteSchedule.resolves(create(DeleteScheduleResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('my-page');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;

      const button = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="armed-row"] .delete-schedule-button'
      );
      button?.click();
      await el.updateComplete;

      const escapeEvent = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true });
      el.shadowRoot?.querySelector('dialog')?.dispatchEvent(escapeEvent);
      await el.updateComplete;
    });

    it('should clear the armed confirmation state', () => {
      expect(el.confirmingDeleteId).to.equal(null);
    });

    it('should not call deleteSchedule', () => {
      expect(fakeClient.deleteSchedule).to.not.have.been.called;
    });

    it('should still render the row', () => {
      const row = el.shadowRoot?.querySelector('[data-schedule-id="armed-row"]');
      expect(row).to.exist;
    });
  });

  describe('when delete is armed and a click happens outside any delete button', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, {
        schedules: [makeSchedule({ id: 'armed-row' })],
      }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      fakeClient.deleteSchedule.resolves(create(DeleteScheduleResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('my-page');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;

      const button = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="armed-row"] .delete-schedule-button'
      );
      button?.click();
      await el.updateComplete;

      const heading = el.shadowRoot?.querySelector<HTMLElement>('.section-heading');
      heading?.click();
      await el.updateComplete;
    });

    it('should clear the armed confirmation state', () => {
      expect(el.confirmingDeleteId).to.equal(null);
    });

    it('should not call deleteSchedule', () => {
      expect(fakeClient.deleteSchedule).to.not.have.been.called;
    });

    it('should still render the row', () => {
      const row = el.shadowRoot?.querySelector('[data-schedule-id="armed-row"]');
      expect(row).to.exist;
    });
  });

  describe('when delete is armed and the auto-disarm timeout elapses', () => {
    let fakeClient: FakeAgentClient;
    let clock: sinon.SinonFakeTimers;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, {
        schedules: [makeSchedule({ id: 'armed-row' })],
      }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      fakeClient.deleteSchedule.resolves(create(DeleteScheduleResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('my-page');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;

      // Install fake timers AFTER all the async setup is settled, so we only
      // control the auto-disarm setTimeout.
      clock = sinon.useFakeTimers();

      const button = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="armed-row"] .delete-schedule-button'
      );
      button?.click();
      await el.updateComplete;

      await clock.tickAsync(5001);
      await el.updateComplete;
    });

    afterEach(() => {
      clock.restore();
    });

    it('should clear the armed confirmation state', () => {
      expect(el.confirmingDeleteId).to.equal(null);
    });

    it('should not call deleteSchedule', () => {
      expect(fakeClient.deleteSchedule).to.not.have.been.called;
    });
  });

  describe('when delete schedule fails after confirmation', () => {
    let fakeClient: FakeAgentClient;
    const failure = new Error('cannot delete');

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, {
        schedules: [makeSchedule({ id: 'sticky' })],
      }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      fakeClient.deleteSchedule.rejects(failure);
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('my-page');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;

      const armButton = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="sticky"] .delete-schedule-button'
      );
      armButton?.click();
      await el.updateComplete;

      const confirmButton = el.shadowRoot?.querySelector<HTMLButtonElement>(
        '[data-schedule-id="sticky"] .delete-schedule-button'
      );
      confirmButton?.click();
      await waitUntil(() => el.augmentedError !== null, 'error not set', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should set augmentedError', () => {
      expect(el.augmentedError).to.be.instanceOf(AugmentedError);
    });

    it('should preserve the original error message', () => {
      expect(el.augmentedError?.message).to.equal('cannot delete');
    });

    it('should render the error-display component', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.exist;
    });

    it('should keep the schedule in local state', () => {
      const ids = el.schedules.map((s) => s.id);
      expect(ids).to.deep.equal(['sticky']);
    });
  });

  describe('when close button is clicked', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, {
        schedules: [makeSchedule({ id: 'a' })],
      }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.openDialog('my-page');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;

      const closeBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.dialog-header .icon-button');
      closeBtn?.click();
      await el.updateComplete;
    });

    it('should set open to false', () => {
      expect(el.open).to.be.false;
    });

    it('should close the native dialog', () => {
      const dialog = el.shadowRoot?.querySelector('dialog');
      expect(dialog?.open).to.be.false;
    });

    it('should reset schedules', () => {
      expect(el.schedules).to.deep.equal([]);
    });

    it('should reset chat context', () => {
      expect(el.chatContext).to.equal(null);
    });
  });

  describe('when Escape key is pressed while dialog is open', () => {
    let closeDialogSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      const fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, { schedules: [] }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      closeDialogSpy = sinon.spy(el, '_closeDialog' as keyof typeof el);
      el.openDialog('my-page');
      await waitUntil(() => !el.loading, 'still loading', { timeout: 1000 });
      await el.updateComplete;

      const dialog = el.shadowRoot?.querySelector('dialog');
      dialog?.dispatchEvent(new Event('cancel', { cancelable: true }));
      await el.updateComplete;
    });

    it('should call _closeDialog', () => {
      expect(closeDialogSpy).to.have.been.calledOnce;
    });

    it('should set open to false', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when the dialog is rendered while not loading and without error', () => {
    let fakeClient: FakeAgentClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentDetailsDialog>(html`<agent-details-dialog></agent-details-dialog>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listSchedules.resolves(create(ListSchedulesResponseSchema, { schedules: [] }));
      fakeClient.getChatContext.resolves(create(GetChatContextResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentDetailsDialog['client'];

      el.augmentedError = new AugmentedError(new Error('Prior error'), ErrorKind.ERROR, 'error');
      await el.updateComplete;
    });

    it('should render the error-display when error is set', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.exist;
    });
  });
});
