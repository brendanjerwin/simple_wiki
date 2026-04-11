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

  describe('ARIA attributes', () => {
    let toolbarDiv: HTMLElement | null | undefined;

    beforeEach(() => {
      toolbarDiv = toolbar.shadowRoot?.querySelector<HTMLElement>('.toolbar');
    });

    it('should have role="toolbar" on the toolbar container', () => {
      expect(toolbarDiv?.getAttribute('role')).to.equal('toolbar');
    });

    it('should have aria-label on the toolbar container', () => {
      expect(toolbarDiv?.getAttribute('aria-label')).to.equal('Editor toolbar');
    });

    it('should have aria-label on the bold button', () => {
      const btn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="bold"]');
      expect(btn?.getAttribute('aria-label')).to.equal('Bold');
    });

    it('should have aria-label on the italic button', () => {
      const btn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="italic"]');
      expect(btn?.getAttribute('aria-label')).to.equal('Italic');
    });

    it('should have aria-label on the link button', () => {
      const btn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="link"]');
      expect(btn?.getAttribute('aria-label')).to.equal('Insert Link');
    });

    it('should have aria-label on the upload image button', () => {
      const btn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="upload-image"]');
      expect(btn?.getAttribute('aria-label')).to.equal('Upload Image');
    });

    it('should have aria-label on the new page button', () => {
      const btn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="new-page"]');
      expect(btn?.getAttribute('aria-label')).to.equal('Create and Link New Page');
    });

    it('should have aria-label on the exit button', () => {
      const btn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="exit"]');
      expect(btn?.getAttribute('aria-label')).to.equal('Done Editing');
    });
  });

  describe('upload dropdown toggle ARIA', () => {
    let toggleBtn: HTMLButtonElement | null | undefined;

    beforeEach(() => {
      toggleBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('.upload-btn-toggle');
    });

    it('should have aria-haspopup="menu"', () => {
      expect(toggleBtn?.getAttribute('aria-haspopup')).to.equal('menu');
    });

    it('should have aria-expanded="false" when closed', () => {
      expect(toggleBtn?.getAttribute('aria-expanded')).to.equal('false');
    });

    describe('when dropdown is opened', () => {
      beforeEach(async () => {
        toggleBtn?.click();
        await toolbar.updateComplete;
        toggleBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('.upload-btn-toggle');
      });

      it('should have aria-expanded="true"', () => {
        expect(toggleBtn?.getAttribute('aria-expanded')).to.equal('true');
      });

      it('should have role="menu" on the dropdown menu', () => {
        const menu = toolbar.shadowRoot?.querySelector('.upload-dropdown-menu');
        expect(menu?.getAttribute('role')).to.equal('menu');
      });

      it('should have role="menuitem" on the upload file button', () => {
        const item = toolbar.shadowRoot?.querySelector('[data-action="upload-file"]');
        expect(item?.getAttribute('role')).to.equal('menuitem');
      });
    });
  });

  describe('keyboard navigation', () => {
    describe('when Escape is pressed while dropdown is open', () => {
      let toggleBtn: HTMLButtonElement | null | undefined;

      beforeEach(async () => {
        toggleBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('.upload-btn-toggle');
        toggleBtn?.click();
        await toolbar.updateComplete;

        const toolbarDiv = toolbar.shadowRoot?.querySelector<HTMLElement>('.toolbar');
        toolbarDiv?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        await toolbar.updateComplete;
      });

      it('should close the dropdown', () => {
        const menu = toolbar.shadowRoot?.querySelector('.upload-dropdown-menu');
        expect(menu).to.not.exist;
      });
    });

    describe('when ArrowRight is pressed from an enabled button', () => {
      let toolbarDiv: HTMLElement | null | undefined;

      beforeEach(async () => {
        // Enable all format buttons so we can navigate through them
        toolbar.hasSelection = true;
        await toolbar.updateComplete;

        toolbarDiv = toolbar.shadowRoot?.querySelector<HTMLElement>('.toolbar');
        const boldBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="bold"]');
        boldBtn?.focus();
        toolbarDiv?.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
        await toolbar.updateComplete;
      });

      it('should move focus to the next enabled button', () => {
        const italicBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="italic"]');
        expect(toolbar.shadowRoot?.activeElement).to.equal(italicBtn);
      });
    });

    describe('when ArrowLeft is pressed from an enabled button', () => {
      let toolbarDiv: HTMLElement | null | undefined;

      beforeEach(async () => {
        // Enable all format buttons so we can navigate through them
        toolbar.hasSelection = true;
        await toolbar.updateComplete;

        toolbarDiv = toolbar.shadowRoot?.querySelector<HTMLElement>('.toolbar');
        const italicBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="italic"]');
        italicBtn?.focus();
        toolbarDiv?.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
        await toolbar.updateComplete;
      });

      it('should move focus to the previous enabled button', () => {
        const boldBtn = toolbar.shadowRoot?.querySelector<HTMLButtonElement>('[data-action="bold"]');
        expect(toolbar.shadowRoot?.activeElement).to.equal(boldBtn);
      });
    });
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
      expect(boldBtn?.disabled).to.be.true;
    });

    it('should disable the italic button', () => {
      expect(italicBtn?.disabled).to.be.true;
    });

    it('should disable the link button', () => {
      expect(linkBtn?.disabled).to.be.true;
    });

    it('should not disable the exit button', () => {
      expect(exitBtn?.disabled).to.be.false;
    });

    it('should not disable the new page button', () => {
      expect(newPageBtn?.disabled).to.be.false;
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
      expect(boldBtn?.disabled).to.be.false;
    });

    it('should enable the italic button', () => {
      expect(italicBtn?.disabled).to.be.false;
    });

    it('should enable the link button', () => {
      expect(linkBtn?.disabled).to.be.false;
    });
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
      expect(eventSpy).to.have.been.calledOnce;
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
      expect(eventSpy).to.have.been.calledOnce;
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
