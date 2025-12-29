import { expect, fixture, html } from '@open-wc/testing';
import { spy, SinonSpy } from 'sinon';
import './editor-context-menu.js';
import type { EditorContextMenu } from './editor-context-menu.js';

describe('EditorContextMenu', () => {
  let el: EditorContextMenu;

  beforeEach(async () => {
    el = await fixture<EditorContextMenu>(html`<editor-context-menu></editor-context-menu>`);
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  describe('when component is initialized', () => {
    it('should not be open by default', () => {
      expect(el.open).to.be.false;
    });

    it('should not render menu items when closed', () => {
      const menuItems = el.shadowRoot?.querySelectorAll('.menu-item');
      expect(menuItems?.length).to.equal(0);
    });
  });

  describe('when openAt is called', () => {
    beforeEach(async () => {
      el.openAt({ x: 100, y: 200 });
      await el.updateComplete;
    });

    it('should set open to true', () => {
      expect(el.open).to.be.true;
    });

    it('should position at specified coordinates', () => {
      expect(el.style.getPropertyValue('--menu-x')).to.equal('100px');
      expect(el.style.getPropertyValue('--menu-y')).to.equal('200px');
    });

    it('should render Upload Image option', () => {
      const items = el.shadowRoot?.querySelectorAll('.menu-item');
      const uploadImage = Array.from(items || []).find(
        item => item.textContent?.includes('Upload Image')
      );
      expect(uploadImage).to.exist;
    });

    it('should render Upload File option', () => {
      const items = el.shadowRoot?.querySelectorAll('.menu-item');
      const uploadFile = Array.from(items || []).find(
        item => item.textContent?.includes('Upload File')
      );
      expect(uploadFile).to.exist;
    });

    it('should render Bold option', () => {
      const items = el.shadowRoot?.querySelectorAll('.menu-item');
      const bold = Array.from(items || []).find(
        item => item.textContent?.includes('Bold')
      );
      expect(bold).to.exist;
    });

    it('should render Italic option', () => {
      const items = el.shadowRoot?.querySelectorAll('.menu-item');
      const italic = Array.from(items || []).find(
        item => item.textContent?.includes('Italic')
      );
      expect(italic).to.exist;
    });

    it('should render Insert Link option', () => {
      const items = el.shadowRoot?.querySelectorAll('.menu-item');
      const link = Array.from(items || []).find(
        item => item.textContent?.includes('Insert Link')
      );
      expect(link).to.exist;
    });

    it('should not render Take Photo option on desktop', () => {
      el.isMobile = false;
      const items = el.shadowRoot?.querySelectorAll('.menu-item');
      const takePhoto = Array.from(items || []).find(
        item => item.textContent?.includes('Take Photo')
      );
      expect(takePhoto).to.not.exist;
    });
  });

  describe('when on mobile device', () => {
    beforeEach(async () => {
      el.isMobile = true;
      el.openAt({ x: 100, y: 200 });
      await el.updateComplete;
    });

    it('should render Take Photo option', () => {
      const items = el.shadowRoot?.querySelectorAll('.menu-item');
      const takePhoto = Array.from(items || []).find(
        item => item.textContent?.includes('Take Photo')
      );
      expect(takePhoto).to.exist;
    });
  });

  describe('when close is called', () => {
    beforeEach(async () => {
      el.openAt({ x: 100, y: 200 });
      await el.updateComplete;
      el.close();
      await el.updateComplete;
    });

    it('should set open to false', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when Escape key is pressed', () => {
    beforeEach(async () => {
      el.openAt({ x: 100, y: 200 });
      await el.updateComplete;
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      await el.updateComplete;
    });

    it('should close the menu', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when clicking outside', () => {
    beforeEach(async () => {
      el.openAt({ x: 100, y: 200 });
      await el.updateComplete;
      document.body.click();
      await el.updateComplete;
    });

    it('should close the menu', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when Upload Image is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      el.addEventListener('upload-image-requested', eventSpy);
      el.openAt({ x: 100, y: 200 });
      await el.updateComplete;

      const items = el.shadowRoot?.querySelectorAll('.menu-item');
      const uploadImage = Array.from(items || []).find(
        item => item.textContent?.includes('Upload Image')
      );
      if (uploadImage instanceof HTMLButtonElement) {
        uploadImage.click();
      }
      await el.updateComplete;
    });

    it('should dispatch upload-image-requested event', () => {
      expect(eventSpy).to.have.been.calledOnce;
    });

    it('should close the menu', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when Upload File is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      el.addEventListener('upload-file-requested', eventSpy);
      el.openAt({ x: 100, y: 200 });
      await el.updateComplete;

      const items = el.shadowRoot?.querySelectorAll('.menu-item');
      const uploadFile = Array.from(items || []).find(
        item => item.textContent?.includes('Upload File')
      );
      if (uploadFile instanceof HTMLButtonElement) {
        uploadFile.click();
      }
      await el.updateComplete;
    });

    it('should dispatch upload-file-requested event', () => {
      expect(eventSpy).to.have.been.calledOnce;
    });

    it('should close the menu', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when Take Photo is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      el.addEventListener('take-photo-requested', eventSpy);
      el.isMobile = true;
      el.openAt({ x: 100, y: 200 });
      await el.updateComplete;

      const items = el.shadowRoot?.querySelectorAll('.menu-item');
      const takePhoto = Array.from(items || []).find(
        item => item.textContent?.includes('Take Photo')
      );
      if (takePhoto instanceof HTMLButtonElement) {
        takePhoto.click();
      }
      await el.updateComplete;
    });

    it('should dispatch take-photo-requested event', () => {
      expect(eventSpy).to.have.been.calledOnce;
    });

    it('should close the menu', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when Bold is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      el.addEventListener('format-bold-requested', eventSpy);
      el.openAt({ x: 100, y: 200 });
      await el.updateComplete;

      const items = el.shadowRoot?.querySelectorAll('.menu-item');
      const bold = Array.from(items || []).find(
        item => item.textContent?.includes('Bold')
      );
      if (bold instanceof HTMLButtonElement) {
        bold.click();
      }
      await el.updateComplete;
    });

    it('should dispatch format-bold-requested event', () => {
      expect(eventSpy).to.have.been.calledOnce;
    });

    it('should close the menu', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when Italic is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      el.addEventListener('format-italic-requested', eventSpy);
      el.openAt({ x: 100, y: 200 });
      await el.updateComplete;

      const items = el.shadowRoot?.querySelectorAll('.menu-item');
      const italic = Array.from(items || []).find(
        item => item.textContent?.includes('Italic')
      );
      if (italic instanceof HTMLButtonElement) {
        italic.click();
      }
      await el.updateComplete;
    });

    it('should dispatch format-italic-requested event', () => {
      expect(eventSpy).to.have.been.calledOnce;
    });

    it('should close the menu', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when Insert Link is clicked', () => {
    let eventSpy: SinonSpy;

    beforeEach(async () => {
      eventSpy = spy();
      el.addEventListener('insert-link-requested', eventSpy);
      el.openAt({ x: 100, y: 200 });
      await el.updateComplete;

      const items = el.shadowRoot?.querySelectorAll('.menu-item');
      const link = Array.from(items || []).find(
        item => item.textContent?.includes('Insert Link')
      );
      if (link instanceof HTMLButtonElement) {
        link.click();
      }
      await el.updateComplete;
    });

    it('should dispatch insert-link-requested event', () => {
      expect(eventSpy).to.have.been.calledOnce;
    });

    it('should close the menu', () => {
      expect(el.open).to.be.false;
    });
  });
});
