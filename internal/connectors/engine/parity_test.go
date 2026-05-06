//revive:disable:dot-imports
package engine_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	enginetesting "github.com/brendanjerwin/simple_wiki/internal/connectors/engine/testing"
	googlekeep "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep"
	keepgw "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
	googletasks "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks"
	tasksgw "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Parity tests run canonical engine-level scenarios through every real
// adapter (KeepAdapter and TasksAdapter, and eventually iCloudAdapter)
// to assert identical engine decisions. They are the load-bearing
// safety net for behavior-preserving extraction: any divergence between
// what the engine does with KeepAdapter vs. TasksAdapter for the same
// scenario is, by construction, a strict-behavior-wins violation —
// either the audit missed it or the adapter is implementing engine
// semantics that should live in the engine.
//
// Per the plan (`to-build-issue-998-warm-glacier.md` Phase 3j) and
// MATRIX.md's "Test parity scenario" column, this file populates the
// canonical scenarios named in MATRIX.md's "Test parity scenario = yes"
// rows. Each scenario sets up the real adapter (TasksAdapter or
// KeepAdapter) wired against an in-memory gateway fake, runs the
// engine's reconcile / precondition-recovery / dead-letter / bind path,
// and asserts identical engine outcomes (mutator calls, AdapterState
// mutations, log signals, gateway primitives invoked).
//
// The scenarios:
//
//   1. Reconcile: inbound apply skipped when wiki diverged (MATRIX row 1)
//   2. Reconcile: outbound push proceeds when wiki not diverged
//   3. Precondition recovery: remote-deleted branch (MATRIX row 6)
//   4. Precondition recovery: remote-unchanged-repatch branch
//   5. Precondition recovery: remote-authoritative-apply branch
//   6. Dead-letter retry: PushFailureCount accumulates (MATRIX row 7)
//   7. Bind ceremony: ValidateRemoteBinding gate (MATRIX row 2)

// parityFixedNow is the timestamp the parity tests pin into the
// FakeClock. Distinct from the other suite-local fixed times so a
// MatchError(...) failure points at the right test suite.
var parityFixedNow = time.Date(2026, 5, 4, 15, 0, 0, 0, time.UTC)

// parityPastChoke is a LastSuccessfulSyncAt value safely past the
// engine's 5s post-success rate-limit choke so the reconcile path
// proceeds.
var parityPastChoke = parityFixedNow.Add(-1 * time.Hour)

// errParityProgrammed is the suite-local sentinel parity tests use
// when programming a fake gateway client to fail.
var errParityProgrammed = errors.New("parity programmed failure")

// --- Tasks gateway fake (subset matching the real client surface) ---

// parityTasksClient is the in-memory stand-in for *gateway.TasksClient
// used by parity tests. Distinct name from the adapter-package's
// fakeTasksClient (which lives in package google_tasks_test) so the
// engine_test package can carry its own fake without an import.
type parityTasksClient struct {
	mu sync.Mutex

	// Programmable behavior.
	listTasks         []tasksgw.Task
	listTasksErr      error
	insertErr         error
	patchErr          error
	deleteErr         error
	listTaskListsErr  error
	taskLists         []tasksgw.TaskList

	// Recorded calls.
	listTasksCalls    int
	insertCalls       []parityTasksInsertCall
	patchCalls        []parityTasksPatchCall
	deleteCalls       []parityTasksDeleteCall

	nextInsertID int
}

type parityTasksInsertCall struct {
	TasklistID, Title, Notes string
	Status                   tasksgw.TaskStatus
}

type parityTasksPatchCall struct {
	TasklistID, TaskID string
	Fields             tasksgw.PatchFields
	Etag               string
}

type parityTasksDeleteCall struct {
	TasklistID, TaskID string
}

func newParityTasksClient() *parityTasksClient {
	return &parityTasksClient{}
}

func (f *parityTasksClient) ListTaskLists(_ context.Context) ([]tasksgw.TaskList, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listTaskListsErr != nil {
		return nil, f.listTaskListsErr
	}
	return append([]tasksgw.TaskList(nil), f.taskLists...), nil
}

func (f *parityTasksClient) CreateTaskList(_ context.Context, title string) (tasksgw.TaskList, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := fmt.Sprintf("created-list-%s", title)
	tl := tasksgw.TaskList{ID: id, Title: title}
	f.taskLists = append(f.taskLists, tl)
	return tl, nil
}

func (f *parityTasksClient) ListTasks(_ context.Context, _ string, _ time.Time, _ string) (tasksgw.TasksPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listTasksCalls++
	if f.listTasksErr != nil {
		return tasksgw.TasksPage{}, f.listTasksErr
	}
	return tasksgw.TasksPage{Tasks: append([]tasksgw.Task(nil), f.listTasks...)}, nil
}

func (f *parityTasksClient) InsertTask(_ context.Context, tasklistID, title, notes string, status tasksgw.TaskStatus, _ time.Time, _ string) (tasksgw.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.insertErr != nil {
		return tasksgw.Task{}, f.insertErr
	}
	f.nextInsertID++
	id := fmt.Sprintf("inserted-%d", f.nextInsertID)
	f.insertCalls = append(f.insertCalls, parityTasksInsertCall{TasklistID: tasklistID, Title: title, Notes: notes, Status: status})
	return tasksgw.Task{ID: id, Title: title, Notes: notes, Status: status}, nil
}

func (f *parityTasksClient) PatchTask(_ context.Context, tasklistID, taskID string, fields tasksgw.PatchFields, etag string) (tasksgw.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.patchCalls = append(f.patchCalls, parityTasksPatchCall{
		TasklistID: tasklistID, TaskID: taskID, Fields: fields, Etag: etag,
	})
	if f.patchErr != nil {
		return tasksgw.Task{}, f.patchErr
	}
	return tasksgw.Task{ID: taskID, Title: fields.Title, Notes: fields.Notes, Status: fields.Status}, nil
}

func (f *parityTasksClient) DeleteTask(_ context.Context, tasklistID, taskID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteCalls = append(f.deleteCalls, parityTasksDeleteCall{TasklistID: tasklistID, TaskID: taskID})
	return f.deleteErr
}

// parityTasksCreds is a fixed-token credential reader.
type parityTasksCreds struct{ token string }

func (c *parityTasksCreds) LoadRefreshToken(_ context.Context, _ wikipage.PageIdentifier) (string, error) {
	return c.token, nil
}

// paritySilentLogger is a logger that swallows all output. Used for
// the adapters' Logger interface (separate from the engine's
// captureLogger which records lines for assertion).
type paritySilentLogger struct{}

func (paritySilentLogger) Info(string, ...any)  {}
func (paritySilentLogger) Error(string, ...any) {}

// --- Keep gateway fake (subset matching the real client surface) ---

// parityKeepClient is the in-memory stand-in for *gateway.KeepClient
// used by parity tests. Distinct from the adapter-package's
// fakeKeepClient (which lives in package google_keep_test).
type parityKeepClient struct {
	mu sync.Mutex

	// Programmable behavior.
	changesResponses []keepgw.ChangesResponse
	changesDefault   keepgw.ChangesResponse
	changesErr       error

	// Recorded calls.
	changes []keepgw.ChangesRequest

	nextCreatedListID int
}

func newParityKeepClient() *parityKeepClient {
	return &parityKeepClient{}
}

func (f *parityKeepClient) Changes(_ context.Context, req keepgw.ChangesRequest) (keepgw.ChangesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.changes = append(f.changes, req)
	if f.changesErr != nil {
		return keepgw.ChangesResponse{}, f.changesErr
	}
	if len(f.changesResponses) > 0 {
		resp := f.changesResponses[0]
		f.changesResponses = f.changesResponses[1:]
		return resp, nil
	}
	return f.changesDefault, nil
}

func (f *parityKeepClient) CreateList(_ context.Context, title string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextCreatedListID++
	return fmt.Sprintf("created-keep-list-%d-%s", f.nextCreatedListID, title), nil
}

func (f *parityKeepClient) CreateListWithItems(_ context.Context, _ string, _ []keepgw.ListItemSpec) (keepgw.CreateListResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextCreatedListID++
	return keepgw.CreateListResult{
		ListServerID: fmt.Sprintf("created-keep-list-%d", f.nextCreatedListID),
	}, nil
}

// parityKeepCreds is a fixed-bundle credential reader for Keep.
type parityKeepCreds struct {
	token  string
	email  string
	device string
}

func (c *parityKeepCreds) LoadMasterToken(_ context.Context, _ wikipage.PageIdentifier) (googlekeep.MasterTokenBundle, error) {
	return googlekeep.MasterTokenBundle{
		MasterToken: c.token, Email: c.email, AndroidID: c.device,
	}, nil
}

// parityKeepClock is a fixed clock for Keep adapter wiring.
type parityKeepClock struct{ now time.Time }

func (c *parityKeepClock) Now() time.Time { return c.now }

// --- shared parity context types ---

// parityContext bundles everything a single scenario needs to drive a
// real adapter through the engine. The two adapter-specific factories
// (newTasksParityContext / newKeepParityContext) build distinct
// instances; the scenario body operates on the shared shape so the
// same assertions run against both backends.
type parityContext struct {
	// Identity (shared across both adapters).
	profileID    wikipage.PageIdentifier
	page         string
	listName     string
	remoteHandle string
	kind         connectors.ConnectorKind

	// Engine wiring.
	store    *enginetesting.FakeBindingStore
	lease    *connectors.LeaseTable
	clock    *enginetesting.FakeClock
	reader   *recordingChecklistReader
	tracker  *orderTracker
	mutator  *trackingChecklistMutator
	supr     *recordingSuppressor
	logger   *captureLogger
	engine   *engine.Engine

	// Backend-specific gateway fakes (only one is non-nil per context).
	tasksClient *parityTasksClient
	keepClient  *parityKeepClient

	// AdapterState seeding helpers (each backend has a different shape
	// for the uid → ref map; the scenario body uses these to translate
	// abstract "wiki uid X is mapped to remote ref Y" into the right
	// AdapterState subtree at seed time).
	seedItemIDMap func(uidToRef map[string]string) connectors.AdapterState
}

// newTasksParityContext wires a TasksAdapter against a parityTasksClient
// and constructs an engine ready to Sync. The remote handle is the
// stable string "tasklist-parity-1" so reviewers can grep for it.
func newTasksParityContext() *parityContext {
	const profile = wikipage.PageIdentifier("alice_profile")
	const page = "groceries"
	const list = "this_week"
	const handle = "tasklist-parity-1"

	tasksClient := newParityTasksClient()
	creds := &parityTasksCreds{token: "rt-parity"}
	clientFactory := func(_ context.Context, _ wikipage.PageIdentifier, _ string) (googletasks.TasksClient, error) {
		return tasksClient, nil
	}
	adapter, err := googletasks.NewTasksAdapter(creds, clientFactory, paritySilentLogger{})
	Expect(err).NotTo(HaveOccurred())

	store := enginetesting.NewFakeBindingStore()
	lease := connectors.NewLeaseTable()
	lease.SignalReady()
	clock := enginetesting.NewFakeClock(parityFixedNow)
	reader := &recordingChecklistReader{checklist: &apiv1.Checklist{}}
	tracker := &orderTracker{}
	mutator := &trackingChecklistMutator{
		recordingChecklistMutator: &recordingChecklistMutator{},
		tracker:                   tracker,
	}
	supr := &recordingSuppressor{tracker: tracker}
	logger := &captureLogger{}

	eng, err := engine.NewEngine(adapter, lease, reader, mutator, supr, logger, clock, store)
	Expect(err).NotTo(HaveOccurred())

	return &parityContext{
		profileID:    profile,
		page:         page,
		listName:     list,
		remoteHandle: handle,
		kind:         connectors.ConnectorKindGoogleTasks,
		store:        store,
		lease:        lease,
		clock:        clock,
		reader:       reader,
		tracker:      tracker,
		mutator:      mutator,
		supr:         supr,
		logger:       logger,
		engine:       eng,
		tasksClient:  tasksClient,
		seedItemIDMap: func(uidToRef map[string]string) connectors.AdapterState {
			ids := map[string]string{}
			for k, v := range uidToRef {
				ids[k] = v
			}
			return connectors.AdapterState{
				googletasks.AdapterStateKeyItemIDMap:      ids,
				googletasks.AdapterStateKeyItemEtags:      map[string]any{},
				googletasks.AdapterStateKeyLastUpdatedMin: parityFixedNow.Format(time.RFC3339),
			}
		},
	}
}

// newKeepParityContext wires a KeepAdapter against a parityKeepClient
// and constructs an engine ready to Sync. The remote handle is the
// stable string "keep-list-parity-1".
func newKeepParityContext() *parityContext {
	const profile = wikipage.PageIdentifier("alice_profile")
	const page = "groceries"
	const list = "this_week"
	const handle = "keep-list-parity-1"

	keepClient := newParityKeepClient()
	creds := &parityKeepCreds{token: "mt-parity", email: "u@example.com", device: "android-parity"}
	clientFactory := func(_ context.Context, _ wikipage.PageIdentifier, _, _ string) (googlekeep.KeepClient, error) {
		return keepClient, nil
	}
	keepClock := &parityKeepClock{now: parityFixedNow}
	adapter, err := googlekeep.NewKeepAdapter(creds, clientFactory, keepClock, paritySilentLogger{})
	Expect(err).NotTo(HaveOccurred())

	store := enginetesting.NewFakeBindingStore()
	lease := connectors.NewLeaseTable()
	lease.SignalReady()
	clock := enginetesting.NewFakeClock(parityFixedNow)
	reader := &recordingChecklistReader{checklist: &apiv1.Checklist{}}
	tracker := &orderTracker{}
	mutator := &trackingChecklistMutator{
		recordingChecklistMutator: &recordingChecklistMutator{},
		tracker:                   tracker,
	}
	supr := &recordingSuppressor{tracker: tracker}
	logger := &captureLogger{}

	eng, err := engine.NewEngine(adapter, lease, reader, mutator, supr, logger, clock, store)
	Expect(err).NotTo(HaveOccurred())

	return &parityContext{
		profileID:    profile,
		page:         page,
		listName:     list,
		remoteHandle: handle,
		kind:         connectors.ConnectorKindGoogleKeep,
		store:        store,
		lease:        lease,
		clock:        clock,
		reader:       reader,
		tracker:      tracker,
		mutator:      mutator,
		supr:         supr,
		logger:       logger,
		engine:       eng,
		keepClient:   keepClient,
		seedItemIDMap: func(uidToRef map[string]string) connectors.AdapterState {
			// Engine's own item_id_map subtree drives the outbound diff.
			// Adapter-side item_mapping subtree carries Keep's per-item
			// (server_id, base_version, client_id) bookkeeping for
			// PatchRemote / DeleteRemote calls.
			engineIDMap := map[string]string{}
			adapterMapping := map[string]any{}
			for uid, ref := range uidToRef {
				engineIDMap[uid] = ref
				adapterMapping[ref] = map[string]any{
					"server_id":    ref,
					"base_version": "bv-" + ref,
					"client_id":    "cli-" + ref,
				}
			}
			return connectors.AdapterState{
				"item_id_map":                            engineIDMap,
				googlekeep.AdapterStateKeyItemMapping:    adapterMapping,
				googlekeep.AdapterStateKeyKeepCursor:     "v100",
				googlekeep.AdapterStateKeyLabelIDs:       map[string]any{},
				googlekeep.AdapterStateKeyKeepNoteClientID: "client-list-parity-1",
			}
		},
	}
}

// keepClientIDFor mirrors the KeepAdapter's adapter-internal
// buildKeepItemID helper so insert-echo Changes responses can stage
// the right ID on the test side. Format: "<ms-hex>.<sha256(salt)[:8]-hex>"
// — matches gkeepapi's _generateId convention.
func keepClientIDFor(now time.Time, salt string) string {
	sum := sha256.Sum256([]byte(salt))
	return fmt.Sprintf("%x.%s", now.UnixMilli(), hex.EncodeToString(sum[:8]))
}

// keepNodeForRef builds a non-trashed Keep gateway node representing a
// LIST_ITEM under the parity remote handle. Used by Keep scenarios that
// stage "the remote item lives on Keep" without coupling the test to
// adapter-internal id-derivation logic.
func keepNodeForRef(ref, text string, checked bool) keepgw.Node {
	return keepgw.Node{
		Kind:           "notes#node",
		ID:             "cli-" + ref,
		ServerID:       ref,
		Type:           keepgw.NodeTypeListItem,
		ParentID:       "keep-list-parity-1",
		ParentServerID: "keep-list-parity-1",
		Text:           text,
		Checked:        checked,
		BaseVersion:    "bv-" + ref,
	}
}

// keepTrashedNodeForRef builds a trashed (soft-deleted) LIST_ITEM node.
// Used by precondition-recovery scenarios staging the remote-deleted
// branch on Keep.
func keepTrashedNodeForRef(ref string) keepgw.Node {
	out := keepNodeForRef(ref, "stale", false)
	out.Timestamps.Trashed = parityFixedNow
	return out
}

var _ = Describe("Parity scenarios across real adapters", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	// --- Reconcile scenarios -----------------------------------------------

	Describe("inbound apply skipped when wiki diverged (MATRIX row 1)", func() {
		// The wiki has a user-source op-log event past LastSyncedSeq for
		// the uid; remote pulls the same uid with a different value.
		// Engine's classifier marks the uid as diverged → no
		// UpdateItemForSync call. AppendSyncEvent is not called for this
		// uid (no successful primitive ran on it), but the binding
		// proceeds to SaveBinding (the tick still completes).
		const knownUID = "uid-diverged-1"
		const ref = "task-diverged-1"
		const refKeep = "keep-srv-diverged-1"

		setupWikiDiverged := func(p *parityContext, mappedRef string) {
			p.reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{
					{Uid: knownUID, Text: "milk-user-edit"},
				},
				Events: []*apiv1.ChecklistEvent{
					{Seq: 11, Src: "connector:other:apply", Op: "set_text", Uid: knownUID},
				},
				MaxSeq: 11,
			}
			p.store.SeedBinding(connectors.Binding{
				ProfileID: p.profileID, Page: p.page, ListName: p.listName,
				RemoteHandle:         p.remoteHandle,
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: parityPastChoke,
				LastSyncedSeq:        10,
				AdapterState:         p.seedItemIDMap(map[string]string{knownUID: mappedRef}),
			}, p.kind)
		}

		runScenario := func(p *parityContext) {
			Expect(p.engine.Sync(ctx, connectors.BindingKey{
				ProfileID: string(p.profileID),
				Page:      p.page,
				ListName:  p.listName,
			})).To(Succeed())
		}

		assertParityOutcome := func(p *parityContext) {
			// The engine must NOT have called UpdateItemForSync for the
			// diverged uid (the strict-behavior-wins assertion).
			Expect(p.mutator.recordingChecklistMutator.updateCalls).To(BeEmpty())
			// The classifier-skip path emits a structured log line;
			// presence of the log signal is part of parity.
			gotDivergedLog := false
			for _, line := range p.logger.snapshot() {
				if containsSubstring(line.Format, "wiki_diverged_skipped_inbound") {
					gotDivergedLog = true
					break
				}
			}
			Expect(gotDivergedLog).To(BeTrue(),
				"engine must emit wiki_diverged_skipped_inbound log signal")
		}

		When("the adapter is TasksAdapter", func() {
			var p *parityContext
			BeforeEach(func() {
				p = newTasksParityContext()
				p.tasksClient.listTasks = []tasksgw.Task{
					{ID: ref, Title: "milk-from-phone", Status: tasksgw.TaskStatusNeedsAction, Updated: parityFixedNow},
				}
				setupWikiDiverged(p, ref)
				runScenario(p)
			})

			It("should not apply the diverged remote item to the wiki", func() {
				assertParityOutcome(p)
			})
		})

		When("the adapter is KeepAdapter", func() {
			var p *parityContext
			BeforeEach(func() {
				p = newKeepParityContext()
				p.keepClient.changesDefault = keepgw.ChangesResponse{
					ToVersion: "v200",
					Nodes: []keepgw.Node{
						keepNodeForRef(refKeep, "milk-from-phone", false),
					},
				}
				setupWikiDiverged(p, refKeep)
				runScenario(p)
			})

			It("should not apply the diverged remote item to the wiki", func() {
				assertParityOutcome(p)
			})
		})
	})

	Describe("outbound push proceeds when wiki not diverged (MATRIX row 1)", func() {
		// Wiki has an item the adapter has never seen (no item_id_map
		// entry) → engine calls InsertRemote for it. The successful
		// primitive triggers AppendSyncEvent("outbound_inserted").
		const newUID = "uid-fresh-1"

		setupOutboundInsert := func(p *parityContext) {
			p.reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{
					{Uid: newUID, Text: "eggs"},
				},
			}
			p.store.SeedBinding(connectors.Binding{
				ProfileID: p.profileID, Page: p.page, ListName: p.listName,
				RemoteHandle:         p.remoteHandle,
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: parityPastChoke,
				AdapterState:         p.seedItemIDMap(map[string]string{}),
			}, p.kind)
		}

		runScenario := func(p *parityContext) {
			Expect(p.engine.Sync(ctx, connectors.BindingKey{
				ProfileID: string(p.profileID),
				Page:      p.page,
				ListName:  p.listName,
			})).To(Succeed())
		}

		assertParityOutcome := func(p *parityContext) {
			// AppendSyncEvent must record an outbound_inserted event for
			// the new uid (engine self-event protocol).
			gotAppendInsert := false
			for _, c := range p.mutator.recordingChecklistMutator.appendCalls {
				if c.UID == newUID && c.Op == "outbound_inserted" {
					gotAppendInsert = true
					break
				}
			}
			Expect(gotAppendInsert).To(BeTrue(),
				"engine must AppendSyncEvent(outbound_inserted) for the new uid")

			// SaveBinding must have been called (the tick succeeded).
			Expect(p.store.RecordedSaveBinding).NotTo(BeEmpty())

			// The persisted binding's item_id_map must now contain the
			// new uid mapping.
			saved := p.store.RecordedSaveBinding[len(p.store.RecordedSaveBinding)-1].Binding
			idMap, ok := saved.AdapterState["item_id_map"].(map[string]string)
			Expect(ok).To(BeTrue(), "item_id_map subtree must be present after Sync")
			Expect(idMap[newUID]).NotTo(BeEmpty(),
				"engine must record the new uid → ref mapping in item_id_map")
		}

		When("the adapter is TasksAdapter", func() {
			var p *parityContext
			BeforeEach(func() {
				p = newTasksParityContext()
				setupOutboundInsert(p)
				runScenario(p)
			})

			It("should call the gateway's InsertTask exactly once", func() {
				Expect(p.tasksClient.insertCalls).To(HaveLen(1))
			})

			It("should record AppendSyncEvent and update item_id_map", func() {
				assertParityOutcome(p)
			})
		})

		When("the adapter is KeepAdapter", func() {
			var p *parityContext
			BeforeEach(func() {
				p = newKeepParityContext()
				// First Changes call is the engine's PullRemote (no items).
				// Second Changes call is InsertRemote: must echo the
				// adapter's deterministic client id back so the adapter
				// can extract the new ServerID.
				clientID := keepClientIDFor(parityFixedNow, newUID)
				p.keepClient.changesResponses = []keepgw.ChangesResponse{
					{ToVersion: "v200"}, // pull
					{ // insert echo
						ToVersion: "v201",
						Nodes: []keepgw.Node{
							{
								ID:       clientID,
								ServerID: "keep-srv-fresh-1",
								Type:     keepgw.NodeTypeListItem,
								ParentID: "keep-list-parity-1",
								Text:     "eggs",
							},
						},
					},
				}
				setupOutboundInsert(p)
				runScenario(p)
			})

			It("should send a Changes request that includes the inserted item", func() {
				// The adapter routes pull then insert through the same
				// Changes endpoint. We expect at least 2 calls (pull +
				// insert) and the insert request must carry one Node.
				Expect(len(p.keepClient.changes)).To(BeNumerically(">=", 2))
				gotInsert := false
				for _, req := range p.keepClient.changes {
					if len(req.Nodes) == 1 && req.Nodes[0].Type == keepgw.NodeTypeListItem {
						gotInsert = true
						break
					}
				}
				Expect(gotInsert).To(BeTrue())
			})

			It("should record AppendSyncEvent and update item_id_map", func() {
				assertParityOutcome(p)
			})
		})
	})

	// --- Precondition recovery scenarios (MATRIX row 6) --------------------

	Describe("precondition recovery: remote-deleted branch", func() {
		const knownUID = "uid-precond-deleted-1"
		const tasksRef = "task-precond-deleted-1"
		const keepRef = "keep-srv-precond-deleted-1"

		runRecovery := func(p *parityContext, ref connectors.RemoteRef) error {
			binding := connectors.Binding{
				ProfileID: p.profileID, Page: p.page, ListName: p.listName,
				RemoteHandle: p.remoteHandle,
				State:        connectors.BindingStateActive,
				AdapterState: p.seedItemIDMap(map[string]string{knownUID: string(ref)}),
			}
			idMap := map[string]string{knownUID: string(ref)}
			wikiItem := connectors.WikiItem{UID: knownUID, Text: "milk"}
			return p.engine.RunPreconditionRecoveryForTest(ctx, binding, ref, knownUID, wikiItem, idMap, errParityProgrammed)
		}

		assertDeletedBranch := func(p *parityContext) {
			Expect(p.mutator.recordingChecklistMutator.deleteCalls).To(HaveLen(1))
			Expect(p.mutator.recordingChecklistMutator.deleteCalls[0].UID).To(Equal(knownUID))
			Expect(p.mutator.recordingChecklistMutator.updateCalls).To(BeEmpty())
			Expect(p.mutator.recordingChecklistMutator.addCalls).To(BeEmpty())
		}

		When("the adapter is TasksAdapter and the task is gone", func() {
			var p *parityContext
			var recoveryErr error
			BeforeEach(func() {
				p = newTasksParityContext()
				// Tasks's ReadRemoteByRef walks ListTasks; an empty list
				// → Deleted=true.
				p.tasksClient.listTasks = nil
				recoveryErr = runRecovery(p, connectors.RemoteRef(tasksRef))
			})

			It("should not return an error", func() {
				Expect(recoveryErr).NotTo(HaveOccurred())
			})

			It("should call DeleteItemForSync once for the uid", func() {
				assertDeletedBranch(p)
			})
		})

		When("the adapter is KeepAdapter and the node is trashed", func() {
			var p *parityContext
			var recoveryErr error
			BeforeEach(func() {
				p = newKeepParityContext()
				// Keep's ReadRemoteByRef pulls Changes; the matching
				// node has a Trashed timestamp → Deleted=true.
				p.keepClient.changesDefault = keepgw.ChangesResponse{
					ToVersion: "v300",
					Nodes: []keepgw.Node{
						keepTrashedNodeForRef(keepRef),
					},
				}
				recoveryErr = runRecovery(p, connectors.RemoteRef(keepRef))
			})

			It("should not return an error", func() {
				Expect(recoveryErr).NotTo(HaveOccurred())
			})

			It("should call DeleteItemForSync once for the uid", func() {
				assertDeletedBranch(p)
			})
		})
	})

	Describe("precondition recovery: wiki-wins re-patch (remote != wiki)", func() {
		// Per ADR-0015 + 2026-05-06 production fix: when the recovery
		// reads a remote that differs from the wiki, the engine
		// re-PATCHes (wiki-wins) instead of applying remote. The
		// patch path is gated on classification[uid].WikiDiverged, so
		// any recovery call site already implies user/cross-connector
		// wiki intent — clobbering it with remote was the regression.
		// True conflicts surface on the next tick via PullRemote +
		// applyInbound's RemoteDiverged path.
		const knownUID = "uid-precond-apply-1"
		const tasksRef = "task-precond-apply-1"
		const keepRef = "keep-srv-precond-apply-1"

		runRecovery := func(p *parityContext, ref connectors.RemoteRef) error {
			binding := connectors.Binding{
				ProfileID: p.profileID, Page: p.page, ListName: p.listName,
				RemoteHandle: p.remoteHandle,
				State:        connectors.BindingStateActive,
				AdapterState: p.seedItemIDMap(map[string]string{knownUID: string(ref)}),
			}
			idMap := map[string]string{knownUID: string(ref)}
			wikiItem := connectors.WikiItem{UID: knownUID, Text: "milk"}
			return p.engine.RunPreconditionRecoveryForTest(ctx, binding, ref, knownUID, wikiItem, idMap, errParityProgrammed)
		}

		assertWikiWinsRepatch := func(p *parityContext) {
			// No mutator writes to the wiki (no remote-wins apply).
			Expect(p.mutator.recordingChecklistMutator.updateCalls).To(BeEmpty())
			Expect(p.mutator.recordingChecklistMutator.addCalls).To(BeEmpty())
			Expect(p.mutator.recordingChecklistMutator.deleteCalls).To(BeEmpty())
		}

		When("the adapter is TasksAdapter and remote returns different fields", func() {
			var p *parityContext
			var recoveryErr error
			BeforeEach(func() {
				p = newTasksParityContext()
				// Remote shows different title than wiki ("milk").
				p.tasksClient.listTasks = []tasksgw.Task{
					{ID: tasksRef, Title: "milk-from-phone", Status: tasksgw.TaskStatusNeedsAction},
				}
				recoveryErr = runRecovery(p, connectors.RemoteRef(tasksRef))
			})

			It("should not return an error", func() {
				Expect(recoveryErr).NotTo(HaveOccurred())
			})

			It("should not apply the remote to the wiki (no remote-wins)", func() {
				assertWikiWinsRepatch(p)
			})

			It("should re-patch via the Tasks gateway", func() {
				Expect(p.tasksClient.patchCalls).NotTo(BeEmpty())
			})
		})

		When("the adapter is KeepAdapter and remote returns different fields", func() {
			var p *parityContext
			var recoveryErr error
			BeforeEach(func() {
				p = newKeepParityContext()
				p.keepClient.changesDefault = keepgw.ChangesResponse{
					ToVersion: "v400",
					Nodes: []keepgw.Node{
						keepNodeForRef(keepRef, "milk-from-phone", false),
					},
				}
				recoveryErr = runRecovery(p, connectors.RemoteRef(keepRef))
			})

			It("should not return an error", func() {
				Expect(recoveryErr).NotTo(HaveOccurred())
			})

			It("should not apply the remote to the wiki (no remote-wins)", func() {
				assertWikiWinsRepatch(p)
			})
		})
	})

	// --- Dead-letter retry scenario (MATRIX row 7) -------------------------

	Describe("dead-letter retry: PushFailureCount accumulates and trips threshold", func() {
		// Drive the recordPushFailure helper directly through the engine
		// for both adapters and assert the bookkeeping shape is identical
		// across backends. The full reconcile-driven retry loop is
		// already covered in dead_letter_test.go using FakeAdapter; this
		// parity scenario asserts that the engine's per-uid bookkeeping
		// is adapter-agnostic.
		const failingUID = "uid-deadletter-1"

		assertThresholdReached := func(p *parityContext, updated connectors.Binding) {
			// After threshold failures, the binding's push_failures
			// subtree must record count >= threshold for the failing uid.
			rec := pushFailuresOf(updated.AdapterState)[failingUID]
			Expect(rec["count"]).To(Equal(engine.DeadLetterThresholdForTest))
			Expect(rec["next_attempt_at"]).NotTo(BeEmpty())

			// The engine must emit a dead_letter_threshold_breached
			// log line at the threshold boundary.
			breached := false
			for _, line := range p.logger.snapshot() {
				if containsSubstring(line.Format, "dead_letter_threshold_breached") {
					breached = true
					break
				}
			}
			Expect(breached).To(BeTrue(),
				"engine must emit dead_letter_threshold_breached log signal")

			// Once at threshold, shouldSkipPush must report dead_letter.
			skip, reason := p.engine.ShouldSkipPushForTest(updated, failingUID)
			Expect(skip).To(BeTrue())
			Expect(reason).To(Equal("dead_letter"))
		}

		runFailureLoop := func(p *parityContext) connectors.Binding {
			binding := connectors.Binding{
				ProfileID: p.profileID, Page: p.page, ListName: p.listName,
				RemoteHandle: p.remoteHandle,
				State:        connectors.BindingStateActive,
				AdapterState: p.seedItemIDMap(map[string]string{}),
			}
			cur := binding
			for i := 0; i < engine.DeadLetterThresholdForTest; i++ {
				cur = p.engine.RecordPushFailureForTest(cur, failingUID, "outbound_inserted", errParityProgrammed)
			}
			return cur
		}

		When("the adapter is TasksAdapter", func() {
			var p *parityContext
			var updated connectors.Binding
			BeforeEach(func() {
				p = newTasksParityContext()
				updated = runFailureLoop(p)
			})

			It("should record threshold count and emit threshold-breached log", func() {
				assertThresholdReached(p, updated)
			})
		})

		When("the adapter is KeepAdapter", func() {
			var p *parityContext
			var updated connectors.Binding
			BeforeEach(func() {
				p = newKeepParityContext()
				updated = runFailureLoop(p)
			})

			It("should record threshold count and emit threshold-breached log", func() {
				assertThresholdReached(p, updated)
			})
		})
	})

	// --- Bind ceremony scenario (MATRIX row 2) -----------------------------

	Describe("bind ceremony: ValidateRemoteBinding gate", func() {
		// When ValidateRemoteBinding refuses, the engine.Bind path must:
		//   - return the wrapped error
		//   - NOT call SeedBindingState
		//   - NOT call SaveBinding
		//   - NOT take the lease (the tuple stays unowned)
		//
		// Each adapter has a different "refuse" path:
		//   - Tasks: subtasks present in the chosen list →
		//     ErrTasksListHasSubtasks
		//   - Keep: chosen note is not a LIST type → ErrKeepNoteNotAList
		//
		// The parity assertion is on the engine's behavior (it gated
		// correctly), not on the specific sentinel.

		assertBindRefused := func(p *parityContext, bindErr error) {
			Expect(bindErr).To(HaveOccurred())
			// SaveBinding must NOT have run.
			Expect(p.store.RecordedSaveBinding).To(BeEmpty())
			// The lease must NOT have been taken for the (page, listName)
			// tuple.
			_, owned := p.lease.LookupOwner(connectors.ChecklistKey{
				Page: p.page, ListName: p.listName,
			})
			Expect(owned).To(BeFalse())
		}

		When("the adapter is TasksAdapter and the list has subtasks", func() {
			var p *parityContext
			var bindErr error
			BeforeEach(func() {
				p = newTasksParityContext()
				// A task with non-empty Parent → translator's HasSubtasks
				// returns true → ValidateRemoteBinding returns
				// ErrTasksListHasSubtasks.
				p.tasksClient.listTasks = []tasksgw.Task{
					{ID: "parent-1", Title: "parent task"},
					{ID: "child-1", Title: "subtask", Parent: "parent-1"},
				}
				_, bindErr = p.engine.Bind(ctx, p.profileID, p.page, p.listName, p.remoteHandle)
			})

			It("should return an error wrapping ErrTasksListHasSubtasks", func() {
				Expect(bindErr).To(MatchError(googletasks.ErrTasksListHasSubtasks))
			})

			It("should not persist the binding or take the lease", func() {
				assertBindRefused(p, bindErr)
			})
		})

		When("the adapter is KeepAdapter and the chosen note is not a LIST", func() {
			var p *parityContext
			var bindErr error
			BeforeEach(func() {
				p = newKeepParityContext()
				// The remote note exists but is a NOTE (not a LIST) →
				// ValidateRemoteBinding returns ErrKeepNoteNotAList.
				p.keepClient.changesDefault = keepgw.ChangesResponse{
					ToVersion: "v500",
					Nodes: []keepgw.Node{
						{
							ID:       "client-bad-note",
							ServerID: p.remoteHandle,
							Type:     keepgw.NodeTypeNote,
							Title:    "free-form note (not a list)",
						},
					},
				}
				_, bindErr = p.engine.Bind(ctx, p.profileID, p.page, p.listName, p.remoteHandle)
			})

			It("should return an error wrapping ErrKeepNoteNotAList", func() {
				Expect(bindErr).To(MatchError(googlekeep.ErrKeepNoteNotAList))
			})

			It("should not persist the binding or take the lease", func() {
				assertBindRefused(p, bindErr)
			})
		})
	})

	// --- Fix #2 parity: push gate (MATRIX row 1) -------------------------

	Describe("outbound patch skipped when wiki not diverged (Fix #2 push gate)", func() {
		// An item exists on both wiki and remote, but the wiki has no
		// user events since LastSyncedSeq → WikiDiverged=false → the
		// engine must NOT call PatchRemote / Changes for this item.
		const knownUID = "uid-no-div-parity-1"
		const ref = "task-no-div-parity-1"
		const refKeep = "keep-srv-no-div-1"

		setupNoDivergence := func(p *parityContext, mappedRef string) {
			// No user events → WikiDiverged=false.
			p.reader.checklist = &apiv1.Checklist{
				Items: []*apiv1.ChecklistItem{
					{Uid: knownUID, Text: "milk"},
				},
			}
			p.store.SeedBinding(connectors.Binding{
				ProfileID: p.profileID, Page: p.page, ListName: p.listName,
				RemoteHandle:         p.remoteHandle,
				State:                connectors.BindingStateActive,
				LastSuccessfulSyncAt: parityPastChoke,
				LastSyncedSeq:        10,
				AdapterState:         p.seedItemIDMap(map[string]string{knownUID: mappedRef}),
			}, p.kind)
		}

		runScenario := func(p *parityContext) {
			Expect(p.engine.Sync(ctx, connectors.BindingKey{
				ProfileID: string(p.profileID),
				Page:      p.page,
				ListName:  p.listName,
			})).To(Succeed())
		}

		When("the adapter is TasksAdapter", func() {
			var p *parityContext
			BeforeEach(func() {
				p = newTasksParityContext()
				// Remote returns the item unchanged (same etag — no
				// RemoteDiverged), wiki has no user events.
				p.tasksClient.listTasks = []tasksgw.Task{
					{ID: ref, Title: "milk", Status: tasksgw.TaskStatusNeedsAction, Updated: parityFixedNow},
				}
				setupNoDivergence(p, ref)
				runScenario(p)
			})

			It("should not call PatchTask (push gate: wiki not diverged)", func() {
				Expect(p.tasksClient.patchCalls).To(BeEmpty())
			})
		})

		When("the adapter is KeepAdapter", func() {
			var p *parityContext
			BeforeEach(func() {
				p = newKeepParityContext()
				// Remote returns the same item with matching BaseVersion
				// (bv-<ref> matches stored) → RemoteDiverged=false,
				// wiki has no user events → WikiDiverged=false.
				p.keepClient.changesDefault = keepgw.ChangesResponse{
					ToVersion: "v200",
					Nodes:     []keepgw.Node{keepNodeForRef(refKeep, "milk", false)},
				}
				setupNoDivergence(p, refKeep)
				runScenario(p)
			})

			It("should not send a Changes mutation for the item (push gate: wiki not diverged)", func() {
				// Keep's PullRemote and PatchRemote both go through Changes.
				// Only one Changes call (the pull) is expected; no patch
				// call should be present (mutation requests carry Nodes).
				patchCalls := 0
				for _, req := range p.keepClient.changes {
					if len(req.Nodes) > 0 {
						patchCalls++
					}
				}
				Expect(patchCalls).To(Equal(0))
			})
		})
	})

	// --- Fix #1 parity: 4-cell merge / conflict-remote-wins (MATRIX row 1) -

	Describe("conflict-remote-wins when both wiki and remote diverged (Fix #1 4-cell merge)", func() {
		// The wiki has user events (WikiDiverged=true) AND the remote
		// item has a different etag/BaseVersion than stored
		// (RemoteDiverged=true). Per ADR-0015 conflict-remote-wins rule,
		// the engine must apply the remote value despite the wiki edit.
		const knownUID = "uid-conflict-parity-1"
		const ref = "task-conflict-parity-1"
		const refKeep = "keep-srv-conflict-1"

		assertConflictRemoteWins := func(p *parityContext, remoteText string) {
			// UpdateItemForSync must have been called with the remote text.
			Expect(p.mutator.recordingChecklistMutator.updateCalls).To(HaveLen(1))
			Expect(p.mutator.recordingChecklistMutator.updateCalls[0].UID).To(Equal(knownUID))
			Expect(p.mutator.recordingChecklistMutator.updateCalls[0].Text).To(Equal(remoteText))
			// The conflict-remote-wins log signal must appear.
			gotLog := false
			for _, line := range p.logger.snapshot() {
				if containsSubstring(line.Format, "conflict_remote_wins") {
					gotLog = true
					break
				}
			}
			Expect(gotLog).To(BeTrue(), "engine must emit conflict_remote_wins log signal")
		}

		When("the adapter is TasksAdapter", func() {
			var p *parityContext
			BeforeEach(func() {
				p = newTasksParityContext()

				// Seed the binding with a stored etag for this task so the
				// adapter can detect RemoteDiverged when a new etag arrives.
				adapterState := connectors.AdapterState{
					googletasks.AdapterStateKeyItemIDMap: map[string]string{knownUID: ref},
					googletasks.AdapterStateKeyItemEtags: map[string]any{ref: "old-etag"},
					googletasks.AdapterStateKeyLastUpdatedMin: parityFixedNow.Add(-1 * time.Hour).Format(time.RFC3339),
				}
				// Wiki diverged: user edited since last sync (seq 11 > LastSyncedSeq 10).
				p.reader.checklist = &apiv1.Checklist{
					Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk-wiki-edit"}},
					Events: []*apiv1.ChecklistEvent{
						{Seq: 11, Src: "connector:other:apply", Op: "set_text", Uid: knownUID},
					},
					MaxSeq: 11,
				}
				p.store.SeedBinding(connectors.Binding{
					ProfileID: p.profileID, Page: p.page, ListName: p.listName,
					RemoteHandle:         p.remoteHandle,
					State:                connectors.BindingStateActive,
					LastSuccessfulSyncAt: parityPastChoke,
					LastSyncedSeq:        10,
					AdapterState:         adapterState,
				}, p.kind)
				// Remote returns the same task with a NEW etag → RemoteDiverged=true.
				p.tasksClient.listTasks = []tasksgw.Task{
					{ID: ref, Etag: "new-etag", Title: "milk-remote-wins", Status: tasksgw.TaskStatusNeedsAction, Updated: parityFixedNow},
				}

				Expect(p.engine.Sync(ctx, connectors.BindingKey{
					ProfileID: string(p.profileID),
					Page:      p.page,
					ListName:  p.listName,
				})).To(Succeed())
			})

			It("should apply the remote value (conflict-remote-wins)", func() {
				assertConflictRemoteWins(p, "milk-remote-wins")
			})
		})

		When("the adapter is KeepAdapter", func() {
			var p *parityContext
			BeforeEach(func() {
				p = newKeepParityContext()

				// Seed with a BaseVersion that DIFFERS from what the pull
				// will return → RemoteDiverged=true.
				// The keep seedItemIDMap sets base_version to "bv-<ref>".
				// The pull node below uses a different BaseVersion ("bv-v2-<ref>").
				adapterState := p.seedItemIDMap(map[string]string{knownUID: refKeep})
				// Wiki diverged.
				p.reader.checklist = &apiv1.Checklist{
					Items: []*apiv1.ChecklistItem{{Uid: knownUID, Text: "milk-wiki-edit"}},
					Events: []*apiv1.ChecklistEvent{
						{Seq: 11, Src: "connector:other:apply", Op: "set_text", Uid: knownUID},
					},
					MaxSeq: 11,
				}
				p.store.SeedBinding(connectors.Binding{
					ProfileID: p.profileID, Page: p.page, ListName: p.listName,
					RemoteHandle:         p.remoteHandle,
					State:                connectors.BindingStateActive,
					LastSuccessfulSyncAt: parityPastChoke,
					LastSyncedSeq:        10,
					AdapterState:         adapterState,
				}, p.kind)
				// Pull returns the item with a NEW BaseVersion → RemoteDiverged=true.
				// seedItemIDMap stores "bv-<refKeep>"; the node below carries "bv-v2-<refKeep>".
				changedNode := keepNodeForRef(refKeep, "milk-remote-wins", false)
				changedNode.BaseVersion = "bv-v2-" + refKeep
				p.keepClient.changesDefault = keepgw.ChangesResponse{
					ToVersion: "v200",
					Nodes:     []keepgw.Node{changedNode},
				}

				Expect(p.engine.Sync(ctx, connectors.BindingKey{
					ProfileID: string(p.profileID),
					Page:      p.page,
					ListName:  p.listName,
				})).To(Succeed())
			})

			It("should apply the remote value (conflict-remote-wins)", func() {
				assertConflictRemoteWins(p, "milk-remote-wins")
			})
		})
	})
})
