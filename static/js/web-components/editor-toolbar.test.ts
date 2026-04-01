import { expect, fixture, html } from '@open-wc/testing';
import type { SinonSpy } from 'sinon';
import { spy } from 'sinon';
import './editor-toolbar.js';
import type { EditorToolbar } from './editor-toolbar.js';

describe('EditorToolbar', () => {
  let toolbar: EditorToolbar;

  beforeEach(async () => {
    toolbar = await fixture<EditorToolbar>(html`<editor-toolbar></editor-toolbar>`);
  });

  it('should exist', () => {
    expect(toolbar).to.not.equal(null);
  });

  it('should have the correct tag name', () => {
    expect(toolbar.tagName.toLowerCase()).to.equal('editor-toolbar');
  });

  describe('when has-selection is not set', () => {
    let boldBtn: HTMLButtonElement | null | undefined;
    let italicBtn: HTMLButtonElement | null | undefined;
    let linkBtn: HTMLButtonElement | null | undefined;
    let exitBtn: HTMLButtonElement | null | undefined;
    let newPageBtn: HTMLButtonElement | null | undefined;

    beforeEach(() => {
      boldBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="bold"]');
      italicBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="italic"]');
      linkBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="link"]');
      exitBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="exit"]');
      newPageBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="new-page"]');
    });

    it('should disable the bold button', () => {
      expect(boldBtn?.disabled).to.equal(true);
    });

    it('should disable the italic button', () => {
      expect(italicBtn?.disabled).to.equal(true);
    });

    it('should disable the link button', () => {
      expect(linkBtn?.disabled).to.equal(true);
    });

    it('should not disable the exit button', () => {
      expect(exitBtn?.disabled).to.equal(false);
    });

    it('should not disable the new page button', () => {
      expect(newPageBtn?.disabled).to.equal(false);
    });
  });

  describe('when has-selection is set', () => {
    let boldBtn: HTMLButtonElement | null | undefined;
    let italicBtn: HTMLButtonElement | null | undefined;
    let linkBtn: HTMLButtonElement | null | undefined;

    beforeEach(async () => {
      toolbar.hasSelection = true;
      await toolbar.updateComplete;
      boldBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="bold"]');
      italicBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="italic"]');
      linkBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="link"]');
    });

    it('should enable the bold button', () => {
      expect(boldBtn?.disabled).to.equal(false);
    });

    it('should enable the italic button', () => {
      expect(italicBtn?.disabled).to.equal(false);
    });

    it('should enable the link button', () => {
      expect(linkBtn?.disabled).to.equal(false);
    });
  });

  describe('when rendered', () => {
    it('should display bold button', () => {
      const boldBtn = toolbar.shadowRoot?.querySelector('[data-action="bold"]');
      expect(boldBtn).to.not.equal(null);
    });

    it('should display italic button', () => {
      const italicBtn = toolbar.shadowRoot?.querySelector('[data-action="italic"]');
      expect(italicBtn).to.not.equal(null);
    });

    it('should display link button', () => {
      const linkBtn = toolbar.shadowRoot?.querySelector('[data-action="link"]');
      expect(linkBtn).to.not.equal(null);
    });

    it('should display upload image button', () => {
      const uploadBtn = toolbar.shadowRoot?.querySelector('[data-action="upload-image"]');
      expect(uploadBtn).to.not.equal(null);
    });

    it('should display upload dropdown toggle', () => {
      const toggleBtn = toolbar.shadowRoot?.querySelector('.upload-btn-toggle');
      expect(toggleBtn).to.not.equal(null);
    });

    it('should display exit button', () => {
      const exitBtn = toolbar.shadowRoot?.querySelector('[data-action="exit"]');
      expect(exitBtn).to.not.equal(null);
    });

    it('should display new page button', () => {
      const newPageBtn = toolbar.shadowRoot?.querySelector('[data-action="new-page"]');
      expect(newPageBtn).to.not.equal(null);
    });
  });

  describe('when bold button is clicked with selection', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      toolbar.hasSelection = true;
      await toolbar.updateComplete;
      eventSpy = spy();
      toolbar.addEventListener('format-bold-requested', eventSpy);
      const boldBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="bold"]');
      boldBtn?.click();
    });

    it('should dispatch format-bold-requested event', () => {
      expect(eventSpy.callCount).to.equal(1);
    });
  });

  describe('when italic button is clicked with selection', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      toolbar.hasSelection = true;
      await toolbar.updateComplete;
      eventSpy = spy();
      toolbar.addEventListener('format-italic-requested', eventSpy);
      const italicBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="italic"]');
      italicBtn?.click();
    });

    it('should dispatch format-italic-requested event', () => {
      expect(eventSpy.callCount).to.equal(1);
    });
  });

  describe('when link button is clicked with selection', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      toolbar.hasSelection = true;
      await toolbar.updateComplete;
      eventSpy = spy();
      toolbar.addEventListener('insert-link-requested', eventSpy);
      const linkBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="link"]');
      linkBtn?.click();
    });

    it('should dispatch insert-link-requested event', () => {
      expect(eventSpy.callCount).to.equal(1);
    });
  });

  describe('when upload image button is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      toolbar.addEventListener('upload-image-requested', eventSpy);
      const uploadBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="upload-image"]');
      uploadBtn?.click();
    });

    it('should dispatch upload-image-requested event', () => {
      expect(eventSpy.callCount).to.equal(1);
    });
  });

  describe('when upload file is clicked via dropdown', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      toolbar.addEventListener('upload-file-requested', eventSpy);

      // First open the dropdown
      const toggleBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('.upload-btn-toggle');
      toggleBtn?.click();
      await toolbar.updateComplete;

      // Then click the file upload option
      const fileBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="upload-file"]');
      fileBtn?.click();
    });

    it('should dispatch upload-file-requested event', () => {
      expect(eventSpy.callCount).to.equal(1);
    });
  });

  describe('upload dropdown behavior', () => {
    describe('when dropdown toggle is clicked', () => {
      beforeEach(async () => {
        const toggleBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('.upload-btn-toggle');
        toggleBtn?.click();
        await toolbar.updateComplete;
      });

      it('should show upload file option in dropdown', () => {
        const fileBtn = toolbar.shadowRoot?.querySelector('[data-action="upload-file"]');
        expect(fileBtn).to.not.equal(null);
      });

      it('should show dropdown menu', () => {
        const menu = toolbar.shadowRoot?.querySelector('.upload-dropdown-menu');
        expect(menu).to.not.equal(null);
      });
    });

    describe('when dropdown is open and image is clicked', () => {
      beforeEach(async () => {
        // Open dropdown first
        const toggleBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('.upload-btn-toggle');
        toggleBtn?.click();
        await toolbar.updateComplete;

        // Click image button
        const imageBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="upload-image"]');
        imageBtn?.click();
        await toolbar.updateComplete;
      });

      it('should close the dropdown', () => {
        const menu = toolbar.shadowRoot?.querySelector('.upload-dropdown-menu');
        expect(menu).to.equal(null);
      });
    });
  });

  describe('when exit button is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      toolbar.addEventListener('exit-requested', eventSpy);
      const exitBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="exit"]');
      exitBtn?.click();
    });

    it('should dispatch exit-requested event', () => {
      expect(eventSpy.callCount).to.equal(1);
    });
  });

  describe('when new page button is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      toolbar.addEventListener('insert-new-page-requested', eventSpy);
      const newPageBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="new-page"]');
      newPageBtn?.click();
    });

    it('should dispatch insert-new-page-requested event', () => {
      expect(eventSpy.callCount).to.equal(1);
    });
  });
});
