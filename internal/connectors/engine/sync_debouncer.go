package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// debounceWindow is the minimum quiet period after the last
// OnChecklistMutated call before SyncDebouncer fires the registered
// SyncFunc. Per MATRIX.md row 9, both pre-extraction connectors used
// 1.5s; the engine adopts that uniformly.
const debounceWindow = 1500 * time.Millisecond

// postSyncChokeWindow is the minimum gap between a successful Sync
// and the next debouncer-driven Sync for the same key. Per MATRIX.md
// row 9, Tasks shipped this 5s choke; under strictest-behavior-wins
// the engine adopts it for every adapter.
//
// Why a choke at all: the unified 30s scheduler tick reaches every
// binding on every fire; back-to-back wiki edits at 1.6–4.9s spacing
// would otherwise trigger their own debouncer-driven Sync immediately
// on top of a just-completed cron-driven Sync, racing on the same
// (page, listName) pair.
const postSyncChokeWindow = 5 * time.Second

// SyncDebouncerKey is the per-binding key the debouncer uses to track
// pending timers and post-success choke windows. Distinct from
// connectors.SubscriptionKey because the wiki-side mutator notify
// already provides ProfileID + page + listName as raw strings — no
// need to round-trip through wikipage.PageIdentifier.
type SyncDebouncerKey struct {
	ProfileID string
	Page      string
	ListName  string
}

// SyncFunc is the callback the debouncer invokes when a key's debounce
// window elapses without further mutation notifications. The engine's
// production wiring binds this to its per-tick reconcile for the
// supplied key.
//
// Errors are logged by the debouncer and do NOT start the post-sync
// choke window — a failed Sync should be retryable on the next
// notification without artificial delay.
type SyncFunc func(ctx context.Context, key SyncDebouncerKey) error

// TimerScheduler is the testable seam the SyncDebouncer uses to
// schedule timed callbacks. Production: time.AfterFunc-backed.
// Tests: FakeClock-driven, where Advance fires any timer whose
// deadline is at or before the new clock time.
//
// Why a separate interface from Clock: extending Clock with NewTimer
// would invalidate every existing engine test stub (stubClock,
// FakeClock callers in bind / unbind / force_resync). The debouncer
// is a standalone struct in Phase 3h with no Engine wiring, so a
// narrow new dependency is the lowest-risk shape. FakeClock
// implements both Clock and TimerScheduler so a single fake covers
// both seams once the debouncer wires into Engine in a later phase.
type TimerScheduler interface {
	// AfterFunc schedules fn to run after d. Returns a Timer the
	// caller can Stop to cancel. The contract matches time.AfterFunc:
	// fn runs on its own goroutine; Stop returns true if the call
	// stops the timer before fn fired, false otherwise.
	AfterFunc(d time.Duration, fn func()) Timer
}

// Timer is the cancel handle the SyncDebouncer holds for each
// outstanding debounce window.
type Timer interface {
	// Stop prevents the timer from firing. Returns true if the call
	// stops the timer, false if the timer has already expired or been
	// stopped. Mirrors time.Timer.Stop.
	Stop() bool
}

// SyncDebouncer turns a stream of "checklist X mutated" notifications
// into at-most-one Sync per (profileID, page, listName) per
// debounceWindow, with a post-success choke that suppresses further
// Sync attempts for postSyncChokeWindow after a successful run.
//
// Per MATRIX.md row 9 the debouncer is engine-owned (one instance
// shared across all adapters) and implements strictest-behavior-wins:
// the 1.5s window comes from both legacy connectors; the 5s
// post-success choke comes from Tasks; Keep gains the choke under
// extraction.
//
// Safe for concurrent use across the wiki mutator's notify goroutine,
// the debouncer's own AfterFunc-driven goroutine, and the engine's
// post-sync NoteSyncSucceeded calls.
type SyncDebouncer struct {
	clock     Clock
	scheduler TimerScheduler
	syncFn    SyncFunc
	logger    Logger

	mu               sync.Mutex
	timers           map[SyncDebouncerKey]Timer
	lastSuccessfulAt map[SyncDebouncerKey]time.Time
}

// NewSyncDebouncer constructs a SyncDebouncer wired to the given
// clock (consulted for post-success choke math), timer scheduler
// (consulted for the debounce window), sync function (invoked when
// the window elapses), and logger.
//
// Returns an error if any collaborator is nil — a missing dependency
// is a wiring bug at startup, not a runtime concern.
func NewSyncDebouncer(
	clock Clock,
	scheduler TimerScheduler,
	syncFn SyncFunc,
	logger Logger,
) (*SyncDebouncer, error) {
	if clock == nil {
		return nil, errors.New("connectors/engine: SyncDebouncer clock must not be nil")
	}
	if scheduler == nil {
		return nil, errors.New("connectors/engine: SyncDebouncer scheduler must not be nil")
	}
	if syncFn == nil {
		return nil, errors.New("connectors/engine: SyncDebouncer sync func must not be nil")
	}
	if logger == nil {
		return nil, errors.New("connectors/engine: SyncDebouncer logger must not be nil")
	}
	return &SyncDebouncer{
		clock:            clock,
		scheduler:        scheduler,
		syncFn:           syncFn,
		logger:           logger,
		timers:           map[SyncDebouncerKey]Timer{},
		lastSuccessfulAt: map[SyncDebouncerKey]time.Time{},
	}, nil
}

// OnChecklistMutated is the wiki-side notification entry point. The
// caller is the checklist mutator's subscriber dispatch; one call
// arrives per successful checklist mutation that the engine should
// consider syncing outbound.
//
// Behavior:
//
//   - If the key is inside its post-success choke window, drop the
//     notification entirely (no timer scheduled, no log noise).
//   - If a timer is already pending for the key, Stop it and start a
//     fresh window (typing a few quick edits coalesces to one Sync).
//   - Otherwise, schedule a new timer that fires the SyncFunc after
//     debounceWindow.
//
// Concurrent safety: two notifications for the same key racing through
// this method will result in at most one live timer. The second
// caller's "any pre-existing timer? Stop it" step covers the case
// where the first caller already stored its timer; the choke re-check
// after scheduling covers a NoteSyncSucceeded racing with the
// notification.
func (d *SyncDebouncer) OnChecklistMutated(key SyncDebouncerKey) {
	d.mu.Lock()
	if d.isChokedLocked(key) {
		d.mu.Unlock()
		return
	}
	if existing, ok := d.timers[key]; ok {
		existing.Stop()
		delete(d.timers, key)
	}
	d.mu.Unlock()

	timer := d.scheduler.AfterFunc(debounceWindow, func() {
		d.fire(key)
	})

	d.mu.Lock()
	// Re-check choke under the lock. A NoteSyncSucceeded racing with
	// this scheduling call could have started a choke window after
	// our isChokedLocked check above; honoring it here keeps the
	// invariant "no Sync ever fires while choked."
	if d.isChokedLocked(key) {
		d.mu.Unlock()
		timer.Stop()
		return
	}
	// Stop any timer a concurrent OnChecklistMutated for the same key
	// installed between our lock-drop above and this re-acquire. We
	// won the race for "newest timer wins"; the older timer is
	// discarded.
	if existing, ok := d.timers[key]; ok {
		existing.Stop()
	}
	d.timers[key] = timer
	d.mu.Unlock()
}

// NoteSyncSucceeded records that a Sync completed successfully for the
// key, starting the post-success choke window. Called by the engine
// after every successful Sync — whether driven by the cron tick OR by
// the debouncer itself — so cron-driven syncs also gate subsequent
// debouncer-driven syncs.
//
// Cancels any pending debounce timer for the key (it would only
// re-fire what just succeeded).
func (d *SyncDebouncer) NoteSyncSucceeded(key SyncDebouncerKey) {
	d.mu.Lock()
	d.lastSuccessfulAt[key] = d.clock.Now()
	if existing, ok := d.timers[key]; ok {
		existing.Stop()
		delete(d.timers, key)
	}
	d.mu.Unlock()
}

// fire is the timer callback. Clears the timer entry, calls SyncFunc,
// and starts the post-success choke on success. SyncFunc errors are
// logged and intentionally do NOT start the choke — a failed sync
// should be retryable on the next mutation without artificial delay.
func (d *SyncDebouncer) fire(key SyncDebouncerKey) {
	d.mu.Lock()
	delete(d.timers, key)
	d.mu.Unlock()

	err := d.syncFn(context.Background(), key)
	if err != nil {
		d.logger.Error("connectors/engine: SyncDebouncer sync failed for %s: %v", key.String(), err)
		return
	}
	d.mu.Lock()
	d.lastSuccessfulAt[key] = d.clock.Now()
	d.mu.Unlock()
}

// isChokedLocked reports whether the key is inside its post-success
// choke window relative to clock.Now(). Caller must hold d.mu.
//
// INVARIANT ASSERTION: caller holds d.mu. Panics if called without the
// lock — TryLock succeeds only when the lock is free.
func (d *SyncDebouncer) isChokedLocked(key SyncDebouncerKey) bool {
	if d.mu.TryLock() {
		d.mu.Unlock()
		panic("connectors/engine: isChokedLocked called without holding d.mu - this is a programming bug")
	}
	last, ok := d.lastSuccessfulAt[key]
	if !ok {
		return false
	}
	elapsed := d.clock.Now().Sub(last)
	return elapsed < postSyncChokeWindow
}

// String renders the key for log lines and test diagnostics. Format
// is stable across releases — log scrapers may parse it.
func (k SyncDebouncerKey) String() string {
	return fmt.Sprintf("%s|%s|%s", k.ProfileID, k.Page, k.ListName)
}
