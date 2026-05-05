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
	"sync"
	"time"

	"connectrpc.com/grpcreflect"
	"connectrpc.com/vanguard"

	"github.com/brendanjerwin/simple_wiki/internal/caldav"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	googletasks "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks"
	tasksgateway "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
	grpcapi "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	wikimcp "github.com/brendanjerwin/simple_wiki/internal/mcp"
	"github.com/brendanjerwin/simple_wiki/internal/observability"
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

	// Google Keep engine path. Same shape as the Tasks engine path:
	// credential store + adapter + binding store + engine + debouncer
	// + scheduler registration. The legacy Keep connector orchestrator
	// has been deleted (Phase 5-B); only the engine path remains.
	keepWiring, err := setupGoogleKeep(site, syncScheduler, checklistMutator, leaseTable, logger)
	if err != nil {
		return nil, nil, err
	}

	// Google Tasks engine path — env-var-conditional. The OAuth client id +
	// secret + redirect URI live in env so the secret never lands in
	// the data dir. If any are unset the engine stays unwired and
	// every Tasks-kind ConnectorService RPC returns FailedPrecondition
	// with a "set up Google Tasks on profile" message.
	tasksWiring, err := setupGoogleTasks(site, syncScheduler, checklistMutator, leaseTable, logger)
	if err != nil {
		return nil, nil, err
	}

	// LeaseTable boot-rebuild: walk every profile that has a Keep or
	// Tasks connector configured, replay each persisted Binding onto
	// the LeaseTable, then signal ready. Per ADR-0011 the LeaseTable
	// is a derived view of the Binding records on profile pages; the
	// boot rebuild reconstitutes it after a process restart so cross-
	// connector existence checks see a consistent picture before any
	// RPC unblocks.
	if err := rebuildLeaseTable(leaseTable, site, keepWiring, tasksWiring, logger); err != nil {
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
		WithChecklistMutator(checklistMutator)

	if keepWiring != nil {
		grpcAPIServer = grpcAPIServer.
			WithGoogleKeep(
				keepWiring.engine,
				keepWiring.adapter,
				keepWiring.bindingStore,
				keepWiring.credentialStore,
			).
			WithKeepAuthVerifier(keepWiring.authVerifier)
	}

	if tasksWiring != nil {
		grpcAPIServer = grpcAPIServer.
			WithGoogleTasks(
				tasksWiring.engine,
				tasksWiring.adapter,
				tasksWiring.bindingStore,
				tasksWiring.credentialStore,
			).
			WithTasksAuthURLBuilder(tasksWiring.authURLBuilder)
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

// tasksProfileTokenStore adapts the engine's CredentialStore to the
// gateway's RefreshTokenStore contract. The gateway calls
// LoadRefreshToken on first refresh and SaveRefreshToken after each
// rotation; both round-trip through the per-profile frontmatter via
// the credential store's read/write helpers.
type tasksProfileTokenStore struct {
	store     *googletasks.FrontmatterCredentialStore
	profileID wikipage.PageIdentifier
}

func (t *tasksProfileTokenStore) LoadRefreshToken(ctx context.Context) (string, error) {
	tok, err := t.store.LoadRefreshToken(ctx, t.profileID)
	if err != nil {
		return "", fmt.Errorf("tasks bridge: load refresh token: %w", err)
	}
	return tok, nil
}

func (t *tasksProfileTokenStore) SaveRefreshToken(ctx context.Context, token string) error {
	if token == "" {
		return errors.New("tasks bridge: refresh token must not be empty")
	}
	// PersistRefreshToken stamps connected_at / last_verified_at and
	// fans out engine.Resume across paused bindings. Email is empty
	// here — the gateway's refresh path doesn't supply it; the
	// existing email on the bundle (if any) is preserved.
	return t.store.PersistRefreshToken(ctx, string(t.profileID), "", token)
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

// tasksWiring bundles the engine-path collaborators the Tasks gRPC
// handlers and lease-table boot rebuild need. setupGoogleTasks returns
// nil when the operator hasn't configured the OAuth env vars — that's
// the documented opt-out.
type tasksWiring struct {
	engine          *engine.Engine
	adapter         *googletasks.TasksAdapter
	bindingStore    engine.BindingStore
	credentialStore *googletasks.FrontmatterCredentialStore
	authURLBuilder  grpcapi.TasksAuthURLBuilder
}

// engineTasksClient is the engine-shaped TasksClient interface. Mirrors
// googletasks.TasksClient — declared here so the bootstrap doesn't
// depend on the adapter package's internal interface name.
type engineTasksClient = googletasks.TasksClient

// setupGoogleTasks wires the engine path for Google Tasks: credential
// store, adapter, engine, debouncer, scheduler registration, OAuth
// handler. Returns nil when the operator hasn't set the required env
// vars — every Tasks-kind ConnectorService RPC then returns
// FailedPrecondition with a "set up Google Tasks on profile" message.
//
// The leaseTable parameter is the cross-connector LeaseTable shared
// with Keep (and reserved for iCloud Reminders); its boot-rebuild
// fan-out scan + SignalReady is the caller's responsibility, NOT this
// function's.
//
//revive:disable-next-line:function-length
func setupGoogleTasks(
	site *server.Site,
	syncScheduler *connectors.SyncScheduler,
	checklistMutator *checklistmutator.Mutator,
	leaseTable *connectors.LeaseTable,
	logger *lumber.ConsoleLogger,
) (*tasksWiring, error) {
	clientID := os.Getenv("SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_ID")
	clientSecret := os.Getenv("SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_SECRET")
	redirectURI := os.Getenv("SIMPLE_WIKI_GOOGLE_TASKS_REDIRECT_URI")
	if clientID == "" || clientSecret == "" || redirectURI == "" {
		logger.Info("Google Tasks connector not configured (env vars unset); Tasks features disabled.")
		return nil, nil
	}

	// BindingStore (engine-owned). Reads/writes profile frontmatter
	// under wiki.connectors.google_tasks.bindings[] (with legacy
	// subscriptions[] dual-read until Phase 7 migrates).
	bindingStore, bsErr := engine.NewFrontmatterBindingStore(
		site,
		&frontmatterIndexProfileLister{index: site.FrontmatterIndexQueryer},
		logger,
	)
	if bsErr != nil {
		return nil, fmt.Errorf("build tasks binding store: %w", bsErr)
	}

	// CredentialStore: hooks for pause-on-disconnect / resume-on-
	// reconnect are wired AFTER the engine is constructed (we pass
	// nil hooks initially, then mutate the store via constructor —
	// done with a builder-style function below).
	httpClient := &http.Client{Timeout: googleTasksHTTPTimeoutSeconds * time.Second}

	// Forward-declared by closures: pause/resume hooks call into the
	// engine, which references the credential store via the per-profile
	// token store. Use late binding via *engine.Engine pointer that's
	// filled in below.
	var tasksEngine *engine.Engine
	pauseAll := func(ctx context.Context, profileID wikipage.PageIdentifier, reason string) error {
		if tasksEngine == nil {
			return nil
		}
		return pauseAllTasksBindings(ctx, tasksEngine, bindingStore, profileID, reason)
	}
	resumeAll := func(ctx context.Context, profileID wikipage.PageIdentifier) error {
		if tasksEngine == nil {
			return nil
		}
		return resumeAllTasksBindings(ctx, tasksEngine, bindingStore, profileID)
	}

	credentialStore, csErr := googletasks.NewFrontmatterCredentialStore(
		site,
		googletasks.SystemClock{},
		logger,
		pauseAll,
		resumeAll,
	)
	if csErr != nil {
		return nil, fmt.Errorf("build tasks credential store: %w", csErr)
	}

	// Per-profile gateway client factory: each call constructs a fresh
	// RefreshClient against the credential-store-backed token store,
	// then builds a TasksClient on top. The adapter calls this once
	// per primitive invocation; RefreshClient caches the access token
	// for the lifetime of that one call.
	tasksClientFactory := googletasks.TasksClientFactory(func(_ context.Context, profileID wikipage.PageIdentifier, _ string) (engineTasksClient, error) {
		tokenStore := &tasksProfileTokenStore{store: credentialStore, profileID: profileID}
		refreshClient, err := tasksgateway.NewRefreshClient(httpClient, tasksgateway.DefaultGoogleTokenURL, clientID, clientSecret, tokenStore)
		if err != nil {
			return nil, fmt.Errorf("build refresh client: %w", err)
		}
		client, err := tasksgateway.NewTasksClient(httpClient, tasksgateway.DefaultTasksBaseURL, refreshClient)
		if err != nil {
			return nil, fmt.Errorf("build tasks client: %w", err)
		}
		return client, nil
	})

	tasksAdapter, aerr := googletasks.NewTasksAdapter(credentialStore, tasksClientFactory, logger)
	if aerr != nil {
		return nil, fmt.Errorf("build tasks adapter: %w", aerr)
	}

	// Wiki-side bridge: wraps the wiki's checklistmutator subscriber
	// shape and the engine's SyncSuppressor / SyncDebouncer hookpoints
	// on the inbound apply path.
	bridge := newTasksMutatorBridge(logger)

	tasksEng, eerr := engine.NewEngine(
		tasksAdapter,
		leaseTable,
		checklistMutator,
		checklistMutator,
		bridge,
		logger,
		systemWallClock{},
		bindingStore,
	)
	if eerr != nil {
		return nil, fmt.Errorf("build tasks sync engine: %w", eerr)
	}
	tasksEngine = tasksEng // late-bound for the credential-store hooks

	// Engine-owned outbound debouncer. The wiki mutator notifies the
	// bridge on every successful checklist mutation; the bridge
	// forwards to the engine debouncer's OnChecklistMutated; on
	// debounceWindow expiry the engine fires Sync via the SyncFunc.
	syncFn := func(ctx context.Context, key engine.SyncDebouncerKey) error {
		return tasksEngine.Sync(ctx, connectors.SubscriptionKey{
			ProfileID: key.ProfileID,
			Page:      key.Page,
			ListName:  key.ListName,
		})
	}
	debouncer, derr := engine.NewSyncDebouncer(
		systemWallClock{},
		realTimerScheduler{},
		syncFn,
		logger,
	)
	if derr != nil {
		return nil, fmt.Errorf("build tasks engine debouncer: %w", derr)
	}
	bridge.attachDebouncer(debouncer)
	checklistMutator.AddSubscriber(bridge)

	tasksSubscriptionLister := func() []connectors.SubscriptionKey {
		out := make([]connectors.SubscriptionKey, 0, 8)
		// Probe by refresh_token (the canonical "is connected?" leaf).
		// Email may be absent on profiles connected via the OAuth
		// callback — refresh_token is set on every connect and cleared
		// by Disconnect.
		for _, p := range site.FrontmatterIndexQueryer.QueryKeyExistence("wiki.connectors.google_tasks.refresh_token") {
			bindings, err := bindingStore.LoadBindings(p, connectors.ConnectorKindGoogleTasks)
			if err != nil {
				continue
			}
			for _, b := range bindings {
				out = append(out, connectors.SubscriptionKey{
					ProfileID: string(p),
					Page:      b.Page,
					ListName:  b.ListName,
				})
			}
		}
		return out
	}

	if regErr := syncScheduler.Register(
		tasksEngine,
		tasksSubscriptionLister,
		func(c connectors.Connector, key connectors.SubscriptionKey) jobs.Job {
			return &engineSyncJob{connector: c, key: key, queueName: tasksOutboundSyncJobName}
		},
	); regErr != nil {
		return nil, fmt.Errorf("register Tasks with sync scheduler: %w", regErr)
	}

	// Single-worker queue: outbound order matters (position deltas);
	// per-user write quota leaves room for a serialized worker.
	if err := site.GetJobQueueCoordinator().RegisterQueue(
		tasksOutboundSyncJobName, 1, googleTasksOutboundSyncQueueDepth,
	); err != nil {
		return nil, fmt.Errorf("register Tasks outbound sync queue: %w", err)
	}

	// OAuth callback handler — bootstrap installs the live wiring; the
	// callback verifies state/iss/PKCE, then calls TokenPersister
	// (= the credential store's PersistRefreshToken).
	oauthStateStore := server.NewInMemoryOAuthStateStore()
	server.SetOAuthGoogleHandler(&server.OAuthGoogleHandler{
		StateStore:     oauthStateStore,
		TokenPersister: credentialStore,
		HTTPClient:     httpClient,
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		RedirectURI:    redirectURI,
		IssuerExpected: tasksgateway.GoogleIssuer,
		TokenEndpoint:  tasksgateway.DefaultGoogleTokenURL,
		RequiredScope:  tasksgateway.TasksScope,
		Logger:         log.Default(),
	})

	authURLBuilder := &tasksAuthURLBuilder{
		stateStore:    oauthStateStore,
		authURL:       tasksgateway.DefaultGoogleAuthURL,
		clientID:      clientID,
		redirectURI:   redirectURI,
		requiredScope: tasksgateway.RequestedScopes,
	}

	// Tombstone GC retention: when any Tasks binding on a checklist is
	// paused, retain its tombstones beyond the default 7-day TTL so the
	// deletion replay on resume isn't undone by GC.
	checklistMutator.SetPausedChecker(&tasksFannedOutPausedChecker{
		bindings: bindingStore,
		index:    site.FrontmatterIndexQueryer,
	})

	const clientIDTailLen = 4
	logger.Info("Google Tasks connector configured (client_id ends ...%s).",
		safeTail(clientID, clientIDTailLen))

	return &tasksWiring{
		engine:          tasksEngine,
		adapter:         tasksAdapter,
		bindingStore:    bindingStore,
		credentialStore: credentialStore,
		authURLBuilder:  authURLBuilder,
	}, nil
}

// tasksOutboundSyncJobName is the queue name for the Tasks outbound
// sync job. Mirrors the legacy taskssync.TasksOutboundSyncJobName so
// the queue identifier on disk is unchanged across the cutover.
const tasksOutboundSyncJobName = "GoogleTasksOutboundSync"

// pauseAllTasksBindings is the engine-side fan-out the credential
// store invokes from ClearCredentials. Walks every binding for the
// profile and transitions active ones to paused.
func pauseAllTasksBindings(ctx context.Context, eng *engine.Engine, store engine.BindingStore, profileID wikipage.PageIdentifier, reason string) error {
	_ = ctx // engine.TransitionToPaused has no context parameter (lock-bound work, no I/O).
	bindings, err := store.LoadBindings(profileID, connectors.ConnectorKindGoogleTasks)
	if err != nil {
		return fmt.Errorf("load bindings for %s: %w", profileID, err)
	}
	var firstErr error
	for _, b := range bindings {
		if b.IsPaused() {
			continue
		}
		if pauseErr := eng.TransitionToPaused(profileID, b.Page, b.ListName, reason); pauseErr != nil {
			if firstErr == nil {
				firstErr = pauseErr
			}
		}
	}
	return firstErr
}

// resumeAllTasksBindings is the engine-side fan-out the credential
// store invokes from PersistRefreshToken. Walks every binding for
// the profile and offers each to engine.Resume — engine.Resume is
// idempotent on active bindings, so the blanket walk is safe.
func resumeAllTasksBindings(ctx context.Context, eng *engine.Engine, store engine.BindingStore, profileID wikipage.PageIdentifier) error {
	bindings, err := store.LoadBindings(profileID, connectors.ConnectorKindGoogleTasks)
	if err != nil {
		return fmt.Errorf("load bindings for %s: %w", profileID, err)
	}
	var firstErr error
	for _, b := range bindings {
		if resumeErr := eng.Resume(ctx, profileID, b.Page, b.ListName); resumeErr != nil {
			if firstErr == nil {
				firstErr = resumeErr
			}
		}
	}
	return firstErr
}

// engineSyncJob is the production *connectors.Connector*-driven sync
// job for the engine path. Built per debouncer fire / scheduler tick;
// Execute() calls Connector.Sync (= Engine.Sync). One queue per kind
// serializes pushes per worker.
type engineSyncJob struct {
	connector connectors.Connector
	key       connectors.SubscriptionKey
	queueName string
}

func (j *engineSyncJob) GetName() string { return j.queueName }
func (j *engineSyncJob) Execute() error {
	return j.connector.Sync(context.Background(), j.key)
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
// Tasks connector and replays each persisted Binding onto the
// LeaseTable. Per ADR-0011 the LeaseTable is a derived view of the
// Binding records on profile pages; this fan-out scan is the
// authoritative reconstitution path on process start.
//
// Per the plan's "Single-Binding invariant" §"Derived view": the
// rebuild **fails loudly on parse errors** rather than silently
// dropping a profile. A profile whose state can't be decoded indicates
// data corruption that an operator must inspect before the wiki keeps
// running with a partially-consistent in-memory view.
//
// keepWiring / tasksWiring may be nil when the operator hasn't
// configured that connector — those branches are skipped silently.
func rebuildLeaseTable(
	leaseTable *connectors.LeaseTable,
	site *server.Site,
	keepWiring *keepWiring,
	tasksWiring *tasksWiring,
	logger *lumber.ConsoleLogger,
) error {
	keepCount := 0
	var err error
	if keepWiring != nil {
		keepCount, err = rebuildLeaseTableKeepFromBindings(leaseTable, site, keepWiring.bindingStore)
		if err != nil {
			return err
		}
	}
	tasksCount := 0
	if tasksWiring != nil {
		tasksCount, err = rebuildLeaseTableTasksFromBindings(leaseTable, site, tasksWiring.bindingStore)
		if err != nil {
			return err
		}
	}
	logger.Info("LeaseTable boot rebuild complete: %d Keep + %d Tasks bindings replayed.",
		keepCount, tasksCount)
	return nil
}

// rebuildLeaseTableKeepFromBindings walks every profile with a
// configured Keep connector and Takes a lease for each persisted
// Binding. Returns the count of leases taken.
func rebuildLeaseTableKeepFromBindings(
	leaseTable *connectors.LeaseTable,
	site *server.Site,
	bindings engine.BindingStore,
) (int, error) {
	count := 0
	// Probe by master_token (the canonical "is connected?" leaf for
	// Keep). Mirrors the Tasks rebuild's refresh_token probe.
	for _, profileID := range site.FrontmatterIndexQueryer.QueryKeyExistence("wiki.connectors.google_keep.master_token") {
		profileBindings, err := bindings.LoadBindings(profileID, connectors.ConnectorKindGoogleKeep)
		if err != nil {
			return count, fmt.Errorf("decode Keep bindings for %s: %w", profileID, err)
		}
		for _, b := range profileBindings {
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

// rebuildLeaseTableTasksFromBindings walks every profile with a
// configured Tasks connector and Takes a lease for each persisted
// Binding. Returns the count of leases taken.
func rebuildLeaseTableTasksFromBindings(
	leaseTable *connectors.LeaseTable,
	site *server.Site,
	bindings engine.BindingStore,
) (int, error) {
	count := 0
	// Probe by refresh_token, not email — Tasks profiles connected via
	// the OAuth callback have only refresh_token populated. Email
	// probing would skip them on boot rebuild and break cross-
	// connector LookupOwner until process restart re-ran with email
	// present.
	for _, profileID := range site.FrontmatterIndexQueryer.QueryKeyExistence("wiki.connectors.google_tasks.refresh_token") {
		profileBindings, err := bindings.LoadBindings(profileID, connectors.ConnectorKindGoogleTasks)
		if err != nil {
			return count, fmt.Errorf("decode Tasks bindings for %s: %w", profileID, err)
		}
		for _, b := range profileBindings {
			key := connectors.ChecklistKey{Page: b.Page, ListName: b.ListName}
			owner := connectors.LeaseOwner{
				Kind:      connectors.ConnectorKindGoogleTasks,
				ProfileID: string(profileID),
			}
			if err := leaseTable.Take(key, owner); err != nil {
				return count, fmt.Errorf("replay Tasks lease %s/%s for %s: %w",
					b.Page, b.ListName, profileID, err)
			}
			count++
		}
	}
	return count, nil
}

// tasksFannedOutPausedChecker satisfies checklistmutator.PausedChecker
// by fanning out the per-profile binding-store query across every
// profile that has the Tasks connector configured. The fan-out is
// keyed off the frontmatter index (same probe the binding lister
// uses), so it picks up profiles connected since process start.
//
// Returns true if any binding on (page, listName) on any profile is
// currently paused. Per ADR-0011 a checklist has at most one owner
// at a time, so the OR-fan-out is conservative-correct.
type tasksFannedOutPausedChecker struct {
	bindings engine.BindingStore
	index    frontmatterKeyQueryer
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
	for _, profileID := range c.index.QueryKeyExistence("wiki.connectors.google_tasks.refresh_token") {
		bindings, err := c.bindings.LoadBindings(profileID, connectors.ConnectorKindGoogleTasks)
		if err != nil {
			continue
		}
		for _, b := range bindings {
			if b.Page == page && b.ListName == listName && b.IsPaused() {
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

// realTimerScheduler is the production engine.TimerScheduler. Wraps
// time.AfterFunc so the engine's SyncDebouncer can fire timers under
// real wall-clock time.
type realTimerScheduler struct{}

// AfterFunc schedules fn to run after d. The returned Timer's Stop()
// cancels the pending fire. Mirrors time.AfterFunc semantics; the
// returned *time.Timer satisfies engine.Timer because *time.Timer
// has a Stop() bool method with the same contract.
func (realTimerScheduler) AfterFunc(d time.Duration, fn func()) engine.Timer {
	return time.AfterFunc(d, fn)
}

// tasksMutatorBridge is the wiki-side glue between the checklistmutator
// notify shape and the engine's SyncDebouncer / SyncSuppressor.
// Phase 4-3 introduces this bridge to replace the legacy package's
// SyncDebouncer (which mixed wiki-side notify dispatch with the
// debounce algorithm itself).
//
// Implements:
//
//   - checklistmutator.Subscriber — receives mutation notifies; resolves
//     the calling identity to a profileID; forwards to the engine
//     debouncer's OnChecklistMutated.
//   - engine.SyncSuppressor — the engine's reconcile path calls
//     Suppress/Unsuppress around inbound apply so the inbound writes
//     don't loop back as outbound triggers.
//
// The bridge filters synthetic identities (system:tasks-sync,
// system:connector-sync, legacy system:keep-sync) so an inbound apply
// doesn't re-enqueue a sync against the same connector.
type tasksMutatorBridge struct {
	logger    *lumber.ConsoleLogger
	debouncer *engine.SyncDebouncer

	mu         sync.Mutex
	suppressed map[string]int // refcount per "<profile>|<page>|<list>"
}

// newTasksMutatorBridge returns a fresh bridge. The engine debouncer is
// attached separately via attachDebouncer so the two collaborators can
// be constructed in either order.
func newTasksMutatorBridge(logger *lumber.ConsoleLogger) *tasksMutatorBridge {
	return &tasksMutatorBridge{
		logger:     logger,
		suppressed: map[string]int{},
	}
}

// attachDebouncer connects the bridge to the engine debouncer. Called
// once at bootstrap; the engine debouncer construction depends on the
// engine's Sync function, which requires the engine, which (in turn)
// requires this bridge — late binding via attachDebouncer breaks the
// circular dependency.
func (b *tasksMutatorBridge) attachDebouncer(d *engine.SyncDebouncer) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.debouncer = d
}

// Suppress implements engine.SyncSuppressor. Refcounts under a per-key
// mutex so nested apply windows compose cleanly.
func (b *tasksMutatorBridge) Suppress(profileID wikipage.PageIdentifier, page, listName string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.suppressed[bridgeKey(profileID, page, listName)]++
}

// Unsuppress implements engine.SyncSuppressor.
func (b *tasksMutatorBridge) Unsuppress(profileID wikipage.PageIdentifier, page, listName string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := bridgeKey(profileID, page, listName)
	b.suppressed[key]--
	if b.suppressed[key] <= 0 {
		delete(b.suppressed, key)
	}
}

// Synthetic identity strings that the bridge drops. These are the
// system:* identities the checklistmutator stamps on inbound apply
// writes; without filtering, every inbound apply would re-trigger the
// same connector's outbound debounce loop.
const (
	tasksSyncIdentityLogin       = "system:tasks-sync"
	connectorSyncIdentityLogin   = "system:connector-sync"
	legacyKeepSyncIdentityLogin  = "system:keep-sync"
)

// OnChecklistMutated implements checklistmutator.Subscriber. The wiki
// mutator notifies after every successful checklist mutation; the
// bridge resolves the calling identity to a profileID and forwards
// to the engine debouncer.
func (b *tasksMutatorBridge) OnChecklistMutated(page, listName string, identity tailscale.IdentityValue) {
	if identity == nil {
		return
	}
	login := identity.LoginName()
	if login == "" {
		return
	}
	if login == tasksSyncIdentityLogin ||
		login == connectorSyncIdentityLogin ||
		login == legacyKeepSyncIdentityLogin {
		return
	}
	profileID, err := wikipage.ProfileIdentifierFor(login)
	if err != nil {
		if b.logger != nil {
			b.logger.Error("tasks bridge: resolve profile for login %q: %v", login, err)
		}
		return
	}

	b.mu.Lock()
	if b.suppressed[bridgeKey(profileID, page, listName)] > 0 {
		b.mu.Unlock()
		return
	}
	debouncer := b.debouncer
	b.mu.Unlock()

	if debouncer == nil {
		return
	}
	debouncer.OnChecklistMutated(engine.SyncDebouncerKey{
		ProfileID: string(profileID),
		Page:      page,
		ListName:  listName,
	})
}

func bridgeKey(profileID wikipage.PageIdentifier, page, listName string) string {
	return fmt.Sprintf("%s|%s|%s", profileID, page, listName)
}

