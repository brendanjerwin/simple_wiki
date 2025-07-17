package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	grpcapi "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
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
	commit  = "n/a"
	logger  *lumber.ConsoleLogger
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
// Otherwise, returns the compiled time.
func getBuildTime() time.Time {
	return time.Now()
}

var app *cli.App

func main() {
	app = cli.NewApp()
	app.Name = "simple_wiki"
	app.Usage = "a simple wiki"
	app.Compiled = time.Now()
	app.Action = func(c *cli.Context) error {
		pathToData := c.GlobalString("data")
		if err := os.MkdirAll(pathToData, 0755); err != nil {
			return err
		}

		grpcServer := grpc.NewServer()
		logger = makeLogger(c.GlobalBool("debug"))
		site := server.NewSite(
			pathToData,
			c.GlobalString("css"),
			c.GlobalString("default-page"),
			c.GlobalString("lock"),
			c.GlobalInt("debounce"),
			c.GlobalString("cookie-secret"),
			c.GlobalString("access-code"),
			!c.GlobalBool("block-file-uploads"),
			c.GlobalUint("max-upload-mb"),
			c.GlobalUint("max-document-length"),
			logger,
		)
		ginRouter := site.GinRouter()
		actualCommit := getCommitHash()
		buildTime := getBuildTime()
		grpcAPIServer := grpcapi.NewServer(actualCommit, buildTime, site, logger)
		grpcServer = grpc.NewServer(grpc.UnaryInterceptor(grpcAPIServer.LoggingInterceptor()))
		grpcAPIServer.RegisterWithServer(grpcServer)

		reflection.Register(grpcServer)

		wrappedGrpc := grpcweb.WrapServer(grpcServer,
			// Enable CORS so browser clients can make requests
			grpcweb.WithOriginFunc(func(_ string) bool { return true }),
		)

		// 5. Create a multiplexer to route traffic to either gRPC or Gin.
		multiplexedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
				logger.Debug("gRPC-ish request: %s %s", r.Method, r.URL.Path)
				wrappedGrpc.ServeHTTP(w, r)
				return
			}
			logger.Debug("Gin request: %s %s", r.Method, r.URL.Path)
			ginRouter.ServeHTTP(w, r)
		})

		// 6. Determine host and port, then start the server
		host := c.GlobalString("host")
		if host == "" {
			host = "0.0.0.0"
		}
		addr := fmt.Sprintf("%s:%s", host, c.GlobalString("port"))
		logger.Info("Running simple_wiki server (commit %s) at http://%s", actualCommit, addr)

		srv := &http.Server{
			Addr:    addr,
			Handler: h2c.NewHandler(multiplexedHandler, &http2.Server{}),
		}
		return srv.ListenAndServe()
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
		cli.StringFlag{
			Name:  "lock",
			Value: "",
			Usage: "password to lock editing all files (default: all pages unlocked)",
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
			Name:  "access-code",
			Value: "",
			Usage: "Secret code to login with before accessing any wiki stuff",
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

func makeLogger(debug bool) *lumber.ConsoleLogger {
	if !debug {
		return lumber.NewConsoleLogger(lumber.WARN)
	}
	return lumber.NewConsoleLogger(lumber.TRACE)
}
