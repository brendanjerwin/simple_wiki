package bootstrap

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"connectrpc.com/grpcreflect"
	"connectrpc.com/vanguard"

	"github.com/brendanjerwin/simple_wiki/internal/caldav"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	keepsync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/sync"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	googletasks "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks"
	tasksgateway "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
	taskssync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/sync"
	grpcapi "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	wikimcp "github.com/brendanjerwin/simple_wiki/internal/mcp"
	"github.com/brendanjerwin/simple_wiki/internal/observability"
	"github.com/brendanjerwin/simple_wiki/migrations/eager"
	"github.com/brendanjerwin/simple_wiki/pkg/chatbuffer"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/gin-gonic/gin"
	"github.com/jcelliott/lumber"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

const (
	networkTCP = "tcp"

	// metricsFlushCronExpression schedules wiki metrics persistence.
	// Format: second minute hour day-of-month month day-of-week
	// This runs every 5 seconds.
	metricsFlushCronExpression = "*/5 * * * * *"

	errCreateHandlerFmt  = "failed to create handler: %w"
	errCreateListenerFmt = "failed to create HTTP listener: %w"
)

// ServerResult holds the created server and listener.
type ServerResult struct {
	MainServer     *http.Server
	MainListener   net.Listener
	RedirectServer *http.Server // Optional: HTTP->HTTPS redirect server (ModeFullTLS only)
	Cleanup        func()       // Cleanup function to call on shutdown (e.g., persist final metrics)
}

// DetermineServerMode determines the appropriate server mode based on Tailscale status.
//
//revive:disable-next-line:flag-parameter useTailscaleServe is a CLI configuration flag
func DetermineServerMode(tsStatus *tailscale.Status, useTailscaleServe bool) ServerMode {
	if tsStatus == nil || !tsStatus.Available || tsStatus.DNSName == "" {
		return ModePlainHTTP
	}
	if useTailscaleServe {
		return ModeTailscaleServe
	}
	return ModeFullTLS
}

// composeCleanup combines two cleanup functions into one. Either may be nil.
func composeCleanup(first, second func()) func() {
	return func() {
		if first != nil {
			first()
		}
		if second != nil {
			second()
		}
	}
}

// stopSiteCron returns a cleanup function that stops the site's cron scheduler
// (waiting for in-flight jobs) so scheduled-agent turns terminate cleanly on
// shutdown. Returns a no-op when the scheduler is nil (defensive — can happen
// in unusual test setups).
func stopSiteCron(site *server.Site) func() {
	return func() {
		if site == nil || site.CronScheduler == nil {
			return
		}
		site.CronScheduler.Stop()
	}
}

// SetupPlainHTTP creates a plain HTTP server without Tailscale integration.
func SetupPlainHTTP(
	httpAddr string,
	site *server.Site,
	logger *lumber.ConsoleLogger,
	commit string,
	buildTime time.Time,
) (*ServerResult, error) {
	logger.Info("Tailscale not available. Running as plain HTTP on %s", httpAddr)
	logger.Info("For secure access with user identity, install Tailscale: https://tailscale.com/download")

	handler, metricsCleanup, err := createMultiplexedHandler(site, logger, commit, buildTime, nil)
	if err != nil {
		return nil, fmt.Errorf(errCreateHandlerFmt, err)
	}

	httpListener, err := net.Listen(networkTCP, httpAddr)
	if err != nil {
		return nil, fmt.Errorf(errCreateListenerFmt, err)
	}

	return &ServerResult{
		MainServer: &http.Server{
			Handler: h2c.NewHandler(handler, &http2.Server{}),
		},
		MainListener: httpListener,
		Cleanup:      composeCleanup(metricsCleanup, stopSiteCron(site)),
	}, nil
}

// SetupTailscaleServe creates an HTTP server with Tailscale identity support.
// Used when Tailscale Serve handles TLS termination externally.
//
//revive:disable-next-line:flag-parameter forceRedirectToHTTPS is a CLI configuration flag
func SetupTailscaleServe(
	httpAddr string,
	tsDNSName string,
	forceRedirectToHTTPS bool,
	site *server.Site,
	logger *lumber.ConsoleLogger,
	commit string,
	buildTime time.Time,
	agentTags []string,
) (*ServerResult, error) {
	logger.Info("Tailscale detected: %s", tsDNSName)
	logger.Info("Tailscale Serve mode. Running HTTP on %s with identity support", httpAddr)

	identityResolver := tailscale.NewIdentityResolver(agentTags)
	handler, metricsCleanup, err := createMultiplexedHandler(site, logger, commit, buildTime, identityResolver)
	if err != nil {
		return nil, fmt.Errorf(errCreateHandlerFmt, err)
	}

	httpListener, err := net.Listen(networkTCP, httpAddr)
	if err != nil {
		return nil, fmt.Errorf(errCreateListenerFmt, err)
	}

	finalHandler := h2c.NewHandler(handler, &http2.Server{})
	if forceRedirectToHTTPS {
		logger.Info("Tailnet clients will be redirected to HTTPS")
		redirector, err := tailscale.NewTailnetRedirector(tsDNSName, tailscale.DefaultHTTPSPort, identityResolver, finalHandler, true, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create tailnet redirector: %w", err)
		}
		finalHandler = redirector
	}

	return &ServerResult{
		MainServer: &http.Server{
			Handler: finalHandler,
		},
		MainListener: httpListener,
		Cleanup:      composeCleanup(metricsCleanup, stopSiteCron(site)),
	}, nil
}

// SetupFullTLS creates both HTTPS and HTTP servers using Tailscale certificates.
// The HTTP server redirects tailnet clients to HTTPS.
func SetupFullTLS(
	httpAddr string,
	tlsPort int,
	tsDNSName string,
	site *server.Site,
	logger *lumber.ConsoleLogger,
	commit string,
	buildTime time.Time,
	agentTags []string,
) (*ServerResult, error) {
	logger.Info("Tailscale detected: %s", tsDNSName)

	identityResolver := tailscale.NewIdentityResolver(agentTags)
	handler, metricsCleanup, err := createMultiplexedHandler(site, logger, commit, buildTime, identityResolver)
	if err != nil {
		return nil, fmt.Errorf(errCreateHandlerFmt, err)
	}

	// Parse host from httpAddr
	host := ""
	if idx := strings.LastIndex(httpAddr, ":"); idx != -1 {
		host = httpAddr[:idx]
	}

	tlsProvider := tailscale.NewTLSProvider()
	tlsConfig := tlsProvider.GetTLSConfig()
	httpsAddr := fmt.Sprintf("%s:%d", host, tlsPort)

	tlsListener, err := tls.Listen(networkTCP, httpsAddr, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS listener: %w", err)
	}

	logger.Info("HTTPS server listening on %s", httpsAddr)

	// Create HTTP redirect server
	redirector, err := tailscale.NewTailnetRedirector(tsDNSName, tlsPort, identityResolver, h2c.NewHandler(handler, &http2.Server{}), false, logger)
	if err != nil {
		if closeErr := tlsListener.Close(); closeErr != nil {
			logger.Error("failed to close TLS listener: %v", closeErr)
		}
		return nil, fmt.Errorf("failed to create tailnet redirector: %w", err)
	}
	httpListener, err := net.Listen(networkTCP, httpAddr)
	if err != nil {
		if closeErr := tlsListener.Close(); closeErr != nil {
			logger.Error("failed to close TLS listener: %v", closeErr)
		}
		return nil, fmt.Errorf(errCreateListenerFmt, err)
	}

	logger.Info("HTTP server listening on %s (redirects tailnet, serves others)", httpAddr)

	result := &ServerResult{
		MainServer: &http.Server{
			Handler: handler,
		},
		MainListener:  tlsListener,
		Cleanup:       composeCleanup(metricsCleanup, stopSiteCron(site)),
		RedirectServer: &http.Server{
			Addr:    httpAddr,
			Handler: redirector,
		},
	}

	// Start redirect server in background
	go func() {
		if err := result.RedirectServer.Serve(httpListener); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP redirect server error: %v", err)
		}
	}()

	return result, nil
}

// createHTTPObservabilityMiddleware creates HTTP observability middleware.
func createHTTPObservabilityMiddleware(counters observability.RequestCounter, logger *lumber.ConsoleLogger) gin.HandlerFunc {
	httpMetrics, err := observability.NewHTTPMetrics()
	if err != nil {
		logger.Warn("Failed to create HTTP metrics, continuing without: %v", err)
	}
	httpInstrumentation := observability.NewHTTPInstrumentation(httpMetrics, counters)
	return httpInstrumentation.GinMiddleware()
}

// buildGRPCInterceptors creates the gRPC interceptor chains for observability and identity.
func buildGRPCInterceptors(
	identityResolver tailscale.IdentityResolver,
	loggingInterceptor grpc.UnaryServerInterceptor,
	counters observability.RequestCounter,
	logger *lumber.ConsoleLogger,
) ([]grpc.UnaryServerInterceptor, []grpc.StreamServerInterceptor, error) {
	var unaryInterceptors []grpc.UnaryServerInterceptor
	var streamInterceptors []grpc.StreamServerInterceptor

	grpcMetrics, err := observability.NewGRPCMetrics()
	if err != nil {
		logger.Warn("Failed to create gRPC metrics, continuing without: %v", err)
	}
	grpcInstrumentation := observability.NewGRPCInstrumentation(grpcMetrics, counters)
	unaryInterceptors = append(unaryInterceptors, grpcInstrumentation.UnaryServerInterceptor())
	streamInterceptors = append(streamInterceptors, grpcInstrumentation.StreamServerInterceptor())

	if identityResolver != nil {
		unaryIdentity, err := tailscale.IdentityInterceptor(identityResolver, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create unary identity interceptor: %w", err)
		}
		unaryInterceptors = append(unaryInterceptors, unaryIdentity)

		streamIdentity, err := tailscale.IdentityStreamInterceptor(identityResolver, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create stream identity interceptor: %w", err)
		}
		streamInterceptors = append(streamInterceptors, streamIdentity)
	}
	unaryInterceptors = append(unaryInterceptors, loggingInterceptor)

	return unaryInterceptors, streamInterceptors, nil
}

func createMultiplexedHandler(
	site *server.Site,
	logger *lumber.ConsoleLogger,
	commit string,
	buildTime time.Time,
	identityResolver tailscale.IdentityResolver,
) (http.Handler, func(), error) {
	// Create counters first so middleware can use them
	counters, metricsCleanup := setupWikiMetrics(site, logger)

	// Build middleware list to pass to GinRouter (added before routes)
	var middleware []gin.HandlerFunc
	middleware = append(middleware, createHTTPObservabilityMiddleware(counters, logger))

	if identityResolver != nil {
		// RequestCounter implements tailscale.MetricsRecorder since both now use observability types
		identityMW, err := tailscale.IdentityMiddlewareWithMetrics(identityResolver, logger, counters)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create identity middleware: %w", err)
		}
		middleware = append(middleware, identityMW)
	}

	// Create router with middleware already attached (before routes)
	ginRouter := site.GinRouter(middleware...)

	grpcServer, grpcAPIServer, err := setupGRPCServer(site, commit, buildTime, identityResolver, counters, logger)
	if err != nil {
		return nil, nil, err
	}

	transcoder, err := BuildVanguardTranscoder(grpcServer, ginRouter)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create vanguard transcoder: %w", err)
	}

	mcpHandler, err := wikimcp.NewStreamableHTTPHandler(grpcAPIServer, commit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create MCP handler: %w", err)
	}

	// Wrap the MCP handler with Tailscale identity middleware so that MCP tool calls
	// receive the same identity context as Gin routes and gRPC requests. Without this,
	// IdentityFromContext would always return Anonymous for MCP callers.
	// Note: the gRPC interceptors for logging and observability metrics still do not
	// apply to in-process MCP tool calls (known limitation — MCP calls go directly to
	// the service implementation, bypassing the gRPC transport layer).
	if identityResolver != nil {
		mcpHandler, err = tailscale.IdentityHTTPMiddlewareWithMetrics(identityResolver, logger, counters, mcpHandler)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create MCP identity middleware: %w", err)
		}
	}

	outerMux := http.NewServeMux()
	// NOTE: The /mcp endpoint is mounted without authentication middleware.
	// This is consistent with the gRPC API handlers which also do not enforce
	// authorization beyond identity injection. If Tailscale identity enforcement
	// is added to gRPC handlers in the future, it should also be applied here.
	outerMux.Handle("/mcp", mcpHandler)
	outerMux.Handle("/", transcoder)

	return outerMux, metricsCleanup, nil
}

// setupWikiMetrics creates and configures the wiki metrics recorder.
// Returns a non-nil RequestCounter and a cleanup function for shutdown.
// The cleanup function should be called during application shutdown to persist final metrics.
// Returns empty composite with no-op cleanup if recorder creation fails.
func setupWikiMetrics(site *server.Site, logger *lumber.ConsoleLogger) (observability.RequestCounter, func()) {
	wikiRecorder, err := observability.NewWikiMetricsRecorder(
		site, site, site.GetJobQueueCoordinator(), logger,
	)
	if err != nil {
		logger.Warn("Failed to create wiki metrics recorder, metrics disabled: %v", err)
		return observability.NewCompositeRequestCounter(), func() {
			// intentionally empty — no metrics to clean up when recorder creation fails
		}
	}

	counters := observability.NewCompositeRequestCounter(wikiRecorder)

	// Schedule periodic persistence
	_, schedErr := site.CronScheduler.Schedule(metricsFlushCronExpression, &metricsPersistJob{recorder: wikiRecorder})
	if schedErr != nil {
		logger.Warn("Failed to schedule metrics persistence: %v", schedErr)
	} else {
		logger.Info("Scheduled wiki metrics persistence every 5 seconds")
	}

	// Return cleanup function that persists final metrics
	cleanup := func() {
		if shutdownErr := wikiRecorder.Shutdown(); shutdownErr != nil {
			logger.Warn("Error persisting final metrics: %v", shutdownErr)
		}
	}

	return counters, cleanup
}

// BuildVanguardTranscoder creates an http.Handler that multiplexes Connect, gRPC, gRPC-Web,
// ConnectRPC reflection, and regular HTTP traffic. Reflection is routed before vanguard
// because vanguard cannot resolve the proto schema for the reflection meta-service and
// would return 505. Non-RPC requests fall through to the Gin router.
func BuildVanguardTranscoder(grpcServer *grpc.Server, ginRouter http.Handler) (http.Handler, error) {
	// grpcServer only speaks gRPC; tell vanguard to use gRPC+proto on the backend leg.
	grpcOpts := []vanguard.ServiceOption{
		vanguard.WithTargetProtocols(vanguard.ProtocolGRPC),
		vanguard.WithTargetCodecs(vanguard.CodecProto),
	}

	// Single source of truth for service names to avoid drift between Vanguard and reflection.
	serviceNames := []string{
		"api.v1.AgentMetadataService",
		"api.v1.ChatService",
		"api.v1.ChecklistService",
		"api.v1.ConnectorService",
		"api.v1.FileStorageService",
		"api.v1.Frontmatter",
		"api.v1.InventoryManagementService",
		"api.v1.PageImportService",
		"api.v1.PageManagementService",
		"api.v1.ScheduledTurnService",
		"api.v1.SearchService",
		"api.v1.SystemInfoService",
	}

	services := make([]*vanguard.Service, 0, len(serviceNames))
	for _, name := range serviceNames {
		services = append(services, vanguard.NewService(name, grpcServer, grpcOpts...))
	}

	// Non-RPC paths fall through to Gin.
	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ginRouter.ServeHTTP(w, r)
	})

	transcoder, err := vanguard.NewTranscoder(services, vanguard.WithUnknownHandler(fallback))
	if err != nil {
		return nil, err
	}

	// ConnectRPC reflection handlers speak Connect protocol natively over HTTP/1.1 —
	// no vanguard transcoding needed. They must be routed before vanguard because vanguard
	// intercepts all /{service}/{method} paths and would return 505 for unregistered services.
	reflector := grpcreflect.NewStaticReflector(serviceNames...)
	mux := http.NewServeMux()
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))
	mux.Handle("/", transcoder)

	return mux, nil
}

// keepOutboundSyncQueueDepth bounds the per-worker queue for Keep
// outbound sync jobs. 256 is generous for household-scale workloads
// (a handful of bindings each ticking every 30s); we'd only hit it
// during sustained Keep unreachability and the metric would surface
// the backlog before it became a problem.
const keepOutboundSyncQueueDepth = 256

// googleTasksOutboundSyncQueueDepth bounds the per-worker queue for
// Google Tasks outbound sync jobs. Same rationale as Keep; Tasks's
// per-user write quota is well above what household-scale workloads
// produce.
const googleTasksOutboundSyncQueueDepth = 256

// googleTasksHTTPTimeoutSeconds is the timeout the Tasks HTTP client
// uses for both refresh-grant and Tasks REST v1 calls. 30s is generous
// enough for a slow consumer broadband round trip without letting a
// stuck connection wedge a sync worker forever.
const googleTasksHTTPTimeoutSeconds = 30

// googleTasksSyncDebounceWindow is how long the SyncDebouncer batches
// rapid checklist edits before enqueuing a single outbound push.
// Mirrors the Keep bridge's 1500ms — the same trade-off (coalesce
// burst edits, propagate within a couple of seconds).
const googleTasksSyncDebounceWindow = 1500 * time.Millisecond

// setupGRPCServer creates and configures the gRPC server with interceptors.
// It returns both the gRPC transport server and the underlying API server for direct in-process calls.
//
//revive:disable-next-line:function-length
func setupGRPCServer(
	site *server.Site,
	commit string,
	buildTime time.Time,
	identityResolver tailscale.IdentityResolver,
	counters observability.RequestCounter,
	logger *lumber.ConsoleLogger,
) (*grpc.Server, *grpcapi.Server, error) {
	// Create chat buffer manager for per-page chat message storage
	chatBufferMgr := chatbuffer.NewManager()

	grpcAPIServer, err := grpcapi.NewServer(
		grpcapi.BuildInfo{Commit: commit, BuildTime: buildTime},
		site, site.BleveIndexQueryer, site.FrontmatterIndexQueryer,
		logger, chatBufferMgr, site,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}
	checklistMutator := checklistmutator.New(site, checklistmutator.SystemClock{}, ulid.NewSystemGenerator())

	// Cross-connector LeaseTable — the in-memory derived view of which
	// (page, list_name) is currently subscribed and by which connector.
	// Per ADR-0011 this is shared across ALL connectors (Keep, Tasks,
	// future iCloud) so the at-most-one-Subscription invariant holds
	// globally. SignalReady is deferred until after the boot-rebuild
	// fan-out scan below.
	leaseTable := connectors.NewLeaseTable()

	// Keep store — instantiated once and shared between the connector,
	// the cron lister, and the boot-rebuild fan-out below.
	keepBindingStore := keepsync.NewSubscriptionStore(site)

	// Keep connector — Google Keep bridge orchestrator. Per-user state on
	// profile pages (wiki.connectors.google_keep.*) plus a sync scheduler
	// (added separately). Optional — without it ConnectorService's
	// GOOGLE_KEEP branches return a clear "not configured" error.
	keepConnector, err := keepsync.NewConnector(
		keepBindingStore,
		leaseTable,
		http.DefaultClient,
		keepsync.SystemClock{},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create Keep connector: %w", err)
	}
	// TEMP: route Keep API responses through the wiki logger so we can
	// see Changes() response shapes in journalctl while diagnosing the
	// "ListNotes returns empty / CreateList silently fails" report.
	// Strip this once the response-shape question is resolved.
	keepConnector.SetDebugLogger(logger)
	// Outbound sync needs read access to wiki checklists; the mutator
	// satisfies keepsync.ChecklistReader's ListItems signature directly.
	keepConnector.SetChecklistReader(checklistMutator)
	// Register the single-worker outbound-sync queue. One worker per
	// account is the right answer because Keep's targetVersion is
	// global; concurrent pushes on the same account would race the
	// version cursor and force resyncs.
	if err := site.GetJobQueueCoordinator().RegisterQueue(
		keepsync.KeepOutboundSyncJobName, 1, keepOutboundSyncQueueDepth,
	); err != nil {
		return nil, nil, fmt.Errorf("register Keep outbound sync queue: %w", err)
	}
	// Mutator hook: every checklist edit fires a debounced enqueue
	// of a sync job for that (page, listName). Debounce coalesces
	// burst edits (toggling 50 items rapidly) into a single push.
	keepSyncDebouncer := keepsync.NewSyncDebouncer(
		site.GetJobQueueCoordinator(),
		keepConnector,
		site,
		logger,
		1500*time.Millisecond,
	)
	checklistMutator.AddSubscriber(keepSyncDebouncer)
	// Inbound apply: SyncToKeep also pulls Keep state into the wiki.
	// Mutator writes during apply must NOT loop back as fresh sync
	// triggers, so the suppressor wraps the apply window. Mutator
	// satisfies keepsync.ChecklistMutator's For-Sync helpers directly.
	keepConnector.SetChecklistMutator(checklistMutator)
	keepConnector.SetSyncSuppressor(keepSyncDebouncer)
	// Bind enqueues a sync job through the same queue cron + debouncer
	// use, unifying every sync trigger behind Connector.SyncToKeep.
	keepConnector.SetJobEnqueuer(site.GetJobQueueCoordinator())
	// Cron tick fires every 30s: enumerate active bindings via a
	// fresh index scan + profile decode each tick (handles pre-existing
	// bindings that weren't bound during this process's lifetime),
	// then enqueue one sync job per binding. Closes the inbound-latency
	// gap so phone-side edits flow back to wiki without a wiki trigger.
	keepBindingsLister := func() []keepsync.BindingKey {
		var out []keepsync.BindingKey
		// Query "...email" (a leaf string) instead of "...bindings"
		// (an array of maps). The frontmatter index doesn't save key
		// entries for non-empty arrays of maps — see index.indexArray
		// "Skip complex types" — so a profile with bindings would be
		// invisible to a query keyed on bindings. email is set on
		// every connected profile alongside bindings, so it's the
		// reliable signal here.
		for _, p := range site.FrontmatterIndexQueryer.QueryKeyExistence("wiki.connectors.google_keep.email") {
			state, err := keepBindingStore.LoadState(p)
			if err != nil || !state.IsConfigured() {
				continue
			}
			for _, b := range state.Subscriptions {
				out = append(out, keepsync.BindingKey{
					ProfileID: p,
					Page:      b.Page,
					ListName:  b.ListName,
				})
			}
		}
		return out
	}
	// Eager fingerprint migration: enqueue ONE scan job that walks
	// the data dir, finds every legacy Keep-bridge binding (those
	// whose MigratedFingerprints flag is unset), and enqueues a
	// per-binding migration job that pulls Keep once and rebases
	// synced_fp using the agreement-or-Keep-wins rule. Must be
	// enqueued BEFORE the cron registration below so the job
	// queue drains migration jobs before the first cron tick can
	// run SyncToKeep against an un-migrated binding (the gate at
	// the top of SyncToKeep would skip those anyway, but the
	// eager job is what flips the flag so they stop being skipped).
	// Source: plan §"Migration".
	keepBridgeMigrationScanner := eager.NewFileSystemDataDirScanner(site.PathToData)
	keepBridgeFingerprintMigration := eager.NewKeepBridgeFingerprintMigrationScanJob(
		keepBridgeMigrationScanner,
		site.GetJobQueueCoordinator(),
		keepConnector,
		keepBindingStore,
	)
	if eerr := site.GetJobQueueCoordinator().EnqueueJob(keepBridgeFingerprintMigration); eerr != nil {
		logger.Error("Failed to enqueue Keep-bridge fingerprint migration scan job: %v", eerr)
	} else {
		logger.Info("Keep-bridge fingerprint migration scan started.")
	}

	// Unified per-30s connector tick. The SyncScheduler walks every
	// registered connector's SubscriptionLister on each fire and
	// enqueues a per-subscription sync job through that connector's
	// per-kind queue. Per-connector pause/rate-limit "skip-this-tick"
	// logic stays inside each Connector's Sync impl — the scheduler
	// doesn't second-guess.
	syncScheduler, err := connectors.NewSyncScheduler(site.GetJobQueueCoordinator(), logger)
	if err != nil {
		return nil, nil, fmt.Errorf("create connector sync scheduler: %w", err)
	}
	keepSubscriptionLister := func() []connectors.SubscriptionKey {
		out := make([]connectors.SubscriptionKey, 0, 16)
		for _, k := range keepBindingsLister() {
			out = append(out, connectors.SubscriptionKey{
				ProfileID: string(k.ProfileID),
				Page:      k.Page,
				ListName:  k.ListName,
			})
		}
		return out
	}
	if regErr := syncScheduler.Register(
		keepConnector,
		keepSubscriptionLister,
		func(_ connectors.Connector, key connectors.SubscriptionKey) jobs.Job {
			// The Keep outbound-sync queue still serializes per-account
			// pushes against Keep's global targetVersion, so the sync
			// job stays Keep-typed at the queue level. The dispatch
			// shape across connectors lives in SyncScheduler; the work
			// inside the queue stays connector-private.
			return keepsync.NewKeepOutboundSyncJob(
				keepConnector,
				wikipage.PageIdentifier(key.ProfileID),
				key.Page,
				key.ListName,
			)
		},
	); regErr != nil {
		return nil, nil, fmt.Errorf("register Keep with sync scheduler: %w", regErr)
	}

	// Google Tasks connector — env-var-conditional. The OAuth client id +
	// secret + redirect URI live in env so the secret never lands in
	// the data dir. If any are unset the connector stays unwired and
	// every Tasks-kind ConnectorService RPC returns FailedPrecondition
	// with a "set up Google Tasks on profile" message.
	tasksConnector, tasksSubscriptionStore, tasksAuthURLBuilder, err := setupGoogleTasksConnector(site, syncScheduler, checklistMutator, leaseTable, logger)
	if err != nil {
		return nil, nil, err
	}

	// LeaseTable boot-rebuild: walk every profile that has a Keep or
	// Tasks connector configured, replay each persisted Subscription
	// onto the LeaseTable, then signal ready. Per ADR-0011 the
	// LeaseTable is a derived view of the Subscription records on
	// profile pages; the boot rebuild reconstitutes it after a
	// process restart so cross-connector existence checks see a
	// consistent picture before any RPC unblocks.
	if err := rebuildLeaseTable(leaseTable, site, keepBindingStore, tasksSubscriptionStore, logger); err != nil {
		return nil, nil, fmt.Errorf("rebuild lease table: %w", err)
	}
	leaseTable.SignalReady()

	if _, err := site.CronScheduler.Schedule("@every 30s", syncScheduler); err != nil {
		return nil, nil, fmt.Errorf("schedule connector sync tick: %w", err)
	}

	// Wire the CalDAV server into the Site so its caldavGateway
	// middleware can dispatch CalDAV-shaped traffic. baseURL is left
	// empty here — defaultBackend uses it only to render the URL
	// property in VTODOs, and we'd rather honor X-Forwarded-Host
	// per-request than bake a fixed value at startup. Refining the
	// codec to derive baseURL from the request is a Phase 4 follow-up.
	caldavBackend := caldav.NewBackend(checklistMutator, "", time.Now)
	caldavServer := &caldav.Server{Backend: caldavBackend}
	site.SetCalDAVServer(caldavServer)

	grpcAPIServer = grpcAPIServer.
		WithJobQueueCoordinator(site.GetJobQueueCoordinator()).
		WithMarkdownRenderer(site.MarkdownRenderer).
		WithTemplateExecutor(server.TemplateExecutor{}).
		WithFileStorer(site.FileStorer).
		WithScheduledTurnDispatcher(site.ScheduledTurnDispatcher).
		WithAgentScheduleStore(site.AgentScheduleStore).
		WithAgentChatContextStore(site.AgentChatContextStore).
		WithChecklistMutator(checklistMutator).
		WithKeepConnector(keepConnector)

	if tasksConnector != nil {
		grpcAPIServer = grpcAPIServer.WithGoogleTasksConnector(tasksConnector)
	}
	if tasksAuthURLBuilder != nil {
		grpcAPIServer = grpcAPIServer.WithTasksAuthURLBuilder(tasksAuthURLBuilder)
	}

	unaryInterceptors, streamInterceptors, err := buildGRPCInterceptors(
		identityResolver, grpcAPIServer.LoggingInterceptor(), counters, logger,
	)
	if err != nil {
		return nil, nil, err
	}

	grpcServer := grpc.NewServer(buildGRPCServerOptions(site.MaxUploadSize, unaryInterceptors, streamInterceptors)...)
	grpcAPIServer.RegisterWithServer(grpcServer)

	return grpcServer, grpcAPIServer, nil
}

// buildGRPCServerOptions constructs the slice of grpc.ServerOption for the gRPC server.
// MaxUploadSize == 0 means no limit; the MaxRecvMsgSize option is omitted in that case.
func buildGRPCServerOptions(maxUploadSizeMB uint, unaryInterceptors []grpc.UnaryServerInterceptor, streamInterceptors []grpc.StreamServerInterceptor) []grpc.ServerOption {
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
	}
	if maxUploadSizeMB > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(int(maxUploadSizeMB)*1024*1024))
	}
	return opts
}

// metricsPersistJob triggers async metrics persistence via the job queue.
type metricsPersistJob struct {
	recorder *observability.WikiMetricsRecorder
}

// GetName returns the job name for the cron scheduler.
func (*metricsPersistJob) GetName() string {
	return "wiki_metrics_persist_trigger"
}

// Execute enqueues the actual persistence job to the job queue.
func (j *metricsPersistJob) Execute() error {
	j.recorder.PersistAsync()
	return nil
}

// tasksProfileTokenStore adapts the Tasks SubscriptionStore to the
// gateway's RefreshTokenStore contract. The gateway calls
// LoadRefreshToken on first refresh and SaveRefreshToken after each
// rotation; both round-trip through the per-profile frontmatter.
type tasksProfileTokenStore struct {
	store     *taskssync.SubscriptionStore
	profileID wikipage.PageIdentifier
}

func (t *tasksProfileTokenStore) LoadRefreshToken(_ context.Context) (string, error) {
	state, err := t.store.LoadState(t.profileID)
	if err != nil {
		return "", fmt.Errorf("tasks bridge: load refresh token: %w", err)
	}
	if state.RefreshToken == "" {
		return "", errors.New("tasks bridge: profile has no refresh token (Disconnect or never connected)")
	}
	return state.RefreshToken, nil
}

func (t *tasksProfileTokenStore) SaveRefreshToken(_ context.Context, token string) error {
	if token == "" {
		return errors.New("tasks bridge: refresh token must not be empty")
	}
	return t.store.WithProfileLock(t.profileID, func() error {
		state, err := t.store.LoadStateLocked(t.profileID)
		if err != nil {
			return fmt.Errorf("tasks bridge: load profile state for token rotation: %w", err)
		}
		state.RefreshToken = token
		return t.store.SaveStateLocked(t.profileID, state)
	})
}

// tasksAuthURLBuilder mints fresh Google authorization URLs for the
// BeginAuth(GOOGLE_TASKS) gRPC. Issues a state token + PKCE pair via
// the OAuth state store, then assembles the URL with the pinned
// scope set (openid + email + tasks read/write — see
// tasksgateway.RequestedScopes for why we ask for openid/email).
type tasksAuthURLBuilder struct {
	stateStore   server.OAuthStateStore
	authURL      string
	clientID     string
	redirectURI  string
	requiredScope string
}

func (b *tasksAuthURLBuilder) BuildAuthURL(ctx context.Context, profileID, _ string) (authURL string, stateToken string, err error) {
	verifier, err := tasksgateway.GeneratePKCEVerifier()
	if err != nil {
		return "", "", fmt.Errorf("generate PKCE verifier: %w", err)
	}
	stateToken, err = b.stateStore.Issue(ctx, profileID, verifier)
	if err != nil {
		return "", "", fmt.Errorf("issue OAuth state: %w", err)
	}
	challenge := tasksgateway.PKCEChallengeS256(verifier)
	authURL, err = tasksgateway.BuildAuthURL(b.authURL, b.clientID, b.redirectURI, b.requiredScope, stateToken, challenge)
	if err != nil {
		return "", "", fmt.Errorf("build auth URL: %w", err)
	}
	return authURL, stateToken, nil
}

// setupGoogleTasksConnector wires the Google Tasks bridge: SubscriptionStore,
// gateway client factory, debouncer, scheduler registration, OAuth handler.
// Returns (nil, nil, nil, nil) if the operator hasn't set the required env
// vars — that's the documented opt-out shape.
//
// The leaseTable parameter is the cross-connector LeaseTable shared with
// Keep (and reserved for iCloud Reminders); its boot-rebuild fan-out
// scan + SignalReady is the caller's responsibility, NOT this function's.
//
//revive:disable-next-line:function-length,function-result-limit
func setupGoogleTasksConnector(
	site *server.Site,
	syncScheduler *connectors.SyncScheduler,
	checklistMutator *checklistmutator.Mutator,
	leaseTable *connectors.LeaseTable,
	logger *lumber.ConsoleLogger,
) (tasksConnector *taskssync.Connector, tasksStore *taskssync.SubscriptionStore, authURLBuilder grpcapi.TasksAuthURLBuilder, err error) {
	clientID := os.Getenv("SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_ID")
	clientSecret := os.Getenv("SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_SECRET")
	redirectURI := os.Getenv("SIMPLE_WIKI_GOOGLE_TASKS_REDIRECT_URI")
	if clientID == "" || clientSecret == "" || redirectURI == "" {
		// Operator opt-out: connector stays unwired, gRPC handlers
		// surface a clear "not configured by this wiki's operator"
		// message.
		logger.Info("Google Tasks connector not configured (env vars unset); Tasks features disabled.")
		return nil, nil, nil, nil
	}

	tasksStore, sErr := taskssync.NewSubscriptionStore(site)
	if sErr != nil {
		return nil, nil, nil, fmt.Errorf("build Tasks subscription store: %w", sErr)
	}

	httpClient := &http.Client{Timeout: googleTasksHTTPTimeoutSeconds * time.Second}

	// Per-profile factory: each call constructs a fresh RefreshClient
	// against the profile's frontmatter-backed token store, then builds
	// a TasksClient on top of it. The connector calls this once per
	// Sync invocation; the RefreshClient caches the access token
	// internally for the lifetime of that one call.
	clientFactory := func(profileID wikipage.PageIdentifier, _ string) (taskssync.TasksClient, tasksgateway.TokenSource, error) {
		tokenStore := &tasksProfileTokenStore{store: tasksStore, profileID: profileID}
		refreshClient, err := tasksgateway.NewRefreshClient(httpClient, tasksgateway.DefaultGoogleTokenURL, clientID, clientSecret, tokenStore)
		if err != nil {
			return nil, nil, fmt.Errorf("build refresh client: %w", err)
		}
		client, err := tasksgateway.NewTasksClient(httpClient, tasksgateway.DefaultTasksBaseURL, refreshClient)
		if err != nil {
			return nil, nil, fmt.Errorf("build tasks client: %w", err)
		}
		return client, refreshClient, nil
	}

	tasksConnector, cerr := taskssync.NewConnector(
		tasksStore,
		leaseTable,
		clientFactory,
		logger,
		taskssync.SystemClock{},
	)
	if cerr != nil {
		return nil, nil, nil, fmt.Errorf("build tasks connector: %w", cerr)
	}
	tasksConnector.SetChecklistReader(checklistMutator)
	tasksConnector.SetChecklistMutator(checklistMutator)

	// Per-key debouncer: rapid checklist edits coalesce into one
	// outbound push; the debouncer also doubles as the SyncSuppressor
	// for inbound apply (so wiki writes during inbound replay don't
	// loop back as new sync triggers).
	tasksDebouncer, err := taskssync.NewSyncDebouncer(
		site.GetJobQueueCoordinator(),
		tasksConnector,
		logger,
		googleTasksSyncDebounceWindow,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build tasks sync debouncer: %w", err)
	}
	tasksConnector.SetSyncSuppressor(tasksDebouncer)
	// Wire the Tasks debouncer to the mutator's fan-out. Without this
	// every wiki UI edit would only trigger Keep's outbound sync —
	// Tasks would silently never push, with the per-30s scheduler
	// tick masking the bug as inbound-only behavior. AddSubscriber
	// (not SetSubscriber) is mandatory because Keep was registered
	// earlier and its single-slot replace would clobber Keep's
	// notify.
	checklistMutator.AddSubscriber(tasksDebouncer)

	// Single-worker queue per Tasks per the same rationale as Keep:
	// outbound order matters (position deltas), and the per-user
	// write quota is more than enough for a serialized worker.
	if err := site.GetJobQueueCoordinator().RegisterQueue(
		taskssync.TasksOutboundSyncJobName, 1, googleTasksOutboundSyncQueueDepth,
	); err != nil {
		return nil, nil, nil, fmt.Errorf("register Tasks outbound sync queue: %w", err)
	}

	tasksSubscriptionLister := func() []connectors.SubscriptionKey {
		out := make([]connectors.SubscriptionKey, 0, 8)
		// Probe by refresh_token, not email: Google's
		// /oauth2/v3/token response doesn't include the user's
		// address, so PersistRefreshToken (the OAuth-callback
		// persister) writes refresh_token but leaves email empty.
		// Probing on email would render every Tasks-only profile
		// invisible to the scheduler. refresh_token is the
		// canonical "is connected?" leaf — it's set on every
		// connect and cleared by Disconnect. Same indexing caveat
		// as Keep: arrays of maps don't appear in the frontmatter
		// index, so we probe a scalar leaf rather than the
		// subscriptions[] array.
		for _, p := range site.FrontmatterIndexQueryer.QueryKeyExistence("wiki.connectors.google_tasks.refresh_token") {
			state, err := tasksStore.LoadState(p)
			if err != nil || !state.IsConfigured() {
				continue
			}
			for _, sub := range state.Subscriptions {
				out = append(out, connectors.SubscriptionKey{
					ProfileID: string(p),
					Page:      sub.Page,
					ListName:  sub.ListName,
				})
			}
		}
		return out
	}
	// Phase 4-2: build the SyncEngine + TasksAdapter alongside the
	// legacy Connector and register the engine (not the legacy
	// connector) as the dispatch shape on the SyncScheduler. Per the
	// extract-sync-engine plan (Phase 4-2 of /home/.../warm-glacier.md),
	// the legacy Connector type is no longer referenced by the dispatch
	// layer but its files still compile — gRPC handlers continue to
	// route Subscribe/Unsubscribe/GetState/Connect/Disconnect through
	// the legacy connector during the brief Phase-4 cohabitation; only
	// the per-30s scheduler tick + ForceFullResync flow goes through
	// the engine in this commit. Phase 5 (Keep collapse) and Phase 6
	// (rename) finish the cutover.
	tasksAdapter, taerr := buildTasksAdapter(site, tasksStore, clientFactory, logger)
	if taerr != nil {
		return nil, nil, nil, taerr
	}
	tasksBindingStore, tbsErr := engine.NewFrontmatterBindingStore(
		site,
		&frontmatterIndexProfileLister{index: site.FrontmatterIndexQueryer},
		logger,
	)
	if tbsErr != nil {
		return nil, nil, nil, fmt.Errorf("build tasks engine binding store: %w", tbsErr)
	}
	tasksEngine, teerr := engine.NewEngine(
		tasksAdapter,
		leaseTable,
		checklistMutator,
		checklistMutator,
		tasksDebouncer,
		logger,
		systemWallClock{},
		tasksBindingStore,
	)
	if teerr != nil {
		return nil, nil, nil, fmt.Errorf("build tasks sync engine: %w", teerr)
	}

	if regErr := syncScheduler.Register(
		tasksEngine,
		tasksSubscriptionLister,
		func(_ connectors.Connector, key connectors.SubscriptionKey) jobs.Job {
			// The outbound sync job still routes through the legacy
			// Connector for the brief Phase-4 cohabitation. The
			// scheduler-tick path goes through the engine; the
			// debouncer-driven sync goes through the legacy connector
			// (its sync_job.go calls connector.Sync). Phase 4-3
			// removes the legacy path entirely.
			return taskssync.NewTasksOutboundSyncJob(
				tasksConnector,
				wikipage.PageIdentifier(key.ProfileID),
				key.Page,
				key.ListName,
			)
		},
	); regErr != nil {
		return nil, nil, nil, fmt.Errorf("register Tasks with sync scheduler: %w", regErr)
	}

	// OAuth callback handler — Phase 6 owns the route; bootstrap calls
	// SetOAuthGoogleHandler to install the live wiring once Tasks is
	// fully configured. Until then, callbacks render the "not
	// configured" 503 page.
	oauthStateStore := server.NewInMemoryOAuthStateStore()
	server.SetOAuthGoogleHandler(&server.OAuthGoogleHandler{
		StateStore:     oauthStateStore,
		TokenPersister: tasksConnector,
		HTTPClient:     httpClient,
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		RedirectURI:    redirectURI,
		IssuerExpected: tasksgateway.GoogleIssuer,
		TokenEndpoint:  tasksgateway.DefaultGoogleTokenURL,
		RequiredScope:  tasksgateway.TasksScope,
		Logger:         log.Default(),
	})

	authURLBuilder = &tasksAuthURLBuilder{
		stateStore:    oauthStateStore,
		authURL:       tasksgateway.DefaultGoogleAuthURL,
		clientID:      clientID,
		redirectURI:   redirectURI,
		requiredScope: tasksgateway.RequestedScopes,
	}

	// Tombstone GC retention: when any Tasks subscription on a
	// checklist is paused, retain its tombstones beyond the default
	// 7-day TTL so the deletion replay on resume isn't undone by GC.
	checklistMutator.SetPausedChecker(&tasksFannedOutPausedChecker{
		store: tasksStore,
		index: site.FrontmatterIndexQueryer,
	})

	// clientIDTailLen is the number of trailing characters of clientID shown in logs
	// to confirm the correct credential is loaded without leaking the full ID.
	const clientIDTailLen = 4
	logger.Info("Google Tasks connector configured (client_id ends ...%s).",
		safeTail(clientID, clientIDTailLen))

	return tasksConnector, tasksStore, authURLBuilder, nil
}

// safeTail returns the last n chars of s, or all of s if shorter. Used
// for log lines that want to confirm "the right credential is loaded"
// without leaking the full client_id.
func safeTail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// rebuildLeaseTable walks all profile pages with a configured Keep or
// Tasks connector and replays each persisted Subscription onto the
// LeaseTable. Per ADR-0011 the LeaseTable is a derived view of the
// Subscription records on profile pages; this fan-out scan is the
// authoritative reconstitution path on process start.
//
// Per the plan's "Single-Subscription invariant" §"Derived view": the
// rebuild **fails loudly on parse errors** rather than silently
// dropping a profile. A profile whose state can't be decoded indicates
// data corruption that an operator must inspect before the wiki keeps
// running with a partially-consistent in-memory view.
//
// tasksStore may be nil when the operator hasn't configured the Tasks
// connector — that branch is skipped silently.
func rebuildLeaseTable(
	leaseTable *connectors.LeaseTable,
	site *server.Site,
	keepStore *keepsync.SubscriptionStore,
	tasksStore *taskssync.SubscriptionStore,
	logger *lumber.ConsoleLogger,
) error {
	keepCount, err := rebuildLeaseTableKeep(leaseTable, site, keepStore)
	if err != nil {
		return err
	}
	tasksCount := 0
	if tasksStore != nil {
		tasksCount, err = rebuildLeaseTableTasks(leaseTable, site, tasksStore)
		if err != nil {
			return err
		}
	}
	logger.Info("LeaseTable boot rebuild complete: %d Keep + %d Tasks subscriptions replayed.",
		keepCount, tasksCount)
	return nil
}

// rebuildLeaseTableKeep walks every profile with a configured Keep
// connector and Takes a lease for each persisted Subscription. Returns the
// count of leases taken.
func rebuildLeaseTableKeep(
	leaseTable *connectors.LeaseTable,
	site *server.Site,
	keepStore *keepsync.SubscriptionStore,
) (int, error) {
	count := 0
	for _, profileID := range site.FrontmatterIndexQueryer.QueryKeyExistence("wiki.connectors.google_keep.email") {
		state, err := keepStore.LoadState(profileID)
		if err != nil {
			return count, fmt.Errorf("decode Keep state for %s: %w", profileID, err)
		}
		for _, b := range state.Subscriptions {
			key := connectors.ChecklistKey{Page: b.Page, ListName: b.ListName}
			owner := connectors.LeaseOwner{
				Kind:      connectors.ConnectorKindGoogleKeep,
				ProfileID: string(profileID),
			}
			if err := leaseTable.Take(key, owner); err != nil {
				return count, fmt.Errorf("replay Keep lease %s/%s for %s: %w",
					b.Page, b.ListName, profileID, err)
			}
			count++
		}
	}
	return count, nil
}

// rebuildLeaseTableTasks walks every profile with a configured Tasks
// connector and Takes a lease for each persisted Subscription. Returns
// the count of leases taken.
func rebuildLeaseTableTasks(
	leaseTable *connectors.LeaseTable,
	site *server.Site,
	tasksStore *taskssync.SubscriptionStore,
) (int, error) {
	count := 0
	// Probe by refresh_token, not email — see tasksSubscriptionLister
	// for the rationale. Tasks profiles connected via the OAuth
	// callback have only refresh_token populated; emailing-probing
	// here would skip them on boot rebuild and break cross-connector
	// LookupOwner until process restart re-ran with email present.
	for _, profileID := range site.FrontmatterIndexQueryer.QueryKeyExistence("wiki.connectors.google_tasks.refresh_token") {
		state, err := tasksStore.LoadState(profileID)
		if err != nil {
			return count, fmt.Errorf("decode Tasks state for %s: %w", profileID, err)
		}
		for _, sub := range state.Subscriptions {
			key := connectors.ChecklistKey{Page: sub.Page, ListName: sub.ListName}
			owner := connectors.LeaseOwner{
				Kind:      connectors.ConnectorKindGoogleTasks,
				ProfileID: string(profileID),
			}
			if err := leaseTable.Take(key, owner); err != nil {
				return count, fmt.Errorf("replay Tasks lease %s/%s for %s: %w",
					sub.Page, sub.ListName, profileID, err)
			}
			count++
		}
	}
	return count, nil
}

// tasksFannedOutPausedChecker satisfies checklistmutator.PausedChecker
// by fanning out the per-profile Tasks Connector check across every
// profile that has the connector configured. The fan-out is keyed off
// the frontmatter index (same probe the SubscriptionLister uses), so
// it picks up profiles connected since process start.
//
// Returns true if any subscription on (page, listName) on any profile
// is currently paused. Per ADR-0011 a checklist has at most one owner
// at a time, so the OR-fan-out is conservative-correct: a non-owning
// profile's response is "no paused subscription here", which won't
// flip the answer to true unless the actual owner is paused.
type tasksFannedOutPausedChecker struct {
	store *taskssync.SubscriptionStore
	index frontmatterKeyQueryer
}

// frontmatterKeyQueryer is the subset of the wiki's frontmatter
// index the paused checker uses. Stated as an interface here so the
// constructor can take site.FrontmatterIndexQueryer without dragging
// in the whole index package.
type frontmatterKeyQueryer interface {
	QueryKeyExistence(key string) []wikipage.PageIdentifier
}

// IsAnyChecklistSubscriptionPaused fans out the per-profile pause
// check.
func (c *tasksFannedOutPausedChecker) IsAnyChecklistSubscriptionPaused(page, listName string) bool {
	// Probe by refresh_token, not email — see tasksSubscriptionLister
	// for the rationale. Probing email would silently miss profiles
	// connected via the OAuth callback (which doesn't supply email),
	// causing the tombstone GC to under-retain on auth-paused
	// subscriptions for those users.
	for _, profileID := range c.index.QueryKeyExistence("wiki.connectors.google_tasks.refresh_token") {
		state, err := c.store.LoadState(profileID)
		if err != nil || !state.IsConfigured() {
			continue
		}
		for _, sub := range state.Subscriptions {
			if sub.Page == page && sub.ListName == listName && sub.IsPaused() {
				return true
			}
		}
	}
	return false
}

// systemWallClock implements engine.Clock against time.Now. Production
// wiring; tests substitute their own clocks.
type systemWallClock struct{}

// Now returns the current wall-clock time.
func (systemWallClock) Now() time.Time { return time.Now() }

// frontmatterIndexProfileLister adapts the wiki's IQueryFrontmatterIndex
// to the engine's ProfileLister contract. Engine's ProfileLister wants a
// ListProfilesWithKey(DottedKeyPath) []PageIdentifier method; the wiki's
// IQueryFrontmatterIndex exposes QueryKeyExistence with the same shape.
type frontmatterIndexProfileLister struct {
	index wikipage.IQueryFrontmatterIndex
}

// ListProfilesWithKey delegates to QueryKeyExistence.
func (l *frontmatterIndexProfileLister) ListProfilesWithKey(dottedKeyPath wikipage.DottedKeyPath) []wikipage.PageIdentifier {
	return l.index.QueryKeyExistence(dottedKeyPath)
}

// buildTasksAdapter wires a TasksAdapter for the engine path. The
// adapter reads refresh tokens from the same per-profile frontmatter
// the legacy SubscriptionStore uses, so both code paths see the same
// credential bundle until Phase 4-3 deletes the legacy code.
func buildTasksAdapter(
	site *server.Site,
	tasksStore *taskssync.SubscriptionStore,
	clientFactory taskssync.TasksClientFactory,
	logger *lumber.ConsoleLogger,
) (*googletasks.TasksAdapter, error) {
	_ = site // reserved for future expansion (e.g., direct frontmatter reads)
	creds := &tasksStoreCredentialReader{store: tasksStore}
	// Engine-shaped client factory: forwards to the legacy
	// taskssync.TasksClientFactory (which the bootstrap already wires
	// against the gateway's RefreshClient). The legacy factory's
	// TokenSource return is dropped — the engine adapter doesn't need
	// it because it asks for a fresh client per call.
	engineFactory := googletasks.TasksClientFactory(func(_ context.Context, profileID wikipage.PageIdentifier, refreshToken string) (googletasks.TasksClient, error) {
		client, _, err := clientFactory(profileID, refreshToken)
		if err != nil {
			return nil, err
		}
		return client, nil
	})
	return googletasks.NewTasksAdapter(creds, engineFactory, logger)
}

// tasksStoreCredentialReader satisfies googletasks.CredentialReader by
// reading refresh tokens from the legacy taskssync.SubscriptionStore.
// The legacy store and the new engine path share the same credential
// bundle on the profile page; once the legacy code is gone, this
// reader is replaced by FrontmatterCredentialReader directly.
type tasksStoreCredentialReader struct {
	store *taskssync.SubscriptionStore
}

// LoadRefreshToken reads the refresh token for the given profile.
// Returns ErrCredentialMissing when the profile has no refresh token.
func (r *tasksStoreCredentialReader) LoadRefreshToken(_ context.Context, profileID wikipage.PageIdentifier) (string, error) {
	state, err := r.store.LoadState(profileID)
	if err != nil {
		return "", fmt.Errorf("load tasks state for %s: %w", profileID, err)
	}
	if !state.IsConfigured() {
		return "", googletasks.ErrCredentialMissing
	}
	return state.RefreshToken, nil
}

