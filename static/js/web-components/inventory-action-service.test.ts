import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import { InventoryActionService } from './inventory-action-service.js';

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
        const serviceWithClient = service as unknown as { inventoryClient: { createInventoryItem: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.inventoryClient, 'createInventoryItem').resolves({
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

    describe('when called with description', () => {
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { inventoryClient: { createInventoryItem: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.inventoryClient, 'createInventoryItem').resolves({
          success: true,
          itemIdentifier: 'screwdriver',
          summary: 'Created screwdriver',
        });

        await service.addItem('drawer_kitchen', 'screwdriver', 'Phillips Screwdriver', 'A yellow-handled Phillips head screwdriver');
      });

      it('should pass description to client', () => {
        const request = clientStub.firstCall.args[0];
        expect(request.description).to.equal('A yellow-handled Phillips head screwdriver');
      });
    });

    describe('when called with valid parameters and client returns error response', () => {
      let result: Awaited<ReturnType<typeof service.addItem>>;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { inventoryClient: { createInventoryItem: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.inventoryClient, 'createInventoryItem').resolves({
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
        const serviceWithClient = service as unknown as { inventoryClient: { createInventoryItem: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.inventoryClient, 'createInventoryItem').rejects(new Error('Network error'));

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

  describe('generateIdentifier', () => {
    describe('when called with empty text', () => {
      let result: Awaited<ReturnType<typeof service.generateIdentifier>>;

      beforeEach(async () => {
        result = await service.generateIdentifier('');
      });

      it('should return empty identifier', () => {
        expect(result.identifier).to.equal('');
      });

      it('should return isUnique true', () => {
        expect(result.isUnique).to.be.true;
      });
    });

    describe('when called with valid text and identifier is unique', () => {
      let result: Awaited<ReturnType<typeof service.generateIdentifier>>;
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { pageManagementClient: { generateIdentifier: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.pageManagementClient, 'generateIdentifier').resolves({
          identifier: 'phillips_screwdriver',
          isUnique: true,
          existingPage: undefined,
        });

        result = await service.generateIdentifier('Phillips Screwdriver');
      });

      it('should return the generated identifier', () => {
        expect(result.identifier).to.equal('phillips_screwdriver');
      });

      it('should return isUnique true', () => {
        expect(result.isUnique).to.be.true;
      });

      it('should not have existingPage', () => {
        expect(result.existingPage).to.be.undefined;
      });

      it('should call client with correct request', () => {
        expect(clientStub).to.have.been.calledOnce;
        const request = clientStub.firstCall.args[0];
        expect(request.text).to.equal('Phillips Screwdriver');
        expect(request.ensureUnique).to.be.false;
      });
    });

    describe('when called with valid text and identifier already exists', () => {
      let result: Awaited<ReturnType<typeof service.generateIdentifier>>;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { pageManagementClient: { generateIdentifier: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.pageManagementClient, 'generateIdentifier').resolves({
          identifier: 'screwdriver',
          isUnique: false,
          existingPage: {
            identifier: 'screwdriver',
            title: 'Screwdriver',
            container: 'toolbox_garage',
          },
        });

        result = await service.generateIdentifier('Screwdriver');
      });

      it('should return the generated identifier', () => {
        expect(result.identifier).to.equal('screwdriver');
      });

      it('should return isUnique false', () => {
        expect(result.isUnique).to.be.false;
      });

      it('should return existingPage with identifier', () => {
        expect(result.existingPage?.identifier).to.equal('screwdriver');
      });

      it('should return existingPage with title', () => {
        expect(result.existingPage?.title).to.equal('Screwdriver');
      });

      it('should return existingPage with container', () => {
        expect(result.existingPage?.container).to.equal('toolbox_garage');
      });
    });

    describe('when called with ensureUnique true', () => {
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { pageManagementClient: { generateIdentifier: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.pageManagementClient, 'generateIdentifier').resolves({
          identifier: 'screwdriver_1',
          isUnique: true,
          existingPage: undefined,
        });

        await service.generateIdentifier('Screwdriver', true);
      });

      it('should call client with ensureUnique true', () => {
        const request = clientStub.firstCall.args[0];
        expect(request.ensureUnique).to.be.true;
      });
    });

    describe('when client throws error', () => {
      let result: Awaited<ReturnType<typeof service.generateIdentifier>>;

      beforeEach(async () => {
        const serviceWithClient = service as unknown as { pageManagementClient: { generateIdentifier: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.pageManagementClient, 'generateIdentifier').rejects(new Error('Network error'));

        result = await service.generateIdentifier('Some Text');
      });

      it('should return empty identifier', () => {
        expect(result.identifier).to.equal('');
      });

      it('should return isUnique true', () => {
        expect(result.isUnique).to.be.true;
      });

      it('should return error message', () => {
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
        const serviceWithClient = service as unknown as { inventoryClient: { moveInventoryItem: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.inventoryClient, 'moveInventoryItem').resolves({
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
        const serviceWithClient = service as unknown as { inventoryClient: { moveInventoryItem: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.inventoryClient, 'moveInventoryItem').resolves({
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
        const serviceWithClient = service as unknown as { inventoryClient: { moveInventoryItem: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.inventoryClient, 'moveInventoryItem').rejects(new Error('Network error'));

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

});
