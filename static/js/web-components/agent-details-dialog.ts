import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient, type Client } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { type Timestamp, timestampDate } from '@bufbuild/protobuf/wkt';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  AgentMetadataService,
  ListSchedulesRequestSchema,
  GetChatContextRequestSchema,
  DeleteScheduleRequestSchema,
  ScheduleStatus,
  type AgentSchedule,
  type ChatContext,
  type BackgroundActivityEntry,
} from '../gen/api/v1/agent_metadata_pb.js';
import { sharedStyles, dialogStyles } from './shared-styles.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import { NativeDialogMixin } from './native-dialog-mixin.js';
import './error-display.js';

/**
 * AgentDetailsDialog - A modal dialog that surfaces the agent.* page metadata
 * (schedules, chat context, background activity) for inspection and repair.
 *
 * Read-only in v1, with the exception of per-schedule deletion. Loads data
 * from the AgentMetadataService gRPC service when opened, displays it grouped
 * by section, and allows the user to delete individual schedules.
 */
export class AgentDetailsDialog extends NativeDialogMixin(LitElement) {
  static override readonly styles = dialogStyles(css`
      dialog {
        padding: 0;
        border: none;
        border-radius: 8px;
        background: var(--color-surface-elevated);
        max-width: 720px;
        width: 90%;
        max-height: 85vh;
        flex-direction: column;
        box-shadow: 0 10px 25px rgba(0, 0, 0, 0.3);
        animation: slideIn 0.2s ease-out;
        overflow: hidden;
      }

      .content {
        flex: 1;
        padding: 20px;
        overflow-y: auto;
        min-height: 200px;
      }

      .loading {
        display: flex;
        align-items: center;
        justify-content: center;
        min-height: 200px;
        font-size: 16px;
        color: var(--color-text-secondary);
        gap: 8px;
      }

      .section {
        margin-bottom: 24px;
      }

      .section:last-child {
        margin-bottom: 0;
      }

      .section-heading {
        font-size: 14px;
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        color: var(--color-text-muted);
        margin: 0 0 8px 0;
        padding-bottom: 4px;
        border-bottom: 1px solid var(--color-border-subtle);
      }

      .empty-message {
        font-style: italic;
        color: var(--color-text-secondary);
        padding: 8px 0;
      }

      .schedule-list,
      .activity-list,
      .goal-list,
      .pending-list {
        list-style: none;
        margin: 0;
        padding: 0;
        display: flex;
        flex-direction: column;
        gap: 8px;
      }

      .schedule-row,
      .activity-row {
        display: flex;
        flex-direction: column;
        gap: 4px;
        padding: 8px 12px;
        background: var(--color-surface-sunken);
        border: 1px solid var(--color-border-subtle);
        border-radius: 4px;
      }

      .schedule-row-header {
        display: flex;
        align-items: center;
        gap: 8px;
        justify-content: space-between;
      }

      .schedule-id,
      .activity-schedule-id {
        font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
        font-weight: 600;
        color: var(--color-text-primary);
      }

      .schedule-cron {
        font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
        color: var(--color-text-secondary);
        font-size: 12px;
      }

      .status-badge {
        display: inline-block;
        font-size: 11px;
        padding: 2px 8px;
        border-radius: 12px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        font-weight: 600;
        background: var(--color-surface-elevated);
        border: 1px solid var(--color-border-subtle);
        color: var(--color-text-secondary);
      }

      .status-badge.status-ok {
        background: var(--color-success-bg);
        color: var(--color-success-text);
        border-color: var(--color-success);
      }

      .status-badge.status-error,
      .status-badge.status-timeout {
        background: var(--color-error-bg);
        color: var(--color-error-text);
        border-color: var(--color-error);
      }

      .status-badge.status-running {
        background: var(--color-warning-bg);
        color: var(--color-warning-text);
        border-color: var(--color-warning);
      }

      .delete-schedule-button {
        background: none;
        border: 1px solid var(--color-border-default);
        color: var(--color-action-danger);
        padding: 2px 8px;
        font-size: 12px;
        border-radius: 4px;
        cursor: pointer;
      }

      .delete-schedule-button:hover:not(:disabled) {
        background: var(--color-action-danger);
        color: var(--color-text-inverse);
        border-color: var(--color-action-danger);
      }

      .delete-schedule-button:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }

      .activity-row-header {
        display: flex;
        align-items: center;
        gap: 8px;
        flex-wrap: wrap;
      }

      .activity-timestamp {
        color: var(--color-text-muted);
        font-size: 12px;
      }

      .activity-summary {
        color: var(--color-text-primary);
        font-size: 13px;
      }

      .chat-field {
        margin-bottom: 8px;
      }

      .chat-field-label {
        font-weight: 600;
        color: var(--color-text-secondary);
        font-size: 12px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        margin-bottom: 2px;
      }

      .chat-field-value {
        color: var(--color-text-primary);
      }

      .chat-list-item {
        padding: 2px 0;
      }

      .chat-last-updated {
        color: var(--color-text-muted);
        font-size: 12px;
        font-style: italic;
      }
    `
  );

  @property({ type: String })
  declare page: string;

  @state()
  declare loading: boolean;

  @state()
  declare deletingScheduleId: string | null;

  @state()
  declare augmentedError: AugmentedError | null;

  @state()
  declare schedules: AgentSchedule[];

  @state()
  declare chatContext: ChatContext | null;

  /**
   * Test seam: the gRPC client used to call AgentMetadataService. Tests can
   * replace this property with a fake client to avoid real network calls.
   */
  client: Client<typeof AgentMetadataService>;

  constructor() {
    super();
    this.page = '';
    this.loading = false;
    this.deletingScheduleId = null;
    this.augmentedError = null;
    this.schedules = [];
    this.chatContext = null;
    this.client = createClient(AgentMetadataService, getGrpcWebTransport());
  }

  public openDialog(page: string): void {
    this.page = page;
    this.open = true;
    void this.loadAgentDetails();
  }

  public close(): void {
    this._closeDialog();
  }

  protected _closeDialog(): void {
    this.open = false;
    this.schedules = [];
    this.chatContext = null;
    this.augmentedError = null;
    this.loading = false;
    this.deletingScheduleId = null;
  }

  public async loadAgentDetails(): Promise<void> {
    if (!this.page) return;

    try {
      this.loading = true;
      this.augmentedError = null;
      this.schedules = [];
      this.chatContext = null;
      this.requestUpdate();

      const listRequest = create(ListSchedulesRequestSchema, { page: this.page });
      const chatRequest = create(GetChatContextRequestSchema, { page: this.page });

      const [listResponse, chatResponse] = await Promise.all([
        this.client.listSchedules(listRequest),
        this.client.getChatContext(chatRequest),
      ]);

      this.schedules = listResponse.schedules;
      this.chatContext = chatResponse.chatContext ?? null;
    } catch (err) {
      this.augmentedError = AugmentErrorService.augmentError(err, 'loading agent details');
    } finally {
      this.loading = false;
      this.requestUpdate();
    }
  }

  private readonly _handleCancel = (): void => {
    this._closeDialog();
  };

  private readonly _handleDeleteSchedule = async (scheduleId: string): Promise<void> => {
    if (!this.page || !scheduleId) return;

    try {
      this.deletingScheduleId = scheduleId;
      this.augmentedError = null;
      this.requestUpdate();

      const request = create(DeleteScheduleRequestSchema, {
        page: this.page,
        scheduleId,
      });
      await this.client.deleteSchedule(request);

      this.schedules = this.schedules.filter((s) => s.id !== scheduleId);
    } catch (err) {
      this.augmentedError = AugmentErrorService.augmentError(err, 'deleting schedule');
    } finally {
      this.deletingScheduleId = null;
      this.requestUpdate();
    }
  };

  private formatTimestamp(timestamp: Timestamp | undefined): string {
    if (!timestamp) return '';
    const date = timestampDate(timestamp);
    return date.toLocaleString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  private statusLabel(status: ScheduleStatus): string {
    switch (status) {
      case ScheduleStatus.RUNNING: return 'Running';
      case ScheduleStatus.OK: return 'OK';
      case ScheduleStatus.ERROR: return 'Error';
      case ScheduleStatus.TIMEOUT: return 'Timeout';
      case ScheduleStatus.UNSPECIFIED:
      default:
        return 'Unknown';
    }
  }

  private statusClass(status: ScheduleStatus): string {
    switch (status) {
      case ScheduleStatus.OK: return 'status-ok';
      case ScheduleStatus.ERROR: return 'status-error';
      case ScheduleStatus.TIMEOUT: return 'status-timeout';
      case ScheduleStatus.RUNNING: return 'status-running';
      case ScheduleStatus.UNSPECIFIED:
      default:
        return 'status-unknown';
    }
  }

  private renderSchedules(): unknown {
    if (this.schedules.length === 0) {
      return html`<div class="empty-message">No schedules yet</div>`;
    }
    return html`
      <ul class="schedule-list">
        ${this.schedules.map((schedule) => html`
          <li class="schedule-row" data-schedule-id="${schedule.id}">
            <div class="schedule-row-header">
              <span class="schedule-id">${schedule.id}</span>
              <span class="status-badge ${this.statusClass(schedule.lastStatus)}">
                ${this.statusLabel(schedule.lastStatus)}
              </span>
              <button
                class="delete-schedule-button"
                aria-label="Delete schedule ${schedule.id}"
                ?disabled="${this.deletingScheduleId === schedule.id || this.loading}"
                @click="${(): void => { void this._handleDeleteSchedule(schedule.id); }}"
              >
                ${this.deletingScheduleId === schedule.id ? 'Deleting...' : 'Delete'}
              </button>
            </div>
            <div class="schedule-cron">${schedule.cron}</div>
          </li>
        `)}
      </ul>
    `;
  }

  private renderChatContext(): unknown {
    const ctx = this.chatContext;
    const empty =
      !ctx ||
      (
        !ctx.lastConversationSummary &&
        ctx.userGoals.length === 0 &&
        ctx.pendingItems.length === 0 &&
        !ctx.keyContext &&
        !ctx.lastUpdated
      );

    if (empty) {
      return html`<div class="empty-message">No chat memory recorded</div>`;
    }

    return html`
      ${ctx.lastConversationSummary ? html`
        <div class="chat-field">
          <div class="chat-field-label">Last conversation</div>
          <div class="chat-field-value">${ctx.lastConversationSummary}</div>
        </div>
      ` : nothing}
      ${ctx.userGoals.length > 0 ? html`
        <div class="chat-field">
          <div class="chat-field-label">User goals</div>
          <ul class="goal-list">
            ${ctx.userGoals.map((goal) => html`<li class="chat-list-item">${goal}</li>`)}
          </ul>
        </div>
      ` : nothing}
      ${ctx.pendingItems.length > 0 ? html`
        <div class="chat-field">
          <div class="chat-field-label">Pending items</div>
          <ul class="pending-list">
            ${ctx.pendingItems.map((item) => html`<li class="chat-list-item">${item}</li>`)}
          </ul>
        </div>
      ` : nothing}
      ${ctx.keyContext ? html`
        <div class="chat-field">
          <div class="chat-field-label">Key context</div>
          <div class="chat-field-value">${ctx.keyContext}</div>
        </div>
      ` : nothing}
      ${ctx.lastUpdated ? html`
        <div class="chat-last-updated">
          Last updated ${this.formatTimestamp(ctx.lastUpdated)}
        </div>
      ` : nothing}
    `;
  }

  private renderBackgroundActivity(): unknown {
    const entries: BackgroundActivityEntry[] = this.chatContext?.backgroundActivity ?? [];
    if (entries.length === 0) {
      return html`<div class="empty-message">No background activity</div>`;
    }

    // Newest first.
    const ordered = [...entries].reverse();

    return html`
      <ul class="activity-list">
        ${ordered.map((entry) => html`
          <li class="activity-row">
            <div class="activity-row-header">
              <span class="activity-timestamp">${this.formatTimestamp(entry.timestamp)}</span>
              <span class="activity-schedule-id">${entry.scheduleId}</span>
              <span class="status-badge ${this.statusClass(entry.status)}">
                ${this.statusLabel(entry.status)}
              </span>
            </div>
            ${entry.summary ? html`<div class="activity-summary">${entry.summary}</div>` : nothing}
          </li>
        `)}
      </ul>
    `;
  }

  private _renderContent(): unknown {
    if (this.loading) {
      return html`
        <div class="loading">
          <i class="fas fa-spinner fa-spin"></i>
          Loading agent details...
        </div>
      `;
    }
    if (this.augmentedError) {
      return html`
        <error-display .augmentedError=${this.augmentedError}></error-display>
      `;
    }

    return html`
      <section class="section" aria-labelledby="agent-details-schedules-heading">
        <h3 id="agent-details-schedules-heading" class="section-heading">Schedules</h3>
        ${this.renderSchedules()}
      </section>
      <section class="section" aria-labelledby="agent-details-chat-heading">
        <h3 id="agent-details-chat-heading" class="section-heading">Chat memory</h3>
        ${this.renderChatContext()}
      </section>
      <section class="section" aria-labelledby="agent-details-activity-heading">
        <h3 id="agent-details-activity-heading" class="section-heading">Background activity</h3>
        ${this.renderBackgroundActivity()}
      </section>
    `;
  }

  override render() {
    return html`
      ${sharedStyles}
      <dialog aria-labelledby="agent-details-dialog-title" @cancel="${this._handleDialogCancel}">
        <div class="dialog-header system-font">
          <h2 id="agent-details-dialog-title" class="dialog-title">Agent Details</h2>
          <button
            class="button-base icon-button"
            aria-label="Close dialog"
            @click="${this._handleCancel}"
            ?disabled="${this.deletingScheduleId !== null}"
          >×</button>
        </div>
        <div class="content">
          ${this._renderContent()}
        </div>
        <div class="footer">
          <button
            class="button-base button-secondary button-large border-radius-small"
            @click="${this._handleCancel}"
            ?disabled="${this.deletingScheduleId !== null}"
          >
            Close
          </button>
        </div>
      </dialog>
    `;
  }
}

customElements.define('agent-details-dialog', AgentDetailsDialog);

declare global {
  interface HTMLElementTagNameMap {
    'agent-details-dialog': AgentDetailsDialog;
  }
}
