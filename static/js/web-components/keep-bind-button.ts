// keep-bind-button — embedded inside <wiki-checklist> to expose the
// Bind/Unbind controls when the calling user has KeepConnect configured.
//
// Renders nothing when the user has no connector. Renders a "Bind to Keep
// List" button when configured but unbound. Renders a status pill +
// Unbind affordance when bound.

import { css, html, LitElement, nothing } from 'lit';
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
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import './error-display.js';
import './confirmation-interlock-button.js';

// Local layout overrides — keep the bind affordance & sync badge
// visually subordinate to the checklist itself. Tokens used:
//   --color-text-muted, --color-text-secondary, --color-action-secondary-*
// (defined in shared-styles foundationCSS).
const localCSS = css`
  :host {
    display: block;
    margin-top: 8px;
  }
  .keep-bind {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  /* Bind-mode trigger: ghost-style, matches the muted text scale of
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
  /* Bound-mode badge: muted info text + small unbind affordance,
     visually similar weight to a "last edited" caption. */
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

type Phase = 'loading' | 'hidden' | 'unbound' | 'picker' | 'binding' | 'bound';

export class KeepBindButton extends LitElement {
  static override styles = [foundationCSS, buttonCSS, inputCSS, pillCSS, localCSS];

  @property({ type: String, attribute: 'page' })
  declare page: string;

  @property({ type: String, attribute: 'list-name' })
  declare listName: string;

  @state() declare private phase: Phase;
  @state() declare private bindingState: ChecklistBindingState | null;
  @state() declare private notes: KeepNoteSummary[];
  @state() declare private selectedNoteID: string;
  @state() declare private error: AugmentedError | null;

  private client = createClient(KeepConnectorService, getGrpcWebTransport());

  constructor() {
    super();
    this.page = '';
    this.listName = '';
    this.phase = 'loading';
    this.bindingState = null;
    this.notes = [];
    this.selectedNoteID = '';
    this.error = null;
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
      // Fail-closed: this component renders inside every Checklist on every
      // page, so a banner per render would be far worse UX than just
      // hiding the bind affordance. Real auth/RPC errors surface on the
      // /profile page where they're actionable.
      this.phase = 'hidden';
    }
  }

  private async openPicker(): Promise<void> {
    this.error = null;
    this.phase = 'picker';
    try {
      const resp = await this.client.listNotes(create(ListNotesRequestSchema, {}));
      this.notes = resp.notes;
    } catch (err: unknown) {
      this.notes = [];
      this.error = AugmentErrorService.augmentError(err, 'list Keep notes');
    }
  }

  private async handleBind(): Promise<void> {
    this.error = null;
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
      this.error = AugmentErrorService.augmentError(err, 'bind checklist to Keep');
      this.phase = 'picker';
    }
  }

  // handleUnbind is invoked by the <confirmation-interlock-button>'s
  // `confirmed` event — the interlock provides the safety prompt; this
  // handler runs only after the user clicks the "Yes" leg.
  private async handleUnbind(): Promise<void> {
    try {
      await this.client.unbindChecklist(
        create(UnbindChecklistRequestSchema, {
          page: this.page,
          listName: this.listName,
        }),
      );
      await this.refresh();
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(err, 'unbind checklist');
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
        ${this.error
          ? html`<error-display .augmentedError=${this.error}></error-display>`
          : nothing}
      </div>
    `;
  }

  private renderPhase() {
    switch (this.phase) {
      case 'unbound':
        return html`
          <button class="bind-trigger" type="button" @click=${this.openPicker}>
            Bind to Keep
          </button>
        `;
      case 'picker':
        return html`
          <div class="picker">
            <p>Bind <strong>${this.listName}</strong> →</p>
            <select
              .value=${this.selectedNoteID}
              @change=${(e: Event) => {
                if (!(e.target instanceof HTMLSelectElement)) return;
                this.selectedNoteID = e.target.value;
              }}
            >
              <option value="">Create new "${this.listName}"</option>
              ${this.notes.map(
                (n) => html`
                  <option value=${n.keepNoteId}>${n.title || '(untitled)'}</option>
                `,
              )}
            </select>
            <button class="bind-trigger" type="button" @click=${this.handleBind}>Bind</button>
            <button
              class="bind-trigger"
              type="button"
              @click=${() => (this.phase = 'unbound')}
            >
              Cancel
            </button>
          </div>
        `;
      case 'binding':
        return html`<span class="sync-badge">Binding…</span>`;
      case 'bound':
        return html`
          <span class="sync-badge"
            >Synced${this.bindingState?.currentBinding?.keepNoteTitle
              ? html` · ${this.bindingState.currentBinding.keepNoteTitle}`
              : nothing}</span
          >
          <confirmation-interlock-button
            label="Unbind"
            confirmLabel="Stop syncing?"
            yesLabel="Unbind"
            noLabel="Cancel"
            class="bind-trigger"
            @confirmed=${this.handleUnbind}
          ></confirmation-interlock-button>
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
