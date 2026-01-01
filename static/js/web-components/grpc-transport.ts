import { createGrpcWebTransport } from '@connectrpc/connect-web';
import type { Transport } from '@connectrpc/connect';

/**
 * Shared gRPC Web Transport configuration
 * This provides a consistent transport instance across all components
 */
let sharedTransport: Transport | null = null;

export function getGrpcWebTransport(): Transport {
  if (!sharedTransport) {
    sharedTransport = createGrpcWebTransport({
      baseUrl: window.location.origin,
    });
  }
  return sharedTransport;
}