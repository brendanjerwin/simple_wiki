package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/url"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1mcp"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	cli "gopkg.in/urfave/cli.v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
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

// runMCPServer starts the stdio MCP server with channel capability and maintains
// a streaming subscription to the wiki's ChatService.
func runMCPServer(baseURL string) (retErr error) {
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

	// Create gRPC client connection
	conn, err := createGRPCConn(baseURL)
	if err != nil {
		return fmt.Errorf("failed to create gRPC connection: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil && retErr == nil {
			retErr = fmt.Errorf("failed to close gRPC connection: %w", err)
		}
	}()

	// Create gRPC clients and register MCP tool handlers
	clients := createAPIClients(conn)
	registerToolHandlers(s, clients)

	// Start subscription to user messages in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go maintainChatSubscription(ctx, s, clients.chat)

	// Start stdio MCP server (blocks until stdin closes)
	return mcpserver.ServeStdio(s)
}

// parseGRPCHost parses a wiki base URL and returns the gRPC target host:port and URL scheme.
// For https:// URLs without an explicit port, :443 is appended.
// For http:// URLs without an explicit port, :80 is appended.
// Returns an error for invalid URLs or unsupported schemes.
func parseGRPCHost(baseURL string) (host, scheme string, err error) {
	u, parseErr := url.Parse(baseURL)
	if parseErr != nil {
		return "", "", fmt.Errorf("invalid base URL: %w", parseErr)
	}

	host = u.Host
	switch u.Scheme {
	case "https":
		if u.Port() == "" {
			host += ":443"
		}
	case "http":
		if u.Port() == "" {
			host += ":80"
		}
	default:
		return "", "", fmt.Errorf("unsupported URL scheme %q: must be http or https", u.Scheme)
	}

	return host, u.Scheme, nil
}

// createGRPCConn creates a gRPC connection to the wiki server.
// For https:// URLs, TLS credentials are used. For http:// URLs, insecure credentials are used.
func createGRPCConn(baseURL string) (*grpc.ClientConn, error) {
	host, scheme, err := parseGRPCHost(baseURL)
	if err != nil {
		return nil, err
	}

	var transportCreds credentials.TransportCredentials
	switch scheme {
	case "https":
		transportCreds = credentials.NewClientTLSFromCert(nil, "")
	default: // "http" — scheme already validated by parseGRPCHost
		transportCreds = insecure.NewCredentials()
	}

	return grpc.NewClient(host, grpc.WithTransportCredentials(transportCreds))
}

// apiClients holds gRPC clients for all wiki services.
type apiClients struct {
	chat           apiv1.ChatServiceClient
	frontmatter    apiv1.FrontmatterClient
	inventory      apiv1.InventoryManagementServiceClient
	pageImport     apiv1.PageImportServiceClient
	pageManagement apiv1.PageManagementServiceClient
	search         apiv1.SearchServiceClient
	systemInfo     apiv1.SystemInfoServiceClient
}

// createAPIClients creates gRPC clients for all wiki services.
func createAPIClients(conn *grpc.ClientConn) *apiClients {
	return &apiClients{
		chat:           apiv1.NewChatServiceClient(conn),
		frontmatter:    apiv1.NewFrontmatterClient(conn),
		inventory:      apiv1.NewInventoryManagementServiceClient(conn),
		pageImport:     apiv1.NewPageImportServiceClient(conn),
		pageManagement: apiv1.NewPageManagementServiceClient(conn),
		search:         apiv1.NewSearchServiceClient(conn),
		systemInfo:     apiv1.NewSystemInfoServiceClient(conn),
	}
}

// registerToolHandlers registers all wiki API tools as MCP handlers.
func registerToolHandlers(s *mcpserver.MCPServer, clients *apiClients) {
	// Register handlers for each service
	// These forward MCP tool calls to the gRPC services
	apiv1mcp.ForwardToChatServiceClient(s, clients.chat)
	apiv1mcp.ForwardToFrontmatterClient(s, clients.frontmatter)
	apiv1mcp.ForwardToInventoryManagementServiceClient(s, clients.inventory)
	apiv1mcp.ForwardToPageImportServiceClient(s, clients.pageImport)
	apiv1mcp.ForwardToPageManagementServiceClient(s, clients.pageManagement)
	apiv1mcp.ForwardToSearchServiceClient(s, clients.search)
	apiv1mcp.ForwardToSystemInfoServiceClient(s, clients.systemInfo)
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

// maintainChatSubscription maintains a streaming gRPC subscription to SubscribeChatMessages.
// It automatically reconnects with exponential backoff if the stream fails.
// After a healthy long-running stream drops, the backoff resets to initialBackoffMs so
// the next reconnect is fast. Rapid consecutive failures accumulate exponential backoff.
func maintainChatSubscription(ctx context.Context, s *mcpserver.MCPServer, client apiv1.ChatServiceClient) {
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

// subscribeToChatMessages establishes a streaming subscription to user messages
// and emits them as Claude Code channel notifications.
func subscribeToChatMessages(ctx context.Context, s *mcpserver.MCPServer, client apiv1.ChatServiceClient) error {
	// Subscribe to chat messages (server streaming)
	stream, err := client.SubscribeChatMessages(ctx, &apiv1.SubscribeChatMessagesRequest{})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Log successful connection
	log.Println("Chat subscription established")

	// Stream messages and emit as channel notifications
	for {
		msg, err := stream.Recv()
		if err != nil {
			// Context cancellation is not an error
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("stream error: %w", err)
		}

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
}

// channelInstructions are the server instructions passed to Claude Code.
const channelInstructions = `Messages arrive as <channel> tags with page, sender, and message_id attributes. You have three tools for responding:
- api_v1_ChatService_SendChatReply: Send a new message. Use reply_to_id to thread a response to a specific message.
- api_v1_ChatService_EditChatMessage: Edit one of your previous messages by ID.
- api_v1_ChatService_ReactToMessage: Add an emoji reaction to any message by ID.

Use wiki MCP tools to read/edit pages. Keep responses conversational and concise.`
