import { html, fixture, expect, waitUntil } from '@open-wc/testing';
import sinon from 'sinon';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import { AgentSelector } from './agent-selector.js';
import {
  AgentInfoSchema,
  AgentProtocol,
  ListAgentsResponseSchema,
  GetPageAgentResponseSchema,
  SetPageAgentResponseSchema,
} from '../gen/api/v1/agent_inventory_pb.js';
import './agent-selector.js';

function timeout(ms: number, message: string): Promise<never> {
  return new Promise((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms)
  );
}

interface FakeInventoryClient {
  listAgents: sinon.SinonStub;
  getPageAgent: sinon.SinonStub;
  setPageAgent: sinon.SinonStub;
}

function buildFakeClient(): FakeInventoryClient {
  return {
    listAgents: sinon.stub(),
    getPageAgent: sinon.stub(),
    setPageAgent: sinon.stub(),
  };
}

function makeAgent(overrides: Partial<{
  id: string;
  displayName: string;
  description: string;
  available: boolean;
  protocol: AgentProtocol;
}> = {}): ReturnType<typeof create<typeof AgentInfoSchema>> {
  return create(AgentInfoSchema, {
    id: overrides.id ?? 'local-acp',
    displayName: overrides.displayName ?? 'Local Agent',
    description: overrides.description ?? 'Local ACP pool',
    available: overrides.available ?? true,
    protocol: overrides.protocol ?? AgentProtocol.ACP_LOCAL,
  });
}

describe('AgentSelector', () => {
  let el: AgentSelector;

  afterEach(() => {
    sinon.restore();
    if (el) {
      el.remove();
    }
  });

  describe('basic existence', () => {
    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);
      await el.updateComplete;
    });

    it('should exist', () => {
      expect(el).not.to.equal(null);
    });

    it('should be an instance of AgentSelector', () => {
      expect(el).to.be.instanceOf(AgentSelector);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName.toLowerCase()).to.equal('agent-selector');
    });

    it('should have an empty page by default', () => {
      expect(el.page).to.equal('');
    });

    it('should render a current-agent button', () => {
      const btn = el.shadowRoot?.querySelector('.current-agent');
      expect(btn).not.to.equal(null);
    });

    it('should render a dropdown', () => {
      const dropdown = el.shadowRoot?.querySelector('.dropdown');
      expect(dropdown).not.to.equal(null);
    });

    it('should have the dropdown hidden by default', () => {
      const dropdown = el.shadowRoot?.querySelector('.dropdown');
      expect(dropdown?.hasAttribute('hidden')).to.equal(true);
    });
  });

  describe('when page is set and client resolves with agents', () => {
    let fakeClient: FakeInventoryClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(
        create(ListAgentsResponseSchema, { agents: [makeAgent()] })
      );
      fakeClient.getPageAgent.resolves(
        create(GetPageAgentResponseSchema, { agent: makeAgent(), explicitlySet: false })
      );
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should call listAgents', () => {
      expect(fakeClient.listAgents.calledOnce).to.equal(true);
    });

    it('should call getPageAgent with the page', () => {
      const callArgs = fakeClient.getPageAgent.getCall(0).args[0];
      expect(callArgs.page).to.equal('test-page');
    });

    it('should display the agent name in the button', () => {
      const btn = el.shadowRoot?.querySelector('.current-agent');
      expect(btn?.textContent).to.include('Local Agent');
    });

    it('should show an available dot for an available agent', () => {
      const dot = el.shadowRoot?.querySelector('.current-agent .availability-dot');
      expect(dot?.classList.contains('available')).to.equal(true);
    });
  });

  describe('when effective agent matches the desired agent', () => {
    let fakeClient: FakeInventoryClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(
        create(ListAgentsResponseSchema, { agents: [makeAgent()] })
      );
      fakeClient.getPageAgent.resolves(
        create(GetPageAgentResponseSchema, { agent: makeAgent(), explicitlySet: true })
      );
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      el.effectiveAgentId = 'local-acp';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should not show the switching hint', () => {
      const hint = el.shadowRoot?.querySelector('.switching-hint');
      expect(hint === null).to.equal(true);
    });
  });

  describe('when effective agent diverges from the desired agent', () => {
    let fakeClient: FakeInventoryClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(
        create(ListAgentsResponseSchema, { agents: [makeAgent()] })
      );
      fakeClient.getPageAgent.resolves(
        create(GetPageAgentResponseSchema, { agent: makeAgent(), explicitlySet: true })
      );
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      el.effectiveAgentId = 'other-agent';
      el.effectiveAgentDisplayName = 'Other Agent';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should show the switching hint', () => {
      const hint = el.shadowRoot?.querySelector('.switching-hint');
      expect(hint).to.not.equal(null);
      expect(hint?.textContent).to.include('switching…');
    });

    it('should clear the hint when the effective agent catches up', async () => {
      el.effectiveAgentId = 'local-acp';
      await el.updateComplete;

      const hint = el.shadowRoot?.querySelector('.switching-hint');
      expect(hint === null).to.equal(true);
    });
  });

  describe('when effective agent is empty', () => {
    let fakeClient: FakeInventoryClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(
        create(ListAgentsResponseSchema, { agents: [makeAgent()] })
      );
      fakeClient.getPageAgent.resolves(
        create(GetPageAgentResponseSchema, { agent: makeAgent(), explicitlySet: false })
      );
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      el.effectiveAgentId = '';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should not show the switching hint', () => {
      const hint = el.shadowRoot?.querySelector('.switching-hint');
      expect(hint === null).to.equal(true);
    });
  });

  describe('when the desired agent is unavailable', () => {
    let fakeClient: FakeInventoryClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(
        create(ListAgentsResponseSchema, {
          agents: [makeAgent({ available: false, displayName: 'Offline Agent' })],
        })
      );
      fakeClient.getPageAgent.resolves(
        create(GetPageAgentResponseSchema, {
          agent: makeAgent({ available: false, displayName: 'Offline Agent' }),
          explicitlySet: true,
        })
      );
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should show the unavailable hint under the trigger', () => {
      const hint = el.shadowRoot?.querySelector('.desired-unavailable-hint');
      expect(hint).to.not.equal(null);
      expect(hint?.textContent).to.include('bound agent unavailable');
    });
  });

  describe('when the desired agent is available', () => {
    let fakeClient: FakeInventoryClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(
        create(ListAgentsResponseSchema, { agents: [makeAgent()] })
      );
      fakeClient.getPageAgent.resolves(
        create(GetPageAgentResponseSchema, { agent: makeAgent() })
      );
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should not show the unavailable hint', () => {
      const hint = el.shadowRoot?.querySelector('.desired-unavailable-hint');
      expect(hint === null).to.equal(true);
    });

    describe('when refresh() is called', () => {
      let listAgentsCallCount: number;

      beforeEach(async () => {
        listAgentsCallCount = fakeClient.listAgents.callCount;
        el.refresh();
        await waitUntil(
          () => fakeClient.listAgents.callCount > listAgentsCallCount,
          'listAgents not re-called',
          { timeout: 1000 },
        );
      });

      it('should refetch the agent inventory', () => {
        expect(fakeClient.listAgents.callCount).to.equal(listAgentsCallCount + 1);
      });

      it('should refetch the page binding', () => {
        expect(fakeClient.getPageAgent.callCount).to.equal(listAgentsCallCount + 1);
      });
    });
  });

  describe('when setPageAgent fails with FailedPrecondition', () => {
    let fakeClient: FakeInventoryClient;
    const conflict = new ConnectError('binding was updated concurrently', Code.FailedPrecondition);

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(
        create(ListAgentsResponseSchema, {
          agents: [
            makeAgent(),
            makeAgent({ id: 'agent-2', displayName: 'Second Agent', available: false }),
          ],
        })
      );
      fakeClient.getPageAgent
        .resolves(
          create(GetPageAgentResponseSchema, {
            agent: makeAgent(),
            explicitlySet: true,
            updatedAt: '2026-10-01T00:00:00Z',
          }),
        )
        .onSecondCall()
        .resolves(
          create(GetPageAgentResponseSchema, {
            agent: makeAgent(),
            explicitlySet: true,
            updatedAt: '2026-10-03T00:00:00Z',
          }),
        );
      fakeClient.setPageAgent.rejects(conflict);
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;

      // open dropdown and click the (different) second agent
      const btn = el.shadowRoot?.querySelector<HTMLButtonElement>('.current-agent');
      btn?.click();
      await el.updateComplete;

      const options = el.shadowRoot?.querySelectorAll<HTMLButtonElement>('.agent-option');
      options?.[1]?.click();
      await waitUntil(() => fakeClient.setPageAgent.called, 'setPageAgent not called', { timeout: 1000 });
      await waitUntil(() => !!el['_error'], 'error not set', { timeout: 2000 });
      await el.updateComplete;
    });

    it('should refuse to overwrite and surface the conflict error', () => {
      // The last client state is the refetched binding, not the selected agent.
      expect(el['_currentAgentId']).to.equal('local-acp');
      expect(el['_error']?.message).to.include('binding changed');
      expect(el['_currentAgentUpdatedAt']).to.equal('2026-10-03T00:00:00Z');
    });
  });

  describe('when setPageAgent succeeds with OCC and updatedAt', () => {
    let fakeClient: FakeInventoryClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(
        create(ListAgentsResponseSchema, {
          agents: [makeAgent(), makeAgent({ id: 'agent-2', displayName: 'Second Agent' })],
        })
      );
      fakeClient.getPageAgent.resolves(
        create(GetPageAgentResponseSchema, {
          agent: makeAgent(),
          explicitlySet: true,
          updatedAt: '2026-10-01T00:00:00Z',
        })
      );
      fakeClient.setPageAgent.resolves(
        create(SetPageAgentResponseSchema, {
          agent: makeAgent({ id: 'agent-2', displayName: 'Second Agent' }),
          updatedAt: '2026-10-02T00:00:00Z',
        }),
      );
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;

      const btn = el.shadowRoot?.querySelector<HTMLButtonElement>('.current-agent');
      btn?.click();
      await el.updateComplete;

      const options = el.shadowRoot?.querySelectorAll<HTMLButtonElement>('.agent-option');
      options?.[1]?.click();
      await waitUntil(() => fakeClient.setPageAgent.called, 'setPageAgent not called', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should send the expectedUpdatedAt OCC guard', () => {
      const callArgs = fakeClient.setPageAgent.getCall(0).args[0];
      expect(callArgs.expectedUpdatedAt).to.equal('2026-10-01T00:00:00Z');
    });

    it('should record the new updatedAt', () => {
      expect(el['_currentAgentUpdatedAt']).to.equal('2026-10-02T00:00:00Z');
    });
  });

  describe('when page is set and client resolves with no agents', () => {
    let fakeClient: FakeInventoryClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(create(ListAgentsResponseSchema, { agents: [] }));
      fakeClient.getPageAgent.resolves(create(GetPageAgentResponseSchema, {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should show "No agents available" in dropdown when opened', async () => {
      const btn = el.shadowRoot?.querySelector<HTMLButtonElement>('.current-agent');
      btn?.click();
      await el.updateComplete;

      const dropdown = el.shadowRoot?.querySelector('.dropdown');
      expect(dropdown?.textContent).to.include('No agents available');
    });
  });

  describe('when page is set and client rejects', () => {
    let fakeClient: FakeInventoryClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.rejects(new Error('network failure'));
      fakeClient.getPageAgent.rejects(new Error('network failure'));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should show the error message in dropdown when opened', async () => {
      const btn = el.shadowRoot?.querySelector<HTMLButtonElement>('.current-agent');
      btn?.click();
      await el.updateComplete;

      const dropdown = el.shadowRoot?.querySelector('.dropdown');
      expect(dropdown?.textContent).to.include('network failure');
    });
  });

  describe('when toggling the dropdown', () => {
    let fakeClient: FakeInventoryClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(
        create(ListAgentsResponseSchema, { agents: [makeAgent()] })
      );
      fakeClient.getPageAgent.resolves(
        create(GetPageAgentResponseSchema, { agent: makeAgent() })
      );
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;
    });

    describe('when the button is clicked', () => {
      beforeEach(async () => {
        const btn = el.shadowRoot?.querySelector<HTMLButtonElement>('.current-agent');
        btn?.click();
        await el.updateComplete;
      });

      it('should show the dropdown', () => {
        const dropdown = el.shadowRoot?.querySelector('.dropdown');
        expect(dropdown?.hasAttribute('hidden')).to.equal(false);
      });

      it('should render agent options in the dropdown', () => {
        const options = el.shadowRoot?.querySelectorAll('.agent-option');
        expect(options).to.have.lengthOf(1);
      });

      it('should render the agent name in the option', () => {
        const option = el.shadowRoot?.querySelector('.agent-option');
        expect(option?.textContent).to.include('Local Agent');
      });

      describe('when the button is clicked again', () => {
        beforeEach(async () => {
          const btn = el.shadowRoot?.querySelector<HTMLButtonElement>('.current-agent');
          btn?.click();
          await el.updateComplete;
        });

        it('should hide the dropdown', () => {
          const dropdown = el.shadowRoot?.querySelector('.dropdown');
          expect(dropdown?.hasAttribute('hidden')).to.equal(true);
        });
      });
    });
  });

  describe('when an agent option is clicked', () => {
    let fakeClient: FakeInventoryClient;
    const secondAgent = makeAgent({
      id: 'agent-2',
      displayName: 'Second Agent',
      available: false,
    });

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(
        create(ListAgentsResponseSchema, { agents: [makeAgent(), secondAgent] })
      );
      fakeClient.getPageAgent.resolves(
        create(GetPageAgentResponseSchema, { agent: makeAgent() })
      );
      fakeClient.setPageAgent.resolves(
        create(SetPageAgentResponseSchema, { agent: secondAgent })
      );
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;

      // open dropdown
      const btn = el.shadowRoot?.querySelector<HTMLButtonElement>('.current-agent');
      btn?.click();
      await el.updateComplete;

      // click second agent option
      const options = el.shadowRoot?.querySelectorAll<HTMLButtonElement>('.agent-option');
      options?.[1]?.click();
      await waitUntil(() => fakeClient.setPageAgent.called, 'setPageAgent not called', { timeout: 1000 });
      await el.updateComplete;
    });

    it('should call setPageAgent with the selected agent id and page', () => {
      const callArgs = fakeClient.setPageAgent.getCall(0).args[0];
      expect(callArgs.agentId).to.equal('agent-2');
      expect(callArgs.page).to.equal('test-page');
    });

    it('should close the dropdown after selection', () => {
      const dropdown = el.shadowRoot?.querySelector('.dropdown');
      expect(dropdown?.hasAttribute('hidden')).to.equal(true);
    });

    it('should update the current agent id to the selected agent', () => {
      expect(el['_currentAgentId']).to.equal('agent-2');
    });
  });

  describe('when clicking the same agent that is already selected', () => {
    let fakeClient: FakeInventoryClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(
        create(ListAgentsResponseSchema, { agents: [makeAgent()] })
      );
      fakeClient.getPageAgent.resolves(
        create(GetPageAgentResponseSchema, { agent: makeAgent() })
      );
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;

      // open dropdown
      const btn = el.shadowRoot?.querySelector<HTMLButtonElement>('.current-agent');
      btn?.click();
      await el.updateComplete;

      // click the already-selected agent
      const option = el.shadowRoot?.querySelector<HTMLButtonElement>('.agent-option');
      option?.click();
      await el.updateComplete;
    });

    it('should not call setPageAgent', () => {
      expect(fakeClient.setPageAgent.callCount).to.equal(0);
    });

    it('should close the dropdown', () => {
      const dropdown = el.shadowRoot?.querySelector('.dropdown');
      expect(dropdown?.hasAttribute('hidden')).to.equal(true);
    });
  });

  describe('when an unavailable agent is in the list', () => {
    let fakeClient: FakeInventoryClient;

    beforeEach(async () => {
      el = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(
        create(ListAgentsResponseSchema, {
          agents: [makeAgent({ available: false, displayName: 'Offline Agent' })],
        })
      );
      fakeClient.getPageAgent.resolves(
        create(GetPageAgentResponseSchema, {
          agent: makeAgent({ available: false }),
        })
      );
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      el.client = fakeClient as unknown as AgentSelector['client'];

      el.page = 'test-page';
      await waitUntil(() => !el['_loading'], 'still loading', { timeout: 1000 });
      await el.updateComplete;

      // open dropdown
      const btn = el.shadowRoot?.querySelector<HTMLButtonElement>('.current-agent');
      btn?.click();
      await el.updateComplete;
    });

    it('should show "Not connected" label for unavailable agent', () => {
      const option = el.shadowRoot?.querySelector('.agent-option');
      expect(option?.textContent).to.include('Not connected');
    });
  });

  describe('viewport-aware positioning', () => {
    let fakeClient: FakeInventoryClient;

    /**
     * Creates a component with an open dropdown, stubs `_getMeasurements` with
     * the supplied values, and directly calls `_repositionDropdown()` so the
     * tests are not sensitive to the exact call-timing inside `updated()`.
     */
    async function buildRepositioned(
      triggerRect: DOMRect,
      viewport: { vpW: number; vpH: number },
    ): Promise<{ el: AgentSelector; dropdown: HTMLElement }> {
      const element = await Promise.race([
        fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
        timeout(5000, 'Component fixture timed out'),
      ]);

      fakeClient = buildFakeClient();
      fakeClient.listAgents.resolves(
        create(ListAgentsResponseSchema, { agents: [makeAgent()] })
      );
      fakeClient.getPageAgent.resolves(
        create(GetPageAgentResponseSchema, { agent: makeAgent() })
      );
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- injecting fake client for tests
      element.client = fakeClient as unknown as AgentSelector['client'];

      element.page = 'test-page';
      await waitUntil(() => !element['_loading'], 'still loading', { timeout: 1000 });
      await element.updateComplete;

      // Open the dropdown so the element is visible in the shadow DOM
      element.shadowRoot!.querySelector<HTMLButtonElement>('.current-agent')!.click();
      await element.updateComplete;

      // Stub the measurement seam and invoke positioning directly so we test
      // the logic independently of Lit's updated() call timing.
      sinon.stub(element, '_getMeasurements').returns({ rect: triggerRect, ...viewport });
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- calling private method under test
      (element as unknown as { _repositionDropdown(): void })._repositionDropdown();

      const dropdown = element.shadowRoot!.querySelector<HTMLElement>('.dropdown')!;
      return { el: element, dropdown };
    }

    afterEach(() => {
      sinon.restore();
    });

    describe('when trigger has sufficient space below in the viewport', () => {
      let dropdown: HTMLElement;

      beforeEach(async () => {
        // trigger at top of page: bottom=80, vpH=800 → spaceBelow=712 ≥ 120
        const result = await buildRepositioned(
          { top: 50, bottom: 80, left: 100, right: 220, width: 120, height: 30, x: 100, y: 50, toJSON: () => ({}) } as DOMRect,
          { vpW: 1200, vpH: 800 },
        );
        el = result.el;
        dropdown = result.dropdown;
      });

      it('should position the dropdown below the trigger', () => {
        const top = parseFloat(dropdown.style.top);
        // top should be rect.bottom (80) + gapPx (4) = 84
        expect(top).to.be.greaterThan(80);
      });

      it('should left-align the dropdown to the trigger', () => {
        const left = parseFloat(dropdown.style.left);
        // left edge aligns with rect.left (100), no clamping needed
        expect(left).to.equal(100);
      });

      it('should set a max-height on the dropdown', () => {
        expect(dropdown.style.maxHeight).to.not.equal('');
      });
    });

    describe('when trigger is near the bottom of the viewport', () => {
      let dropdown: HTMLElement;

      beforeEach(async () => {
        // trigger near bottom: top=720, bottom=750, vpH=800
        // spaceBelow = 800 - 750 - 8 = 42 < 120
        // spaceAbove = 720 - 8 = 712 > 42 → flip above
        const result = await buildRepositioned(
          { top: 720, bottom: 750, left: 100, right: 220, width: 120, height: 30, x: 100, y: 720, toJSON: () => ({}) } as DOMRect,
          { vpW: 1200, vpH: 800 },
        );
        el = result.el;
        dropdown = result.dropdown;
      });

      it('should position the dropdown above the trigger', () => {
        const top = parseFloat(dropdown.style.top);
        expect(top).to.be.lessThan(720);
      });
    });

    describe('when trigger is near the right edge of the viewport', () => {
      let dropdown: HTMLElement;

      beforeEach(async () => {
        // trigger left=1050, vpW=1200 → 1050+220=1270 > 1192 → clamp
        // expected left = max(8, 1200 - 220 - 8) = 972
        const result = await buildRepositioned(
          { top: 50, bottom: 80, left: 1050, right: 1170, width: 120, height: 30, x: 1050, y: 50, toJSON: () => ({}) } as DOMRect,
          { vpW: 1200, vpH: 800 },
        );
        el = result.el;
        dropdown = result.dropdown;
      });

      it('should clamp the dropdown within the right viewport edge', () => {
        const left = parseFloat(dropdown.style.left);
        expect(left + 220).to.be.at.most(1200 - 8);
      });
    });

  });

  describe('event listener wiring', () => {
    describe('when connected to DOM', () => {
      let addEventListenerSpy: sinon.SinonSpy;

      beforeEach(async () => {
        addEventListenerSpy = sinon.spy(document, 'addEventListener');
        el = await Promise.race([
          fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
          timeout(5000, 'Component fixture timed out'),
        ]);
        await el.updateComplete;
      });

      it('should add click event listener to document', () => {
        expect(addEventListenerSpy).to.have.been.calledWith('click', el['_handleGlobalClick']);
      });
    });

    describe('when disconnected from DOM', () => {
      let removeEventListenerSpy: sinon.SinonSpy;

      beforeEach(async () => {
        el = await Promise.race([
          fixture<AgentSelector>(html`<agent-selector></agent-selector>`),
          timeout(5000, 'Component fixture timed out'),
        ]);
        await el.updateComplete;

        removeEventListenerSpy = sinon.spy(document, 'removeEventListener');
        el.remove();
      });

      it('should remove the click event listener from document', () => {
        expect(removeEventListenerSpy).to.have.been.calledWith('click', el['_handleGlobalClick']);
      });
    });
  });
});
