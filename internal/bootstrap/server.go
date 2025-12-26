package bootstrap

import (
	"context"
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

const (
	networkTCP       = "tcp"
	defaultHTTPSPort = 443
)

// ServerConfig holds the configuration for running the server.
type ServerConfig struct {
	MainServer     *http.Server
	MainListener   net.Listener
	RedirectServer *http.Server // Optional: HTTP->HTTPS redirect server (ModeFullTLS only)
}

// Options contains all configuration options for server setup.
type Options struct {
	Host                      string
	Port                      int
	TLSPort                   int
	Mode                      ServerMode
	ForceRedirectTailnetHTTPS bool
}

// Dependencies contains all external dependencies needed for server setup.
type Dependencies struct {
	Site      *server.Site
	Logger    *lumber.ConsoleLogger
	Commit    string
	BuildTime time.Time
}

// DetermineServerMode determines the appropriate server mode based on options and Tailscale status.
func DetermineServerMode(tailscaleServe bool, tsAvailable bool, tsDNSName string) ServerMode {
	if !tsAvailable || tsDNSName == "" {
		return ModePlainHTTP
	}
	if tailscaleServe {
		return ModeTailscaleServe
	}
	return ModeFullTLS
}

// SetupServer configures and creates the server based on the provided options.
func SetupServer(ctx context.Context, opts Options, deps Dependencies) (*ServerConfig, error) {
	httpAddr := fmt.Sprintf("%s:%d", opts.Host, opts.Port)

	// Detect Tailscale availability
	detector := tailscale.NewDetector()
	tsStatus, err := detector.Detect(ctx)
	if err != nil {
		deps.Logger.Warn("Error detecting Tailscale: %v", err)
		tsStatus = &tailscale.Status{Available: false}
	}

	switch opts.Mode {
	case ModePlainHTTP:
		return setupPlainHTTP(httpAddr, deps)

	case ModeTailscaleServe:
		return setupTailscaleServe(httpAddr, tsStatus.DNSName, opts.ForceRedirectTailnetHTTPS, deps)

	case ModeFullTLS:
		return setupFullTLS(httpAddr, opts.TLSPort, tsStatus.DNSName, deps)

	default:
		return nil, fmt.Errorf("unknown server mode: %v", opts.Mode)
	}
}

func setupPlainHTTP(httpAddr string, deps Dependencies) (*ServerConfig, error) {
	deps.Logger.Info("Tailscale not available. Running as plain HTTP on %s", httpAddr)
	deps.Logger.Info("For secure access with user identity, install Tailscale: https://tailscale.com/download")

	handler := createMultiplexedHandler(deps, nil)

	httpListener, err := net.Listen(networkTCP, httpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP listener: %w", err)
	}

	return &ServerConfig{
		MainServer: &http.Server{
			Handler: h2c.NewHandler(handler, &http2.Server{}),
		},
		MainListener: httpListener,
	}, nil
}

func setupTailscaleServe(httpAddr, tsDNSName string, forceRedirect bool, deps Dependencies) (*ServerConfig, error) {
	deps.Logger.Info("Tailscale detected: %s", tsDNSName)
	deps.Logger.Info("Tailscale Serve mode. Running HTTP on %s with identity support", httpAddr)

	identityResolver := tailscale.NewIdentityResolver()
	handler := createMultiplexedHandler(deps, identityResolver)

	httpListener, err := net.Listen(networkTCP, httpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP listener: %w", err)
	}

	var finalHandler http.Handler = h2c.NewHandler(handler, &http2.Server{})
	if forceRedirect {
		deps.Logger.Info("Tailnet clients will be redirected to HTTPS")
		finalHandler = tailscale.NewRedirectHandler(tsDNSName, defaultHTTPSPort, identityResolver, finalHandler, true, deps.Logger)
	}

	return &ServerConfig{
		MainServer: &http.Server{
			Handler: finalHandler,
		},
		MainListener: httpListener,
	}, nil
}

func setupFullTLS(httpAddr string, tlsPort int, tsDNSName string, deps Dependencies) (*ServerConfig, error) {
	deps.Logger.Info("Tailscale detected: %s", tsDNSName)

	identityResolver := tailscale.NewIdentityResolver()
	handler := createMultiplexedHandler(deps, identityResolver)

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

	deps.Logger.Info("HTTPS server listening on %s", httpsAddr)

	// Create HTTP redirect server
	redirectHandler := tailscale.NewRedirectHandler(tsDNSName, tlsPort, identityResolver, h2c.NewHandler(handler, &http2.Server{}), false, deps.Logger)
	httpListener, err := net.Listen(networkTCP, httpAddr)
	if err != nil {
		_ = tlsListener.Close()
		return nil, fmt.Errorf("failed to create HTTP listener: %w", err)
	}

	deps.Logger.Info("HTTP server listening on %s (redirects tailnet, serves others)", httpAddr)

	config := &ServerConfig{
		MainServer: &http.Server{
			Handler: handler,
		},
		MainListener: tlsListener,
		RedirectServer: &http.Server{
			Addr:    httpAddr,
			Handler: redirectHandler,
		},
	}

	// Start redirect server in background
	go func() {
		if err := config.RedirectServer.Serve(httpListener); err != nil && err != http.ErrServerClosed {
			deps.Logger.Error("HTTP redirect server error: %v", err)
		}
	}()

	return config, nil
}

func createMultiplexedHandler(deps Dependencies, identityResolver tailscale.IResolveIdentity) http.Handler {
	ginRouter := deps.Site.GinRouter()

	// Add Tailscale identity middleware if resolver is available
	if identityResolver != nil {
		ginRouter.Use(tailscale.IdentityMiddleware(identityResolver, deps.Logger))
	}

	grpcAPIServer := grpcapi.NewServer(
		deps.Commit,
		deps.BuildTime,
		deps.Site,
		deps.Site.BleveIndexQueryer,
		deps.Site.GetJobQueueCoordinator(),
		deps.Logger,
		deps.Site.MarkdownRenderer,
		server.TemplateExecutor{},
		deps.Site.FrontmatterIndexQueryer,
	)

	// Build interceptor chain
	var interceptors []grpc.UnaryServerInterceptor
	if identityResolver != nil {
		interceptors = append(interceptors, tailscale.IdentityInterceptor(identityResolver, deps.Logger))
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
			deps.Logger.Debug("gRPC-ish request: %s %s", r.Method, r.URL.Path)
			wrappedGrpc.ServeHTTP(w, r)
			return
		}
		deps.Logger.Debug("Gin request: %s %s", r.Method, r.URL.Path)
		ginRouter.ServeHTTP(w, r)
	})
}
