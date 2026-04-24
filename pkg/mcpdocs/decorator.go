// Package mcpdocs surfaces proto-declared MCP documentation extensions onto
// runtime-registered MCP tools. The codegen plugin
// (protoc-gen-go-mcp) bakes Description and Annotations into package-level
// mcp.Tool variables from proto comments. Decorate (called once after the
// codegen-generated ForwardToConnect* functions register the tools) walks the
// global proto registry, looks up each method's matching MCP tool, and
// re-registers it with description / annotations sourced from the
// (api.v1.description), (api.v1.example_request), (api.v1.read_only), and
// (api.v1.long_running) extensions.
//
// Services that declare none of these extensions are unaffected — the
// decorator is strictly additive, so it is safe to call before all services
// have been ported.
package mcpdocs

import (
	"fmt"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// ServiceDescription is what the global walk emits for each service that
// declares (api.v1.service_description). Returned by Decorate so callers (for
// example, an HTTP catalog page) can surface the service-level overview that
// MCP itself does not yet have a slot for.
type ServiceDescription struct {
	// Service is the fully-qualified proto service name (e.g.
	// "api.v1.AgentMetadataService").
	Service string
	// Description is the verbatim text of the (api.v1.service_description)
	// extension.
	Description string
}

// Decorate walks every proto service registered in the global proto registry
// and applies extension-driven overrides to the matching MCP tools on s. It
// returns the per-service descriptions so callers can publish them somewhere
// MCP itself does not surface (catalog page, /help, etc.).
//
// Must be called AFTER the codegen-generated ForwardToConnect* functions so
// the tools exist on the server.
func Decorate(s *mcpserver.MCPServer) []ServiceDescription {
	var serviceDescriptions []ServiceDescription
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		serviceDescriptions = append(serviceDescriptions, decorateFileServices(s, fd)...)
		return true
	})
	return serviceDescriptions
}

// decorateFileServices iterates every service in fd and its methods, applying
// extension-driven overrides to the matching MCP tools.
func decorateFileServices(s *mcpserver.MCPServer, fd protoreflect.FileDescriptor) []ServiceDescription {
	var out []ServiceDescription
	services := fd.Services()
	for i := 0; i < services.Len(); i++ {
		svc := services.Get(i)
		if desc := readServiceDescription(svc); desc != "" {
			out = append(out, ServiceDescription{
				Service:     string(svc.FullName()),
				Description: desc,
			})
		}
		methods := svc.Methods()
		for j := 0; j < methods.Len(); j++ {
			method := methods.Get(j)
			toolName := computeMCPToolName(svc, method)
			tool := s.GetTool(toolName)
			if tool == nil {
				// Service is not exposed as MCP (no apiv1mcp file). Skip.
				continue
			}
			decorateOne(s, tool, method)
		}
	}
	return out
}

// decorateOne re-registers the tool with description and annotations sourced
// from the method's extensions, when set. The handler is preserved.
func decorateOne(s *mcpserver.MCPServer, tool *mcpserver.ServerTool, method protoreflect.MethodDescriptor) {
	if !methodHasMCPOverrides(method) {
		return
	}
	updated := tool.Tool
	if desc := readStringExtension(method, apiv1.E_Description); desc != "" {
		updated.Description = desc
	}
	applyAnnotations(&updated.Annotations, method)
	if example := readStringExtension(method, apiv1.E_ExampleRequest); example != "" {
		updated.Description = strings.TrimRight(updated.Description, "\n") +
			"\n\nExample request:\n```json\n" + example + "\n```"
	}
	s.AddTool(updated, tool.Handler)
}

// applyAnnotations sets the MCP tool annotation hints that map cleanly onto
// our extensions. (api.v1.read_only) maps to ReadOnlyHint=true.
// (api.v1.long_running) is surfaced as IdempotentHint=false (a long-running
// job may not be safe to retry without coordination) and an OpenWorldHint
// hint of true (the operation interacts with external state). When a richer
// MCP annotation slot for "long-running" lands upstream, we'll switch to it
// here without touching call sites.
func applyAnnotations(ann *mcpgo.ToolAnnotation, method protoreflect.MethodDescriptor) {
	if readBoolExtension(method, apiv1.E_ReadOnly) {
		readOnly := true
		ann.ReadOnlyHint = &readOnly
	}
	if readBoolExtension(method, apiv1.E_LongRunning) {
		notIdempotent := false
		open := true
		ann.IdempotentHint = &notIdempotent
		ann.OpenWorldHint = &open
	}
}

// methodHasMCPOverrides returns true when the method declares any of the
// known MCP doc extensions. Lets us short-circuit when nothing has been
// ported.
func methodHasMCPOverrides(method protoreflect.MethodDescriptor) bool {
	opts := method.Options()
	if opts == nil {
		return false
	}
	return proto.HasExtension(opts, apiv1.E_Description) ||
		proto.HasExtension(opts, apiv1.E_ExampleRequest) ||
		proto.HasExtension(opts, apiv1.E_ReadOnly) ||
		proto.HasExtension(opts, apiv1.E_LongRunning)
}

// readStringExtension returns the string value of the supplied method
// extension, or the empty string when unset.
func readStringExtension(method protoreflect.MethodDescriptor, ext protoreflect.ExtensionType) string {
	opts := method.Options()
	if opts == nil {
		return ""
	}
	if !proto.HasExtension(opts, ext) {
		return ""
	}
	v, ok := proto.GetExtension(opts, ext).(string)
	if !ok {
		return ""
	}
	return v
}

// readBoolExtension returns the boolean value of the supplied method
// extension, or false when unset.
func readBoolExtension(method protoreflect.MethodDescriptor, ext protoreflect.ExtensionType) bool {
	opts := method.Options()
	if opts == nil {
		return false
	}
	if !proto.HasExtension(opts, ext) {
		return false
	}
	v, ok := proto.GetExtension(opts, ext).(bool)
	if !ok {
		return false
	}
	return v
}

// readServiceDescription returns the (api.v1.service_description) value on
// the service, or the empty string when unset.
func readServiceDescription(svc protoreflect.ServiceDescriptor) string {
	opts := svc.Options()
	if opts == nil {
		return ""
	}
	if !proto.HasExtension(opts, apiv1.E_ServiceDescription) {
		return ""
	}
	v, ok := proto.GetExtension(opts, apiv1.E_ServiceDescription).(string)
	if !ok {
		return ""
	}
	return v
}

// computeMCPToolName mirrors the naming convention the apiv1mcp codegen
// plugin uses: <package_with_underscores>_<ServiceName>_<MethodName>.
func computeMCPToolName(svc protoreflect.ServiceDescriptor, method protoreflect.MethodDescriptor) string {
	pkg := strings.ReplaceAll(string(svc.ParentFile().Package()), ".", "_")
	return fmt.Sprintf("%s_%s_%s", pkg, svc.Name(), method.Name())
}
