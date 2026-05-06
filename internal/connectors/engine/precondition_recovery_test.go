//revive:disable:dot-imports
package engine_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	enginetesting "github.com/brendanjerwin/simple_wiki/internal/connectors/engine/testing"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// errPrecondProgrammed is the sentinel the precondition_recovery
// tests use when programming a fake collaborator to fail. Distinct
// name from the other suite-local sentinels (errProgrammed,
// errReconcileProgrammed, etc.) so a MatchError(...) failure points
// at the right test suite.
var errPrecondProgrammed = errors.New("precondition recovery programmed failure")

// errPrecondPatchTrigger is the original PatchRemote error the
// recovery is invoked with. The recovery wraps and returns this
// error only when its own re-PATCH (branch B) fails again — so
// tests that exercise branch B's re-PATCH can pass this in and
// verify the *re-PATCH* error is wrapped, not the trigger.
var errPrecondPatchTrigger = errors.New("original 412 trigger")

// errPrecondReadFailed is the sentinel a transient ReadRemoteByRef
// failure uses; the recovery should wrap and return it (caller
// abort-and-retry-next-tick).
var errPrecondReadFailed = errors.New("read remote by ref failed")

// errPrecondNotFound is the sentinel a NotFound-classified
// ReadRemoteByRef failure uses. ClassifyError is programmed on the
// FakeAdapter to map this to ErrorClassNotFound, exercising branch
// A via the error path (rather than the Deleted=true field path).
var errPrecondNotFound = errors.New("read returned 404")

var _ = Describe("Engine.runPreconditionRecovery", func() {
	const (
		profileID = wikipage.PageIdentifier("alice_profile")
		page      = "groceries"
		listName  = "this_week"
		ownerKind = connectors.ConnectorKindGoogleTasks
		uid       = "uid-precond-1"
		ref       = connectors.RemoteRef("task-1")
	)

	var (
		fa      *enginetesting.FakeAdapter
		fbs     *enginetesting.FakeBindingStore
		lease   *connectors.LeaseTable
		clock   *enginetesting.FakeClock
		reader  *recordingChecklistReader
		mutator *trackingChecklistMutator
		supr    *recordingSuppressor
		tracker *orderTracker
		eng     *engine.Engine
		ctx     context.Context

		binding  connectors.Binding
		wikiItem connectors.WikiItem
		idMap    map[string]string
	)

	BeforeEach(func() {
		ctx = context.Background()

		fa = &enginetesting.FakeAdapter{ConnectorKind: ownerKind}
		fbs = enginetesting.NewFakeBindingStore()
		lease = connectors.NewLeaseTable()
		lease.SignalReady()
		clock = enginetesting.NewFakeClock(reconcileFixedNow)
		reader = &recordingChecklistReader{}
		tracker = &orderTracker{}
		mutator = &trackingChecklistMutator{
			recordingChecklistMutator: &recordingChecklistMutator{},
			tracker:                   tracker,
		}
		supr = &recordingSuppressor{tracker: tracker}

		var err error
		eng, err = engine.NewEngine(
			fa, lease,
			reader, mutator, supr,
			stubLogger{}, clock, fbs,
		)
		Expect(err).NotTo(HaveOccurred())

		binding = connectors.Binding{
			ProfileID: profileID, Page: page, ListName: listName,
			RemoteHandle: "tasklist-1",
			State:        connectors.BindingStateActive,
		}
		wikiItem = connectors.WikiItem{
			UID:  uid,
			Text: "milk",
		}
		idMap = map[string]string{uid: string(ref)}
	})

	When("ReadRemoteByRef returns a remote item with Deleted=true", func() {
		var recoveryErr error

		BeforeEach(func() {
			fa.SetReadRemoteByRefResponse(connectors.RemoteItem{Ref: ref, Deleted: true}, nil)

			recoveryErr = eng.RunPreconditionRecoveryForTest(
				ctx, binding, ref, uid, wikiItem, idMap, errPrecondPatchTrigger,
			)
		})

		It("should not return an error", func() {
			Expect(recoveryErr).NotTo(HaveOccurred())
		})

		It("should call DeleteItemForSync once with the correct uid", func() {
			Expect(mutator.recordingChecklistMutator.deleteCalls).To(HaveLen(1))
			Expect(mutator.recordingChecklistMutator.deleteCalls[0].UID).To(Equal(uid))
		})

		It("should pass the binding's page to DeleteItemForSync", func() {
			Expect(mutator.recordingChecklistMutator.deleteCalls[0].Page).To(Equal(page))
		})

		It("should pass the binding's listName to DeleteItemForSync", func() {
			Expect(mutator.recordingChecklistMutator.deleteCalls[0].ListName).To(Equal(listName))
		})

		It("should call Suppress before DeleteItemForSync", func() {
			seq := tracker.snapshot()
			suppressIdx := indexOfFirst(seq, "Suppress")
			deleteIdx := indexOfFirst(seq, "Mutator.Delete")
			Expect(suppressIdx).To(BeNumerically(">=", 0))
			Expect(deleteIdx).To(BeNumerically(">=", 0))
			Expect(suppressIdx).To(BeNumerically("<", deleteIdx))
		})

		It("should call Unsuppress after DeleteItemForSync", func() {
			seq := tracker.snapshot()
			deleteIdx := indexOfFirst(seq, "Mutator.Delete")
			unsuppressIdx := indexOfFirst(seq, "Unsuppress")
			Expect(deleteIdx).To(BeNumerically(">=", 0))
			Expect(unsuppressIdx).To(BeNumerically(">", deleteIdx))
		})

		It("should remove the uid from idMap", func() {
			_, present := idMap[uid]
			Expect(present).To(BeFalse())
		})

		It("should not call PatchRemote (no re-PATCH on deleted branch)", func() {
			Expect(fa.RecordedPatchRemote).To(BeEmpty())
		})

		It("should not call AddItemForSync (no apply on deleted branch)", func() {
			Expect(mutator.recordingChecklistMutator.addCalls).To(BeEmpty())
		})

		It("should not call UpdateItemForSync (no apply on deleted branch)", func() {
			Expect(mutator.recordingChecklistMutator.updateCalls).To(BeEmpty())
		})
	})

	When("ReadRemoteByRef returns an error classified as NotFound", func() {
		var recoveryErr error

		BeforeEach(func() {
			fa.SetReadRemoteByRefResponse(connectors.RemoteItem{}, errPrecondNotFound)
			fa.SetClassifyErrorResponse(connectors.ErrorClassNotFound)

			recoveryErr = eng.RunPreconditionRecoveryForTest(
				ctx, binding, ref, uid, wikiItem, idMap, errPrecondPatchTrigger,
			)
		})

		It("should not return an error (treated as remote-deleted)", func() {
			Expect(recoveryErr).NotTo(HaveOccurred())
		})

		It("should call DeleteItemForSync once", func() {
			Expect(mutator.recordingChecklistMutator.deleteCalls).To(HaveLen(1))
		})

		It("should remove the uid from idMap", func() {
			_, present := idMap[uid]
			Expect(present).To(BeFalse())
		})

		It("should not call PatchRemote", func() {
			Expect(fa.RecordedPatchRemote).To(BeEmpty())
		})
	})

	When("ReadRemoteByRef returns a remote item whose fields match the wiki translation (phantom 412)", func() {
		var recoveryErr error

		BeforeEach(func() {
			// Wiki translation: "milk" / "" / "needsAction" / no due.
			fa.SetWikiToRemoteResponse(connectors.RemoteItem{
				Title:  "milk",
				Notes:  "",
				Status: "needsAction",
			}, nil)
			// Read returns the same fields (phantom 412 — server etag bumped
			// without any user-facing change).
			fa.SetReadRemoteByRefResponse(connectors.RemoteItem{
				Ref:    ref,
				Etag:   "fresh-etag",
				Title:  "milk",
				Notes:  "",
				Status: "needsAction",
			}, nil)
			fa.SetPatchRemoteResponse(ref, nil)

			recoveryErr = eng.RunPreconditionRecoveryForTest(
				ctx, binding, ref, uid, wikiItem, idMap, errPrecondPatchTrigger,
			)
		})

		It("should not return an error", func() {
			Expect(recoveryErr).NotTo(HaveOccurred())
		})

		It("should call PatchRemote a second time (the re-PATCH)", func() {
			Expect(fa.RecordedPatchRemote).To(HaveLen(1))
		})

		It("should pass the freshly-read ref to the re-PATCH", func() {
			Expect(fa.RecordedPatchRemote[0].Ref).To(Equal(ref))
		})

		It("should pass the wiki item to the re-PATCH", func() {
			Expect(fa.RecordedPatchRemote[0].Item).To(Equal(wikiItem))
		})

		It("should preserve the uid → ref mapping in idMap", func() {
			Expect(idMap[uid]).To(Equal(string(ref)))
		})

		It("should not call DeleteItemForSync", func() {
			Expect(mutator.recordingChecklistMutator.deleteCalls).To(BeEmpty())
		})

		It("should not call AddItemForSync", func() {
			Expect(mutator.recordingChecklistMutator.addCalls).To(BeEmpty())
		})

		It("should not call UpdateItemForSync", func() {
			Expect(mutator.recordingChecklistMutator.updateCalls).To(BeEmpty())
		})
	})

	When("the re-PATCH itself fails again", func() {
		var recoveryErr error

		BeforeEach(func() {
			fa.SetWikiToRemoteResponse(connectors.RemoteItem{
				Title: "milk", Notes: "", Status: "needsAction",
			}, nil)
			fa.SetReadRemoteByRefResponse(connectors.RemoteItem{
				Ref: ref, Title: "milk", Notes: "", Status: "needsAction",
			}, nil)
			fa.SetPatchRemoteResponse("", errPrecondProgrammed)

			recoveryErr = eng.RunPreconditionRecoveryForTest(
				ctx, binding, ref, uid, wikiItem, idMap, errPrecondPatchTrigger,
			)
		})

		It("should return an error wrapping the re-PATCH failure", func() {
			Expect(recoveryErr).To(MatchError(errPrecondProgrammed))
		})

		It("should call PatchRemote exactly once (no infinite loop)", func() {
			Expect(fa.RecordedPatchRemote).To(HaveLen(1))
		})

		It("should not call DeleteItemForSync", func() {
			Expect(mutator.recordingChecklistMutator.deleteCalls).To(BeEmpty())
		})
	})

	When("ReadRemoteByRef returns a remote item whose fields differ from the wiki translation", func() {
		// PRE-ADR-0015: legacy Tasks v2 had a third branch
		// (remote-authoritative-apply) that overwrote the wiki with the
		// remote's value when wiki-now and remote-now differed. That
		// behavior REVERTED user wiki edits when the etag bumped on the
		// remote without a real content change (production regression
		// 2026-05-06).
		//
		// Post-fix: the patch path is gated on WikiDiverged, so any
		// recovery-call site has unsynced wiki user/cross-connector
		// intent for this uid. Per ADR-0015's 4-cell merge:
		//   WikiDiverged=T && RemoteDiverged=T → conflict, remote wins
		//   WikiDiverged=T && RemoteDiverged=F → push wiki
		// The recovery cannot distinguish these cleanly without tracking
		// last-known-remote, so it always re-patches (preserves user
		// intent). True conflicts surface on the next tick via
		// PullRemote → applyInbound's RemoteDiverged path, which
		// honors ADR-0015's conflict-remote-wins.
		var recoveryErr error

		BeforeEach(func() {
			fa.SetWikiToRemoteResponse(connectors.RemoteItem{
				Title: "milk", Notes: "", Status: "needsAction",
			}, nil)
			// Remote diverges from wiki-now (e.g., older content the
			// user has since updated, OR a real concurrent third-party
			// edit — recovery can't tell, defaults to wiki-wins).
			fa.SetReadRemoteByRefResponse(connectors.RemoteItem{
				Ref:    ref,
				Title:  "milk-from-phone",
				Notes:  "",
				Status: "needsAction",
			}, nil)

			recoveryErr = eng.RunPreconditionRecoveryForTest(
				ctx, binding, ref, uid, wikiItem, idMap, errPrecondPatchTrigger,
			)
		})

		It("should not return an error", func() {
			Expect(recoveryErr).NotTo(HaveOccurred())
		})

		It("should re-patch (wiki-wins) instead of applying remote", func() {
			Expect(fa.RecordedPatchRemote).To(HaveLen(1))
		})

		It("should pass the freshly-read ref to the re-PATCH", func() {
			Expect(fa.RecordedPatchRemote[0].Ref).To(Equal(ref))
		})

		It("should pass the wiki item to the re-PATCH", func() {
			Expect(fa.RecordedPatchRemote[0].Item).To(Equal(wikiItem))
		})

		It("should not call UpdateItemForSync (no remote-wins apply)", func() {
			Expect(mutator.recordingChecklistMutator.updateCalls).To(BeEmpty())
		})

		It("should not call AddItemForSync", func() {
			Expect(mutator.recordingChecklistMutator.addCalls).To(BeEmpty())
		})

		It("should not call DeleteItemForSync", func() {
			Expect(mutator.recordingChecklistMutator.deleteCalls).To(BeEmpty())
		})

		It("should preserve the uid → ref mapping in idMap", func() {
			Expect(idMap[uid]).To(Equal(string(ref)))
		})
	})

	When("ReadRemoteByRef returns a transient (non-NotFound) error", func() {
		var recoveryErr error

		BeforeEach(func() {
			fa.SetReadRemoteByRefResponse(connectors.RemoteItem{}, errPrecondReadFailed)
			fa.SetClassifyErrorResponse(connectors.ErrorClassTransient)

			recoveryErr = eng.RunPreconditionRecoveryForTest(
				ctx, binding, ref, uid, wikiItem, idMap, errPrecondPatchTrigger,
			)
		})

		It("should return an error wrapping the read failure", func() {
			Expect(recoveryErr).To(MatchError(errPrecondReadFailed))
		})

		It("should not call PatchRemote", func() {
			Expect(fa.RecordedPatchRemote).To(BeEmpty())
		})

		It("should not call DeleteItemForSync", func() {
			Expect(mutator.recordingChecklistMutator.deleteCalls).To(BeEmpty())
		})

		It("should not call UpdateItemForSync", func() {
			Expect(mutator.recordingChecklistMutator.updateCalls).To(BeEmpty())
		})

		It("should preserve the uid → ref mapping in idMap (no mutation on read failure)", func() {
			Expect(idMap[uid]).To(Equal(string(ref)))
		})
	})

	When("ReadRemoteByRef returns an auth-failed error", func() {
		var recoveryErr error

		BeforeEach(func() {
			fa.SetReadRemoteByRefResponse(connectors.RemoteItem{}, errPrecondReadFailed)
			fa.SetClassifyErrorResponse(connectors.ErrorClassAuthFailed)

			recoveryErr = eng.RunPreconditionRecoveryForTest(
				ctx, binding, ref, uid, wikiItem, idMap, errPrecondPatchTrigger,
			)
		})

		It("should return an error wrapping the read failure (caller aborts; next tick's PullRemote routes to handleAuthFailure)", func() {
			Expect(recoveryErr).To(MatchError(errPrecondReadFailed))
		})

		It("should not call PatchRemote", func() {
			Expect(fa.RecordedPatchRemote).To(BeEmpty())
		})

		It("should not transition the binding to paused inline (the recovery body stays focused; pause happens via the next tick's PullRemote auth-failed branch)", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})
})
