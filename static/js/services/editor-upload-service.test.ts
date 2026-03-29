import { expect, fixture, html } from '@open-wc/testing';
import type { SinonStub, SinonSpy } from 'sinon';
import { stub, spy } from 'sinon';
import type { Client } from '@connectrpc/connect';
import { ConnectError, Code } from '@connectrpc/connect';
import type { UploadResult } from './editor-upload-service.js';
import { EditorUploadService } from './editor-upload-service.js';
import type { FileStorageService } from '../gen/api/v1/file_storage_pb.js';

describe('EditorUploadService', () => {
  let service: EditorUploadService;
  let uploadFileStub: SinonStub;
  let mockClient: Client<typeof FileStorageService>;

  beforeEach(() => {
    uploadFileStub = stub();
    mockClient = {
      uploadFile: uploadFileStub,
    } as unknown as Client<typeof FileStorageService>;
    service = new EditorUploadService(mockClient);
  });

  it('should exist', () => {
    expect(service).to.exist;
  });

  describe('uploadFile', () => {
    describe('when uploading an image', () => {
      let result: UploadResult;
      const mockFile = new File(['content'], 'test.png', { type: 'image/png' });

      beforeEach(async () => {
        uploadFileStub.resolves({
          hash: 'abc123',
          uploadUrl: '/uploads/sha256-abc123?filename=test.png',
        });
        result = await service.uploadFile(mockFile);
      });

      it('should call gRPC uploadFile', () => {
        expect(uploadFileStub).to.have.been.calledOnce;
      });

      it('should send file content and filename in the request', () => {
        const request = uploadFileStub.firstCall.args[0];
        expect(request.filename).to.equal('test.png');
        expect(request.content).to.be.instanceOf(Uint8Array);
      });

      it('should return markdown with image prefix', () => {
        expect(result.markdownLink).to.equal('![test.png](/uploads/sha256-abc123?filename=test.png)');
      });

      it('should return the filename', () => {
        expect(result.filename).to.equal('test.png');
      });
    });

    describe('when uploading a non-image file', () => {
      let result: UploadResult;
      const mockFile = new File(['content'], 'document.pdf', { type: 'application/pdf' });

      beforeEach(async () => {
        uploadFileStub.resolves({
          hash: 'def456',
          uploadUrl: '/uploads/sha256-def456?filename=document.pdf',
        });
        result = await service.uploadFile(mockFile);
      });

      it('should return markdown without image prefix', () => {
        expect(result.markdownLink).to.equal('[document.pdf](/uploads/sha256-def456?filename=document.pdf)');
      });
    });

    describe('when gRPC call fails', () => {
      let caughtError: unknown;

      beforeEach(async () => {
        uploadFileStub.rejects(
          new ConnectError('upload failed', Code.Internal)
        );
        try {
          await service.uploadFile(new File([''], 'test.txt', { type: 'text/plain' }));
        } catch (e) {
          caughtError = e;
        }
      });

      it('should throw a ConnectError', () => {
        expect(caughtError).to.be.instanceOf(ConnectError);
      });

      it('should preserve the error code', () => {
        expect((caughtError as ConnectError).code).to.equal(Code.Internal);
      });
    });
  });

  describe('insertMarkdownAtCursor', () => {
    let textarea: HTMLTextAreaElement;
    let keyupSpy: SinonSpy;

    beforeEach(async () => {
      textarea = await fixture<HTMLTextAreaElement>(html`<textarea>Hello world!</textarea>`);
      keyupSpy = spy();
      textarea.addEventListener('keyup', keyupSpy);
    });

    describe('when cursor is in middle of text', () => {
      beforeEach(() => {
        textarea.selectionStart = 6;
        textarea.selectionEnd = 6;
        service.insertMarkdownAtCursor(textarea, '![image](url)');
      });

      it('should insert markdown at cursor position', () => {
        expect(textarea.value).to.equal('Hello ![image](url)world!');
      });

      it('should position cursor after inserted text', () => {
        expect(textarea.selectionStart).to.equal(19);
        expect(textarea.selectionEnd).to.equal(19);
      });

      it('should trigger keyup event for auto-save', () => {
        expect(keyupSpy).to.have.been.calledOnce;
      });
    });

    describe('when text is selected', () => {
      beforeEach(() => {
        textarea.selectionStart = 6;
        textarea.selectionEnd = 11; // "world" is selected
        service.insertMarkdownAtCursor(textarea, '![image](url)');
      });

      it('should replace selection with markdown', () => {
        expect(textarea.value).to.equal('Hello ![image](url)!');
      });
    });

    describe('when cursor is at beginning', () => {
      beforeEach(() => {
        textarea.selectionStart = 0;
        textarea.selectionEnd = 0;
        service.insertMarkdownAtCursor(textarea, '[link](url)');
      });

      it('should insert at beginning', () => {
        expect(textarea.value).to.equal('[link](url)Hello world!');
      });
    });

    describe('when cursor is at end', () => {
      beforeEach(() => {
        const len = textarea.value.length;
        textarea.selectionStart = len;
        textarea.selectionEnd = len;
        service.insertMarkdownAtCursor(textarea, '[link](url)');
      });

      it('should insert at end', () => {
        expect(textarea.value).to.equal('Hello world![link](url)');
      });
    });
  });

  describe('openFilePicker (via selectAndUploadFile)', () => {
    function waitForInputAppended(): Promise<HTMLInputElement> {
      return new Promise<HTMLInputElement>((resolve) => {
        const observer = new MutationObserver((mutations) => {
          for (const mutation of mutations) {
            for (const node of mutation.addedNodes) {
              if (node instanceof HTMLInputElement) {
                observer.disconnect();
                resolve(node);
                return;
              }
            }
          }
        });
        observer.observe(document.body, { childList: true });
      });
    }

    describe('when the cancel event fires', () => {
      let result: UploadResult | undefined;

      beforeEach(async () => {
        const inputAddedPromise = waitForInputAppended();
        const resultPromise = service.selectAndUploadFile();
        const input = await inputAddedPromise;
        input.dispatchEvent(new Event('cancel'));
        result = await resultPromise;
      });

      it('should return undefined', () => {
        expect(result).to.be.undefined;
      });
    });

    describe('when the change event fires with no file selected', () => {
      let result: UploadResult | undefined;

      beforeEach(async () => {
        const inputAddedPromise = waitForInputAppended();
        const resultPromise = service.selectAndUploadFile();
        const input = await inputAddedPromise;
        input.dispatchEvent(new Event('change'));
        result = await resultPromise;
      });

      it('should return undefined', () => {
        expect(result).to.be.undefined;
      });
    });
  });

  describe('extractFilename', () => {
    // Type interface for accessing private method in tests
    interface ServiceWithPrivateMethods {
      extractFilename: (s: string) => string;
    }

    describe('when location has filename parameter', () => {
      it('should extract filename from URL', () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        const result = (service as unknown as ServiceWithPrivateMethods).extractFilename(
          '/uploads/sha256-abc?filename=my-image.png'
        );
        expect(result).to.equal('my-image.png');
      });
    });

    describe('when location has encoded filename', () => {
      it('should decode the filename', () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        const result = (service as unknown as ServiceWithPrivateMethods).extractFilename(
          '/uploads/sha256-abc?filename=my%20image.png'
        );
        expect(result).to.equal('my image.png');
      });
    });

    describe('when location has no filename parameter', () => {
      it('should return default upload', () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        const result = (service as unknown as ServiceWithPrivateMethods).extractFilename(
          '/uploads/sha256-abc'
        );
        expect(result).to.equal('upload');
      });
    });
  });
});
