package connectors

import (
	"context"
	"errors"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
)

// SchedulerLogger is the lumber-style log surface the scheduler uses.
// Method shapes match jcelliott/lumber's ConsoleLogger so the wiki's
// existing logger plugs in directly.
type SchedulerLogger interface {
	Info(format string, args ...any)
	Error(format string, args ...any)
}

// BindingLister returns the set of subscriptions a connector is
// currently responsible for. Called fresh on every tick so changes
// to on-disk subscription state (including subscriptions added since
// process start) are picked up without a process restart.
//
// One BindingLister is registered per ConnectorKind — the
// SyncScheduler walks all listers on each tick and dispatches a
// per-subscription sync job through the Connector's Sync method.
type BindingLister func() []BindingKey

// JobEnqueuer is the subset of jobs.JobCoordinator the scheduler uses.
// Stated as an interface so tests can substitute a fake.
type JobEnqueuer interface {
	EnqueueJob(job jobs.Job) error
}

// SyncScheduler is the unified cron-tick job that fires across every
// registered connector. One scheduler instance is registered with
// the wiki's CronScheduler at the 30s cadence; on each tick it walks
// each connector's BindingLister and enqueues a per-subscription
// sync job via the connector's per-kind queue.
//
// Per-connector rate-limit choke (skip-this-tick) is the connector's
// own responsibility — Sync returns nil when the connector decides to
// skip; SyncScheduler doesn't second-guess.
type SyncScheduler struct {
	enqueuer JobEnqueuer
	logger   SchedulerLogger

	// connectors maps each registered backend to its BindingLister.
	// Lookup is by ConnectorKind so the scheduler can route per-tick
	// metric attribution and per-kind queue selection without hardcoding
	// the set.
	connectors map[ConnectorKind]connectorEntry
}

type connectorEntry struct {
	connector Connector
	lister    BindingLister
	jobMaker  func(c Connector, key BindingKey) jobs.Job
}

// NewSyncScheduler constructs an empty scheduler. Connectors register
// themselves at bootstrap with Register before the scheduler is
// scheduled with the cron infrastructure.
func NewSyncScheduler(enqueuer JobEnqueuer, logger SchedulerLogger) (*SyncScheduler, error) {
	if enqueuer == nil {
		return nil, errors.New("connectors: SyncScheduler requires a non-nil JobEnqueuer")
	}
	if logger == nil {
		return nil, errors.New("connectors: SyncScheduler requires a non-nil SchedulerLogger")
	}
	return &SyncScheduler{
		enqueuer:   enqueuer,
		logger:     logger,
		connectors: map[ConnectorKind]connectorEntry{},
	}, nil
}

// Register wires a connector into the scheduler. lister is called
// fresh each tick so the scheduler picks up subscriptions added since
// process start; jobMaker turns a (connector, key) pair into a queueable
// job that the per-kind queue runs.
//
// Registering the same kind twice replaces the previous entry. This
// matches the bootstrap reality where there is exactly one Connector
// per kind per process.
func (s *SyncScheduler) Register(c Connector, lister BindingLister, jobMaker func(Connector, BindingKey) jobs.Job) error {
	if c == nil {
		return errors.New("connectors: Register requires a non-nil Connector")
	}
	if lister == nil {
		return errors.New("connectors: Register requires a non-nil BindingLister")
	}
	if jobMaker == nil {
		return errors.New("connectors: Register requires a non-nil job maker")
	}
	s.connectors[c.Kind()] = connectorEntry{
		connector: c,
		lister:    lister,
		jobMaker:  jobMaker,
	}
	return nil
}

// GetName satisfies jobs.Job — also doubles as the cron schedule's
// job name in scheduler logs.
func (*SyncScheduler) GetName() string { return "ConnectorSyncScheduler" }

// Execute fires off one sync job per active subscription across every
// registered connector. Errors from EnqueueJob (queue full, worker
// stopped) are logged and counted as the job's overall failure but
// don't interrupt enqueuing the rest.
//
// Returns nil even on partial enqueue failures; individual sync errors
// land in each per-kind queue's per-job log line.
func (s *SyncScheduler) Execute() error {
	for kind, entry := range s.connectors {
		keys := entry.lister()
		s.logger.Info("ConnectorSyncScheduler: tick fired for %s, %d active subscription(s)", kind, len(keys))
		for _, k := range keys {
			job := entry.jobMaker(entry.connector, k)
			if err := s.enqueuer.EnqueueJob(job); err != nil {
				s.logger.Error("ConnectorSyncScheduler: enqueue %s for %s/%s/%s failed: %v",
					kind, k.ProfileID, k.Page, k.ListName, err)
			}
		}
	}
	return nil
}

// Tick is a context-aware variant of Execute used by tests that want
// to assert on the dispatched calls without going through the cron
// infrastructure. The ctx is currently unused (the underlying Job.Execute
// signature predates context.Context) but is accepted so callers can
// pass per-test cancellation through.
func (s *SyncScheduler) Tick(_ context.Context) error {
	return s.Execute()
}
