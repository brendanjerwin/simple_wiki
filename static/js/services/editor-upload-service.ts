import { createClient, type Client } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from '../web-components/grpc-transport.js';
import { FileStorageService, UploadFileRequestSchema } from '../gen/api/v1/file_storage_pb.js';

/**
 * Result of a file upload operation.
 */
export interface UploadResult {
  /** The markdown link to insert */
  markdownLink: string;
  /** The original filename */
  filename: string;
}

/**
 * EditorUploadService handles file uploads and markdown insertion for the editor.
 */
export class EditorUploadService {
  private readonly client: Client<typeof FileStorageService>;

  constructor(client?: Client<typeof FileStorageService>) {
    this.client = client ?? createClient(FileStorageService, getGrpcWebTransport());
  }

  /**
   * Opens file picker for images and uploads the selected file.
   * @returns Promise resolving to upload result, or undefined if cancelled
   */
  async selectAndUploadImage(): Promise<UploadResult | undefined> {
    return this.selectAndUpload('image/*');
  }

  /**
   * Opens file picker for any file type and uploads the selected file.
   * @returns Promise resolving to upload result, or undefined if cancelled
   */
  async selectAndUploadFile(): Promise<UploadResult | undefined> {
    return this.selectAndUpload('*/*');
  }

  /**
   * Opens camera for photo capture and uploads the captured photo.
   * @returns Promise resolving to upload result, or undefined if cancelled
   */
  async capturePhoto(): Promise<UploadResult | undefined> {
    return this.selectAndUpload('image/*', true);
  }

  /**
   * Uploads a file to the server via gRPC.
   * @param file The file to upload
   * @returns Promise resolving to upload result
   */
  async uploadFile(file: File): Promise<UploadResult> {
    const content = new Uint8Array(await file.arrayBuffer());
    const request = create(UploadFileRequestSchema, {
      content,
      filename: file.name,
    });

    const response = await this.client.uploadFile(request);

    const uploadUrl = response.uploadUrl;
    const filename = this.extractFilename(uploadUrl);
    const isImage = file.type.startsWith('image/');
    const prefix = isImage ? '!' : '';
    const markdownLink = `${prefix}[${filename}](${uploadUrl})`;

    return { markdownLink, filename };
  }

  /**
   * Inserts markdown at the current cursor position in a textarea.
   * @param textarea The textarea element
   * @param markdown The markdown to insert
   */
  insertMarkdownAtCursor(textarea: HTMLTextAreaElement, markdown: string): void {
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const text = textarea.value;

    textarea.value = text.substring(0, start) + markdown + text.substring(end);

    // Position cursor after inserted text
    const newPosition = start + markdown.length;
    textarea.selectionStart = newPosition;
    textarea.selectionEnd = newPosition;

    // Trigger keyup for auto-save (matches existing Dropzone behavior)
    textarea.dispatchEvent(new Event('keyup', { bubbles: true }));
  }

  private async selectAndUpload(accept: string, capture: boolean = false): Promise<UploadResult | undefined> {
    const file = await this.openFilePicker(accept, capture);
    if (!file) {
      return undefined;
    }
    return this.uploadFile(file);
  }

  private openFilePicker(accept: string, capture: boolean): Promise<File | undefined> {
    return new Promise((resolve) => {
      const input = document.createElement('input');
      input.type = 'file';
      input.accept = accept;
      if (capture) {
        input.setAttribute('capture', 'environment');
      }
      input.style.display = 'none';

      input.addEventListener('change', () => {
        const file = input.files?.[0];
        input.remove();
        resolve(file);
      });

      input.addEventListener('cancel', () => {
        input.remove();
        resolve(undefined);
      });

      document.body.appendChild(input);
      input.click();
    });
  }

  private extractFilename(location: string): string {
    try {
      const url = new URL(location, window.location.origin);
      return url.searchParams.get('filename') || 'upload';
    } catch {
      // Fallback: try to extract from query string manually
      const match = location.match(/filename=([^&]+)/);
      if (match?.[1]) {
        return decodeURIComponent(match[1]);
      }
      return 'upload';
    }
  }
}

// Export a singleton instance for convenience
export const editorUploadService = new EditorUploadService();
