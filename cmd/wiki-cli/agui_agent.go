package main

// agui_agent.go implements the AG-UI pool backend: an acpAgent that drives a
// remote AG-UI endpoint (HTTP+SSE) instead of a local ACP process.
//
// Design (issue #1275 §6.2):
//   - agUIAgent implements the acpAgent interface so it fits into instanceEntry.conn
//     without any changes to the ACP path.
//   - Initialize and NewSession are no-ops (no process handshake needed).
//   - Prompt POSTs a RunAgentInput to the endpoint, consumes the SSE stream,
//     translates AG-UI events into wikiChatClient callbacks, and handles
//     interrupt/resume cycles (first-answer-wins, per §6.4.2).
//   - The threadId is pinned to the wiki page name so the session is owned and
//     governed by the wiki (§6.4 — wiki owns the session).
//
// Configuration (agUIBackendConfig) is the pool-side companion to the server's
// aguiAgentEntry: same agent id, but enriched with the bearer-token env-var
// reference for credentials.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	acp "github.com/coder/acp-go-sdk"
	"github.com/google/uuid"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"connectrpc.com/connect"
)

// ─── pool-side config ───────────────────────────────────────────────────────

// agUIBackendConfig is the pool's per-agent configuration for one remote AG-UI
// endpoint. The wiki server's companion config (server/agent_config.go) holds
// the public display metadata; this struct adds the credential reference.
type agUIBackendConfig struct {
	// ID is the stable inventory id (must match the server-side aguiAgentEntry).
	ID string `json:"id"`
	// DisplayName is the human-readable name shown in the selector.
	DisplayName string `json:"display_name"`
	// Description is the short capability summary shown in the selector.
	Description string `json:"description"`
	// Endpoint is the AG-UI run URL (POST here to start a run).
	Endpoint string `json:"endpoint"`
	// TokenEnv is the name of the environment variable that holds the bearer
	// token. Leave empty for unauthenticated endpoints.
	TokenEnv string `json:"token_env"`
}

// agUIBackendsFileFormat is the on-disk shape of the pool's AG-UI agents
// configuration file (--agui-agents-config).
type agUIBackendsFileFormat struct {
	Agents []agUIBackendConfig `json:"agents"`
}

// loadAGUIBackendConfigs reads the pool's AG-UI agent config from path.
// Missing file, empty path, or parse failures yield an empty slice rather
// than an error — the pool still serves ACP pages normally.
func loadAGUIBackendConfigs(path string) []agUIBackendConfig {
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("agui: cannot read backend config; no AG-UI agents will be loaded", "path", path, logKeyError, err)
		return nil
	}
	var cfg agUIBackendsFileFormat
	if err := json.Unmarshal(raw, &cfg); err != nil {
		slog.Warn("agui: malformed backend config; no AG-UI agents will be loaded", "path", path, logKeyError, err)
		return nil
	}
	valid := cfg.Agents[:0]
	for _, a := range cfg.Agents {
		if a.ID == "" || a.Endpoint == "" {
			slog.Warn("agui: skipping backend config entry with missing id or endpoint", "entry", a.ID)
			continue
		}
		valid = append(valid, a)
	}
	return valid
}

// ─── AG-UI wire types (SSE codec) ───────────────────────────────────────────

// agUIEventType enumerates the AG-UI event types we care about.
const (
	agUITypeRunStarted          = "RUN_STARTED"
	agUITypeRunFinished         = "RUN_FINISHED"
	agUITypeRunError            = "RUN_ERROR"
	agUITypeTextMessageStart    = "TEXT_MESSAGE_START"
	agUITypeTextMessageContent  = "TEXT_MESSAGE_CONTENT"
	agUITypeTextMessageEnd      = "TEXT_MESSAGE_END"
	agUITypeTextMessageChunk    = "TEXT_MESSAGE_CHUNK"
	agUITypeToolCallStart       = "TOOL_CALL_START"
	agUITypeToolCallArgs        = "TOOL_CALL_ARGS"
	agUITypeToolCallEnd         = "TOOL_CALL_END"
	agUITypeToolCallChunk       = "TOOL_CALL_CHUNK"
	agUITypeReasoningStart      = "REASONING_START"
	agUITypeReasoningEnd        = "REASONING_END"
	agUITypeMsgReasoningStart   = "REASONING_MESSAGE_START"
	agUITypeMsgReasoningContent = "REASONING_MESSAGE_CONTENT"
	agUITypeMsgReasoningEnd     = "REASONING_MESSAGE_END"
	agUITypeMsgReasoningChunk   = "REASONING_MESSAGE_CHUNK"
	agUITypeStepStarted         = "STEP_STARTED"
	agUITypeStepFinished        = "STEP_FINISHED"
	agUITypeStateSnapshot       = "STATE_SNAPSHOT"
	agUITypeStateDelta          = "STATE_DELTA"
	agUITypeMessagesSnapshot    = "MESSAGES_SNAPSHOT"
)

// AG-UI run outcome status values.
const (
	agUIStatusSuccess   = "success"
	agUIStatusCancelled = "cancelled"
	agUIStatusInterrupt = "interrupt"
)

// SSE field key constants.
const (
	sseKeyMessageID   = "messageId"
	sseKeyToolCallID  = "toolCallId"
	sseKeyToolCallName = "toolCallName"
	sseKeyDelta       = "delta"
	sseKeyRole        = "role"
	sseKeyStatus      = "status"
	sseKeyType        = "type"
	sseKeyMessage     = "message"
	sseKeyCode        = "code"
	sseKeyInterrupts  = "interrupts"
)

// agUIRunInput is the request body for starting a run.
type agUIRunInput struct {
	ThreadID string            `json:"threadId"`
	RunID    string            `json:"runId"`
	Messages []agUIMessage     `json:"messages"`
	Context  []agUIContextItem `json:"context,omitempty"`
	State    any               `json:"state,omitempty"`
	Resume   []agUIResumeEntry `json:"resume,omitempty"`
}

// agUIMessage is a single conversation message in a RunAgentInput.
type agUIMessage struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// agUIContextItem is a context entry for the model (description + value pair).
type agUIContextItem struct {
	Description string `json:"description"`
	Value       string `json:"value"`
}

// agUIResumeEntry carries the answer to one interrupt in a resumed run.
type agUIResumeEntry struct {
	InterruptID string `json:"interruptId"`
	Response    any    `json:"response"`
}

// agUIInterrupt is one pending interrupt in a RUN_FINISHED interrupt outcome.
type agUIInterrupt struct {
	ID      string `json:"id"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// agUIRunOutcome accumulates the result of one SSE stream.
type agUIRunOutcome struct {
	// Status is "success", "interrupt", "cancelled", or "" (treat as success).
	Status string
	// Interrupts is non-nil only when Status == "interrupt".
	Interrupts []agUIInterrupt
}

// agUIRunError signals that the remote producer emitted RUN_ERROR.
type agUIRunError struct {
	Message string
	Code    string
}

func (e *agUIRunError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("AG-UI run error [%s]: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("AG-UI run error: %s", e.Message)
}

// ─── agUIAgent ───────────────────────────────────────────────────────────────

// agUIAgent implements acpAgent using a remote AG-UI HTTP+SSE endpoint.
// It satisfies the same interface seam as *acp.ClientSideConnection so it
// fits into instanceEntry.conn without changing the ACP path.
type agUIAgent struct {
	cfg        agUIBackendConfig
	chatClient *wikiChatClient
	threadID   string // pinned to the wiki page name for wiki-owned sessions
	token      string // bearer token loaded from env at construction time
	httpClient *http.Client
}

// newAgUIAgent constructs an agUIAgent. envGetter is injectable for tests
// (pass os.Getenv in production).
func newAgUIAgent(cfg agUIBackendConfig, chatClient *wikiChatClient, threadID string, envGetter func(string) string) *agUIAgent {
	token := ""
	if cfg.TokenEnv != "" && envGetter != nil {
		token = envGetter(cfg.TokenEnv)
	}
	return &agUIAgent{
		cfg:        cfg,
		chatClient: chatClient,
		threadID:   threadID,
		token:      token,
		httpClient: &http.Client{},
	}
}

// Initialize is a no-op: AG-UI endpoints require no handshake.
func (*agUIAgent) Initialize(_ context.Context, _ acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersionNumber}, nil
}

// NewSession is a no-op: the threadID is fixed at construction time.
// Returns the threadID as the session id so the instanceEntry can log it.
func (a *agUIAgent) NewSession(_ context.Context, _ acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: acp.SessionId(a.threadID)}, nil
}

// Prompt sends the user message to the remote AG-UI agent and processes the
// response stream. It blocks until the run completes, handling interrupt/resume
// cycles (approvals) inline — first answer wins per §6.4.2.
//
// The prompt text is taken directly from the first text content block in req,
// which already contains the "[Name]: " sender prefix and any first-message
// context injected by buildPromptText.
func (a *agUIAgent) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	// Extract message text from the ACP content blocks (always a single TextBlock).
	var msgText strings.Builder
	for _, block := range req.Prompt {
		if block.Text != nil {
			msgText.WriteString(block.Text.Text)
		}
	}

	var resume []agUIResumeEntry

	for {
		runID := newAgUIRunID()
		input := a.buildRunInput(runID, msgText.String(), resume)

		outcome, err := a.executeRun(ctx, input)
		if err != nil {
			return acp.PromptResponse{}, err
		}

		switch outcome.Status {
		case "", agUIStatusSuccess, agUIStatusCancelled:
			return acp.PromptResponse{}, nil
		case agUIStatusInterrupt:
			resume, err = a.handleInterrupts(ctx, outcome.Interrupts)
			if err != nil {
				return acp.PromptResponse{}, err
			}
			// Resumed runs carry no new user message — only the resume answers.
			msgText.Reset()
		default:
			// Unknown outcome status: treat as success.
			slog.Warn("agui: unknown run outcome status, treating as success", "status", outcome.Status, logKeyPage, a.threadID)
			return acp.PromptResponse{}, nil
		}
	}
}

// ─── Run construction ────────────────────────────────────────────────────────

// buildRunInput assembles a RunAgentInput for one prompt or resume cycle.
func (a *agUIAgent) buildRunInput(runID, msgText string, resume []agUIResumeEntry) agUIRunInput {
	input := agUIRunInput{
		ThreadID: a.threadID,
		RunID:    runID,
		Context: []agUIContextItem{
			{
				Description: "Multiplayer chat protocol",
				Value: "You are in an interactive multiplayer chat session on a wiki page. " +
					"Each user message is prefixed with [Name]: to identify the speaker. " +
					"Multiple distinct humans may participate. Address users by name, " +
					"attribute preferences and requests to the correct person, and never " +
					"conflate one user's preferences with another's.",
			},
		},
	}

	if msgText != "" {
		input.Messages = []agUIMessage{
			{
				ID:      newAgUIRunID(),
				Role:    "user",
				Content: msgText,
			},
		}
	}

	if len(resume) > 0 {
		input.Resume = resume
	}

	return input
}

// newAgUIRunID generates a UUID for use as a run id or message id.
func newAgUIRunID() string {
	return uuid.New().String()
}

// ─── SSE stream execution ────────────────────────────────────────────────────

// executeRun POSTs input to the AG-UI endpoint and processes the SSE stream
// until RUN_FINISHED or RUN_ERROR. AG-UI events are translated into
// wikiChatClient callbacks (same downstream path as ACP).
func (a *agUIAgent) executeRun(ctx context.Context, input agUIRunInput) (*agUIRunOutcome, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("agui: marshal run input: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("agui: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return &agUIRunOutcome{Status: "cancelled"}, nil
		}
		return nil, fmt.Errorf("agui: HTTP request to %s: %w", a.cfg.Endpoint, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agui: endpoint returned HTTP %d", resp.StatusCode)
	}

	return a.consumeSSEStream(ctx, resp.Body)
}

// sseStreamState holds mutable tracking state for an active SSE stream.
type sseStreamState struct {
	// openMessages tracks in-flight text message accumulation by messageId.
	openMessages map[string]*sseOpenMsg
	// openToolCalls tracks active tool calls by toolCallId → tool name.
	openToolCalls map[string]string
}

// sseOpenMsg accumulates one in-progress text message during streaming.
type sseOpenMsg struct {
	role string
	buf  strings.Builder
}

// consumeSSEStream reads AG-UI events from the SSE body and returns the run
// outcome. Returns early on context cancellation (cancelled outcome).
func (a *agUIAgent) consumeSSEStream(ctx context.Context, body io.Reader) (*agUIRunOutcome, error) {
	state := &sseStreamState{
		openMessages:  make(map[string]*sseOpenMsg),
		openToolCalls: make(map[string]string),
	}

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return &agUIRunOutcome{Status: agUIStatusCancelled}, nil
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "" || data == "[DONE]" {
			continue
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			slog.Warn("agui: skipping malformed SSE event", logKeyPage, a.threadID, logKeyError, err)
			continue
		}

		eventType := jsonString(raw, sseKeyType)
		if eventType == "" {
			continue
		}

		outcome, err := a.dispatchSSEEvent(state, eventType, raw)
		if err != nil {
			return nil, err
		}
		if outcome != nil {
			return outcome, nil
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return &agUIRunOutcome{Status: agUIStatusCancelled}, nil
		}
		return nil, fmt.Errorf("agui: SSE stream read error: %w", err)
	}

	// Stream ended without a terminal event — truncated run.
	slog.Warn("agui: SSE stream closed without terminal event (truncated run)", logKeyPage, a.threadID)
	return &agUIRunOutcome{Status: agUIStatusSuccess}, nil
}

// dispatchSSEEvent routes a parsed SSE event to the appropriate handler.
// Returns a non-nil outcome on terminal events (RUN_FINISHED, RUN_ERROR).
func (a *agUIAgent) dispatchSSEEvent(state *sseStreamState, eventType string, raw map[string]json.RawMessage) (*agUIRunOutcome, error) {
	switch eventType {
	case agUITypeRunStarted:
		// No-op: turn status already sent by forwardUserMessage before Prompt.

	case agUITypeTextMessageStart, agUITypeTextMessageContent,
		agUITypeTextMessageEnd, agUITypeTextMessageChunk:
		a.handleSSETextEvent(state, eventType, raw)

	case agUITypeToolCallStart, agUITypeToolCallArgs,
		agUITypeToolCallEnd, agUITypeToolCallChunk:
		a.handleSSEToolEvent(state, eventType, raw)

	case agUITypeMsgReasoningContent, agUITypeMsgReasoningChunk:
		a.handleSSEReasoningEvent(raw)

	case agUITypeRunFinished:
		return a.parseRunFinished(raw), nil

	case agUITypeRunError:
		return nil, &agUIRunError{
			Message: jsonString(raw, sseKeyMessage),
			Code:    jsonString(raw, sseKeyCode),
		}

	// Gracefully ignore events not handled in P2.
	case agUITypeStepStarted, agUITypeStepFinished,
		agUITypeReasoningStart, agUITypeReasoningEnd,
		agUITypeMsgReasoningStart, agUITypeMsgReasoningEnd,
		agUITypeStateSnapshot, agUITypeStateDelta,
		agUITypeMessagesSnapshot:

	default:
		slog.Debug("agui: dropping unknown event type", "event_type", eventType, logKeyPage, a.threadID)
	}
	return nil, nil
}

// handleSSETextEvent processes TEXT_MESSAGE_* events.
func (a *agUIAgent) handleSSETextEvent(state *sseStreamState, eventType string, raw map[string]json.RawMessage) {
	switch eventType {
	case agUITypeTextMessageStart:
		msgID := jsonString(raw, sseKeyMessageID)
		if msgID == "" {
			return
		}
		if _, already := state.openMessages[msgID]; !already {
			state.openMessages[msgID] = &sseOpenMsg{role: jsonStringDefault(raw, sseKeyRole, "assistant")}
		}

	case agUITypeTextMessageContent:
		msgID := jsonString(raw, sseKeyMessageID)
		delta := jsonString(raw, sseKeyDelta)
		if msgID == "" || delta == "" {
			return
		}
		if m, ok := state.openMessages[msgID]; ok {
			m.buf.WriteString(delta)
		}
		a.streamTextDelta(delta)

	case agUITypeTextMessageEnd:
		delete(state.openMessages, jsonString(raw, sseKeyMessageID))

	case agUITypeTextMessageChunk:
		msgID := jsonString(raw, sseKeyMessageID)
		if msgID == "" {
			return
		}
		if _, already := state.openMessages[msgID]; !already {
			state.openMessages[msgID] = &sseOpenMsg{role: "assistant"}
		}
		if delta := jsonString(raw, sseKeyDelta); delta != "" {
			state.openMessages[msgID].buf.WriteString(delta)
			a.streamTextDelta(delta)
		}
		delete(state.openMessages, msgID)

	default:
		// Unknown TEXT_MESSAGE_* subtype — no-op.
	}
}

// handleSSEToolEvent processes TOOL_CALL_* events.
func (a *agUIAgent) handleSSEToolEvent(state *sseStreamState, eventType string, raw map[string]json.RawMessage) {
	switch eventType {
	case agUITypeToolCallStart:
		toolCallID := jsonString(raw, sseKeyToolCallID)
		if toolCallID == "" {
			return
		}
		toolCallName := jsonString(raw, sseKeyToolCallName)
		state.openToolCalls[toolCallID] = toolCallName
		status := acp.ToolCallStatusInProgress
		a.chatClient.handleToolCall(&acp.SessionUpdateToolCall{
			ToolCallId: acp.ToolCallId(toolCallID),
			Title:      toolCallName,
			Status:     status,
		})

	case agUITypeToolCallArgs:
		// Args delta — not surfaced separately in P2.

	case agUITypeToolCallEnd:
		toolCallID := jsonString(raw, sseKeyToolCallID)
		name, ok := state.openToolCalls[toolCallID]
		if !ok {
			return
		}
		delete(state.openToolCalls, toolCallID)
		status := acp.ToolCallStatusCompleted
		a.chatClient.handleToolCallUpdate(&acp.SessionToolCallUpdate{
			ToolCallId: acp.ToolCallId(toolCallID),
			Title:      &name,
			Status:     &status,
		})

	case agUITypeToolCallChunk:
		toolCallID := jsonString(raw, sseKeyToolCallID)
		if toolCallID == "" {
			return
		}
		if _, already := state.openToolCalls[toolCallID]; !already {
			toolCallName := jsonString(raw, sseKeyToolCallName)
			state.openToolCalls[toolCallID] = toolCallName
			status := acp.ToolCallStatusInProgress
			a.chatClient.handleToolCall(&acp.SessionUpdateToolCall{
				ToolCallId: acp.ToolCallId(toolCallID),
				Title:      toolCallName,
				Status:     status,
			})
		}

	default:
		// Unknown TOOL_CALL_* subtype — no-op.
	}
}

// handleSSEReasoningEvent processes REASONING_MESSAGE_CONTENT and _CHUNK events.
func (a *agUIAgent) handleSSEReasoningEvent(raw map[string]json.RawMessage) {
	delta := jsonString(raw, sseKeyDelta)
	if delta == "" {
		return
	}
	a.chatClient.handleThought(&acp.SessionUpdateAgentThoughtChunk{
		Content: acp.ContentBlock{
			Text: &acp.ContentBlockText{Text: delta, Type: "text"},
		},
	})
}

// streamTextDelta appends delta to the chat reply via wikiChatClient.
func (a *agUIAgent) streamTextDelta(delta string) {
	_ = a.chatClient.handleAgentMessage(&acp.SessionUpdateAgentMessageChunk{
		Content: acp.ContentBlock{
			Text: &acp.ContentBlockText{Text: delta, Type: "text"},
		},
	})
}

// parseRunFinished extracts the outcome from a RUN_FINISHED event's JSON.
func (a *agUIAgent) parseRunFinished(raw map[string]json.RawMessage) *agUIRunOutcome {
	// AG-UI RUN_FINISHED carries a status string ("success"|"interrupt"|"cancelled")
	// and, for interrupt outcomes, an array of Interrupt objects.
	statusStr := jsonString(raw, sseKeyStatus)
	switch statusStr {
	case "", agUIStatusSuccess:
		return &agUIRunOutcome{Status: agUIStatusSuccess}
	case agUIStatusCancelled:
		return &agUIRunOutcome{Status: agUIStatusCancelled}
	case agUIStatusInterrupt:
		var interrupts []agUIInterrupt
		if irRaw, ok := raw[sseKeyInterrupts]; ok {
			_ = json.Unmarshal(irRaw, &interrupts)
		}
		return &agUIRunOutcome{Status: agUIStatusInterrupt, Interrupts: interrupts}
	default:
		slog.Warn("agui: unrecognised RUN_FINISHED status, treating as success", "status", statusStr, logKeyPage, a.threadID)
		return &agUIRunOutcome{Status: agUIStatusSuccess}
	}
}

// ─── Interrupt / approval relay ──────────────────────────────────────────────

// handleInterrupts relays each AG-UI interrupt to the wiki permission UI (using
// the existing RequestPermissionFromUser RPC) and returns the resume entries
// needed for the next run. First-answer-wins semantics are provided by the wiki
// server's chatbuffer (§6.4.2).
func (a *agUIAgent) handleInterrupts(ctx context.Context, interrupts []agUIInterrupt) ([]agUIResumeEntry, error) {
	resume := make([]agUIResumeEntry, 0, len(interrupts))

	for _, interrupt := range interrupts {
		title := interrupt.Reason
		if interrupt.Message != "" {
			title = interrupt.Message
		}
		if title == "" {
			title = "Approval required"
		}

		// Reuse the existing ACP permission relay machinery (state transitions,
		// timeout, broadcast) by constructing a synthetic ACP permission request.
		titleCopy := title
		permReq := acp.RequestPermissionRequest{
			ToolCall: acp.ToolCallUpdate{
				ToolCallId: acp.ToolCallId(interrupt.ID),
				Title:      &titleCopy,
			},
			Options: []acp.PermissionOption{
				{OptionId: "approve", Name: "Approve", Kind: acp.PermissionOptionKindAllowOnce},
				{OptionId: "deny", Name: "Deny", Kind: acp.PermissionOptionKindRejectOnce},
			},
		}

		resp, err := a.chatClient.RequestPermission(ctx, permReq)
		if err != nil {
			return nil, fmt.Errorf("agui: interrupt relay for %q: %w", interrupt.ID, err)
		}

		var answer any
		if resp.Outcome.Selected != nil && string(resp.Outcome.Selected.OptionId) == "approve" {
			answer = "approved"
		} else {
			answer = "denied"
		}

		resume = append(resume, agUIResumeEntry{
			InterruptID: interrupt.ID,
			Response:    answer,
		})
	}

	return resume, nil
}

// ─── pool registration helper ─────────────────────────────────────────────────

// registerAGUIAgentsWithInventory registers all configured AG-UI backends with
// the server's agent inventory. Called by the pool at startup and on reconnect.
// Best-effort: failures are logged so the pool still serves ACP pages.
func (d *poolDaemon) registerAGUIAgentsWithInventory(ctx context.Context) {
	for id, cfg := range d.agUIBackends {
		_, err := d.inventoryFor().RegisterPoolAgent(ctx, connect.NewRequest(&apiv1.RegisterPoolAgentRequest{
			Id:             id,
			DisplayName:    cfg.DisplayName,
			Description:    cfg.Description,
			Protocol:       apiv1.AgentProtocol_AG_UI_REMOTE,
			Route:          cfg.Endpoint,
			PoolInstanceId: d.poolInstanceID,
		}))
		if err != nil {
			slog.Warn("agui: failed to register agent with inventory", "agent_id", id, logKeyError, err, logKeyAction, "pool_register")
			continue
		}
		slog.Info("agui: agent registered with inventory", "agent_id", id, "endpoint", cfg.Endpoint, logKeyAction, "pool_register")
	}
}

// ─── JSON helpers ─────────────────────────────────────────────────────────────

// jsonString extracts a string value from a raw JSON map by key.
// Returns "" on missing key or non-string value.
func jsonString(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// jsonStringDefault is like jsonString but returns def when the key is absent
// or the value is not a string.
func jsonStringDefault(m map[string]json.RawMessage, key, def string) string {
	if s := jsonString(m, key); s != "" {
		return s
	}
	return def
}
