package index

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// IndexCoordinator manages indexing across multiple index types using the job queue coordinator.
// It knows about all the different indexes and schedules jobs for them, so the Site doesn't need to.
type IndexCoordinator struct {
	coordinator      *jobs.JobQueueCoordinator
	frontmatterIndex IndexOperator
	bleveIndex       IndexOperator
}

// NewIndexCoordinator creates a new IndexCoordinator.
func NewIndexCoordinator(coordinator *jobs.JobQueueCoordinator, frontmatterIndex, bleveIndex IndexOperator) *IndexCoordinator {
	return &IndexCoordinator{
		coordinator:      coordinator,
		frontmatterIndex: frontmatterIndex,
		bleveIndex:       bleveIndex,
	}
}

// EnqueueIndexJob enqueues indexing jobs to all relevant indexes for the given page and operation.
func (c *IndexCoordinator) EnqueueIndexJob(pageIdentifier wikipage.PageIdentifier, operation Operation) {
	// Create and enqueue frontmatter index job
	frontmatterJob := NewFrontmatterIndexJob(c.frontmatterIndex, pageIdentifier, operation)
	c.coordinator.EnqueueJob(frontmatterJob)

	// Create and enqueue bleve index job
	bleveJob := NewBleveIndexJob(c.bleveIndex, pageIdentifier, operation)
	c.coordinator.EnqueueJob(bleveJob)
}

// BulkEnqueuePages enqueues multiple pages for indexing across all indexes.
func (c *IndexCoordinator) BulkEnqueuePages(pageIdentifiers []wikipage.PageIdentifier, operation Operation) {
	for _, pageID := range pageIdentifiers {
		c.EnqueueIndexJob(pageID, operation)
	}
}

// BulkEnqueuePagesWithCompletion enqueues multiple pages for indexing and calls the callback
// when all frontmatter indexing jobs complete. This allows dependent jobs to be scheduled
// after the frontmatter index is fully populated.
func (c *IndexCoordinator) BulkEnqueuePagesWithCompletion(
	pageIdentifiers []wikipage.PageIdentifier,
	operation Operation,
	onAllComplete func(),
) {
	if len(pageIdentifiers) == 0 {
		if onAllComplete != nil {
			onAllComplete()
		}
		return
	}

	// Track completion of frontmatter jobs (the ones normalization depends on)
	var remaining int32 = int32(len(pageIdentifiers))

	for _, pageID := range pageIdentifiers {
		// Capture pageID for closure
		pageID := pageID

		// Enqueue frontmatter job with completion tracking
		frontmatterJob := NewFrontmatterIndexJob(c.frontmatterIndex, pageID, operation)
		c.coordinator.EnqueueJobWithCompletion(frontmatterJob, func(_ error) {
			if atomic.AddInt32(&remaining, -1) == 0 {
				if onAllComplete != nil {
					onAllComplete()
				}
			}
		})

		// Enqueue bleve job normally (normalization doesn't depend on it)
		bleveJob := NewBleveIndexJob(c.bleveIndex, pageID, operation)
		c.coordinator.EnqueueJob(bleveJob)
	}
}

// GetJobQueueCoordinator returns the underlying job queue coordinator for status monitoring.
func (c *IndexCoordinator) GetJobQueueCoordinator() *jobs.JobQueueCoordinator {
	return c.coordinator
}

// WaitForCompletionWithTimeout waits for all indexing jobs to complete or until timeout/cancellation.
// Returns (completed, timedOut) where:
// - completed=true, timedOut=false: all jobs completed successfully
// - completed=false, timedOut=true: timeout occurred
// - completed=false, timedOut=false: context was cancelled
func (c *IndexCoordinator) WaitForCompletionWithTimeout(ctx context.Context, timeout time.Duration) (completed bool, timedOut bool) {
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
			activeQueues := c.coordinator.GetActiveQueues()
			if len(activeQueues) == 0 {
				return true, false // completed
			}
		}
	}
}