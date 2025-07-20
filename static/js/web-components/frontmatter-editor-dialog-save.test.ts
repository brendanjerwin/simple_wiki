import { html, fixture, expect } from '@open-wc/testing';
import { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';
import { Struct } from '@bufbuild/protobuf';
import { GetFrontmatterResponse, ReplaceFrontmatterRequest, ReplaceFrontmatterResponse } from '../gen/api/v1/frontmatter_pb.js';
import sinon from 'sinon';
import './frontmatter-editor-dialog.js';

describe('FrontmatterEditorDialog - Save Functionality', () => {
  let el: FrontmatterEditorDialog;
  let clientStub: sinon.SinonStub;

  function timeout(ms: number, message: string): Promise<never> {
    return new Promise((_, reject) => 
      setTimeout(() => reject(new Error(message)), ms)
    );
  }

  beforeEach(async () => {
    el = await Promise.race([
      fixture(html`<frontmatter-editor-dialog></frontmatter-editor-dialog>`),
      timeout(5000, 'Component fixture timed out')
    ]);
    
    // Stub the loadFrontmatter method to prevent network calls
    sinon.stub(el, 'loadFrontmatter').resolves();
    
    // Stub the client replaceFrontmatter method
    clientStub = sinon.stub(el['client'], 'replaceFrontmatter');
    
    await el.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
    if (el) {
      el.remove();
    }
  });

  describe('when convertPlainObjectToStruct is called', () => {
    it('should convert plain object to Struct', () => {
      const plainObject = {
        title: 'Test Page',
        identifier: 'home',
        tags: ['test', 'example']
      };
      
      const struct = el['convertPlainObjectToStruct'](plainObject);
      expect(struct).to.be.instanceOf(Struct);
      
      // Convert back to verify the conversion worked
      const converted = struct.toJson();
      expect(converted).to.deep.equal(plainObject);
    });

    it('should handle empty object', () => {
      const emptyObject = {};
      const struct = el['convertPlainObjectToStruct'](emptyObject);
      expect(struct).to.be.instanceOf(Struct);
      expect(struct.toJson()).to.deep.equal({});
    });

    it('should handle nested objects', () => {
      const nestedObject = {
        config: {
          debug: true,
          port: 8080
        },
        metadata: {
          author: 'Test Author'
        }
      };
      
      const struct = el['convertPlainObjectToStruct'](nestedObject);
      expect(struct.toJson()).to.deep.equal(nestedObject);
    });
  });

  describe('when save button is clicked', () => {
    beforeEach(async () => {
      // Set up the component with test data
      el.page = 'test-page';
      el.workingFrontmatter = {
        title: 'Modified Page',
        identifier: 'test-page',
        tags: ['modified', 'test']
      };
      await el.updateComplete;
    });

    describe('when save is successful', () => {
      let mockResponse: ReplaceFrontmatterResponse;

      beforeEach(() => {
        // Create a successful response
        const responseStruct = new Struct({
          fields: {
            title: { kind: { case: 'stringValue', value: 'Modified Page' } },
            identifier: { kind: { case: 'stringValue', value: 'test-page' } },
            tags: { 
              kind: { 
                case: 'listValue', 
                value: { 
                  values: [
                    { kind: { case: 'stringValue', value: 'modified' } },
                    { kind: { case: 'stringValue', value: 'test' } }
                  ]
                }
              }
            }
          }
        });
        
        mockResponse = new ReplaceFrontmatterResponse({ frontmatter: responseStruct });
        clientStub.resolves(mockResponse);
      });

      it('should call replaceFrontmatter with correct parameters', async () => {
        await el['_handleSaveClick']();

        expect(clientStub).to.have.been.calledOnce;
        const callArgs = clientStub.getCall(0).args[0] as ReplaceFrontmatterRequest;
        expect(callArgs.page).to.equal('test-page');
        expect(callArgs.frontmatter).to.be.instanceOf(Struct);
        
        // Verify the frontmatter content
        const frontmatterData = callArgs.frontmatter!.toJson();
        expect(frontmatterData).to.deep.equal({
          title: 'Modified Page',
          identifier: 'test-page',
          tags: ['modified', 'test']
        });
      });

      it('should set saving state during save operation', async () => {
        // Start the save operation
        const savePromise = el['_handleSaveClick']();
        
        // Check that saving state is set
        expect(el.saving).to.be.true;
        
        // Wait for completion
        await savePromise;
        
        // Check that saving state is cleared
        expect(el.saving).to.be.false;
      });

      it('should close the dialog after successful save', async () => {
        el.open = true;
        await el['_handleSaveClick']();
        expect(el.open).to.be.false;
      });

      it('should update frontmatter data with server response', async () => {
        // Since close() clears frontmatter, we need to check before the save completes
        let frontmatterBeforeClose: GetFrontmatterResponse | undefined;
        
        // Stub the close method to capture the frontmatter before it's cleared
        const originalClose = el.close;
        sinon.stub(el, 'close').callsFake(() => {
          frontmatterBeforeClose = el.frontmatter;
          originalClose.call(el);
        });
        
        await el['_handleSaveClick']();
        
        expect(frontmatterBeforeClose).to.be.instanceOf(GetFrontmatterResponse);
        expect(frontmatterBeforeClose!.frontmatter).to.equal(mockResponse.frontmatter);
      });

      it('should clear any previous error', async () => {
        el.error = 'Previous error';
        await el['_handleSaveClick']();
        expect(el.error).to.be.undefined;
      });
    });

    describe('when save fails', () => {
      let mockError: Error;

      beforeEach(() => {
        mockError = new Error('Failed to save frontmatter');
        clientStub.rejects(mockError);
      });

      it('should set error message on failure', async () => {
        await el['_handleSaveClick']();
        expect(el.error).to.equal('Failed to save frontmatter');
      });

      it('should clear saving state on failure', async () => {
        await el['_handleSaveClick']();
        expect(el.saving).to.be.false;
      });

      it('should not close dialog on failure', async () => {
        el.open = true;
        await el['_handleSaveClick']();
        expect(el.open).to.be.true;
      });

      it('should handle non-Error exceptions', async () => {
        // Use a rejected promise with a string directly 
        clientStub.returns(Promise.reject('String error'));
        await el['_handleSaveClick']();
        expect(el.error).to.equal('String error');
      });
    });

    describe('when page is not set', () => {
      it('should not make network call if page is empty', async () => {
        el.page = '';
        await el['_handleSaveClick']();
        expect(clientStub).not.to.have.been.called;
      });
    });

    describe('when workingFrontmatter is not set', () => {
      it('should not make network call if workingFrontmatter is undefined', async () => {
        el.workingFrontmatter = undefined;
        await el['_handleSaveClick']();
        expect(clientStub).not.to.have.been.called;
      });
    });
  });

  describe('when rendering save button', () => {
    beforeEach(async () => {
      el.page = 'test-page';
      el.workingFrontmatter = { title: 'Test' };
      await el.updateComplete;
    });

    it('should disable save button when saving', async () => {
      el.saving = true;
      await el.updateComplete;
      
      const saveButton = el.shadowRoot!.querySelector('.footer button:last-child') as HTMLButtonElement;
      expect(saveButton.disabled).to.be.true;
      expect(saveButton.textContent!.trim()).to.equal('Saving...');
    });

    it('should disable save button when loading', async () => {
      el.loading = true;
      await el.updateComplete;
      
      const saveButton = el.shadowRoot!.querySelector('.footer button:last-child') as HTMLButtonElement;
      expect(saveButton.disabled).to.be.true;
    });

    it('should enable save button when not saving or loading', async () => {
      el.saving = false;
      el.loading = false;
      await el.updateComplete;
      
      const saveButton = el.shadowRoot!.querySelector('.footer button:last-child') as HTMLButtonElement;
      expect(saveButton.disabled).to.be.false;
      expect(saveButton.textContent!.trim()).to.equal('Save');
    });

    it('should disable cancel button when saving', async () => {
      el.saving = true;
      await el.updateComplete;
      
      const cancelButton = el.shadowRoot!.querySelector('.footer button:first-child') as HTMLButtonElement;
      expect(cancelButton.disabled).to.be.true;
    });
  });
});