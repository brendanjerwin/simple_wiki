import { html, fixture, expect } from '@open-wc/testing';
import { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';
import sinon from 'sinon';
import './frontmatter-editor-dialog.js';

describe('FrontmatterEditorDialog', () => {
  let el: FrontmatterEditorDialog;

  function timeout(ms: number, message: string): Promise<never> {
    return new Promise((_, reject) => 
      setTimeout(() => reject(new Error(message)), ms)
    );
  }

  beforeEach(async () => {
    try {
      // Use Promise.race to add explicit timeout for fixture creation
      el = await Promise.race([
        fixture(html`<frontmatter-editor-dialog></frontmatter-editor-dialog>`),
        timeout(5000, 'Component fixture timed out')
      ]);
      
      // Stub the loadFrontmatter method to prevent network calls
      sinon.stub(el, 'loadFrontmatter').resolves();
      
      await el.updateComplete;
    } catch (e) {
      throw e;
    }
  });

  afterEach(() => {
    sinon.restore();
    if (el) {
      el.remove();
    }
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  it('should be an instance of FrontmatterEditorDialog', () => {
    expect(el).to.be.instanceOf(FrontmatterEditorDialog);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('frontmatter-editor-dialog');
  });

  describe('when component is initialized', () => {
    it('should not be open by default', () => {
      expect(el.open).to.be.false;
    });

    it('should have empty page by default', () => {
      expect(el.page).to.equal('');
    });

    it('should not be loading by default', () => {
      expect(el.loading).to.be.false;
    });

    it('should have no error by default', () => {
      expect(el.error).to.be.undefined;
    });
  });
});