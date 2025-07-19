import { fixture, html, expect } from '@open-wc/testing';
import { TemplateResult } from 'lit';
import { restore } from 'sinon';
import { FrontmatterValueArray } from './frontmatter-value-array.js';

function createFixtureWithTimeout(template: TemplateResult, timeoutMs = 5000): Promise<FrontmatterValueArray> {
  const timeout = (ms: number, message: string) =>
    new Promise<never>((_, reject) => 
      setTimeout(() => reject(new Error(message)), ms)
    );
  
  return Promise.race([
    fixture(template),
    timeout(timeoutMs, 'Component fixture timed out')
  ]) as Promise<FrontmatterValueArray>;
}

describe('FrontmatterValueArray', () => {
  let el: FrontmatterValueArray;

  afterEach(() => {
    restore();
  });

  describe('should exist', () => {
    beforeEach(async () => {
      try {
        el = await createFixtureWithTimeout(html`<frontmatter-value-array></frontmatter-value-array>`);
      } catch (e) {
        console.error(e);
        throw e;
      }
    });

    it('should exist', () => {
      expect(el).to.exist;
    });

    it('should be an instance of FrontmatterValueArray', () => {
      expect(el).to.be.instanceOf(FrontmatterValueArray);
    });

    it('should have the correct tag name', () => {
      expect(el.tagName.toLowerCase()).to.equal('frontmatter-value-array');
    });
  });

  describe('when component is initialized', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-array></frontmatter-value-array>`);
    });

    it('should have empty array by default', () => {
      expect(el.values).to.deep.equal([]);
    });

    it('should not be disabled by default', () => {
      expect(el.disabled).to.be.false;
    });

    it('should have empty placeholder by default', () => {
      expect(el.placeholder).to.equal('');
    });
  });

  describe('when rendered with array values', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-array .values="${['item1', 'item2', 'item3']}"></frontmatter-value-array>`);
    });

    it('should render correct number of array items', () => {
      const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
      expect(arrayItems?.length).to.equal(3);
    });

    it('should render frontmatter-value-string components for each item', () => {
      const stringComponents = el.shadowRoot?.querySelectorAll('frontmatter-value-string');
      expect(stringComponents?.length).to.equal(3);
    });

    it('should display the values in string components', () => {
      const stringComponents = el.shadowRoot?.querySelectorAll('frontmatter-value-string') as NodeListOf<Element>;
      expect(stringComponents[0].value).to.equal('item1');
      expect(stringComponents[1].value).to.equal('item2');
      expect(stringComponents[2].value).to.equal('item3');
    });

    it('should render remove buttons for each item', () => {
      const removeButtons = el.shadowRoot?.querySelectorAll('.remove-item-button');
      expect(removeButtons?.length).to.equal(3);
    });
  });

  describe('when array is empty', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-array .values="${[]}"></frontmatter-value-array>`);
    });

    it('should display empty array message', () => {
      const emptyMessage = el.shadowRoot?.querySelector('.empty-array-message');
      expect(emptyMessage).to.exist;
      expect(emptyMessage?.textContent?.trim()).to.equal('No items in array');
    });

    it('should not render any array items', () => {
      const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
      expect(arrayItems?.length).to.equal(0);
    });
  });

  describe('when add item button is clicked', () => {
    let arrayChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      arrayChangeEvent = null;
      el = await createFixtureWithTimeout(html`<frontmatter-value-array .values="${['existing']}"></frontmatter-value-array>`);
      
      el.addEventListener('array-change', (event) => {
        arrayChangeEvent = event as CustomEvent;
      });

      const addButton = el.shadowRoot?.querySelector('.add-item-button') as HTMLButtonElement;
      addButton.click();
    });

    it('should dispatch array-change event', () => {
      expect(arrayChangeEvent).to.exist;
    });

    it('should include the new array in event detail', () => {
      expect(arrayChangeEvent?.detail.newArray).to.deep.equal(['existing', '']);
    });

    it('should include the old array in event detail', () => {
      expect(arrayChangeEvent?.detail.oldArray).to.deep.equal(['existing']);
    });

    it('should update the values property', () => {
      expect(el.values).to.deep.equal(['existing', '']);
    });

    it('should render additional array item', () => {
      const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
      expect(arrayItems?.length).to.equal(2);
    });
  });

  describe('when remove item button is clicked', () => {
    let arrayChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      arrayChangeEvent = null;
      el = await createFixtureWithTimeout(html`<frontmatter-value-array .values="${['item1', 'item2', 'item3']}"></frontmatter-value-array>`);
      
      el.addEventListener('array-change', (event) => {
        arrayChangeEvent = event as CustomEvent;
      });

      // Click remove button for second item (index 1)
      const removeButtons = el.shadowRoot?.querySelectorAll('.remove-item-button') as NodeListOf<HTMLButtonElement>;
      removeButtons[1].click();
    });

    it('should dispatch array-change event', () => {
      expect(arrayChangeEvent).to.exist;
    });

    it('should include the new array with item removed', () => {
      expect(arrayChangeEvent?.detail.newArray).to.deep.equal(['item1', 'item3']);
    });

    it('should include the old array in event detail', () => {
      expect(arrayChangeEvent?.detail.oldArray).to.deep.equal(['item1', 'item2', 'item3']);
    });

    it('should update the values property', () => {
      expect(el.values).to.deep.equal(['item1', 'item3']);
    });

    it('should render fewer array items', () => {
      const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
      expect(arrayItems?.length).to.equal(2);
    });
  });

  describe('when array item value changes', () => {
    let arrayChangeEvent: CustomEvent | null;

    beforeEach(async () => {
      arrayChangeEvent = null;
      el = await createFixtureWithTimeout(html`<frontmatter-value-array .values="${['original1', 'original2']}"></frontmatter-value-array>`);
      
      el.addEventListener('array-change', (event) => {
        arrayChangeEvent = event as CustomEvent;
      });

      // Simulate value change in first string component
      const stringComponents = el.shadowRoot?.querySelectorAll('frontmatter-value-string') as NodeListOf<Element>;
      stringComponents[0].dispatchEvent(new CustomEvent('value-change', {
        detail: {
          oldValue: 'original1',
          newValue: 'modified1'
        },
        bubbles: true
      }));
    });

    it('should dispatch array-change event', () => {
      expect(arrayChangeEvent).to.exist;
    });

    it('should include the updated array in event detail', () => {
      expect(arrayChangeEvent?.detail.newArray).to.deep.equal(['modified1', 'original2']);
    });

    it('should include the old array in event detail', () => {
      expect(arrayChangeEvent?.detail.oldArray).to.deep.equal(['original1', 'original2']);
    });

    it('should update the values property', () => {
      expect(el.values).to.deep.equal(['modified1', 'original2']);
    });
  });

  describe('when disabled is true', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-array .values="${['test']}" disabled></frontmatter-value-array>`);
    });

    it('should disable all string components', () => {
      const stringComponents = el.shadowRoot?.querySelectorAll('frontmatter-value-string') as NodeListOf<Element>;
      stringComponents.forEach(component => {
        expect(component.disabled).to.be.true;
      });
    });

    it('should disable the add button', () => {
      const addButton = el.shadowRoot?.querySelector('.add-item-button') as HTMLButtonElement;
      expect(addButton.disabled).to.be.true;
    });

    it('should disable all remove buttons', () => {
      const removeButtons = el.shadowRoot?.querySelectorAll('.remove-item-button') as NodeListOf<HTMLButtonElement>;
      removeButtons.forEach(button => {
        expect(button.disabled).to.be.true;
      });
    });
  });

  describe('when placeholder is provided', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-array .values="${['', 'filled']}" placeholder="Enter item"></frontmatter-value-array>`);
    });

    it('should set placeholder on all string components', () => {
      const stringComponents = el.shadowRoot?.querySelectorAll('frontmatter-value-string') as NodeListOf<Element>;
      stringComponents.forEach(component => {
        expect(component.placeholder).to.equal('Enter item');
      });
    });
  });

  describe('when array has single item', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-array .values="${['single']}"></frontmatter-value-array>`);
    });

    it('should render single array item', () => {
      const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
      expect(arrayItems?.length).to.equal(1);
    });

    it('should render remove button for single item', () => {
      const removeButtons = el.shadowRoot?.querySelectorAll('.remove-item-button');
      expect(removeButtons?.length).to.equal(1);
    });

    describe('when last item is removed', () => {
      let arrayChangeEvent: CustomEvent | null;

      beforeEach(async () => {
        arrayChangeEvent = null;
        el.addEventListener('array-change', (event) => {
          arrayChangeEvent = event as CustomEvent;
        });

        const removeButton = el.shadowRoot?.querySelector('.remove-item-button') as HTMLButtonElement;
        removeButton.click();
        await el.updateComplete;
      });

      it('should result in empty array', () => {
        expect(el.values).to.deep.equal([]);
      });

      it('should dispatch array-change event with empty array', () => {
        expect(arrayChangeEvent?.detail.newArray).to.deep.equal([]);
      });

      it('should show empty array message', () => {
        const emptyMessage = el.shadowRoot?.querySelector('.empty-array-message');
        expect(emptyMessage).to.exist;
      });
    });
  });

  describe('when values property is updated programmatically', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-array .values="${['initial']}"></frontmatter-value-array>`);
    });

    describe('when values array changes', () => {
      beforeEach(async () => {
        el.values = ['updated1', 'updated2'];
        await el.updateComplete;
      });

      it('should render new number of items', () => {
        const arrayItems = el.shadowRoot?.querySelectorAll('.array-item');
        expect(arrayItems?.length).to.equal(2);
      });

      it('should update string component values', () => {
        const stringComponents = el.shadowRoot?.querySelectorAll('frontmatter-value-string') as NodeListOf<Element>;
        expect(stringComponents[0].value).to.equal('updated1');
        expect(stringComponents[1].value).to.equal('updated2');
      });
    });
  });

  describe('when styling is applied', () => {
    beforeEach(async () => {
      el = await createFixtureWithTimeout(html`<frontmatter-value-array .values="${['test']}"></frontmatter-value-array>`);
    });

    it('should have proper array item styling', () => {
      const arrayItem = el.shadowRoot?.querySelector('.array-item') as HTMLElement;
      const computedStyle = getComputedStyle(arrayItem);
      
      expect(computedStyle.display).to.equal('flex');
      expect(computedStyle.gap).to.contain('4px');
    });

    it('should have proper button styling', () => {
      const addButton = el.shadowRoot?.querySelector('.add-item-button') as HTMLElement;
      const computedStyle = getComputedStyle(addButton);
      
      expect(computedStyle.padding).to.contain('4px');
      expect(computedStyle.borderRadius).to.equal('4px');
    });
  });
});