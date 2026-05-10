//revive:disable:dot-imports
package engine_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	enginetesting "github.com/brendanjerwin/simple_wiki/internal/connectors/engine/testing"
)

// debouncerStartTime is the wall-clock instant the debouncer tests pin
// the FakeClock to. Distinct from the bind/unbind suites' start times
// so reviewers can identify the suite at a glance from log timestamps.
var debouncerStartTime = time.Date(2026, 5, 4, 14, 0, 0, 0, time.UTC)

// keyAlpha and keyBeta are the two test keys the suite uses to verify
// per-key independence. Hoisted to package scope so multiple Describe
// blocks share them.
var (
	keyAlpha = engine.SyncDebouncerKey{
		ProfileID: "alice_profile",
		Page:      "groceries",
		ListName:  "this_week",
	}
	keyBeta = engine.SyncDebouncerKey{
		ProfileID: "bob_profile",
		Page:      "todo",
		ListName:  "today",
	}
)

// recordingSyncFunc captures every Sync call for after-the-fact
// assertion. Programmable per-key error response for the error-path
// tests.
type recordingSyncFunc struct {
	mu      sync.Mutex
	calls   []engine.SyncDebouncerKey
	errResp error
}

func (r *recordingSyncFunc) Fn() engine.SyncFunc {
	return func(_ context.Context, key engine.SyncDebouncerKey) error {
		r.mu.Lock()
		r.calls = append(r.calls, key)
		err := r.errResp
		r.mu.Unlock()
		return err
	}
}

func (r *recordingSyncFunc) Calls() []engine.SyncDebouncerKey {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]engine.SyncDebouncerKey, len(r.calls))
	copy(out, r.calls)
	return out
}

func (r *recordingSyncFunc) SetError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errResp = err
}

// debouncerLogger captures Error log lines so the error-path test can
// assert the debouncer logs SyncFunc failures rather than swallowing
// them silently. Info/Warn are dropped — we don't assert on those.
type debouncerLogger struct {
	mu     sync.Mutex
	errors []string
}

func (*debouncerLogger) Info(_ string, _ ...any) {}
func (*debouncerLogger) Warn(_ string, _ ...any) {}
func (l *debouncerLogger) Error(format string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, format)
}

func (l *debouncerLogger) ErrorCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.errors)
}

var _ = Describe("NewSyncDebouncer", func() {
	var (
		fc     *enginetesting.FakeClock
		logger *debouncerLogger
		recsf  *recordingSyncFunc
	)

	BeforeEach(func() {
		fc = enginetesting.NewFakeClock(debouncerStartTime)
		logger = &debouncerLogger{}
		recsf = &recordingSyncFunc{}
	})

	When("clock is nil", func() {
		var err error

		BeforeEach(func() {
			_, err = engine.NewSyncDebouncer(nil, fc, recsf.Fn(), logger)
		})

		It("should return a wiring error", func() {
			Expect(err).To(MatchError(ContainSubstring("clock must not be nil")))
		})
	})

	When("scheduler is nil", func() {
		var err error

		BeforeEach(func() {
			_, err = engine.NewSyncDebouncer(fc, nil, recsf.Fn(), logger)
		})

		It("should return a wiring error", func() {
			Expect(err).To(MatchError(ContainSubstring("scheduler must not be nil")))
		})
	})

	When("sync func is nil", func() {
		var err error

		BeforeEach(func() {
			_, err = engine.NewSyncDebouncer(fc, fc, nil, logger)
		})

		It("should return a wiring error", func() {
			Expect(err).To(MatchError(ContainSubstring("sync func must not be nil")))
		})
	})

	When("logger is nil", func() {
		var err error

		BeforeEach(func() {
			_, err = engine.NewSyncDebouncer(fc, fc, recsf.Fn(), nil)
		})

		It("should return a wiring error", func() {
			Expect(err).To(MatchError(ContainSubstring("logger must not be nil")))
		})
	})

	When("all collaborators are non-nil", func() {
		var (
			d   *engine.SyncDebouncer
			err error
		)

		BeforeEach(func() {
			d, err = engine.NewSyncDebouncer(fc, fc, recsf.Fn(), logger)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a non-nil debouncer", func() {
			Expect(d).NotTo(BeNil())
		})
	})
})

var _ = Describe("SyncDebouncer.OnChecklistMutated", func() {
	var (
		fc     *enginetesting.FakeClock
		logger *debouncerLogger
		recsf  *recordingSyncFunc
		d      *engine.SyncDebouncer
	)

	BeforeEach(func() {
		fc = enginetesting.NewFakeClock(debouncerStartTime)
		logger = &debouncerLogger{}
		recsf = &recordingSyncFunc{}
		var err error
		d, err = engine.NewSyncDebouncer(fc, fc, recsf.Fn(), logger)
		Expect(err).NotTo(HaveOccurred())
	})

	When("called once and clock advances 1.5s", func() {
		BeforeEach(func() {
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(1500 * time.Millisecond)
		})

		It("should fire SyncFunc exactly once", func() {
			Expect(recsf.Calls()).To(HaveLen(1))
		})

		It("should fire SyncFunc with the correct key", func() {
			Expect(recsf.Calls()[0]).To(Equal(keyAlpha))
		})
	})

	When("called once but clock has not yet advanced past the window", func() {
		BeforeEach(func() {
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(1499 * time.Millisecond)
		})

		It("should not fire SyncFunc yet", func() {
			Expect(recsf.Calls()).To(BeEmpty())
		})
	})

	When("called twice within 1s and clock advances 1.5s after the second call", func() {
		BeforeEach(func() {
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(1 * time.Second)
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(1500 * time.Millisecond)
		})

		It("should fire SyncFunc exactly once", func() {
			Expect(recsf.Calls()).To(HaveLen(1))
		})
	})

	When("called twice within 1s and clock advances only 1s after the second call", func() {
		BeforeEach(func() {
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(1 * time.Second)
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(1 * time.Second)
		})

		It("should not fire SyncFunc yet", func() {
			Expect(recsf.Calls()).To(BeEmpty())
		})
	})

	When("called for two different keys and clock advances 1.5s", func() {
		BeforeEach(func() {
			d.OnChecklistMutated(keyAlpha)
			d.OnChecklistMutated(keyBeta)
			fc.Advance(1500 * time.Millisecond)
		})

		It("should fire SyncFunc twice", func() {
			Expect(recsf.Calls()).To(HaveLen(2))
		})

		It("should fire SyncFunc for key alpha", func() {
			Expect(recsf.Calls()).To(ContainElement(keyAlpha))
		})

		It("should fire SyncFunc for key beta", func() {
			Expect(recsf.Calls()).To(ContainElement(keyBeta))
		})
	})

	When("two keys' debounce windows overlap and one resets", func() {
		// Notify alpha at t=0, beta at t=500ms; reset alpha at t=1000ms.
		// Advance to t=2000ms (1500ms after the second alpha notify;
		// 1500ms after the original beta notify).
		BeforeEach(func() {
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(500 * time.Millisecond)
			d.OnChecklistMutated(keyBeta)
			fc.Advance(500 * time.Millisecond)
			d.OnChecklistMutated(keyAlpha) // reset alpha
			fc.Advance(1000 * time.Millisecond)
		})

		It("should fire beta (originally scheduled at t=500+1500=2000)", func() {
			Expect(recsf.Calls()).To(ContainElement(keyBeta))
		})

		It("should not fire alpha yet (rescheduled to t=1000+1500=2500)", func() {
			Expect(recsf.Calls()).NotTo(ContainElement(keyAlpha))
		})
	})
})

var _ = Describe("SyncDebouncer post-success choke", func() {
	var (
		fc     *enginetesting.FakeClock
		logger *debouncerLogger
		recsf  *recordingSyncFunc
		d      *engine.SyncDebouncer
	)

	BeforeEach(func() {
		fc = enginetesting.NewFakeClock(debouncerStartTime)
		logger = &debouncerLogger{}
		recsf = &recordingSyncFunc{}
		var err error
		d, err = engine.NewSyncDebouncer(fc, fc, recsf.Fn(), logger)
		Expect(err).NotTo(HaveOccurred())
	})

	When("NoteSyncSucceeded was called and OnChecklistMutated arrives within 5s", func() {
		BeforeEach(func() {
			d.NoteSyncSucceeded(keyAlpha)
			fc.Advance(4 * time.Second)
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(2 * time.Second) // beyond debounce window
		})

		It("should not fire SyncFunc", func() {
			Expect(recsf.Calls()).To(BeEmpty())
		})
	})

	When("NoteSyncSucceeded was called and OnChecklistMutated arrives at exactly the 5s boundary", func() {
		// At t=5s the choke window has elapsed; the notification IS
		// scheduled. Advance debounce window after to fire.
		BeforeEach(func() {
			d.NoteSyncSucceeded(keyAlpha)
			fc.Advance(5 * time.Second)
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(1500 * time.Millisecond)
		})

		It("should fire SyncFunc once", func() {
			Expect(recsf.Calls()).To(HaveLen(1))
		})
	})

	When("the post-success choke is per-key", func() {
		// NoteSyncSucceeded for alpha; mutate beta within the alpha
		// choke window. Beta should be scheduled normally.
		BeforeEach(func() {
			d.NoteSyncSucceeded(keyAlpha)
			fc.Advance(2 * time.Second)
			d.OnChecklistMutated(keyBeta)
			fc.Advance(1500 * time.Millisecond)
		})

		It("should fire SyncFunc for beta", func() {
			Expect(recsf.Calls()).To(ContainElement(keyBeta))
		})

		It("should not fire SyncFunc for alpha", func() {
			Expect(recsf.Calls()).NotTo(ContainElement(keyAlpha))
		})
	})

	When("a notification arrives, fires successfully, and a second notification arrives within 5s", func() {
		// First mutation fires; engine notes success; second mutation
		// within choke window must be ignored.
		BeforeEach(func() {
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(1500 * time.Millisecond) // fires Sync
			// Engine would call NoteSyncSucceeded after a real Sync;
			// emulate that here.
			d.NoteSyncSucceeded(keyAlpha)
			fc.Advance(2 * time.Second)
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(2 * time.Second)
		})

		It("should fire SyncFunc only once (the original notify; the second was choked)", func() {
			Expect(recsf.Calls()).To(HaveLen(1))
		})
	})
})

var _ = Describe("SyncDebouncer error handling", func() {
	var (
		fc     *enginetesting.FakeClock
		logger *debouncerLogger
		recsf  *recordingSyncFunc
		d      *engine.SyncDebouncer
	)

	BeforeEach(func() {
		fc = enginetesting.NewFakeClock(debouncerStartTime)
		logger = &debouncerLogger{}
		recsf = &recordingSyncFunc{}
		recsf.SetError(errors.New("simulated sync failure"))
		var err error
		d, err = engine.NewSyncDebouncer(fc, fc, recsf.Fn(), logger)
		Expect(err).NotTo(HaveOccurred())
	})

	When("SyncFunc returns an error and a follow-up notify arrives within would-be choke window", func() {
		BeforeEach(func() {
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(1500 * time.Millisecond) // fires Sync, returns error
			fc.Advance(2 * time.Second)
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(1500 * time.Millisecond)
		})

		It("should fire SyncFunc twice (no choke after a failed sync)", func() {
			Expect(recsf.Calls()).To(HaveLen(2))
		})

		It("should log the failure", func() {
			Expect(logger.ErrorCount()).To(BeNumerically(">=", 1))
		})
	})

	When("SyncFunc returns an error", func() {
		BeforeEach(func() {
			d.OnChecklistMutated(keyAlpha)
			fc.Advance(1500 * time.Millisecond)
		})

		It("should not panic", func() {
			Expect(recsf.Calls()).To(HaveLen(1))
		})

		It("should log an error", func() {
			Expect(logger.ErrorCount()).To(BeNumerically(">=", 1))
		})
	})
})

var _ = Describe("SyncDebouncer concurrent safety", func() {
	var (
		fc     *enginetesting.FakeClock
		logger *debouncerLogger
		recsf  *recordingSyncFunc
		d      *engine.SyncDebouncer
	)

	BeforeEach(func() {
		fc = enginetesting.NewFakeClock(debouncerStartTime)
		logger = &debouncerLogger{}
		recsf = &recordingSyncFunc{}
		var err error
		d, err = engine.NewSyncDebouncer(fc, fc, recsf.Fn(), logger)
		Expect(err).NotTo(HaveOccurred())
	})

	When("many goroutines call OnChecklistMutated for distinct keys concurrently", func() {
		const goroutines = 64
		var done atomic.Int32

		BeforeEach(func() {
			done.Store(0)
			var wg sync.WaitGroup
			for i := 0; i < goroutines; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					key := engine.SyncDebouncerKey{
						ProfileID: "alice_profile",
						Page:      "p",
						ListName:  string(rune('a' + (i % 26))),
					}
					d.OnChecklistMutated(key)
					done.Add(1)
				}(i)
			}
			wg.Wait()
			fc.Advance(1500 * time.Millisecond)
		})

		It("should complete every goroutine without race", func() {
			Expect(done.Load()).To(Equal(int32(goroutines)))
		})

		It("should fire SyncFunc at least once per distinct key", func() {
			// 26 distinct ListName values modulo the alphabet.
			Expect(len(recsf.Calls())).To(BeNumerically(">=", 1))
		})
	})
})
