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
    expect(toolbar).to.exist;
  });

  it('should have the correct tag name', () => {
    expect(toolbar.tagName.toLowerCase()).to.equal('editor-toolbar');
  });

  describe('when rendered', () => {
    it('should display bold button', () => {
      const boldBtn = toolbar.shadowRoot?.querySelector('[data-action="bold"]');
      expect(boldBtn).to.exist;
    });

    it('should display italic button', () => {
      const italicBtn = toolbar.shadowRoot?.querySelector('[data-action="italic"]');
      expect(italicBtn).to.exist;
    });

    it('should display link button', () => {
      const linkBtn = toolbar.shadowRoot?.querySelector('[data-action="link"]');
      expect(linkBtn).to.exist;
    });

    it('should display upload image button', () => {
      const uploadBtn = toolbar.shadowRoot?.querySelector('[data-action="upload-image"]');
      expect(uploadBtn).to.exist;
    });

    it('should display upload dropdown toggle', () => {
      const toggleBtn = toolbar.shadowRoot?.querySelector('.upload-btn-toggle');
      expect(toggleBtn).to.exist;
    });

    it('should display exit button', () => {
      const exitBtn = toolbar.shadowRoot?.querySelector('[data-action="exit"]');
      expect(exitBtn).to.exist;
    });

    it('should display new page button', () => {
      const newPageBtn = toolbar.shadowRoot?.querySelector('[data-action="new-page"]');
      expect(newPageBtn).to.exist;
    });
  });

  describe('when bold button is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      toolbar.addEventListener('format-bold-requested', eventSpy);
      const boldBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="bold"]');
      boldBtn?.click();
    });

    it('should dispatch format-bold-requested event', () => {
      expect(eventSpy).to.have.been.calledOnce;
    });
  });

  describe('when italic button is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      toolbar.addEventListener('format-italic-requested', eventSpy);
      const italicBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="italic"]');
      italicBtn?.click();
    });

    it('should dispatch format-italic-requested event', () => {
      expect(eventSpy).to.have.been.calledOnce;
    });
  });

  describe('when link button is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      toolbar.addEventListener('insert-link-requested', eventSpy);
      const linkBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="link"]');
      linkBtn?.click();
    });

    it('should dispatch insert-link-requested event', () => {
      expect(eventSpy).to.have.been.calledOnce;
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
      expect(eventSpy).to.have.been.calledOnce;
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
      expect(eventSpy).to.have.been.calledOnce;
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
        expect(fileBtn).to.exist;
      });

      it('should show dropdown menu', () => {
        const menu = toolbar.shadowRoot?.querySelector('.upload-dropdown-menu');
        expect(menu).to.exist;
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
        expect(menu).to.not.exist;
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
      expect(eventSpy).to.have.been.calledOnce;
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
      expect(eventSpy).to.have.been.calledOnce;
    });
  });
});
