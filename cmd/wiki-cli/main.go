// Package main is the wiki-cli command-line tool for interacting with the simple_wiki gRPC API.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"connectrpc.com/grpcreflect"
	// Blank imports register all wiki API proto types in protoregistry.GlobalFiles / GlobalTypes.
	_ "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/pelletier/go-toml/v2"
	cli "gopkg.in/urfave/cli.v1"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

const (
	defaultWikiURL   = "https://wiki.monster-orfe.ts.net"
	pathSeparator    = "/"
	writeErrTemplate = "write error: %w"

	// versionCheckTimeoutMs is the maximum time to wait for the version check API call.
	versionCheckTimeoutMs = 3000
)

var (
	// version is set by ldflags at build time.
	version = "dev"
	// commit is the git commit hash this binary was built from, set by ldflags.
	commit = "dev"
)

func main() {
	app := cli.NewApp()
	app.Name = "wiki-cli"
	app.Usage = "CLI for the simple_wiki gRPC API. Discover services, inspect schemas, and call methods."
	app.Description = appDescription()
	app.Version = version
	app.HideVersion = false
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "url, u",
			Usage:  "wiki base URL (env: WIKI_URL)",
			EnvVar: "WIKI_URL",
			Value:  defaultWikiURL,
		},
	}
	app.Before = func(c *cli.Context) error {
		if err := checkVersionCompatibility(c.GlobalString("url")); err != nil {
			// Print directly and exit to avoid urfave/cli dumping help text on error.
			if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
				os.Exit(2)
			}
			os.Exit(1)
		}
		return nil
	}
	app.Commands = buildCommands()

	if err := app.Run(os.Args); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

// versionResponse is the JSON shape returned by SystemInfoService/GetVersion.
type versionResponse struct {
	Commit string `json:"commit"`
}

// checkVersionCompatibility calls the wiki's GetVersion endpoint and compares
// the server's commit with this binary's embedded commit. If they differ, it
// returns an error telling the caller to download the latest binary.
// Skipped only for dev builds. All other failures (unreachable server, bad
// response, version mismatch) are hard errors — there is no offline mode.
func checkVersionCompatibility(baseURL string) error {
	if commit == "dev" {
		return nil // dev build, skip check
	}

	ctx, cancel := context.WithTimeout(context.Background(), versionCheckTimeoutMs*time.Millisecond)
	defer cancel()

	reqURL := strings.TrimRight(baseURL, pathSeparator) +
		pathSeparator + "api.v1.SystemInfoService" +
		pathSeparator + "GetVersion"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("UNREACHABLE: could not build version check request for %s: %w", baseURL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("UNREACHABLE: cannot connect to wiki server at %s\n\n"+
			"Ensure the wiki is running and the URL is correct.\n"+
			"Set WIKI_URL or pass --url to override (default: %s)",
			baseURL, defaultWikiURL)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("UNREACHABLE: wiki server at %s returned HTTP %d during version check", baseURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("UNREACHABLE: failed to read version response from %s: %w", baseURL, err)
	}

	var ver versionResponse
	if err := json.Unmarshal(body, &ver); err != nil {
		return fmt.Errorf("UNREACHABLE: wiki server at %s returned invalid version response", baseURL)
	}

	if ver.Commit != "" && !commitsMatch(commit, ver.Commit) {
		return fmt.Errorf(
			"VERSION MISMATCH: this wiki-cli was built from commit %.8s but the wiki server is running %s\n\n"+
				"Download the latest version:\n"+
				"  curl -o wiki-cli %s/cli/wiki-cli-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/') && chmod +x wiki-cli\n\n"+
				"Or manually from: %s/cli/",
			commit, ver.Commit, baseURL, baseURL,
		)
	}

	return nil
}

// commitsMatch checks whether the CLI's embedded commit matches the server's
// reported commit. The server may report either a raw hash ("adbef9d2...") or
// a tagged format like "v3.5.0 (adbef9d)". This function extracts the hash
// portion and compares using prefix matching so that a short hash from the
// server matches the full hash embedded in the CLI.
func commitsMatch(cliCommit, serverCommit string) bool {
	serverHash := serverCommit

	// If the server commit is in tagged format "v3.5.0 (adbef9d)", extract
	// the hash from inside the parentheses.
	if open := strings.LastIndex(serverCommit, "("); open >= 0 {
		if closeIdx := strings.LastIndex(serverCommit, ")"); closeIdx > open {
			serverHash = serverCommit[open+1 : closeIdx]
		}
	}

	// Compare using prefix matching: whichever is shorter must be a prefix
	// of the longer one. This handles short-hash vs full-hash comparison.
	if len(serverHash) < len(cliCommit) {
		return strings.HasPrefix(cliCommit, serverHash)
	}
	return strings.HasPrefix(serverHash, cliCommit)
}

func appDescription() string {
	return `wiki-cli lets you explore and interact with the wiki's gRPC API using the
Connect protocol over HTTP/1.1. It works through reverse proxies (e.g. Tailscale Serve).

GETTING STARTED — DISCOVERY WORKFLOW:

  1. List available services:
       wiki-cli list

  2. See what methods a service offers:
       wiki-cli methods api.v1.SearchService

  3. Inspect the request message schema to learn what fields to send:
       wiki-cli describe api.v1.SearchContentRequest

  4. Call the method with a JSON payload:
       wiki-cli call api.v1.SearchService/SearchContent -d '{"query":"test"}'

  Or use TOML for the payload:
       wiki-cli call api.v1.PageManagementService/ReadPage -d 'page_name = "home"'

Short service names work too (e.g. "SearchService" instead of "api.v1.SearchService").
The describe command works on both services (shows methods) and message types (shows fields).
To inspect a response type, use describe on the output type shown in the method signature.

TARGET URL:

  Set WIKI_URL env var or pass --url. Default: ` + defaultWikiURL + `

TIPS FOR AI AGENTS:

  - Start with "wiki-cli list" to discover all available services.
  - Use "wiki-cli methods" (no args) to see every callable method across all services.
  - Use "wiki-cli describe <RequestMessageType>" to learn the exact fields, types,
    and cardinality (repeated/optional) before constructing a call payload.
  - The -d flag accepts both JSON and TOML. TOML is convenient for simple key=value payloads.
  - Responses are always pretty-printed JSON.
  - All methods are unary (request/response) except StreamJobStatus (server streaming).`
}

func buildCommands() []cli.Command {
	urlFlag := cli.StringFlag{
		Name:   "url, u",
		Usage:  "wiki base URL (env: WIKI_URL)",
		EnvVar: "WIKI_URL",
		Value:  defaultWikiURL,
	}

	return []cli.Command{
		{
			Name:  "list",
			Usage: "List all gRPC services available on the wiki",
			Description: `Lists all gRPC services. Tries server reflection first (requires a running wiki),
falls back to the proto registry embedded in this binary.

Example:
  wiki-cli list

Output is one fully-qualified service name per line. Use "wiki-cli methods <service>"
to see the methods available on a service, or "wiki-cli describe <service>" for a
full description including input/output types.`,
			Flags: []cli.Flag{urlFlag},
			Action: func(c *cli.Context) error {
				return runList(c.String("url"))
			},
		},
		{
			Name:      "methods",
			Usage:     "List callable methods (all services, or one specific service)",
			ArgsUsage: "[service]",
			Description: `Lists methods in the format "service/Method", ready to use with "wiki-cli call".

Without arguments, lists every method across all services:
  wiki-cli methods

With a service name, lists only that service's methods:
  wiki-cli methods api.v1.SearchService
  wiki-cli methods SearchService          # short name also works

Each output line can be passed directly to "wiki-cli call".`,
			Flags: []cli.Flag{urlFlag},
			Action: func(c *cli.Context) error {
				return runMethods(c.Args().First())
			},
		},
		{
			Name:      "describe",
			Usage:     "Describe a service (methods + types) or a message type (fields + types)",
			ArgsUsage: "<name>",
			Description: `Inspects a service or message type from the embedded proto registry (works offline).

Describe a service to see its methods and their input/output types:
  wiki-cli describe api.v1.SearchService

Describe a message type to see its fields, types, and cardinality:
  wiki-cli describe api.v1.SearchContentRequest

This is essential for learning what fields to include in a "wiki-cli call" payload.
Fields marked (repeated) accept JSON arrays. Fields marked (optional) can be omitted.
When a field's type is another message (e.g. "google.protobuf.Struct"), you can
describe that type too to see its nested structure.`,
			Action: func(c *cli.Context) error {
				name := c.Args().First()
				if name == "" {
					return errors.New("name argument required — try: wiki-cli describe api.v1.SearchService")
				}
				return runDescribe(name)
			},
		},
		buildCallCommand(urlFlag),
	}
}

func buildCallCommand(urlFlag cli.StringFlag) cli.Command {
	return cli.Command{
		Name:      "call",
		Usage:     "Call a gRPC method with a JSON or TOML payload",
		ArgsUsage: "<Service/Method>",
		Description: `Performs a unary RPC call using the Connect protocol over HTTP/1.1.

The method argument is "Service/Method" as shown by "wiki-cli methods":
  wiki-cli call api.v1.SearchService/SearchContent -d '{"query":"test"}'

Short service names work:
  wiki-cli call SearchService/SearchContent -d '{"query":"test"}'

TOML payloads are auto-detected (anything not starting with { or [):
  wiki-cli call PageManagementService/ReadPage -d 'page_name = "home"'

Omitting -d sends an empty JSON object {}:
  wiki-cli call SystemInfoService/GetVersion

The response is always pretty-printed JSON. Non-2xx status codes cause a non-zero exit.

To learn what fields a method expects, use:
  wiki-cli describe <RequestMessageType>`,
		Flags: []cli.Flag{
			urlFlag,
			cli.StringFlag{
				Name:  "data, d",
				Usage: "request payload (JSON or TOML); omit for empty request",
			},
		},
		Action: func(c *cli.Context) error {
			method := c.Args().First()
			if method == "" {
				return errors.New("method argument required — try: wiki-cli call api.v1.SearchService/SearchContent -d '{\"query\":\"test\"}'")
			}
			return runCall(context.Background(), c.String("url"), method, c.String("data"))
		},
	}
}

// runList lists all gRPC services.  It tries server reflection when a URL is
// reachable, and falls back to the types embedded in this binary.
func runList(baseURL string) error {
	services, err := listServicesFromReflection(context.Background(), baseURL)
	if err != nil {
		// Fall back to embedded registry
		services = listServicesFromRegistry()
	}

	slices.SortFunc(services, func(a, b protoreflect.FullName) int {
		return strings.Compare(string(a), string(b))
	})
	for _, s := range services {
		if _, err := fmt.Println(s); err != nil {
			return fmt.Errorf(writeErrTemplate, err)
		}
	}
	return nil
}

// runMethods lists methods for one service (or all services when serviceName is empty).
func runMethods(serviceName string) error {
	if serviceName == "" {
		return printAllMethods()
	}
	return printServiceMethods(serviceName)
}

func printAllMethods() error {
	var svcs []protoreflect.ServiceDescriptor
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for i := range fd.Services().Len() {
			svc := fd.Services().Get(i)
			if !isInternalService(svc.FullName()) {
				svcs = append(svcs, svc)
			}
		}
		return true
	})

	slices.SortFunc(svcs, func(a, b protoreflect.ServiceDescriptor) int {
		return strings.Compare(string(a.FullName()), string(b.FullName()))
	})
	for _, svc := range svcs {
		if err := printMethodsForService(svc); err != nil {
			return err
		}
	}
	return nil
}

func printServiceMethods(serviceName string) error {
	svc, err := findServiceDescriptor(serviceName)
	if err != nil {
		return err
	}
	return printMethodsForService(svc)
}

func printMethodsForService(svc protoreflect.ServiceDescriptor) error {
	for i := range svc.Methods().Len() {
		m := svc.Methods().Get(i)
		if _, err := fmt.Printf("%s%s%s\n", svc.FullName(), pathSeparator, m.Name()); err != nil {
			return fmt.Errorf(writeErrTemplate, err)
		}
	}
	return nil
}

// runDescribe describes a service or message type by its full or short name.
func runDescribe(name string) error {
	// Try as a message type first.
	if msgType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(name)); err == nil {
		return describeMessage(msgType.Descriptor())
	}

	// Try as a service descriptor.
	if svc, err := findServiceDescriptor(name); err == nil {
		return describeService(svc)
	}

	return fmt.Errorf("no service or message type found with name %q", name)
}

// describeService prints a human-readable description of a gRPC service.
func describeService(svc protoreflect.ServiceDescriptor) error {
	if _, err := fmt.Printf("Service: %s\n", svc.FullName()); err != nil {
		return fmt.Errorf(writeErrTemplate, err)
	}
	if _, err := fmt.Println("Methods:"); err != nil {
		return fmt.Errorf(writeErrTemplate, err)
	}
	for i := range svc.Methods().Len() {
		m := svc.Methods().Get(i)
		streaming := streamingLabel(m)
		if _, err := fmt.Printf("  %s(%s) -> %s%s\n",
			m.Name(),
			m.Input().FullName(),
			m.Output().FullName(),
			streaming,
		); err != nil {
			return fmt.Errorf(writeErrTemplate, err)
		}
	}
	return nil
}

func streamingLabel(m protoreflect.MethodDescriptor) string {
	switch {
	case m.IsStreamingClient() && m.IsStreamingServer():
		return " [bidi streaming]"
	case m.IsStreamingClient():
		return " [client streaming]"
	case m.IsStreamingServer():
		return " [server streaming]"
	default:
		return ""
	}
}

// describeMessage prints a human-readable description of a proto message type.
func describeMessage(md protoreflect.MessageDescriptor) error {
	if _, err := fmt.Printf("Message: %s\n", md.FullName()); err != nil {
		return fmt.Errorf(writeErrTemplate, err)
	}
	if _, err := fmt.Println("Fields:"); err != nil {
		return fmt.Errorf(writeErrTemplate, err)
	}
	for i := range md.Fields().Len() {
		f := md.Fields().Get(i)
		typeName := fieldTypeName(f)
		repeated := ""
		if f.IsList() {
			repeated = " (repeated)"
		}
		optional := ""
		if f.HasOptionalKeyword() {
			optional = " (optional)"
		}
		if _, err := fmt.Printf("  %s: %s%s%s\n", f.Name(), typeName, repeated, optional); err != nil {
			return fmt.Errorf(writeErrTemplate, err)
		}
	}
	return nil
}

// fieldTypeName returns a human-readable type name for a proto field.
func fieldTypeName(f protoreflect.FieldDescriptor) string {
	switch f.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return string(f.Message().FullName())
	case protoreflect.EnumKind:
		return string(f.Enum().FullName())
	default:
		return f.Kind().String()
	}
}

// runCall performs a unary RPC call using the Connect protocol over HTTP/1.1.
func runCall(ctx context.Context, baseURL, method, rawInput string) error {
	// Split "api.v1.SearchService/SearchContent" → service + method.
	slash := strings.LastIndex(method, pathSeparator)
	if slash < 0 {
		return fmt.Errorf("invalid method %q: expected Service/Method (e.g. api.v1.SearchService/SearchContent)", method)
	}
	serviceArg := method[:slash]
	methodName := method[slash+1:]

	// Resolve the full service name (supports short names like "SearchService").
	fullServiceName, err := resolveServiceName(serviceArg)
	if err != nil {
		return err
	}

	// Convert the input payload to JSON.
	jsonBody, err := toJSON(rawInput)
	if err != nil {
		return fmt.Errorf("failed to parse request data: %w", err)
	}

	// Build the Connect-protocol URL.
	reqURL := strings.TrimRight(baseURL, pathSeparator) + pathSeparator + fullServiceName + pathSeparator + methodName

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if err := printResponseBody(body); err != nil {
		return err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	return nil
}

// printResponseBody pretty-prints JSON response body, or prints raw text if not valid JSON.
func printResponseBody(body []byte) error {
	var v any
	if jsonErr := json.Unmarshal(body, &v); jsonErr == nil {
		out, _ := json.MarshalIndent(v, "", "  ")
		if _, err := fmt.Println(string(out)); err != nil {
			return fmt.Errorf(writeErrTemplate, err)
		}
	} else {
		if _, err := fmt.Println(string(body)); err != nil {
			return fmt.Errorf(writeErrTemplate, err)
		}
	}
	return nil
}

// toJSON converts a raw string payload to JSON bytes.
// Strings that start with '{' or '[' are treated as JSON; anything else is
// parsed as TOML and converted to JSON.
func toJSON(input string) ([]byte, error) {
	if input == "" {
		return []byte("{}"), nil
	}
	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return []byte(input), nil
	}
	// Treat as TOML.
	var m map[string]any
	if err := toml.Unmarshal([]byte(input), &m); err != nil {
		return nil, fmt.Errorf("input is neither valid JSON nor valid TOML: %w", err)
	}
	return json.Marshal(m)
}

// listServicesFromReflection contacts the server reflection API and returns service names.
func listServicesFromReflection(ctx context.Context, baseURL string) ([]protoreflect.FullName, error) {
	client := grpcreflect.NewClient(http.DefaultClient, baseURL)
	stream := client.NewStream(ctx)
	defer func() { _, _ = stream.Close() }()
	return stream.ListServices()
}

// isInternalService returns true for gRPC reflection and other internal services
// that should not be shown in list/methods output.
func isInternalService(name protoreflect.FullName) bool {
	n := string(name)
	return strings.HasPrefix(n, "grpc.reflection") ||
		strings.HasPrefix(n, "connectext.grpc.reflection")
}

// listServicesFromRegistry returns all service names registered in the binary.
func listServicesFromRegistry() []protoreflect.FullName {
	var services []protoreflect.FullName
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for i := range fd.Services().Len() {
			svc := fd.Services().Get(i)
			if !isInternalService(svc.FullName()) {
				services = append(services, svc.FullName())
			}
		}
		return true
	})
	return services
}

// findServiceDescriptor finds a ServiceDescriptor by full or short name.
func findServiceDescriptor(name string) (protoreflect.ServiceDescriptor, error) {
	var found protoreflect.ServiceDescriptor
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for i := range fd.Services().Len() {
			svc := fd.Services().Get(i)
			fullName := string(svc.FullName())
			if fullName == name || strings.HasSuffix(fullName, "."+name) {
				found = svc
				return false
			}
		}
		return true
	})
	if found == nil {
		return nil, fmt.Errorf("service %q not found", name)
	}
	return found, nil
}

// resolveServiceName resolves a short or full service name to its fully-qualified name.
func resolveServiceName(name string) (string, error) {
	svc, err := findServiceDescriptor(name)
	if err != nil {
		return "", err
	}
	return string(svc.FullName()), nil
}
