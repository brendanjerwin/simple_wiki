// Package main is the wiki-cli command-line tool for interacting with the simple_wiki gRPC API.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"connectrpc.com/grpcreflect"
	// Blank imports register all wiki API proto types in protoregistry.GlobalFiles / GlobalTypes.
	_ "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/pelletier/go-toml/v2"
	cli "gopkg.in/urfave/cli.v1"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

const (
	defaultWikiURL = "https://wiki.monster-orfe.ts.net"
)

// version is set by ldflags at build time.
var version = "dev"

func main() {
	app := cli.NewApp()
	app.Name = "wiki-cli"
	app.Usage = "command-line interface for the simple_wiki gRPC API"
	app.Version = version
	app.HideVersion = false

	urlFlag := cli.StringFlag{
		Name:   "url, u",
		Usage:  "wiki base URL",
		EnvVar: "WIKI_URL",
		Value:  defaultWikiURL,
	}

	app.Commands = []cli.Command{
		{
			Name:  "list",
			Usage: "list all gRPC services",
			Flags: []cli.Flag{urlFlag},
			Action: func(c *cli.Context) error {
				return runList(c.String("url"))
			},
		},
		{
			Name:      "methods",
			Usage:     "list methods for a service (or all services if none given)",
			ArgsUsage: "[service]",
			Flags:     []cli.Flag{urlFlag},
			Action: func(c *cli.Context) error {
				return runMethods(c.Args().First())
			},
		},
		{
			Name:      "describe",
			Usage:     "describe a service or message type",
			ArgsUsage: "<name>",
			Action: func(c *cli.Context) error {
				name := c.Args().First()
				if name == "" {
					return fmt.Errorf("name argument required")
				}
				return runDescribe(name)
			},
		},
		{
			Name:      "call",
			Usage:     "call a gRPC method",
			ArgsUsage: "<Service/Method>",
			Flags: []cli.Flag{
				urlFlag,
				cli.StringFlag{
					Name:  "data, d",
					Usage: "request payload (JSON or TOML)",
				},
			},
			Action: func(c *cli.Context) error {
				method := c.Args().First()
				if method == "" {
					return fmt.Errorf("method argument required (e.g. api.v1.SearchService/SearchContent)")
				}
				return runCall(context.Background(), c.String("url"), method, c.String("data"))
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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

	sort.Slice(services, func(i, j int) bool { return services[i] < services[j] })
	for _, s := range services {
		fmt.Println(s)
	}
	return nil
}

// runMethods lists methods for one service (or all services when serviceName is empty).
func runMethods(serviceName string) error {
	if serviceName == "" {
		// List all methods across all services
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
		// sort by service name for deterministic output
		sort.Slice(svcs, func(i, j int) bool { return svcs[i].FullName() < svcs[j].FullName() })
		for _, svc := range svcs {
			for i := range svc.Methods().Len() {
				m := svc.Methods().Get(i)
				fmt.Printf("%s/%s\n", svc.FullName(), m.Name())
			}
		}
		return nil
	}

	svc, err := findServiceDescriptor(serviceName)
	if err != nil {
		return err
	}
	for i := range svc.Methods().Len() {
		m := svc.Methods().Get(i)
		fmt.Printf("%s/%s\n", svc.FullName(), m.Name())
	}
	return nil
}

// runDescribe describes a service or message type by its full or short name.
func runDescribe(name string) error {
	// Try as a message type first.
	if msgType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(name)); err == nil {
		describeMessage(msgType.Descriptor())
		return nil
	}

	// Try as a service descriptor.
	if svc, err := findServiceDescriptor(name); err == nil {
		describeService(svc)
		return nil
	}

	return fmt.Errorf("no service or message type found with name %q", name)
}

// describeService prints a human-readable description of a gRPC service.
func describeService(svc protoreflect.ServiceDescriptor) {
	fmt.Printf("Service: %s\n", svc.FullName())
	fmt.Println("Methods:")
	for i := range svc.Methods().Len() {
		m := svc.Methods().Get(i)
		streaming := ""
		switch {
		case m.IsStreamingClient() && m.IsStreamingServer():
			streaming = " [bidi streaming]"
		case m.IsStreamingClient():
			streaming = " [client streaming]"
		case m.IsStreamingServer():
			streaming = " [server streaming]"
		}
		fmt.Printf("  %s(%s) -> %s%s\n",
			m.Name(),
			m.Input().FullName(),
			m.Output().FullName(),
			streaming,
		)
	}
}

// describeMessage prints a human-readable description of a proto message type.
func describeMessage(md protoreflect.MessageDescriptor) {
	fmt.Printf("Message: %s\n", md.FullName())
	fmt.Println("Fields:")
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
		fmt.Printf("  %s: %s%s%s\n", f.Name(), typeName, repeated, optional)
	}
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
	slash := strings.LastIndex(method, "/")
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
	reqURL := strings.TrimRight(baseURL, "/") + "/" + fullServiceName + "/" + methodName

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
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Pretty-print JSON output.
	var v any
	if jsonErr := json.Unmarshal(body, &v); jsonErr == nil {
		out, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(out))
	} else {
		fmt.Println(string(body))
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("server returned %s", resp.Status)
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
			// Skip the built-in gRPC reflection service.
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
