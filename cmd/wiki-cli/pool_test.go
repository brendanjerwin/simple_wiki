package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cli "gopkg.in/urfave/cli.v1"
)

// fakeCmd is a test double satisfying commandStarter (same interface as *exec.Cmd).
type fakeCmd struct {
	startErr error
	waitCh   chan struct{}
}

func (f *fakeCmd) Start() error { return f.startErr }

func (f *fakeCmd) Wait() error {
	<-f.waitCh
	return nil
}

// fakeCommandTracker tracks commands created by the fake builder and allows cleanup.
type fakeCommandTracker struct {
	shouldFailStart bool
	cmds            []*fakeCmd
}

func (t *fakeCommandTracker) builder(_ context.Context, _ string, _ ...string) commandStarter {
	cmd := &fakeCmd{waitCh: make(chan struct{})}
	if t.shouldFailStart {
		cmd.startErr = errors.New("start failed")
	}
	t.cmds = append(t.cmds, cmd)
	return cmd
}

func (t *fakeCommandTracker) cleanup() {
	for _, c := range t.cmds {
		select {
		case <-c.waitCh:
		default:
			close(c.waitCh)
		}
	}
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

	It("should include the expected flags", func() {
		Expect(cmd.Flags).To(HaveLen(5))
	})

	It("should have a non-nil action", func() {
		Expect(cmd.Action).NotTo(BeNil())
	})
})

var _ = Describe("buildDirectClaudeArgs", func() {
	var (
		binary   string
		args     []string
		unitName string
	)

	BeforeEach(func() {
		binary, args, unitName = buildDirectClaudeArgs("/usr/bin/claude", "/usr/bin/wiki-cli mcp --url http://localhost --page test")
	})

	It("should use the claude path as binary", func() {
		Expect(binary).To(Equal("/usr/bin/claude"))
	})

	It("should include --channels and --mcp-server", func() {
		Expect(args).To(ContainElement("--channels"))
		Expect(args).To(ContainElement("--mcp-server"))
	})

	It("should return empty unit name", func() {
		Expect(unitName).To(BeEmpty())
	})
})

var _ = Describe("buildSystemdClaudeArgs", func() {
	var (
		binary   string
		args     []string
		unitName string
	)

	BeforeEach(func() {
		binary, args, unitName = buildSystemdClaudeArgs("/usr/bin/claude", "/usr/bin/wiki-cli mcp --url http://localhost --page my_page", "my_page")
	})

	It("should use systemd-run as binary", func() {
		Expect(binary).To(Equal("systemd-run"))
	})

	It("should include --user and --scope", func() {
		Expect(args).To(ContainElement("--user"))
		Expect(args).To(ContainElement("--scope"))
	})

	It("should return a sanitized unit name", func() {
		Expect(unitName).To(Equal("wiki-chat-my-page"))
	})
})

var _ = Describe("buildMCPServerArg", func() {
	var result string

	BeforeEach(func() {
		result = buildMCPServerArg("/usr/bin/wiki-cli", "http://localhost:8050", "my_page")
	})

	It("should build the correct command string", func() {
		Expect(result).To(Equal("/usr/bin/wiki-cli mcp --url http://localhost:8050 --page my_page"))
	})
})

var _ = Describe("instanceEntry", func() {
	Describe("touch", func() {
		When("called", func() {
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
	Describe("ensureInstance", func() {
		When("an instance already exists for the page", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				tracker := &fakeCommandTracker{}
				daemon = &poolDaemon{
					maxInstances: 5,
					newCommand:   tracker.builder,
					instances: map[string]*instanceEntry{
						"existing-page": {
							page:       "existing-page",
							lastActive: time.Now().Add(-10 * time.Minute),
						},
					},
				}
				daemon.ensureInstance(context.Background(), "existing-page")
			})

			It("should update lastActive instead of spawning", func() {
				Expect(daemon.instances["existing-page"].lastActive).To(BeTemporally("~", time.Now(), time.Second))
			})

			It("should not change the instance count", func() {
				Expect(daemon.instances).To(HaveLen(1))
			})
		})

		When("at max capacity and spawn fails", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					maxInstances: 2,
					newCommand:   (&fakeCommandTracker{shouldFailStart: true}).builder,
					instances: map[string]*instanceEntry{
						"page-a": {
							page:       "page-a",
							lastActive: time.Now().Add(-20 * time.Minute),
							cancel: func() {
								// no-op: satisfies context.CancelFunc for testing
							},
						},
						"page-b": {
							page:       "page-b",
							lastActive: time.Now().Add(-5 * time.Minute),
							cancel: func() {
								// no-op: satisfies context.CancelFunc for testing
							},
						},
					},
				}
				daemon.ensureInstance(context.Background(), "page-c")
			})

			It("should not evict any instance when spawn fails", func() {
				Expect(daemon.instances).To(HaveKey("page-a"))
				Expect(daemon.instances).To(HaveKey("page-b"))
			})

			It("should not add the failed instance", func() {
				Expect(daemon.instances).NotTo(HaveKey("page-c"))
			})

			It("should preserve capacity", func() {
				Expect(daemon.instances).To(HaveLen(2))
			})
		})

		When("at max capacity and spawn succeeds", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				tracker := &fakeCommandTracker{}
				DeferCleanup(tracker.cleanup)
				daemon = &poolDaemon{
					maxInstances: 2,
					newCommand:   tracker.builder,
					instances: map[string]*instanceEntry{
						"page-a": {
							page:       "page-a",
							lastActive: time.Now().Add(-20 * time.Minute),
							cancel: func() {
								// no-op: satisfies context.CancelFunc for testing
							},
						},
						"page-b": {
							page:       "page-b",
							lastActive: time.Now().Add(-5 * time.Minute),
							cancel: func() {
								// no-op: satisfies context.CancelFunc for testing
							},
						},
					},
				}
				daemon.ensureInstance(context.Background(), "page-c")
			})

			It("should evict the least recently active instance", func() {
				Expect(daemon.instances).NotTo(HaveKey("page-a"))
			})

			It("should add the new instance", func() {
				Expect(daemon.instances).To(HaveKey("page-c"))
			})

			It("should keep the more recently active instance", func() {
				Expect(daemon.instances).To(HaveKey("page-b"))
			})
		})

		When("spawning a new instance with available capacity", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				tracker := &fakeCommandTracker{}
				DeferCleanup(tracker.cleanup)
				daemon = &poolDaemon{
					maxInstances: 5,
					newCommand:   tracker.builder,
					instances:    make(map[string]*instanceEntry),
				}
				daemon.ensureInstance(context.Background(), "new-page")
			})

			It("should add the instance", func() {
				Expect(daemon.instances).To(HaveKey("new-page"))
			})
		})
	})

	Describe("spawnInstance", func() {
		When("command starts successfully", func() {
			var (
				entry *instanceEntry
				err   error
			)

			BeforeEach(func() {
				tracker := &fakeCommandTracker{}
				DeferCleanup(tracker.cleanup)
				daemon := &poolDaemon{
					newCommand: tracker.builder,
					instances:  make(map[string]*instanceEntry),
				}
				daemon.mu.Lock()
				entry, err = daemon.spawnInstance(context.Background(), "test-page")
				daemon.mu.Unlock()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an entry with the page name", func() {
				Expect(entry.page).To(Equal("test-page"))
			})

			It("should set lastActive to near now", func() {
				Expect(entry.lastActive).To(BeTemporally("~", time.Now(), time.Second))
			})

			It("should have a cancel function", func() {
				Expect(entry.cancel).NotTo(BeNil())
			})
		})

		When("command fails to start", func() {
			var err error

			BeforeEach(func() {
				daemon := &poolDaemon{
					newCommand: (&fakeCommandTracker{shouldFailStart: true}).builder,
					instances:  make(map[string]*instanceEntry),
				}
				daemon.mu.Lock()
				_, err = daemon.spawnInstance(context.Background(), "test-page")
				daemon.mu.Unlock()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("evictLeastActive", func() {
		When("multiple instances exist", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					newCommand: (&fakeCommandTracker{}).builder,
					instances: map[string]*instanceEntry{
						"new-page": {
							page:       "new-page",
							lastActive: time.Now(),
							cancel: func() {
								// no-op: satisfies context.CancelFunc for testing
							},
						},
						"old-page": {
							page:       "old-page",
							lastActive: time.Now().Add(-30 * time.Minute),
							cancel: func() {
								// no-op: satisfies context.CancelFunc for testing
							},
						},
						"mid-page": {
							page:       "mid-page",
							lastActive: time.Now().Add(-10 * time.Minute),
							cancel: func() {
								// no-op: satisfies context.CancelFunc for testing
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
					newCommand: (&fakeCommandTracker{}).builder,
					instances:  make(map[string]*instanceEntry),
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
		When("instance has a systemd unit name", func() {
			var (
				daemon  *poolDaemon
				tracker *fakeCommandTracker
				canceled bool
			)

			BeforeEach(func() {
				canceled = false
				tracker = &fakeCommandTracker{}
				DeferCleanup(tracker.cleanup)
				daemon = &poolDaemon{
					newCommand: tracker.builder,
					instances: map[string]*instanceEntry{
						"page-a": {
							page:     "page-a",
							unitName: "wiki-chat-page-a",
							cancel:   func() { canceled = true },
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

			It("should invoke systemctl stop via newCommand", func() {
				Expect(tracker.cmds).To(HaveLen(1))
			})

			It("should remove the instance", func() {
				Expect(daemon.instances).NotTo(HaveKey("page-a"))
			})
		})

		When("instance does not exist", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					newCommand: (&fakeCommandTracker{}).builder,
					instances:  make(map[string]*instanceEntry),
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
				daemon   *poolDaemon
				canceled bool
			)

			BeforeEach(func() {
				canceled = false
				daemon = &poolDaemon{
					newCommand: (&fakeCommandTracker{}).builder,
					instances: map[string]*instanceEntry{
						"page-a": {
							page:   "page-a",
							cancel: func() { canceled = true },
						},
					},
				}
				daemon.stopAll()
			})

			It("should cancel all instances", func() {
				Expect(canceled).To(BeTrue())
			})

			It("should clear the instances map", func() {
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
					newCommand:  (&fakeCommandTracker{}).builder,
					instances: map[string]*instanceEntry{
						"idle-page": {
							page:       "idle-page",
							lastActive: time.Now().Add(-20 * time.Minute),
							cancel: func() {
								// no-op: satisfies context.CancelFunc for testing
							},
						},
						"active-page": {
							page:       "active-page",
							lastActive: time.Now(),
							cancel: func() {
								// no-op: satisfies context.CancelFunc for testing
							},
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
			var err error

			BeforeEach(func() {
				daemon := &poolDaemon{
					wikiURL:      "http://localhost:1",
					maxInstances: 5,
					idleTimeout:  30 * time.Minute,
					newCommand:   (&fakeCommandTracker{}).builder,
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
