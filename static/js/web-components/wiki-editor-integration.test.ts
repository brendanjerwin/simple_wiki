import { expect, waitUntil } from '@open-wc/testing';
import sinon, { type SinonStub } from 'sinon';
import { create } from '@bufbuild/protobuf';
import {
  ReadPageResponseSchema,
  UpdateWholePageResponseSchema,
} from '../gen/api/v1/page_management_pb.js';
import type { WikiEditor } from './wiki-editor.js';
import type { EditorToolbar } from './editor-toolbar.js';
import type { SaveStatus } from '../services/editor-save-queue.js';
import type { AugmentedError } from './augment-error-service.js';

// Must import to register custom elements
import './wiki-editor.js';

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

describe('WikiEditor integration with toolbar', () => {
  let wikiEditor: WikiEditor;

  function getTextarea(): HTMLTextAreaElement {
    const textarea = wikiEditor.shadowRoot?.querySelector('textarea');
    if (!textarea) throw new Error('textarea not found in shadow DOM');
    return textarea;
  }

  function getToolbar(): EditorToolbar {
    const toolbar = wikiEditor.shadowRoot?.querySelector<EditorToolbar>('editor-toolbar');
    if (!toolbar) throw new Error('editor-toolbar not found in shadow DOM');
    return toolbar;
  }

  function selectText(textarea: HTMLTextAreaElement, start: number, end: number): void {
    textarea.focus();
    textarea.selectionStart = start;
    textarea.selectionEnd = end;
  }

  function clickToolbarButton(action: string): void {
    const toolbar = getToolbar();
    const button = toolbar.shadowRoot?.querySelector(`[data-action="${action}"]`);
    if (!button) throw new Error(`toolbar button [data-action="${action}"] not found`);

    toolbar.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    button.dispatchEvent(new MouseEvent('click', { bubbles: true, composed: true }));
  }

  // Single shared instance — avoids browser crash from repeated create/destroy
  before(async () => {
    wikiEditor = document.createElement('wiki-editor') as WikiEditor;
    wikiEditor.setAttribute('page', 'integration-test');
    wikiEditor.setAttribute('debounce-ms', '500');

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

    document.body.appendChild(wikiEditor);
    await waitUntil(
      () => !(wikiEditor as unknown as WikiEditorInternal).loading,
      'should finish loading',
      { timeout: 3000 }
    );
    await wikiEditor.updateComplete;
  });

  after(() => {
    wikiEditor.remove();
    sinon.restore();
  });

  // Reset textarea content between tests
  beforeEach(async () => {
    const textarea = getTextarea();
    textarea.value = 'Hello world';
    (wikiEditor as unknown as WikiEditorInternal).content = 'Hello world';
    await wikiEditor.updateComplete;
  });

  it('should set up the coordinator and render toolbar', () => {
    expect((wikiEditor as unknown as WikiEditorInternal).coordinator).to.not.be.null;
    expect(getToolbar()).to.exist;
    expect(getTextarea().value).to.equal('Hello world');
  });

  it('should wrap selected text in bold markers and sync state', async () => {
    selectText(getTextarea(), 6, 11);
    clickToolbarButton('bold');
    await wikiEditor.updateComplete;

    expect(getTextarea().value).to.equal('Hello **world**');
    expect((wikiEditor as unknown as WikiEditorInternal).content).to.equal('Hello **world**');
  });

  it('should wrap selected text in italic markers', async () => {
    selectText(getTextarea(), 6, 11);
    clickToolbarButton('italic');
    await wikiEditor.updateComplete;

    expect(getTextarea().value).to.equal('Hello *world*');
  });

  it('should wrap selected text in link syntax', async () => {
    selectText(getTextarea(), 6, 11);
    clickToolbarButton('link');
    await wikiEditor.updateComplete;

    expect(getTextarea().value).to.equal('Hello [world](url)');
  });

  it('should insert bold placeholder at cursor when no text selected', async () => {
    selectText(getTextarea(), 5, 5);
    clickToolbarButton('bold');
    await wikiEditor.updateComplete;

    expect(getTextarea().value).to.equal('Hello**bold** world');
  });
});
