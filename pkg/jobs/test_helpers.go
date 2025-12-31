package jobs

import "sync"

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
