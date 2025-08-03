package jobs

import (
	"sync"

	"github.com/mborders/artifex"
)

// JobQueueCoordinator manages multiple job queues using Artifex.
type JobQueueCoordinator struct {
	queues map[string]*artifex.Dispatcher
	stats  map[string]*QueueStats
	mu     sync.RWMutex
}

// NewJobQueueCoordinator creates a new JobQueueCoordinator.
func NewJobQueueCoordinator() *JobQueueCoordinator {
	return &JobQueueCoordinator{
		queues: make(map[string]*artifex.Dispatcher),
		stats:  make(map[string]*QueueStats),
	}
}

// RegisterQueue registers a new job queue with the given name.
func (c *JobQueueCoordinator) RegisterQueue(queueName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create Artifex dispatcher with 1 worker
	const defaultQueueCapacity = 10
	dispatcher := artifex.NewDispatcher(1, defaultQueueCapacity)
	dispatcher.Start()

	c.queues[queueName] = dispatcher
	c.stats[queueName] = &QueueStats{
		QueueName:     queueName,
		JobsRemaining: 0,
		HighWaterMark: 0,
		IsActive:      false,
	}
}

// EnqueueJob adds a job to the specified queue.
func (c *JobQueueCoordinator) EnqueueJob(queueName string, job Job) {
	c.mu.Lock()
	defer c.mu.Unlock()

	dispatcher, exists := c.queues[queueName]
	if !exists {
		return // Queue not registered
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
		
		_ = job.Execute() // Execute the job (ignore errors for now)
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