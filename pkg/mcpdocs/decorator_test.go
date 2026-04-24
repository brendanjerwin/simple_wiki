//revive:disable:dot-imports
package mcpdocs_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	mcpserver "github.com/mark3labs/mcp-go/server"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1mcp"
	"github.com/brendanjerwin/simple_wiki/pkg/mcpdocs"
)

func TestMCPDocsDecorator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP Docs Decorator Suite")
}

// noopAgentMetadataServer satisfies apiv1mcp.ConnectAgentMetadataServiceClient
// just enough to register tools — handlers are never actually called by these
// tests; the decorator only inspects MCP tool metadata.
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

var _ = Describe("Decorate", func() {
	var (
		server               *mcpserver.MCPServer
		serviceDescriptions  []mcpdocs.ServiceDescription
	)

	BeforeEach(func() {
		server = mcpserver.NewMCPServer("test", "0", mcpserver.WithToolCapabilities(false))
		apiv1mcp.ForwardToConnectAgentMetadataServiceClient(server, &noopAgentMetadataServer{})
		serviceDescriptions = mcpdocs.Decorate(server)
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

	Describe("service-level descriptions", func() {
		It("should include the AgentMetadataService description", func() {
			found := false
			for _, sd := range serviceDescriptions {
				if sd.Service == "api.v1.AgentMetadataService" {
					found = true
					Expect(sd.Description).To(ContainSubstring("agent-managed page state"))
				}
			}
			Expect(found).To(BeTrue(), "expected api.v1.AgentMetadataService in service descriptions")
		})
	})

	Describe("tool name convention", func() {
		It("should produce names matching api_v1_<Service>_<Method>", func() {
			Expect(server.GetTool("api_v1_AgentMetadataService_DeleteSchedule")).NotTo(BeNil())
		})
	})
})
