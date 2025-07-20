import { html, fixture, expect } from '@open-wc/testing';
import { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';
import sinon from 'sinon';
import './frontmatter-editor-dialog.js';

describe('FrontmatterEditorDialog - Component Integration', () => {
  let el: FrontmatterEditorDialog;

  function timeout(ms: number, message: string): Promise<never> {
    return new Promise((_, reject) => 
      setTimeout(() => reject(new Error(message)), ms)
    );
  }

  beforeEach(async () => {
    el = await Promise.race([
      fixture(html`<frontmatter-editor-dialog></frontmatter-editor-dialog>`),
      timeout(5000, 'Component fixture timed out')
    ]);
    
    sinon.stub(el, 'loadFrontmatter').resolves();
    await el.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
    if (el) {
      el.remove();
    }
  });

  describe('when rendering with frontmatter data', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {
        title: 'Test Title',
        items: ['item1', 'item2'],
        section: { key1: 'value1' }
      };
      el.open = true;
      await el.updateComplete;
    });

    it('should render sub-components for fields', () => {
      const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section') as HTMLElement & {shadowRoot: ShadowRoot};
      const keyComponents = sectionComponent?.shadowRoot?.querySelectorAll('frontmatter-key');
      const valueComponents = sectionComponent?.shadowRoot?.querySelectorAll('frontmatter-value');
      
      expect(keyComponents).to.have.length.greaterThan(0);
      expect(valueComponents).to.have.length.greaterThan(0);
    });

    it('should render remove buttons for top-level fields', () => {
      const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section') as HTMLElement & {shadowRoot: ShadowRoot};
      const removeButtons = sectionComponent?.shadowRoot?.querySelectorAll('.remove-field-button');
      expect(removeButtons).to.have.length.greaterThan(0);
    });
  });

  describe('when key changes occur', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {
        identifier: 'test_item',
        title: 'Test Title'
      };
      el.open = true;
      await el.updateComplete;
    });

    describe('when a key is renamed via component event', () => {
      beforeEach(async () => {
        const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section') as HTMLElement & {shadowRoot: ShadowRoot};
        const keyChangeEvent = new CustomEvent('key-change', {
          detail: { oldKey: 'identifier', newKey: 'id' },
          bubbles: true
        });
        
        const keyComponent = sectionComponent?.shadowRoot?.querySelector('frontmatter-key');
        keyComponent?.dispatchEvent(keyChangeEvent);
        await el.updateComplete;
      });

      it('should update the data model with new key', () => {
        expect(el.workingFrontmatter).to.have.property('id');
        expect(el.workingFrontmatter).to.not.have.property('identifier');
      });

      it('should preserve the value for the renamed key', () => {
        expect(el.workingFrontmatter.id).to.equal('test_item');
      });

      it('should preserve other fields unchanged', () => {
        expect(el.workingFrontmatter.title).to.equal('Test Title');
      });
    });
  });

  describe('when value changes occur', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {
        title: 'Original Title'
      };
      el.open = true;
      await el.updateComplete;
    });

    describe('when a value is changed via component event', () => {
      beforeEach(async () => {
        const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section') as HTMLElement & {shadowRoot: ShadowRoot};
        const valueChangeEvent = new CustomEvent('value-change', {
          detail: { oldValue: 'Original Title', newValue: 'New Title' },
          bubbles: true
        });
        
        const valueComponent = sectionComponent?.shadowRoot?.querySelector('frontmatter-value');
        valueComponent?.dispatchEvent(valueChangeEvent);
        await el.updateComplete;
      });

      it('should update the data model with new value', () => {
        expect(el.workingFrontmatter.title).to.equal('New Title');
      });
    });
  });

  describe('when fields are removed', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {
        field1: 'value1',
        field2: 'value2',
        field3: 'value3'
      };
      el.open = true;
      await el.updateComplete;
    });

    describe('when remove button is clicked', () => {
      beforeEach(async () => {
        const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section') as HTMLElement & {shadowRoot: ShadowRoot};
        const removeButton = sectionComponent?.shadowRoot?.querySelector('.remove-field-button') as HTMLButtonElement;
        removeButton?.click();
        await el.updateComplete;
      });

      it('should remove a field from the data model', () => {
        expect(Object.keys(el.workingFrontmatter)).to.have.length(2);
      });

      it('should preserve remaining fields', () => {
        const keys = Object.keys(el.workingFrontmatter);
        expect(keys.length).to.equal(2);
        keys.forEach(key => {
          expect(['field1', 'field2', 'field3']).to.include(key);
        });
      });
    });
  });

  describe('when adding new fields', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {};
      el.open = true;
      await el.updateComplete;
    });

    describe('when Add Field dropdown option is selected', () => {
      beforeEach(async () => {
        // Navigate to the add field button through the component hierarchy
        const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section') as HTMLElement & {shadowRoot: ShadowRoot};
        const addFieldButton = sectionComponent?.shadowRoot?.querySelector('frontmatter-add-field-button') as HTMLElement & {shadowRoot: ShadowRoot, updateComplete: Promise<unknown>};
        const dropdownButton = addFieldButton?.shadowRoot?.querySelector('button') as HTMLButtonElement;
        
        dropdownButton?.click();
        await addFieldButton?.updateComplete;

        const addFieldOption = addFieldButton?.shadowRoot?.querySelector('.dropdown-item') as HTMLButtonElement;
        addFieldOption?.click();
        await el.updateComplete;
      });

      it('should add a new field to the data model', () => {
        expect(Object.keys(el.workingFrontmatter)).to.have.length(1);
      });

      it('should create a string field by default', () => {
        const keys = Object.keys(el.workingFrontmatter);
        expect(typeof el.workingFrontmatter[keys[0]]).to.equal('string');
      });

      it('should render the new field with sub-components', () => {
        const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section') as HTMLElement & {shadowRoot: ShadowRoot};
        const keyComponents = sectionComponent?.shadowRoot?.querySelectorAll('frontmatter-key');
        const valueComponents = sectionComponent?.shadowRoot?.querySelectorAll('frontmatter-value');
        
        expect(keyComponents).to.have.length(1);
        expect(valueComponents).to.have.length(1);
      });
    });
  });

  describe('when working with complex data structures', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {
        simple_field: 'value',
        array_field: ['item1', 'item2'],
        section_field: {
          nested_key: 'nested_value'
        }
      };
      el.open = true;
      await el.updateComplete;
    });

    it('should render appropriate sub-components for different value types', () => {
      const sectionComponent = el.shadowRoot?.querySelector('frontmatter-value-section') as HTMLElement & {shadowRoot: ShadowRoot};
      const valueComponents = sectionComponent?.shadowRoot?.querySelectorAll('frontmatter-value');
      expect(valueComponents).to.have.length(3); // One for each top-level field
    });

    it('should maintain data integrity when components change', () => {
      expect(el.workingFrontmatter.simple_field).to.equal('value');
      expect(el.workingFrontmatter.array_field).to.deep.equal(['item1', 'item2']);
      expect(el.workingFrontmatter.section_field).to.deep.equal({ nested_key: 'nested_value' });
    });
  });
});