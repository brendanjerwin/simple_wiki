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
      expect(headers?.[0]?.textContent).to.contain('Name');
      expect(headers?.[1]?.textContent).to.contain('Price');
    });

    it('should render data rows', () => {
      const rows = el.shadowRoot?.querySelectorAll('tbody tr');
      expect(rows).to.have.length(3);
    });

    it('should display total row count', () => {
      const rowCount = el.shadowRoot?.querySelector('.row-count');
      expect(rowCount?.textContent).to.contain('3 rows');
    });

    it('should render sort indicators on headers', () => {
      const indicators = el.shadowRoot?.querySelectorAll('.sort-indicator');
      expect(indicators).to.have.length(3);
    });
  });

  describe('when clicking a column header', () => {
    let el: WikiTable;

    beforeEach(async () => {
      el = await createBasicFixture();
    });

    describe('when clicking once (ascending)', () => {
      beforeEach(async () => {
        const header = el.shadowRoot?.querySelector('thead th');
        header?.dispatchEvent(new Event('click'));
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

    describe('when clicking twice (descending)', () => {
      beforeEach(async () => {
        const header = el.shadowRoot?.querySelector('thead th');
        header?.dispatchEvent(new Event('click'));
        await el.updateComplete;
        header?.dispatchEvent(new Event('click'));
        await el.updateComplete;
      });

      it('should set sort direction to descending', () => {
        expect(el.sortDirection).to.equal('descending');
      });

      it('should sort data reverse alphabetically', () => {
        const firstCells = el.shadowRoot?.querySelectorAll('tbody tr td:first-child');
        const values = Array.from(firstCells ?? []).map(td => td.textContent);
        expect(values).to.deep.equal(['Widget', 'Gadget', 'Doohickey']);
      });
    });

    describe('when clicking three times (reset)', () => {
      beforeEach(async () => {
        const header = el.shadowRoot?.querySelector('thead th');
        header?.dispatchEvent(new Event('click'));
        await el.updateComplete;
        header?.dispatchEvent(new Event('click'));
        await el.updateComplete;
        header?.dispatchEvent(new Event('click'));
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

  describe('when sorting numeric columns', () => {
    let el: WikiTable;

    beforeEach(async () => {
      el = await createBasicFixture();
      const headers = el.shadowRoot?.querySelectorAll('thead th');
      headers?.[2]?.dispatchEvent(new Event('click'));
      await el.updateComplete;
    });

    it('should sort numerically, not lexicographically', () => {
      const countCells = el.shadowRoot?.querySelectorAll('tbody tr td:nth-child(3)');
      const values = Array.from(countCells ?? []).map(td => td.textContent);
      expect(values).to.deep.equal(['5', '10', '100']);
    });
  });

  describe('when toggling filters', () => {
    let el: WikiTable;

    beforeEach(async () => {
      el = await createBasicFixture();
      const filterBtn = el.shadowRoot?.querySelector('[aria-label="Toggle filters"]') as HTMLButtonElement;
      filterBtn?.click();
      await el.updateComplete;
    });

    it('should show filter inputs', () => {
      expect(el.filtersVisible).to.be.true;
    });

    it('should render filter input fields', () => {
      const filterInputs = el.shadowRoot?.querySelectorAll('.filter-input');
      expect(filterInputs).to.have.length(3);
    });
  });

  describe('when entering a filter value', () => {
    let el: WikiTable;

    beforeEach(async () => {
      el = await createBasicFixture();
      const filterBtn = el.shadowRoot?.querySelector('[aria-label="Toggle filters"]') as HTMLButtonElement;
      filterBtn?.click();
      await el.updateComplete;

      const filterInput = el.shadowRoot?.querySelector('.filter-input') as HTMLInputElement;
      filterInput.value = 'Widget';
      filterInput.dispatchEvent(new Event('input'));
      await el.updateComplete;
    });

    it('should filter rows by the input', () => {
      const rows = el.shadowRoot?.querySelectorAll('tbody tr');
      expect(rows).to.have.length(1);
    });

    it('should show filtered row count', () => {
      const rowCount = el.shadowRoot?.querySelector('.row-count');
      expect(rowCount?.textContent).to.contain('1 of 3');
    });
  });

  describe('when clearing filters', () => {
    let el: WikiTable;

    beforeEach(async () => {
      el = await createBasicFixture();
      const filterBtn = el.shadowRoot?.querySelector('[aria-label="Toggle filters"]') as HTMLButtonElement;
      filterBtn?.click();
      await el.updateComplete;

      const filterInput = el.shadowRoot?.querySelector('.filter-input') as HTMLInputElement;
      filterInput.value = 'Widget';
      filterInput.dispatchEvent(new Event('input'));
      await el.updateComplete;

      const clearBtn = el.shadowRoot?.querySelector('[aria-label="Clear filters"]') as HTMLButtonElement;
      clearBtn?.click();
      await el.updateComplete;
    });

    it('should show all rows again', () => {
      const rows = el.shadowRoot?.querySelectorAll('tbody tr');
      expect(rows).to.have.length(3);
    });

    it('should show total row count', () => {
      const rowCount = el.shadowRoot?.querySelector('.row-count');
      expect(rowCount?.textContent).to.contain('3 rows');
    });
  });

  describe('when toggling card view', () => {
    let el: WikiTable;

    beforeEach(async () => {
      el = await createBasicFixture();
      const cardBtn = el.shadowRoot?.querySelector('[aria-label="Toggle card view"]') as HTMLButtonElement;
      cardBtn?.click();
      await el.updateComplete;
    });

    it('should activate card view', () => {
      expect(el.cardViewActive).to.be.true;
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
