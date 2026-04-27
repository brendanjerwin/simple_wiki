//revive:disable:dot-imports
package eager

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/jcelliott/lumber"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
)

var _ = Describe("pageNeedsNamespaceMigration", func() {
	When("a page has no legacy keys and is already migrated", func() {
		It("should be skipped by the scan", func() {
			fm := map[string]any{
				"identifier": "p",
				"wiki": map[string]any{
					"system":              true,
					"migrated_namespaces": true,
				},
			}
			Expect(pageNeedsNamespaceMigration(fm)).To(BeFalse())
		})
	})

	When("a page has a legacy system key", func() {
		It("should be flagged for migration", func() {
			fm := map[string]any{
				"identifier": "p",
				"system":     true,
			}
			Expect(pageNeedsNamespaceMigration(fm)).To(BeTrue())
		})
	})

	When("a page has a legacy template key", func() {
		It("should be flagged for migration", func() {
			fm := map[string]any{
				"identifier": "p",
				"template":   true,
			}
			Expect(pageNeedsNamespaceMigration(fm)).To(BeTrue())
		})
	})

	When("a brand-new page has only wiki.system (no migration marker, no legacy)", func() {
		It("should be skipped", func() {
			fm := map[string]any{
				"identifier": "p",
				"wiki": map[string]any{
					"system": true,
				},
			}
			Expect(pageNeedsNamespaceMigration(fm)).To(BeFalse())
		})
	})
})

// pageWithLegacySystem builds a TOML frontmatter blob that still uses the
// pre-migration top-level system flag.
func pageWithLegacySystem(id string) []byte {
	return []byte("+++\nidentifier = '" + id + "'\nsystem = true\n+++\n\n# Body\n")
}

// pageWithLegacyTemplate builds a TOML frontmatter blob that still uses the
// pre-migration top-level template flag.
func pageWithLegacyTemplate(id string) []byte {
	return []byte("+++\nidentifier = '" + id + "'\ntemplate = true\n+++\n\n# Body\n")
}

// pageAlreadyMigrated builds a TOML frontmatter blob that's already in the
// new wiki.* shape; the scan must skip it.
func pageAlreadyMigrated(id string) []byte {
	return []byte("+++\nidentifier = '" + id + "'\n\n[wiki]\nsystem = true\nmigrated_namespaces = true\n+++\n\n# Body\n")
}

var _ = Describe("SystemTemplateNamespaceMigrationScanJob", func() {
	var (
		scanner     *MockDataDirScanner
		coordinator *jobs.JobQueueCoordinator
		readerMut   *scanFakeReaderMutator
		job         *SystemTemplateNamespaceMigrationScanJob
	)

	BeforeEach(func() {
		scanner = NewMockDataDirScanner()
		logger := lumber.NewConsoleLogger(lumber.WARN)
		coordinator = jobs.NewJobQueueCoordinator(logger)
		readerMut = &scanFakeReaderMutator{}
		job = NewSystemTemplateNamespaceMigrationScanJob(scanner, coordinator, readerMut)
	})

	Describe("GetName", func() {
		It("should report the canonical scan-job name", func() {
			Expect(job.GetName()).To(Equal("SystemTemplateNamespaceMigrationScanJob"))
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
				Expect(coordinator.GetActiveQueues()).To(BeEmpty())
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

		When("a page still has a legacy top-level flag", func() {
			var err error

			BeforeEach(func() {
				scanner.AddFile("legacy_system.md", pageWithLegacySystem("legacy_system"))
				scanner.AddFile("legacy_template.md", pageWithLegacyTemplate("legacy_template"))
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should enqueue a per-page migration job for each legacy page", func() {
				queues := coordinator.GetActiveQueues()
				totalJobs := 0
				for _, q := range queues {
					totalJobs += int(q.HighWaterMark)
				}
				Expect(totalJobs).To(BeNumerically(">=", 2))
			})
		})

		When("a page is already in the new wiki.* shape", func() {
			var err error

			BeforeEach(func() {
				scanner.AddFile("done.md", pageAlreadyMigrated("done"))
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not enqueue any jobs (idempotent)", func() {
				Expect(coordinator.GetActiveQueues()).To(BeEmpty())
			})
		})

		When("the same identifier appears in multiple files (dedup)", func() {
			var err error

			BeforeEach(func() {
				scanner.AddFile("a.md", pageWithLegacySystem("dup"))
				scanner.AddFile("b.md", pageWithLegacySystem("dup"))
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should enqueue at most one job per unique identifier", func() {
				queues := coordinator.GetActiveQueues()
				totalJobs := 0
				for _, q := range queues {
					totalJobs += int(q.HighWaterMark)
				}
				Expect(totalJobs).To(BeNumerically("<=", 1))
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
				Expect(coordinator.GetActiveQueues()).To(BeEmpty())
			})
		})
	})
})

var _ = Describe("SystemTemplateNamespaceMigrationJob.GetName", func() {
	It("should include the per-page identifier for queue tracing", func() {
		j := NewSystemTemplateNamespaceMigrationJob(&scanFakeReaderMutator{}, "alpha")
		Expect(j.GetName()).To(Equal("SystemTemplateNamespaceMigrationJob-alpha"))
	})
})
