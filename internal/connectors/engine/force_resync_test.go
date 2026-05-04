//revive:disable:dot-imports
package engine_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	enginetesting "github.com/brendanjerwin/simple_wiki/internal/connectors/engine/testing"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// errResyncProgrammed is the sentinel the force-resync tests use when
// programming a fake collaborator to fail. Distinct from errProgrammed
// (unbind_test.go) and errBindProgrammed (bind_test.go) — same compile
// unit, distinct names.
var errResyncProgrammed = errors.New("force-resync programmed failure")

// fixedPausedAt is the timestamp the force-resync tests pin into a
// pre-seeded paused binding so reviewers can verify the engine clears
// PausedAt back to the zero value as part of the resync.
var fixedPausedAt = time.Date(2026, 4, 27, 9, 15, 0, 0, time.UTC)

// fixedLastSuccessfulSyncAt is the pre-resync LastSuccessfulSyncAt the
// tests stamp on the seeded binding so the "engine clears the
// rate-limit cookie" assertion has a non-zero baseline to compare
// against.
var fixedLastSuccessfulSyncAt = time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)

// preResyncSeq is the LastSyncedSeq value the tests pin onto the
// seeded binding so the "ForceFullResync preserves the wiki cursor"
// assertion has a known non-zero baseline.
const preResyncSeq int64 = 42

var _ = Describe("Engine.ForceFullResync", func() {
	const (
		profileID = wikipage.PageIdentifier("alice_profile")
		page      = "groceries"
		listName  = "this_week"
	)

	var (
		fa    *enginetesting.FakeAdapter
		fbs   *enginetesting.FakeBindingStore
		lease *connectors.LeaseTable
		eng   *engine.Engine
		ctx   context.Context
		key   connectors.SubscriptionKey
	)

	BeforeEach(func() {
		ctx = context.Background()

		fa = &enginetesting.FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleTasks}
		fbs = enginetesting.NewFakeBindingStore()
		lease = connectors.NewLeaseTable()
		lease.SignalReady() // force-resync tests skip the boot-rebuild gate

		var err error
		eng, err = engine.NewEngine(
			fa, lease,
			stubChecklistReader{}, stubChecklistMutator{}, stubSuppressor{},
			stubLogger{}, stubClock{}, fbs,
		)
		Expect(err).NotTo(HaveOccurred())

		key = connectors.SubscriptionKey{
			ProfileID: string(profileID),
			Page:      page,
			ListName:  listName,
		}
	})

	When("a paused binding exists for (profile, page, listName)", func() {
		var (
			oldState connectors.AdapterState
			newState connectors.AdapterState
			resyncErr error
			savedBinding connectors.Binding
		)

		BeforeEach(func() {
			oldState = connectors.AdapterState{"item_id_map": map[string]string{"u1": "old"}}
			newState = connectors.AdapterState{"item_id_map": map[string]string{"u1": "new"}}

			fbs.SeedBinding(connectors.Binding{
				ProfileID:            profileID,
				Page:                 page,
				ListName:             listName,
				RemoteHandle:         "tasklist-abc123",
				LastSyncedSeq:        preResyncSeq,
				State:                connectors.BindingStatePaused,
				PausedReason:         "auth_failed",
				PausedAt:             fixedPausedAt,
				LastSuccessfulSyncAt: fixedLastSuccessfulSyncAt,
				AdapterState:         oldState,
			}, connectors.ConnectorKindGoogleTasks)

			fa.SetRebuildAdapterStateResponse(newState, nil)

			resyncErr = eng.ForceFullResync(ctx, key)

			if len(fbs.RecordedSaveBinding) > 0 {
				savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
			}
		})

		It("should not return an error", func() {
			Expect(resyncErr).NotTo(HaveOccurred())
		})

		It("should call RebuildAdapterState once", func() {
			Expect(fa.RecordedRebuildAdapterState).To(HaveLen(1))
		})

		It("should pass the existing binding to RebuildAdapterState", func() {
			Expect(fa.RecordedRebuildAdapterState[0].ProfileID).To(Equal(profileID))
			Expect(fa.RecordedRebuildAdapterState[0].Page).To(Equal(page))
			Expect(fa.RecordedRebuildAdapterState[0].ListName).To(Equal(listName))
		})

		It("should call SaveBinding once", func() {
			Expect(fbs.RecordedSaveBinding).To(HaveLen(1))
		})

		It("should pass the profile ID to SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding[0].ProfileID).To(Equal(profileID))
		})

		It("should pass the connector kind to SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding[0].Kind).To(Equal(connectors.ConnectorKindGoogleTasks))
		})

		It("should replace AdapterState with the rebuilt state", func() {
			Expect(savedBinding.AdapterState).To(Equal(newState))
		})

		It("should reset State to Active", func() {
			Expect(savedBinding.State).To(Equal(connectors.BindingStateActive))
		})

		It("should clear PausedReason", func() {
			Expect(savedBinding.PausedReason).To(BeEmpty())
		})

		It("should clear PausedAt", func() {
			Expect(savedBinding.PausedAt.IsZero()).To(BeTrue())
		})

		It("should clear LastSuccessfulSyncAt to unrate-limit the next tick", func() {
			Expect(savedBinding.LastSuccessfulSyncAt.IsZero()).To(BeTrue())
		})

		It("should preserve LastSyncedSeq across the resync", func() {
			Expect(savedBinding.LastSyncedSeq).To(Equal(preResyncSeq))
		})
	})

	When("no binding exists for (profile, page, listName)", func() {
		var resyncErr error

		BeforeEach(func() {
			// No SeedBinding; FindBinding returns (Binding{}, false, nil).
			resyncErr = eng.ForceFullResync(ctx, key)
		})

		It("should return ErrBindingNotFound", func() {
			Expect(resyncErr).To(MatchError(engine.ErrBindingNotFound))
		})

		It("should not call RebuildAdapterState", func() {
			Expect(fa.RecordedRebuildAdapterState).To(BeEmpty())
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})

	When("the lease table is not ready (WaitReady context cancelled)", func() {
		var (
			notReadyLease *connectors.LeaseTable
			notReadyEng   *engine.Engine
			cancelledCtx  context.Context
			cancel        context.CancelFunc
			resyncErr     error
		)

		BeforeEach(func() {
			notReadyLease = connectors.NewLeaseTable()
			var err error
			notReadyEng, err = engine.NewEngine(
				fa, notReadyLease,
				stubChecklistReader{}, stubChecklistMutator{}, stubSuppressor{},
				stubLogger{}, stubClock{}, fbs,
			)
			Expect(err).NotTo(HaveOccurred())

			cancelledCtx, cancel = context.WithCancel(context.Background())
			cancel() // cancel before calling so WaitReady returns ctx.Err()

			resyncErr = notReadyEng.ForceFullResync(cancelledCtx, key)
		})

		It("should return an error wrapping the cancellation", func() {
			Expect(resyncErr).To(MatchError(context.Canceled))
		})

		It("should not call RebuildAdapterState", func() {
			Expect(fa.RecordedRebuildAdapterState).To(BeEmpty())
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})

	When("RebuildAdapterState returns an error", func() {
		var resyncErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID:    profileID,
				Page:         page,
				ListName:     listName,
				RemoteHandle: "tasklist-abc123",
				State:        connectors.BindingStatePaused,
			}, connectors.ConnectorKindGoogleTasks)

			fa.SetRebuildAdapterStateResponse(connectors.AdapterState{}, errResyncProgrammed)

			resyncErr = eng.ForceFullResync(ctx, key)
		})

		It("should return an error wrapping the rebuild error", func() {
			Expect(resyncErr).To(MatchError(errResyncProgrammed))
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})

	When("SaveBinding returns an error", func() {
		var resyncErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID:    profileID,
				Page:         page,
				ListName:     listName,
				RemoteHandle: "tasklist-abc123",
				State:        connectors.BindingStatePaused,
			}, connectors.ConnectorKindGoogleTasks)

			fa.SetRebuildAdapterStateResponse(connectors.AdapterState{"k": "v"}, nil)
			fbs.SetSaveBindingError(errResyncProgrammed)

			resyncErr = eng.ForceFullResync(ctx, key)
		})

		It("should return an error wrapping the save error", func() {
			Expect(resyncErr).To(MatchError(errResyncProgrammed))
		})
	})

	When("the resync ceremony succeeds", func() {
		var (
			rebuildIdx     int
			profileLockIdx int
			saveIdx        int
		)

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID:    profileID,
				Page:         page,
				ListName:     listName,
				RemoteHandle: "tasklist-abc123",
				State:        connectors.BindingStatePaused,
			}, connectors.ConnectorKindGoogleTasks)

			fa.SetRebuildAdapterStateResponse(connectors.AdapterState{"k": "v"}, nil)

			Expect(eng.ForceFullResync(ctx, key)).To(Succeed())

			// Rebuild ordering needs the FakeAdapter timeline; the
			// FakeBindingStore CallOrder only tracks store interactions.
			// The simplest way to assert "rebuild happened, then profile
			// lock, then save" is by length: 1 rebuild call, then the
			// per-store ordering.
			rebuildIdx = len(fa.RecordedRebuildAdapterState) - 1
			profileLockIdx = indexOfFirst(fbs.CallOrder, "WithProfileLock")
			saveIdx = indexOfFirst(fbs.CallOrder, "SaveBinding")
		})

		It("should call RebuildAdapterState", func() {
			Expect(rebuildIdx).To(BeNumerically(">=", 0))
		})

		It("should call WithProfileLock", func() {
			Expect(profileLockIdx).To(BeNumerically(">=", 0))
		})

		It("should call SaveBinding", func() {
			Expect(saveIdx).To(BeNumerically(">=", 0))
		})

		It("should call WithProfileLock before SaveBinding", func() {
			Expect(profileLockIdx).To(BeNumerically("<", saveIdx))
		})

		It("should call WithProfileLock for the bound profile", func() {
			Expect(fbs.RecordedWithProfileLock).To(ContainElement(profileID))
		})
	})
})
