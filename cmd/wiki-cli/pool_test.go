package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
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
							cancel:     func() {},
						},
						"old-page": {
							page:       "old-page",
							lastActive: time.Now().Add(-30 * time.Minute),
							cancel:     func() {},
						},
						"mid-page": {
							page:       "mid-page",
							lastActive: time.Now().Add(-10 * time.Minute),
							cancel:     func() {},
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

	Describe("reapIdleInstances", func() {
		When("an instance exceeds idle timeout", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					idleTimeout: 10 * time.Minute,
					instances: map[string]*instanceEntry{
						"idle-page": {
							page:       "idle-page",
							lastActive: time.Now().Add(-20 * time.Minute),
							cancel:     func() {},
						},
						"active-page": {
							page:       "active-page",
							lastActive: time.Now(),
							cancel:     func() {},
						},
					},
				}

				// Simulate one reaper tick
				daemon.mu.Lock()
				for page, entry := range daemon.instances {
					if entry.idleSince() > daemon.idleTimeout {
						daemon.stopInstanceLocked(page)
					}
				}
				daemon.mu.Unlock()
			})

			It("should reap the idle instance", func() {
				Expect(daemon.instances).NotTo(HaveKey("idle-page"))
			})

			It("should keep the active instance", func() {
				Expect(daemon.instances).To(HaveKey("active-page"))
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

			It("should set the daemon context", func() {
				Expect(daemon.ctx).NotTo(BeNil())
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
