//revive:disable:dot-imports
//revive:disable:add-constant
//revive:disable:unused-receiver
package eager

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/keep/bridge"
	"github.com/brendanjerwin/simple_wiki/internal/keep/protocol"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// --- fakes ----------------------------------------------------------------

// fakePageReaderMutator is a minimal in-memory PageReaderMutator
// the bridge.BindingStore writes against.
type fakePageReaderMutator struct {
	mu       sync.Mutex
	pages    map[wikipage.PageIdentifier]wikipage.FrontMatter
	markdown map[wikipage.PageIdentifier]wikipage.Markdown
}

func newFakePageReaderMutator() *fakePageReaderMutator {
	return &fakePageReaderMutator{
		pages:    map[wikipage.PageIdentifier]wikipage.FrontMatter{},
		markdown: map[wikipage.PageIdentifier]wikipage.Markdown{},
	}
}

func (f *fakePageReaderMutator) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fm, ok := f.pages[id]
	if !ok {
		// Bridge BindingStore branches on os.ErrNotExist to treat
		// missing profile pages as "not connected" rather than a
		// hard error; mirror that here so SaveState can write the
		// initial state on a fresh page.
		return id, nil, os.ErrNotExist
	}
	return id, deepCopyFrontmatter(fm), nil
}

func (f *fakePageReaderMutator) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pages[id] = deepCopyFrontmatter(fm)
	return nil
}

func (f *fakePageReaderMutator) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	md, ok := f.markdown[id]
	if !ok {
		return id, "", nil
	}
	return id, md, nil
}

func (f *fakePageReaderMutator) WriteMarkdown(id wikipage.PageIdentifier, md wikipage.Markdown) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.markdown[id] = md
	return nil
}

func (*fakePageReaderMutator) DeletePage(_ wikipage.PageIdentifier) error { return nil }

func (*fakePageReaderMutator) ModifyMarkdown(_ wikipage.PageIdentifier, _ func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	return nil
}

func deepCopyFrontmatter(fm wikipage.FrontMatter) wikipage.FrontMatter {
	if fm == nil {
		return nil
	}
	out := make(wikipage.FrontMatter, len(fm))
	for k, v := range fm {
		out[k] = deepCopyAnyForFM(v)
	}
	return out
}

func deepCopyAnyForFM(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = deepCopyAnyForFM(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = deepCopyAnyForFM(vv)
		}
		return out
	default:
		return v
	}
}

// fakeMigrationKeepClient records every Changes() call and returns a
// canned pull response.
type fakeMigrationKeepClient struct {
	mu             sync.Mutex
	pullState      protocol.ChangesResponse
	pullError      error
	pulledRequests []protocol.ChangesRequest
}

func (c *fakeMigrationKeepClient) Changes(_ context.Context, req protocol.ChangesRequest) (protocol.ChangesResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pullError != nil {
		return protocol.ChangesResponse{}, c.pullError
	}
	if len(req.Nodes) == 0 && len(req.Labels) == 0 {
		c.pulledRequests = append(c.pulledRequests, req)
		return c.pullState, nil
	}
	return protocol.ChangesResponse{}, errors.New("migration must not push")
}

func (*fakeMigrationKeepClient) CreateList(_ context.Context, _ string) (string, error) {
	return "", errors.New("CreateList not used in migration tests")
}

func (*fakeMigrationKeepClient) CreateListWithItems(_ context.Context, _ string, _ []protocol.ListItemSpec) (protocol.CreateListResult, error) {
	return protocol.CreateListResult{}, errors.New("CreateListWithItems not used in migration tests")
}

func (c *fakeMigrationKeepClient) pullCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pulledRequests)
}

// fakeMigrationAuth bypasses the gpsoauth round-trip.
type fakeMigrationAuth struct{}

func (fakeMigrationAuth) ExchangeOAuthTokenForMasterToken(_ context.Context, _, _ string) (string, error) {
	return "mt", nil
}
func (fakeMigrationAuth) ExchangeMasterTokenForBearer(_ context.Context, _, _ string) (string, error) {
	return "bearer", nil
}

// fakeMigrationChecklist plays both ChecklistReader and ChecklistMutator
// for the migration's read-then-Keep-wins flow.
type fakeMigrationChecklist struct {
	mu          sync.Mutex
	items       []*apiv1.ChecklistItem
	updateCalls []migrationUpdateCall
}

type migrationUpdateCall struct {
	OwnerEmail, Page, ListName, UID, Text string
	Checked                               bool
	Tags                                  []string
}

func (c *fakeMigrationChecklist) ListItems(_ context.Context, _, _ string) (*apiv1.Checklist, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := &apiv1.Checklist{Items: make([]*apiv1.ChecklistItem, len(c.items))}
	for i, it := range c.items {
		cloned, ok := proto.Clone(it).(*apiv1.ChecklistItem)
		if !ok {
			return nil, errors.New("proto.Clone returned wrong type")
		}
		out.Items[i] = cloned
	}
	return out, nil
}

func (c *fakeMigrationChecklist) AddItemForSync(_ context.Context, _, _, _, _ string, _ bool, _ []string, _, _ string) (string, error) {
	return "", errors.New("AddItemForSync should not be called by migration")
}

func (c *fakeMigrationChecklist) UpdateItemForSync(_ context.Context, page, listName, ownerEmail, uid, text string, checked bool, tags []string, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updateCalls = append(c.updateCalls, migrationUpdateCall{
		OwnerEmail: ownerEmail, Page: page, ListName: listName,
		UID: uid, Text: text, Checked: checked, Tags: tags,
	})
	for _, it := range c.items {
		if it.GetUid() == uid {
			it.Text = text
			it.Checked = checked
			it.Tags = tags
		}
	}
	return nil
}

func (c *fakeMigrationChecklist) DeleteItemForSync(_ context.Context, _, _, _, _ string) error {
	return errors.New("DeleteItemForSync should not be called by migration")
}

// fakeMigrationSuppressor satisfies bridge.SyncSuppressor with no-op behavior.
type fakeMigrationSuppressor struct{}

func (fakeMigrationSuppressor) Suppress(_ wikipage.PageIdentifier, _, _ string)   {}
func (fakeMigrationSuppressor) Unsuppress(_ wikipage.PageIdentifier, _, _ string) {}

// fakeMigratorRecorder satisfies KeepBridgeFingerprintMigrator and
// records every call. Used by scan-job tests where we don't need
// real rebaseline behavior, just want to assert which bindings get
// enqueued.
type fakeMigratorRecorder struct {
	mu    sync.Mutex
	calls []migratorCall
	err   error
}

type migratorCall struct {
	ProfileID         wikipage.PageIdentifier
	Page, ListName string
}

func (f *fakeMigratorRecorder) MigrateBindingFingerprints(_ context.Context, profileID wikipage.PageIdentifier, page, listName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, migratorCall{ProfileID: profileID, Page: page, ListName: listName})
	return f.err
}

// fakeStateLoader satisfies KeepBridgeBindingStateLoader for the
// scan-job tests so we can express bindings without a real
// frontmatter file.
type fakeStateLoader struct {
	states map[wikipage.PageIdentifier]bridge.ConnectorState
}

func (f *fakeStateLoader) LoadState(profileID wikipage.PageIdentifier) (bridge.ConnectorState, error) {
	if f.states == nil {
		return bridge.ConnectorState{}, nil
	}
	return f.states[profileID], nil
}

// migrationClock returns a fixed time so cursor stamps are deterministic.
type migrationClock struct{}

func (migrationClock) Now() time.Time {
	return time.Date(2026, time.May, 1, 12, 0, 0, 0, time.UTC)
}

// freshConnectorForMigration returns a real bridge.Connector wired
// with all fakes plus the BindingStore against the given pages
// store. The seeded binding (page, listName) is MigratedFingerprints
// =false (legacy) by default.
//
//revive:disable-next-line:function-result-limit
func freshConnectorForMigration(
	profileID wikipage.PageIdentifier,
	page, listName, keepNoteID string,
	itemBindings map[string]bridge.ItemBinding,
) (*bridge.Connector, *bridge.BindingStore, *fakeMigrationKeepClient, *fakeMigrationChecklist) {
	pages := newFakePageReaderMutator()
	store := bridge.NewBindingStore(pages)

	// Seed the connector state on the profile page.
	state := bridge.ConnectorState{
		Email:       "user@example.com",
		MasterToken: "tok",
		Bindings: []bridge.Binding{{
			Page: page, ListName: listName,
			KeepNoteID: keepNoteID, KeepNoteTitle: listName,
			BoundAt:              time.Now().UTC(),
			MigratedFingerprints: false,
			ItemIDMap:            itemBindings,
		}},
	}
	Expect(store.SaveState(profileID, state)).To(Succeed())

	keep := &fakeMigrationKeepClient{}
	chk := &fakeMigrationChecklist{}

	c := bridge.NewConnector(store, nil, migrationClock{})
	c.SetClientBuilder(func(_ string) bridge.KeepClient { return keep })
	c.SetAuthBuilder(func(_ string) bridge.AuthExchanger { return fakeMigrationAuth{} })
	c.SetChecklistReader(chk)
	c.SetChecklistMutator(chk)
	c.SetSyncSuppressor(fakeMigrationSuppressor{})
	return c, store, keep, chk
}

// keepNodeForMigration is a helper to build pull-side LIST_ITEM nodes.
func keepNodeForMigration(serverID, parentNoteID, text string, checked bool, sortValue string) protocol.Node {
	return protocol.Node{
		Kind:           "notes#node",
		ID:             "client-" + serverID,
		ServerID:       serverID,
		ParentID:       parentNoteID,
		ParentServerID: parentNoteID,
		Type:           protocol.NodeTypeListItem,
		Text:           text,
		Checked:        checked,
		SortValue:      sortValue,
		BaseVersion:    "v-base-" + serverID,
	}
}

// --- scan job -------------------------------------------------------------

var _ = Describe("KeepBridgeFingerprintMigrationScanJob", func() {
	const profileFile = "profile_test.md"
	const profileFileMigrated = "profile_done.md"
	const profileID = wikipage.PageIdentifier("profile_test")
	const profileIDMigrated = wikipage.PageIdentifier("profile_done")

	Describe("when scanning a data dir with mixed bindings", func() {
		var (
			recorder    *fakeMigratorRecorder
			coordinator *jobs.JobQueueCoordinator
			scanErr     error
		)

		BeforeEach(func() {
			scanner := NewMockDataDirScanner()
			scanner.AddFile(profileFile, []byte(`+++
identifier = "profile_test"
[wiki]
[wiki.connectors]
[wiki.connectors.google_keep]
email = "user@example.com"
+++
`))
			scanner.AddFile(profileFileMigrated, []byte(`+++
identifier = "profile_done"
[wiki]
[wiki.connectors]
[wiki.connectors.google_keep]
email = "other@example.com"
+++
`))
			scanner.AddFile("regular_page.md", []byte(`+++
identifier = "regular"
+++
This page has no Keep connector configured.`))

			recorder = &fakeMigratorRecorder{}
			coordinator = jobs.NewJobQueueCoordinator(stubLogger{})

			loader := &fakeStateLoader{states: map[wikipage.PageIdentifier]bridge.ConnectorState{
				profileID: {
					Email: "user@example.com", MasterToken: "tok",
					Bindings: []bridge.Binding{
						{Page: "shopping", ListName: "Grocery", MigratedFingerprints: false},
						{Page: "shopping", ListName: "Hardware", MigratedFingerprints: false},
					},
				},
				profileIDMigrated: {
					Email: "other@example.com", MasterToken: "tok",
					Bindings: []bridge.Binding{
						{Page: "x", ListName: "Y", MigratedFingerprints: true},
					},
				},
			}}

			scanJob := NewKeepBridgeFingerprintMigrationScanJob(scanner, coordinator, recorder, loader)
			scanErr = scanJob.Execute()
		})

		It("should not error", func() {
			Expect(scanErr).ToNot(HaveOccurred())
		})

		// Hold migration_scan_job_finds_old_shape_bindings_only.
		It("should enqueue per-binding jobs only for un-migrated bindings", func() {
			// We cannot inspect the queue contents directly; instead,
			// drain by calling Execute on the recorded calls path.
			// JobQueueCoordinator runs jobs asynchronously, so wait
			// briefly for both expected calls.
			Eventually(func() int {
				recorder.mu.Lock()
				defer recorder.mu.Unlock()
				return len(recorder.calls)
			}, "2s", "10ms").Should(Equal(2))

			recorder.mu.Lock()
			defer recorder.mu.Unlock()
			seen := map[string]bool{}
			for _, c := range recorder.calls {
				seen[string(c.ProfileID)+"/"+c.Page+"/"+c.ListName] = true
			}
			Expect(seen).To(HaveKey("profile_test/shopping/Grocery"))
			Expect(seen).To(HaveKey("profile_test/shopping/Hardware"))
			Expect(seen).ToNot(HaveKey("profile_done/x/Y"))
		})
	})
})

// stubLogger is a do-nothing logger for the JobQueueCoordinator.
type stubLogger struct{}

func (stubLogger) Info(_ string, _ ...any)  {}
func (stubLogger) Warn(_ string, _ ...any)  {}
func (stubLogger) Error(_ string, _ ...any) {}
func (stubLogger) Debug(_ string, _ ...any) {}
func (stubLogger) Fatal(_ string, _ ...any) {}
func (stubLogger) Trace(_ string, _ ...any) {}

// --- per-binding job: rebaseline scenarios --------------------------------

var _ = Describe("KeepBridgeFingerprintMigrationJob — silent rebaseline", func() {
	const profileID = wikipage.PageIdentifier("profile_test")
	const page = "shopping"
	const listName = "Grocery"
	const noteID = "list-server-id"

	var (
		c     *bridge.Connector
		store *bridge.BindingStore
		keep  *fakeMigrationKeepClient
		chk   *fakeMigrationChecklist
	)

	Describe("when wiki and Keep already agree on every paired item", func() {
		BeforeEach(func() {
			c, store, keep, chk = freshConnectorForMigration(profileID, page, listName, noteID,
				map[string]bridge.ItemBinding{
					"uid-A": {ServerID: "srv-A"},
				},
			)
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: timestamppb.New(time.Now())},
			}
			keep.pullState = protocol.ChangesResponse{
				ToVersion: "v-after-pull",
				Nodes: []protocol.Node{
					keepNodeForMigration("srv-A", noteID, "Apples", false, "1000"),
				},
			}

			job := NewKeepBridgeFingerprintMigrationJob(c, profileID, page, listName)
			Expect(job.Execute()).To(Succeed())
		})

		It("should pull Keep exactly once", func() {
			Expect(keep.pullCount()).To(Equal(1))
		})

		It("should NOT call UpdateItemForSync (silent rebaseline)", func() {
			chk.mu.Lock()
			defer chk.mu.Unlock()
			Expect(chk.updateCalls).To(BeEmpty())
		})

		It("should populate synced_fp from the agreed content", func() {
			st, err := store.LoadState(profileID)
			Expect(err).ToNot(HaveOccurred())
			Expect(st.Bindings[0].ItemIDMap["uid-A"].SyncedText).To(Equal("Apples"))
			Expect(st.Bindings[0].ItemIDMap["uid-A"].SyncedSortValue).To(Equal("1000"))
		})

		It("should stamp MigratedFingerprints=true", func() {
			st, err := store.LoadState(profileID)
			Expect(err).ToNot(HaveOccurred())
			Expect(st.Bindings[0].MigratedFingerprints).To(BeTrue())
		})

		It("should clear KeepCursor so the next sync does a full pull", func() {
			// The first post-migration sync MUST be a full (non-
			// incremental) pull so the class-4 hard-delete pass can
			// observe Keep's complete current state. An incremental
			// pull on a freshly-migrated binding would expose paired
			// items whose serverIDs aren't in the delta to false
			// "Keep deleted this" classification. Source: post-deploy
			// mass-delete bug remediation.
			st, err := store.LoadState(profileID)
			Expect(err).ToNot(HaveOccurred())
			Expect(st.Bindings[0].KeepCursor).To(BeEmpty())
		})
	})
})

var _ = Describe("KeepBridgeFingerprintMigrationJob — Keep-wins divergence", func() {
	const profileID = wikipage.PageIdentifier("profile_test")
	const page = "shopping"
	const listName = "Grocery"
	const noteID = "list-server-id"

	var (
		c     *bridge.Connector
		store *bridge.BindingStore
		keep  *fakeMigrationKeepClient
		chk   *fakeMigrationChecklist
	)

	Describe("when wiki and Keep have different content for a paired item", func() {
		BeforeEach(func() {
			c, store, keep, chk = freshConnectorForMigration(profileID, page, listName, noteID,
				map[string]bridge.ItemBinding{
					"uid-A": {ServerID: "srv-A"},
				},
			)
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples (wiki version)", SortOrder: 1000, UpdatedAt: timestamppb.New(time.Now())},
			}
			keep.pullState = protocol.ChangesResponse{
				ToVersion: "v-after-pull",
				Nodes: []protocol.Node{
					keepNodeForMigration("srv-A", noteID, "Apples (Keep version)", false, "1000"),
				},
			}

			job := NewKeepBridgeFingerprintMigrationJob(c, profileID, page, listName)
			Expect(job.Execute()).To(Succeed())
		})

		It("should call UpdateItemForSync with Keep's content", func() {
			chk.mu.Lock()
			defer chk.mu.Unlock()
			Expect(chk.updateCalls).To(HaveLen(1))
			Expect(chk.updateCalls[0].UID).To(Equal("uid-A"))
			Expect(chk.updateCalls[0].Text).To(Equal("Apples (Keep version)"))
		})

		It("should populate synced_fp from Keep's content", func() {
			st, err := store.LoadState(profileID)
			Expect(err).ToNot(HaveOccurred())
			Expect(st.Bindings[0].ItemIDMap["uid-A"].SyncedText).To(Equal("Apples (Keep version)"))
		})

		It("should stamp MigratedFingerprints=true after applying Keep", func() {
			st, err := store.LoadState(profileID)
			Expect(err).ToNot(HaveOccurred())
			Expect(st.Bindings[0].MigratedFingerprints).To(BeTrue())
		})

		It("should pass the binding owner's email as ownerEmail to the mutator", func() {
			chk.mu.Lock()
			defer chk.mu.Unlock()
			Expect(chk.updateCalls[0].OwnerEmail).To(Equal("user@example.com"))
		})
	})
})

var _ = Describe("KeepBridgeFingerprintMigrationJob — drops entries Keep no longer has", func() {
	const profileID = wikipage.PageIdentifier("profile_test")
	const page = "shopping"
	const listName = "Grocery"
	const noteID = "list-server-id"

	var (
		c     *bridge.Connector
		store *bridge.BindingStore
		keep  *fakeMigrationKeepClient
		chk   *fakeMigrationChecklist
	)

	Describe("when an id_map entry's serverID is missing from the pull", func() {
		BeforeEach(func() {
			c, store, keep, chk = freshConnectorForMigration(profileID, page, listName, noteID,
				map[string]bridge.ItemBinding{
					"uid-A":   {ServerID: "srv-A"}, // present in pull
					"uid-OLD": {ServerID: "srv-OLD"}, // absent from pull (Keep deleted)
				},
			)
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: timestamppb.New(time.Now())},
				{Uid: "uid-OLD", Text: "OldItem", SortOrder: 2000, UpdatedAt: timestamppb.New(time.Now())},
			}
			keep.pullState = protocol.ChangesResponse{
				ToVersion: "v-after-pull",
				Nodes: []protocol.Node{
					keepNodeForMigration("srv-A", noteID, "Apples", false, "1000"),
				},
			}

			job := NewKeepBridgeFingerprintMigrationJob(c, profileID, page, listName)
			Expect(job.Execute()).To(Succeed())
		})

		It("should drop the unpaired uid from id_map", func() {
			st, err := store.LoadState(profileID)
			Expect(err).ToNot(HaveOccurred())
			Expect(st.Bindings[0].ItemIDMap).ToNot(HaveKey("uid-OLD"))
		})

		It("should keep the still-paired uid in id_map", func() {
			st, err := store.LoadState(profileID)
			Expect(err).ToNot(HaveOccurred())
			Expect(st.Bindings[0].ItemIDMap).To(HaveKey("uid-A"))
		})

		It("should NOT call UpdateItemForSync for the dropped entry", func() {
			chk.mu.Lock()
			defer chk.mu.Unlock()
			for _, call := range chk.updateCalls {
				Expect(call.UID).ToNot(Equal("uid-OLD"))
			}
		})
	})
})

var _ = Describe("KeepBridgeFingerprintMigrationJob — idempotent on already-migrated", func() {
	const profileID = wikipage.PageIdentifier("profile_test")
	const page = "shopping"
	const listName = "Grocery"
	const noteID = "list-server-id"

	Describe("when MigratedFingerprints is already true", func() {
		var (
			c     *bridge.Connector
			store *bridge.BindingStore
			keep  *fakeMigrationKeepClient
			chk   *fakeMigrationChecklist
		)

		BeforeEach(func() {
			c, store, keep, chk = freshConnectorForMigration(profileID, page, listName, noteID,
				map[string]bridge.ItemBinding{
					"uid-A": {ServerID: "srv-A", SyncedText: "Apples", SyncedSortValue: "1000"},
				},
			)
			// Flip the flag to true; second-tick re-run case.
			st, err := store.LoadState(profileID)
			Expect(err).ToNot(HaveOccurred())
			st.Bindings[0].MigratedFingerprints = true
			Expect(store.SaveState(profileID, st)).To(Succeed())

			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: timestamppb.New(time.Now())},
			}
			keep.pullState = protocol.ChangesResponse{ToVersion: "v"}

			job := NewKeepBridgeFingerprintMigrationJob(c, profileID, page, listName)
			Expect(job.Execute()).To(Succeed())
		})

		It("should NOT pull Keep (skipped at the pre-check)", func() {
			Expect(keep.pullCount()).To(Equal(0))
		})

		It("should NOT call UpdateItemForSync", func() {
			chk.mu.Lock()
			defer chk.mu.Unlock()
			Expect(chk.updateCalls).To(BeEmpty())
		})

		It("should leave the binding's synced_fp untouched", func() {
			st, err := store.LoadState(profileID)
			Expect(err).ToNot(HaveOccurred())
			Expect(st.Bindings[0].ItemIDMap["uid-A"].SyncedText).To(Equal("Apples"))
		})
	})
})

var _ = Describe("KeepBridgeFingerprintMigrationJob — failure leaves binding un-migrated", func() {
	const profileID = wikipage.PageIdentifier("profile_test")
	const page = "shopping"
	const listName = "Grocery"
	const noteID = "list-server-id"

	Describe("when the Keep pull returns an error", func() {
		var (
			c     *bridge.Connector
			store *bridge.BindingStore
			keep  *fakeMigrationKeepClient
			jobErr error
		)

		BeforeEach(func() {
			c, store, keep, _ = freshConnectorForMigration(profileID, page, listName, noteID,
				map[string]bridge.ItemBinding{
					"uid-A": {ServerID: "srv-A"},
				},
			)
			keep.pullError = errors.New("auth_revoked")

			job := NewKeepBridgeFingerprintMigrationJob(c, profileID, page, listName)
			jobErr = job.Execute()
		})

		It("should return the wrapped error", func() {
			Expect(jobErr).To(HaveOccurred())
			Expect(jobErr.Error()).To(ContainSubstring("auth_revoked"))
		})

		It("should leave MigratedFingerprints=false so the queue retries", func() {
			st, err := store.LoadState(profileID)
			Expect(err).ToNot(HaveOccurred())
			Expect(st.Bindings[0].MigratedFingerprints).To(BeFalse())
		})
	})
})

var _ = Describe("KeepBridgeFingerprintMigrationJob — profile mutex serialization", func() {
	const profileID = wikipage.PageIdentifier("profile_test")
	const page = "shopping"
	const listName = "Grocery"
	const noteID = "list-server-id"

	Describe("when a concurrent BindingStore.AddBinding waits for the migration write window", func() {
		// We test the mutex by holding the write window: the migration's
		// final SaveStateLocked happens inside WithProfileLock. While
		// the job is running, a concurrent AddBinding call must block
		// until the migration releases the lock.
		//
		// We cannot easily inject a "pause" inside the Connector, so
		// instead we drive the test by:
		//   1. running the migration successfully (acquires + releases
		//      the lock).
		//   2. immediately running an AddBinding for a different
		//      checklist on the same profile.
		//   3. asserting both completed and the new binding is present.
		// This exercises the full lock-acquire/release path and would
		// deadlock if WithProfileLock failed to release.
		var store *bridge.BindingStore

		BeforeEach(func() {
			c, st, _, chk := freshConnectorForMigration(profileID, page, listName, noteID,
				map[string]bridge.ItemBinding{
					"uid-A": {ServerID: "srv-A"},
				},
			)
			store = st

			fakeKeep := &fakeMigrationKeepClient{}
			fakeKeep.pullState = protocol.ChangesResponse{
				ToVersion: "v",
				Nodes: []protocol.Node{
					keepNodeForMigration("srv-A", noteID, "Apples", false, "1000"),
				},
			}
			c.SetClientBuilder(func(_ string) bridge.KeepClient { return fakeKeep })
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: timestamppb.New(time.Now())},
			}

			job := NewKeepBridgeFingerprintMigrationJob(c, profileID, page, listName)
			Expect(job.Execute()).To(Succeed())

			// Concurrent macro-style add: must succeed (lock was released).
			Expect(store.AddBinding(profileID, bridge.Binding{
				Page: "shopping", ListName: "AnotherList",
				KeepNoteID: "another-note-id",
				BoundAt:    time.Now().UTC(),
				MigratedFingerprints: true,
			})).To(Succeed())
		})

		It("should have both the migrated original binding and the concurrently-added one", func() {
			final, err := store.LoadState(profileID)
			Expect(err).ToNot(HaveOccurred())
			Expect(final.Bindings).To(HaveLen(2))
		})
	})
})

