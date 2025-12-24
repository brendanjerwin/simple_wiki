import { expect, fixture, html } from '@open-wc/testing';
import { stub, spy, SinonStub, SinonSpy, useFakeTimers, SinonFakeTimers } from 'sinon';
import '../web-components/editor-context-menu.js';
import type { EditorContextMenu } from '../web-components/editor-context-menu.js';
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

  describe('when long-pressing on textarea', () => {
    let openAtSpy: SinonSpy;
    let clock: SinonFakeTimers;

    beforeEach(async () => {
      clock = useFakeTimers();
      openAtSpy = spy(menu, 'openAt');
    });

    afterEach(() => {
      openAtSpy.restore();
      clock.restore();
    });

    describe('when holding for 500ms without movement', () => {
      beforeEach(async () => {
        const event = new PointerEvent('pointerdown', {
          bubbles: true,
          clientX: 100,
          clientY: 200,
        });
        textarea.dispatchEvent(event);
        await clock.tickAsync(500);
        await menu.updateComplete;
      });

      it('should open the menu', () => {
        expect(openAtSpy).to.have.been.calledOnce;
        expect(openAtSpy).to.have.been.calledWith({ x: 100, y: 200 });
      });
    });

    describe('when releasing before 500ms', () => {
      beforeEach(async () => {
        textarea.dispatchEvent(new PointerEvent('pointerdown', {
          bubbles: true,
          clientX: 100,
          clientY: 200,
        }));
        await clock.tickAsync(300);
        textarea.dispatchEvent(new PointerEvent('pointerup', { bubbles: true }));
        await clock.tickAsync(300);
      });

      it('should not open the menu', () => {
        expect(openAtSpy).to.not.have.been.called;
      });
    });

    describe('when moving more than 10px', () => {
      beforeEach(async () => {
        textarea.dispatchEvent(new PointerEvent('pointerdown', {
          bubbles: true,
          clientX: 100,
          clientY: 200,
        }));
        await clock.tickAsync(200);
        textarea.dispatchEvent(new PointerEvent('pointermove', {
          bubbles: true,
          clientX: 115, // 15px movement
          clientY: 200,
        }));
        await clock.tickAsync(400);
      });

      it('should not open the menu (user is selecting text)', () => {
        expect(openAtSpy).to.not.have.been.called;
      });
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
});
