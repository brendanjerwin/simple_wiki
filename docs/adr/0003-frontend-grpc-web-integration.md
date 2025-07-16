# ADR 0003: Frontend gRPC-web Integration and Version Display Component

## Status

Accepted

## Context

We need to establish a reliable pattern for frontend components to communicate with our gRPC services via gRPC-web. The first implementation is a version display component that fetches server version information to demonstrate the gRPC-web integration working end-to-end.

The existing infrastructure already supports gRPC-web via the `improbable-eng/grpc-web` library, but we need to decide on the frontend implementation approach for consuming these services.

## Decision

We will implement a modern gRPC-web client pattern using Connect-ES and @bufbuild/protobuf for ES module compatibility.

### 1. Frontend gRPC-web Client Architecture

**Decision**: Use a simple gRPC-web client implementation with manual protobuf parsing for better compatibility.

**Rationale**:
- Eliminates complex dependency management issues between Connect-ES and @bufbuild/protobuf versions
- Provides full control over the gRPC-web request format and response parsing
- Reduces bundle size by avoiding heavy client libraries
- Maintains compatibility with existing gRPC-web server infrastructure
- Clear, predictable error handling without library abstractions

**Implementation**:
- Direct fetch() calls to gRPC-web endpoints with proper headers
- Manual protobuf response parsing for GetVersionResponse
- Endpoint URL generation from proto definitions (using generated constants)
- Proper gRPC-web framing for request/response handling

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

**Decision**: Use existing build process with minimal dependencies.

**Implementation**:
- Use standard `lit` for web components
- Generate endpoint URLs from proto definitions
- Manual protobuf parsing to avoid dependency conflicts
- Integrate with existing npm/bun build pipeline

### 4. Testing Strategy

**Decision**: Implement comprehensive unit and integration tests.

**Approach**:
- Mock gRPC-web endpoints for unit testing
- Create integration tests that verify gRPC-web request format
- Test protobuf parsing with known response formats
- Verify error handling when endpoints are unavailable
- Test endpoint URL generation from proto definitions

### 5. Error Handling

**Decision**: Implement clear error handling with no fallback data.

**Behavior**:
- When gRPC request fails, component remains blank (no DOM output)
- Error messages are logged to console for debugging
- Component does not display misleading fallback data (aligns with CONVENTIONS.md principle)
- Clear indication when services are unavailable without fake data
- Respects "Never Hide Broken Functionality" design principle

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
- Migration required from legacy `google-protobuf` and `grpc-web` dependencies

## Future Considerations

1. **Service Expansion**: Apply this pattern to other gRPC services as needed
2. **Performance Optimization**: Monitor gRPC-web request performance and optimize as needed
3. **Error Reporting**: Consider adding error reporting/telemetry for production debugging
4. **Authentication**: Add authentication headers to gRPC-web requests as needed