import { css, html, LitElement, nothing } from 'lit';
import { state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import type { Timestamp } from '@bufbuild/protobuf/wkt';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  PageManagementService,
  ListTrashRequestSchema,
  RestorePageRequestSchema,
  PurgePageRequestSchema,
  EmptyTrashRequestSchema,
  type TrashPageEntry,
} from '../gen/api/v1/page_management_pb.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import { showToast } from './toast-message.js';
import { foundationCSS, buttonCSS, sharedStyles } from './shared-styles.js';
import './error-display.js';

function timestampToDate(timestamp: Timestamp | undefined): Date | null {
  if (!timestamp) return null;
  const milliseconds = Number(timestamp.seconds) * 1000 + Math.floor(timestamp.nanos / 1_000_000);
  if (!Number.isFinite(milliseconds)) return null;
  return new Date(milliseconds);
}

function formatTimestamp(timestamp: Timestamp | undefined): string {
  const date = timestampToDate(timestamp);
  if (!date) return '';
  return date.toLocaleString();
}

export class PageTrash extends LitElement {
  static override styles = [
    foundationCSS,
    buttonCSS,
    css`
      :host {
        display: block;
      }

      .trash-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 16px;
        margin-bottom: 16px;
      }

      h1 {
        margin: 0;
        font-size: 1.75rem;
      }

      .empty {
        padding: 24px 0;
        color: var(--color-text-muted);
      }

      .table-wrap {
        overflow-x: auto;
      }

      table {
        width: 100%;
        border-collapse: collapse;
      }

      th,
      td {
        padding: 10px 8px;
        border-bottom: 1px solid var(--color-border-subtle);
        text-align: left;
        vertical-align: middle;
      }

      th {
        font-size: 0.85rem;
        color: var(--color-text-muted);
        font-weight: 600;
      }

      .identifier {
        font-family: monospace;
        font-size: 0.9rem;
      }

      .actions {
        display: flex;
        justify-content: flex-end;
        gap: 8px;
        white-space: nowrap;
      }

      .danger {
        color: var(--color-action-danger);
      }

      @media (max-width: 700px) {
        .trash-header {
          align-items: flex-start;
          flex-direction: column;
        }

        th,
        td {
          padding: 8px 6px;
        }
      }
    `,
  ];

  @state() declare private loading: boolean;
  @state() declare private pages: TrashPageEntry[];
  @state() declare private error: AugmentedError | null;

  autoLoad = true;

  client = createClient(PageManagementService, getGrpcWebTransport());

  constructor() {
    super();
    this.loading = true;
    this.pages = [];
    this.error = null;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    if (this.autoLoad) {
      void this.refresh();
    }
  }

  async refresh(): Promise<void> {
    this.loading = true;
    this.error = null;
    try {
      const resp = await this.client.listTrash(create(ListTrashRequestSchema, {}));
      this.pages = resp.pages;
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(err, 'list trash');
    } finally {
      this.loading = false;
    }
  }

  private async restorePage(entry: TrashPageEntry): Promise<void> {
    this.error = null;
    try {
      const resp = await this.client.restorePage(create(RestorePageRequestSchema, { trashId: entry.trashId }));
      const restoredIdentifier = resp.restoredIdentifier || entry.identifier;
      showToast(`Restored ${restoredIdentifier}`, 'success', 5);
      await this.refresh();
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(err, 'restore page');
    }
  }

  private async purgePage(entry: TrashPageEntry): Promise<void> {
    if (!globalThis.confirm(`Permanently purge ${entry.identifier}?`)) {
      return;
    }
    this.error = null;
    try {
      await this.client.purgePage(create(PurgePageRequestSchema, { trashId: entry.trashId }));
      showToast(`Purged ${entry.identifier}`, 'success', 5);
      await this.refresh();
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(err, 'purge page');
    }
  }

  private async emptyTrash(): Promise<void> {
    if (!globalThis.confirm('Permanently purge every page in trash?')) {
      return;
    }
    this.error = null;
    try {
      const resp = await this.client.emptyTrash(create(EmptyTrashRequestSchema, {}));
      showToast(`Purged ${resp.purgedCount} trashed pages`, 'success', 5);
      await this.refresh();
    } catch (err: unknown) {
      this.error = AugmentErrorService.augmentError(err, 'empty trash');
    }
  }

  private renderRows() {
    if (this.pages.length === 0) {
      return html`<p class="empty">Trash is empty.</p>`;
    }

    return html`
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Page</th>
              <th>Identifier</th>
              <th>Deleted</th>
              <th>Deleted By</th>
              <th>Days Remaining</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            ${this.pages.map((entry) => html`
              <tr>
                <td>${entry.title || entry.identifier}</td>
                <td class="identifier">${entry.identifier}</td>
                <td>${formatTimestamp(entry.deletedAt)}</td>
                <td>${entry.deletedBy}</td>
                <td>${entry.daysRemaining}</td>
                <td class="actions">
                  <button class="button-base button-secondary" @click=${() => { void this.restorePage(entry); }}>Restore</button>
                  <button class="button-base button-secondary danger" @click=${() => { void this.purgePage(entry); }}>Purge</button>
                </td>
              </tr>
            `)}
          </tbody>
        </table>
      </div>
    `;
  }

  override render() {
    return html`
      ${sharedStyles}
      <section>
        <div class="trash-header">
          <h1>Trash</h1>
          ${this.pages.length > 0
            ? html`<button class="button-base button-secondary danger" @click=${() => { void this.emptyTrash(); }}>Empty Trash</button>`
            : nothing}
        </div>
        ${this.error ? html`<error-display .error=${this.error}></error-display>` : nothing}
        ${this.loading ? html`<p>Loading trash...</p>` : this.renderRows()}
      </section>
    `;
  }
}

customElements.define('page-trash', PageTrash);

declare global {
  interface HTMLElementTagNameMap {
    'page-trash': PageTrash;
  }
}
