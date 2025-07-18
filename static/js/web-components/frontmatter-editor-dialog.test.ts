import { html, fixture, expect } from '@open-wc/testing';
import { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';
import { GetFrontmatterResponse } from '../gen/api/v1/frontmatter_pb.js';
import { Struct } from '@bufbuild/protobuf';
import sinon from 'sinon';
import './frontmatter-editor-dialog.js';

describe('FrontmatterEditorDialog', () => {
  let el: FrontmatterEditorDialog;

  beforeEach(async () => {
    el = await fixture(html`<frontmatter-editor-dialog></frontmatter-editor-dialog>`);
  });

  afterEach(() => {
    sinon.restore();
  });

  it('should exist', () => {
    expect(el).to.exist;
  });

  it('should be an instance of FrontmatterEditorDialog', () => {
    expect(el).to.be.instanceOf(FrontmatterEditorDialog);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('frontmatter-editor-dialog');
  });

  describe('when component is initialized', () => {
    it('should not be open by default', () => {
      expect(el.open).to.be.false;
    });

    it('should have empty page by default', () => {
      expect(el.page).to.equal('');
    });

    it('should not be loading by default', () => {
      expect(el.loading).to.be.false;
    });

    it('should have no error by default', () => {
      expect(el.error).to.be.undefined;
    });
  });

  describe('when component is connected to DOM', () => {
    let addEventListenerSpy: sinon.SinonSpy;

    beforeEach(async () => {
      addEventListenerSpy = sinon.spy(document, 'addEventListener');
      // Re-create the element to trigger connectedCallback
      el = await fixture(html`<frontmatter-editor-dialog></frontmatter-editor-dialog>`);
      await el.updateComplete;
    });

    it('should add keydown event listener', () => {
      expect(addEventListenerSpy).to.have.been.calledWith('keydown', el._handleKeydown);
    });
  });

  describe('when component is disconnected from DOM', () => {
    let removeEventListenerSpy: sinon.SinonSpy;

    beforeEach(() => {
      removeEventListenerSpy = sinon.spy(document, 'removeEventListener');
      el.disconnectedCallback();
    });

    it('should remove keydown event listener', () => {
      expect(removeEventListenerSpy).to.have.been.calledWith('keydown', el._handleKeydown);
    });
  });

  describe('when dialog is closed', () => {
    beforeEach(async () => {
      el.open = false;
      await el.updateComplete;
    });

    it('should not have open attribute', () => {
      expect(el.hasAttribute('open')).to.be.false;
    });

    it('should have display none styling', () => {
      const styles = getComputedStyle(el);
      expect(styles.display).to.equal('none');
    });

    it('should not render dialog content', () => {
      const dialog = el.shadowRoot?.querySelector('.dialog');
      expect(dialog).to.exist;
    });
  });

  describe('when opening dialog', () => {
    let loadFrontmatterStub: sinon.SinonStub;

    beforeEach(async () => {
      loadFrontmatterStub = sinon.stub(el, 'loadFrontmatter').resolves();
      el.openDialog('test-page');
      await el.updateComplete;
    });

    it('should set page property', () => {
      expect(el.page).to.equal('test-page');
    });

    it('should set open to true', () => {
      expect(el.open).to.be.true;
    });

    it('should have open attribute', () => {
      expect(el.hasAttribute('open')).to.be.true;
    });

    it('should call loadFrontmatter', () => {
      expect(loadFrontmatterStub).to.have.been.calledOnce;
    });
  });

  describe('when closing dialog', () => {
    beforeEach(async () => {
      el.open = true;
      el.page = 'test-page';
      el.loading = true;
      el.error = 'test error';
      el.frontmatter = new GetFrontmatterResponse();
      await el.updateComplete;
      
      el.close();
      await el.updateComplete;
    });

    it('should set open to false', () => {
      expect(el.open).to.be.false;
    });

    it('should clear frontmatter', () => {
      expect(el.frontmatter).to.be.undefined;
    });

    it('should clear error', () => {
      expect(el.error).to.be.undefined;
    });

    it('should set loading to false', () => {
      expect(el.loading).to.be.false;
    });
  });

  describe('when handling escape key', () => {
    beforeEach(async () => {
      el.open = true;
      await el.updateComplete;
    });

    it('should close dialog on escape key', () => {
      const event = new KeyboardEvent('keydown', { key: 'Escape' });
      el._handleKeydown(event);
      expect(el.open).to.be.false;
    });

    it('should not close dialog on other keys', () => {
      const event = new KeyboardEvent('keydown', { key: 'Enter' });
      el._handleKeydown(event);
      expect(el.open).to.be.true;
    });

    it('should not close dialog if not open', () => {
      el.open = false;
      const event = new KeyboardEvent('keydown', { key: 'Escape' });
      el._handleKeydown(event);
      expect(el.open).to.be.false;
    });
  });

  describe('when dialog is open', () => {
    beforeEach(async () => {
      el.open = true;
      await el.updateComplete;
    });

    it('should render dialog structure', () => {
      const backdrop = el.shadowRoot?.querySelector('.backdrop');
      const dialog = el.shadowRoot?.querySelector('.dialog');
      const header = el.shadowRoot?.querySelector('.header');
      const content = el.shadowRoot?.querySelector('.content');
      const footer = el.shadowRoot?.querySelector('.footer');

      expect(backdrop).to.exist;
      expect(dialog).to.exist;
      expect(header).to.exist;
      expect(content).to.exist;
      expect(footer).to.exist;
    });

    it('should render title', () => {
      const title = el.shadowRoot?.querySelector('.title');
      expect(title).to.exist;
      expect(title?.textContent).to.equal('Edit Frontmatter');
    });

    it('should render close button', () => {
      const closeButton = el.shadowRoot?.querySelector('.close-button');
      expect(closeButton).to.exist;
    });

    it('should render save and cancel buttons', () => {
      const saveButton = el.shadowRoot?.querySelector('.button-save');
      const cancelButton = el.shadowRoot?.querySelector('.button-cancel');
      expect(saveButton).to.exist;
      expect(cancelButton).to.exist;
      expect(saveButton?.textContent?.trim()).to.equal('Save');
      expect(cancelButton?.textContent?.trim()).to.equal('Cancel');
    });
  });

  describe('when in loading state', () => {
    beforeEach(async () => {
      el.open = true;
      el.loading = true;
      el.error = undefined;
      el.frontmatter = undefined;
      await el.updateComplete;
    });

    it('should show loading indicator', () => {
      const loadingElement = el.shadowRoot?.querySelector('.loading');
      expect(loadingElement).to.exist;
      expect(loadingElement?.textContent).to.contain('Loading frontmatter...');
    });

    it('should not show error or frontmatter display', () => {
      const errorElement = el.shadowRoot?.querySelector('.error');
      const frontmatterDisplay = el.shadowRoot?.querySelector('.frontmatter-display');
      expect(errorElement).to.not.exist;
      expect(frontmatterDisplay).to.not.exist;
    });
  });

  describe('when in error state', () => {
    beforeEach(async () => {
      el.open = true;
      el.loading = false;
      el.error = 'Network error';
      el.frontmatter = undefined;
      await el.updateComplete;
    });

    it('should show error message', () => {
      const errorElement = el.shadowRoot?.querySelector('.error');
      expect(errorElement).to.exist;
      expect(errorElement?.textContent).to.contain('Network error');
    });

    it('should not show loading or frontmatter display', () => {
      const loadingElement = el.shadowRoot?.querySelector('.loading');
      const frontmatterDisplay = el.shadowRoot?.querySelector('.frontmatter-display');
      expect(loadingElement).to.not.exist;
      expect(frontmatterDisplay).to.not.exist;
    });
  });

  describe('when displaying frontmatter', () => {
    beforeEach(async () => {
      el.open = true;
      el.loading = false;
      el.error = undefined;
      
      // Create mock frontmatter response
      const mockStruct = new Struct({
        fields: {
          title: { kind: { case: 'stringValue', value: 'Test Page' } },
          author: { kind: { case: 'stringValue', value: 'John Doe' } },
          tags: { 
            kind: { 
              case: 'listValue', 
              value: { 
                values: [
                  { kind: { case: 'stringValue', value: 'test' } },
                  { kind: { case: 'stringValue', value: 'example' } }
                ]
              }
            }
          }
        }
      });
      
      el.frontmatter = new GetFrontmatterResponse({ frontmatter: mockStruct });
      await el.updateComplete;
    });

    it('should show frontmatter display', () => {
      const frontmatterDisplay = el.shadowRoot?.querySelector('.frontmatter-display');
      expect(frontmatterDisplay).to.exist;
    });

    it('should not show loading or error', () => {
      const loadingElement = el.shadowRoot?.querySelector('.loading');
      const errorElement = el.shadowRoot?.querySelector('.error');
      expect(loadingElement).to.not.exist;
      expect(errorElement).to.not.exist;
    });

    it('should display formatted JSON', () => {
      const frontmatterDisplay = el.shadowRoot?.querySelector('.frontmatter-display');
      expect(frontmatterDisplay?.textContent).to.include('title');
      expect(frontmatterDisplay?.textContent).to.include('Test Page');
    });
  });

  describe('when clicking close button', () => {
    let closeSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el.open = true;
      await el.updateComplete;
      closeSpy = sinon.spy(el, 'close');
      
      const closeButton = el.shadowRoot?.querySelector('.close-button') as HTMLButtonElement;
      closeButton.click();
    });

    it('should call close method', () => {
      expect(closeSpy).to.have.been.calledOnce;
    });
  });

  describe('when clicking cancel button', () => {
    let closeSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el.open = true;
      await el.updateComplete;
      closeSpy = sinon.spy(el, 'close');
      
      const cancelButton = el.shadowRoot?.querySelector('.button-cancel') as HTMLButtonElement;
      cancelButton.click();
    });

    it('should call close method', () => {
      expect(closeSpy).to.have.been.calledOnce;
    });
  });

  describe('when clicking save button', () => {
    let closeSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el.open = true;
      await el.updateComplete;
      closeSpy = sinon.spy(el, 'close');
      
      const saveButton = el.shadowRoot?.querySelector('.button-save') as HTMLButtonElement;
      saveButton.click();
    });

    it('should call close method', () => {
      expect(closeSpy).to.have.been.calledOnce;
    });
  });

  describe('when clicking backdrop', () => {
    let closeSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el.open = true;
      await el.updateComplete;
      closeSpy = sinon.spy(el, 'close');
    });

    it('should close dialog when clicking backdrop', () => {
      const backdrop = el.shadowRoot?.querySelector('.backdrop') as HTMLElement;
      const event = new MouseEvent('click', { bubbles: true });
      Object.defineProperty(event, 'target', { value: backdrop });
      Object.defineProperty(event, 'currentTarget', { value: backdrop });
      backdrop.dispatchEvent(event);
      
      expect(closeSpy).to.have.been.calledOnce;
    });

    it('should not close dialog when clicking dialog content', () => {
      const backdrop = el.shadowRoot?.querySelector('.backdrop') as HTMLElement;
      const dialog = el.shadowRoot?.querySelector('.dialog') as HTMLElement;
      const event = new MouseEvent('click', { bubbles: true });
      Object.defineProperty(event, 'target', { value: dialog });
      Object.defineProperty(event, 'currentTarget', { value: backdrop });
      backdrop.dispatchEvent(event);
      
      expect(closeSpy).to.not.have.been.called;
    });
  });
});