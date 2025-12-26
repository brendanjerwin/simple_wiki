package bootstrap

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	grpcapi "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/tailscale"
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

	handler := createMultiplexedHandler(site, logger, commit, buildTime, nil)

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
	handler := createMultiplexedHandler(site, logger, commit, buildTime, identityResolver)

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
	handler := createMultiplexedHandler(site, logger, commit, buildTime, identityResolver)

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
		_ = tlsListener.Close()
		return nil, fmt.Errorf("failed to create tailnet redirector: %w", err)
	}
	httpListener, err := net.Listen(networkTCP, httpAddr)
	if err != nil {
		_ = tlsListener.Close()
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

func createMultiplexedHandler(
	site *server.Site,
	logger *lumber.ConsoleLogger,
	commit string,
	buildTime time.Time,
	identityResolver tailscale.IdentityResolver,
) http.Handler {
	ginRouter := site.GinRouter()

	// Add Tailscale identity middleware if resolver is available
	if identityResolver != nil {
		ginRouter.Use(tailscale.IdentityMiddleware(identityResolver, logger))
	}

	grpcAPIServer := grpcapi.NewServer(
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

	// Build interceptor chain
	var interceptors []grpc.UnaryServerInterceptor
	if identityResolver != nil {
		interceptors = append(interceptors, tailscale.IdentityInterceptor(identityResolver, logger))
	}
	interceptors = append(interceptors, grpcAPIServer.LoggingInterceptor())

	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptors...))
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
	})
}
