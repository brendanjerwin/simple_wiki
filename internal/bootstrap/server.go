package bootstrap

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/grpcreflect"
	"connectrpc.com/vanguard"
	grpcapi "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/observability"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/tailscale"
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
		return nil, fmt.Errorf("failed to create handler: %w", err)
	}

	httpListener, err := net.Listen(networkTCP, httpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP listener: %w", err)
	}

	return &ServerResult{
		MainServer: &http.Server{
			Handler: h2c.NewHandler(handler, &http2.Server{}),
		},
		MainListener: httpListener,
		Cleanup:      metricsCleanup,
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
) (*ServerResult, error) {
	logger.Info("Tailscale detected: %s", tsDNSName)
	logger.Info("Tailscale Serve mode. Running HTTP on %s with identity support", httpAddr)

	identityResolver := tailscale.NewIdentityResolver()
	handler, metricsCleanup, err := createMultiplexedHandler(site, logger, commit, buildTime, identityResolver)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler: %w", err)
	}

	httpListener, err := net.Listen(networkTCP, httpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP listener: %w", err)
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
		Cleanup:      metricsCleanup,
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
) (*ServerResult, error) {
	logger.Info("Tailscale detected: %s", tsDNSName)

	identityResolver := tailscale.NewIdentityResolver()
	handler, metricsCleanup, err := createMultiplexedHandler(site, logger, commit, buildTime, identityResolver)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler: %w", err)
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
		return nil, fmt.Errorf("failed to create HTTP listener: %w", err)
	}

	logger.Info("HTTP server listening on %s (redirects tailnet, serves others)", httpAddr)

	result := &ServerResult{
		MainServer: &http.Server{
			Handler: handler,
		},
		MainListener:  tlsListener,
		Cleanup:       metricsCleanup,
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

	grpcServer, err := setupGRPCServer(site, commit, buildTime, identityResolver, counters, logger)
	if err != nil {
		return nil, nil, err
	}

	transcoder, err := buildVanguardTranscoder(grpcServer, ginRouter, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create vanguard transcoder: %w", err)
	}

	return transcoder, metricsCleanup, nil
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
		return observability.NewCompositeRequestCounter(), func() {}
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

// buildVanguardTranscoder creates a vanguard Transcoder that accepts Connect, gRPC, and
// gRPC-Web requests and transcodes them to gRPC for the underlying grpcServer. ConnectRPC
// reflection is mounted alongside, enabling service discovery over HTTP/1.1 — compatible
// with proxies like Tailscale Serve that don't forward HTTP/2 trailers. Non-RPC requests
// fall through to the Gin router.
func buildVanguardTranscoder(grpcServer *grpc.Server, ginRouter http.Handler, logger *lumber.ConsoleLogger) (*vanguard.Transcoder, error) {
	// grpcServer only speaks gRPC; tell vanguard to use gRPC+proto on the backend leg.
	grpcOpts := []vanguard.ServiceOption{
		vanguard.WithTargetProtocols(vanguard.ProtocolGRPC),
		vanguard.WithTargetCodecs(vanguard.CodecProto),
	}

	services := []*vanguard.Service{
		vanguard.NewService("api.v1.Frontmatter", grpcServer, grpcOpts...),
		vanguard.NewService("api.v1.InventoryManagementService", grpcServer, grpcOpts...),
		vanguard.NewService("api.v1.PageImportService", grpcServer, grpcOpts...),
		vanguard.NewService("api.v1.PageManagementService", grpcServer, grpcOpts...),
		vanguard.NewService("api.v1.SearchService", grpcServer, grpcOpts...),
		vanguard.NewService("api.v1.SystemInfoService", grpcServer, grpcOpts...),
	}

	// ConnectRPC reflection handler works over HTTP/1.1 — no HTTP/2 trailers needed.
	reflector := grpcreflect.NewStaticReflector(
		"api.v1.Frontmatter",
		"api.v1.InventoryManagementService",
		"api.v1.PageImportService",
		"api.v1.PageManagementService",
		"api.v1.SearchService",
		"api.v1.SystemInfoService",
	)
	reflectMux := http.NewServeMux()
	reflectMux.Handle(grpcreflect.NewHandlerV1(reflector))
	reflectMux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	// Unknown paths: reflection requests go to grpcreflect; everything else to Gin.
	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/grpc.reflection.") {
			logger.Debug("gRPC reflection request: %s %s", r.Method, r.URL.Path)
			reflectMux.ServeHTTP(w, r)
			return
		}
		logger.Debug("Gin request: %s %s", r.Method, r.URL.Path)
		ginRouter.ServeHTTP(w, r)
	})

	return vanguard.NewTranscoder(services, vanguard.WithUnknownHandler(fallback))
}

// setupGRPCServer creates and configures the gRPC server with interceptors.
func setupGRPCServer(
	site *server.Site,
	commit string,
	buildTime time.Time,
	identityResolver tailscale.IdentityResolver,
	counters observability.RequestCounter,
	logger *lumber.ConsoleLogger,
) (*grpc.Server, error) {
	grpcAPIServer, err := grpcapi.NewServer(
		commit, buildTime, site, site.BleveIndexQueryer, site.GetJobQueueCoordinator(),
		logger, site.MarkdownRenderer, server.TemplateExecutor{}, site.FrontmatterIndexQueryer,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	unaryInterceptors, streamInterceptors, err := buildGRPCInterceptors(
		identityResolver, grpcAPIServer.LoggingInterceptor(), counters, logger,
	)
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
	)
	grpcAPIServer.RegisterWithServer(grpcServer)

	return grpcServer, nil
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

