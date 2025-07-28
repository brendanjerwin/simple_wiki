import { html, fixture, expect } from '@open-wc/testing';
import '../page-delete-dialog.js';

describe('PageDeleteDialog - Basic', () => {
  it('should exist', async () => {
    const el = await fixture(html`<page-delete-dialog></page-delete-dialog>`);
    expect(el).to.exist;
  });
});