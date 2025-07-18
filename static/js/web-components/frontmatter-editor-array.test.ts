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

    it('should handle array item changes', async () => {
      const firstItemInput = el.shadowRoot?.querySelector('.array-item input[type="text"]') as HTMLInputElement;
      expect(firstItemInput).to.exist;
      
      firstItemInput.value = 'Updated Earbuds';
      firstItemInput.dispatchEvent(new Event('input', { bubbles: true }));
      
      await el.updateComplete;
      
      const items = (el.workingFrontmatter as any)?.inventory?.items;
      expect(items[0]).to.equal('Updated Earbuds');
    });
  });

  describe('when handling section names', () => {
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

    it('should allow changing section names', async () => {
      const sectionInput = el.shadowRoot?.querySelector('.section-title-input') as HTMLInputElement;
      expect(sectionInput).to.exist;
      
      sectionInput.value = 'new_section_name';
      sectionInput.dispatchEvent(new Event('input', { bubbles: true }));
      
      await el.updateComplete;
      
      expect(el.workingFrontmatter).to.have.property('new_section_name');
      expect(el.workingFrontmatter).to.not.have.property('rename_this_section');
      expect((el.workingFrontmatter as any)?.new_section_name?.total).to.equal('32');
    });
  });

  describe('when handling complex frontmatter structure', () => {
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

    it('should allow editing nested field values', async () => {
      const containerInput = el.shadowRoot?.querySelector('input[name="inventory.container"]') as HTMLInputElement;
      expect(containerInput).to.exist;
      expect(containerInput?.value).to.equal('lab_small_parts');
      
      containerInput.value = 'new_container';
      containerInput.dispatchEvent(new Event('input', { bubbles: true }));
      
      await el.updateComplete;
      
      expect((el.workingFrontmatter as any)?.inventory?.container).to.equal('new_container');
    });
  });
});