package jobs

import (
	"sync"

	"github.com/jcelliott/lumber"
	"github.com/mborders/artifex"
)

// JobQueueCoordinator manages multiple job queues using Artifex.
type JobQueueCoordinator struct {
	queues map[string]*artifex.Dispatcher
	stats  map[string]*QueueStats
	logger lumber.Logger
	mu     sync.RWMutex
}

// NewJobQueueCoordinator creates a new JobQueueCoordinator.
func NewJobQueueCoordinator(logger lumber.Logger) *JobQueueCoordinator {
	return &JobQueueCoordinator{
		queues: make(map[string]*artifex.Dispatcher),
		stats:  make(map[string]*QueueStats),
		logger: logger,
	}
}


// EnqueueJob adds a job to its appropriate queue based on the job's name.
func (c *JobQueueCoordinator) EnqueueJob(job Job) {
	c.mu.Lock()
	defer c.mu.Unlock()

	queueName := job.GetName()
	
	// Auto-register queue if it doesn't exist
	dispatcher, exists := c.queues[queueName]
	if !exists {
		// Create new queue for this job type
		const defaultQueueCapacity = 10
		dispatcher = artifex.NewDispatcher(1, defaultQueueCapacity)
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

	// Submit job to Artifex dispatcher
	dispatcher.Dispatch(func() {
		defer func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			
			stats.JobsRemaining--
			if stats.JobsRemaining == 0 {
				stats.IsActive = false
				stats.HighWaterMark = 0 // Reset high water mark when queue is empty
			}
		}()
		
		err := job.Execute()
		if err != nil {
			c.logger.Error("Job execution failed: queue=%s job=%s error=%v", queueName, job.GetName(), err)
		}
	})
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