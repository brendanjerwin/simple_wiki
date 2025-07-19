import { fixture, html, expect } from '@open-wc/testing';
import { restore } from 'sinon';
import { FrontmatterValueString } from './frontmatter-value-string.js';

describe('FrontmatterValueString', () => {
  let el: FrontmatterValueString;

  afterEach(() => {
    restore();
  });

  describe('should exist', () => {
    beforeEach(async () => {
      el = await fixture(html`<frontmatter-value-string></frontmatter-value-string>`);
    });

    it('should exist', () => {
      expect(el).to.exist;
    });

    it('should be an instance of FrontmatterValueString', () => {
      expect(el).to.be.instanceOf(FrontmatterValueString);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName.toLowerCase()).to.equal('frontmatter-value-string');
    });
  });

  describe('when component is initialized', () => {
    beforeEach(async () => {
      el = await fixture(html`<frontmatter-value-string></frontmatter-value-string>`);
    });

    it('should have empty value by default', () => {
      expect(el.value).to.equal('');
    });

    it('should not be disabled by default', () => {
      expect(el.disabled).to.be.false;
    });

    it('should have empty placeholder by default', () => {
      expect(el.placeholder).to.equal('');
    });
  });

  describe('when value is provided', () => {
    beforeEach(async () => {
      el = await fixture(html`<frontmatter-value-string value="test value"></frontmatter-value-string>`);
    });

    it('should display the value in input field', () => {
      const inputElement = el.shadowRoot?.querySelector('.value-input') as HTMLInputElement;
      expect(inputElement?.value).to.equal('test value');
    });
  });

  describe('when input value changes', () => {
    let valueChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      valueChangeEvent = null;
      el = await fixture(html`<frontmatter-value-string value="initial"></frontmatter-value-string>`);
      
      el.addEventListener('value-change', (event) => {
        valueChangeEvent = event as CustomEvent;
      });

      const inputElement = el.shadowRoot?.querySelector('.value-input') as HTMLInputElement;
      inputElement.value = 'updated';
      inputElement.dispatchEvent(new Event('input', { bubbles: true }));
    });

    it('should dispatch value-change event', () => {
      expect(valueChangeEvent).to.exist;
    });

    it('should include new value in event detail', () => {
      expect(valueChangeEvent?.detail.newValue).to.equal('updated');
    });

    it('should update the value property', () => {
      expect(el.value).to.equal('updated');
    });
  });
});