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