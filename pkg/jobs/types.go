package jobs

// Job represents a unit of work that can be executed by a job queue.
type Job interface {
	Execute() error
	GetName() string
}

// QueueStats represents the current statistics for a job queue.
type QueueStats struct {
	QueueName     string
	JobsRemaining int32
	HighWaterMark int32 // Resets to 0 when queue is empty
	IsActive      bool  // jobs_remaining > 0
}

// JobProgress represents the overall progress of all job queues.
type JobProgress struct {
	IsRunning    bool
	QueueStats   []*QueueStats
	TotalActive  int32
	TotalQueues  int32
}

// IProvideJobProgress defines the interface for components that can provide job queue progress.
type IProvideJobProgress interface {
	GetJobProgress() JobProgress
}

// MockJob is a test implementation of the Job interface.
type MockJob struct {
	Name string
	Err  error
}

// Execute implements the Job interface for MockJob.
func (m *MockJob) Execute() error {
	return m.Err
}

// GetName implements the Job interface for MockJob.
func (m *MockJob) GetName() string {
	return m.Name
}