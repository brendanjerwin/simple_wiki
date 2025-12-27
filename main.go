package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/bootstrap"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/jcelliott/lumber"

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

func createSite(c *cli.Context) (*server.Site, error) {
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

	return server.NewSite(
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
}

func detectTailscale(ctx context.Context) *tailscale.Status {
	detector := tailscale.NewDetector()
	tsStatus, err := detector.Detect(ctx)
	if err != nil {
		logger.Warn("Error detecting Tailscale: %v", err)
		return &tailscale.Status{Available: false}
	}
	return tsStatus
}

func setupServer(c *cli.Context) (*bootstrap.ServerResult, error) {
	site, err := createSite(c)
	if err != nil {
		return nil, err
	}

	host := c.GlobalString("host")
	if host == "" {
		host = "0.0.0.0"
	}

	port := c.GlobalInt("port")
	tlsPort := c.GlobalInt("tls-port")
	if tlsPort == 0 {
		tlsPort = port + 1 // Default to adjacent port
	}

	tsStatus := detectTailscale(context.Background())
	mode := bootstrap.DetermineServerMode(tsStatus, c.GlobalBool("tailscale-serve"))

	actualCommit := getCommitHash()
	logger.Info("Running simple_wiki server (commit %s)", actualCommit)

	httpAddr := fmt.Sprintf("%s:%d", host, port)
	buildTime := getBuildTime()

	switch mode {
	case bootstrap.ModePlainHTTP:
		return bootstrap.SetupPlainHTTP(httpAddr, site, logger, actualCommit, buildTime)
	case bootstrap.ModeTailscaleServe:
		return bootstrap.SetupTailscaleServe(
			httpAddr, tsStatus.DNSName, c.GlobalBool("force-redirect-tailnet-https"),
			site, logger, actualCommit, buildTime,
		)
	case bootstrap.ModeFullTLS:
		return bootstrap.SetupFullTLS(
			httpAddr, tlsPort, tsStatus.DNSName,
			site, logger, actualCommit, buildTime,
		)
	default:
		return nil, fmt.Errorf("unknown server mode: %v", mode)
	}
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

		// Start the main server in a goroutine
		serverErr := make(chan error, 1)
		go func() {
			serverErr <- config.MainServer.Serve(config.MainListener)
		}()

		// Wait for shutdown signal or server error
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		select {
		case err := <-serverErr:
			return err
		case sig := <-quit:
			logger.Info("Received signal %v, shutting down gracefully...", sig)
		}

		// Graceful shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Shutdown redirect server first if it exists
		if config.RedirectServer != nil {
			if err := config.RedirectServer.Shutdown(ctx); err != nil {
				logger.Error("Error shutting down redirect server: %v", err)
			}
		}

		// Shutdown main server
		if err := config.MainServer.Shutdown(ctx); err != nil {
			logger.Error("Error shutting down main server: %v", err)
			return err
		}

		logger.Info("Server shutdown complete")
		return nil
	}
	app.Flags = getFlags()

	if err := app.Run(os.Args); err != nil {
		if logger != nil {
			logger.Error("Error running app: %v", err)
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		}
		os.Exit(1)
	}
}

const (
	defaultHTTPPort          = 8050
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
		cli.IntFlag{
			Name:  "port,p",
			Value: defaultHTTPPort,
			Usage: "HTTP port to use",
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
		cli.BoolFlag{
			Name:  "force-redirect-tailnet-https",
			Usage: "Force redirect tailnet clients to HTTPS on the tailnet hostname",
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
