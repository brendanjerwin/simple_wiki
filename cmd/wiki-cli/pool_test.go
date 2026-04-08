package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	acp "github.com/coder/acp-go-sdk"

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
