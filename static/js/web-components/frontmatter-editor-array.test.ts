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

    describe('when rendering array UI elements', () => {
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

      describe('when checking control buttons', () => {
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
      });
    });

    describe('when add item is clicked', () => {
      let addButton: Element;

      beforeEach(async () => {
        addButton = el.shadowRoot?.querySelector('.array-section .add-field-button') as Element;
        addButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        await el.updateComplete;
      });

      describe('when checking data changes', () => {
        it('should add a new empty item to the array', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items).to.have.length(4);
          expect(items[3]).to.equal('');
        });

        it('should preserve existing array items', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items[0]).to.equal('AKG Wired Earbuds');
          expect(items[1]).to.equal('Steel Series Arctis 5 Headphone 3.5mm Adapter Cable');
          expect(items[2]).to.equal('Steel Series Arctis 5 Headphone USB Dongle');
        });
      });

      describe('when checking UI updates', () => {
        it('should render the new array item', () => {
          const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
          expect(arrayItems).to.have.length(4);
        });

        it('should render new remove button', () => {
          const removeButtons = el.shadowRoot?.querySelectorAll('.array-item .remove-field-button');
          expect(removeButtons).to.have.length(4);
        });
      });
    });

    describe('when remove item is clicked', () => {
      let removeButton: Element;

      beforeEach(async () => {
        removeButton = el.shadowRoot?.querySelector('.array-item .remove-field-button') as Element;
        removeButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        await el.updateComplete;
      });

      describe('when checking data changes', () => {
        it('should remove the item from the array', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items).to.have.length(2);
          expect(items[0]).to.equal('Steel Series Arctis 5 Headphone 3.5mm Adapter Cable');
        });

        it('should preserve remaining items in correct order', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items[0]).to.equal('Steel Series Arctis 5 Headphone 3.5mm Adapter Cable');
          expect(items[1]).to.equal('Steel Series Arctis 5 Headphone USB Dongle');
        });
      });

      describe('when checking UI updates', () => {
        it('should update the rendered array items', () => {
          const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
          expect(arrayItems).to.have.length(2);
        });

        it('should update remove button count', () => {
          const removeButtons = el.shadowRoot?.querySelectorAll('.array-item .remove-field-button');
          expect(removeButtons).to.have.length(2);
        });
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

      describe('when checking data changes', () => {
        it('should update the array value in working frontmatter', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items[0]).to.equal('Updated Earbuds');
        });

        it('should not affect other array items', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items[1]).to.equal('Steel Series Arctis 5 Headphone 3.5mm Adapter Cable');
          expect(items[2]).to.equal('Steel Series Arctis 5 Headphone USB Dongle');
        });
      });

      describe('when checking UI state', () => {
        it('should maintain correct input value', () => {
          expect(firstItemInput.value).to.equal('Updated Earbuds');
        });

        it('should preserve other input values', () => {
          const secondInput = el.shadowRoot?.querySelectorAll('.array-item input[type="text"]')?.[1] as HTMLInputElement;
          expect(secondInput?.value).to.equal('Steel Series Arctis 5 Headphone 3.5mm Adapter Cable');
        });
      });
    });

    describe('when handling empty arrays', () => {
      beforeEach(async () => {
        el.workingFrontmatter = {
          inventory: {
            items: []
          }
        };
        el.open = true;
        await el.updateComplete;
      });

      it('should render empty array section', () => {
        const arraySection = el.shadowRoot?.querySelector('.array-section');
        expect(arraySection).to.exist;
      });

      it('should show no array items', () => {
        const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
        expect(arrayItems).to.have.length(0);
      });

      it('should still show add button', () => {
        const addButton = el.shadowRoot?.querySelector('.array-section .add-field-button');
        expect(addButton).to.exist;
      });

      describe('when adding first item to empty array', () => {
        beforeEach(async () => {
          const addButton = el.shadowRoot?.querySelector('.array-section .add-field-button') as Element;
          addButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
          await el.updateComplete;
        });

        it('should create array with one item', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items).to.have.length(1);
          expect(items[0]).to.equal('');
        });

        it('should render first array item', () => {
          const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
          expect(arrayItems).to.have.length(1);
        });
      });
    });

    describe('when handling single item arrays', () => {
      beforeEach(async () => {
        el.workingFrontmatter = {
          inventory: {
            items: ['Single Item']
          }
        };
        el.open = true;
        await el.updateComplete;
      });

      it('should render single array item', () => {
        const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
        expect(arrayItems).to.have.length(1);
      });

      describe('when removing the only item', () => {
        beforeEach(async () => {
          const removeButton = el.shadowRoot?.querySelector('.array-item .remove-field-button') as Element;
          removeButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
          await el.updateComplete;
        });

        it('should result in empty array', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items).to.have.length(0);
        });

        it('should show no array items in UI', () => {
          const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
          expect(arrayItems).to.have.length(0);
        });
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

    describe('when rendering section name editor', () => {
      it('should render section name as editable input', () => {
        const sectionInput = el.shadowRoot?.querySelector('.section-title-input') as HTMLInputElement;
        expect(sectionInput).to.exist;
        expect(sectionInput?.value).to.equal('rename_this_section');
      });

      it('should have appropriate input attributes', () => {
        const sectionInput = el.shadowRoot?.querySelector('.section-title-input') as HTMLInputElement;
        expect(sectionInput?.type).to.equal('text');
      });
    });

    describe('when section name is changed', () => {
      let sectionInput: HTMLInputElement;

      beforeEach(async () => {
        sectionInput = el.shadowRoot?.querySelector('.section-title-input') as HTMLInputElement;
        sectionInput.value = 'new_section_name';
        sectionInput.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
      });

      describe('when checking frontmatter structure changes', () => {
        it('should update the frontmatter with new section name', () => {
          expect(el.workingFrontmatter).to.have.property('new_section_name');
          expect(el.workingFrontmatter).to.not.have.property('rename_this_section');
        });

        it('should preserve the section content', () => {
          expect((el.workingFrontmatter as any)?.new_section_name?.total).to.equal('32');
        });
      });

      describe('when checking UI state', () => {
        it('should update the input value', () => {
          expect(sectionInput.value).to.equal('new_section_name');
        });

        it('should maintain input focus state', () => {
          expect(document.activeElement).to.equal(sectionInput);
        });
      });
    });

    describe('when handling edge cases for section names', () => {
      describe('when setting empty section name', () => {
        beforeEach(async () => {
          const sectionInput = el.shadowRoot?.querySelector('.section-title-input') as HTMLInputElement;
          sectionInput.value = '';
          sectionInput.dispatchEvent(new Event('input', { bubbles: true }));
          await el.updateComplete;
        });

        it('should handle empty section name gracefully', () => {
          expect(el.workingFrontmatter).to.not.have.property('rename_this_section');
          expect(Object.keys(el.workingFrontmatter)).to.have.length(1);
        });
      });

      describe('when setting section name with special characters', () => {
        beforeEach(async () => {
          const sectionInput = el.shadowRoot?.querySelector('.section-title-input') as HTMLInputElement;
          sectionInput.value = 'section-with_special.chars';
          sectionInput.dispatchEvent(new Event('input', { bubbles: true }));
          await el.updateComplete;
        });

        it('should preserve special characters in section name', () => {
          expect(el.workingFrontmatter).to.have.property('section-with_special.chars');
          expect((el.workingFrontmatter as any)?.['section-with_special.chars']?.total).to.equal('32');
        });
      });
    });

    describe('when managing multiple sections', () => {
      beforeEach(async () => {
        el.workingFrontmatter = {
          section_one: {
            value: 'first'
          },
          section_two: {
            value: 'second'
          }
        };
        el.open = true;
        await el.updateComplete;
      });

      it('should render multiple section name inputs', () => {
        const sectionInputs = el.shadowRoot?.querySelectorAll('.section-title-input');
        expect(sectionInputs).to.have.length(2);
      });

      describe('when renaming one section', () => {
        beforeEach(async () => {
          const firstSectionInput = el.shadowRoot?.querySelector('.section-title-input') as HTMLInputElement;
          firstSectionInput.value = 'renamed_section';
          firstSectionInput.dispatchEvent(new Event('input', { bubbles: true }));
          await el.updateComplete;
        });

        it('should update only the renamed section', () => {
          expect(el.workingFrontmatter).to.have.property('renamed_section');
          expect(el.workingFrontmatter).to.have.property('section_two');
          expect(el.workingFrontmatter).to.not.have.property('section_one');
        });

        it('should preserve other section content', () => {
          expect((el.workingFrontmatter as any)?.section_two?.value).to.equal('second');
        });
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

    describe('when rendering complex structure elements', () => {
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
    });

    describe('when editing nested field values', () => {
      let containerInput: HTMLInputElement;

      beforeEach(async () => {
        containerInput = el.shadowRoot?.querySelector('input[name="inventory.container"]') as HTMLInputElement;
        containerInput.value = 'new_container';
        containerInput.dispatchEvent(new Event('input', { bubbles: true }));
        await el.updateComplete;
      });

      describe('when checking data integrity', () => {
        it('should update the nested field value', () => {
          expect((el.workingFrontmatter as any)?.inventory?.container).to.equal('new_container');
        });

        it('should preserve other nested fields', () => {
          expect((el.workingFrontmatter as any)?.inventory?.items).to.have.length(7);
        });

        it('should preserve top-level fields', () => {
          expect((el.workingFrontmatter as any)?.identifier).to.equal('inventory_item');
          expect((el.workingFrontmatter as any)?.title).to.equal('Inventory Item');
        });
      });
    });

    describe('when adding items to arrays', () => {
      let addButton: Element;

      beforeEach(async () => {
        addButton = el.shadowRoot?.querySelector('.array-section .add-field-button') as Element;
        addButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        await el.updateComplete;
      });

      describe('when checking array modifications', () => {
        it('should increase the array length', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items).to.have.length(8);
        });

        it('should add an empty string as the new item', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items[7]).to.equal('');
        });

        it('should preserve existing array items', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items[0]).to.equal('AKG Wired Earbuds');
          expect(items[6]).to.equal('Male 3.5mm to Male 3.5mm Cable');
        });
      });

      describe('when checking structure integrity', () => {
        it('should maintain nested structure', () => {
          expect((el.workingFrontmatter as any)?.inventory?.container).to.equal('lab_small_parts');
        });

        it('should preserve other sections', () => {
          expect((el.workingFrontmatter as any)?.rename_this_section?.total).to.equal('32');
        });
      });
    });

    describe('when removing items from arrays', () => {
      let removeButton: Element;

      beforeEach(async () => {
        removeButton = el.shadowRoot?.querySelector('.array-item:first-child .remove-field-button') as Element;
        removeButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        await el.updateComplete;
      });

      describe('when checking array modifications', () => {
        it('should decrease the array length', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items).to.have.length(6);
        });

        it('should remove the correct item', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items[0]).to.equal('Steel Series Arctis 5 Headphone 3.5mm Adapter Cable');
        });

        it('should maintain order of remaining items', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items[5]).to.equal('Male 3.5mm to Male 3.5mm Cable');
        });
      });

      describe('when checking structure integrity', () => {
        it('should preserve nested container field', () => {
          expect((el.workingFrontmatter as any)?.inventory?.container).to.equal('lab_small_parts');
        });

        it('should preserve top-level structure', () => {
          expect((el.workingFrontmatter as any)?.identifier).to.equal('inventory_item');
        });
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

      describe('when checking specific item changes', () => {
        it('should update the specific array item', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items[2]).to.equal('Modified USB Dongle');
        });

        it('should not affect other array items', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items[0]).to.equal('AKG Wired Earbuds');
          expect(items[1]).to.equal('Steel Series Arctis 5 Headphone 3.5mm Adapter Cable');
        });

        it('should maintain array length', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items).to.have.length(7);
        });
      });

      describe('when checking overall structure', () => {
        it('should preserve nested structure', () => {
          expect((el.workingFrontmatter as any)?.inventory?.container).to.equal('lab_small_parts');
        });

        it('should preserve other sections', () => {
          expect((el.workingFrontmatter as any)?.rename_this_section?.total).to.equal('32');
        });
      });
    });

    describe('when handling mixed operations on complex structure', () => {
      describe('when editing section name and array simultaneously', () => {
        beforeEach(async () => {
          // Change section name
          const sectionInput = el.shadowRoot?.querySelector('.section-title-input') as HTMLInputElement;
          sectionInput.value = 'updated_section';
          sectionInput.dispatchEvent(new Event('input', { bubbles: true }));
          
          // Add array item
          const addButton = el.shadowRoot?.querySelector('.array-section .add-field-button') as Element;
          addButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
          
          await el.updateComplete;
        });

        it('should apply both changes correctly', () => {
          expect(el.workingFrontmatter).to.have.property('updated_section');
          expect(el.workingFrontmatter).to.not.have.property('rename_this_section');
          
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items).to.have.length(8);
        });

        it('should preserve section content during rename', () => {
          expect((el.workingFrontmatter as any)?.updated_section?.total).to.equal('32');
        });
      });

      describe('when performing multiple array operations', () => {
        beforeEach(async () => {
          // Add an item
          const addButton = el.shadowRoot?.querySelector('.array-section .add-field-button') as Element;
          addButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
          await el.updateComplete;
          
          // Remove first item
          const removeButton = el.shadowRoot?.querySelector('.array-item:first-child .remove-field-button') as Element;
          removeButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
          await el.updateComplete;
          
          // Modify second item
          const secondInput = el.shadowRoot?.querySelectorAll('.array-item input[type="text"]')?.[1] as HTMLInputElement;
          secondInput.value = 'Modified Second Item';
          secondInput.dispatchEvent(new Event('input', { bubbles: true }));
          await el.updateComplete;
        });

        it('should maintain correct array length after add and remove', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items).to.have.length(7); // 7 original - 1 removed + 1 added = 7
        });

        it('should reflect all modifications correctly', () => {
          const items = (el.workingFrontmatter as any)?.inventory?.items;
          expect(items[1]).to.equal('Modified Second Item');
          expect(items[0]).to.equal('Steel Series Arctis 5 Headphone 3.5mm Adapter Cable'); // Original first item removed
        });
      });
    });
  });
});