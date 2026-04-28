// keep-connect — profile-page Lit component for Google Keep connector.
//
// On render: queries KeepConnectorService.GetState; displays disconnected
// or connected UI accordingly. The form takes email + ASP and never
// retains the ASP after submission. Bindings list is rendered with a
// per-row Unbind affordance.
//
// See plan Phase A and help_google_keep for the trust model.

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
  static override styles = [
    foundationCSS,
    buttonCSS,
    inputCSS,
    pillCSS,
  ];

  @state() declare private phase: Phase;
  @state() declare private state: ConnectorState | null;
  @state() declare private formEmail: string;
  @state() declare private formAsp: string;
  @state() declare private error: AugmentedError | null;

  private client = createClient(KeepConnectorService, getGrpcWebTransport());

  constructor() {
    super();
    this.phase = 'loading';
    this.state = null;
    this.formEmail = '';
    this.formAsp = '';
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
      // Stay on the disconnect form so the user can act; the
      // <error-display> renders the augmented error inline.
      this.phase = 'disconnected';
    }
  }

  private async handleConnect(e: SubmitEvent): Promise<void> {
    e.preventDefault();
    this.error = null;
    if (!this.formEmail || !this.formAsp) {
      this.error = AugmentErrorService.augmentError(
        new Error('Email and App-Specific Password are required.'),
        'connect Google Keep',
      );
      return;
    }
    this.phase = 'connecting';
    try {
      const resp = await this.client.exchangeAndStore(
        create(ExchangeAndStoreRequestSchema, {
          email: this.formEmail,
          appSpecificPassword: this.formAsp,
        }),
      );
      this.state = resp.state ?? null;
      this.phase = this.state?.configured ? 'connected' : 'disconnected';
      this.formAsp = ''; // never retain the ASP
    } catch (err: unknown) {
      this.phase = 'disconnected';
      this.formAsp = '';
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
        notes on your phone. See <a href="/help_google_keep/view">help</a> for
        setup steps. You will need an App-Specific Password (Google Account →
        Security → 2-Step Verification → App passwords).
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
          App-Specific Password
          <input
            type="password"
            required
            autocomplete="off"
            placeholder="xxxx xxxx xxxx xxxx"
            .value=${this.formAsp}
            @input=${(e: Event) => {
              if (!(e.target instanceof HTMLInputElement)) return;
              this.formAsp = e.target.value;
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
              ${b.paused ? html` <span class="pill pill-warn">paused</span>` : nothing}
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
