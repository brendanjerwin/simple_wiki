//revive:disable:dot-imports
package server_test

import (
	"context"
	"time"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// MockIntegrationIndexOperator is a test implementation of index.IndexOperator for integration tests.
type MockIntegrationIndexOperator struct {
	AddPageToIndexFunc    func(identifier wikipage.PageIdentifier) error
	RemovePageFromIndexFunc func(identifier wikipage.PageIdentifier) error
	addCalled             []wikipage.PageIdentifier
	removeCalled          []wikipage.PageIdentifier
}

func (m *MockIntegrationIndexOperator) AddPageToIndex(identifier wikipage.PageIdentifier) error {
	m.addCalled = append(m.addCalled, identifier)
	if m.AddPageToIndexFunc != nil {
		return m.AddPageToIndexFunc(identifier)
	}
	return nil
}

func (m *MockIntegrationIndexOperator) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	m.removeCalled = append(m.removeCalled, identifier)
	if m.RemovePageFromIndexFunc != nil {
		return m.RemovePageFromIndexFunc(identifier)
	}
	return nil
}

var _ = Describe("IndexingQueueIntegration", func() {
	var (
		coordinator          *jobs.JobQueueCoordinator
		frontmatterMock      *MockIntegrationIndexOperator
		bleveMock           *MockIntegrationIndexOperator
		indexingService     *server.IndexingService
	)

	BeforeEach(func() {
		logger := lumber.NewConsoleLogger(lumber.WARN) // Quiet logger for tests
		coordinator = jobs.NewJobQueueCoordinator(logger)
		frontmatterMock = &MockIntegrationIndexOperator{}
		bleveMock = &MockIntegrationIndexOperator{}
		indexingService = server.NewIndexingService(coordinator, frontmatterMock, bleveMock)
	})

	It("should exist", func() {
		Expect(indexingService).NotTo(BeNil())
	})


	Describe("EnqueueIndexJob", func() {
		Describe("when enqueuing add job", func() {
			BeforeEach(func() {
				indexingService.EnqueueIndexJob("test-page", index.Add)
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
				indexingService.EnqueueIndexJob("test-page", index.Remove)
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
			returnedCoordinator = indexingService.GetJobQueueCoordinator()
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
				indexingService.BulkEnqueuePages(pageIdentifiers, index.Add)
				// Allow time for job execution
				time.Sleep(200 * time.Millisecond)
			})

			It("should call AddPageToIndex for all pages on frontmatter index", func() {
				for _, pageID := range pageIdentifiers {
					Expect(frontmatterMock.addCalled).To(ContainElement(pageID))
				}
			})

			It("should call AddPageToIndex for all pages on bleve index", func() {
				for _, pageID := range pageIdentifiers {
					Expect(bleveMock.addCalled).To(ContainElement(pageID))
				}
			})

			It("should have called add method 3 times on each index", func() {
				Expect(frontmatterMock.addCalled).To(HaveLen(3))
				Expect(bleveMock.addCalled).To(HaveLen(3))
			})
		})
	})

	Describe("WaitForCompletionWithTimeout", func() {
		Describe("when jobs complete quickly", func() {
			var completed bool
			var timedOut bool

			BeforeEach(func() {
				// Enqueue jobs that will complete quickly
				indexingService.EnqueueIndexJob("fast-page", index.Add)
				
				completed, timedOut = indexingService.WaitForCompletionWithTimeout(context.Background(), 1*time.Second)
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
				
				indexingService.EnqueueIndexJob("slow-page", index.Add)
				completed, timedOut = indexingService.WaitForCompletionWithTimeout(ctx, 1*time.Second)
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
				
				indexingService.EnqueueIndexJob("slow-page", index.Add)
				completed, timedOut = indexingService.WaitForCompletionWithTimeout(context.Background(), 50*time.Millisecond)
			})

			It("should timeout", func() {
				Expect(completed).To(BeFalse())
				Expect(timedOut).To(BeTrue())
			})
		})
	})
})