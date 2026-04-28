// keep-bind-button — embedded inside <wiki-checklist> to expose the
// Bind/Unbind controls when the calling user has KeepConnect configured.
//
// Renders nothing when the user has no connector. Renders a "Bind to Keep
// List" button when configured but unbound. Renders a status pill +
// Unbind affordance when bound.

import { html, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  KeepConnectorService,
  GetChecklistBindingStateRequestSchema,
  ListNotesRequestSchema,
  BindChecklistRequestSchema,
  UnbindChecklistRequestSchema,
} from '../gen/api/v1/keep_connector_pb.js';
import type {
  ChecklistBindingState,
  KeepNoteSummary,
} from '../gen/api/v1/keep_connector_pb.js';
import {
  foundationCSS,
  buttonCSS,
  inputCSS,
  pillCSS,
  sharedStyles,
} from './shared-styles.js';

type Phase =
  | 'loading'
  | 'hidden'
  | 'unbound'
  | 'picker'
  | 'binding'
  | 'bound'
  | 'error';

export class KeepBindButton extends LitElement {
  static override styles = [foundationCSS, buttonCSS, inputCSS, pillCSS];

  @property({ type: String, attribute: 'page' })
  declare page: string;

  @property({ type: String, attribute: 'list-name' })
  declare listName: string;

  @state() declare private phase: Phase;
  @state() declare private bindingState: ChecklistBindingState | null;
  @state() declare private notes: KeepNoteSummary[];
  @state() declare private selectedNoteID: string;
  @state() declare private errorMessage: string;

  private client = createClient(KeepConnectorService, getGrpcWebTransport());

  constructor() {
    super();
    this.page = '';
    this.listName = '';
    this.phase = 'loading';
    this.bindingState = null;
    this.notes = [];
    this.selectedNoteID = '';
    this.errorMessage = '';
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
      this.bindingState = resp.state ?? null;
      if (!this.bindingState?.connectorConfigured) {
        this.phase = 'hidden';
      } else if (this.bindingState.currentBinding) {
        this.phase = 'bound';
      } else {
        this.phase = 'unbound';
      }
    } catch {
      this.phase = 'hidden'; // fail closed: don't show broken UI
    }
  }

  private async openPicker(): Promise<void> {
    this.errorMessage = '';
    this.phase = 'picker';
    try {
      const resp = await this.client.listNotes(create(ListNotesRequestSchema, {}));
      this.notes = resp.notes;
    } catch (err: unknown) {
      this.notes = [];
      this.errorMessage = err instanceof Error ? err.message : String(err);
    }
  }

  private async handleBind(): Promise<void> {
    this.errorMessage = '';
    this.phase = 'binding';
    try {
      await this.client.bindChecklist(
        create(BindChecklistRequestSchema, {
          page: this.page,
          listName: this.listName,
          keepNoteId: this.selectedNoteID, // empty → server creates new note
        }),
      );
      this.selectedNoteID = '';
      await this.refresh();
    } catch (err: unknown) {
      this.errorMessage = err instanceof Error ? err.message : String(err);
      this.phase = 'picker';
    }
  }

  private async handleUnbind(): Promise<void> {
    if (!confirm('Stop syncing this checklist with Google Keep? Both sides keep their data as-is.')) {
      return;
    }
    try {
      await this.client.unbindChecklist(
        create(UnbindChecklistRequestSchema, {
          page: this.page,
          listName: this.listName,
        }),
      );
      await this.refresh();
    } catch (err: unknown) {
      this.errorMessage = err instanceof Error ? err.message : String(err);
    }
  }

  override render() {
    if (this.phase === 'hidden' || this.phase === 'loading') {
      return nothing;
    }
    return html`
      ${sharedStyles}
      <div class="keep-bind">
        ${this.renderPhase()}
        ${this.errorMessage
          ? html`<div class="error-banner">${this.errorMessage}</div>`
          : nothing}
      </div>
    `;
  }

  private renderPhase() {
    switch (this.phase) {
      case 'unbound':
        return html`
          <button type="button" @click=${this.openPicker}>
            🔗 Bind to Keep List
          </button>
        `;
      case 'picker':
        return html`
          <div class="picker">
            <p>Pick a Google Keep note to bind <strong>${this.listName}</strong> to:</p>
            <select
              .value=${this.selectedNoteID}
              @change=${(e: Event) => {
                if (!(e.target instanceof HTMLSelectElement)) return;
                this.selectedNoteID = e.target.value;
              }}
            >
              <option value="">Create new Keep note named "${this.listName}"</option>
              ${this.notes.map(
                (n) => html`
                  <option value=${n.keepNoteId}>${n.title || '(untitled)'}</option>
                `,
              )}
            </select>
            <button type="button" @click=${this.handleBind}>Bind</button>
            <button
              type="button"
              class="secondary"
              @click=${() => (this.phase = 'unbound')}
            >
              Cancel
            </button>
          </div>
        `;
      case 'binding':
        return html`<p class="muted">Binding…</p>`;
      case 'bound':
        return html`
          <span class="pill pill-ok">
            Synced with Keep
            ${this.bindingState?.currentBinding?.keepNoteTitle
              ? html` ("${this.bindingState.currentBinding.keepNoteTitle}")`
              : nothing}
          </span>
          <button type="button" class="secondary" @click=${this.handleUnbind}>
            Unbind
          </button>
        `;
      default:
        return nothing;
    }
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'keep-bind-button': KeepBindButton;
  }
}

customElements.define('keep-bind-button', KeepBindButton);
