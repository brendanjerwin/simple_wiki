import { html, fixture, expect } from '@open-wc/testing';
import { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';
import { create, type JsonObject } from '@bufbuild/protobuf';
import type { GetFrontmatterResponseSchema} from '../gen/api/v1/frontmatter_pb.js';
import { ReplaceFrontmatterResponseSchema } from '../gen/api/v1/frontmatter_pb.js';
import sinon from 'sinon';

describe('FrontmatterEditorDialog - Save Functionality', () => {
  let el: FrontmatterEditorDialog;
  let clientStub: sinon.SinonStub;
  let sessionStorageStub: sinon.SinonStub;
  let refreshPageStub: sinon.SinonStub;
  let clock: sinon.SinonFakeTimers;

  function timeout(ms: number, message: string): Promise<never> {
    return new Promise((_, reject) =>
      setTimeout(() => reject(new Error(message)), ms)
    );
  }

  beforeEach(async () => {
    // Install fake timers before anything else to prevent the 100ms setTimeout in
    // showToastAfter() from firing after sinon.restore() tears down stubs in afterEach.
    clock = sinon.useFakeTimers();

    // Create mock client before fixture creation
    const mockClient = {
      replaceFrontmatter: sinon.stub(),
      getFrontmatter: sinon.stub(),
    };
    clientStub = mockClient.replaceFrontmatter;

    // Stub ALL prototype methods BEFORE fixture creation.
    // This prevents any real methods from firing during element initialization
    // or during async operations before instance-level stubs could be applied.
    // Critical: refreshPage calls window.location.reload() which disconnects the browser.
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private/public methods for testing
    sinon.stub(FrontmatterEditorDialog.prototype as unknown as { loadFrontmatter: () => Promise<void> }, 'loadFrontmatter').resolves();
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
    sinon.stub(FrontmatterEditorDialog.prototype as unknown as { getClient: () => typeof mockClient }, 'getClient').returns(mockClient);
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
    refreshPageStub = sinon.stub(FrontmatterEditorDialog.prototype as unknown as { refreshPage: () => void }, 'refreshPage');

    // Stub sessionStorage.setItem to test toast storage
    sessionStorageStub = sinon.stub(sessionStorage, 'setItem');

    // NOW create the fixture — all stubs are in place before the element connects to DOM
    el = await Promise.race([
      fixture<FrontmatterEditorDialog>(html`<frontmatter-editor-dialog></frontmatter-editor-dialog>`),
      timeout(5000, 'Component fixture timed out')
    ]);

    await el.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
    if (el) {
      el.remove();
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
      let mockResponse: ReturnType<typeof create<typeof ReplaceFrontmatterResponseSchema>>;

      beforeEach(async () => {
        // Create a successful response with JsonObject
        const responseJson: JsonObject = {
          title: 'Modified Page',
          identifier: 'test-page',
          tags: ['modified', 'test']
        };

        mockResponse = create(ReplaceFrontmatterResponseSchema, { frontmatter: responseJson });
        clientStub.resolves(mockResponse);

        // Execute the save action
        await el['_handleSaveClick']();
        // Flush the 100ms setTimeout in showToastAfter() before afterEach teardown
        clock.tick(200);
      });

      it('should call replaceFrontmatter with correct parameters', () => {
        expect(clientStub).to.have.been.calledOnce;
        const callArgs = clientStub.getCall(0).args[0];
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
          clientStub.reset();
          clientStub.resolves(mockResponse);

          // Start the save operation and capture saving state
          savePromise = el['_handleSaveClick']();
          savingStateDuringOperation = el.saving;

          // Wait for completion
          await savePromise;
          // Flush the 100ms setTimeout in showToastAfter()
          clock.tick(200);
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
          clock.tick(200);
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
          el.augmentedError = new (await import('./augment-error-service.js')).AugmentedError(
            new Error('Previous error'),
            (await import('./augment-error-service.js')).ErrorKind.ERROR,
            'error'
          );

          await el['_handleSaveClick']();
          clock.tick(200);
        });

        it('should clear the error', () => {
          expect(el.augmentedError).to.be.undefined;
        });
      });

      describe('when testing frontmatter update behavior', () => {
        let frontmatterBeforeClose: ReturnType<typeof create<typeof GetFrontmatterResponseSchema>> | undefined;

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
          clock.tick(200);
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
        clientStub.rejects(mockError);

        // Execute the save action
        await el['_handleSaveClick']();
        clock.tick(200);
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
          clientStub.reset();
          clientStub.rejects(mockError);
          el.open = true;

          await el['_handleSaveClick']();
          clock.tick(200);
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
        clock.tick(200);
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
        clock.tick(200);
      });

      it('should not make network call', () => {
        expect(clientStub).not.to.have.been.called;
      });
    });

    describe('when workingFrontmatter is not set', () => {
      beforeEach(async () => {
        delete el.workingFrontmatter;

        // Execute the save action
        await el['_handleSaveClick']();
        clock.tick(200);
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
