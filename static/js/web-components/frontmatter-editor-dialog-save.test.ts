import { html, fixture, expect } from '@open-wc/testing';
import { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';
import { Struct } from '@bufbuild/protobuf';
import { GetFrontmatterResponse, ReplaceFrontmatterRequest, ReplaceFrontmatterResponse } from '../gen/api/v1/frontmatter_pb.js';
import sinon from 'sinon';
import './frontmatter-editor-dialog.js';

describe('FrontmatterEditorDialog - Save Functionality', () => {
  let el: FrontmatterEditorDialog;
  let clientStub: sinon.SinonStub;
  let sessionStorageStub: sinon.SinonStub;
  let refreshPageStub: sinon.SinonStub;

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
    
    // Stub sessionStorage.setItem to test toast storage
    sessionStorageStub = sinon.stub(sessionStorage, 'setItem');
    
    // Stub the refreshPage method to prevent actual page refresh
    refreshPageStub = sinon.stub(el, 'refreshPage' as keyof FrontmatterEditorDialog);
    
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

      beforeEach(async () => {
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
        
        // Execute the save action
        await el['_handleSaveClick']();
      });

      it('should call replaceFrontmatter with correct parameters', () => {
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

      it('should clear saving state', () => {
        expect(el.saving).to.be.false;
      });

      it('should store success toast message', () => {
        expect(sessionStorageStub).to.have.been.calledWith('toast-message', 'Frontmatter saved successfully!');
        expect(sessionStorageStub).to.have.been.calledWith('toast-type', 'success');
      });

      it('should trigger page refresh', () => {
        expect(refreshPageStub).to.have.been.calledOnce;
      });

      it('should close the dialog', () => {
        expect(el.open).to.be.false;
      });

      it('should clear any previous error', () => {
        expect(el.error).to.be.undefined;
      });

      describe('when saving state is observed during operation', () => {
        let savePromise: Promise<void>;
        let savingStateDuringOperation: boolean;

        beforeEach(async () => {
          // Reset the stub to prepare for a new save operation
          clientStub.reset();
          clientStub.resolves(mockResponse);
          
          // Start the save operation and capture saving state
          savePromise = el['_handleSaveClick']();
          savingStateDuringOperation = el.saving;
          
          // Wait for completion
          await savePromise;
        });

        it('should set saving state during operation', () => {
          expect(savingStateDuringOperation).to.be.true;
        });
      });

      describe('when dialog was open before save', () => {
        beforeEach(async () => {
          // Reset and setup for testing dialog close behavior
          clientStub.reset();
          clientStub.resolves(mockResponse);
          el.open = true;
          
          await el['_handleSaveClick']();
        });

        it('should close the dialog', () => {
          expect(el.open).to.be.false;
        });
      });

      describe('when there was a previous error', () => {
        beforeEach(async () => {
          // Reset and setup for testing error clearing
          clientStub.reset();
          clientStub.resolves(mockResponse);
          el.error = 'Previous error';
          
          await el['_handleSaveClick']();
        });

        it('should clear the error', () => {
          expect(el.error).to.be.undefined;
        });
      });

      describe('when testing frontmatter update behavior', () => {
        let frontmatterBeforeClose: GetFrontmatterResponse | undefined;

        beforeEach(async () => {
          // Reset and setup for testing frontmatter update
          clientStub.reset();
          clientStub.resolves(mockResponse);
          
          // Stub the close method to capture the frontmatter before it's cleared
          const originalClose = el.close;
          sinon.stub(el, 'close').callsFake(() => {
            frontmatterBeforeClose = el.frontmatter;
            originalClose.call(el);
          });
          
          await el['_handleSaveClick']();
        });

        it('should update frontmatter data with server response', () => {
          expect(frontmatterBeforeClose).to.be.instanceOf(GetFrontmatterResponse);
          expect(frontmatterBeforeClose!.frontmatter).to.equal(mockResponse.frontmatter);
        });
      });
    });

    describe('when save fails', () => {
      let mockError: Error;

      beforeEach(async () => {
        mockError = new Error('Failed to save frontmatter');
        clientStub.rejects(mockError);
        
        // Execute the save action
        await el['_handleSaveClick']();
      });

      it('should set error message', () => {
        expect(el.error).to.equal('Failed to save frontmatter');
      });

      it('should clear saving state', () => {
        expect(el.saving).to.be.false;
      });

      it('should not refresh page', () => {
        expect(refreshPageStub).not.to.have.been.called;
      });

      it('should not store toast message', () => {
        expect(sessionStorageStub).not.to.have.been.called;
      });

      describe('when dialog was open before save failure', () => {
        beforeEach(async () => {
          // Reset and setup for testing dialog behavior on failure
          clientStub.reset();
          clientStub.rejects(mockError);
          el.open = true;
          
          await el['_handleSaveClick']();
        });

        it('should not close dialog', () => {
          expect(el.open).to.be.true;
        });
      });
    });

    describe('when a non-Error exception is raised', () => {
      beforeEach(async () => {
        // Use a rejected promise with a string directly 
        clientStub.returns(Promise.reject('String error'));
        
        // Execute the save action
        await el['_handleSaveClick']();
      });

      it('should handle non-Error exceptions', () => {
        expect(el.error).to.equal('Failed to save frontmatter');
        expect(el.errorDetails).to.equal('String error');
      });
    });

    describe('when page is not set', () => {
      beforeEach(async () => {
        el.page = '';
        
        // Execute the save action
        await el['_handleSaveClick']();
      });

      it('should not make network call', () => {
        expect(clientStub).not.to.have.been.called;
      });
    });

    describe('when workingFrontmatter is not set', () => {
      beforeEach(async () => {
        el.workingFrontmatter = undefined;
        
        // Execute the save action
        await el['_handleSaveClick']();
      });

      it('should not make network call', () => {
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

    describe('when saving', () => {
      beforeEach(async () => {
        el.saving = true;
        await el.updateComplete;
      });

      it('should disable save button', () => {
        const saveButton = el.shadowRoot!.querySelector('.footer button:last-child') as HTMLButtonElement;
        expect(saveButton.disabled).to.be.true;
        expect(saveButton.textContent!.trim()).to.equal('Saving...');
      });

      it('should disable cancel button', () => {
        const cancelButton = el.shadowRoot!.querySelector('.footer button:first-child') as HTMLButtonElement;
        expect(cancelButton.disabled).to.be.true;
      });
    });

    describe('when loading', () => {
      beforeEach(async () => {
        el.loading = true;
        await el.updateComplete;
      });

      it('should disable save button', () => {
        const saveButton = el.shadowRoot!.querySelector('.footer button:last-child') as HTMLButtonElement;
        expect(saveButton.disabled).to.be.true;
      });
    });

    describe('when not saving or loading', () => {
      beforeEach(async () => {
        el.saving = false;
        el.loading = false;
        await el.updateComplete;
      });

      it('should enable save button', () => {
        const saveButton = el.shadowRoot!.querySelector('.footer button:last-child') as HTMLButtonElement;
        expect(saveButton.disabled).to.be.false;
        expect(saveButton.textContent!.trim()).to.equal('Save');
      });
    });
  });
});