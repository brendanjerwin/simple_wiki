import { html, fixture, expect } from '@open-wc/testing';
import { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';
import sinon from 'sinon';
import './frontmatter-editor-dialog.js';

interface TestFrontmatter {
  identifier: string;
  title: string;
  rename_this_section: {
    total: string;
  };
  inventory: {
    container: string;
    items: string[];
  };
}

describe('FrontmatterEditorDialog - Array and Section Handling', () => {
  let el: FrontmatterEditorDialog;

  function timeout(ms: number, message: string): Promise<never> {
    return new Promise((_, reject) => 
      setTimeout(() => reject(new Error(message)), ms)
    );
  }

  beforeEach(async () => {
    try {
      // Use Promise.race to add explicit timeout for fixture creation
      el = await Promise.race([
        fixture(html`<frontmatter-editor-dialog></frontmatter-editor-dialog>`),
        timeout(5000, 'Component fixture timed out')
      ]);
      
      // Stub the loadFrontmatter method to prevent network calls
      sinon.stub(el, 'loadFrontmatter').resolves();
      
      await el.updateComplete;
    } catch (e) {
      console.error('Fixture creation failed:', e);
      throw e;
    }
  });

  afterEach(() => {
    sinon.restore();
    if (el) {
      el.remove();
    }
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
      } as TestFrontmatter;
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
    });

    describe('when add item is clicked', () => {
      let addButton: Element;

      beforeEach(async () => {
        addButton = el.shadowRoot?.querySelector('.array-section .add-field-button') as Element;
        addButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should add a new empty item to the array', () => {
        const items = (el.workingFrontmatter as TestFrontmatter)?.inventory?.items;
        expect(items).to.have.length(4);
        expect(items[3]).to.equal('');
      });
    });
  });
});