package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
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

	cli "gopkg.in/urfave/cli.v1"
)

var (
	commit    = "n/a"
	buildTime = ""
	logger    *lumber.ConsoleLogger
)

// getCommitHash retrieves the current git commit hash.
// If git is not available or not in a git repository, returns the default commit value.
func getCommitHash() string {
	if commit != "n/a" {
		// If commit was set at build time, use that
		return commit
	}
	
	// Try to get commit from git
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "dev"
	}
	
	return strings.TrimSpace(string(output))
}

// getBuildTime returns the build time.
// If running in development (via go run), returns the current time.
// Otherwise, returns the compiled time set via ldflags.
func getBuildTime() time.Time {
	if buildTime != "" {
		// If buildTime was set at build time, parse and use that
		if t, err := time.Parse(time.RFC3339, buildTime); err == nil {
			return t
		}
	}
	
	// Fallback to current time (development mode)
	return time.Now()
}

var app *cli.App

// serverConfig holds the configuration for running the server.
type serverConfig struct {
	mainServer     *http.Server
	mainListener   net.Listener
	redirectServer *http.Server // Optional: HTTP->HTTPS redirect server
}

func setupServer(c *cli.Context) (*serverConfig, error) {
	pathToData := c.GlobalString("data")
	if err := os.MkdirAll(pathToData, 0755); err != nil {
		return nil, err
	}

	if c.GlobalBool("debug") {
		logger = makeDebugLogger()
	} else {
		logger = makeProductionLogger()
	}

	logger.Info("Starting simple_wiki server...")

	site, err := server.NewSite(
		pathToData,
		c.GlobalString("css"),
		c.GlobalString("default-page"),
		c.GlobalInt("debounce"),
		c.GlobalString("cookie-secret"),
		!c.GlobalBool("block-file-uploads"),
		c.GlobalUint("max-upload-mb"),
		c.GlobalUint("max-document-length"),
		logger,
	)
	if err != nil {
		return nil, err
	}

	host := c.GlobalString("host")
	if host == "" {
		host = "0.0.0.0"
	}
	httpAddr := fmt.Sprintf("%s:%s", host, c.GlobalString("port"))

	// Detect Tailscale availability
	ctx := context.Background()
	detector := tailscale.NewDetector()
	tsStatus, err := detector.Detect(ctx)
	if err != nil {
		logger.Warn("Error detecting Tailscale: %v", err)
		tsStatus = &tailscale.Status{Available: false}
	}

	var identityResolver tailscale.IResolveIdentity
	var config *serverConfig
	tailscaleServe := c.GlobalBool("tailscale-serve")

	if tsStatus.Available && tsStatus.DNSName != "" {
		// Tailscale is available
		logger.Info("Tailscale detected: %s", tsStatus.DNSName)
		identityResolver = tailscale.NewIdentityResolver()
		handler := createMultiplexedHandler(site, identityResolver, logger)

		if tailscaleServe {
			// --tailscale-serve: Let Tailscale Serve handle HTTPS
			logger.Info("Tailscale Serve mode. Running HTTP on %s with identity support", httpAddr)

			httpListener, err := net.Listen("tcp", httpAddr)
			if err != nil {
				return nil, fmt.Errorf("failed to create HTTP listener: %w", err)
			}

			// Wrap handler with redirect to tailnet HTTPS (port 443 via Tailscale Serve)
			redirectHandler := tailscale.NewRedirectHandler(tsStatus.DNSName, 443, identityResolver, h2c.NewHandler(handler, &http2.Server{}))

			config = &serverConfig{
				mainServer: &http.Server{
					Handler: redirectHandler,
				},
				mainListener: httpListener,
			}
		} else {
			// Full TLS mode: HTTPS + HTTP redirect
			portStr := c.GlobalString("port")
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return nil, fmt.Errorf("invalid port %q: %w", portStr, err)
			}
			tlsPort := c.GlobalInt("tls-port")
			if tlsPort == 0 {
				tlsPort = port + 1 // Default to adjacent port (e.g., 80 -> 81, 8050 -> 8051)
			}
			tlsProvider := tailscale.NewTLSProvider()
			tlsConfig := tlsProvider.GetTLSConfig()
			httpsAddr := fmt.Sprintf("%s:%d", host, tlsPort)
			tlsListener, err := tls.Listen("tcp", httpsAddr, tlsConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to create TLS listener: %w", err)
			}

			logger.Info("HTTPS server listening on %s", httpsAddr)

			// Create HTTP redirect server on the configured port
			redirectHandler := tailscale.NewRedirectHandler(tsStatus.DNSName, tlsPort, identityResolver, h2c.NewHandler(handler, &http2.Server{}))
			httpListener, err := net.Listen("tcp", httpAddr)
			if err != nil {
				tlsListener.Close()
				return nil, fmt.Errorf("failed to create HTTP listener: %w", err)
			}

			logger.Info("HTTP server listening on %s (redirects tailnet, serves others)", httpAddr)

			config = &serverConfig{
				mainServer: &http.Server{
					Handler: handler,
				},
				mainListener: tlsListener,
				redirectServer: &http.Server{
					Addr:    httpAddr,
					Handler: redirectHandler,
				},
			}
			// Start redirect server in background
			go func() {
				if err := config.redirectServer.Serve(httpListener); err != nil && err != http.ErrServerClosed {
					logger.Error("HTTP redirect server error: %v", err)
				}
			}()
		}
	} else {
		// Tailscale not available - plain HTTP fallback
		logger.Info("Tailscale not available. Running as plain HTTP on %s", httpAddr)
		logger.Info("For secure access with user identity, install Tailscale: https://tailscale.com/download")

		handler := createMultiplexedHandler(site, nil, logger)

		httpListener, err := net.Listen("tcp", httpAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP listener: %w", err)
		}

		config = &serverConfig{
			mainServer: &http.Server{
				Handler: h2c.NewHandler(handler, &http2.Server{}),
			},
			mainListener: httpListener,
		}
	}

	actualCommit := getCommitHash()
	logger.Info("Running simple_wiki server (commit %s)", actualCommit)

	return config, nil
}

func createMultiplexedHandler(site *server.Site, identityResolver tailscale.IResolveIdentity, log *lumber.ConsoleLogger) http.Handler {
	ginRouter := site.GinRouter()

	// Add Tailscale identity middleware if resolver is available
	if identityResolver != nil {
		ginRouter.Use(tailscale.IdentityMiddleware(identityResolver, log))
	}

	actualCommit := getCommitHash()
	buildTime := getBuildTime()
	grpcAPIServer := grpcapi.NewServer(
		actualCommit,
		buildTime,
		site,
		site.BleveIndexQueryer,
		site.GetJobQueueCoordinator(),
		log,
		site.MarkdownRenderer,
		server.TemplateExecutor{},
		site.FrontmatterIndexQueryer,
	)

	// Build interceptor chain
	var interceptors []grpc.UnaryServerInterceptor
	if identityResolver != nil {
		interceptors = append(interceptors, tailscale.IdentityInterceptor(identityResolver, log))
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
			log.Debug("gRPC-ish request: %s %s", r.Method, r.URL.Path)
			wrappedGrpc.ServeHTTP(w, r)
			return
		}
		log.Debug("Gin request: %s %s", r.Method, r.URL.Path)
		ginRouter.ServeHTTP(w, r)
	})
}

func main() {
	app = cli.NewApp()
	app.Name = "simple_wiki"
	app.Usage = "a simple wiki"
	app.Version = getCommitHash()
	app.Compiled = time.Now()
	app.Action = func(c *cli.Context) error {
		config, err := setupServer(c)
		if err != nil {
			return err
		}
		return config.mainServer.Serve(config.mainListener)
	}
	app.Flags = getFlags()

	if err := app.Run(os.Args); err != nil {
		if logger != nil {
			logger.Error("Error running app: %v", err)
		} else {
			fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		}
		os.Exit(1)
	}
}

const (
	defaultDebounce          = 500
	defaultMaxUploadMB       = 100
	defaultMaxDocumentLength = 100000000
)

func getFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:  "data",
			Value: "data",
			Usage: "data folder to use",
		},
		cli.StringFlag{
			Name:  "host",
			Value: "",
			Usage: "host to use",
		},
		cli.StringFlag{
			Name:  "port,p",
			Value: "8050",
			Usage: "port to use",
		},
		cli.IntFlag{
			Name:  "tls-port",
			Value: 0,
			Usage: "TLS port for HTTPS when Tailscale is available (0 = auto: port+1)",
		},
		cli.BoolFlag{
			Name:  "tailscale-serve",
			Usage: "Let Tailscale Serve handle HTTPS (no local TLS listener)",
		},
		cli.StringFlag{
			Name:  "css",
			Value: "",
			Usage: "use a custom CSS file",
		},
		cli.StringFlag{
			Name:  "default-page",
			Value: "home",
			Usage: "show default-page/read instead of editing (default: show random editing)",
		},
		cli.IntFlag{
			Name:  "debounce",
			Value: defaultDebounce,
			Usage: "debounce time for saving data, in milliseconds",
		},
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "turn on debugging",
		},
		cli.StringFlag{
			Name:  "cookie-secret",
			Value: "secret",
			Usage: "random data to use for cookies; changing it will invalidate all sessions",
		},
		cli.BoolFlag{
			Name:  "block-file-uploads",
			Usage: "Block file uploads",
		},
		cli.UintFlag{
			Name:  "max-upload-mb",
			Value: defaultMaxUploadMB,
			Usage: "Largest file upload (in mb) allowed",
		},
		cli.UintFlag{
			Name:  "max-document-length",
			Value: defaultMaxDocumentLength,
			Usage: "Largest wiki page (in characters) allowed",
		},
	}
}

func makeDebugLogger() *lumber.ConsoleLogger {
	return lumber.NewConsoleLogger(lumber.TRACE)
}

func makeProductionLogger() *lumber.ConsoleLogger {
	return lumber.NewConsoleLogger(lumber.INFO)
}
