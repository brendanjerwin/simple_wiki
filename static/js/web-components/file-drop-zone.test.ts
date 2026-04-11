import { expect, fixture, html } from '@open-wc/testing';
import { stub } from 'sinon';
import type { SinonStub } from 'sinon';
import './file-drop-zone.js';
import type { FileDropZone } from './file-drop-zone.js';
import { AugmentedError } from './augment-error-service.js';

// Minimal stub interface matching the shape of FileStorageService client
interface StubClient {
  uploadFile: SinonStub;
}

function createStubClient(): StubClient {
  return {
    uploadFile: stub(),
  };
}

function createDragEvent(type: string, options?: Partial<DragEvent>): DragEvent {
  const event = new Event(type, { bubbles: true, cancelable: true });
  // DragEvent constructor is not well-supported in test environments,
  // so we create a base Event and add dataTransfer
  Object.defineProperty(event, 'dataTransfer', {
    value: options?.dataTransfer ?? {
      files: [],
      dropEffect: 'none',
    },
  });
  return event as DragEvent;
}

function createFile(
  name: string,
  sizeBytes: number,
  type: string = 'text/plain'
): File {
  const content = new Uint8Array(sizeBytes);
  return new File([content], name, { type });
}

describe('FileDropZone', () => {
  let el: FileDropZone;

  describe('when created with defaults', () => {
    beforeEach(async () => {
      el = await fixture(html`<file-drop-zone></file-drop-zone>`);
    });

    it('should exist', () => {
      expect(el).to.exist;
    });

    it('should have allowUploads false by default', () => {
      expect(el.allowUploads).to.be.false;
    });

    it('should have maxUploadMb of 10 by default', () => {
      expect(el.maxUploadMb).to.equal(10);
    });

    it('should not be dragging', () => {
      expect(el.dragging).to.be.false;
    });

    it('should not be uploading', () => {
      expect(el.uploading).to.be.false;
    });

    it('should have no error', () => {
      expect(el.error).to.be.null;
    });
  });

  describe('when created with attributes', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <file-drop-zone allow-uploads max-upload-mb="25"></file-drop-zone>
      `);
    });

    it('should have allowUploads true', () => {
      expect(el.allowUploads).to.be.true;
    });

    it('should have maxUploadMb of 25', () => {
      expect(el.maxUploadMb).to.equal(25);
    });
  });

  describe('ARIA attributes', () => {
    beforeEach(async () => {
      el = await fixture(html`<file-drop-zone></file-drop-zone>`);
    });

    it('should have role="region" on the drop zone', () => {
      const dropZone = el.shadowRoot?.querySelector('.drop-zone');
      expect(dropZone?.getAttribute('role')).to.equal('region');
    });

    it('should have aria-label on the drop zone', () => {
      const dropZone = el.shadowRoot?.querySelector('.drop-zone');
      expect(dropZone?.getAttribute('aria-label')).to.equal('File upload area');
    });

    it('should have aria-busy="false" when not uploading', () => {
      const dropZone = el.shadowRoot?.querySelector('.drop-zone');
      expect(dropZone?.getAttribute('aria-busy')).to.equal('false');
    });

    it('should have a polite live region for status', () => {
      const liveRegion = el.shadowRoot?.querySelector('[role="status"][aria-live="polite"]');
      expect(liveRegion).to.exist;
    });

    it('should have an alert region for errors', () => {
      const alertRegion = el.shadowRoot?.querySelector('[role="alert"]');
      expect(alertRegion).to.exist;
    });
  });

  describe('when uploading', () => {
    let stubClient: StubClient;

    beforeEach(async () => {
      el = await fixture(html`<file-drop-zone allow-uploads></file-drop-zone>`);
      stubClient = createStubClient();
      // Never-resolving promise to keep upload in progress
      stubClient.uploadFile.returns(new Promise(() => {}));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- stub client matches the shape needed for testing
      el.setClient(stubClient as unknown as Parameters<FileDropZone['setClient']>[0]);

      const file = createFile('test.txt', 100);
      const fileList = { 0: file, length: 1 } as unknown as FileList;
      const event = createDragEvent('drop', {
        dataTransfer: { files: fileList, dropEffect: 'none' } as unknown as DataTransfer,
      });

      const dropZone = el.shadowRoot?.querySelector('.drop-zone');
      dropZone?.dispatchEvent(event);

      await new Promise(resolve => setTimeout(resolve, 10));
      await el.updateComplete;
    });

    it('should have aria-busy="true"', () => {
      const dropZone = el.shadowRoot?.querySelector('.drop-zone');
      expect(dropZone?.getAttribute('aria-busy')).to.equal('true');
    });

    it('should announce uploading status in the live region', () => {
      const liveRegion = el.shadowRoot?.querySelector('[role="status"]');
      expect(liveRegion?.textContent?.trim()).to.include('Uploading');
    });
  });

  describe('when an error occurs', () => {
    beforeEach(async () => {
      el = await fixture(html`<file-drop-zone allow-uploads max-upload-mb="1"></file-drop-zone>`);

      const stubClient = createStubClient();
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- stub client matches the shape needed for testing
      el.setClient(stubClient as unknown as Parameters<FileDropZone['setClient']>[0]);

      const file = createFile('big.txt', 2 * 1024 * 1024);
      const fileList = { 0: file, length: 1 } as unknown as FileList;
      const event = createDragEvent('drop', {
        dataTransfer: { files: fileList, dropEffect: 'none' } as unknown as DataTransfer,
      });

      const dropZone = el.shadowRoot?.querySelector('.drop-zone');
      dropZone?.dispatchEvent(event);

      await new Promise(resolve => setTimeout(resolve, 10));
      await el.updateComplete;
    });

    it('should announce error in the alert region', () => {
      const alertRegion = el.shadowRoot?.querySelector('[role="alert"]');
      expect(alertRegion?.textContent?.trim()).to.not.equal('');
    });
  });

  describe('slot content', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <file-drop-zone>
          <textarea>Some content</textarea>
        </file-drop-zone>
      `);
    });

    it('should render a slot element', () => {
      const slot = el.shadowRoot?.querySelector('slot');
      expect(slot).to.exist;
    });

    it('should project the slotted content', () => {
      const slot = el.shadowRoot?.querySelector('slot') as HTMLSlotElement;
      const assigned = slot.assignedElements();
      expect(assigned.length).to.equal(1);
      expect(assigned[0]?.tagName).to.equal('TEXTAREA');
    });
  });

  describe('when allowUploads is false', () => {
    beforeEach(async () => {
      el = await fixture(html`<file-drop-zone></file-drop-zone>`);
    });

    describe('when dragenter event fires', () => {
      beforeEach(() => {
        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        const event = createDragEvent('dragenter');
        dropZone?.dispatchEvent(event);
      });

      it('should not set dragging to true', () => {
        expect(el.dragging).to.be.false;
      });
    });

    describe('when drop event fires', () => {
      let stubClient: StubClient;

      beforeEach(async () => {
        stubClient = createStubClient();
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- stub client matches the shape needed for testing
        el.setClient(stubClient as unknown as Parameters<FileDropZone['setClient']>[0]);

        const file = createFile('test.txt', 100);
        const fileList = { 0: file, length: 1 } as unknown as FileList;
        const event = createDragEvent('drop', {
          dataTransfer: { files: fileList, dropEffect: 'none' } as unknown as DataTransfer,
        });

        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        dropZone?.dispatchEvent(event);

        await el.updateComplete;
      });

      it('should not call uploadFile', () => {
        expect(stubClient.uploadFile).to.not.have.been.called;
      });
    });
  });

  describe('when allowUploads is true', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <file-drop-zone allow-uploads></file-drop-zone>
      `);
    });

    describe('when dragenter event fires', () => {
      beforeEach(async () => {
        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        const event = createDragEvent('dragenter');
        dropZone?.dispatchEvent(event);
        await el.updateComplete;
      });

      it('should set dragging to true', () => {
        expect(el.dragging).to.be.true;
      });

      it('should show the drop overlay', () => {
        const overlay = el.shadowRoot?.querySelector('.drop-overlay');
        expect(overlay).to.exist;
      });

      it('should display the upload indicator text', () => {
        const indicator = el.shadowRoot?.querySelector('.drop-indicator span');
        expect(indicator?.textContent).to.equal('Drop file to upload');
      });
    });

    describe('when dragleave event fires', () => {
      beforeEach(async () => {
        // First trigger dragenter to set dragging true
        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        dropZone?.dispatchEvent(createDragEvent('dragenter'));
        await el.updateComplete;

        // Then trigger dragleave with relatedTarget outside
        const leaveEvent = createDragEvent('dragleave');
        Object.defineProperty(leaveEvent, 'relatedTarget', { value: document.body });
        dropZone?.dispatchEvent(leaveEvent);
        await el.updateComplete;
      });

      it('should set dragging to false', () => {
        expect(el.dragging).to.be.false;
      });

      it('should hide the drop overlay', () => {
        const overlay = el.shadowRoot?.querySelector('.drop-overlay');
        expect(overlay).to.not.exist;
      });
    });

    describe('when dragover event fires', () => {
      let dragEvent: DragEvent;

      beforeEach(() => {
        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        const dataTransfer = { files: [], dropEffect: 'none' };
        dragEvent = createDragEvent('dragover', {
          dataTransfer: dataTransfer as unknown as DataTransfer,
        });
        dropZone?.dispatchEvent(dragEvent);
      });

      it('should set dropEffect to copy', () => {
        expect(dragEvent.dataTransfer?.dropEffect).to.equal('copy');
      });
    });

    describe('when a file is dropped', () => {
      let stubClient: StubClient;
      let uploadedEvent: CustomEvent | null;

      beforeEach(async () => {
        stubClient = createStubClient();
        stubClient.uploadFile.resolves({
          hash: 'sha256-abc123',
          uploadUrl: '/uploads/sha256-abc123?filename=test.txt',
        });
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- stub client matches the shape needed for testing
        el.setClient(stubClient as unknown as Parameters<FileDropZone['setClient']>[0]);

        uploadedEvent = null;
        el.addEventListener('file-uploaded', (e: Event) => {
          uploadedEvent = e as CustomEvent;
        });

        const file = createFile('test.txt', 100);
        const fileList = { 0: file, length: 1, item: (i: number) => i === 0 ? file : null } as unknown as FileList;
        const event = createDragEvent('drop', {
          dataTransfer: { files: fileList, dropEffect: 'none' } as unknown as DataTransfer,
        });

        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        dropZone?.dispatchEvent(event);

        // Wait for async upload to complete
        await new Promise(resolve => setTimeout(resolve, 50));
        await el.updateComplete;
      });

      it('should call uploadFile on the client', () => {
        expect(stubClient.uploadFile).to.have.been.calledOnce;
      });

      it('should dispatch file-uploaded event', () => {
        expect(uploadedEvent).to.not.be.null;
      });

      it('should include uploadUrl in event detail', () => {
        expect(uploadedEvent?.detail.uploadUrl).to.equal(
          '/uploads/sha256-abc123?filename=test.txt'
        );
      });

      it('should include filename in event detail', () => {
        expect(uploadedEvent?.detail.filename).to.equal('test.txt');
      });

      it('should include isImage false for text file', () => {
        expect(uploadedEvent?.detail.isImage).to.be.false;
      });

      it('should reset uploading to false', () => {
        expect(el.uploading).to.be.false;
      });

      it('should reset dragging to false', () => {
        expect(el.dragging).to.be.false;
      });
    });

    describe('when an image file is dropped', () => {
      let uploadedEvent: CustomEvent | null;

      beforeEach(async () => {
        const stubClient = createStubClient();
        stubClient.uploadFile.resolves({
          hash: 'sha256-img123',
          uploadUrl: '/uploads/sha256-img123?filename=photo.png',
        });
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- stub client matches the shape needed for testing
        el.setClient(stubClient as unknown as Parameters<FileDropZone['setClient']>[0]);

        uploadedEvent = null;
        el.addEventListener('file-uploaded', (e: Event) => {
          uploadedEvent = e as CustomEvent;
        });

        const file = createFile('photo.png', 100, 'image/png');
        const fileList = { 0: file, length: 1 } as unknown as FileList;
        const event = createDragEvent('drop', {
          dataTransfer: { files: fileList, dropEffect: 'none' } as unknown as DataTransfer,
        });

        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        dropZone?.dispatchEvent(event);

        await new Promise(resolve => setTimeout(resolve, 50));
        await el.updateComplete;
      });

      it('should include isImage true for image file', () => {
        expect(uploadedEvent?.detail.isImage).to.be.true;
      });
    });

    describe('when upload is in progress', () => {
      let stubClient: StubClient;

      beforeEach(async () => {
        stubClient = createStubClient();
        // Create a promise that never resolves to simulate in-progress upload
        stubClient.uploadFile.returns(new Promise(() => {}));
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- stub client matches the shape needed for testing
        el.setClient(stubClient as unknown as Parameters<FileDropZone['setClient']>[0]);

        const file = createFile('test.txt', 100);
        const fileList = { 0: file, length: 1 } as unknown as FileList;
        const event = createDragEvent('drop', {
          dataTransfer: { files: fileList, dropEffect: 'none' } as unknown as DataTransfer,
        });

        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        dropZone?.dispatchEvent(event);

        // Wait for uploading state to be set
        await new Promise(resolve => setTimeout(resolve, 10));
        await el.updateComplete;
      });

      it('should set uploading to true', () => {
        expect(el.uploading).to.be.true;
      });

      it('should show the upload overlay', () => {
        const overlay = el.shadowRoot?.querySelector('.upload-overlay');
        expect(overlay).to.exist;
      });

      it('should display uploading text', () => {
        const overlay = el.shadowRoot?.querySelector('.upload-overlay span');
        expect(overlay?.textContent).to.equal('Uploading...');
      });
    });
  });

  describe('file size validation', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <file-drop-zone allow-uploads max-upload-mb="1"></file-drop-zone>
      `);
    });

    describe('when file exceeds maxUploadMb', () => {
      let stubClient: StubClient;

      beforeEach(async () => {
        stubClient = createStubClient();
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- stub client matches the shape needed for testing
        el.setClient(stubClient as unknown as Parameters<FileDropZone['setClient']>[0]);

        // 2 MB file, max is 1 MB
        const file = createFile('big-file.txt', 2 * 1024 * 1024);
        const fileList = { 0: file, length: 1 } as unknown as FileList;
        const event = createDragEvent('drop', {
          dataTransfer: { files: fileList, dropEffect: 'none' } as unknown as DataTransfer,
        });

        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        dropZone?.dispatchEvent(event);

        await new Promise(resolve => setTimeout(resolve, 10));
        await el.updateComplete;
      });

      it('should set error as AugmentedError', () => {
        expect(el.error).to.be.instanceOf(AugmentedError);
      });

      it('should include file name in error message', () => {
        expect(el.error?.message).to.include('big-file.txt');
      });

      it('should include maximum size in error message', () => {
        expect(el.error?.message).to.include('1 MB');
      });

      it('should not call uploadFile', () => {
        expect(stubClient.uploadFile).to.not.have.been.called;
      });

      it('should not set uploading to true', () => {
        expect(el.uploading).to.be.false;
      });

      it('should render error-display component', () => {
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        expect(errorDisplay).to.exist;
      });
    });

    describe('when file is within maxUploadMb', () => {
      let stubClient: StubClient;

      beforeEach(async () => {
        stubClient = createStubClient();
        stubClient.uploadFile.resolves({
          hash: 'sha256-ok',
          uploadUrl: '/uploads/sha256-ok?filename=small.txt',
        });
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- stub client matches the shape needed for testing
        el.setClient(stubClient as unknown as Parameters<FileDropZone['setClient']>[0]);

        // 0.5 MB file, max is 1 MB
        const file = createFile('small.txt', 512 * 1024);
        const fileList = { 0: file, length: 1 } as unknown as FileList;
        const event = createDragEvent('drop', {
          dataTransfer: { files: fileList, dropEffect: 'none' } as unknown as DataTransfer,
        });

        const dropZone = el.shadowRoot?.querySelector('.drop-zone');
        dropZone?.dispatchEvent(event);

        await new Promise(resolve => setTimeout(resolve, 50));
        await el.updateComplete;
      });

      it('should call uploadFile', () => {
        expect(stubClient.uploadFile).to.have.been.calledOnce;
      });

      it('should not set error', () => {
        expect(el.error).to.be.null;
      });
    });
  });

  describe('upload error handling', () => {
    let stubClient: StubClient;

    beforeEach(async () => {
      el = await fixture(html`
        <file-drop-zone allow-uploads></file-drop-zone>
      `);

      stubClient = createStubClient();
      stubClient.uploadFile.rejects(new Error('Network error: upload failed'));
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- stub client matches the shape needed for testing
      el.setClient(stubClient as unknown as Parameters<FileDropZone['setClient']>[0]);

      const file = createFile('test.txt', 100);
      const fileList = { 0: file, length: 1 } as unknown as FileList;
      const event = createDragEvent('drop', {
        dataTransfer: { files: fileList, dropEffect: 'none' } as unknown as DataTransfer,
      });

      const dropZone = el.shadowRoot?.querySelector('.drop-zone');
      dropZone?.dispatchEvent(event);

      await new Promise(resolve => setTimeout(resolve, 50));
      await el.updateComplete;
    });

    it('should set error as AugmentedError', () => {
      expect(el.error).to.be.instanceOf(AugmentedError);
    });

    it('should preserve the original error message', () => {
      expect(el.error?.message).to.equal('Network error: upload failed');
    });

    it('should reset uploading to false', () => {
      expect(el.uploading).to.be.false;
    });

    it('should render error-display component', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.exist;
    });
  });

  describe('when drop event has no files', () => {
    let stubClient: StubClient;

    beforeEach(async () => {
      el = await fixture(html`
        <file-drop-zone allow-uploads></file-drop-zone>
      `);

      stubClient = createStubClient();
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- stub client matches the shape needed for testing
      el.setClient(stubClient as unknown as Parameters<FileDropZone['setClient']>[0]);

      const event = createDragEvent('drop', {
        dataTransfer: { files: { length: 0 }, dropEffect: 'none' } as unknown as DataTransfer,
      });

      const dropZone = el.shadowRoot?.querySelector('.drop-zone');
      dropZone?.dispatchEvent(event);

      await el.updateComplete;
    });

    it('should not call uploadFile', () => {
      expect(stubClient.uploadFile).to.not.have.been.called;
    });
  });

  describe('file-uploaded event properties', () => {
    let uploadedEvent: CustomEvent | null;

    beforeEach(async () => {
      el = await fixture(html`
        <file-drop-zone allow-uploads></file-drop-zone>
      `);

      const stubClient = createStubClient();
      stubClient.uploadFile.resolves({
        hash: 'sha256-test',
        uploadUrl: '/uploads/sha256-test?filename=doc.pdf',
      });
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- stub client matches the shape needed for testing
      el.setClient(stubClient as unknown as Parameters<FileDropZone['setClient']>[0]);

      uploadedEvent = null;
      // Listen on parent to test bubbles
      const wrapper = el.parentElement;
      wrapper?.addEventListener('file-uploaded', (e: Event) => {
        uploadedEvent = e as CustomEvent;
      });

      const file = createFile('doc.pdf', 100, 'application/pdf');
      const fileList = { 0: file, length: 1 } as unknown as FileList;
      const event = createDragEvent('drop', {
        dataTransfer: { files: fileList, dropEffect: 'none' } as unknown as DataTransfer,
      });

      const dropZone = el.shadowRoot?.querySelector('.drop-zone');
      dropZone?.dispatchEvent(event);

      await new Promise(resolve => setTimeout(resolve, 50));
      await el.updateComplete;
    });

    it('should bubble the event', () => {
      expect(uploadedEvent).to.not.be.null;
    });

    it('should have composed true', () => {
      expect(uploadedEvent?.composed).to.be.true;
    });
  });
});
