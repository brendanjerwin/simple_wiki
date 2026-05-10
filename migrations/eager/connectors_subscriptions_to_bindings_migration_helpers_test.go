//revive:disable:dot-imports
package eager

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/jcelliott/lumber"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
)

var _ = Describe("pageNeedsConnectorsMigration", func() {
	When("a page has no wiki subtree", func() {
		It("should be skipped by the scan", func() {
			fm := map[string]any{
				"identifier": "p",
			}
			Expect(pageNeedsConnectorsMigration(fm)).To(BeFalse())
		})
	})

	When("a page has wiki but no connectors subtree", func() {
		It("should be skipped by the scan", func() {
			fm := map[string]any{
				"identifier": "p",
				"wiki": map[string]any{
					"system": true,
				},
			}
			Expect(pageNeedsConnectorsMigration(fm)).To(BeFalse())
		})
	})

	When("a connector kind has only the new bindings[] key", func() {
		It("should be skipped (already migrated)", func() {
			fm := map[string]any{
				"identifier": "p",
				"wiki": map[string]any{
					"connectors": map[string]any{
						"google_tasks": map[string]any{
							"bindings": []any{
								map[string]any{"page": "p1", "list_name": "l1"},
							},
						},
					},
				},
			}
			Expect(pageNeedsConnectorsMigration(fm)).To(BeFalse())
		})
	})

	When("a connector kind has the legacy subscriptions[] key", func() {
		It("should be flagged for migration", func() {
			fm := map[string]any{
				"identifier": "p",
				"wiki": map[string]any{
					"connectors": map[string]any{
						"google_tasks": map[string]any{
							"subscriptions": []any{
								map[string]any{"page": "p1"},
							},
						},
					},
				},
			}
			Expect(pageNeedsConnectorsMigration(fm)).To(BeTrue())
		})
	})

	When("a connector kind has BOTH bindings[] and subscriptions[]", func() {
		It("should be flagged for migration so the legacy key gets dropped", func() {
			fm := map[string]any{
				"identifier": "p",
				"wiki": map[string]any{
					"connectors": map[string]any{
						"google_tasks": map[string]any{
							"bindings":      []any{map[string]any{"page": "p"}},
							"subscriptions": []any{map[string]any{"page": "p"}},
						},
					},
				},
			}
			Expect(pageNeedsConnectorsMigration(fm)).To(BeTrue())
		})
	})

	When("only one of two connector kinds has the legacy key", func() {
		It("should still be flagged for migration", func() {
			fm := map[string]any{
				"identifier": "p",
				"wiki": map[string]any{
					"connectors": map[string]any{
						"google_keep": map[string]any{
							"bindings": []any{map[string]any{"page": "k"}},
						},
						"google_tasks": map[string]any{
							"subscriptions": []any{map[string]any{"page": "t"}},
						},
					},
				},
			}
			Expect(pageNeedsConnectorsMigration(fm)).To(BeTrue())
		})
	})
})

// pageWithLegacySubscription builds a TOML frontmatter blob that still
// uses the pre-migration subscriptions[] shape under one connector kind.
func pageWithLegacySubscription(id string) []byte {
	return []byte("+++\nidentifier = '" + id + "'\n\n[wiki.connectors.google_tasks]\nrefresh_token = 'rt'\n\n[[wiki.connectors.google_tasks.subscriptions]]\npage = 'shopping'\nlist_name = 'groceries'\nremote_list_id = 'task-1'\n+++\n\n# Body\n")
}

// pageAlreadyMigratedConnectors builds a TOML frontmatter blob with the
// new bindings[] shape; the scan must skip it.
func pageAlreadyMigratedConnectors(id string) []byte {
	return []byte("+++\nidentifier = '" + id + "'\n\n[wiki.connectors.google_tasks]\nrefresh_token = 'rt'\n\n[[wiki.connectors.google_tasks.bindings]]\npage = 'shopping'\nlist_name = 'groceries'\nremote_handle = 'task-1'\n+++\n\n# Body\n")
}

var _ = Describe("ConnectorsSubscriptionsToBindingsMigrationScanJob", func() {
	var (
		scanner     *MockDataDirScanner
		coordinator *jobs.JobQueueCoordinator
		readerMut   *scanFakeReaderMutator
		job         *ConnectorsSubscriptionsToBindingsMigrationScanJob
	)

	BeforeEach(func() {
		scanner = NewMockDataDirScanner()
		logger := lumber.NewConsoleLogger(lumber.WARN)
		coordinator = jobs.NewJobQueueCoordinator(logger)
		readerMut = &scanFakeReaderMutator{}
		job = NewConnectorsSubscriptionsToBindingsMigrationScanJob(scanner, coordinator, readerMut)
	})

	Describe("GetName", func() {
		It("should report the canonical scan-job name", func() {
			Expect(job.GetName()).To(Equal("ConnectorsSubscriptionsToBindingsMigrationScanJob"))
		})
	})

	Describe("Execute", func() {
		When("the data directory is missing", func() {
			var err error

			BeforeEach(func() {
				scanner.SetDirExists(false)
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not enqueue any jobs", func() {
				Expect(coordinator.GetJobProgress().QueueStats).To(BeEmpty())
			})
		})

		When("ListMDFiles fails", func() {
			var err error

			BeforeEach(func() {
				scanner.SetListError(errors.New("disk gone"))
				err = job.Execute()
			})

			It("should return a wrapped error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("list .md files"))
			})
		})

		When("a profile page still has a legacy subscriptions[] key", func() {
			var err error

			BeforeEach(func() {
				scanner.AddFile("profile_alice.md", pageWithLegacySubscription("profile_alice"))
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should enqueue a per-page migration job", func() {
				Expect(coordinator.GetJobProgress().QueueStats).To(HaveLen(1))
			})
		})

		When("a profile page is already in the new bindings[] shape", func() {
			var err error

			BeforeEach(func() {
				scanner.AddFile("done.md", pageAlreadyMigratedConnectors("done"))
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not enqueue any jobs (idempotent)", func() {
				Expect(coordinator.GetJobProgress().QueueStats).To(BeEmpty())
			})
		})

		When("the same identifier appears in multiple files (dedup)", func() {
			var err error

			BeforeEach(func() {
				scanner.AddFile("a.md", pageWithLegacySubscription("dup"))
				scanner.AddFile("b.md", pageWithLegacySubscription("dup"))
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should enqueue at most one job per unique identifier", func() {
				Expect(coordinator.GetJobProgress().QueueStats).To(HaveLen(1))
			})
		})

		When("a file is malformed (not parseable as TOML)", func() {
			var err error

			BeforeEach(func() {
				scanner.AddFile("garbage.md", []byte("not even +++ delimited\n"))
				err = job.Execute()
			})

			It("should silently skip the file rather than error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(coordinator.GetJobProgress().QueueStats).To(BeEmpty())
			})
		})
	})
})

var _ = Describe("ConnectorsSubscriptionsToBindingsMigrationJob.GetName", func() {
	It("should include the per-page identifier for queue tracing", func() {
		j := NewConnectorsSubscriptionsToBindingsMigrationJob(&scanFakeReaderMutator{}, "alpha")
		Expect(j.GetName()).To(Equal("ConnectorsSubscriptionsToBindingsMigrationJob-alpha"))
	})
})
