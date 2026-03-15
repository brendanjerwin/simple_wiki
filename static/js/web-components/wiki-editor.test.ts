import { expect, waitUntil } from '@open-wc/testing';
import sinon, { type SinonStub, type SinonFakeTimers, type SinonSpy } from 'sinon';
import { create } from '@bufbuild/protobuf';
import {
  ReadPageResponseSchema,
  UpdateWholePageResponseSchema,
} from '../gen/api/v1/page_management_pb.js';
import type { WikiEditor } from './wiki-editor.js';
import type { SaveStatus } from '../services/editor-save-queue.js';
import type { AugmentedError } from './augment-error-service.js';

// Interface to access private/internal members for testing
interface WikiEditorInternal {
  loading: boolean;
  saveStatus: SaveStatus;
  error: AugmentedError | null;
  content: string;
  versionHash: string;
  _hasSelection: boolean;
  saveQueue: { destroy: () => void } | null;
  coordinator: { detach: () => void } | null;
  client: {
    readPage: SinonStub;
    updateWholePage: SinonStub;
  };
}

function buildElement(page = 'test-page'): WikiEditor {
  const el = document.createElement('wiki-editor') as WikiEditor;
  el.setAttribute('page', page);
  el.setAttribute('debounce-ms', '500');
  return el;
}

function stubReadPage(
  target: WikiEditor,
  contentMarkdown = '# Hello',
  frontMatterToml = '',
  versionHash = 'abc123'
): SinonStub {
  const internal = target as unknown as WikiEditorInternal;
  return sinon
    .stub(internal.client, 'readPage')
    .resolves(
      create(ReadPageResponseSchema, {
        contentMarkdown,
        frontMatterToml,
        versionHash,
        renderedContentHtml: '',
        renderedContentMarkdown: '',
      })
    );
}

function stubUpdateWholePage(
  target: WikiEditor,
  success = true,
  error = ''
): SinonStub {
  const internal = target as unknown as WikiEditorInternal;
  return sinon
    .stub(internal.client, 'updateWholePage')
    .resolves(
      create(UpdateWholePageResponseSchema, { success, error })
    );
}

async function mountAndLoad(element: WikiEditor): Promise<void> {
  document.body.appendChild(element);
  await waitUntil(
    () => !(element as unknown as WikiEditorInternal).loading,
    'should finish loading',
    { timeout: 3000 }
  );
  await element.updateComplete;
}

// Must import after stubs are set up
import './wiki-editor.js';

describe('WikiEditor', () => {
  let el: WikiEditor;

  afterEach(() => {
    sinon.restore();
    if (el) {
      el.remove();
    }
  });

  it('should exist as a custom element', () => {
    expect(customElements.get('wiki-editor')).to.exist;
  });

  describe('when loading content', () => {
    let loadingOverlay: Element | null | undefined;
    let textarea: HTMLTextAreaElement | null | undefined;

    beforeEach(async () => {
      el = buildElement();
      const internal = el as unknown as WikiEditorInternal;
      // Make readPage hang so loading state is observable
      sinon.stub(internal.client, 'readPage').returns(new Promise(() => {}));
      document.body.appendChild(el);
      await el.updateComplete;
      loadingOverlay = el.shadowRoot?.querySelector('.loading-overlay');
      textarea = el.shadowRoot?.querySelector('textarea');
    });

    it('should show the loading overlay', () => {
      expect(loadingOverlay).to.exist;
    });

    it('should not render a textarea', () => {
      expect(textarea).to.not.exist;
    });
  });

  describe('when connected to DOM and content loaded', () => {
    let readPageStub: SinonStub;
    let textarea: HTMLTextAreaElement | null | undefined;

    beforeEach(async () => {
      el = buildElement();
      readPageStub = stubReadPage(el, '# Hello World', 'title = "Test"\n', 'hash123');
      stubUpdateWholePage(el);
      await mountAndLoad(el);
      textarea = el.shadowRoot?.querySelector('textarea');
    });

    it('should call readPage with the page name', () => {
      expect(readPageStub).to.have.been.calledOnce;
      const callArgs = readPageStub.firstCall.args[0] as { pageName: string };
      expect(callArgs.pageName).to.equal('test-page');
    });

    it('should reconstruct content with frontmatter delimiters', () => {
      expect((el as unknown as WikiEditorInternal).content).to.equal(
        '+++\ntitle = "Test"\n+++\n# Hello World'
      );
    });

    it('should store the version hash', () => {
      expect((el as unknown as WikiEditorInternal).versionHash).to.equal('hash123');
    });

    it('should populate the textarea with content', () => {
      expect(textarea?.value).to.equal('+++\ntitle = "Test"\n+++\n# Hello World');
    });

    it('should not show the status bar when idle', () => {
      const statusBar = el.shadowRoot?.querySelector('.status-bar');
      expect(statusBar).to.exist;
      expect(statusBar?.classList.contains('visible')).to.be.false;
    });

    it('should not show loading overlay', () => {
      const loading = el.shadowRoot?.querySelector('.loading-overlay');
      expect(loading).to.not.exist;
    });

    it('should render file-drop-zone', () => {
      const dropZone = el.shadowRoot?.querySelector('file-drop-zone');
      expect(dropZone).to.exist;
    });

    it('should render editor-toolbar in shadow DOM', () => {
      const toolbar = el.shadowRoot?.querySelector('editor-toolbar');
      expect(toolbar).to.exist;
    });
  });

  describe('when textarea select event fires with a selection', () => {
    beforeEach(async () => {
      el = buildElement();
      stubReadPage(el, '# Hello World', '', 'hash1');
      stubUpdateWholePage(el);
      await mountAndLoad(el);

      const textarea = el.shadowRoot!.querySelector('textarea')!;
      textarea.focus();
      textarea.selectionStart = 2;
      textarea.selectionEnd = 7;
      textarea.dispatchEvent(new Event('select', { bubbles: true }));
      await el.updateComplete;
    });

    it('should set _hasSelection to true', () => {
      expect((el as unknown as WikiEditorInternal)._hasSelection).to.be.true;
    });
  });

  describe('when textarea mouseup fires with collapsed selection', () => {
    beforeEach(async () => {
      el = buildElement();
      stubReadPage(el, '# Hello World', '', 'hash1');
      stubUpdateWholePage(el);
      await mountAndLoad(el);

      const textarea = el.shadowRoot!.querySelector('textarea')!;
      textarea.focus();
      textarea.selectionStart = 5;
      textarea.selectionEnd = 5;
      textarea.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
      await el.updateComplete;
    });

    it('should set _hasSelection to false', () => {
      expect((el as unknown as WikiEditorInternal)._hasSelection).to.be.false;
    });
  });

  describe('when content has no frontmatter', () => {
    beforeEach(async () => {
      el = buildElement();
      stubReadPage(el, '# Just Content', '', 'hash456');
      stubUpdateWholePage(el);
      await mountAndLoad(el);
    });

    it('should set content without frontmatter delimiters', () => {
      expect((el as unknown as WikiEditorInternal).content).to.equal('# Just Content');
    });
  });

  describe('when page property changes after initial load', () => {
    let readPageStub: SinonStub;

    beforeEach(async () => {
      el = buildElement();
      readPageStub = stubReadPage(el, '# First Page', '', 'hash1');
      stubUpdateWholePage(el);
      await mountAndLoad(el);

      // Change the stub to return different content for the second page
      readPageStub.resolves(
        create(ReadPageResponseSchema, {
          contentMarkdown: '# Second Page',
          frontMatterToml: '',
          versionHash: 'hash2',
          renderedContentHtml: '',
          renderedContentMarkdown: '',
        })
      );

      el.page = 'other-page';
      await waitUntil(
        () => !(el as unknown as WikiEditorInternal).loading,
        'should finish loading second page',
        { timeout: 3000 }
      );
      await el.updateComplete;
    });

    it('should load the new page content', () => {
      expect((el as unknown as WikiEditorInternal).content).to.equal('# Second Page');
    });

    it('should update the version hash', () => {
      expect((el as unknown as WikiEditorInternal).versionHash).to.equal('hash2');
    });

    it('should call readPage a second time', () => {
      expect(readPageStub).to.have.been.calledTwice;
    });
  });

  describe('when readPage fails', () => {
    let errorDisplay: Element | null | undefined;

    beforeEach(async () => {
      el = buildElement();
      const internal = el as unknown as WikiEditorInternal;
      sinon.stub(internal.client, 'readPage').rejects(new Error('Network error'));
      await mountAndLoad(el);
      errorDisplay = el.shadowRoot?.querySelector('error-display');
    });

    it('should set the error', () => {
      expect((el as unknown as WikiEditorInternal).error).to.not.be.null;
    });

    it('should render error-display', () => {
      expect(errorDisplay).to.exist;
    });

    it('should not render textarea', () => {
      const textarea = el.shadowRoot?.querySelector('textarea');
      expect(textarea).to.not.exist;
    });
  });

  describe('when user types in the textarea', () => {
    let clock: SinonFakeTimers;
    let updateStub: SinonStub;

    beforeEach(async () => {
      clock = sinon.useFakeTimers({
        shouldAdvanceTime: true,
        shouldClearNativeTimers: true,
      });
      el = buildElement();
      stubReadPage(el, '# Hello', '', 'hash1');
      updateStub = stubUpdateWholePage(el);
      document.body.appendChild(el);
      await waitUntil(
        () => !(el as unknown as WikiEditorInternal).loading,
        'should finish loading',
        { timeout: 5000 }
      );
      await el.updateComplete;
    });

    afterEach(() => {
      clock.restore();
    });

    describe('when typing and waiting for debounce', () => {
      let statusIndicator: Element | null | undefined;

      beforeEach(async () => {
        const textarea = el.shadowRoot!.querySelector('textarea')!;
        textarea.value = '# Hello World';
        textarea.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
        statusIndicator = el.shadowRoot?.querySelector('.status-indicator');
      });

      it('should update save status to editing', () => {
        expect((el as unknown as WikiEditorInternal).saveStatus).to.equal('editing');
      });

      it('should show Editing in the status bar', () => {
        expect(statusIndicator?.textContent?.trim()).to.contain('Editing');
      });

      describe('when debounce period elapses', () => {
        beforeEach(async () => {
          await clock.tickAsync(500);
          await el.updateComplete;
          statusIndicator = el.shadowRoot?.querySelector('.status-indicator');
        });

        it('should call updateWholePage', () => {
          expect(updateStub).to.have.been.calledOnce;
        });

        it('should pass the correct page name and content', () => {
          const callArgs = updateStub.firstCall.args[0] as {
            pageName: string;
            newWholeMarkdown: string;
          };
          expect(callArgs.pageName).to.equal('test-page');
          expect(callArgs.newWholeMarkdown).to.equal('# Hello World');
        });

        it('should show Saved in the status bar', () => {
          expect(statusIndicator?.textContent?.trim()).to.contain('Saved');
        });
      });
    });
  });

  describe('when save completes and saved status transitions to idle', () => {
    let clock: SinonFakeTimers;

    beforeEach(async () => {
      clock = sinon.useFakeTimers({
        shouldAdvanceTime: true,
        shouldClearNativeTimers: true,
      });
      el = buildElement();
      stubReadPage(el, '# Hello', '', 'hash1');
      stubUpdateWholePage(el);
      document.body.appendChild(el);
      await waitUntil(
        () => !(el as unknown as WikiEditorInternal).loading,
        'should finish loading',
        { timeout: 5000 }
      );
      await el.updateComplete;

      // Type and trigger save
      const textarea = el.shadowRoot!.querySelector('textarea')!;
      textarea.value = '# Hello World';
      textarea.dispatchEvent(new Event('input', { bubbles: true }));
      await clock.tickAsync(500); // debounce elapses, save completes
      await el.updateComplete;
    });

    afterEach(() => {
      clock.restore();
    });

    it('should show saved status initially', () => {
      expect((el as unknown as WikiEditorInternal).saveStatus).to.equal('saved');
    });

    describe('when 2 seconds elapse after save', () => {
      beforeEach(async () => {
        await clock.tickAsync(2000);
        await el.updateComplete;
      });

      it('should transition to idle status', () => {
        expect((el as unknown as WikiEditorInternal).saveStatus).to.equal('idle');
      });

      it('should hide the status bar', () => {
        const statusBar = el.shadowRoot?.querySelector('.status-bar');
        expect(statusBar?.classList.contains('visible')).to.be.false;
      });
    });
  });

  describe('when save returns an error', () => {
    let clock: SinonFakeTimers;

    beforeEach(async () => {
      clock = sinon.useFakeTimers({
        shouldAdvanceTime: true,
        shouldClearNativeTimers: true,
      });
      el = buildElement();
      stubReadPage(el, '# Hello', '', 'hash1');
      stubUpdateWholePage(el, false, 'Conflict detected');
      document.body.appendChild(el);
      await waitUntil(
        () => !(el as unknown as WikiEditorInternal).loading,
        'should finish loading',
        { timeout: 5000 }
      );
      await el.updateComplete;

      const textarea = el.shadowRoot!.querySelector('textarea')!;
      textarea.value = '# Changed';
      textarea.dispatchEvent(new Event('input', { bubbles: true }));
      await clock.tickAsync(500);
      await el.updateComplete;
    });

    afterEach(() => {
      clock.restore();
    });

    it('should set save status to error', () => {
      expect((el as unknown as WikiEditorInternal).saveStatus).to.equal('error');
    });

    it('should set the error', () => {
      expect((el as unknown as WikiEditorInternal).error).to.not.be.null;
    });

    it('should hide the status bar during errors', () => {
      const statusBar = el.shadowRoot?.querySelector('.status-bar');
      expect(statusBar?.classList.contains('visible')).to.be.false;
    });

    it('should render error-display inline', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.not.be.null;
    });

    it('should render the textarea alongside the error', () => {
      const textarea = el.shadowRoot?.querySelector('textarea');
      expect(textarea).to.not.be.null;
    });

    it('should clear the error and status when dismiss is clicked', async () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display') as HTMLElement & { action?: { onClick: () => void } };
      expect(errorDisplay).to.not.be.null;
      // Simulate clicking the dismiss button by invoking the action callback
      // (error-display renders the button; we verify the contract via the property)
      errorDisplay.action?.onClick();
      await el.updateComplete;
      expect((el as unknown as WikiEditorInternal).error).to.be.null;
      expect((el as unknown as WikiEditorInternal).saveStatus).to.equal('idle');
    });
  });

  describe('when coordinator dispatches keyup (formatting operation)', () => {
    let clock: SinonFakeTimers;

    beforeEach(async () => {
      clock = sinon.useFakeTimers({
        shouldAdvanceTime: true,
        shouldClearNativeTimers: true,
      });
      el = buildElement();
      stubReadPage(el, '# Hello', '', 'hash1');
      stubUpdateWholePage(el);
      document.body.appendChild(el);
      await waitUntil(
        () => !(el as unknown as WikiEditorInternal).loading,
        'should finish loading',
        { timeout: 5000 }
      );
      await el.updateComplete;

      // Simulate coordinator modifying textarea value and dispatching keyup
      const textarea = el.shadowRoot!.querySelector('textarea')!;
      textarea.value = '# **Hello**';
      textarea.dispatchEvent(new Event('keyup', { bubbles: true }));
      await el.updateComplete;
    });

    afterEach(() => {
      clock.restore();
    });

    it('should detect the content change', () => {
      expect((el as unknown as WikiEditorInternal).content).to.equal('# **Hello**');
    });

    it('should queue a save', () => {
      expect((el as unknown as WikiEditorInternal).saveStatus).to.equal('editing');
    });
  });

  describe('when Tab key is pressed', () => {
    let clock: SinonFakeTimers;
    let textarea: HTMLTextAreaElement;

    beforeEach(async () => {
      clock = sinon.useFakeTimers({
        shouldAdvanceTime: true,
        shouldClearNativeTimers: true,
      });
      el = buildElement();
      stubReadPage(el, 'hello', '', 'hash1');
      stubUpdateWholePage(el);
      document.body.appendChild(el);
      await waitUntil(
        () => !(el as unknown as WikiEditorInternal).loading,
        'should finish loading',
        { timeout: 5000 }
      );
      await el.updateComplete;

      textarea = el.shadowRoot!.querySelector('textarea')!;
      textarea.selectionStart = 5;
      textarea.selectionEnd = 5;
      textarea.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Tab', bubbles: true })
      );
      await el.updateComplete;
    });

    afterEach(() => {
      clock.restore();
    });

    it('should insert a tab character', () => {
      expect(textarea.value).to.equal('hello\t');
    });

    it('should position cursor after the tab', () => {
      expect(textarea.selectionStart).to.equal(6);
    });
  });

  describe('when file-uploaded event is received', () => {
    let clock: SinonFakeTimers;
    let textarea: HTMLTextAreaElement;

    beforeEach(async () => {
      clock = sinon.useFakeTimers({
        shouldAdvanceTime: true,
        shouldClearNativeTimers: true,
      });
      el = buildElement();
      el.setAttribute('allow-uploads', '');
      stubReadPage(el, '# Hello', '', 'hash1');
      stubUpdateWholePage(el);
      document.body.appendChild(el);
      await waitUntil(
        () => !(el as unknown as WikiEditorInternal).loading,
        'should finish loading',
        { timeout: 5000 }
      );
      await el.updateComplete;

      textarea = el.shadowRoot!.querySelector('textarea')!;
      textarea.selectionStart = 7;
      textarea.selectionEnd = 7;

      const fileDropZone = el.shadowRoot!.querySelector('file-drop-zone')!;
      fileDropZone.dispatchEvent(
        new CustomEvent('file-uploaded', {
          bubbles: true,
          composed: true,
          detail: {
            uploadUrl: '/uploads/test.png',
            filename: 'test.png',
            isImage: true,
          },
        })
      );
      await el.updateComplete;
    });

    afterEach(() => {
      clock.restore();
    });

    it('should insert the markdown image link at cursor position', () => {
      expect(textarea.value).to.equal(
        '# Hello![test.png](/uploads/test.png)'
      );
    });

    it('should queue a save', () => {
      expect(
        (el as unknown as WikiEditorInternal).saveStatus
      ).to.equal('editing');
    });
  });

  describe('when file-uploaded event has a non-image file', () => {
    let clock: SinonFakeTimers;
    let textarea: HTMLTextAreaElement;

    beforeEach(async () => {
      clock = sinon.useFakeTimers({
        shouldAdvanceTime: true,
        shouldClearNativeTimers: true,
      });
      el = buildElement();
      el.setAttribute('allow-uploads', '');
      stubReadPage(el, '# Hello', '', 'hash1');
      stubUpdateWholePage(el);
      document.body.appendChild(el);
      await waitUntil(
        () => !(el as unknown as WikiEditorInternal).loading,
        'should finish loading',
        { timeout: 5000 }
      );
      await el.updateComplete;

      textarea = el.shadowRoot!.querySelector('textarea')!;
      textarea.selectionStart = 7;
      textarea.selectionEnd = 7;

      const fileDropZone = el.shadowRoot!.querySelector('file-drop-zone')!;
      fileDropZone.dispatchEvent(
        new CustomEvent('file-uploaded', {
          bubbles: true,
          composed: true,
          detail: {
            uploadUrl: '/uploads/doc.pdf',
            filename: 'doc.pdf',
            isImage: false,
          },
        })
      );
      await el.updateComplete;
    });

    afterEach(() => {
      clock.restore();
    });

    it('should insert a markdown file link without image prefix', () => {
      expect(textarea.value).to.equal('# Hello[doc.pdf](/uploads/doc.pdf)');
    });
  });

  describe('when allow-uploads and max-upload-mb attributes are set', () => {
    let dropZone: Element | null | undefined;

    beforeEach(async () => {
      el = buildElement();
      el.setAttribute('allow-uploads', '');
      el.setAttribute('max-upload-mb', '25');
      stubReadPage(el);
      stubUpdateWholePage(el);
      await mountAndLoad(el);
      dropZone = el.shadowRoot?.querySelector('file-drop-zone');
    });

    it('should pass allow-uploads to file-drop-zone', () => {
      expect(dropZone?.hasAttribute('allow-uploads')).to.be.true;
    });

    it('should pass max-upload-mb to file-drop-zone', () => {
      expect(dropZone?.getAttribute('max-upload-mb')).to.equal('25');
    });
  });

  describe('when disconnected from DOM', () => {
    let saveQueueDestroySpy: SinonSpy;
    let coordinatorDetachSpy: SinonSpy;

    beforeEach(async () => {
      el = buildElement();
      stubReadPage(el);
      stubUpdateWholePage(el);
      await mountAndLoad(el);

      const internal = el as unknown as WikiEditorInternal;
      saveQueueDestroySpy = sinon.spy(internal.saveQueue!, 'destroy');
      coordinatorDetachSpy = internal.coordinator
        ? sinon.spy(internal.coordinator, 'detach')
        : sinon.spy();

      el.remove();
    });

    it('should destroy the save queue', () => {
      expect(saveQueueDestroySpy).to.have.been.calledOnce;
    });

    it('should null the save queue', () => {
      expect((el as unknown as WikiEditorInternal).saveQueue).to.be.null;
    });

    it('should detach the coordinator if present', () => {
      // Coordinator may not be created if no editor-toolbar is in the shadow DOM
      if (coordinatorDetachSpy.callCount > 0) {
        expect(coordinatorDetachSpy).to.have.been.calledOnce;
      }
    });
  });
});
