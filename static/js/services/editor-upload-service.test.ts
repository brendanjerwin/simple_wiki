import { expect, fixture, html } from '@open-wc/testing';
import { stub, SinonStub, spy, SinonSpy } from 'sinon';
import { EditorUploadService, UploadResult } from './editor-upload-service.js';

describe('EditorUploadService', () => {
  let service: EditorUploadService;

  beforeEach(() => {
    service = new EditorUploadService();
  });

  it('should exist', () => {
    expect(service).to.exist;
  });

  describe('uploadFile', () => {
    let fetchStub: SinonStub;

    beforeEach(() => {
      fetchStub = stub(window, 'fetch');
    });

    afterEach(() => {
      fetchStub.restore();
    });

    describe('when uploading an image', () => {
      let result: UploadResult;
      const mockFile = new File(['content'], 'test.png', { type: 'image/png' });

      beforeEach(async () => {
        fetchStub.resolves(new Response('', {
          status: 200,
          headers: { 'Location': '/uploads/sha256-abc123?filename=test.png' }
        }));
        result = await service.uploadFile(mockFile);
      });

      it('should POST to /uploads', () => {
        expect(fetchStub).to.have.been.calledOnce;
        const [url, options] = fetchStub.firstCall.args;
        expect(url).to.equal('/uploads');
        expect(options.method).to.equal('POST');
      });

      it('should send file in FormData', () => {
        const [, options] = fetchStub.firstCall.args;
        expect(options.body).to.be.instanceOf(FormData);
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
        fetchStub.resolves(new Response('', {
          status: 200,
          headers: { 'Location': '/uploads/sha256-def456?filename=document.pdf' }
        }));
        result = await service.uploadFile(mockFile);
      });

      it('should return markdown without image prefix', () => {
        expect(result.markdownLink).to.equal('[document.pdf](/uploads/sha256-def456?filename=document.pdf)');
      });
    });

    describe('when upload fails', () => {
      let error: Error;

      beforeEach(async () => {
        fetchStub.resolves(new Response('', {
          status: 500,
          statusText: 'Internal Server Error'
        }));
        try {
          await service.uploadFile(new File([''], 'test.txt', { type: 'text/plain' }));
        } catch (e) {
          if (e instanceof Error) {
            error = e;
          }
        }
      });

      it('should throw error with status', () => {
        expect(error).to.exist;
        expect(error.message).to.include('500');
        expect(error.message).to.include('Internal Server Error');
      });
    });

    describe('when response has no Location header', () => {
      let error: Error;

      beforeEach(async () => {
        fetchStub.resolves(new Response('', { status: 200 }));
        try {
          await service.uploadFile(new File([''], 'test.txt', { type: 'text/plain' }));
        } catch (e) {
          if (e instanceof Error) {
            error = e;
          }
        }
      });

      it('should throw error about missing Location', () => {
        expect(error).to.exist;
        expect(error.message).to.include('Location');
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
