// Package mcp provides MCP (Model Context Protocol) server integration for simple_wiki.
// It exposes the gRPC API as MCP tools using the Streamable HTTP transport.
package mcp

import (
	"net/http"

	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1mcp"
	grpcapi "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// NewStreamableHTTPHandler creates an MCP Streamable HTTP handler that wires MCP tool
// invocations directly to the gRPC API server in-process.
//
// KNOWN LIMITATION: MCP calls bypass gRPC interceptors (identity resolution, logging,
// and observability). This means MCP callers have no user identity injected into context,
// MCP calls are not visible in gRPC request logs, and are not counted in request metrics.
// When the MCP server runtime adds middleware support, these should be added.
func NewStreamableHTTPHandler(apiServer *grpcapi.Server, version string) (http.Handler, error) {
	s := mcpserver.NewMCPServer(
		"simple-wiki",
		version,
		mcpserver.WithToolCapabilities(false),
	)

	// Use the OpenAI-compatible handler for Frontmatter tools. This applies FixOpenAI to
	// convert google.protobuf.Struct fields from JSON-encoded strings back to objects before
	// proto parsing. Without this, clients sending frontmatter as a JSON string (a common
	// behaviour in OpenAI-compatible tool call mode) would receive a proto syntax error.
	// The handler also handles the regular object format, so both input styles work.
	apiv1mcp.RegisterFrontmatterHandlerOpenAI(s, apiServer)
	apiv1mcp.RegisterInventoryManagementServiceHandler(s, apiServer)
	apiv1mcp.RegisterPageImportServiceHandler(s, apiServer)
	apiv1mcp.RegisterPageManagementServiceHandler(s, apiServer)
	apiv1mcp.RegisterSearchServiceHandler(s, apiServer)
	apiv1mcp.RegisterSystemInfoServiceHandler(s, apiServer)

	return mcpserver.NewStreamableHTTPServer(s), nil
}
