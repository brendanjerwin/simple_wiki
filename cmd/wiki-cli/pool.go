package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"connectrpc.com/connect"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1connect"
	cli "gopkg.in/urfave/cli.v1"
)

const (
	defaultMaxInstances        = 5
	defaultIdleTimeoutMinutes  = 30
)

// instanceEntry tracks a running Claude Code instance for a page.
type instanceEntry struct {
	page       string
	unitName   string
	cmd        *exec.Cmd
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

// poolDaemon manages per-page Claude Code instances.
type poolDaemon struct {
	wikiURL      string
	claudePath   string
	maxInstances int
	idleTimeout  time.Duration
	useSystemd   bool

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
		Usage: "Manage per-page Claude Code instances",
		Description: `Runs a pool daemon that subscribes to instance requests from the wiki server
and spawns dedicated Claude Code instances per page on demand.

Each instance gets its own wiki-cli MCP subprocess scoped to a specific page,
with the page's frontmatter ai_agent_chat_context injected as initial context.

When running under systemd, instances are spawned as transient units for
per-page journal logging (journalctl -u wiki-chat-<page>).

Example:
  wiki-cli pool --url https://wiki.monster-orfe.ts.net --max-instances 5

The daemon should be run in a directory containing your Claude agent configuration
(CLAUDE.md, agent files, etc.) as Claude Code will use that directory's context.`,
		Flags: []cli.Flag{
			urlFlag,
			cli.IntFlag{
				Name:  "max-instances",
				Value: defaultMaxInstances,
				Usage: "Maximum concurrent Claude instances",
			},
			cli.DurationFlag{
				Name:  "idle-timeout",
				Value: defaultIdleTimeoutMinutes * time.Minute,
				Usage: "Reclaim idle instances after this duration",
			},
			cli.StringFlag{
				Name:  "claude-path",
				Value: "claude",
				Usage: "Path to claude binary",
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
				log.Println("Systemd detected — instances will be spawned as transient units")
			} else {
				log.Println("Systemd not available or disabled — using direct process management")
			}

			d := &poolDaemon{
				wikiURL:      normalizedURL,
				claudePath:   c.String("claude-path"),
				maxInstances: c.Int("max-instances"),
				idleTimeout:  c.Duration("idle-timeout"),
				useSystemd:   useSystemd,
				instances:    make(map[string]*instanceEntry),
			}

			d.ctx = context.Background()
			return d.run(d.ctx)
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

// ensureInstance spawns a Claude instance for a page if one doesn't already exist.
func (d *poolDaemon) ensureInstance(page string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Already running?
	if entry, ok := d.instances[page]; ok {
		entry.touch()
		log.Printf("Instance for %q already running — updated lastActive", page)
		return
	}

	// Spawn new instance first, then evict if needed — avoids dropping a working
	// instance when the spawn fails.
	entry, err := d.spawnInstance(page)
	if err != nil {
		log.Printf("Failed to spawn instance for %q: %v", page, err)
		return
	}

	// At capacity? Evict least recently active to make room.
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

// spawnInstance starts a Claude Code process for a page.
// Must be called with d.mu held.
func (d *poolDaemon) spawnInstance(page string) (*instanceEntry, error) {
	ctx, cancel := context.WithCancel(d.ctx)

	// Build wiki-cli mcp command for the --mcp-server flag
	wikiCLIBin, err := os.Executable()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to find wiki-cli binary: %w", err)
	}

	mcpServerCmd := fmt.Sprintf("%s mcp --url %s --page %s", wikiCLIBin, d.wikiURL, page)

	entry := &instanceEntry{
		page:       page,
		lastActive: time.Now(),
		cancel:     cancel,
	}

	if d.useSystemd {
		entry.unitName = "wiki-chat-" + sanitizeUnitName(page)
		entry.cmd = exec.CommandContext(ctx, "systemd-run",
			"--user",
			"--unit="+entry.unitName,
			"--scope",
			d.claudePath,
			"--channels", "wiki-channel:wiki-channel",
			"--mcp-server", mcpServerCmd,
		)
	} else {
		entry.cmd = exec.CommandContext(ctx, d.claudePath,
			"--channels", "wiki-channel:wiki-channel",
			"--mcp-server", mcpServerCmd,
		)
		// In non-systemd mode, prefix stdout/stderr with page name
		entry.cmd.Stdout = newPrefixWriter(os.Stdout, page)
		entry.cmd.Stderr = newPrefixWriter(os.Stderr, page)
	}

	if err := entry.cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start claude for page %q: %w", page, err)
	}

	// Monitor process exit in background
	go func() {
		waitErr := entry.cmd.Wait()
		if waitErr != nil {
			log.Printf("Claude instance for %q exited: %v", page, waitErr)
		} else {
			log.Printf("Claude instance for %q exited cleanly", page)
		}

		d.mu.Lock()
		// Only remove if this is still the same entry (not replaced by a new spawn)
		if current, ok := d.instances[page]; ok && current == entry {
			delete(d.instances, page)
		}
		d.mu.Unlock()
	}()

	return entry, nil
}

// stopInstanceLocked stops an instance. Must be called with d.mu held.
func (d *poolDaemon) stopInstanceLocked(page string) {
	entry, ok := d.instances[page]
	if !ok {
		return
	}

	entry.cancel()

	// In systemd mode, also stop the transient unit to prevent orphaned processes
	if entry.unitName != "" {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		if err := exec.CommandContext(stopCtx, "systemctl", "--user", "stop", entry.unitName+".scope").Run(); err != nil {
			log.Printf("Failed to stop systemd unit %q: %v", entry.unitName, err)
		}
	}

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

