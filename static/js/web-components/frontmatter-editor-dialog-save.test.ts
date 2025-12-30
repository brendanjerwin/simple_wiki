import { expect } from '@open-wc/testing';
import type { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';
import sinon from 'sinon';
import './frontmatter-editor-dialog.js';

describe('FrontmatterEditorDialog - Save Functionality', () => {
  let el: FrontmatterEditorDialog;
  let mockClient: {
    getFrontmatter: sinon.SinonStub;
    replaceFrontmatter: sinon.SinonStub;
  };
  let getClientStub: sinon.SinonStub;
  let sessionStorageStub: sinon.SinonStub;
  let refreshPageStub: sinon.SinonStub;

  beforeEach(async () => {
    // Create mock client
    mockClient = {
      getFrontmatter: sinon.stub(),
      replaceFrontmatter: sinon.stub(),
    };

    // Create element without connecting it to DOM first
    el = document.createElement('frontmatter-editor-dialog') as FrontmatterEditorDialog;
    
    // Stub methods BEFORE adding to DOM to prevent any initialization issues
    // Stub getClient to return our mock client
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
    getClientStub = sinon.stub(el, 'getClient' as keyof FrontmatterEditorDialog).returns(mockClient);
    
    // Stub the loadFrontmatter method to prevent network calls
    sinon.stub(el, 'loadFrontmatter').resolves();
    
    // Stub sessionStorage.setItem to test toast storage
    sessionStorageStub = sinon.stub(sessionStorage, 'setItem');
    
    // Stub the refreshPage method to prevent actual page refresh
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
    refreshPageStub = sinon.stub(el, 'refreshPage' as keyof FrontmatterEditorDialog);
    
    // Now add to DOM
    document.body.appendChild(el);
    await el.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
    if (el && el.parentNode) {
      el.parentNode.removeChild(el);
    }
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
      let mockResponse: { frontmatter: any };

      beforeEach(async () => {
        // Create a successful response as a plain object
        const responseJson = {
          title: 'Modified Page',
          identifier: 'test-page',
          tags: ['modified', 'test']
        };
        
        mockResponse = { frontmatter: responseJson };
        mockClient.replaceFrontmatter.resolves(mockResponse);
        
        // Execute the save action
        await el['_handleSaveClick']();
      });

      it('should call replaceFrontmatter with correct parameters', () => {
        expect(mockClient.replaceFrontmatter).to.have.been.calledOnce;
        const callArgs = mockClient.replaceFrontmatter.getCall(0).args[0];
        expect(callArgs.page).to.equal('test-page');
        expect(callArgs.frontmatter).to.be.an('object');
        
        // Verify the frontmatter content - it's already a JsonObject in v2
        const frontmatterData = callArgs.frontmatter;
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
        expect(el.augmentedError).to.be.undefined;
      });

      describe('when saving state is observed during operation', () => {
        let savePromise: Promise<void>;
        let savingStateDuringOperation: boolean;

        beforeEach(async () => {
          // Reset the stub to prepare for a new save operation
          mockClient.replaceFrontmatter.reset();
          mockClient.replaceFrontmatter.resolves(mockResponse);
          
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
          mockClient.replaceFrontmatter.reset();
          mockClient.replaceFrontmatter.resolves(mockResponse);
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
          mockClient.replaceFrontmatter.reset();
          mockClient.replaceFrontmatter.resolves(mockResponse);
          el.augmentedError = new (await import('./augment-error-service.js')).AugmentedError(
            new Error('Previous error'),
            (await import('./augment-error-service.js')).ErrorKind.ERROR,
            'error'
          );
          
          await el['_handleSaveClick']();
        });

        it('should clear the error', () => {
          expect(el.augmentedError).to.be.undefined;
        });
      });

      describe('when testing frontmatter update behavior', () => {
        let frontmatterBeforeClose: any;

        beforeEach(async () => {
          // Reset and setup for testing frontmatter update
          mockClient.replaceFrontmatter.reset();
          mockClient.replaceFrontmatter.resolves(mockResponse);
          
          // Stub the close method to capture the frontmatter before it's cleared
          const originalClose = el.close;
          sinon.stub(el, 'close').callsFake(() => {
            frontmatterBeforeClose = el.frontmatter;
            originalClose.call(el);
          });
          
          await el['_handleSaveClick']();
        });

        it('should update frontmatter data with server response', () => {
          expect(frontmatterBeforeClose).to.exist;
          expect(frontmatterBeforeClose!.frontmatter).to.equal(mockResponse.frontmatter);
        });
      });
    });

    describe('when save fails', () => {
      let mockError: Error;

      beforeEach(async () => {
        mockError = new Error('Failed to save frontmatter');
        mockClient.replaceFrontmatter.rejects(mockError);
        
        // Execute the save action
        await el['_handleSaveClick']();
      });

      it('should set augmented error', () => {
        expect(el.augmentedError).to.exist;
        expect(el.augmentedError?.message).to.include('Failed to save frontmatter');
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
          mockClient.replaceFrontmatter.reset();
          mockClient.replaceFrontmatter.rejects(mockError);
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
        mockClient.replaceFrontmatter.returns(Promise.reject('String error'));
        
        // Execute the save action
        await el['_handleSaveClick']();
      });

      it('should handle non-Error exceptions', () => {
        expect(el.augmentedError).to.exist;
        expect(el.augmentedError?.message).to.equal('String error');
        expect(el.augmentedError?.originalError).to.be.instanceOf(Error);
      });
    });

    describe('when page is not set', () => {
      beforeEach(async () => {
        el.page = '';
        
        // Execute the save action
        await el['_handleSaveClick']();
      });

      it('should not make network call', () => {
        expect(mockClient.replaceFrontmatter).not.to.have.been.called;
      });
    });

    describe('when workingFrontmatter is not set', () => {
      beforeEach(async () => {
        delete el.workingFrontmatter;

        // Execute the save action
        await el['_handleSaveClick']();
      });

      it('should not make network call', () => {
        expect(mockClient.replaceFrontmatter).not.to.have.been.called;
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
        const saveButton = el.shadowRoot!.querySelector<HTMLButtonElement>('.footer button:last-child');
        expect(saveButton!.disabled).to.be.true;
        expect(saveButton!.textContent!.trim()).to.equal('Saving...');
      });

      it('should disable cancel button', () => {
        const cancelButton = el.shadowRoot!.querySelector<HTMLButtonElement>('.footer button:first-child');
        expect(cancelButton!.disabled).to.be.true;
      });
    });

    describe('when loading', () => {
      beforeEach(async () => {
        el.loading = true;
        await el.updateComplete;
      });

      it('should disable save button', () => {
        const saveButton = el.shadowRoot!.querySelector<HTMLButtonElement>('.footer button:last-child');
        expect(saveButton!.disabled).to.be.true;
      });
    });

    describe('when not saving or loading', () => {
      beforeEach(async () => {
        el.saving = false;
        el.loading = false;
        await el.updateComplete;
      });

      it('should enable save button', () => {
        const saveButton = el.shadowRoot!.querySelector<HTMLButtonElement>('.footer button:last-child');
        expect(saveButton!.disabled).to.be.false;
        expect(saveButton!.textContent!.trim()).to.equal('Save');
      });
    });
  });
});