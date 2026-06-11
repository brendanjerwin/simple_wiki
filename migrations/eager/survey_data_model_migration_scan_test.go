//revive:disable:dot-imports
package eager

import (
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SurveyDataModelMigrationScanJob", func() {
	var (
		scanner     *MockDataDirScanner
		coordinator *jobs.JobQueueCoordinator
		job         *SurveyDataModelMigrationScanJob
		executeErr  error
	)

	BeforeEach(func() {
		scanner = NewMockDataDirScanner()
		coordinator = jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.WARN))
		job = NewSurveyDataModelMigrationScanJob(scanner, coordinator, &scanFakeReaderMutator{})
	})

	Describe("GetName", func() {
		It("should report the canonical scan-job name", func() {
			Expect(job.GetName()).To(Equal("SurveyDataModelMigrationScanJob"))
		})
	})

	When("the scan contains mixed page shapes", func() {
		BeforeEach(func() {
			scanner.AddFile("legacy-a.md", []byte("+++\nidentifier = 'weekly_menu'\n\n[surveys.meal]\nquestion = 'Dinner?'\n+++\n\n# Body\n"))
			scanner.AddFile("legacy-b.md", []byte("+++\nidentifier = 'weekly_menu'\n\n[surveys.snack]\nquestion = 'Snack?'\n+++\n\n# Body\n"))
			scanner.AddFile("clean.md", []byte("+++\nidentifier = 'clean'\ntitle = 'Clean'\n+++\n\n# Body\n"))
			scanner.AddFile("markdown-only.md", []byte("# Body only\n"))
			scanner.AddFile("bad-toml.md", []byte("+++\nidentifier = [bad\n+++\n\n# Body\n"))
			executeErr = job.Execute()
		})

		It("should not error", func() {
			Expect(executeErr).NotTo(HaveOccurred())
		})

		It("should enqueue one migration job per identifier", func() {
			Expect(coordinator.GetJobProgress().QueueStats).To(HaveLen(1))
		})
	})

	When("listing markdown files fails", func() {
		BeforeEach(func() {
			scanner.SetListError(assertAnError{})
			executeErr = job.Execute()
		})

		It("should return a wrapped list error", func() {
			Expect(executeErr).To(MatchError(ContainSubstring("list .md files")))
		})
	})
})

type assertAnError struct{}

func (assertAnError) Error() string {
	return "assertion error"
}
