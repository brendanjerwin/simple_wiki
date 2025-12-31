package jobs

import "sync"

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

// CompletionCallback is called when a job completes, with the error (if any).
type CompletionCallback func(err error)

// JobCoordinator defines the interface for components that can enqueue jobs and provide progress.
type JobCoordinator interface {
	GetJobProgress() JobProgress
	EnqueueJob(job Job) error
	EnqueueJobWithCompletion(job Job, onComplete CompletionCallback) error
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

// BlockingMockJob is a test job that blocks until Release is called.
type BlockingMockJob struct {
	Name        string
	Err         error
	release     chan struct{}
	releaseOnce sync.Once
}

// NewBlockingMockJob creates a new BlockingMockJob.
func NewBlockingMockJob(name string) *BlockingMockJob {
	return &BlockingMockJob{
		Name:    name,
		release: make(chan struct{}),
	}
}

// Execute implements the Job interface for BlockingMockJob.
// It blocks until Release is called.
func (m *BlockingMockJob) Execute() error {
	<-m.release
	return m.Err
}

// GetName implements the Job interface for BlockingMockJob.
func (m *BlockingMockJob) GetName() string {
	return m.Name
}

// Release unblocks the Execute method. Safe to call multiple times concurrently.
func (m *BlockingMockJob) Release() {
	m.releaseOnce.Do(func() {
		close(m.release)
	})
}

// MockDispatcher is a test implementation of the Dispatcher interface.
type MockDispatcher struct {
	started       bool
	dispatchErr   error
	dispatchCount int
	lastFunc      func()
}

// NewMockDispatcher creates a new MockDispatcher.
func NewMockDispatcher() *MockDispatcher {
	return &MockDispatcher{}
}

// Start implements the Dispatcher interface.
func (m *MockDispatcher) Start() {
	m.started = true
}

// Dispatch implements the Dispatcher interface.
func (m *MockDispatcher) Dispatch(run func()) error {
	m.dispatchCount++
	m.lastFunc = run
	if m.dispatchErr != nil {
		return m.dispatchErr
	}
	// Execute the function immediately in a goroutine (like real dispatcher)
	go run()
	return nil
}

// SetDispatchError configures the dispatcher to return an error on Dispatch.
func (m *MockDispatcher) SetDispatchError(err error) {
	m.dispatchErr = err
}

// DispatchCount returns the number of times Dispatch was called.
func (m *MockDispatcher) DispatchCount() int {
	return m.dispatchCount
}

// FailingDispatcherFactory creates a factory that returns dispatchers that fail on Dispatch.
func FailingDispatcherFactory(err error) DispatcherFactory {
	return func(_, _ int) Dispatcher {
		d := NewMockDispatcher()
		d.SetDispatchError(err)
		return d
	}
}