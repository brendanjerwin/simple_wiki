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

// errResumeProgrammed is the sentinel the resume tests use when
// programming a fake collaborator to fail. Distinct name because the
// resume_test.go file shares the engine_test compile unit with
// unbind_test.go (errProgrammed), bind_test.go (errBindProgrammed),
// and force_resync_test.go (errResyncProgrammed).
var errResumeProgrammed = errors.New("resume programmed failure")

// resumeFixedNow is the timestamp the resume tests pin into the
// FakeClock so reviewers can verify the engine reads the clock seam
// (rather than calling time.Now directly) when it stamps PausedAt.
var resumeFixedNow = time.Date(2026, 5, 4, 12, 30, 45, 0, time.UTC)

// resumeWithinHorizonPausedAt is a PausedAt value that, paired with
// resumeFixedNow, yields a pause duration well under the 7-day
// horizon. The Resume tests use this to exercise the in-place
// active-transition path.
var resumeWithinHorizonPausedAt = resumeFixedNow.Add(-1 * time.Hour)

// resumeAtHorizonPausedAt is a PausedAt value that, paired with
// resumeFixedNow, yields exactly the 7-day horizon. The Resume tests
// use this to exercise the boundary case where the force-full-resync
// path triggers (>= horizon).
var resumeAtHorizonPausedAt = resumeFixedNow.Add(-7 * 24 * time.Hour)

// resumePastHorizonPausedAt is a PausedAt value that yields a pause
// duration well past the 7-day horizon (10 days).
var resumePastHorizonPausedAt = resumeFixedNow.Add(-10 * 24 * time.Hour)

// resumeSeq is the LastSyncedSeq value the tests pin onto the seeded
// binding so the "Resume preserves the wiki cursor" assertion has a
// known non-zero baseline.
const resumeSeq int64 = 99

var _ = Describe("Engine.PausedReason", func() {
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
		key   connectors.BindingKey
	)

	BeforeEach(func() {
		fa = &enginetesting.FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleTasks}
		fbs = enginetesting.NewFakeBindingStore()
		lease = connectors.NewLeaseTable()
		lease.SignalReady()

		var err error
		eng, err = engine.NewEngine(
			fa, lease,
			stubChecklistReader{}, stubChecklistMutator{}, stubSuppressor{},
			stubLogger{}, enginetesting.NewFakeClock(resumeFixedNow), fbs,
		)
		Expect(err).NotTo(HaveOccurred())

		key = connectors.BindingKey{
			ProfileID: string(profileID),
			Page:      page,
			ListName:  listName,
		}
	})

	When("no binding exists for (profile, page, listName)", func() {
		var (
			reason string
			ok     bool
		)

		BeforeEach(func() {
			reason, ok = eng.PausedReason(key)
		})

		It("should report ok=false", func() {
			Expect(ok).To(BeFalse())
		})

		It("should return an empty reason", func() {
			Expect(reason).To(BeEmpty())
		})
	})

	When("the binding exists and is in the active state", func() {
		var (
			reason string
			ok     bool
		)

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				State: connectors.BindingStateActive,
			}, connectors.ConnectorKindGoogleTasks)

			reason, ok = eng.PausedReason(key)
		})

		It("should report ok=false", func() {
			Expect(ok).To(BeFalse())
		})

		It("should return an empty reason", func() {
			Expect(reason).To(BeEmpty())
		})
	})

	When("the binding exists and is in the paused state", func() {
		const pausedReason = "auth_failed"

		var (
			reason string
			ok     bool
		)

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				State:        connectors.BindingStatePaused,
				PausedReason: pausedReason,
			}, connectors.ConnectorKindGoogleTasks)

			reason, ok = eng.PausedReason(key)
		})

		It("should report ok=true", func() {
			Expect(ok).To(BeTrue())
		})

		It("should return the configured PausedReason verbatim", func() {
			Expect(reason).To(Equal(pausedReason))
		})
	})

	When("the binding store FindBinding returns an error", func() {
		var (
			reason string
			ok     bool
		)

		BeforeEach(func() {
			fbs.SetFindBindingResponse(connectors.Binding{}, false, errResumeProgrammed)

			reason, ok = eng.PausedReason(key)
		})

		It("should report ok=false (status-display path swallows errors)", func() {
			Expect(ok).To(BeFalse())
		})

		It("should return an empty reason", func() {
			Expect(reason).To(BeEmpty())
		})
	})
})

var _ = Describe("Engine.transitionToPaused (via test seam)", func() {
	const (
		profileID    = wikipage.PageIdentifier("alice_profile")
		page         = "groceries"
		listName     = "this_week"
		reason       = "auth_failed"
		preBoundSeq  = int64(42)
		remoteHandle = "tasklist-abc123"
	)

	var (
		fa           *enginetesting.FakeAdapter
		fbs          *enginetesting.FakeBindingStore
		lease        *connectors.LeaseTable
		eng          *engine.Engine
		preState     connectors.AdapterState
	)

	BeforeEach(func() {
		fa = &enginetesting.FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleTasks}
		fbs = enginetesting.NewFakeBindingStore()
		lease = connectors.NewLeaseTable()
		lease.SignalReady()

		var err error
		eng, err = engine.NewEngine(
			fa, lease,
			stubChecklistReader{}, stubChecklistMutator{}, stubSuppressor{},
			stubLogger{}, enginetesting.NewFakeClock(resumeFixedNow), fbs,
		)
		Expect(err).NotTo(HaveOccurred())

		preState = connectors.AdapterState{"item_id_map": map[string]string{"u1": "t1"}}
	})

	When("an active binding exists", func() {
		var (
			transitionErr error
			savedBinding  connectors.Binding
		)

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID:     profileID,
				Page:          page,
				ListName:      listName,
				RemoteHandle:  remoteHandle,
				LastSyncedSeq: preBoundSeq,
				State:         connectors.BindingStateActive,
				AdapterState:  preState,
			}, connectors.ConnectorKindGoogleTasks)

			transitionErr = eng.TransitionToPausedForTest(
				profileID, connectors.ConnectorKindGoogleTasks,
				page, listName, reason,
			)

			if len(fbs.RecordedSaveBinding) > 0 {
				savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
			}
		})

		It("should not return an error", func() {
			Expect(transitionErr).NotTo(HaveOccurred())
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

		It("should set State to Paused", func() {
			Expect(savedBinding.State).To(Equal(connectors.BindingStatePaused))
		})

		It("should set PausedReason to the configured reason", func() {
			Expect(savedBinding.PausedReason).To(Equal(reason))
		})

		It("should stamp PausedAt with clock.Now()", func() {
			Expect(savedBinding.PausedAt).To(Equal(resumeFixedNow))
		})

		It("should preserve LastSyncedSeq across the pause transition", func() {
			Expect(savedBinding.LastSyncedSeq).To(Equal(preBoundSeq))
		})

		It("should preserve AdapterState across the pause transition", func() {
			Expect(savedBinding.AdapterState).To(Equal(preState))
		})

		It("should preserve the remote handle", func() {
			Expect(savedBinding.RemoteHandle).To(Equal(remoteHandle))
		})
	})

	When("no binding exists for (profile, page, listName)", func() {
		var transitionErr error

		BeforeEach(func() {
			transitionErr = eng.TransitionToPausedForTest(
				profileID, connectors.ConnectorKindGoogleTasks,
				page, listName, reason,
			)
		})

		It("should return ErrBindingNotFound", func() {
			Expect(transitionErr).To(MatchError(engine.ErrBindingNotFound))
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})

	When("SaveBinding returns an error", func() {
		var transitionErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID:    profileID,
				Page:         page,
				ListName:     listName,
				RemoteHandle: remoteHandle,
				State:        connectors.BindingStateActive,
			}, connectors.ConnectorKindGoogleTasks)

			fbs.SetSaveBindingError(errResumeProgrammed)

			transitionErr = eng.TransitionToPausedForTest(
				profileID, connectors.ConnectorKindGoogleTasks,
				page, listName, reason,
			)
		})

		It("should return an error wrapping the save error", func() {
			Expect(transitionErr).To(MatchError(errResumeProgrammed))
		})
	})

	When("the transition succeeds", func() {
		var (
			profileLockIdx int
			saveIdx        int
		)

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID:    profileID,
				Page:         page,
				ListName:     listName,
				RemoteHandle: remoteHandle,
				State:        connectors.BindingStateActive,
			}, connectors.ConnectorKindGoogleTasks)

			Expect(eng.TransitionToPausedForTest(
				profileID, connectors.ConnectorKindGoogleTasks,
				page, listName, reason,
			)).To(Succeed())

			profileLockIdx = indexOfFirst(fbs.CallOrder, "WithProfileLock")
			saveIdx = indexOfFirst(fbs.CallOrder, "SaveBinding")
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

var _ = Describe("Engine.Resume", func() {
	const (
		profileID    = wikipage.PageIdentifier("alice_profile")
		page         = "groceries"
		listName     = "this_week"
		remoteHandle = "tasklist-abc123"
	)

	var (
		fa    *enginetesting.FakeAdapter
		fbs   *enginetesting.FakeBindingStore
		lease *connectors.LeaseTable
		clock *enginetesting.FakeClock
		eng   *engine.Engine
		ctx   context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()

		fa = &enginetesting.FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleTasks}
		fbs = enginetesting.NewFakeBindingStore()
		lease = connectors.NewLeaseTable()
		lease.SignalReady()
		clock = enginetesting.NewFakeClock(resumeFixedNow)

		var err error
		eng, err = engine.NewEngine(
			fa, lease,
			stubChecklistReader{}, stubChecklistMutator{}, stubSuppressor{},
			stubLogger{}, clock, fbs,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	When("no binding exists for (profile, page, listName)", func() {
		var resumeErr error

		BeforeEach(func() {
			resumeErr = eng.Resume(ctx, profileID, page, listName)
		})

		It("should return ErrBindingNotFound", func() {
			Expect(resumeErr).To(MatchError(engine.ErrBindingNotFound))
		})

		It("should not call RebuildAdapterState", func() {
			Expect(fa.RecordedRebuildAdapterState).To(BeEmpty())
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})

	When("the binding is already active", func() {
		var resumeErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID:    profileID,
				Page:         page,
				ListName:     listName,
				RemoteHandle: remoteHandle,
				State:        connectors.BindingStateActive,
			}, connectors.ConnectorKindGoogleTasks)

			resumeErr = eng.Resume(ctx, profileID, page, listName)
		})

		It("should not return an error (idempotent)", func() {
			Expect(resumeErr).NotTo(HaveOccurred())
		})

		It("should not call RebuildAdapterState", func() {
			Expect(fa.RecordedRebuildAdapterState).To(BeEmpty())
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})

	When("the binding is paused for less than the 7-day horizon", func() {
		var (
			preState     connectors.AdapterState
			resumeErr    error
			savedBinding connectors.Binding
		)

		BeforeEach(func() {
			preState = connectors.AdapterState{"item_id_map": map[string]string{"u1": "t1"}}

			fbs.SeedBinding(connectors.Binding{
				ProfileID:     profileID,
				Page:          page,
				ListName:      listName,
				RemoteHandle:  remoteHandle,
				LastSyncedSeq: resumeSeq,
				State:         connectors.BindingStatePaused,
				PausedReason:  "auth_failed",
				PausedAt:      resumeWithinHorizonPausedAt,
				AdapterState:  preState,
			}, connectors.ConnectorKindGoogleTasks)

			resumeErr = eng.Resume(ctx, profileID, page, listName)

			if len(fbs.RecordedSaveBinding) > 0 {
				savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
			}
		})

		It("should not return an error", func() {
			Expect(resumeErr).NotTo(HaveOccurred())
		})

		It("should NOT call RebuildAdapterState (in-place transition)", func() {
			Expect(fa.RecordedRebuildAdapterState).To(BeEmpty())
		})

		It("should call SaveBinding once", func() {
			Expect(fbs.RecordedSaveBinding).To(HaveLen(1))
		})

		It("should set State back to Active", func() {
			Expect(savedBinding.State).To(Equal(connectors.BindingStateActive))
		})

		It("should clear PausedReason", func() {
			Expect(savedBinding.PausedReason).To(BeEmpty())
		})

		It("should clear PausedAt", func() {
			Expect(savedBinding.PausedAt.IsZero()).To(BeTrue())
		})

		It("should preserve LastSyncedSeq", func() {
			Expect(savedBinding.LastSyncedSeq).To(Equal(resumeSeq))
		})

		It("should preserve AdapterState", func() {
			Expect(savedBinding.AdapterState).To(Equal(preState))
		})
	})

	When("the binding is paused for exactly the 7-day horizon", func() {
		var resumeErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID:    profileID,
				Page:         page,
				ListName:     listName,
				RemoteHandle: remoteHandle,
				State:        connectors.BindingStatePaused,
				PausedReason: "auth_failed",
				PausedAt:     resumeAtHorizonPausedAt,
			}, connectors.ConnectorKindGoogleTasks)

			fa.SetRebuildAdapterStateResponse(connectors.AdapterState{"k": "fresh"}, nil)

			resumeErr = eng.Resume(ctx, profileID, page, listName)
		})

		It("should not return an error", func() {
			Expect(resumeErr).NotTo(HaveOccurred())
		})

		It("should call RebuildAdapterState (force-full-resync path)", func() {
			Expect(fa.RecordedRebuildAdapterState).To(HaveLen(1))
		})
	})

	When("the binding is paused beyond the 7-day horizon", func() {
		var (
			resumeErr    error
			savedBinding connectors.Binding
		)

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID:     profileID,
				Page:          page,
				ListName:      listName,
				RemoteHandle:  remoteHandle,
				LastSyncedSeq: resumeSeq,
				State:         connectors.BindingStatePaused,
				PausedReason:  "auth_failed",
				PausedAt:      resumePastHorizonPausedAt,
			}, connectors.ConnectorKindGoogleTasks)

			fa.SetRebuildAdapterStateResponse(connectors.AdapterState{"k": "fresh"}, nil)

			resumeErr = eng.Resume(ctx, profileID, page, listName)

			if len(fbs.RecordedSaveBinding) > 0 {
				savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
			}
		})

		It("should not return an error", func() {
			Expect(resumeErr).NotTo(HaveOccurred())
		})

		It("should call RebuildAdapterState (force-full-resync path)", func() {
			Expect(fa.RecordedRebuildAdapterState).To(HaveLen(1))
		})

		It("should set State back to Active via the resync path", func() {
			Expect(savedBinding.State).To(Equal(connectors.BindingStateActive))
		})

		It("should clear PausedReason via the resync path", func() {
			Expect(savedBinding.PausedReason).To(BeEmpty())
		})

		It("should clear PausedAt via the resync path", func() {
			Expect(savedBinding.PausedAt.IsZero()).To(BeTrue())
		})
	})

	When("the lease table is not ready (WaitReady context cancelled)", func() {
		var (
			notReadyLease *connectors.LeaseTable
			notReadyEng   *engine.Engine
			cancelledCtx  context.Context
			cancel        context.CancelFunc
			resumeErr     error
		)

		BeforeEach(func() {
			notReadyLease = connectors.NewLeaseTable()

			var err error
			notReadyEng, err = engine.NewEngine(
				fa, notReadyLease,
				stubChecklistReader{}, stubChecklistMutator{}, stubSuppressor{},
				stubLogger{}, clock, fbs,
			)
			Expect(err).NotTo(HaveOccurred())

			cancelledCtx, cancel = context.WithCancel(context.Background())
			cancel()

			resumeErr = notReadyEng.Resume(cancelledCtx, profileID, page, listName)
		})

		It("should return an error wrapping the cancellation", func() {
			Expect(resumeErr).To(MatchError(context.Canceled))
		})

		It("should not call RebuildAdapterState", func() {
			Expect(fa.RecordedRebuildAdapterState).To(BeEmpty())
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})

	When("SaveBinding returns an error on the within-horizon path", func() {
		var resumeErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID:    profileID,
				Page:         page,
				ListName:     listName,
				RemoteHandle: remoteHandle,
				State:        connectors.BindingStatePaused,
				PausedReason: "auth_failed",
				PausedAt:     resumeWithinHorizonPausedAt,
			}, connectors.ConnectorKindGoogleTasks)

			fbs.SetSaveBindingError(errResumeProgrammed)

			resumeErr = eng.Resume(ctx, profileID, page, listName)
		})

		It("should return an error wrapping the save error", func() {
			Expect(resumeErr).To(MatchError(errResumeProgrammed))
		})
	})

	When("RebuildAdapterState returns an error on the past-horizon path", func() {
		var resumeErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID:    profileID,
				Page:         page,
				ListName:     listName,
				RemoteHandle: remoteHandle,
				State:        connectors.BindingStatePaused,
				PausedReason: "auth_failed",
				PausedAt:     resumePastHorizonPausedAt,
			}, connectors.ConnectorKindGoogleTasks)

			fa.SetRebuildAdapterStateResponse(connectors.AdapterState{}, errResumeProgrammed)

			resumeErr = eng.Resume(ctx, profileID, page, listName)
		})

		It("should return an error wrapping the rebuild error", func() {
			Expect(resumeErr).To(MatchError(errResumeProgrammed))
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})
})
