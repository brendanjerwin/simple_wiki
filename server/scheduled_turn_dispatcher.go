package server

import (
	"errors"
	"fmt"
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

// ScheduledTurnDispatcher is the server-side bridge between AgentTurnJob and
// the pool's headless scheduled-turn handler. It maintains a fan-out queue of
// dispatched requests so the pool subscriber can read them, and a per-request
// completion channel so the originating job can wait on the outcome.
type ScheduledTurnDispatcher struct {
	mu          sync.Mutex
	subscribers map[int]chan *apiv1.ScheduledTurnRequest
	pending     map[string]chan *ScheduledTurnOutcome
	nextSubID   int
}

// NewScheduledTurnDispatcher constructs an empty dispatcher.
func NewScheduledTurnDispatcher() *ScheduledTurnDispatcher {
	return &ScheduledTurnDispatcher{
		subscribers: map[int]chan *apiv1.ScheduledTurnRequest{},
		pending:     map[string]chan *ScheduledTurnOutcome{},
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
	d.pending[req.GetRequestId()] = completion

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
	completion, ok := d.pending[req.GetRequestId()]
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf("no pending scheduled turn with request_id %q", req.GetRequestId())
	}
	delete(d.pending, req.GetRequestId())
	d.mu.Unlock()

	completion <- &ScheduledTurnOutcome{
		TerminalStatus:  req.GetTerminalStatus(),
		ErrorMessage:    req.GetErrorMessage(),
		DurationSeconds: req.GetDurationSeconds(),
	}
	close(completion)
	return nil
}
