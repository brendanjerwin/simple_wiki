package server

import (
	"context"
	"time"

	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// IndexingService manages separate indexing queues using the job queue coordinator.
type IndexingService struct {
	coordinator      *jobs.JobQueueCoordinator
	frontmatterIndex index.IndexOperator
	bleveIndex       index.IndexOperator
}

// NewIndexingService creates a new IndexingService.
func NewIndexingService(coordinator *jobs.JobQueueCoordinator, frontmatterIndex, bleveIndex index.IndexOperator) *IndexingService {
	return &IndexingService{
		coordinator:      coordinator,
		frontmatterIndex: frontmatterIndex,
		bleveIndex:       bleveIndex,
	}
}


// EnqueueIndexJob enqueues indexing jobs to both queues for the given page and operation.
func (s *IndexingService) EnqueueIndexJob(pageIdentifier wikipage.PageIdentifier, operation index.Operation) {
	// Create and enqueue frontmatter index job
	frontmatterJob := index.NewFrontmatterIndexJob(s.frontmatterIndex, pageIdentifier, operation)
	s.coordinator.EnqueueJob(frontmatterJob)

	// Create and enqueue bleve index job
	bleveJob := index.NewBleveIndexJob(s.bleveIndex, pageIdentifier, operation)
	s.coordinator.EnqueueJob(bleveJob)
}

// BulkEnqueuePages enqueues multiple pages for indexing.
func (s *IndexingService) BulkEnqueuePages(pageIdentifiers []wikipage.PageIdentifier, operation index.Operation) {
	for _, pageID := range pageIdentifiers {
		s.EnqueueIndexJob(pageID, operation)
	}
}

// GetJobQueueCoordinator returns the underlying job queue coordinator for status monitoring.
func (s *IndexingService) GetJobQueueCoordinator() *jobs.JobQueueCoordinator {
	return s.coordinator
}

// WaitForCompletionWithTimeout waits for all indexing jobs to complete or until timeout/cancellation.
// Returns (completed, timedOut) where:
// - completed=true, timedOut=false: all jobs completed successfully
// - completed=false, timedOut=true: timeout occurred
// - completed=false, timedOut=false: context was cancelled
func (s *IndexingService) WaitForCompletionWithTimeout(ctx context.Context, timeout time.Duration) (completed bool, timedOut bool) {
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			// Check if it was timeout or context cancellation
			if timeoutCtx.Err() == context.DeadlineExceeded {
				return false, true // timeout
			}
			return false, false // context cancelled

		case <-ticker.C:
			// Check if all queues are idle
			activeQueues := s.coordinator.GetActiveQueues()
			if len(activeQueues) == 0 {
				return true, false // completed
			}
		}
	}
}

