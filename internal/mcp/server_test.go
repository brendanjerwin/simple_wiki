//revive:disable:dot-imports
package mcp_test

import (
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

// noOpBleveIndexQueryer satisfies bleve.IQueryBleveIndex for tests.
type noOpBleveIndexQueryer struct{}

func (noOpBleveIndexQueryer) Query(string) ([]bleve.SearchResult, error) { return nil, nil }

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

func (noOpChatBufferManager) AddUserMessage(string, string, string) (string, error) {
	return "", nil
}
func (noOpChatBufferManager) AddAssistantMessage(string, string, string) (string, error) {
	return "", nil
}
func (noOpChatBufferManager) EditMessage(string, string) error {
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
	return ch, func() {}
}
func (noOpChatBufferManager) SubscribeToChannel() (<-chan *chatbuffer.Message, func()) {
	ch := make(chan *chatbuffer.Message)
	close(ch)
	return ch, func() {}
}
func (noOpChatBufferManager) HasChannelSubscribers() bool {
	return false
}

func mustNewAPIServer() *grpcapi.Server {
	srv, err := grpcapi.NewServer(
		"test-commit",
		time.Now(),
		noOpPageReaderMutator{},
		noOpBleveIndexQueryer{},
		nil,
		lumber.NewConsoleLogger(lumber.WARN),
		nil,
		nil,
		noOpFrontmatterIndexQueryer{},
		nil,
		noOpChatBufferManager{}, // chatBufferManager
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
