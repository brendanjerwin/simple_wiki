import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import { InventoryActionService, inventoryActionService } from './inventory-action-service.js';

describe('InventoryActionService', () => {
  let service: InventoryActionService;

  beforeEach(() => {
    service = new InventoryActionService();
  });

  afterEach(() => {
    sinon.restore();
  });

  it('should exist', () => {
    expect(service).to.exist;
  });

  describe('addItem', () => {
    describe('when called with empty container identifier', () => {
      let result: Awaited<ReturnType<typeof service.addItem>>;

      beforeEach(async () => {
        result = await service.addItem('', 'screwdriver');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should return validation error', () => {
        expect(result.error).to.equal('Container and item identifier are required');
      });
    });

    describe('when called with empty item identifier', () => {
      let result: Awaited<ReturnType<typeof service.addItem>>;

      beforeEach(async () => {
        result = await service.addItem('drawer_kitchen', '');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should return validation error', () => {
        expect(result.error).to.equal('Container and item identifier are required');
      });
    });

    describe('when called with valid parameters and client returns success', () => {
      let result: Awaited<ReturnType<typeof service.addItem>>;
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        // Access private client via type assertion and stub its method
        const serviceWithClient = service as unknown as { client: { createInventoryItem: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.client, 'createInventoryItem').resolves({
          success: true,
          itemIdentifier: 'screwdriver',
          summary: 'Created screwdriver in drawer_kitchen',
        });

        result = await service.addItem('drawer_kitchen', 'screwdriver', 'Phillips Screwdriver');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should return the item identifier', () => {
        expect(result.itemIdentifier).to.equal('screwdriver');
      });

      it('should return the summary', () => {
        expect(result.summary).to.equal('Created screwdriver in drawer_kitchen');
      });

      it('should call client with correct request', () => {
        expect(clientStub).to.have.been.calledOnce;
        const request = clientStub.firstCall.args[0];
        expect(request.itemIdentifier).to.equal('screwdriver');
        expect(request.container).to.equal('drawer_kitchen');
        expect(request.title).to.equal('Phillips Screwdriver');
      });
    });

    describe('when called with valid parameters and client returns error response', () => {
      let result: Awaited<ReturnType<typeof service.addItem>>;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { client: { createInventoryItem: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.client, 'createInventoryItem').resolves({
          success: false,
          error: 'Item already exists',
        });

        result = await service.addItem('drawer_kitchen', 'screwdriver');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should return the error message', () => {
        expect(result.error).to.equal('Item already exists');
      });
    });

    describe('when called with valid parameters and client throws', () => {
      let result: Awaited<ReturnType<typeof service.addItem>>;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { client: { createInventoryItem: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.client, 'createInventoryItem').rejects(new Error('Network error'));

        result = await service.addItem('drawer_kitchen', 'screwdriver');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should return an error message', () => {
        expect(result.error).to.exist;
      });
    });
  });

  describe('moveItem', () => {
    describe('when called with empty item identifier', () => {
      let result: Awaited<ReturnType<typeof service.moveItem>>;

      beforeEach(async () => {
        result = await service.moveItem('', 'toolbox_garage');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should return validation error', () => {
        expect(result.error).to.equal('Item identifier is required');
      });
    });

    describe('when called with valid parameters and client returns success', () => {
      let result: Awaited<ReturnType<typeof service.moveItem>>;
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { client: { moveInventoryItem: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.client, 'moveInventoryItem').resolves({
          success: true,
          previousContainer: 'drawer_kitchen',
          newContainer: 'toolbox_garage',
          summary: 'Moved screwdriver from drawer_kitchen to toolbox_garage',
        });

        result = await service.moveItem('screwdriver', 'toolbox_garage');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should return the previous container', () => {
        expect(result.previousContainer).to.equal('drawer_kitchen');
      });

      it('should return the new container', () => {
        expect(result.newContainer).to.equal('toolbox_garage');
      });

      it('should return the summary', () => {
        expect(result.summary).to.equal('Moved screwdriver from drawer_kitchen to toolbox_garage');
      });

      it('should call client with correct request', () => {
        expect(clientStub).to.have.been.calledOnce;
        const request = clientStub.firstCall.args[0];
        expect(request.itemIdentifier).to.equal('screwdriver');
        expect(request.newContainer).to.equal('toolbox_garage');
      });
    });

    describe('when called with valid parameters and client returns error response', () => {
      let result: Awaited<ReturnType<typeof service.moveItem>>;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { client: { moveInventoryItem: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.client, 'moveInventoryItem').resolves({
          success: false,
          error: 'Container not found',
        });

        result = await service.moveItem('screwdriver', 'nonexistent');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should return the error message', () => {
        expect(result.error).to.equal('Container not found');
      });
    });

    describe('when called with valid parameters and client throws', () => {
      let result: Awaited<ReturnType<typeof service.moveItem>>;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { client: { moveInventoryItem: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.client, 'moveInventoryItem').rejects(new Error('Network error'));

        result = await service.moveItem('screwdriver', 'toolbox_garage');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should return an error message', () => {
        expect(result.error).to.exist;
      });
    });
  });

  describe('findItem', () => {
    describe('when called with empty item identifier', () => {
      let result: Awaited<ReturnType<typeof service.findItem>>;

      beforeEach(async () => {
        result = await service.findItem('');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should return validation error', () => {
        expect(result.error).to.equal('Item identifier is required');
      });
    });

    describe('when called with valid parameters and item is found', () => {
      let result: Awaited<ReturnType<typeof service.findItem>>;
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { client: { findItemLocation: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.client, 'findItemLocation').resolves({
          found: true,
          locations: [
            { container: 'drawer_kitchen', path: ['house', 'kitchen', 'drawer_kitchen'] },
          ],
          summary: 'Found screwdriver in drawer_kitchen',
        });

        result = await service.findItem('screwdriver');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should return found true', () => {
        expect(result.found).to.be.true;
      });

      it('should return locations array', () => {
        expect(result.locations).to.have.length(1);
        expect(result.locations![0].container).to.equal('drawer_kitchen');
        expect(result.locations![0].path).to.deep.equal(['house', 'kitchen', 'drawer_kitchen']);
      });

      it('should return the summary', () => {
        expect(result.summary).to.equal('Found screwdriver in drawer_kitchen');
      });

      it('should call client with correct request', () => {
        expect(clientStub).to.have.been.calledOnce;
        const request = clientStub.firstCall.args[0];
        expect(request.itemIdentifier).to.equal('screwdriver');
        expect(request.includeHierarchy).to.be.true;
      });
    });

    describe('when item is found in multiple locations', () => {
      let result: Awaited<ReturnType<typeof service.findItem>>;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { client: { findItemLocation: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.client, 'findItemLocation').resolves({
          found: true,
          locations: [
            { container: 'drawer_kitchen', path: ['kitchen', 'drawer_kitchen'] },
            { container: 'toolbox_garage', path: ['garage', 'toolbox_garage'] },
          ],
          summary: 'Found in 2 locations',
        });

        result = await service.findItem('screwdriver');
      });

      it('should return all locations', () => {
        expect(result.locations).to.have.length(2);
      });
    });

    describe('when item is not found', () => {
      let result: Awaited<ReturnType<typeof service.findItem>>;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { client: { findItemLocation: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.client, 'findItemLocation').resolves({
          found: false,
          locations: [],
          summary: 'Item not found',
        });

        result = await service.findItem('nonexistent');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should return found false', () => {
        expect(result.found).to.be.false;
      });

      it('should return empty locations array', () => {
        expect(result.locations).to.have.length(0);
      });
    });

    describe('when client throws', () => {
      let result: Awaited<ReturnType<typeof service.findItem>>;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { client: { findItemLocation: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.client, 'findItemLocation').rejects(new Error('Network error'));

        result = await service.findItem('screwdriver');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should return an error message', () => {
        expect(result.error).to.exist;
      });
    });
  });

  describe('showSuccess', () => {
    it('should have showSuccess method', () => {
      expect(service.showSuccess).to.be.a('function');
    });
  });

  describe('showError', () => {
    it('should have showError method', () => {
      expect(service.showError).to.be.a('function');
    });
  });

  describe('singleton export', () => {
    it('should export a singleton instance', () => {
      expect(inventoryActionService).to.be.instanceOf(InventoryActionService);
    });
  });
});
