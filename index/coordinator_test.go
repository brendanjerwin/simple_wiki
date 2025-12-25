//revive:disable:dot-imports
package index_test

import (
	"context"
	"time"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// MockCoordinatorIndexOperator is a test implementation of index.IndexOperator for coordinator tests.
type MockCoordinatorIndexOperator struct {
	AddPageToIndexFunc    func(identifier wikipage.PageIdentifier) error
	RemovePageFromIndexFunc func(identifier wikipage.PageIdentifier) error
	addCalled             []wikipage.PageIdentifier
	removeCalled          []wikipage.PageIdentifier
}

func (m *MockCoordinatorIndexOperator) AddPageToIndex(identifier wikipage.PageIdentifier) error {
	m.addCalled = append(m.addCalled, identifier)
	if m.AddPageToIndexFunc != nil {
		return m.AddPageToIndexFunc(identifier)
	}
	return nil
}

func (m *MockCoordinatorIndexOperator) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	m.removeCalled = append(m.removeCalled, identifier)
	if m.RemovePageFromIndexFunc != nil {
		return m.RemovePageFromIndexFunc(identifier)
	}
	return nil
}

var _ = Describe("IndexCoordinator", func() {
	var (
		coordinator          *jobs.JobQueueCoordinator
		frontmatterMock      *MockCoordinatorIndexOperator
		bleveMock           *MockCoordinatorIndexOperator
		indexCoordinator    *index.IndexCoordinator
	)

	BeforeEach(func() {
		logger := lumber.NewConsoleLogger(lumber.WARN) // Quiet logger for tests
		coordinator = jobs.NewJobQueueCoordinator(logger)
		frontmatterMock = &MockCoordinatorIndexOperator{}
		bleveMock = &MockCoordinatorIndexOperator{}
		indexCoordinator = index.NewIndexCoordinator(coordinator, frontmatterMock, bleveMock)
	})

	It("should exist", func() {
		Expect(indexCoordinator).NotTo(BeNil())
	})

	Describe("EnqueueIndexJob", func() {
		Describe("when enqueuing add job", func() {
			BeforeEach(func() {
				indexCoordinator.EnqueueIndexJob("test-page", index.Add)
				// Allow time for job execution
				time.Sleep(100 * time.Millisecond)
			})

			It("should call AddPageToIndex on frontmatter index", func() {
				Expect(frontmatterMock.addCalled).To(ContainElement("test-page"))
			})

			It("should call AddPageToIndex on bleve index", func() {
				Expect(bleveMock.addCalled).To(ContainElement("test-page"))
			})

			It("should not call remove methods", func() {
				Expect(frontmatterMock.removeCalled).To(BeEmpty())
				Expect(bleveMock.removeCalled).To(BeEmpty())
			})
		})

		Describe("when enqueuing remove job", func() {
			BeforeEach(func() {
				indexCoordinator.EnqueueIndexJob("test-page", index.Remove)
				// Allow time for job execution
				time.Sleep(100 * time.Millisecond)
			})

			It("should call RemovePageFromIndex on frontmatter index", func() {
				Expect(frontmatterMock.removeCalled).To(ContainElement("test-page"))
			})

			It("should call RemovePageFromIndex on bleve index", func() {
				Expect(bleveMock.removeCalled).To(ContainElement("test-page"))
			})

			It("should not call add methods", func() {
				Expect(frontmatterMock.addCalled).To(BeEmpty())
				Expect(bleveMock.addCalled).To(BeEmpty())
			})
		})
	})

	Describe("GetJobQueueCoordinator", func() {
		var returnedCoordinator *jobs.JobQueueCoordinator

		BeforeEach(func() {
			returnedCoordinator = indexCoordinator.GetJobQueueCoordinator()
		})

		It("should return the coordinator", func() {
			Expect(returnedCoordinator).To(Equal(coordinator))
		})
	})

	Describe("BulkEnqueuePages", func() {
		var pageIdentifiers []wikipage.PageIdentifier

		BeforeEach(func() {
			pageIdentifiers = []wikipage.PageIdentifier{"page1", "page2", "page3"}
		})

		Describe("when enqueuing add jobs", func() {
			BeforeEach(func() {
				indexCoordinator.BulkEnqueuePages(pageIdentifiers, index.Add)
				// Allow time for job execution
				time.Sleep(200 * time.Millisecond)
			})

			It("should call AddPageToIndex for all pages on frontmatter index", func() {
				Expect(frontmatterMock.addCalled).To(ContainElements("page1", "page2", "page3"))
			})

			It("should call AddPageToIndex for all pages on bleve index", func() {
				Expect(bleveMock.addCalled).To(ContainElements("page1", "page2", "page3"))
			})
		})
	})

	Describe("WaitForCompletionWithTimeout", func() {
		Describe("when jobs complete quickly", func() {
			var completed bool
			var timedOut bool

			BeforeEach(func() {
				// Enqueue jobs that will complete quickly
				indexCoordinator.EnqueueIndexJob("fast-page", index.Add)
				
				completed, timedOut = indexCoordinator.WaitForCompletionWithTimeout(context.Background(), 1*time.Second)
			})

			It("should complete without timeout", func() {
				Expect(completed).To(BeTrue())
				Expect(timedOut).To(BeFalse())
			})
		})

		Describe("when context is cancelled", func() {
			var completed bool
			var timedOut bool
			var ctx context.Context
			var cancel context.CancelFunc

			BeforeEach(func() {
				// Create context that will be cancelled immediately
				ctx, cancel = context.WithCancel(context.Background())
				cancel() // Cancel immediately
				
				indexCoordinator.EnqueueIndexJob("slow-page", index.Add)
				completed, timedOut = indexCoordinator.WaitForCompletionWithTimeout(ctx, 1*time.Second)
			})

			It("should not complete and should indicate context cancellation", func() {
				Expect(completed).To(BeFalse())
				Expect(timedOut).To(BeFalse()) // Not a timeout, but context cancellation
			})
		})

		Describe("when timeout occurs", func() {
			var completed bool
			var timedOut bool

			BeforeEach(func() {
				// Mock slow operations
				frontmatterMock.AddPageToIndexFunc = func(identifier wikipage.PageIdentifier) error {
					time.Sleep(200 * time.Millisecond)
					return nil
				}
				bleveMock.AddPageToIndexFunc = func(identifier wikipage.PageIdentifier) error {
					time.Sleep(200 * time.Millisecond)
					return nil
				}
				
				indexCoordinator.EnqueueIndexJob("slow-page", index.Add)
				completed, timedOut = indexCoordinator.WaitForCompletionWithTimeout(context.Background(), 50*time.Millisecond)
			})

			It("should timeout", func() {
				Expect(completed).To(BeFalse())
				Expect(timedOut).To(BeTrue())
			})
		})
	})
})