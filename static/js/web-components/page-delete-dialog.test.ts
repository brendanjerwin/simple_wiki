import { html, fixture, expect } from '@open-wc/testing';
import { stub, restore } from 'sinon';
import * as sinon from 'sinon';
import '../page-delete-dialog.js';
import type { PageDeleteDialog } from '../page-delete-dialog.js';
import { DeletePageRequest, DeletePageResponse } from '../../gen/api/v1/page_management_pb.js';

describe('PageDeleteDialog', () => {
  let el: PageDeleteDialog;
  let clientStub: {
    deletePage: sinon.SinonStub;
  };

  beforeEach(async () => {
    el = await fixture(html`<page-delete-dialog></page-delete-dialog>`);
  });

  afterEach(() => {
    restore();
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  it('should be an instance of PageDeleteDialog', () => {
    expect(el).to.be.instanceOf(customElements.get('page-delete-dialog'));
  });

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('page-delete-dialog');
  });

  describe('when component is initialized', () => {
    it('should have default properties', () => {
      expect(el.pageName).to.equal('');
      expect(el.hidden).to.be.true;
    });

    it('should not render dialog when closed', () => {
      const dialogOverlay = el.shadowRoot?.querySelector('.dialog-overlay');
      expect(dialogOverlay).to.not.exist;
    });
  });

  describe('when openDialog is called', () => {
    beforeEach(() => {
      el.openDialog('test-page');
    });

    it('should set pageName', () => {
      expect(el.pageName).to.equal('test-page');
    });

    it('should make component visible', () => {
      expect(el.hidden).to.be.false;
    });

    it('should render dialog overlay', () => {
      const dialogOverlay = el.shadowRoot?.querySelector('.dialog-overlay');
      expect(dialogOverlay).to.exist;
    });

    it('should display confirmation message with page name', () => {
      const pageNameElement = el.shadowRoot?.querySelector('.page-name');
      expect(pageNameElement?.textContent).to.equal('test-page');
    });

    it('should display warning icon', () => {
      const warningIcon = el.shadowRoot?.querySelector('.warning-icon');
      expect(warningIcon?.textContent).to.equal('⚠️');
    });

    it('should render cancel and delete buttons', () => {
      const cancelButton = el.shadowRoot?.querySelector('.button-cancel');
      const deleteButton = el.shadowRoot?.querySelector('.button-delete');
      expect(cancelButton).to.exist;
      expect(deleteButton).to.exist;
      expect(cancelButton?.textContent).to.equal('Cancel');
      expect(deleteButton?.textContent).to.equal('Delete Page');
    });
  });

  describe('when cancel button is clicked', () => {
    beforeEach(() => {
      el.openDialog('test-page');
      const cancelButton = el.shadowRoot?.querySelector('.button-cancel') as HTMLButtonElement;
      cancelButton.click();
    });

    it('should close the dialog', () => {
      expect(el.hidden).to.be.true;
    });

    it('should not render dialog overlay', () => {
      const dialogOverlay = el.shadowRoot?.querySelector('.dialog-overlay');
      expect(dialogOverlay).to.not.exist;
    });
  });

  describe('when clicking outside dialog', () => {
    beforeEach(() => {
      el.openDialog('test-page');
      const dialogOverlay = el.shadowRoot?.querySelector('.dialog-overlay') as HTMLElement;
      dialogOverlay.click();
    });

    it('should close the dialog', () => {
      expect(el.hidden).to.be.true;
    });
  });

  describe('when clicking inside dialog box', () => {
    beforeEach(() => {
      el.openDialog('test-page');
      const dialogBox = el.shadowRoot?.querySelector('.dialog-box') as HTMLElement;
      dialogBox.click();
    });

    it('should not close the dialog', () => {
      expect(el.hidden).to.be.false;
    });
  });

  describe('when delete button is clicked', () => {
    beforeEach(() => {
      // Stub the gRPC client
      clientStub = {
        deletePage: stub()
      };
      (el as PageDeleteDialog & { client: typeof clientStub }).client = clientStub;
    });

    describe('when deletion succeeds', () => {
      let originalLocation: Location;
      let locationStub: { href: string };

      beforeEach(async () => {
        // Mock successful response
        const successResponse = new DeletePageResponse({
          success: true,
          error: ''
        });
        clientStub.deletePage.resolves(successResponse);

        // Mock window.location
        originalLocation = window.location;
        locationStub = { href: '' };
        Object.defineProperty(window, 'location', {
          value: locationStub,
          writable: true
        });

        // Mock sessionStorage
        const sessionStorageStub = {
          setItem: stub(),
          getItem: stub(),
          removeItem: stub()
        };
        Object.defineProperty(window, 'sessionStorage', {
          value: sessionStorageStub,
          writable: true
        });

        el.openDialog('test-page');
        const deleteButton = el.shadowRoot?.querySelector('.button-delete') as HTMLButtonElement;
        deleteButton.click();

        // Wait for async operation
        await new Promise(resolve => setTimeout(resolve, 0));
      });

      afterEach(() => {
        Object.defineProperty(window, 'location', {
          value: originalLocation,
          writable: true
        });
      });

      it('should call deletePage with correct request', () => {
        expect(clientStub.deletePage).to.have.been.calledOnce;
        const request = clientStub.deletePage.getCall(0).args[0];
        expect(request).to.be.instanceOf(DeletePageRequest);
        expect(request.pageName).to.equal('test-page');
      });

      it('should close the dialog', () => {
        expect(el.hidden).to.be.true;
      });

      it('should redirect to homepage', () => {
        expect(locationStub.href).to.equal('/');
      });
    });

    describe('when deletion fails with server error', () => {
      beforeEach(async () => {
        // Mock error response
        const errorResponse = new DeletePageResponse({
          success: false,
          error: 'Page not found'
        });
        clientStub.deletePage.resolves(errorResponse);

        el.openDialog('test-page');
        const deleteButton = el.shadowRoot?.querySelector('.button-delete') as HTMLButtonElement;
        deleteButton.click();

        // Wait for async operation
        await new Promise(resolve => setTimeout(resolve, 0));
        await el.updateComplete;
      });

      it('should display error message', () => {
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        expect(errorDisplay).to.exist;
      });

      it('should not close the dialog', () => {
        expect(el.hidden).to.be.false;
      });
    });

    describe('when deletion fails with network error', () => {
      beforeEach(async () => {
        // Mock network error
        clientStub.deletePage.rejects(new Error('Network error'));

        el.openDialog('test-page');
        const deleteButton = el.shadowRoot?.querySelector('.button-delete') as HTMLButtonElement;
        deleteButton.click();

        // Wait for async operation
        await new Promise(resolve => setTimeout(resolve, 0));
        await el.updateComplete;
      });

      it('should display error message', () => {
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        expect(errorDisplay).to.exist;
      });

      it('should not close the dialog', () => {
        expect(el.hidden).to.be.false;
      });
    });

    describe('when deletion is in progress', () => {
      beforeEach(async () => {
        // Mock pending promise
        const pendingPromise = new Promise(() => {
          // Never resolves to keep it pending
        });
        clientStub.deletePage.returns(pendingPromise);

        el.openDialog('test-page');
        const deleteButton = el.shadowRoot?.querySelector('.button-delete') as HTMLButtonElement;
        deleteButton.click();

        // Wait for next tick to let loading state set
        await new Promise(resolve => setTimeout(resolve, 0));
        await el.updateComplete;
      });

      it('should show loading state on delete button', () => {
        const deleteButton = el.shadowRoot?.querySelector('.button-delete') as HTMLButtonElement;
        expect(deleteButton.textContent).to.equal('Deleting...');
        expect(deleteButton.disabled).to.be.true;
      });

      it('should disable cancel button', () => {
        const cancelButton = el.shadowRoot?.querySelector('.button-cancel') as HTMLButtonElement;
        expect(cancelButton.disabled).to.be.true;
      });
    });
  });

  describe('when pageName is empty', () => {
    beforeEach(() => {
      clientStub = {
        deletePage: stub()
      };
      (el as PageDeleteDialog & { client: typeof clientStub }).client = clientStub;
      
      el.openDialog('');
      const deleteButton = el.shadowRoot?.querySelector('.button-delete') as HTMLButtonElement;
      deleteButton.click();
    });

    it('should not call deletePage', () => {
      expect(clientStub.deletePage).to.not.have.been.called;
    });
  });
});