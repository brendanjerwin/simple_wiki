//revive:disable:dot-imports
package main

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1mcp"
)

// noopAgentMetadataServer is an apiv1mcp.ConnectAgentMetadataServiceClient
// whose RPCs are never actually called by these tests — the decorator only
// inspects the registered MCP tool metadata, not the handler.
type noopAgentMetadataServer struct{}

var errAgentMetadataNoop = errors.New("noopAgentMetadataServer: not used in this test")

func (*noopAgentMetadataServer) AppendBackgroundActivitySummary(_ context.Context, _ *connect.Request[apiv1.AppendBackgroundActivitySummaryRequest]) (*connect.Response[apiv1.AppendBackgroundActivitySummaryResponse], error) {
	return nil, errAgentMetadataNoop
}

func (*noopAgentMetadataServer) DeleteSchedule(_ context.Context, _ *connect.Request[apiv1.DeleteScheduleRequest]) (*connect.Response[apiv1.DeleteScheduleResponse], error) {
	return nil, errAgentMetadataNoop
}

func (*noopAgentMetadataServer) GetChatContext(_ context.Context, _ *connect.Request[apiv1.GetChatContextRequest]) (*connect.Response[apiv1.GetChatContextResponse], error) {
	return nil, errAgentMetadataNoop
}

func (*noopAgentMetadataServer) ListSchedules(_ context.Context, _ *connect.Request[apiv1.ListSchedulesRequest]) (*connect.Response[apiv1.ListSchedulesResponse], error) {
	return nil, errAgentMetadataNoop
}

func (*noopAgentMetadataServer) UpdateChatContext(_ context.Context, _ *connect.Request[apiv1.UpdateChatContextRequest]) (*connect.Response[apiv1.UpdateChatContextResponse], error) {
	return nil, errAgentMetadataNoop
}

func (*noopAgentMetadataServer) UpsertSchedule(_ context.Context, _ *connect.Request[apiv1.UpsertScheduleRequest]) (*connect.Response[apiv1.UpsertScheduleResponse], error) {
	return nil, errAgentMetadataNoop
}

var _ = Describe("decorateMCPToolsFromExtensions", func() {
	var server *mcpserver.MCPServer

	BeforeEach(func() {
		server = mcpserver.NewMCPServer("test", "0", mcpserver.WithToolCapabilities(false))
		// Register the AgentMetadataService — the only currently-ported
		// service. Other services would surface unchanged proto comments and
		// are not exercised here.
		apiv1mcp.ForwardToConnectAgentMetadataServiceClient(server, &noopAgentMetadataServer{})
		decorateMCPToolsFromExtensions(server)
	})

	Describe("for a method with (api.v1.description) set", func() {
		var tool *mcpserver.ServerTool

		BeforeEach(func() {
			tool = server.GetTool("api_v1_AgentMetadataService_GetChatContext")
		})

		It("should be registered", func() {
			Expect(tool).NotTo(BeNil())
		})

		It("should override the description with the extension text", func() {
			Expect(tool.Tool.Description).To(ContainSubstring("Read the agent.chat_context for a page"))
		})

		It("should NOT contain the proto comment stub", func() {
			Expect(tool.Tool.Description).NotTo(ContainSubstring("see (api.v1.description)"))
		})
	})

	Describe("for a read-only method", func() {
		var tool *mcpserver.ServerTool

		BeforeEach(func() {
			tool = server.GetTool("api_v1_AgentMetadataService_ListSchedules")
		})

		It("should set the read-only annotation", func() {
			Expect(tool.Tool.Annotations.ReadOnlyHint).NotTo(BeNil())
			Expect(*tool.Tool.Annotations.ReadOnlyHint).To(BeTrue())
		})
	})

	Describe("for a method with (api.v1.example_request) set", func() {
		var tool *mcpserver.ServerTool

		BeforeEach(func() {
			tool = server.GetTool("api_v1_AgentMetadataService_UpsertSchedule")
		})

		It("should append the example to the description", func() {
			Expect(tool.Tool.Description).To(ContainSubstring("Example request:"))
		})

		It("should include the example payload", func() {
			Expect(tool.Tool.Description).To(ContainSubstring("friday_draft"))
		})
	})

	// computeMCPToolName has its own self-contained behavior that's worth a
	// minimal sanity check independent of the decorator wiring.
	Describe("computeMCPToolName via decorator output", func() {
		It("should produce names matching the api_v1_<Service>_<Method> convention", func() {
			Expect(server.GetTool("api_v1_AgentMetadataService_DeleteSchedule")).NotTo(BeNil())
		})
	})
})

// Suppress the unused-import warning for "strings" on builds where Ginkgo
// rearranges the test compilation.
var _ = strings.TrimSpace

// noopHandler is a no-op tool handler used only when registering throwaway
// tools in tests. The decorator preserves the existing handler when it
// re-registers a tool with new metadata; we never invoke it.
func noopHandler(_ context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return &mcpgo.CallToolResult{}, nil
}

// Suppress the unused-warning for noopHandler when no test happens to
// reference it directly. It documents the contract that tools must have a
// handler at registration.
var _ = noopHandler
