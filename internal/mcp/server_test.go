//revive:disable:dot-imports
package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	grpcapi "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	wikimcp "github.com/brendanjerwin/simple_wiki/internal/mcp"
	"github.com/brendanjerwin/simple_wiki/pkg/chatbuffer"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/index/bleve"
)

// noOpPageReaderMutator satisfies wikipage.PageReaderMutator for tests.
type noOpPageReaderMutator struct{}

func (noOpPageReaderMutator) ReadFrontMatter(wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	return "", nil, os.ErrNotExist
}
func (noOpPageReaderMutator) WriteFrontMatter(wikipage.PageIdentifier, wikipage.FrontMatter) error {
	return nil
}
func (noOpPageReaderMutator) ReadMarkdown(wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return "", "", os.ErrNotExist
}
func (noOpPageReaderMutator) WriteMarkdown(wikipage.PageIdentifier, wikipage.Markdown) error {
	return nil
}
func (noOpPageReaderMutator) DeletePage(wikipage.PageIdentifier) error { return nil }
func (noOpPageReaderMutator) ModifyMarkdown(_ wikipage.PageIdentifier, modifier func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	_, err := modifier("")
	return err
}

// noOpPageOpener satisfies wikipage.PageOpener for tests.
type noOpPageOpener struct{}

func (noOpPageOpener) ReadPage(wikipage.PageIdentifier) (*wikipage.Page, error) {
	return &wikipage.Page{}, nil
}

// noOpBleveIndexQueryer satisfies bleve.BleveIndexQueryer for tests.
type noOpBleveIndexQueryer struct{}

func (noOpBleveIndexQueryer) Query(string) ([]bleve.SearchResult, error) { return nil, nil }
func (noOpBleveIndexQueryer) QueryWithTags(string, []string, []string) ([]bleve.SearchResult, error) {
	return nil, nil
}

// noOpFrontmatterIndexQueryer satisfies wikipage.IQueryFrontmatterIndex for tests.
type noOpFrontmatterIndexQueryer struct{}

func (noOpFrontmatterIndexQueryer) QueryExactMatch(wikipage.DottedKeyPath, wikipage.Value) []wikipage.PageIdentifier {
	return nil
}
func (noOpFrontmatterIndexQueryer) QueryKeyExistence(wikipage.DottedKeyPath) []wikipage.PageIdentifier {
	return nil
}
func (noOpFrontmatterIndexQueryer) QueryPrefixMatch(wikipage.DottedKeyPath, string) []wikipage.PageIdentifier {
	return nil
}
func (noOpFrontmatterIndexQueryer) GetValue(wikipage.PageIdentifier, wikipage.DottedKeyPath) wikipage.Value {
	return ""
}
func (noOpFrontmatterIndexQueryer) QueryExactMatchSortedBy(wikipage.DottedKeyPath, wikipage.Value, wikipage.DottedKeyPath, bool, int) []wikipage.PageIdentifier {
	return nil
}

// noOpChatBufferManager satisfies grpcapi.ChatBufferManager for tests.
type noOpChatBufferManager struct{}

// noopUnsubscribe is a no-op unsubscribe function returned by mock subscription methods.
func noopUnsubscribe() {
	// No-op — intentionally empty test stub.
}

func (noOpChatBufferManager) AddUserMessage(string, string, string) (string, error) {
	return "", nil
}
func (noOpChatBufferManager) AddAssistantMessage(string, string, string) (string, error) {
	return "", nil
}
func (noOpChatBufferManager) EditMessage(string, string, bool) error {
	return nil
}
func (noOpChatBufferManager) AddReaction(string, string, string) error {
	return nil
}
func (noOpChatBufferManager) GetMessages(string) []*chatbuffer.Message {
	return nil
}
func (noOpChatBufferManager) SubscribeToPage(string) (<-chan chatbuffer.Event, func()) {
	ch := make(chan chatbuffer.Event)
	close(ch)
	return ch, noopUnsubscribe
}
func (noOpChatBufferManager) SubscribeToPageWithReplay(string) ([]*chatbuffer.Message, <-chan chatbuffer.Event, func()) {
	ch := make(chan chatbuffer.Event)
	close(ch)
	return nil, ch, noopUnsubscribe
}
func (noOpChatBufferManager) SubscribeToPageChannelWithReplay(string) ([]*chatbuffer.Message, <-chan *chatbuffer.Message, func()) {
	ch := make(chan *chatbuffer.Message)
	close(ch)
	return nil, ch, func() {
		// no-op: nothing to unsubscribe from a closed channel
	}
}

func (noOpChatBufferManager) SubscribeToPageChannel(string) (<-chan *chatbuffer.Message, func()) {
	ch := make(chan *chatbuffer.Message)
	close(ch)
	return ch, func() {
		// no-op: nothing to unsubscribe from a closed channel
	}
}

func (noOpChatBufferManager) HasPageChannelSubscriber(string) bool {
	return false
}

func (noOpChatBufferManager) RequestInstance(string) {
	// no-op: satisfies interface; this implementation ignores instance requests
}

func (noOpChatBufferManager) SubscribeToInstanceRequests() (<-chan string, func()) {
	ch := make(chan string)
	close(ch)
	return ch, func() {
		// no-op: nothing to unsubscribe from a closed channel
	}
}

func (noOpChatBufferManager) HasInstanceRequestSubscribers() bool {
	return false
}

func (noOpChatBufferManager) IsInstanceRequested(string) bool {
	return false
}

func (noOpChatBufferManager) NotifyToolCall(string, string, string, string, string) {
	// no-op: satisfies interface; this implementation ignores tool call notifications
}

func (noOpChatBufferManager) CancelPage(string) bool {
	return false
}

func (noOpChatBufferManager) SubscribeToCancellation(string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	return ch, func() {
		// no-op: nothing to unsubscribe from this mock cancellation channel
	}
}

func (noOpChatBufferManager) EmitPermissionRequest(string, *chatbuffer.PermissionRequestEvent) {
	// no-op: satisfies interface; this implementation ignores permission request emissions
}

func (noOpChatBufferManager) RespondToPermission(string, string) {
	// no-op: satisfies interface; this implementation ignores permission responses
}

func mustNewAPIServer() *grpcapi.Server {
	srv, err := grpcapi.NewServer(
		grpcapi.BuildInfo{Commit: "test-commit", BuildTime: time.Now()},
		noOpPageReaderMutator{},
		noOpBleveIndexQueryer{},
		noOpFrontmatterIndexQueryer{},
		lumber.NewConsoleLogger(lumber.WARN),
		noOpChatBufferManager{},
		noOpPageOpener{},
	)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return srv
}

var _ = Describe("NewStreamableHTTPHandler", func() {
	var apiServer *grpcapi.Server

	BeforeEach(func() {
		apiServer = mustNewAPIServer()
	})

	It("returns a non-nil handler without error", func() {
		handler, err := wikimcp.NewStreamableHTTPHandler(apiServer, "test-version")

		Expect(err).NotTo(HaveOccurred())
		Expect(handler).NotTo(BeNil())
	})

	Describe("HTTP handler behaviour", func() {
		var handler http.Handler

		BeforeEach(func() {
			var err error
			handler, err = wikimcp.NewStreamableHTTPHandler(apiServer, "test-version")
			Expect(err).NotTo(HaveOccurred())
		})

		When("receiving an MCP initialize request", func() {
			var resp *httptest.ResponseRecorder

			BeforeEach(func() {
				body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}}`
				req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				resp = httptest.NewRecorder()
				handler.ServeHTTP(resp, req)
			})

			It("returns HTTP 200", func() {
				Expect(resp.Code).To(Equal(http.StatusOK))
			})

			It("returns a JSON-RPC response with serverInfo", func() {
				var result map[string]any
				Expect(json.Unmarshal(resp.Body.Bytes(), &result)).To(Succeed())
				Expect(result).To(HaveKey("result"))
				resultMap, ok := result["result"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(resultMap).To(HaveKey("serverInfo"))
			})
		})

		When("invoking the api_v1_Frontmatter_ReplaceFrontmatter tool with frontmatter as a JSON object", func() {
			var callResp *httptest.ResponseRecorder
			var callResult map[string]any

			BeforeEach(func() {
				// Initialize to get a session
				initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}}`
				initReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(initBody))
				initReq.Header.Set("Content-Type", "application/json")
				initResp := httptest.NewRecorder()
				handler.ServeHTTP(initResp, initReq)
				sessionID := initResp.Header().Get("Mcp-Session-Id")

				// Call the tool with frontmatter as a JSON object
				callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"api_v1_Frontmatter_ReplaceFrontmatter","arguments":{"page":"test-page","frontmatter":{"title":"Test Page","tags":["alpha","beta"]}}}}`
				callReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(callBody))
				callReq.Header.Set("Content-Type", "application/json")
				if sessionID != "" {
					callReq.Header.Set("Mcp-Session-Id", sessionID)
				}
				callResp = httptest.NewRecorder()
				handler.ServeHTTP(callResp, callReq)

				Expect(json.Unmarshal(callResp.Body.Bytes(), &callResult)).To(Succeed())
			})

			It("returns HTTP 200", func() {
				Expect(callResp.Code).To(Equal(http.StatusOK))
			})

			It("returns a JSON-RPC result without error", func() {
				Expect(callResult).NotTo(HaveKey("error"), "unexpected JSON-RPC error: %v", callResult["error"])
				Expect(callResult).To(HaveKey("result"))
			})

			It("returns a non-error tool result", func() {
				resultMap, ok := callResult["result"].(map[string]any)
				Expect(ok).To(BeTrue(), "result should be a map, got: %T", callResult["result"])
				Expect(resultMap["isError"]).NotTo(Equal(true), "tool result isError should be false, content: %v", resultMap["content"])
			})
		})

		// The server now uses the standard handler, which advertises frontmatter as
		// "type": "object". String-encoded frontmatter is no longer accepted — clients
		// that send a JSON-encoded string instead of an object will receive a JSON-RPC error.
		// This intentional change eliminates the fragile FixOpenAI conversion step that
		// silently failed with complex/large frontmatter (e.g. ai_agent_chat_context),
		// causing intermittent "proto: syntax error" failures (issue #956).
		When("invoking the api_v1_Frontmatter_ReplaceFrontmatter tool with frontmatter as a JSON-encoded string", func() {
			var callResp *httptest.ResponseRecorder
			var callResult map[string]any

			BeforeEach(func() {
				// Initialize to get a session
				initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}}`
				initReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(initBody))
				initReq.Header.Set("Content-Type", "application/json")
				initResp := httptest.NewRecorder()
				handler.ServeHTTP(initResp, initReq)
				sessionID := initResp.Header().Get("Mcp-Session-Id")

				// Send frontmatter as a JSON-encoded string. The standard handler does not
				// apply FixOpenAI, so this results in a JSON-RPC error (not a tool success).
				callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"api_v1_Frontmatter_ReplaceFrontmatter","arguments":{"page":"test-page","frontmatter":"{\"title\":\"Test Page\",\"tags\":[\"alpha\",\"beta\"]}"}}}`
				callReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(callBody))
				callReq.Header.Set("Content-Type", "application/json")
				if sessionID != "" {
					callReq.Header.Set("Mcp-Session-Id", sessionID)
				}
				callResp = httptest.NewRecorder()
				handler.ServeHTTP(callResp, callReq)

				Expect(json.Unmarshal(callResp.Body.Bytes(), &callResult)).To(Succeed())
			})

			It("returns HTTP 200", func() {
				Expect(callResp.Code).To(Equal(http.StatusOK))
			})

			It("returns a JSON-RPC error because the standard handler requires objects not strings", func() {
				Expect(callResult).To(HaveKey("error"), "expected a JSON-RPC error for string-encoded frontmatter")
			})
		})

		// Regression test for issue #956: ReplaceFrontmatter intermittently failed with
		// "proto: syntax error" when the frontmatter contained deeply nested structures
		// (e.g. ai_agent_chat_context written by the wiki-chat daemon). The fix switches
		// from the OpenAI-compatible handler (which required frontmatter as a JSON-encoded
		// string) to the standard handler (which accepts frontmatter as a JSON object),
		// eliminating the fragile FixOpenAI string-decoding step.
		When("invoking the api_v1_Frontmatter_ReplaceFrontmatter tool with deeply nested frontmatter as a JSON object", func() {
			var callResp *httptest.ResponseRecorder
			var callResult map[string]any

			BeforeEach(func() {
				// Initialize to get a session
				initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}}`
				initReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(initBody))
				initReq.Header.Set("Content-Type", "application/json")
				initResp := httptest.NewRecorder()
				handler.ServeHTTP(initResp, initReq)
				sessionID := initResp.Header().Get("Mcp-Session-Id")

				// Send deeply nested frontmatter as a JSON object, mirroring the
				// ai_agent_chat_context structure that triggered issue #956.
				callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"api_v1_Frontmatter_ReplaceFrontmatter","arguments":{"page":"ai_assistant","frontmatter":{"title":"AI Assistant","ai_agent_chat_context":{"session_id":"abc123","messages":[{"role":"user","content":"Hello, can you help me?"},{"role":"assistant","content":"Sure! What do you need?"}],"metadata":{"model":"claude-sonnet","temperature":0.7,"nested":{"deep":{"deeper":{"deepest":"value with \"quotes\" and special chars"}}}}}}}}}`
				callReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(callBody))
				callReq.Header.Set("Content-Type", "application/json")
				if sessionID != "" {
					callReq.Header.Set("Mcp-Session-Id", sessionID)
				}
				callResp = httptest.NewRecorder()
				handler.ServeHTTP(callResp, callReq)

				Expect(json.Unmarshal(callResp.Body.Bytes(), &callResult)).To(Succeed())
			})

			It("returns HTTP 200", func() {
				Expect(callResp.Code).To(Equal(http.StatusOK))
			})

			It("returns a JSON-RPC result without proto syntax error", func() {
				Expect(callResult).NotTo(HaveKey("error"), "unexpected JSON-RPC error (proto syntax error?): %v", callResult["error"])
				Expect(callResult).To(HaveKey("result"))
			})

			It("returns a non-error tool result", func() {
				resultMap, ok := callResult["result"].(map[string]any)
				Expect(ok).To(BeTrue(), "result should be a map, got: %T", callResult["result"])
				Expect(resultMap["isError"]).NotTo(Equal(true), "tool result isError should be false, content: %v", resultMap["content"])
			})
		})

		When("invoking the api_v1_Frontmatter_MergeFrontmatter tool", func() {
			var callResp *httptest.ResponseRecorder
			var callResult map[string]any

			BeforeEach(func() {
				// Initialize to get a session
				initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}}`
				initReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(initBody))
				initReq.Header.Set("Content-Type", "application/json")
				initResp := httptest.NewRecorder()
				handler.ServeHTTP(initResp, initReq)
				sessionID := initResp.Header().Get("Mcp-Session-Id")

				// Call the tool
				callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"api_v1_Frontmatter_MergeFrontmatter","arguments":{"page":"test-page","frontmatter":{"title":"Test Page","tags":["alpha","beta"]}}}}`
				callReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(callBody))
				callReq.Header.Set("Content-Type", "application/json")
				if sessionID != "" {
					callReq.Header.Set("Mcp-Session-Id", sessionID)
				}
				callResp = httptest.NewRecorder()
				handler.ServeHTTP(callResp, callReq)

				Expect(json.Unmarshal(callResp.Body.Bytes(), &callResult)).To(Succeed())
			})

			It("returns HTTP 200", func() {
				Expect(callResp.Code).To(Equal(http.StatusOK))
			})

			It("returns a JSON-RPC result without error", func() {
				Expect(callResult).NotTo(HaveKey("error"), "unexpected JSON-RPC error: %v", callResult["error"])
				Expect(callResult).To(HaveKey("result"))
			})

			It("returns a non-error tool result", func() {
				resultMap, ok := callResult["result"].(map[string]any)
				Expect(ok).To(BeTrue(), "result should be a map, got: %T", callResult["result"])
				Expect(resultMap["isError"]).NotTo(Equal(true), "tool result isError should be false, content: %v", resultMap["content"])
			})
		})

		When("receiving an MCP tools/list request", func() {
			var toolNames []string

			BeforeEach(func() {
				// First initialize to get a session
				initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}}`
				initReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(initBody))
				initReq.Header.Set("Content-Type", "application/json")
				initResp := httptest.NewRecorder()
				handler.ServeHTTP(initResp, initReq)
				sessionID := initResp.Header().Get("Mcp-Session-Id")

				// Send tools/list
				listBody := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
				listReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(listBody))
				listReq.Header.Set("Content-Type", "application/json")
				if sessionID != "" {
					listReq.Header.Set("Mcp-Session-Id", sessionID)
				}
				listResp := httptest.NewRecorder()
				handler.ServeHTTP(listResp, listReq)

				var result map[string]any
				Expect(json.Unmarshal(listResp.Body.Bytes(), &result)).To(Succeed())
				resultMap, ok := result["result"].(map[string]any)
				Expect(ok).To(BeTrue(), "expected result to be a map")
				toolList, ok := resultMap["tools"].([]any)
				Expect(ok).To(BeTrue(), "expected tools to be a list")
				for _, t := range toolList {
					toolMap, ok := t.(map[string]any)
					Expect(ok).To(BeTrue(), "expected tool to be a map")
					name, ok := toolMap["name"].(string)
					Expect(ok).To(BeTrue(), "expected tool name to be a string")
					toolNames = append(toolNames, name)
				}
			})

			It("includes the expected gRPC-derived tool names", func() {
				Expect(toolNames).To(ContainElements(
					"api_v1_Frontmatter_GetFrontmatter",
					"api_v1_Frontmatter_MergeFrontmatter",
					"api_v1_Frontmatter_ReplaceFrontmatter",
					"api_v1_Frontmatter_RemoveKeyAtPath",
					"api_v1_InventoryManagementService_CreateInventoryItem",
					"api_v1_InventoryManagementService_MoveInventoryItem",
					"api_v1_InventoryManagementService_ListContainerContents",
					"api_v1_InventoryManagementService_FindItemLocation",
					"api_v1_PageImportService_ParseCSVPreview",
					"api_v1_PageImportService_StartPageImportJob",
					"api_v1_PageManagementService_CreatePage",
					"api_v1_PageManagementService_ReadPage",
					"api_v1_PageManagementService_DeletePage",
					"api_v1_PageManagementService_UpdatePageContent",
					"api_v1_PageManagementService_UpdateWholePage",
					"api_v1_PageManagementService_GenerateIdentifier",
					"api_v1_PageManagementService_ListTemplates",
					"api_v1_SearchService_SearchContent",
					"api_v1_SystemInfoService_GetVersion",
					"api_v1_SystemInfoService_GetJobStatus",
				))
			})
		})
	})
})

func (noOpChatBufferManager) RequestPermission(_ context.Context, _ string, _ string, _ string, _ string, _ []chatbuffer.PermissionOption) string {
	return ""
}

func (noOpChatBufferManager) GetPendingPermissionsForPage(string) []*chatbuffer.PermissionRequestEvent {
	return nil
}
