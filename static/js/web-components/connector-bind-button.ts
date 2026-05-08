// connector-bind-button — embedded inside <wiki-checklist> as the
// single subscribe affordance. Renders nothing when the user has no
// connectors authenticated. Otherwise:
//
//   • Unsubscribed state: a "Subscribe to <product>" button. If only one
//     connector is authenticated, click goes straight to that connector's
//     list picker (default-to-authed). If multiple are authenticated, the
//     user picks the connector first, then the list.
//
//   • Subscribed state: a "✓ Synced with <product> <list-type> <title>"
//     pill, plus an unsubscribe interlock. If the subscription is paused,
//     a <connector-paused-badge> takes the place of the synced pill.
//
// Word "connector" never appears in user-facing copy — UI uses the
// product names ("Google Keep", "Google Tasks").

import { css, html, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  ConnectorService,
  ConnectorKind,
  GetChecklistBindingStateRequestSchema,
  GetStateRequestSchema,
  ListRemoteListsRequestSchema,
  BindRequestSchema,
  UnbindRequestSchema,
  SyncNowRequestSchema,
} from '../gen/api/v1/connector_service_pb.js';
import type {
  ChecklistBindingState,
  RemoteListSummary,
  BindingState,
} from '../gen/api/v1/connector_service_pb.js';
import {
  foundationCSS,
  buttonCSS,
  inputCSS,
  pillCSS,
  sharedStyles,
} from './shared-styles.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import './error-display.js';
import './confirmation-interlock-button.js';
import './connector-paused-badge.js';

// Local layout overrides — keep the subscribe affordance & sync badge
// visually subordinate to the checklist itself.
const localCSS = css`
  :host {
    display: block;
    margin-top: 8px;
  }
  .subscribe-row {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  /* Subscribe-mode trigger: ghost-style, matches the muted text scale of
     a checklist's caption row, doesn't compete with primary actions. */
  .bind-trigger {
    background: transparent;
    border: 1px solid var(--color-border-default, rgba(0, 0, 0, 0.12));
    color: var(--color-text-secondary, #6c757d);
    font-size: 11px;
    font-weight: 500;
    padding: 4px 10px;
    border-radius: 4px;
    cursor: pointer;
    line-height: 1.2;
  }
  .bind-trigger:hover {
    color: var(--color-text-primary, inherit);
    border-color: var(--color-border-strong, rgba(0, 0, 0, 0.24));
  }
  /* Connector-pick variant: a row of "Google Keep" / "Google Tasks"
     buttons. Uses the same ghost weight as the trigger so it doesn't
     look like a modal. */
  .connector-choice {
    background: transparent;
    border: 1px solid var(--color-border-default, rgba(0, 0, 0, 0.12));
    color: var(--color-text-secondary, #6c757d);
    font-size: 11px;
    font-weight: 500;
    padding: 4px 10px;
    border-radius: 4px;
    cursor: pointer;
    line-height: 1.2;
  }
  .connector-choice:hover {
    color: var(--color-text-primary, inherit);
    border-color: var(--color-border-strong, rgba(0, 0, 0, 0.24));
  }
  /* Bound-mode badge: muted info text + small unsubscribe affordance. */
  .sync-badge {
    color: var(--color-text-muted, #868e96);
    font-size: 11px;
    font-weight: 400;
    line-height: 1.2;
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .sync-badge::before {
    content: '✓';
    color: var(--color-success, #2f9e44);
    font-weight: 600;
  }
  /* Strip the default secondary-button weight off the interlock's
     trigger when used here — matches the muted caption surroundings. */
  confirmation-interlock-button::part(trigger) {
    background: transparent;
    border: none;
    color: var(--color-text-muted, #868e96);
    font-size: 11px;
    font-weight: 400;
    padding: 0 4px;
    text-decoration: underline;
    text-decoration-style: dotted;
    text-underline-offset: 2px;
  }
  confirmation-interlock-button::part(trigger):hover {
    color: var(--color-text-primary, inherit);
    text-decoration-style: solid;
  }
  /* Sync-now affordance — sits between the synced pill and the
     unbind interlock. Visual gap (margin-left auto + extra padding)
     and a separator pip make it noticeably distinct from the
     destructive Unbind so a stray click on the wrong target is
     unlikely. */
  .sync-now-button {
    background: transparent;
    border: 1px solid var(--color-border-default, rgba(0, 0, 0, 0.12));
    color: var(--color-text-secondary, #6c757d);
    font-size: 11px;
    font-weight: 500;
    padding: 3px 8px;
    margin-left: 8px;
    border-radius: 4px;
    cursor: pointer;
    line-height: 1.2;
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .sync-now-button:hover:not(:disabled) {
    color: var(--color-text-primary, inherit);
    border-color: var(--color-border-strong, rgba(0, 0, 0, 0.24));
  }
  .sync-now-button:disabled {
    opacity: 0.5;
    cursor: progress;
  }
  /* Visual separator between Sync Now and Unbind — a small dot plus
     extra horizontal margin discourages accidental Unbind clicks. */
  .action-separator {
    display: inline-block;
    width: 4px;
    height: 4px;
    margin: 0 12px;
    background: var(--color-border-default, rgba(0, 0, 0, 0.16));
    border-radius: 50%;
    vertical-align: middle;
  }
  .picker {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
    font-size: 12px;
  }
  .picker p {
    margin: 0;
  }
  .picker select {
    font-size: 12px;
    padding: 2px 6px;
  }
`;

type Phase =
  | 'loading'
  | 'hidden'
  | 'unsubscribed'
  | 'connector-pick'
  | 'list-pick'
  | 'subscribing'
  | 'subscribed';

// Pretty-printed product names. Centralised so no caller has to remember
// "Google Keep" vs "Google keep" vs "google_keep" — a real risk given the
// proto enum slug shape.
const PRODUCT_NAME: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: '',
  [ConnectorKind.GOOGLE_KEEP]: 'Google Keep',
  [ConnectorKind.GOOGLE_TASKS]: 'Google Tasks',
  [ConnectorKind.ICLOUD_REMINDERS]: 'iCloud Reminders',
};

// Per-connector noun for the remote list. Keep notes are "notes"; Tasks
// lists are "lists". Lets the synced-badge copy read as English.
const REMOTE_LIST_NOUN: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: '',
  [ConnectorKind.GOOGLE_KEEP]: 'note',
  [ConnectorKind.GOOGLE_TASKS]: 'list',
  [ConnectorKind.ICLOUD_REMINDERS]: 'list',
};

// kindToSlug converts a ConnectorKind enum to the wire-level slug used by
// the paused-badge component (which is connector-shape-agnostic).
function kindToSlug(kind: ConnectorKind): 'google_keep' | 'google_tasks' {
  switch (kind) {
    case ConnectorKind.GOOGLE_TASKS:
      return 'google_tasks';
    default:
      // GOOGLE_KEEP and any unexpected value default to keep — the badge
      // listener will route accordingly. Defensive default rather than
      // throwing, since the badge is purely informational.
      return 'google_keep';
  }
}

export class ConnectorBindButton extends LitElement {
  static override styles = [foundationCSS, buttonCSS, inputCSS, pillCSS, localCSS];

  @property({ type: String, attribute: 'page' })
  declare page: string;

  @property({ type: String, attribute: 'list-name' })
  declare listName: string;

  @state() declare private phase: Phase;
  @state() declare private subscriptionState: ChecklistBindingState | null;
  @state() declare private authedKinds: ConnectorKind[];
  @state() declare private chosenKind: ConnectorKind;
  @state() declare private remoteLists: RemoteListSummary[];
  @state() declare private selectedRemoteListHandle: string;
  @state() declare private error: AugmentedError | null;
  @state() declare private syncing: boolean;

  private client = createClient(ConnectorService, getGrpcWebTransport());

  constructor() {
    super();
    this.page = '';
    this.listName = '';
    this.phase = 'loading';
    this.subscriptionState = null;
    this.authedKinds = [];
    this.chosenKind = ConnectorKind.UNSPECIFIED;
    this.remoteLists = [];
    this.selectedRemoteListHandle = '';
    this.error = null;
    this.syncing = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    void this.refresh();
  }

  private async refresh(): Promise<void> {
    if (!this.page || !this.listName) {
      this.phase = 'hidden';
      return;
    }
    try {
      const resp = await this.client.getChecklistBindingState(
        create(GetChecklistBindingStateRequestSchema, {
          page: this.page,
          listName: this.listName,
        }),
      );
      this.subscriptionState = resp.state ?? null;
      if (!this.subscriptionState?.connectorConfigured) {
        this.phase = 'hidden';
      } else if (this.subscriptionState.currentBinding) {
        this.phase = 'subscribed';
      } else {
        this.phase = 'unsubscribed';
      }
    } catch {
      // Fail-closed: this component renders inside every Checklist on every
      // page, so a banner per render would be far worse UX than just
      // hiding the subscribe affordance. Real auth/RPC errors surface on
      // the /profile page where they're actionable.
      this.phase = 'hidden';
    }
  }

  // detectAuthedConnectors queries every supported connector kind in
  // parallel and returns the list that's actually authenticated. Cached on
  // the component instance so repeat picker opens are cheap.
  private async detectAuthedConnectors(): Promise<ConnectorKind[]> {
    if (this.authedKinds.length > 0) return this.authedKinds;
    const candidates: ConnectorKind[] = [
      ConnectorKind.GOOGLE_KEEP,
      ConnectorKind.GOOGLE_TASKS,
    ];
    const results = await Promise.all(
      candidates.map(async (kind) => {
        try {
          const resp = await this.client.getState(
            create(GetStateRequestSchema, { connectorKind: kind }),
          );
          return resp.state?.configured ? kind : null;
        } catch {
          // A failed GetState is treated as "not authed" — the subscribe
          // path bails to no-op rather than surfacing a noisy error here.
          return null;
        }
      }),
    );
    this.authedKinds = results.filter((k): k is ConnectorKind => k !== null);
    return this.authedKinds;
  }

  private async handleBindClick(): Promise<void> {
    this.error = null;
    const authed = await this.detectAuthedConnectors();
    if (authed.length === 0) {
      // No connectors authed — leave the button visible but inert. The
      // user's profile page is where they configure connectors.
      return;
    }
    if (authed.length === 1) {
      // Default-to-authed: skip the connector pick and go straight to the
      // list picker for the single configured connector.
      this.chosenKind = authed[0]!;
      await this.openListPicker(this.chosenKind);
      return;
    }
    this.phase = 'connector-pick';
  }

  private async handleConnectorChoice(kind: ConnectorKind): Promise<void> {
    this.chosenKind = kind;
    await this.openListPicker(kind);
  }

  private async openListPicker(kind: ConnectorKind): Promise<void> {
    this.phase = 'list-pick';
    try {
      const resp = await this.client.listRemoteLists(
        create(ListRemoteListsRequestSchema, { connectorKind: kind }),
      );
      this.remoteLists = resp.lists;
    } catch (err: unknown) {
      this.remoteLists = [];
      this.error = AugmentErrorService.augmentError(
        err,
        `list ${PRODUCT_NAME[kind]} ${REMOTE_LIST_NOUN[kind]}s`,
      );
    }
  }

  private async handleSubscribeConfirm(): Promise<void> {
    this.error = null;
    this.phase = 'subscribing';
    try {
      await this.client.bind(
        create(BindRequestSchema, {
          connectorKind: this.chosenKind,
          page: this.page,
          listName: this.listName,
          remoteListHandle: this.selectedRemoteListHandle, // empty → server creates new list
        }),
      );
      this.selectedRemoteListHandle = '';
      await this.refresh();
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(
        err,
        `subscribe checklist to ${PRODUCT_NAME[this.chosenKind]}`,
      );
      this.phase = 'list-pick';
    }
  }

  // handleSyncNow triggers an immediate sync via the SyncNow RPC.
  // Lightweight: reuses the engine's standard reconcile path, so the
  // per-checklist lease serializes against any concurrent cron /
  // debouncer / adaptive-ticker tick. Disabled while a previous click
  // is still in flight.
  private async handleSyncNow(): Promise<void> {
    const sub = this.subscriptionState?.currentBinding;
    if (!sub || this.syncing) return;
    this.syncing = true;
    try {
      await this.client.syncNow(
        create(SyncNowRequestSchema, {
          connectorKind: sub.connectorKind,
          page: this.page,
          listName: this.listName,
        }),
      );
      // Refresh the binding state so freshness signals
      // (last_pull_at / last_successful_sync_at) re-render.
      await this.refresh();
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(err, 'sync now');
    } finally {
      this.syncing = false;
    }
  }

  // handleUnbind is invoked by <confirmation-interlock-button>'s
  // `confirmed` event — the interlock provides the safety prompt; this
  // handler runs only after the user clicks the "Yes" leg.
  private async handleUnbind(): Promise<void> {
    const sub = this.subscriptionState?.currentBinding;
    if (!sub) return;
    try {
      await this.client.unbind(
        create(UnbindRequestSchema, {
          connectorKind: sub.connectorKind,
          page: this.page,
          listName: this.listName,
        }),
      );
      await this.refresh();
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(err, 'unsubscribe checklist');
    }
  }

  override render() {
    if (this.phase === 'hidden' || this.phase === 'loading') {
      return nothing;
    }
    return html`
      ${sharedStyles}
      <div class="subscribe-row">
        ${this.renderPhase()}
        ${this.error
          ? html`<error-display .augmentedError=${this.error}></error-display>`
          : nothing}
      </div>
    `;
  }

  private renderPhase() {
    switch (this.phase) {
      case 'unsubscribed':
        return this.renderSubscribeTrigger();
      case 'connector-pick':
        return this.renderConnectorPicker();
      case 'list-pick':
        return this.renderListPicker();
      case 'subscribing':
        return html`<span class="sync-badge">Binding…</span>`;
      case 'subscribed':
        return this.renderSubscribed();
      default:
        return nothing;
    }
  }

  private renderSubscribeTrigger() {
    // Without knowing yet which connectors are authed (we haven't asked
    // GetState), give a connector-agnostic label. Once the user clicks,
    // detectAuthedConnectors decides single-pick vs multi-pick.
    return html`
      <button class="bind-trigger" type="button" @click=${this.handleBindClick}>
        Bind to a cloud service
      </button>
    `;
  }

  private renderConnectorPicker() {
    return html`
      <div class="picker">
        <p>Bind <strong>${this.listName}</strong> to:</p>
        ${this.authedKinds.map(
          (kind) => html`
            <button
              class="connector-choice"
              type="button"
              @click=${() => this.handleConnectorChoice(kind)}
            >
              ${PRODUCT_NAME[kind]}
            </button>
          `,
        )}
        <button
          class="bind-trigger"
          type="button"
          @click=${() => (this.phase = 'unsubscribed')}
        >
          Cancel
        </button>
      </div>
    `;
  }

  private renderListPicker() {
    const noun = REMOTE_LIST_NOUN[this.chosenKind];
    const productName = PRODUCT_NAME[this.chosenKind];
    return html`
      <div class="picker">
        <p>Bind <strong>${this.listName}</strong> to ${productName} ${noun}:</p>
        <select
          .value=${this.selectedRemoteListHandle}
          @change=${(e: Event) => {
            if (!(e.target instanceof HTMLSelectElement)) return;
            this.selectedRemoteListHandle = e.target.value;
          }}
        >
          <option value="">Create new "${this.listName}"</option>
          ${this.remoteLists.map(
            (n) => html`
              <option value=${n.remoteListHandle}>${n.title || '(untitled)'}</option>
            `,
          )}
        </select>
        <button
          class="bind-trigger"
          type="button"
          @click=${this.handleSubscribeConfirm}
        >
          Bind
        </button>
        <button
          class="bind-trigger"
          type="button"
          @click=${() => (this.phase = 'unsubscribed')}
        >
          Cancel
        </button>
      </div>
    `;
  }

  private renderSubscribed() {
    const sub = this.subscriptionState?.currentBinding;
    if (!sub) return nothing;
    if (sub.paused) return this.renderPausedBadge(sub);
    return this.renderSyncedBadge(sub);
  }

  private renderSyncedBadge(sub: BindingState) {
    const productName = PRODUCT_NAME[sub.connectorKind];
    const noun = REMOTE_LIST_NOUN[sub.connectorKind];
    const titleSuffix = sub.remoteListTitle ? ` ${sub.remoteListTitle}` : '';
    // Copy: "✓ Bound to Google Keep note <title>". The leading checkmark
    // comes from the .sync-badge::before rule. Verb in the user-facing copy
    // is "Bound" (the binding state); the underlying activity is still
    // "syncing", which is why the unbind confirm reads "Stop syncing?".
    //
    // Layout: synced pill | Sync Now | • | Unbind. The bullet
    // separator + extra horizontal margin keep Sync Now visually
    // distinct from the destructive Unbind so a stray click on the
    // wrong target is unlikely.
    return html`
      <span class="sync-badge">Bound to ${productName} ${noun}${titleSuffix}</span>
      <button
        class="sync-now-button"
        type="button"
        ?disabled=${this.syncing}
        title="Trigger an immediate sync (pulls remote, pushes pending wiki edits)"
        @click=${this.handleSyncNow}
      >
        ${this.syncing ? '⟳ Syncing…' : '⟳ Sync now'}
      </button>
      <span class="action-separator" aria-hidden="true"></span>
      <confirmation-interlock-button
        label="Unbind"
        confirmLabel="Stop syncing?"
        yesLabel="Unbind"
        noLabel="Cancel"
        class="bind-trigger"
        @confirmed=${this.handleUnbind}
      ></confirmation-interlock-button>
    `;
  }

  private renderPausedBadge(sub: BindingState) {
    // Pause start: best signal we have today is last_pull_at — the most
    // recent successful inbound poll. (last_verified_at is also a
    // candidate, but pull is the user-facing "last time we heard from
    // the list".) Fall back to subscribed_at if neither is set.
    const pausedAt = timestampToDate(
      sub.lastPullAt ?? sub.lastVerifiedAt ?? sub.boundAt,
    );
    return html`
      <connector-paused-badge
        .connectorKind=${kindToSlug(sub.connectorKind)}
        .pausedAt=${pausedAt}
        .subscriptionTitle=${sub.remoteListTitle}
      ></connector-paused-badge>
    `;
  }
}

// timestampToDate converts a protobuf Timestamp wrapper into a JS Date.
// Returns the unix epoch when the input is undefined — we render the
// resulting paused-badge immediately, so a stand-in Date is preferable to
// throwing here.
interface ProtoTimestamp {
  seconds: bigint;
  nanos: number;
}

function timestampToDate(ts: ProtoTimestamp | undefined): Date {
  if (!ts) return new Date(0);
  const ms = Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1_000_000);
  return new Date(ms);
}

declare global {
  interface HTMLElementTagNameMap {
    'connector-bind-button': ConnectorBindButton;
  }
}

customElements.define('connector-bind-button', ConnectorBindButton);
