import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import './blog-new-post-dialog.js';
import type { BlogNewPostDialog } from './blog-new-post-dialog.js';
import type { TitleInput } from './title-input.js';

describe('BlogNewPostDialog', () => {
  let el: BlogNewPostDialog;

  function buildElement(): BlogNewPostDialog {
    const dialog = document.createElement('blog-new-post-dialog') as BlogNewPostDialog;
    dialog.setAttribute('blog-id', 'test-blog');
    return dialog;
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

  describe('when title is entered', () => {
    beforeEach(async () => {
      el = buildElement();
      document.body.appendChild(el);
      el.open = true;
      await el.updateComplete;

      const titleInput = el.shadowRoot?.querySelector<TitleInput>('#post-title');
      if (!titleInput) throw new Error('title-input not found');
      titleInput.value = 'My New Post';
      titleInput.dispatchEvent(new Event('input', { bubbles: true, composed: true }));
      await el.updateComplete;
    });

    it('should enable the Create Post button', () => {
      const btn = el.shadowRoot?.querySelector('.btn-primary') as HTMLButtonElement;
      expect(btn.disabled).to.be.false;
    });
  });
});
