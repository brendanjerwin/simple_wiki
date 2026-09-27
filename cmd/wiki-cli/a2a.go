package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

// a2aPart is one content part of an A2A message (v0.3 wire shape).
type a2aPart struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// a2aMessage is an A2A message (v0.3 wire shape).
type a2aMessage struct {
	Role      string    `json:"role"`
	Parts     []a2aPart `json:"parts"`
	TaskID    string    `json:"taskId,omitempty"`
	ContextID string    `json:"contextId,omitempty"`
}

// a2aStatus is an A2A task status snapshot.
type a2aStatus struct {
	State     string      `json:"state"`
	Message   *a2aMessage `json:"message,omitempty"`
	Timestamp string      `json:"timestamp"` // RFC3339
}

// a2aArtifact is an A2A task artifact.
type a2aArtifact struct {
	Parts []a2aPart `json:"parts"`
}

// a2aTaskJSON is the task object returned over the wire (v0.3, camelCase —
// the pi-a2a-adaptor normalizes on contextId presence).
type a2aTaskJSON struct {
	ID        string        `json:"id"`
	ContextID string        `json:"contextId"`
	Status    a2aStatus     `json:"status"`
	Artifacts []a2aArtifact `json:"artifacts,omitempty"`
	History   []a2aMessage  `json:"history,omitempty"`
}

// a2aServerConfig configures the A2A server. Secrets are env-only so they
// never appear in `ps` output.
type a2aServerConfig struct {
	Port        int           // flag --a2a-port, 0 = disabled (default 0)
	Bind        string        // flag --a2a-bind, default "0.0.0.0"
	PublicURL   string        // flag --a2a-public-url (card.url; gateway path in prod)
	BearerToken string        // env DORIUM_A2A_BEARER_TOKEN
	ProxySecret string        // env DORIUM_TRUSTED_PROXY_SECRET
	TLSCertPath string        // flag --a2a-tls-cert (empty = plain HTTP)
	TLSKeyPath  string        // flag --a2a-tls-key
	TaskTimeout time.Duration // flag --a2a-task-timeout, default 9m
	MaxTasks    int           // flag --a2a-max-tasks, default 128
}

// trustedProxySecretHeader is the shared-secret header the mcp-gateway sets
// when forwarding authenticated A2A traffic to this server.
const trustedProxySecretHeader = "X-Dorium-Trusted-Proxy-Secret"

// a2aCaller is the resolved identity of whoever dispatched the task, so the
// agent (and the journal) know who is delegating.
type a2aCaller struct {
	Login      string // verified human identity (email), gateway path only
	ClientID   string // gateway client_id (e.g. gwmcp-1a2b…), gateway path only
	ClientName string // human-readable client name (e.g. "Mars", "claude.ai"), gateway path only
}

// describe renders a human-readable one-line caller identity.
func (c a2aCaller) describe() string {
	var who string
	switch {
	case c.ClientName != "" && c.ClientID != "":
		who = fmt.Sprintf("%s (%s)", c.ClientName, c.ClientID)
	case c.ClientName != "":
		who = c.ClientName
	case c.ClientID != "":
		who = c.ClientID
	default:
		who = "unknown caller"
	}
	if c.Login != "" {
		return fmt.Sprintf("%s authorized by %s", who, c.Login)
	}
	if c.ClientName == "pi-agent" {
		return "pi-agent (shared tailnet bearer)"
	}
	return who
}

// a2aTask is the internal in-memory task record.
type a2aTask struct {
	ID        string
	ContextID string
	UserText  string
	FinalText string
	Caller    a2aCaller
	State     string // "working" | "completed" | "failed" | "canceled"
	CreatedAt time.Time
	cancel    context.CancelFunc
}

func (t *a2aTask) terminal() bool {
	return t.State == "completed" || t.State == "failed" || t.State == "canceled"
}

// a2aJSONRPCRequest is an inbound JSON-RPC 2.0 request. The id is echoed
// verbatim (number or string).
type a2aRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	ID      json.RawMessage `json:"id,omitempty"`
	Params  struct {
		Message       *a2aMessage `json:"message"`
		Configuration struct {
			AcceptedOutputModes []string `json:"acceptedOutputModes"`
			Blocking            bool     `json:"blocking"`
		} `json:"configuration"`
		Metadata  map[string]any `json:"metadata"`
		ID        string         `json:"id"`
		ContextID string         `json:"contextId"`
	} `json:"params"`
}

type a2aResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  *a2aTaskJSON    `json:"result,omitempty"`
	Error   *a2aError       `json:"error,omitempty"`
}

type a2aError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC error codes.
const (
	a2aErrParse          = -32700
	a2aErrInvalidRequest = -32600
	a2aErrMethodNotFound = -32601
	a2aErrInvalidParams  = -32602
	a2aErrTaskNotFound   = -32001
)

// a2aPreamble is prepended to every A2A task prompt. Tells the agent it is
// executing a delegated one-shot task, not an interactive chat.
const a2aPreamble = `You are Dorium, a household AI assistant, executing a delegated task received via the A2A protocol. This is NOT an interactive wiki chat: no user is watching a chat page, you cannot ask clarifying questions, and permission requests are auto-denied. Use your normal tools to complete the task. Your final message is returned verbatim to the calling agent as the task result, so end with a complete, self-contained answer. Requesting party: %s.

## Task

%s`

// taskRunner is the test seam for task execution. nil = real execution.
type taskRunner func(ctx context.Context, task *a2aTask, caller a2aCaller) (string, error)

// a2aServer serves the A2A JSON-RPC surface for one-shot delegated tasks.
type a2aServer struct {
	cfg    a2aServerConfig
	daemon *poolDaemon

	// taskRunner, when non-nil, replaces real task execution (tests).
	taskRunner taskRunner

	mu    sync.Mutex
	tasks map[string]*a2aTask
}

// newA2AServer validates config and constructs the server. Fail-closed: A2A
// never starts unauthenticated.
func newA2AServer(cfg a2aServerConfig, d *poolDaemon) (*a2aServer, error) {
	if cfg.Bind == "" {
		cfg.Bind = "0.0.0.0"
	}
	if cfg.TaskTimeout <= 0 {
		cfg.TaskTimeout = 9 * time.Minute
	}
	if cfg.MaxTasks <= 0 {
		cfg.MaxTasks = 128
	}
	if cfg.BearerToken == "" && cfg.ProxySecret == "" {
		return nil, errors.New("a2a: refusing to start without credentials: set DORIUM_A2A_BEARER_TOKEN and/or DORIUM_TRUSTED_PROXY_SECRET")
	}
	if (cfg.TLSCertPath == "") != (cfg.TLSKeyPath == "") {
		return nil, errors.New("a2a: --a2a-tls-cert and --a2a-tls-key must be set together")
	}
	if cfg.TLSCertPath == "" && cfg.Bind != "127.0.0.1" && cfg.Bind != "localhost" && cfg.Bind != "::1" {
		slog.Warn("a2a: TLS disabled and bind is not loopback; traffic is unencrypted", "bind", cfg.Bind)
	}
	return &a2aServer{cfg: cfg, daemon: d, tasks: make(map[string]*a2aTask)}, nil
}

// serve runs the HTTP server until ctx is done.
func (s *a2aServer) serve(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/agent-card.json", s.handleCard)
	mux.HandleFunc("POST /{$}", s.handleDispatch)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	srv := &http.Server{
		Addr:              net.JoinHostPort(s.cfg.Bind, strconv.Itoa(s.cfg.Port)),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		var err error
		if s.cfg.TLSCertPath != "" {
			err = srv.ListenAndServeTLS(s.cfg.TLSCertPath, s.cfg.TLSKeyPath)
		} else {
			err = srv.ListenAndServe()
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("a2a: server error: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		<-errCh
		return nil
	}
}

// agentCard renders the public AgentCard for this server.
func (s *a2aServer) agentCard() map[string]any {
	url := s.cfg.PublicURL
	return map[string]any{
		"name":        "dorium",
		"description": "Dorium household AI assistant (wiki-chat agent). Executes delegated one-shot tasks with its normal toolset (wiki MCP tools, file/system access via allowlisted commands) and returns the final answer as task text.",
		"version":     version,
		"url":         url,
		"provider":    map[string]any{"organization": "home_lab"},
		"capabilities": map[string]any{
			"streaming":         false,
			"pushNotifications": false,
		},
		"securitySchemes": map[string]any{
			"bearer": map[string]any{
				"httpAuthSecurityScheme": map[string]any{
					"scheme":       "Bearer",
					"bearerFormat": "string",
				},
			},
		},
		"defaultInputModes":  []string{"text/plain"},
		"defaultOutputModes": []string{"text/plain"},
		"supportedInterfaces": []map[string]any{{
			"url":             url,
			"protocolBinding": "JSONRPC",
			"protocolVersion": "0.3",
			"tenant":          "",
		}},
		"skills": []map[string]any{{
			"id":          "dorium-delegate",
			"name":        "Dorium Delegated Task",
			"description": "Run a one-shot task through Dorium. The agent completes the task non-interactively (permission requests are auto-denied) and its final message is returned verbatim as the task result.",
			"tags":        []string{"dorium", "delegation", "household", "wiki"},
			"examples":    []string{"Summarize this week's chore chart", "Check the dinner plan and list missing groceries"},
			"inputModes":  []string{"text/plain"},
			"outputModes": []string{"text/plain"},
		}},
	}
}

func (s *a2aServer) handleCard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.agentCard())
}

// authorize resolves the caller identity from the request. Dispatch-only;
// the card is public. Constant-time compares everywhere.
func (s *a2aServer) authorize(r *http.Request) (a2aCaller, bool) {
	if proxySecret := r.Header.Get(trustedProxySecretHeader); proxySecret != "" {
		expected := s.cfg.ProxySecret
		if expected != "" && subtle.ConstantTimeCompare([]byte(proxySecret), []byte(expected)) == 1 {
			return a2aCaller{
				Login:      r.Header.Get("Tailscale-User-Login"),
				ClientID:   r.Header.Get("X-Gateway-Client-ID"),
				ClientName: r.Header.Get("X-Gateway-Client-Name"),
			}, true
		}
		return a2aCaller{}, false
	}

	if auth := r.Header.Get("Authorization"); auth != "" {
		const prefix = "Bearer "
		if strings.HasPrefix(auth, prefix) {
			token := auth[len(prefix):]
			if s.cfg.BearerToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.BearerToken)) == 1 {
				return a2aCaller{ClientName: "pi-agent"}, true
			}
		}
	}
	return a2aCaller{}, false
}

func (s *a2aServer) handleDispatch(w http.ResponseWriter, r *http.Request) {
	caller, ok := s.authorize(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		s.writeError(w, nil, a2aErrInvalidRequest, fmt.Sprintf("read body: %v", err))
		return
	}

	// Single JSON object only — batch/array bodies are not supported (v0.3
	// single-request dialect).
	var probe any
	if err := json.Unmarshal(body, &probe); err != nil {
		s.writeError(w, nil, a2aErrParse, fmt.Sprintf("invalid JSON: %v", err))
		return
	}
	if _, ok := probe.(map[string]any); !ok {
		s.writeError(w, nil, a2aErrInvalidRequest, "single JSON object request expected")
		return
	}

	var req a2aRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeError(w, nil, a2aErrParse, fmt.Sprintf("invalid JSON: %v", err))
		return
	}
	if req.JSONRPC != "2.0" {
		s.writeError(w, req.ID, a2aErrInvalidRequest, "jsonrpc must be \"2.0\"")
		return
	}

	switch req.Method {
	case "message/send":
		s.handleMessageSend(w, r, req, caller)
	case "tasks/get":
		s.handleTaskGet(w, req)
	case "tasks/cancel":
		s.handleTaskCancel(w, req)
	case "tasks/list":
		s.handleTaskList(w, req)
	default:
		s.writeError(w, req.ID, a2aErrMethodNotFound, fmt.Sprintf("unknown method: %s", req.Method))
	}
}

func (s *a2aServer) handleMessageSend(w http.ResponseWriter, r *http.Request, req a2aRequest, caller a2aCaller) {
	if req.Params.Message == nil {
		s.writeError(w, req.ID, a2aErrInvalidParams, "params.message is required")
		return
	}
	text := extractA2AText(req.Params.Message.Parts)
	if strings.TrimSpace(text) == "" {
		s.writeError(w, req.ID, a2aErrInvalidParams, "message must contain non-empty text part")
		return
	}

	task := &a2aTask{
		ID:        "a2a-" + randomHex(16),
		ContextID: req.Params.Message.ContextID,
		UserText:  text,
		Caller:    caller,
		State:     "working",
		CreatedAt: time.Now(),
	}
	if task.ContextID == "" {
		task.ContextID = "ctx-" + randomHex(8)
	}

	taskCtx, cancel := context.WithCancel(context.Background())
	task.cancel = cancel

	s.mu.Lock()
	s.pruneLocked()
	s.tasks[task.ID] = task
	snap := s.snapshot(task)
	s.mu.Unlock()

	slog.Info("a2a task started",
		"task_id", task.ID,
		"context_id", task.ContextID,
		"caller", caller.describe(),
		logKeyAction, "a2a_task_start")

	go s.executeA2ATask(taskCtx, task)

	s.writeResult(w, req.ID, snap)
}

func extractA2AText(parts []a2aPart) string {
	var b strings.Builder
	for _, p := range parts {
		if p.Kind == "text" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing means the OS entropy source is broken; failing
		// loudly here is safer than issuing predictable task IDs.
		panic(fmt.Sprintf("a2a: crypto/rand read failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// tasksCount returns the number of in-memory tasks (test helper).
func (s *a2aServer) tasksCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.tasks)
}

// pruneLocked evicts the oldest terminal tasks when the store exceeds
// MaxTasks. Never evicts non-terminal tasks. Must be called with s.mu held.
func (s *a2aServer) pruneLocked() {
	if len(s.tasks) < s.cfg.MaxTasks {
		return
	}
	ids := make([]string, 0, len(s.tasks))
	for id := range s.tasks {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		ti, tj := s.tasks[ids[i]], s.tasks[ids[j]]
		// Terminal-first, then oldest-first.
		if ti.terminal() != tj.terminal() {
			return ti.terminal()
		}
		return ti.CreatedAt.Before(tj.CreatedAt)
	})
	for _, id := range ids {
		if len(s.tasks) < s.cfg.MaxTasks {
			break
		}
		t := s.tasks[id]
		if !t.terminal() {
			break
		}
		delete(s.tasks, id)
	}
}

// snapshot renders the wire-shape task object. Must be called with s.mu held
// OR on a task whose goroutine no longer mutates it.
func (s *a2aServer) snapshot(t *a2aTask) *a2aTaskJSON {
	status := a2aStatus{
		State:     t.State,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	out := &a2aTaskJSON{
		ID:        t.ID,
		ContextID: t.ContextID,
		Status:    status,
	}
	if t.terminal() {
		msg := &a2aMessage{
			Role:      "agent",
			Parts:     []a2aPart{{Kind: "text", Text: t.FinalText}},
			TaskID:    t.ID,
			ContextID: t.ContextID,
		}
		status.Message = msg
		out.Status = status
		out.Artifacts = []a2aArtifact{{Parts: []a2aPart{{Kind: "text", Text: t.FinalText}}}}
		userMsg := a2aMessage{Role: "user", Parts: []a2aPart{{Kind: "text", Text: t.UserText}}, TaskID: t.ID, ContextID: t.ContextID}
		out.History = []a2aMessage{userMsg, *msg}
	}
	return out
}

func (s *a2aServer) handleTaskGet(w http.ResponseWriter, req a2aRequest) {
	if req.Params.ID == "" {
		s.writeError(w, req.ID, a2aErrInvalidParams, "params.id is required")
		return
	}
	s.mu.Lock()
	task, ok := s.tasks[req.Params.ID]
	if !ok {
		s.mu.Unlock()
		s.writeError(w, req.ID, a2aErrTaskNotFound, fmt.Sprintf("task not found: %s", req.Params.ID))
		return
	}
	snap := s.snapshot(task)
	s.mu.Unlock()
	s.writeResult(w, req.ID, snap)
}

func (s *a2aServer) handleTaskCancel(w http.ResponseWriter, req a2aRequest) {
	if req.Params.ID == "" {
		s.writeError(w, req.ID, a2aErrInvalidParams, "params.id is required")
		return
	}
	s.mu.Lock()
	task, ok := s.tasks[req.Params.ID]
	if ok && !task.terminal() {
		task.cancel()
		task.State = "canceled"
		slog.Info("a2a task canceled", "task_id", task.ID, logKeyAction, "a2a_task_cancel")
	}
	if !ok {
		s.mu.Unlock()
		s.writeError(w, req.ID, a2aErrTaskNotFound, fmt.Sprintf("task not found: %s", req.Params.ID))
		return
	}
	snap := s.snapshot(task)
	s.mu.Unlock()
	s.writeResult(w, req.ID, snap)
}

func (s *a2aServer) handleTaskList(w http.ResponseWriter, req a2aRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.tasks))
	for id := range s.tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]*a2aTaskJSON, 0, len(s.tasks))
	for _, id := range ids {
		t := s.tasks[id]
		if req.Params.ContextID != "" && t.ContextID != req.Params.ContextID {
			continue
		}
		out = append(out, s.snapshot(t))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result":  map[string]any{"tasks": out},
	})
}

// executeA2ATask runs the task to completion in the background, updating the
// task record as it goes.
func (s *a2aServer) executeA2ATask(taskCtx context.Context, task *a2aTask) {
	runner := s.taskRunner
	if runner == nil {
		runner = s.runTaskWithAgent
	}

	timeoutCtx, cancelTimeout := context.WithTimeout(taskCtx, s.cfg.TaskTimeout)
	defer cancelTimeout()

	finalText, err := runner(timeoutCtx, task, task.Caller)

	s.mu.Lock()
	defer s.mu.Unlock()
	// tasks/cancel may already have flipped the state; never resurrect a task.
	if task.State == "canceled" {
		return
	}
	switch {
	case err != nil && errors.Is(err, context.DeadlineExceeded) && taskCtx.Err() == nil:
		task.State = "failed"
		task.FinalText = fmt.Sprintf("task deadline exceeded (%s)", s.cfg.TaskTimeout)
	case err != nil && taskCtx.Err() != nil:
		task.State = "canceled"
		task.FinalText = "server shutting down"
	case err != nil:
		task.State = "failed"
		task.FinalText = fmt.Sprintf("prompt failed: %v", err)
	default:
		task.State = "completed"
		task.FinalText = finalText
	}
	slog.Info("a2a task finished",
		"task_id", task.ID,
		"state", task.State,
		"caller", task.Caller.describe(),
		logKeyAction, "a2a_task_finish")
}

// runTaskWithAgent performs real execution: spawn an ephemeral pi-acp agent,
// send the preamble + task text as one Prompt, and return the accumulated
// agent text.
func (s *a2aServer) runTaskWithAgent(ctx context.Context, task *a2aTask, caller a2aCaller) (string, error) {
	client := &a2aTaskClient{task: task}
	agent, spawnErr := s.daemon.spawnEphemeralAgent(ctx, client, a2aUnitPrefix, "a2a-"+task.ID[len("a2a-"):len("a2a-")+scheduledTurnRequestIDInUnit])
	if spawnErr != nil {
		return "", fmt.Errorf("spawn failed: %w", spawnErr)
	}
	defer agent.cleanup()

	promptText := fmt.Sprintf(a2aPreamble, caller.describe(), task.UserText)
	_, promptErr := agent.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: agent.sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(promptText)},
	})
	if promptErr != nil {
		return "", promptErr
	}
	return client.finalText(), nil
}

func (s *a2aServer) writeResult(w http.ResponseWriter, id json.RawMessage, result *a2aTaskJSON) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(a2aResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *a2aServer) writeError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(a2aResponse{JSONRPC: "2.0", ID: id, Error: &a2aError{Code: code, Message: message}})
}

// a2aTaskClient implements acp.Client for one A2A task: it accumulates agent
// message chunks into the task's final text, auto-denies permissions, and
// denies filesystem/terminal access (the agent should use wiki MCP tools).
type a2aTaskClient struct {
	task *a2aTask

	mu   sync.Mutex
	text strings.Builder
}

// SessionUpdate implements acp.Client. It accumulates agent message chunks;
// the accumulated text becomes the task's final result.
func (c *a2aTaskClient) SessionUpdate(_ context.Context, n acp.SessionNotification) error {
	if n.Update.AgentMessageChunk == nil {
		return nil
	}
	chunk := n.Update.AgentMessageChunk
	if chunk.Content.Text == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.text.WriteString(chunk.Content.Text.Text)
	return nil
}

// RequestPermission implements acp.Client. A2A tasks are non-interactive —
// auto-deny so the agent finishes deterministically.
func (c *a2aTaskClient) RequestPermission(_ context.Context, _ acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return permissionCancelledResponse(), nil
}

// finalText returns the accumulated agent text.
func (c *a2aTaskClient) finalText() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text.String()
}

// ReadTextFile implements acp.Client. Filesystem access is disabled.
func (*a2aTaskClient) ReadTextFile(_ context.Context, _ acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, errors.New("file system access not available in a2a tasks")
}

// WriteTextFile implements acp.Client. Filesystem access is disabled.
func (*a2aTaskClient) WriteTextFile(_ context.Context, _ acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, errors.New("file system access not available in a2a tasks")
}

// CreateTerminal implements acp.Client. Terminal access is disabled.
func (*a2aTaskClient) CreateTerminal(_ context.Context, _ acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, errors.New(errTerminalAccessUnavailable)
}

// TerminalOutput implements acp.Client. Terminal access is disabled.
func (*a2aTaskClient) TerminalOutput(_ context.Context, _ acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, errors.New(errTerminalAccessUnavailable)
}

// ReleaseTerminal implements acp.Client. Terminal access is disabled.
func (*a2aTaskClient) ReleaseTerminal(_ context.Context, _ acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, errors.New(errTerminalAccessUnavailable)
}

// WaitForTerminalExit implements acp.Client. Terminal access is disabled.
func (*a2aTaskClient) WaitForTerminalExit(_ context.Context, _ acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, errors.New(errTerminalAccessUnavailable)
}

// KillTerminal implements acp.Client. Terminal access is disabled.
func (*a2aTaskClient) KillTerminal(_ context.Context, _ acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, errors.New(errTerminalAccessUnavailable)
}
