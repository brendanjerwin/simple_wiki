import { html, fixture, expect } from '@open-wc/testing';
import { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';
import { GetFrontmatterResponse } from '../gen/api/v1/frontmatter_pb.js';
import { Struct } from '@bufbuild/protobuf';
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

  describe('when convertStructToPlainObject is called', () => {
    it('should convert struct to plain object', () => {
      const mockStruct = new Struct({
        fields: {
          title: { kind: { case: 'stringValue', value: 'Test Page' } },
          identifier: { kind: { case: 'stringValue', value: 'home' } }
        }
      });
      
      const result = el['convertStructToPlainObject'](mockStruct);
      expect(result).to.deep.equal({
        title: 'Test Page',
        identifier: 'home'
      });
    });

    describe('when struct is undefined', () => {
      it('should return empty object', () => {
        const result = el['convertStructToPlainObject'](undefined);
        expect(result).to.deep.equal({});
      });
    });
  });

  describe('when updating working frontmatter', () => {
    it('should update working frontmatter from frontmatter response', () => {
      const mockStruct = new Struct({
        fields: {
          title: { kind: { case: 'stringValue', value: 'Test Page' } },
          identifier: { kind: { case: 'stringValue', value: 'home' } }
        }
      });
      
      el.frontmatter = new GetFrontmatterResponse({ frontmatter: mockStruct });
      el['updateWorkingFrontmatter']();
      
      expect(el.workingFrontmatter).to.deep.equal({
        title: 'Test Page',
        identifier: 'home'
      });
    });
  });
});