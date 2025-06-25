package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	grpcApi "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
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
	version string = "dev"
	commit  string = "n/a"
)

func main() {
	app := cli.NewApp()
	app.Name = "simple_wiki"
	app.Usage = "a simple wiki"
	app.Version = version
	app.Compiled = time.Now()
	app.Action = func(c *cli.Context) error {
		pathToData := c.GlobalString("data")
		os.MkdirAll(pathToData, 0755)

		// 1. Create the gRPC Server
		grpcServer := grpc.NewServer()

		// 2. Create and register your gRPC services
		debugSvc := grpcApi.NewServer(version, commit, app.Compiled)
		debugSvc.RegisterWithServer(grpcServer)

		// Enable reflection for gRPC services
		reflection.Register(grpcServer)

		// 3. Create the gRPC-web wrapper
		wrappedGrpc := grpcweb.WrapServer(grpcServer,
			// Enable CORS so browser clients can make requests
			grpcweb.WithOriginFunc(func(origin string) bool { return true }),
		)

		logger := makeLogger(c.GlobalBool("debug"))
		// 4. Create the Gin router using our new router function
		router := server.NewRouter(
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

		// 5. Create a multiplexer to route traffic to either gRPC or Gin.
		multiplexedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
				logger.Debug("gRPC-ish request: %s %s", r.Method, r.URL.Path)
				wrappedGrpc.ServeHTTP(w, r)
				return
			}
			logger.Debug("Gin request: %s %s", r.Method, r.URL.Path)
			router.ServeHTTP(w, r)
		})

		// 6. Determine host and port, then start the server
		host := c.GlobalString("host")
		if host == "" {
			host = "0.0.0.0"
		}
		addr := fmt.Sprintf("%s:%s", host, c.GlobalString("port"))
		fmt.Printf("\nRunning simple_wiki server (version %s) at http://%s\n\n", version, addr)

		srv := &http.Server{
			Addr:    addr,
			Handler: h2c.NewHandler(multiplexedHandler, &http2.Server{}),
		}
		return srv.ListenAndServe()
	}
	app.Flags = []cli.Flag{
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
			Value: 500,
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
			Value: 100,
			Usage: "Largest file upload (in mb) allowed",
		},
		cli.UintFlag{
			Name:  "max-document-length",
			Value: 100000000,
			Usage: "Largest wiki page (in characters) allowed",
		},
	}

	app.Run(os.Args)
}

func makeLogger(debug bool) *lumber.ConsoleLogger {
	if !debug {
		return lumber.NewConsoleLogger(lumber.WARN)
	}
	return lumber.NewConsoleLogger(lumber.TRACE)
}
