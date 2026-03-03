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
func NewStreamableHTTPHandler(apiServer *grpcapi.Server) (http.Handler, error) {
	s := mcpserver.NewMCPServer(
		"simple-wiki",
		"1.0.0",
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
