# ADR 0003: Frontend gRPC-web Integration and Version Display Component

## Status

Accepted

## Context

We need to establish a reliable pattern for frontend components to communicate with our gRPC services via gRPC-web. The first implementation is a version display component that fetches server version information to demonstrate the gRPC-web integration working end-to-end.

The existing infrastructure already supports gRPC-web via the `improbable-eng/grpc-web` library, but we need to decide on the frontend implementation approach for consuming these services.

## Decision

We will implement a simplified gRPC-web client pattern for frontend components, focusing on practical usability over strict protocol adherence.

### 1. Frontend gRPC-web Client Architecture

**Decision**: Use a simplified fetch-based gRPC-web implementation rather than the generated client libraries.

**Rationale**:
- The generated gRPC-web client libraries use CommonJS, which creates complexity in our ES module-based frontend build system
- A simplified implementation provides better control over error handling and debugging
- Direct fetch implementation allows for better error handling and debugging
- Testing is simpler with direct fetch mocking

**Implementation**:
- Create a custom `makeGrpcWebRequest()` method that handles gRPC-web protocol headers
- Use fetch API with appropriate `Content-Type: application/grpc-web+proto` headers
- Display clear error messages when requests fail
- Component remains blank when gRPC services are unavailable

### 2. Version Display Component

**Decision**: Create a `<version-display>` web component that demonstrates gRPC-web integration.

**Features**:
- Positioned as a semi-transparent panel in bottom-right corner
- Fetches version, commit hash, and build time from the `Version` gRPC service
- Shows loading states and error handling
- Remains blank when gRPC requests fail
- Styled with monospace font for developer-friendly display

**Styling Approach**:
- Fixed positioning (`position: fixed; bottom: 20px; right: 20px`)
- Semi-transparent background (`rgba(0, 0, 0, 0.7)`)
- Backdrop blur effect for modern appearance
- High z-index (1000) to ensure visibility above other content
- Responsive design that works on various screen sizes

### 3. Frontend Build Integration

**Decision**: Create a unified build process that includes all web components.

**Implementation**:
- Add `main.js` entry point that imports all web components
- Update build script to bundle from single entry point
- Add version-display component to main template (`index.tmpl`)
- Ensure component is always visible across all pages

### 4. Testing Strategy

**Decision**: Implement comprehensive unit tests with high coverage.

**Approach**:
- Mock fetch API for testing gRPC-web requests
- Test all component states: loading, success, error
- Verify styling and positioning requirements
- Test gRPC-web message encoding/decoding logic
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
- Clean separation of concerns between gRPC-web integration and UI components
- Simplified testing and debugging compared to generated client libraries
- Clear error handling prevents misleading user experience
- Pattern can be easily replicated for other gRPC services
- High test coverage ensures reliability

### Negative
- Custom gRPC-web implementation may need updates if protocol changes
- Requires proper protobuf parsing implementation for production use
- Limited functionality for complex message types without proper protobuf support

## Future Considerations

1. **Enhanced Protobuf Support**: Consider implementing proper protobuf parsing or using alternative code generation tools (protoc-gen-js, protoc-gen-ts, or other generators) for production use
2. **Performance Optimization**: Monitor gRPC-web request performance and optimize as needed
3. **Error Reporting**: Consider adding error reporting/telemetry for production debugging
4. **Component Library**: Expand pattern to create reusable gRPC-web utility functions
5. **Authentication**: Add authentication headers to gRPC-web requests as needed