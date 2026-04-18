import { expect, waitUntil } from '@open-wc/testing';
import sinon from 'sinon';
import './blog-new-post-dialog.js';
import type { BlogNewPostDialog } from './blog-new-post-dialog.js';
import type { TitleInput } from './title-input.js';
import type { PageCreator } from './page-creator.js';

describe('BlogNewPostDialog', () => {
  let el: BlogNewPostDialog;

  function buildElement(): BlogNewPostDialog {
    const dialog = document.createElement('blog-new-post-dialog') as BlogNewPostDialog;
    dialog.setAttribute('blog-id', 'test-blog');
    return dialog;
  }

  function stubPageCreator(dialog: BlogNewPostDialog): {
    generateIdentifierStub: sinon.SinonStub;
    createPageStub: sinon.SinonStub;
    showSuccessStub: sinon.SinonStub;
  } {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
    const elWithCreator = dialog as unknown as { pageCreator: PageCreator };
    const generateIdentifierStub = sinon.stub(elWithCreator.pageCreator, 'generateIdentifier');
    const createPageStub = sinon.stub(elWithCreator.pageCreator, 'createPage');
    const showSuccessStub = sinon.stub(elWithCreator.pageCreator, 'showSuccess');
    return { generateIdentifierStub, createPageStub, showSuccessStub };
  }

  async function setTitle(dialog: BlogNewPostDialog, value: string): Promise<void> {
    const titleInput = dialog.shadowRoot?.querySelector<TitleInput>('#post-title');
    if (!titleInput) throw new Error('title-input not found');
    titleInput.value = value;
    titleInput.dispatchEvent(new Event('input', { bubbles: true, composed: true }));
    await dialog.updateComplete;
  }

  afterEach(() => {
    sinon.restore();
    if (el) el.remove();
  });

  it('should exist', () => {
    el = buildElement();
    document.body.appendChild(el);
    expect(el).to.exist;
  });

  describe('when closed', () => {

    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      await el.updateComplete;
    });

    it('should not render dialog content', () => {
      const dialog = el.shadowRoot?.querySelector('.dialog');
      expect(dialog).to.not.exist;
    });

  });

  describe('when opened', () => {

    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      el.open = true;
      await el.updateComplete;
    });

    it('should render the dialog', () => {
      const dialog = el.shadowRoot?.querySelector('.dialog');
      expect(dialog).to.exist;
    });

    it('should have a title input', () => {
      const input = el.shadowRoot?.querySelector<TitleInput>('#post-title');
      expect(input).to.exist;
      expect(input?.tagName.toLowerCase()).to.equal('title-input');
    });

    it('should have a date input defaulting to today', () => {
      const input = el.shadowRoot?.querySelector('#post-date') as HTMLInputElement;
      expect(input).to.exist;
      expect(input.type).to.equal('date');
      expect(input.value).to.equal(new Date().toISOString().slice(0, 10));
    });

    it('should have a subtitle input', () => {
      const input = el.shadowRoot?.querySelector('#post-subtitle') as HTMLInputElement;
      expect(input).to.exist;
    });

    it('should have date and subtitle on the same row', () => {
      const row = el.shadowRoot?.querySelector('.form-row');
      expect(row).to.exist;
      expect(row?.querySelector('#post-date')).to.exist;
      expect(row?.querySelector('#post-subtitle')).to.exist;
    });

    it('should have a Create Post button', () => {
      const btn = el.shadowRoot?.querySelector('.btn-primary');
      expect(btn).to.exist;
      expect(btn?.textContent?.trim()).to.equal('Create Post');
    });

    it('should disable Create Post button when title is empty', () => {
      const btn = el.shadowRoot?.querySelector('.btn-primary') as HTMLButtonElement;
      expect(btn.disabled).to.be.true;
    });

    it('should have an embedded wiki-editor', () => {
      const editor = el.shadowRoot?.querySelector('wiki-editor');
      expect(editor).to.exist;
    });

    it('should have a collapsed summary section', () => {
      const toggle = el.shadowRoot?.querySelector('.summary-toggle');
      expect(toggle).to.exist;
      expect(toggle?.textContent).to.contain('Summary');
    });

    it('should not show the summary textarea when collapsed', () => {
      const textarea = el.shadowRoot?.querySelector('#post-summary');
      expect(textarea).to.not.exist;
    });

  });

  describe('when summary section is expanded', () => {

    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      el.open = true;
      await el.updateComplete;

      const toggle = el.shadowRoot?.querySelector('.summary-toggle') as HTMLButtonElement;
      toggle.click();
      await el.updateComplete;
    });

    it('should show the summary textarea', () => {
      const textarea = el.shadowRoot?.querySelector('#post-summary') as HTMLTextAreaElement;
      expect(textarea).to.exist;
    });

  });

  describe('when close button is clicked', () => {

    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      el.open = true;
      await el.updateComplete;

      const closeBtn = el.shadowRoot?.querySelector('.close-btn') as HTMLButtonElement;
      closeBtn.click();
      await el.updateComplete;
    });

    it('should close the dialog', () => {
      expect(el.open).to.be.false;
    });

  });

  describe('when backdrop is clicked', () => {

    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      el.open = true;
      await el.updateComplete;

      const backdrop = el.shadowRoot?.querySelector('.backdrop') as HTMLElement;
      backdrop.click();
      await el.updateComplete;
    });

    it('should close the dialog', () => {
      expect(el.open).to.be.false;
    });

  });

  describe('when Escape key is pressed while open', () => {

    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      el.open = true;
      await el.updateComplete;

      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      await el.updateComplete;
    });

    it('should close the dialog', () => {
      expect(el.open).to.be.false;
    });

  });

  describe('when Escape key is pressed while closed', () => {

    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      await el.updateComplete;

      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      await el.updateComplete;
    });

    it('should remain closed', () => {
      expect(el.open).to.be.false;
    });

  });

  describe('when title is entered', () => {

    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      el.open = true;
      await el.updateComplete;

      await setTitle(el, 'My New Post');
    });

    it('should enable the Create Post button', () => {
      const btn = el.shadowRoot?.querySelector('.btn-primary') as HTMLButtonElement;
      expect(btn.disabled).to.be.false;
    });

  });

  describe('when date is changed', () => {

    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      el.open = true;
      await el.updateComplete;

      const input = el.shadowRoot?.querySelector('#post-date') as HTMLInputElement;
      input.value = '2025-06-15';
      input.dispatchEvent(new Event('input', { bubbles: true }));
      await el.updateComplete;
    });

    it('should update the date property', () => {
      expect(el.date).to.equal('2025-06-15');
    });

  });

  describe('when subtitle is entered', () => {

    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      el.open = true;
      await el.updateComplete;

      const input = el.shadowRoot?.querySelector('#post-subtitle') as HTMLInputElement;
      input.value = 'My Subtitle';
      input.dispatchEvent(new Event('input', { bubbles: true }));
      await el.updateComplete;
    });

    it('should update the subtitle property', () => {
      expect(el.subtitle).to.equal('My Subtitle');
    });

  });

  describe('when summary is entered after expanding', () => {

    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      el.open = true;
      await el.updateComplete;

      const toggle = el.shadowRoot?.querySelector('.summary-toggle') as HTMLButtonElement;
      toggle.click();
      await el.updateComplete;

      const textarea = el.shadowRoot?.querySelector('#post-summary') as HTMLTextAreaElement;
      textarea.value = 'My summary text';
      textarea.dispatchEvent(new Event('input', { bubbles: true }));
      await el.updateComplete;
    });

    it('should update the summary property', () => {
      expect(el.summary).to.equal('My summary text');
    });

  });

  describe('when dialog is closed after fields are filled', () => {

    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      el.open = true;
      el.date = '2025-01-01';
      el.summary = 'Some summary';
      el.subtitle = 'Some subtitle';
      el.summaryExpanded = true;
      await el.updateComplete;

      await setTitle(el, 'Test Title');

      const closeBtn = el.shadowRoot?.querySelector('.close-btn') as HTMLButtonElement;
      closeBtn.click();
      await el.updateComplete;
    });

    it('should reset title', () => {
      expect(el.title).to.equal('');
    });

    it('should reset summary', () => {
      expect(el.summary).to.equal('');
    });

    it('should reset subtitle', () => {
      expect(el.subtitle).to.equal('');
    });

    it('should reset summaryExpanded', () => {
      expect(el.summaryExpanded).to.be.false;
    });

    it('should reset error', () => {
      expect(el.error).to.be.null;
    });

  });

  describe('when element is disconnected from DOM', () => {
    let abortSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      await el.updateComplete;

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion, @typescript-eslint/no-explicit-any
      const elAny = el as any;
      const controller: AbortController = elAny._keydownController;
      abortSpy = sinon.spy(controller, 'abort');

      el.remove();
    });

    it('should abort the keydown controller', () => {
      expect(abortSpy).to.have.been.calledOnce;
    });

  });

  describe('identifier preview', () => {

    describe('when blogId, date, and title are all set', () => {

      beforeEach(async () => {
        el = buildElement();
        document.body.appendChild(el);
        el.open = true;
        el.date = '2026-01-15';
        await el.updateComplete;

        await setTitle(el, 'My New Post!  Extra---Dashes');
      });

      it('should show the identifier preview', () => {
        const preview = el.shadowRoot?.querySelector('.identifier-preview');
        expect(preview).to.exist;
      });

      it('should sanitize special characters and collapse whitespace/dashes in the slug', () => {
        const preview = el.shadowRoot?.querySelector('.identifier-preview');
        expect(preview?.textContent).to.equal('test-blog-2026-01-15-my-new-post-extra-dashes');
      });

    });

    describe('when blogId is not set', () => {

      beforeEach(async () => {
        el = document.createElement('blog-new-post-dialog') as BlogNewPostDialog;
        document.body.appendChild(el);
        el.open = true;
        el.date = '2026-01-15';
        await el.updateComplete;

        await setTitle(el, 'My Post');
      });

      it('should not show identifier preview', () => {
        const preview = el.shadowRoot?.querySelector('.identifier-preview');
        expect(preview).to.not.exist;
      });

    });

    describe('when date is empty', () => {

      beforeEach(async () => {
        el = buildElement();
        document.body.appendChild(el);
        el.open = true;
        el.date = '';
        await el.updateComplete;

        await setTitle(el, 'My Post');
      });

      it('should not show identifier preview', () => {
        const preview = el.shadowRoot?.querySelector('.identifier-preview');
        expect(preview).to.not.exist;
      });

    });

  });

  describe('submission', () => {

    describe('when form is submitted successfully', () => {
      let createPageStub: sinon.SinonStub;
      let showSuccessStub: sinon.SinonStub;
      let postCreatedEvent: CustomEvent | null;

      beforeEach(async () => {
        el = buildElement();
        document.body.appendChild(el);

        const stubs = stubPageCreator(el);
        stubs.generateIdentifierStub.resolves({
          identifier: 'test-blog-2026-01-15-my-post',
          isUnique: true,
        });
        stubs.createPageStub.resolves({ success: true });
        createPageStub = stubs.createPageStub;
        showSuccessStub = stubs.showSuccessStub;

        postCreatedEvent = null;
        el.addEventListener('post-created', (e) => {
          postCreatedEvent = e as CustomEvent;
        });

        el.open = true;
        await el.updateComplete;

        await setTitle(el, 'My Post');

        const submitBtn = el.shadowRoot?.querySelector('.btn-primary') as HTMLButtonElement;
        submitBtn.click();

        await waitUntil(() => !el.creating, 'creating should be false after success', { timeout: 3000 });
        await el.updateComplete;
      });

      it('should close the dialog', () => {
        expect(el.open).to.be.false;
      });

      it('should dispatch post-created event', () => {
        expect(postCreatedEvent).to.exist;
      });

      it('should include identifier in event detail', () => {
        expect(postCreatedEvent?.detail.identifier).to.equal('test-blog-2026-01-15-my-post');
      });

      it('should include title in event detail', () => {
        expect(postCreatedEvent?.detail.title).to.equal('My Post');
      });

      it('should call showSuccess', () => {
        expect(showSuccessStub).to.have.been.calledOnce;
      });

      it('should call createPage with the generated identifier', () => {
        expect(createPageStub.firstCall.args[0]).to.equal('test-blog-2026-01-15-my-post');
      });

      it('should call createPage with blog frontmatter including title', () => {
        const frontmatter = createPageStub.firstCall.args[3] as Record<string, unknown>;
        expect(frontmatter['title']).to.equal('My Post');
      });

      it('should not have an error', () => {
        expect(el.error).to.be.null;
      });

      it('should set creating to false', () => {
        expect(el.creating).to.be.false;
      });

    });

    describe('when subtitle and summary are provided', () => {
      let createPageStub: sinon.SinonStub;

      beforeEach(async () => {
        el = buildElement();
        document.body.appendChild(el);

        const stubs = stubPageCreator(el);
        stubs.generateIdentifierStub.resolves({
          identifier: 'test-blog-2026-01-15-my-post',
          isUnique: true,
        });
        stubs.createPageStub.resolves({ success: true });
        createPageStub = stubs.createPageStub;

        el.open = true;
        el.subtitle = 'My Subtitle';
        await el.updateComplete;

        const toggle = el.shadowRoot?.querySelector('.summary-toggle') as HTMLButtonElement;
        toggle.click();
        await el.updateComplete;

        const textarea = el.shadowRoot?.querySelector('#post-summary') as HTMLTextAreaElement;
        textarea.value = 'My summary';
        textarea.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;

        await setTitle(el, 'My Post');

        const submitBtn = el.shadowRoot?.querySelector('.btn-primary') as HTMLButtonElement;
        submitBtn.click();

        await waitUntil(() => !el.creating, 'creating should be false after success', { timeout: 3000 });
        await el.updateComplete;
      });

      it('should include subtitle in blog frontmatter', () => {
        const frontmatter = createPageStub.firstCall.args[3] as Record<string, unknown>;
        const blog = frontmatter['blog'] as Record<string, unknown>;
        expect(blog['subtitle']).to.equal('My Subtitle');
      });

      it('should include summary in blog frontmatter', () => {
        const frontmatter = createPageStub.firstCall.args[3] as Record<string, unknown>;
        const blog = frontmatter['blog'] as Record<string, unknown>;
        expect(blog['summary_markdown']).to.equal('My summary');
      });

    });

    describe('when identifier generation fails', () => {

      beforeEach(async () => {
        el = buildElement();
        document.body.appendChild(el);

        const stubs = stubPageCreator(el);
        stubs.generateIdentifierStub.resolves({
          identifier: '',
          isUnique: false,
          error: new Error('Network error'),
        });

        el.open = true;
        await el.updateComplete;

        await setTitle(el, 'My Post');

        const submitBtn = el.shadowRoot?.querySelector('.btn-primary') as HTMLButtonElement;
        submitBtn.click();

        await waitUntil(() => !el.creating, 'creating should be false after error', { timeout: 3000 });
        await el.updateComplete;
      });

      it('should set error', () => {
        expect(el.error).to.exist;
      });

      it('should remain open', () => {
        expect(el.open).to.be.true;
      });

      it('should display error-display component', () => {
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        expect(errorDisplay).to.exist;
      });

      it('should set creating to false', () => {
        expect(el.creating).to.be.false;
      });

    });

    describe('when page creation fails', () => {

      beforeEach(async () => {
        el = buildElement();
        document.body.appendChild(el);

        const stubs = stubPageCreator(el);
        stubs.generateIdentifierStub.resolves({
          identifier: 'test-blog-2026-01-15-my-post',
          isUnique: true,
        });
        stubs.createPageStub.resolves({
          success: false,
          error: new Error('Page already exists'),
        });

        el.open = true;
        await el.updateComplete;

        await setTitle(el, 'My Post');

        const submitBtn = el.shadowRoot?.querySelector('.btn-primary') as HTMLButtonElement;
        submitBtn.click();

        await waitUntil(() => !el.creating, 'creating should be false after error', { timeout: 3000 });
        await el.updateComplete;
      });

      it('should set error', () => {
        expect(el.error).to.exist;
      });

      it('should remain open', () => {
        expect(el.open).to.be.true;
      });

      it('should display error-display component', () => {
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        expect(errorDisplay).to.exist;
      });

      it('should set creating to false', () => {
        expect(el.creating).to.be.false;
      });

    });

    describe('when submission throws an unexpected exception', () => {

      beforeEach(async () => {
        el = buildElement();
        document.body.appendChild(el);

        const stubs = stubPageCreator(el);
        stubs.generateIdentifierStub.rejects(new Error('Unexpected failure'));

        el.open = true;
        await el.updateComplete;

        await setTitle(el, 'My Post');

        const submitBtn = el.shadowRoot?.querySelector('.btn-primary') as HTMLButtonElement;
        submitBtn.click();

        await waitUntil(() => !el.creating, 'creating should be false after exception', { timeout: 3000 });
        await el.updateComplete;
      });

      it('should set error', () => {
        expect(el.error).to.exist;
      });

      it('should remain open', () => {
        expect(el.open).to.be.true;
      });

      it('should set creating to false', () => {
        expect(el.creating).to.be.false;
      });

    });

    describe('when title is empty', () => {
      let generateIdentifierStub: sinon.SinonStub;

      beforeEach(async () => {
        el = buildElement();
        document.body.appendChild(el);

        const stubs = stubPageCreator(el);
        generateIdentifierStub = stubs.generateIdentifierStub;

        el.open = true;
        // Leave title empty
        await el.updateComplete;

        // Manually call _submit with empty title via direct property access
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion, @typescript-eslint/no-explicit-any
        await (el as any)._submit();
        await el.updateComplete;
      });

      it('should not call generateIdentifier', () => {
        expect(generateIdentifierStub).to.not.have.been.called;
      });

      it('should not set creating to true', () => {
        expect(el.creating).to.be.false;
      });

    });

    describe('creating state', () => {
      let creatingDuringSubmit: boolean;

      beforeEach(async () => {
        el = buildElement();
        document.body.appendChild(el);

        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
        const elWithCreator = el as unknown as { pageCreator: PageCreator };
        let resolveGenerate!: (val: { identifier: string; isUnique: boolean }) => void;
        sinon.stub(elWithCreator.pageCreator, 'generateIdentifier').returns(
          new Promise(resolve => { resolveGenerate = resolve; })
        );
        sinon.stub(elWithCreator.pageCreator, 'createPage').resolves({ success: true });
        sinon.stub(elWithCreator.pageCreator, 'showSuccess');

        el.open = true;
        await el.updateComplete;

        await setTitle(el, 'My Post');

        const submitBtn = el.shadowRoot?.querySelector('.btn-primary') as HTMLButtonElement;
        submitBtn.click();

        await waitUntil(() => el.creating, 'creating should be true during submission', { timeout: 3000 });
        creatingDuringSubmit = el.creating;
        await el.updateComplete;

        resolveGenerate({ identifier: 'test-id', isUnique: true });
        await waitUntil(() => !el.creating, 'creating should be false after completion', { timeout: 3000 });
      });

      it('should set creating to true while the async operation is in progress', () => {
        expect(creatingDuringSubmit).to.be.true;
      });

    });

  });

});
