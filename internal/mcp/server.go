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

	// Use the standard handler for Frontmatter tools. The standard schema advertises
	// google.protobuf.Struct fields (e.g. frontmatter) as "type": "object", so LLMs send
	// them as JSON objects directly. protojson.Unmarshal handles Struct fields natively
	// without any intermediate string-encoding step.
	//
	// Previously the OpenAI-compatible handler was used, which told clients to send
	// frontmatter as a JSON-encoded string and relied on FixOpenAI to decode it back.
	// However, for complex frontmatter (e.g. ai_agent_chat_context with deeply nested
	// content and special characters), the LLM sometimes produced improperly escaped JSON.
	// FixOpenAI silently ignored json.Unmarshal failures, leaving the string as-is and
	// causing protojson to fail with: "proto: syntax error (line 1:16): unexpected token".
	apiv1mcp.RegisterFrontmatterHandler(s, apiServer)
	apiv1mcp.RegisterInventoryManagementServiceHandler(s, apiServer)
	apiv1mcp.RegisterPageImportServiceHandler(s, apiServer)
	apiv1mcp.RegisterPageManagementServiceHandler(s, apiServer)
	apiv1mcp.RegisterSearchServiceHandler(s, apiServer)
	apiv1mcp.RegisterSystemInfoServiceHandler(s, apiServer)

	return mcpserver.NewStreamableHTTPServer(s), nil
}
