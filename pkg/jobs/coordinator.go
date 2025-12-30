package jobs

import (
	"fmt"
	"sync"

	"github.com/jcelliott/lumber"
	"github.com/mborders/artifex"
)

// Dispatcher is an interface for job dispatching, allowing for testing.
type Dispatcher interface {
	Start()
	Dispatch(run func()) error
}

// DispatcherFactory creates new dispatchers.
type DispatcherFactory func(maxWorkers, maxQueue int) Dispatcher

// artifexDispatcher wraps artifex.Dispatcher to implement our Dispatcher interface.
type artifexDispatcher struct {
	*artifex.Dispatcher
}

// defaultDispatcherFactory creates real Artifex dispatchers.
func defaultDispatcherFactory(maxWorkers, maxQueue int) Dispatcher {
	return &artifexDispatcher{artifex.NewDispatcher(maxWorkers, maxQueue)}
}

// JobQueueCoordinator manages multiple job queues using Artifex.
type JobQueueCoordinator struct {
	queues            map[string]Dispatcher
	stats             map[string]*QueueStats
	logger            lumber.Logger
	mu                sync.RWMutex
	dispatcherFactory DispatcherFactory
}

// rollbackStats decrements job count when a dispatch fails.
// Must be called while holding the coordinator's mu lock.
func (c *JobQueueCoordinator) rollbackStats(stats *QueueStats) {
	// Assert that the mutex is held - if TryLock succeeds, we have a bug
	if c.mu.TryLock() {
		c.mu.Unlock()
		panic("rollbackStats called without holding mu lock")
	}
	stats.JobsRemaining--
	if stats.JobsRemaining == 0 {
		stats.IsActive = false
	}
}

// NewJobQueueCoordinator creates a new JobQueueCoordinator.
func NewJobQueueCoordinator(logger lumber.Logger) *JobQueueCoordinator {
	return &JobQueueCoordinator{
		queues:            make(map[string]Dispatcher),
		stats:             make(map[string]*QueueStats),
		logger:            logger,
		dispatcherFactory: defaultDispatcherFactory,
	}
}

// NewJobQueueCoordinatorWithFactory creates a JobQueueCoordinator with a custom dispatcher factory (for testing).
func NewJobQueueCoordinatorWithFactory(logger lumber.Logger, factory DispatcherFactory) *JobQueueCoordinator {
	return &JobQueueCoordinator{
		queues:            make(map[string]Dispatcher),
		stats:             make(map[string]*QueueStats),
		logger:            logger,
		dispatcherFactory: factory,
	}
}


// EnqueueJob adds a job to its appropriate queue based on the job's name.
// Returns an error if the job could not be dispatched.
func (c *JobQueueCoordinator) EnqueueJob(job Job) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	queueName := job.GetName()

	// Auto-register queue if it doesn't exist
	dispatcher, exists := c.queues[queueName]
	if !exists {
		// Create new queue for this job type
		const defaultQueueCapacity = 10
		dispatcher = c.dispatcherFactory(1, defaultQueueCapacity)
		dispatcher.Start()

		c.queues[queueName] = dispatcher
		c.stats[queueName] = &QueueStats{
			QueueName:     queueName,
			JobsRemaining: 0,
			HighWaterMark: 0,
			IsActive:      false,
		}
	}

	stats := c.stats[queueName]

	// Increment jobs remaining and update high water mark
	stats.JobsRemaining++
	if stats.JobsRemaining > stats.HighWaterMark {
		stats.HighWaterMark = stats.JobsRemaining
	}
	stats.IsActive = true

	// Submit job to dispatcher
	err := dispatcher.Dispatch(func() {
		defer func() {
			c.mu.Lock()
			defer c.mu.Unlock()

			stats.JobsRemaining--
			if stats.JobsRemaining == 0 {
				stats.IsActive = false
				stats.HighWaterMark = 0 // Reset high water mark when queue is empty
			}
		}()

		execErr := job.Execute()
		if execErr != nil {
			c.logger.Error("Job execution failed: queue=%s job=%s error=%v", queueName, job.GetName(), execErr)
		}
	})
	if err != nil {
		c.rollbackStats(stats)
		return fmt.Errorf("dispatch job %s: %w", queueName, err)
	}
	return nil
}

// CompletionCallback is called when a job completes, with the error (if any).
type CompletionCallback func(err error)

// EnqueueJobWithCompletion adds a job to its queue and calls the callback when it completes.
// This allows job chaining - the callback can enqueue dependent jobs.
// Returns an error if the job could not be dispatched.
func (c *JobQueueCoordinator) EnqueueJobWithCompletion(job Job, onComplete CompletionCallback) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	queueName := job.GetName()

	// Auto-register queue if it doesn't exist
	dispatcher, exists := c.queues[queueName]
	if !exists {
		// Create new queue for this job type
		const defaultQueueCapacity = 10
		dispatcher = c.dispatcherFactory(1, defaultQueueCapacity)
		dispatcher.Start()

		c.queues[queueName] = dispatcher
		c.stats[queueName] = &QueueStats{
			QueueName:     queueName,
			JobsRemaining: 0,
			HighWaterMark: 0,
			IsActive:      false,
		}
	}

	stats := c.stats[queueName]

	// Increment jobs remaining and update high water mark
	stats.JobsRemaining++
	if stats.JobsRemaining > stats.HighWaterMark {
		stats.HighWaterMark = stats.JobsRemaining
	}
	stats.IsActive = true

	// Submit job to dispatcher
	err := dispatcher.Dispatch(func() {
		defer func() {
			c.mu.Lock()
			defer c.mu.Unlock()

			stats.JobsRemaining--
			if stats.JobsRemaining == 0 {
				stats.IsActive = false
				stats.HighWaterMark = 0 // Reset high water mark when queue is empty
			}
		}()

		execErr := job.Execute()
		if execErr != nil {
			c.logger.Error("Job execution failed: queue=%s job=%s error=%v", queueName, job.GetName(), execErr)
		}

		// Call completion callback after job execution
		if onComplete != nil {
			onComplete(execErr)
		}
	})
	if err != nil {
		c.rollbackStats(stats)
		return fmt.Errorf("dispatch job with completion %s: %w", queueName, err)
	}
	return nil
}

// GetQueueStats returns the current statistics for the specified queue.
func (c *JobQueueCoordinator) GetQueueStats(queueName string) *QueueStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats, exists := c.stats[queueName]
	if !exists {
		return nil
	}

	// Return a copy to avoid race conditions
	return &QueueStats{
		QueueName:     stats.QueueName,
		JobsRemaining: stats.JobsRemaining,
		HighWaterMark: stats.HighWaterMark,
		IsActive:      stats.IsActive,
	}
}

// GetActiveQueues returns statistics for all queues that have jobs remaining.
func (c *JobQueueCoordinator) GetActiveQueues() []*QueueStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var activeQueues []*QueueStats
	for _, stats := range c.stats {
		if stats.IsActive {
			// Return a copy to avoid race conditions
			activeQueues = append(activeQueues, &QueueStats{
				QueueName:     stats.QueueName,
				JobsRemaining: stats.JobsRemaining,
				HighWaterMark: stats.HighWaterMark,
				IsActive:      stats.IsActive,
			})
		}
	}

	return activeQueues
}

// GetJobProgress implements the IProvideJobProgress interface.
func (c *JobQueueCoordinator) GetJobProgress() JobProgress {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var allQueues []*QueueStats
	var activeQueues int32
	totalQueues := int32(len(c.stats))

	for _, stats := range c.stats {
		// Return a copy to avoid race conditions
		queueCopy := &QueueStats{
			QueueName:     stats.QueueName,
			JobsRemaining: stats.JobsRemaining,
			HighWaterMark: stats.HighWaterMark,
			IsActive:      stats.IsActive,
		}
		allQueues = append(allQueues, queueCopy)
		
		if stats.IsActive {
			activeQueues++
		}
	}

	return JobProgress{
		IsRunning:   activeQueues > 0,
		QueueStats:  allQueues,
		TotalActive: activeQueues,
		TotalQueues: totalQueues,
	}
}