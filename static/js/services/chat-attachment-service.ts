/**
 * Service for uploading chat attachments to the transient server-side store.
 * Attachments are session-scoped and not persisted to the wiki.
 */

export interface ChatAttachment {
  /** Opaque server-assigned ID */
  id: string;
  /** URL to retrieve the attachment from the server */
  url: string;
  /** Original filename */
  filename: string;
  /** MIME type */
  mimeType: string;
  /** Object URL for local preview (must be revoked when no longer needed) */
  previewUrl: string;
}

const UPLOAD_ENDPOINT = '/api/chat-attachments';

/**
 * Uploads a file to the transient chat attachment store.
 * Returns metadata including a server URL and a local preview URL.
 */
export async function uploadChatAttachment(file: File): Promise<ChatAttachment> {
  const formData = new FormData();
  formData.append('file', file, file.name);

  const response = await fetch(UPLOAD_ENDPOINT, {
    method: 'POST',
    body: formData,
  });

  if (!response.ok) {
    let message = `Upload failed: ${response.statusText}`;
    try {
      const body: unknown = await response.json();
      if (body !== null && typeof body === 'object' && 'error' in body) {
        const err = (body as Record<string, unknown>)['error'];
        if (typeof err === 'string') {
          message = err;
        }
      }
    } catch {
      // Keep default message
    }
    throw new Error(message);
  }

  const rawData: unknown = await response.json();
  if (rawData === null || typeof rawData !== 'object') {
    throw new Error('Unexpected response format from attachment upload');
  }
  // Extract fields from the unknown object safely, without a narrowing assertion
  const r = rawData as { [k: string]: unknown }; // eslint-disable-line @typescript-eslint/no-unsafe-type-assertion -- narrowing to indexed access is the only practical approach here
  const data = {
    id: String(r['id'] ?? ''),
    url: String(r['url'] ?? ''),
    filename: String(r['filename'] ?? ''),
    mimeType: String(r['mimeType'] ?? ''),
  };

  const previewUrl = URL.createObjectURL(file);

  return {
    id: data.id,
    url: data.url,
    filename: data.filename,
    mimeType: data.mimeType,
    previewUrl,
  };
}

/**
 * Builds a markdown string embedding all attachments in the message.
 * Images use the `![]()` syntax; other files use `[]()`.
 */
export function buildAttachmentMarkdown(attachments: ChatAttachment[]): string {
  return attachments
    .map((a) => {
      const isImage = a.mimeType.startsWith('image/');
      const prefix = isImage ? '!' : '';
      return `${prefix}[${a.filename}](${a.url})`;
    })
    .join('\n');
}

/**
 * Revokes the object URLs for all pending attachments to free browser memory.
 */
export function revokePreviews(attachments: ChatAttachment[]): void {
  for (const a of attachments) {
    URL.revokeObjectURL(a.previewUrl);
  }
}
