package main

import (
	"bytes"
	"os"
	"time"

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

	It("should include the expected flags", func() {
		Expect(cmd.Flags).To(HaveLen(5))
	})

	It("should have a non-nil action", func() {
		Expect(cmd.Action).NotTo(BeNil())
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
			var (
				entry *instanceEntry
				idle  time.Duration
			)

			BeforeEach(func() {
				entry = &instanceEntry{
					page:       "test-page",
					lastActive: time.Now(),
				}
				idle = entry.idleSince()
			})

			It("should return a very short duration", func() {
				Expect(idle).To(BeNumerically("<", time.Second))
			})
		})

		When("entry has been idle for a while", func() {
			var (
				entry *instanceEntry
				idle  time.Duration
			)

			BeforeEach(func() {
				entry = &instanceEntry{
					page:       "test-page",
					lastActive: time.Now().Add(-5 * time.Minute),
				}
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
				daemon = &poolDaemon{
					maxInstances: 5,
					instances: map[string]*instanceEntry{
						"existing-page": {
							page:       "existing-page",
							lastActive: time.Now().Add(-10 * time.Minute),
						},
					},
					// Use a non-existent claude path so spawn would fail if attempted
					claudePath: "/nonexistent/claude",
				}
				daemon.ensureInstance("existing-page")
			})

			It("should update lastActive instead of spawning", func() {
				Expect(daemon.instances["existing-page"].lastActive).To(BeTemporally("~", time.Now(), time.Second))
			})

			It("should not change the instance count", func() {
				Expect(daemon.instances).To(HaveLen(1))
			})
		})

		When("at max capacity", func() {
			var daemon *poolDaemon

			BeforeEach(func() {
				daemon = &poolDaemon{
					maxInstances: 2,
					claudePath:   "/nonexistent/claude",
					wikiURL:      "http://localhost:8050",
					instances: map[string]*instanceEntry{
						"page-a": {
							page:       "page-a",
							lastActive: time.Now().Add(-20 * time.Minute),
							cancel:     func() {},
						},
						"page-b": {
							page:       "page-b",
							lastActive: time.Now().Add(-5 * time.Minute),
							cancel:     func() {},
						},
					},
				}
				// Try to spawn a new instance — the spawn will fail (bad binary)
				// but eviction should still happen
				daemon.ensureInstance("page-c")
			})

			It("should evict the least recently active instance", func() {
				Expect(daemon.instances).NotTo(HaveKey("page-a"))
			})

			It("should keep the more recently active instance", func() {
				Expect(daemon.instances).To(HaveKey("page-b"))
			})
		})
	})

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

	Describe("stopAll", func() {
		When("instances are running", func() {
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
})

var _ = Describe("prefixWriter", func() {
	When("writing a complete line", func() {
		var buf bytes.Buffer

		BeforeEach(func() {
			// Create a temp file to use as the writer target
			tmpFile, err := os.CreateTemp("", "prefix-test")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { os.Remove(tmpFile.Name()) })

			pw := newPrefixWriter(tmpFile, "my-page")
			_, err = pw.Write([]byte("hello world\n"))
			Expect(err).NotTo(HaveOccurred())

			// Read back what was written
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
			DeferCleanup(func() { os.Remove(tmpFile.Name()) })

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
