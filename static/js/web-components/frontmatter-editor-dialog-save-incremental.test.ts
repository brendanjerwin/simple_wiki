import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import './frontmatter-editor-dialog.js';
import type { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';

describe('FrontmatterEditorDialog - Save Test Incremental', () => {
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
    
    // Stub the loadFrontmatter method to prevent network calls
    sinon.stub(el, 'loadFrontmatter').resolves();
    
    // Stub sessionStorage.setItem to test toast storage
    sinon.stub(sessionStorage, 'setItem');
    
    // Stub the refreshPage method to prevent actual page refresh
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
    sinon.stub(el, 'refreshPage' as keyof FrontmatterEditorDialog);
    
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

  describe('when calling save with no data', () => {
    it('should not make network call when page is not set', async () => {
      el.page = '';
      await (el as any)._handleSaveClick();
      expect(mockClient.replaceFrontmatter).not.to.have.been.called;
    });
  });

  describe('when calling save with data', () => {
    beforeEach(async () => {
      el.page = 'test-page';
      el.workingFrontmatter = {
        title: 'Modified Page',
        identifier: 'test-page',
        tags: ['modified', 'test']
      };
      
      mockClient.replaceFrontmatter.resolves({
        frontmatter: {
          title: 'Modified Page',
          identifier: 'test-page',
          tags: ['modified', 'test']
        }
      });
      
      await el.updateComplete;
      await (el as any)._handleSaveClick();
    });

    it('should call replaceFrontmatter', () => {
      expect(mockClient.replaceFrontmatter).to.have.been.calledOnce;
    });

    it('should clear saving state', () => {
      expect(el.saving).to.be.false;
    });
  });
});
