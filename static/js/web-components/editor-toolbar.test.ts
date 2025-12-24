import { expect, fixture, html } from '@open-wc/testing';
import { spy, SinonSpy } from 'sinon';
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

    it('should display upload file button', () => {
      const fileBtn = toolbar.shadowRoot?.querySelector('[data-action="upload-file"]');
      expect(fileBtn).to.exist;
    });

    it('should display exit button', () => {
      const exitBtn = toolbar.shadowRoot?.querySelector('[data-action="exit"]');
      expect(exitBtn).to.exist;
    });
  });

  describe('when bold button is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      toolbar.addEventListener('format-bold-requested', eventSpy);
      const boldBtn = toolbar.shadowRoot?.querySelector('[data-action="bold"]') as HTMLButtonElement;
      boldBtn.click();
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
      const italicBtn = toolbar.shadowRoot?.querySelector('[data-action="italic"]') as HTMLButtonElement;
      italicBtn.click();
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
      const linkBtn = toolbar.shadowRoot?.querySelector('[data-action="link"]') as HTMLButtonElement;
      linkBtn.click();
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
      const uploadBtn = toolbar.shadowRoot?.querySelector('[data-action="upload-image"]') as HTMLButtonElement;
      uploadBtn.click();
    });

    it('should dispatch upload-image-requested event', () => {
      expect(eventSpy).to.have.been.calledOnce;
    });
  });

  describe('when upload file button is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      toolbar.addEventListener('upload-file-requested', eventSpy);
      const fileBtn = toolbar.shadowRoot?.querySelector('[data-action="upload-file"]') as HTMLButtonElement;
      fileBtn.click();
    });

    it('should dispatch upload-file-requested event', () => {
      expect(eventSpy).to.have.been.calledOnce;
    });
  });

  describe('when exit button is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      toolbar.addEventListener('exit-requested', eventSpy);
      const exitBtn = toolbar.shadowRoot?.querySelector('[data-action="exit"]') as HTMLButtonElement;
      exitBtn.click();
    });

    it('should dispatch exit-requested event', () => {
      expect(eventSpy).to.have.been.calledOnce;
    });
  });
});
