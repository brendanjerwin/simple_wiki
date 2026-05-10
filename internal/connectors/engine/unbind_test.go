//revive:disable:dot-imports
package engine_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	enginetesting "github.com/brendanjerwin/simple_wiki/internal/connectors/engine/testing"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// stubChecklistReader is the no-op ChecklistReader shared by the
// bind / unbind / force_resync / resume tests. Returns an empty
// checklist (no items) so callers that consult it (e.g., Bind's
// bind-time alignment in #182) get a defined empty list instead of
// nil. Unbind / force_resync / resume don't read the checklist;
// their behavior is asserted via saved binding state.
type stubChecklistReader struct{}

func (stubChecklistReader) ListItems(_ context.Context, _, _ string) (*apiv1.Checklist, error) {
	return &apiv1.Checklist{}, nil
}

// stubChecklistMutator is the no-op ChecklistMutator the unbind tests
// use. Unbind must not mutate the wiki checklist; any call fails the test.
type stubChecklistMutator struct{}

func (stubChecklistMutator) AddItemForSync(_ context.Context, _, _, _, _ string, _ bool, _ []string, _, _ string, _ *time.Time) (string, error) {
	Fail("Unbind must not call ChecklistMutator.AddItemForSync")
	return "", nil
}

func (stubChecklistMutator) UpdateItemForSync(_ context.Context, _, _, _, _, _ string, _ bool, _ []string, _ string, _ *time.Time) error {
	Fail("Unbind must not call ChecklistMutator.UpdateItemForSync")
	return nil
}

func (stubChecklistMutator) DeleteItemForSync(_ context.Context, _, _, _, _ string) error {
	Fail("Unbind must not call ChecklistMutator.DeleteItemForSync")
	return nil
}

func (stubChecklistMutator) AppendSyncEvent(_ context.Context, _, _, _, _ string) error {
	Fail("Unbind must not call ChecklistMutator.AppendSyncEvent")
	return nil
}

// stubSuppressor is the no-op SyncSuppressor; Unbind has no inbound
// apply pass and must not touch the suppressor.
type stubSuppressor struct{}

func (stubSuppressor) Suppress(_ wikipage.PageIdentifier, _, _ string) {
	Fail("Unbind must not call SyncSuppressor.Suppress")
}

func (stubSuppressor) Unsuppress(_ wikipage.PageIdentifier, _, _ string) {
	Fail("Unbind must not call SyncSuppressor.Unsuppress")
}

// stubLogger is a logger that swallows messages — Unbind may emit Info
// lines describing the unbind, but tests don't assert on log content.
type stubLogger struct{}

func (stubLogger) Info(_ string, _ ...any)  {}
func (stubLogger) Warn(_ string, _ ...any)  {}
func (stubLogger) Error(_ string, _ ...any) {}

// stubClockYear is the fixed year stubClock reports. Hoisted to avoid
// the magic-number lint and to document that the value is arbitrary —
// Unbind does not consult the clock in the current contract.
const stubClockYear = 2026

// stubClock returns a fixed instant. Unbind does not consult the clock
// in the current contract, but NewEngine requires non-nil.
type stubClock struct{}

func (stubClock) Now() time.Time { return time.Date(stubClockYear, 1, 1, 0, 0, 0, 0, time.UTC) }

// errProgrammed is the sentinel the tests use when programming
// FakeBindingStore to fail.
var errProgrammed = errors.New("programmed failure")

// indexOfFirst returns the index of the first element of names equal to
// target, or -1 if absent. Hoisted so call-ordering assertions can be
// computed in BeforeEach (per the project's "no actions in It" rule).
func indexOfFirst(names []string, target string) int {
	for i, n := range names {
		if n == target {
			return i
		}
	}
	return -1
}

var _ = Describe("Engine.Unbind", func() {
	const (
		profileID = wikipage.PageIdentifier("alice_profile")
		page      = "groceries"
		listName  = "this_week"
	)

	var (
		fa     *enginetesting.FakeAdapter
		fbs    *enginetesting.FakeBindingStore
		lease  *connectors.LeaseTable
		eng    *engine.Engine
		ctx    context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()

		fa = &enginetesting.FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleTasks}
		fbs = enginetesting.NewFakeBindingStore()
		lease = connectors.NewLeaseTable()
		lease.SignalReady() // unbind tests skip the boot-rebuild gate

		var err error
		eng, err = engine.NewEngine(
			fa, lease,
			stubChecklistReader{}, stubChecklistMutator{}, stubSuppressor{},
			stubLogger{}, stubClock{}, fbs,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	When("a binding exists for (profile, page, listName)", func() {
		var (
			seededOwner    connectors.LeaseOwner
			checklistKey   connectors.ChecklistKey
			unbindErr      error
			profileLockIdx int
			deleteIdx      int
		)

		BeforeEach(func() {
			seededOwner = connectors.LeaseOwner{
				Kind: connectors.ConnectorKindGoogleTasks, ProfileID: string(profileID),
			}
			checklistKey = connectors.ChecklistKey{Page: page, ListName: listName}

			// Seed an existing binding + lease so Unbind has something
			// to remove.
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle: "handle-1",
			}, connectors.ConnectorKindGoogleTasks)
			Expect(lease.Take(checklistKey, seededOwner)).To(Succeed())

			unbindErr = eng.Unbind(ctx, profileID, page, listName)

			// Capture call ordering for the lock-before-delete assertions.
			profileLockIdx = indexOfFirst(fbs.CallOrder, "WithProfileLock")
			deleteIdx = indexOfFirst(fbs.CallOrder, "DeleteBinding")
		})

		It("should not return an error", func() {
			Expect(unbindErr).NotTo(HaveOccurred())
		})

		It("should call DeleteBinding once", func() {
			Expect(fbs.RecordedDeleteBinding).To(HaveLen(1))
		})

		It("should pass the profileID to DeleteBinding", func() {
			Expect(fbs.RecordedDeleteBinding[0].ProfileID).To(Equal(profileID))
		})

		It("should pass the connector kind to DeleteBinding", func() {
			Expect(fbs.RecordedDeleteBinding[0].Kind).To(Equal(connectors.ConnectorKindGoogleTasks))
		})

		It("should pass the page to DeleteBinding", func() {
			Expect(fbs.RecordedDeleteBinding[0].Page).To(Equal(page))
		})

		It("should pass the listName to DeleteBinding", func() {
			Expect(fbs.RecordedDeleteBinding[0].ListName).To(Equal(listName))
		})

		It("should release the lease for (page, listName)", func() {
			_, ok := lease.LookupOwner(checklistKey)
			Expect(ok).To(BeFalse())
		})

		It("should call WithProfileLock", func() {
			Expect(profileLockIdx).To(BeNumerically(">=", 0))
		})

		It("should call DeleteBinding", func() {
			Expect(deleteIdx).To(BeNumerically(">=", 0))
		})

		It("should call WithProfileLock before DeleteBinding", func() {
			Expect(profileLockIdx).To(BeNumerically("<", deleteIdx))
		})

		It("should call WithProfileLock for the correct profile", func() {
			Expect(fbs.RecordedWithProfileLock).To(ContainElement(profileID))
		})
	})

	When("no binding exists for (profile, page, listName)", func() {
		var (
			checklistKey connectors.ChecklistKey
			unbindErr    error
		)

		BeforeEach(func() {
			checklistKey = connectors.ChecklistKey{Page: page, ListName: listName}
			// No SeedBinding; no lease taken. The contract says
			// DeleteBinding is a no-op when the binding is missing,
			// so Unbind must still succeed.
			unbindErr = eng.Unbind(ctx, profileID, page, listName)
		})

		It("should not return an error (idempotent)", func() {
			Expect(unbindErr).NotTo(HaveOccurred())
		})

		It("should still call DeleteBinding (the store treats missing as no-op)", func() {
			Expect(fbs.RecordedDeleteBinding).To(HaveLen(1))
		})

		It("should leave the lease table unchanged", func() {
			_, ok := lease.LookupOwner(checklistKey)
			Expect(ok).To(BeFalse())
		})
	})

	When("DeleteBinding returns an error", func() {
		var (
			seededOwner  connectors.LeaseOwner
			checklistKey connectors.ChecklistKey
			unbindErr    error
		)

		BeforeEach(func() {
			seededOwner = connectors.LeaseOwner{
				Kind: connectors.ConnectorKindGoogleTasks, ProfileID: string(profileID),
			}
			checklistKey = connectors.ChecklistKey{Page: page, ListName: listName}

			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle: "handle-1",
			}, connectors.ConnectorKindGoogleTasks)
			Expect(lease.Take(checklistKey, seededOwner)).To(Succeed())

			fbs.SetDeleteBindingError(errProgrammed)
			unbindErr = eng.Unbind(ctx, profileID, page, listName)
		})

		It("should return an error wrapping the store error", func() {
			Expect(unbindErr).To(MatchError(errProgrammed))
		})

		It("should not release the lease", func() {
			owner, ok := lease.LookupOwner(checklistKey)
			Expect(ok).To(BeTrue())
			Expect(owner).To(Equal(seededOwner))
		})
	})

	When("WithProfileLock returns an error", func() {
		var (
			seededOwner  connectors.LeaseOwner
			checklistKey connectors.ChecklistKey
			unbindErr    error
		)

		BeforeEach(func() {
			seededOwner = connectors.LeaseOwner{
				Kind: connectors.ConnectorKindGoogleTasks, ProfileID: string(profileID),
			}
			checklistKey = connectors.ChecklistKey{Page: page, ListName: listName}

			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle: "handle-1",
			}, connectors.ConnectorKindGoogleTasks)
			Expect(lease.Take(checklistKey, seededOwner)).To(Succeed())

			fbs.SetWithProfileLockError(errProgrammed)
			unbindErr = eng.Unbind(ctx, profileID, page, listName)
		})

		It("should return an error wrapping the lock error", func() {
			Expect(unbindErr).To(MatchError(errProgrammed))
		})

		It("should not call DeleteBinding (lock failed before critical section)", func() {
			Expect(fbs.RecordedDeleteBinding).To(BeEmpty())
		})

		It("should not release the lease", func() {
			owner, ok := lease.LookupOwner(checklistKey)
			Expect(ok).To(BeTrue())
			Expect(owner).To(Equal(seededOwner))
		})
	})
})

// Compile-time assertion: FakeBindingStore satisfies engine.BindingStore.
// If ports.go grows a method, this assertion fails and the test build
// breaks until the fake is extended.
var _ engine.BindingStore = (*enginetesting.FakeBindingStore)(nil)
