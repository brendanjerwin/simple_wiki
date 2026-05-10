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

// errBindProgrammed is the sentinel the bind tests use when programming
// a fake collaborator to fail. Distinct from errProgrammed in
// unbind_test.go (same package _test, same compile unit).
var errBindProgrammed = errors.New("bind programmed failure")

// fixedBoundAt is the timestamp the bind tests pin the FakeClock to.
// Distinct from stubClockYear (used by unbind) so reviewers can
// distinguish the two suites at a glance. The bind tests assert the
// engine stamps Binding.BoundAt with this exact instant — proving the
// engine reads the clock seam rather than calling time.Now directly.
//
// Note: stubChecklistReader / stubChecklistMutator / stubSuppressor /
// stubLogger / indexOfFirst all come from unbind_test.go (same
// package _test, same compile unit) and are reused here.
var fixedBoundAt = time.Date(2026, 5, 4, 12, 30, 45, 0, time.UTC)

var _ = Describe("Engine.Bind", func() {
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
		lease.SignalReady() // bind tests skip the boot-rebuild gate
		clock = enginetesting.NewFakeClock(fixedBoundAt)

		var err error
		eng, err = engine.NewEngine(
			fa, lease,
			stubChecklistReader{}, stubChecklistMutator{}, stubSuppressor{},
			stubLogger{}, clock, fbs,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	When("no binding exists for (page, listName)", func() {
		var (
			seededState  connectors.AdapterState
			result       connectors.Binding
			bindErr      error
			checklistKey connectors.ChecklistKey
		)

		BeforeEach(func() {
			seededState = connectors.AdapterState{"item_id_map": map[string]string{"u1": "t1"}}
			fa.SetSeedBindingStateResponse(seededState, nil)
			fa.SetFetchRemoteListTitleResponse("Grocery Shopping", true, nil)
			checklistKey = connectors.ChecklistKey{Page: page, ListName: listName}

			result, bindErr = eng.Bind(ctx, profileID, page, listName, remoteHandle)
		})

		It("should not return an error", func() {
			Expect(bindErr).NotTo(HaveOccurred())
		})

		It("should return a Binding with the profile ID", func() {
			Expect(result.ProfileID).To(Equal(profileID))
		})

		It("should return a Binding with the page", func() {
			Expect(result.Page).To(Equal(page))
		})

		It("should return a Binding with the list name", func() {
			Expect(result.ListName).To(Equal(listName))
		})

		It("should return a Binding with the remote handle", func() {
			Expect(result.RemoteHandle).To(Equal(remoteHandle))
		})

		It("should return a Binding with the seeded adapter state", func() {
			Expect(result.AdapterState).To(Equal(seededState))
		})

		It("should return a Binding in the active state", func() {
			Expect(result.State).To(Equal(connectors.BindingStateActive))
		})

		It("should stamp BoundAt with clock.Now()", func() {
			Expect(result.BoundAt).To(Equal(fixedBoundAt))
		})

		It("should initialize LastSyncedSeq to zero", func() {
			Expect(result.LastSyncedSeq).To(BeZero())
		})

		It("should call ValidateRemoteBinding once", func() {
			Expect(fa.RecordedValidateRemoteBinding).To(HaveLen(1))
		})

		It("should pass the profile ID to ValidateRemoteBinding", func() {
			Expect(fa.RecordedValidateRemoteBinding[0].ProfileID).To(Equal(profileID))
		})

		It("should pass the remote handle to ValidateRemoteBinding", func() {
			Expect(fa.RecordedValidateRemoteBinding[0].RemoteHandle).To(Equal(remoteHandle))
		})

		It("should call SeedBindingState once", func() {
			Expect(fa.RecordedSeedBindingState).To(HaveLen(1))
		})

		// Production fix 2026-05-09: KeepConnect was rendering a UID
		// for the list because Binding.RemoteListTitle was never
		// populated and the UI fell back to remote_list_handle.
		// Bind now calls FetchRemoteListTitle right after seed.
		It("should call FetchRemoteListTitle to populate the user-visible title", func() {
			Expect(fa.RecordedFetchRemoteListTitle).To(HaveLen(1))
		})

		It("should store the fetched remote list title on the Binding", func() {
			Expect(result.RemoteListTitle).To(Equal("Grocery Shopping"))
		})

		It("should pass the profile ID to SeedBindingState", func() {
			Expect(fa.RecordedSeedBindingState[0].ProfileID).To(Equal(profileID))
		})

		It("should pass the remote handle to SeedBindingState", func() {
			Expect(fa.RecordedSeedBindingState[0].RemoteHandle).To(Equal(remoteHandle))
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

		It("should persist the seeded adapter state via SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding[0].Binding.AdapterState).To(Equal(seededState))
		})

		It("should take the lease for (page, listName)", func() {
			owner, ok := lease.LookupOwner(checklistKey)
			Expect(ok).To(BeTrue())
			Expect(owner).To(Equal(connectors.LeaseOwner{
				Kind: connectors.ConnectorKindGoogleTasks, ProfileID: string(profileID),
			}))
		})
	})

	When("the lease table is not ready (WaitReady context cancelled)", func() {
		var (
			notReadyLease *connectors.LeaseTable
			notReadyEng   *engine.Engine
			cancelledCtx  context.Context
			cancel        context.CancelFunc
			bindErr       error
		)

		BeforeEach(func() {
			// Fresh lease table without SignalReady so WaitReady blocks.
			notReadyLease = connectors.NewLeaseTable()
			var err error
			notReadyEng, err = engine.NewEngine(
				fa, notReadyLease,
				stubChecklistReader{}, stubChecklistMutator{}, stubSuppressor{},
				stubLogger{}, clock, fbs,
			)
			Expect(err).NotTo(HaveOccurred())

			cancelledCtx, cancel = context.WithCancel(context.Background())
			cancel() // cancel before calling Bind so WaitReady returns ctx.Err()

			_, bindErr = notReadyEng.Bind(cancelledCtx, profileID, page, listName, remoteHandle)
		})

		It("should return an error wrapping the cancellation", func() {
			Expect(bindErr).To(MatchError(context.Canceled))
		})

		It("should not call ValidateRemoteBinding", func() {
			Expect(fa.RecordedValidateRemoteBinding).To(BeEmpty())
		})

		It("should not call SeedBindingState", func() {
			Expect(fa.RecordedSeedBindingState).To(BeEmpty())
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})

	When("ValidateRemoteBinding returns an error", func() {
		var (
			bindErr      error
			checklistKey connectors.ChecklistKey
		)

		BeforeEach(func() {
			checklistKey = connectors.ChecklistKey{Page: page, ListName: listName}
			fa.SetValidateRemoteBindingResponse(errBindProgrammed)

			_, bindErr = eng.Bind(ctx, profileID, page, listName, remoteHandle)
		})

		It("should return an error wrapping the validate error", func() {
			Expect(bindErr).To(MatchError(errBindProgrammed))
		})

		It("should not call SeedBindingState", func() {
			Expect(fa.RecordedSeedBindingState).To(BeEmpty())
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})

		It("should not take the lease", func() {
			_, ok := lease.LookupOwner(checklistKey)
			Expect(ok).To(BeFalse())
		})
	})

	When("the checklist is already bound by another owner", func() {
		var (
			bindErr        error
			checklistKey   connectors.ChecklistKey
			existingOwner  connectors.LeaseOwner
		)

		BeforeEach(func() {
			checklistKey = connectors.ChecklistKey{Page: page, ListName: listName}
			existingOwner = connectors.LeaseOwner{
				Kind: connectors.ConnectorKindGoogleKeep, ProfileID: "bob_profile",
			}
			Expect(lease.Take(checklistKey, existingOwner)).To(Succeed())

			_, bindErr = eng.Bind(ctx, profileID, page, listName, remoteHandle)
		})

		It("should return ErrAlreadyBoundForChecklist", func() {
			Expect(bindErr).To(MatchError(engine.ErrAlreadyBoundForChecklist))
		})

		It("should not call SeedBindingState", func() {
			Expect(fa.RecordedSeedBindingState).To(BeEmpty())
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})

		It("should leave the existing lease unchanged", func() {
			owner, ok := lease.LookupOwner(checklistKey)
			Expect(ok).To(BeTrue())
			Expect(owner).To(Equal(existingOwner))
		})
	})

	When("SeedBindingState returns an error", func() {
		var (
			bindErr      error
			checklistKey connectors.ChecklistKey
		)

		BeforeEach(func() {
			checklistKey = connectors.ChecklistKey{Page: page, ListName: listName}
			fa.SetSeedBindingStateResponse(connectors.AdapterState{}, errBindProgrammed)

			_, bindErr = eng.Bind(ctx, profileID, page, listName, remoteHandle)
		})

		It("should return an error wrapping the seed error", func() {
			Expect(bindErr).To(MatchError(errBindProgrammed))
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})

		It("should not take the lease", func() {
			_, ok := lease.LookupOwner(checklistKey)
			Expect(ok).To(BeFalse())
		})
	})

	When("SaveBinding returns an error", func() {
		var (
			bindErr      error
			checklistKey connectors.ChecklistKey
		)

		BeforeEach(func() {
			checklistKey = connectors.ChecklistKey{Page: page, ListName: listName}
			fa.SetSeedBindingStateResponse(connectors.AdapterState{"k": "v"}, nil)
			fbs.SetSaveBindingError(errBindProgrammed)

			_, bindErr = eng.Bind(ctx, profileID, page, listName, remoteHandle)
		})

		It("should return an error wrapping the save error", func() {
			Expect(bindErr).To(MatchError(errBindProgrammed))
		})

		It("should not take the lease", func() {
			_, ok := lease.LookupOwner(checklistKey)
			Expect(ok).To(BeFalse())
		})
	})

	When("the bind ceremony succeeds", func() {
		var (
			profileLockIdx int
			saveIdx        int
		)

		BeforeEach(func() {
			fa.SetSeedBindingStateResponse(connectors.AdapterState{"k": "v"}, nil)

			_, err := eng.Bind(ctx, profileID, page, listName, remoteHandle)
			Expect(err).NotTo(HaveOccurred())

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
