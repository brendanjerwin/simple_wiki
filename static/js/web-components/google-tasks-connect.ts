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

import { css, html, LitElement, nothing } from 'lit';
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
  UnbindRequestSchema,
} from '../gen/api/v1/connector_service_pb.js';
import type {
  ConnectorState,
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

type Phase = 'loading' | 'unconfigured' | 'disconnected' | 'connecting' | 'connected';

// Local styles for the in-card reconnect banner. Mirrors the
// <profile-paused-banner> visual language so the two surfaces feel
// like one experience: same icon, same warning palette, same button
// shape. The banner here is what the page-level banner's Reconnect
// button scrolls the user *to* — without an actionable CTA inside
// the card the user lands somewhere with no clear next step (just a
// "paused" pill next to a binding row), which is the gap this fixes.
//
// Theme tokens used here are defined in static/css/default.css for both
// :root (light) and the prefers-color-scheme: dark media query:
//   --color-warning      → amber accent  (light: #ffc107, dark: #d29922)
//   --color-warning-bg   → tinted surface (light: #fff3cd, dark: #2d2000)
//   --color-warning-text → readable text  (light: #856404, dark: #ffda6a)
// The button keeps a hardcoded #1e1e1e for foreground because amber-
// on-amber text is unreadable in either mode and there's no token for
// "text on warning accent."
const reconnectBannerCSS = css`
  .reconnect-banner {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 12px 16px;
    margin: 12px 0;
    border-radius: 6px;
    background: var(--color-warning-bg);
    border: 1px solid var(--color-warning);
    color: var(--color-warning-text);
    font-size: 14px;
    line-height: 1.45;
  }
  .reconnect-banner .banner-icon {
    font-size: 18px;
    line-height: 1.4;
    flex-shrink: 0;
  }
  .reconnect-banner .banner-copy {
    flex: 1;
  }
  .reconnect-banner .banner-copy strong {
    display: block;
    margin-bottom: 4px;
  }
  .reconnect-banner button.reconnect-btn {
    font-family: inherit;
    font-size: 13px;
    font-weight: 600;
    padding: 8px 16px;
    border-radius: 4px;
    background: var(--color-warning);
    color: #1e1e1e;
    border: 1px solid var(--color-warning);
    cursor: pointer;
    flex-shrink: 0;
    align-self: center;
  }
  .reconnect-banner button.reconnect-btn:hover {
    filter: brightness(0.9);
  }
`;

export class GoogleTasksConnect extends LitElement {
  static override styles = [foundationCSS, buttonCSS, inputCSS, pillCSS, reconnectBannerCSS];

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

  // handleUnbind removes a single subscription. Mirrors the
  // per-subscription remove control on <keep-connect>; the binding row
  // is rendered with a <confirmation-interlock-button> so this only
  // runs after the user confirms.
  private async handleUnbind(subscription: BindingState): Promise<void> {
    try {
      await this.client.unbind(
        create(UnbindRequestSchema, {
          connectorKind: ConnectorKind.GOOGLE_TASKS,
          page: subscription.page,
          listName: subscription.listName,
        }),
      );
      await this.refresh();
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(err, 'unsubscribe checklist');
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
    // Invariant: never render "Connected as ." with an empty email.
    // The OAuth callback now refuses to persist an empty email
    // (see emailFromIDToken in oauth_google_handler.go), but
    // pre-existing connections from before that fix can still be on
    // disk with state.email == "". Surface the gap as actionable
    // rather than as a half-rendered sentence.
    const identityLine = this.state.email
      ? html`Connected as <strong>${this.state.email}</strong>.
          <span class="muted">Last verified: ${verified}.</span>`
      : html`<span class="muted">
          Connected, but the account email is missing on this profile.
          Disconnect and reconnect to refresh — recent OAuth scope changes
          add the email claim. Last verified: ${verified}.
        </span>`;
    return html`
      <p>${identityLine}</p>
      ${this.renderReconnectBannerIfPaused()}
      <h4>Bindings</h4>
      ${this.renderBindings()}
      <p>
        <confirmation-interlock-button
          label="Disconnect Google Tasks"
          confirmLabel="Disconnect — bindings will be paused"
          yesLabel="Disconnect"
          noLabel="Cancel"
          @confirmed=${this.handleDisconnect}
        ></confirmation-interlock-button>
      </p>
    `;
  }

  // hasPausedSubscriptions reports whether any subscription on this
  // connector is currently paused. The connector-level OAuth credential
  // is shared across all subscriptions (one refresh_token per profile),
  // so a single auth failure pauses *every* subscription on that profile
  // simultaneously. We surface that as a single connector-level banner
  // rather than a per-row mention.
  private get hasPausedSubscriptions(): boolean {
    return this.state?.bindings?.some((s) => s.paused) ?? false;
  }

  // renderReconnectBannerIfPaused shows a prominent CTA + plain-language
  // "what happened and what to do" explanation when at least one
  // subscription on this connector is paused. The page-level
  // <profile-paused-banner> at the top of the profile page scrolls the
  // user *to* this component on click; without an actionable CTA in
  // the card itself, the user lands here with only a tiny "paused" pill
  // next to a binding row and no obvious next step. The Reconnect
  // button reuses handleConnect() — same OAuth re-authorization that
  // the from-scratch Connect flow uses; existing bindings auto-resume
  // on success.
  private renderReconnectBannerIfPaused() {
    if (!this.hasPausedSubscriptions) return nothing;
    return html`
      <div class="reconnect-banner" role="status">
        <span class="banner-icon" aria-hidden="true">⚠️</span>
        <div class="banner-copy">
          <strong>Sync needs reconnection.</strong>
          The wiki can't refresh its Google Tasks access right now. The most
          common causes are: the authorization was revoked at
          <em>myaccount.google.com</em>, the refresh token expired (Google's
          7-day inactivity window for unverified clients), or the OAuth
          client configuration changed on the wiki side. Click
          <em>Reconnect</em> to re-authorize — your existing bindings will
          resume automatically.
        </div>
        <button
          type="button"
          class="reconnect-btn"
          @click=${this.handleConnect}
        >
          Reconnect Google Tasks
        </button>
      </div>
    `;
  }

  // Render the per-binding rows. Mirror of <keep-connect>'s
  // renderBindings, but uses the Tasks-specific noun ("Tasks list")
  // so the row reads "Bound to Google Tasks list <title>" — matching
  // the vocabulary established by <connector-bind-button>.
  private renderBindings() {
    const subscriptions = this.state?.bindings ?? [];
    if (subscriptions.length === 0) {
      return html`<p class="muted">
        No checklists bound yet. Open a checklist page and click
        <em>Bind to Google Tasks</em>.
      </p>`;
    }
    return html`
      <ul class="bindings">
        ${subscriptions.map(
          (s) => html`
            <li>
              <strong>${s.page} / ${s.listName}</strong>
              → Bound to Google Tasks list "${s.remoteListTitle || s.remoteListHandle}"
              ${s.paused ? html`<span class="pill pill-warn">paused</span>` : nothing}
              <confirmation-interlock-button
                label="✕"
                confirmLabel="Stop syncing this binding?"
                yesLabel="Unbind"
                noLabel="Cancel"
                @confirmed=${() => this.handleUnbind(s)}
              ></confirmation-interlock-button>
            </li>
          `,
        )}
      </ul>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'google-tasks-connect': GoogleTasksConnect;
  }
}

customElements.define('google-tasks-connect', GoogleTasksConnect);
