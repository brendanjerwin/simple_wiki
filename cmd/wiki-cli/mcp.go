package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1connect"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1mcp"
	"github.com/brendanjerwin/simple_wiki/pkg/mcpdocs"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	"google.golang.org/protobuf/reflect/protoreflect"
	cli "gopkg.in/urfave/cli.v1"
)

const (
	initialBackoffMs  = 1000
	maxBackoffMs      = 30000
	backoffMultiplier = 2.0
)

// buildMCPCommand creates the `mcp` subcommand that runs a stdio MCP server with wiki API tools.
func buildMCPCommand(urlFlag cli.StringFlag) cli.Command {
	return cli.Command{
		Name:  "mcp",
		Usage: "Run a stdio MCP server with wiki API tools",
		Description: `Starts a Model Context Protocol (MCP) server that communicates via stdio.
This server exposes all wiki API tools for use by Claude Code.

Example:
  wiki-cli mcp --url https://wiki.monster-orfe.ts.net
`,
		Flags: []cli.Flag{
			urlFlag,
		},
		Action: func(c *cli.Context) error {
			baseURL := c.String("url")
			return runMCPServer(baseURL)
		},
	}
}

// setupMCPServer creates the MCP server and establishes the HTTP client for Connect protocol.
// The caller is responsible for managing the httpClient.
func setupMCPServer(_ string) (*mcpserver.MCPServer, *http.Client, error) {
	// Create MCP server
	s := mcpserver.NewMCPServer(
		"simple-wiki",
		version,
		mcpserver.WithToolCapabilities(false),
	)

	// Create HTTP client for Connect protocol. Wrap with the agent-header
	// transport so all wiki-cli MCP calls carry x-wiki-is-agent: true by
	// default (suppressed only when WIKI_CLI_HUMAN=1).
	httpClient := newAgentAwareHTTPClient(nil)

	return s, httpClient, nil
}

// runMCPServer starts the stdio MCP server with wiki API tools.
func runMCPServer(baseURL string) error {
	mcpServer, httpClient, err := setupMCPServer(baseURL)
	if err != nil {
		return err
	}

	// Validate and normalize base URL
	normalizedURL, err := normalizeBaseURL(baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	// Create Connect clients and register MCP tool handlers
	clients := createAPIClients(httpClient, normalizedURL)
	registerToolHandlers(mcpServer, clients)

	// Redirect Go's default logger to stderr explicitly (it already defaults
	// to stderr, but being explicit prevents future surprises with stdio MCP).
	log.SetOutput(os.Stderr)

	// Start stdio MCP server (blocks until stdin closes)
	return mcpserver.ServeStdio(mcpServer)
}

// normalizeBaseURL validates and normalizes a wiki base URL for Connect protocol.
// Returns an error for invalid URLs or unsupported schemes.
func normalizeBaseURL(baseURL string) (string, error) {
	u, parseErr := url.Parse(baseURL)
	if parseErr != nil {
		return "", fmt.Errorf("invalid base URL: %w", parseErr)
	}

	switch u.Scheme {
	case "https", "http":
		// Valid schemes
	default:
		return "", fmt.Errorf("unsupported URL scheme %q: must be http or https", u.Scheme)
	}

	if u.Host == "" {
		return "", fmt.Errorf("invalid base URL: missing host in %q", baseURL)
	}

	// Trim trailing slashes for Connect protocol compatibility
	return strings.TrimRight(u.String(), "/"), nil
}

// apiClients holds Connect protocol clients for all wiki services.
type apiClients struct {
	agentMetadata  apiv1connect.AgentMetadataServiceClient
	chat           apiv1connect.ChatServiceClient
	checklist      apiv1connect.ChecklistServiceClient
	frontmatter    apiv1connect.FrontmatterClient
	inventory      apiv1connect.InventoryManagementServiceClient
	pageImport     apiv1connect.PageImportServiceClient
	pageManagement apiv1connect.PageManagementServiceClient
	search         apiv1connect.SearchServiceClient
	systemInfo     apiv1connect.SystemInfoServiceClient
}

// createAPIClients creates Connect protocol clients for all wiki services.
func createAPIClients(httpClient connect.HTTPClient, baseURL string) *apiClients {
	return &apiClients{
		agentMetadata:  apiv1connect.NewAgentMetadataServiceClient(httpClient, baseURL),
		chat:           apiv1connect.NewChatServiceClient(httpClient, baseURL),
		checklist:      apiv1connect.NewChecklistServiceClient(httpClient, baseURL),
		frontmatter:    apiv1connect.NewFrontmatterClient(httpClient, baseURL),
		inventory:      apiv1connect.NewInventoryManagementServiceClient(httpClient, baseURL),
		pageImport:     apiv1connect.NewPageImportServiceClient(httpClient, baseURL),
		pageManagement: apiv1connect.NewPageManagementServiceClient(httpClient, baseURL),
		search:         apiv1connect.NewSearchServiceClient(httpClient, baseURL),
		systemInfo:     apiv1connect.NewSystemInfoServiceClient(httpClient, baseURL),
	}
}

// registerToolHandlers registers all wiki API tools as MCP handlers using Connect protocol.
func registerToolHandlers(s *mcpserver.MCPServer, clients *apiClients) {
	// Register handlers for each service
	// These forward MCP tool calls to the Connect protocol services
	apiv1mcp.ForwardToConnectAgentMetadataServiceClient(s, clients.agentMetadata)
	apiv1mcp.ForwardToConnectChatServiceClient(s, clients.chat)
	apiv1mcp.ForwardToConnectChecklistServiceClient(s, clients.checklist)
	apiv1mcp.ForwardToConnectFrontmatterClient(s, clients.frontmatter)
	apiv1mcp.ForwardToConnectInventoryManagementServiceClient(s, clients.inventory)
	apiv1mcp.ForwardToConnectPageImportServiceClient(s, clients.pageImport)
	apiv1mcp.ForwardToConnectPageManagementServiceClient(s, clients.pageManagement)
	apiv1mcp.ForwardToConnectSearchServiceClient(s, clients.search)
	apiv1mcp.ForwardToConnectSystemInfoServiceClient(s, clients.systemInfo)

	// Replace input schemas that Anthropic's API rejects (top-level
	// anyOf/oneOf/allOf, emitted by the generator for any request message
	// containing a proto `oneof` block) with the *ToolOpenAI variants the
	// generator also produces. Those variants use a flat nullable-string
	// representation Anthropic accepts. See #1054.
	swapBrokenSchemasForAnthropic(s)

	// Override descriptions and annotations for any service whose proto
	// methods declare the api.v1.* MCP doc extensions. Services that haven't
	// been ported still surface their proto comments unchanged.
	_ = mcpdocs.Decorate(s)
}

// anthropicSchemaOverride pairs an OpenAI-variant input schema with the
// proto request descriptor it was generated for. Both pieces are needed at
// swap time: the schema overrides the registered tool's RawInputSchema, and
// the descriptor lets the wrapped handler run runtime.FixOpenAI to translate
// the OpenAI-flavored argument map back to standard protojson before
// delegating to the original handler.
type anthropicSchemaOverride struct {
	schema     json.RawMessage
	descriptor protoreflect.MessageDescriptor
}

// anthropicCompatibleSchemas returns the OpenAI-variant input schemas for
// tools whose default (non-OpenAI) generated schema is rejected by
// Anthropic's API, paired with the proto request descriptor for each tool.
//
// The OpenAI schema represents:
//   - oneof scalars as `type: ["string","null"]`
//   - proto maps as arrays of {key,value} objects
//   - google.protobuf.Struct / Value / ListValue as JSON-encoded strings
//
// runtime.FixOpenAI (called by the wrapped handler) reverses the second and
// third transformations so protojson can decode the result.
//
// Tools currently affected (request message contains a proto `oneof`):
//   - api_v1_PageManagementService_ReadPage (ReadPageRequest.page_identifier)
//   - api_v1_Frontmatter_RemoveKeyAtPath    (PathComponent.component, nested
//     in RemoveKeyAtPathRequest.key_path)
//
// If a new tool starts emitting top-level anyOf/oneOf/allOf the regression
// test in mcp_anthropic_schema_test.go will fail and surface that the map
// here needs another entry.
func anthropicCompatibleSchemas() map[string]anthropicSchemaOverride {
	return map[string]anthropicSchemaOverride{
		apiv1mcp.PageManagementService_ReadPageToolOpenAI.Name: {
			schema:     readPageAnthropicInputSchema,
			descriptor: (&apiv1.ReadPageRequest{}).ProtoReflect().Descriptor(),
		},
		apiv1mcp.Frontmatter_RemoveKeyAtPathToolOpenAI.Name: {
			schema:     apiv1mcp.Frontmatter_RemoveKeyAtPathToolOpenAI.RawInputSchema,
			descriptor: (&apiv1.RemoveKeyAtPathRequest{}).ProtoReflect().Descriptor(),
		},
	}
}

var readPageAnthropicInputSchema = json.RawMessage(`{
  "additionalProperties": false,
  "minProperties": 1,
  "properties": {
    "identifier": {
      "description": "Page identifier. Set either identifier or page_name, not both.",
      "type": "string"
    },
    "page_name": {
      "description": "Page name. Set either page_name or identifier, not both.",
      "type": "string"
    }
  },
  "required": [],
  "type": "object"
}`)

// wrapHandlerForOpenAISchema wraps an existing tool handler so that it
// normalizes incoming arguments using runtime.FixOpenAI before delegating.
// This matches the behavior of the generator-emitted *HandlerOpenAI variants,
// which run FixOpenAI between request.GetArguments() and protojson.Unmarshal.
//
// Without this normalization the OpenAI-variant schema and the default
// (non-OpenAI) handler are mismatched: the schema instructs the model to
// represent google.protobuf.Struct fields as JSON strings and proto maps as
// arrays of {key,value} objects, but the default handler's protojson decode
// expects the standard wire format. FixOpenAI converts the OpenAI shape back
// to the standard shape in-place on the args map so the original handler
// then sees a payload protojson can decode.
//
// For arguments whose type is plain scalars/oneofs (the two tools currently
// in the override map: ReadPageRequest and RemoveKeyAtPathRequest),
// FixOpenAI is a no-op and the wrapper is transparent. The wrapping is
// nonetheless applied so that any future override entry covering a request
// with a Struct/Value/Map field is correctly handled without further
// changes here.
func wrapHandlerForOpenAISchema(originalHandler mcpserver.ToolHandlerFunc, requestDescriptor protoreflect.MessageDescriptor) mcpserver.ToolHandlerFunc {
	if originalHandler == nil {
		return nil
	}
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// GetArguments returns the underlying map directly when Arguments
		// is already a map[string]any (the common path for stdio MCP).
		// Mutating it in place propagates the normalization to the inner
		// handler's view of the request.
		args := req.GetArguments()
		if args != nil {
			runtime.FixOpenAI(requestDescriptor, args)
		}
		return originalHandler(ctx, req)
	}
}

// swapBrokenSchemasForAnthropic rewrites the RawInputSchema of any
// already-registered tool listed in anthropicCompatibleSchemas and wraps the
// original handler with runtime.FixOpenAI normalization so the schema and
// handler stay a matched pair (mirroring the generator-emitted
// *HandlerOpenAI variants). It is a no-op for tools that are not in the
// override map.
func swapBrokenSchemasForAnthropic(s *mcpserver.MCPServer) {
	overrides := anthropicCompatibleSchemas()
	for name, override := range overrides {
		existing := s.GetTool(name)
		if existing == nil {
			// Forwarder did not register this tool (e.g. service not wired);
			// nothing to swap.
			continue
		}
		swapped := existing.Tool
		swapped.RawInputSchema = override.schema
		// Defensive: clear the parsed form so the new RawInputSchema is the
		// sole source of truth on serialization.
		swapped.InputSchema = mcp.ToolInputSchema{}
		wrapped := wrapHandlerForOpenAISchema(existing.Handler, override.descriptor)
		s.AddTool(swapped, wrapped)
	}
}

// computeBackoffAfterFailure returns the delay to wait before the next reconnect attempt
// and the backoff value to carry into the following iteration.
//
// If streamDuration exceeds initialBackoffMs the connection is considered healthy and
// the delay resets to initialBackoffMs (fast reconnect). Otherwise the current backoff
// is kept, accumulating exponential growth on rapid consecutive failures. In both cases
// the returned nextBackoffMs is the doubled value (capped at maxBackoffMs) ready for the
// iteration after the next failure.
func computeBackoffAfterFailure(currentBackoffMs int, streamDuration time.Duration) (delayMs, nextBackoffMs int) {
	delayMs = currentBackoffMs
	if streamDuration > time.Duration(initialBackoffMs)*time.Millisecond {
		delayMs = initialBackoffMs
	}
	nextBackoffMs = int(math.Min(float64(delayMs)*backoffMultiplier, maxBackoffMs))
	return delayMs, nextBackoffMs
}
