//revive:disable:dot-imports
package main

import (
	"context"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Regression guard for copilot review comment on #1055.
//
// Background: swapBrokenSchemasForAnthropic replaces a tool's RawInputSchema
// with the OpenAI-variant schema produced by the proto-mcp generator. That
// variant schema instructs the model to:
//   - Represent google.protobuf.Struct (and Value/ListValue) fields as
//     JSON-encoded strings.
//   - Represent proto maps as arrays of {key,value} objects.
//   - Mark optional scalars as type: ["string","null"] / ["integer","null"]
//     so the model emits explicit `null` for unused oneof members.
//
// The generator-emitted *HandlerOpenAI variants run runtime.FixOpenAI
// between request.GetArguments() and protojson.Unmarshal to translate the
// OpenAI shape back to the standard protojson shape before decoding. The
// default (non-OpenAI) ForwardToConnect* handlers do NOT call FixOpenAI, so
// if we swap only the schema and reuse the default handler unchanged, any
// future override tool whose request contains a Struct/Value/Map field will
// fail at protojson.Unmarshal with a protobuf syntax error.
//
// wrapHandlerForOpenAISchema closes this gap: it wraps the default handler
// with the same FixOpenAI step the generator emits in the OpenAI variant.
var _ = Describe("wrapHandlerForOpenAISchema", func() {
	When("called with an inner handler", func() {
		var innerCalled bool
		var capturedArgs map[string]any
		var inner mcpserver.ToolHandlerFunc

		BeforeEach(func() {
			innerCalled = false
			capturedArgs = nil
			inner = func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				innerCalled = true
				capturedArgs = req.GetArguments()
				return mcp.NewToolResultText("ok"), nil
			}
		})

		When("the inner handler's request type has a google.protobuf.Struct field and the model sends it as a JSON-encoded string (OpenAI shape)", func() {
			// MergeFrontmatterRequest is intentionally NOT one of the
			// currently-overridden tools. We use it here because it has a
			// google.protobuf.Struct field, which lets us pin down the
			// FixOpenAI normalization contract in a way the current
			// override tools (ReadPage / RemoveKeyAtPath) cannot — their
			// request types only contain scalars/oneofs, for which
			// FixOpenAI is a no-op. If the override set grows to include
			// a Struct/Value/Map-bearing request, this test is what
			// guarantees the existing wrapper will normalize it correctly
			// without further changes.
			var descriptor protoreflect.MessageDescriptor
			var wrapped mcpserver.ToolHandlerFunc
			var openAIShapedArgs map[string]any
			var req mcp.CallToolRequest
			var resultErr error

			BeforeEach(func() {
				descriptor = (&apiv1.MergeFrontmatterRequest{}).ProtoReflect().Descriptor()
				wrapped = wrapHandlerForOpenAISchema(inner, descriptor)

				openAIShapedArgs = map[string]any{
					"page":        "p",
					"frontmatter": `{"title":"hello"}`,
				}
				req = mcp.CallToolRequest{}
				req.Params.Name = "api_v1_Frontmatter_MergeFrontmatter"
				req.Params.Arguments = openAIShapedArgs

				_, resultErr = wrapped(context.Background(), req)
			})

			It("should not return an error", func() {
				Expect(resultErr).NotTo(HaveOccurred())
			})

			It("should invoke the inner handler", func() {
				Expect(innerCalled).To(BeTrue())
			})

			It("should normalize the JSON-stringified Struct into a decoded JSON object before delegating", func() {
				Expect(capturedArgs).NotTo(BeNil())
				Expect(capturedArgs).To(HaveKeyWithValue("frontmatter", map[string]any{
					"title": "hello",
				}))
			})
		})

		When("the model sends OpenAI-shaped args with null for an unused oneof scalar (the case copilot raised on #1055)", func() {
			// ReadPageRequest.page_identifier is the only oneof in this
			// override; under the OpenAI schema both members are marked
			// nullable, so a model selecting page_name will emit
			// identifier: null and vice-versa.
			var descriptor protoreflect.MessageDescriptor
			var wrapped mcpserver.ToolHandlerFunc
			var req mcp.CallToolRequest
			var resultErr error

			BeforeEach(func() {
				descriptor = (&apiv1.ReadPageRequest{}).ProtoReflect().Descriptor()
				wrapped = wrapHandlerForOpenAISchema(inner, descriptor)

				req = mcp.CallToolRequest{}
				req.Params.Name = "api_v1_PageManagementService_ReadPage"
				req.Params.Arguments = map[string]any{
					"page_name":  "foo",
					"identifier": nil,
				}

				_, resultErr = wrapped(context.Background(), req)
			})

			It("should not return an error", func() {
				Expect(resultErr).NotTo(HaveOccurred())
			})

			It("should invoke the inner handler", func() {
				Expect(innerCalled).To(BeTrue())
			})

			It("should preserve the active oneof member", func() {
				Expect(capturedArgs).To(HaveKeyWithValue("page_name", "foo"))
			})
		})

		When("the model sends the happy path (no nulls, no stringified Structs)", func() {
			var descriptor protoreflect.MessageDescriptor
			var wrapped mcpserver.ToolHandlerFunc
			var req mcp.CallToolRequest
			var resultErr error

			BeforeEach(func() {
				descriptor = (&apiv1.ReadPageRequest{}).ProtoReflect().Descriptor()
				wrapped = wrapHandlerForOpenAISchema(inner, descriptor)

				req = mcp.CallToolRequest{}
				req.Params.Name = "api_v1_PageManagementService_ReadPage"
				req.Params.Arguments = map[string]any{
					"page_name": "foo",
				}

				_, resultErr = wrapped(context.Background(), req)
			})

			It("should not return an error", func() {
				Expect(resultErr).NotTo(HaveOccurred())
			})

			It("should pass the active oneof member through unchanged", func() {
				Expect(capturedArgs).To(HaveKeyWithValue("page_name", "foo"))
			})

			It("should not invent extra keys", func() {
				Expect(capturedArgs).NotTo(HaveKey("identifier"))
			})
		})
	})
})
