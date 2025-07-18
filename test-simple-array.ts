import { html, fixture, expect } from '@open-wc/testing';
import { FrontmatterEditorDialog } from './static/js/web-components/frontmatter-editor-dialog.js';
import sinon from 'sinon';
import './static/js/web-components/frontmatter-editor-dialog.js';

describe('Simple Array Test', () => {
  let el: FrontmatterEditorDialog;

  function timeout(ms: number, message: string): Promise<never> {
    return new Promise((_, reject) => 
      setTimeout(() => reject(new Error(message)), ms)
    );
  }

  beforeEach(async () => {
    try {
      el = await Promise.race([
        fixture(html`<frontmatter-editor-dialog></frontmatter-editor-dialog>`),
        timeout(5000, 'Component fixture timed out')
      ]);
    } catch (e) {
      console.error('Fixture creation failed:', e);
      throw e;
    }
  });

  afterEach(() => {
    sinon.restore();
  });

  it('should exist', () => {
    expect(el).to.exist;
  });
});