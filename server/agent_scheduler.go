package server

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// CronJob is an alias for the jobs.Job interface, exposed under a local name
// so test fakes don't have to import pkg/jobs.
type CronJob = jobs.Job

// CronRegistrar is the subset of pkg/jobs.CronScheduler that AgentScheduler
// needs. Defined as an interface so tests can supply a fake registrar without
// spinning up a real cron daemon. The pkg/jobs.CronScheduler satisfies it
// directly.
type CronRegistrar interface {
	Schedule(cron string, job jobs.Job) (int, error)
	Remove(id int)
}

// AgentScheduleQueryer is the subset of the frontmatter index AgentScheduler
// uses to enumerate pages with schedules.
type AgentScheduleQueryer interface {
	QueryKeyExistence(dottedKeyPath string) []wikipage.PageIdentifier
}

// logFieldPage is the structured-log key for the page identifier. Hoisted into
// a constant so a typo in any one log call surfaces at compile time.
const logFieldPage = "page"

// AgentScheduler keeps the in-memory mapping {page, schedule_id} -> cron entry
// id current. LoadAll is called once at startup; Refresh(page) is called from
// the save-hook to react to user edits.
type AgentScheduler struct {
	store       *AgentScheduleStore
	dispatcher  agentTurnDispatcher
	index       AgentScheduleQueryer
	cron        CronRegistrar
	hardTimeout time.Duration

	mu      sync.Mutex
	entries map[scheduleKey]int
}

type scheduleKey struct {
	page       string
	scheduleID string
}

// NewAgentScheduler constructs an AgentScheduler.
func NewAgentScheduler(store *AgentScheduleStore, dispatcher agentTurnDispatcher, index AgentScheduleQueryer, cronReg CronRegistrar, hardTimeout time.Duration) *AgentScheduler {
	return &AgentScheduler{
		store:       store,
		dispatcher:  dispatcher,
		index:       index,
		cron:        cronReg,
		hardTimeout: hardTimeout,
		entries:     map[scheduleKey]int{},
	}
}

// LoadAll enumerates every page that has agent.schedules and registers each
// enabled schedule with the cron registrar. Disabled schedules are parsed but
// not registered. Schedules with invalid cron expressions are logged and
// skipped (they should already have been rejected at write time, but the
// scheduler hardens against bad data on disk).
func (s *AgentScheduler) LoadAll() error {
	pages := s.index.QueryKeyExistence("agent.schedules")
	for _, page := range pages {
		if err := s.loadPage(string(page)); err != nil {
			slog.Error("agent scheduler: load page failed", logFieldPage, page, "error", err)
		}
	}
	return nil
}

// Refresh re-reads one page's schedules and reconciles the in-memory cron
// registrations: removes entries whose schedule no longer exists or is now
// disabled, adds entries that just appeared, and re-registers entries whose
// cron expression changed.
func (s *AgentScheduler) Refresh(page string) error {
	return s.loadPage(page)
}

// UnregisterPage removes every cron entry registered for the given page,
// without consulting the store. Called from DeletePage so the registrations
// stop firing immediately — Refresh would also work here, but only if List
// returns "no schedules" cleanly for a missing page; UnregisterPage is the
// explicit, store-independent path that always does the right thing.
func (s *AgentScheduler) UnregisterPage(page string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, entryID := range s.entries {
		if key.page != page {
			continue
		}
		s.cron.Remove(entryID)
		delete(s.entries, key)
	}
}

func (s *AgentScheduler) loadPage(page string) error {
	schedules, err := s.store.List(page)
	if err != nil {
		return fmt.Errorf("list schedules for %q: %w", page, err)
	}

	// Build the set of {schedule_id -> AgentSchedule} that should be registered
	// after this call.
	desired := map[string]*apiv1.AgentSchedule{}
	for _, sc := range schedules {
		if !sc.GetEnabled() {
			continue
		}
		if !isValidCron(sc.GetCron()) {
			slog.Warn("agent scheduler: skipping schedule with invalid cron",
				logFieldPage, page, "schedule_id", sc.GetId(), "cron", sc.GetCron())
			continue
		}
		desired[sc.GetId()] = sc
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove or replace stale entries for this page.
	for key, entryID := range s.entries {
		if key.page != page {
			continue
		}
		want, exists := desired[key.scheduleID]
		if !exists {
			s.cron.Remove(entryID)
			delete(s.entries, key)
			continue
		}
		// Always re-register so cron-expression edits take effect. The
		// alternative (track prior cron string and compare) bloats state for
		// little benefit.
		s.cron.Remove(entryID)
		delete(s.entries, key)
		newID, scheduleErr := s.cron.Schedule(scheduleExpression(want), s.newJob(page, want.GetId()))
		if scheduleErr != nil {
			slog.Warn("agent scheduler: re-register failed", logFieldPage, page, "schedule_id", want.GetId(), "error", scheduleErr)
			continue
		}
		s.entries[key] = newID
	}

	// Add brand-new entries.
	for id, sc := range desired {
		key := scheduleKey{page: page, scheduleID: id}
		if _, alreadyRegistered := s.entries[key]; alreadyRegistered {
			continue
		}
		entryID, scheduleErr := s.cron.Schedule(scheduleExpression(sc), s.newJob(page, id))
		if scheduleErr != nil {
			slog.Warn("agent scheduler: register failed", logFieldPage, page, "schedule_id", id, "error", scheduleErr)
			continue
		}
		s.entries[key] = entryID
	}
	return nil
}

// scheduleExpression prefixes the schedule's cron expression with
// "CRON_TZ=<timezone> " so robfig/cron interprets the schedule in the named
// IANA timezone. Empty/unset timezone defaults to UTC — chosen to remove
// timezone ambiguity from the contract; users who want local time set
// schedule.timezone explicitly. The handler validates the IANA name on
// UpsertSchedule, so by the time the scheduler sees it the value is either
// empty (→ UTC) or known-good.
func scheduleExpression(sc *apiv1.AgentSchedule) string {
	tz := sc.GetTimezone()
	if tz == "" {
		tz = "UTC"
	}
	return "CRON_TZ=" + tz + " " + sc.GetCron()
}

func (s *AgentScheduler) newJob(page, scheduleID string) CronJob {
	return NewAgentTurnJob(s.store, s.dispatcher, page, scheduleID, s.hardTimeout)
}

// isValidCron returns true if expr parses as a 6-field (with seconds) cron.
func isValidCron(expr string) bool {
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(expr)
	return err == nil
}

