# ADR 0003: Frontend gRPC-web Integration and Version Display Component

## Status

Accepted

## Context

We need to establish a reliable pattern for frontend components to communicate with our gRPC services via gRPC-web. The first implementation is a version display component that fetches server version information to demonstrate the gRPC-web integration working end-to-end.

The existing infrastructure already supports gRPC-web via the `improbable-eng/grpc-web` library, but we need to decide on the frontend implementation approach for consuming these services.

## Decision

We will implement a modern gRPC-web client pattern using Connect-ES and @bufbuild/protobuf for ES module compatibility.

### 1. Frontend gRPC-web Client Architecture

**Decision**: Use Connect-ES with @bufbuild/protobuf for modern ES module-based gRPC-web client implementation.

**Rationale**:
- Connect-ES provides native ES module support, eliminating CommonJS compatibility issues
- @bufbuild/protobuf offers modern TypeScript-first protobuf implementation
- Generated code is type-safe and conforms to protocol buffer contracts
- Better developer experience with modern tooling and error handling
- Established pattern for future service integrations

**Implementation**:
- Configure `buf.gen.yaml` with `buf.build/bufbuild/es` and `buf.build/connectrpc/es` plugins
- Generate ES module-compatible protobuf files
- Use `createGrpcWebTransport()` and `createClient()` from Connect-ES
- Implement proper type-safe request/response handling

### 2. Version Display Component

**Decision**: Create a `<version-display>` web component that demonstrates gRPC-web integration.

**Features**:
- Positioned as a low-profile, semi-transparent panel in bottom-right corner
- Fetches version, commit hash, and build time from the `Version` gRPC service
- Shows loading states and error handling
- Remains blank when gRPC requests fail
- Styled with monospace font for developer-friendly display

**Styling Approach**:
- Fixed positioning (`position: fixed; bottom: 5px; right: 5px`)
- Highly transparent background (`rgba(0, 0, 0, 0.2)`) with hover darkening
- Minimal footprint with single-row horizontal layout
- High z-index (1000) to ensure visibility above other content

### 3. Frontend Build Integration

**Decision**: Create a unified build process that includes all web components.

**Implementation**:
- Update `package.json` to use Connect-ES and @bufbuild/protobuf dependencies
- Generate ES module protobuf files using modern buf plugins
- Add version-display component to main template (`index.tmpl`)
- Ensure component is always visible across all pages

### 4. Testing Strategy

**Decision**: Implement comprehensive unit tests with high coverage.

**Approach**:
- Mock Connect client for testing gRPC-web requests
- Test all component states: loading, success, error
- Verify styling and positioning requirements
- Test type-safe protobuf message handling
- Aim for high code coverage on code that matters

### 5. Error Handling

**Decision**: Implement clear error handling with no fallback data.

**Behavior**:
- When gRPC request fails, component remains blank
- Error messages are logged to console for debugging
- Component does not display misleading fallback data
- Clear indication when services are unavailable

## Consequences

### Positive
- Native ES module support eliminates CommonJS compatibility issues
- Type-safe protobuf messages ensure contract compliance
- Modern tooling provides better developer experience
- Clear error handling prevents misleading user experience
- Pattern can be easily replicated for other gRPC services
- High test coverage ensures reliability

### Negative
- Requires updated build tooling and dependencies
- Generated files must be kept in sync with protocol changes
- Connect-ES is a newer technology with smaller ecosystem

## Future Considerations

1. **Service Expansion**: Apply this pattern to other gRPC services as needed
2. **Performance Optimization**: Monitor gRPC-web request performance and optimize as needed
3. **Error Reporting**: Consider adding error reporting/telemetry for production debugging
4. **Authentication**: Add authentication headers to gRPC-web requests as needed