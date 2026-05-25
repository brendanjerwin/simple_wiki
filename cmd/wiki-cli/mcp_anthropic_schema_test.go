//revive:disable:dot-imports
package main

import (
	"encoding/json"

	mcpserver "github.com/mark3labs/mcp-go/server"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Regression guard for #1054.
//
// Background: Anthropic's tool registration API rejects any tool whose
// JSON Schema contains "anyOf", "allOf", or "oneOf" at the top level
// (the API responds with HTTP 400 "input_schema does not support oneOf,
// allOf, or anyOf at the top level"). The protoc-gen-go-mcp generator
// emits this exact broken shape for any request message that contains a
// proto `oneof` block (e.g. ReadPageRequest.page_identifier introduced
// by #1029). The fix is to register the *ToolOpenAI variant for those
// tools — it emits a flat nullable schema Anthropic accepts.
//
// This guard fails if any wiki-cli-registered MCP tool exposes a
// top-level anyOf/allOf/oneOf in its input schema. Every non-interactive
// `claude --print` call that pulls the wiki MCP tool catalog (scheduled
// agents, daily reflections, etc.) depends on this invariant holding.
var _ = Describe("MCP tool input schemas (Anthropic API compatibility, #1054)", func() {
	var tools map[string]*mcpserver.ServerTool

	BeforeEach(func() {
		server := mcpserver.NewMCPServer("anthropic-compat", version, mcpserver.WithToolCapabilities(false))
		apiClients := createAPIClients(noopHTTPClient{}, "http://example.invalid")
		registerToolHandlers(server, apiClients)
		tools = server.ListTools()
	})

	It("should register at least one tool (sanity check)", func() {
		Expect(tools).NotTo(BeEmpty())
	})

	It("should not expose any tool with top-level anyOf/allOf/oneOf in inputSchema", func() {
		for name, tool := range tools {
			rawSchema := tool.Tool.RawInputSchema
			if len(rawSchema) == 0 {
				continue
			}

			var schema map[string]any
			err := json.Unmarshal(rawSchema, &schema)
			Expect(err).NotTo(HaveOccurred(), "tool %q has unparseable RawInputSchema", name)

			for _, forbidden := range []string{"anyOf", "allOf", "oneOf"} {
				Expect(schema).NotTo(HaveKey(forbidden),
					"tool %q exposes top-level %q in its inputSchema; "+
						"Anthropic's API will reject this with HTTP 400. "+
						"Switch the registration to the *ToolOpenAI variant for this tool "+
						"(see cmd/wiki-cli/mcp.go and #1054).",
					name, forbidden)
			}
		}
	})

	When("inspecting the ReadPage tool schema", func() {
		var schema map[string]any

		BeforeEach(func() {
			tool := tools["api_v1_PageManagementService_ReadPage"]
			Expect(tool).NotTo(BeNil())

			err := json.Unmarshal(tool.Tool.RawInputSchema, &schema)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not require both oneof members", func() {
			Expect(schema).NotTo(HaveKeyWithValue("required", ConsistOf("page_name", "identifier")))
		})

		It("should require at least one identifier property", func() {
			Expect(schema).To(HaveKeyWithValue("minProperties", BeNumerically("==", 1)))
		})

		It("should allow at most one identifier property", func() {
			Expect(schema).To(HaveKeyWithValue("maxProperties", BeNumerically("==", 1)))
		})
	})

	When("inspecting the ReadPageSection tool schema", func() {
		var schema map[string]any

		BeforeEach(func() {
			tool := tools["api_v1_PageManagementService_ReadPageSection"]
			Expect(tool).NotTo(BeNil())

			err := json.Unmarshal(tool.Tool.RawInputSchema, &schema)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should require the range inputs", func() {
			Expect(schema).To(HaveKeyWithValue("required", ConsistOf("page_name", "byte_offset", "byte_length")))
		})

		It("should not require the version hash", func() {
			Expect(schema).NotTo(HaveKeyWithValue("required", ContainElement("expected_version_hash")))
		})
	})
})
