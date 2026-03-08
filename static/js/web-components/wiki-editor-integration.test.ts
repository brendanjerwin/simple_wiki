import { expect, waitUntil } from '@open-wc/testing';
import sinon, { type SinonStub } from 'sinon';
import { create } from '@bufbuild/protobuf';
import {
  ReadPageResponseSchema,
  UpdateWholePageResponseSchema,
} from '../gen/api/v1/page_management_pb.js';
import type { WikiEditor } from './wiki-editor.js';
import type { EditorToolbar } from './editor-toolbar.js';
import type { EditorContextMenu } from './editor-context-menu.js';
import type { SaveStatus } from '../services/editor-save-queue.js';
import type { AugmentedError } from './augment-error-service.js';

// Must import to register custom elements
import './wiki-editor.js';
import './editor-toolbar.js';
import './editor-context-menu.js';

interface WikiEditorInternal {
  loading: boolean;
  saveStatus: SaveStatus;
  error: AugmentedError | null;
  content: string;
  versionHash: string;
  saveQueue: { destroy: () => void } | null;
  coordinator: { detach: () => void } | null;
  client: {
    readPage: SinonStub;
    updateWholePage: SinonStub;
  };
}

/**
 * Integration tests for wiki-editor with editor-toolbar and editor-context-menu.
 *
 * These test the full chain: toolbar/menu button click → coordinator → textarea modification.
 * The toolbar and context menu are placed in the document (light DOM) as they are in production.
 */
describe('WikiEditor integration with toolbar and context menu', () => {
  let wikiEditor: WikiEditor;
  let toolbar: EditorToolbar;
  let contextMenu: EditorContextMenu;

  function stubGrpc(): void {
    const internal = wikiEditor as unknown as WikiEditorInternal;
    sinon.stub(internal.client, 'readPage').resolves(
      create(ReadPageResponseSchema, {
        contentMarkdown: 'Hello world',
        frontMatterToml: '',
        versionHash: 'hash1',
        renderedContentHtml: '',
        renderedContentMarkdown: '',
      })
    );
    sinon.stub(internal.client, 'updateWholePage').resolves(
      create(UpdateWholePageResponseSchema, { success: true, error: '' })
    );
  }

  function getTextarea(): HTMLTextAreaElement {
    const textarea = wikiEditor.shadowRoot?.querySelector('textarea');
    if (!textarea) throw new Error('textarea not found in shadow DOM');
    return textarea;
  }

  function selectText(textarea: HTMLTextAreaElement, start: number, end: number): void {
    textarea.focus();
    textarea.selectionStart = start;
    textarea.selectionEnd = end;
  }

  function clickToolbarButton(action: string): void {
    const button = toolbar.shadowRoot?.querySelector(`[data-action="${action}"]`);
    if (!button) throw new Error(`toolbar button [data-action="${action}"] not found`);

    // Simulate the real browser event sequence:
    // mousedown (saves selection) → click (dispatches action event)
    button.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, composed: true }));
    button.dispatchEvent(new MouseEvent('click', { bubbles: true, composed: true }));
  }

  beforeEach(async () => {
    // Create toolbar and context menu in light DOM (as in production template)
    toolbar = document.createElement('editor-toolbar');
    document.body.appendChild(toolbar);
    await toolbar.updateComplete;

    contextMenu = document.createElement('editor-context-menu');
    document.body.appendChild(contextMenu);
    await contextMenu.updateComplete;

    // Create wiki-editor, stub gRPC, then mount
    wikiEditor = document.createElement('wiki-editor') as WikiEditor;
    wikiEditor.setAttribute('page', 'integration-test');
    wikiEditor.setAttribute('debounce-ms', '500');
    stubGrpc();

    document.body.appendChild(wikiEditor);
    await waitUntil(
      () => !(wikiEditor as unknown as WikiEditorInternal).loading,
      'should finish loading',
      { timeout: 3000 }
    );
    await wikiEditor.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
    wikiEditor.remove();
    toolbar.remove();
    contextMenu.remove();
  });

  it('should set up the coordinator', () => {
    expect((wikiEditor as unknown as WikiEditorInternal).coordinator).to.not.be.null;
  });

  it('should have a textarea with the loaded content', () => {
    const textarea = getTextarea();
    expect(textarea.value).to.equal('Hello world');
  });

  describe('when Bold toolbar button is clicked with text selected', () => {
    beforeEach(async () => {
      const textarea = getTextarea();
      selectText(textarea, 6, 11); // select "world"
      clickToolbarButton('bold');
      await wikiEditor.updateComplete;
    });

    it('should wrap the selected text in bold markers', () => {
      expect(getTextarea().value).to.equal('Hello **world**');
    });
  });

  describe('when Italic toolbar button is clicked with text selected', () => {
    beforeEach(async () => {
      const textarea = getTextarea();
      selectText(textarea, 6, 11); // select "world"
      clickToolbarButton('italic');
      await wikiEditor.updateComplete;
    });

    it('should wrap the selected text in italic markers', () => {
      expect(getTextarea().value).to.equal('Hello *world*');
    });
  });

  describe('when Link toolbar button is clicked with text selected', () => {
    beforeEach(async () => {
      const textarea = getTextarea();
      selectText(textarea, 6, 11); // select "world"
      clickToolbarButton('link');
      await wikiEditor.updateComplete;
    });

    it('should wrap the selected text in link syntax', () => {
      expect(getTextarea().value).to.equal('Hello [world](url)');
    });
  });

  describe('when Bold toolbar button is clicked with no selection', () => {
    beforeEach(async () => {
      const textarea = getTextarea();
      selectText(textarea, 5, 5); // cursor at position 5, no selection
      clickToolbarButton('bold');
      await wikiEditor.updateComplete;
    });

    it('should insert bold placeholder at cursor', () => {
      expect(getTextarea().value).to.equal('Hello**bold** world');
    });
  });

  describe('when right-clicking textarea and selecting Bold', () => {
    beforeEach(async () => {
      const textarea = getTextarea();
      selectText(textarea, 6, 11); // select "world"

      // Simulate right-click
      textarea.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true,
        composed: true,
        clientX: 100,
        clientY: 200,
      }));
      await contextMenu.updateComplete;

      // Simulate clicking Bold in the context menu
      contextMenu.dispatchEvent(
        new CustomEvent('format-bold-requested', { bubbles: true, composed: true })
      );
      await wikiEditor.updateComplete;
    });

    it('should wrap the selected text in bold markers', () => {
      expect(getTextarea().value).to.equal('Hello **world**');
    });
  });

  describe('when Done toolbar button is clicked', () => {
    let exitHandler: SinonStub;

    beforeEach(async () => {
      exitHandler = sinon.stub();
      toolbar.addEventListener('exit-requested', exitHandler);
      clickToolbarButton('exit');
      await toolbar.updateComplete;
    });

    it('should dispatch exit-requested event', () => {
      expect(exitHandler).to.have.been.calledOnce;
    });
  });

  describe('when formatting is applied', () => {
    beforeEach(async () => {
      const textarea = getTextarea();
      selectText(textarea, 6, 11);
      clickToolbarButton('bold');
      await wikiEditor.updateComplete;
    });

    it('should sync the content to the component state', () => {
      expect((wikiEditor as unknown as WikiEditorInternal).content).to.equal('Hello **world**');
    });
  });
});
