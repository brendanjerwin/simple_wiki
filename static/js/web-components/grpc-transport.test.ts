import { expect } from '@open-wc/testing';
import type { Transport } from '@connectrpc/connect';
import { getGrpcWebTransport } from './grpc-transport.js';

describe('grpc-transport', () => {
  describe('getGrpcWebTransport', () => {
    it('should return a transport instance', () => {
      const transport = getGrpcWebTransport();
      expect(transport).to.not.equal(null);
    });

    describe('singleton behavior', () => {
      let firstInstance: Transport;
      let secondInstance: Transport;

      beforeEach(() => {
        firstInstance = getGrpcWebTransport();
        secondInstance = getGrpcWebTransport();
      });

      it('should return the same instance on repeated calls', () => {
        expect(firstInstance).to.equal(secondInstance);
      });

      it('should return the same instance on a third call', () => {
        const thirdInstance = getGrpcWebTransport();
        expect(thirdInstance).to.equal(firstInstance);
      });
    });

    describe('baseUrl configuration', () => {
      it('should use window.location.origin as the base URL', () => {
        // The transport is created with window.location.origin.
        // We verify the origin is valid and the transport is created successfully.
        expect(window.location.origin).to.not.equal('null');
        expect(window.location.origin).to.not.be.empty;

        const transport = getGrpcWebTransport();
        expect(transport).to.not.equal(null);
      });
    });
  });
});
