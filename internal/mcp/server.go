// Package mcp provides MCP (Model Context Protocol) server integration for simple_wiki.
// It exposes the gRPC API as MCP tools using the Streamable HTTP transport.
package mcp

import (
	"encoding/json"
	"net/http"

	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1mcp"
	grpcapi "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/mcpdocs"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// NewStreamableHTTPHandler creates an MCP Streamable HTTP handler that wires MCP tool
// invocations directly to the gRPC API server in-process. The returned
// ServiceDescription slice is the curated per-service catalog (sourced from
// (api.v1.service_description) proto extensions) for publishing at a
// discoverable URL; see NewServiceCatalogHandler.
//
// KNOWN LIMITATION: MCP calls bypass gRPC interceptors (identity resolution, logging,
// and observability). This means MCP callers have no user identity injected into context,
// MCP calls are not visible in gRPC request logs, and are not counted in request metrics.
// When the MCP server runtime adds middleware support, these should be added.
func NewStreamableHTTPHandler(apiServer *grpcapi.Server, version string) (http.Handler, []mcpdocs.ServiceDescription, error) {
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
	apiv1mcp.RegisterAgentMetadataServiceHandler(s, apiServer)
	apiv1mcp.RegisterChecklistServiceHandler(s, apiServer)
	apiv1mcp.RegisterChatServiceHandler(s, apiServer)
	apiv1mcp.RegisterConnectorServiceHandler(s, apiServer)
	apiv1mcp.RegisterFileStorageServiceHandler(s, apiServer)
	apiv1mcp.RegisterInventoryManagementServiceHandler(s, apiServer)
	apiv1mcp.RegisterMapServiceHandler(s, apiServer)
	apiv1mcp.RegisterPageHistoryServiceHandler(s, apiServer)
	apiv1mcp.RegisterPageImportServiceHandler(s, apiServer)
	apiv1mcp.RegisterPageManagementServiceHandler(s, apiServer)
	apiv1mcp.RegisterSearchServiceHandler(s, apiServer)
	apiv1mcp.RegisterSurveyServiceHandler(s, apiServer)
	apiv1mcp.RegisterSystemInfoServiceHandler(s, apiServer)

	// Override descriptions and annotations for any service whose proto
	// methods declare the api.v1.* MCP doc extensions, and remove any tool
	// whose RPC declares (api.v1.exclude_from_mcp). Mirrors the wiki-cli
	// stdio path so both MCP surfaces advertise identical, curated tool
	// descriptions. Must run AFTER the Register*Handler calls above.
	serviceDescriptions := mcpdocs.Decorate(s)

	return mcpserver.NewStreamableHTTPServer(s), serviceDescriptions, nil
}

// NewServiceCatalogHandler returns an HTTP handler that serves the curated
// MCP service catalog as JSON. The slice is produced by NewStreamableHTTPHandler
// from the (api.v1.service_description) proto extensions. Mount it at a
// discoverable URL (e.g. /mcp/catalog) so agents and tooling can enumerate
// the wiki's MCP services and their use-cases without negotiating a full
// MCP session.
func NewServiceCatalogHandler(descriptions []mcpdocs.ServiceDescription) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(descriptions); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}
