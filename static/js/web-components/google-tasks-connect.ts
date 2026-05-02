// google-tasks-connect — profile-page Lit component for the Google
// Tasks connector. Mirrors the shape of <keep-connect> but uses the
// OAuth web-server flow instead of a captured cookie.
//
// Phases:
//   • loading       — initial GetState in flight
//   • unconfigured  — operator hasn't set OAuth client env vars; the
//                     server signals this by returning FailedPrecondition
//                     from BeginAuth(GOOGLE_TASKS). Render disabled with
//                     a helper pointer to /help_google_tasks.
//   • disconnected  — env vars set, user has no refresh token. Show a
//                     "Connect Google Tasks" button. Click → BeginAuth
//                     → window.location.href = response.authorizationUrl.
//   • connecting    — BeginAuth in flight or post-success redirect.
//   • connected     — user has a refresh token. Show "Connected as
//                     <email>" + a Disconnect interlock that calls
//                     ConnectorService.Disconnect(GOOGLE_TASKS).
//
// Why we detect "unconfigured" lazily (on connect-click) rather than
// eagerly: the only way the server signals "operator hasn't wired OAuth"
// is through the FailedPrecondition return from BeginAuth. GetState
// returns the same configured=false shape for both "not set up" and
// "set up but disconnected", so we can't distinguish the two from
// GetState alone. Triggering BeginAuth eagerly on connectedCallback
// would bombard the server with unwanted RPCs on every profile-page
// load. So we wait until the user clicks Connect, then surface the
// distinction by catching FailedPrecondition.

import { html, LitElement, nothing } from 'lit';
import { state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { ConnectError, Code } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  ConnectorService,
  ConnectorKind,
  GetStateRequestSchema,
  BeginAuthRequestSchema,
  DisconnectRequestSchema,
} from '../gen/api/v1/connector_service_pb.js';
import type { ConnectorState } from '../gen/api/v1/connector_service_pb.js';
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

type Phase = 'loading' | 'unconfigured' | 'disconnected' | 'connecting' | 'connected';

export class GoogleTasksConnect extends LitElement {
  static override styles = [foundationCSS, buttonCSS, inputCSS, pillCSS];

  @state() declare private phase: Phase;
  @state() declare private state: ConnectorState | null;
  @state() declare private error: AugmentedError | null;

  private client = createClient(ConnectorService, getGrpcWebTransport());

  // redirect is the navigation seam used by handleConnect to send the
  // browser to Google's authorization endpoint. Real builds use
  // window.location.href; tests stub this so they don't actually
  // navigate the test runner.
  redirect: (url: string) => void = (url: string) => {
    window.location.href = url;
  };

  constructor() {
    super();
    this.phase = 'loading';
    this.state = null;
    this.error = null;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    void this.refresh();
  }

  private async refresh(): Promise<void> {
    this.error = null;
    try {
      const resp = await this.client.getState(
        create(GetStateRequestSchema, {
          connectorKind: ConnectorKind.GOOGLE_TASKS,
        }),
      );
      this.state = resp.state ?? null;
      this.phase = this.state?.configured ? 'connected' : 'disconnected';
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(err, 'load Google Tasks setup');
      this.phase = 'disconnected';
    }
  }

  private async handleConnect(): Promise<void> {
    this.error = null;
    this.phase = 'connecting';
    try {
      const resp = await this.client.beginAuth(
        create(BeginAuthRequestSchema, {
          connectorKind: ConnectorKind.GOOGLE_TASKS,
        }),
      );
      // The server returns the URL the browser must visit to complete
      // the OAuth consent. A full-page navigation is intentional —
      // Google's consent screen will redirect back to /oauth/callback
      // with the code+state, where the server completes the handshake.
      this.redirect(resp.authorizationUrl);
    } catch (err: unknown) {
      // FailedPrecondition is the server's signal that the operator
      // hasn't wired OAuth client credentials. Surface this as a
      // distinct UI state so the user knows it's not their problem to
      // fix. All other errors render as a generic error-display so the
      // user can retry.
      if (err instanceof ConnectError && err.code === Code.FailedPrecondition) {
        this.phase = 'unconfigured';
        return;
      }
      this.phase = 'disconnected';
      this.error = AugmentErrorService.augmentError(err, 'begin Google Tasks auth');
    }
  }

  // handleDisconnect runs only after <confirmation-interlock-button>
  // fires its `confirmed` event. The interlock provides the
  // anti-misclick prompt; this handler just performs the RPC.
  private async handleDisconnect(): Promise<void> {
    try {
      const resp = await this.client.disconnect(
        create(DisconnectRequestSchema, {
          connectorKind: ConnectorKind.GOOGLE_TASKS,
        }),
      );
      this.state = resp.state ?? null;
      this.phase = 'disconnected';
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(err, 'disconnect Google Tasks');
    }
  }

  override render() {
    return html`
      ${sharedStyles}
      <section class="google-tasks-connect">
        <h3>Google Tasks</h3>
        ${this.renderPhase()}
        ${this.error
          ? html`<error-display .augmentedError=${this.error}></error-display>`
          : nothing}
      </section>
    `;
  }

  private renderPhase() {
    switch (this.phase) {
      case 'loading':
        return html`<p class="muted">Checking Google Tasks setup…</p>`;
      case 'unconfigured':
        return this.renderUnconfigured();
      case 'disconnected':
        return this.renderDisconnected();
      case 'connecting':
        return html`<p class="muted">Redirecting to Google…</p>`;
      case 'connected':
        return this.renderConnected();
      default:
        return nothing;
    }
  }

  private renderUnconfigured() {
    return html`
      <p>
        Google Tasks integration is not configured by this wiki's operator.
      </p>
      <p>
        See <a href="/help_google_tasks/view">help_google_tasks</a> for
        operator setup instructions.
      </p>
      <button type="button" disabled>Connect Google Tasks</button>
    `;
  }

  private renderDisconnected() {
    return html`
      <p>
        Connect your Google account to sync wiki checklists with a Google
        Tasks list on your phone.
      </p>
      <p>
        Clicking <strong>Connect</strong> will redirect you to Google to
        authorize this wiki. After you approve, Google will send you back
        here with a code, and the wiki will exchange it for a refresh
        token. Only the refresh token is stored.
      </p>
      <p>
        See <a href="/help_google_tasks/view">help</a> for the full
        walkthrough.
      </p>
      <button type="button" @click=${this.handleConnect}>
        Connect Google Tasks
      </button>
    `;
  }

  private renderConnected() {
    if (!this.state) return nothing;
    const verified = this.state.lastVerifiedAt
      ? new Date(Number(this.state.lastVerifiedAt.seconds) * 1000).toLocaleString()
      : 'never';
    return html`
      <p>
        Connected as <strong>${this.state.email}</strong>.
        <span class="muted">Last verified: ${verified}.</span>
      </p>
      <p>
        <confirmation-interlock-button
          label="Disconnect Google Tasks"
          confirmLabel="Disconnect — subscriptions will be paused"
          yesLabel="Disconnect"
          noLabel="Cancel"
          @confirmed=${this.handleDisconnect}
        ></confirmation-interlock-button>
      </p>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'google-tasks-connect': GoogleTasksConnect;
  }
}

customElements.define('google-tasks-connect', GoogleTasksConnect);
