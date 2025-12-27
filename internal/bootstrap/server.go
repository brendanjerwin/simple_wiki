package bootstrap

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	grpcapi "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/observability"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/gin-gonic/gin"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/jcelliott/lumber"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const networkTCP = "tcp"

// ServerResult holds the created server and listener.
type ServerResult struct {
	MainServer     *http.Server
	MainListener   net.Listener
	RedirectServer *http.Server // Optional: HTTP->HTTPS redirect server (ModeFullTLS only)
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

	handler, err := createMultiplexedHandler(site, logger, commit, buildTime, nil)
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
	handler, err := createMultiplexedHandler(site, logger, commit, buildTime, identityResolver)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler: %w", err)
	}

	httpListener, err := net.Listen(networkTCP, httpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP listener: %w", err)
	}

	var finalHandler http.Handler = h2c.NewHandler(handler, &http2.Server{})
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
	handler, err := createMultiplexedHandler(site, logger, commit, buildTime, identityResolver)
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
		MainListener: tlsListener,
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

// setupHTTPObservability adds HTTP observability middleware to the Gin router.
func setupHTTPObservability(ginRouter gin.IRouter, counters observability.RequestCounter, logger *lumber.ConsoleLogger) {
	httpMetrics, err := observability.NewHTTPMetrics()
	if err != nil {
		logger.Warn("Failed to create HTTP metrics, continuing without: %v", err)
	}
	httpInstrumentation := observability.NewHTTPInstrumentation(httpMetrics, counters)
	ginRouter.Use(httpInstrumentation.GinMiddleware())
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
		interceptor, err := tailscale.IdentityInterceptor(identityResolver, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create identity interceptor: %w", err)
		}
		unaryInterceptors = append(unaryInterceptors, interceptor)
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
) (http.Handler, error) {
	ginRouter := site.GinRouter()

	// Create wiki metrics recorder for lightweight aggregate tracking
	wikiRecorder, err := observability.NewWikiMetricsRecorder(
		site, site, site.GetJobQueueCoordinator(), logger,
	)
	if err != nil {
		logger.Warn("Failed to create wiki metrics recorder: %v", err)
	}

	// Create composite counter (currently just wiki recorder, extensible for future backends)
	var counters observability.RequestCounter
	if wikiRecorder != nil {
		counters = observability.NewCompositeRequestCounter(wikiRecorder)

		// Schedule periodic persistence (every minute)
		_, schedErr := site.CronScheduler.Schedule("0 * * * * *", &metricsPersistJob{recorder: wikiRecorder})
		if schedErr != nil {
			logger.Warn("Failed to schedule metrics persistence: %v", schedErr)
		} else {
			logger.Info("Scheduled wiki metrics persistence every minute")
		}
	}

	setupHTTPObservability(ginRouter, counters, logger)

	if identityResolver != nil {
		middleware, err := tailscale.IdentityMiddleware(identityResolver, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create identity middleware: %w", err)
		}
		ginRouter.Use(middleware)
	}

	grpcAPIServer, err := grpcapi.NewServer(
		commit,
		buildTime,
		site,
		site.BleveIndexQueryer,
		site.GetJobQueueCoordinator(),
		logger,
		site.MarkdownRenderer,
		server.TemplateExecutor{},
		site.FrontmatterIndexQueryer,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	unaryInterceptors, streamInterceptors, err := buildGRPCInterceptors(
		identityResolver,
		grpcAPIServer.LoggingInterceptor(),
		counters,
		logger,
	)
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
	)
	grpcAPIServer.RegisterWithServer(grpcServer)

	reflection.Register(grpcServer)

	wrappedGrpc := grpcweb.WrapServer(grpcServer,
		grpcweb.WithOriginFunc(func(_ string) bool { return true }),
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			logger.Debug("gRPC-ish request: %s %s", r.Method, r.URL.Path)
			wrappedGrpc.ServeHTTP(w, r)
			return
		}
		logger.Debug("Gin request: %s %s", r.Method, r.URL.Path)
		ginRouter.ServeHTTP(w, r)
	}), nil
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
