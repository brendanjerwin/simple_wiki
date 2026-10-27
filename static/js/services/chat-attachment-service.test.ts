import { expect } from '@esm-bundle/chai';
import sinon, { type SinonStub } from 'sinon';
import { uploadChatAttachment, buildAttachmentMarkdown, revokePreviews, type ChatAttachment } from './chat-attachment-service.js';

describe('chat-attachment-service', () => {
  let fetchStub: SinonStub;
  let createObjectURLStub: SinonStub;
  let revokeObjectURLStub: SinonStub;

  beforeEach(() => {
    fetchStub = sinon.stub(window, 'fetch');
    createObjectURLStub = sinon.stub(URL, 'createObjectURL').returns('blob:fake-preview-url');
    revokeObjectURLStub = sinon.stub(URL, 'revokeObjectURL');
  });

  afterEach(() => {
    fetchStub.restore();
    createObjectURLStub.restore();
    revokeObjectURLStub.restore();
  });

  describe('uploadChatAttachment', () => {
    describe('when the server responds with success', () => {
      let result: ChatAttachment;

      beforeEach(async () => {
        fetchStub.resolves(
          new Response(
            JSON.stringify({
              id: 'test-id-123',
              url: '/api/chat-attachments/test-id-123',
              filename: 'photo.png',
              mimeType: 'image/png',
            }),
            { status: 201, headers: { 'Content-Type': 'application/json' } },
          ),
        );

        const file = new File(['fake image data'], 'photo.png', { type: 'image/png' });
        result = await uploadChatAttachment(file);
      });

      it('should return the attachment ID', () => {
        expect(result.id).to.equal('test-id-123');
      });

      it('should return the server URL', () => {
        expect(result.url).to.equal('/api/chat-attachments/test-id-123');
      });

      it('should return the filename', () => {
        expect(result.filename).to.equal('photo.png');
      });

      it('should return the mimeType', () => {
        expect(result.mimeType).to.equal('image/png');
      });

      it('should return a previewUrl from createObjectURL', () => {
        expect(result.previewUrl).to.equal('blob:fake-preview-url');
      });

      it('should POST to the upload endpoint', () => {
        expect(fetchStub.calledOnce).to.equal(true);
        const [url, options] = fetchStub.firstCall.args as [string, RequestInit];
        expect(url).to.equal('/api/chat-attachments');
        expect(options.method).to.equal('POST');
      });
    });

    describe('when the server responds with an error', () => {
      let error: Error;

      beforeEach(async () => {
        fetchStub.resolves(
          new Response(
            JSON.stringify({ error: 'attachment exceeds maximum size' }),
            { status: 422, headers: { 'Content-Type': 'application/json' } },
          ),
        );

        const file = new File(['data'], 'big.bin', { type: 'application/octet-stream' });
        try {
          await uploadChatAttachment(file);
        } catch (e) {
          error = e as Error;
        }
      });

      it('should throw an error with the server message', () => {
        expect(error).to.be.instanceOf(Error);
        expect(error.message).to.equal('attachment exceeds maximum size');
      });
    });

    describe('when the server returns a non-JSON error body', () => {
      let error: Error;

      beforeEach(async () => {
        fetchStub.resolves(new Response('', { status: 503, statusText: 'Service Unavailable' }));

        const file = new File(['data'], 'test.txt', { type: 'text/plain' });
        try {
          await uploadChatAttachment(file);
        } catch (e) {
          error = e as Error;
        }
      });

      it('should throw an error with the status text', () => {
        expect(error).to.be.instanceOf(Error);
        expect(error.message).to.contain('Service Unavailable');
      });
    });
  });

  describe('buildAttachmentMarkdown', () => {
    describe('with image attachments', () => {
      let result: string;

      beforeEach(() => {
        const attachments: ChatAttachment[] = [
          { id: '1', url: '/api/chat-attachments/1', filename: 'photo.png', mimeType: 'image/png', previewUrl: 'blob:1' },
        ];
        result = buildAttachmentMarkdown(attachments);
      });

      it('should use image markdown syntax', () => {
        expect(result).to.equal('![photo.png](/api/chat-attachments/1)');
      });
    });

    describe('with non-image attachments', () => {
      let result: string;

      beforeEach(() => {
        const attachments: ChatAttachment[] = [
          { id: '2', url: '/api/chat-attachments/2', filename: 'report.pdf', mimeType: 'application/pdf', previewUrl: 'blob:2' },
        ];
        result = buildAttachmentMarkdown(attachments);
      });

      it('should use link markdown syntax without !', () => {
        expect(result).to.equal('[report.pdf](/api/chat-attachments/2)');
      });
    });

    describe('with multiple attachments', () => {
      let result: string;

      beforeEach(() => {
        const attachments: ChatAttachment[] = [
          { id: '1', url: '/api/chat-attachments/1', filename: 'photo.png', mimeType: 'image/png', previewUrl: 'blob:1' },
          { id: '2', url: '/api/chat-attachments/2', filename: 'doc.pdf', mimeType: 'application/pdf', previewUrl: 'blob:2' },
        ];
        result = buildAttachmentMarkdown(attachments);
      });

      it('should join with newlines', () => {
        expect(result).to.equal('![photo.png](/api/chat-attachments/1)\n[doc.pdf](/api/chat-attachments/2)');
      });
    });

    describe('with empty list', () => {
      let result: string;

      beforeEach(() => {
        result = buildAttachmentMarkdown([]);
      });

      it('should return empty string', () => {
        expect(result).to.equal('');
      });
    });
  });

  describe('revokePreviews', () => {
    describe('when called with attachments', () => {
      beforeEach(() => {
        const attachments: ChatAttachment[] = [
          { id: '1', url: '/api/chat-attachments/1', filename: 'a.png', mimeType: 'image/png', previewUrl: 'blob:preview-1' },
          { id: '2', url: '/api/chat-attachments/2', filename: 'b.png', mimeType: 'image/png', previewUrl: 'blob:preview-2' },
        ];
        revokePreviews(attachments);
      });

      it('should call revokeObjectURL twice', () => {
        expect(revokeObjectURLStub.callCount).to.equal(2);
      });

      it('should revoke the first attachment preview', () => {
        expect(revokeObjectURLStub.calledWith('blob:preview-1')).to.equal(true);
      });

      it('should revoke the second attachment preview', () => {
        expect(revokeObjectURLStub.calledWith('blob:preview-2')).to.equal(true);
      });
    });
  });
});
