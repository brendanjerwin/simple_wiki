package server

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// ScheduledTurnOutcome carries the terminal result of one scheduled-agent turn
// from the pool back to the AgentTurnJob that dispatched it.
type ScheduledTurnOutcome struct {
	TerminalStatus  apiv1.ScheduleStatus
	ErrorMessage    string
	DurationSeconds int32
}

// pendingTurn pairs the completion channel for a dispatched request with the
// original request. Storing the request allows DrainPending to emit descriptive
// log messages and ERROR outcomes without a separate lookup.
type pendingTurn struct {
	completion chan *ScheduledTurnOutcome
	request    *apiv1.ScheduledTurnRequest
}

// ScheduledTurnDispatcher is the server-side bridge between AgentTurnJob and
// the pool's headless scheduled-turn handler. It maintains a fan-out queue of
// dispatched requests so the pool subscriber can read them, and a per-request
// completion channel so the originating job can wait on the outcome.
type ScheduledTurnDispatcher struct {
	mu          sync.Mutex
	subscribers map[int]chan *apiv1.ScheduledTurnRequest
	pending     map[string]pendingTurn
	nextSubID   int
}

// NewScheduledTurnDispatcher constructs an empty dispatcher.
func NewScheduledTurnDispatcher() *ScheduledTurnDispatcher {
	return &ScheduledTurnDispatcher{
		subscribers: map[int]chan *apiv1.ScheduledTurnRequest{},
		pending:     map[string]pendingTurn{},
	}
}

// Subscribe registers a new pool subscriber. The returned channel receives
// every dispatched request; the caller should range over it (typically in a
// goroutine that forwards to a gRPC server stream). Calling unsubscribe stops
// further deliveries and closes the channel.
func (d *ScheduledTurnDispatcher) Subscribe() (<-chan *apiv1.ScheduledTurnRequest, func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	id := d.nextSubID
	d.nextSubID++
	const subBuffer = 16
	ch := make(chan *apiv1.ScheduledTurnRequest, subBuffer)
	d.subscribers[id] = ch

	unsubscribe := func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		if existing, ok := d.subscribers[id]; ok {
			delete(d.subscribers, id)
			close(existing)
		}
	}
	return ch, unsubscribe
}

// Dispatch publishes req to all current subscribers and registers a per-request
// completion channel. AgentTurnJob blocks on this channel until Complete (or
// some upstream timeout) fires.
//
// Returns an error if there are no subscribers; the caller (AgentTurnJob)
// records this as a terminal ERROR status without ever transitioning to
// RUNNING.
func (d *ScheduledTurnDispatcher) Dispatch(req *apiv1.ScheduledTurnRequest) (<-chan *ScheduledTurnOutcome, error) {
	if req == nil {
		return nil, errors.New("request is required")
	}
	if req.GetRequestId() == "" {
		return nil, errors.New("request_id is required")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.subscribers) == 0 {
		return nil, errors.New("no scheduled-turn subscribers connected (pool not running?)")
	}
	if _, exists := d.pending[req.GetRequestId()]; exists {
		return nil, fmt.Errorf("scheduled turn %q already in flight", req.GetRequestId())
	}

	completion := make(chan *ScheduledTurnOutcome, 1)
	d.pending[req.GetRequestId()] = pendingTurn{completion: completion, request: req}

	for _, sub := range d.subscribers {
		select {
		case sub <- req:
		default:
			// Subscriber buffer is full; the pool is overwhelmed. Skip this
			// subscriber rather than blocking server work. With a 16-slot
			// buffer this only happens under pathological backlog.
		}
	}
	return completion, nil
}

// Complete delivers a terminal outcome for the given request_id. Returns an
// error for orphan completions (no matching pending request) or duplicate
// completions for an already-resolved request.
func (d *ScheduledTurnDispatcher) Complete(req *apiv1.CompleteScheduledTurnRequest) error {
	if req == nil {
		return errors.New("request is required")
	}
	if req.GetRequestId() == "" {
		return errors.New("request_id is required")
	}

	d.mu.Lock()
	pt, ok := d.pending[req.GetRequestId()]
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf("no pending scheduled turn with request_id %q", req.GetRequestId())
	}
	delete(d.pending, req.GetRequestId())
	d.mu.Unlock()

	pt.completion <- &ScheduledTurnOutcome{
		TerminalStatus:  req.GetTerminalStatus(),
		ErrorMessage:    req.GetErrorMessage(),
		DurationSeconds: req.GetDurationSeconds(),
	}
	close(pt.completion)
	return nil
}

// DrainPending terminates all in-flight scheduled turns with an ERROR outcome.
// Each pending AgentTurnJob's completion channel receives an error outcome
// carrying reason as the error message, allowing the job to write a terminal
// status to the schedule's frontmatter before the process exits.
//
// Returns the number of turns drained. Safe to call concurrently and multiple
// times; subsequent calls are no-ops when the pending map is already empty.
//
// Typical call site: the process shutdown cleanup function, so that a
// binary-swap restart does not leave schedules perpetually RUNNING (zombie)
// when the pool later calls CompleteScheduledTurn on the fresh process and
// finds no matching pending record.
func (d *ScheduledTurnDispatcher) DrainPending(reason string) int {
	d.mu.Lock()
	toSignal := make([]pendingTurn, 0, len(d.pending))
	for id, pt := range d.pending {
		toSignal = append(toSignal, pt)
		delete(d.pending, id)
	}
	d.mu.Unlock()

	for _, pt := range toSignal {
		slog.Error("scheduled turn dispatcher: draining pending turn on shutdown",
			"request_id", pt.request.GetRequestId(),
			"page", pt.request.GetPage(),
			"reason", reason,
		)
		pt.completion <- &ScheduledTurnOutcome{
			TerminalStatus: apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR,
			ErrorMessage:   fmt.Sprintf("process shutting down: %s", reason),
		}
		close(pt.completion)
	}
	return len(toSignal)
}
