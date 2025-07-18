import { fixture, html, expect } from '@open-wc/testing';
import { TemplateResult } from 'lit';
import { restore } from 'sinon';
import { FrontmatterValueString } from './frontmatter-value-string.js';

function createFixtureWithTimeout(template: TemplateResult, timeoutMs = 5000): Promise<FrontmatterValueString> {
  const timeout = (ms: number, message: string) =>
    new Promise<never>((_, reject) => 
      setTimeout(() => reject(new Error(message)), ms)
    );
  
  return Promise.race([
    fixture(template),
    timeout(timeoutMs, 'Component fixture timed out')
  ]) as Promise<FrontmatterValueString>;
}

describe('FrontmatterValueString', () => {
  let el: FrontmatterValueString;

  afterEach(() => {
    restore();
  });

  describe('should exist', () => {
    beforeEach(async () => {
      try {
        el = await createFixtureWithTimeout(html`<frontmatter-value-string></frontmatter-value-string>`);
      } catch (e) {
        console.error(e);
        throw e;
      }
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
      el = await createFixtureWithTimeout(html`<frontmatter-value-string></frontmatter-value-string>`);
    });

    it('should have empty value by default', () => {
      expect(el.value).to.equal('');
    });

    it('should have empty placeholder by default', () => {
      expect(el.placeholder).to.equal('');
    });

    it('should not be disabled by default', () => {
      expect(el.disabled).to.be.false;
    });
  });

  describe('when rendered with value', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-string value="test value"></frontmatter-value-string>`);
    });

    it('should display the value in input field', () => {
      const inputElement = el.shadowRoot?.querySelector('.value-input') as HTMLInputElement;
      expect(inputElement.value).to.equal('test value');
    });

    it('should render an input element', () => {
      const inputElement = el.shadowRoot?.querySelector('.value-input');
      expect(inputElement).to.exist;
      expect(inputElement?.tagName.toLowerCase()).to.equal('input');
    });
  });

  describe('when placeholder is provided', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-string value="" placeholder="Enter value"></frontmatter-value-string>`);
    });

    it('should set the input placeholder', () => {
      const inputElement = el.shadowRoot?.querySelector('.value-input') as HTMLInputElement;
      expect(inputElement.placeholder).to.equal('Enter value');
    });
  });

  describe('when disabled is true', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-string value="test" disabled></frontmatter-value-string>`);
    });

    it('should disable the input field', () => {
      const inputElement = el.shadowRoot?.querySelector('.value-input') as HTMLInputElement;
      expect(inputElement.disabled).to.be.true;
    });
  });

  describe('when input value changes', () => {
    let valueChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      valueChangeEvent = null;
      el = await createFixtureWithTimeout(html`<frontmatter-value-string value="original"></frontmatter-value-string>`);
      
      el.addEventListener('value-change', (event) => {
        valueChangeEvent = event as CustomEvent;
      });

      const inputElement = el.shadowRoot?.querySelector('.value-input') as HTMLInputElement;
      inputElement.value = 'modified';
      inputElement.dispatchEvent(new Event('input', { bubbles: true }));
    });

    it('should dispatch value-change event', () => {
      expect(valueChangeEvent).to.exist;
    });

    it('should include old value in event detail', () => {
      expect(valueChangeEvent?.detail.oldValue).to.equal('original');
    });

    it('should include new value in event detail', () => {
      expect(valueChangeEvent?.detail.newValue).to.equal('modified');
    });

    it('should update the value property', () => {
      expect(el.value).to.equal('modified');
    });
  });

  describe('when input value changes to empty string', () => {
    let valueChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      valueChangeEvent = null;
      el = await createFixtureWithTimeout(html`<frontmatter-value-string value="original"></frontmatter-value-string>`);
      
      el.addEventListener('value-change', (event) => {
        valueChangeEvent = event as CustomEvent;
      });

      const inputElement = el.shadowRoot?.querySelector('.value-input') as HTMLInputElement;
      inputElement.value = '';
      inputElement.dispatchEvent(new Event('input', { bubbles: true }));
    });

    it('should dispatch value-change event', () => {
      expect(valueChangeEvent).to.exist;
    });

    it('should include empty string as new value', () => {
      expect(valueChangeEvent?.detail.newValue).to.equal('');
    });

    it('should update the value property to empty string', () => {
      expect(el.value).to.equal('');
    });
  });

  describe('when input value changes to same value', () => {
    let valueChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      valueChangeEvent = null;
      el = await createFixtureWithTimeout(html`<frontmatter-value-string value="unchanged"></frontmatter-value-string>`);
      
      el.addEventListener('value-change', (event) => {
        valueChangeEvent = event as CustomEvent;
      });

      const inputElement = el.shadowRoot?.querySelector('.value-input') as HTMLInputElement;
      inputElement.value = 'unchanged';
      inputElement.dispatchEvent(new Event('input', { bubbles: true }));
    });

    it('should not dispatch value-change event', () => {
      expect(valueChangeEvent).to.be.null;
    });

    it('should not update the value property unnecessarily', () => {
      expect(el.value).to.equal('unchanged');
    });
  });

  describe('when focus events occur', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-string value="test"></frontmatter-value-string>`);
    });

    it('should handle focus event properly', () => {
      const inputElement = el.shadowRoot?.querySelector('.value-input') as HTMLInputElement;
      inputElement.focus();
      
      // Should not throw any errors and should maintain proper styling
      expect(inputElement).to.equal(document.activeElement);
    });

    it('should handle blur event properly', () => {
      const inputElement = el.shadowRoot?.querySelector('.value-input') as HTMLInputElement;
      inputElement.focus();
      inputElement.blur();
      
      // Should not throw any errors
      expect(inputElement).to.not.equal(document.activeElement);
    });
  });

  describe('when styling is applied', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-string value="test"></frontmatter-value-string>`);
    });

    it('should have proper input styling', () => {
      const inputElement = el.shadowRoot?.querySelector('.value-input') as HTMLInputElement;
      const computedStyle = getComputedStyle(inputElement);
      
      expect(computedStyle.borderWidth).to.equal('1px');
      expect(computedStyle.borderRadius).to.equal('4px');
      expect(computedStyle.padding).to.contain('8px');
    });

    it('should have proper disabled styling when disabled', async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-string value="test" disabled></frontmatter-value-string>`);
      const inputElement = el.shadowRoot?.querySelector('.value-input') as HTMLInputElement;
      
      expect(inputElement.disabled).to.be.true;
    });
  });

  describe('when value property is updated programmatically', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-string value="initial"></frontmatter-value-string>`);
    });

    describe('when value property changes', () => {
      beforeEach(() => {
        el.value = 'programmatic';
      });

      it('should update the input field value', async () => {
        await el.updateComplete;
        const inputElement = el.shadowRoot?.querySelector('.value-input') as HTMLInputElement;
        expect(inputElement.value).to.equal('programmatic');
      });
    });
  });
});