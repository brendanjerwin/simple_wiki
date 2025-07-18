import { html, fixture, expect, waitUntil } from '@open-wc/testing';
import { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';
import './frontmatter-editor-dialog.js';

describe('FrontmatterEditorDialog - Array and Section Handling', () => {
  let el: FrontmatterEditorDialog;

  beforeEach(async () => {
    el = await fixture(html`<frontmatter-editor-dialog></frontmatter-editor-dialog>`);
    await el.updateComplete;
  });

  describe('when handling arrays', () => {
    beforeEach(async () => {
      // Set up test data with arrays
      el.workingFrontmatter = {
        identifier: 'inventory_item',
        title: 'Inventory Item',
        rename_this_section: {
          total: '32'
        },
        inventory: {
          container: 'lab_small_parts',
          items: [
            'AKG Wired Earbuds',
            'Steel Series Arctis 5 Headphone 3.5mm Adapter Cable',
            'Steel Series Arctis 5 Headphone USB Dongle'
          ]
        }
      };
      el.open = true;
      await el.updateComplete;
    });

    it('should render array sections with proper styling', () => {
      const arraySection = el.shadowRoot?.querySelector('.array-section');
      expect(arraySection).to.exist;
      expect(arraySection?.textContent).to.include('items (Array)');
    });

    it('should render individual array items', () => {
      const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
      expect(arrayItems).to.have.length(3);
      
      const firstItem = arrayItems?.[0];
      const input = firstItem?.querySelector('input[type="text"]') as HTMLInputElement;
      expect(input?.value).to.equal('AKG Wired Earbuds');
    });

    it('should have add item button for arrays', () => {
      const addButton = el.shadowRoot?.querySelector('.array-section .add-field-button');
      expect(addButton).to.exist;
      expect(addButton?.textContent?.trim()).to.equal('Add Item');
    });

    it('should have remove buttons for each array item', () => {
      const removeButtons = el.shadowRoot?.querySelectorAll('.array-item .remove-field-button');
      expect(removeButtons).to.have.length(3);
      removeButtons?.forEach(button => {
        expect(button.textContent?.trim()).to.equal('Remove');
      });
    });

    describe('when add item is clicked', () => {
      let addButton: Element;

      beforeEach(async () => {
        addButton = el.shadowRoot?.querySelector('.array-section .add-field-button') as Element;
        addButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should add a new empty item to the array', () => {
        const items = (el.workingFrontmatter as any)?.inventory?.items;
        expect(items).to.have.length(4);
        expect(items[3]).to.equal('');
      });

      it('should render the new array item', () => {
        const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
        expect(arrayItems).to.have.length(4);
      });
    });

    describe('when remove item is clicked', () => {
      let removeButton: Element;

      beforeEach(async () => {
        removeButton = el.shadowRoot?.querySelector('.array-item .remove-field-button') as Element;
        removeButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should remove the item from the array', () => {
        const items = (el.workingFrontmatter as any)?.inventory?.items;
        expect(items).to.have.length(2);
        expect(items[0]).to.equal('Steel Series Arctis 5 Headphone 3.5mm Adapter Cable');
      });

      it('should update the rendered array items', () => {
        const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
        expect(arrayItems).to.have.length(2);
      });
    });

    describe('when an array item is changed', () => {
      let firstItemInput: HTMLInputElement;

      beforeEach(async () => {
        firstItemInput = el.shadowRoot?.querySelector('.array-item input[type="text"]') as HTMLInputElement;
        firstItemInput.value = 'Updated Earbuds';
        firstItemInput.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
      });

      it('should update the array value in working frontmatter', () => {
        const items = (el.workingFrontmatter as any)?.inventory?.items;
        expect(items[0]).to.equal('Updated Earbuds');
      });
    });
  });

  describe('when managing section names', () => {
    beforeEach(async () => {
      el.workingFrontmatter = {
        rename_this_section: {
          total: '32'
        }
      };
      el.open = true;
      await el.updateComplete;
    });

    it('should render section name as editable input', () => {
      const sectionInput = el.shadowRoot?.querySelector('.section-title-input') as HTMLInputElement;
      expect(sectionInput).to.exist;
      expect(sectionInput?.value).to.equal('rename_this_section');
    });

    describe('when section name is changed', () => {
      let sectionInput: HTMLInputElement;

      beforeEach(async () => {
        sectionInput = el.shadowRoot?.querySelector('.section-title-input') as HTMLInputElement;
        sectionInput.value = 'new_section_name';
        sectionInput.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
      });

      it('should update the frontmatter with new section name', () => {
        expect(el.workingFrontmatter).to.have.property('new_section_name');
        expect(el.workingFrontmatter).to.not.have.property('rename_this_section');
      });

      it('should preserve the section content', () => {
        expect((el.workingFrontmatter as any)?.new_section_name?.total).to.equal('32');
      });
    });
  });

  describe('when working with complex frontmatter structure', () => {
    beforeEach(async () => {
      // Use the exact structure from the user's comment
      el.workingFrontmatter = {
        identifier: 'inventory_item',
        title: 'Inventory Item',
        rename_this_section: {
          total: '32'
        },
        inventory: {
          container: 'lab_small_parts',
          items: [
            'AKG Wired Earbuds',
            'Steel Series Arctis 5 Headphone 3.5mm Adapter Cable',
            'Steel Series Arctis 5 Headphone USB Dongle',
            'Male 3.5mm to Male 3.5mm Coiled Cable',
            'Random Earbud Tips',
            '3.5mm to RCA Cable',
            'Male 3.5mm to Male 3.5mm Cable'
          ]
        }
      };
      el.open = true;
      await el.updateComplete;
    });

    it('should render all top-level fields', () => {
      const fields = el.shadowRoot?.querySelectorAll('.form-field');
      // Should have at least identifier and title fields
      expect(fields?.length).to.be.greaterThan(1);
    });

    it('should render nested sections', () => {
      const sections = el.shadowRoot?.querySelectorAll('.field-section');
      // Should have rename_this_section and inventory sections, plus the array section
      expect(sections?.length).to.be.greaterThan(1);
    });

    it('should render the inventory array with all items', () => {
      const arrayItems = el.shadowRoot?.querySelectorAll('.array-section .array-item');
      expect(arrayItems).to.have.length(7);
    });

    describe('when editing nested field values', () => {
      let containerInput: HTMLInputElement;

      beforeEach(async () => {
        containerInput = el.shadowRoot?.querySelector('input[name="inventory.container"]') as HTMLInputElement;
        containerInput.value = 'new_container';
        containerInput.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
      });

      it('should update the nested field value', () => {
        expect((el.workingFrontmatter as any)?.inventory?.container).to.equal('new_container');
      });
    });

    describe('when adding items to arrays', () => {
      let addButton: Element;

      beforeEach(async () => {
        addButton = el.shadowRoot?.querySelector('.array-section .add-field-button') as Element;
        addButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should increase the array length', () => {
        const items = (el.workingFrontmatter as any)?.inventory?.items;
        expect(items).to.have.length(8);
      });

      it('should add an empty string as the new item', () => {
        const items = (el.workingFrontmatter as any)?.inventory?.items;
        expect(items[7]).to.equal('');
      });
    });

    describe('when removing items from arrays', () => {
      let removeButton: Element;

      beforeEach(async () => {
        removeButton = el.shadowRoot?.querySelector('.array-item:first-child .remove-field-button') as Element;
        removeButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should decrease the array length', () => {
        const items = (el.workingFrontmatter as any)?.inventory?.items;
        expect(items).to.have.length(6);
      });

      it('should remove the correct item', () => {
        const items = (el.workingFrontmatter as any)?.inventory?.items;
        expect(items[0]).to.equal('Steel Series Arctis 5 Headphone 3.5mm Adapter Cable');
      });
    });

    describe('when modifying array items', () => {
      let thirdItemInput: HTMLInputElement;

      beforeEach(async () => {
        const arrayItems = el.shadowRoot?.querySelectorAll('.array-item input[type="text"]');
        thirdItemInput = arrayItems?.[2] as HTMLInputElement;
        thirdItemInput.value = 'Modified USB Dongle';
        thirdItemInput.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
      });

      it('should update the specific array item', () => {
        const items = (el.workingFrontmatter as any)?.inventory?.items;
        expect(items[2]).to.equal('Modified USB Dongle');
      });

      it('should not affect other array items', () => {
        const items = (el.workingFrontmatter as any)?.inventory?.items;
        expect(items[0]).to.equal('AKG Wired Earbuds');
        expect(items[1]).to.equal('Steel Series Arctis 5 Headphone 3.5mm Adapter Cable');
      });
    });
  });
});