package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	acp "github.com/coder/acp-go-sdk"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1connect"
)

// scheduledTurnPreamble is prepended to every scheduled-agent prompt. Tells
// the agent it is a one-shot background task, not an interactive chat, and
// MUST close out by recording a summary so chat users can see what happened.
const scheduledTurnPreamble = `You are a scheduled background agent acting on wiki page %q. This is NOT an interactive chat. ` +
	`Complete the scheduled task below using wiki MCP tools as needed, then stop. Do NOT ask clarifying questions. ` +
	`Before finishing, call api_v1_AgentMetadataService_AppendBackgroundActivitySummary with a concise one-sentence ` +
	`summary of what you did — this is the only way a user chatting on this page later will know what happened.

## Scheduled task

%s`

// scheduledTurnClient implements the acp.Client interface for one ephemeral
// scheduled-turn instance. It counts AgentMessageChunk callbacks and cancels
// the prompt context when the count reaches maxTurns.
type scheduledTurnClient struct {
	page         string
	maxTurns     int32
	turnCount    atomic.Int32
	cancelPrompt context.CancelFunc

	mu       sync.Mutex
	hitLimit bool
}

// newScheduledTurnClient constructs a client. cancelPrompt is the function
// returned by the context.WithCancel that wraps the Prompt call.
func newScheduledTurnClient(page string, maxTurns int32, cancelPrompt context.CancelFunc) *scheduledTurnClient {
	return &scheduledTurnClient{
		page:         page,
		maxTurns:     maxTurns,
		cancelPrompt: cancelPrompt,
	}
}

// SessionUpdate implements acp.Client. It counts agent message chunks and
// triggers cancellation when the count reaches maxTurns.
func (c *scheduledTurnClient) SessionUpdate(_ context.Context, n acp.SessionNotification) error {
	if n.Update.AgentMessageChunk == nil {
		return nil
	}
	count := c.turnCount.Add(1)
	if c.maxTurns > 0 && count >= c.maxTurns {
		c.mu.Lock()
		alreadyTriggered := c.hitLimit
		c.hitLimit = true
		c.mu.Unlock()
		if !alreadyTriggered {
			slog.Warn("scheduled turn: max_turns reached, cancelling prompt",
				logKeyPage, c.page, "max_turns", c.maxTurns)
			c.cancelPrompt()
		}
	}
	return nil
}

// HitLimit returns true if max_turns was reached during this turn.
func (c *scheduledTurnClient) HitLimit() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hitLimit
}

// RequestPermission implements acp.Client. Scheduled turns must not block on
// user permission — auto-deny so the agent finishes deterministically.
func (*scheduledTurnClient) RequestPermission(_ context.Context, _ acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	// Use the cancelled outcome so the agent knows the request was refused.
	return permissionCancelledResponse(), nil
}

// ReadTextFile implements acp.Client. Filesystem access is disabled.
func (*scheduledTurnClient) ReadTextFile(_ context.Context, _ acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, errors.New("file system access not available in scheduled turns")
}

// WriteTextFile implements acp.Client. Filesystem access is disabled.
func (*scheduledTurnClient) WriteTextFile(_ context.Context, _ acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, errors.New("file system access not available in scheduled turns")
}

// CreateTerminal implements acp.Client. Terminal access is disabled.
func (*scheduledTurnClient) CreateTerminal(_ context.Context, _ acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, errors.New(errTerminalAccessUnavailable)
}

// TerminalOutput implements acp.Client. Terminal access is disabled.
func (*scheduledTurnClient) TerminalOutput(_ context.Context, _ acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, errors.New(errTerminalAccessUnavailable)
}

// ReleaseTerminal implements acp.Client. Terminal access is disabled.
func (*scheduledTurnClient) ReleaseTerminal(_ context.Context, _ acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, errors.New(errTerminalAccessUnavailable)
}

// WaitForTerminalExit implements acp.Client. Terminal access is disabled.
func (*scheduledTurnClient) WaitForTerminalExit(_ context.Context, _ acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, errors.New(errTerminalAccessUnavailable)
}

// KillTerminal implements acp.Client. Terminal access is disabled.
func (*scheduledTurnClient) KillTerminal(_ context.Context, _ acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, errors.New(errTerminalAccessUnavailable)
}

// runScheduledTurnLoop subscribes to the wiki's ScheduledTurnService stream
// and dispatches each request to runScheduledTurn. It reconnects on transient
// errors using the existing backoff helper.
func (d *poolDaemon) runScheduledTurnLoop(ctx context.Context) {
	backoffMs := initialBackoffMs

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		start := time.Now()
		err := d.subscribeScheduledTurns(ctx)
		if err == nil {
			return
		}

		delayMs, nextMs := computeBackoffAfterFailure(backoffMs, time.Since(start))
		slog.Warn("scheduled-turn subscription error, reconnecting",
			logKeyError, err, "delay_ms", delayMs, logKeyAction, "reconnect")

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(delayMs) * time.Millisecond):
		}
		backoffMs = nextMs
	}
}

// subscribeScheduledTurns opens the ScheduledTurnService stream and dispatches
// each ScheduledTurnRequest to its own goroutine. Returns the underlying
// stream error if the connection drops.
func (d *poolDaemon) subscribeScheduledTurns(ctx context.Context) error {
	httpClient := newAgentAwareHTTPClient(nil)
	client := apiv1connect.NewScheduledTurnServiceClient(httpClient, d.wikiURL)

	stream, err := client.SubscribeScheduledTurns(ctx, connect.NewRequest(&apiv1.SubscribeScheduledTurnsRequest{}))
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("subscribe scheduled turns: %w", err)
	}
	defer func() { _ = stream.Close() }()

	slog.Info("connected to wiki for scheduled turns", logKeyAction, "scheduled_turns_connected")

	for stream.Receive() {
		req := stream.Msg()
		go d.runScheduledTurn(ctx, client, req)
	}

	if streamErr := stream.Err(); streamErr != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("scheduled-turn stream error: %w", streamErr)
	}
	if ctx.Err() != nil {
		return nil
	}
	return errors.New("scheduled-turn stream closed by server")
}

// runScheduledTurn handles one scheduled-turn request end-to-end: spawn an
// ephemeral ACP instance outside the d.instances pool, send a single Prompt
// with the scheduled-turn preamble, count AgentMessageChunk callbacks for
// max_turns enforcement, then complete the turn back to the wiki regardless
// of outcome.
func (d *poolDaemon) runScheduledTurn(ctx context.Context, completer apiv1connect.ScheduledTurnServiceClient, req *apiv1.ScheduledTurnRequest) {
	start := time.Now()
	slog.Info("scheduled turn: starting",
		logKeyPage, req.GetPage(),
		"request_id", req.GetRequestId(),
		"max_turns", req.GetMaxTurns())

	terminalStatus, errMsg := d.executeScheduledTurn(ctx, req)

	duration := time.Since(start)
	completeReq := &apiv1.CompleteScheduledTurnRequest{
		RequestId:       req.GetRequestId(),
		TerminalStatus:  terminalStatus,
		ErrorMessage:    errMsg,
		DurationSeconds: int32(duration / time.Second),
	}
	if _, completeErr := completer.CompleteScheduledTurn(ctx, connect.NewRequest(completeReq)); completeErr != nil {
		slog.Error("scheduled turn: complete failed",
			logKeyPage, req.GetPage(),
			"request_id", req.GetRequestId(),
			logKeyError, completeErr)
	}
	slog.Info("scheduled turn: finished",
		logKeyPage, req.GetPage(),
		"request_id", req.GetRequestId(),
		"terminal", terminalStatus.String(),
		"duration_seconds", completeReq.DurationSeconds)
}

// executeScheduledTurn does the real work: spawn a fresh ephemeral ACP
// instance (NOT registered in d.instances), send the preamble + prompt, watch
// for max_turns, tear down. Returns the terminal status to report.
func (d *poolDaemon) executeScheduledTurn(ctx context.Context, req *apiv1.ScheduledTurnRequest) (apiv1.ScheduleStatus, string) {
	turnCtx, cancelTurn := context.WithCancel(ctx)
	defer cancelTurn()

	conn, cleanup, spawnErr := d.spawnEphemeralForScheduledTurn(turnCtx, req.GetPage(), req.GetRequestId(), req.GetMaxTurns(), cancelTurn)
	if spawnErr != nil {
		return apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR, fmt.Sprintf("spawn failed: %v", spawnErr)
	}
	defer cleanup()

	promptText := fmt.Sprintf(scheduledTurnPreamble, req.GetPage(), req.GetPrompt())
	_, promptErr := conn.connection.Prompt(turnCtx, acp.PromptRequest{
		SessionId: conn.sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(promptText)},
	})

	if conn.client.HitLimit() {
		return apiv1.ScheduleStatus_SCHEDULE_STATUS_TIMEOUT, fmt.Sprintf("max_turns (%d) reached", req.GetMaxTurns())
	}
	if promptErr != nil {
		// If we cancelled because the parent context was done (process
		// shutdown, etc.), report that as ERROR with a clear message.
		if ctx.Err() != nil {
			return apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR, "shutting down before turn completed"
		}
		return apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR, fmt.Sprintf("prompt failed: %v", promptErr)
	}
	return apiv1.ScheduleStatus_SCHEDULE_STATUS_OK, ""
}

// scheduledEphemeralConnection bundles the bits the caller needs to drive one
// ephemeral instance: the ACP connection, the session id, and the
// scheduledTurnClient (for HitLimit inspection after the prompt returns).
type scheduledEphemeralConnection struct {
	connection *acp.ClientSideConnection
	sessionID  acp.SessionId
	client     *scheduledTurnClient
}

// spawnEphemeralForScheduledTurn spawns a one-shot ACP agent for a scheduled
// turn. Unlike the chat path, the resulting instance is NOT added to
// d.instances — it lives only for the duration of one Prompt call.
//
// Each ephemeral spawn uses a UNIQUE systemd unit name
// ("wiki-scheduled-<page>-<short-request-id>") so concurrent fires and
// collisions with the per-page interactive chat unit ("wiki-chat-<page>")
// are avoided. The earlier code reused buildAgentCmd which always
// stop+restart's wiki-chat-<page>.scope — that killed both interactive
// chat instances and competing scheduled turns mid-turn.
func (d *poolDaemon) spawnEphemeralForScheduledTurn(ctx context.Context, page, requestID string, maxTurns int32, cancelTurn context.CancelFunc) (*scheduledEphemeralConnection, func(), error) {
	cmd := d.buildScheduledTurnAgentCmd(ctx, page, requestID)
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, func() {}, fmt.Errorf("stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, func() {}, fmt.Errorf("stdout pipe: %w", err)
	}
	if startErr := cmd.Start(); startErr != nil {
		return nil, func() {}, fmt.Errorf("start agent: %w", startErr)
	}

	cleanup := func() {
		// Closing stdin is the polite way to ask the agent to exit; if it does
		// not, the cancelled context will eventually kill it via
		// CommandContext.
		_ = stdinPipe.Close()
		// Best-effort wait so we don't leak zombies. The wait races with the
		// kill, which is fine.
		go func() {
			_ = cmd.Wait()
		}()
	}

	client := newScheduledTurnClient(page, maxTurns, cancelTurn)
	conn := acp.NewClientSideConnection(client, stdinPipe, stdoutPipe)

	if _, initErr := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{
				ReadTextFile:  false,
				WriteTextFile: false,
			},
			Terminal: false,
		},
	}); initErr != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("ACP handshake: %w", initErr)
	}

	sess, sessionErr := d.initializeACPSession(ctx, conn, page)
	if sessionErr != nil {
		cleanup()
		return nil, func() {}, sessionErr
	}

	return &scheduledEphemeralConnection{
		connection: conn,
		sessionID:  sess.SessionId,
		client:     client,
	}, cleanup, nil
}

// scheduledTurnUnitPrefix is the unit-name prefix for ephemeral
// scheduled-turn systemd scope units. Distinct from the chat path's
// "wiki-chat-<page>" prefix so concurrent scheduled turns and interactive
// chats on the same page don't kill each other.
const scheduledTurnUnitPrefix = "wiki-scheduled-"

// scheduledTurnRequestIDInUnit is how many characters of the request_id we
// embed in the systemd unit name. Full UUIDs would push the unit name past
// the 256-char systemd limit on some pages.
const scheduledTurnRequestIDInUnit = 8

// buildScheduledTurnAgentCmd constructs the exec.Cmd for one ephemeral
// scheduled-turn agent. Mirrors buildAgentCmd's systemd vs. plain exec
// behavior, with two important differences:
//
//   - Unit name is "wiki-scheduled-<sanitized-page>-<short-request-id>"
//     (unique per fire) so concurrent scheduled turns and the per-page
//     interactive chat unit ("wiki-chat-<page>") cannot collide.
//   - The working directory is pinned to the daemon's cwd so the
//     ephemeral agent inherits the same shell env (1Password tokens,
//     ~/.claude config, etc.) the long-lived interactive instances see.
func (d *poolDaemon) buildScheduledTurnAgentCmd(ctx context.Context, page, requestID string) *exec.Cmd {
	shortID := requestID
	if len(shortID) > scheduledTurnRequestIDInUnit {
		shortID = shortID[:scheduledTurnRequestIDInUnit]
	}
	if d.useSystemd {
		unitName := scheduledTurnUnitPrefix + sanitizeUnitName(page) + "-" + shortID
		// Pin cwd so 1Password / claude config / agent CLAUDE.md are found
		// the same way the interactive chat instances find them.
		cwd, _ := os.Getwd()
		args := []string{
			"--user",
			"--unit=" + unitName,
			"--scope",
		}
		if cwd != "" {
			args = append(args, "--working-directory="+cwd)
		}
		args = append(args, d.agentPath)
		return exec.CommandContext(ctx, "systemd-run", args...)
	}
	cmd := exec.CommandContext(ctx, d.agentPath)
	cmd.Stderr = newPrefixWriter(os.Stderr, page)
	return cmd
}
