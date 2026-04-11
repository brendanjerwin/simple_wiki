import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import type { Client } from '@connectrpc/connect';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  FileStorageService,
  UploadFileRequestSchema,
} from '../gen/api/v1/file_storage_pb.js';
import { foundationCSS, sharedStyles } from './shared-styles.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import './error-display.js';

const MB_TO_BYTES = 1024 * 1024;

/**
 * FileDropZone - A drag-and-drop file upload zone that wraps content via a slot.
 *
 * Uploads files via gRPC FileStorageService and dispatches `file-uploaded` events.
 *
 * @element file-drop-zone
 *
 * @property {boolean} allowUploads - Whether uploads are enabled (attribute: allow-uploads)
 * @property {number} maxUploadMb - Maximum file size in MB (attribute: max-upload-mb)
 *
 * @fires file-uploaded - Dispatched when a file is successfully uploaded
 *   detail: { uploadUrl: string, filename: string, isImage: boolean }
 */
export class FileDropZone extends LitElement {
  static override readonly styles = [
    foundationCSS,
    css`
      :host {
        display: block;
      }

      .drop-zone {
        position: relative;
        min-height: 200px;
        height: 100%;
      }

      .drop-overlay,
      .upload-overlay {
        position: absolute;
        inset: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 10;
        border-radius: 4px;
      }

      .drop-overlay {
        background: rgba(74, 144, 217, 0.15);
        border: 2px dashed #4a90d9;
      }

      .upload-overlay {
        background: rgba(0, 0, 0, 0.3);
      }

      .drop-indicator {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 8px;
        color: #4a90d9;
        font-size: 16px;
        font-weight: 500;
        pointer-events: none;
      }

      .drop-indicator i {
        font-size: 32px;
      }

      .upload-overlay span {
        color: #fff;
        font-size: 16px;
        font-weight: 500;
      }

      /* Visually hidden but announced by screen readers */
      .sr-only {
        position: absolute;
        width: 1px;
        height: 1px;
        padding: 0;
        margin: -1px;
        overflow: hidden;
        clip: rect(0, 0, 0, 0);
        white-space: nowrap;
        border: 0;
      }
    `,
  ];

  @property({ type: Boolean, attribute: 'allow-uploads' })
  declare allowUploads: boolean;

  @property({ type: Number, attribute: 'max-upload-mb' })
  declare maxUploadMb: number;

  @state()
  declare dragging: boolean;

  @state()
  declare uploading: boolean;

  @state()
  declare error: AugmentedError | null;

  private _client: Client<typeof FileStorageService> | null = null;

  private get client(): Client<typeof FileStorageService> {
    this._client ??= createClient(FileStorageService, getGrpcWebTransport());
    return this._client;
  }

  setClient(client: Client<typeof FileStorageService>): void {
    this._client = client;
  }

  constructor() {
    super();
    this.allowUploads = false;
    this.maxUploadMb = 10;
    this.dragging = false;
    this.uploading = false;
    this.error = null;
  }

  _onDragEnter(e: DragEvent): void {
    e.preventDefault();
    if (!this.allowUploads) return;
    this.dragging = true;
  }

  _onDragOver(e: DragEvent): void {
    e.preventDefault();
    if (!this.allowUploads) return;
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = 'copy';
    }
  }

  _onDragLeave(e: DragEvent): void {
    e.preventDefault();
    if (!this.allowUploads) return;

    // Only set dragging to false when leaving the drop zone itself,
    // not when entering a child element
    if (
      e.currentTarget instanceof HTMLElement &&
      e.relatedTarget instanceof Node &&
      e.currentTarget.contains(e.relatedTarget)
    ) {
      return;
    }
    this.dragging = false;
  }

  async _onDrop(e: DragEvent): Promise<void> {
    e.preventDefault();
    this.dragging = false;

    if (!this.allowUploads) return;

    const file = e.dataTransfer?.files[0];
    if (!file) return;

    await this._uploadFile(file);
  }

  async _uploadFile(file: File): Promise<void> {
    const maxSizeBytes = this.maxUploadMb * MB_TO_BYTES;
    if (file.size > maxSizeBytes) {
      this.error = AugmentErrorService.augmentError(
        new Error(`File "${file.name}" is too large (${(file.size / MB_TO_BYTES).toFixed(1)} MB). Maximum size is ${this.maxUploadMb} MB.`),
        'validate file size'
      );
      return;
    }

    this.uploading = true;
    this.error = null;

    try {
      const content = new Uint8Array(await file.arrayBuffer());
      const request = create(UploadFileRequestSchema, {
        content,
        filename: file.name,
      });
      const response = await this.client.uploadFile(request);

      this.dispatchEvent(
        new CustomEvent('file-uploaded', {
          bubbles: true,
          composed: true,
          detail: {
            uploadUrl: response.uploadUrl,
            filename: file.name,
            isImage: file.type.startsWith('image/'),
          },
        })
      );
    } catch (err) {
      this.error = AugmentErrorService.augmentError(err, 'upload file');
    } finally {
      this.uploading = false;
    }
  }

  private _statusMessage(): string {
    if (this.uploading) return 'Uploading file, please wait…';
    if (this.dragging) return 'Drop file to upload';
    return '';
  }

  override render() {
    return html`
      ${sharedStyles}
      <div
        class="drop-zone"
        role="region"
        aria-label="File upload area"
        aria-busy="${this.uploading ? 'true' : 'false'}"
        @dragenter=${this._onDragEnter}
        @dragover=${this._onDragOver}
        @dragleave=${this._onDragLeave}
        @drop=${this._onDrop}
      >
        <slot></slot>

        <!-- Persistent live region: announces drag/upload status changes -->
        <div class="sr-only" role="status" aria-live="polite" aria-atomic="true">
          ${this._statusMessage()}
        </div>

        <!-- Persistent alert region: announces errors -->
        <div class="sr-only" role="alert" aria-atomic="true">
          ${this.error ? this.error.message : ''}
        </div>

        ${this.dragging
          ? html`
              <div class="drop-overlay">
                <div class="drop-indicator">
                  <i class="fa-solid fa-cloud-arrow-up"></i>
                  <span>Drop file to upload</span>
                </div>
              </div>
            `
          : nothing}
        ${this.uploading
          ? html`
              <div class="upload-overlay">
                <span>Uploading...</span>
              </div>
            `
          : nothing}
        ${this.error
          ? html`<error-display
              .augmentedError=${this.error}
              .action=${{ label: 'Dismiss', onClick: () => { this.error = null; } }}
            ></error-display>`
          : nothing}
      </div>
    `;
  }
}

customElements.define('file-drop-zone', FileDropZone);

declare global {
  interface HTMLElementTagNameMap {
    'file-drop-zone': FileDropZone;
  }
}
