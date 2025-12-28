import { html, fixture, expect } from '@open-wc/testing';
import { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';
import { GetFrontmatterResponseSchema } from '../gen/api/v1/frontmatter_pb.js';
import { create, type JsonObject } from '@bufbuild/protobuf';
import './frontmatter-editor-dialog.js';

describe('FrontmatterEditorDialog - Basic Tests', () => {
  let el: FrontmatterEditorDialog;

  beforeEach(async () => {
    el = await fixture(html`<frontmatter-editor-dialog></frontmatter-editor-dialog>`);
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  it('should be an instance of FrontmatterEditorDialog', () => {
    expect(el).to.be.instanceOf(FrontmatterEditorDialog);
  });

  it('should have workingFrontmatter property', () => {
    expect(el.workingFrontmatter).to.exist;
    expect(el.workingFrontmatter).to.be.an('object');
  });

  describe('when updating working frontmatter', () => {
    it('should update working frontmatter from frontmatter response', () => {
      const mockJson: JsonObject = {
        title: 'Test Page',
        identifier: 'home'
      };
      
      el.frontmatter = create(GetFrontmatterResponseSchema, { frontmatter: mockJson });
      el['updateWorkingFrontmatter']();
      
      expect(el.workingFrontmatter).to.deep.equal({
        title: 'Test Page',
        identifier: 'home'
      });
    });
  });
});