import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient, type Client } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  AgentInventoryService,
  GetPageAgentRequestSchema,
  ListAgentsRequestSchema,
  SetPageAgentRequestSchema,
  type AgentInfo,
} from '../gen/api/v1/agent_inventory_pb.js';
import { colorCSS, typographyCSS } from './shared-styles.js';

declare global {
  interface HTMLElementTagNameMap {
    'agent-selector': AgentSelector;
  }
}

/**
 * AgentSelector — a compact inline control that shows which agent is currently
 * bound to the page and lets any participant switch to a different agent from
 * the registered inventory.
 *
 * The selection is persisted to the page via SetPageAgent — it survives
 * session close/reopen and pool restarts. Switching mid-session starts fresh
 * with the new agent; no conversation context is transferred.
 */
export class AgentSelector extends LitElement {
  static override readonly styles = [
    colorCSS,
    typographyCSS,
    css`
      :host {
        display: block;
      }

      .selector-wrapper {
        position: relative;
        display: inline-flex;
        align-items: center;
        gap: 6px;
      }

      .current-agent {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        background: none;
        border: 1px solid var(--color-editor-border);
        border-radius: 4px;
        color: var(--color-text-muted);
        font-size: 0.8rem;
        padding: 3px 8px;
        cursor: pointer;
        white-space: nowrap;
      }

      .current-agent:hover:not(:disabled) {
        color: var(--color-text-primary);
        border-color: var(--color-text-muted);
      }

      .current-agent:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }

      .availability-dot {
        width: 6px;
        height: 6px;
        border-radius: 50%;
        flex-shrink: 0;
      }

      .availability-dot.available {
        background: var(--color-success, #28a745);
      }

      .availability-dot.unavailable {
        background: var(--color-text-muted);
      }

      .dropdown {
        position: absolute;
        top: calc(100% + 4px);
        left: 0;
        min-width: 220px;
        background: var(--color-editor-surface);
        border: 1px solid var(--color-editor-border);
        border-radius: 6px;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
        z-index: 200;
        overflow: hidden;
      }

      .dropdown[hidden] {
        display: none;
      }

      .dropdown-header {
        padding: 8px 12px 4px;
        font-size: 0.72rem;
        color: var(--color-text-muted);
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        border-bottom: 1px solid var(--color-editor-border);
      }

      .agent-option {
        display: flex;
        align-items: flex-start;
        gap: 8px;
        padding: 8px 12px;
        cursor: pointer;
        border: none;
        background: none;
        width: 100%;
        text-align: left;
        color: var(--color-text-primary);
        font-size: 0.85rem;
      }

      .agent-option:hover {
        background: rgba(255, 255, 255, 0.06);
      }

      .agent-option.selected {
        background: rgba(255, 255, 255, 0.04);
      }

      .agent-option-body {
        flex: 1;
        min-width: 0;
      }

      .agent-option-name {
        font-weight: 500;
        display: flex;
        align-items: center;
        gap: 6px;
      }

      .agent-option-desc {
        font-size: 0.75rem;
        color: var(--color-text-muted);
        margin-top: 2px;
        white-space: normal;
        line-height: 1.3;
      }

      .agent-option-unavailable {
        font-size: 0.72rem;
        color: var(--color-warning, #ffc107);
        margin-top: 2px;
      }

      .loading-text {
        padding: 8px 12px;
        font-size: 0.8rem;
        color: var(--color-text-muted);
      }

      .error-text {
        padding: 8px 12px;
        font-size: 0.8rem;
        color: var(--color-error);
      }
    `,
  ];

  @property({ type: String })
  declare page: string;

  @state()
  declare _agents: AgentInfo[];

  @state()
  declare _currentAgentId: string;

  @state()
  declare _open: boolean;

  @state()
  declare _loading: boolean;

  @state()
  declare _error: Error | null;

  /**
   * Test seam: the gRPC client used to call AgentInventoryService. Tests can
   * replace this property with a fake client to avoid real network calls.
   */
  client: Client<typeof AgentInventoryService>;

  private readonly _handleGlobalClick: (e: MouseEvent) => void;

  constructor() {
    super();
    this.page = '';
    this._agents = [];
    this._currentAgentId = '';
    this._open = false;
    this._loading = false;
    this._error = null;
    this.client = createClient(AgentInventoryService, getGrpcWebTransport());
    this._handleGlobalClick = this._onGlobalClick.bind(this);
  }

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('click', this._handleGlobalClick);
    void this._loadData();
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('click', this._handleGlobalClick);
  }

  override updated(changed: Map<string, unknown>): void {
    super.updated(changed);
    if (changed.has('page') && this.page) {
      void this._loadData();
    }
  }

  private async _loadData(): Promise<void> {
    if (!this.page) return;

    this._loading = true;
    this._error = null;

    try {
      const [listResp, pageResp] = await Promise.all([
        this.client.listAgents(create(ListAgentsRequestSchema, {})),
        this.client.getPageAgent(create(GetPageAgentRequestSchema, { page: this.page })),
      ]);
      this._agents = listResp.agents;
      this._currentAgentId = pageResp.agent?.id ?? '';
    } catch (err) {
      this._error = err instanceof Error ? err : new Error(String(err));
    } finally {
      this._loading = false;
    }
  }

  private async _selectAgent(agentId: string): Promise<void> {
    if (agentId === this._currentAgentId) {
      this._open = false;
      return;
    }

    try {
      const resp = await this.client.setPageAgent(
        create(SetPageAgentRequestSchema, { page: this.page, agentId }),
      );
      this._currentAgentId = resp.agent?.id ?? agentId;
    } catch (err) {
      this._error = err instanceof Error ? err : new Error(String(err));
    } finally {
      this._open = false;
    }
  }

  private _toggleDropdown(): void {
    this._open = !this._open;
  }

  private _onGlobalClick(e: MouseEvent): void {
    if (!this._open) return;
    const composedTarget = e.composedPath()[0];
    if (!(composedTarget instanceof Node) || !this.shadowRoot?.contains(composedTarget)) {
      this._open = false;
    }
  }

  private get _currentAgent(): AgentInfo | undefined {
    return this._agents.find((a) => a.id === this._currentAgentId);
  }

  private _renderCurrentLabel(): string {
    if (this._loading) return 'Loading…';
    const agent = this._currentAgent;
    if (!agent) return this._currentAgentId || 'Agent';
    return agent.displayName;
  }

  override render() {
    const current = this._currentAgent;
    const available = current?.available ?? false;

    return html`
      <div class="selector-wrapper">
        <button
          class="current-agent"
          ?disabled=${this._loading}
          @click=${this._toggleDropdown}
          aria-haspopup="listbox"
          aria-expanded=${this._open ? 'true' : 'false'}
          title="Switch agent"
        >
          <span class="availability-dot ${available ? 'available' : 'unavailable'}"></span>
          ${this._renderCurrentLabel()}
          <i class="fa-solid fa-chevron-down" style="font-size:0.65rem"></i>
        </button>

        <div class="dropdown" ?hidden=${!this._open} role="listbox">
          <div class="dropdown-header">Chat agent</div>
          ${this._renderDropdownContent()}
        </div>
      </div>
    `;
  }

  private _renderDropdownContent() {
    if (this._loading) {
      return html`<div class="loading-text">Loading agents…</div>`;
    }
    if (this._error) {
      return html`<div class="error-text">${this._error.message}</div>`;
    }
    if (this._agents.length === 0) {
      return html`<div class="loading-text">No agents available</div>`;
    }

    return this._agents.map((agent) => html`
      <button
        class="agent-option ${agent.id === this._currentAgentId ? 'selected' : ''}"
        role="option"
        aria-selected=${agent.id === this._currentAgentId ? 'true' : 'false'}
        @click=${() => this._selectAgent(agent.id)}
      >
        <span class="availability-dot ${agent.available ? 'available' : 'unavailable'}" style="margin-top:3px"></span>
        <div class="agent-option-body">
          <div class="agent-option-name">${agent.displayName}</div>
          ${agent.description
            ? html`<div class="agent-option-desc">${agent.description}</div>`
            : nothing}
          ${!agent.available
            ? html`<div class="agent-option-unavailable">Not connected</div>`
            : nothing}
        </div>
      </button>
    `);
  }
}

customElements.define('agent-selector', AgentSelector);
