//revive:disable:dot-imports
package bootstrap

import (
	"context"
	"sync"
	"time"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	enginetesting "github.com/brendanjerwin/simple_wiki/internal/connectors/engine/testing"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// stubIdentity is a minimal tailscale.IdentityValue for bridge tests.
type stubIdentity struct {
	login string
}

func (s *stubIdentity) IsAnonymous() bool { return s.login == "" }
func (s *stubIdentity) LoginName() string { return s.login }
func (*stubIdentity) DisplayName() string { return "" }
func (*stubIdentity) NodeName() string    { return "" }
func (s *stubIdentity) ForLog() string    { return s.login }
func (s *stubIdentity) String() string    { return s.login }
func (*stubIdentity) IsAgent() bool       { return false }
func (s *stubIdentity) Name() string      { return s.login }

var _ tailscale.IdentityValue = (*stubIdentity)(nil)

// bridgeFixedNow is the wall-clock fixed point used by the debouncer
// fixtures in this file. Extracted to a package-level var so the date
// components don't trip revive's magic-number rule inline.
var bridgeFixedNow = time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // test fixture

// buildDebouncerWithCapture creates a SyncDebouncer + FakeClock pair
// whose syncFn stores the most recently fired key into *out.
func buildDebouncerWithCapture(out *engine.SyncDebouncerKey) (*engine.SyncDebouncer, *enginetesting.FakeClock) {
	fc := enginetesting.NewFakeClock(bridgeFixedNow)
	debouncer, _ := engine.NewSyncDebouncer(fc, fc, func(_ context.Context, key engine.SyncDebouncerKey) error {
		*out = key
		return nil
	}, lumber.NewConsoleLogger(lumber.WARN))
	return debouncer, fc
}

// advanceToFireDebounce advances the clock by 2s (> the 1.5s debounceWindow)
// so any outstanding debounce timer fires synchronously.
func advanceToFireDebounce(fc *enginetesting.FakeClock) {
	fc.Advance(2 * time.Second)
}

// fakeBridgeTimerScheduler is a minimal engine.TimerScheduler that
// records AfterFunc calls without firing the timer. Used by the
// bridge tests to assert "the ticker was kicked" without driving the
// adaptive ticker's syncFn (which would re-enter the engine).
type fakeBridgeTimerScheduler struct {
	mu        sync.Mutex
	scheduled []fakeBridgeScheduled
}

type fakeBridgeScheduled struct {
	delay time.Duration
	fn    func()
}

func (f *fakeBridgeTimerScheduler) AfterFunc(d time.Duration, fn func()) engine.Timer {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scheduled = append(f.scheduled, fakeBridgeScheduled{delay: d, fn: fn})
	return &fakeBridgeTimer{}
}

func (f *fakeBridgeTimerScheduler) scheduledCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.scheduled)
}

func (f *fakeBridgeTimerScheduler) lastScheduledDelay() time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.scheduled) == 0 {
		return 0
	}
	return f.scheduled[len(f.scheduled)-1].delay
}

type fakeBridgeTimer struct{}

func (*fakeBridgeTimer) Stop() bool { return true }

// fakeBridgeLogger satisfies engine.Logger with no output.
type fakeBridgeLogger struct{}

func (fakeBridgeLogger) Info(_ string, _ ...any)  {}
func (fakeBridgeLogger) Error(_ string, _ ...any) {}
func (fakeBridgeLogger) Warn(_ string, _ ...any)  {}

// buildAdaptiveTickerWithFakeScheduler returns an AdaptiveTicker bound
// to a fake TimerScheduler. Tests assert against the scheduler's
// recorded AfterFunc calls to verify the bridge kicked the ticker
// without actually re-entering the engine.
func buildAdaptiveTickerWithFakeScheduler() (*engine.AdaptiveTicker, *fakeBridgeTimerScheduler) {
	sched := &fakeBridgeTimerScheduler{}
	noopSync := func(_ context.Context, _ connectors.BindingKey) {}
	t, err := engine.NewAdaptiveTicker(sched, noopSync, fakeBridgeLogger{})
	if err != nil {
		panic(err)
	}
	return t, sched
}

// zeroDebouncerKey is an empty SyncDebouncerKey used to verify no sync fired.
var zeroDebouncerKey engine.SyncDebouncerKey

// --- tasksMutatorBridge ---------------------------------------------------

var _ = Describe("tasksMutatorBridge", func() {
	var (
		bridge *tasksMutatorBridge
		logger = lumber.NewConsoleLogger(lumber.WARN)
	)

	BeforeEach(func() {
		bridge = newTasksMutatorBridge(logger)
	})

	Describe("OnChecklistMutated", func() {
		When("identity is nil", func() {
			It("should not panic and not route to debouncer", func() {
				var capturedKey engine.SyncDebouncerKey
				debouncer, fc := buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				Expect(func() {
					bridge.OnChecklistMutated("page", "list", nil)
				}).NotTo(Panic())

				advanceToFireDebounce(fc)
				Expect(capturedKey).To(Equal(zeroDebouncerKey))
			})
		})

		When("identity has an empty login", func() {
			It("should not route to debouncer", func() {
				var capturedKey engine.SyncDebouncerKey
				debouncer, fc := buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				bridge.OnChecklistMutated("page", "list", &stubIdentity{login: ""})
				advanceToFireDebounce(fc)
				Expect(capturedKey).To(Equal(zeroDebouncerKey))
			})
		})

		When("identity is the tasks-sync synthetic login", func() {
			It("should filter it and not route to debouncer", func() {
				var capturedKey engine.SyncDebouncerKey
				debouncer, fc := buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				bridge.OnChecklistMutated("page", "list", &stubIdentity{login: tasksSyncIdentityLogin})
				advanceToFireDebounce(fc)
				Expect(capturedKey).To(Equal(zeroDebouncerKey))
			})
		})

		When("identity is the connector-sync synthetic login", func() {
			It("should filter it and not route to debouncer", func() {
				var capturedKey engine.SyncDebouncerKey
				debouncer, fc := buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				bridge.OnChecklistMutated("page", "list", &stubIdentity{login: connectorSyncIdentityLogin})
				advanceToFireDebounce(fc)
				Expect(capturedKey).To(Equal(zeroDebouncerKey))
			})
		})

		When("identity is the legacy keep-sync synthetic login", func() {
			It("should filter it and not route to debouncer", func() {
				var capturedKey engine.SyncDebouncerKey
				debouncer, fc := buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				bridge.OnChecklistMutated("page", "list", &stubIdentity{login: legacyKeepSyncIdentityLogin})
				advanceToFireDebounce(fc)
				Expect(capturedKey).To(Equal(zeroDebouncerKey))
			})
		})

		When("debouncer is not yet attached", func() {
			It("should not panic when a valid user login is received", func() {
				Expect(func() {
					bridge.OnChecklistMutated("page", "list", &stubIdentity{login: "user@example.com"})
				}).NotTo(Panic())
			})
		})

		When("a real user login is received and debouncer is attached", func() {
			var (
				capturedKey engine.SyncDebouncerKey
				fc          *enginetesting.FakeClock
			)

			BeforeEach(func() {
				var debouncer *engine.SyncDebouncer
				debouncer, fc = buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				bridge.OnChecklistMutated("groceries", "Shopping", &stubIdentity{login: "alice@example.com"})
				advanceToFireDebounce(fc)
			})

			It("should route to the debouncer with the correct page and listName", func() {
				Expect(capturedKey.Page).To(Equal("groceries"))
				Expect(capturedKey.ListName).To(Equal("Shopping"))
			})

			It("should derive the profileID from the login", func() {
				expectedID, err := wikipage.ProfileIdentifierFor("alice@example.com")
				Expect(err).NotTo(HaveOccurred())
				Expect(capturedKey.ProfileID).To(Equal(string(expectedID)))
			})
		})

		When("the binding key is suppressed", func() {
			It("should not route to debouncer while suppressed", func() {
				var capturedKey engine.SyncDebouncerKey
				debouncer, fc := buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				profileID, _ := wikipage.ProfileIdentifierFor("alice@example.com")
				bridge.Suppress(profileID, "groceries", "Shopping")

				bridge.OnChecklistMutated("groceries", "Shopping", &stubIdentity{login: "alice@example.com"})
				advanceToFireDebounce(fc)
				Expect(capturedKey).To(Equal(zeroDebouncerKey))
			})

			It("should route after Unsuppress clears the refcount", func() {
				var capturedKey engine.SyncDebouncerKey
				debouncer, fc := buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				profileID, _ := wikipage.ProfileIdentifierFor("alice@example.com")
				bridge.Suppress(profileID, "groceries", "Shopping")
				bridge.Suppress(profileID, "groceries", "Shopping") // refcount = 2

				bridge.Unsuppress(profileID, "groceries", "Shopping") // refcount = 1 — still suppressed
				bridge.OnChecklistMutated("groceries", "Shopping", &stubIdentity{login: "alice@example.com"})
				advanceToFireDebounce(fc)
				Expect(capturedKey).To(Equal(zeroDebouncerKey))

				bridge.Unsuppress(profileID, "groceries", "Shopping") // refcount = 0 — now clear

				capturedKey = zeroDebouncerKey
				bridge.OnChecklistMutated("groceries", "Shopping", &stubIdentity{login: "alice@example.com"})
				advanceToFireDebounce(fc)
				Expect(capturedKey.Page).To(Equal("groceries"))
			})
		})

		// User ask 2026-05-08: a wiki edit should immediately kick the
		// adaptive cycle, not wait for the 1.5s debouncer to fire. The
		// bridge calls ticker.RecordTick(activity=true) on every wiki
		// mutation that passes the suppression filter; the debouncer
		// continues to drive the actual outbound push 1.5s later.
		When("an adaptive ticker is attached and a real user edits", func() {
			It("should immediately schedule a follow-up at the base delay (5s)", func() {
				ticker, sched := buildAdaptiveTickerWithFakeScheduler()
				bridge.attachAdaptiveTicker(ticker)

				bridge.OnChecklistMutated("groceries", "Shopping", &stubIdentity{login: "alice@example.com"})

				Expect(sched.scheduledCount()).To(Equal(1),
					"bridge did not kick the adaptive ticker on wiki edit — user-reported regression 2026-05-08")
				Expect(sched.lastScheduledDelay()).To(Equal(engine.AdaptiveTickerBaseDelay),
					"bridge kicked the ticker but at the wrong delay; should be base (5s)")
			})

			It("should NOT kick the ticker if the edit is suppressed", func() {
				ticker, sched := buildAdaptiveTickerWithFakeScheduler()
				bridge.attachAdaptiveTicker(ticker)

				profileID, _ := wikipage.ProfileIdentifierFor("alice@example.com")
				bridge.Suppress(profileID, "groceries", "Shopping")

				bridge.OnChecklistMutated("groceries", "Shopping", &stubIdentity{login: "alice@example.com"})

				Expect(sched.scheduledCount()).To(Equal(0),
					"bridge kicked the ticker for a suppressed mutation; should respect suppression refcount")
			})
		})
	})
})

// --- keepMutatorBridge ----------------------------------------------------

var _ = Describe("keepMutatorBridge", func() {
	var (
		bridge *keepMutatorBridge
		logger = lumber.NewConsoleLogger(lumber.WARN)
	)

	BeforeEach(func() {
		bridge = newKeepMutatorBridge(logger)
	})

	Describe("OnChecklistMutated", func() {
		When("identity is nil", func() {
			It("should not panic", func() {
				Expect(func() {
					bridge.OnChecklistMutated("page", "list", nil)
				}).NotTo(Panic())
			})
		})

		When("identity has an empty login", func() {
			It("should not route to debouncer", func() {
				var capturedKey engine.SyncDebouncerKey
				debouncer, fc := buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				bridge.OnChecklistMutated("page", "list", &stubIdentity{login: ""})
				advanceToFireDebounce(fc)
				Expect(capturedKey).To(Equal(zeroDebouncerKey))
			})
		})

		When("identity is the keep-sync synthetic login", func() {
			It("should filter it and not route to debouncer", func() {
				var capturedKey engine.SyncDebouncerKey
				debouncer, fc := buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				bridge.OnChecklistMutated("page", "list", &stubIdentity{login: keepSyncIdentityLogin})
				advanceToFireDebounce(fc)
				Expect(capturedKey).To(Equal(zeroDebouncerKey))
			})
		})

		When("identity is the connector-sync synthetic login", func() {
			It("should filter it and not route to debouncer", func() {
				var capturedKey engine.SyncDebouncerKey
				debouncer, fc := buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				bridge.OnChecklistMutated("page", "list", &stubIdentity{login: keepConnectorSyncIdentityLogin})
				advanceToFireDebounce(fc)
				Expect(capturedKey).To(Equal(zeroDebouncerKey))
			})
		})

		When("a real user login is received and debouncer is attached", func() {
			var (
				capturedKey engine.SyncDebouncerKey
				fc          *enginetesting.FakeClock
			)

			BeforeEach(func() {
				var debouncer *engine.SyncDebouncer
				debouncer, fc = buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				bridge.OnChecklistMutated("notes", "Todo", &stubIdentity{login: "bob@example.com"})
				advanceToFireDebounce(fc)
			})

			It("should route to the debouncer with the correct page and listName", func() {
				Expect(capturedKey.Page).To(Equal("notes"))
				Expect(capturedKey.ListName).To(Equal("Todo"))
			})

			It("should derive the profileID from the login", func() {
				expectedID, err := wikipage.ProfileIdentifierFor("bob@example.com")
				Expect(err).NotTo(HaveOccurred())
				Expect(capturedKey.ProfileID).To(Equal(string(expectedID)))
			})
		})

		When("the binding key is suppressed", func() {
			It("should not route to debouncer while suppressed", func() {
				var capturedKey engine.SyncDebouncerKey
				debouncer, fc := buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				profileID, _ := wikipage.ProfileIdentifierFor("bob@example.com")
				bridge.Suppress(profileID, "notes", "Todo")

				bridge.OnChecklistMutated("notes", "Todo", &stubIdentity{login: "bob@example.com"})
				advanceToFireDebounce(fc)
				Expect(capturedKey).To(Equal(zeroDebouncerKey))
			})

			It("should route after Unsuppress clears the refcount", func() {
				var capturedKey engine.SyncDebouncerKey
				debouncer, fc := buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				profileID, _ := wikipage.ProfileIdentifierFor("bob@example.com")
				bridge.Suppress(profileID, "notes", "Todo")
				bridge.Suppress(profileID, "notes", "Todo") // refcount = 2

				bridge.Unsuppress(profileID, "notes", "Todo") // refcount = 1 — still suppressed
				bridge.OnChecklistMutated("notes", "Todo", &stubIdentity{login: "bob@example.com"})
				advanceToFireDebounce(fc)
				Expect(capturedKey).To(Equal(zeroDebouncerKey))

				bridge.Unsuppress(profileID, "notes", "Todo") // refcount = 0 — now clear

				capturedKey = zeroDebouncerKey
				bridge.OnChecklistMutated("notes", "Todo", &stubIdentity{login: "bob@example.com"})
				advanceToFireDebounce(fc)
				Expect(capturedKey.Page).To(Equal("notes"))
			})
		})

		When("identity has a login with no alphanumeric characters", func() {
			It("should not route to debouncer and not panic", func() {
				var capturedKey engine.SyncDebouncerKey
				debouncer, fc := buildDebouncerWithCapture(&capturedKey)
				bridge.attachDebouncer(debouncer)

				Expect(func() {
					bridge.OnChecklistMutated("notes", "Todo", &stubIdentity{login: "@@@"})
				}).NotTo(Panic())

				advanceToFireDebounce(fc)
				Expect(capturedKey).To(Equal(zeroDebouncerKey))
			})
		})

		// User ask 2026-05-08: wiki edit kicks the adaptive cycle at
		// edit time, not 1.5s later when the debouncer fires. Same
		// logic as the tasks bridge; mirrored here for parity.
		When("an adaptive ticker is attached and a real user edits", func() {
			It("should immediately schedule a follow-up at the base delay (5s)", func() {
				ticker, sched := buildAdaptiveTickerWithFakeScheduler()
				bridge.attachAdaptiveTicker(ticker)

				bridge.OnChecklistMutated("groceries", "Shopping", &stubIdentity{login: "alice@example.com"})

				Expect(sched.scheduledCount()).To(Equal(1),
					"keep bridge did not kick the adaptive ticker on wiki edit — user-reported regression 2026-05-08")
				Expect(sched.lastScheduledDelay()).To(Equal(engine.AdaptiveTickerBaseDelay))
			})

			It("should NOT kick the ticker if the edit is suppressed", func() {
				ticker, sched := buildAdaptiveTickerWithFakeScheduler()
				bridge.attachAdaptiveTicker(ticker)

				profileID, _ := wikipage.ProfileIdentifierFor("alice@example.com")
				bridge.Suppress(profileID, "groceries", "Shopping")

				bridge.OnChecklistMutated("groceries", "Shopping", &stubIdentity{login: "alice@example.com"})

				Expect(sched.scheduledCount()).To(Equal(0),
					"keep bridge kicked the ticker for a suppressed mutation; should respect suppression refcount")
			})
		})
	})
})

// --- bridgeKey function ---------------------------------------------------

var _ = Describe("bridgeKey", func() {
	It("should produce a deterministic pipe-separated string", func() {
		key := bridgeKey("profile_alice_abc123", "groceries", "Shopping")
		Expect(key).To(Equal("profile_alice_abc123|groceries|Shopping"))
	})
})
