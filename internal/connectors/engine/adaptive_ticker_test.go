package engine

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
)

// fakeTimerScheduler is an in-package fake that records AfterFunc
// calls for assertion. Tests advance the simulated clock by invoking
// the recorded fn directly.
type fakeTimerScheduler struct {
	mu        sync.Mutex
	scheduled []scheduledTimer
}

type scheduledTimer struct {
	delay   time.Duration
	fn      func()
	stopped bool
}

func (f *fakeTimerScheduler) AfterFunc(d time.Duration, fn func()) Timer {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := &fakeTimer{idx: len(f.scheduled), parent: f}
	f.scheduled = append(f.scheduled, scheduledTimer{delay: d, fn: fn})
	return t
}

func (f *fakeTimerScheduler) lastScheduledDelay() time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.scheduled) == 0 {
		return 0
	}
	return f.scheduled[len(f.scheduled)-1].delay
}

func (f *fakeTimerScheduler) scheduledCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.scheduled)
}

type fakeTimer struct {
	idx    int
	parent *fakeTimerScheduler
}

func (f *fakeTimer) Stop() bool {
	f.parent.mu.Lock()
	defer f.parent.mu.Unlock()
	if f.parent.scheduled[f.idx].stopped {
		return false
	}
	f.parent.scheduled[f.idx].stopped = true
	return true
}

// recordingSyncFn captures keys passed to its Sync call so tests can
// assert the ticker invoked syncFn with the expected key.
type recordingSyncFn struct {
	mu    sync.Mutex
	calls []connectors.BindingKey
}

func (r *recordingSyncFn) Sync(_ context.Context, key connectors.BindingKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, key)
}

func (r *recordingSyncFn) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

var _ = Describe("AdaptiveTicker", func() {
	var (
		sched   *fakeTimerScheduler
		recSync *recordingSyncFn
		ticker  *AdaptiveTicker
		key     connectors.BindingKey
	)

	BeforeEach(func() {
		sched = &fakeTimerScheduler{}
		recSync = &recordingSyncFn{}
		var err error
		ticker, err = NewAdaptiveTicker(sched, recSync.Sync, noopLogger{})
		Expect(err).NotTo(HaveOccurred())
		key = connectors.BindingKey{
			ProfileID: "profile_a",
			Page:      "shopping_lists",
			ListName:  "Grocery",
		}
	})

	When("RecordTick is called with hadActivity=true", func() {
		BeforeEach(func() {
			ticker.RecordTick(key, true)
		})

		It("should schedule a follow-up at the base delay (5s)", func() {
			Expect(sched.scheduledCount()).To(Equal(1))
			Expect(sched.lastScheduledDelay()).To(Equal(AdaptiveTickerBaseDelay))
		})
	})

	When("RecordTick is called with hadActivity=false multiple times (decay)", func() {
		BeforeEach(func() {
			ticker.RecordTick(key, false) // quietRuns=1 → 10s
			ticker.RecordTick(key, false) // quietRuns=2 → 20s
		})

		It("should schedule the first quiet tick at 2× base (10s)", func() {
			Expect(sched.scheduledCount()).To(BeNumerically(">=", 1))
			Expect(sched.scheduled[0].delay).To(Equal(AdaptiveTickerBaseDelay * 2))
		})

		It("should schedule the second quiet tick at 4× base (20s, the cap)", func() {
			Expect(sched.scheduledCount()).To(BeNumerically(">=", 2))
			Expect(sched.scheduled[1].delay).To(Equal(AdaptiveTickerBaseDelay * 4))
		})
	})

	When("RecordTick is called enough times that the next delay would exceed the cap", func() {
		BeforeEach(func() {
			// quietRuns goes 1, 2, 3, 4. Delays: 10s, 20s, 40s (>cap), 80s (>cap).
			ticker.RecordTick(key, false)
			ticker.RecordTick(key, false)
			ticker.RecordTick(key, false)
			ticker.RecordTick(key, false)
		})

		It("should schedule only the first two follow-ups (yields to cron beyond cap)", func() {
			Expect(sched.scheduledCount()).To(Equal(2),
				"adaptive ticker scheduled a follow-up beyond the cap; should yield to cron")
		})
	})

	When("RecordTick fires hadActivity=true after several quiet runs", func() {
		BeforeEach(func() {
			ticker.RecordTick(key, false) // quietRuns=1 → 10s
			ticker.RecordTick(key, false) // quietRuns=2 → 20s
			ticker.RecordTick(key, true)  // resets to 0 → 5s
		})

		It("should reset the delay back to base on the activity tick", func() {
			Expect(sched.scheduledCount()).To(Equal(3))
			Expect(sched.scheduled[2].delay).To(Equal(AdaptiveTickerBaseDelay),
				"adaptive ticker did not reset to base delay after observed activity")
		})
	})

	When("a follow-up timer fires", func() {
		BeforeEach(func() {
			ticker.RecordTick(key, true)
			Expect(sched.scheduledCount()).To(Equal(1))
			// Simulate the timer firing by invoking the captured fn.
			sched.scheduled[0].fn()
		})

		It("should invoke syncFn with the recorded key", func() {
			Expect(recSync.callCount()).To(Equal(1))
			Expect(recSync.calls[0]).To(Equal(key))
		})
	})

	When("RecordTick supersedes a pending timer", func() {
		BeforeEach(func() {
			ticker.RecordTick(key, true)
			ticker.RecordTick(key, true) // second call should stop the first timer
		})

		It("should mark the previous timer as stopped", func() {
			Expect(sched.scheduledCount()).To(Equal(2))
			Expect(sched.scheduled[0].stopped).To(BeTrue(),
				"adaptive ticker did not cancel the prior timer when a new RecordTick arrived")
		})
	})

	When("CancelKey is called for an active binding", func() {
		BeforeEach(func() {
			ticker.RecordTick(key, true)
			ticker.CancelKey(key)
		})

		It("should mark the active timer as stopped", func() {
			Expect(sched.scheduled[0].stopped).To(BeTrue())
		})

		It("should clear the quiet-run counter so a future RecordTick starts fresh", func() {
			ticker.RecordTick(key, false)
			Expect(sched.scheduledCount()).To(Equal(2))
			// quietRuns was reset by CancelKey; first quiet RecordTick → quietRuns=1 → 10s.
			Expect(sched.scheduled[1].delay).To(Equal(AdaptiveTickerBaseDelay * 2))
		})
	})

	When("constructed with a nil scheduler", func() {
		var newErr error

		BeforeEach(func() {
			_, newErr = NewAdaptiveTicker(nil, recSync.Sync, noopLogger{})
		})

		It("should return an error naming the missing dependency", func() {
			Expect(newErr).To(MatchError(ContainSubstring("scheduler must not be nil")))
		})
	})

	When("constructed with a nil syncFn", func() {
		var newErr error

		BeforeEach(func() {
			_, newErr = NewAdaptiveTicker(sched, nil, noopLogger{})
		})

		It("should return an error naming the missing dependency", func() {
			Expect(newErr).To(MatchError(ContainSubstring("syncFn must not be nil")))
		})
	})

	When("constructed with a nil logger", func() {
		var newErr error

		BeforeEach(func() {
			_, newErr = NewAdaptiveTicker(sched, recSync.Sync, nil)
		})

		It("should return an error naming the missing dependency", func() {
			Expect(newErr).To(MatchError(ContainSubstring("logger must not be nil")))
		})
	})
})

// noopLogger is a Logger implementation that drops all messages.
// Used by tests that don't care about log output.
type noopLogger struct{}

func (noopLogger) Info(_ string, _ ...any)  {}
func (noopLogger) Error(_ string, _ ...any) {}
func (noopLogger) Warn(_ string, _ ...any)  {}
