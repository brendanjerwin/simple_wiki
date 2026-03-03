// Package mcp provides MCP (Model Context Protocol) server integration for simple_wiki.
// It exposes the gRPC API as MCP tools using the Streamable HTTP transport.
package mcp

import (
	"net/http"

	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1mcp"
	grpcapi "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// NewStreamableHTTPHandler creates an MCP Streamable HTTP handler that exposes
// all gRPC API services as MCP tools. The apiServer is called in-process with
// no network hop.
//
// Known limitation: MCP tool calls invoke the gRPC service methods directly on
// apiServer, bypassing the gRPC interceptor chain. This means gRPC-level logging
// (LoggingInterceptor) and gRPC observability metrics (GRPCInstrumentation) do not
// capture MCP traffic. Tailscale identity is injected by the HTTP-level wrapper in
// bootstrap (IdentityHTTPMiddlewareWithMetrics) so IdentityFromContext works correctly.
//
// The error return is reserved for future validation and is currently always nil.
func NewStreamableHTTPHandler(apiServer *grpcapi.Server, commit string) (http.Handler, error) {
	s := mcpserver.NewMCPServer(
		"simple-wiki",
		commit,
		mcpserver.WithToolCapabilities(false),
	)

	apiv1mcp.RegisterFrontmatterHandler(s, apiServer)
	apiv1mcp.RegisterInventoryManagementServiceHandler(s, apiServer)
	apiv1mcp.RegisterPageImportServiceHandler(s, apiServer)
	apiv1mcp.RegisterPageManagementServiceHandler(s, apiServer)
	apiv1mcp.RegisterSearchServiceHandler(s, apiServer)
	apiv1mcp.RegisterSystemInfoServiceHandler(s, apiServer)

	return mcpserver.NewStreamableHTTPServer(s), nil
}
