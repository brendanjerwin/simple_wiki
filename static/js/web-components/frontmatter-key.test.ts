import { fixture, html, expect } from '@open-wc/testing';
import { restore } from 'sinon';
import { FrontmatterKey } from './frontmatter-key.js';

describe('FrontmatterKey', () => {
  let el: FrontmatterKey;

  afterEach(() => {
    restore();
  });

  describe('should exist', () => {
    beforeEach(async () => {
      el = await fixture(html`<frontmatter-key></frontmatter-key>`);
    });

    it('should exist', () => {
      expect(el).to.exist;
    });

    it('should be an instance of FrontmatterKey', () => {
      expect(el).to.be.instanceOf(FrontmatterKey);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName.toLowerCase()).to.equal('frontmatter-key');
    });
  });

  describe('when component is initialized', () => {
    beforeEach(async () => {
      el = await fixture(html`<frontmatter-key></frontmatter-key>`);
    });

    it('should have empty key by default', () => {
      expect(el.key).to.equal('');
    });

    it('should not be editable by default', () => {
      expect(el.editable).to.be.false;
    });

    it('should have empty placeholder by default', () => {
      expect(el.placeholder).to.equal('');
    });
  });

  describe('when rendered with key value', () => {
    beforeEach(async () => {
      el = await fixture(html`<frontmatter-key key="title"></frontmatter-key>`);
    });

    it('should display the key value', () => {
      const keyElement = el.shadowRoot?.querySelector('.key-display');
      expect(keyElement?.textContent?.trim()).to.equal('title');
    });

    it('should not display an input field', () => {
      const inputElement = el.shadowRoot?.querySelector('.key-input');
      expect(inputElement).to.not.exist;
    });
  });

  describe('when editable is true', () => {
    beforeEach(async () => {
      el = await fixture(html`<frontmatter-key key="title" editable></frontmatter-key>`);
    });

    it('should display an input field', () => {
      const inputElement = el.shadowRoot?.querySelector('.key-input');
      expect(inputElement).to.exist;
    });

    it('should not display a static key display', () => {
      const keyElement = el.shadowRoot?.querySelector('.key-display');
      expect(keyElement).to.not.exist;
    });

    it('should set the input value to the key', () => {
      const inputElement = el.shadowRoot?.querySelector('.key-input') as HTMLInputElement;
      expect(inputElement.value).to.equal('title');
    });

    describe('when placeholder is provided', () => {
      beforeEach(async () => {
        el = await fixture(html`<frontmatter-key key="" editable placeholder="Enter key name"></frontmatter-key>`);
      });

      it('should set the input placeholder', () => {
        const inputElement = el.shadowRoot?.querySelector('.key-input') as HTMLInputElement;
        expect(inputElement.placeholder).to.equal('Enter key name');
      });
    });
  });

  describe('when key input value changes', () => {
    let keyChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      keyChangeEvent = null;
      el = await fixture(html`<frontmatter-key key="original" editable></frontmatter-key>`);
      
      el.addEventListener('key-change', (event) => {
        keyChangeEvent = event as CustomEvent;
      });

      const inputElement = el.shadowRoot?.querySelector('.key-input') as HTMLInputElement;
      inputElement.value = 'modified';
      inputElement.dispatchEvent(new Event('input', { bubbles: true }));
    });

    it('should dispatch key-change event', () => {
      expect(keyChangeEvent).to.exist;
    });

    it('should include old key in event detail', () => {
      expect(keyChangeEvent?.detail.oldKey).to.equal('original');
    });

    it('should include new key in event detail', () => {
      expect(keyChangeEvent?.detail.newKey).to.equal('modified');
    });

    it('should update the key property', () => {
      expect(el.key).to.equal('modified');
    });
  });

  describe('when key input value changes to empty', () => {
    let keyChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      keyChangeEvent = null;
      el = await fixture(html`<frontmatter-key key="original" editable></frontmatter-key>`);
      
      el.addEventListener('key-change', (event) => {
        keyChangeEvent = event as CustomEvent;
      });

      const inputElement = el.shadowRoot?.querySelector('.key-input') as HTMLInputElement;
      inputElement.value = '';
      inputElement.dispatchEvent(new Event('input', { bubbles: true }));
    });

    it('should not dispatch key-change event', () => {
      expect(keyChangeEvent).to.be.null;
    });

    it('should revert the input value to original key', () => {
      const inputElement = el.shadowRoot?.querySelector('.key-input') as HTMLInputElement;
      expect(inputElement.value).to.equal('original');
    });

    it('should not update the key property', () => {
      expect(el.key).to.equal('original');
    });
  });

  describe('when key input value changes to whitespace only', () => {
    let keyChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      keyChangeEvent = null;
      el = await fixture(html`<frontmatter-key key="original" editable></frontmatter-key>`);
      
      el.addEventListener('key-change', (event) => {
        keyChangeEvent = event as CustomEvent;
      });

      const inputElement = el.shadowRoot?.querySelector('.key-input') as HTMLInputElement;
      inputElement.value = '   ';
      inputElement.dispatchEvent(new Event('input', { bubbles: true }));
    });

    it('should not dispatch key-change event', () => {
      expect(keyChangeEvent).to.be.null;
    });

    it('should revert the input value to original key', () => {
      const inputElement = el.shadowRoot?.querySelector('.key-input') as HTMLInputElement;
      expect(inputElement.value).to.equal('original');
    });

    it('should not update the key property', () => {
      expect(el.key).to.equal('original');
    });
  });

  describe('when key input value changes to same value', () => {
    let keyChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      keyChangeEvent = null;
      el = await fixture(html`<frontmatter-key key="unchanged" editable></frontmatter-key>`);
      
      el.addEventListener('key-change', (event) => {
        keyChangeEvent = event as CustomEvent;
      });

      const inputElement = el.shadowRoot?.querySelector('.key-input') as HTMLInputElement;
      inputElement.value = 'unchanged';
      inputElement.dispatchEvent(new Event('input', { bubbles: true }));
    });

    it('should not dispatch key-change event', () => {
      expect(keyChangeEvent).to.be.null;
    });

    it('should not update the key property unnecessarily', () => {
      expect(el.key).to.equal('unchanged');
    });
  });

  describe('when styling is applied', () => {
    beforeEach(async () => {
      el = await fixture(html`<frontmatter-key key="test" editable></frontmatter-key>`);
    });

    it('should have proper input styling', () => {
      const inputElement = el.shadowRoot?.querySelector('.key-input') as HTMLInputElement;
      const computedStyle = getComputedStyle(inputElement);
      
      expect(computedStyle.fontWeight).to.equal('600');
      expect(computedStyle.borderWidth).to.equal('1px');
    });

    it('should have proper display styling for non-editable', async () => {
      el = await fixture(html`<frontmatter-key key="test"></frontmatter-key>`);
      const displayElement = el.shadowRoot?.querySelector('.key-display') as HTMLElement;
      const computedStyle = getComputedStyle(displayElement);
      
      expect(computedStyle.fontWeight).to.equal('600');
    });
  });
});