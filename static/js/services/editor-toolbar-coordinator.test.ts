import { expect, fixture, html } from '@open-wc/testing';
import type { SinonStub, SinonSpy } from 'sinon';
import { stub, spy } from 'sinon';
import '../web-components/editor-toolbar.js';
import '../web-components/insert-new-page-dialog.js';
import type { EditorToolbar } from '../web-components/editor-toolbar.js';
import type { InsertNewPageDialog } from '../web-components/insert-new-page-dialog.js';
import { EditorToolbarCoordinator } from './editor-toolbar-coordinator.js';
import { EditorUploadService } from './editor-upload-service.js';
import { TextFormattingService } from './text-formatting-service.js';

describe('EditorToolbarCoordinator', () => {
  let textarea: HTMLTextAreaElement;
  let toolbar: EditorToolbar;
  let coordinator: EditorToolbarCoordinator;
  let uploadService: EditorUploadService;
  let formattingService: TextFormattingService;

  beforeEach(async () => {
    textarea = await fixture<HTMLTextAreaElement>(html`<textarea>Hello world!</textarea>`);
    toolbar = await fixture<EditorToolbar>(html`<editor-toolbar></editor-toolbar>`);
    uploadService = new EditorUploadService();
    formattingService = new TextFormattingService();
    coordinator = new EditorToolbarCoordinator(textarea, toolbar, uploadService, formattingService);
  });

  afterEach(() => {
    coordinator.detach();
  });

  it('should exist', () => {
    expect(coordinator).to.exist;
  });

  describe('when Bold toolbar button is clicked', () => {
    let keyupSpy: SinonSpy;

    beforeEach(() => {
      keyupSpy = spy();
      textarea.addEventListener('keyup', keyupSpy);
      textarea.value = 'Hello world!';
      textarea.selectionStart = 6;
      textarea.selectionEnd = 11; // "world" selected

      // Simulate mousedown to save selection, then dispatch Bold event
      toolbar.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      toolbar.dispatchEvent(new CustomEvent('format-bold-requested', { bubbles: true }));
    });

    it('should wrap selection in bold markers', () => {
      expect(textarea.value).to.equal('Hello **world**!');
    });

    it('should trigger keyup for auto-save', () => {
      expect(keyupSpy).to.have.been.called;
    });
  });

  describe('when Italic toolbar button is clicked', () => {
    beforeEach(() => {
      textarea.value = 'Hello world!';
      textarea.selectionStart = 6;
      textarea.selectionEnd = 11;

      toolbar.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      toolbar.dispatchEvent(new CustomEvent('format-italic-requested', { bubbles: true }));
    });

    it('should wrap selection in italic markers', () => {
      expect(textarea.value).to.equal('Hello *world*!');
    });
  });

  describe('when Insert Link toolbar button is clicked', () => {
    beforeEach(() => {
      textarea.value = 'Click here!';
      textarea.selectionStart = 6;
      textarea.selectionEnd = 10; // "here" selected

      toolbar.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      toolbar.dispatchEvent(new CustomEvent('insert-link-requested', { bubbles: true }));
    });

    it('should wrap selection in link syntax', () => {
      expect(textarea.value).to.equal('Click [here](url)!');
    });
  });

  describe('when Upload Image toolbar button is clicked', () => {
    let selectAndUploadImageStub: SinonStub;

    beforeEach(async () => {
      selectAndUploadImageStub = stub(uploadService, 'selectAndUploadImage');
      selectAndUploadImageStub.resolves({
        markdownLink: '![test.png](/uploads/sha256-abc?filename=test.png)',
        filename: 'test.png',
      });

      textarea.value = 'Some text';
      textarea.selectionStart = 5;
      textarea.selectionEnd = 5;

      toolbar.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      toolbar.dispatchEvent(new CustomEvent('upload-image-requested', { bubbles: true }));
      await new Promise(resolve => setTimeout(resolve, 0));
    });

    afterEach(() => {
      selectAndUploadImageStub.restore();
    });

    it('should call selectAndUploadImage', () => {
      expect(selectAndUploadImageStub).to.have.been.calledOnce;
    });

    it('should insert markdown at cursor position', () => {
      expect(textarea.value).to.equal('Some ![test.png](/uploads/sha256-abc?filename=test.png)text');
    });
  });

  describe('when upload is cancelled', () => {
    let selectAndUploadImageStub: SinonStub;

    beforeEach(async () => {
      selectAndUploadImageStub = stub(uploadService, 'selectAndUploadImage');
      selectAndUploadImageStub.resolves(undefined); // Cancelled

      textarea.value = 'Original text';
      textarea.selectionStart = 5;
      textarea.selectionEnd = 5;

      toolbar.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      toolbar.dispatchEvent(new CustomEvent('upload-image-requested', { bubbles: true }));
      await new Promise(resolve => setTimeout(resolve, 0));
    });

    afterEach(() => {
      selectAndUploadImageStub.restore();
    });

    it('should not modify textarea', () => {
      expect(textarea.value).to.equal('Original text');
    });
  });

  describe('detach', () => {
    let boldSpy: SinonSpy;

    beforeEach(() => {
      boldSpy = spy();
      coordinator.detach();

      textarea.value = 'Hello world!';
      textarea.selectionStart = 6;
      textarea.selectionEnd = 11;
      textarea.addEventListener('keyup', boldSpy);

      toolbar.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      toolbar.dispatchEvent(new CustomEvent('format-bold-requested', { bubbles: true }));
    });

    it('should stop responding to toolbar events', () => {
      expect(boldSpy).to.not.have.been.called;
      expect(textarea.value).to.equal('Hello world!');
    });
  });

  describe('when Insert New Page toolbar button is clicked', () => {
    let insertedDialog: InsertNewPageDialog | null;

    beforeEach(async () => {
      textarea.value = 'Some existing text';
      textarea.selectionStart = 5;
      textarea.selectionEnd = 5;

      toolbar.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      toolbar.dispatchEvent(new CustomEvent('insert-new-page-requested', { bubbles: true }));

      // Wait for the dialog to be created
      await new Promise(resolve => setTimeout(resolve, 0));

      insertedDialog = document.body.querySelector('insert-new-page-dialog');
    });

    afterEach(() => {
      if (insertedDialog) {
        insertedDialog.remove();
      }
    });

    it('should create the dialog element', () => {
      expect(insertedDialog).to.exist;
    });

    it('should have openDialog as a function', () => {
      expect(insertedDialog?.openDialog).to.be.a('function');
    });

    it('should append dialog to document body', () => {
      expect(insertedDialog?.parentElement).to.equal(document.body);
    });
  });

  describe('when textarea is inside a shadow DOM', () => {
    let shadowHost: HTMLDivElement;
    let shadowTextarea: HTMLTextAreaElement;
    let shadowCoordinator: EditorToolbarCoordinator;

    beforeEach(() => {
      shadowHost = document.createElement('div');
      document.body.appendChild(shadowHost);
      const shadowRoot = shadowHost.attachShadow({ mode: 'open' });
      shadowTextarea = document.createElement('textarea');
      shadowTextarea.value = 'Shadow content';
      shadowRoot.appendChild(shadowTextarea);

      shadowCoordinator = new EditorToolbarCoordinator(
        shadowTextarea, toolbar, uploadService, formattingService
      );

      shadowTextarea.selectionStart = 7;
      shadowTextarea.selectionEnd = 14; // "content" selected

      // Simulate toolbar interaction
      toolbar.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      toolbar.dispatchEvent(new CustomEvent('format-bold-requested', { bubbles: true }));
    });

    afterEach(() => {
      shadowCoordinator.detach();
      shadowHost.remove();
    });

    it('should apply formatting to the shadow DOM textarea', () => {
      expect(shadowTextarea.value).to.equal('Shadow **content**');
    });
  });

  describe('when page-created event is dispatched', () => {
    let insertedDialog: InsertNewPageDialog | null;

    beforeEach(async () => {
      textarea.value = 'Some existing text';
      textarea.selectionStart = 5;
      textarea.selectionEnd = 5;

      toolbar.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      toolbar.dispatchEvent(new CustomEvent('insert-new-page-requested', { bubbles: true }));
      await new Promise(resolve => setTimeout(resolve, 0));

      insertedDialog = document.body.querySelector('insert-new-page-dialog');

      // Dispatch page-created event from the dialog
      insertedDialog?.dispatchEvent(new CustomEvent('page-created', {
        bubbles: true,
        composed: true,
        detail: {
          identifier: 'my_new_page',
          title: 'My New Page',
          markdownLink: '[My New Page](/my_new_page)',
        },
      }));
    });

    afterEach(() => {
      if (insertedDialog) {
        insertedDialog.remove();
      }
    });

    it('should insert the markdown link at cursor position', () => {
      expect(textarea.value).to.equal('Some [My New Page](/my_new_page)existing text');
    });
  });
});
