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
	"os"
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
	Progress  string      `json:"progress,omitempty"` // in-flight agent text while state=working
	Timestamp string      `json:"timestamp"`          // RFC3339
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
	Port           int           // flag --a2a-port, 0 = disabled (default 0)
	Bind           string        // flag --a2a-bind, default "0.0.0.0"
	PublicURL      string        // flag --a2a-public-url (card.url; gateway path in prod)
	AgentName      string        // flag --a2a-agent-name (agent card name/skill naming)
	ChatPersona    string        // display name of the chat AI persona (preamble identity)
	BearerToken    string        // env WIKI_CLI_A2A_BEARER_TOKEN
	ProxySecret    string        // env WIKI_CLI_A2A_TRUSTED_PROXY_SECRET
	ProxyHeader    string        // flag --a2a-trusted-proxy-header, default X-A2A-Trusted-Proxy-Secret
	StatePath      string        // flag --a2a-state-path: JSON file for persisted task results (empty = memory-only)
	TLSCertPath    string        // flag --a2a-tls-cert (empty = plain HTTP)
	TLSKeyPath     string        // flag --a2a-tls-key
	TaskTimeout    time.Duration // flag --a2a-task-timeout, default 9m
	MaxTaskTimeout time.Duration // flag --a2a-max-task-timeout, default 60m

	MaxTasks int // flag --a2a-max-tasks, default 128
}

// trustedProxySecretHeader is the default shared-secret header a trusted
// reverse proxy (e.g. the mcp-gateway) sets when forwarding authenticated
// A2A traffic to this server. Configurable via --a2a-trusted-proxy-header.
const trustedProxySecretHeader = "X-A2A-Trusted-Proxy-Secret"

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
	switch c.ClientName {
	case a2aCallerPIAgent:
		return "pi-agent (shared tailnet bearer)"
	case a2aCallerTailnet:
		return "tailnet caller (no credentials)"
	default:
	}
	return who
}

// a2aTask is the internal in-memory task record.
type a2aTask struct {
	ID        string
	ContextID string
	UserText  string
	FinalText string
	// ProgressText is the agent's in-flight output for a working task,
	// mirrored live from the session's message-chunk collector. Empty
	// until the agent streams its first visible text. Rendered on
	// tasks/get as status.progress so pollers see liveness mid-run.
	ProgressText string
	Caller       a2aCaller
	State        string // "working" | "completed" | "failed" | "canceled"
	CreatedAt    time.Time
	cancel       context.CancelFunc
}

func (t *a2aTask) terminal() bool {
	return t.State == "completed" || t.State == "failed" || t.State == "canceled"
}

// a2aConfig is the message/send configuration block.
type a2aConfig struct {
	AcceptedOutputModes []string `json:"acceptedOutputModes"`
	Blocking            bool     `json:"blocking"`
}

// a2aParams is the JSON-RPC params object.
type a2aParams struct {
	Message       *a2aMessage    `json:"message"`
	Configuration a2aConfig      `json:"configuration"`
	Metadata      map[string]any `json:"metadata"`
	ID            string         `json:"id"`
	ContextID     string         `json:"contextId"`
}

// a2aJSONRPCRequest is an inbound JSON-RPC 2.0 request. The id is echoed
// verbatim (number or string).
type a2aRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	ID      json.RawMessage `json:"id,omitempty"`
	Params  a2aParams       `json:"params"`
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

// a2a tunables and wire literals.
const (
	defaultA2ATaskTimeout    = 30 * time.Minute
	defaultA2AMaxTaskTimeout = 60 * time.Minute
	// a2aMetadataTimeoutKey lets callers override the per-task deadline via
	// message/send metadata (number of seconds). Clamped to [1s, cfg.MaxTaskTimeout].
	a2aMetadataTimeoutKey = "a2a_task_timeout_seconds"
	defaultA2AMaxTasks    = 128

	a2aIDRandomBytes = 16 // 128-bit task ids
	a2aCtxRandomHex  = 8  // context ids: "ctx-" + 8 hex bytes

	a2aHeaderReadTimeout  = 10 * time.Second
	a2aReadTimeout        = 30 * time.Second
	a2aWriteTimeout       = 30 * time.Second
	a2aIdleTimeout        = 120 * time.Second
	a2aMaxRequestBodyByte = 1 << 20 // 1 MiB JSON-RPC body cap

	a2aTaskIDPrefix = "a2a-"
	a2aCtxIDPrefix  = "ctx-"

	a2aPartKindText   = "text"
	a2aModeText       = "text/plain"
	a2aRoleAgent      = "agent"
	a2aRoleUser       = "user"
	a2aStateWorking   = "working"
	a2aStateCanceled  = "canceled"
	a2aStateCompleted = "completed"
	a2aStateFailed    = "failed"
	a2aCallerPIAgent  = "pi-agent"
	a2aCallerTailnet  = "tailnet caller"

	logKeyTaskID = "task_id"
	logKeyCtxID  = "context_id"
	logKeyCaller = "caller"

	headerContentType   = "Content-Type"
	headerAuthorization = "Authorization"
	contentTypeJSON     = "application/json"
)

// a2aPreamble is prepended to every A2A task prompt (with the persona name
// and caller identity interpolated). Tells the agent it is executing a
// delegated one-shot task, not an interactive chat.
const a2aPreamble = `You are %s, a household AI assistant, executing a delegated task received via the A2A protocol. This is NOT an interactive wiki chat: no user is watching a chat page, you cannot ask clarifying questions, and permission requests are auto-denied. Use your normal tools to complete the task. Your final message is returned verbatim to the calling agent as the task result, so end with a complete, self-contained answer. Requesting party: %s.

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

	// persistPath, when non-empty, persists terminal task records to a JSON
	// file so tasks/get keeps returning results across service restarts and
	// deploys (in-flight tasks are swept to a truthful terminal state at
	// startup instead of vanishing into -32001). Empty = memory-only (tests).
	persistPath string

	// sessions persists ACP agent sessions per contextId so a follow-up
	// message/send with the same contextId (spec §3.4.3 continuation)
	// continues the same conversation instead of spawning a fresh agent.
	// Entries are evicted when a newer session takes the contextId or via
	// pruneLocked when the store exceeds MaxTasks. In-memory: sessions are
	// lost on service restart (the next message/send respawns).
	sessions map[string]*a2aSession
}

// a2aSession is one live ACP conversation bound to a contextId.
type a2aSession struct {
	agent     *ephemeralAgent
	client    *a2aTaskClient
	createdAt time.Time
	lastUsed  time.Time
	turnCount int

	// promptMu serializes Prompt calls on the session's ACP connection
	// (concurrent message/send on one contextId must not race the conn).
	promptMu sync.Mutex

	// sessionCtx owns the agent process lifetime — derived from the server,
	// NOT the task's timeout context, so the agent survives between turns.
	sessionCtx    context.Context
	sessionCancel context.CancelFunc
}

// spawningSession is the placeholder stored while a session is being
// spawned for a contextId. Waiters poll on it; only a real *a2aSession
// satisfies type assertions.
var spawningSession = &a2aSession{}

// isSpawning reports whether the map entry is a spawn-in-progress sentinel.
func isSpawning(s *a2aSession) bool { return s == spawningSession }

// a2a session lifecycle tunables.
const (
	a2aSessionIdleTTL     = 30 * time.Minute // evict idle conversations
	a2aSessionReapEvery   = 5 * time.Minute
	a2aSessionTurnCap     = 50 // max turns per conversation
	a2aSessionMaxSessions = 64 // max concurrent live conversations
)

// beginTurn swaps in a fresh text collector for one turn and returns the
// accessor for that turn's agent text (the session client's builder is
// shared across turns, so each turn must start empty).
func (sess *a2aSession) beginTurn() func() string {
	sess.client.mu.Lock()
	sess.client.text.Reset()
	sess.client.mu.Unlock()
	return sess.client.finalText
}

// turnAccessor returns a bound beginTurn for this session (usable from
// result structs without re-deriving the session).
func (sess *a2aSession) turnAccessor() func() string {
	return sess.client.finalText
}

// newA2AServer validates config and constructs the server. Trust model
// (Brendan's 2026-09-27 final directive): every dispatch requires a
// credential (trusted-proxy secret or bearer), same as every other MCP
// route — fail-closed. A deployment MUST configure at least one.
func newA2AServer(cfg a2aServerConfig, d *poolDaemon) (*a2aServer, error) {
	if cfg.Bind == "" {
		cfg.Bind = "0.0.0.0"
	}
	if cfg.ProxyHeader == "" {
		cfg.ProxyHeader = trustedProxySecretHeader
	}
	if cfg.AgentName == "" {
		cfg.AgentName = "wiki-chat-agent"
	}
	if cfg.TaskTimeout <= 0 {
		cfg.TaskTimeout = defaultA2ATaskTimeout
	}
	if cfg.MaxTaskTimeout <= 0 {
		cfg.MaxTaskTimeout = defaultA2AMaxTaskTimeout
	}
	if cfg.MaxTaskTimeout < cfg.TaskTimeout {
		return nil, fmt.Errorf("a2a: --a2a-max-task-timeout (%s) must be >= --a2a-task-timeout (%s)", cfg.MaxTaskTimeout, cfg.TaskTimeout)
	}
	if cfg.MaxTasks <= 0 {
		cfg.MaxTasks = defaultA2AMaxTasks
	}
	if cfg.BearerToken == "" && cfg.ProxySecret == "" {
		return nil, errors.New("a2a: refusing to start without credentials: set WIKI_CLI_A2A_BEARER_TOKEN and/or WIKI_CLI_A2A_TRUSTED_PROXY_SECRET")
	}
	if (cfg.TLSCertPath == "") != (cfg.TLSKeyPath == "") {
		return nil, errors.New("a2a: --a2a-tls-cert and --a2a-tls-key must be set together")
	}
	if cfg.TLSCertPath == "" && cfg.Bind != "127.0.0.1" && cfg.Bind != "localhost" && cfg.Bind != "::1" {
		slog.Warn("a2a: TLS disabled and bind is not loopback; traffic is unencrypted", "bind", cfg.Bind)
	}
	s := &a2aServer{cfg: cfg, daemon: d, tasks: make(map[string]*a2aTask), sessions: make(map[string]*a2aSession)}
	if cfg.StatePath != "" {
		s.persistPath = cfg.StatePath
		if err := s.loadPersistedTasks(); err != nil {
			slog.Warn("a2a: could not load persisted task records; starting fresh", "path", cfg.StatePath, "error", err)
		}
	}
	return s, nil
}

// persistedTaskRecord is the on-disk shape of a terminal task.
type persistedTaskRecord struct {
	ID        string    `json:"id"`
	ContextID string    `json:"contextId"`
	UserText  string    `json:"userText"`
	FinalText string    `json:"finalText"`
	State     string    `json:"state"`
	Caller    a2aCaller `json:"caller"`
	CreatedAt time.Time `json:"createdAt"`
}

// loadPersistedTasks reads terminal task records from the state file into
// memory, and sweeps any persisted non-terminal (i.e. in-flight-at-crash)
// tasks to failed with a truthful reason — clients polling those ids get a
// terminal state instead of task-not-found. Malformed state files start
// fresh (the file is best-effort bookkeeping, not a database).
func (s *a2aServer) loadPersistedTasks() error {
	data, err := os.ReadFile(s.cfg.StatePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil // first boot with persistence; nothing to load
	}
	if err != nil {
		return err
	}
	var records []persistedTaskRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return err
	}
	for _, r := range records {
		task := &a2aTask{
			ID:        r.ID,
			ContextID: r.ContextID,
			UserText:  r.UserText,
			FinalText: r.FinalText,
			Caller:    r.Caller,
			State:     r.State,
			CreatedAt: r.CreatedAt,
		}
		if !task.terminal() {
			// In-flight at shutdown: sweep to a truthful terminal state.
			task.State = a2aStateFailed
			task.FinalText = "service restarted while task was in flight"
			slog.Info("a2a swept in-flight task on startup",
				logKeyTaskID, task.ID,
				logKeyAction, "a2a_task_swept")
		}
		task.cancel = func() {} // terminal; cancel is a no-op
		s.tasks[task.ID] = task
	}
	slog.Info("a2a loaded persisted task records",
		"count", len(records),
		logKeyAction, "a2a_tasks_loaded")
	return nil
}

// persistTaskRecords writes every in-memory task record to the state file.
// Tasks in `working` state are persisted too — a crash mid-flight must
// leave a durable row so the next startup can sweep it to a truthful
// terminal state (otherwise clients get -32001 instead of "restarted").
// Best-effort: persistence failures are logged, not fatal — the in-memory
// record remains authoritative for the life of the process.
func (s *a2aServer) persistTaskRecords() {
	if s.persistPath == "" {
		return
	}
	s.mu.Lock()
	records := make([]persistedTaskRecord, 0, len(s.tasks))
	for _, t := range s.tasks {
		records = append(records, persistedTaskRecord{
			ID: t.ID, ContextID: t.ContextID, UserText: t.UserText,
			FinalText: t.FinalText, State: t.State, Caller: t.Caller,
			CreatedAt: t.CreatedAt,
		})
	}
	s.mu.Unlock()

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		slog.Error("a2a: marshal task records failed", logKeyError, err)
		return
	}
	// Atomic write: temp file + rename.
	tmp := s.persistPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		slog.Error("a2a: persist task records failed", "path", s.persistPath, logKeyError, err)
		return
	}
	if err := os.Rename(tmp, s.persistPath); err != nil {
		slog.Error("a2a: persist task records failed", "path", s.persistPath, logKeyError, err)
	}
}

// serve runs the HTTP server until ctx is done.
func (s *a2aServer) serve(ctx context.Context) error {
	// Session reaper: evicts idle/over-cap conversations and shuts all
	// sessions down on ctx.Done.
	go s.runSessionReaper(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/agent-card.json", s.handleCard)
	mux.HandleFunc("POST /{$}", s.handleDispatch)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	srv := &http.Server{
		Addr:              net.JoinHostPort(s.cfg.Bind, strconv.Itoa(s.cfg.Port)),
		Handler:           mux,
		ReadHeaderTimeout: a2aHeaderReadTimeout,
		ReadTimeout:       a2aReadTimeout,
		WriteTimeout:      a2aWriteTimeout,
		IdleTimeout:       a2aIdleTimeout,
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

// agentCard renders the public AgentCard for this server. The agent name is
// deployment config (cfg.AgentName) — this repo ships a generic wiki-chat
// agent and never hard-codes a personal assistant's name.
func (s *a2aServer) agentCard() map[string]any {
	url := s.cfg.PublicURL
	name := s.cfg.AgentName
	displayName := s.cfg.ChatPersona
	if displayName == "" {
		displayName = name
	}
	return map[string]any{
		"name":        name,
		"description": displayName + " household AI assistant (wiki-chat agent). Executes delegated one-shot tasks and returns the final answer as task text.",
		"version":     version,
		"url":         url,
		"provider":    map[string]any{"organization": "home_lab"},
		"capabilities": map[string]any{
			"streaming":         false,
			"pushNotifications": false,
		},
		// Bearer is the documented credential shape (trusted-proxy secret or
		// shared bearer token, per deployment).
		"securitySchemes": map[string]any{
			"bearer": map[string]any{
				"httpAuthSecurityScheme": map[string]any{
					"scheme":       "Bearer",
					"bearerFormat": "string",
				},
			},
		},
		"defaultInputModes":  []string{a2aModeText},
		"defaultOutputModes": []string{a2aModeText},
		"supportedInterfaces": []map[string]any{{
			"url":             url,
			"protocolBinding": "JSONRPC",
			"protocolVersion": "0.3",
			"tenant":          "",
		}},
		"skills": []map[string]any{{
			"id":          name + "-delegate",
			"name":        displayName + " Delegated Task",
			"description": "Run a one-shot task through " + displayName + ". The agent completes the task non-interactively (permission requests are auto-denied) and its final message is returned verbatim as the task result.",
			"tags":        []string{name, "delegation", "household", "wiki"},
			"examples":    []string{"Summarize this week's chore chart", "Check the dinner plan and list missing groceries"},
			"inputModes":  []string{a2aModeText},
			"outputModes": []string{a2aModeText},
		}},
	}
}

func (s *a2aServer) handleCard(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(headerContentType, contentTypeJSON)
	_ = json.NewEncoder(w).Encode(s.agentCard())
}

// authorize resolves the caller identity from the request. Dispatch-only;
// the card is public.
//
// Trust model (Brendan's 2026-09-27 final directive): the listener requires
// a credential on EVERY dispatch, same as every other MCP route — no
// token-free exceptions. Fail-closed: no valid credential → 401. Accepted
// credentials:
//   - The configured trusted-proxy header + secret (constant-time compared)
//     — this is how the mcp-gateway forwards authenticated traffic.
//   - A matching Bearer token — programmatic callers (pi-adaptor).
//
// A present-but-wrong proxy secret is a spoofing attempt: fail closed.
func (s *a2aServer) authorize(r *http.Request) (a2aCaller, bool) {
	if proxySecret := r.Header.Get(s.cfg.ProxyHeader); proxySecret != "" {
		expected := s.cfg.ProxySecret
		if expected != "" && subtle.ConstantTimeCompare([]byte(proxySecret), []byte(expected)) == 1 {
			return a2aCaller{
				Login:      r.Header.Get("Tailscale-User-Login"),
				ClientID:   r.Header.Get("X-Gateway-Client-ID"),
				ClientName: r.Header.Get("X-Gateway-Client-Name"),
			}, true
		}
		// Present-but-wrong secret: spoofing attempt, fail closed.
		return a2aCaller{}, false
	}

	if auth := r.Header.Get(headerAuthorization); auth != "" {
		const prefix = "Bearer "
		if strings.HasPrefix(auth, prefix) {
			token := auth[len(prefix):]
			if s.cfg.BearerToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.BearerToken)) == 1 {
				return a2aCaller{ClientName: a2aCallerPIAgent}, true
			}
		}
	}
	// No credential or non-matching credential: unauthorized.
	return a2aCaller{}, false
}

func (s *a2aServer) handleDispatch(w http.ResponseWriter, r *http.Request) {
	// Fail-closed: no valid credential → 401. No WWW-Authenticate challenge
	// (the credential shapes are documented out-of-band, not negotiated).
	caller, ok := s.authorize(r)
	if !ok {
		w.Header().Set(headerContentType, contentTypeJSON)
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

func (s *a2aServer) handleMessageSend(w http.ResponseWriter, _ *http.Request, req a2aRequest, caller a2aCaller) {
	if req.Params.Message == nil {
		s.writeError(w, req.ID, a2aErrInvalidParams, "params.message is required")
		return
	}
	text := extractA2AText(req.Params.Message.Parts)
	if strings.TrimSpace(text) == "" {
		s.writeError(w, req.ID, a2aErrInvalidParams, "message must contain non-empty text part")
		return
	}
	taskTimeout := s.taskTimeoutFromMetadata(req.Params.Metadata)
	if taskTimeout < 0 {
		s.writeError(w, req.ID, a2aErrInvalidParams, taskTimeoutMetadataErr)
		return
	}

	task := &a2aTask{
		ID:        a2aTaskIDPrefix + randomHex(a2aIDRandomBytes),
		ContextID: req.Params.Message.ContextID,
		UserText:  text,
		Caller:    caller,
		State:     a2aStateWorking,
		CreatedAt: time.Now(),
	}
	if task.ContextID == "" {
		task.ContextID = a2aCtxIDPrefix + randomHex(a2aCtxRandomHex)
	}

	taskCtx, cancel := context.WithCancel(context.Background())
	task.cancel = cancel

	s.mu.Lock()
	s.pruneLocked()
	s.tasks[task.ID] = task
	snap := s.snapshot(task)
	s.mu.Unlock()

	slog.Info("a2a task started",
		logKeyTaskID, task.ID,
		logKeyCtxID, task.ContextID,
		logKeyCaller, caller.describe(),
		logKeyAction, "a2a_task_start")

	// Persist the `working` row immediately: if the service dies mid-flight,
	// the next startup sweeps this id to a truthful terminal state instead
	// of the client polling into -32001.
	s.persistTaskRecords()

	go s.executeA2ATask(taskCtx, task, taskTimeout)

	s.writeResult(w, req.ID, snap)
}

func extractA2AText(parts []a2aPart) string {
	var b strings.Builder
	for _, p := range parts {
		if p.Kind == a2aPartKindText {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

// taskTimeoutMetadataErr is the rejection message for a malformed
// metadata timeout override ("metadata.a2a_task_timeout_seconds").
const taskTimeoutMetadataErr = "metadata.a2a_task_timeout_seconds must be a positive number of seconds within the server's max task timeout"

// a2aTimeoutParseFloatBits is the bit size for parsing the metadata
// timeout override as a decimal floating-point number of seconds.
const a2aTimeoutParseFloatBits = 64

// taskTimeoutFromMetadata resolves the per-task deadline override. Returns
// s.cfg.TaskTimeout when the key is absent. Returns -1 for a malformed
// value (caller rejects the dispatch). Valid values are clamped to
// [1s, cfg.MaxTaskTimeout]; accepted types: number (seconds, JSON decodes
// as float64), numeric string, or json.Number.
func (s *a2aServer) taskTimeoutFromMetadata(metadata map[string]any) time.Duration {
	raw, ok := metadata[a2aMetadataTimeoutKey]
	if !ok || raw == nil {
		return s.cfg.TaskTimeout
	}
	var seconds float64
	switch v := raw.(type) {
	case float64:
		seconds = v
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), a2aTimeoutParseFloatBits)
		if err != nil {
			return -1
		}
		seconds = f
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return -1
		}
		seconds = f
	default:
		return -1
	}
	if seconds <= 0 {
		return -1
	}
	d := time.Duration(seconds * float64(time.Second))
	if d < time.Second {
		return -1
	}
	if d > s.cfg.MaxTaskTimeout {
		d = s.cfg.MaxTaskTimeout
	}
	return d
}

// a2aProgressMaxRunes caps the mirrored in-flight progress text: long
// turns stream megabytes of accumulated chunks, and the mirror only needs
// a tail window for tasks/get liveness. Grows are dropped, not buffered.
const a2aProgressMaxRunes = 400

// truncateProgress keeps at most a2aProgressMaxRunes runes of s: an
// ellipsis marker plus the trailing window. Total length is capped.
func truncateProgress(s string) string {
	r := []rune(s)
	if len(r) <= a2aProgressMaxRunes {
		return s
	}
	return "…" + string(r[len(r)-(a2aProgressMaxRunes-1):])
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
func (*a2aServer) snapshot(t *a2aTask) *a2aTaskJSON {
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
			Role:      a2aRoleAgent,
			Parts:     []a2aPart{{Kind: a2aPartKindText, Text: t.FinalText}},
			TaskID:    t.ID,
			ContextID: t.ContextID,
		}
		status.Message = msg
		out.Status = status
		out.Artifacts = []a2aArtifact{{Parts: []a2aPart{{Kind: a2aPartKindText, Text: t.FinalText}}}}
		userMsg := a2aMessage{Role: a2aRoleUser, Parts: []a2aPart{{Kind: a2aPartKindText, Text: t.UserText}}, TaskID: t.ID, ContextID: t.ContextID}
		out.History = []a2aMessage{userMsg, *msg}
	} else if t.ProgressText != "" {
		// Working task with in-flight agent output: expose it so pollers
		// can show liveness and sequence while the task runs.
		status.Progress = t.ProgressText
		out.Status = status
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
		task.State = a2aStateCanceled
		slog.Info("a2a task canceled", logKeyTaskID, task.ID, logKeyAction, "a2a_task_cancel")
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
	w.Header().Set(headerContentType, contentTypeJSON)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result":  map[string]any{"tasks": out},
	})
}

// executeA2ATask runs the task to completion in the background, updating the
// task record as it goes.
func (s *a2aServer) executeA2ATask(taskCtx context.Context, task *a2aTask, timeout time.Duration) {
	runner := s.taskRunner
	if runner == nil {
		runner = s.runTaskInSession
	}

	timeoutCtx, cancelTimeout := context.WithTimeout(taskCtx, timeout)
	defer cancelTimeout()

	finalText, err := runner(timeoutCtx, task, task.Caller)

	s.mu.Lock()
	// tasks/cancel may already have flipped the state; never resurrect a task.
	if task.State != a2aStateCanceled {
		switch {
		case err != nil && (errors.Is(err, context.DeadlineExceeded) || timeoutCtx.Err() != nil) && taskCtx.Err() == nil:
			// Deadline fired (mine, not shutdown). The acp SDK coerces
			// DeadlineExceeded into a JSON-RPC -32603 RequestError before
			// it reaches us, so errors.Is alone is not enough — also check
			// whether OUR timeout context expired.
			task.State = a2aStateFailed
			task.FinalText = fmt.Sprintf("task deadline exceeded (%s)", timeout)
		case err != nil && taskCtx.Err() != nil:
			task.State = a2aStateCanceled
			task.FinalText = "server shutting down"
		case err != nil:
			task.State = a2aStateFailed
			task.FinalText = fmt.Sprintf("prompt failed: %v", err)
		default:
			task.State = a2aStateCompleted
			task.FinalText = finalText
		}
	}
	s.mu.Unlock()

	slog.Info("a2a task finished",
		logKeyTaskID, task.ID,
		"state", task.State,
		logKeyCaller, task.Caller.describe(),
		logKeyAction, "a2a_task_finish")

	// Persist AFTER the critical section: persistTerminalTask takes s.mu
	// itself (persist the record whether canceled, failed, or completed —
	// and `working` rows were persisted at task creation so a crash
	// mid-flight sweeps to a truthful terminal state at next startup).
	s.persistTaskRecords()
}

// runTaskInSession performs real execution: acquire (or spawn) the agent
// session bound to the task's contextId, send the preamble (first turn) or
// bare task text (continuation turn) as one Prompt, and return the
// accumulated agent text. The session stays registered after the task ends
// so follow-up messages with the same contextId continue the conversation
// (A2A spec §3.4.3).
func (s *a2aServer) runTaskInSession(ctx context.Context, task *a2aTask, caller a2aCaller) (string, error) {
	turn, err := s.acquireSession(ctx, task)
	if err != nil {
		return "", fmt.Errorf("spawn failed: %w", err)
	}
	agent := turn.agent

	// One Prompt at a time per session (ACP conns aren't safe concurrently).
	s.mu.Lock()
	sess := s.sessions[task.ContextID]
	s.mu.Unlock()
	if sess == nil {
		return "", fmt.Errorf("a2a session vanished for context %s", task.ContextID)
	}
	sess.promptMu.Lock()
	defer sess.promptMu.Unlock()

	// Fresh text collector for THIS turn (the session client's builder
	// accumulates across turns otherwise); client.finalText reads it after
	// the Prompt completes. The progress sink below mirrors chunks onto
	// the task record for the duration of this turn; the defer tears down
	// both the sink and its drain goroutine when the turn ends.
	sess.beginTurn()
	client := sess.client
	teardown := s.installProgressSink(client, task)
	defer teardown()

	var promptText string
	if turn.firstTurn {
		persona := s.cfg.ChatPersona
		if persona == "" {
			persona = s.cfg.AgentName
		}
		promptText = fmt.Sprintf(a2aPreamble, persona, caller.describe(), task.UserText)
	} else {
		// Continuation turn: the persona/context is established; send the
		// user text bare so the agent treats it as the next conversation turn.
		promptText = task.UserText
	}
	_, promptErr := agent.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: agent.sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(promptText)},
	})
	if promptErr != nil {
		// A failed continuation turn may have killed the session's agent;
		// drop the session so the next message/send spawns fresh. Cancellation
		// (task timeout, shutdown) is NOT a session failure — the session
		// stays for the next turn.
		if ctx.Err() == nil {
			s.dropSession(task.ContextID)
		}
		return "", promptErr
	}
	return sess.client.finalText(), nil
}

// installProgressSink attaches a per-turn live-progress mirror: the
// client's accumulated text is drained onto task.ProgressText (truncated)
// by a background goroutine until the returned teardown func detaches the
// sink and closes it. Callers defer the teardown for the turn's duration.
func (s *a2aServer) installProgressSink(client *a2aTaskClient, task *a2aTask) func() {
	sink := make(chan string, 1)
	client.mu.Lock()
	client.progressSink = sink
	client.mu.Unlock()
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for text := range sink {
			s.mu.Lock()
			task.ProgressText = truncateProgress(text)
			s.mu.Unlock()
		}
	}()
	return func() {
		client.mu.Lock()
		client.progressSink = nil
		client.mu.Unlock()
		close(sink)
		<-drainDone
	}
}

// a2aSessionRef is what a turn needs from acquireSession: the agent
// connection, the turn-text collector accessor, and whether the session was
// just created (firstTurn → prepend the preamble).
type a2aSessionTurn struct {
	agent     *ephemeralAgent
	getText   func() string
	firstTurn bool
}

// acquireSession returns the live agent session for the task's contextId,
// spawning and registering a new one when absent. firstTurn is true when the
// session was just created (callers prepend the preamble). The session's
// agent lives under a session-lifetime context (server scope), not the
// task's timeout context, so it survives between turns. Concurrent callers
// on the same contextId serialize: the first reserves via the spawning
// sentinel, the rest poll until it resolves.
func (s *a2aServer) acquireSession(ctx context.Context, task *a2aTask) (*a2aSessionTurn, error) {
	// Wait for any in-flight spawn on this contextId.
	for {
		s.mu.Lock()
		existing, ok := s.sessions[task.ContextID]
		if !ok || isSpawning(existing) {
			// Absent (we will spawn) or someone else is spawning (wait).
			if !ok {
				break
			}
			s.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(20 * time.Millisecond):
			}
			continue
		}
		existing.lastUsed = time.Now()
		existing.turnCount++
		s.mu.Unlock()
		return &a2aSessionTurn{agent: existing.agent, getText: existing.turnAccessor(), firstTurn: false}, nil
	}

	// Reserve with the sentinel, then spawn outside the lock.
	s.sessions[task.ContextID] = spawningSession
	s.mu.Unlock()

	// The session outlives this task's timeout context: bind it to a fresh
	// context; the reaper and dropSession call sessionCancel on eviction.
	sessionCtx, sessionCancel := context.WithCancel(context.Background())

	client := &a2aTaskClient{task: task}
	agent, spawnErr := s.daemon.spawnEphemeralAgent(sessionCtx, client, a2aUnitPrefix, a2aTaskIDPrefix+shortIDForUnit(task.ID[len(a2aTaskIDPrefix):]))
	if spawnErr != nil {
		s.mu.Lock()
		// Only clear the sentinel if it is still ours.
		if s.sessions[task.ContextID] == spawningSession {
			delete(s.sessions, task.ContextID)
		}
		s.mu.Unlock()
		sessionCancel()
		return nil, spawnErr
	}

	sess := &a2aSession{
		agent:         agent,
		client:        client,
		createdAt:     time.Now(),
		lastUsed:      time.Now(),
		turnCount:     1,
		sessionCtx:    sessionCtx,
		sessionCancel: sessionCancel,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[task.ContextID] != spawningSession {
		// Someone replaced the sentinel (reaper/shutdown); tear ours down
		// and fail the turn — the caller retries on a fresh dispatch.
		agent.cleanup()
		sessionCancel()
		return nil, fmt.Errorf("a2a session spawn raced with eviction for context %s", task.ContextID)
	}
	s.sessions[task.ContextID] = sess
	slog.Info("a2a session created",
		logKeyCtxID, task.ContextID,
		logKeyTaskID, task.ID,
		logKeyAction, "a2a_session_create")
	return &a2aSessionTurn{agent: agent, getText: sess.turnAccessor(), firstTurn: true}, nil
}

// dropSession tears down and removes the session for a contextId (used when
// a continuation Prompt fails — the agent process may be dead).
func (s *a2aServer) dropSession(contextID string) {
	s.mu.Lock()
	sess, ok := s.sessions[contextID]
	if ok {
		delete(s.sessions, contextID)
	}
	s.mu.Unlock()
	if ok && sess != nil {
		if sess.agent != nil && sess.agent.cleanup != nil {
			sess.agent.cleanup()
		}
		if sess.sessionCancel != nil {
			sess.sessionCancel()
		}
	}
}

// runSessionReaper periodically evicts idle and over-cap A2A sessions,
// tearing down their agent processes. On ctx.Done it shuts down all
// sessions, so no agent process outlives the server.
func (s *a2aServer) runSessionReaper(ctx context.Context) {
	ticker := time.NewTicker(a2aSessionReapEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.shutdownSessions()
			return
		case <-ticker.C:
			s.reapSessionsOnce()
		}
	}
}

// reapSessionsOnce evicts conversations past the idle TTL or the turn cap,
// and the oldest live sessions beyond the cap. Busy sessions (mid-Prompt)
// are skipped atomically — they stay in the map and are re-visited on the
// next tick, so eviction never races a turn and never lands mid-Prompt.
func (s *a2aServer) reapSessionsOnce() {
	s.mu.Lock()
	now := time.Now()

	// Phase 1 (under lock): collect candidates.
	expired := s.collectExpiredSessions()
	live := len(s.sessions) - len(expired)
	candidates := expired
	if live > a2aSessionMaxSessions {
		excluded := make(map[string]bool, len(candidates))
		for _, e := range candidates {
			excluded[e.id] = true
		}
		// live-a2aSessionMaxSessions is the overflow COUNT to evict;
		// collectOverflowSessions receives len(candidates) as the count
		// already queued so its inner live matches this outer live.
		candidates = append(candidates, s.collectOverflowSessions(len(candidates), excluded)...)
	}

	// Phase 2 (still under lock): TryLock each candidate's promptMu; only
	// non-busy sessions are deleted from the map and queued for teardown.
	// Busy sessions simply stay in the map for the next tick.
	var evicted []a2aEvictEntry
	var deferred []string
	for _, e := range candidates {
		if !e.sess.promptMu.TryLock() {
			deferred = append(deferred, e.id)
			continue
		}
		e.sess.promptMu.Unlock()
		delete(s.sessions, e.id)
		evicted = append(evicted, e)
	}
	s.mu.Unlock()

	// Phase 3 (no locks): tear down the evicted agents.
	for _, e := range evicted {
		if e.sess.agent != nil && e.sess.agent.cleanup != nil {
			e.sess.agent.cleanup()
		}
		if e.sess.sessionCancel != nil {
			e.sess.sessionCancel()
		}
		slog.Info("a2a session reaped",
			logKeyCtxID, e.id,
			"turns", e.sess.turnCount,
			"idle", now.Sub(e.sess.lastUsed).Round(time.Second),
			logKeyAction, "a2a_session_reap")
	}
	for _, id := range deferred {
		slog.Info("a2a session busy, deferring reap",
			logKeyCtxID, id,
			logKeyAction, "a2a_session_reap_deferred")
	}
}

// a2aEvictEntry pairs a contextId with its session for reaping.
type a2aEvictEntry struct {
	id   string
	sess *a2aSession
}

// collectExpiredSessions (called with s.mu held) returns live sessions past
// the idle TTL or the turn cap.
func (s *a2aServer) collectExpiredSessions() []a2aEvictEntry {
	var out []a2aEvictEntry
	now := time.Now()
	for id, sess := range s.sessions {
		if sess == nil || isSpawning(sess) {
			continue // spawn-in-progress: the reaper never touches it
		}
		if now.Sub(sess.lastUsed) > a2aSessionIdleTTL || sess.turnCount >= a2aSessionTurnCap {
			out = append(out, a2aEvictEntry{id, sess})
		}
	}
	return out
}

// collectOverflowSessions (called with s.mu held) returns the
// oldest-by-last-used live sessions to evict when the count exceeds the
// cap. alreadyEvicted is the count already queued; excluded holds the ids
// already queued (skipped by this pass).
func (s *a2aServer) collectOverflowSessions(alreadyEvicted int, excluded map[string]bool) []a2aEvictEntry {
	live := len(s.sessions) - alreadyEvicted
	if live <= a2aSessionMaxSessions {
		return nil
	}
	var liveSet []a2aEvictEntry
	for id, sess := range s.sessions {
		if sess == nil || isSpawning(sess) || excluded[id] {
			continue
		}
		liveSet = append(liveSet, a2aEvictEntry{id, sess})
	}
	sort.Slice(liveSet, func(i, j int) bool { return liveSet[i].sess.lastUsed.Before(liveSet[j].sess.lastUsed) })
	var out []a2aEvictEntry
	for i := 0; i < live-a2aSessionMaxSessions && i < len(liveSet); i++ {
		out = append(out, liveSet[i])
	}
	return out
}

// shutdownSessions tears down every live session (server shutdown path).
func (s *a2aServer) shutdownSessions() {
	s.mu.Lock()
	sessions := make([]*a2aSession, 0, len(s.sessions))
	for id, sess := range s.sessions {
		if sess == nil || isSpawning(sess) {
			continue
		}
		sessions = append(sessions, sess)
		delete(s.sessions, id)
	}
	s.mu.Unlock()
	for _, sess := range sessions {
		if sess.agent != nil && sess.agent.cleanup != nil {
			sess.agent.cleanup()
		}
		if sess.sessionCancel != nil {
			sess.sessionCancel()
		}
	}
}

func (*a2aServer) writeResult(w http.ResponseWriter, id json.RawMessage, result *a2aTaskJSON) {
	w.Header().Set(headerContentType, contentTypeJSON)
	_ = json.NewEncoder(w).Encode(a2aResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (*a2aServer) writeError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	w.Header().Set(headerContentType, contentTypeJSON)
	_ = json.NewEncoder(w).Encode(a2aResponse{JSONRPC: "2.0", ID: id, Error: &a2aError{Code: code, Message: message}})
}

// a2aTaskClient implements acp.Client for one A2A task: it accumulates agent
// message chunks into the task's final text, auto-denies permissions, and
// denies filesystem/terminal access (the agent should use wiki MCP tools).
type a2aTaskClient struct {
	task *a2aTask

	// progressSink, when non-nil, receives the accumulated text after
	// each chunk append (buffered chan, 1-deep, non-blocking) so the
	// server can mirror in-flight output onto the task record for
	// tasks/get progress. Drained by a server-side goroutine per task.
	progressSink chan<- string

	mu   sync.Mutex
	text strings.Builder
}

// SessionUpdate implements acp.Client. It accumulates agent message chunks;
// the accumulated text becomes the task's final result. Chunks also feed
// progressSink (non-blocking) for live progress mirroring.
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
	if c.progressSink != nil {
		select {
		case c.progressSink <- c.text.String():
		default: // server-side mirror goroutine is behind; it will catch up
		}
	}
	return nil
}

// RequestPermission implements acp.Client. A2A tasks are non-interactive —
// auto-deny so the agent finishes deterministically.
func (*a2aTaskClient) RequestPermission(_ context.Context, _ acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
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
