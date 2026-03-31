import { expect } from '@open-wc/testing';
import {
  INVENTORY_ITEM_TEMPLATE,
  SYSTEM_TEMPLATE_IDENTIFIERS,
} from './system-templates.js';

describe('system-templates', () => {
  describe('INVENTORY_ITEM_TEMPLATE', () => {
    it('should equal "inv_item"', () => {
      expect(INVENTORY_ITEM_TEMPLATE).to.equal('inv_item');
    });
  });

  describe('SYSTEM_TEMPLATE_IDENTIFIERS', () => {
    it('should exist', () => {
      expect(SYSTEM_TEMPLATE_IDENTIFIERS).to.exist;
    });

    it('should be a non-empty array', () => {
      expect(SYSTEM_TEMPLATE_IDENTIFIERS.length).to.be.greaterThan(0);
    });

    it('should contain INVENTORY_ITEM_TEMPLATE', () => {
      expect(SYSTEM_TEMPLATE_IDENTIFIERS).to.include(INVENTORY_ITEM_TEMPLATE);
    });

    describe('exclusion list synchronization', () => {
      it('should contain exactly the known system template identifiers', () => {
        expect(Array.from(SYSTEM_TEMPLATE_IDENTIFIERS)).to.deep.equal([
          INVENTORY_ITEM_TEMPLATE,
        ]);
      });

      it('should have INVENTORY_ITEM_TEMPLATE as the only entry', () => {
        expect(SYSTEM_TEMPLATE_IDENTIFIERS).to.have.lengthOf(1);
      });
    });
  });
});
