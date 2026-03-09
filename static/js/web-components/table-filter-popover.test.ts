import { expect, fixture } from '@open-wc/testing';
import { html } from 'lit';
import sinon from 'sinon';
import type { SinonSpy } from 'sinon';
import './table-filter-popover.js';
import type { TableFilterPopover } from './table-filter-popover.js';
import type { TableColumnDefinition } from './table-data-extractor.js';
import type { ColumnFilterState, SortDirection } from './table-sorter-filterer.js';

function makeTextColumn(headerText: string): TableColumnDefinition {
  return {
    headerText,
    typeInfo: { detectedType: 'text', confidenceRatio: 1 },
    columnIndex: 0,
  };
}

function makeNumberColumn(headerText: string): TableColumnDefinition {
  return {
    headerText,
    typeInfo: { detectedType: 'number', confidenceRatio: 1 },
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
      let eventDetail: { direction: SortDirection } | null;

      beforeEach(async () => {
        eventDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .currentSortDirection=${'none' as SortDirection}
            .open=${true}
            @sort-direction-changed=${(e: CustomEvent) => { eventDetail = e.detail as { direction: SortDirection }; }}
          ></table-filter-popover>
        `);
        const ascBtn = el.shadowRoot!.querySelector('[aria-label="Sort ascending"]') as HTMLButtonElement;
        ascBtn.click();
      });

      it('should emit sort-direction-changed with ascending', () => {
        expect(eventDetail).to.deep.equal({ direction: 'ascending' });
      });
    });

    describe('when clicking ascending sort while already ascending', () => {
      let el: TableFilterPopover;
      let eventDetail: { direction: SortDirection } | null;

      beforeEach(async () => {
        eventDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Name')}
            .uniqueValues=${['A', 'B']}
            .currentSortDirection=${'ascending' as SortDirection}
            .open=${true}
            @sort-direction-changed=${(e: CustomEvent) => { eventDetail = e.detail as { direction: SortDirection }; }}
          ></table-filter-popover>
        `);
        const ascBtn = el.shadowRoot!.querySelector('[aria-label="Sort ascending"]') as HTMLButtonElement;
        ascBtn.click();
      });

      it('should emit sort-direction-changed with none to toggle off', () => {
        expect(eventDetail).to.deep.equal({ direction: 'none' });
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
      let eventDetail: { filter: ColumnFilterState | null } | null;

      beforeEach(async () => {
        eventDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Fruit')}
            .uniqueValues=${['Apple', 'Banana', 'Cherry']}
            .open=${true}
            @filter-changed=${(e: CustomEvent) => { eventDetail = e.detail as { filter: ColumnFilterState | null }; }}
          ></table-filter-popover>
        `);
        const inputs = el.shadowRoot!.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        inputs[0]!.checked = false;
        inputs[0]!.dispatchEvent(new Event('change'));
        await el.updateComplete;
      });

      it('should emit filter-changed with excluded value', () => {
        expect(eventDetail).to.not.be.null;
        expect(eventDetail!.filter).to.not.be.null;
        expect(eventDetail!.filter!.kind).to.equal('checkbox');
        const cbFilter = eventDetail!.filter as { kind: 'checkbox'; excludedValues: Set<string> };
        expect(cbFilter.excludedValues.has('Apple')).to.be.true;
      });
    });

    describe('when clicking Select None', () => {
      let el: TableFilterPopover;
      let eventDetail: { filter: ColumnFilterState | null } | null;

      beforeEach(async () => {
        eventDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Fruit')}
            .uniqueValues=${['Apple', 'Banana', 'Cherry']}
            .open=${true}
            @filter-changed=${(e: CustomEvent) => { eventDetail = e.detail as { filter: ColumnFilterState | null }; }}
          ></table-filter-popover>
        `);
        const selectNone = el.shadowRoot!.querySelectorAll('.checkbox-link')[1] as HTMLButtonElement;
        selectNone.click();
        await el.updateComplete;
      });

      it('should emit filter with all values excluded', () => {
        expect(eventDetail).to.not.be.null;
        const cbFilter = eventDetail!.filter as { kind: 'checkbox'; excludedValues: Set<string> };
        expect(cbFilter.excludedValues.size).to.equal(3);
      });
    });

    describe('when clicking Select All after some excluded', () => {
      let el: TableFilterPopover;
      let eventDetail: { filter: ColumnFilterState | null } | null;

      beforeEach(async () => {
        eventDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Fruit')}
            .uniqueValues=${['Apple', 'Banana', 'Cherry']}
            .currentFilter=${{ kind: 'checkbox', excludedValues: new Set(['Apple']) } as ColumnFilterState}
            .open=${true}
            @filter-changed=${(e: CustomEvent) => { eventDetail = e.detail as { filter: ColumnFilterState | null }; }}
          ></table-filter-popover>
        `);
        const selectAll = el.shadowRoot!.querySelectorAll('.checkbox-link')[0] as HTMLButtonElement;
        selectAll.click();
        await el.updateComplete;
      });

      it('should emit null filter (all included)', () => {
        expect(eventDetail).to.not.be.null;
        expect(eventDetail!.filter).to.be.null;
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

    describe('when typing in search input', () => {
      let el: TableFilterPopover;
      let eventDetail: { filter: ColumnFilterState | null } | null;
      const manyValues = Array.from({ length: 20 }, (_, i) => `Value${i}`);

      beforeEach(async () => {
        eventDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Items')}
            .uniqueValues=${manyValues}
            .open=${true}
            @filter-changed=${(e: CustomEvent) => { eventDetail = e.detail as { filter: ColumnFilterState | null }; }}
          ></table-filter-popover>
        `);
        const input = el.shadowRoot!.querySelector('.search-input') as HTMLInputElement;
        input.value = 'hello';
        input.dispatchEvent(new Event('input'));
        await el.updateComplete;
      });

      it('should emit filter-changed with text-search filter', () => {
        expect(eventDetail).to.not.be.null;
        expect(eventDetail!.filter).to.not.be.null;
        expect(eventDetail!.filter!.kind).to.equal('text-search');
        const tsFilter = eventDetail!.filter as { kind: 'text-search'; searchText: string };
        expect(tsFilter.searchText).to.equal('hello');
      });
    });

    describe('when clearing search input', () => {
      let el: TableFilterPopover;
      let eventDetail: { filter: ColumnFilterState | null } | null;
      const manyValues = Array.from({ length: 20 }, (_, i) => `Value${i}`);

      beforeEach(async () => {
        eventDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeTextColumn('Items')}
            .uniqueValues=${manyValues}
            .currentFilter=${{ kind: 'text-search', searchText: 'hello' } as ColumnFilterState}
            .open=${true}
            @filter-changed=${(e: CustomEvent) => { eventDetail = e.detail as { filter: ColumnFilterState | null }; }}
          ></table-filter-popover>
        `);
        const input = el.shadowRoot!.querySelector('.search-input') as HTMLInputElement;
        input.value = '';
        input.dispatchEvent(new Event('input'));
        await el.updateComplete;
      });

      it('should emit null filter', () => {
        expect(eventDetail).to.not.be.null;
        expect(eventDetail!.filter).to.be.null;
      });
    });
  });

  describe('range filter', () => {

    describe('when column is numeric with range', () => {
      let el: TableFilterPopover;

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeNumberColumn('Count')}
            .uniqueValues=${['5', '10', '15', '20']}
            .numericRange=${{ min: 5, max: 20 }}
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
    });

    describe('when changing min value', () => {
      let el: TableFilterPopover;
      let eventDetail: { filter: ColumnFilterState | null } | null;

      beforeEach(async () => {
        eventDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeNumberColumn('Count')}
            .uniqueValues=${['5', '10', '15', '20']}
            .numericRange=${{ min: 5, max: 20 }}
            .open=${true}
            @filter-changed=${(e: CustomEvent) => { eventDetail = e.detail as { filter: ColumnFilterState | null }; }}
          ></table-filter-popover>
        `);
        const minInput = el.shadowRoot!.querySelector('[aria-label="Minimum value"]') as HTMLInputElement;
        minInput.value = '10';
        minInput.dispatchEvent(new Event('input'));
        await el.updateComplete;
      });

      it('should emit filter-changed with range filter', () => {
        expect(eventDetail).to.not.be.null;
        expect(eventDetail!.filter).to.not.be.null;
        expect(eventDetail!.filter!.kind).to.equal('range');
        const rFilter = eventDetail!.filter as { kind: 'range'; min: number | null; max: number | null };
        expect(rFilter.min).to.equal(10);
      });
    });

    describe('when clearing range values', () => {
      let el: TableFilterPopover;
      let eventDetail: { filter: ColumnFilterState | null } | null;

      beforeEach(async () => {
        eventDetail = null;
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeNumberColumn('Count')}
            .uniqueValues=${['5', '10', '15', '20']}
            .numericRange=${{ min: 5, max: 20 }}
            .currentFilter=${{ kind: 'range', min: 10, max: null } as ColumnFilterState}
            .open=${true}
            @filter-changed=${(e: CustomEvent) => { eventDetail = e.detail as { filter: ColumnFilterState | null }; }}
          ></table-filter-popover>
        `);
        const minInput = el.shadowRoot!.querySelector('[aria-label="Minimum value"]') as HTMLInputElement;
        minInput.value = '';
        minInput.dispatchEvent(new Event('input'));
        await el.updateComplete;
      });

      it('should emit null filter', () => {
        expect(eventDetail).to.not.be.null;
        expect(eventDetail!.filter).to.be.null;
      });
    });

    describe('when currency column has range', () => {
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

      it('should render range filter for currency', () => {
        const container = el.shadowRoot!.querySelector('.range-container');
        expect(container).to.exist;
      });
    });
  });

  describe('click-outside handling', () => {

    describe('when clicking outside the popover', () => {
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

        el.handleClickOutside(new Event('click'));
      });

      it('should emit popover-closed event', () => {
        expect(closedSpy).to.have.been.calledOnce;
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
        el.handleClickOutside(mockEvent);
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
        el.handleKeydown(event);
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
        el.handleKeydown(event);
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

      beforeEach(async () => {
        el = await fixture(html`
          <table-filter-popover
            .columnDefinition=${makeNumberColumn('Count')}
            .uniqueValues=${['5', '10', '15', '20']}
            .numericRange=${{ min: 5, max: 20 }}
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

    describe('when component is connected to DOM', () => {
      let addEventListenerSpy: SinonSpy;

      beforeEach(async () => {
        addEventListenerSpy = sinon.spy(document, 'addEventListener');
        await fixture(html`<table-filter-popover></table-filter-popover>`);
      });

      afterEach(() => {
        addEventListenerSpy.restore();
      });

      it('should add click event listener to document', () => {
        expect(addEventListenerSpy).to.have.been.calledWith('click');
      });

      it('should add keydown event listener to document', () => {
        expect(addEventListenerSpy).to.have.been.calledWith('keydown');
      });
    });

    describe('when component is disconnected from DOM', () => {
      let removeEventListenerSpy: SinonSpy;
      let el: TableFilterPopover;

      beforeEach(async () => {
        el = await fixture(html`<table-filter-popover></table-filter-popover>`);
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
});
