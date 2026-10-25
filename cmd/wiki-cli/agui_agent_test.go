//revive:disable:dot-imports
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	acp "github.com/coder/acp-go-sdk"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// sseEvent formats a single AG-UI event as an SSE data line.
func sseEvent(eventType string, fields map[string]any) string {
	fields["type"] = eventType
	raw, _ := json.Marshal(fields)
	return "data: " + string(raw) + "\n\n"
}

// newTestChatClient builds a wikiChatClient with an injected mock chatReplier
// so tests do not need a real wiki HTTP server.
func newTestChatClient(replier chatReplier) *wikiChatClient {
	return &wikiChatClient{
		page:       "test-page",
		wikiURL:    "http://localhost:0",
		chatClient: replier,
	}
}

// newTestAGUIAgent builds an agUIAgent wired to a test HTTP endpoint and a
// stub chatReplier.
func newTestAGUIAgent(endpointURL string, replier chatReplier) *agUIAgent {
	cfg := agUIBackendConfig{
		ID:       "test-agent",
		Endpoint: endpointURL,
	}
	chatClient := newTestChatClient(replier)
	return newAgUIAgent(cfg, chatClient, "test-page", func(_ string) string { return "" })
}

// sseServer returns an httptest.Server that serves the given SSE body verbatim.
// The caller must close the server after the test.
func sseServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
}

// ─── loadAGUIBackendConfigs ───────────────────────────────────────────────────

var _ = Describe("loadAGUIBackendConfigs", func() {
	Describe("when path is empty", func() {
		var result []agUIBackendConfig

		BeforeEach(func() {
			result = loadAGUIBackendConfigs("")
		})

		It("should return nil", func() {
			Expect(result).To(BeNil())
		})
	})

	Describe("when file does not exist", func() {
		var result []agUIBackendConfig

		BeforeEach(func() {
			result = loadAGUIBackendConfigs("/nonexistent/path/agui.json")
		})

		It("should return nil", func() {
			Expect(result).To(BeNil())
		})
	})

	Describe("when file contains malformed JSON", func() {
		var result []agUIBackendConfig
		var tmpFile *os.File

		BeforeEach(func() {
			var err error
			tmpFile, err = os.CreateTemp("", "agui-*.json")
			Expect(err).NotTo(HaveOccurred())
			_, err = tmpFile.WriteString("{not valid json}")
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpFile.Close()).To(Succeed())

			result = loadAGUIBackendConfigs(tmpFile.Name())
		})

		AfterEach(func() {
			_ = os.Remove(tmpFile.Name())
		})

		It("should return nil", func() {
			Expect(result).To(BeNil())
		})
	})

	Describe("when file contains valid entries", func() {
		var result []agUIBackendConfig
		var tmpFile *os.File

		BeforeEach(func() {
			var err error
			tmpFile, err = os.CreateTemp("", "agui-*.json")
			Expect(err).NotTo(HaveOccurred())
			_, err = tmpFile.WriteString(`{
				"agents": [
					{"id": "agent-a", "display_name": "Agent A", "endpoint": "https://a.example/run", "token_env": "A_TOKEN"},
					{"id": "agent-b", "endpoint": "https://b.example/run"},
					{"id": "", "endpoint": "https://bad.example/run"},
					{"id": "no-endpoint", "endpoint": ""}
				]
			}`)
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpFile.Close()).To(Succeed())

			result = loadAGUIBackendConfigs(tmpFile.Name())
		})

		AfterEach(func() {
			_ = os.Remove(tmpFile.Name())
		})

		It("should return only entries with non-empty id and endpoint", func() {
			Expect(result).To(HaveLen(2))
		})

		It("should populate fields from JSON", func() {
			Expect(result[0].ID).To(Equal("agent-a"))
			Expect(result[0].DisplayName).To(Equal("Agent A"))
			Expect(result[0].Endpoint).To(Equal("https://a.example/run"))
			Expect(result[0].TokenEnv).To(Equal("A_TOKEN"))
		})

		It("should include entries without a token_env", func() {
			Expect(result[1].ID).To(Equal("agent-b"))
			Expect(result[1].TokenEnv).To(Equal(""))
		})
	})
})

// ─── agUIAgent.Initialize ────────────────────────────────────────────────────

var _ = Describe("agUIAgent", func() {
	Describe("Initialize", func() {
		var resp acp.InitializeResponse
		var err error

		BeforeEach(func() {
			agent := newTestAGUIAgent("http://unused", &mockChatReplier{})
			resp, err = agent.Initialize(context.Background(), acp.InitializeRequest{})
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the protocol version", func() {
			Expect(resp.ProtocolVersion).To(BeEquivalentTo(acp.ProtocolVersionNumber))
		})
	})

	// ─── NewSession ──────────────────────────────────────────────────────────

	Describe("NewSession", func() {
		var resp acp.NewSessionResponse
		var err error

		BeforeEach(func() {
			agent := newTestAGUIAgent("http://unused", &mockChatReplier{})
			resp, err = agent.NewSession(context.Background(), acp.NewSessionRequest{})
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the page name as the session id", func() {
			Expect(string(resp.SessionId)).To(Equal("test-page"))
		})
	})

	// ─── Prompt — happy path ─────────────────────────────────────────────────

	Describe("Prompt", func() {
		Describe("when the endpoint returns a text message stream", func() {
			var err error
			var replier *mockChatReplier
			var sentRequests []string

			BeforeEach(func() {
				replier = &mockChatReplier{
					sendReplyResp: &apiv1.SendChatReplyResponse{MessageId: "msg-1"},
				}

				// Build SSE stream: TEXT_MESSAGE_START → two CONTENT deltas → TEXT_MESSAGE_END → RUN_FINISHED
				stream := strings.Join([]string{
					sseEvent(agUITypeTextMessageStart, map[string]any{"messageId": "m1", "role": "assistant"}),
					sseEvent(agUITypeTextMessageContent, map[string]any{"messageId": "m1", "delta": "Hello, "}),
					sseEvent(agUITypeTextMessageContent, map[string]any{"messageId": "m1", "delta": "world!"}),
					sseEvent(agUITypeTextMessageEnd, map[string]any{"messageId": "m1"}),
					sseEvent(agUITypeRunFinished, map[string]any{"status": "success"}),
				}, "")

				srv := sseServer(stream)
				DeferCleanup(srv.Close)

				agent := newTestAGUIAgent(srv.URL, replier)
				_, err = agent.Prompt(context.Background(), acp.PromptRequest{
					Prompt: []acp.ContentBlock{
						{Text: &acp.ContentBlockText{Text: "Hi!", Type: "text"}},
					},
				})

				_ = sentRequests
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should send a chat reply with the first delta", func() {
				Expect(replier.sendReplyCalled).To(BeTrue())
			})

			It("should accumulate text into the chat reply", func() {
				Expect(replier.editCalled).To(BeTrue())
				Expect(replier.editReq.GetNewContent()).To(ContainSubstring("Hello, world!"))
			})
		})

		Describe("when the endpoint returns a TEXT_MESSAGE_CHUNK shorthand", func() {
			var err error
			var replier *mockChatReplier

			BeforeEach(func() {
				replier = &mockChatReplier{
					sendReplyResp: &apiv1.SendChatReplyResponse{MessageId: "msg-2"},
				}

				stream := sseEvent(agUITypeTextMessageChunk, map[string]any{"messageId": "m2", "delta": "Chunk text"}) +
					sseEvent(agUITypeRunFinished, map[string]any{"status": "success"})

				srv := sseServer(stream)
				DeferCleanup(srv.Close)

				agent := newTestAGUIAgent(srv.URL, replier)
				_, err = agent.Prompt(context.Background(), acp.PromptRequest{
					Prompt: []acp.ContentBlock{
						{Text: &acp.ContentBlockText{Text: "x", Type: "text"}},
					},
				})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should send a chat reply", func() {
				Expect(replier.sendReplyCalled).To(BeTrue())
			})
		})

		Describe("when the endpoint returns tool call events", func() {
			var err error
			var replier *mockChatReplier

			BeforeEach(func() {
				replier = &mockChatReplier{
					// Provide a message ID so handleToolCall can attach the notification.
					sendReplyResp: &apiv1.SendChatReplyResponse{MessageId: "msg-tool"},
				}

				// Text message establishes currentMsg; tool call events follow.
				stream := strings.Join([]string{
					sseEvent(agUITypeTextMessageStart, map[string]any{"messageId": "m1", "role": "assistant"}),
					sseEvent(agUITypeTextMessageContent, map[string]any{"messageId": "m1", "delta": "Using a tool..."}),
					sseEvent(agUITypeTextMessageEnd, map[string]any{"messageId": "m1"}),
					sseEvent(agUITypeToolCallStart, map[string]any{"toolCallId": "tc-1", "toolCallName": "read_file"}),
					sseEvent(agUITypeToolCallArgs, map[string]any{"toolCallId": "tc-1", "delta": `{"path":"/tmp/x"}`}),
					sseEvent(agUITypeToolCallEnd, map[string]any{"toolCallId": "tc-1"}),
					sseEvent(agUITypeRunFinished, map[string]any{"status": "success"}),
				}, "")

				srv := sseServer(stream)
				DeferCleanup(srv.Close)

				agent := newTestAGUIAgent(srv.URL, replier)
				_, err = agent.Prompt(context.Background(), acp.PromptRequest{
					Prompt: []acp.ContentBlock{
						{Text: &acp.ContentBlockText{Text: "use tool", Type: "text"}},
					},
				})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should send a tool call notification", func() {
				Expect(replier.toolNotifyCalled).To(BeTrue())
				Expect(replier.toolNotifyReq).NotTo(BeNil())
			})
		})

		Describe("when the endpoint returns reasoning events", func() {
			var err error
			var replier *mockChatReplier

			BeforeEach(func() {
				replier = &mockChatReplier{
					sendReplyResp: &apiv1.SendChatReplyResponse{MessageId: "thought-msg"},
				}

				stream := strings.Join([]string{
					sseEvent(agUITypeMsgReasoningContent, map[string]any{"messageId": "r1", "delta": "Thinking..."}),
					sseEvent(agUITypeRunFinished, map[string]any{"status": "success"}),
				}, "")

				srv := sseServer(stream)
				DeferCleanup(srv.Close)

				agent := newTestAGUIAgent(srv.URL, replier)
				_, err = agent.Prompt(context.Background(), acp.PromptRequest{
					Prompt: []acp.ContentBlock{
						{Text: &acp.ContentBlockText{Text: "think", Type: "text"}},
					},
				})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should stream the thought as a chat reply (collapsible section)", func() {
				// handleThought routes through streamOrCreateReply → SendChatReply
				Expect(replier.sendReplyCalled).To(BeTrue())
			})
		})

		Describe("when the endpoint emits RUN_ERROR", func() {
			var err error

			BeforeEach(func() {
				stream := sseEvent(agUITypeRunError, map[string]any{"message": "agent crashed", "code": "E500"})

				srv := sseServer(stream)
				DeferCleanup(srv.Close)

				agent := newTestAGUIAgent(srv.URL, &mockChatReplier{})
				_, err = agent.Prompt(context.Background(), acp.PromptRequest{
					Prompt: []acp.ContentBlock{
						{Text: &acp.ContentBlockText{Text: "hello", Type: "text"}},
					},
				})
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should include the RUN_ERROR message in the error", func() {
				Expect(err.Error()).To(ContainSubstring("agent crashed"))
			})

			It("should include the error code in the error", func() {
				Expect(err.Error()).To(ContainSubstring("E500"))
			})
		})

		Describe("when the endpoint returns an interrupt and then success on resume", func() {
			var err error
			var replier *mockChatReplier
			var requestCount int

			BeforeEach(func() {
				replier = &mockChatReplier{
					permResp: &apiv1.RequestPermissionFromUserResponse{
						SelectedOptionId: "approve",
					},
					sendReplyResp: &apiv1.SendChatReplyResponse{MessageId: "msg-x"},
				}

				// First request → interrupt; second request → success + text.
				requestCount = 0
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestCount++
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)

					if requestCount == 1 {
						// First call: interrupt
						body := sseEvent(agUITypeRunFinished, map[string]any{
							"status": "interrupt",
							"interrupts": []map[string]any{
								{"id": "ir-1", "message": "Allow network access?"},
							},
						})
						_, _ = fmt.Fprint(w, body)
					} else {
						// Second call (resume): success with text
						body := strings.Join([]string{
							sseEvent(agUITypeTextMessageStart, map[string]any{"messageId": "m1", "role": "assistant"}),
							sseEvent(agUITypeTextMessageContent, map[string]any{"messageId": "m1", "delta": "Done!"}),
							sseEvent(agUITypeTextMessageEnd, map[string]any{"messageId": "m1"}),
							sseEvent(agUITypeRunFinished, map[string]any{"status": "success"}),
						}, "")
						_, _ = fmt.Fprint(w, body)
					}
				}))
				DeferCleanup(srv.Close)

				agent := newTestAGUIAgent(srv.URL, replier)
				_, err = agent.Prompt(context.Background(), acp.PromptRequest{
					Prompt: []acp.ContentBlock{
						{Text: &acp.ContentBlockText{Text: "do something risky", Type: "text"}},
					},
				})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should relay the interrupt as a permission request", func() {
				Expect(replier.permCalled).To(BeTrue())
			})

			It("should make two requests to the endpoint (initial + resume)", func() {
				Expect(requestCount).To(Equal(2))
			})

			It("should stream the resumed response text", func() {
				Expect(replier.sendReplyCalled).To(BeTrue())
			})
		})

		Describe("when the endpoint returns HTTP 500", func() {
			var err error

			BeforeEach(func() {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
				DeferCleanup(srv.Close)

				agent := newTestAGUIAgent(srv.URL, &mockChatReplier{})
				_, err = agent.Prompt(context.Background(), acp.PromptRequest{
					Prompt: []acp.ContentBlock{
						{Text: &acp.ContentBlockText{Text: "hello", Type: "text"}},
					},
				})
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("HTTP 500"))
			})
		})

		Describe("when a bearer token env var is set", func() {
			var receivedAuthHeader string

			BeforeEach(func() {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					receivedAuthHeader = r.Header.Get("Authorization")
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)
					_, _ = fmt.Fprint(w, sseEvent(agUITypeRunFinished, map[string]any{"status": "success"}))
				}))
				DeferCleanup(srv.Close)

				cfg := agUIBackendConfig{
					ID:       "auth-agent",
					Endpoint: srv.URL,
					TokenEnv: "TEST_AGUI_TOKEN",
				}
				chatClient := newTestChatClient(&mockChatReplier{})
				agent := newAgUIAgent(cfg, chatClient, "test-page", func(key string) string {
					if key == "TEST_AGUI_TOKEN" {
						return "secret-bearer-token"
					}
					return ""
				})
				_, _ = agent.Prompt(context.Background(), acp.PromptRequest{
					Prompt: []acp.ContentBlock{
						{Text: &acp.ContentBlockText{Text: "hi", Type: "text"}},
					},
				})
			})

			It("should send the bearer token in the Authorization header", func() {
				Expect(receivedAuthHeader).To(Equal("Bearer secret-bearer-token"))
			})
		})

		Describe("when the context is cancelled before the request completes", func() {
			var err error

			BeforeEach(func() {
				// Server that blocks indefinitely
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)
					<-r.Context().Done()
				}))
				DeferCleanup(srv.Close)

				ctx, cancel := context.WithCancel(context.Background())
				cancel() // cancel immediately

				agent := newTestAGUIAgent(srv.URL, &mockChatReplier{})
				_, err = agent.Prompt(ctx, acp.PromptRequest{
					Prompt: []acp.ContentBlock{
						{Text: &acp.ContentBlockText{Text: "hello", Type: "text"}},
					},
				})
			})

			It("should not return an error (cancelled = clean exit)", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	// ─── jsonString helpers ──────────────────────────────────────────────────

	Describe("jsonString", func() {
		Describe("when the key exists and is a string", func() {
			var result string

			BeforeEach(func() {
				raw := map[string]json.RawMessage{
					"key": json.RawMessage(`"value"`),
				}
				result = jsonString(raw, "key")
			})

			It("should return the string value", func() {
				Expect(result).To(Equal("value"))
			})
		})

		Describe("when the key is missing", func() {
			var result string

			BeforeEach(func() {
				result = jsonString(map[string]json.RawMessage{}, "missing")
			})

			It("should return empty string", func() {
				Expect(result).To(Equal(""))
			})
		})

		Describe("when the value is not a string", func() {
			var result string

			BeforeEach(func() {
				raw := map[string]json.RawMessage{
					"num": json.RawMessage(`42`),
				}
				result = jsonString(raw, "num")
			})

			It("should return empty string", func() {
				Expect(result).To(Equal(""))
			})
		})
	})

	Describe("jsonStringDefault", func() {
		Describe("when the key is missing", func() {
			var result string

			BeforeEach(func() {
				result = jsonStringDefault(map[string]json.RawMessage{}, "missing", "fallback")
			})

			It("should return the default", func() {
				Expect(result).To(Equal("fallback"))
			})
		})

		Describe("when the key exists", func() {
			var result string

			BeforeEach(func() {
				raw := map[string]json.RawMessage{"k": json.RawMessage(`"present"`)}
				result = jsonStringDefault(raw, "k", "fallback")
			})

			It("should return the actual value", func() {
				Expect(result).To(Equal("present"))
			})
		})
	})
})
