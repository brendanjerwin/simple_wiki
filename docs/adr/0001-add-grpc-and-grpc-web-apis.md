# ADR 0001: Add gRPC and gRPC-web APIs

## Status

Accepted

## Context

We need to add modern, strongly-typed APIs to the application. The primary goals are to support efficient service-to-service communication and to provide a high-performance API for our web frontend. gRPC is an excellent choice for this. To make gRPC APIs accessible from browsers, we will use gRPC-web.

Our existing backend is a Go application using the Gin web framework. We need a solution that integrates cleanly with our current stack. For our Protocol Buffer toolchain, we want a modern, reliable, and easy-to-use solution.

## Decision

We will adopt gRPC for our new APIs and integrate it into our existing Gin server. We will use `buf` as our primary toolchain for managing Protocol Buffers.

1. **API Definitions**: All APIs will be defined using Protocol Buffers in a new `api/proto/` directory.

2. **Toolchain and Code Generation**:

    - We will use `buf` to manage our Protobuf workflow. `buf` provides linting, breaking change detection, and simplified code generation against a defined configuration.
    - A `buf.gen.yaml` file will define our code generation strategy, producing:
      - Go server stubs (`*_grpc.pb.go`) and message types (`*.pb.go`).
      - JavaScript client stubs for gRPC-web.

3. **Backend Integration**:

    - We will implement our gRPC services in Go.
    - We will use the `improbable-eng/grpc-web` library to wrap our Go `grpc.Server` instance. This library provides an `http.Handler` that translates gRPC-web requests into standard gRPC.
    - This `http.Handler` will be registered within our Gin router on a dedicated path prefix (e.g., `/grpc`). This allows us to serve both gRPC-web and our existing REST/JSON APIs from the same server and port.

    Example of Gin integration:

    ```go
    //
    import (
      "github.com/gin-gonic/gin"
      "github.com/improbable-eng/grpc-web/go/grpcweb"
      "google.golang.org/grpc"
    )

    func main() {
      // ... assume grpcServer is a configured *grpc.Server
      // ... assume router is a configured *gin.Engine

      wrappedGrpc := grpcweb.WrapServer(grpcServer)

      router.Any("/grpc/*path", gin.WrapH(wrappedGrpc))

      // ...
    }
    ```

## Consequences

- **Pros**:
  - `buf` simplifies our Protobuf toolchain, provides consistency, and helps prevent breaking changes.
  - Single server and port for all API traffic, simplifying deployment and operations.
  - Clean integration with our existing Gin application.
  - Provides a clear path for migrating existing AJAX endpoints to gRPC-web over time.
  - Enables strongly-typed APIs, reducing bugs, and improving developer experience.
- **Cons**:
  - Adds new dependencies (`buf`, `improbable-eng/grpc-web`, and various `protoc` plugins).
  - Requires a build step (`buf generate`) to generate code from `.proto` files. Frontend development will require these generated assets.
