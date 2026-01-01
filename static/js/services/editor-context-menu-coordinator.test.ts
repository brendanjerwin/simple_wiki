import { expect, fixture, html } from '@open-wc/testing';
import type { SinonStub, SinonSpy } from 'sinon';
import { stub, spy } from 'sinon';
import '../web-components/editor-context-menu.js';
import '../web-components/insert-new-page-dialog.js';
import type { EditorContextMenu } from '../web-components/editor-context-menu.js';
import type { InsertNewPageDialog } from '../web-components/insert-new-page-dialog.js';
import { EditorContextMenuCoordinator } from './editor-context-menu-coordinator.js';
import { EditorUploadService } from './editor-upload-service.js';
import { TextFormattingService } from './text-formatting-service.js';

describe('EditorContextMenuCoordinator', () => {
  let textarea: HTMLTextAreaElement;
  let menu: EditorContextMenu;
  let coordinator: EditorContextMenuCoordinator;
  let uploadService: EditorUploadService;
  let formattingService: TextFormattingService;

  beforeEach(async () => {
    textarea = await fixture<HTMLTextAreaElement>(html`<textarea>Hello world!</textarea>`);
    menu = await fixture<EditorContextMenu>(html`<editor-context-menu></editor-context-menu>`);
    uploadService = new EditorUploadService();
    formattingService = new TextFormattingService();
    coordinator = new EditorContextMenuCoordinator(textarea, menu, uploadService, formattingService);
  });

  afterEach(() => {
    coordinator.detach();
  });

  it('should exist', () => {
    expect(coordinator).to.exist;
  });

  describe('when right-clicking on textarea', () => {
    let openAtSpy: SinonSpy;

    beforeEach(async () => {
      openAtSpy = spy(menu, 'openAt');
      const event = new MouseEvent('contextmenu', {
        bubbles: true,
        clientX: 150,
        clientY: 250,
      });
      textarea.dispatchEvent(event);
      await menu.updateComplete;
    });

    afterEach(() => {
      openAtSpy.restore();
    });

    it('should open the menu at click position', () => {
      expect(openAtSpy).to.have.been.calledOnce;
      expect(openAtSpy).to.have.been.calledWith({ x: 150, y: 250 });
    });

    it('should prevent default context menu', () => {
      // Menu should be open, indicating default was prevented
      expect(menu.open).to.be.true;
    });
  });

  describe('when Bold menu item is clicked', () => {
    let keyupSpy: SinonSpy;

    beforeEach(async () => {
      keyupSpy = spy();
      textarea.addEventListener('keyup', keyupSpy);
      textarea.value = 'Hello world!';
      textarea.selectionStart = 6;
      textarea.selectionEnd = 11; // "world" selected

      // Simulate right-click to save selection
      textarea.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true,
        clientX: 100,
        clientY: 200,
      }));
      await menu.updateComplete;

      // Simulate clicking Bold
      menu.dispatchEvent(new CustomEvent('format-bold-requested', { bubbles: true }));
    });

    it('should wrap selection in bold markers', () => {
      expect(textarea.value).to.equal('Hello **world**!');
    });

    it('should trigger keyup for auto-save', () => {
      expect(keyupSpy).to.have.been.called;
    });
  });

  describe('when Italic menu item is clicked', () => {
    beforeEach(async () => {
      textarea.value = 'Hello world!';
      textarea.selectionStart = 6;
      textarea.selectionEnd = 11;

      textarea.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true,
        clientX: 100,
        clientY: 200,
      }));
      await menu.updateComplete;

      menu.dispatchEvent(new CustomEvent('format-italic-requested', { bubbles: true }));
    });

    it('should wrap selection in italic markers', () => {
      expect(textarea.value).to.equal('Hello *world*!');
    });
  });

  describe('when Insert Link menu item is clicked', () => {
    beforeEach(async () => {
      textarea.value = 'Click here!';
      textarea.selectionStart = 6;
      textarea.selectionEnd = 10; // "here" selected

      textarea.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true,
        clientX: 100,
        clientY: 200,
      }));
      await menu.updateComplete;

      menu.dispatchEvent(new CustomEvent('insert-link-requested', { bubbles: true }));
    });

    it('should wrap selection in link syntax', () => {
      expect(textarea.value).to.equal('Click [here](url)!');
    });
  });

  describe('when Upload Image menu item is clicked', () => {
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

      textarea.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true,
        clientX: 100,
        clientY: 200,
      }));
      await menu.updateComplete;

      menu.dispatchEvent(new CustomEvent('upload-image-requested', { bubbles: true }));
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

      textarea.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true,
        clientX: 100,
        clientY: 200,
      }));
      await menu.updateComplete;

      menu.dispatchEvent(new CustomEvent('upload-image-requested', { bubbles: true }));
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
    let contextMenuSpy: SinonSpy;

    beforeEach(() => {
      contextMenuSpy = spy(menu, 'openAt');
      coordinator.detach();
    });

    afterEach(() => {
      contextMenuSpy.restore();
    });

    it('should stop responding to context menu events', () => {
      textarea.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true,
        clientX: 100,
        clientY: 200,
      }));
      expect(contextMenuSpy).to.not.have.been.called;
    });
  });

  describe('when Insert New Page menu item is clicked', () => {
    let insertedDialog: InsertNewPageDialog | null;

    beforeEach(async () => {
      textarea.value = 'Some existing text';
      textarea.selectionStart = 5;
      textarea.selectionEnd = 5;

      // Open context menu first
      textarea.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true,
        clientX: 100,
        clientY: 200,
      }));
      await menu.updateComplete;

      // Dispatch the insert-new-page-requested event
      menu.dispatchEvent(new CustomEvent('insert-new-page-requested', { bubbles: true }));

      // Wait for the dialog to be created
      await new Promise(resolve => setTimeout(resolve, 0));

      // Find the dialog that was inserted
      insertedDialog = document.body.querySelector('insert-new-page-dialog');
    });

    afterEach(() => {
      // Clean up the dialog from the DOM
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

  describe('when page-created event is dispatched', () => {
    let insertedDialog: InsertNewPageDialog | null;

    beforeEach(async () => {
      textarea.value = 'Some existing text';
      textarea.selectionStart = 5;
      textarea.selectionEnd = 5;

      // Open context menu and trigger insert new page
      textarea.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true,
        clientX: 100,
        clientY: 200,
      }));
      await menu.updateComplete;

      menu.dispatchEvent(new CustomEvent('insert-new-page-requested', { bubbles: true }));
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
