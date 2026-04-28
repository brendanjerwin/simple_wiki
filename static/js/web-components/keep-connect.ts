// keep-connect — profile-page Lit component for Google Keep connector.
//
// Renders the "Connect Google Keep" form, the connected state, and the
// per-binding remove controls. The connect form takes the user's Google
// email + an oauth_token cookie value captured from a Google sign-in
// (see help_google_keep for capture instructions). The wiki exchanges
// the oauth_token for a long-lived master token via gpsoauth and stores
// only the master token. The captured oauth_token is consumed once and
// discarded.
//
// Why oauth_token (not an App-Specific Password): Google deprecated
// password-based gpsoauth master-login. ASPs reliably get rejected
// regardless of correctness. The browser-captured oauth_token is the
// only credential type that still works for the unofficial Keep API.

import { html, LitElement, nothing } from 'lit';
import { state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  KeepConnectorService,
  GetStateRequestSchema,
  ExchangeAndStoreRequestSchema,
  DisconnectRequestSchema,
  UnbindChecklistRequestSchema,
} from '../gen/api/v1/keep_connector_pb.js';
import type {
  ConnectorState,
  BindingState,
} from '../gen/api/v1/keep_connector_pb.js';
import {
  foundationCSS,
  buttonCSS,
  inputCSS,
  pillCSS,
  sharedStyles,
} from './shared-styles.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import './error-display.js';

type Phase = 'loading' | 'disconnected' | 'connecting' | 'connected';

export class KeepConnect extends LitElement {
  static override styles = [foundationCSS, buttonCSS, inputCSS, pillCSS];

  @state() declare private phase: Phase;
  @state() declare private state: ConnectorState | null;
  @state() declare private formEmail: string;
  @state() declare private formOAuthToken: string;
  @state() declare private error: AugmentedError | null;

  private client = createClient(KeepConnectorService, getGrpcWebTransport());

  constructor() {
    super();
    this.phase = 'loading';
    this.state = null;
    this.formEmail = '';
    this.formOAuthToken = '';
    this.error = null;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    void this.refresh();
  }

  private async refresh(): Promise<void> {
    this.error = null;
    try {
      const resp = await this.client.getState(create(GetStateRequestSchema, {}));
      this.state = resp.state ?? null;
      this.phase = this.state?.configured ? 'connected' : 'disconnected';
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(err, 'load Google Keep connector state');
      this.phase = 'disconnected';
    }
  }

  private async handleConnect(e: SubmitEvent): Promise<void> {
    e.preventDefault();
    this.error = null;
    if (!this.formEmail || !this.formOAuthToken) {
      this.error = AugmentErrorService.augmentError(
        new Error('Google email and oauth_token are both required.'),
        'connect Google Keep',
      );
      return;
    }
    this.phase = 'connecting';
    try {
      const resp = await this.client.exchangeAndStore(
        create(ExchangeAndStoreRequestSchema, {
          email: this.formEmail,
          oauthToken: this.formOAuthToken,
        }),
      );
      this.state = resp.state ?? null;
      this.phase = this.state?.configured ? 'connected' : 'disconnected';
      this.formOAuthToken = ''; // never retain the captured credential
    } catch (err: unknown) {
      this.phase = 'disconnected';
      this.formOAuthToken = '';
      this.error = AugmentErrorService.augmentError(err, 'connect Google Keep');
    }
  }

  private async handleDisconnect(): Promise<void> {
    if (!confirm('Disconnect Google Keep? Your bindings will be paused but kept; reconnect resumes them.')) {
      return;
    }
    try {
      const resp = await this.client.disconnect(create(DisconnectRequestSchema, {}));
      this.state = resp.state ?? null;
      this.phase = 'disconnected';
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(err, 'disconnect Google Keep');
    }
  }

  private async handleUnbind(binding: BindingState): Promise<void> {
    if (!confirm(`Stop syncing ${binding.page} / ${binding.listName}? Wiki and Keep data are both left as-is.`)) {
      return;
    }
    try {
      await this.client.unbindChecklist(
        create(UnbindChecklistRequestSchema, {
          page: binding.page,
          listName: binding.listName,
        }),
      );
      await this.refresh();
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(err, 'unbind checklist');
    }
  }

  override render() {
    return html`
      ${sharedStyles}
      <section class="keep-connect">
        <h3>Google Keep</h3>
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
        return html`<p class="muted">Loading connector state…</p>`;
      case 'disconnected':
        return this.renderDisconnected();
      case 'connecting':
        return html`<p class="muted">Verifying credentials…</p>`;
      case 'connected':
        return this.renderConnected();
      default:
        return nothing;
    }
  }

  private renderDisconnected() {
    return html`
      <p>
        Connect your Google account to sync wiki checklists with Google Keep
        notes on your phone.
      </p>
      <ol>
        <li>
          Open
          <a
            href="https://accounts.google.com/EmbeddedSetup"
            target="_blank"
            rel="noopener noreferrer"
            >accounts.google.com/EmbeddedSetup</a
          >
          and sign in.
        </li>
        <li>Open DevTools → Application → Cookies → <code>accounts.google.com</code>.</li>
        <li>
          Copy the value of the <code>oauth_token</code> cookie. It's HttpOnly,
          so it only appears in the Application panel — not the Console.
        </li>
        <li>Paste it below with your Google email.</li>
      </ol>
      <p>
        See <a href="/help_google_keep/view">help</a> for the full walkthrough
        and the trust-model warning.
      </p>
      <form @submit=${this.handleConnect}>
        <label>
          Google email
          <input
            type="email"
            required
            .value=${this.formEmail}
            @input=${(e: Event) => {
              if (!(e.target instanceof HTMLInputElement)) return;
              this.formEmail = e.target.value;
            }}
          />
        </label>
        <label>
          oauth_token cookie value
          <input
            type="password"
            required
            autocomplete="off"
            placeholder="oauth2_4/0Ad…"
            .value=${this.formOAuthToken}
            @input=${(e: Event) => {
              if (!(e.target instanceof HTMLInputElement)) return;
              this.formOAuthToken = e.target.value;
            }}
          />
        </label>
        <button type="submit">Test &amp; Save</button>
      </form>
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
      <h4>Bindings</h4>
      ${this.renderBindings()}
      <p>
        <button type="button" class="secondary" @click=${this.handleDisconnect}>
          Disconnect Google Keep
        </button>
      </p>
    `;
  }

  private renderBindings() {
    const bindings = this.state?.bindings ?? [];
    if (bindings.length === 0) {
      return html`<p class="muted">
        No checklists bound yet. Open a checklist page and click
        <em>Bind to Keep List</em>.
      </p>`;
    }
    return html`
      <ul class="bindings">
        ${bindings.map(
          (b) => html`
            <li>
              <strong>${b.page} / ${b.listName}</strong>
              → Keep note "${b.keepNoteTitle || b.keepNoteId}"
              ${b.paused ? html`<span class="pill pill-warn">paused</span>` : nothing}
              <button type="button" @click=${() => this.handleUnbind(b)}>✕</button>
            </li>
          `,
        )}
      </ul>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'keep-connect': KeepConnect;
  }
}

customElements.define('keep-connect', KeepConnect);
