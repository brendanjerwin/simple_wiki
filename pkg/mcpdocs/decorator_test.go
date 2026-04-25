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

// noopFileStorageServer satisfies apiv1mcp.ConnectFileStorageServiceClient.
// FileStorageService methods declare none of the MCP extensions, so it
// exercises the methodHasMCPOverrides short-circuit branch in the decorator.
type noopFileStorageServer struct{}

var errFileStorageNoop = errors.New("noopFileStorageServer: not used in this test")

func (*noopFileStorageServer) DeleteFile(_ context.Context, _ *connect.Request[apiv1.DeleteFileRequest]) (*connect.Response[apiv1.DeleteFileResponse], error) {
	return nil, errFileStorageNoop
}

func (*noopFileStorageServer) GetFileInfo(_ context.Context, _ *connect.Request[apiv1.GetFileInfoRequest]) (*connect.Response[apiv1.GetFileInfoResponse], error) {
	return nil, errFileStorageNoop
}

func (*noopFileStorageServer) UploadFile(_ context.Context, _ *connect.Request[apiv1.UploadFileRequest]) (*connect.Response[apiv1.UploadFileResponse], error) {
	return nil, errFileStorageNoop
}

// noopPageImportServer satisfies apiv1mcp.ConnectPageImportServiceClient.
// PageImportService.StartPageImportJob declares (api.v1.long_running) = true
// and exercises the long-running annotation branch in applyAnnotations.
type noopPageImportServer struct{}

var errPageImportNoop = errors.New("noopPageImportServer: not used in this test")

func (*noopPageImportServer) ParseCSVPreview(_ context.Context, _ *connect.Request[apiv1.ParseCSVPreviewRequest]) (*connect.Response[apiv1.ParseCSVPreviewResponse], error) {
	return nil, errPageImportNoop
}

func (*noopPageImportServer) StartPageImportJob(_ context.Context, _ *connect.Request[apiv1.StartPageImportJobRequest]) (*connect.Response[apiv1.StartPageImportJobResponse], error) {
	return nil, errPageImportNoop
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

	Describe("when Decorate returns ServiceDescriptions", func() {
		It("should be non-empty", func() {
			Expect(serviceDescriptions).NotTo(BeEmpty())
		})

		It("should include the api.v1.AgentMetadataService entry", func() {
			services := []string{}
			for _, sd := range serviceDescriptions {
				services = append(services, sd.Service)
			}
			Expect(services).To(ContainElement("api.v1.AgentMetadataService"))
		})
	})
})

var _ = Describe("Decorate with no MCP tools registered", func() {
	var (
		emptyServer         *mcpserver.MCPServer
		serviceDescriptions []mcpdocs.ServiceDescription
		decorateInvocation  func()
	)

	BeforeEach(func() {
		emptyServer = mcpserver.NewMCPServer("empty", "0", mcpserver.WithToolCapabilities(false))
		decorateInvocation = func() {
			serviceDescriptions = mcpdocs.Decorate(emptyServer)
		}
	})

	It("should not panic", func() {
		Expect(decorateInvocation).NotTo(Panic())
	})

	Describe("when Decorate completes", func() {
		BeforeEach(func() {
			decorateInvocation()
		})

		It("should still return service-level descriptions for services with (api.v1.service_description)", func() {
			// AgentMetadataService declares service_description even though no
			// MCP tools were registered on this server.
			services := []string{}
			for _, sd := range serviceDescriptions {
				services = append(services, sd.Service)
			}
			Expect(services).To(ContainElement("api.v1.AgentMetadataService"))
		})
	})
})

var _ = Describe("Decorate for a service whose methods declare no MCP extensions", func() {
	var (
		server *mcpserver.MCPServer
		tool   *mcpserver.ServerTool
	)

	BeforeEach(func() {
		server = mcpserver.NewMCPServer("test", "0", mcpserver.WithToolCapabilities(false))
		apiv1mcp.ForwardToConnectFileStorageServiceClient(server, &noopFileStorageServer{})
		mcpdocs.Decorate(server)
		tool = server.GetTool("api_v1_FileStorageService_DeleteFile")
	})

	It("should still register the tool from the codegen plugin", func() {
		Expect(tool).NotTo(BeNil())
	})

	It("should preserve the codegen description verbatim (no override)", func() {
		Expect(tool.Tool.Description).To(Equal("DeleteFile removes an uploaded file.\n"))
	})

	It("should not set the read-only annotation", func() {
		Expect(tool.Tool.Annotations.ReadOnlyHint).To(BeNil())
	})

	It("should not set the idempotent annotation", func() {
		Expect(tool.Tool.Annotations.IdempotentHint).To(BeNil())
	})

	It("should not set the open-world annotation", func() {
		Expect(tool.Tool.Annotations.OpenWorldHint).To(BeNil())
	})
})

var _ = Describe("Decorate for a method declaring (api.v1.long_running)", func() {
	var (
		server *mcpserver.MCPServer
		tool   *mcpserver.ServerTool
	)

	BeforeEach(func() {
		server = mcpserver.NewMCPServer("test", "0", mcpserver.WithToolCapabilities(false))
		apiv1mcp.ForwardToConnectPageImportServiceClient(server, &noopPageImportServer{})
		mcpdocs.Decorate(server)
		tool = server.GetTool("api_v1_PageImportService_StartPageImportJob")
	})

	It("should register the tool", func() {
		Expect(tool).NotTo(BeNil())
	})

	It("should set IdempotentHint to false", func() {
		Expect(tool.Tool.Annotations.IdempotentHint).NotTo(BeNil())
		Expect(*tool.Tool.Annotations.IdempotentHint).To(BeFalse())
	})

	It("should set OpenWorldHint to true", func() {
		Expect(tool.Tool.Annotations.OpenWorldHint).NotTo(BeNil())
		Expect(*tool.Tool.Annotations.OpenWorldHint).To(BeTrue())
	})
})

// noopChatServer satisfies apiv1mcp.ConnectChatServiceClient. ChatService
// declares neither (api.v1.service_description) nor any method-level MCP
// extensions, so it exercises the "no overrides" branches in
// readServiceDescription, readBoolExtension, and readStringExtension via the
// real registered service.
type noopChatServer struct{}

var errChatNoop = errors.New("noopChatServer: not used in this test")

func (*noopChatServer) CancelAgentPrompt(_ context.Context, _ *connect.Request[apiv1.CancelAgentPromptRequest]) (*connect.Response[apiv1.CancelAgentPromptResponse], error) {
	return nil, errChatNoop
}

func (*noopChatServer) EditChatMessage(_ context.Context, _ *connect.Request[apiv1.EditChatMessageRequest]) (*connect.Response[apiv1.EditChatMessageResponse], error) {
	return nil, errChatNoop
}

func (*noopChatServer) GetChatStatus(_ context.Context, _ *connect.Request[apiv1.GetChatStatusRequest]) (*connect.Response[apiv1.GetChatStatusResponse], error) {
	return nil, errChatNoop
}

func (*noopChatServer) ReactToMessage(_ context.Context, _ *connect.Request[apiv1.ReactToMessageRequest]) (*connect.Response[apiv1.ReactToMessageResponse], error) {
	return nil, errChatNoop
}

func (*noopChatServer) RequestPermissionFromUser(_ context.Context, _ *connect.Request[apiv1.RequestPermissionFromUserRequest]) (*connect.Response[apiv1.RequestPermissionFromUserResponse], error) {
	return nil, errChatNoop
}

func (*noopChatServer) RespondToPermission(_ context.Context, _ *connect.Request[apiv1.RespondToPermissionRequest]) (*connect.Response[apiv1.RespondToPermissionResponse], error) {
	return nil, errChatNoop
}

func (*noopChatServer) SendChatReply(_ context.Context, _ *connect.Request[apiv1.SendChatReplyRequest]) (*connect.Response[apiv1.SendChatReplyResponse], error) {
	return nil, errChatNoop
}

func (*noopChatServer) SendMessage(_ context.Context, _ *connect.Request[apiv1.SendChatMessageRequest]) (*connect.Response[apiv1.SendChatMessageResponse], error) {
	return nil, errChatNoop
}

func (*noopChatServer) SendToolCallNotification(_ context.Context, _ *connect.Request[apiv1.SendToolCallNotificationRequest]) (*connect.Response[apiv1.SendToolCallNotificationResponse], error) {
	return nil, errChatNoop
}

var _ = Describe("Decorate for a service that declares no service_description", func() {
	var (
		server              *mcpserver.MCPServer
		serviceDescriptions []mcpdocs.ServiceDescription
	)

	BeforeEach(func() {
		server = mcpserver.NewMCPServer("test", "0", mcpserver.WithToolCapabilities(false))
		apiv1mcp.ForwardToConnectChatServiceClient(server, &noopChatServer{})
		serviceDescriptions = mcpdocs.Decorate(server)
	})

	It("should not include the chat service in service descriptions", func() {
		services := []string{}
		for _, sd := range serviceDescriptions {
			services = append(services, sd.Service)
		}
		Expect(services).NotTo(ContainElement("api.v1.ChatService"))
	})

	It("should still register the chat tool from the codegen plugin", func() {
		Expect(server.GetTool("api_v1_ChatService_GetChatStatus")).NotTo(BeNil())
	})

	It("should preserve the codegen description verbatim for chat tools (no override)", func() {
		tool := server.GetTool("api_v1_ChatService_GetChatStatus")
		Expect(tool.Tool.Description).To(ContainSubstring("GetChatStatus"))
	})

	It("should not set the read-only annotation on chat tools", func() {
		tool := server.GetTool("api_v1_ChatService_GetChatStatus")
		Expect(tool.Tool.Annotations.ReadOnlyHint).To(BeNil())
	})
})

var _ = Describe("Decorate when a service declares extensions but tools are not registered", func() {
	var (
		server *mcpserver.MCPServer
		tool   *mcpserver.ServerTool
	)

	BeforeEach(func() {
		// Construct a fresh server WITHOUT calling any ForwardToConnect*
		// functions. The decorator walks the global proto registry and finds
		// services declaring extensions (e.g. AgentMetadataService), but
		// s.GetTool returns nil for each one — exercising the tool == nil
		// short-circuit branch in decorateFileServices.
		server = mcpserver.NewMCPServer("test", "0", mcpserver.WithToolCapabilities(false))
		mcpdocs.Decorate(server)
		tool = server.GetTool("api_v1_AgentMetadataService_GetChatContext")
	})

	It("should silently skip the unregistered tool", func() {
		Expect(tool).To(BeNil())
	})
})
