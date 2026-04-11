package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	acp "github.com/coder/acp-go-sdk"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1connect"
	cli "gopkg.in/urfave/cli.v1"
)

const (
	defaultMaxInstances       = 5
	defaultIdleTimeoutMinutes = 30
)

// instanceEntry tracks a running agent instance for a page.
type instanceEntry struct {
	page       string
	conn       *acp.ClientSideConnection
	sessionID  acp.SessionId
	cancel     context.CancelFunc
	lastActive time.Time
	mu         sync.Mutex
}

func (e *instanceEntry) touch() {
	e.mu.Lock()
	e.lastActive = time.Now()
	e.mu.Unlock()
}

func (e *instanceEntry) idleSince() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	return time.Since(e.lastActive)
}

// poolDaemon manages per-page agent instances via ACP.
type poolDaemon struct {
	wikiURL      string
	agentPath    string
	useSystemd   bool
	maxInstances int
	idleTimeout  time.Duration

	ctx       context.Context
	instances map[string]*instanceEntry
	mu        sync.Mutex
}

// sanitizeUnitName converts a page identifier into a valid systemd unit name suffix.
var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func sanitizeUnitName(page string) string {
	return nonAlphanumeric.ReplaceAllString(page, "-")
}

// isSystemdAvailable checks if systemd is the init system.
func isSystemdAvailable() bool {
	_, err := os.Stat("/run/systemd/system")
	return err == nil
}

func buildPoolCommand(urlFlag cli.StringFlag) cli.Command {
	return cli.Command{
		Name:  "pool",
		Usage: "Manage per-page AI agent instances via ACP",
		Description: `Runs a pool daemon that subscribes to instance requests from the wiki server
and spawns dedicated AI agent instances per page on demand using the Agent
Client Protocol (ACP).

Each agent gets its own session with the wiki MCP server for page tools,
and receives the page's ai_agent_chat_context frontmatter as initial context.

When running under systemd, agent processes are spawned as transient units
for per-page journal logging (journalctl -u wiki-chat-<page>).

Example:
  wiki-cli pool --url https://wiki.monster-orfe.ts.net --max-instances 5

The daemon should be run in a directory containing your agent configuration
(CLAUDE.md, agent files, etc.) as the agent will use that directory's context.`,
		Flags: []cli.Flag{
			urlFlag,
			cli.IntFlag{
				Name:  "max-instances",
				Value: defaultMaxInstances,
				Usage: "Maximum concurrent agent instances",
			},
			cli.DurationFlag{
				Name:  "idle-timeout",
				Value: defaultIdleTimeoutMinutes * time.Minute,
				Usage: "Reclaim idle instances after this duration",
			},
			cli.StringFlag{
				Name:  "agent-path",
				Usage: "Path to ACP-compatible agent binary (required)",
			},
			cli.BoolFlag{
				Name:  "no-systemd",
				Usage: "Disable systemd integration even when available",
			},
		},
		Action: func(c *cli.Context) error {
			baseURL := c.String("url")
			normalizedURL, err := normalizeBaseURL(baseURL)
			if err != nil {
				return fmt.Errorf("invalid base URL: %w", err)
			}

			useSystemd := isSystemdAvailable() && !c.Bool("no-systemd")
			if useSystemd {
				log.Println("Systemd detected — agent processes will be spawned as transient units")
			} else {
				log.Println("Systemd not available or disabled — using direct process management")
			}

			agentPath := c.String("agent-path")
			if agentPath == "" {
				return errors.New("--agent-path is required: provide the path to an ACP-compatible agent binary")
			}

			d := &poolDaemon{
				wikiURL:      normalizedURL,
				agentPath:    agentPath,
				useSystemd:   useSystemd,
				maxInstances: c.Int("max-instances"),
				idleTimeout:  c.Duration("idle-timeout"),
				instances:    make(map[string]*instanceEntry),
			}

			return d.run(context.Background())
		},
	}
}

// run starts the pool daemon's main loop.
func (d *poolDaemon) run(ctx context.Context) error {
	d.ctx = ctx
	log.Printf("Pool daemon starting (max=%d, idle-timeout=%s, wiki=%s)", d.maxInstances, d.idleTimeout, d.wikiURL)

	// Start the idle reaper
	go d.reapIdleInstances(ctx)

	// Maintain subscription to instance requests with reconnect loop
	backoffMs := initialBackoffMs

	for {
		select {
		case <-ctx.Done():
			d.stopAll()
			return nil
		default:
		}

		start := time.Now()
		err := d.subscribeAndHandle(ctx)
		if err == nil {
			d.stopAll()
			return nil
		}

		delayMs, nextMs := computeBackoffAfterFailure(backoffMs, time.Since(start))
		log.Printf("Instance request subscription error: %v. Reconnecting in %dms...", err, delayMs)

		select {
		case <-ctx.Done():
			d.stopAll()
			return nil
		case <-time.After(time.Duration(delayMs) * time.Millisecond):
		}

		backoffMs = nextMs
	}
}

// subscribeAndHandle subscribes to instance requests and handles them.
func (d *poolDaemon) subscribeAndHandle(ctx context.Context) error {
	httpClient := &http.Client{}
	client := apiv1connect.NewChatServiceClient(httpClient, d.wikiURL)

	stream, err := client.SubscribeInstanceRequests(ctx, connect.NewRequest(&apiv1.SubscribeInstanceRequestsRequest{}))
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("failed to subscribe to instance requests: %w", err)
	}
	defer func() { _ = stream.Close() }()

	log.Println("Connected to wiki — listening for instance requests")

	for stream.Receive() {
		req := stream.Msg()
		log.Printf("Instance requested for page %q", req.Page)
		d.ensureInstance(req.Page)
	}

	if err := stream.Err(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("stream error: %w", err)
	}

	if ctx.Err() != nil {
		return nil
	}
	return errors.New("stream closed by server")
}

// ensureInstance spawns an agent instance for a page if one doesn't already exist.
func (d *poolDaemon) ensureInstance(page string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Already running?
	if entry, ok := d.instances[page]; ok {
		entry.touch()
		log.Printf("Instance for %q already running — updated lastActive", page)
		return
	}

	// Spawn new instance first, then evict if needed.
	entry, err := d.spawnInstance(page)
	if err != nil {
		log.Printf("Failed to spawn instance for %q: %v", page, err)
		return
	}

	if len(d.instances) >= d.maxInstances {
		d.evictLeastActive()
	}

	d.instances[page] = entry
	log.Printf("Spawned instance for %q (total: %d/%d)", page, len(d.instances), d.maxInstances)
}

// evictLeastActive finds and stops the least recently active instance.
// Must be called with d.mu held.
func (d *poolDaemon) evictLeastActive() {
	var oldestPage string
	var oldestIdle time.Duration

	for page, entry := range d.instances {
		idle := entry.idleSince()
		if oldestPage == "" || idle > oldestIdle {
			oldestPage = page
			oldestIdle = idle
		}
	}

	if oldestPage != "" {
		log.Printf("Evicting instance for %q (idle %s) to make room", oldestPage, oldestIdle.Round(time.Second))
		d.stopInstanceLocked(oldestPage)
	}
}

// wikiChatClient implements acp.Client — handles ACP callbacks from the agent.
// It streams agent responses to the wiki in real-time: the first text chunk
// creates a new chat reply, subsequent chunks edit it in place. Tool calls
// are shown as emoji reactions on the message.
type wikiChatClient struct {
	page       string
	wikiURL    string
	mu              sync.Mutex
	textBuf         strings.Builder
	thoughtBuf      strings.Builder
	permissionNotes strings.Builder
	planEntries     []acp.PlanEntry
	replyToID       string // set before each prompt to thread responses
	currentMsg      string // message ID of the in-progress streaming reply
	pageContext     string // context to prepend to the first user message
	chatClient      apiv1connect.ChatServiceClient
}

func newWikiChatClient(page, wikiURL string) *wikiChatClient {
	httpClient := &http.Client{}
	return &wikiChatClient{
		page:       page,
		wikiURL:    wikiURL,
		chatClient: apiv1connect.NewChatServiceClient(httpClient, wikiURL),
	}
}

func (c *wikiChatClient) SessionUpdate(_ context.Context, n acp.SessionNotification) error {
	u := n.Update

	switch {
	case u.AgentMessageChunk != nil:
		content := u.AgentMessageChunk.Content
		if content.Text == nil {
			return nil
		}

		c.mu.Lock()
		c.textBuf.WriteString(content.Text.Text)
		fullText := c.buildFullText()
		msgID := c.currentMsg
		replyTo := c.replyToID
		c.mu.Unlock()

		if msgID == "" {
			// First chunk — create the reply message
			resp, err := c.chatClient.SendChatReply(context.Background(), connect.NewRequest(&apiv1.SendChatReplyRequest{
				Page:      c.page,
				Content:   fullText,
				ReplyToId: replyTo,
			}))
			if err != nil {
				log.Printf("Failed to create streaming reply for %q: %v", c.page, err)
				return nil
			}
			c.mu.Lock()
			c.currentMsg = resp.Msg.MessageId
			c.mu.Unlock()
		} else {
			// Subsequent chunks — streaming update (not a user edit)
			_, err := c.chatClient.EditChatMessage(context.Background(), connect.NewRequest(&apiv1.EditChatMessageRequest{
				MessageId:  msgID,
				NewContent: fullText,
				Streaming:  true,
			}))
			if err != nil {
				log.Printf("Failed to update streaming reply for %q: %v", c.page, err)
			}
		}

	case u.ToolCall != nil:
		c.mu.Lock()
		msgID := c.currentMsg
		c.mu.Unlock()

		if msgID != "" {
			_, _ = c.chatClient.SendToolCallNotification(context.Background(), connect.NewRequest(&apiv1.SendToolCallNotificationRequest{
				Page:       c.page,
				MessageId:  msgID,
				ToolCallId: string(u.ToolCall.ToolCallId),
				Title:      u.ToolCall.Title,
				Status:     string(u.ToolCall.Status),
			}))
		}
		log.Printf("[%s] Tool call: %s (%s)", c.page, u.ToolCall.Title, u.ToolCall.Status)

	case u.ToolCallUpdate != nil:
		c.mu.Lock()
		msgID := c.currentMsg
		c.mu.Unlock()

		if msgID != "" {
			status := ""
			if u.ToolCallUpdate.Status != nil {
				status = string(*u.ToolCallUpdate.Status)
			}
			title := ""
			if u.ToolCallUpdate.Title != nil {
				title = *u.ToolCallUpdate.Title
			}
			_, _ = c.chatClient.SendToolCallNotification(context.Background(), connect.NewRequest(&apiv1.SendToolCallNotificationRequest{
				Page:       c.page,
				MessageId:  msgID,
				ToolCallId: string(u.ToolCallUpdate.ToolCallId),
				Title:      title,
				Status:     status,
			}))
		}

	case u.AgentThoughtChunk != nil:
		if u.AgentThoughtChunk.Content.Text != nil {
			log.Printf("[%s] Thinking: %s", c.page, truncate(u.AgentThoughtChunk.Content.Text.Text, 100))

			c.mu.Lock()
			c.thoughtBuf.WriteString(u.AgentThoughtChunk.Content.Text.Text)
			thoughtText := c.buildFullText()
			msgID := c.currentMsg
			replyTo := c.replyToID
			c.mu.Unlock()

			// Stream the thinking indicator to chat so the user sees activity
			// Skip if thought text is too short to be meaningful
			if strings.TrimSpace(c.thoughtBuf.String()) == "" {
				// Nothing meaningful to show yet
			} else if msgID == "" {
				resp, err := c.chatClient.SendChatReply(context.Background(), connect.NewRequest(&apiv1.SendChatReplyRequest{
					Page:      c.page,
					Content:   thoughtText,
					ReplyToId: replyTo,
				}))
				if err != nil {
					log.Printf("Failed to create streaming reply for %q: %v", c.page, err)
				} else {
					c.mu.Lock()
					c.currentMsg = resp.Msg.MessageId
					c.mu.Unlock()
				}
			} else {
				_, err := c.chatClient.EditChatMessage(context.Background(), connect.NewRequest(&apiv1.EditChatMessageRequest{
					MessageId:  msgID,
					NewContent: thoughtText,
					Streaming:  true,
				}))
				if err != nil {
					log.Printf("Failed to update streaming reply for %q: %v", c.page, err)
				}
			}
		}

	case u.Plan != nil:
		c.mu.Lock()
		c.planEntries = u.Plan.Entries
		fullText := c.buildFullText()
		msgID := c.currentMsg
		replyTo := c.replyToID
		c.mu.Unlock()

		log.Printf("[%s] Plan update: %d entries", c.page, len(u.Plan.Entries))

		if msgID == "" {
			resp, err := c.chatClient.SendChatReply(context.Background(), connect.NewRequest(&apiv1.SendChatReplyRequest{
				Page:      c.page,
				Content:   fullText,
				ReplyToId: replyTo,
			}))
			if err != nil {
				log.Printf("Failed to create streaming reply for %q: %v", c.page, err)
			} else {
				c.mu.Lock()
				c.currentMsg = resp.Msg.MessageId
				c.mu.Unlock()
			}
		} else {
			_, err := c.chatClient.EditChatMessage(context.Background(), connect.NewRequest(&apiv1.EditChatMessageRequest{
				MessageId:  msgID,
				NewContent: fullText,
				Streaming:  true,
			}))
			if err != nil {
				log.Printf("Failed to update streaming reply for %q: %v", c.page, err)
			}
		}
	}

	return nil
}

// beginTurn prepares for a new prompt turn — resets the streaming state.
// buildFullText returns the combined response text, prepending any accumulated
// thinking content as a collapsible markdown <details> section.
// Must be called with c.mu held.
func (c *wikiChatClient) buildFullText() string {
	thought := c.thoughtBuf.String()
	response := c.textBuf.String()
	permissions := c.permissionNotes.String()

	var result string
	if thought == "" {
		result = response
	} else {
		result = "<details><summary>Thinking...</summary>\n\n" + thought + "\n\n</details>\n\n" + response
	}

	if len(c.planEntries) > 0 {
		result += "\n\n" + c.buildPlanSection()
	}

	if permissions != "" {
		result += "\n\n" + permissions
	}

	return result
}

// buildPlanSection renders plan entries as a markdown checklist.
// Must be called with c.mu held.
func (c *wikiChatClient) buildPlanSection() string {
	var sb strings.Builder
	sb.WriteString("**Plan:**\n")

	for _, entry := range c.planEntries {
		switch entry.Status {
		case acp.PlanEntryStatusCompleted:
			sb.WriteString("- [x] " + entry.Content + "\n")
		case acp.PlanEntryStatusInProgress:
			sb.WriteString("- 🔄 " + entry.Content + "\n")
		default: // pending
			sb.WriteString("- [ ] " + entry.Content + "\n")
		}
	}

	return sb.String()
}

func (c *wikiChatClient) beginTurn(replyToID string) {
	c.mu.Lock()
	c.textBuf.Reset()
	c.thoughtBuf.Reset()
	c.permissionNotes.Reset()
	c.planEntries = nil
	c.replyToID = replyToID
	c.currentMsg = ""
	c.mu.Unlock()
}

// endTurn cleans up after a prompt turn completes.
func (c *wikiChatClient) endTurn() {
	c.mu.Lock()
	c.currentMsg = ""
	c.mu.Unlock()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func (c *wikiChatClient) RequestPermission(_ context.Context, p acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	// Auto-approve all permissions — the agent is trusted.
	// Log and notify the chat so the user sees what was granted.
	title := string(p.ToolCall.ToolCallId)
	if p.ToolCall.Title != nil {
		title = *p.ToolCall.Title
	}

	if len(p.Options) == 0 {
		log.Printf("[%s] Permission request cancelled (no options): %s", c.page, title)

		c.mu.Lock()
		fmt.Fprintf(&c.permissionNotes, "> \U0001F510 **Permission cancelled** (no options): %s\n", title)
		c.mu.Unlock()

		return acp.RequestPermissionResponse{
			Outcome: acp.RequestPermissionOutcome{
				Cancelled: &acp.RequestPermissionOutcomeCancelled{},
			},
		}, nil
	}

	selected := p.Options[0]
	log.Printf("[%s] Permission granted: %s — %s", c.page, title, selected.Name)

	c.mu.Lock()
	fmt.Fprintf(&c.permissionNotes, "> \U0001F510 **Permission granted:** %s — %s\n", title, selected.Name)
	c.mu.Unlock()

	return acp.RequestPermissionResponse{
		Outcome: acp.RequestPermissionOutcome{
			Selected: &acp.RequestPermissionOutcomeSelected{OptionId: selected.OptionId},
		},
	}, nil
}

// File system operations — deny all, agent should use wiki MCP tools
func (*wikiChatClient) ReadTextFile(_ context.Context, _ acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, errors.New("file system access not available — use wiki MCP tools")
}

func (*wikiChatClient) WriteTextFile(_ context.Context, _ acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, errors.New("file system access not available — use wiki MCP tools")
}

// Terminal operations — deny all
func (*wikiChatClient) CreateTerminal(_ context.Context, _ acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, errors.New("terminal access not available")
}

func (*wikiChatClient) KillTerminalCommand(_ context.Context, _ acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	return acp.KillTerminalCommandResponse{}, errors.New("terminal access not available")
}

func (*wikiChatClient) TerminalOutput(_ context.Context, _ acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, errors.New("terminal access not available")
}

func (*wikiChatClient) ReleaseTerminal(_ context.Context, _ acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, errors.New("terminal access not available")
}

func (*wikiChatClient) WaitForTerminalExit(_ context.Context, _ acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, errors.New("terminal access not available")
}

// spawnInstance starts an ACP agent process for a page, performs the handshake,
// creates a session with wiki MCP tools, and starts the message bridge.
// Must be called with d.mu held.
func (d *poolDaemon) spawnInstance(page string) (*instanceEntry, error) {
	if d.ctx == nil {
		return nil, errors.New("pool daemon context not initialized")
	}
	ctx, cancel := context.WithCancel(d.ctx)

	// Spawn agent process
	var cmd *exec.Cmd
	if d.useSystemd {
		unitName := "wiki-chat-" + sanitizeUnitName(page)
		// Stop any stale scope unit from a previous spawn
		_ = exec.Command("systemctl", "--user", "stop", unitName+".scope").Run()
		cmd = exec.CommandContext(ctx, "systemd-run",
			"--user",
			"--unit="+unitName,
			"--scope",
			d.agentPath,
		)
	} else {
		cmd = exec.CommandContext(ctx, d.agentPath)
		cmd.Stderr = newPrefixWriter(os.Stderr, page)
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start agent for page %q: %w", page, err)
	}

	// Monitor process exit
	go func() {
		waitErr := cmd.Wait()
		if waitErr != nil && ctx.Err() == nil {
			log.Printf("Agent for %q exited: %v", page, waitErr)
		} else {
			log.Printf("Agent for %q exited cleanly", page)
		}
	}()

	// Create ACP client connection
	chatClient := newWikiChatClient(page, d.wikiURL)
	conn := acp.NewClientSideConnection(chatClient, stdinPipe, stdoutPipe)

	// ACP handshake
	_, err = conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapability{
				ReadTextFile:  false,
				WriteTextFile: false,
			},
			Terminal: false,
		},
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("ACP handshake failed for page %q: %w", page, err)
	}

	// Build MCP server config for the wiki
	wikiCLIBin, execErr := os.Executable()
	if execErr != nil {
		cancel()
		return nil, fmt.Errorf("failed to find wiki-cli binary: %w", execErr)
	}

	cwd, _ := os.Getwd()

	// Create session with wiki MCP server
	sess, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd: cwd,
		McpServers: []acp.McpServer{{
			Stdio: &acp.McpServerStdio{
				Name:    "wiki",
				Command: wikiCLIBin,
				Args:    []string{"mcp", "--url", d.wikiURL},
				Env:     []acp.EnvVariable{},
			},
		}},
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("ACP session creation failed for page %q: %w", page, err)
	}

	entry := &instanceEntry{
		page:       page,
		conn:       conn,
		sessionID:  sess.SessionId,
		cancel:     cancel,
		lastActive: time.Now(),
	}

	// Inject page context and start message bridge
	go d.runMessageBridge(ctx, entry, chatClient)

	return entry, nil
}

// runMessageBridge subscribes to user messages and forwards them as ACP prompts.
// Page context is prepended to the first user message rather than sent as a
// separate prompt, ensuring the bridge is listening before any prompt is sent.
func (d *poolDaemon) runMessageBridge(ctx context.Context, entry *instanceEntry, chatClient *wikiChatClient) {
	page := entry.page

	// Fetch page context (but don't send it as a prompt yet — prepend to first message)
	chatClient.mu.Lock()
	chatClient.pageContext = d.fetchPageContext(ctx, page)
	chatClient.mu.Unlock()

	// Subscribe to page messages FIRST — this ensures we don't miss the triggering message
	httpClient := &http.Client{}
	wikiClient := apiv1connect.NewChatServiceClient(httpClient, d.wikiURL)

	backoffMs := initialBackoffMs
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		start := time.Now()
		err := d.bridgeMessages(ctx, entry, wikiClient, chatClient)
		if err == nil {
			return
		}

		delayMs, nextMs := computeBackoffAfterFailure(backoffMs, time.Since(start))
		log.Printf("Message bridge for %q error: %v. Reconnecting in %dms...", page, err, delayMs)

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(delayMs) * time.Millisecond):
		}

		backoffMs = nextMs
	}
}

// bridgeMessages subscribes to page chat messages and forwards them as ACP prompts.
// It also subscribes to cancellation signals so the frontend "Stop" button can
// cancel an in-progress prompt.
func (d *poolDaemon) bridgeMessages(ctx context.Context, entry *instanceEntry, wikiClient apiv1connect.ChatServiceClient, chatClient *wikiChatClient) error {
	stream, err := wikiClient.SubscribePageChatMessages(ctx, connect.NewRequest(&apiv1.SubscribePageChatMessagesRequest{
		Page: entry.page,
	}))
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("failed to subscribe to page messages: %w", err)
	}
	defer func() { _ = stream.Close() }()

	// Subscribe to cancellation signals for this page
	cancelStream, cancelErr := wikiClient.SubscribePageCancellations(ctx, connect.NewRequest(&apiv1.SubscribePageCancellationsRequest{
		Page: entry.page,
	}))
	if cancelErr != nil {
		log.Printf("Warning: failed to subscribe to cancellations for %q: %v", entry.page, cancelErr)
	}

	// Channel that receives cancel signals from the cancellation stream
	cancelChan := make(chan struct{}, 1)
	if cancelStream != nil {
		go func() {
			defer close(cancelChan)
			for cancelStream.Receive() {
				select {
				case cancelChan <- struct{}{}:
				default:
				}
			}
		}()
	}
	defer func() {
		if cancelStream != nil {
			_ = cancelStream.Close()
		}
	}()

	log.Printf("Message bridge connected for page %q", entry.page)

	for stream.Receive() {
		msg := stream.Msg()

		// Only forward user messages
		if msg.Sender != apiv1.Sender_USER {
			continue
		}

		entry.touch()

		// Prepare for streaming response
		chatClient.beginTurn(msg.Id)

		// Create a cancellable context for this prompt
		promptCtx, promptCancel := context.WithCancel(ctx)

		// Listen for cancel signals during this prompt
		go func() {
			select {
			case <-cancelChan:
				log.Printf("Cancelling prompt for page %q", entry.page)
				promptCancel()
			case <-promptCtx.Done():
			}
		}()

		// Prepend page context to the first user message so the agent gets
		// context + question in one prompt (no separate context turn that blocks the bridge).
		promptText := msg.Content
		chatClient.mu.Lock()
		if chatClient.pageContext != "" {
			promptText = chatClient.pageContext + "\n\n---\n\nUser message: " + msg.Content
			chatClient.pageContext = "" // only prepend once
		}
		chatClient.mu.Unlock()

		// Send as ACP prompt — blocks until the agent completes its turn.
		// During the turn, SessionUpdate streams text chunks to the wiki in real-time.
		_, promptErr := entry.conn.Prompt(promptCtx, acp.PromptRequest{
			SessionId: entry.sessionID,
			Prompt:    []acp.ContentBlock{acp.TextBlock(promptText)},
		})
		promptCancel() // Clean up the cancel goroutine

		if promptErr != nil {
			if promptCtx.Err() != nil && ctx.Err() == nil {
				// Prompt was cancelled by user, not by parent context shutdown
				log.Printf("Prompt cancelled for page %q", entry.page)
			} else {
				log.Printf("Failed to send prompt to agent for %q: %v", entry.page, promptErr)
			}
		}

		chatClient.endTurn()
	}

	if err := stream.Err(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("stream error: %w", err)
	}

	if ctx.Err() != nil {
		return nil
	}
	return errors.New("stream closed by server")
}

// chatPreamble is prepended to the first prompt to establish the interactive chat context.
const chatPreamble = `You are in an INTERACTIVE CHAT session with a user on a wiki page. This is a conversation, not a coding task.

CRITICAL: Respond quickly and conversationally. Do NOT explore the codebase, run startup hooks, or do extensive initialization. The user is waiting for your reply in a chat window.

Rules:
- Keep responses concise and conversational
- Use the wiki MCP tools only when the user asks you to read or edit pages
- Each message you receive is from a user chatting on this wiki page

CONTINUITY: After each conversation, update the [ai_agent_chat_context] section in the page's frontmatter (via MergeFrontmatter MCP tool) with:
- A brief summary of what was discussed
- Any goals, decisions, or action items the user mentioned
- Key context that would help you continue the conversation next time
This is your per-page memory — without it you have no history between sessions. Always maintain it.

`

// fetchPageContext retrieves the page's ai_agent_chat_context frontmatter and returns
// it as a text string to prepend to the first user message.
func (d *poolDaemon) fetchPageContext(ctx context.Context, page string) string {
	httpClient := &http.Client{}
	fmClient := apiv1connect.NewFrontmatterClient(httpClient, d.wikiURL)

	fmResp, err := fmClient.GetFrontmatter(ctx, connect.NewRequest(&apiv1.GetFrontmatterRequest{Page: page}))
	if err != nil {
		log.Printf("Failed to fetch frontmatter for page %q: %v", page, err)
		return chatPreamble + fmt.Sprintf(
			"You are the assistant for wiki page '%s'. Failed to fetch page context: %v. Do NOT attempt to create or modify the [ai_agent_chat_context] section.",
			page, err,
		)
	}

	fm := fmResp.Msg.Frontmatter.AsMap()
	agentContext, ok := fm["ai_agent_chat_context"]
	if !ok {
		return chatPreamble + fmt.Sprintf(
			"You are the assistant for wiki page '%s'. No [ai_agent_chat_context] section exists yet. You can create one using the MergeFrontmatter MCP tool to store per-page memory for future sessions.",
			page,
		)
	}

	contextJSON, err := json.MarshalIndent(agentContext, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal ai_agent_chat_context for page %q: %v", page, err)
		return chatPreamble + fmt.Sprintf("You are the assistant for wiki page '%s'.", page)
	}

	return chatPreamble + fmt.Sprintf(
		"You are the assistant for wiki page '%s'. Here is your per-page working memory from [ai_agent_chat_context]:\n```json\n%s\n```\nUpdate it using the MergeFrontmatter MCP tool when you learn something worth remembering.",
		page, string(contextJSON),
	)
}

// stopInstanceLocked stops an instance. Must be called with d.mu held.
func (d *poolDaemon) stopInstanceLocked(page string) {
	entry, ok := d.instances[page]
	if !ok {
		return
	}

	entry.cancel()
	delete(d.instances, page)
}

// stopAll stops all running instances.
func (d *poolDaemon) stopAll() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for page := range d.instances {
		d.stopInstanceLocked(page)
	}
	log.Println("All instances stopped")
}

// reapIdleInstances periodically stops instances that have been idle too long.
func (d *poolDaemon) reapIdleInstances(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.mu.Lock()
			for page, entry := range d.instances {
				if entry.idleSince() > d.idleTimeout {
					log.Printf("Reaping idle instance for %q (idle %s)", page, entry.idleSince().Round(time.Second))
					d.stopInstanceLocked(page)
				}
			}
			d.mu.Unlock()
		}
	}
}

// prefixWriter wraps an io.Writer and prefixes each line with a tag.
type prefixWriter struct {
	w      *os.File
	prefix string
	buf    []byte
}

func newPrefixWriter(w *os.File, page string) *prefixWriter {
	return &prefixWriter{
		w:      w,
		prefix: "[" + page + "] ",
	}
}

func (pw *prefixWriter) Write(p []byte) (int, error) {
	pw.buf = append(pw.buf, p...)

	for {
		idx := bytes.IndexByte(pw.buf, '\n')
		if idx < 0 {
			break
		}

		line := pw.buf[:idx+1]
		if _, err := fmt.Fprint(pw.w, pw.prefix+string(line)); err != nil {
			return 0, err
		}
		pw.buf = pw.buf[idx+1:]
	}

	return len(p), nil
}
