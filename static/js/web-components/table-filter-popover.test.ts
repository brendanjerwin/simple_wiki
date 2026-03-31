import { expect, fixture } from '@open-wc/testing';
import { html } from 'lit';
import sinon from 'sinon';
import type { SinonSpy } from 'sinon';
import './table-filter-popover.js';
import type { TableFilterPopover } from './table-filter-popover.js';

interface TestableTableFilterPopover {
  columnDefinition: unknown;
  _renderContent: () => unknown;
}
import type { TableColumnDefinition } from './table-data-extractor.js';
import type { ColumnFilterState, SortDirection } from './table-sorter-filterer.js';

function makeTextColumn(headerText: string): TableColumnDefinition {
  return {
    headerText,
    typeInfo: { detectedType: 'text', confidenceRatio: 1 },
    columnIndex: 0,
  };
}

function makeIntegerColumn(headerText: string): TableColumnDefinition {
  return {
    headerText,
    typeInfo: { detectedType: 'integer', confidenceRatio: 1 },
    columnIndex: 0,
  };
}

function makeCurrencyColumn(headerText: string): TableColumnDefinition {
  return {
    headerText,
    typeInfo: { detectedType: 'currency', confidenceRatio: 1 },
    columnIndex: 0,
  };
}

describe('TableFilterPopover', () => {

  describe('when created', () => {
    let el: TableFilterPopover;

    beforeEach(async () => {
      el = await fixture(html`<table-filter-popover></table-filter-popover>`);
    });

    it('should exist', () => {
      expect(el).to.exist;
    });

    it('should be registered as a custom element', () => {
      expect(customElements.get('table-filter-popover')).to.exist;
    });
  });

  describe('when open is false', () => {
    let el: TableFilterPopover;

    beforeEach(async () => {
      el = await fixture(html`
        <table-filter-popover
          .columnDefinition=${makeTextColumn('Name')}
          .uniqueValues=${['A', 'B', 'C']}
        ></table-filter-popover>
      `);
    });

    it('should not display popover content', () => {
      const popover = el.shadowRoot!.querySelector('.popover') as HTMLElement;
      const display = getComputedStyle(popover).display;
      expect(display).to.equal('none');
    });
  });

  describe('when open is true', () => {
    let el: TableFilterPopover;

    beforeEach(async () => {
      el = await fixture(html`
        <table-filter-popover
          .columnDefinition=${makeTextColumn('Name')}
          .uniqueValues=${['Apple', 'Banana', 'Cherry']}
          .open=${true}
        ></table-filter-popover>
      `);
    });

    it('should display the popover', () => {
      const popover = el.shadowRoot!.querySelector('.popover') as HTMLElement;
      const display = getComputedStyle(popover).display;
      expect(display).to.not.equal('none');
    });

    it('should show the column name in the title', () => {
      const title = el.shadowRoot!.querySelector('.popover-title');
      expect(title?.textContent).to.contain('Name');
    });

    it('should show the detected type', () => {
      const type = el.shadowRoot!.querySelector('.popover-type');
      expect(type?.textContent).to.contain('text');
    });

    it('should show a close button', () => {
      const closeBtn = el.shadowRoot!.querySelector('.close-btn');
      expect(closeBtn).to.exist;
    });

    it('should show sort controls', () => {
      const sortControls = el.shadowRoot!.querySelector('.sort-controls');
      expect(sortControls).to.exist;
    });

    it('should show OK and Cancel buttons', () => {
      const okBtn = el.shadowRoot!.querySelector('[aria-label="Apply"]');
      const cancelBtn = el.shadowRoot!.querySelector('[aria-label="Cancel"]');
      expect(okBtn).to.exist;
      expect(cancelBtn).to.exist;
    });
  });

  describe('sort controls', () => {

    describe('when current sort is none', () => {
      let el: TableFilterPopover;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .currentSortDirection=${'none' as SortDirection}
            .open=${true}
          ></table-filter-popover>
        `);
      });

      it('should show ascending and descending pills', () => {
        const pills = el.shadowRoot!.querySelectorAll('.sort-pill');
        expect(pills).to.have.length(2);
      });

      it('should not have active class on any pill', () => {
        const active = el.shadowRoot!.querySelectorAll('.sort-pill-active');
        expect(active).to.have.length(0);
      });
    });

    describe('when clicking ascending sort', () => {
      let el: TableFilterPopover;
      let sortSpy: SinonSpy;

      beforeEach(async () => {
        sortSpy = sinon.spy();
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .currentSortDirection=${'none' as SortDirection}
            .open=${true}
            @sort-direction-changed=${sortSpy}
          ></table-filter-popover>
        `);
        const ascBtn = el.shadowRoot!.querySelector('[aria-label="Sort ascending"]') as HTMLButtonElement;
        ascBtn.click();
        await el.updateComplete;
      });

      it('should show ascending pill as active', () => {
        const active = el.shadowRoot!.querySelectorAll('.sort-pill-active');
        expect(active).to.have.length(1);
        expect(active[0]?.textContent).to.contain('Ascending');
      });

      it('should not emit sort-direction-changed yet', () => {
        expect(sortSpy).to.not.have.been.called;
      });
    });

    describe('when clicking ascending sort while already ascending', () => {
      let el: TableFilterPopover;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .currentSortDirection=${'ascending' as SortDirection}
            .open=${true}
          ></table-filter-popover>
        `);
        const ascBtn = el.shadowRoot!.querySelector('[aria-label="Sort ascending"]') as HTMLButtonElement;
        ascBtn.click();
        await el.updateComplete;
      });

      it('should toggle ascending pill off', () => {
        const active = el.shadowRoot!.querySelectorAll('.sort-pill-active');
        expect(active).to.have.length(0);
      });
    });

    describe('when sort is active', () => {
      let el: TableFilterPopover;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .currentSortDirection=${'ascending' as SortDirection}
            .open=${true}
          ></table-filter-popover>
        `);
      });

      it('should show ascending pill as active', () => {
        const active = el.shadowRoot!.querySelectorAll('.sort-pill-active');
        expect(active).to.have.length(1);
        expect(active[0]?.textContent).to.contain('Ascending');
      });

      it('should show a clear sort pill', () => {
        const clearBtn = el.shadowRoot!.querySelector('[aria-label="Clear sort"]');
        expect(clearBtn).to.exist;
      });
    });
  });

  describe('checkbox filter', () => {

    describe('when text column has 3 unique values', () => {
      let el: TableFilterPopover;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Fruit')}
            .uniqueValues=${['Apple', 'Banana', 'Cherry']}
            .open=${true}
          ></table-filter-popover>
        `);
      });

      it('should render checkbox filter', () => {
        const checkboxList = el.shadowRoot!.querySelector('.checkbox-list');
        expect(checkboxList).to.exist;
      });

      it('should render one checkbox per unique value', () => {
        const checkboxes = el.shadowRoot!.querySelectorAll('.checkbox-item');
        expect(checkboxes).to.have.length(3);
      });

      it('should have all checkboxes checked by default', () => {
        const inputs = el.shadowRoot!.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        const allChecked = Array.from(inputs).every(input => input.checked);
        expect(allChecked).to.be.true;
      });

      it('should show Select All and Select None links', () => {
        const links = el.shadowRoot!.querySelectorAll('.checkbox-link');
        expect(links).to.have.length(2);
        expect(links[0]?.textContent).to.contain('Select All');
        expect(links[1]?.textContent).to.contain('Select None');
      });
    });

    describe('when unchecking a value', () => {
      let el: TableFilterPopover;
      let filterSpy: SinonSpy;

      beforeEach(async () => {
        filterSpy = sinon.spy();
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Fruit')}
            .uniqueValues=${['Apple', 'Banana', 'Cherry']}
            .open=${true}
            @filter-changed=${filterSpy}
          ></table-filter-popover>
        `);
        const inputs = el.shadowRoot!.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        inputs[0]!.checked = false;
        inputs[0]!.dispatchEvent(new Event('change'));
        await el.updateComplete;
      });

      it('should uncheck the checkbox in the UI', () => {
        const inputs = el.shadowRoot!.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        expect(inputs[0]!.checked).to.be.false;
      });

      it('should not emit filter-changed before OK is clicked', () => {
        expect(filterSpy).to.not.have.been.called;
      });
    });

    describe('when clicking Select None', () => {
      let el: TableFilterPopover;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Fruit')}
            .uniqueValues=${['Apple', 'Banana', 'Cherry']}
            .open=${true}
          ></table-filter-popover>
        `);
        const selectNone = el.shadowRoot!.querySelectorAll('.checkbox-link')[1] as HTMLButtonElement;
        selectNone.click();
        await el.updateComplete;
      });

      it('should uncheck all checkboxes', () => {
        const inputs = el.shadowRoot!.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        const allUnchecked = Array.from(inputs).every(input => !input.checked);
        expect(allUnchecked).to.be.true;
      });
    });

    describe('when clicking Select All after some excluded', () => {
      let el: TableFilterPopover;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Fruit')}
            .uniqueValues=${['Apple', 'Banana', 'Cherry']}
            .currentFilter=${{ kind: 'checkbox', excludedValues: new Set(['Apple']) } as ColumnFilterState}
            .open=${true}
          ></table-filter-popover>
        `);
        const selectAll = el.shadowRoot!.querySelectorAll('.checkbox-link')[0] as HTMLButtonElement;
        selectAll.click();
        await el.updateComplete;
      });

      it('should check all checkboxes', () => {
        const inputs = el.shadowRoot!.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        const allChecked = Array.from(inputs).every(input => input.checked);
        expect(allChecked).to.be.true;
      });
    });
  });

  describe('text-search filter', () => {

    describe('when text column has more than 15 unique values', () => {
      let el: TableFilterPopover;
      const manyValues = Array.from({ length: 20 }, (_, i) => `Value${i}`);

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Items')}
            .uniqueValues=${manyValues}
            .open=${true}
          ></table-filter-popover>
        `);
      });

      it('should render text search input', () => {
        const input = el.shadowRoot!.querySelector('.search-input');
        expect(input).to.exist;
      });

      it('should not render checkbox list', () => {
        const checkboxList = el.shadowRoot!.querySelector('.checkbox-list');
        expect(checkboxList).to.not.exist;
      });
    });
  });

  describe('numeric columns with few unique values', () => {

    describe('when integer column has 4 unique values', () => {
      let el: TableFilterPopover;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeIntegerColumn('Count')}
            .uniqueValues=${['5', '10', '15', '20']}
            .numericRange=${{ min: 5, max: 20 }}
            .open=${true}
          ></table-filter-popover>
        `);
      });

      it('should render checkbox filter', () => {
        const checkboxList = el.shadowRoot!.querySelector('.checkbox-list');
        expect(checkboxList).to.exist;
      });

      it('should not render range container', () => {
        const container = el.shadowRoot!.querySelector('.range-container');
        expect(container).to.not.exist;
      });
    });

    describe('when currency column has 3 unique values', () => {
      let el: TableFilterPopover;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeCurrencyColumn('Price')}
            .uniqueValues=${['$5.00', '$10.00', '$20.00']}
            .numericRange=${{ min: 5, max: 20 }}
            .open=${true}
          ></table-filter-popover>
        `);
      });

      it('should render checkbox filter', () => {
        const checkboxList = el.shadowRoot!.querySelector('.checkbox-list');
        expect(checkboxList).to.exist;
      });
    });
  });

  describe('range filter', () => {

    describe('when integer column has many unique values', () => {
      let el: TableFilterPopover;
      const manyValues = Array.from({ length: 20 }, (_, i) => String(i * 5));

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeIntegerColumn('Count')}
            .uniqueValues=${manyValues}
            .numericRange=${{ min: 0, max: 95 }}
            .open=${true}
          ></table-filter-popover>
        `);
      });

      it('should render range container', () => {
        const container = el.shadowRoot!.querySelector('.range-container');
        expect(container).to.exist;
      });

      it('should render two range sliders', () => {
        const sliders = el.shadowRoot!.querySelectorAll('.range-slider');
        expect(sliders).to.have.length(2);
      });

      it('should render two number inputs', () => {
        const inputs = el.shadowRoot!.querySelectorAll('.range-input');
        expect(inputs).to.have.length(2);
      });

      it('should not render checkbox list', () => {
        const checkboxList = el.shadowRoot!.querySelector('.checkbox-list');
        expect(checkboxList).to.not.exist;
      });

      it('should use step 1 for integer column sliders', () => {
        const sliders = el.shadowRoot!.querySelectorAll('.range-slider') as NodeListOf<HTMLInputElement>;
        expect(sliders[0]!.step).to.equal('1');
        expect(sliders[1]!.step).to.equal('1');
      });
    });

    describe('when decimal column has many unique values', () => {
      let el: TableFilterPopover;

      function makeDecimalColumn(headerText: string): TableColumnDefinition {
        return {
          headerText,
          typeInfo: { detectedType: 'decimal', confidenceRatio: 1 },
          columnIndex: 0,
        };
      }

      const manyValues = Array.from({ length: 20 }, (_, i) => String(i * 0.5));

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeDecimalColumn('Rating')}
            .uniqueValues=${manyValues}
            .numericRange=${{ min: 0, max: 9.5 }}
            .open=${true}
          ></table-filter-popover>
        `);
      });

      it('should render range container', () => {
        const container = el.shadowRoot!.querySelector('.range-container');
        expect(container).to.exist;
      });

      it('should use step any for decimal column sliders', () => {
        const sliders = el.shadowRoot!.querySelectorAll('.range-slider') as NodeListOf<HTMLInputElement>;
        expect(sliders[0]!.step).to.equal('any');
        expect(sliders[1]!.step).to.equal('any');
      });
    });

    describe('when currency column has many unique values', () => {
      let el: TableFilterPopover;
      const manyValues = Array.from({ length: 20 }, (_, i) => `$${i * 10}.00`);

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeCurrencyColumn('Price')}
            .uniqueValues=${manyValues}
            .numericRange=${{ min: 0, max: 190 }}
            .open=${true}
          ></table-filter-popover>
        `);
      });

      it('should render range filter for currency', () => {
        const container = el.shadowRoot!.querySelector('.range-container');
        expect(container).to.exist;
      });

      it('should use step 0.01 for currency column sliders', () => {
        const sliders = el.shadowRoot!.querySelectorAll('.range-slider') as NodeListOf<HTMLInputElement>;
        expect(sliders[0]!.step).to.equal('0.01');
        expect(sliders[1]!.step).to.equal('0.01');
      });
    });
  });

  describe('OK button', () => {

    describe('when clicking OK with checkbox filter changes', () => {
      let el: TableFilterPopover;
      let filterDetail: { filter: ColumnFilterState | null } | null;
      let closedSpy: SinonSpy;

      beforeEach(async () => {
        filterDetail = null;
        closedSpy = sinon.spy();
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Fruit')}
            .uniqueValues=${['Apple', 'Banana', 'Cherry']}
            .open=${true}
            @filter-changed=${(e: CustomEvent) => { filterDetail = e.detail as { filter: ColumnFilterState | null }; }}
            @popover-closed=${closedSpy}
          ></table-filter-popover>
        `);
        const inputs = el.shadowRoot!.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        inputs[0]!.checked = false;
        inputs[0]!.dispatchEvent(new Event('change'));
        await el.updateComplete;

        const okBtn = el.shadowRoot!.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
      });

      it('should emit filter-changed with excluded value', () => {
        expect(filterDetail).to.not.be.null;
        expect(filterDetail!.filter).to.not.be.null;
        expect(filterDetail!.filter!.kind).to.equal('checkbox');
        const cbFilter = filterDetail!.filter as { kind: 'checkbox'; excludedValues: Set<string> };
        expect(cbFilter.excludedValues.has('Apple')).to.be.true;
      });

      it('should emit popover-closed', () => {
        expect(closedSpy).to.have.been.calledOnce;
      });
    });

    describe('when clicking OK with sort direction change', () => {
      let el: TableFilterPopover;
      let sortDetail: { direction: SortDirection } | null;

      beforeEach(async () => {
        sortDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .currentSortDirection=${'none' as SortDirection}
            .open=${true}
            @sort-direction-changed=${(e: CustomEvent) => { sortDetail = e.detail as { direction: SortDirection }; }}
          ></table-filter-popover>
        `);
        const ascBtn = el.shadowRoot!.querySelector('[aria-label="Sort ascending"]') as HTMLButtonElement;
        ascBtn.click();
        await el.updateComplete;

        const okBtn = el.shadowRoot!.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
      });

      it('should emit sort-direction-changed with ascending', () => {
        expect(sortDetail).to.deep.equal({ direction: 'ascending' });
      });
    });

    describe('when clicking OK with range filter changes', () => {
      let el: TableFilterPopover;
      let filterDetail: { filter: ColumnFilterState | null } | null;
      const manyValues = Array.from({ length: 20 }, (_, i) => String(i * 5));

      beforeEach(async () => {
        filterDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeIntegerColumn('Count')}
            .uniqueValues=${manyValues}
            .numericRange=${{ min: 0, max: 95 }}
            .open=${true}
            @filter-changed=${(e: CustomEvent) => { filterDetail = e.detail as { filter: ColumnFilterState | null }; }}
          ></table-filter-popover>
        `);
        const minInput = el.shadowRoot!.querySelector('[aria-label="Minimum value"]') as HTMLInputElement;
        minInput.value = '10';
        minInput.dispatchEvent(new Event('input'));
        await el.updateComplete;

        const okBtn = el.shadowRoot!.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
      });

      it('should emit filter-changed with range filter', () => {
        expect(filterDetail).to.not.be.null;
        expect(filterDetail!.filter).to.not.be.null;
        expect(filterDetail!.filter!.kind).to.equal('range');
        const rFilter = filterDetail!.filter as { kind: 'range'; min: number | null; max: number | null };
        expect(rFilter.min).to.equal(10);
      });
    });

    describe('when typing intermediate non-numeric value into range min input', () => {
      let el: TableFilterPopover;
      let filterSpy: SinonSpy;
      const manyValues = Array.from({ length: 20 }, (_, i) => String(i * 5));

      beforeEach(async () => {
        filterSpy = sinon.spy();
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeIntegerColumn('Count')}
            .uniqueValues=${manyValues}
            .numericRange=${{ min: 0, max: 95 }}
            .open=${true}
            @filter-changed=${filterSpy}
          ></table-filter-popover>
        `);
        const minInput = el.shadowRoot!.querySelector('[aria-label="Minimum value"]') as HTMLInputElement;
        minInput.value = '-';
        minInput.dispatchEvent(new Event('input'));
        await el.updateComplete;

        const okBtn = el.shadowRoot!.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
      });

      it('should not emit filter-changed for NaN-producing input', () => {
        expect(filterSpy).to.not.have.been.called;
      });
    });

    describe('when typing intermediate non-numeric value into range max input', () => {
      let el: TableFilterPopover;
      let filterSpy: SinonSpy;
      const manyValues = Array.from({ length: 20 }, (_, i) => String(i * 5));

      beforeEach(async () => {
        filterSpy = sinon.spy();
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeIntegerColumn('Count')}
            .uniqueValues=${manyValues}
            .numericRange=${{ min: 0, max: 95 }}
            .open=${true}
            @filter-changed=${filterSpy}
          ></table-filter-popover>
        `);
        const maxInput = el.shadowRoot!.querySelector('[aria-label="Maximum value"]') as HTMLInputElement;
        maxInput.value = 'e';
        maxInput.dispatchEvent(new Event('input'));
        await el.updateComplete;

        const okBtn = el.shadowRoot!.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
      });

      it('should not emit filter-changed for NaN-producing input', () => {
        expect(filterSpy).to.not.have.been.called;
      });
    });

    describe('when clicking OK with text search filter', () => {
      let el: TableFilterPopover;
      let filterDetail: { filter: ColumnFilterState | null } | null;
      const manyValues = Array.from({ length: 20 }, (_, i) => `Value${i}`);

      beforeEach(async () => {
        filterDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Items')}
            .uniqueValues=${manyValues}
            .open=${true}
            @filter-changed=${(e: CustomEvent) => { filterDetail = e.detail as { filter: ColumnFilterState | null }; }}
          ></table-filter-popover>
        `);
        const input = el.shadowRoot!.querySelector('.search-input') as HTMLInputElement;
        input.value = 'hello';
        input.dispatchEvent(new Event('input'));
        await el.updateComplete;

        const okBtn = el.shadowRoot!.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
      });

      it('should emit filter-changed with text-search filter', () => {
        expect(filterDetail).to.not.be.null;
        expect(filterDetail!.filter).to.not.be.null;
        expect(filterDetail!.filter!.kind).to.equal('text-search');
        const tsFilter = filterDetail!.filter as { kind: 'text-search'; searchText: string };
        expect(tsFilter.searchText).to.equal('hello');
      });
    });

    describe('when clicking OK with no changes', () => {
      let el: TableFilterPopover;
      let filterSpy: SinonSpy;
      let sortSpy: SinonSpy;
      let closedSpy: SinonSpy;

      beforeEach(async () => {
        filterSpy = sinon.spy();
        sortSpy = sinon.spy();
        closedSpy = sinon.spy();
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .open=${true}
            @filter-changed=${filterSpy}
            @sort-direction-changed=${sortSpy}
            @popover-closed=${closedSpy}
          ></table-filter-popover>
        `);

        const okBtn = el.shadowRoot!.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
      });

      it('should emit popover-closed', () => {
        expect(closedSpy).to.have.been.calledOnce;
      });

      it('should not emit filter-changed', () => {
        expect(filterSpy).to.not.have.been.called;
      });

      it('should not emit sort-direction-changed', () => {
        expect(sortSpy).to.not.have.been.called;
      });
    });

    describe('when clicking OK with Select None', () => {
      let el: TableFilterPopover;
      let filterDetail: { filter: ColumnFilterState | null } | null;

      beforeEach(async () => {
        filterDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Fruit')}
            .uniqueValues=${['Apple', 'Banana', 'Cherry']}
            .open=${true}
            @filter-changed=${(e: CustomEvent) => { filterDetail = e.detail as { filter: ColumnFilterState | null }; }}
          ></table-filter-popover>
        `);
        const selectNone = el.shadowRoot!.querySelectorAll('.checkbox-link')[1] as HTMLButtonElement;
        selectNone.click();
        await el.updateComplete;

        const okBtn = el.shadowRoot!.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
      });

      it('should emit filter with all values excluded', () => {
        expect(filterDetail).to.not.be.null;
        const cbFilter = filterDetail!.filter as { kind: 'checkbox'; excludedValues: Set<string> };
        expect(cbFilter.excludedValues.size).to.equal(3);
      });
    });

    describe('when clicking OK after Select All clears filter', () => {
      let el: TableFilterPopover;
      let filterDetail: { filter: ColumnFilterState | null } | null;

      beforeEach(async () => {
        filterDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Fruit')}
            .uniqueValues=${['Apple', 'Banana', 'Cherry']}
            .currentFilter=${{ kind: 'checkbox', excludedValues: new Set(['Apple']) } as ColumnFilterState}
            .open=${true}
            @filter-changed=${(e: CustomEvent) => { filterDetail = e.detail as { filter: ColumnFilterState | null }; }}
          ></table-filter-popover>
        `);
        const selectAll = el.shadowRoot!.querySelectorAll('.checkbox-link')[0] as HTMLButtonElement;
        selectAll.click();
        await el.updateComplete;

        const okBtn = el.shadowRoot!.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
      });

      it('should emit null filter (all included)', () => {
        expect(filterDetail).to.not.be.null;
        expect(filterDetail!.filter).to.be.null;
      });
    });
  });

  describe('Cancel button', () => {

    describe('when clicking Cancel after making changes', () => {
      let el: TableFilterPopover;
      let filterSpy: SinonSpy;
      let sortSpy: SinonSpy;
      let closedSpy: SinonSpy;

      beforeEach(async () => {
        filterSpy = sinon.spy();
        sortSpy = sinon.spy();
        closedSpy = sinon.spy();
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Fruit')}
            .uniqueValues=${['Apple', 'Banana', 'Cherry']}
            .open=${true}
            @filter-changed=${filterSpy}
            @sort-direction-changed=${sortSpy}
            @popover-closed=${closedSpy}
          ></table-filter-popover>
        `);
        const inputs = el.shadowRoot!.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        inputs[0]!.checked = false;
        inputs[0]!.dispatchEvent(new Event('change'));
        await el.updateComplete;

        const cancelBtn = el.shadowRoot!.querySelector('[aria-label="Cancel"]') as HTMLButtonElement;
        cancelBtn.click();
      });

      it('should emit popover-closed', () => {
        expect(closedSpy).to.have.been.calledOnce;
      });

      it('should not emit filter-changed', () => {
        expect(filterSpy).to.not.have.been.called;
      });

      it('should not emit sort-direction-changed', () => {
        expect(sortSpy).to.not.have.been.called;
      });
    });
  });

  describe('click-outside handling', () => {

    describe('when clicking outside the popover', () => {
      let el: TableFilterPopover;
      let closedSpy: SinonSpy;
      let filterSpy: SinonSpy;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .open=${true}
          ></table-filter-popover>
        `);
        closedSpy = sinon.spy();
        filterSpy = sinon.spy();
        el.addEventListener('popover-closed', closedSpy);
        el.addEventListener('filter-changed', filterSpy);

        el._handleClickOutside(new Event('click'));
      });

      it('should emit popover-closed event', () => {
        expect(closedSpy).to.have.been.calledOnce;
      });

      it('should not emit filter-changed', () => {
        expect(filterSpy).to.not.have.been.called;
      });
    });

    describe('when clicking inside the popover', () => {
      let el: TableFilterPopover;
      let closedSpy: SinonSpy;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .open=${true}
          ></table-filter-popover>
        `);
        closedSpy = sinon.spy();
        el.addEventListener('popover-closed', closedSpy);

        const popover = el.shadowRoot!.querySelector('.popover')!;
        const mockEvent = {
          composedPath: () => [popover],
        } as unknown as Event;
        el._handleClickOutside(mockEvent);
      });

      it('should not emit popover-closed event', () => {
        expect(closedSpy).to.not.have.been.called;
      });
    });
  });

  describe('ESC key handling', () => {

    describe('when pressing ESC while open', () => {
      let el: TableFilterPopover;
      let closedSpy: SinonSpy;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .open=${true}
          ></table-filter-popover>
        `);
        closedSpy = sinon.spy();
        el.addEventListener('popover-closed', closedSpy);

        const event = new KeyboardEvent('keydown', { key: 'Escape' });
        el._handleKeydown(event);
      });

      it('should emit popover-closed event', () => {
        expect(closedSpy).to.have.been.calledOnce;
      });
    });

    describe('when pressing ESC while closed', () => {
      let el: TableFilterPopover;
      let closedSpy: SinonSpy;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .open=${false}
          ></table-filter-popover>
        `);
        closedSpy = sinon.spy();
        el.addEventListener('popover-closed', closedSpy);

        const event = new KeyboardEvent('keydown', { key: 'Escape' });
        el._handleKeydown(event);
      });

      it('should not emit popover-closed event', () => {
        expect(closedSpy).to.not.have.been.called;
      });
    });
  });

  describe('pre-populated state', () => {

    describe('when opened with existing checkbox filter', () => {
      let el: TableFilterPopover;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Fruit')}
            .uniqueValues=${['Apple', 'Banana', 'Cherry']}
            .currentFilter=${{ kind: 'checkbox', excludedValues: new Set(['Banana']) } as ColumnFilterState}
            .open=${true}
          ></table-filter-popover>
        `);
      });

      it('should show checkbox filter', () => {
        const checkboxList = el.shadowRoot!.querySelector('.checkbox-list');
        expect(checkboxList).to.exist;
      });

      it('should have Banana unchecked', () => {
        const inputs = el.shadowRoot!.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        const bananaInput = inputs[1]!;
        expect(bananaInput.checked).to.be.false;
      });

      it('should have Apple and Cherry checked', () => {
        const inputs = el.shadowRoot!.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        expect(inputs[0]!.checked).to.be.true;
        expect(inputs[2]!.checked).to.be.true;
      });
    });

    describe('when opened with existing range filter', () => {
      let el: TableFilterPopover;
      const manyValues = Array.from({ length: 20 }, (_, i) => String(i * 5));

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeIntegerColumn('Count')}
            .uniqueValues=${manyValues}
            .numericRange=${{ min: 0, max: 95 }}
            .currentFilter=${{ kind: 'range', min: 10, max: 15 } as ColumnFilterState}
            .open=${true}
          ></table-filter-popover>
        `);
      });

      it('should show range filter', () => {
        const container = el.shadowRoot!.querySelector('.range-container');
        expect(container).to.exist;
      });

      it('should pre-populate min input', () => {
        const minInput = el.shadowRoot!.querySelector('[aria-label="Minimum value"]') as HTMLInputElement;
        expect(minInput.value).to.equal('10');
      });

      it('should pre-populate max input', () => {
        const maxInput = el.shadowRoot!.querySelector('[aria-label="Maximum value"]') as HTMLInputElement;
        expect(maxInput.value).to.equal('15');
      });
    });

    describe('when opened with existing text-search filter', () => {
      let el: TableFilterPopover;
      const manyValues = Array.from({ length: 20 }, (_, i) => `Value${i}`);

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Items')}
            .uniqueValues=${manyValues}
            .currentFilter=${{ kind: 'text-search', searchText: 'hello' } as ColumnFilterState}
            .open=${true}
          ></table-filter-popover>
        `);
      });

      it('should show text search filter', () => {
        const input = el.shadowRoot!.querySelector('.search-input');
        expect(input).to.exist;
      });

      it('should pre-populate search text', () => {
        const input = el.shadowRoot!.querySelector('.search-input') as HTMLInputElement;
        expect(input.value).to.equal('hello');
      });
    });
  });

  describe('close button', () => {

    describe('when clicking close button', () => {
      let el: TableFilterPopover;
      let closedSpy: SinonSpy;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .open=${true}
          ></table-filter-popover>
        `);
        closedSpy = sinon.spy();
        el.addEventListener('popover-closed', closedSpy);

        const closeBtn = el.shadowRoot!.querySelector('.close-btn') as HTMLButtonElement;
        closeBtn.click();
      });

      it('should emit popover-closed event', () => {
        expect(closedSpy).to.have.been.calledOnce;
      });
    });
  });

  describe('event listener wiring', () => {

    describe('when component opens', () => {
      let addEventListenerSpy: SinonSpy;

      beforeEach(async () => {
        addEventListenerSpy = sinon.spy(document, 'addEventListener');
        await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .open=${true}
          ></table-filter-popover>
        `);
        await new Promise(resolve => setTimeout(resolve, 0));
      });

      afterEach(() => {
        addEventListenerSpy.restore();
      });

      it('should add click event listener to document after animation frame', () => {
        expect(addEventListenerSpy).to.have.been.calledWith('click');
      });

      it('should add keydown event listener to document after animation frame', () => {
        expect(addEventListenerSpy).to.have.been.calledWith('keydown');
      });
    });

    describe('when component is connected but not open', () => {
      let addEventListenerSpy: SinonSpy;

      beforeEach(async () => {
        addEventListenerSpy = sinon.spy(document, 'addEventListener');
        await fixture(html`<table-filter-popover></table-filter-popover>`);
        await new Promise(resolve => setTimeout(resolve, 0));
      });

      afterEach(() => {
        addEventListenerSpy.restore();
      });

      it('should not add click event listener to document', () => {
        expect(addEventListenerSpy).to.not.have.been.calledWith('click');
      });
    });

    describe('when component is disconnected from DOM', () => {
      let removeEventListenerSpy: SinonSpy;
      let el: TableFilterPopover;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .open=${true}
          ></table-filter-popover>
        `);
        await new Promise(resolve => setTimeout(resolve, 0));
        removeEventListenerSpy = sinon.spy(document, 'removeEventListener');
        el.remove();
      });

      afterEach(() => {
        removeEventListenerSpy.restore();
      });

      it('should remove click event listener from document', () => {
        expect(removeEventListenerSpy).to.have.been.calledWith('click');
      });

      it('should remove keydown event listener from document', () => {
        expect(removeEventListenerSpy).to.have.been.calledWith('keydown');
      });
    });
  });

  describe('_renderContent', () => {

    describe('when called without columnDefinition (programming error guard)', () => {
      let thrownError: Error | null;

      beforeEach(async () => {
        thrownError = null;
        const el = await fixture(html`<table-filter-popover></table-filter-popover>`);
        const testable = el as unknown as TestableTableFilterPopover;
        testable.columnDefinition = null;
        try {
          testable._renderContent();
        } catch (e) {
          thrownError = e instanceof Error ? e : null;
        }
      });

      it('should throw an error', () => {
        expect(thrownError).to.exist;
      });

      it('should include columnDefinition in the error message', () => {
        expect(thrownError?.message).to.include('columnDefinition');
      });
    });

  });

});
