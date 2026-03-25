package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"time"

	"connectrpc.com/connect"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1connect"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1mcp"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	cli "gopkg.in/urfave/cli.v1"
)

const (
	initialBackoffMs  = 1000
	maxBackoffMs      = 30000
	backoffMultiplier = 2.0
)

// buildMCPCommand creates the `mcp` subcommand that runs a stdio MCP server with channel capability.
func buildMCPCommand(urlFlag cli.StringFlag) cli.Command {
	return cli.Command{
		Name:  "mcp",
		Usage: "Run a stdio MCP server with Claude Code channel capability",
		Description: `Starts a Model Context Protocol (MCP) server that communicates via stdio.
This server exposes all wiki API tools and implements the Claude Code channel protocol
for bidirectional chat integration.

The server subscribes to user messages from the wiki and emits them as channel notifications.
Claude can respond using the reply, edit_message, and react tools.

Example:
  wiki-cli mcp --url https://wiki.monster-orfe.ts.net

This command is designed to be spawned as a subprocess by Claude Code via:
  claude --channels wiki-channel:wiki-channel
`,
		Flags: []cli.Flag{urlFlag},
		Action: func(c *cli.Context) error {
			baseURL := c.String("url")
			return runMCPServer(baseURL)
		},
	}
}

// setupMCPServer creates the MCP server with channel capability and establishes
// the HTTP client for Connect protocol. The caller is responsible for managing the httpClient.
func setupMCPServer(baseURL string) (*mcpserver.MCPServer, *http.Client, error) {
	// Add hook to inject claude/channel experimental capability
	hooks := &mcpserver.Hooks{
		OnAfterInitialize: []mcpserver.OnAfterInitializeFunc{
			func(_ context.Context, _ any, _ *mcp.InitializeRequest, result *mcp.InitializeResult) {
				if result.Capabilities.Experimental == nil {
					result.Capabilities.Experimental = make(map[string]any)
				}
				result.Capabilities.Experimental["claude/channel"] = true
			},
		},
	}

	// Create MCP server with instructions and hooks
	s := mcpserver.NewMCPServer(
		"simple-wiki",
		version,
		mcpserver.WithInstructions(channelInstructions),
		mcpserver.WithHooks(hooks),
		mcpserver.WithToolCapabilities(false),
	)

	// Create HTTP client for Connect protocol
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	return s, httpClient, nil
}

// runMCPServer starts the stdio MCP server with channel capability and maintains
// a streaming subscription to the wiki's ChatService.
func runMCPServer(baseURL string) error {
	s, httpClient, err := setupMCPServer(baseURL)
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
	registerToolHandlers(s, clients)

	// Start subscription to user messages in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go maintainChatSubscription(ctx, s, clients.chat)

	// Start stdio MCP server (blocks until stdin closes)
	return mcpserver.ServeStdio(s)
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

	// Ensure no trailing slash for Connect protocol
	return baseURL, nil
}

// apiClients holds Connect protocol clients for all wiki services.
type apiClients struct {
	chat           apiv1connect.ChatServiceClient
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
		chat:           apiv1connect.NewChatServiceClient(httpClient, baseURL),
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
	apiv1mcp.ForwardToConnectChatServiceClient(s, clients.chat)
	apiv1mcp.ForwardToConnectFrontmatterClient(s, clients.frontmatter)
	apiv1mcp.ForwardToConnectInventoryManagementServiceClient(s, clients.inventory)
	apiv1mcp.ForwardToConnectPageImportServiceClient(s, clients.pageImport)
	apiv1mcp.ForwardToConnectPageManagementServiceClient(s, clients.pageManagement)
	apiv1mcp.ForwardToConnectSearchServiceClient(s, clients.search)
	apiv1mcp.ForwardToConnectSystemInfoServiceClient(s, clients.systemInfo)
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

// maintainChatSubscription maintains a streaming Connect protocol subscription to SubscribeChatMessages.
// It automatically reconnects with exponential backoff if the stream fails.
// After a healthy long-running stream drops, the backoff resets to initialBackoffMs so
// the next reconnect is fast. Rapid consecutive failures accumulate exponential backoff.
func maintainChatSubscription(ctx context.Context, s *mcpserver.MCPServer, client apiv1connect.ChatServiceClient) {
	backoffMs := initialBackoffMs

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		start := time.Now()

		// Try to subscribe
		err := subscribeToChatMessages(ctx, s, client)
		if err == nil {
			// Clean disconnect (context cancelled)
			return
		}

		delayMs, nextMs := computeBackoffAfterFailure(backoffMs, time.Since(start))

		// Log error and reconnect with backoff
		log.Printf("Chat subscription error: %v. Reconnecting in %dms...", err, delayMs)

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(delayMs) * time.Millisecond):
		}

		backoffMs = nextMs
	}
}

// subscribeToChatMessages establishes a streaming subscription to user messages using Connect protocol
// and emits them as Claude Code channel notifications.
func subscribeToChatMessages(ctx context.Context, s *mcpserver.MCPServer, client apiv1connect.ChatServiceClient) error {
	// Subscribe to chat messages (server streaming) using Connect protocol
	stream, err := client.SubscribeChatMessages(ctx, connect.NewRequest(&apiv1.SubscribeChatMessagesRequest{}))
	if err != nil {
		// Context cancellation is not an error
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Log successful connection
	log.Println("Chat subscription established")

	// Stream messages and emit as channel notifications
	for stream.Receive() {
		msg := stream.Msg()

		// Only emit user messages (assistant messages come from Claude itself)
		if msg.Sender != apiv1.Sender_USER {
			continue
		}

		// Emit channel notification to all clients
		// The notification params include the message content and metadata
		s.SendNotificationToAllClients(
			"notifications/claude/channel",
			map[string]any{
				"content": msg.Content,
				"meta": map[string]any{
					"page":       msg.Page,
					"sender":     "user",
					"message_id": msg.Id,
				},
			},
		)
	}

	// Check for stream error
	if err := stream.Err(); err != nil {
		// Context cancellation is not an error
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("stream error: %w", err)
	}

	return nil
}

// channelInstructions are the server instructions passed to Claude Code.
const channelInstructions = `Messages arrive as <channel> tags with page, sender, and message_id attributes. You have three tools for responding:
- api_v1_ChatService_SendChatReply: Send a new message. Use reply_to_id to thread a response to a specific message.
- api_v1_ChatService_EditChatMessage: Edit one of your previous messages by ID.
- api_v1_ChatService_ReactToMessage: Add an emoji reaction to any message by ID.

Use wiki MCP tools to read/edit pages. Keep responses conversational and concise.`
