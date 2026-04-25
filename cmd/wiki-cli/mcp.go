package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1connect"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1mcp"
	"github.com/brendanjerwin/simple_wiki/pkg/mcpdocs"
	mcpserver "github.com/mark3labs/mcp-go/server"
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

	// Create HTTP client for Connect protocol.
	httpClient := &http.Client{}

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

	// Override descriptions and annotations for any service whose proto
	// methods declare the api.v1.* MCP doc extensions. Services that haven't
	// been ported still surface their proto comments unchanged.
	_ = mcpdocs.Decorate(s)
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
