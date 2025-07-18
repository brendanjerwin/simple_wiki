import { html, fixture, expect, waitUntil } from '@open-wc/testing';
import { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';
import sinon from 'sinon';
import './frontmatter-editor-dialog.js';

describe('FrontmatterEditorDialog - Editable Features', () => {
  let el: FrontmatterEditorDialog;

  beforeEach(async () => {
    el = await fixture(html`<frontmatter-editor-dialog></frontmatter-editor-dialog>`);
    await el.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
  });

  describe('when editing field keys', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {
        identifier: 'test_item',
        title: 'Test Title',
        description: 'Test Description'
      };
      el.open = true;
      await el.updateComplete;
    });

    describe('when a field key is changed', () => {
      let keyInput: HTMLInputElement;
      let valueInput: HTMLInputElement;

      beforeEach(async () => {
        const keyValueRow = el.shadowRoot?.querySelector('.key-value-row');
        keyInput = keyValueRow?.querySelector('.key-input') as HTMLInputElement;
        valueInput = keyValueRow?.querySelector('.value-input') as HTMLInputElement;
        
        // Change the key from 'identifier' to 'id'
        keyInput.value = 'id';
        keyInput.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
      });

      it('should update the key in the data model', () => {
        expect(el.workingFrontmatter).to.have.property('id');
        expect(el.workingFrontmatter).to.not.have.property('identifier');
      });

      it('should preserve the original value', () => {
        expect(el.workingFrontmatter.id).to.equal('test_item');
      });

      it('should preserve other fields unchanged', () => {
        expect(el.workingFrontmatter.title).to.equal('Test Title');
        expect(el.workingFrontmatter.description).to.equal('Test Description');
      });
    });

    describe('when a field key is changed to empty value', () => {
      let keyInput: HTMLInputElement;

      beforeEach(async () => {
        const keyValueRow = el.shadowRoot?.querySelector('.key-value-row');
        keyInput = keyValueRow?.querySelector('.key-input') as HTMLInputElement;
        
        // Change the key to empty string
        keyInput.value = '';
        keyInput.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
      });

      it('should not update the data model', () => {
        expect(el.workingFrontmatter).to.have.property('identifier');
        expect(el.workingFrontmatter.identifier).to.equal('test_item');
      });
    });

    describe('when a field key is changed to only whitespace', () => {
      let keyInput: HTMLInputElement;

      beforeEach(async () => {
        const keyValueRow = el.shadowRoot?.querySelector('.key-value-row');
        keyInput = keyValueRow?.querySelector('.key-input') as HTMLInputElement;
        
        // Change the key to whitespace
        keyInput.value = '   ';
        keyInput.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
      });

      it('should not update the data model', () => {
        expect(el.workingFrontmatter).to.have.property('identifier');
        expect(el.workingFrontmatter.identifier).to.equal('test_item');
      });
    });

    describe('when a field key is changed to the same value', () => {
      let keyInput: HTMLInputElement;

      beforeEach(async () => {
        const keyValueRow = el.shadowRoot?.querySelector('.key-value-row');
        keyInput = keyValueRow?.querySelector('.key-input') as HTMLInputElement;
        
        // Change the key to the same value
        keyInput.value = 'identifier';
        keyInput.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
      });

      it('should not modify the data model', () => {
        expect(el.workingFrontmatter).to.have.property('identifier');
        expect(el.workingFrontmatter.identifier).to.equal('test_item');
        expect(Object.keys(el.workingFrontmatter)).to.have.length(3);
      });
    });

    describe('when multiple field keys are changed', () => {
      beforeEach(async () => {
        // First change: 'identifier' to 'id'
        let keyValueRows = el.shadowRoot?.querySelectorAll('.key-value-row');
        let firstKeyInput = keyValueRows?.[0]?.querySelector('.key-input') as HTMLInputElement;
        expect(firstKeyInput.value).to.equal('identifier'); // Verify we're changing the right field
        firstKeyInput.value = 'id';
        firstKeyInput.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
        
        // Second change: need to re-query after the DOM update
        keyValueRows = el.shadowRoot?.querySelectorAll('.key-value-row');
        // Find the 'title' field after the DOM has been updated
        let titleKeyInput: HTMLInputElement | undefined;
        for (const row of Array.from(keyValueRows || [])) {
          const keyInput = row.querySelector('.key-input') as HTMLInputElement;
          if (keyInput.value === 'title') {
            titleKeyInput = keyInput;
            break;
          }
        }
        expect(titleKeyInput).to.exist;
        titleKeyInput!.value = 'name';
        titleKeyInput!.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
      });

      it('should update both keys in the data model', () => {
        expect(el.workingFrontmatter).to.have.property('id');
        expect(el.workingFrontmatter).to.have.property('name');
        expect(el.workingFrontmatter).to.not.have.property('identifier');
        expect(el.workingFrontmatter).to.not.have.property('title');
      });

      it('should preserve the values for both changed keys', () => {
        expect(el.workingFrontmatter.id).to.equal('test_item');
        expect(el.workingFrontmatter.name).to.equal('Test Title');
      });

      it('should preserve unchanged fields', () => {
        expect(el.workingFrontmatter.description).to.equal('Test Description');
      });
    });
  });

  describe('when editing nested field keys', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {
        inventory: {
          container: 'lab_small_parts',
          location: 'shelf_a'
        }
      };
      el.open = true;
      await el.updateComplete;
    });

    describe('when a nested field key is changed', () => {
      beforeEach(async () => {
        const keyValueRows = el.shadowRoot?.querySelectorAll('.key-value-row');
        const nestedKeyInput = keyValueRows?.[0]?.querySelector('.key-input') as HTMLInputElement;
        
        // Change nested key from 'container' to 'box'
        nestedKeyInput.value = 'box';
        nestedKeyInput.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
      });

      it('should update the nested key in the data model', () => {
        expect(el.workingFrontmatter.inventory).to.have.property('box');
        expect(el.workingFrontmatter.inventory).to.not.have.property('container');
      });

      it('should preserve the nested value', () => {
        expect(el.workingFrontmatter.inventory.box).to.equal('lab_small_parts');
      });

      it('should preserve other nested fields', () => {
        expect(el.workingFrontmatter.inventory.location).to.equal('shelf_a');
      });
    });
  });

  describe('when using top-level dropdown controls', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {
        existing_field: 'value'
      };
      el.open = true;
      await el.updateComplete;
    });

    describe('when dropdown button is clicked', () => {
      let dropdownButton: HTMLButtonElement;

      beforeEach(async () => {
        dropdownButton = el.shadowRoot?.querySelector('.dropdown-button') as HTMLButtonElement;
        dropdownButton.click();
        await el.updateComplete;
      });

      it('should open the dropdown menu', () => {
        expect(el.dropdownOpen).to.be.true;
      });

      it('should display dropdown menu options', () => {
        const dropdownMenu = el.shadowRoot?.querySelector('.dropdown-menu');
        expect(dropdownMenu).to.exist;
        
        const options = dropdownMenu?.querySelectorAll('.dropdown-item');
        expect(options).to.have.length(3);
        expect(options?.[0]?.textContent).to.equal('Add Field');
        expect(options?.[1]?.textContent).to.equal('Add Array');
        expect(options?.[2]?.textContent).to.equal('Add Section');
      });
    });

    describe('when dropdown is already open and button is clicked again', () => {
      beforeEach(async () => {
        el.dropdownOpen = true;
        await el.updateComplete;
        
        const dropdownButton = el.shadowRoot?.querySelector('.dropdown-button') as HTMLButtonElement;
        dropdownButton.click();
        await el.updateComplete;
      });

      it('should close the dropdown menu', () => {
        expect(el.dropdownOpen).to.be.false;
      });

      it('should hide dropdown menu options', () => {
        const dropdownMenu = el.shadowRoot?.querySelector('.dropdown-menu');
        expect(dropdownMenu).to.not.exist;
      });
    });

    describe('when "Add Field" dropdown option is clicked', () => {
      beforeEach(async () => {
        const dropdownButton = el.shadowRoot?.querySelector('.dropdown-button') as HTMLButtonElement;
        dropdownButton.click();
        await el.updateComplete;
        
        const addFieldOption = el.shadowRoot?.querySelector('.dropdown-item') as HTMLButtonElement;
        addFieldOption.click();
        await el.updateComplete;
      });

      it('should add a new field to the data model', () => {
        expect(el.workingFrontmatter).to.have.property('new_field');
        expect(el.workingFrontmatter.new_field).to.equal('');
      });

      it('should close the dropdown', () => {
        expect(el.dropdownOpen).to.be.false;
      });

      it('should preserve existing fields', () => {
        expect(el.workingFrontmatter.existing_field).to.equal('value');
      });
    });

    describe('when "Add Array" dropdown option is clicked', () => {
      beforeEach(async () => {
        const dropdownButton = el.shadowRoot?.querySelector('.dropdown-button') as HTMLButtonElement;
        dropdownButton.click();
        await el.updateComplete;
        
        const addArrayOption = el.shadowRoot?.querySelectorAll('.dropdown-item')[1] as HTMLButtonElement;
        addArrayOption.click();
        await el.updateComplete;
      });

      it('should add a new array to the data model', () => {
        expect(el.workingFrontmatter).to.have.property('new_array');
        expect(Array.isArray(el.workingFrontmatter.new_array)).to.be.true;
        expect(el.workingFrontmatter.new_array).to.have.length(0);
      });

      it('should close the dropdown', () => {
        expect(el.dropdownOpen).to.be.false;
      });

      it('should preserve existing fields', () => {
        expect(el.workingFrontmatter.existing_field).to.equal('value');
      });
    });

    describe('when "Add Section" dropdown option is clicked', () => {
      beforeEach(async () => {
        const dropdownButton = el.shadowRoot?.querySelector('.dropdown-button') as HTMLButtonElement;
        dropdownButton.click();
        await el.updateComplete;
        
        const addSectionOption = el.shadowRoot?.querySelectorAll('.dropdown-item')[2] as HTMLButtonElement;
        addSectionOption.click();
        await el.updateComplete;
      });

      it('should add a new section to the data model', () => {
        expect(el.workingFrontmatter).to.have.property('new_section');
        expect(typeof el.workingFrontmatter.new_section).to.equal('object');
        expect(el.workingFrontmatter.new_section).to.not.be.null;
        expect(Array.isArray(el.workingFrontmatter.new_section)).to.be.false;
      });

      it('should close the dropdown', () => {
        expect(el.dropdownOpen).to.be.false;
      });

      it('should preserve existing fields', () => {
        expect(el.workingFrontmatter.existing_field).to.equal('value');
      });
    });

    describe('when adding multiple fields with unique keys', () => {
      beforeEach(async () => {
        // Add first field
        const dropdownButton = el.shadowRoot?.querySelector('.dropdown-button') as HTMLButtonElement;
        dropdownButton.click();
        await el.updateComplete;
        
        const addFieldOption = el.shadowRoot?.querySelector('.dropdown-item') as HTMLButtonElement;
        addFieldOption.click();
        await el.updateComplete;
        
        // Add second field
        dropdownButton.click();
        await el.updateComplete;
        
        const addFieldOption2 = el.shadowRoot?.querySelector('.dropdown-item') as HTMLButtonElement;
        addFieldOption2.click();
        await el.updateComplete;
      });

      it('should generate unique keys for multiple additions', () => {
        expect(el.workingFrontmatter).to.have.property('new_field');
        expect(el.workingFrontmatter).to.have.property('new_field_1');
      });

      it('should preserve the original field', () => {
        expect(el.workingFrontmatter.existing_field).to.equal('value');
      });
    });
  });

  describe('when using top-level dropdown with empty frontmatter', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {};
      el.open = true;
      await el.updateComplete;
    });

    it('should display the dropdown controls', () => {
      const topLevelControls = el.shadowRoot?.querySelector('.top-level-controls');
      expect(topLevelControls).to.exist;
      
      const dropdownButton = el.shadowRoot?.querySelector('.dropdown-button');
      expect(dropdownButton).to.exist;
      expect(dropdownButton?.textContent?.trim()).to.equal('Add Field ▼');
    });

    it('should display helpful message when no frontmatter exists', () => {
      const loadingDiv = el.shadowRoot?.querySelector('.loading');
      expect(loadingDiv?.textContent).to.include('No frontmatter to edit - use "Add Field" to get started');
    });

    describe('when adding first field to empty frontmatter', () => {
      beforeEach(async () => {
        const dropdownButton = el.shadowRoot?.querySelector('.dropdown-button') as HTMLButtonElement;
        dropdownButton.click();
        await el.updateComplete;
        
        const addFieldOption = el.shadowRoot?.querySelector('.dropdown-item') as HTMLButtonElement;
        addFieldOption.click();
        await el.updateComplete;
      });

      it('should add the field to previously empty frontmatter', () => {
        expect(el.workingFrontmatter).to.have.property('new_field');
        expect(Object.keys(el.workingFrontmatter)).to.have.length(1);
      });

      it('should render the frontmatter editor instead of the empty message', () => {
        const frontmatterEditor = el.shadowRoot?.querySelector('.frontmatter-editor');
        expect(frontmatterEditor).to.exist;
        
        const loadingDiv = el.shadowRoot?.querySelector('.loading');
        expect(loadingDiv).to.not.exist;
      });
    });
  });

  describe('when using remove buttons for top-level fields', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {
        identifier: 'test_item',
        title: 'Test Title',
        description: 'Test Description'
      };
      el.open = true;
      await el.updateComplete;
    });

    describe('when remove button is clicked for a top-level field', () => {
      beforeEach(async () => {
        const keyValueRow = el.shadowRoot?.querySelector('.key-value-row');
        const removeButton = keyValueRow?.querySelector('.remove-field-button') as HTMLButtonElement;
        removeButton.click();
        await el.updateComplete;
      });

      it('should remove the field from the data model', () => {
        expect(el.workingFrontmatter).to.not.have.property('identifier');
      });

      it('should preserve other fields', () => {
        expect(el.workingFrontmatter.title).to.equal('Test Title');
        expect(el.workingFrontmatter.description).to.equal('Test Description');
      });

      it('should reduce the total number of fields', () => {
        expect(Object.keys(el.workingFrontmatter)).to.have.length(2);
      });
    });

    describe('when all top-level fields are removed', () => {
      beforeEach(async () => {
        // Remove fields one by one, re-querying after each removal
        while (Object.keys(el.workingFrontmatter).length > 0) {
          const removeButtons = el.shadowRoot?.querySelectorAll('.remove-field-button');
          if (removeButtons && removeButtons.length > 0) {
            (removeButtons[0] as HTMLButtonElement).click();
            await el.updateComplete;
          } else {
            break;
          }
        }
      });

      it('should result in empty frontmatter', () => {
        expect(Object.keys(el.workingFrontmatter)).to.have.length(0);
      });

      it('should display the empty frontmatter message', () => {
        const loadingDiv = el.shadowRoot?.querySelector('.loading');
        expect(loadingDiv).to.exist;
        expect(loadingDiv?.textContent).to.include('No frontmatter to edit - use "Add Field" to get started');
      });
    });
  });

  describe('when rendering arrays without pseudo-names', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {
        items: [
          'AKG Wired Earbuds',
          'Steel Series Adapter',
          'USB Dongle'
        ]
      };
      el.open = true;
      await el.updateComplete;
    });

    it('should render array items without item labels', () => {
      const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
      expect(arrayItems).to.have.length(3);
      
      // Check that each array item contains just the input without "Item 1", "Item 2" labels
      arrayItems?.forEach((item, index) => {
        const input = item.querySelector('input[type="text"]') as HTMLInputElement;
        expect(input?.value).to.equal(['AKG Wired Earbuds', 'Steel Series Adapter', 'USB Dongle'][index]);
        
        // Verify no item label text exists
        const textContent = item.textContent || '';
        expect(textContent).to.not.include('Item 1');
        expect(textContent).to.not.include('Item 2');
        expect(textContent).to.not.include('Item 3');
      });
    });

    it('should render array section with proper header', () => {
      const arraySection = el.shadowRoot?.querySelector('.array-section');
      expect(arraySection).to.exist;
      
      const sectionHeader = arraySection?.querySelector('.section-header');
      expect(sectionHeader).to.exist;
    });

    describe('when array items are modified', () => {
      beforeEach(async () => {
        const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
        const firstInput = arrayItems?.[0]?.querySelector('input[type="text"]') as HTMLInputElement;
        
        firstInput.value = 'Modified Earbuds';
        firstInput.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
      });

      it('should update the array value in the data model', () => {
        expect(el.workingFrontmatter.items[0]).to.equal('Modified Earbuds');
      });

      it('should preserve other array items', () => {
        expect(el.workingFrontmatter.items[1]).to.equal('Steel Series Adapter');
        expect(el.workingFrontmatter.items[2]).to.equal('USB Dongle');
      });
    });
  });

  describe('when using remove buttons for top-level sections', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {
        simple_field: 'value',
        inventory: {
          container: 'lab_small_parts',
          location: 'shelf_a'
        },
        another_section: {
          key: 'value'
        }
      };
      el.open = true;
      await el.updateComplete;
    });

    describe('when remove section button is clicked', () => {
      beforeEach(async () => {
        const removeSectionButton = el.shadowRoot?.querySelector('.remove-section-button') as HTMLButtonElement;
        removeSectionButton.click();
        await el.updateComplete;
      });

      it('should remove the entire section from the data model', () => {
        expect(el.workingFrontmatter).to.not.have.property('inventory');
      });

      it('should preserve other fields and sections', () => {
        expect(el.workingFrontmatter.simple_field).to.equal('value');
        expect(el.workingFrontmatter.another_section).to.deep.equal({ key: 'value' });
      });
    });
  });

  describe('when dropdown button prevents event bubbling', () => {
    beforeEach(async () => {
      el.workingFrontmatter = { test: 'value' };
      el.open = true;
      await el.updateComplete;
    });

    it('should prevent click event from bubbling when dropdown button is clicked', () => {
      const dropdownButton = el.shadowRoot?.querySelector('.dropdown-button') as HTMLButtonElement;
      const clickEvent = new Event('click', { bubbles: true, cancelable: true });
      
      let eventBubbled = false;
      el.addEventListener('click', () => {
        eventBubbled = true;
      });
      
      dropdownButton.dispatchEvent(clickEvent);
      
      expect(clickEvent.defaultPrevented).to.be.false;
      // The stopPropagation should prevent the event from reaching the parent
      expect(eventBubbled).to.be.false;
    });
  });
});