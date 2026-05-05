//revive:disable:dot-imports
package bootstrap

import (
	"context"
	"time"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	enginetesting "github.com/brendanjerwin/simple_wiki/internal/connectors/engine/testing"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// stubIdentity is a minimal tailscale.IdentityValue for bridge tests.
type stubIdentity struct {
	login string
}

func (s *stubIdentity) IsAnonymous() bool   { return s.login == "" }
func (s *stubIdentity) LoginName() string   { return s.login }
func (s *stubIdentity) DisplayName() string { return "" }
func (s *stubIdentity) NodeName() string    { return "" }
func (s *stubIdentity) ForLog() string      { return s.login }
func (s *stubIdentity) String() string      { return s.login }
func (s *stubIdentity) IsAgent() bool       { return false }
func (s *stubIdentity) Name() string        { return s.login }

var _ tailscale.IdentityValue = (*stubIdentity)(nil)

// buildDebouncerWithCapture creates a SyncDebouncer + FakeClock pair
// whose syncFn stores the most recently fired key into *out.
func buildDebouncerWithCapture(out *engine.SyncDebouncerKey) (*engine.SyncDebouncer, *enginetesting.FakeClock) {
	fc := enginetesting.NewFakeClock(time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC))
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
