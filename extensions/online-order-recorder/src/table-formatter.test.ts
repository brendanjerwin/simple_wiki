import { describe, it, expect, beforeEach } from 'vitest';
import {
  formatCentsAsDollars,
  formatOrderRow,
  isDuplicate,
  appendRowsToTable,
} from './table-formatter.js';
import { Order } from './merchants/types.js';

describe('formatCentsAsDollars', () => {
  describe('when given zero cents', () => {
    let result: string;

    beforeEach(() => {
      result = formatCentsAsDollars(0);
    });

    it('should return $0.00', () => {
      expect(result).to.equal('$0.00');
    });
  });

  describe('when given a value under one dollar', () => {
    let result: string;

    beforeEach(() => {
      result = formatCentsAsDollars(99);
    });

    it('should return $0.99', () => {
      expect(result).to.equal('$0.99');
    });
  });

  describe('when given a round dollar amount', () => {
    let result: string;

    beforeEach(() => {
      result = formatCentsAsDollars(1000);
    });

    it('should return $10.00', () => {
      expect(result).to.equal('$10.00');
    });
  });

  describe('when given a typical price', () => {
    let result: string;

    beforeEach(() => {
      result = formatCentsAsDollars(2999);
    });

    it('should return $29.99', () => {
      expect(result).to.equal('$29.99');
    });
  });

  describe('when given a large value', () => {
    let result: string;

    beforeEach(() => {
      result = formatCentsAsDollars(123456);
    });

    it('should return $1234.56', () => {
      expect(result).to.equal('$1234.56');
    });
  });

  describe('when given a single-digit cents value', () => {
    let result: string;

    beforeEach(() => {
      result = formatCentsAsDollars(105);
    });

    it('should pad cents with leading zero', () => {
      expect(result).to.equal('$1.05');
    });
  });
});

describe('formatOrderRow', () => {
  describe('when given an order with a single item', () => {
    let result: string;

    beforeEach(() => {
      const order: Order = {
        merchant: 'Amazon',
        orderNumber: '222-3456789-0123456',
        orderDate: '2026-03-01',
        items: [{ name: 'Water Bottle', priceCents: 1699, quantity: 1 }],
        totalCents: 1699,
        deliveryStatus: 'Delivered',
      };
      result = formatOrderRow(order);
    });

    it('should produce a pipe-delimited row', () => {
      expect(result).to.equal(
        '| 2026-03-01 | Amazon | 222-3456789-0123456 | Water Bottle | $16.99 | $16.99 | Delivered |'
      );
    });
  });

  describe('when given an order with multiple items', () => {
    let result: string;

    beforeEach(() => {
      const order: Order = {
        merchant: 'Amazon',
        orderNumber: '111-2345678-9012345',
        orderDate: '2026-03-05',
        items: [
          { name: 'Headphones', priceCents: 2999, quantity: 1 },
          { name: 'Cable', priceCents: 1553, quantity: 1 },
        ],
        totalCents: 4552,
        deliveryStatus: 'Arriving March 12',
      };
      result = formatOrderRow(order);
    });

    it('should join item names with semicolons', () => {
      expect(result).to.contain('Headphones; Cable');
    });

    it('should join item prices with semicolons', () => {
      expect(result).to.contain('$29.99; $15.53');
    });
  });

  describe('when an item has quantity greater than 1', () => {
    let result: string;

    beforeEach(() => {
      const order: Order = {
        merchant: 'Amazon',
        orderNumber: '111-0000000-0000000',
        orderDate: '2026-03-05',
        items: [{ name: 'Widget', priceCents: 999, quantity: 2 }],
        totalCents: 1998,
        deliveryStatus: 'Delivered',
      };
      result = formatOrderRow(order);
    });

    it('should include quantity suffix', () => {
      expect(result).to.contain('Widget x2');
    });
  });
});

describe('isDuplicate', () => {
  const existingMarkdown =
    '| 2026-03-01 | Amazon | 222-3456789-0123456 | Water Bottle | $16.99 | $16.99 | Delivered |\n';

  describe('when the order number exists in the markdown', () => {
    let result: boolean;

    beforeEach(() => {
      result = isDuplicate(existingMarkdown, '222-3456789-0123456');
    });

    it('should return true', () => {
      expect(result).to.be.true;
    });
  });

  describe('when the order number does not exist', () => {
    let result: boolean;

    beforeEach(() => {
      result = isDuplicate(existingMarkdown, '999-0000000-0000000');
    });

    it('should return false', () => {
      expect(result).to.be.false;
    });
  });
});

describe('appendRowsToTable', () => {
  const sampleRow =
    '| 2026-03-01 | Amazon | 222-3456789-0123456 | Water Bottle | $16.99 | $16.99 | Delivered |';

  describe('when existing markdown is empty', () => {
    let result: string;

    beforeEach(() => {
      result = appendRowsToTable('', [sampleRow]);
    });

    it('should create a table with header', () => {
      expect(result).to.contain('| Date | Merchant | Order # | Items | Prices | Total | Status |');
    });

    it('should include the separator row', () => {
      expect(result).to.contain('|------|----------|---------|-------|--------|-------|--------|');
    });

    it('should include the data row', () => {
      expect(result).to.contain(sampleRow);
    });

    it('should end with a newline', () => {
      expect(result).to.match(/\n$/);
    });
  });

  describe('when existing markdown already has the table', () => {
    let result: string;
    const existingTable =
      '| Date | Merchant | Order # | Items | Prices | Total | Status |\n' +
      '|------|----------|---------|-------|--------|-------|--------|\n' +
      '| 2026-02-28 | Amazon | 111-0000000-0000000 | Widget | $9.99 | $9.99 | Delivered |\n';

    beforeEach(() => {
      result = appendRowsToTable(existingTable, [sampleRow]);
    });

    it('should append the new row', () => {
      expect(result).to.contain(sampleRow);
    });

    it('should keep the existing row', () => {
      expect(result).to.contain('111-0000000-0000000');
    });

    it('should not duplicate the header', () => {
      const headerCount = (result.match(/\| Date \| Merchant/g) || []).length;
      expect(headerCount).to.equal(1);
    });
  });

  describe('when existing markdown has other content but no table', () => {
    let result: string;

    beforeEach(() => {
      result = appendRowsToTable('# My Orders\n\nSome notes here.', [sampleRow]);
    });

    it('should append the table after existing content', () => {
      expect(result).to.match(/^# My Orders/);
    });

    it('should include the table header', () => {
      expect(result).to.contain('| Date | Merchant | Order # |');
    });

    it('should include the data row', () => {
      expect(result).to.contain(sampleRow);
    });
  });

  describe('when no new rows are provided', () => {
    let result: string;

    beforeEach(() => {
      result = appendRowsToTable('existing content', []);
    });

    it('should return the existing markdown unchanged', () => {
      expect(result).to.equal('existing content');
    });
  });
});
