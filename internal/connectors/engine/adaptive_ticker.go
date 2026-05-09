package engine

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
)

// AdaptiveTickerBaseDelay is the most-reactive follow-up interval. After
// a successful Sync that observed activity (cursor advanced), the next
// follow-up is scheduled at this delay. Subsequent quiet runs decay the
// interval exponentially up to AdaptiveTickerCapDelay; once the next
// computed delay would meet or exceed the cron tick interval, no
// follow-up is scheduled and the 30s scheduler is the steady-state.
//
// Exposed as `var` (not `const`) so adapter-level integration tests can
// shorten it; production reads the same shared value uniformly.
var AdaptiveTickerBaseDelay = 5 * time.Second

// AdaptiveTickerCapDelay is the largest follow-up interval the ticker
// will schedule. Beyond this, the ticker yields to the 30s cron. The
// effective sequence is 5s, 10s, 20s, then yield.
var AdaptiveTickerCapDelay = 20 * time.Second

// AdaptiveTicker schedules per-binding follow-up syncs after every
// successful Sync. The decay policy: activity → 5s; one quiet run →
// 10s; two quiet runs → 20s; three quiet runs → no follow-up (cron
// handles the steady-state). After observed activity the counter
// resets, so a single new change re-triggers the most-reactive
// schedule.
//
// Per the round-3 panel discussion of §11.5 (the legacy 5s
// rateLimitChoke), the AdaptiveTicker is the principled replacement:
// rather than choking debouncer-driven syncs to keep the cron the
// authoritative driver, it actively schedules reactive follow-ups
// while activity is observed and yields to the cron when it isn't.
//
// Safe for concurrent use across the scheduler tick, debouncer fires,
// and the ticker's own follow-up goroutines.
type AdaptiveTicker struct {
	scheduler TimerScheduler
	syncFn    func(ctx context.Context, key connectors.BindingKey)
	logger    Logger

	mu        sync.Mutex
	timers    map[connectors.BindingKey]Timer
	quietRuns map[connectors.BindingKey]int
}

// NewAdaptiveTicker constructs an AdaptiveTicker. scheduler drives the
// follow-up timers (production: time.AfterFunc; tests: FakeClock).
// syncFn is invoked when a follow-up timer fires; the engine binds
// this to its own Sync method so the ticker can re-enter the
// reconcile path. Errors are logged and ignored — a failed follow-up
// will be retried on the next cron tick.
func NewAdaptiveTicker(
	scheduler TimerScheduler,
	syncFn func(ctx context.Context, key connectors.BindingKey),
	logger Logger,
) (*AdaptiveTicker, error) {
	if scheduler == nil {
		return nil, errors.New("connectors/engine: AdaptiveTicker scheduler must not be nil")
	}
	if syncFn == nil {
		return nil, errors.New("connectors/engine: AdaptiveTicker syncFn must not be nil")
	}
	if logger == nil {
		return nil, errors.New("connectors/engine: AdaptiveTicker logger must not be nil")
	}
	return &AdaptiveTicker{
		scheduler: scheduler,
		syncFn:    syncFn,
		logger:    logger,
		timers:    map[connectors.BindingKey]Timer{},
		quietRuns: map[connectors.BindingKey]int{},
	}, nil
}

// RecordTick records the result of a completed Sync for the binding
// and decides whether to schedule a follow-up.
//
//   - hadActivity=true: reset the quiet counter; schedule a follow-up
//     at AdaptiveTickerBaseDelay (the most-reactive interval).
//   - hadActivity=false: increment the quiet counter; schedule a
//     follow-up at base × 2^quietRuns. Beyond AdaptiveTickerCapDelay,
//     no follow-up is scheduled and the cron handles the steady-state.
//
// Cancels any prior pending follow-up for the binding before
// scheduling the new one — the most recent RecordTick wins.
//
// `hadActivity` is the activity signal (cursor advanced during the
// caller's reconcile). It's not control-flag coupling — splitting
// into RecordActiveTick/RecordQuietTick would just duplicate the
// supersede-and-schedule logic that's identical apart from one
// counter increment.
//
//revive:disable-next-line:flag-parameter
func (a *AdaptiveTicker) RecordTick(key connectors.BindingKey, hadActivity bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if hadActivity {
		a.quietRuns[key] = 0
	} else {
		a.quietRuns[key]++
	}
	runs := a.quietRuns[key]

	if existing, ok := a.timers[key]; ok {
		existing.Stop()
		delete(a.timers, key)
	}

	delay := computeFollowUpDelay(runs)
	if delay <= 0 {
		a.logger.Info("connectors/engine: adaptive_follow_up_yielded key=%s|%s|%s quiet_runs=%d",
			key.ProfileID, key.Page, key.ListName, runs)
		return
	}

	keyCopy := key
	timer := a.scheduler.AfterFunc(delay, func() {
		a.fire(keyCopy)
	})
	a.timers[keyCopy] = timer
	a.logger.Info("connectors/engine: adaptive_follow_up_scheduled key=%s|%s|%s delay=%s quiet_runs=%d activity=%t",
		key.ProfileID, key.Page, key.ListName, delay, runs, hadActivity)
}

// CancelKey stops any pending follow-up for the binding and clears its
// quiet-run counter. Called on Unbind so the ticker doesn't fire
// follow-ups for an unbound key.
func (a *AdaptiveTicker) CancelKey(key connectors.BindingKey) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if existing, ok := a.timers[key]; ok {
		existing.Stop()
		delete(a.timers, key)
	}
	delete(a.quietRuns, key)
}

// fire is the timer callback. Clears the timer entry, then invokes
// syncFn. The Sync's own RecordTick call (after reconcile completes)
// will re-arm or yield based on whether the follow-up sync observed
// activity.
func (a *AdaptiveTicker) fire(key connectors.BindingKey) {
	a.mu.Lock()
	delete(a.timers, key)
	a.mu.Unlock()
	a.syncFn(context.Background(), key)
}

// maxQuietRunsBeforeShiftOverflow is the largest quietRuns value
// computeFollowUpDelay will shift base by before bailing. Past this
// the left-shift would overflow on 64-bit ints (treating
// AdaptiveTickerBaseDelay as a small positive duration) — but we'd
// have yielded to cron well before reaching it via the cap check
// anyway. Defensive cap keeps the math safe.
const maxQuietRunsBeforeShiftOverflow = 8

// computeFollowUpDelay returns the next follow-up interval given the
// quiet-run counter, or 0 to signal "no follow-up; yield to cron."
//
// Sequence: quietRuns=0 → base; quietRuns=1 → base*2; quietRuns=2 →
// base*4; beyond, the delay would exceed the cap and we yield. With
// base=5s, cap=20s the effective ladder is 5s, 10s, 20s, yield.
func computeFollowUpDelay(quietRuns int) time.Duration {
	if quietRuns < 0 {
		quietRuns = 0
	}
	if quietRuns > maxQuietRunsBeforeShiftOverflow {
		return 0
	}
	delay := AdaptiveTickerBaseDelay << quietRuns
	if delay > AdaptiveTickerCapDelay {
		return 0
	}
	return delay
}
