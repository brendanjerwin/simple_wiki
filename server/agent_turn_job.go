package server

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// AgentTurnJobName is the queue name shared by every AgentTurnJob; concurrency
// is controlled by JobQueueCoordinator.RegisterQueue at startup.
const AgentTurnJobName = "AgentTurn"

// Structured log keys repeated across this file. Hoisted into constants so
// rename / typo / consistency concerns surface at compile time rather than as
// silently-divergent log fields.
const (
	logKeyError      = "error"
	logKeyPage       = "page"
	logKeyScheduleID = "schedule_id"
)

// agentTurnDispatcher captures the subset of the ScheduledTurnDispatcher API
// that AgentTurnJob needs. Defined as an interface so tests can supply a fake.
type agentTurnDispatcher interface {
	Dispatch(req *apiv1.ScheduledTurnRequest) (<-chan *ScheduledTurnOutcome, error)
}

// AgentTurnJob drives one fire of one schedule on one page. It is a Job
// (compatible with JobQueueCoordinator) so cron firings flow through the
// existing concurrency machinery.
//
// Lifecycle of one Execute() call:
//  1. Snapshot the schedule so we know prompt + max_turns.
//  2. Transition to RUNNING (validated by the schedule state machine).
//  3. Dispatch a ScheduledTurnRequest to the pool and block on the
//     completion channel (with a hard timeout as a safety net).
//  4. Transition to OK / ERROR / TIMEOUT based on the pool's outcome.
//
// If Dispatch itself fails (no pool subscribers), the job records ERROR
// directly without ever going through RUNNING — see the state machine note in
// the design doc.
type AgentTurnJob struct {
	store       *AgentScheduleStore
	dispatcher  agentTurnDispatcher
	page        string
	scheduleID  string
	hardTimeout time.Duration
}

// NewAgentTurnJob constructs a job for one cron fire of one schedule.
func NewAgentTurnJob(store *AgentScheduleStore, dispatcher agentTurnDispatcher, page, scheduleID string, hardTimeout time.Duration) *AgentTurnJob {
	return &AgentTurnJob{
		store:       store,
		dispatcher:  dispatcher,
		page:        page,
		scheduleID:  scheduleID,
		hardTimeout: hardTimeout,
	}
}

// GetName implements jobs.Job.
func (*AgentTurnJob) GetName() string {
	return AgentTurnJobName
}

// Execute implements jobs.Job. Returns nil on every "expected" failure mode
// (the failure is recorded on the schedule status); only programming bugs that
// genuinely should not happen surface as a non-nil error to the queue's error
// channel.
func (j *AgentTurnJob) Execute() error {
	// Snapshot the schedule so we have the prompt + max_turns.
	schedules, err := j.store.List(j.page)
	if err != nil {
		slog.Error("agent turn: list schedules failed", logKeyPage, j.page, logKeyScheduleID, j.scheduleID, logKeyError, err)
		return nil
	}
	var snapshot *apiv1.AgentSchedule
	for _, sc := range schedules {
		if sc.GetId() == j.scheduleID {
			snapshot = sc
			break
		}
	}
	if snapshot == nil {
		slog.Warn("agent turn: schedule no longer exists", logKeyPage, j.page, logKeyScheduleID, j.scheduleID)
		return nil
	}

	// Single-in-flight: skip this fire when the schedule is already RUNNING. A
	// stuck RUNNING (e.g. the pool died mid-turn) eventually clears via
	// awaitOutcome's hard timeout, after which the next cron fire proceeds.
	// Without this guard, overlapping fires dispatch a new turn but then fail
	// the RUNNING -> RUNNING state-machine transition, leaving the new
	// dispatch's completion channel orphaned and the schedule status frozen
	// at the original RUNNING.
	if snapshot.GetLastStatus() == apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING {
		slog.Info("agent turn: skipping fire — previous run still in flight",
			logKeyPage, j.page, logKeyScheduleID, j.scheduleID)
		return nil
	}

	// Pre-flight Dispatch so we can fail fast (and skip the RUNNING transition)
	// if no pool is connected.
	requestID := uuid.NewString()
	maxTurns := snapshot.GetMaxTurns()
	if maxTurns == 0 {
		const defaultMaxTurns = 20
		maxTurns = defaultMaxTurns
	}
	completion, dispatchErr := j.dispatcher.Dispatch(&apiv1.ScheduledTurnRequest{
		RequestId: requestID,
		Page:      j.page,
		Prompt:    snapshot.GetPrompt(),
		MaxTurns:  maxTurns,
	})
	if dispatchErr != nil {
		// Record terminal ERROR via the FROM-current-state transition. The
		// schedule may have any prior status; if the prior status was a
		// terminal one, ERROR is reachable via the pseudo-fire that didn't
		// actually run. Mirror that: synthesize a RUNNING transition first if
		// needed so the state machine remains coherent.
		j.recordDispatchFailure(dispatchErr)
		return nil
	}

	// Transition to RUNNING now that dispatch succeeded.
	if err := j.store.TransitionStatus(j.page, j.scheduleID, apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0); err != nil {
		slog.Error("agent turn: transition to RUNNING failed", logKeyPage, j.page, logKeyScheduleID, j.scheduleID, logKeyError, err)
		return nil
	}

	outcome, terminalErr := j.awaitOutcome(completion)
	if terminalErr != nil {
		_ = j.store.TransitionStatus(j.page, j.scheduleID, apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR, terminalErr.Error(), 0)
		return nil
	}

	if err := j.store.TransitionStatus(j.page, j.scheduleID, outcome.TerminalStatus, outcome.ErrorMessage, outcome.DurationSeconds); err != nil {
		slog.Error("agent turn: terminal transition failed", logKeyPage, j.page, logKeyScheduleID, j.scheduleID, logKeyError, err)
	}
	return nil
}

// recordDispatchFailure records a terminal ERROR status when the pool is not
// available. The state machine forbids skipping RUNNING when transitioning to
// ERROR, so we synthesize a RUNNING transition first when the prior status
// allows it. If even RUNNING is illegal (e.g. the prior status is already
// RUNNING from a stuck previous fire), we log and bail rather than corrupting
// the on-disk record.
func (j *AgentTurnJob) recordDispatchFailure(dispatchErr error) {
	msg := fmt.Sprintf("dispatch failed: %v", dispatchErr)

	if err := j.store.TransitionStatus(j.page, j.scheduleID, apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0); err != nil {
		var illegal *IllegalScheduleTransitionError
		if errors.As(err, &illegal) {
			slog.Warn("agent turn: dispatch failed but prior state forbids RUNNING transition; leaving status unchanged",
				logKeyPage, j.page, logKeyScheduleID, j.scheduleID, "from", illegal.From, logKeyError, dispatchErr)
			return
		}
		slog.Error("agent turn: synthetic RUNNING transition failed", logKeyPage, j.page, logKeyScheduleID, j.scheduleID, logKeyError, err)
		return
	}
	if err := j.store.TransitionStatus(j.page, j.scheduleID, apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR, msg, 0); err != nil {
		slog.Error("agent turn: ERROR transition after dispatch failure failed", logKeyPage, j.page, logKeyScheduleID, j.scheduleID, logKeyError, err)
	}
}

// awaitOutcome blocks on the completion channel up to hardTimeout. A nil
// outcome with a returned error means the timeout fired.
func (j *AgentTurnJob) awaitOutcome(completion <-chan *ScheduledTurnOutcome) (*ScheduledTurnOutcome, error) {
	if j.hardTimeout <= 0 {
		// Without a positive timeout, just wait indefinitely.
		return <-completion, nil
	}
	select {
	case outcome := <-completion:
		return outcome, nil
	case <-time.After(j.hardTimeout):
		return nil, fmt.Errorf("scheduled turn timed out after %s", j.hardTimeout)
	}
}
