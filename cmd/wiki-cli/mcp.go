package main

import (
	"context"
	"encoding/json"
	"errors"
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

With page scoping (used by pool daemon):
  wiki-cli mcp --url https://wiki.monster-orfe.ts.net --page my_page

This command is designed to be spawned as a subprocess by Claude Code via:
  claude --channels wiki-channel:wiki-channel
`,
		Flags: []cli.Flag{
			urlFlag,
			cli.StringFlag{
				Name:  "page, p",
				Usage: "Scope this MCP instance to a specific wiki page",
			},
		},
		Action: func(c *cli.Context) error {
			baseURL := c.String("url")
			page := c.String("page")
			return runMCPServer(baseURL, page)
		},
	}
}

// setupMCPServer creates the MCP server with channel capability and establishes
// the HTTP client for Connect protocol. The caller is responsible for managing the httpClient.
// onInitialized is called after the MCP init handshake completes — use it to start
// work that may write to stdout (e.g., channel notifications) so it doesn't race
// with the init response.
func setupMCPServer(_ string, instructions string, onInitialized func()) (*mcpserver.MCPServer, *http.Client, error) {
	// Add hook to inject claude/channel experimental capability
	hooks := &mcpserver.Hooks{
		OnAfterInitialize: []mcpserver.OnAfterInitializeFunc{
			func(_ context.Context, _ any, _ *mcp.InitializeRequest, result *mcp.InitializeResult) {
				if result.Capabilities.Experimental == nil {
					result.Capabilities.Experimental = make(map[string]any)
				}
				result.Capabilities.Experimental["claude/channel"] = map[string]any{}

				if onInitialized != nil {
					onInitialized()
				}
			},
		},
	}

	// Create MCP server with instructions and hooks
	s := mcpserver.NewMCPServer(
		"simple-wiki",
		version,
		mcpserver.WithInstructions(instructions),
		mcpserver.WithHooks(hooks),
		mcpserver.WithToolCapabilities(false),
	)

	// Create HTTP client for Connect protocol. No global Timeout is set here so
	// long-lived streaming requests (e.g., chat subscriptions) are not forcibly
	// cancelled by the client. Per-request contexts handle deadlines instead.
	httpClient := &http.Client{}

	return s, httpClient, nil
}

// runMCPServer starts the stdio MCP server with channel capability and maintains
// a streaming subscription to the wiki's ChatService.
// If page is non-empty, the server is scoped to that page only and injects frontmatter context.
func runMCPServer(baseURL, page string) error {
	// Defer chat subscription until after MCP init handshake completes.
	// Starting it earlier would send channel notifications to stdout before
	// Claude Code has finished the init exchange, breaking the MCP protocol.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		mcpServer *mcpserver.MCPServer
		clients   *apiClients
	)
	onInitialized := func() {
		if page != "" {
			go injectPageContext(ctx, mcpServer, clients, page)
			go maintainPageChatSubscription(ctx, mcpServer, clients.chat, page)
		} else {
			go maintainChatSubscription(ctx, mcpServer, clients.chat)
		}
	}

	instructions := channelInstructions
	if page != "" {
		instructions = buildPageScopedInstructions(page)
	}

	var httpClient *http.Client
	var err error
	mcpServer, httpClient, err = setupMCPServer(baseURL, instructions, onInitialized)
	if err != nil {
		return err
	}

	// Validate and normalize base URL
	normalizedURL, err := normalizeBaseURL(baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	// Create Connect clients and register MCP tool handlers
	clients = createAPIClients(httpClient, normalizedURL)
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

// chatMessageReceiver is the minimal interface for reading from a Connect server-streaming response.
// It is satisfied by *connect.ServerStreamForClient[apiv1.ChatMessage].
type chatMessageReceiver interface {
	Receive() bool
	Msg() *apiv1.ChatMessage
	Err() error
	Close() error
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
	defer func() { _ = stream.Close() }()

	// Log successful connection
	log.Println("Chat subscription established")

	return receiveChatMessages(ctx, s, stream)
}

// receiveChatMessages reads messages from stream and emits USER messages as channel notifications.
// Returns nil only when the context is cancelled; returns a non-nil error otherwise (including
// on a clean server-side EOF) so that maintainChatSubscription can reconnect.
func receiveChatMessages(ctx context.Context, s *mcpserver.MCPServer, stream chatMessageReceiver) error {
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

	// Check for stream error (nil means clean EOF from server)
	if err := stream.Err(); err != nil {
		// Context cancellation is not an error
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("stream error: %w", err)
	}

	// Clean EOF: server closed the stream. If context is still active, signal
	// reconnect by returning a non-nil error; maintainChatSubscription treats
	// nil as a clean client-side shutdown.
	if ctx.Err() != nil {
		return nil
	}
	return errors.New("stream closed by server")
}

// channelInstructions are the server instructions passed to Claude Code.
const channelInstructions = `Messages arrive as <channel> tags with page, sender, and message_id attributes. You have three tools for responding:
- api_v1_ChatService_SendChatReply: Send a new message. Use reply_to_id to thread a response to a specific message.
- api_v1_ChatService_EditChatMessage: Edit one of your previous messages by ID.
- api_v1_ChatService_ReactToMessage: Add an emoji reaction to any message by ID.

Use wiki MCP tools to read/edit pages. Keep responses conversational and concise.`

// buildPageScopedInstructions creates instructions for a page-scoped Claude instance.
func buildPageScopedInstructions(page string) string {
	return fmt.Sprintf(`You are the assistant for the wiki page '%s'. All messages you receive are about this page.

This page has an [ai_agent_chat_context] section in its frontmatter (TOML). This section
contains per-page instructions and memory that persist across chat sessions. You should:
- Read it at the start of each session to understand page-specific context, instructions, and memory.
- Update it using the api_v1_Frontmatter_MergeFrontmatter tool when you learn something worth
  remembering for next time, or when the user asks you to remember something about this page.
- Treat it as YOUR per-page working memory — store instructions, preferences, key facts,
  and context that will help you be more effective in future conversations about this page.

The [ai_agent_chat_context] section is provided as your initial context via a system notification.

Messages arrive as <channel> tags with page, sender, and message_id attributes. You have three tools for responding:
- api_v1_ChatService_SendChatReply: Send a new message. Use reply_to_id to thread a response to a specific message.
- api_v1_ChatService_EditChatMessage: Edit one of your previous messages by ID.
- api_v1_ChatService_ReactToMessage: Add an emoji reaction to any message by ID.

Use wiki MCP tools to read/edit pages. Keep responses conversational and concise.`, page)
}

// injectPageContext fetches the page's ai_agent_chat_context frontmatter section and sends
// it as an initial system channel notification to Claude.
func injectPageContext(ctx context.Context, s *mcpserver.MCPServer, clients *apiClients, page string) {
	fmResp, err := clients.frontmatter.GetFrontmatter(ctx, connect.NewRequest(&apiv1.GetFrontmatterRequest{Page: page}))
	if err != nil {
		log.Printf("Failed to fetch frontmatter for page %q: %v", page, err)
		s.SendNotificationToAllClients(
			"notifications/claude/channel",
			map[string]any{
				"content": fmt.Sprintf("Failed to fetch frontmatter for page '%s': %v. Page context is unavailable for this session. You should NOT attempt to create or modify the [ai_agent_chat_context] section, as existing data may be present but inaccessible due to this error.", page, err),
				"meta": map[string]any{
					"page":   page,
					"sender": "system",
				},
			},
		)
		return
	}

	fm := fmResp.Msg.Frontmatter.AsMap()
	agentContext, ok := fm["ai_agent_chat_context"]
	if !ok {
		s.SendNotificationToAllClients(
			"notifications/claude/channel",
			map[string]any{
				"content": fmt.Sprintf("No [ai_agent_chat_context] section found in page '%s' frontmatter. You can create one using the MergeFrontmatter tool to store per-page instructions and memory for future sessions.", page),
				"meta": map[string]any{
					"page":   page,
					"sender": "system",
				},
			},
		)
		return
	}

	// Format the context as JSON for the notification
	contextJSON, err := json.MarshalIndent(agentContext, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal ai_agent_chat_context for page %q: %v", page, err)
		s.SendNotificationToAllClients(
			"notifications/claude/channel",
			map[string]any{
				"content": fmt.Sprintf("Failed to format [ai_agent_chat_context] for page '%s': %v. The section exists but could not be serialized.", page, err),
				"meta": map[string]any{
					"page":   page,
					"sender": "system",
				},
			},
		)
		return
	}

	s.SendNotificationToAllClients(
		"notifications/claude/channel",
		map[string]any{
			"content": fmt.Sprintf("Here is the [ai_agent_chat_context] from page '%s' frontmatter:\n```json\n%s\n```", page, string(contextJSON)),
			"meta": map[string]any{
				"page":   page,
				"sender": "system",
			},
		},
	)
}

// maintainPageChatSubscription maintains a streaming subscription to page-specific chat messages.
// Same pattern as maintainChatSubscription but uses SubscribePageChatMessages.
func maintainPageChatSubscription(ctx context.Context, s *mcpserver.MCPServer, client apiv1connect.ChatServiceClient, page string) {
	backoffMs := initialBackoffMs

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		start := time.Now()

		err := subscribeToPageChatMessages(ctx, s, client, page)
		if err == nil {
			return
		}

		delayMs, nextMs := computeBackoffAfterFailure(backoffMs, time.Since(start))
		log.Printf("Page chat subscription error for %q: %v. Reconnecting in %dms...", page, err, delayMs)

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(delayMs) * time.Millisecond):
		}

		backoffMs = nextMs
	}
}

// subscribeToPageChatMessages establishes a streaming subscription to user messages for a specific page.
func subscribeToPageChatMessages(ctx context.Context, s *mcpserver.MCPServer, client apiv1connect.ChatServiceClient, page string) error {
	stream, err := client.SubscribePageChatMessages(ctx, connect.NewRequest(&apiv1.SubscribePageChatMessagesRequest{Page: page}))
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("failed to subscribe to page %q: %w", page, err)
	}
	defer func() { _ = stream.Close() }()

	log.Printf("Page chat subscription established for %q", page)

	return receiveChatMessages(ctx, s, stream)
}
