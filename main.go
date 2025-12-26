package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	grpcapi "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/observability"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/jcelliott/lumber"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	cli "gopkg.in/urfave/cli.v1"
)

var (
	commit           = "n/a"
	buildTime        = ""
	logger           *lumber.ConsoleLogger
	telemetry        *observability.TelemetryProvider
	grpcInstrumentation *observability.GRPCInstrumentation
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

func setupServer(c *cli.Context) (*http.Server, func(), error) {
	pathToData := c.GlobalString("data")
	if err := os.MkdirAll(pathToData, 0755); err != nil {
		return nil, nil, err
	}

	if c.GlobalBool("debug") {
		logger = makeDebugLogger()
	} else {
		logger = makeProductionLogger()
	}

	logger.Info("Starting simple_wiki server...")

	// Initialize OpenTelemetry
	actualCommit := getCommitHash()
	var err error
	telemetry, err = observability.Initialize(context.Background(), actualCommit)
	if err != nil {
		logger.Warn("Failed to initialize OpenTelemetry: %v", err)
	} else if telemetry.IsEnabled() {
		logger.Info("OpenTelemetry instrumentation enabled")
		
		// Initialize gRPC metrics and instrumentation
		grpcMetrics, metricsErr := observability.NewGRPCMetrics()
		if metricsErr != nil {
			logger.Warn("Failed to create gRPC metrics: %v", metricsErr)
		} else {
			grpcInstrumentation = observability.NewGRPCInstrumentation(grpcMetrics)
		}
	}

	// Cleanup function to shutdown telemetry
	cleanup := func() {
		if telemetry != nil && telemetry.IsEnabled() {
			logger.Info("Shutting down OpenTelemetry...")
			if shutdownErr := telemetry.Shutdown(context.Background()); shutdownErr != nil {
				logger.Warn("Error shutting down OpenTelemetry: %v", shutdownErr)
			}
		}
	}

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
		cleanup()
		return nil, nil, err
	}

	logger.Info("Setting up HTTP and gRPC servers...")
	handler := createMultiplexedHandler(site)

	host := c.GlobalString("host")
	if host == "" {
		host = "0.0.0.0"
	}
	addr := fmt.Sprintf("%s:%s", host, c.GlobalString("port"))
	logger.Info("Running simple_wiki server (commit %s) at http://%s", actualCommit, addr)

	return &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(handler, &http2.Server{}),
	}, cleanup, nil
}

func createMultiplexedHandler(site *server.Site) http.Handler {
	ginRouter := site.GinRouter()
	actualCommit := getCommitHash()
	buildTime := getBuildTime()
	grpcAPIServer := grpcapi.NewServer(
		actualCommit,
		buildTime,
		site,
		site.BleveIndexQueryer,
		site.GetJobQueueCoordinator(),
		logger,
		site.MarkdownRenderer,
		server.TemplateExecutor{},
		site.FrontmatterIndexQueryer,
	)
	
	// Build gRPC server options with interceptors
	var opts []grpc.ServerOption
	
	// Add OpenTelemetry instrumentation if enabled
	if grpcInstrumentation != nil {
		opts = append(opts,
			grpc.ChainUnaryInterceptor(
				grpcInstrumentation.UnaryServerInterceptor(),
				grpcAPIServer.LoggingInterceptor(),
			),
			grpc.ChainStreamInterceptor(
				grpcInstrumentation.StreamServerInterceptor(),
			),
		)
	} else {
		opts = append(opts, grpc.UnaryInterceptor(grpcAPIServer.LoggingInterceptor()))
	}
	
	grpcServer := grpc.NewServer(opts...)
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

func main() {
	app = cli.NewApp()
	app.Name = "simple_wiki"
	app.Usage = "a simple wiki"
	app.Version = getCommitHash()
	app.Compiled = time.Now()
	app.Action = func(c *cli.Context) error {
		srv, cleanup, err := setupServer(c)
		if err != nil {
			return err
		}
		defer cleanup()

		// Handle graceful shutdown
		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt, syscall.SIGTERM)

		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("Server error: %v", err)
			}
		}()

		<-done
		logger.Info("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		return srv.Shutdown(ctx)
	}
	app.Flags = getFlags()

	if err := app.Run(os.Args); err != nil {
		logger.Error("Error running app: %v", err)
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
