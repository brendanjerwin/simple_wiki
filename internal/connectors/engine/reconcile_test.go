//revive:disable:dot-imports
package engine_test

import (
	"context"
	"errors"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	enginetesting "github.com/brendanjerwin/simple_wiki/internal/connectors/engine/testing"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// errReconcileProgrammed is the sentinel the reconcile tests use when
// programming a fake collaborator to fail. Distinct from the other
// suite-local sentinels (errProgrammed, errBindProgrammed, etc.) so a
// MatchError(...) failure points at the right test suite.
var errReconcileProgrammed = errors.New("reconcile programmed failure")

// reconcileFixedNow is the timestamp the reconcile tests pin into the
// FakeClock so the "rate-limit choke" boundary tests have a known
// reference point and so SaveBinding's LastSuccessfulSyncAt can be
// asserted to equal exactly this instant.
var reconcileFixedNow = time.Date(2026, 5, 4, 13, 0, 0, 0, time.UTC)

// reconcilePastChokePausedAt is a LastSuccessfulSyncAt value that, paired
// with reconcileFixedNow, yields a delta well past the 5-second
// rate-limit choke. The reconcile tests use this when they want the
// choke to be inactive (the normal path).
var reconcilePastChokePausedAt = reconcileFixedNow.Add(-1 * time.Hour)

// reconcileWithinChokePausedAt is a LastSuccessfulSyncAt value that,
// paired with reconcileFixedNow, yields a delta WELL within the 5s
// post-success choke. The reconcile tests use this to exercise the
// "skip this tick" branch.
var reconcileWithinChokePausedAt = reconcileFixedNow.Add(-1 * time.Second)

// recordingChecklistReader is a test fake that returns a programmable
// *apiv1.Checklist for every ListItems call and records each invocation.
// Distinct from the unbind-style stubChecklistReader, which fails on
// any call — the reconcile path legitimately reads the checklist.
type recordingChecklistReader struct {
	mu        sync.Mutex
	checklist *apiv1.Checklist
	err       error
	calls     int
}

func (r *recordingChecklistReader) ListItems(_ context.Context, _, _ string) (*apiv1.Checklist, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return r.checklist, r.err
}

// recordingChecklistMutator is a test fake that records every mutation
// the reconcile path drives, returning programmable results.
type recordingChecklistMutator struct {
	mu sync.Mutex

	addUIDToReturn string
	addErr         error
	updateErr      error
	deleteErr      error
	appendErr      error

	addCalls          []addCall
	updateCalls       []updateCall
	deleteCalls       []deleteCall
	appendCalls       []appendCall
	callOrder         []string
}

type addCall struct {
	Page, ListName, OwnerEmail string
	Text                       string
	Checked                    bool
	Tags                       []string
	Description                string
	Position                   string
}

type updateCall struct {
	Page, ListName, OwnerEmail, UID string
	Text                            string
	Checked                         bool
	Tags                            []string
	Description                     string
}

type deleteCall struct {
	Page, ListName, OwnerEmail, UID string
}

type appendCall struct {
	Page, ListName, UID, Op string
}

func (m *recordingChecklistMutator) AddItemForSync(_ context.Context, page, listName, ownerEmail, text string, checked bool, tags []string, description, position string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addCalls = append(m.addCalls, addCall{Page: page, ListName: listName, OwnerEmail: ownerEmail, Text: text, Checked: checked, Tags: tags, Description: description, Position: position})
	m.callOrder = append(m.callOrder, "Add")
	return m.addUIDToReturn, m.addErr
}

func (m *recordingChecklistMutator) UpdateItemForSync(_ context.Context, page, listName, ownerEmail, uid, text string, checked bool, tags []string, description string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalls = append(m.updateCalls, updateCall{Page: page, ListName: listName, OwnerEmail: ownerEmail, UID: uid, Text: text, Checked: checked, Tags: tags, Description: description})
	m.callOrder = append(m.callOrder, "Update")
	return m.updateErr
}

func (m *recordingChecklistMutator) DeleteItemForSync(_ context.Context, page, listName, ownerEmail, uid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCalls = append(m.deleteCalls, deleteCall{Page: page, ListName: listName, OwnerEmail: ownerEmail, UID: uid})
	m.callOrder = append(m.callOrder, "Delete")
	return m.deleteErr
}

func (m *recordingChecklistMutator) AppendSyncEvent(_ context.Context, page, listName, uid, op string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appendCalls = append(m.appendCalls, appendCall{Page: page, ListName: listName, UID: uid, Op: op})
	m.callOrder = append(m.callOrder, "Append")
	return m.appendErr
}

// recordingSuppressor is a test fake SyncSuppressor that records the
// order of Suppress / Unsuppress calls relative to mutator calls. The
// "suppressor wraps inbound apply" assertion needs an interleaved
// timeline; the recordingSuppressor + recordingChecklistMutator share
// CallOrder slices via the suite-level orderTracker.
type recordingSuppressor struct {
	mu        sync.Mutex
	tracker   *orderTracker
	suppress  []suppressCall
	unsupress []suppressCall
}

type suppressCall struct {
	ProfileID wikipage.PageIdentifier
	Page      string
	ListName  string
}

type orderTracker struct {
	mu  sync.Mutex
	seq []string
}

func (t *orderTracker) record(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq = append(t.seq, name)
}

func (t *orderTracker) snapshot() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]string, len(t.seq))
	copy(out, t.seq)
	return out
}

func (s *recordingSuppressor) Suppress(profileID wikipage.PageIdentifier, page, listName string) {
	s.mu.Lock()
	s.suppress = append(s.suppress, suppressCall{ProfileID: profileID, Page: page, ListName: listName})
	s.mu.Unlock()
	if s.tracker != nil {
		s.tracker.record("Suppress")
	}
}

func (s *recordingSuppressor) Unsuppress(profileID wikipage.PageIdentifier, page, listName string) {
	s.mu.Lock()
	s.unsupress = append(s.unsupress, suppressCall{ProfileID: profileID, Page: page, ListName: listName})
	s.mu.Unlock()
	if s.tracker != nil {
		s.tracker.record("Unsuppress")
	}
}

// trackingChecklistMutator is recordingChecklistMutator wrapped to
// record its mutation calls into the same orderTracker the suppressor
// uses, so tests can assert "Suppress fired before any mutator call,
// Unsuppress fired after the last."
type trackingChecklistMutator struct {
	*recordingChecklistMutator
	tracker *orderTracker
}

func (m *trackingChecklistMutator) AddItemForSync(ctx context.Context, page, listName, ownerEmail, text string, checked bool, tags []string, description, position string) (string, error) {
	if m.tracker != nil {
		m.tracker.record("Mutator.Add")
	}
	return m.recordingChecklistMutator.AddItemForSync(ctx, page, listName, ownerEmail, text, checked, tags, description, position)
}

func (m *trackingChecklistMutator) UpdateItemForSync(ctx context.Context, page, listName, ownerEmail, uid, text string, checked bool, tags []string, description string) error {
	if m.tracker != nil {
		m.tracker.record("Mutator.Update")
	}
	return m.recordingChecklistMutator.UpdateItemForSync(ctx, page, listName, ownerEmail, uid, text, checked, tags, description)
}

func (m *trackingChecklistMutator) DeleteItemForSync(ctx context.Context, page, listName, ownerEmail, uid string) error {
	if m.tracker != nil {
		m.tracker.record("Mutator.Delete")
	}
	return m.recordingChecklistMutator.DeleteItemForSync(ctx, page, listName, ownerEmail, uid)
}

func (m *trackingChecklistMutator) AppendSyncEvent(ctx context.Context, page, listName, uid, op string) error {
	if m.tracker != nil {
		m.tracker.record("Mutator.Append")
	}
	return m.recordingChecklistMutator.AppendSyncEvent(ctx, page, listName, uid, op)
}

// indexOfFirstSubstring is a thin alias over the suite-shared
// indexOfFirst (defined in unbind_test.go's compile unit). The local
// name documents that the suppressor-ordering assertions match by
// exact equality, not substring — this is here so the
// "should call Suppress before any mutator call" reads as an
// equality scan rather than a substring scan.
func indexOfFirstSubstring(names []string, target string) int {
	return indexOfFirst(names, target)
}

var _ = Describe("Engine.reconcile", func() {
	const (
		profileID = wikipage.PageIdentifier("alice_profile")
		page      = "groceries"
		listName  = "this_week"
		ownerKind = connectors.ConnectorKindGoogleTasks
	)

	var (
		fa       *enginetesting.FakeAdapter
		fbs      *enginetesting.FakeBindingStore
		lease    *connectors.LeaseTable
		clock    *enginetesting.FakeClock
		reader   *recordingChecklistReader
		mutator  *trackingChecklistMutator
		supr     *recordingSuppressor
		tracker  *orderTracker
		eng      *engine.Engine
		ctx      context.Context
		key      connectors.BindingKey
	)

	BeforeEach(func() {
		ctx = context.Background()

		fa = &enginetesting.FakeAdapter{ConnectorKind: ownerKind}
		fbs = enginetesting.NewFakeBindingStore()
		lease = connectors.NewLeaseTable()
		lease.SignalReady() // reconcile tests skip the boot-rebuild gate
		clock = enginetesting.NewFakeClock(reconcileFixedNow)
		reader = &recordingChecklistReader{checklist: &apiv1.Checklist{}}
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

		key = connectors.BindingKey{
			ProfileID: string(profileID),
			Page:      page,
			ListName:  listName,
		}
	})

	When("no binding exists for the key", func() {
		var syncErr error

		BeforeEach(func() {
			syncErr = eng.Sync(ctx, key)
		})

		It("should not return an error", func() {
			Expect(syncErr).NotTo(HaveOccurred())
		})

		It("should not call PullRemote", func() {
			Expect(fa.RecordedPullRemote).To(BeEmpty())
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})

	When("the binding is paused", func() {
		var syncErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle: "tasklist-1",
				State:        connectors.BindingStatePaused,
				PausedReason: "auth_failed",
			}, ownerKind)

			syncErr = eng.Sync(ctx, key)
		})

		It("should not return an error", func() {
			Expect(syncErr).NotTo(HaveOccurred())
		})

		It("should not call PullRemote", func() {
			Expect(fa.RecordedPullRemote).To(BeEmpty())
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})

	// Production regression 2026-05-08: user observed an
	// insert-recovery loop on a Keep binding whose RemoteHandle was
	// empty (legacy migration gap). Every 30s tick: InsertRemote
	// sent a payload with empty parentId/parentServerId (Go's
	// json:omitempty strips the zero string), Keep silently dropped
	// the orphan LIST_ITEM, response didn't echo our clientItemID,
	// adapter returned ErrProtocolDrift → PreconditionFailed →
	// runInsertRecovery → RebuildAdapterState (full pull every 30s).
	// Documented in §11.1 as "Operator must unbind + rebind" but
	// the engine kept hammering Keep forever instead of pausing.
	// Fix: at reconcile entry, if binding.RemoteHandle is empty,
	// transition to paused with reason "remote_handle_empty" so
	// the operator sees a clear paused-binding badge and can
	// re-bind without consuming RPC quota.
	When("the binding has an empty RemoteHandle (migration gap)", func() {
		var (
			syncErr      error
			savedBinding connectors.Binding
		)

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
			}, ownerKind)

			syncErr = eng.Sync(ctx, key)
			if len(fbs.RecordedSaveBinding) > 0 {
				savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
			}
		})

		It("should not return an error (paused is a steady-state)", func() {
			Expect(syncErr).NotTo(HaveOccurred())
		})

		It("should NOT call PullRemote (the binding is fundamentally broken)", func() {
			Expect(fa.RecordedPullRemote).To(BeEmpty(),
				"engine called PullRemote with empty RemoteHandle — production regression 2026-05-08, insert-recovery loop")
		})

		It("should transition the binding to paused", func() {
			Expect(savedBinding.State).To(Equal(connectors.BindingStatePaused))
		})

		It("should set PausedReason to 'remote_handle_empty'", func() {
			Expect(savedBinding.PausedReason).To(Equal("remote_handle_empty"))
		})
	})

	// Production regression 2026-05-08: user reported "Sync Now
	// doesn't seem to be polling Keep/Tasks." Root cause: the engine
	// had a 5s post-success rate-limit choke that returned nil before
	// any RPC when LastSuccessfulSyncAt was recent. SyncNow goes
	// through Engine.Sync → choke fired → manual click was a no-op.
	// Removed the choke (rules §11.5 already called it "unjustified
	// historical constant"). The AdaptiveTicker base delay (5s) and
	// SyncDebouncer's own 5s post-success choke provide the rate
	// floor; the engine choke was redundant.
	When("LastSuccessfulSyncAt is recent (former rate-limit-choke window)", func() {
		var syncErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcileWithinChokePausedAt,
			}, ownerKind)

			syncErr = eng.Sync(ctx, key)
		})

		It("should not return an error", func() {
			Expect(syncErr).NotTo(HaveOccurred())
		})

		It("should still call PullRemote (no engine-level choke)", func() {
			Expect(fa.RecordedPullRemote).NotTo(BeEmpty(),
				"engine skipped PullRemote on a recent-success binding — production regression 2026-05-08, SyncNow not polling")
		})

		It("should still call SaveBinding (cursor and timestamps update)", func() {
			Expect(fbs.RecordedSaveBinding).NotTo(BeEmpty())
		})
	})

	When("PullRemote returns an auth-failed error", func() {
		var (
			syncErr      error
			savedBinding connectors.Binding
		)

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, errReconcileProgrammed)
			fa.SetClassifyErrorResponse(connectors.ErrorClassAuthFailed)

			syncErr = eng.Sync(ctx, key)
			if len(fbs.RecordedSaveBinding) > 0 {
				savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
			}
		})

		It("should not return an error (paused is a steady-state condition)", func() {
			Expect(syncErr).NotTo(HaveOccurred())
		})

		It("should transition the binding to paused", func() {
			Expect(savedBinding.State).To(Equal(connectors.BindingStatePaused))
		})

		It("should set PausedReason=auth_failed", func() {
			Expect(savedBinding.PausedReason).To(Equal("auth_failed"))
		})
	})

	// Production fix 2026-05-09: legacy bindings created before the
	// bind-time FetchRemoteListTitle fix have RemoteListTitle="";
	// the UI falls back to the opaque RemoteHandle UID. Reconcile
	// self-heals: when the saved binding has empty title and the
	// reconcile completes successfully, fetch the title and store it.
	When("the binding has an empty RemoteListTitle on a successful reconcile", func() {
		var savedBinding connectors.Binding

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				RemoteListTitle:      "",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			fa.SetFetchRemoteListTitleResponse("Grocery Shopping", true, nil)
			reader.checklist = &apiv1.Checklist{}

			Expect(eng.Sync(ctx, key)).To(Succeed())
			if len(fbs.RecordedSaveBinding) > 0 {
				savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
			}
		})

		It("should call FetchRemoteListTitle (self-heal for legacy bindings)", func() {
			Expect(fa.RecordedFetchRemoteListTitle).To(HaveLen(1))
		})

		It("should populate RemoteListTitle on the saved binding", func() {
			Expect(savedBinding.RemoteListTitle).To(Equal("Grocery Shopping"))
		})
	})

	When("the binding already has a RemoteListTitle on a successful reconcile", func() {
		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				RemoteListTitle:      "Existing Title",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			reader.checklist = &apiv1.Checklist{}

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should NOT call FetchRemoteListTitle (self-heal only fires when title is empty)", func() {
			Expect(fa.RecordedFetchRemoteListTitle).To(BeEmpty(),
				"engine refetched the title every tick; self-heal should be lazy and one-shot")
		})
	})

	When("PullRemote returns a rate-limited error", func() {
		var syncErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, errReconcileProgrammed)
			fa.SetClassifyErrorResponse(connectors.ErrorClassRateLimited)

			syncErr = eng.Sync(ctx, key)
		})

		It("should not return an error (back off this tick)", func() {
			Expect(syncErr).NotTo(HaveOccurred())
		})

		It("should not call SaveBinding (no progress made)", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})

	When("PullRemote returns a transient error", func() {
		var syncErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, errReconcileProgrammed)
			fa.SetClassifyErrorResponse(connectors.ErrorClassTransient)

			syncErr = eng.Sync(ctx, key)
		})

		It("should return an error wrapping the pull error", func() {
			Expect(syncErr).To(MatchError(errReconcileProgrammed))
		})

		It("should not call SaveBinding", func() {
			Expect(fbs.RecordedSaveBinding).To(BeEmpty())
		})
	})

	When("PullRemote returns Truncated=true", func() {
		var (
			syncErr      error
			savedBinding connectors.Binding
			rebuiltState connectors.AdapterState
		)

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
			}, ownerKind)

			rebuiltState = connectors.AdapterState{"item_id_map": map[string]string{}}
			fa.SetPullRemoteResponse(connectors.RemotePullResult{Truncated: true}, nil)
			fa.SetRebuildAdapterStateResponse(rebuiltState, nil)

			syncErr = eng.Sync(ctx, key)
			if len(fbs.RecordedSaveBinding) > 0 {
				savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
			}
		})

		It("should not return an error", func() {
			Expect(syncErr).NotTo(HaveOccurred())
		})

		It("should call RebuildAdapterState (delegated to runForceFullResync)", func() {
			Expect(fa.RecordedRebuildAdapterState).To(HaveLen(1))
		})

		It("should persist the rebuilt adapter state", func() {
			Expect(savedBinding.AdapterState).To(Equal(rebuiltState))
		})
	})

	When("there are no remote items and no wiki items", func() {
		var (
			syncErr      error
			savedBinding connectors.Binding
		)

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			reader.checklist = &apiv1.Checklist{}

			syncErr = eng.Sync(ctx, key)
			Expect(fbs.RecordedSaveBinding).NotTo(BeEmpty())
			savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
		})

		It("should not return an error", func() {
			Expect(syncErr).NotTo(HaveOccurred())
		})

		It("should call SaveBinding once", func() {
			Expect(fbs.RecordedSaveBinding).To(HaveLen(1))
		})

		It("should stamp LastSuccessfulSyncAt with clock.Now()", func() {
			Expect(savedBinding.LastSuccessfulSyncAt).To(Equal(reconcileFixedNow))
		})

		It("should call AdvanceCursor", func() {
			Expect(fa.RecordedAdvanceCursor).To(HaveLen(1))
		})
	})

	When("inbound has a new remote item with no matching wiki uid", func() {
		var newUID string

		BeforeEach(func() {
			newUID = "uid-new-1"

			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				AdapterState:         connectors.AdapterState{},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{
				Items: []connectors.RemoteItem{{Ref: "task-1", Title: "milk"}},
			}, nil)
			// Explicit RemoteToWiki: empty UID signals a new item.
			fa.SetRemoteToWikiResponse(connectors.WikiItem{UID: "", Text: "milk"}, nil)
			mutator.recordingChecklistMutator.addUIDToReturn = newUID

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should call AddItemForSync once", func() {
			Expect(mutator.recordingChecklistMutator.addCalls).To(HaveLen(1))
		})

		It("should pass the resolved item text to AddItemForSync", func() {
			Expect(mutator.recordingChecklistMutator.addCalls[0].Text).To(Equal("milk"))
		})
	})

	When("inbound has an existing remote item, wiki uid known, NOT diverged", func() {
		const knownUID = "uid-existing-1"

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				LastSyncedSeq:        10,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "task-1"},
				},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{
				Items: []connectors.RemoteItem{{Ref: "task-1", Title: "milk-updated"}},
			}, nil)
			fa.SetRemoteToWikiResponse(connectors.WikiItem{UID: knownUID, Text: "milk-updated"}, nil)
			// Reader returns a checklist with no events (no divergence) and
			// the wiki item already present.
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk"}},
			}

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should call UpdateItemForSync once", func() {
			Expect(mutator.recordingChecklistMutator.updateCalls).To(HaveLen(1))
		})

		It("should pass the known uid to UpdateItemForSync", func() {
			Expect(mutator.recordingChecklistMutator.updateCalls[0].UID).To(Equal(knownUID))
		})
	})

	When("inbound has an existing remote item but the wiki has diverged", func() {
		const knownUID = "uid-existing-1"

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				LastSyncedSeq:        10,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "task-1"},
				},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{
				Items: []connectors.RemoteItem{{Ref: "task-1", Title: "milk-updated"}},
			}, nil)
			fa.SetRemoteToWikiResponse(connectors.WikiItem{UID: knownUID, Text: "milk-updated"}, nil)
			// Op-log shows a user event after LastSyncedSeq for this uid:
			// the wiki diverged, applying remote would clobber.
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk-user-edited"}},
				Events: []*apiv1.ChecklistEvent{
					{Seq: 11, Src: "user:alice@example.com", Op: "set_text", Uid: knownUID},
				},
				MaxSeq: 11,
			}

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should NOT call UpdateItemForSync (wiki diverged → skip apply)", func() {
			Expect(mutator.recordingChecklistMutator.updateCalls).To(BeEmpty())
		})
	})

	When("inbound has a deleted remote item that the wiki knows about", func() {
		const knownUID = "uid-existing-1"

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "task-1"},
				},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{
				Items: []connectors.RemoteItem{{Ref: "task-1", Deleted: true}},
			}, nil)
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk"}},
			}

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should call DeleteItemForSync once", func() {
			Expect(mutator.recordingChecklistMutator.deleteCalls).To(HaveLen(1))
		})

		It("should pass the known uid to DeleteItemForSync", func() {
			Expect(mutator.recordingChecklistMutator.deleteCalls[0].UID).To(Equal(knownUID))
		})
	})

	When("inbound has a deleted remote item the wiki does not track", func() {
		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				AdapterState:         connectors.AdapterState{"item_id_map": map[string]string{}},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{
				Items: []connectors.RemoteItem{{Ref: "task-unknown", Deleted: true}},
			}, nil)
			reader.checklist = &apiv1.Checklist{}

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should not call DeleteItemForSync (no-op when uid is unknown)", func() {
			Expect(mutator.recordingChecklistMutator.deleteCalls).To(BeEmpty())
		})
	})

	// Sticky user-wins applies to the Deleted cell too (round-3
	// panel review, Lamport finding §10.13). When the remote
	// deletes a uid AND the wiki op-log carries an uncovered user
	// event for the same uid, the engine MUST NOT mirror the
	// remote delete to the wiki — the user click expresses ground-
	// truth intent at the wiki replica.
	When("inbound item has Deleted=true AND uid has UncoveredUserEvent", func() {
		const knownUID = "uid-deleted-user-wins-1"

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				LastSyncedSeq:        10,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "task-1"},
				},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{
				Items: []connectors.RemoteItem{{Ref: "task-1", Deleted: true}},
			}, nil)
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk-user-wants"}},
				Events: []*apiv1.ChecklistEvent{
					{Seq: 11, Src: "user:bren@example.com", Op: "set_text", Uid: knownUID},
				},
				MaxSeq: 11,
			}

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should NOT call DeleteItemForSync (sticky user-wins protects the wiki state from remote delete)", func() {
			Expect(mutator.recordingChecklistMutator.deleteCalls).To(BeEmpty(),
				"engine mirrored a remote delete despite an uncovered user event — Lamport round 3, §10.13")
		})
	})

	When("outbound has a new wiki item not in AdapterState mapping", func() {
		const newUID = "uid-new-wiki-1"
		var savedBinding connectors.Binding

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				AdapterState:         connectors.AdapterState{"item_id_map": map[string]string{}},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			fa.SetInsertRemoteResponse("task-fresh", nil)
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: newUID, Text: "eggs"}},
			}

			Expect(eng.Sync(ctx, key)).To(Succeed())
			Expect(fbs.RecordedSaveBinding).NotTo(BeEmpty())
			savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
		})

		It("should call InsertRemote once", func() {
			Expect(fa.RecordedInsertRemote).To(HaveLen(1))
		})

		It("should record the inserted ref under the wiki uid in AdapterState", func() {
			idMap, ok := savedBinding.AdapterState["item_id_map"].(map[string]string)
			Expect(ok).To(BeTrue())
			Expect(idMap[newUID]).To(Equal("task-fresh"))
		})

		It("should append outbound_inserted to the op-log", func() {
			Expect(mutator.recordingChecklistMutator.appendCalls).To(ContainElement(
				appendCall{Page: page, ListName: listName, UID: newUID, Op: "outbound_inserted"},
			))
		})
	})

	When("outbound has an updated wiki item already in AdapterState mapping and wiki has diverged", func() {
		const knownUID = "uid-update-1"

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				LastSyncedSeq:        10,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "task-1"},
				},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			fa.SetPatchRemoteResponse("task-1", nil)
			// User event after LastSyncedSeq=10 → WikiDiverged = true for knownUID.
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk-updated"}},
				Events: []*apiv1.ChecklistEvent{
					{Seq: 11, Src: "user:alice@example.com", Op: "set_text", Uid: knownUID},
				},
				MaxSeq: 11,
			}

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should call PatchRemote once", func() {
			Expect(fa.RecordedPatchRemote).To(HaveLen(1))
		})

		It("should append outbound_patched to the op-log", func() {
			Expect(mutator.recordingChecklistMutator.appendCalls).To(ContainElement(
				appendCall{Page: page, ListName: listName, UID: knownUID, Op: "outbound_patched"},
			))
		})
	})

	// Fix #2: push gate — only patch when WikiDiverged = true.
	When("outbound has a wiki item in AdapterState mapping but wiki has NOT diverged", func() {
		const knownUID = "uid-nodiv-1"

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				LastSyncedSeq:        10,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "task-1"},
				},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			// No user events → WikiDiverged = false → patch must be skipped.
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk"}},
			}

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should NOT call PatchRemote (wiki not diverged — no outbound push needed)", func() {
			Expect(fa.RecordedPatchRemote).To(BeEmpty())
		})
	})

	When("outbound has a deleted wiki item still in AdapterState mapping", func() {
		const knownUID = "uid-delete-1"
		var savedBinding connectors.Binding

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "task-1"},
				},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			fa.SetDeleteRemoteResponse(nil)
			// Wiki has no items; the AdapterState lists task-1 → must delete.
			reader.checklist = &apiv1.Checklist{}

			Expect(eng.Sync(ctx, key)).To(Succeed())
			Expect(fbs.RecordedSaveBinding).NotTo(BeEmpty())
			savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
		})

		It("should call DeleteRemote once", func() {
			Expect(fa.RecordedDeleteRemote).To(HaveLen(1))
		})

		It("should remove the entry from AdapterState's item_id_map", func() {
			idMap, ok := savedBinding.AdapterState["item_id_map"].(map[string]string)
			Expect(ok).To(BeTrue())
			_, present := idMap[knownUID]
			Expect(present).To(BeFalse())
		})

		It("should append outbound_deleted to the op-log", func() {
			Expect(mutator.recordingChecklistMutator.appendCalls).To(ContainElement(
				appendCall{Page: page, ListName: listName, UID: knownUID, Op: "outbound_deleted"},
			))
		})
	})

	When("PatchRemote returns precondition_failed", func() {
		const knownUID = "uid-412-1"
		var syncErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				LastSyncedSeq:        10,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "task-1"},
				},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			fa.SetPatchRemoteResponse("", errReconcileProgrammed)
			fa.SetClassifyErrorResponse(connectors.ErrorClassPreconditionFailed)
			// Recovery's ReadRemoteByRef returns a default
			// RemoteItem{Ref: "task-1"} (not deleted; fields differ from
			// wiki) → branch C (authoritative apply). Default mutator
			// returns nil; recovery completes and sync continues to the
			// happy path. The detailed branch behavior is exercised in
			// precondition_recovery_test.go; this test verifies only the
			// integration (recovery is invoked and the sync succeeds).
			// User event after LastSyncedSeq=10 → WikiDiverged = true so
			// the push gate proceeds to PatchRemote (Fix #2).
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk-updated"}},
				Events: []*apiv1.ChecklistEvent{
					{Seq: 11, Src: "user:alice@example.com", Op: "set_text", Uid: knownUID},
				},
				MaxSeq: 11,
			}

			syncErr = eng.Sync(ctx, key)
		})

		It("should invoke runPreconditionRecovery (ReadRemoteByRef called)", func() {
			Expect(fa.RecordedReadRemoteByRef).To(HaveLen(1))
		})

		It("should not abort the sync (recovery handled the precondition)", func() {
			Expect(syncErr).NotTo(HaveOccurred())
		})
	})

	// Production regression 2026-05-08: user reported "shopping list is
	// doubled (tripled actually)." Root cause: Keep's InsertRemote
	// can return ErrProtocolDrift (response doesn't echo our
	// clientItemID) even when the insert actually succeeded server-
	// side. Without dedup, the next reconcile sees the wiki uid as
	// still-not-in-idMap and re-Inserts — creating a duplicate. Fix:
	// before InsertRemote, scan the latest pull for a content-match
	// (Title) and adopt the existing ref into idMap if found. The
	// adapter's WikiToRemote produces the canonical Title that pulled
	// items also carry.
	When("a remote pull item content-matches an unbound wiki item (Keep-style empty wikiItem.UID)", func() {
		const wikiUID = "uid-dedup-1"
		var (
			syncErr      error
			savedBinding connectors.Binding
		)

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				AdapterState:         connectors.AdapterState{"item_id_map": map[string]string{}},
			}, ownerKind)

			// Realistic Keep scenario: RemoteToWiki returns wikiItem
			// with UID="" because Keep has no wiki-uid marker. Without
			// dedup, applyInbound takes the AddItemForSync path and
			// duplicates the wiki item. With the dedup-by-text fix,
			// the engine matches against the existing wiki uid and
			// adopts the remote ref via UpdateItemForSync.
			fa.SetPullRemoteResponse(connectors.RemotePullResult{
				Items: []connectors.RemoteItem{{Ref: "ref-existing", Title: "milk"}},
			}, nil)
			fa.SetRemoteToWikiResponse(connectors.WikiItem{UID: "", Text: "milk"}, nil)

			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: wikiUID, Text: "milk"}},
			}

			syncErr = eng.Sync(ctx, key)
			if len(fbs.RecordedSaveBinding) > 0 {
				savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
			}
		})

		It("should not return an error", func() {
			Expect(syncErr).NotTo(HaveOccurred())
		})

		It("should NOT call AddItemForSync (no duplicate wiki item)", func() {
			Expect(mutator.recordingChecklistMutator.addCalls).To(BeEmpty(),
				"engine duplicated a wiki item that text-matched an unbound pulled item — production regression 2026-05-08, shopping list duplicates")
		})

		It("should record the existing ref into idMap under the original wiki uid", func() {
			idMap, _ := savedBinding.AdapterState["item_id_map"].(map[string]string)
			Expect(idMap).To(HaveKeyWithValue(wikiUID, "ref-existing"))
		})

		It("should NOT call InsertRemote (the wiki uid now routes through Patch)", func() {
			Expect(fa.RecordedInsertRemote).To(BeEmpty())
		})
	})

	// Production regression 2026-05-08: user reported "wiki side delete
	// didn't propagate to keep side." Logs showed Keep DeleteRemote
	// returning ErrProtocolDrift (stage3 HTTP 500) which the adapter
	// classifies as ErrorClassPreconditionFailed. The DELETE branch in
	// pushOutbound had no PreconditionFailed handler — fell into
	// `default:` and aborted the entire tick on every retry, blocking
	// every other item in the binding from syncing. Fix: route through
	// runDeletePreconditionRecovery (mirrors PATCH's recovery shape).
	When("DeleteRemote returns precondition_failed AND remote is already gone", func() {
		const knownUID = "uid-delete-precond-1"
		var syncErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				LastSyncedSeq:        10,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "ref-1"},
				},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			fa.SetDeleteRemoteResponse(errReconcileProgrammed)
			fa.SetClassifyErrorResponse(connectors.ErrorClassPreconditionFailed)
			// Recovery's ReadRemoteByRef returns a Deleted=true tombstone
			// (the remote already removed the item; our DELETE was
			// redundant). Should be idempotent success path: emit
			// outbound_deleted, clear idMap, do not abort.
			fa.SetReadRemoteByRefResponse(connectors.RemoteItem{Ref: "ref-1", Deleted: true}, nil)
			// Wiki has no items — uid-1 is a pending DELETE because
			// idMap still references it.
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{},
				Events: []*apiv1.ChecklistEvent{
					{Seq: 11, Src: "user:bren@example.com", Op: "delete", Uid: knownUID},
				},
				MaxSeq: 11,
			}

			syncErr = eng.Sync(ctx, key)
		})

		It("should NOT abort the tick (recovery handled the precondition)", func() {
			Expect(syncErr).NotTo(HaveOccurred(),
				"engine aborted tick on DELETE precondition_failed instead of routing through recovery — production regression 2026-05-08")
		})

		It("should invoke ReadRemoteByRef (recovery was called)", func() {
			Expect(fa.RecordedReadRemoteByRef).To(HaveLen(1))
		})

		It("should append outbound_deleted (idempotent success)", func() {
			Expect(mutator.recordingChecklistMutator.appendCalls).To(ContainElement(
				appendCall{Page: page, ListName: listName, UID: knownUID, Op: "outbound_deleted"},
			))
		})
	})

	When("InsertRemote returns a retryable error", func() {
		const newUID = "uid-retryable-1"
		var syncErr error

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				AdapterState:         connectors.AdapterState{"item_id_map": map[string]string{}},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			fa.SetInsertRemoteResponse("", errReconcileProgrammed)
			fa.SetClassifyErrorResponse(connectors.ErrorClassRetryable)
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: newUID, Text: "eggs"}},
			}

			syncErr = eng.Sync(ctx, key)
		})

		It("should not abort the whole sync (retryable continues)", func() {
			Expect(syncErr).NotTo(HaveOccurred())
		})
	})

	When("InsertRemote returns an auth-failed error", func() {
		const newUID = "uid-auth-1"
		var (
			syncErr      error
			savedBinding connectors.Binding
		)

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				AdapterState:         connectors.AdapterState{"item_id_map": map[string]string{}},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			fa.SetInsertRemoteResponse("", errReconcileProgrammed)
			fa.SetClassifyErrorResponse(connectors.ErrorClassAuthFailed)
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: newUID, Text: "eggs"}},
			}

			syncErr = eng.Sync(ctx, key)
			if len(fbs.RecordedSaveBinding) > 0 {
				savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
			}
		})

		It("should not return an error (paused is steady-state)", func() {
			Expect(syncErr).NotTo(HaveOccurred())
		})

		It("should transition the binding to paused", func() {
			Expect(savedBinding.State).To(Equal(connectors.BindingStatePaused))
		})

		It("should set PausedReason=auth_failed", func() {
			Expect(savedBinding.PausedReason).To(Equal("auth_failed"))
		})
	})

	When("InsertRemote returns ErrorClassPreconditionFailed", func() {
		// Insert-recovery: when an Insert returns a precondition failure
		// (e.g., Keep stage3-500 from a stale cursor or a duplicate), the
		// engine must call RebuildAdapterState to refresh state and
		// persist the rebuilt binding. Skips the item this tick; the
		// next tick will Patch via the rebuilt mapping rather than
		// Insert. Without this path Insert dead-letters every item
		// after 10 retries (production bug 2026-05-05).
		const newUID = "uid-precondition-1"
		var (
			syncErr      error
			savedBinding connectors.Binding
			rebuiltState = connectors.AdapterState{
				"item_id_map":  map[string]string{},
				"item_mapping": map[string]any{"server-fresh": map[string]any{"server_id": "server-fresh"}},
			}
		)

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				AdapterState:         connectors.AdapterState{"item_id_map": map[string]string{}},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			fa.SetInsertRemoteResponse("", errReconcileProgrammed)
			fa.SetClassifyErrorResponse(connectors.ErrorClassPreconditionFailed)
			fa.SetRebuildAdapterStateResponse(rebuiltState, nil)
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: newUID, Text: "eggs"}},
			}

			syncErr = eng.Sync(ctx, key)
			if len(fbs.RecordedSaveBinding) > 0 {
				savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
			}
		})

		It("should not return an error (recovery is steady-state)", func() {
			Expect(syncErr).NotTo(HaveOccurred())
		})

		It("should call RebuildAdapterState once", func() {
			Expect(fa.RecordedRebuildAdapterState).To(HaveLen(1))
		})

		It("should persist the rebuilt AdapterState on the binding", func() {
			Expect(savedBinding.AdapterState).To(HaveKey("item_mapping"))
			Expect(savedBinding.AdapterState["item_mapping"]).To(Equal(rebuiltState["item_mapping"]))
		})
	})

	When("the suppressor wraps the inbound apply pass", func() {
		const knownUID = "uid-supr-1"

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "task-1"},
				},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{
				Items: []connectors.RemoteItem{{Ref: "task-1", Title: "milk-updated"}},
			}, nil)
			fa.SetRemoteToWikiResponse(connectors.WikiItem{UID: knownUID, Text: "milk-updated"}, nil)
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk"}},
			}

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should call Suppress before any mutator call", func() {
			seq := tracker.snapshot()
			suppressIdx := indexOfFirstSubstring(seq, "Suppress")
			updateIdx := indexOfFirstSubstring(seq, "Mutator.Update")
			Expect(suppressIdx).To(BeNumerically(">=", 0))
			Expect(updateIdx).To(BeNumerically(">=", 0))
			Expect(suppressIdx).To(BeNumerically("<", updateIdx))
		})

		It("should call Unsuppress after the last mutator call", func() {
			seq := tracker.snapshot()
			lastUpdateIdx := -1
			lastUnsuppressIdx := -1
			for i, name := range seq {
				switch name {
				case "Mutator.Update", "Mutator.Add", "Mutator.Delete":
					lastUpdateIdx = i
				case "Unsuppress":
					lastUnsuppressIdx = i
				default:
					// Other tracker entries (Suppress, Mutator.Append) are
					// not used by this assertion; ignore.
				}
			}
			Expect(lastUpdateIdx).To(BeNumerically(">=", 0))
			Expect(lastUnsuppressIdx).To(BeNumerically(">", lastUpdateIdx))
		})
	})

	When("the reconcile completes successfully", func() {
		var savedBinding connectors.Binding

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				AdapterState:         connectors.AdapterState{"item_id_map": map[string]string{}},
			}, ownerKind)

			advancedBinding := connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				AdapterState:         connectors.AdapterState{"item_id_map": map[string]string{}, "last_updated_min": reconcileFixedNow},
			}
			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			fa.SetAdvanceCursorResponse(advancedBinding)

			Expect(eng.Sync(ctx, key)).To(Succeed())
			Expect(fbs.RecordedSaveBinding).NotTo(BeEmpty())
			savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
		})

		It("should pass the pull result to AdvanceCursor", func() {
			Expect(fa.RecordedAdvanceCursor).To(HaveLen(1))
		})

		It("should persist the cursor-advanced AdapterState", func() {
			Expect(savedBinding.AdapterState["last_updated_min"]).To(Equal(reconcileFixedNow))
		})

		It("should stamp LastSuccessfulSyncAt with clock.Now()", func() {
			Expect(savedBinding.LastSuccessfulSyncAt).To(Equal(reconcileFixedNow))
		})
	})

	When("the op-log has self-events past LastSyncedSeq", func() {
		const knownUID = "uid-selfseq-1"
		var savedBinding connectors.Binding

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				LastSyncedSeq:        10,
				AdapterState:         connectors.AdapterState{"item_id_map": map[string]string{}},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			// Two events past the cursor: one self-event, one user
			// event. Cursor should advance ONLY past the self-event so
			// the user event remains visible to next tick's classify.
			reader.checklist = &apiv1.Checklist{
				Events: []*apiv1.ChecklistEvent{
					{Seq: 11, Src: "user:alice@example.com", Op: "set_text", Uid: knownUID},
					{Seq: 12, Src: "connector:google_tasks:apply", Op: "set_text", Uid: knownUID},
				},
				MaxSeq: 12,
			}

			Expect(eng.Sync(ctx, key)).To(Succeed())
			Expect(fbs.RecordedSaveBinding).NotTo(BeEmpty())
			savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
		})

		It("should advance LastSyncedSeq past the latest self-event", func() {
			Expect(savedBinding.LastSyncedSeq).To(BeNumerically(">=", int64(12)))
		})
	})

	// Fix #3: atomicity — partial idMap updates from successful inserts
	// must be persisted even when a subsequent outbound operation fails.
	When("outbound insert succeeds then a subsequent outbound insert fails fatally", func() {
		const uid1 = "uid-partial-1"
		const uid2 = "uid-partial-2"
		var savedBinding connectors.Binding

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				AdapterState:         connectors.AdapterState{"item_id_map": map[string]string{}},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{}, nil)
			// First insert succeeds; second fails fatally.
			fa.SetInsertRemoteResponse("task-ok", nil)
			fa.SetInsertRemoteResponse("", errReconcileProgrammed)
			fa.SetClassifyErrorResponse(connectors.ErrorClassFatal)
			// Two wiki items, neither in idMap → both will be inserted.
			// The order of iteration over a Go map is non-deterministic;
			// the test asserts that exactly ONE insert succeeded and was
			// persisted, regardless of which uid was processed first.
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{
					{Uid: uid1, Text: "item-one"},
					{Uid: uid2, Text: "item-two"},
				},
			}

			_ = eng.Sync(ctx, key) // error is expected; ignore it here
			if len(fbs.RecordedSaveBinding) > 0 {
				savedBinding = fbs.RecordedSaveBinding[len(fbs.RecordedSaveBinding)-1].Binding
			}
		})

		It("should call SaveBinding to persist partial progress", func() {
			Expect(fbs.RecordedSaveBinding).NotTo(BeEmpty())
		})

		It("should persist the successfully-inserted item's ref in idMap", func() {
			idMap, ok := savedBinding.AdapterState["item_id_map"].(map[string]string)
			Expect(ok).To(BeTrue())
			// Exactly one item should have been inserted (task-ok).
			insertedCount := 0
			for _, ref := range idMap {
				if ref == "task-ok" {
					insertedCount++
				}
			}
			Expect(insertedCount).To(Equal(1))
		})
	})

	// 4-cell merge refinement (production fix 2026-05-06): when the
	// wiki's latest event is a USER write and the remote's etag has
	// diverged, the engine must NOT apply remote — that reverts user
	// intent. The user just expressed their will; wiki wins.
	//
	// Tasks's RemoteDiverged is etag-based, and Tasks bumps etags for
	// non-content reasons (position, metadata, server-side denorm), so
	// "etag differs" doesn't reliably mean "content differs." Relying on
	// it for ADR-0015's conflict-remote-wins clobbered user check-offs
	// in production.
	//
	// Cross-connector divergence (LatestEventSource starts with
	// "connector:") still applies the remote per ADR-0015 — the wiki's
	// state was previously written by another connector, so a fresh
	// remote change from THIS connector is authoritative.
	When("inbound item has wiki diverged from a USER write AND RemoteDiverged=true", func() {
		const knownUID = "uid-user-conflict-1"

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				LastSyncedSeq:        10,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "task-1"},
				},
			}, ownerKind)

			// RemoteDiverged=true via etag bump (e.g., Tasks bumped etag
			// for a non-content metadata change).
			fa.SetPullRemoteResponse(connectors.RemotePullResult{
				Items: []connectors.RemoteItem{{
					Ref:            "task-1",
					Title:          "milk-remote-stale-content",
					RemoteDiverged: true,
				}},
			}, nil)
			fa.SetRemoteToWikiResponse(connectors.WikiItem{UID: knownUID, Text: "milk-remote-stale-content"}, nil)
			// Wiki diverged: USER edited this item (latest src = user:).
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk-user-edit"}},
				Events: []*apiv1.ChecklistEvent{
					{Seq: 11, Src: "user:alice@example.com", Op: "set_text", Uid: knownUID},
				},
				MaxSeq: 11,
			}

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should NOT call UpdateItemForSync (user-wins; preserve user edit)", func() {
			Expect(mutator.recordingChecklistMutator.updateCalls).To(BeEmpty())
		})
	})

	// Cross-connector conflict still follows ADR-0015's
	// conflict-remote-wins. The wiki's latest event was an apply from
	// ANOTHER connector (not this connector), so the remote is the
	// authoritative recent write from this side.
	When("inbound item has wiki diverged from a CROSS-CONNECTOR apply AND RemoteDiverged=true", func() {
		const knownUID = "uid-cross-conflict-1"

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				LastSyncedSeq:        10,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "task-1"},
				},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{
				Items: []connectors.RemoteItem{{
					Ref:            "task-1",
					Title:          "milk-remote-wins",
					RemoteDiverged: true,
				}},
			}, nil)
			fa.SetRemoteToWikiResponse(connectors.WikiItem{UID: knownUID, Text: "milk-remote-wins"}, nil)
			// Wiki diverged from a different connector's apply (not this
			// connector — Tasks here, the diverging event is from Keep).
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk-keep-applied"}},
				Events: []*apiv1.ChecklistEvent{
					{Seq: 11, Src: "connector:google_keep:apply", Op: "toggle", Uid: knownUID},
				},
				MaxSeq: 11,
			}

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should call UpdateItemForSync (cross-connector → conflict-remote-wins)", func() {
			Expect(mutator.recordingChecklistMutator.updateCalls).To(HaveLen(1))
		})

		It("should apply the remote text", func() {
			Expect(mutator.recordingChecklistMutator.updateCalls[0].Text).To(Equal("milk-remote-wins"))
		})
	})

	// Sticky user-wins (panel review v2): when the op-log carries
	// [connector:OTHER:apply, user:bren, connector:OTHER:apply] for
	// the same uid in the uncovered window, the engine MUST treat
	// this as user-wins even though the LATEST event is a cross-
	// connector apply (the "sandwich" case Lamport flagged in the
	// 2026-05-07 panel review).
	When("inbound item has wiki diverged with a USER event sandwiched between cross-connector applies AND RemoteDiverged=true", func() {
		const knownUID = "uid-sandwich-1"

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				LastSyncedSeq:        10,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "task-1"},
				},
			}, ownerKind)

			fa.SetPullRemoteResponse(connectors.RemotePullResult{
				Items: []connectors.RemoteItem{{
					Ref:            "task-1",
					Title:          "milk-from-other-connector",
					RemoteDiverged: true,
				}},
			}, nil)
			fa.SetRemoteToWikiResponse(connectors.WikiItem{UID: knownUID, Text: "milk-from-other-connector"}, nil)
			// Op-log sandwich: cross-connector apply, then user click,
			// then another cross-connector apply. Latest event is
			// cross-connector, but the user click is in the uncovered
			// window — sticky user-wins must fire.
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk-user-wants"}},
				Events: []*apiv1.ChecklistEvent{
					{Seq: 11, Src: "connector:google_keep:apply", Op: "toggle", Uid: knownUID},
					{Seq: 12, Src: "user:bren@example.com", Op: "set_text", Uid: knownUID},
					{Seq: 13, Src: "connector:google_keep:apply", Op: "toggle", Uid: knownUID},
				},
				MaxSeq: 13,
			}

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should NOT apply remote (sticky user-wins; the user click in the middle wins despite latest being cross-connector)", func() {
			Expect(mutator.recordingChecklistMutator.updateCalls).To(BeEmpty(),
				"engine reverted the user's wiki edit despite a user event in the sandwich — Lamport panel finding 2026-05-07")
		})
	})

	// Fix #1 complement: when wiki diverged AND RemoteDiverged=false
	// (remote is stale/re-delivered), engine must skip inbound apply.
	When("inbound item has wiki diverged AND RemoteDiverged=false (push-wiki path)", func() {
		const knownUID = "uid-wikiwins-1"

		BeforeEach(func() {
			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID, Page: page, ListName: listName,
				RemoteHandle:         "tasklist-1",
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: reconcilePastChokePausedAt,
				LastSyncedSeq:        10,
				AdapterState: connectors.AdapterState{
					"item_id_map": map[string]string{knownUID: "task-1"},
				},
			}, ownerKind)

			// RemoteDiverged=false: remote item is a stale re-delivery.
			fa.SetPullRemoteResponse(connectors.RemotePullResult{
				Items: []connectors.RemoteItem{{
					Ref:            "task-1",
					Title:          "milk-stale",
					RemoteDiverged: false,
				}},
			}, nil)
			fa.SetRemoteToWikiResponse(connectors.WikiItem{UID: knownUID, Text: "milk-stale"}, nil)
			// Wiki diverged: user edited since last sync.
			reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk-user-edit"}},
				Events: []*apiv1.ChecklistEvent{
					{Seq: 11, Src: "user:alice@example.com", Op: "set_text", Uid: knownUID},
				},
				MaxSeq: 11,
			}

			Expect(eng.Sync(ctx, key)).To(Succeed())
		})

		It("should NOT call UpdateItemForSync (wiki diverged, remote unchanged — preserve wiki edit)", func() {
			Expect(mutator.recordingChecklistMutator.updateCalls).To(BeEmpty())
		})
	})
})
