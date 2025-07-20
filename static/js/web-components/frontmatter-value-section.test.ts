import { fixture, html, expect } from '@open-wc/testing';
import { TemplateResult } from 'lit';
import { restore } from 'sinon';
import { FrontmatterValueSection } from './frontmatter-value-section.js';

function createFixtureWithTimeout(template: TemplateResult, timeoutMs = 5000): Promise<FrontmatterValueSection> {
  const timeout = (ms: number, message: string) =>
    new Promise<never>((_, reject) => 
      setTimeout(() => reject(new Error(message)), ms)
    );
  
  return Promise.race([
    fixture(template),
    timeout(timeoutMs, 'Component fixture timed out')
  ]) as Promise<FrontmatterValueSection>;
}

describe('FrontmatterValueSection', () => {
  let el: FrontmatterValueSection;

  afterEach(() => {
    restore();
  });

  describe('should exist', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-section></frontmatter-value-section>`);
    });

    it('should exist', () => {
      expect(el).to.exist;
    });

    it('should be an instance of FrontmatterValueSection', () => {
      expect(el).to.be.instanceOf(FrontmatterValueSection);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName.toLowerCase()).to.equal('frontmatter-value-section');
    });
  });

  describe('when component is initialized', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-section></frontmatter-value-section>`);
    });

    it('should have empty fields by default', () => {
      expect(el.fields).to.deep.equal({});
    });

    it('should not be disabled by default', () => {
      expect(el.disabled).to.be.false;
    });
  });

  describe('when rendered with section fields', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-section .fields="${{
        title: 'Test Title',
        description: 'Test Description',
        count: '42'
      }}"></frontmatter-value-section>`);
    });

    it('should render correct number of field rows', () => {
      const fieldRows = el.shadowRoot?.querySelectorAll('.field-row');
      expect(fieldRows?.length).to.equal(3);
    });

    it('should render frontmatter-key components for each field', () => {
      const keyComponents = el.shadowRoot?.querySelectorAll('frontmatter-key');
      expect(keyComponents?.length).to.equal(3);
    });

    it('should render frontmatter-value components for each field', () => {
      const valueComponents = el.shadowRoot?.querySelectorAll('frontmatter-value');
      expect(valueComponents?.length).to.equal(3);
    });

    it('should display the correct keys', () => {
      const keyComponents = el.shadowRoot?.querySelectorAll('frontmatter-key') as NodeListOf<Element>;
      const keys = Array.from(keyComponents).map(comp => comp.key);
      expect(keys).to.include.members(['title', 'description', 'count']);
    });

    it('should display the correct values', () => {
      const valueDispatcherComponents = el.shadowRoot?.querySelectorAll('frontmatter-value') as NodeListOf<HTMLElement & {value: unknown}>;
      const values = Array.from(valueDispatcherComponents).map(comp => comp.value);
      expect(values).to.include.members(['Test Title', 'Test Description', '42']);
    });

    it('should render remove buttons for each field', () => {
      const removeButtons = el.shadowRoot?.querySelectorAll('.remove-field-button');
      expect(removeButtons?.length).to.equal(3);
    });
  });

  describe('when section is empty', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-section .fields="${{}}"></frontmatter-value-section>`);
    });

    it('should display empty section message', () => {
      const emptyMessage = el.shadowRoot?.querySelector('.empty-section-message');
      expect(emptyMessage).to.exist;
      expect(emptyMessage?.textContent?.trim()).to.equal('No fields in section');
    });

    it('should not render any field rows', () => {
      const fieldRows = el.shadowRoot?.querySelectorAll('.field-row');
      expect(fieldRows?.length).to.equal(0);
    });
  });

  describe('when add field button is clicked', () => {
    let sectionChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      sectionChangeEvent = null;
      el = await createFixtureWithTimeout(html`<frontmatter-value-section .fields="${{existing: 'value'}}"></frontmatter-value-section>`);
      
      el.addEventListener('section-change', (event) => {
        sectionChangeEvent = event as CustomEvent;
      });

      // First click to open dropdown
      const addButton = el.shadowRoot?.querySelector('frontmatter-add-field-button') as HTMLElement & {shadowRoot: ShadowRoot, updateComplete: Promise<unknown>};
      addButton?.shadowRoot?.querySelector('button')?.click();
      await addButton?.updateComplete;
      
      // Then click on "Add Field" option to actually add a field
      const addFieldOption = addButton?.shadowRoot?.querySelector('.dropdown-item') as HTMLButtonElement;
      addFieldOption?.click();
      await el.updateComplete;
    });

    it('should dispatch section-change event', () => {
      expect(sectionChangeEvent).to.exist;
    });

    it('should include the new fields in event detail', () => {
      expect(sectionChangeEvent?.detail.newFields).to.have.property('existing', 'value');
      expect(sectionChangeEvent?.detail.newFields).to.have.property('new_field', '');
    });

    it('should include the old fields in event detail', () => {
      expect(sectionChangeEvent?.detail.oldFields).to.deep.equal({existing: 'value'});
    });

    it('should update the fields property', () => {
      expect(el.fields).to.have.property('existing', 'value');
      expect(el.fields).to.have.property('new_field', '');
    });

    it('should render additional field row', () => {
      const fieldRows = el.shadowRoot?.querySelectorAll('.field-row');
      expect(fieldRows?.length).to.equal(2);
    });
  });

  describe('when remove field button is clicked', () => {
    let sectionChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      sectionChangeEvent = null;
      el = await createFixtureWithTimeout(html`<frontmatter-value-section .fields="${{
        field1: 'value1',
        field2: 'value2',
        field3: 'value3'
      }}"></frontmatter-value-section>`);
      
      el.addEventListener('section-change', (event) => {
        sectionChangeEvent = event as CustomEvent;
      });

      // Click remove button for second field
      const removeButtons = el.shadowRoot?.querySelectorAll('.remove-field-button') as NodeListOf<HTMLButtonElement>;
      removeButtons[1].click();
    });

    it('should dispatch section-change event', () => {
      expect(sectionChangeEvent).to.exist;
    });

    it('should include the new fields with field removed', () => {
      expect(Object.keys(sectionChangeEvent?.detail.newFields || {})).to.have.length(2);
      expect(sectionChangeEvent?.detail.newFields).to.have.property('field1', 'value1');
      expect(sectionChangeEvent?.detail.newFields).to.have.property('field3', 'value3');
      expect(sectionChangeEvent?.detail.newFields).to.not.have.property('field2');
    });

    it('should update the fields property', () => {
      expect(Object.keys(el.fields)).to.have.length(2);
      expect(el.fields).to.have.property('field1', 'value1');
      expect(el.fields).to.have.property('field3', 'value3');
      expect(el.fields).to.not.have.property('field2');
    });

    it('should render fewer field rows', () => {
      const fieldRows = el.shadowRoot?.querySelectorAll('.field-row');
      expect(fieldRows?.length).to.equal(2);
    });
  });

  describe('when field key changes', () => {
    let sectionChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      sectionChangeEvent = null;
      el = await createFixtureWithTimeout(html`<frontmatter-value-section .fields="${{
        oldKey: 'test value'
      }}"></frontmatter-value-section>`);
      
      el.addEventListener('section-change', (event) => {
        sectionChangeEvent = event as CustomEvent;
      });

      // Simulate key change
      const keyComponent = el.shadowRoot?.querySelector('frontmatter-key') as HTMLElement & { [key: string]: unknown };
      keyComponent.dispatchEvent(new CustomEvent('key-change', {
        detail: {
          oldKey: 'oldKey',
          newKey: 'newKey'
        },
        bubbles: true
      }));
    });

    it('should dispatch section-change event', () => {
      expect(sectionChangeEvent).to.exist;
    });

    it('should include the updated fields with new key', () => {
      expect(sectionChangeEvent?.detail.newFields).to.have.property('newKey', 'test value');
      expect(sectionChangeEvent?.detail.newFields).to.not.have.property('oldKey');
    });

    it('should update the fields property', () => {
      expect(el.fields).to.have.property('newKey', 'test value');
      expect(el.fields).to.not.have.property('oldKey');
    });
  });

  describe('when field value changes', () => {
    let sectionChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      sectionChangeEvent = null;
      el = await createFixtureWithTimeout(html`<frontmatter-value-section .fields="${{
        testKey: 'original value'
      }}"></frontmatter-value-section>`);
      
      el.addEventListener('section-change', (event) => {
        sectionChangeEvent = event as CustomEvent;
      });

      // Simulate value change
      const valueComponent = el.shadowRoot?.querySelector('frontmatter-value') as HTMLElement & { [key: string]: unknown };
      valueComponent?.dispatchEvent(new CustomEvent('value-change', {
        detail: {
          oldValue: 'original value',
          newValue: 'modified value'
        },
        bubbles: true
      }));
    });

    it('should dispatch section-change event', () => {
      expect(sectionChangeEvent).to.exist;
    });

    it('should include the updated fields with new value', () => {
      expect(sectionChangeEvent?.detail.newFields).to.have.property('testKey', 'modified value');
    });

    it('should update the fields property', () => {
      expect(el.fields).to.have.property('testKey', 'modified value');
    });
  });

  describe('when fields of different types are rendered', () => {
    beforeEach(async () => {
      // Create a complex object with mixed types in non-alphabetical order
      el = await createFixtureWithTimeout(html`<frontmatter-value-section .fields="${{
        zebra_section: { nested: 'value' },
        apple_field: 'string value',
        orange_array: ['item1', 'item2'],
        banana_field: 'another string',
        charlie_section: { another: 'nested' },
        delta_array: ['item3']
      }}"></frontmatter-value-section>`);
    });

    it('should render fields sorted by type then alphabetically', () => {
      const fieldRows = el.shadowRoot?.querySelectorAll('.field-row') as NodeListOf<Element>;
      const keys: string[] = [];
      
      fieldRows.forEach(row => {
        const keyComponent = row.querySelector('frontmatter-key') as HTMLElement & { key: string };
        keys.push(keyComponent.key);
      });

      // Expected order: strings first (alphabetical), then arrays (alphabetical), then objects (alphabetical)
      expect(keys).to.deep.equal([
        'apple_field',     // string
        'banana_field',    // string
        'delta_array',     // array
        'orange_array',    // array
        'charlie_section', // object
        'zebra_section'    // object
      ]);
    });

    it('should render appropriate components for each type', () => {
      const fieldRows = el.shadowRoot?.querySelectorAll('.field-row') as NodeListOf<Element>;
      
      // Check first field (apple_field - string)
      const firstValueComponent = fieldRows[0].querySelector('frontmatter-value') as HTMLElement;
      expect(firstValueComponent.shadowRoot?.querySelector('frontmatter-value-string')).to.exist;
      
      // Check third field (delta_array - array)  
      const thirdValueComponent = fieldRows[2].querySelector('frontmatter-value') as HTMLElement;
      expect(thirdValueComponent.shadowRoot?.querySelector('frontmatter-value-array')).to.exist;
      
      // Check last field (zebra_section - object)
      const lastValueComponent = fieldRows[fieldRows.length - 1].querySelector('frontmatter-value') as HTMLElement;
      expect(lastValueComponent.shadowRoot?.querySelector('frontmatter-value-section')).to.exist;
    });
  });

  describe('when disabled is true', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-section .fields="${{test: 'value'}}" disabled></frontmatter-value-section>`);
    });

    it('should disable all key components', () => {
      const keyComponents = el.shadowRoot?.querySelectorAll('frontmatter-key') as NodeListOf<Element>;
      keyComponents.forEach(component => {
        expect(component.editable).to.be.false;
      });
    });

    it('should disable all value components', () => {
      const valueComponents = el.shadowRoot?.querySelectorAll('frontmatter-value-string') as NodeListOf<Element>;
      valueComponents.forEach(component => {
        expect(component.disabled).to.be.true;
      });
    });

    it('should disable the add field button', () => {
      const addButton = el.shadowRoot?.querySelector('frontmatter-add-field-button') as HTMLElement & {disabled: boolean};
      expect(addButton?.disabled).to.be.true;
    });

    it('should disable all remove buttons', () => {
      const removeButtons = el.shadowRoot?.querySelectorAll('.remove-field-button') as NodeListOf<HTMLButtonElement>;
      removeButtons.forEach(button => {
        expect(button.disabled).to.be.true;
      });
    });
  });

  describe('when fields property is updated programmatically', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-section .fields="${{initial: 'value'}}"></frontmatter-value-section>`);
    });

    describe('when fields object changes', () => {
      beforeEach(async () => {
        el.fields = {
          updated1: 'value1',
          updated2: 'value2'
        };
        await el.updateComplete;
      });

      it('should render new number of field rows', () => {
        const fieldRows = el.shadowRoot?.querySelectorAll('.field-row');
        expect(fieldRows?.length).to.equal(2);
      });

      it('should update key components', () => {
        const keyComponents = el.shadowRoot?.querySelectorAll('frontmatter-key') as NodeListOf<Element>;
        const keys = Array.from(keyComponents).map(comp => comp.key);
        expect(keys).to.include.members(['updated1', 'updated2']);
      });

      it('should update value components', () => {
        const valueDispatcherComponents = el.shadowRoot?.querySelectorAll('frontmatter-value') as NodeListOf<HTMLElement & {value: unknown}>;
        const values = Array.from(valueDispatcherComponents).map(comp => comp.value);
        expect(values).to.include.members(['value1', 'value2']);
      });
    });
  });
});