//revive:disable:dot-imports
package main

import (
	"errors"
	"net/http"
	"strings"

	mcpserver "github.com/mark3labs/mcp-go/server"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var errNotImplementedForRegression = errors.New("noopHTTPClient: not used in regression test")

// Phase 1E regression guard.
//
// Goal: when a service is ported from proto comments to (api.v1.description)
// extensions, no documentation goes silently missing. Concretely:
//
//   - Every registered MCP tool MUST have a non-empty Description after the
//     decorator runs.
//   - No tool's Description may end up as the placeholder "see (api.v1.description)"
//     stub — that means the proto comment was reduced to a stub but the
//     extension was not wired up correctly.
//
// This guard does not require maintaining a hand-curated golden snapshot of
// per-RPC text; it just catches the failure mode the snapshot was meant to
// catch: lost or stub-only descriptions after a port.
var _ = Describe("MCP extension regression (Phase 1E.0)", func() {
	var server *mcpserver.MCPServer

	BeforeEach(func() {
		server = mcpserver.NewMCPServer("regression", version, mcpserver.WithToolCapabilities(false))
		// Register every MCP-exposed service against no-op clients. We don't
		// invoke handlers; the test only inspects the descriptions baked into
		// the registered tools.
		apiClients := createAPIClients(noopHTTPClient{}, "http://example.invalid")
		registerToolHandlers(server, apiClients)
	})

	Describe("after registerToolHandlers (which runs the decorator)", func() {
		var tools map[string]*mcpserver.ServerTool

		BeforeEach(func() {
			tools = server.ListTools()
		})

		It("should register at least the AgentMetadataService tools", func() {
			Expect(tools).To(HaveKey("api_v1_AgentMetadataService_GetChatContext"))
		})

		It("should register the ChecklistService tools", func() {
			Expect(tools).To(HaveKey("api_v1_ChecklistService_AddItem"))
			Expect(tools).To(HaveKey("api_v1_ChecklistService_ToggleItem"))
			Expect(tools).To(HaveKey("api_v1_ChecklistService_ListItems"))
		})

		It("should leave no tool with an empty description", func() {
			for name, tool := range tools {
				Expect(strings.TrimSpace(tool.Tool.Description)).
					NotTo(BeEmpty(), "tool %q has an empty description", name)
			}
		})

		It("should leave no tool surfacing the proto-comment stub", func() {
			for name, tool := range tools {
				Expect(tool.Tool.Description).NotTo(
					ContainSubstring("see (api.v1.description)"),
					"tool %q surfaces the proto-comment stub instead of the extension content; the (api.v1.description) extension is missing or misnamed",
					name,
				)
			}
		})

		It("should leave no service surfacing the proto-comment stub for service description", func() {
			// Belt-and-suspenders: the same stub pattern at service level.
			for name, tool := range tools {
				Expect(tool.Tool.Description).NotTo(
					ContainSubstring("see (api.v1.service_description)"),
					"tool %q surfaces a service_description stub", name,
				)
			}
		})
	})
})

// noopHTTPClient is a minimal connect.HTTPClient that satisfies the interface
// for createAPIClients but does not actually make any HTTP calls. Tests that
// only inspect tool metadata never invoke the handlers, so no real HTTP is
// needed.
type noopHTTPClient struct{}

func (noopHTTPClient) Do(*http.Request) (*http.Response, error) {
	return nil, errNotImplementedForRegression
}
