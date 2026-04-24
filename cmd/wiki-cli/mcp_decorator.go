package main

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

// decorateMCPToolsFromExtensions walks every proto service registered in the
// global proto registry and, for each method, looks up the matching MCP tool
// the codegen plugin emitted. When the method declares any of the
// (api.v1.description), (api.v1.example_request), (api.v1.read_only),
// (api.v1.long_running) extensions, the tool's description and annotations are
// overridden in place so the runtime surfaces the proto-author's intent
// instead of whatever stub comment ended up baked in by the codegen plugin.
//
// Services that have not been ported (no extension set) are unaffected — the
// decorator is strictly additive.
//
// Must be called AFTER registerToolHandlers so the tools exist on the server.
func decorateMCPToolsFromExtensions(s *mcpserver.MCPServer) {
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		decorateFileServices(s, fd)
		return true
	})
}

// decorateFileServices iterates every service in fd and its methods, applying
// extension-driven overrides to the matching MCP tools.
func decorateFileServices(s *mcpserver.MCPServer, fd protoreflect.FileDescriptor) {
	services := fd.Services()
	for i := 0; i < services.Len(); i++ {
		svc := services.Get(i)
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
	if readBoolExtension(method, apiv1.E_ReadOnly) {
		readOnly := true
		updated.Annotations.ReadOnlyHint = &readOnly
	}
	// (api.v1.example_request) is currently informational; the example is
	// surfaced through the description so callers see it inline. When mcp-go
	// grows a richer schema for examples this is where we will populate it.
	if example := readStringExtension(method, apiv1.E_ExampleRequest); example != "" {
		updated.Description = strings.TrimRight(updated.Description, "\n") +
			"\n\nExample request:\n```json\n" + example + "\n```"
	}
	s.AddTool(updated, tool.Handler)
}

// methodHasMCPOverrides returns true when the method declares any of the
// known MCP doc extensions. Lets us short-circuit the more expensive lookup
// when nothing has been ported.
func methodHasMCPOverrides(method protoreflect.MethodDescriptor) bool {
	opts := method.Options()
	if opts == nil {
		return false
	}
	if proto.HasExtension(opts, apiv1.E_Description) ||
		proto.HasExtension(opts, apiv1.E_ExampleRequest) ||
		proto.HasExtension(opts, apiv1.E_ReadOnly) ||
		proto.HasExtension(opts, apiv1.E_LongRunning) {
		return true
	}
	return false
}

// readStringExtension returns the string value of the supplied extension on
// the method, or the empty string when unset.
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

// readBoolExtension returns the boolean value of the supplied extension on
// the method, or false when unset.
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

// computeMCPToolName mirrors the naming convention the apiv1mcp codegen
// plugin uses: api_v1_<ServiceName>_<MethodName>.
func computeMCPToolName(svc protoreflect.ServiceDescriptor, method protoreflect.MethodDescriptor) string {
	pkg := strings.ReplaceAll(string(svc.ParentFile().Package()), ".", "_")
	return fmt.Sprintf("%s_%s_%s", pkg, svc.Name(), method.Name())
}

// Suppress the unused-import warning for the mcpgo package on builds where
// other files in this package don't import it. We keep the import explicit so
// future contributors see at a glance which mcp.Tool fields are mutable.
var _ = mcpgo.Tool{}
