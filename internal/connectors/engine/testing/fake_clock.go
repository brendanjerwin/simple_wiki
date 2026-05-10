package testing

import (
	"slices"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
)

// FakeClock is a programmable engine.Clock + engine.TimerScheduler for
// unit tests. The engine's Bind ceremony stamps Binding.BoundAt with
// clock.Now(); tests assert that value equals the configured Now to
// verify the engine consults the clock seam (rather than calling
// time.Now directly).
//
// Beyond Now/SetNow, FakeClock implements engine.TimerScheduler:
// AfterFunc registers a deadline-keyed timer; Advance moves the clock
// forward and fires (synchronously, on the calling goroutine) every
// timer whose deadline is at or before the new now. This matches the
// sinon-style fake-clock contract: tests control time deterministically
// without sleeping.
//
// Why fire synchronously: tests want a happens-before edge between
// "Advance returned" and "the fired callback's side effects are
// observable". A goroutine-fired callback would force tests to
// Eventually-poll, defeating the purpose of a fake clock. Note that
// the SyncDebouncer's production wiring expects fn to run on its own
// goroutine (matching time.AfterFunc); the fake's synchronous firing
// is a test-only convenience and the debouncer must not rely on
// goroutine isolation for correctness.
//
// Usage:
//
//	fc := testing.NewFakeClock(time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC))
//	debouncer, _ := engine.NewSyncDebouncer(fc, fc, syncFn, logger)
//	debouncer.OnChecklistMutated(key)
//	fc.Advance(1500 * time.Millisecond) // fires the debounce timer
type FakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer // sorted by deadline ascending
}

// fakeTimer is the test-side handle returned by FakeClock.AfterFunc.
// Fields under FakeClock.mu.
type fakeTimer struct {
	deadline time.Time
	fn       func()
	stopped  bool
	fired    bool
	clock    *FakeClock
}

// NewFakeClock constructs a FakeClock pinned to the given time.
func NewFakeClock(now time.Time) *FakeClock {
	return &FakeClock{now: now}
}

// Now returns the currently configured time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// SetNow updates the time the clock reports on subsequent Now calls.
// Does NOT fire pending timers — use Advance for that semantics. Kept
// because earlier engine tests (bind, force_resync) used SetNow as
// pure clock manipulation without timer semantics.
func (c *FakeClock) SetNow(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// Advance moves the clock forward by d and fires every pending timer
// whose deadline is at or before the new now, in deadline order. Each
// callback runs synchronously on the calling goroutine; tests get a
// happens-before edge from "Advance returned" to "fired callbacks'
// side effects observable".
//
// Calling Advance(0) is a no-op. Negative durations panic — moving
// the fake clock backwards would invalidate timer ordering invariants
// and almost certainly indicates a test bug.
func (c *FakeClock) Advance(d time.Duration) {
	if d < 0 {
		panic("FakeClock.Advance: negative duration")
	}
	c.mu.Lock()
	c.now = c.now.Add(d)
	due := c.dueTimersLocked()
	c.mu.Unlock()
	for _, t := range due {
		t.fn()
	}
}

// AfterFunc implements engine.TimerScheduler. Registers a timer to
// fire fn after d, returning a handle the caller can Stop. If d is
// zero or negative, fires immediately (matching time.AfterFunc).
func (c *FakeClock) AfterFunc(d time.Duration, fn func()) engine.Timer {
	c.mu.Lock()
	t := &fakeTimer{
		deadline: c.now.Add(d),
		fn:       fn,
		clock:    c,
	}
	c.timers = append(c.timers, t)
	slices.SortStableFunc(c.timers, func(a, b *fakeTimer) int {
		switch {
		case a.deadline.Before(b.deadline):
			return -1
		case a.deadline.After(b.deadline):
			return 1
		default:
			return 0
		}
	})
	immediate := !c.now.Before(t.deadline) // d <= 0
	if immediate {
		// Mark fired before unlocking so Stop after fire is a no-op.
		t.fired = true
	}
	c.mu.Unlock()
	if immediate {
		fn()
	}
	return fakeTimerHandle{inner: t}
}

// dueTimersLocked drains every timer whose deadline is <= c.now and
// returns them in deadline order. Marks each as fired; Stop on a
// fired timer returns false.
//
// INVARIANT ASSERTION: caller holds c.mu. Panics if called without
// the lock — TryLock succeeds only when the lock is free, so a true
// return indicates the invariant is violated.
func (c *FakeClock) dueTimersLocked() []*fakeTimer {
	if c.mu.TryLock() {
		c.mu.Unlock()
		panic("FakeClock.dueTimersLocked called without holding mu - this is a programming bug")
	}
	var due []*fakeTimer
	var remaining []*fakeTimer
	for _, t := range c.timers {
		if t.stopped || t.fired {
			continue
		}
		if !t.deadline.After(c.now) {
			t.fired = true
			due = append(due, t)
			continue
		}
		remaining = append(remaining, t)
	}
	c.timers = remaining
	return due
}

// fakeTimerHandle is the engine.Timer-satisfying cancel handle
// returned by AfterFunc. Concrete type is unexported because tests
// don't need to inspect it directly — they only call Stop, which the
// interface exposes.
type fakeTimerHandle struct {
	inner *fakeTimer
}

// Stop prevents the timer from firing. Returns true if the call stops
// the timer (i.e., it had not yet fired or been stopped), false
// otherwise. Mirrors time.Timer.Stop and engine.Timer.Stop.
func (h fakeTimerHandle) Stop() bool {
	h.inner.clock.mu.Lock()
	defer h.inner.clock.mu.Unlock()
	if h.inner.stopped || h.inner.fired {
		return false
	}
	h.inner.stopped = true
	return true
}

// Compile-time check: *FakeClock satisfies engine.TimerScheduler.
var _ engine.TimerScheduler = (*FakeClock)(nil)
var _ engine.Timer = fakeTimerHandle{}
var _ engine.Clock = (*FakeClock)(nil)
