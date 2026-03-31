// Internal tests for package v1 that need access to unexported types and methods.
// These complement the external tests in server_test.go (package v1_test).
package v1

import (
	"github.com/brendanjerwin/simple_wiki/pageimport"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// errorJobCoordinator always returns an error from EnqueueJob and EnqueueJobWithCompletion.
type errorJobCoordinator struct{}

func (*errorJobCoordinator) GetJobProgress() jobs.JobProgress { return jobs.JobProgress{} }

func (*errorJobCoordinator) EnqueueJobWithCompletion(_ jobs.Job, _ jobs.CompletionCallback) error {
	return nil
}

func (*errorJobCoordinator) EnqueueJob(_ jobs.Job) error {
	return nil
}

var _ = Describe("makeReportJobCallback internal", func() {
	Describe("when pageReaderMutator is nil on the server", func() {
		var callback func(error)

		BeforeEach(func() {
			// Construct a Server directly to bypass NewServer validation —
			// this lets us test the defensive createErr != nil path inside the callback.
			s := &Server{
				logger:              lumber.NewConsoleLogger(lumber.WARN),
				jobQueueCoordinator: &errorJobCoordinator{},
				// pageReaderMutator intentionally left nil
			}
			acc := server.NewPageImportResultAccumulator()
			callback = s.makeReportJobCallback(acc)
		})

		It("should not panic when the callback is invoked", func() {
			Expect(func() { callback(nil) }).NotTo(Panic())
		})
	})
})

var _ = Describe("enqueueImportJobs internal", func() {
	Describe("when pageReaderMutator is nil on the server", func() {
		var err error

		BeforeEach(func() {
			s := &Server{
				logger: lumber.NewConsoleLogger(lumber.WARN),
				// pageReaderMutator intentionally left nil to trigger NewSinglePageImportJob error
			}
			acc := server.NewPageImportResultAccumulator()
			records := []pageimport.ParsedRecord{
				{Identifier: "test_page"},
			}
			err = s.enqueueImportJobs(records, acc)
		})

		It("should return an error when NewSinglePageImportJob fails", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should include the record number in the error message", func() {
			Expect(err.Error()).To(ContainSubstring("1"))
		})
	})
})
