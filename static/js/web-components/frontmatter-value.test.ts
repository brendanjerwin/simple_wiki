import { fixture, html, expect } from '@open-wc/testing';
import { TemplateResult } from 'lit';
import { restore } from 'sinon';
import { FrontmatterValue } from './frontmatter-value.js';

function createFixtureWithTimeout(template: TemplateResult, timeoutMs = 5000): Promise<FrontmatterValue> {
  const timeout = (ms: number, message: string) =>
    new Promise<never>((_, reject) => 
      setTimeout(() => reject(new Error(message)), ms)
    );
  
  return Promise.race([
    fixture(template),
    timeout(timeoutMs, 'Component fixture timed out')
  ]) as Promise<FrontmatterValue>;
}

describe('FrontmatterValue', () => {
  let el: FrontmatterValue;

  afterEach(() => {
    restore();
  });

  describe('should exist', () => {
    beforeEach(async () => {
      try {
        el = await createFixtureWithTimeout(html`<frontmatter-value></frontmatter-value>`);
      } catch (e) {
        console.error(e);
        throw e;
      }
    });

    it('should exist', () => {
      expect(el).to.exist;
    });

    it('should be an instance of FrontmatterValue', () => {
      expect(el).to.be.instanceOf(FrontmatterValue);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName.toLowerCase()).to.equal('frontmatter-value');
    });
  });

  describe('when component is initialized', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value></frontmatter-value>`);
    });

    it('should have null value by default', () => {
      expect(el.value).to.be.null;
    });

    it('should not be disabled by default', () => {
      expect(el.disabled).to.be.false;
    });

    it('should have empty placeholder by default', () => {
      expect(el.placeholder).to.equal('');
    });
  });

  describe('when value is a string', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value .value="${'test string'}"></frontmatter-value>`);
    });

    it('should render frontmatter-value-string component', () => {
      const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string');
      expect(stringComponent).to.exist;
    });

    it('should not render array or section components', () => {
      const arrayComponent = el.shadowRoot?.querySelector('frontmatter-value-array');
      const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section');
      expect(arrayComponent).to.not.exist;
      expect(sectionComponent).to.not.exist;
    });

    it('should pass the value to string component', () => {
      const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string') as HTMLElement & { [key: string]: unknown };
      expect(stringComponent.value).to.equal('test string');
    });

    it('should pass placeholder to string component', async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value .value="${'test'}" placeholder="Enter text"></frontmatter-value>`);
      const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string') as HTMLElement & { [key: string]: unknown };
      expect(stringComponent.placeholder).to.equal('Enter text');
    });

    it('should pass disabled state to string component', async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value .value="${'test'}" disabled></frontmatter-value>`);
      const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string') as HTMLElement & { [key: string]: unknown };
      expect(stringComponent.disabled).to.be.true;
    });
  });

  describe('when value is an array', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value .value="${['item1', 'item2']}"></frontmatter-value>`);
    });

    it('should render frontmatter-value-array component', () => {
      const arrayComponent = el.shadowRoot?.querySelector('frontmatter-value-array');
      expect(arrayComponent).to.exist;
    });

    it('should not render string or section components', () => {
      const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string');
      const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section');
      expect(stringComponent).to.not.exist;
      expect(sectionComponent).to.not.exist;
    });

    it('should pass the array to array component', () => {
      const arrayComponent = el.shadowRoot?.querySelector('frontmatter-value-array') as HTMLElement & { [key: string]: unknown };
      expect(arrayComponent.values).to.deep.equal(['item1', 'item2']);
    });

    it('should pass placeholder to array component', async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value .value="${['test']}" placeholder="Enter item"></frontmatter-value>`);
      const arrayComponent = el.shadowRoot?.querySelector('frontmatter-value-array') as HTMLElement & { [key: string]: unknown };
      expect(arrayComponent.placeholder).to.equal('Enter item');
    });

    it('should pass disabled state to array component', async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value .value="${['test']}" disabled></frontmatter-value>`);
      const arrayComponent = el.shadowRoot?.querySelector('frontmatter-value-array') as HTMLElement & { [key: string]: unknown };
      expect(arrayComponent.disabled).to.be.true;
    });
  });

  describe('when value is an object', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value .value="${{key1: 'value1', key2: 'value2'}}"></frontmatter-value>`);
    });

    it('should render frontmatter-value-section component', () => {
      const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section');
      expect(sectionComponent).to.exist;
    });

    it('should not render string or array components', () => {
      const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string');
      const arrayComponent = el.shadowRoot?.querySelector('frontmatter-value-array');
      expect(stringComponent).to.not.exist;
      expect(arrayComponent).to.not.exist;
    });

    it('should pass the fields to section component', () => {
      const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section') as HTMLElement & { [key: string]: unknown };
      expect(sectionComponent.fields).to.deep.equal({key1: 'value1', key2: 'value2'});
    });

    it('should pass disabled state to section component', async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value .value="${{test: 'value'}}" disabled></frontmatter-value>`);
      const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section') as HTMLElement & { [key: string]: unknown };
      expect(sectionComponent.disabled).to.be.true;
    });
  });

  describe('when value is null or undefined', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value .value="${null}"></frontmatter-value>`);
    });

    it('should render empty state message', () => {
      const emptyMessage = el.shadowRoot?.querySelector('.empty-value-message');
      expect(emptyMessage).to.exist;
      expect(emptyMessage?.textContent?.trim()).to.equal('No value to display');
    });

    it('should not render any value components', () => {
      const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string');
      const arrayComponent = el.shadowRoot?.querySelector('frontmatter-value-array');
      const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section');
      expect(stringComponent).to.not.exist;
      expect(arrayComponent).to.not.exist;
      expect(sectionComponent).to.not.exist;
    });
  });

  describe('when value is a number', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value .value="${42}"></frontmatter-value>`);
    });

    it('should render frontmatter-value-string component', () => {
      const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string');
      expect(stringComponent).to.exist;
    });

    it('should convert number to string for string component', () => {
      const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string') as HTMLElement & { [key: string]: unknown };
      expect(stringComponent.value).to.equal('42');
    });
  });

  describe('when value is a boolean', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value .value="${true}"></frontmatter-value>`);
    });

    it('should render frontmatter-value-string component', () => {
      const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string');
      expect(stringComponent).to.exist;
    });

    it('should convert boolean to string for string component', () => {
      const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string') as HTMLElement & { [key: string]: unknown };
      expect(stringComponent.value).to.equal('true');
    });
  });

  describe('when value change events are dispatched', () => {
    describe('when string value changes', () => {
      let valueChangeEvent: CustomEvent | null;

      beforeEach(async () => {
        valueChangeEvent = null;
        el = await createFixtureWithTimeout(html`<frontmatter-value .value="${'original'}"></frontmatter-value>`);
        
        el.addEventListener('value-change', (event) => {
          valueChangeEvent = event as CustomEvent;
        });

        const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string') as HTMLElement & { [key: string]: unknown };
        stringComponent.dispatchEvent(new CustomEvent('value-change', {
          detail: {
            oldValue: 'original',
            newValue: 'modified'
          },
          bubbles: true
        }));
      });

      it('should dispatch value-change event', () => {
        expect(valueChangeEvent).to.exist;
      });

      it('should include the new value in event detail', () => {
        expect(valueChangeEvent?.detail.newValue).to.equal('modified');
      });

      it('should include the old value in event detail', () => {
        expect(valueChangeEvent?.detail.oldValue).to.equal('original');
      });

      it('should update the value property', () => {
        expect(el.value).to.equal('modified');
      });
    });

    describe('when array value changes', () => {
      let valueChangeEvent: CustomEvent | null;

      beforeEach(async () => {
        valueChangeEvent = null;
        el = await createFixtureWithTimeout(html`<frontmatter-value .value="${['item1']}"></frontmatter-value>`);
        
        el.addEventListener('value-change', (event) => {
          valueChangeEvent = event as CustomEvent;
        });

        const arrayComponent = el.shadowRoot?.querySelector('frontmatter-value-array') as HTMLElement & { [key: string]: unknown };
        arrayComponent.dispatchEvent(new CustomEvent('array-change', {
          detail: {
            oldArray: ['item1'],
            newArray: ['item1', 'item2']
          },
          bubbles: true
        }));
      });

      it('should dispatch value-change event', () => {
        expect(valueChangeEvent).to.exist;
      });

      it('should include the new array in event detail', () => {
        expect(valueChangeEvent?.detail.newValue).to.deep.equal(['item1', 'item2']);
      });

      it('should update the value property', () => {
        expect(el.value).to.deep.equal(['item1', 'item2']);
      });
    });

    describe('when section value changes', () => {
      let valueChangeEvent: CustomEvent | null;

      beforeEach(async () => {
        valueChangeEvent = null;
        el = await createFixtureWithTimeout(html`<frontmatter-value .value="${{key: 'value'}}"></frontmatter-value>`);
        
        el.addEventListener('value-change', (event) => {
          valueChangeEvent = event as CustomEvent;
        });

        const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section') as HTMLElement & { [key: string]: unknown };
        sectionComponent.dispatchEvent(new CustomEvent('section-change', {
          detail: {
            oldFields: {key: 'value'},
            newFields: {key: 'modified', newKey: 'newValue'}
          },
          bubbles: true
        }));
      });

      it('should dispatch value-change event', () => {
        expect(valueChangeEvent).to.exist;
      });

      it('should include the new fields in event detail', () => {
        expect(valueChangeEvent?.detail.newValue).to.deep.equal({key: 'modified', newKey: 'newValue'});
      });

      it('should update the value property', () => {
        expect(el.value).to.deep.equal({key: 'modified', newKey: 'newValue'});
      });
    });
  });

  describe('when value property is updated programmatically', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value .value="${'initial'}"></frontmatter-value>`);
    });

    describe('when value changes from string to array', () => {
      beforeEach(async () => {
        el.value = ['new', 'array'];
        await el.updateComplete;
      });

      it('should render array component instead of string component', () => {
        const arrayComponent = el.shadowRoot?.querySelector('frontmatter-value-array');
        const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string');
        expect(arrayComponent).to.exist;
        expect(stringComponent).to.not.exist;
      });
    });

    describe('when value changes from string to object', () => {
      beforeEach(async () => {
        el.value = {new: 'object'};
        await el.updateComplete;
      });

      it('should render section component instead of string component', () => {
        const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section');
        const stringComponent = el.shadowRoot?.querySelector('frontmatter-value-string');
        expect(sectionComponent).to.exist;
        expect(stringComponent).to.not.exist;
      });
    });
  });
});