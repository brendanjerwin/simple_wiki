package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	defaultMaxInstances                   = 5
	defaultIdleTimeoutMinutes             = 30
	defaultMaxInstanceAgeHours            = 2
	defaultPermissionPendingTimeoutMinutes = 5

	errTerminalAccessUnavailable = "terminal access not available"
	truncateLimitForLog          = 100
	truncateLimitForBridge       = 80

	// Structured log field keys.
	logKeyPage   = "page"
	logKeyAction = "action"
	logKeyError  = "error"
	logKeyTool   = "tool"

	// Structured log messages.
	logMsgStateTransitionError = "state transition error"
)

// permissionRequestTimeout is the maximum time to wait for a user to respond
// to a permission request. After this duration the request is auto-denied so
// the agent is not stuck in PermissionPending state forever.
var permissionRequestTimeout = 5 * time.Minute

// InstanceState represents the lifecycle state of an agent instance.
type InstanceState int

const (
	StateSpawning          InstanceState = iota
	StateInitializing      // handshake done, session being created
	StateBridgeConnecting  // subscribing to page messages
	StateIdle              // bridge connected, waiting for user messages
	StatePrompting         // Prompt call in progress
	StatePermissionPending // waiting for user permission response
	StateStopping          // being shut down
	StateDead              // terminated
)

func (s InstanceState) String() string {
	switch s {
	case StateSpawning:
		return "Spawning"
	case StateInitializing:
		return "Initializing"
	case StateBridgeConnecting:
		return "BridgeConnecting"
	case StateIdle:
		return "Idle"
	case StatePrompting:
		return "Prompting"
	case StatePermissionPending:
		return "PermissionPending"
	case StateStopping:
		return "Stopping"
	case StateDead:
		return "Dead"
	default:
		return fmt.Sprintf("Unknown(%d)", int(s))
	}
}

// validTransitions defines the legal state transitions for an instance.
var validTransitions = map[InstanceState][]InstanceState{
	StateSpawning:          {StateInitializing, StateDead},
	StateInitializing:      {StateBridgeConnecting, StateDead},
	StateBridgeConnecting:  {StateIdle, StateDead},
	StateIdle:              {StatePrompting, StateStopping, StateDead},
	StatePrompting:         {StateIdle, StatePermissionPending, StateStopping, StateDead},
	StatePermissionPending: {StatePrompting, StateStopping, StateDead},
	StateStopping:          {StateDead},
	StateDead:              {},
}

// instanceEntry tracks a running agent instance for a page.
type instanceEntry struct {
	page           string
	conn           acpAgent
	sessionID      acp.SessionId
	cancel         context.CancelFunc
	lastActive     time.Time
	createdAt      time.Time
	stateChangedAt time.Time
	state          InstanceState
	mu             sync.Mutex
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

// age returns how long ago this instance was created.
func (e *instanceEntry) age() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	return time.Since(e.createdAt)
}

// inStateSince returns how long the instance has been in its current state.
func (e *instanceEntry) inStateSince() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	return time.Since(e.stateChangedAt)
}

// setState transitions the instance to a new state. It validates that the
// transition is legal according to validTransitions and logs the change.
// Must be called without e.mu held — it acquires the lock internally.
func (e *instanceEntry) setState(newState InstanceState) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.setStateLocked(newState)
}

// setStateLocked transitions the instance to a new state while the mutex is
// already held. It validates the transition and logs the change.
func (e *instanceEntry) setStateLocked(newState InstanceState) error {
	oldState := e.state
	allowed := validTransitions[oldState]

	for _, s := range allowed {
		if s == newState {
			e.state = newState
			e.stateChangedAt = time.Now()
			slog.Info("state transition", logKeyPage, e.page, "from_state", oldState.String(), "to_state", newState.String(), logKeyAction, "state_change")
			return nil
		}
	}

	return fmt.Errorf("[%s] invalid state transition: %s -> %s", e.page, oldState, newState)
}

// State returns the current state of the instance (thread-safe).
func (e *instanceEntry) State() InstanceState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

// poolDaemon manages per-page agent instances via ACP.
type poolDaemon struct {
	wikiURL                  string
	agentPath                string
	useSystemd               bool
	maxInstances             int
	idleTimeout              time.Duration
	maxInstanceAge           time.Duration
	permissionPendingTimeout time.Duration

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
			cli.DurationFlag{
				Name:  "max-instance-age",
				Value: defaultMaxInstanceAgeHours * time.Hour,
				Usage: "Maximum instance lifetime regardless of state (0 to disable)",
			},
			cli.DurationFlag{
				Name:  "permission-pending-timeout",
				Value: defaultPermissionPendingTimeoutMinutes * time.Minute,
				Usage: "Reclaim PermissionPending instances after this duration (0 to disable)",
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
			return runPoolAction(c)
		},
	}
}

func runPoolAction(c *cli.Context) error {
	normalizedURL, err := normalizeBaseURL(c.String("url"))
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	useSystemd := isSystemdAvailable() && !c.Bool("no-systemd")
	if useSystemd {
		slog.Info("systemd detected, agent processes will be spawned as transient units", logKeyAction, "startup")
	} else {
		slog.Info("systemd not available or disabled, using direct process management", logKeyAction, "startup")
	}

	agentPath := c.String("agent-path")
	if agentPath == "" {
		return errors.New("--agent-path is required: provide the path to an ACP-compatible agent binary")
	}

	d := &poolDaemon{
		wikiURL:                  normalizedURL,
		agentPath:                agentPath,
		useSystemd:               useSystemd,
		maxInstances:             c.Int("max-instances"),
		idleTimeout:              c.Duration("idle-timeout"),
		maxInstanceAge:           c.Duration("max-instance-age"),
		permissionPendingTimeout: c.Duration("permission-pending-timeout"),
		instances:                make(map[string]*instanceEntry),
	}

	return d.run(context.Background())
}

// run starts the pool daemon's main loop.
func (d *poolDaemon) run(ctx context.Context) error {
	slog.Info("pool daemon starting",
		"max_instances", d.maxInstances,
		"idle_timeout", d.idleTimeout,
		"max_age", d.maxInstanceAge,
		"perm_pending_timeout", d.permissionPendingTimeout,
		"wiki", d.wikiURL,
		logKeyAction, "startup")

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
		slog.Warn("instance request subscription error, reconnecting", logKeyError, err, "delay_ms", delayMs, logKeyAction, "reconnect")

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

	slog.Info("connected to wiki, listening for instance requests", logKeyAction, "connected")

	for stream.Receive() {
		req := stream.Msg()
		slog.Info("instance requested", logKeyPage, req.Page, logKeyAction, "instance_requested")
		d.ensureInstance(ctx, req.Page)
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
func (d *poolDaemon) ensureInstance(ctx context.Context, page string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Already running?
	if entry, ok := d.instances[page]; ok {
		entry.touch()
		slog.Info("instance already running, updated lastActive", logKeyPage, page, logKeyAction, "touch")
		return
	}

	// Spawn new instance first, then evict if needed.
	entry, err := d.spawnInstance(ctx, page)
	if err != nil {
		slog.Error("failed to spawn instance", logKeyPage, page, logKeyError, err, logKeyAction, "spawn")
		return
	}

	if len(d.instances) >= d.maxInstances {
		d.evictLeastActive()
	}

	d.instances[page] = entry
	slog.Info("instance spawned", logKeyPage, page, "instance_count", len(d.instances), "max_instances", d.maxInstances, logKeyAction, "created")
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
		slog.Info("evicting least active instance to make room", logKeyPage, oldestPage, "idle_duration", oldestIdle.Round(time.Second), logKeyAction, "evict")
		d.stopInstanceLocked(oldestPage)
	}
}

// chatReplier is the interface for sending chat replies to the wiki.
// Extracted from apiv1connect.ChatServiceClient for testability.
type chatReplier interface {
	SendChatReply(context.Context, *connect.Request[apiv1.SendChatReplyRequest]) (*connect.Response[apiv1.SendChatReplyResponse], error)
	EditChatMessage(context.Context, *connect.Request[apiv1.EditChatMessageRequest]) (*connect.Response[apiv1.EditChatMessageResponse], error)
	SendToolCallNotification(context.Context, *connect.Request[apiv1.SendToolCallNotificationRequest]) (*connect.Response[apiv1.SendToolCallNotificationResponse], error)
	RequestPermissionFromUser(context.Context, *connect.Request[apiv1.RequestPermissionFromUserRequest]) (*connect.Response[apiv1.RequestPermissionFromUserResponse], error)
}

// pageMessageSource subscribes to page messages and cancellations.
// Extracted from apiv1connect.ChatServiceClient for testability.
type pageMessageSource interface {
	SubscribePageChatMessages(context.Context, *connect.Request[apiv1.SubscribePageChatMessagesRequest]) (*connect.ServerStreamForClient[apiv1.ChatMessage], error)
	SubscribePageCancellations(context.Context, *connect.Request[apiv1.SubscribePageCancellationsRequest]) (*connect.ServerStreamForClient[apiv1.PageCancellation], error)
}

// acpAgent provides session and prompt capabilities.
// Extracted from *acp.ClientSideConnection for testability.
type acpAgent interface {
	Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error)
	NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error)
	Prompt(context.Context, acp.PromptRequest) (acp.PromptResponse, error)
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
	chatClient      chatReplier
	entry           *instanceEntry // back-reference for state transitions
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
		return c.handleAgentMessage(u.AgentMessageChunk)
	case u.ToolCall != nil:
		c.handleToolCall(u.ToolCall)
	case u.ToolCallUpdate != nil:
		c.handleToolCallUpdate(u.ToolCallUpdate)
	case u.AgentThoughtChunk != nil:
		c.handleThought(u.AgentThoughtChunk)
	case u.Plan != nil:
		c.handlePlan(u.Plan)
	default:
		// Unknown update type — nothing to do
	}

	return nil
}

// handleAgentMessage processes an agent text chunk: accumulates text and streams
// the reply to the wiki chat (creating a new message or editing the existing one).
func (c *wikiChatClient) handleAgentMessage(chunk *acp.SessionUpdateAgentMessageChunk) error {
	content := chunk.Content
	if content.Text == nil {
		return nil
	}

	c.mu.Lock()
	_, _ = c.textBuf.WriteString(content.Text.Text)
	fullText := c.buildFullText()
	msgID := c.currentMsg
	replyTo := c.replyToID
	c.mu.Unlock()

	if strings.TrimSpace(c.textBuf.String()) == "" {
		return nil
	}

	if msgID == "" {
		resp, err := c.chatClient.SendChatReply(context.Background(), connect.NewRequest(&apiv1.SendChatReplyRequest{
			Page:      c.page,
			Content:   fullText,
			ReplyToId: replyTo,
		}))
		if err != nil {
			slog.Error("failed to create streaming reply", logKeyPage, c.page, logKeyError, err)
			return nil
		}
		c.mu.Lock()
		c.currentMsg = resp.Msg.MessageId
		c.mu.Unlock()
	} else {
		_, err := c.chatClient.EditChatMessage(context.Background(), connect.NewRequest(&apiv1.EditChatMessageRequest{
			MessageId:  msgID,
			NewContent: fullText,
			Streaming:  true,
		}))
		if err != nil {
			slog.Error("failed to update streaming reply", logKeyPage, c.page, logKeyError, err)
		}
	}

	return nil
}

// handleToolCall sends a tool call notification to the wiki chat.
func (c *wikiChatClient) handleToolCall(tc *acp.SessionUpdateToolCall) {
	c.mu.Lock()
	msgID := c.currentMsg
	c.mu.Unlock()

	if msgID != "" {
		_, _ = c.chatClient.SendToolCallNotification(context.Background(), connect.NewRequest(&apiv1.SendToolCallNotificationRequest{
			Page:       c.page,
			MessageId:  msgID,
			ToolCallId: string(tc.ToolCallId),
			Title:      tc.Title,
			Status:     string(tc.Status),
		}))
	}
	slog.Info("tool call", logKeyPage, c.page, logKeyTool, tc.Title, "status", string(tc.Status))
}

// handleToolCallUpdate sends an updated tool call notification to the wiki chat.
func (c *wikiChatClient) handleToolCallUpdate(tcu *acp.SessionToolCallUpdate) {
	c.mu.Lock()
	msgID := c.currentMsg
	c.mu.Unlock()

	if msgID == "" {
		return
	}

	status := ""
	if tcu.Status != nil {
		status = string(*tcu.Status)
	}
	title := ""
	if tcu.Title != nil {
		title = *tcu.Title
	}
	_, _ = c.chatClient.SendToolCallNotification(context.Background(), connect.NewRequest(&apiv1.SendToolCallNotificationRequest{
		Page:       c.page,
		MessageId:  msgID,
		ToolCallId: string(tcu.ToolCallId),
		Title:      title,
		Status:     status,
	}))
}

// handleThought accumulates thinking text and streams it as a collapsible section.
func (c *wikiChatClient) handleThought(chunk *acp.SessionUpdateAgentThoughtChunk) {
	if chunk.Content.Text == nil {
		return
	}

	slog.Info("agent thinking", logKeyPage, c.page, "thought", truncate(chunk.Content.Text.Text, truncateLimitForLog))

	c.mu.Lock()
	_, _ = c.thoughtBuf.WriteString(chunk.Content.Text.Text)
	thoughtText := c.buildFullText()
	msgID := c.currentMsg
	replyTo := c.replyToID
	c.mu.Unlock()

	if strings.TrimSpace(c.thoughtBuf.String()) == "" {
		return
	}

	c.streamOrCreateReply(msgID, replyTo, thoughtText)
}

// handlePlan updates the plan entries and streams the updated text to the chat.
func (c *wikiChatClient) handlePlan(plan *acp.SessionUpdatePlan) {
	c.mu.Lock()
	c.planEntries = plan.Entries
	fullText := c.buildFullText()
	msgID := c.currentMsg
	replyTo := c.replyToID
	c.mu.Unlock()

	slog.Info("plan update", logKeyPage, c.page, "entry_count", len(plan.Entries))

	c.streamOrCreateReply(msgID, replyTo, fullText)
}

// streamOrCreateReply either creates a new streaming reply or edits the existing one.
func (c *wikiChatClient) streamOrCreateReply(msgID, replyTo, text string) {
	if msgID == "" {
		resp, err := c.chatClient.SendChatReply(context.Background(), connect.NewRequest(&apiv1.SendChatReplyRequest{
			Page:      c.page,
			Content:   text,
			ReplyToId: replyTo,
		}))
		if err != nil {
			slog.Error("failed to create streaming reply", logKeyPage, c.page, logKeyError, err)
		} else {
			c.mu.Lock()
			c.currentMsg = resp.Msg.MessageId
			c.mu.Unlock()
		}
	} else {
		_, err := c.chatClient.EditChatMessage(context.Background(), connect.NewRequest(&apiv1.EditChatMessageRequest{
			MessageId:  msgID,
			NewContent: text,
			Streaming:  true,
		}))
		if err != nil {
			slog.Error("failed to update streaming reply", logKeyPage, c.page, logKeyError, err)
		}
	}
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
	_, _ = sb.WriteString("**Plan:**\n")

	for _, entry := range c.planEntries {
		switch entry.Status {
		case acp.PlanEntryStatusCompleted:
			_, _ = sb.WriteString("- [x] " + entry.Content + "\n")
		case acp.PlanEntryStatusInProgress:
			_, _ = sb.WriteString("- 🔄 " + entry.Content + "\n")
		default: // pending
			_, _ = sb.WriteString("- [ ] " + entry.Content + "\n")
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

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}

func (c *wikiChatClient) RequestPermission(ctx context.Context, p acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	if c.entry != nil {
		if err := c.entry.setState(StatePermissionPending); err != nil {
			slog.Warn(logMsgStateTransitionError, logKeyPage, c.page, logKeyError, err)
		}
		defer func() {
			if err := c.entry.setState(StatePrompting); err != nil {
				slog.Warn(logMsgStateTransitionError, logKeyPage, c.page, logKeyError, err)
			}
		}()
	}

	title := string(p.ToolCall.ToolCallId)
	if p.ToolCall.Title != nil {
		title = *p.ToolCall.Title
	}

	if len(p.Options) == 0 {
		slog.Warn("permission request cancelled, no options", logKeyPage, c.page, logKeyTool, title, logKeyAction, "permission_denied")
		return permissionCancelledResponse(), nil
	}

	slog.Info("permission requested from user", logKeyPage, c.page, logKeyTool, title, "option_count", len(p.Options), logKeyAction, "permission_requested")

	return c.relayPermissionToUser(ctx, p, title)
}

// relayPermissionToUser forwards the permission request to the wiki chat UI and
// blocks until the user responds or the permission request timeout elapses.
// If the timeout expires the request is auto-denied so the agent does not
// remain stuck in PermissionPending state when the user navigates away.
func (c *wikiChatClient) relayPermissionToUser(ctx context.Context, p acp.RequestPermissionRequest, title string) (acp.RequestPermissionResponse, error) {
	requestID := fmt.Sprintf("perm-%d", time.Now().UnixNano())

	var protoOptions []*apiv1.ChatPermissionOption
	for _, opt := range p.Options {
		protoOptions = append(protoOptions, &apiv1.ChatPermissionOption{
			OptionId: string(opt.OptionId),
			Label:    opt.Name,
		})
	}

	permCtx, cancel := context.WithTimeout(ctx, permissionRequestTimeout)
	defer cancel()

	resp, err := c.chatClient.RequestPermissionFromUser(permCtx, connect.NewRequest(&apiv1.RequestPermissionFromUserRequest{
		Page:        c.page,
		RequestId:   requestID,
		Title:       title,
		Description: title,
		Options:     protoOptions,
	}))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			slog.Warn("permission request timed out, auto-denying", logKeyPage, c.page, "timeout", permissionRequestTimeout, logKeyTool, title, logKeyAction, "permission_denied")
			return permissionCancelledResponse(), nil
		}
		slog.Warn("permission request failed, auto-approving", logKeyPage, c.page, logKeyError, err, logKeyAction, "permission_auto_approved")
		selected := p.Options[0]
		return permissionSelectedResponse(selected.OptionId), nil
	}

	return c.processPermissionResponse(resp.Msg.SelectedOptionId, p.Options, title)
}

// processPermissionResponse interprets the user's selection (or denial) and
// records it in the permission notes buffer.
func (c *wikiChatClient) processPermissionResponse(selectedID string, options []acp.PermissionOption, title string) (acp.RequestPermissionResponse, error) {
	if selectedID == "" {
		slog.Info("permission denied by user", logKeyPage, c.page, logKeyTool, title, logKeyAction, "permission_denied")
		c.mu.Lock()
		_, _ = fmt.Fprintf(&c.permissionNotes, "> \U0001F510 **Permission denied:** %s\n", title)
		c.mu.Unlock()
		return permissionCancelledResponse(), nil
	}

	selectedName := selectedID
	for _, opt := range options {
		if string(opt.OptionId) == selectedID {
			selectedName = opt.Name
			break
		}
	}
	slog.Info("permission granted by user", logKeyPage, c.page, logKeyTool, title, "option", selectedName, logKeyAction, "permission_granted")

	c.mu.Lock()
	_, _ = fmt.Fprintf(&c.permissionNotes, "> \U0001F510 **Permission granted:** %s — %s\n", title, selectedName)
	c.mu.Unlock()

	return permissionSelectedResponse(acp.PermissionOptionId(selectedID)), nil
}

func permissionCancelledResponse() acp.RequestPermissionResponse {
	return acp.RequestPermissionResponse{
		Outcome: acp.RequestPermissionOutcome{
			Cancelled: &acp.RequestPermissionOutcomeCancelled{},
		},
	}
}

func permissionSelectedResponse(optionID acp.PermissionOptionId) acp.RequestPermissionResponse {
	return acp.RequestPermissionResponse{
		Outcome: acp.RequestPermissionOutcome{
			Selected: &acp.RequestPermissionOutcomeSelected{OptionId: optionID},
		},
	}
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
	return acp.CreateTerminalResponse{}, errors.New(errTerminalAccessUnavailable)
}

func (*wikiChatClient) KillTerminal(_ context.Context, _ acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, errors.New(errTerminalAccessUnavailable)
}

func (*wikiChatClient) TerminalOutput(_ context.Context, _ acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, errors.New(errTerminalAccessUnavailable)
}

func (*wikiChatClient) ReleaseTerminal(_ context.Context, _ acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, errors.New(errTerminalAccessUnavailable)
}

func (*wikiChatClient) WaitForTerminalExit(_ context.Context, _ acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, errors.New(errTerminalAccessUnavailable)
}

// spawnInstance starts an ACP agent process for a page, performs the handshake,
// creates a session with wiki MCP tools, and starts the message bridge.
// Must be called with d.mu held.
func (d *poolDaemon) spawnInstance(ctx context.Context, page string) (*instanceEntry, error) {
	ctx, cancel := context.WithCancel(ctx)

	conn, chatClient, err := d.startAgentProcess(ctx, page)
	if err != nil {
		cancel()
		return nil, err
	}

	sess, err := d.initializeACPSession(ctx, conn, page)
	if err != nil {
		cancel()
		return nil, err
	}

	now := time.Now()
	entry := &instanceEntry{
		page:           page,
		conn:           conn,
		sessionID:      sess.SessionId,
		cancel:         cancel,
		lastActive:     now,
		createdAt:      now,
		stateChangedAt: now,
		state:          StateInitializing,
	}
	slog.Info("state transition", logKeyPage, page, "from_state", "Spawning", "to_state", "Initializing", logKeyAction, "state_change")

	chatClient.entry = entry
	go d.runMessageBridge(ctx, entry, chatClient)

	return entry, nil
}

// startAgentProcess spawns the ACP agent binary (optionally via systemd), performs
// the ACP handshake, and returns the ready connection and chat client.
func (d *poolDaemon) startAgentProcess(ctx context.Context, page string) (*acp.ClientSideConnection, *wikiChatClient, error) {
	cmd := d.buildAgentCmd(ctx, page)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start agent for page %q: %w", page, err)
	}

	go func() {
		waitErr := cmd.Wait()
		if waitErr != nil && ctx.Err() == nil {
			slog.Warn("agent process exited unexpectedly", logKeyPage, page, logKeyError, waitErr, logKeyAction, "agent_exit")
		} else {
			slog.Info("agent process exited cleanly", logKeyPage, page, logKeyAction, "agent_exit")
		}
		d.markInstanceDead(page)
	}()

	chatClient := newWikiChatClient(page, d.wikiURL)
	conn := acp.NewClientSideConnection(chatClient, stdinPipe, stdoutPipe)

	_, err = conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{
				ReadTextFile:  false,
				WriteTextFile: false,
			},
			Terminal: false,
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("ACP handshake failed for page %q: %w", page, err)
	}

	return conn, chatClient, nil
}

// buildAgentCmd constructs the exec.Cmd for the agent process, using systemd-run
// when systemd integration is enabled.
func (d *poolDaemon) buildAgentCmd(ctx context.Context, page string) *exec.Cmd {
	if d.useSystemd {
		unitName := "wiki-chat-" + sanitizeUnitName(page)
		_ = exec.Command("systemctl", "--user", "stop", unitName+".scope").Run()
		return exec.CommandContext(ctx, "systemd-run",
			"--user",
			"--unit="+unitName,
			"--scope",
			d.agentPath,
		)
	}

	cmd := exec.CommandContext(ctx, d.agentPath)
	cmd.Stderr = newPrefixWriter(os.Stderr, page)
	return cmd
}

// initializeACPSession creates an ACP session with the wiki MCP server configured.
func (d *poolDaemon) initializeACPSession(ctx context.Context, conn *acp.ClientSideConnection, page string) (acp.NewSessionResponse, error) {
	wikiCLIBin, execErr := os.Executable()
	if execErr != nil {
		return acp.NewSessionResponse{}, fmt.Errorf("failed to find wiki-cli binary: %w", execErr)
	}

	cwd, _ := os.Getwd()

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
		return acp.NewSessionResponse{}, fmt.Errorf("ACP session creation failed for page %q: %w", page, err)
	}

	return sess, nil
}

// runMessageBridge subscribes to user messages and forwards them as ACP prompts.
// Page context is prepended to the first user message rather than sent as a
// separate prompt, ensuring the bridge is listening before any prompt is sent.
func (d *poolDaemon) runMessageBridge(ctx context.Context, entry *instanceEntry, chatClient *wikiChatClient) {
	page := entry.page

	if err := entry.setState(StateBridgeConnecting); err != nil {
		slog.Warn(logMsgStateTransitionError, logKeyPage, page, logKeyError, err)
	}

	slog.Info("bridge: fetching page context", logKeyPage, page)
	// Fetch page context (but don't send it as a prompt yet — prepend to first message)
	chatClient.mu.Lock()
	chatClient.pageContext = d.fetchPageContext(ctx, page)
	chatClient.mu.Unlock()
	slog.Info("bridge: page context fetched, subscribing to messages", logKeyPage, page)

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
		slog.Warn("message bridge error, reconnecting", logKeyPage, page, logKeyError, err, "delay_ms", delayMs, logKeyAction, "reconnect")

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
func (*poolDaemon) bridgeMessages(ctx context.Context, entry *instanceEntry, wikiClient pageMessageSource, chatClient *wikiChatClient) error {
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

	slog.Info("bridge: message stream connected, setting up cancellation", logKeyPage, entry.page)

	cancelChan := subscribeCancellations(ctx, wikiClient, entry.page)

	slog.Info("message bridge connected", logKeyPage, entry.page, logKeyAction, "connected")

	if err := entry.setState(StateIdle); err != nil {
		slog.Warn(logMsgStateTransitionError, logKeyPage, entry.page, logKeyError, err)
	}

	for stream.Receive() {
		msg := stream.Msg()

		// Only forward user messages
		if msg.Sender != apiv1.Sender_USER {
			slog.Info("bridge: skipping non-user message", logKeyPage, entry.page, "sender", msg.Sender.String())
			continue
		}

		forwardUserMessage(ctx, entry, chatClient, cancelChan, msg)
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

// subscribeCancellations starts a background goroutine that listens for cancellation
// signals on the given page and forwards them to the returned channel.
func subscribeCancellations(ctx context.Context, wikiClient pageMessageSource, page string) <-chan struct{} {
	cancelChan := make(chan struct{}, 1)
	go func() {
		cancelStream, cancelErr := wikiClient.SubscribePageCancellations(ctx, connect.NewRequest(&apiv1.SubscribePageCancellationsRequest{
			Page: page,
		}))
		if cancelErr != nil {
			slog.Warn("failed to subscribe to cancellations", logKeyPage, page, logKeyError, cancelErr)
			return
		}
		defer func() { _ = cancelStream.Close() }()
		for cancelStream.Receive() {
			select {
			case cancelChan <- struct{}{}:
			default:
			}
		}
	}()
	return cancelChan
}

// forwardUserMessage processes a single user message from the chat stream: prepares
// context, sends it as an ACP prompt, and manages heartbeats and cancellation.
func forwardUserMessage(ctx context.Context, entry *instanceEntry, chatClient *wikiChatClient, cancelChan <-chan struct{}, msg *apiv1.ChatMessage) {
	slog.Info("bridge: received user message", logKeyPage, entry.page, "message_id", msg.Id, "content_preview", truncate(msg.Content, truncateLimitForBridge))
	entry.touch()

	chatClient.beginTurn(msg.Id)
	defer chatClient.endTurn()

	promptCtx, promptCancel := context.WithCancel(ctx)
	defer promptCancel()

	go heartbeatWhilePrompting(promptCtx, entry)

	go listenForCancelSignal(promptCtx, promptCancel, cancelChan, entry.page)

	promptText := buildPromptText(chatClient, msg.SenderName, msg.Content)

	if err := entry.setState(StatePrompting); err != nil {
		slog.Warn(logMsgStateTransitionError, logKeyPage, entry.page, logKeyError, err)
	}

	slog.Info("bridge: sending prompt", logKeyPage, entry.page, "prompt_length", len(promptText))
	_, promptErr := entry.conn.Prompt(promptCtx, acp.PromptRequest{
		SessionId: entry.sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(promptText)},
	})

	if promptErr != nil {
		if promptCtx.Err() != nil && ctx.Err() == nil {
			slog.Info("prompt cancelled", logKeyPage, entry.page, logKeyAction, "cancelled")
		} else {
			slog.Error("failed to send prompt to agent", logKeyPage, entry.page, logKeyError, promptErr)
		}
	}

	if err := entry.setState(StateIdle); err != nil {
		slog.Warn(logMsgStateTransitionError, logKeyPage, entry.page, logKeyError, err)
	}
}

// heartbeatWhilePrompting periodically touches the instance entry to prevent the
// idle reaper from killing it during long-running prompts.
func heartbeatWhilePrompting(ctx context.Context, entry *instanceEntry) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			entry.touch()
			slog.Info("heartbeat: touched lastActive", logKeyPage, entry.page)
		case <-ctx.Done():
			slog.Info("heartbeat: stopped, prompt done", logKeyPage, entry.page)
			return
		}
	}
}

// listenForCancelSignal waits for either a cancel signal from the user or for
// the prompt context to finish, cancelling the prompt if requested.
func listenForCancelSignal(ctx context.Context, cancel context.CancelFunc, cancelChan <-chan struct{}, page string) {
	select {
	case <-cancelChan:
		slog.Info("cancelling prompt on user request", logKeyPage, page, logKeyAction, "cancel")
		cancel()
	case <-ctx.Done():
	}
}

// buildPromptText prepends page context to the user message content if available,
// consuming the context so it is only prepended once. If senderName is non-empty,
// the message is prefixed with "[senderName]: " to identify the speaker in group chats.
func buildPromptText(chatClient *wikiChatClient, senderName string, messageContent string) string {
	chatClient.mu.Lock()
	pageContext := chatClient.pageContext
	chatClient.pageContext = "" // only prepend once
	chatClient.mu.Unlock()

	formattedMessage := messageContent
	if senderName != "" {
		formattedMessage = fmt.Sprintf("[%s]: %s", senderName, messageContent)
	}

	if pageContext == "" {
		return formattedMessage
	}

	return pageContext + "\n\n---\n\nUser message: " + formattedMessage
}

// chatPreamble is prepended to the first prompt to establish the interactive chat context.
const chatPreamble = `You are in an INTERACTIVE CHAT session with one or more users on a wiki page. This is a conversation, not a coding task.

CRITICAL: Respond quickly and conversationally. Do NOT explore the codebase, run startup hooks, or do extensive initialization. The user is waiting for your reply in a chat window.

Rules:
- Keep responses concise and conversational
- Use the wiki MCP tools only when the user asks you to read or edit pages
- Multiple users may be chatting on the same page simultaneously
- Each message is prefixed with the sender's name in the format "[Name]: message"
- Address users by name in your responses when their identity is known

## MANDATORY: Update Your Memory After EVERY Response

You have NO chat history between sessions. The ONLY way to maintain continuity is the
[ai_agent_chat_context] section in this page's frontmatter. Without it, you start every
session completely blank.

REQUIREMENT: Before you finish responding to EVERY message, you MUST call the
MergeFrontmatter MCP tool to update [ai_agent_chat_context]. This is not optional.
Do it every single turn, even for short exchanges.

Update it with a structured object containing these fields:
- "last_conversation_summary": 1-2 sentence summary of this exchange
- "user_goals": array of goals or preferences the user has mentioned (accumulate across sessions)
- "pending_items": array of action items that need follow-up
- "key_context": important facts, decisions, or preferences to remember
- "last_updated": current date/time

Example MergeFrontmatter call:
` + "```" + `json
{
  "ai_agent_chat_context": {
    "last_conversation_summary": "User asked me to help reorganize the project roadmap section. I restructured it into Q1/Q2 milestones.",
    "user_goals": ["reorganize roadmap", "keep page concise"],
    "pending_items": ["add Q3 milestones when user provides them"],
    "key_context": "User prefers bullet points over tables for roadmap items. This page tracks the Alpha project.",
    "last_updated": "2025-01-15T10:30:00Z"
  }
}
` + "```" + `

IMPORTANT: Merge new information with existing context — do not discard previous entries
from user_goals or key_context unless they are no longer relevant. Accumulate knowledge
across sessions.

## Wiki Syntax Help

When users ask about wiki features, formatting, or syntax (e.g., macros, special markdown,
collapsible headers, checklists, blog macros, or any other wiki-specific feature):

1. Search the wiki itself for help pages using the SearchPages tool with terms like "help"
   combined with the feature name (e.g., "help collapsible", "help checklist").
2. Read the relevant help page using the ReadPage tool (e.g., "help/collapsible-headers",
   "help/checklists").
3. Answer based on the wiki's own help pages — they are the authoritative source for
   user-facing features and syntax.

Do NOT search the codebase for this. The wiki's own help pages contain accurate,
user-facing documentation for all wiki-specific syntax and features.

`

// fetchPageContext retrieves the page's ai_agent_chat_context frontmatter and returns
// it as a text string to prepend to the first user message.
func (d *poolDaemon) fetchPageContext(ctx context.Context, page string) string {
	httpClient := &http.Client{}
	fmClient := apiv1connect.NewFrontmatterClient(httpClient, d.wikiURL)

	fmResp, err := fmClient.GetFrontmatter(ctx, connect.NewRequest(&apiv1.GetFrontmatterRequest{Page: page}))
	if err != nil {
		slog.Error("failed to fetch frontmatter", logKeyPage, page, logKeyError, err)
		return chatPreamble + fmt.Sprintf(
			"You are the assistant for wiki page '%s'. Failed to fetch page context: %v. Do NOT attempt to create or modify the [ai_agent_chat_context] section.",
			page, err,
		)
	}

	fm := fmResp.Msg.Frontmatter.AsMap()
	agentContext, ok := fm["ai_agent_chat_context"]
	if !ok {
		return chatPreamble + fmt.Sprintf(
			"You are the assistant for wiki page '%s'. No [ai_agent_chat_context] exists yet — this is your first session on this page.\n\n"+
			"You MUST create it now by calling MergeFrontmatter after your first response. Use the structured format described above "+
			"(last_conversation_summary, user_goals, pending_items, key_context, last_updated).",
			page,
		)
	}

	contextJSON, err := json.MarshalIndent(agentContext, "", "  ")
	if err != nil {
		slog.Error("failed to marshal ai_agent_chat_context", logKeyPage, page, logKeyError, err)
		return chatPreamble + fmt.Sprintf("You are the assistant for wiki page '%s'.", page)
	}

	return chatPreamble + fmt.Sprintf(
		"You are the assistant for wiki page '%s'.\n\n"+
			"## Your Restored Memory (from [ai_agent_chat_context])\n\n"+
			"This is everything you remember from previous sessions. Review it before responding:\n\n"+
			"```json\n%s\n```\n\n"+
			"REMINDER: You MUST update this memory via MergeFrontmatter before finishing your response. "+
			"Merge new information with the existing data above — do not overwrite fields, accumulate them.",
		page, string(contextJSON),
	)
}

// stopInstanceLocked stops an instance. Must be called with d.mu held.
func (d *poolDaemon) stopInstanceLocked(page string) {
	entry, ok := d.instances[page]
	if !ok {
		return
	}

	entry.mu.Lock()
	if err := entry.setStateLocked(StateStopping); err != nil {
		slog.Warn(logMsgStateTransitionError, logKeyPage, page, logKeyError, err)
	}
	entry.mu.Unlock()

	entry.cancel()

	entry.mu.Lock()
	if err := entry.setStateLocked(StateDead); err != nil {
		slog.Warn(logMsgStateTransitionError, logKeyPage, page, logKeyError, err)
	}
	entry.mu.Unlock()

	delete(d.instances, page)
}

// markInstanceDead transitions an instance to Dead and removes it from the pool.
// This is called when the agent process exits unexpectedly (signal, crash, or clean exit).
// Safe to call without d.mu held.
func (d *poolDaemon) markInstanceDead(page string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, ok := d.instances[page]
	if !ok {
		return
	}

	entry.cancel()

	if err := entry.setState(StateDead); err != nil {
		slog.Warn(logMsgStateTransitionError, logKeyPage, page, logKeyError, err)
	}

	delete(d.instances, page)
	slog.Info("instance removed from pool after process exit", logKeyPage, page, logKeyAction, "reaped")
}

// stopAll stops all running instances.
func (d *poolDaemon) stopAll() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for page := range d.instances {
		d.stopInstanceLocked(page)
	}
	slog.Info("all instances stopped", logKeyAction, "shutdown")
}

// shouldReap returns a non-empty string describing why the instance should be reaped,
// or an empty string if it should be kept.
func (d *poolDaemon) shouldReap(entry *instanceEntry) string {
	entry.mu.Lock()
	defer entry.mu.Unlock()

	state := entry.state

	// Clean up dead instances that were not removed from the map properly.
	if state == StateDead {
		return "already dead"
	}

	// Safety net: forcibly reclaim any instance that exceeds the maximum lifetime,
	// regardless of state. This prevents instances stuck in Spawning, BridgeConnecting,
	// Prompting, or other non-terminal states from living forever.
	if d.maxInstanceAge > 0 && time.Since(entry.createdAt) > d.maxInstanceAge {
		return fmt.Sprintf("exceeded max age %s (age %s, state=%s)",
			d.maxInstanceAge, time.Since(entry.createdAt).Round(time.Second), state)
	}

	// PermissionPending-specific timeout: the heartbeat keeps lastActive fresh during
	// prompts, so the idle check alone cannot reap a stuck permission request.
	if state == StatePermissionPending && d.permissionPendingTimeout > 0 {
		if time.Since(entry.stateChangedAt) > d.permissionPendingTimeout {
			return fmt.Sprintf("PermissionPending for %s exceeds timeout %s",
				time.Since(entry.stateChangedAt).Round(time.Second), d.permissionPendingTimeout)
		}
	}

	// Idle timeout: applies to all live states. The heartbeat prevents false positives
	// during active prompts by touching lastActive every minute.
	if time.Since(entry.lastActive) > d.idleTimeout {
		return fmt.Sprintf("idle %s (state=%s)", time.Since(entry.lastActive).Round(time.Second), state)
	}

	return ""
}

// reapIdleInstances periodically stops stale instances based on idle timeout,
// max instance age, PermissionPending timeout, and dead-but-unremoved entries.
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
				if reason := d.shouldReap(entry); reason != "" {
					slog.Info("reaping instance", logKeyPage, page, "reason", reason, logKeyAction, "reaped")
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
