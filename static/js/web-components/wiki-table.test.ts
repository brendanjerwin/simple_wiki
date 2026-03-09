import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import type { SinonStub } from 'sinon';
import './wiki-table.js';
import type { WikiTable } from './wiki-table.js';

async function createBasicFixture(): Promise<WikiTable> {
  const container = document.createElement('div');
  container.innerHTML = `
    <wiki-table>
      <table>
        <thead><tr><th>Name</th><th>Price</th><th>Count</th></tr></thead>
        <tbody>
          <tr><td>Widget</td><td>$9.99</td><td>10</td></tr>
          <tr><td>Gadget</td><td>$24.50</td><td>5</td></tr>
          <tr><td>Doohickey</td><td>$1.50</td><td>100</td></tr>
        </tbody>
      </table>
    </wiki-table>
  `;
  document.body.appendChild(container);
  const el = container.querySelector('wiki-table') as WikiTable;
  await el.updateComplete;
  return el;
}

describe('WikiTable', () => {

  afterEach(() => {
    document.querySelectorAll('wiki-table').forEach(el => el.parentElement?.remove());
  });

  describe('when connected with a basic table', () => {
    let el: WikiTable;

    beforeEach(async () => {
      el = await createBasicFixture();
    });

    it('should exist', () => {
      expect(el).to.exist;
    });

    it('should hide the original table', () => {
      const originalTable = el.querySelector('table');
      expect(originalTable?.style.display).to.equal('none');
    });

    it('should extract data from the table', () => {
      expect(el.extractedData).to.not.be.null;
      expect(el.extractedData!.columns).to.have.length(3);
      expect(el.extractedData!.rows).to.have.length(3);
    });

    it('should render an enhanced table in shadow DOM', () => {
      const shadowTable = el.shadowRoot?.querySelector('table');
      expect(shadowTable).to.exist;
    });

    it('should render column headers', () => {
      const headers = el.shadowRoot?.querySelectorAll('thead th');
      expect(headers).to.have.length(3);
    });

    it('should render header text in main area', () => {
      const headerMains = el.shadowRoot?.querySelectorAll('.header-main');
      expect(headerMains?.[0]?.textContent).to.contain('Name');
      expect(headerMains?.[1]?.textContent).to.contain('Price');
    });

    it('should render data rows', () => {
      const rows = el.shadowRoot?.querySelectorAll('tbody tr');
      expect(rows).to.have.length(3);
    });

    it('should render sort arrows in headers', () => {
      const arrows = el.shadowRoot?.querySelectorAll('.sort-arrows');
      expect(arrows).to.have.length(3);
    });
  });

  describe('status bar', () => {

    describe('when no filters are active', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
      });

      it('should render the status bar', () => {
        const statusBar = el.shadowRoot?.querySelector('.status-bar');
        expect(statusBar).to.exist;
      });

      it('should display total row count', () => {
        const rowCount = el.shadowRoot?.querySelector('.row-count');
        expect(rowCount?.textContent).to.contain('3 rows');
      });

      it('should not show row count as filtered', () => {
        const filtered = el.shadowRoot?.querySelector('.row-count-filtered');
        expect(filtered).to.not.exist;
      });

      it('should not show clear all button', () => {
        const clearAll = el.shadowRoot?.querySelector('[aria-label="Clear all filters"]');
        expect(clearAll).to.not.exist;
      });

      it('should show the view toggle', () => {
        const viewToggle = el.shadowRoot?.querySelector('[aria-label="View mode"]');
        expect(viewToggle).to.exist;
      });
    });

    describe('when filters are active', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.tableFilters = new Map([
          [0, { kind: 'checkbox', excludedValues: new Set(['Gadget', 'Doohickey']) }],
        ]);
        await el.updateComplete;
      });

      it('should show filtered row count', () => {
        const rowCount = el.shadowRoot?.querySelector('.row-count');
        expect(rowCount?.textContent).to.contain('1 of 3 rows');
      });

      it('should show row count with filtered styling', () => {
        const filtered = el.shadowRoot?.querySelector('.row-count-filtered');
        expect(filtered).to.exist;
      });

      it('should show clear all button', () => {
        const clearAll = el.shadowRoot?.querySelector('[aria-label="Clear all filters"]');
        expect(clearAll).to.exist;
      });
    });

    describe('when clicking clear all', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.tableFilters = new Map([
          [0, { kind: 'checkbox', excludedValues: new Set(['Gadget']) }],
        ]);
        await el.updateComplete;

        const clearAll = el.shadowRoot?.querySelector('[aria-label="Clear all filters"]') as HTMLButtonElement;
        clearAll.click();
        await el.updateComplete;
      });

      it('should clear all filters', () => {
        expect(el.tableFilters.size).to.equal(0);
      });

      it('should show all rows', () => {
        const rows = el.shadowRoot?.querySelectorAll('tbody tr');
        expect(rows).to.have.length(3);
      });

      it('should show total row count', () => {
        const rowCount = el.shadowRoot?.querySelector('.row-count');
        expect(rowCount?.textContent).to.contain('3 rows');
      });
    });
  });

  describe('view toggle pill', () => {

    describe('when clicking view toggle in table mode', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        const viewToggle = el.shadowRoot?.querySelector('[aria-label="View mode"]') as HTMLElement;
        viewToggle.click();
        await el.updateComplete;
      });

      it('should activate card view', () => {
        expect(el.cardViewActive).to.be.true;
      });

      it('should render cards instead of table', () => {
        const cardView = el.shadowRoot?.querySelector('.card-view');
        expect(cardView).to.exist;
      });
    });

    describe('when in card view', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.cardViewActive = true;
        await el.updateComplete;
      });

      it('should show the active view toggle segment', () => {
        const activeSegment = el.shadowRoot?.querySelector('.view-toggle-active');
        expect(activeSegment?.textContent).to.contain('cards');
      });

      it('should show sort/filter pill', () => {
        const sortFilterPill = el.shadowRoot?.querySelector('[aria-label="Sort and filter"]');
        expect(sortFilterPill).to.exist;
      });

      it('should not show sort/filter pill in table view', async () => {
        el.cardViewActive = false;
        await el.updateComplete;
        const sortFilterPill = el.shadowRoot?.querySelector('[aria-label="Sort and filter"]');
        expect(sortFilterPill).to.not.exist;
      });
    });
  });

  describe('card view column picker', () => {

    describe('when clicking sort/filter pill in card view', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.cardViewActive = true;
        await el.updateComplete;
        const pill = el.shadowRoot?.querySelector('[aria-label="Sort and filter"]') as HTMLButtonElement;
        pill.click();
        await el.updateComplete;
      });

      it('should open column picker', () => {
        expect(el.columnPickerOpen).to.be.true;
      });

      it('should show select column title', () => {
        const title = el.shadowRoot?.querySelector('.column-picker-title');
        expect(title?.textContent).to.contain('Select column');
      });
    });

    describe('when selecting a column from picker', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.cardViewActive = true;
        el.columnPickerOpen = true;
        await el.updateComplete;
        const items = el.shadowRoot?.querySelectorAll('.column-picker-item') as NodeListOf<HTMLButtonElement>;
        items[0]?.click();
        await el.updateComplete;
      });

      it('should close the column picker', () => {
        expect(el.columnPickerOpen).to.be.false;
      });

      it('should open the popover for the selected column', () => {
        expect(el.popoverColumnIndex).to.equal(0);
      });
    });
  });

  describe('header title attribute', () => {
    let el: WikiTable;

    beforeEach(async () => {
      el = await createBasicFixture();
    });

    it('should show detected type on header hover', () => {
      const headers = el.shadowRoot?.querySelectorAll('thead th');
      expect(headers?.[0]?.getAttribute('title')).to.contain('text');
      expect(headers?.[1]?.getAttribute('title')).to.contain('currency');
    });
  });

  describe('sort arrows shortcut', () => {

    describe('when clicking sort arrows once', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        const arrows = el.shadowRoot?.querySelector('.sort-arrows');
        arrows?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should set sort direction to ascending', () => {
        expect(el.sortDirection).to.equal('ascending');
      });

      it('should set sort column index', () => {
        expect(el.sortColumnIndex).to.equal(0);
      });

      it('should sort data alphabetically', () => {
        const firstCells = el.shadowRoot?.querySelectorAll('tbody tr td:first-child');
        const values = Array.from(firstCells ?? []).map(td => td.textContent);
        expect(values).to.deep.equal(['Doohickey', 'Gadget', 'Widget']);
      });
    });

    describe('when clicking sort arrows twice', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        const arrows = el.shadowRoot?.querySelector('.sort-arrows');
        arrows?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;
        arrows?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should set sort direction to descending', () => {
        expect(el.sortDirection).to.equal('descending');
      });
    });

    describe('when clicking sort arrows three times', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        const arrows = el.shadowRoot?.querySelector('.sort-arrows');
        arrows?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;
        arrows?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;
        arrows?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should reset sort direction to none', () => {
        expect(el.sortDirection).to.equal('none');
      });

      it('should reset sort column index', () => {
        expect(el.sortColumnIndex).to.be.null;
      });
    });
  });

  describe('popover opening', () => {

    describe('when clicking header main area', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        const headerMain = el.shadowRoot?.querySelector('.header-main');
        headerMain?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should set popoverColumnIndex', () => {
        expect(el.popoverColumnIndex).to.equal(0);
      });

      it('should render the filter popover', () => {
        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        expect(popover).to.exist;
      });

      it('should render a popover overlay backdrop', () => {
        const overlay = el.shadowRoot?.querySelector('.popover-overlay');
        expect(overlay).to.exist;
      });
    });

    describe('when clicking the popover overlay', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.popoverColumnIndex = 0;
        await el.updateComplete;

        const overlay = el.shadowRoot?.querySelector('.popover-overlay') as HTMLElement;
        overlay.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should close the popover', () => {
        expect(el.popoverColumnIndex).to.be.null;
      });
    });

    describe('when popover-closed event fires', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.popoverColumnIndex = 0;
        await el.updateComplete;

        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        popover?.dispatchEvent(new CustomEvent('popover-closed', { bubbles: true, composed: true }));
        await el.updateComplete;
      });

      it('should close the popover', () => {
        expect(el.popoverColumnIndex).to.be.null;
      });
    });
  });

  describe('popover filter integration', () => {

    describe('when popover emits filter-changed', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.popoverColumnIndex = 0;
        await el.updateComplete;

        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        popover?.dispatchEvent(new CustomEvent('filter-changed', {
          detail: { filter: { kind: 'checkbox', excludedValues: new Set(['Gadget']) } },
          bubbles: true,
          composed: true,
        }));
        await el.updateComplete;
      });

      it('should update tableFilters', () => {
        expect(el.tableFilters.has(0)).to.be.true;
      });

      it('should filter the rows', () => {
        const rows = el.shadowRoot?.querySelectorAll('tbody tr');
        expect(rows).to.have.length(2);
      });
    });

    describe('when popover emits sort-direction-changed', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.popoverColumnIndex = 0;
        await el.updateComplete;

        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        popover?.dispatchEvent(new CustomEvent('sort-direction-changed', {
          detail: { direction: 'descending' },
          bubbles: true,
          composed: true,
        }));
        await el.updateComplete;
      });

      it('should update sort direction', () => {
        expect(el.sortDirection).to.equal('descending');
      });

      it('should update sort column index', () => {
        expect(el.sortColumnIndex).to.equal(0);
      });
    });
  });

  describe('filter badge visibility', () => {

    describe('when a column has an active filter', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.tableFilters = new Map([
          [0, { kind: 'checkbox', excludedValues: new Set(['Gadget']) }],
        ]);
        await el.updateComplete;
      });

      it('should show a filter dot on the filtered column', () => {
        const filterDots = el.shadowRoot?.querySelectorAll('.filter-dot');
        expect(filterDots).to.have.length(1);
      });
    });

    describe('when no columns have active filters', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
      });

      it('should not show any filter dots', () => {
        const filterDots = el.shadowRoot?.querySelectorAll('.filter-dot');
        expect(filterDots).to.have.length(0);
      });
    });
  });

  describe('sorting numeric columns', () => {
    let el: WikiTable;

    beforeEach(async () => {
      el = await createBasicFixture();
      const arrows = el.shadowRoot?.querySelectorAll('.sort-arrows');
      arrows?.[2]?.dispatchEvent(new Event('click', { bubbles: true }));
      await el.updateComplete;
    });

    it('should sort numerically, not lexicographically', () => {
      const countCells = el.shadowRoot?.querySelectorAll('tbody tr td:nth-child(3)');
      const values = Array.from(countCells ?? []).map(td => td.textContent);
      expect(values).to.deep.equal(['5', '10', '100']);
    });
  });

  describe('card view', () => {

    describe('when toggled to card view', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.cardViewActive = true;
        await el.updateComplete;
      });

      it('should render cards instead of table', () => {
        const cardView = el.shadowRoot?.querySelector('.card-view');
        expect(cardView).to.exist;
      });

      it('should render one card per row', () => {
        const cards = el.shadowRoot?.querySelectorAll('.card');
        expect(cards).to.have.length(3);
      });

      it('should show label-value pairs in cards', () => {
        const firstCard = el.shadowRoot?.querySelector('.card');
        const labels = firstCard?.querySelectorAll('.card-label');
        expect(labels).to.have.length(3);
        expect(labels?.[0]?.textContent).to.equal('Name');
      });

      it('should not render a table element', () => {
        const table = el.shadowRoot?.querySelector('table');
        expect(table).to.not.exist;
      });
    });
  });

  describe('card view column picker overlay', () => {

    describe('when column picker is open in card view', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.cardViewActive = true;
        el.columnPickerOpen = true;
        await el.updateComplete;
      });

      it('should render column picker overlay', () => {
        const overlay = el.shadowRoot?.querySelector('.column-picker-overlay');
        expect(overlay).to.exist;
      });

      it('should list all columns', () => {
        const items = el.shadowRoot?.querySelectorAll('.column-picker-item');
        expect(items).to.have.length(3);
      });
    });

    describe('when selecting a column from picker', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.cardViewActive = true;
        el.columnPickerOpen = true;
        await el.updateComplete;
        const items = el.shadowRoot?.querySelectorAll('.column-picker-item') as NodeListOf<HTMLButtonElement>;
        items[1]?.click();
        await el.updateComplete;
      });

      it('should close the column picker', () => {
        expect(el.columnPickerOpen).to.be.false;
      });

      it('should open the popover for the selected column', () => {
        expect(el.popoverColumnIndex).to.equal(1);
      });
    });
  });

  describe('when media query triggers card view', () => {
    let el: WikiTable;
    let matchMediaStub: SinonStub;

    beforeEach(async () => {
      matchMediaStub = sinon.stub(window, 'matchMedia');
      matchMediaStub.returns({
        matches: true,
        addEventListener: sinon.stub(),
        removeEventListener: sinon.stub(),
      });
      el = await createBasicFixture();
    });

    afterEach(() => {
      matchMediaStub.restore();
    });

    it('should start in card view on narrow screens', () => {
      expect(el.cardViewActive).to.be.true;
    });
  });

  describe('progressive enhancement', () => {

    describe('when JavaScript processes the table', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
      });

      it('should still contain the original table in light DOM', () => {
        const lightTable = el.querySelector('table');
        expect(lightTable).to.exist;
      });
    });
  });

  describe('when connected without a table child', () => {
    let el: WikiTable;

    beforeEach(async () => {
      const container = document.createElement('div');
      container.innerHTML = '<wiki-table><p>No table here</p></wiki-table>';
      document.body.appendChild(container);
      el = container.querySelector('wiki-table') as WikiTable;
      await el.updateComplete;
    });

    it('should not extract data', () => {
      expect(el.extractedData).to.be.null;
    });

    it('should render a slot for the content', () => {
      const slot = el.shadowRoot?.querySelector('slot');
      expect(slot).to.exist;
    });
  });

  describe('end-to-end popover filter flow', () => {

    describe('when opening popover for a text column', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        const headerMain = el.shadowRoot?.querySelector('.header-main');
        headerMain?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should render checkbox filter for text column with few values', () => {
        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        const checkboxList = popover?.shadowRoot?.querySelector('.checkbox-list');
        expect(checkboxList).to.exist;
      });

      it('should show checkboxes for each unique value', () => {
        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        const items = popover?.shadowRoot?.querySelectorAll('.checkbox-item');
        expect(items).to.have.length(3);
      });
    });

    describe('when opening popover for a currency column', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        const headerMains = el.shadowRoot?.querySelectorAll('.header-main');
        headerMains?.[1]?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should render range filter for currency column', () => {
        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        const rangeContainer = popover?.shadowRoot?.querySelector('.range-container');
        expect(rangeContainer).to.exist;
      });
    });

    describe('when unchecking a value in the popover checkbox filter', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        const headerMain = el.shadowRoot?.querySelector('.header-main');
        headerMain?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;

        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        const checkboxes = popover?.shadowRoot?.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        checkboxes[0]!.checked = false;
        checkboxes[0]!.dispatchEvent(new Event('change'));
        await popover!.updateComplete;

        const okBtn = popover?.shadowRoot?.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
        await el.updateComplete;
      });

      it('should filter rows in the table', () => {
        const rows = el.shadowRoot?.querySelectorAll('tbody tr');
        expect(rows).to.have.length(2);
      });

      it('should update the status bar row count', () => {
        const rowCount = el.shadowRoot?.querySelector('.row-count');
        expect(rowCount?.textContent).to.contain('2 of 3 rows');
      });

      it('should show a filter dot on the column header', () => {
        const filterDots = el.shadowRoot?.querySelectorAll('.filter-dot');
        expect(filterDots).to.have.length(1);
      });
    });

    describe('when setting range min in the popover range filter', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        const headerMains = el.shadowRoot?.querySelectorAll('.header-main');
        headerMains?.[2]?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;

        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        const minInput = popover?.shadowRoot?.querySelector('[aria-label="Minimum value"]') as HTMLInputElement;
        minInput.value = '10';
        minInput.dispatchEvent(new Event('input'));
        await popover!.updateComplete;

        const okBtn = popover?.shadowRoot?.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
        await el.updateComplete;
      });

      it('should filter rows by numeric range', () => {
        const rows = el.shadowRoot?.querySelectorAll('tbody tr');
        expect(rows).to.have.length(2);
      });

      it('should show filtered row count', () => {
        const rowCount = el.shadowRoot?.querySelector('.row-count');
        expect(rowCount?.textContent).to.contain('2 of 3 rows');
      });
    });

    describe('when clicking ascending sort in popover', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        const headerMain = el.shadowRoot?.querySelector('.header-main');
        headerMain?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;

        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        const ascBtn = popover?.shadowRoot?.querySelector('[aria-label="Sort ascending"]') as HTMLButtonElement;
        ascBtn.click();
        await popover!.updateComplete;

        const okBtn = popover?.shadowRoot?.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
        await el.updateComplete;
      });

      it('should sort table rows ascending', () => {
        const firstCells = el.shadowRoot?.querySelectorAll('tbody tr td:first-child');
        const values = Array.from(firstCells ?? []).map(td => td.textContent);
        expect(values).to.deep.equal(['Doohickey', 'Gadget', 'Widget']);
      });

      it('should show ascending sort indicator on the column', () => {
        const sortedTh = el.shadowRoot?.querySelector('th.sorted');
        expect(sortedTh).to.exist;
      });
    });

    describe('when filter is applied, popover closed, then reopened', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        const headerMain = el.shadowRoot?.querySelector('.header-main');
        headerMain?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;

        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        const checkboxes = popover?.shadowRoot?.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        checkboxes[0]!.checked = false;
        checkboxes[0]!.dispatchEvent(new Event('change'));
        await popover!.updateComplete;

        const okBtn = popover?.shadowRoot?.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
        await el.updateComplete;

        headerMain?.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should show the previously excluded value still unchecked', () => {
        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        const checkboxes = popover?.shadowRoot?.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        expect(checkboxes[0]!.checked).to.be.false;
      });

      it('should still show the filtered rows', () => {
        const rows = el.shadowRoot?.querySelectorAll('tbody tr');
        expect(rows).to.have.length(2);
      });
    });

    describe('when filtering two columns simultaneously', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.tableFilters = new Map([
          [0, { kind: 'checkbox', excludedValues: new Set(['Widget']) }],
          [2, { kind: 'range', min: 10, max: null }],
        ]);
        await el.updateComplete;
      });

      it('should apply both filters to the rows', () => {
        const rows = el.shadowRoot?.querySelectorAll('tbody tr');
        expect(rows).to.have.length(1);
      });

      it('should show filter dots on both columns', () => {
        const filterDots = el.shadowRoot?.querySelectorAll('.filter-dot');
        expect(filterDots).to.have.length(2);
      });

      it('should show the correct filtered count', () => {
        const rowCount = el.shadowRoot?.querySelector('.row-count');
        expect(rowCount?.textContent).to.contain('1 of 3 rows');
      });
    });
  });

  describe('end-to-end card view picker flows', () => {

    describe('when using filter picker to open popover in card view', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.cardViewActive = true;
        await el.updateComplete;

        const filterPill = el.shadowRoot?.querySelector('[aria-label="Sort and filter"]') as HTMLButtonElement;
        filterPill.click();
        await el.updateComplete;

        const items = el.shadowRoot?.querySelectorAll('.column-picker-item') as NodeListOf<HTMLButtonElement>;
        items[0]?.click();
        await el.updateComplete;
      });

      it('should open the popover', () => {
        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        expect(popover).to.exist;
      });

      it('should show checkbox filter in the popover', () => {
        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        const checkboxList = popover?.shadowRoot?.querySelector('.checkbox-list');
        expect(checkboxList).to.exist;
      });
    });

    describe('when applying filter via card view picker flow', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.cardViewActive = true;
        await el.updateComplete;

        const filterPill = el.shadowRoot?.querySelector('[aria-label="Sort and filter"]') as HTMLButtonElement;
        filterPill.click();
        await el.updateComplete;

        const items = el.shadowRoot?.querySelectorAll('.column-picker-item') as NodeListOf<HTMLButtonElement>;
        items[0]?.click();
        await el.updateComplete;

        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        const checkboxes = popover?.shadowRoot?.querySelectorAll('.checkbox-item input[type="checkbox"]') as NodeListOf<HTMLInputElement>;
        checkboxes[0]!.checked = false;
        checkboxes[0]!.dispatchEvent(new Event('change'));
        await popover!.updateComplete;

        const okBtn = popover?.shadowRoot?.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
        await el.updateComplete;
      });

      it('should filter cards', () => {
        const cards = el.shadowRoot?.querySelectorAll('.card');
        expect(cards).to.have.length(2);
      });

      it('should show filtered row count', () => {
        const rowCount = el.shadowRoot?.querySelector('.row-count');
        expect(rowCount?.textContent).to.contain('2 of 3 rows');
      });
    });

    describe('when sorting via popover in card view', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.cardViewActive = true;
        await el.updateComplete;

        const pill = el.shadowRoot?.querySelector('[aria-label="Sort and filter"]') as HTMLButtonElement;
        pill.click();
        await el.updateComplete;

        const items = el.shadowRoot?.querySelectorAll('.column-picker-item') as NodeListOf<HTMLButtonElement>;
        items[0]?.click();
        await el.updateComplete;

        const popover = el.shadowRoot?.querySelector('table-filter-popover');
        const ascBtn = popover?.shadowRoot?.querySelector('[aria-label="Sort ascending"]') as HTMLButtonElement;
        ascBtn.click();
        await popover!.updateComplete;

        const okBtn = popover?.shadowRoot?.querySelector('[aria-label="Apply"]') as HTMLButtonElement;
        okBtn.click();
        await el.updateComplete;
      });

      it('should sort the cards', () => {
        const cards = el.shadowRoot?.querySelectorAll('.card');
        const firstCardValue = cards?.[0]?.querySelector('.card-value');
        expect(firstCardValue?.textContent).to.equal('Doohickey');
      });
    });

    describe('when clicking column picker overlay background', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.cardViewActive = true;
        el.columnPickerOpen = true;
        await el.updateComplete;

        const overlay = el.shadowRoot?.querySelector('.column-picker-overlay') as HTMLElement;
        overlay.dispatchEvent(new Event('click', { bubbles: true }));
        await el.updateComplete;
      });

      it('should close the column picker', () => {
        expect(el.columnPickerOpen).to.be.false;
      });
    });
  });

  describe('end-to-end clear all flow', () => {

    describe('when clearing all after sort and filter are active', () => {
      let el: WikiTable;

      beforeEach(async () => {
        el = await createBasicFixture();
        el.sortColumnIndex = 0;
        el.sortDirection = 'ascending';
        el.tableFilters = new Map([
          [0, { kind: 'checkbox', excludedValues: new Set(['Widget']) }],
        ]);
        await el.updateComplete;

        const clearAll = el.shadowRoot?.querySelector('[aria-label="Clear all filters"]') as HTMLButtonElement;
        clearAll.click();
        await el.updateComplete;
      });

      it('should clear all filters', () => {
        expect(el.tableFilters.size).to.equal(0);
      });

      it('should show all rows', () => {
        const rows = el.shadowRoot?.querySelectorAll('tbody tr');
        expect(rows).to.have.length(3);
      });

      it('should preserve the sort', () => {
        expect(el.sortDirection).to.equal('ascending');
      });
    });
  });

  describe('cleanup on disconnect', () => {
    let el: WikiTable;
    let matchMediaStub: SinonStub;
    let removeListenerSpy: sinon.SinonSpy;

    beforeEach(async () => {
      removeListenerSpy = sinon.spy();
      matchMediaStub = sinon.stub(window, 'matchMedia');
      matchMediaStub.returns({
        matches: false,
        addEventListener: sinon.stub(),
        removeEventListener: removeListenerSpy,
      });
      el = await createBasicFixture();
      el.remove();
    });

    afterEach(() => {
      matchMediaStub.restore();
    });

    it('should remove media query listener on disconnect', () => {
      expect(removeListenerSpy).to.have.been.calledOnce;
    });
  });
});
