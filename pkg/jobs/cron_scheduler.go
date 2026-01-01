package jobs

import (
	"sync"

	"github.com/brendanjerwin/simple_wiki/pkg/logging"
	"github.com/robfig/cron/v3"
)

// CronScheduler manages scheduled jobs using cron expressions.
type CronScheduler struct {
	cron   *cron.Cron
	logger logging.Logger
	mu     sync.RWMutex
}

// NewCronScheduler creates a new CronScheduler with the given logger.
func NewCronScheduler(logger logging.Logger) *CronScheduler {
	// Use cron with seconds support for more granular scheduling
	c := cron.New(cron.WithSeconds())
	return &CronScheduler{
		cron:   c,
		logger: logger,
	}
}

// Schedule adds a job to run on the given cron schedule.
// The schedule string supports standard cron format with optional seconds field.
// Returns the entry ID for the scheduled job, which can be used to remove it.
func (s *CronScheduler) Schedule(schedule string, job Job) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, err := s.cron.AddFunc(schedule, func() {
		if err := job.Execute(); err != nil {
			s.logger.Error("Cron job execution failed: job=%s error=%v", job.GetName(), err)
		}
	})

	if err != nil {
		return 0, err
	}

	s.logger.Info("Scheduled cron job: name=%s schedule=%s entryID=%d", job.GetName(), schedule, entryID)
	return int(entryID), nil
}

// Start begins executing scheduled jobs.
func (s *CronScheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cron.Start()
	s.logger.Info("Cron scheduler started")
}

// Stop halts the cron scheduler, waiting for running jobs to complete.
func (s *CronScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("Cron scheduler stopped")
}

// Remove unschedules a job by its entry ID.
func (s *CronScheduler) Remove(entryID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cron.Remove(cron.EntryID(entryID))
	s.logger.Info("Removed cron job: entryID=%d", entryID)
}

// Entries returns the number of scheduled entries.
func (s *CronScheduler) Entries() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.cron.Entries())
}
