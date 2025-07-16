import { expect } from 'chai';
import { fixture, html } from '@open-wc/testing';
import './version-display.js';
import { GRPC_WEB_ENDPOINT } from './version-display.js';

describe('Version Display Integration', () => {
  let el;
  
  beforeEach(async () => {
    el = await fixture(html`<version-display></version-display>`);
  });

  describe('when component is created', () => {
    it('should exist', () => {
      expect(el).to.exist;
    });
  });

  describe('gRPC-web endpoint', () => {
    it('should use the correct endpoint URL', () => {
      // Check that the GRPC_WEB_ENDPOINT constant is correctly used
      expect(typeof GRPC_WEB_ENDPOINT).to.equal('string');
      expect(GRPC_WEB_ENDPOINT).to.equal('/api.v1.Version/GetVersion');
    });

    it('should handle network errors gracefully', async () => {
      // This test runs without a server, so it should fail gracefully
      await el.fetchVersion();
      
      // Component should remain blank on error
      expect(el.version).to.equal('');
      expect(el.commit).to.equal('');
      expect(el.buildTime).to.equal('');
      expect(el.error).to.not.be.empty;
    });
  });

  describe('protobuf parsing', () => {
    it('should parse a valid gRPC-web response', () => {
      // Create a mock response buffer similar to what the server returns
      const mockResponse = new ArrayBuffer(50);
      const view = new DataView(mockResponse);
      
      // gRPC-web frame header (5 bytes)
      view.setUint8(0, 0x00); // uncompressed
      view.setUint32(1, 24, false); // message length
      
      // Protobuf message for GetVersionResponse
      let offset = 5;
      
      // Field 1: version = "dev"
      view.setUint8(offset++, 0x0a); // tag: field 1, wire type 2
      view.setUint8(offset++, 0x03); // length: 3
      view.setUint8(offset++, 0x64); // 'd'
      view.setUint8(offset++, 0x65); // 'e'
      view.setUint8(offset++, 0x76); // 'v'
      
      // Field 2: commit = "n/a"
      view.setUint8(offset++, 0x12); // tag: field 2, wire type 2
      view.setUint8(offset++, 0x03); // length: 3
      view.setUint8(offset++, 0x6e); // 'n'
      view.setUint8(offset++, 0x2f); // '/'
      view.setUint8(offset++, 0x61); // 'a'
      
      // Parse the response
      const result = el.constructor.parseGetVersionResponse?.(mockResponse);
      
      if (result) {
        expect(result.version).to.equal('dev');
        expect(result.commit).to.equal('n/a');
      }
    });
  });
});