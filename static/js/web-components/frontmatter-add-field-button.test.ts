import { expect, fixture, html } from '@open-wc/testing';
import { FrontmatterAddFieldButton } from './frontmatter-add-field-button.js';
import sinon from 'sinon';

describe('FrontmatterAddFieldButton', () => {
  describe('should exist', () => {
    it('should exist', () => {
      expect(customElements.get('frontmatter-add-field-button')).to.exist;
    });

    it('should be an instance of FrontmatterAddFieldButton', async () => {
      const el = await fixture<FrontmatterAddFieldButton>(html`<frontmatter-add-field-button></frontmatter-add-field-button>`);
      expect(el).to.be.instanceOf(FrontmatterAddFieldButton);
    });

    it('should have the correct tag name', async () => {
      const el = await fixture<FrontmatterAddFieldButton>(html`<frontmatter-add-field-button></frontmatter-add-field-button>`);
      expect(el.tagName).to.equal('FRONTMATTER-ADD-FIELD-BUTTON');
    });
  });

  describe('when component is initialized', () => {
    let el: FrontmatterAddFieldButton;

    beforeEach(async () => {
      el = await fixture<FrontmatterAddFieldButton>(html`<frontmatter-add-field-button></frontmatter-add-field-button>`);
    });

    it('should not be open by default', () => {
      expect(el.open).to.be.false;
    });

    it('should not be disabled by default', () => {
      expect(el.disabled).to.be.false;
    });

    it('should render dropdown button', () => {
      const button = el.shadowRoot?.querySelector('.dropdown-button');
      expect(button).to.exist;
    });

    it('should not render dropdown menu when closed', () => {
      const menu = el.shadowRoot?.querySelector('.dropdown-menu');
      expect(menu).to.not.exist;
    });
  });

  describe('when dropdown button is clicked', () => {
    let el: FrontmatterAddFieldButton;
    let button: HTMLElement;

    beforeEach(async () => {
      el = await fixture<FrontmatterAddFieldButton>(html`<frontmatter-add-field-button></frontmatter-add-field-button>`);
      button = el.shadowRoot?.querySelector('.dropdown-button') as HTMLElement;
      button.click();
      await el.updateComplete;
    });

    it('should open the dropdown', () => {
      expect(el.open).to.be.true;
    });

    it('should render dropdown menu', () => {
      const menu = el.shadowRoot?.querySelector('.dropdown-menu');
      expect(menu).to.exist;
    });

    it('should render three dropdown items', () => {
      const items = el.shadowRoot?.querySelectorAll('.dropdown-item');
      expect(items).to.have.length(3);
    });

    it('should render Add Field option', () => {
      const items = el.shadowRoot?.querySelectorAll('.dropdown-item');
      expect(items?.[0]?.textContent?.trim()).to.equal('Add Field');
    });

    it('should render Add Array option', () => {
      const items = el.shadowRoot?.querySelectorAll('.dropdown-item');
      expect(items?.[1]?.textContent?.trim()).to.equal('Add Array');
    });

    it('should render Add Section option', () => {
      const items = el.shadowRoot?.querySelectorAll('.dropdown-item');
      expect(items?.[2]?.textContent?.trim()).to.equal('Add Section');
    });

    describe('when clicking dropdown button again', () => {
      beforeEach(async () => {
        button.click();
        await el.updateComplete;
      });

      it('should close the dropdown', () => {
        expect(el.open).to.be.false;
      });

      it('should not render dropdown menu', () => {
        const menu = el.shadowRoot?.querySelector('.dropdown-menu');
        expect(menu).to.not.exist;
      });
    });
  });

  describe('when dropdown is open and outside click occurs', () => {
    let el: FrontmatterAddFieldButton;

    beforeEach(async () => {
      el = await fixture<FrontmatterAddFieldButton>(html`<frontmatter-add-field-button></frontmatter-add-field-button>`);
      
      // Open dropdown
      const button = el.shadowRoot?.querySelector('.dropdown-button') as HTMLElement;
      button.click();
      await el.updateComplete;
      
      // Simulate outside click
      const outsideEvent = new Event('click');
      Object.defineProperty(outsideEvent, 'target', {
        value: document.body,
        writable: false
      });
      document.dispatchEvent(outsideEvent);
      await el.updateComplete;
    });

    it('should close the dropdown', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when Add Field is clicked', () => {
    let el: FrontmatterAddFieldButton;
    let addFieldSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el = await fixture<FrontmatterAddFieldButton>(html`<frontmatter-add-field-button></frontmatter-add-field-button>`);
      addFieldSpy = sinon.spy();
      el.addEventListener('add-field', addFieldSpy);
      
      // Open dropdown and click Add Field
      const button = el.shadowRoot?.querySelector('.dropdown-button') as HTMLElement;
      button.click();
      await el.updateComplete;
      
      const addFieldItem = el.shadowRoot?.querySelector('.dropdown-item') as HTMLElement;
      addFieldItem.click();
      await el.updateComplete;
    });

    it('should dispatch add-field event', () => {
      expect(addFieldSpy).to.have.been.calledOnce;
    });

    it('should include field type in event detail', () => {
      expect(addFieldSpy.firstCall.args[0].detail.type).to.equal('field');
    });

    it('should close the dropdown', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when Add Array is clicked', () => {
    let el: FrontmatterAddFieldButton;
    let addFieldSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el = await fixture<FrontmatterAddFieldButton>(html`<frontmatter-add-field-button></frontmatter-add-field-button>`);
      addFieldSpy = sinon.spy();
      el.addEventListener('add-field', addFieldSpy);
      
      // Open dropdown and click Add Array
      const button = el.shadowRoot?.querySelector('.dropdown-button') as HTMLElement;
      button.click();
      await el.updateComplete;
      
      const addArrayItem = el.shadowRoot?.querySelectorAll('.dropdown-item')[1] as HTMLElement;
      addArrayItem.click();
      await el.updateComplete;
    });

    it('should dispatch add-field event', () => {
      expect(addFieldSpy).to.have.been.calledOnce;
    });

    it('should include array type in event detail', () => {
      expect(addFieldSpy.firstCall.args[0].detail.type).to.equal('array');
    });

    it('should close the dropdown', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when Add Section is clicked', () => {
    let el: FrontmatterAddFieldButton;
    let addFieldSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el = await fixture<FrontmatterAddFieldButton>(html`<frontmatter-add-field-button></frontmatter-add-field-button>`);
      addFieldSpy = sinon.spy();
      el.addEventListener('add-field', addFieldSpy);
      
      // Open dropdown and click Add Section
      const button = el.shadowRoot?.querySelector('.dropdown-button') as HTMLElement;
      button.click();
      await el.updateComplete;
      
      const addSectionItem = el.shadowRoot?.querySelectorAll('.dropdown-item')[2] as HTMLElement;
      addSectionItem.click();
      await el.updateComplete;
    });

    it('should dispatch add-field event', () => {
      expect(addFieldSpy).to.have.been.calledOnce;
    });

    it('should include section type in event detail', () => {
      expect(addFieldSpy.firstCall.args[0].detail.type).to.equal('section');
    });

    it('should close the dropdown', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when disabled', () => {
    let el: FrontmatterAddFieldButton;

    beforeEach(async () => {
      el = await fixture<FrontmatterAddFieldButton>(html`<frontmatter-add-field-button disabled></frontmatter-add-field-button>`);
    });

    it('should have disabled attribute on button', () => {
      const button = el.shadowRoot?.querySelector('.dropdown-button') as HTMLButtonElement;
      expect(button.disabled).to.be.true;
    });

    describe('when disabled button is clicked', () => {
      beforeEach(async () => {
        const button = el.shadowRoot?.querySelector('.dropdown-button') as HTMLElement;
        button.click();
        await el.updateComplete;
      });

      it('should not open the dropdown', () => {
        expect(el.open).to.be.false;
      });
    });
  });

  describe('when component is disconnected from DOM', () => {
    let el: FrontmatterAddFieldButton;
    let removeEventListenerSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el = await fixture<FrontmatterAddFieldButton>(html`<frontmatter-add-field-button></frontmatter-add-field-button>`);
      removeEventListenerSpy = sinon.spy(document, 'removeEventListener');
      el.remove();
    });

    afterEach(() => {
      removeEventListenerSpy.restore();
    });

    it('should remove click event listener', () => {
      expect(removeEventListenerSpy).to.have.been.calledWith('click', el._handleClickOutside);
    });
  });
});