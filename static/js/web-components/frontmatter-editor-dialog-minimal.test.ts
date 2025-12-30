import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import './frontmatter-editor-dialog.js';
import type { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';

describe('FrontmatterEditorDialog - Minimal Test', () => {
  let el: FrontmatterEditorDialog;
  let mockClient: {
    getFrontmatter: sinon.SinonStub;
    replaceFrontmatter: sinon.SinonStub;
  };

  beforeEach(async () => {
    // Create mock client
    mockClient = {
      getFrontmatter: sinon.stub(),
      replaceFrontmatter: sinon.stub(),
    };

    // Create element
    el = document.createElement('frontmatter-editor-dialog') as FrontmatterEditorDialog;
    
    // Stub getClient BEFORE adding to DOM
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
    sinon.stub(el, 'getClient' as keyof FrontmatterEditorDialog).returns(mockClient);
    
    // Add to DOM
    document.body.appendChild(el);
    await el.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
    if (el && el.parentNode) {
      el.parentNode.removeChild(el);
    }
  });

  it('should create the element', () => {
    expect(el).to.exist;
  });

  it('should be an instance of FrontmatterEditorDialog', () => {
    expect(el.tagName.toLowerCase()).to.equal('frontmatter-editor-dialog');
  });

  it('should have default properties', () => {
    expect(el.page).to.equal('');
    expect(el.open).to.be.false;
    expect(el.loading).to.be.false;
    expect(el.saving).to.be.false;
  });
  
  it('should use stubbed client', () => {
    // Verify our stub is in place
    const client = (el as any).getClient();
    expect(client).to.equal(mockClient);
  });
});
