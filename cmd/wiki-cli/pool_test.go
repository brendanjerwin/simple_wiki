package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"time"

	"connectrpc.com/connect"
	acp "github.com/coder/acp-go-sdk"
	"google.golang.org/protobuf/types/known/structpb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1connect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cli "gopkg.in/urfave/cli.v1"
)

// stubChatServiceHandler is a mock Connect handler for the ChatService
// that tracks which RPCs were called and returns predefined responses.
type stubChatServiceHandler struct {
	apiv1connect.UnimplementedChatServiceHandler
	sendReplyID    string
	sendReplyErr   error
	editErr        error
	sendReplyCalled bool
	editCalled      bool
}

func (h *stubChatServiceHandler) SendChatReply(_ context.Context, _ *connect.Request[apiv1.SendChatReplyRequest]) (*connect.Response[apiv1.SendChatReplyResponse], error) {
	h.sendReplyCalled = true
	if h.sendReplyErr != nil {
		return nil, h.sendReplyErr
	}
	return connect.NewResponse(&apiv1.SendChatReplyResponse{
		MessageId: h.sendReplyID,
	}), nil
}

func (h *stubChatServiceHandler) EditChatMessage(_ context.Context, _ *connect.Request[apiv1.EditChatMessageRequest]) (*connect.Response[apiv1.EditChatMessageResponse], error) {
	h.editCalled = true
	if h.editErr != nil {
		return nil, h.editErr
	}
	return connect.NewResponse(&apiv1.EditChatMessageResponse{}), nil
}

// mockChatReplier is a mock implementation of the chatReplier interface
// for unit testing without HTTP servers.
type mockChatReplier struct {
	sendReplyReq     *apiv1.SendChatReplyRequest
	sendReplyResp    *apiv1.SendChatReplyResponse
	sendReplyErr     error
	sendReplyCalled  bool

	editReq          *apiv1.EditChatMessageRequest
	editResp         *apiv1.EditChatMessageResponse
	editErr          error
	editCalled       bool

	toolNotifyReq    *apiv1.SendToolCallNotificationRequest
	toolNotifyCalled bool
	toolNotifyErr    error

	permReq          *apiv1.RequestPermissionFromUserRequest
	permResp         *apiv1.RequestPermissionFromUserResponse
	permErr          error
	permCalled       bool
}

func (m *mockChatReplier) SendChatReply(_ context.Context, req *connect.Request[apiv1.SendChatReplyRequest]) (*connect.Response[apiv1.SendChatReplyResponse], error) {
	m.sendReplyCalled = true
	m.sendReplyReq = req.Msg
	if m.sendReplyErr != nil {
		return nil, m.sendReplyErr
	}
	resp := m.sendReplyResp
	if resp == nil {
		resp = &apiv1.SendChatReplyResponse{MessageId: "mock-msg-1"}
	}
	return connect.NewResponse(resp), nil
}

func (m *mockChatReplier) EditChatMessage(_ context.Context, req *connect.Request[apiv1.EditChatMessageRequest]) (*connect.Response[apiv1.EditChatMessageResponse], error) {
	m.editCalled = true
	m.editReq = req.Msg
	if m.editErr != nil {
		return nil, m.editErr
	}
	resp := m.editResp
	if resp == nil {
		resp = &apiv1.EditChatMessageResponse{}
	}
	return connect.NewResponse(resp), nil
}

func (m *mockChatReplier) SendToolCallNotification(_ context.Context, req *connect.Request[apiv1.SendToolCallNotificationRequest]) (*connect.Response[apiv1.SendToolCallNotificationResponse], error) {
	m.toolNotifyCalled = true
	m.toolNotifyReq = req.Msg
	if m.toolNotifyErr != nil {
		return nil, m.toolNotifyErr
	}
	return connect.NewResponse(&apiv1.SendToolCallNotificationResponse{}), nil
}

func (m *mockChatReplier) RequestPermissionFromUser(_ context.Context, req *connect.Request[apiv1.RequestPermissionFromUserRequest]) (*connect.Response[apiv1.RequestPermissionFromUserResponse], error) {
	m.permCalled = true
	m.permReq = req.Msg
	if m.permErr != nil {
		return nil, m.permErr
	}
	resp := m.permResp
	if resp == nil {
		resp = &apiv1.RequestPermissionFromUserResponse{}
	}
	return connect.NewResponse(resp), nil
}

// mockACPAgent is a mock implementation of the acpAgent interface.
type mockACPAgent struct {
	initResp    acp.InitializeResponse
	initErr     error
	initCalled  bool

	sessionResp acp.NewSessionResponse
	sessionErr  error
	sessionCalled bool

	promptResp  acp.PromptResponse
	promptErr   error
	promptCalled bool
	promptReq    *acp.PromptRequest
}

func (m *mockACPAgent) Initialize(_ context.Context, _ acp.InitializeRequest) (acp.InitializeResponse, error) {
	m.initCalled = true
	return m.initResp, m.initErr
}

func (m *mockACPAgent) NewSession(_ context.Context, _ acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	m.sessionCalled = true
	return m.sessionResp, m.sessionErr
}

func (m *mockACPAgent) Prompt(_ context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	m.promptCalled = true
	m.promptReq = &req
	return m.promptResp, m.promptErr
}

var _ = Describe("sanitizeUnitName", func() {
	When("given a simple identifier", func() {
		var result string

		BeforeEach(func() {
			result = sanitizeUnitName("my_page")
		})

		It("should replace underscores with hyphens", func() {
			Expect(result).To(Equal("my-page"))
		})
	})

	When("given an identifier with multiple special characters", func() {
		var result string

		BeforeEach(func() {
			result = sanitizeUnitName("page/with spaces!and@symbols")
		})

		It("should replace all non-alphanumeric sequences with a single hyphen", func() {
			Expect(result).To(Equal("page-with-spaces-and-symbols"))
		})
	})

	When("given a pure alphanumeric identifier", func() {
		var result string

		BeforeEach(func() {
			result = sanitizeUnitName("simplepage123")
		})

		It("should return it unchanged", func() {
			Expect(result).To(Equal("simplepage123"))
		})
	})
})

var _ = Describe("buildPoolCommand", func() {
	var cmd cli.Command

	BeforeEach(func() {
		cmd = buildPoolCommand(cli.StringFlag{
			Name:  "url, u",
			Value: "http://localhost:8050",
		})
	})

	It("should have name pool", func() {
		Expect(cmd.Name).To(Equal("pool"))
	})

	It("should have a non-empty usage", func() {
		Expect(cmd.Usage).NotTo(BeEmpty())
	})

	It("should have a non-nil action", func() {
		Expect(cmd.Action).NotTo(BeNil())
	})

	When("inspecting the flags", func() {
		var flagNames []string

		BeforeEach(func() {
			flagNames = make([]string, 0, len(cmd.Flags))
			for _, f := range cmd.Flags {
				flagNames = append(flagNames, f.GetName())
			}
		})

		It("should include the url flag", func() {
			Expect(flagNames).To(ContainElement("url, u"))
		})

		It("should include the max-instances flag", func() {
			Expect(flagNames).To(ContainElement("max-instances"))
		})

		It("should include the idle-timeout flag", func() {
			Expect(flagNames).To(ContainElement("idle-timeout"))
		})

		It("should include the agent-path flag", func() {
			Expect(flagNames).To(ContainElement("agent-path"))
		})

		It("should include the no-systemd flag", func() {
			Expect(flagNames).To(ContainElement("no-systemd"))
		})
	})
})

var _ = Describe("instanceEntry", func() {
	Describe("touch", func() {
		When("called on an entry with stale lastActive", func() {
			var entry *instanceEntry

			BeforeEach(func() {
				entry = &instanceEntry{
					page:       "test-page",
					lastActive: time.Now().Add(-10 * time.Minute),
				}
				entry.touch()
			})

			It("should update lastActive to near now", func() {
				Expect(entry.lastActive).To(BeTemporally("~", time.Now(), time.Second))
			})
		})
	})

	Describe("idleSince", func() {
		When("entry was recently active", func() {
			var idle time.Duration

			BeforeEach(func() {
				entry := &instanceEntry{page: "test-page", lastActive: time.Now()}
				idle = entry.idleSince()
			})

			It("should return a very short duration", func() {
				Expect(idle).To(BeNumerically("<", time.Second))
			})
		})

		When("entry has been idle for a while", func() {
			var idle time.Duration

			BeforeEach(func() {
				entry := &instanceEntry{page: "test-page", lastActive: time.Now().Add(-5 * time.Minute)}
				idle = entry.idleSince()
			})

			It("should return approximately the idle duration", func() {
				Expect(idle).To(BeNumerically("~", 5*time.Minute, time.Second))
			})
		})
	})
})

var _ = Describe("poolDaemon", func() {
	Describe("evictLeastActive", func() {
		When("multiple instances exist", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					instances: map[string]*instanceEntry{
						"new-page": {
							page:       "new-page",
							lastActive: time.Now(),
							cancel: func() {
								// no-op: test stub; no real goroutine to cancel
							},
						},
						"old-page": {
							page:       "old-page",
							lastActive: time.Now().Add(-30 * time.Minute),
							cancel: func() {
								// no-op: test stub; no real goroutine to cancel
							},
						},
						"mid-page": {
							page:       "mid-page",
							lastActive: time.Now().Add(-10 * time.Minute),
							cancel: func() {
								// no-op: test stub; no real goroutine to cancel
							},
						},
					},
				}
				daemon.mu.Lock()
				daemon.evictLeastActive()
				daemon.mu.Unlock()
			})

			It("should evict the oldest instance", func() {
				Expect(daemon.instances).NotTo(HaveKey("old-page"))
			})

			It("should keep the other instances", func() {
				Expect(daemon.instances).To(HaveKey("new-page"))
				Expect(daemon.instances).To(HaveKey("mid-page"))
			})
		})

		When("no instances exist", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					instances: make(map[string]*instanceEntry),
				}
				daemon.mu.Lock()
				daemon.evictLeastActive()
				daemon.mu.Unlock()
			})

			It("should not panic", func() {
				Expect(daemon.instances).To(BeEmpty())
			})
		})
	})

	Describe("stopInstanceLocked", func() {
		When("instance exists", func() {
			var (
				daemon   *poolDaemon
				canceled bool
			)

			BeforeEach(func() {
				canceled = false
				daemon = &poolDaemon{
					instances: map[string]*instanceEntry{
						"page-a": {
							page:   "page-a",
							cancel: func() { canceled = true },
						},
					},
				}
				daemon.mu.Lock()
				daemon.stopInstanceLocked("page-a")
				daemon.mu.Unlock()
			})

			It("should cancel the instance", func() {
				Expect(canceled).To(BeTrue())
			})

			It("should remove the instance", func() {
				Expect(daemon.instances).NotTo(HaveKey("page-a"))
			})
		})

		When("instance does not exist", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					instances: make(map[string]*instanceEntry),
				}
				daemon.mu.Lock()
				daemon.stopInstanceLocked("nonexistent")
				daemon.mu.Unlock()
			})

			It("should not panic", func() {
				Expect(daemon.instances).To(BeEmpty())
			})
		})
	})

	Describe("stopAll", func() {
		When("instances are running", func() {
			var (
				daemon    *poolDaemon
				canceledA bool
				canceledB bool
			)

			BeforeEach(func() {
				canceledA = false
				canceledB = false
				daemon = &poolDaemon{
					instances: map[string]*instanceEntry{
						"page-a": {
							page:   "page-a",
							cancel: func() { canceledA = true },
						},
						"page-b": {
							page:   "page-b",
							cancel: func() { canceledB = true },
						},
					},
				}
				daemon.stopAll()
			})

			It("should cancel all instances", func() {
				Expect(canceledA).To(BeTrue())
				Expect(canceledB).To(BeTrue())
			})

			It("should clear the instances map", func() {
				Expect(daemon.instances).To(BeEmpty())
			})
		})

		When("no instances exist", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					instances: make(map[string]*instanceEntry),
				}
				daemon.stopAll()
			})

			It("should leave the map empty", func() {
				Expect(daemon.instances).To(BeEmpty())
			})
		})
	})

	Describe("reapOnce", func() {
		When("an instance exceeds idle timeout", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					idleTimeout: 10 * time.Minute,
					instances: map[string]*instanceEntry{
						"idle-page": {
							page:       "idle-page",
							createdAt:  time.Now(),
							lastActive: time.Now().Add(-20 * time.Minute),
							state:      StateIdle,
							cancel: func() {
								// no-op: test stub; no real goroutine to cancel
							},
						},
						"active-page": {
							page:       "active-page",
							createdAt:  time.Now(),
							lastActive: time.Now(),
							state:      StateIdle,
							cancel: func() {
								// no-op: test stub; no real goroutine to cancel
							},
						},
					},
				}

				daemon.reapOnce()
			})

			It("should reap the idle instance", func() {
				Expect(daemon.instances).NotTo(HaveKey("idle-page"))
			})

			It("should keep the active instance", func() {
				Expect(daemon.instances).To(HaveKey("active-page"))
			})
		})

		When("an instance exceeds max instance age", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					idleTimeout:    10 * time.Minute,
					maxInstanceAge: 1 * time.Hour,
					instances: map[string]*instanceEntry{
						"old-page": {
							page:       "old-page",
							createdAt:  time.Now().Add(-2 * time.Hour),
							lastActive: time.Now(), // active recently, but too old
							state:      StatePrompting,
							cancel: func() {
								// no-op: test stub
							},
						},
						"young-page": {
							page:       "young-page",
							createdAt:  time.Now(),
							lastActive: time.Now(),
							state:      StateIdle,
							cancel: func() {
								// no-op: test stub
							},
						},
					},
				}

				daemon.reapOnce()
			})

			It("should reap the overaged instance regardless of activity", func() {
				Expect(daemon.instances).NotTo(HaveKey("old-page"))
			})

			It("should keep the young instance", func() {
				Expect(daemon.instances).To(HaveKey("young-page"))
			})
		})

		When("max instance age is zero (disabled)", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					idleTimeout:    10 * time.Minute,
					maxInstanceAge: 0, // disabled
					instances: map[string]*instanceEntry{
						"old-active-page": {
							page:       "old-active-page",
							createdAt:  time.Now().Add(-48 * time.Hour),
							lastActive: time.Now(), // recently active
							state:      StateIdle,
							cancel: func() {
								// no-op: test stub
							},
						},
					},
				}

				daemon.reapOnce()
			})

			It("should not reap the old but active instance", func() {
				Expect(daemon.instances).To(HaveKey("old-active-page"))
			})
		})

		When("an instance is stuck in PermissionPending past the timeout", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					idleTimeout:               10 * time.Minute,
					permissionPendingTimeout:  5 * time.Minute,
					instances: map[string]*instanceEntry{
						"stuck-perm-page": {
							page:       "stuck-perm-page",
							createdAt:  time.Now().Add(-30 * time.Minute),
							lastActive: time.Now().Add(-20 * time.Minute),
							state:      StatePermissionPending,
							cancel: func() {
								// no-op: test stub
							},
						},
						"recent-perm-page": {
							page:       "recent-perm-page",
							createdAt:  time.Now(),
							lastActive: time.Now().Add(-1 * time.Minute),
							state:      StatePermissionPending,
							cancel: func() {
								// no-op: test stub
							},
						},
					},
				}

				daemon.reapOnce()
			})

			It("should reap the stuck permission-pending instance", func() {
				Expect(daemon.instances).NotTo(HaveKey("stuck-perm-page"))
			})

			It("should keep the recently-entered permission-pending instance", func() {
				Expect(daemon.instances).To(HaveKey("recent-perm-page"))
			})
		})

		When("permission pending timeout is zero (disabled)", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					idleTimeout:              10 * time.Minute,
					permissionPendingTimeout: 0, // disabled
					instances: map[string]*instanceEntry{
						"stuck-perm-page": {
							page:       "stuck-perm-page",
							createdAt:  time.Now().Add(-2 * time.Hour),
							lastActive: time.Now().Add(-1 * time.Hour),
							state:      StatePermissionPending,
							cancel: func() {
								// no-op: test stub
							},
						},
					},
				}

				daemon.reapOnce()
			})

			It("should not reap permission-pending instances when timeout is disabled", func() {
				Expect(daemon.instances).To(HaveKey("stuck-perm-page"))
			})
		})
	})

	Describe("run", func() {
		When("context is cancelled immediately", func() {
			var (
				daemon *poolDaemon
				err    error
			)

			BeforeEach(func() {
				daemon = &poolDaemon{
					wikiURL:      "http://localhost:1",
					maxInstances: 5,
					idleTimeout:  30 * time.Minute,
					instances:    make(map[string]*instanceEntry),
				}
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				err = daemon.run(ctx)
			})

			It("should return nil", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

var _ = Describe("wikiChatClient", func() {
	Describe("SessionUpdate", func() {
		When("receiving agent message chunks", func() {
			var (
				client *wikiChatClient
				err    error
			)

			BeforeEach(func() {
				// Set up a mock server that accepts SendChatReply and EditChatMessage
				mux := http.NewServeMux()
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"messageId":"msg-1"}`))
				})
				server := httptest.NewServer(mux)
				DeferCleanup(server.Close)

				client = newWikiChatClient("test-page", server.URL)

				chunk1 := acp.SessionNotification{
					SessionId: "session-1",
					Update:    acp.UpdateAgentMessageText("Hello "),
				}
				chunk2 := acp.SessionNotification{
					SessionId: "session-1",
					Update:    acp.UpdateAgentMessageText("world!"),
				}

				err = client.SessionUpdate(context.Background(), chunk1)
				Expect(err).NotTo(HaveOccurred())
				err = client.SessionUpdate(context.Background(), chunk2)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should accumulate the text from both chunks", func() {
				Expect(client.textBuf.String()).To(Equal("Hello world!"))
			})
		})

		When("receiving a thought chunk", func() {
			var (
				client *wikiChatClient
				err    error
			)

			BeforeEach(func() {
				mux := http.NewServeMux()
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"messageId":"msg-1"}`))
				})
				server := httptest.NewServer(mux)
				DeferCleanup(server.Close)

				client = newWikiChatClient("test-page", server.URL)

				thought := acp.SessionNotification{
					SessionId: "session-1",
					Update:    acp.UpdateAgentThoughtText("thinking..."),
				}
				err = client.SessionUpdate(context.Background(), thought)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should accumulate thought text", func() {
				Expect(client.thoughtBuf.String()).To(Equal("thinking..."))
			})
		})

		When("receiving a chunk with non-text content", func() {
			var (
				client *wikiChatClient
				err    error
			)

			BeforeEach(func() {
				client = &wikiChatClient{
					page: "test-page",
				}

				imageChunk := acp.SessionNotification{
					SessionId: "session-1",
					Update: acp.SessionUpdate{
						AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
							Content: acp.ImageBlock("base64data", "image/png"),
						},
					},
				}
				err = client.SessionUpdate(context.Background(), imageChunk)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not accumulate any text", func() {
				Expect(client.textBuf.String()).To(BeEmpty())
			})
		})
	})

	Describe("beginTurn", func() {
		When("called", func() {
			var client *wikiChatClient

			BeforeEach(func() {
				client = newWikiChatClient("my-page", "http://localhost:1")
				client.textBuf.WriteString("leftover text")
				client.currentMsg = "old-msg"
				client.beginTurn("reply-123")
			})

			It("should reset the text buffer", func() {
				Expect(client.textBuf.String()).To(BeEmpty())
			})

			It("should set the replyToID", func() {
				Expect(client.replyToID).To(Equal("reply-123"))
			})

			It("should clear the current message ID", func() {
				Expect(client.currentMsg).To(BeEmpty())
			})
		})
	})

	Describe("endTurn", func() {
		When("called", func() {
			var client *wikiChatClient

			BeforeEach(func() {
				client = newWikiChatClient("my-page", "http://localhost:1")
				client.currentMsg = "some-msg"
				client.endTurn()
			})

			It("should clear the current message ID", func() {
				Expect(client.currentMsg).To(BeEmpty())
			})
		})
	})
})

var _ = Describe("prefixWriter", func() {
	When("writing a complete line", func() {
		var buf bytes.Buffer

		BeforeEach(func() {
			tmpFile, err := os.CreateTemp("", "prefix-test")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = os.Remove(tmpFile.Name()) })

			pw := newPrefixWriter(tmpFile, "my-page")
			_, err = pw.Write([]byte("hello world\n"))
			Expect(err).NotTo(HaveOccurred())

			content, err := os.ReadFile(tmpFile.Name())
			Expect(err).NotTo(HaveOccurred())
			buf.Write(content)
		})

		It("should prefix the line with the page name", func() {
			Expect(buf.String()).To(Equal("[my-page] hello world\n"))
		})
	})

	When("writing multiple lines at once", func() {
		var buf bytes.Buffer

		BeforeEach(func() {
			tmpFile, err := os.CreateTemp("", "prefix-test")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = os.Remove(tmpFile.Name()) })

			pw := newPrefixWriter(tmpFile, "pg")
			_, err = pw.Write([]byte("line1\nline2\n"))
			Expect(err).NotTo(HaveOccurred())

			content, err := os.ReadFile(tmpFile.Name())
			Expect(err).NotTo(HaveOccurred())
			buf.Write(content)
		})

		It("should prefix each line", func() {
			Expect(buf.String()).To(Equal("[pg] line1\n[pg] line2\n"))
		})
	})
})

var _ = Describe("truncate", func() {
	When("the string is shorter than the max", func() {
		var result string

		BeforeEach(func() {
			result = truncate("hello", 10)
		})

		It("should return the string unchanged", func() {
			Expect(result).To(Equal("hello"))
		})
	})

	When("the string is exactly the max length", func() {
		var result string

		BeforeEach(func() {
			result = truncate("12345", 5)
		})

		It("should return the string unchanged", func() {
			Expect(result).To(Equal("12345"))
		})
	})

	When("the string exceeds the max length", func() {
		var result string

		BeforeEach(func() {
			result = truncate("hello world", 5)
		})

		It("should truncate and add ellipsis", func() {
			Expect(result).To(Equal("hello..."))
		})
	})

	When("the string is empty", func() {
		var result string

		BeforeEach(func() {
			result = truncate("", 10)
		})

		It("should return an empty string", func() {
			Expect(result).To(BeEmpty())
		})
	})

	When("max is zero", func() {
		var result string

		BeforeEach(func() {
			result = truncate("hello", 0)
		})

		It("should return just the ellipsis", func() {
			Expect(result).To(Equal("..."))
		})
	})
})

var _ = Describe("wikiChatClient buildFullText", func() {
	When("only text is present", func() {
		var result string

		BeforeEach(func() {
			client := &wikiChatClient{}
			client.textBuf.WriteString("Hello world")
			client.mu.Lock()
			result = client.buildFullText()
			client.mu.Unlock()
		})

		It("should return just the text", func() {
			Expect(result).To(Equal("Hello world"))
		})
	})

	When("thought and text are both present", func() {
		var result string

		BeforeEach(func() {
			client := &wikiChatClient{}
			client.thoughtBuf.WriteString("Let me think about this")
			client.textBuf.WriteString("Here is my answer")
			client.mu.Lock()
			result = client.buildFullText()
			client.mu.Unlock()
		})

		It("should wrap thought in a details block", func() {
			Expect(result).To(ContainSubstring("<details><summary>Thinking...</summary>"))
			Expect(result).To(ContainSubstring("Let me think about this"))
			Expect(result).To(ContainSubstring("</details>"))
		})

		It("should include the response text after the details block", func() {
			Expect(result).To(ContainSubstring("Here is my answer"))
		})
	})

	When("plan entries are present", func() {
		var result string

		BeforeEach(func() {
			client := &wikiChatClient{}
			client.textBuf.WriteString("Working on it")
			client.planEntries = []acp.PlanEntry{
				{Content: "Step 1", Status: acp.PlanEntryStatusCompleted},
				{Content: "Step 2", Status: acp.PlanEntryStatusInProgress},
				{Content: "Step 3", Status: acp.PlanEntryStatusPending},
			}
			client.mu.Lock()
			result = client.buildFullText()
			client.mu.Unlock()
		})

		It("should include the plan section", func() {
			Expect(result).To(ContainSubstring("**Plan:**"))
		})

		It("should show completed entries with checkmarks", func() {
			Expect(result).To(ContainSubstring("- [x] Step 1"))
		})

		It("should show in-progress entries with spinner emoji", func() {
			Expect(result).To(ContainSubstring("- 🔄 Step 2"))
		})

		It("should show pending entries with empty checkboxes", func() {
			Expect(result).To(ContainSubstring("- [ ] Step 3"))
		})
	})

	When("permission notes are present", func() {
		var result string

		BeforeEach(func() {
			client := &wikiChatClient{}
			client.textBuf.WriteString("Done")
			client.permissionNotes.WriteString("> 🔐 **Permission granted:** edit — Allow\n")
			client.mu.Lock()
			result = client.buildFullText()
			client.mu.Unlock()
		})

		It("should append the permission notes", func() {
			Expect(result).To(ContainSubstring("Permission granted"))
		})
	})

	When("all sections are present", func() {
		var result string

		BeforeEach(func() {
			client := &wikiChatClient{}
			client.thoughtBuf.WriteString("Thinking hard")
			client.textBuf.WriteString("Final answer")
			client.planEntries = []acp.PlanEntry{
				{Content: "Do the thing", Status: acp.PlanEntryStatusCompleted},
			}
			client.permissionNotes.WriteString("> 🔐 **Permission granted:** x\n")
			client.mu.Lock()
			result = client.buildFullText()
			client.mu.Unlock()
		})

		It("should include thought details", func() {
			Expect(result).To(ContainSubstring("<details>"))
		})

		It("should include the response", func() {
			Expect(result).To(ContainSubstring("Final answer"))
		})

		It("should include the plan", func() {
			Expect(result).To(ContainSubstring("**Plan:**"))
		})

		It("should include the permission notes", func() {
			Expect(result).To(ContainSubstring("Permission granted"))
		})
	})

	When("everything is empty", func() {
		var result string

		BeforeEach(func() {
			client := &wikiChatClient{}
			client.mu.Lock()
			result = client.buildFullText()
			client.mu.Unlock()
		})

		It("should return an empty string", func() {
			Expect(result).To(BeEmpty())
		})
	})
})

var _ = Describe("wikiChatClient SessionUpdate with Plan", func() {
	When("receiving a plan update", func() {
		var (
			client *wikiChatClient
			err    error
		)

		BeforeEach(func() {
			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"messageId":"msg-1"}`))
			})
			server := httptest.NewServer(mux)
			DeferCleanup(server.Close)

			client = newWikiChatClient("test-page", server.URL)

			planNotification := acp.SessionNotification{
				SessionId: "session-1",
				Update: acp.UpdatePlan(
					acp.PlanEntry{Content: "Analyze code", Status: acp.PlanEntryStatusCompleted},
					acp.PlanEntry{Content: "Write tests", Status: acp.PlanEntryStatusInProgress},
					acp.PlanEntry{Content: "Submit PR", Status: acp.PlanEntryStatusPending},
				),
			}
			err = client.SessionUpdate(context.Background(), planNotification)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should store the plan entries", func() {
			Expect(client.planEntries).To(HaveLen(3))
		})

		It("should preserve the entry content", func() {
			Expect(client.planEntries[0].Content).To(Equal("Analyze code"))
			Expect(client.planEntries[1].Content).To(Equal("Write tests"))
			Expect(client.planEntries[2].Content).To(Equal("Submit PR"))
		})

		It("should preserve the entry statuses", func() {
			Expect(client.planEntries[0].Status).To(Equal(acp.PlanEntryStatusCompleted))
			Expect(client.planEntries[1].Status).To(Equal(acp.PlanEntryStatusInProgress))
			Expect(client.planEntries[2].Status).To(Equal(acp.PlanEntryStatusPending))
		})
	})
})

var _ = Describe("wikiChatClient SessionUpdate with ToolCall", func() {
	When("receiving a tool call with no current message", func() {
		var (
			client *wikiChatClient
			err    error
		)

		BeforeEach(func() {
			client = &wikiChatClient{
				page: "test-page",
			}

			toolCallUpdate := acp.SessionNotification{
				SessionId: "session-1",
				Update:    acp.StartToolCall("tc-1", "Read file"),
			}
			err = client.SessionUpdate(context.Background(), toolCallUpdate)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("receiving a tool call with a current message", func() {
		var (
			client      *wikiChatClient
			err         error
			requestPath string
		)

		BeforeEach(func() {
			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				requestPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			})
			server := httptest.NewServer(mux)
			DeferCleanup(server.Close)

			client = newWikiChatClient("test-page", server.URL)
			client.currentMsg = "msg-42"

			toolCallUpdate := acp.SessionNotification{
				SessionId: "session-1",
				Update:    acp.StartToolCall("tc-1", "Read file"),
			}
			err = client.SessionUpdate(context.Background(), toolCallUpdate)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should send a tool call notification to the server", func() {
			Expect(requestPath).To(ContainSubstring("SendToolCallNotification"))
		})
	})
})

var _ = Describe("wikiChatClient beginTurn", func() {
	When("called with accumulated state", func() {
		var client *wikiChatClient

		BeforeEach(func() {
			client = newWikiChatClient("my-page", "http://localhost:1")
			client.textBuf.WriteString("leftover text")
			client.thoughtBuf.WriteString("leftover thought")
			client.permissionNotes.WriteString("leftover permissions")
			client.planEntries = []acp.PlanEntry{
				{Content: "old step", Status: acp.PlanEntryStatusCompleted},
			}
			client.currentMsg = "old-msg"
			client.beginTurn("reply-456")
		})

		It("should reset the text buffer", func() {
			Expect(client.textBuf.String()).To(BeEmpty())
		})

		It("should reset the thought buffer", func() {
			Expect(client.thoughtBuf.String()).To(BeEmpty())
		})

		It("should reset the permission notes", func() {
			Expect(client.permissionNotes.String()).To(BeEmpty())
		})

		It("should clear the plan entries", func() {
			Expect(client.planEntries).To(BeNil())
		})

		It("should set the replyToID", func() {
			Expect(client.replyToID).To(Equal("reply-456"))
		})

		It("should clear the current message ID", func() {
			Expect(client.currentMsg).To(BeEmpty())
		})
	})
})

// stubFrontmatterHandler is a mock Connect handler for the Frontmatter service
// that returns predefined frontmatter data.
type stubFrontmatterHandler struct {
	apiv1connect.UnimplementedFrontmatterHandler
	frontmatter map[string]any
	err         error
}

func (h *stubFrontmatterHandler) GetFrontmatter(_ context.Context, req *connect.Request[apiv1.GetFrontmatterRequest]) (*connect.Response[apiv1.GetFrontmatterResponse], error) {
	if h.err != nil {
		return nil, h.err
	}

	s, err := structpb.NewStruct(h.frontmatter)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&apiv1.GetFrontmatterResponse{
		Frontmatter: s,
	}), nil
}

var _ = Describe("poolDaemon fetchPageContext", func() {
	When("the page has ai_agent_chat_context in frontmatter", func() {
		var result string

		BeforeEach(func() {
			handler := &stubFrontmatterHandler{
				frontmatter: map[string]any{
					"title": "Test Page",
					"ai_agent_chat_context": map[string]any{
						"summary": "We discussed testing",
						"goals":   "Add more test coverage",
					},
				},
			}
			mux := http.NewServeMux()
			path, h := apiv1connect.NewFrontmatterHandler(handler)
			mux.Handle(path, h)
			server := httptest.NewServer(mux)
			DeferCleanup(server.Close)

			daemon := &poolDaemon{wikiURL: server.URL}
			result = daemon.fetchPageContext(context.Background(), "test-page")
		})

		It("should include the chat preamble", func() {
			Expect(result).To(HavePrefix(chatPreamble))
		})

		It("should include the page name", func() {
			Expect(result).To(ContainSubstring("test-page"))
		})

		It("should include the context JSON", func() {
			Expect(result).To(ContainSubstring("We discussed testing"))
			Expect(result).To(ContainSubstring("Add more test coverage"))
		})

		It("should mention MergeFrontmatter for updates", func() {
			Expect(result).To(ContainSubstring("MergeFrontmatter"))
		})
	})

	When("the page has no ai_agent_chat_context", func() {
		var result string

		BeforeEach(func() {
			handler := &stubFrontmatterHandler{
				frontmatter: map[string]any{
					"title": "Test Page",
				},
			}
			mux := http.NewServeMux()
			path, h := apiv1connect.NewFrontmatterHandler(handler)
			mux.Handle(path, h)
			server := httptest.NewServer(mux)
			DeferCleanup(server.Close)

			daemon := &poolDaemon{wikiURL: server.URL}
			result = daemon.fetchPageContext(context.Background(), "test-page")
		})

		It("should include the chat preamble", func() {
			Expect(result).To(HavePrefix(chatPreamble))
		})

		It("should indicate no context exists yet", func() {
			Expect(result).To(ContainSubstring("No [ai_agent_chat_context] exists yet"))
		})

		It("should suggest creating one with MergeFrontmatter", func() {
			Expect(result).To(ContainSubstring("MergeFrontmatter"))
		})
	})

	When("the frontmatter service returns an error", func() {
		var result string

		BeforeEach(func() {
			handler := &stubFrontmatterHandler{
				err: connect.NewError(connect.CodeNotFound, nil),
			}
			mux := http.NewServeMux()
			path, h := apiv1connect.NewFrontmatterHandler(handler)
			mux.Handle(path, h)
			server := httptest.NewServer(mux)
			DeferCleanup(server.Close)

			daemon := &poolDaemon{wikiURL: server.URL}
			result = daemon.fetchPageContext(context.Background(), "test-page")
		})

		It("should include the chat preamble", func() {
			Expect(result).To(HavePrefix(chatPreamble))
		})

		It("should include a failure message", func() {
			Expect(result).To(ContainSubstring("Failed to fetch page context"))
		})

		It("should warn against modifying the context section", func() {
			Expect(result).To(ContainSubstring("Do NOT attempt to create or modify"))
		})
	})
})

var _ = Describe("InstanceState", func() {
	Describe("String", func() {
		When("called on each state", func() {
			It("should return Spawning for StateSpawning", func() {
				Expect(StateSpawning.String()).To(Equal("Spawning"))
			})

			It("should return Initializing for StateInitializing", func() {
				Expect(StateInitializing.String()).To(Equal("Initializing"))
			})

			It("should return BridgeConnecting for StateBridgeConnecting", func() {
				Expect(StateBridgeConnecting.String()).To(Equal("BridgeConnecting"))
			})

			It("should return Idle for StateIdle", func() {
				Expect(StateIdle.String()).To(Equal("Idle"))
			})

			It("should return Prompting for StatePrompting", func() {
				Expect(StatePrompting.String()).To(Equal("Prompting"))
			})

			It("should return PermissionPending for StatePermissionPending", func() {
				Expect(StatePermissionPending.String()).To(Equal("PermissionPending"))
			})

			It("should return Stopping for StateStopping", func() {
				Expect(StateStopping.String()).To(Equal("Stopping"))
			})

			It("should return Dead for StateDead", func() {
				Expect(StateDead.String()).To(Equal("Dead"))
			})
		})

		When("called on an unknown state value", func() {
			var result string

			BeforeEach(func() {
				result = InstanceState(99).String()
			})

			It("should return Unknown with the numeric value", func() {
				Expect(result).To(Equal("Unknown(99)"))
			})
		})
	})
})

var _ = Describe("buildPromptText", func() {
	When("the chat client has page context set", func() {
		var result string
		var client *wikiChatClient

		BeforeEach(func() {
			client = newWikiChatClient("test-page", "http://localhost:1")
			client.pageContext = "You are an assistant for page X."
			result = buildPromptText(client, "Hello there")
		})

		It("should prepend the page context to the message", func() {
			Expect(result).To(HavePrefix("You are an assistant for page X."))
		})

		It("should include the user message after the separator", func() {
			Expect(result).To(ContainSubstring("User message: Hello there"))
		})

		It("should include a separator between context and message", func() {
			Expect(result).To(ContainSubstring("---"))
		})

		It("should consume the page context so it is only prepended once", func() {
			Expect(client.pageContext).To(BeEmpty())
		})
	})

	When("the chat client has no page context", func() {
		var result string

		BeforeEach(func() {
			client := newWikiChatClient("test-page", "http://localhost:1")
			client.pageContext = ""
			result = buildPromptText(client, "Just a message")
		})

		It("should return only the message content", func() {
			Expect(result).To(Equal("Just a message"))
		})
	})

	When("called twice with page context initially set", func() {
		var firstResult string
		var secondResult string

		BeforeEach(func() {
			client := newWikiChatClient("test-page", "http://localhost:1")
			client.pageContext = "Some context"
			firstResult = buildPromptText(client, "First message")
			secondResult = buildPromptText(client, "Second message")
		})

		It("should include context in the first call", func() {
			Expect(firstResult).To(ContainSubstring("Some context"))
		})

		It("should not include context in the second call", func() {
			Expect(secondResult).To(Equal("Second message"))
		})
	})
})

var _ = Describe("streamOrCreateReply", func() {
	When("no current message exists (msgID is empty)", func() {
		var (
			client       *wikiChatClient
			receivedPath string
		)

		BeforeEach(func() {
			handler := &stubChatServiceHandler{
				sendReplyID: "new-msg-id",
			}
			mux := http.NewServeMux()
			path, h := apiv1connect.NewChatServiceHandler(handler)
			mux.Handle(path, h)
			server := httptest.NewServer(mux)
			DeferCleanup(server.Close)

			client = newWikiChatClient("test-page", server.URL)
			client.replyToID = "parent-msg"

			_ = receivedPath
			client.streamOrCreateReply("", "parent-msg", "Hello world")
		})

		It("should create a new message and store the message ID", func() {
			Expect(client.currentMsg).To(Equal("new-msg-id"))
		})
	})

	When("a current message already exists", func() {
		var (
			client  *wikiChatClient
			handler *stubChatServiceHandler
		)

		BeforeEach(func() {
			handler = &stubChatServiceHandler{
				sendReplyID: "should-not-be-used",
			}
			mux := http.NewServeMux()
			path, h := apiv1connect.NewChatServiceHandler(handler)
			mux.Handle(path, h)
			server := httptest.NewServer(mux)
			DeferCleanup(server.Close)

			client = newWikiChatClient("test-page", server.URL)
			client.currentMsg = "existing-msg"

			client.streamOrCreateReply("existing-msg", "parent-msg", "Updated text")
		})

		It("should not change the current message ID", func() {
			Expect(client.currentMsg).To(Equal("existing-msg"))
		})

		It("should have called EditChatMessage", func() {
			Expect(handler.editCalled).To(BeTrue())
		})

		It("should not have called SendChatReply", func() {
			Expect(handler.sendReplyCalled).To(BeFalse())
		})
	})

	When("creating a new message fails", func() {
		var client *wikiChatClient

		BeforeEach(func() {
			handler := &stubChatServiceHandler{
				sendReplyErr: connect.NewError(connect.CodeInternal, errors.New("server error")),
			}
			mux := http.NewServeMux()
			path, h := apiv1connect.NewChatServiceHandler(handler)
			mux.Handle(path, h)
			server := httptest.NewServer(mux)
			DeferCleanup(server.Close)

			client = newWikiChatClient("test-page", server.URL)
			client.streamOrCreateReply("", "parent-msg", "Hello")
		})

		It("should not set a current message ID", func() {
			Expect(client.currentMsg).To(BeEmpty())
		})
	})
})

var _ = Describe("buildAgentCmd", func() {
	When("systemd is disabled", func() {
		var daemon *poolDaemon
		var cmd *exec.Cmd

		BeforeEach(func() {
			daemon = &poolDaemon{
				agentPath:  "/usr/bin/my-agent",
				useSystemd: false,
			}
			cmd = daemon.buildAgentCmd(context.Background(), "test-page")
		})

		It("should use the agent path directly as the command", func() {
			Expect(cmd.Path).To(Equal("/usr/bin/my-agent"))
		})

		It("should not include systemd-run arguments", func() {
			Expect(cmd.Args).NotTo(ContainElement("systemd-run"))
		})

		It("should have stderr configured with a prefix writer", func() {
			Expect(cmd.Stderr).NotTo(BeNil())
		})
	})

	When("systemd is enabled", func() {
		var daemon *poolDaemon
		var cmd *exec.Cmd

		BeforeEach(func() {
			daemon = &poolDaemon{
				agentPath:  "/usr/bin/my-agent",
				useSystemd: true,
			}
			cmd = daemon.buildAgentCmd(context.Background(), "my/test page")
		})

		It("should use systemd-run as the command", func() {
			Expect(cmd.Args[0]).To(Equal("systemd-run"))
		})

		It("should include the --user flag", func() {
			Expect(cmd.Args).To(ContainElement("--user"))
		})

		It("should include the --scope flag", func() {
			Expect(cmd.Args).To(ContainElement("--scope"))
		})

		It("should include a sanitized unit name", func() {
			Expect(cmd.Args).To(ContainElement("--unit=wiki-chat-my-test-page"))
		})

		It("should include the agent path as the last argument", func() {
			Expect(cmd.Args[len(cmd.Args)-1]).To(Equal("/usr/bin/my-agent"))
		})
	})
})

var _ = Describe("chatPreamble", func() {
	It("should describe the interactive chat context", func() {
		Expect(chatPreamble).To(ContainSubstring("INTERACTIVE CHAT"))
	})

	It("should mention responding quickly", func() {
		Expect(chatPreamble).To(ContainSubstring("Respond quickly"))
	})

	It("should mention wiki MCP tools", func() {
		Expect(chatPreamble).To(ContainSubstring("wiki MCP tools"))
	})

	It("should mention MergeFrontmatter for memory updates", func() {
		Expect(chatPreamble).To(ContainSubstring("MergeFrontmatter"))
	})

	It("should describe the ai_agent_chat_context memory mechanism", func() {
		Expect(chatPreamble).To(ContainSubstring("ai_agent_chat_context"))
	})

	It("should mention the required memory fields", func() {
		Expect(chatPreamble).To(ContainSubstring("last_conversation_summary"))
		Expect(chatPreamble).To(ContainSubstring("user_goals"))
		Expect(chatPreamble).To(ContainSubstring("pending_items"))
		Expect(chatPreamble).To(ContainSubstring("key_context"))
		Expect(chatPreamble).To(ContainSubstring("last_updated"))
	})
})

var _ = Describe("validTransitions map", func() {
	It("should allow Spawning to transition to Initializing and Dead only", func() {
		Expect(validTransitions[StateSpawning]).To(ConsistOf(StateInitializing, StateDead))
	})

	It("should allow Initializing to transition to BridgeConnecting and Dead only", func() {
		Expect(validTransitions[StateInitializing]).To(ConsistOf(StateBridgeConnecting, StateDead))
	})

	It("should allow BridgeConnecting to transition to Idle and Dead only", func() {
		Expect(validTransitions[StateBridgeConnecting]).To(ConsistOf(StateIdle, StateDead))
	})

	It("should allow Idle to transition to Prompting and Stopping only", func() {
		Expect(validTransitions[StateIdle]).To(ConsistOf(StatePrompting, StateStopping))
	})

	It("should allow Prompting to transition to Idle, PermissionPending, Stopping, and Dead", func() {
		Expect(validTransitions[StatePrompting]).To(ConsistOf(StateIdle, StatePermissionPending, StateStopping, StateDead))
	})

	It("should allow PermissionPending to transition to Prompting, Stopping, and Dead", func() {
		Expect(validTransitions[StatePermissionPending]).To(ConsistOf(StatePrompting, StateStopping, StateDead))
	})

	It("should allow Stopping to transition to Dead only", func() {
		Expect(validTransitions[StateStopping]).To(ConsistOf(StateDead))
	})

	It("should not allow Dead to transition to any state", func() {
		Expect(validTransitions[StateDead]).To(BeEmpty())
	})

	It("should have entries for all defined states", func() {
		allStates := []InstanceState{
			StateSpawning, StateInitializing, StateBridgeConnecting,
			StateIdle, StatePrompting, StatePermissionPending,
			StateStopping, StateDead,
		}
		for _, s := range allStates {
			_, ok := validTransitions[s]
			Expect(ok).To(BeTrue(), "validTransitions should have an entry for %s", s)
		}
	})
})

var _ = Describe("permissionCancelledResponse", func() {
	When("called", func() {
		var resp acp.RequestPermissionResponse

		BeforeEach(func() {
			resp = permissionCancelledResponse()
		})

		It("should have a Cancelled outcome", func() {
			Expect(resp.Outcome.Cancelled).NotTo(BeNil())
		})

		It("should not have a Selected outcome", func() {
			Expect(resp.Outcome.Selected).To(BeNil())
		})
	})
})

var _ = Describe("permissionSelectedResponse", func() {
	When("called with an option ID", func() {
		var resp acp.RequestPermissionResponse

		BeforeEach(func() {
			resp = permissionSelectedResponse("opt-42")
		})

		It("should have a Selected outcome", func() {
			Expect(resp.Outcome.Selected).NotTo(BeNil())
		})

		It("should contain the correct option ID", func() {
			Expect(resp.Outcome.Selected.OptionId).To(Equal(acp.PermissionOptionId("opt-42")))
		})

		It("should not have a Cancelled outcome", func() {
			Expect(resp.Outcome.Cancelled).To(BeNil())
		})
	})
})

var _ = Describe("processPermissionResponse", func() {
	When("the selected ID is empty (user denied)", func() {
		var (
			client *wikiChatClient
			resp   acp.RequestPermissionResponse
			err    error
		)

		BeforeEach(func() {
			client = newWikiChatClient("test-page", "http://localhost:1")
			options := []acp.PermissionOption{
				{OptionId: "opt-1", Name: "Allow"},
			}
			resp, err = client.processPermissionResponse("", options, "Edit file")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a cancelled response", func() {
			Expect(resp.Outcome.Cancelled).NotTo(BeNil())
		})

		It("should record the denial in permission notes", func() {
			Expect(client.permissionNotes.String()).To(ContainSubstring("Permission denied"))
			Expect(client.permissionNotes.String()).To(ContainSubstring("Edit file"))
		})
	})

	When("the selected ID matches an option", func() {
		var (
			client *wikiChatClient
			resp   acp.RequestPermissionResponse
			err    error
		)

		BeforeEach(func() {
			client = newWikiChatClient("test-page", "http://localhost:1")
			options := []acp.PermissionOption{
				{OptionId: "opt-1", Name: "Allow Once"},
				{OptionId: "opt-2", Name: "Allow Always"},
			}
			resp, err = client.processPermissionResponse("opt-2", options, "Write file")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a selected response with the correct option", func() {
			Expect(resp.Outcome.Selected).NotTo(BeNil())
			Expect(resp.Outcome.Selected.OptionId).To(Equal(acp.PermissionOptionId("opt-2")))
		})

		It("should record the grant with the option name in permission notes", func() {
			Expect(client.permissionNotes.String()).To(ContainSubstring("Permission granted"))
			Expect(client.permissionNotes.String()).To(ContainSubstring("Allow Always"))
		})
	})

	When("the selected ID does not match any option", func() {
		var (
			client *wikiChatClient
			resp   acp.RequestPermissionResponse
			err    error
		)

		BeforeEach(func() {
			client = newWikiChatClient("test-page", "http://localhost:1")
			options := []acp.PermissionOption{
				{OptionId: "opt-1", Name: "Allow"},
			}
			resp, err = client.processPermissionResponse("unknown-opt", options, "Delete file")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a selected response with the unknown option ID", func() {
			Expect(resp.Outcome.Selected).NotTo(BeNil())
			Expect(resp.Outcome.Selected.OptionId).To(Equal(acp.PermissionOptionId("unknown-opt")))
		})

		It("should use the selected ID as the name in permission notes", func() {
			Expect(client.permissionNotes.String()).To(ContainSubstring("unknown-opt"))
		})
	})
})

var _ = Describe("wikiChatClient file system and terminal denials", func() {
	var client *wikiChatClient

	BeforeEach(func() {
		client = &wikiChatClient{page: "test-page"}
	})

	Describe("ReadTextFile", func() {
		When("called", func() {
			var err error

			BeforeEach(func() {
				_, err = client.ReadTextFile(context.Background(), acp.ReadTextFileRequest{})
			})

			It("should return an error about using wiki MCP tools", func() {
				Expect(err).To(MatchError(ContainSubstring("wiki MCP tools")))
			})
		})
	})

	Describe("WriteTextFile", func() {
		When("called", func() {
			var err error

			BeforeEach(func() {
				_, err = client.WriteTextFile(context.Background(), acp.WriteTextFileRequest{})
			})

			It("should return an error about using wiki MCP tools", func() {
				Expect(err).To(MatchError(ContainSubstring("wiki MCP tools")))
			})
		})
	})

	Describe("CreateTerminal", func() {
		When("called", func() {
			var err error

			BeforeEach(func() {
				_, err = client.CreateTerminal(context.Background(), acp.CreateTerminalRequest{})
			})

			It("should return an error about terminal access", func() {
				Expect(err).To(MatchError(ContainSubstring("terminal access not available")))
			})
		})
	})

	Describe("KillTerminalCommand", func() {
		When("called", func() {
			var err error

			BeforeEach(func() {
				_, err = client.KillTerminalCommand(context.Background(), acp.KillTerminalCommandRequest{})
			})

			It("should return an error about terminal access", func() {
				Expect(err).To(MatchError(ContainSubstring("terminal access not available")))
			})
		})
	})

	Describe("TerminalOutput", func() {
		When("called", func() {
			var err error

			BeforeEach(func() {
				_, err = client.TerminalOutput(context.Background(), acp.TerminalOutputRequest{})
			})

			It("should return an error about terminal access", func() {
				Expect(err).To(MatchError(ContainSubstring("terminal access not available")))
			})
		})
	})

	Describe("ReleaseTerminal", func() {
		When("called", func() {
			var err error

			BeforeEach(func() {
				_, err = client.ReleaseTerminal(context.Background(), acp.ReleaseTerminalRequest{})
			})

			It("should return an error about terminal access", func() {
				Expect(err).To(MatchError(ContainSubstring("terminal access not available")))
			})
		})
	})

	Describe("WaitForTerminalExit", func() {
		When("called", func() {
			var err error

			BeforeEach(func() {
				_, err = client.WaitForTerminalExit(context.Background(), acp.WaitForTerminalExitRequest{})
			})

			It("should return an error about terminal access", func() {
				Expect(err).To(MatchError(ContainSubstring("terminal access not available")))
			})
		})
	})
})

var _ = Describe("instanceEntry setState", func() {
	Describe("valid transitions", func() {
		When("transitioning from Spawning to Initializing", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StateSpawning}
				err = entry.setState(StateInitializing)
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("transitioning from Spawning to Dead", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StateSpawning}
				err = entry.setState(StateDead)
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("transitioning from Initializing to BridgeConnecting", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StateInitializing}
				err = entry.setState(StateBridgeConnecting)
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("transitioning from BridgeConnecting to Idle", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StateBridgeConnecting}
				err = entry.setState(StateIdle)
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("transitioning from Idle to Prompting", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StateIdle}
				err = entry.setState(StatePrompting)
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("transitioning from Idle to Stopping", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StateIdle}
				err = entry.setState(StateStopping)
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("transitioning from Prompting to Idle", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StatePrompting}
				err = entry.setState(StateIdle)
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("transitioning from Prompting to PermissionPending", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StatePrompting}
				err = entry.setState(StatePermissionPending)
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("transitioning from PermissionPending to Prompting", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StatePermissionPending}
				err = entry.setState(StatePrompting)
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("transitioning from Stopping to Dead", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StateStopping}
				err = entry.setState(StateDead)
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("invalid transitions", func() {
		When("transitioning from Dead to Idle", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StateDead}
				err = entry.setState(StateIdle)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid state transition: Dead -> Idle")))
			})
		})

		When("transitioning from Idle to Initializing", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StateIdle}
				err = entry.setState(StateInitializing)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid state transition: Idle -> Initializing")))
			})
		})

		When("transitioning from Spawning to Idle", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StateSpawning}
				err = entry.setState(StateIdle)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid state transition: Spawning -> Idle")))
			})
		})

		When("transitioning from Dead to Spawning", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StateDead}
				err = entry.setState(StateSpawning)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid state transition: Dead -> Spawning")))
			})
		})

		When("transitioning from BridgeConnecting to Prompting", func() {
			var err error

			BeforeEach(func() {
				entry := &instanceEntry{page: "test", state: StateBridgeConnecting}
				err = entry.setState(StatePrompting)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid state transition: BridgeConnecting -> Prompting")))
			})
		})
	})

	Describe("state updates correctly", func() {
		When("transitioning through a full lifecycle", func() {
			var entry *instanceEntry
			var errors []error

			BeforeEach(func() {
				entry = &instanceEntry{page: "lifecycle-test", state: StateSpawning}
				errors = nil
				errors = append(errors, entry.setState(StateInitializing))
				errors = append(errors, entry.setState(StateBridgeConnecting))
				errors = append(errors, entry.setState(StateIdle))
				errors = append(errors, entry.setState(StatePrompting))
				errors = append(errors, entry.setState(StatePermissionPending))
				errors = append(errors, entry.setState(StatePrompting))
				errors = append(errors, entry.setState(StateIdle))
				errors = append(errors, entry.setState(StateStopping))
				errors = append(errors, entry.setState(StateDead))
			})

			It("should complete all transitions without error", func() {
				for _, err := range errors {
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("should end in Dead state", func() {
				Expect(entry.State()).To(Equal(StateDead))
			})
		})
	})

	Describe("State", func() {
		When("called on an entry", func() {
			var entry *instanceEntry
			var state InstanceState

			BeforeEach(func() {
				entry = &instanceEntry{page: "test", state: StateIdle}
				state = entry.State()
			})

			It("should return the current state", func() {
				Expect(state).To(Equal(StateIdle))
			})
		})
	})
})

var _ = Describe("handleAgentMessage (mock-based)", func() {
	Describe("first chunk creates a reply", func() {
		When("no current message exists and text is non-empty", func() {
			var (
				client *wikiChatClient
				mock   *mockChatReplier
				err    error
			)

			BeforeEach(func() {
				mock = &mockChatReplier{
					sendReplyResp: &apiv1.SendChatReplyResponse{MessageId: "new-msg-42"},
				}
				client = &wikiChatClient{
					page:       "test-page",
					chatClient: mock,
					replyToID:  "parent-1",
				}

				chunk := &acp.SessionUpdateAgentMessageChunk{
					Content: acp.TextBlock("Hello from agent"),
				}
				err = client.handleAgentMessage(chunk)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should call SendChatReply", func() {
				Expect(mock.sendReplyCalled).To(BeTrue())
			})

			It("should send the correct page", func() {
				Expect(mock.sendReplyReq.Page).To(Equal("test-page"))
			})

			It("should send the correct reply-to ID", func() {
				Expect(mock.sendReplyReq.ReplyToId).To(Equal("parent-1"))
			})

			It("should store the new message ID", func() {
				Expect(client.currentMsg).To(Equal("new-msg-42"))
			})

			It("should not call EditChatMessage", func() {
				Expect(mock.editCalled).To(BeFalse())
			})
		})
	})

	Describe("subsequent chunks edit the existing message", func() {
		When("a current message already exists", func() {
			var (
				client *wikiChatClient
				mock   *mockChatReplier
				err    error
			)

			BeforeEach(func() {
				mock = &mockChatReplier{}
				client = &wikiChatClient{
					page:       "test-page",
					chatClient: mock,
					currentMsg: "existing-msg-1",
				}
				client.textBuf.WriteString("Previous text. ")

				chunk := &acp.SessionUpdateAgentMessageChunk{
					Content: acp.TextBlock("More text"),
				}
				err = client.handleAgentMessage(chunk)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should call EditChatMessage", func() {
				Expect(mock.editCalled).To(BeTrue())
			})

			It("should edit the correct message", func() {
				Expect(mock.editReq.MessageId).To(Equal("existing-msg-1"))
			})

			It("should set streaming to true", func() {
				Expect(mock.editReq.Streaming).To(BeTrue())
			})

			It("should not call SendChatReply", func() {
				Expect(mock.sendReplyCalled).To(BeFalse())
			})

			It("should accumulate the text", func() {
				Expect(client.textBuf.String()).To(Equal("Previous text. More text"))
			})
		})
	})

	Describe("whitespace-only text is skipped", func() {
		When("the chunk contains only whitespace", func() {
			var (
				client *wikiChatClient
				mock   *mockChatReplier
				err    error
			)

			BeforeEach(func() {
				mock = &mockChatReplier{}
				client = &wikiChatClient{
					page:       "test-page",
					chatClient: mock,
				}

				chunk := &acp.SessionUpdateAgentMessageChunk{
					Content: acp.TextBlock("   \n  "),
				}
				err = client.handleAgentMessage(chunk)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not call SendChatReply", func() {
				Expect(mock.sendReplyCalled).To(BeFalse())
			})

			It("should not call EditChatMessage", func() {
				Expect(mock.editCalled).To(BeFalse())
			})
		})
	})

	Describe("non-text content is skipped", func() {
		When("the chunk has no text field", func() {
			var (
				client *wikiChatClient
				mock   *mockChatReplier
				err    error
			)

			BeforeEach(func() {
				mock = &mockChatReplier{}
				client = &wikiChatClient{
					page:       "test-page",
					chatClient: mock,
				}

				chunk := &acp.SessionUpdateAgentMessageChunk{
					Content: acp.ImageBlock("base64data", "image/png"),
				}
				err = client.handleAgentMessage(chunk)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not call any chat methods", func() {
				Expect(mock.sendReplyCalled).To(BeFalse())
				Expect(mock.editCalled).To(BeFalse())
			})
		})
	})
})

var _ = Describe("handleThought (mock-based)", func() {
	Describe("non-empty thought text", func() {
		When("there is no current message", func() {
			var (
				client *wikiChatClient
				mock   *mockChatReplier
			)

			BeforeEach(func() {
				mock = &mockChatReplier{
					sendReplyResp: &apiv1.SendChatReplyResponse{MessageId: "thought-msg-1"},
				}
				client = &wikiChatClient{
					page:       "test-page",
					chatClient: mock,
					replyToID:  "parent-1",
				}

				chunk := &acp.SessionUpdateAgentThoughtChunk{
					Content: acp.TextBlock("Let me think..."),
				}
				client.handleThought(chunk)
			})

			It("should accumulate the thought text", func() {
				Expect(client.thoughtBuf.String()).To(Equal("Let me think..."))
			})

			It("should call SendChatReply to create a new message", func() {
				Expect(mock.sendReplyCalled).To(BeTrue())
			})

			It("should store the message ID", func() {
				Expect(client.currentMsg).To(Equal("thought-msg-1"))
			})
		})
	})

	Describe("empty thought is skipped", func() {
		When("the thought text is nil", func() {
			var (
				client *wikiChatClient
				mock   *mockChatReplier
			)

			BeforeEach(func() {
				mock = &mockChatReplier{}
				client = &wikiChatClient{
					page:       "test-page",
					chatClient: mock,
				}

				chunk := &acp.SessionUpdateAgentThoughtChunk{
					Content: acp.ImageBlock("data", "image/png"),
				}
				client.handleThought(chunk)
			})

			It("should not call any chat methods", func() {
				Expect(mock.sendReplyCalled).To(BeFalse())
				Expect(mock.editCalled).To(BeFalse())
			})

			It("should not accumulate any text", func() {
				Expect(client.thoughtBuf.String()).To(BeEmpty())
			})
		})
	})

	Describe("whitespace-only thought is skipped", func() {
		When("the thought text is only whitespace", func() {
			var (
				client *wikiChatClient
				mock   *mockChatReplier
			)

			BeforeEach(func() {
				mock = &mockChatReplier{}
				client = &wikiChatClient{
					page:       "test-page",
					chatClient: mock,
				}

				chunk := &acp.SessionUpdateAgentThoughtChunk{
					Content: acp.TextBlock("   "),
				}
				client.handleThought(chunk)
			})

			It("should not call any chat methods", func() {
				Expect(mock.sendReplyCalled).To(BeFalse())
				Expect(mock.editCalled).To(BeFalse())
			})
		})
	})
})

var _ = Describe("handleToolCall (mock-based)", func() {
	When("a current message exists", func() {
		var (
			mock *mockChatReplier
		)

		BeforeEach(func() {
			mock = &mockChatReplier{}
			client := &wikiChatClient{
				page:       "test-page",
				chatClient: mock,
				currentMsg: "msg-99",
			}

			tc := &acp.SessionUpdateToolCall{
				ToolCallId: "tc-1",
				Title:      "Read file",
				Status:     acp.ToolCallStatusInProgress,
			}
			client.handleToolCall(tc)
		})

		It("should call SendToolCallNotification", func() {
			Expect(mock.toolNotifyCalled).To(BeTrue())
		})

		It("should send the correct page", func() {
			Expect(mock.toolNotifyReq.Page).To(Equal("test-page"))
		})

		It("should send the correct message ID", func() {
			Expect(mock.toolNotifyReq.MessageId).To(Equal("msg-99"))
		})

		It("should send the correct tool call ID", func() {
			Expect(mock.toolNotifyReq.ToolCallId).To(Equal("tc-1"))
		})

		It("should send the correct title", func() {
			Expect(mock.toolNotifyReq.Title).To(Equal("Read file"))
		})
	})

	When("no current message exists", func() {
		var mock *mockChatReplier

		BeforeEach(func() {
			mock = &mockChatReplier{}
			client := &wikiChatClient{
				page:       "test-page",
				chatClient: mock,
			}

			tc := &acp.SessionUpdateToolCall{
				ToolCallId: "tc-1",
				Title:      "Read file",
				Status:     acp.ToolCallStatusInProgress,
			}
			client.handleToolCall(tc)
		})

		It("should not call SendToolCallNotification", func() {
			Expect(mock.toolNotifyCalled).To(BeFalse())
		})
	})
})

var _ = Describe("handleToolCallUpdate (mock-based)", func() {
	When("a current message exists and status/title are set", func() {
		var mock *mockChatReplier

		BeforeEach(func() {
			mock = &mockChatReplier{}
			client := &wikiChatClient{
				page:       "test-page",
				chatClient: mock,
				currentMsg: "msg-77",
			}

			status := acp.ToolCallStatusCompleted
			title := "Write file"
			tcu := &acp.SessionToolCallUpdate{
				ToolCallId: "tc-2",
				Status:     &status,
				Title:      &title,
			}
			client.handleToolCallUpdate(tcu)
		})

		It("should call SendToolCallNotification", func() {
			Expect(mock.toolNotifyCalled).To(BeTrue())
		})

		It("should send the correct title", func() {
			Expect(mock.toolNotifyReq.Title).To(Equal("Write file"))
		})

		It("should send the correct status", func() {
			Expect(mock.toolNotifyReq.Status).To(Equal(string(acp.ToolCallStatusCompleted)))
		})
	})

	When("no current message exists", func() {
		var mock *mockChatReplier

		BeforeEach(func() {
			mock = &mockChatReplier{}
			client := &wikiChatClient{
				page:       "test-page",
				chatClient: mock,
			}

			tcu := &acp.SessionToolCallUpdate{
				ToolCallId: "tc-2",
			}
			client.handleToolCallUpdate(tcu)
		})

		It("should not call SendToolCallNotification", func() {
			Expect(mock.toolNotifyCalled).To(BeFalse())
		})
	})

	When("status and title are nil", func() {
		var mock *mockChatReplier

		BeforeEach(func() {
			mock = &mockChatReplier{}
			client := &wikiChatClient{
				page:       "test-page",
				chatClient: mock,
				currentMsg: "msg-77",
			}

			tcu := &acp.SessionToolCallUpdate{
				ToolCallId: "tc-3",
			}
			client.handleToolCallUpdate(tcu)
		})

		It("should call SendToolCallNotification with empty strings", func() {
			Expect(mock.toolNotifyCalled).To(BeTrue())
			Expect(mock.toolNotifyReq.Title).To(BeEmpty())
			Expect(mock.toolNotifyReq.Status).To(BeEmpty())
		})
	})
})

var _ = Describe("streamOrCreateReply (mock-based)", func() {
	When("no current message exists", func() {
		var (
			client *wikiChatClient
			mock   *mockChatReplier
		)

		BeforeEach(func() {
			mock = &mockChatReplier{
				sendReplyResp: &apiv1.SendChatReplyResponse{MessageId: "new-msg-77"},
			}
			client = &wikiChatClient{
				page:       "test-page",
				chatClient: mock,
			}
			client.streamOrCreateReply("", "parent-1", "Hello")
		})

		It("should call SendChatReply", func() {
			Expect(mock.sendReplyCalled).To(BeTrue())
		})

		It("should store the returned message ID", func() {
			Expect(client.currentMsg).To(Equal("new-msg-77"))
		})

		It("should send the correct content", func() {
			Expect(mock.sendReplyReq.Content).To(Equal("Hello"))
		})

		It("should send the correct reply-to ID", func() {
			Expect(mock.sendReplyReq.ReplyToId).To(Equal("parent-1"))
		})
	})

	When("a current message exists", func() {
		var mock *mockChatReplier

		BeforeEach(func() {
			mock = &mockChatReplier{}
			client := &wikiChatClient{
				page:       "test-page",
				chatClient: mock,
				currentMsg: "existing-msg",
			}
			client.streamOrCreateReply("existing-msg", "parent-1", "Updated")
		})

		It("should call EditChatMessage", func() {
			Expect(mock.editCalled).To(BeTrue())
		})

		It("should not call SendChatReply", func() {
			Expect(mock.sendReplyCalled).To(BeFalse())
		})

		It("should send the correct message ID to edit", func() {
			Expect(mock.editReq.MessageId).To(Equal("existing-msg"))
		})

		It("should set streaming to true", func() {
			Expect(mock.editReq.Streaming).To(BeTrue())
		})
	})

	When("SendChatReply fails", func() {
		var (
			client *wikiChatClient
			mock   *mockChatReplier
		)

		BeforeEach(func() {
			mock = &mockChatReplier{
				sendReplyErr: errors.New("connection refused"),
			}
			client = &wikiChatClient{
				page:       "test-page",
				chatClient: mock,
			}
			client.streamOrCreateReply("", "parent-1", "Hello")
		})

		It("should not store a message ID", func() {
			Expect(client.currentMsg).To(BeEmpty())
		})
	})
})

var _ = Describe("RequestPermission (mock-based)", func() {
	When("the permission request has no options", func() {
		var (
			client *wikiChatClient
			resp   acp.RequestPermissionResponse
			err    error
		)

		BeforeEach(func() {
			mock := &mockChatReplier{}
			client = &wikiChatClient{
				page:       "test-page",
				chatClient: mock,
			}

			req := acp.RequestPermissionRequest{
				ToolCall: acp.RequestPermissionToolCall{
					ToolCallId: "tc-1",
				},
				Options: []acp.PermissionOption{},
			}
			resp, err = client.RequestPermission(context.Background(), req)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a cancelled response", func() {
			Expect(resp.Outcome.Cancelled).NotTo(BeNil())
		})
	})

	When("the user grants permission", func() {
		var (
			client *wikiChatClient
			mock   *mockChatReplier
			resp   acp.RequestPermissionResponse
			err    error
		)

		BeforeEach(func() {
			mock = &mockChatReplier{
				permResp: &apiv1.RequestPermissionFromUserResponse{
					SelectedOptionId: "opt-allow",
				},
			}
			client = &wikiChatClient{
				page:       "test-page",
				chatClient: mock,
				entry: &instanceEntry{
					page:  "test-page",
					state: StatePrompting,
				},
			}

			title := "Edit file.txt"
			req := acp.RequestPermissionRequest{
				ToolCall: acp.RequestPermissionToolCall{
					ToolCallId: "tc-1",
					Title:      &title,
				},
				Options: []acp.PermissionOption{
					{OptionId: "opt-allow", Name: "Allow"},
					{OptionId: "opt-deny", Name: "Deny"},
				},
			}
			resp, err = client.RequestPermission(context.Background(), req)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call RequestPermissionFromUser", func() {
			Expect(mock.permCalled).To(BeTrue())
		})

		It("should forward the page name", func() {
			Expect(mock.permReq.Page).To(Equal("test-page"))
		})

		It("should forward the title", func() {
			Expect(mock.permReq.Title).To(Equal("Edit file.txt"))
		})

		It("should forward the options", func() {
			Expect(mock.permReq.Options).To(HaveLen(2))
			Expect(mock.permReq.Options[0].OptionId).To(Equal("opt-allow"))
			Expect(mock.permReq.Options[0].Label).To(Equal("Allow"))
		})

		It("should return a selected response with the chosen option", func() {
			Expect(resp.Outcome.Selected).NotTo(BeNil())
			Expect(resp.Outcome.Selected.OptionId).To(Equal(acp.PermissionOptionId("opt-allow")))
		})

		It("should record the grant in permission notes", func() {
			Expect(client.permissionNotes.String()).To(ContainSubstring("Permission granted"))
			Expect(client.permissionNotes.String()).To(ContainSubstring("Allow"))
		})

		It("should transition state to PermissionPending and back", func() {
			// After the deferred setState back to Prompting, the entry should be Prompting
			Expect(client.entry.State()).To(Equal(StatePrompting))
		})
	})

	When("the user denies permission (empty selectedOptionId)", func() {
		var (
			client *wikiChatClient
			resp   acp.RequestPermissionResponse
			err    error
		)

		BeforeEach(func() {
			mock := &mockChatReplier{
				permResp: &apiv1.RequestPermissionFromUserResponse{
					SelectedOptionId: "",
				},
			}
			client = &wikiChatClient{
				page:       "test-page",
				chatClient: mock,
				entry: &instanceEntry{
					page:  "test-page",
					state: StatePrompting,
				},
			}

			title := "Delete data"
			req := acp.RequestPermissionRequest{
				ToolCall: acp.RequestPermissionToolCall{
					ToolCallId: "tc-2",
					Title:      &title,
				},
				Options: []acp.PermissionOption{
					{OptionId: "opt-allow", Name: "Allow"},
				},
			}
			resp, err = client.RequestPermission(context.Background(), req)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a cancelled response", func() {
			Expect(resp.Outcome.Cancelled).NotTo(BeNil())
		})

		It("should record the denial in permission notes", func() {
			Expect(client.permissionNotes.String()).To(ContainSubstring("Permission denied"))
			Expect(client.permissionNotes.String()).To(ContainSubstring("Delete data"))
		})
	})

	When("the wiki server returns an error", func() {
		var (
			resp acp.RequestPermissionResponse
			err  error
		)

		BeforeEach(func() {
			mock := &mockChatReplier{
				permErr: errors.New("server unavailable"),
			}
			client := &wikiChatClient{
				page:       "test-page",
				chatClient: mock,
			}

			title := "Write file"
			req := acp.RequestPermissionRequest{
				ToolCall: acp.RequestPermissionToolCall{
					ToolCallId: "tc-3",
					Title:      &title,
				},
				Options: []acp.PermissionOption{
					{OptionId: "opt-allow", Name: "Allow"},
					{OptionId: "opt-deny", Name: "Deny"},
				},
			}
			resp, err = client.RequestPermission(context.Background(), req)
		})

		It("should not return an error (auto-approves)", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should auto-approve with the first option", func() {
			Expect(resp.Outcome.Selected).NotTo(BeNil())
			Expect(resp.Outcome.Selected.OptionId).To(Equal(acp.PermissionOptionId("opt-allow")))
		})
	})
})

var _ = Describe("forwardUserMessage (mock-based)", func() {
	When("forwarding a user message to the agent", func() {
		var (
			mockAgent *mockACPAgent
			mockChat  *mockChatReplier
			entry     *instanceEntry
			client    *wikiChatClient
		)

		BeforeEach(func() {
			mockAgent = &mockACPAgent{}
			mockChat = &mockChatReplier{
				sendReplyResp: &apiv1.SendChatReplyResponse{MessageId: "reply-1"},
			}

			entry = &instanceEntry{
				page:       "test-page",
				conn:       mockAgent,
				sessionID:  "session-1",
				lastActive: time.Now().Add(-5 * time.Minute),
				state:      StateIdle,
			}

			client = &wikiChatClient{
				page:       "test-page",
				chatClient: mockChat,
			}

			cancelChan := make(chan struct{})
			msg := &apiv1.ChatMessage{
				Id:      "user-msg-1",
				Content: "What is this page about?",
				Sender:  apiv1.Sender_USER,
			}

			forwardUserMessage(context.Background(), entry, client, cancelChan, msg)
		})

		It("should call Prompt on the ACP agent", func() {
			Expect(mockAgent.promptCalled).To(BeTrue())
		})

		It("should send the correct session ID", func() {
			Expect(mockAgent.promptReq.SessionId).To(Equal(acp.SessionId("session-1")))
		})

		It("should include the user message content in the prompt", func() {
			Expect(mockAgent.promptReq.Prompt).To(HaveLen(1))
			Expect(mockAgent.promptReq.Prompt[0].Text).NotTo(BeNil())
			Expect(mockAgent.promptReq.Prompt[0].Text.Text).To(ContainSubstring("What is this page about?"))
		})

		It("should update lastActive on the entry", func() {
			Expect(entry.idleSince()).To(BeNumerically("<", time.Second))
		})

		It("should clear the current message after the turn ends", func() {
			Expect(client.currentMsg).To(BeEmpty())
		})

		It("should transition back to Idle after prompting", func() {
			Expect(entry.State()).To(Equal(StateIdle))
		})
	})

	When("the agent prompt fails", func() {
		var (
			mockAgent *mockACPAgent
			entry     *instanceEntry
		)

		BeforeEach(func() {
			mockAgent = &mockACPAgent{
				promptErr: errors.New("agent crashed"),
			}
			mockChat := &mockChatReplier{}

			entry = &instanceEntry{
				page:       "test-page",
				conn:       mockAgent,
				sessionID:  "session-1",
				lastActive: time.Now(),
				state:      StateIdle,
			}

			client := &wikiChatClient{
				page:       "test-page",
				chatClient: mockChat,
			}

			cancelChan := make(chan struct{})
			msg := &apiv1.ChatMessage{
				Id:      "user-msg-2",
				Content: "Do something",
				Sender:  apiv1.Sender_USER,
			}

			forwardUserMessage(context.Background(), entry, client, cancelChan, msg)
		})

		It("should still transition back to Idle", func() {
			Expect(entry.State()).To(Equal(StateIdle))
		})
	})

	When("page context is available for the first message", func() {
		var (
			mockAgent *mockACPAgent
		)

		BeforeEach(func() {
			mockAgent = &mockACPAgent{}
			mockChat := &mockChatReplier{
				sendReplyResp: &apiv1.SendChatReplyResponse{MessageId: "reply-1"},
			}

			entry := &instanceEntry{
				page:       "test-page",
				conn:       mockAgent,
				sessionID:  "session-1",
				lastActive: time.Now(),
				state:      StateIdle,
			}

			client := &wikiChatClient{
				page:        "test-page",
				chatClient:  mockChat,
				pageContext: "You are the assistant for page X.",
			}

			cancelChan := make(chan struct{})
			msg := &apiv1.ChatMessage{
				Id:      "user-msg-3",
				Content: "Hello",
				Sender:  apiv1.Sender_USER,
			}

			forwardUserMessage(context.Background(), entry, client, cancelChan, msg)
		})

		It("should include the page context in the prompt", func() {
			Expect(mockAgent.promptReq.Prompt[0].Text.Text).To(ContainSubstring("You are the assistant for page X."))
		})

		It("should include the user message after the context", func() {
			Expect(mockAgent.promptReq.Prompt[0].Text.Text).To(ContainSubstring("User message: Hello"))
		})
	})
})

var _ = Describe("interface compliance", func() {
	It("should satisfy chatReplier with the connect ChatServiceClient", func() {
		var _ chatReplier = (apiv1connect.ChatServiceClient)(nil)
	})

	It("should satisfy pageMessageSource with the connect ChatServiceClient", func() {
		var _ pageMessageSource = (apiv1connect.ChatServiceClient)(nil)
	})

	It("should satisfy acpAgent with *acp.ClientSideConnection", func() {
		var _ acpAgent = (*acp.ClientSideConnection)(nil)
	})
})
