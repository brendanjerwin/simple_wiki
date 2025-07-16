# 3. Adopt Connect-ES for Frontend gRPC Communication

Date: 2025-01-16

## Status

Accepted

## Context

The existing gRPC-Web implementation uses the legacy `buf.build/grpc/web` plugin and `google-protobuf` JavaScript library. This approach has several limitations:

1. Legacy protobuf JavaScript library with CommonJS-style requires
2. More complex client setup and configuration
3. Less modern TypeScript support
4. Larger bundle sizes due to legacy dependencies

The newer Connect-ES approach offers modern TypeScript-first gRPC client capabilities with better developer experience and smaller bundle sizes.

## Decision

We will adopt Connect-ES (`@connectrpc/connect` and `@connectrpc/connect-web`) as the primary frontend gRPC client technology, replacing the legacy gRPC-Web implementation.

The implementation includes:
- Using `buf.build/bufbuild/es` for TypeScript protobuf message generation
- Using `buf.build/connectrpc/es` for Connect-ES client generation
- Modern TypeScript clients with better tree-shaking and bundle optimization

## Consequences

### Positive
- Modern TypeScript-first API with better type safety
- Smaller bundle sizes through better tree-shaking
- Simplified client setup and configuration
- Better developer experience with modern tooling
- Future-proof approach aligned with Buf's current recommendations

### Negative
- Breaking change from existing gRPC-Web implementation
- Requires updates to build tooling and configuration
- New dependency on Connect-ES libraries

### Neutral
- Generated files structure changes from `.js` to `.ts`
- Client creation API changes but remains straightforward