//revive:disable:dot-imports
package bridge_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/keep/bridge"
	"github.com/brendanjerwin/simple_wiki/internal/keep/protocol"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// fakeKeepClient records every Changes() call. Pull responses come from
// pullState (set per-test); push responses default to an empty downSync
// with synthesized SUCCESS WriteResults for every pushed LIST_ITEM
// (matches the steady-state happy path so tests don't have to wire
// WriteResults explicitly when they don't care). Tests that need to
// exercise per-node failure handling override pushResponse, including
// pushResponse.WriteResults; the default-synth is only applied when
// pushResponse.ToVersion is empty.
type fakeKeepClient struct {
	mu             sync.Mutex
	pullState      protocol.ChangesResponse
	pushResponse   protocol.ChangesResponse
	pushedRequests []protocol.ChangesRequest
	pulledRequests []protocol.ChangesRequest
}

func (c *fakeKeepClient) Changes(_ context.Context, req protocol.ChangesRequest) (protocol.ChangesResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(req.Nodes) == 0 && len(req.Labels) == 0 {
		// pull
		c.pulledRequests = append(c.pulledRequests, req)
		return c.pullState, nil
	}
	c.pushedRequests = append(c.pushedRequests, req)
	if c.pushResponse.ToVersion == "" {
		// Default: every pushed node succeeds. The connector gates
		// synced_fp on per-node SUCCESS, so without this the existing
		// happy-path tests would all interpret default fakes as
		// failure.
		writeResults := make([]protocol.NodeWriteResult, 0, len(req.Nodes))
		for _, n := range req.Nodes {
			if n.Type != protocol.NodeTypeListItem {
				continue
			}
			writeResults = append(writeResults, protocol.NodeWriteResult{
				ID:     n.ID,
				Status: "SUCCESS",
			})
		}
		return protocol.ChangesResponse{
			ToVersion:    "v-after-push",
			WriteResults: writeResults,
		}, nil
	}
	return c.pushResponse, nil
}

func (*fakeKeepClient) CreateList(_ context.Context, _ string) (string, error) {
	return "", errors.New("CreateList not used in these tests")
}

func (*fakeKeepClient) CreateListWithItems(_ context.Context, _ string, _ []protocol.ListItemSpec) (protocol.CreateListResult, error) {
	return protocol.CreateListResult{}, errors.New("CreateListWithItems not used in these tests")
}

// lastPush returns the most recent push request body, or fails if none.
func (c *fakeKeepClient) lastPush() protocol.ChangesRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	Expect(c.pushedRequests).ToNot(BeEmpty(), "expected at least one push")
	return c.pushedRequests[len(c.pushedRequests)-1]
}

// pushCount returns how many pushes happened.
func (c *fakeKeepClient) pushCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pushedRequests)
}

// lastPull returns the most recent pull request body, or fails if none.
func (c *fakeKeepClient) lastPull() protocol.ChangesRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	Expect(c.pulledRequests).ToNot(BeEmpty(), "expected at least one pull")
	return c.pulledRequests[len(c.pulledRequests)-1]
}

// findPushedItem returns the LIST_ITEM in the latest push with the given
// serverID, or fails if not found.
func (c *fakeKeepClient) findPushedItem(serverID string) protocol.Node {
	push := c.lastPush()
	for _, n := range push.Nodes {
		if n.Type == protocol.NodeTypeListItem && n.ServerID == serverID {
			return n
		}
	}
	Fail(fmt.Sprintf("no LIST_ITEM with serverID=%s in last push", serverID))
	return protocol.Node{}
}

// fakeChecklist holds wiki-side state — an items slice that AddItemFor
// Sync / UpdateItemForSync / DeleteItemForSync mutate. Both bridge.
// ChecklistReader and bridge.ChecklistMutator are satisfied.
type fakeChecklist struct {
	mu       sync.Mutex
	items    []*apiv1.ChecklistItem
	addCalls []addRecord
	upsCalls []updateRecord
	delCalls []deleteRecord
	uidCount int
}

type addRecord struct {
	OwnerEmail        string
	Text, Description string
	Tags              []string
	Checked           bool
}
type updateRecord struct {
	OwnerEmail             string
	UID, Text, Description string
	Tags                   []string
	Checked                bool
}
type deleteRecord struct {
	OwnerEmail, UID string
}

func (c *fakeChecklist) ListItems(_ context.Context, _, _ string) (*apiv1.Checklist, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := &apiv1.Checklist{Items: make([]*apiv1.ChecklistItem, len(c.items))}
	for i, it := range c.items {
		cloned, ok := proto.Clone(it).(*apiv1.ChecklistItem)
		Expect(ok).To(BeTrue(), "proto.Clone returned wrong type")
		out.Items[i] = cloned
	}
	return out, nil
}

func (c *fakeChecklist) AddItemForSync(_ context.Context, _, _, ownerEmail, text string, checked bool, tags []string, description, sortValueHint string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addCalls = append(c.addCalls, addRecord{OwnerEmail: ownerEmail, Text: text, Description: description, Tags: tags, Checked: checked})
	c.uidCount++
	uid := fmt.Sprintf("test-uid-%d", c.uidCount)
	now := timestamppb.New(time.Now())
	var sortOrder int64
	if sortValueHint != "" {
		// Mirror the real mutator: parse the hint as the new SortOrder.
		if n, err := strconv.ParseInt(sortValueHint, 10, 64); err == nil {
			sortOrder = n
		}
	}
	item := &apiv1.ChecklistItem{
		Uid:       uid,
		Text:      text,
		Tags:      tags,
		Checked:   checked,
		SortOrder: sortOrder,
		UpdatedAt: now,
	}
	if description != "" {
		d := description
		item.Description = &d
	}
	c.items = append(c.items, item)
	return uid, nil
}

func (c *fakeChecklist) UpdateItemForSync(_ context.Context, _, _, ownerEmail, uid, text string, checked bool, tags []string, description string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.upsCalls = append(c.upsCalls, updateRecord{OwnerEmail: ownerEmail, UID: uid, Text: text, Description: description, Tags: tags, Checked: checked})
	for _, it := range c.items {
		if it.GetUid() == uid {
			it.Text = text
			it.Tags = tags
			it.Checked = checked
			it.UpdatedAt = timestamppb.New(time.Now())
			if description != "" {
				d := description
				it.Description = &d
			} else {
				it.Description = nil
			}
			return nil
		}
	}
	return fmt.Errorf("uid %s not found", uid)
}

func (c *fakeChecklist) DeleteItemForSync(_ context.Context, _, _, ownerEmail, uid string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.delCalls = append(c.delCalls, deleteRecord{OwnerEmail: ownerEmail, UID: uid})
	out := make([]*apiv1.ChecklistItem, 0, len(c.items))
	for _, it := range c.items {
		if it.GetUid() != uid {
			out = append(out, it)
		}
	}
	c.items = out
	return nil
}

// fakeSuppressor satisfies bridge.SyncSuppressor with no-op behavior.
type fakeSuppressor struct{}

func (fakeSuppressor) Suppress(_ wikipage.PageIdentifier, _, _ string)   {}
func (fakeSuppressor) Unsuppress(_ wikipage.PageIdentifier, _, _ string) {}

// freshConnector returns a Connector wired up with all fakes, plus a
// pre-populated binding for (page, listName) keyed off the given
// keepNoteID.
//
//revive:disable-next-line:max-public-structs,function-result-limit
func freshConnector(profileID wikipage.PageIdentifier, page, listName, keepNoteID string, idMap map[string]string) (*bridge.Connector, *fakeStore, *fakeKeepClient, *fakeChecklist) {
	store := newFakeStore()
	c := bridge.NewConnector(bridge.NewBindingStore(store), nil, fakeClock{})
	keep := &fakeKeepClient{}
	c.SetClientBuilder(func(_ string) bridge.KeepClient { return keep })
	c.SetAuthBuilder(func(_ string) bridge.AuthExchanger { return fakeAuth{} })
	chk := &fakeChecklist{}
	c.SetChecklistReader(chk)
	c.SetChecklistMutator(chk)
	c.SetSyncSuppressor(fakeSuppressor{})

	// Convert legacy flat idMap to the new structured shape. Tests that
	// need richer fingerprint state set ItemBinding values directly.
	itemBindings := make(map[string]bridge.ItemBinding, len(idMap))
	for uid, serverID := range idMap {
		itemBindings[uid] = bridge.ItemBinding{ServerID: serverID}
	}

	// Default test bindings are MigratedFingerprints=true so the sync
	// engine's pre-pull migration gate doesn't reject them. Tests that
	// pin legacy/un-migrated behavior override this on the saved state.
	state := bridge.ConnectorState{
		Email:       "test@example.com",
		MasterToken: "token",
		Bindings: []bridge.Binding{{
			Page: page, ListName: listName,
			KeepNoteID: keepNoteID, KeepNoteTitle: listName,
			BoundAt:              time.Now().UTC(),
			MigratedFingerprints: true,
			ItemIDMap:            itemBindings,
		}},
	}
	Expect(bridge.NewBindingStore(store).SaveState(profileID, state)).To(Succeed())
	return c, store, keep, chk
}

// seedSyncedFromKeep copies each pull node's content fingerprint into
// the corresponding ItemBinding's Synced{Text,Checked,SortValue}
// fields, keyed by serverID. Use to express the test's pre-tick
// synced baseline for scenarios where wiki edited and Keep stayed
// at the previously-synced state — so wiki_fp != synced_fp (wd) and
// keep_fp == synced_fp (¬kd) → "push wiki" route.
func seedSyncedFromKeep(profileID wikipage.PageIdentifier, page, listName string, store *fakeStore, nodes []protocol.Node) {
	GinkgoHelper()
	bs := bridge.NewBindingStore(store)
	state, err := bs.LoadState(profileID)
	Expect(err).NotTo(HaveOccurred())
	nodeByServerID := make(map[string]protocol.Node, len(nodes))
	for _, n := range nodes {
		if n.ServerID == "" {
			continue
		}
		nodeByServerID[n.ServerID] = n
	}
	for i, b := range state.Bindings {
		if b.Page != page || b.ListName != listName {
			continue
		}
		for uid, ib := range b.ItemIDMap {
			node, ok := nodeByServerID[ib.ServerID]
			if !ok {
				continue
			}
			ib.SyncedText = node.Text
			ib.SyncedChecked = node.Checked
			ib.SyncedSortValue = node.SortValue
			state.Bindings[i].ItemIDMap[uid] = ib
		}
	}
	Expect(bs.SaveState(profileID, state)).To(Succeed())
}

// seedSyncedFromWiki seeds each ItemBinding's synced_fp from the
// matching wiki item (by uid). Use for scenarios where Keep edited
// and wiki stayed at the previously-synced state — so keep_fp !=
// synced_fp (kd) and wiki_fp == synced_fp (¬wd) → "apply Keep" route.
func seedSyncedFromWiki(profileID wikipage.PageIdentifier, page, listName string, store *fakeStore, items []*apiv1.ChecklistItem) {
	GinkgoHelper()
	bs := bridge.NewBindingStore(store)
	state, err := bs.LoadState(profileID)
	Expect(err).NotTo(HaveOccurred())
	itemByUID := make(map[string]*apiv1.ChecklistItem, len(items))
	for _, it := range items {
		itemByUID[it.GetUid()] = it
	}
	for i, b := range state.Bindings {
		if b.Page != page || b.ListName != listName {
			continue
		}
		for uid, ib := range b.ItemIDMap {
			it, ok := itemByUID[uid]
			if !ok {
				continue
			}
			fp := bridge.FingerprintWiki(it)
			ib.SyncedText = fp.Text
			ib.SyncedChecked = fp.Checked
			ib.SyncedSortValue = fp.SortValue
			state.Bindings[i].ItemIDMap[uid] = ib
		}
	}
	Expect(bs.SaveState(profileID, state)).To(Succeed())
}

// Time anchors used across the matrix. Keeping these as named values
// avoids the lint nag about repeated time.Date(year, month, ...) calls
// and makes the relative ordering at a glance.
//
// Ordering: tStaleA < tKeepRecent < tKeepRecent2 < tWikiAnchor < tNow < tFuture.
var (
	tStaleA      = time.Date(2026, time.April, 26, 0, 0, 0, 0, time.UTC)         //nolint:gochecknoglobals
	tKeepRecent  = time.Date(2026, time.April, 28, 12, 0, 0, 0, time.UTC)        //nolint:gochecknoglobals
	tKeepRecent2 = time.Date(2026, time.April, 30, 12, 0, 0, 0, time.UTC)        //nolint:gochecknoglobals
	tWikiAnchor  = time.Date(2026, time.May, 1, 9, 0, 0, 0, time.UTC)            //nolint:gochecknoglobals
	tNow         = time.Date(2026, time.May, 1, 12, 0, 0, 0, time.UTC)           //nolint:gochecknoglobals
	tFuture      = time.Date(2026, time.May, 2, 12, 0, 0, 0, time.UTC)           //nolint:gochecknoglobals
	tEpochPlusMs = time.Date(1970, time.January, 1, 0, 0, 0, 1000000, time.UTC)  //nolint:gochecknoglobals
)

// fakeClock returns a fixed time.
type fakeClock struct{}

func (fakeClock) Now() time.Time { return tNow }

// fakeAuth bypasses bearer exchange.
type fakeAuth struct{}

func (fakeAuth) ExchangeOAuthTokenForMasterToken(_ context.Context, _, _ string) (string, error) {
	return "mt", nil
}
func (fakeAuth) ExchangeMasterTokenForBearer(_ context.Context, _, _ string) (string, error) {
	return "bearer", nil
}

// keepItem is a tiny builder for pull-state items. Sets Updated to a
// recent fixed time so freshness comparisons work; tests that need
// staler/newer Keep timestamps mutate the returned node.
func keepItem(serverID, clientID, parentServer, text string, checked bool, sortValue string) protocol.Node {
	return protocol.Node{
		Kind:     "notes#node",
		ID:       clientID,
		ServerID: serverID,
		ParentID: parentServer, ParentServerID: parentServer,
		Type:        protocol.NodeTypeListItem,
		Text:        text,
		Checked:     checked,
		SortValue:   sortValue,
		BaseVersion: "v-base-" + serverID,
		Timestamps: protocol.Timestamps{
			Updated: tKeepRecent,
		},
	}
}

// recentTime is "now minus a few hours" — used as wiki UpdatedAt for
// items that aren't supposed to look stale.
func recentTime() *timestamppb.Timestamp {
	return timestamppb.New(tWikiAnchor)
}

// staleTime is "two days ago" — used to make wiki state look older
// than Keep's edits.
func staleTime() *timestamppb.Timestamp {
	return timestamppb.New(tStaleA)
}

// futureTime is "tomorrow" — used to make wiki state look newer than
// Keep's edits (so the inbound apply gate skips).
func futureTime() *timestamppb.Timestamp {
	return timestamppb.New(tFuture)
}

var _ = Describe("Connector.SyncToKeep — interaction matrix", func() {
	const (
		profile  = wikipage.PageIdentifier("profile_test")
		page     = "shopping"
		listName = "Grocery"
		listSrv  = "list-server-id"
	)

	var (
		ctx context.Context
		c   *bridge.Connector
		kc  *fakeKeepClient
		chk *fakeChecklist
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	// W0 — no changes either side.
	Describe("W0 — no-op tick", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should not push anything when wiki and Keep agree", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// W1 — wiki added a new item with no id_map entry.
	Describe("W1 — wiki adds a new item", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
				{Uid: "uid-NEW", Text: "Bread", SortOrder: 2000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push exactly one node — the fresh item", func() {
			Expect(kc.pushCount()).To(Equal(1))
			Expect(kc.lastPush().Nodes).To(HaveLen(1))
			Expect(kc.lastPush().Nodes[0].Text).To(Equal("Bread"))
		})

		It("should push the fresh item without a server_id", func() {
			Expect(kc.lastPush().Nodes[0].ServerID).To(BeEmpty())
		})

		It("should push with parentServerId = list serverID", func() {
			Expect(kc.lastPush().Nodes[0].ParentServerID).To(Equal(listSrv))
		})
	})

	// W2 — wiki check toggled on an existing item.
	Describe("W2 — wiki toggles an item's checked state", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", Checked: true, SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			// Seed synced_fp from Keep: previous baseline was Keep's
			// current state; wiki has since edited (checked=true).
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push checked=true with the original client_id and serverID", func() {
			n := kc.findPushedItem("srv-A")
			Expect(n.Checked).To(BeTrue())
			Expect(n.ID).To(Equal("client-A"))
		})

		It("should carry the baseVersion captured from the pull", func() {
			Expect(kc.findPushedItem("srv-A").BaseVersion).To(Equal("v-base-srv-A"))
		})
	})

	// W3 — wiki text edited.
	Describe("W3 — wiki text edit", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Green Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			// Seed synced_fp from Keep: wiki has the edit (Green Apples);
			// the synced baseline is Keep's current Apples.
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push exactly one node", func() {
			Expect(kc.pushCount()).To(Equal(1))
			Expect(kc.lastPush().Nodes).To(HaveLen(1))
		})

		It("should push the new text", func() {
			Expect(kc.findPushedItem("srv-A").Text).To(Equal("Green Apples"))
		})

		It("should use the original client_id, not the server_id, for `id`", func() {
			n := kc.findPushedItem("srv-A")
			Expect(n.ID).To(Equal("client-A"))
			Expect(n.ServerID).To(Equal("srv-A"))
		})
	})

	// W4 — wiki tags edit (encoded into text).
	Describe("W4 — wiki adds tags to an item", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", Tags: []string{"fruit", "produce"}, SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			// Seed synced_fp from Keep: wiki has the edit (added tags);
			// Keep is at the previously-synced state.
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push the encoded text with #fruit and #produce appended", func() {
			Expect(kc.findPushedItem("srv-A").Text).To(Equal("Apples #fruit #produce"))
		})
	})

	// W5 — wiki description edit.
	Describe("W5 — wiki adds a description", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			desc := "the red kind"
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", Description: &desc, SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			// Seed synced_fp from Keep: wiki has the edit (added description);
			// Keep is at the previously-synced state.
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push text with the description after the separator", func() {
			Expect(kc.findPushedItem("srv-A").Text).To(Equal("Apples\n— the red kind"))
		})
	})

	// W6 — wiki sort order changed.
	Describe("W6 — wiki sort order changed", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 5000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			// Seed synced_fp from Keep: wiki has the edit (sort changed);
			// Keep is at the previously-synced state (SortValue=1000).
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push with the new SortValue", func() {
			Expect(kc.findPushedItem("srv-A").SortValue).To(Equal("5000"))
		})
	})

	// W7 — wiki removed an item.
	Describe("W7 — wiki delete (item removed from wiki)", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
				"uid-B": "srv-B",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
				// uid-B removed
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
					keepItem("srv-B", "client-B", listSrv, "Bread", false, "2000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push exactly one soft-delete node", func() {
			Expect(kc.pushCount()).To(Equal(1))
			Expect(kc.lastPush().Nodes).To(HaveLen(1))
		})

		It("should use Deleted, not Trashed, on the soft-delete", func() {
			n := kc.lastPush().Nodes[0]
			Expect(n.Timestamps.Deleted.IsZero()).To(BeFalse())
			Expect(n.Timestamps.Trashed.IsZero()).To(BeTrue())
		})

		It("should target the removed item's serverID", func() {
			Expect(kc.lastPush().Nodes[0].ServerID).To(Equal("srv-B"))
		})
	})

	// K1a — new item from Keep, no text-match in wiki.
	Describe("K1a — new item from Keep (no text-match in wiki) → AddItemForSync", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
					keepItem("srv-NEW", "client-NEW", listSrv, "Bread", false, "2000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should call AddItemForSync exactly once with the new text", func() {
			Expect(chk.addCalls).To(HaveLen(1))
			Expect(chk.addCalls[0].Text).To(Equal("Bread"))
		})
	})

	// K1b — new item from Keep with text-match in wiki — adopt.
	Describe("K1b — new item from Keep with text-match → adopt, do not duplicate", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "wiki-uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should NOT call AddItemForSync (adopt instead)", func() {
			Expect(chk.addCalls).To(BeEmpty())
		})
	})

	// K2 — Keep checkbox toggled, wiki stale.
	Describe("K2 — Keep toggles checked, wiki is older", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", Checked: false, SortOrder: 1000, UpdatedAt: staleTime()},
			}
			n := keepItem("srv-A", "client-A", listSrv, "Apples", true, "1000")
			n.Timestamps.UserEdited = tKeepRecent2
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should call UpdateItemForSync with checked=true", func() {
			Expect(chk.upsCalls).To(HaveLen(1))
			Expect(chk.upsCalls[0].UID).To(Equal("uid-A"))
			Expect(chk.upsCalls[0].Checked).To(BeTrue())
		})

		It("should attribute the apply to the binding owner's email, not a system actor", func() {
			Expect(chk.upsCalls[0].OwnerEmail).To(Equal("test@example.com"))
		})

		It("should NOT push back to Keep — content equality after apply", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// K3 — Keep text edited, wiki stale.
	Describe("K3 — Keep text edit (inbound apply)", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: staleTime()},
			}
			n := keepItem("srv-A", "client-A", listSrv, "Green Apples", false, "1000")
			n.Timestamps.UserEdited = tKeepRecent2
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should call UpdateItemForSync with the Keep text", func() {
			Expect(chk.upsCalls).To(HaveLen(1))
			Expect(chk.upsCalls[0].UID).To(Equal("uid-A"))
			Expect(chk.upsCalls[0].Text).To(Equal("Green Apples"))
		})
	})

	// K4 — Keep trashed.
	Describe("K4 — Keep marks an item trashed", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			n := keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000")
			n.Timestamps.Trashed = tKeepRecent2
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should call DeleteItemForSync for the trashed item", func() {
			Expect(chk.delCalls).To(HaveLen(1)); Expect(chk.delCalls[0].UID).To(Equal("uid-A"))
		})

		It("should NOT push the deletion back (already gone Keep-side)", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// K4-hard — Keep removed the item entirely (not in pull at all).
	// User did the swipe-to-delete or Keep app's "Remove" action,
	// which omits the item from subsequent pulls instead of flipping
	// a Trashed/Deleted timestamp. Without the class-4 pass, the
	// id_map orphans and the wiki item stays alive.
	Describe("K4-hard — Keep hard-deletes (item missing from pull)", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
				"uid-B": "srv-B",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: timestamppb.New(tStaleA)},
				{Uid: "uid-B", Text: "Bread", SortOrder: 2000, UpdatedAt: timestamppb.New(tStaleA)},
			}
			// Pull only returns srv-A; srv-B was hard-deleted in Keep.
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
				Truncated: false,
			}
			// Seed synced_fp from wiki: wiki has not edited since the
			// last successful sync. uid-A's synced matches both wiki
			// and Keep (no divergence). uid-B's synced matches wiki
			// (no wiki edit) — class-4 pass sees ¬wiki_diverged →
			// safe to apply Keep's hard-delete.
			seedSyncedFromWiki(profile, page, listName, store, chk.items)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should call DeleteItemForSync for the hard-deleted item", func() {
			Expect(chk.delCalls).To(HaveLen(1))
			Expect(chk.delCalls[0].UID).To(Equal("uid-B"))
		})

		It("should attribute the delete to the binding owner", func() {
			Expect(chk.delCalls[0].OwnerEmail).To(Equal("test@example.com"))
		})

		It("should not push anything (Keep already has its way)", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// K4-hard-wiki-only — wiki has an item that was NEVER pushed to
	// Keep (not in id_map). The hard-delete pass must NOT touch it.
	// Otherwise wiki-only items get silently dropped by every pull.
	Describe("K4-hard-wiki-only — wiki-only item not in id_map should never be deleted by Keep absence", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A", // only this one is paired
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: timestamppb.New(tStaleA)},
				{Uid: "uid-NEW", Text: "FreshLocal", SortOrder: 2000, UpdatedAt: timestamppb.New(tStaleA)}, // never pushed
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
				Truncated: false,
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should NOT delete the wiki-only item", func() {
			Expect(chk.delCalls).To(BeEmpty())
		})

		It("should push the wiki-only item to Keep (it's a fresh add)", func() {
			Expect(kc.pushCount()).To(Equal(1))
			Expect(kc.lastPush().Nodes[0].Text).To(Equal("FreshLocal"))
		})
	})

	// K4-hard-bogus-pull — pull returned NONE of id_map's serverIDs.
	// Suggests an auth/server hiccup; refuse to mass-delete.
	Describe("K4-hard-bogus-pull — pull missing all expected items, refuse to delete", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
				"uid-B": "srv-B",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: timestamppb.New(tStaleA)},
				{Uid: "uid-B", Text: "Bread", SortOrder: 2000, UpdatedAt: timestamppb.New(tStaleA)},
			}
			// Pull returns nothing for this binding.
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{},
				Truncated: false,
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should NOT delete anything (bogus pull is not deletion signal)", func() {
			Expect(chk.delCalls).To(BeEmpty())
		})
	})

	// K4-hard-truncated — same as K4-hard but pull was truncated.
	// Should NOT delete (truncation means pagination, not deletion).
	Describe("K4-hard-truncated — pull truncated, item missing is ambiguous", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
				"uid-B": "srv-B",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: timestamppb.New(tStaleA)},
				{Uid: "uid-B", Text: "Bread", SortOrder: 2000, UpdatedAt: timestamppb.New(tStaleA)},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
				Truncated: true, // <- key difference
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should NOT delete uid-B (truncation could explain its absence)", func() {
			Expect(chk.delCalls).To(BeEmpty())
		})
	})

	// keep_hard_delete_propagates_after_no_op_pull — bug 1 fix pinned.
	// User added "bananas" on Keep, deleted it before any sync ran;
	// in the meantime an earlier tick saw bananas alive and added
	// it to wiki + id_map (with synced_fp seeded from that tick).
	// Now this tick: bananas absent from pull, wiki idle (wiki_fp ==
	// synced_fp). Class-4 hard-delete pass must fire, dropping
	// bananas from wiki and from id_map. **No clock anywhere.**
	Describe("keep_hard_delete_propagates_after_no_op_pull", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-bananas": "srv-bananas",
				"uid-A":       "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000},
				{Uid: "uid-bananas", Text: "Bananas", SortOrder: 2000},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-after",
				Nodes: []protocol.Node{
					// Only Apples; bananas hard-deleted on Keep.
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			// Both wiki items match their last-synced state (wiki idle).
			seedSyncedFromWiki(profile, page, listName, store, chk.items)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should call DeleteItemForSync for the hard-deleted item", func() {
			Expect(chk.delCalls).To(HaveLen(1))
			Expect(chk.delCalls[0].UID).To(Equal("uid-bananas"))
		})

		It("should not push anything", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})

		It("should drop the hard-deleted item from id_map", func() {
			b, found, ferr := c.FindBinding(ctx, profile, page, listName)
			Expect(ferr).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(b.ItemIDMap).NotTo(HaveKey("uid-bananas"))
		})
	})

	// keep_add_then_delete_within_one_tick — pull never sees the
	// ephemeral item (Keep added then deleted before our pull).
	// Nothing arrives in pull.Nodes for it; nothing in id_map either.
	// No AddItemForSync, no churn.
	Describe("keep_add_then_delete_within_one_tick", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-after",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should not call AddItemForSync (ephemeral item never seen)", func() {
			Expect(chk.addCalls).To(BeEmpty())
		})

		It("should not call DeleteItemForSync", func() {
			Expect(chk.delCalls).To(BeEmpty())
		})

		It("should not push", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// keep_add_then_delete_across_two_ticks — tick 1 sees the alive
	// add; tick 2 sees the delete (item absent from pull). Adds then
	// removes from wiki without any clock comparisons.
	Describe("keep_add_then_delete_across_two_ticks", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			// Tick 1: wiki has Apples; Keep has Apples + new Bananas.
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-1",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
					keepItem("srv-bananas", "client-bananas", listSrv, "Bananas", false, "2000"),
				},
			}
			// Wiki Apples matches synced baseline (no wiki edit).
			seedSyncedFromWiki(profile, page, listName, store, chk.items)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should add Bananas on tick 1 and drop it on tick 2", func() {
			// Tick 1 added bananas via AddItemForSync.
			Expect(chk.addCalls).To(HaveLen(1))
			Expect(chk.addCalls[0].Text).To(Equal("Bananas"))

			// Verify id_map gained the entry with seeded synced_fp.
			b, found, ferr := c.FindBinding(ctx, profile, page, listName)
			Expect(ferr).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			var bananasUID string
			for uid, ib := range b.ItemIDMap {
				if ib.ServerID == "srv-bananas" {
					bananasUID = uid
					break
				}
			}
			Expect(bananasUID).NotTo(BeEmpty(), "bananas should be in id_map after tick 1")

			// Tick 2: Keep no longer has bananas. Wiki idle.
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-2",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())

			// DeleteItemForSync should have fired for bananas.
			Expect(chk.delCalls).To(HaveLen(1))
			Expect(chk.delCalls[0].UID).To(Equal(bananasUID))

			// Bananas dropped from id_map.
			b2, found2, ferr2 := c.FindBinding(ctx, profile, page, listName)
			Expect(ferr2).ToNot(HaveOccurred())
			Expect(found2).To(BeTrue())
			Expect(b2.ItemIDMap).NotTo(HaveKey(bananasUID))

			// Tick 2 issued no push.
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// wiki_edit_concurrent_with_keep_idle_does_not_block_keep_hard_delete_of_other_item
	// Item A: wiki-edited (uncommitted). Item B: Keep hard-deleted.
	// The two operations are independent — A pushed, B applied, neither
	// blocks the other.
	Describe("wiki_edit_concurrent_with_keep_idle_does_not_block_keep_hard_delete_of_other_item", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
				"uid-B": "srv-B",
			})
			chk.items = []*apiv1.ChecklistItem{
				// uid-A: wiki edited (text changed).
				{Uid: "uid-A", Text: "Apples GREEN", SortOrder: 1000},
				// uid-B: wiki idle.
				{Uid: "uid-B", Text: "Bread", SortOrder: 2000},
			}
			// Pull contains only uid-A; uid-B hard-deleted on Keep.
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-after",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			// Synced baseline: pre-edit Apples for uid-A; current
			// Bread for uid-B (wiki idle).
			seedSyncedFromKeep(profile, page, listName, store, []protocol.Node{
				keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				keepItem("srv-B", "client-B", listSrv, "Bread", false, "2000"),
			})
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should DeleteItemForSync uid-B (Keep hard-delete)", func() {
			var foundB bool
			for _, d := range chk.delCalls {
				if d.UID == "uid-B" {
					foundB = true
				}
			}
			Expect(foundB).To(BeTrue(), "uid-B should be deleted")
		})

		It("should push uid-A's wiki edit", func() {
			Expect(kc.pushCount()).To(Equal(1))
			Expect(kc.findPushedItem("srv-A").Text).To(Equal("Apples GREEN"))
		})

		It("should not apply Keep to uid-A (wiki diverged, Keep at synced)", func() {
			for _, u := range chk.upsCalls {
				Expect(u.UID).NotTo(Equal("uid-A"), "uid-A should not be apply-from-Keep")
			}
		})
	})

	// K5 — Keep deleted (Deleted timestamp non-zero).
	Describe("K5 — Keep marks an item deleted", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			n := keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000")
			n.Timestamps.Deleted = tKeepRecent2
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should call DeleteItemForSync for the deleted item", func() {
			Expect(chk.delCalls).To(HaveLen(1)); Expect(chk.delCalls[0].UID).To(Equal("uid-A"))
		})
	})

	// K6 — Keep returns epoch-sentinel `Updated` only, but real `UserEdited`.
	Describe("K6 — Keep `Updated` is epoch sentinel but `UserEdited` is real", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", Checked: false, SortOrder: 1000, UpdatedAt: staleTime()},
			}
			// Updated stuck at epoch+ms (Keep's stale sentinel for items
			// it created server-side); UserEdited carries the real edit time.
			n := keepItem("srv-A", "client-A", listSrv, "Apples", true, "1000")
			n.Timestamps.Updated = tEpochPlusMs // 1970-01-01T00:00:00.001Z
			n.Timestamps.UserEdited = tKeepRecent2
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should still apply the Keep edit because UserEdited is fresher than wiki", func() {
			Expect(chk.upsCalls).To(HaveLen(1))
			Expect(chk.upsCalls[0].Checked).To(BeTrue())
		})
	})

	// K7 — Keep returns exact-zero on both Updated and UserEdited.
	Describe("K7 — Keep `Updated` and `UserEdited` are both zero", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			n := keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000")
			n.Timestamps = protocol.Timestamps{} // both zero
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should not apply (no Keep edit signal)", func() {
			Expect(chk.upsCalls).To(BeEmpty())
		})

		It("should not push (content equality)", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// B1 — both edited; Keep newer.
	Describe("B1 — both edited, Keep newer", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "wiki edit", SortOrder: 1000, UpdatedAt: staleTime()},
			}
			n := keepItem("srv-A", "client-A", listSrv, "keep edit", false, "1000")
			n.Timestamps.UserEdited = tKeepRecent2
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should apply Keep's value to wiki", func() {
			Expect(chk.upsCalls[0].Text).To(Equal("keep edit"))
		})

		It("should not push (post-apply content equality)", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// B2 — wiki edited, Keep stayed at synced baseline. Under the
	// fingerprint divergence rule this is no longer a "both edited"
	// scenario — synced_fp = keep_fp expresses "Keep is unchanged
	// since last sync." The plan deferred B2 (wiki-wins config) to
	// future work; the canonical form here is just "wiki edit pushes."
	Describe("B2 — wiki edit while Keep at synced baseline", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "wiki edit", SortOrder: 1000, UpdatedAt: futureTime()},
			}
			n := keepItem("srv-A", "client-A", listSrv, "keep edit", false, "1000")
			n.Timestamps.UserEdited = tKeepRecent2
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			// Seed synced_fp = keep_fp: synced baseline matches Keep's
			// current state, so kd=false. Only wiki diverged.
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should not call UpdateItemForSync (¬keep_diverged → no apply)", func() {
			Expect(chk.upsCalls).To(BeEmpty())
		})

		It("should push wiki's text to Keep", func() {
			Expect(kc.findPushedItem("srv-A").Text).To(Equal("wiki edit"))
		})
	})

	// S1 — id_map points at a Keep item that's already gone.
	Describe("S1 — stale id_map entry, Keep already deleted item", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A":     "srv-A",
				"uid-stale": "srv-already-gone",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			// pull DOES NOT contain srv-already-gone
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push nothing — wiki agrees on srv-A, srv-already-gone is gone-gone", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// S2 — empty id_map but matching-text items on both sides.
	Describe("S2 — empty id_map, adopt by text", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "wiki-uid-1", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
				{Uid: "wiki-uid-2", Text: "Bread", SortOrder: 2000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
					keepItem("srv-B", "client-B", listSrv, "Bread", false, "2000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should NOT call AddItemForSync (matched both via text)", func() {
			Expect(chk.addCalls).To(BeEmpty())
		})

		It("should not push (post-adopt content equality)", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// K2-realworld — the user's reported bug. Wiki text edit synced
	// recently (so wiki.UpdatedAt is fresh — minutes ago), then user
	// checks an item via Keep web UI. Keep's per-item Updated bumps
	// to "now"; but a checked-only mutation might not bump UserEdited
	// on Keep's server. Verifying that latestKeepTimestamp picks the
	// fresher of (Updated, UserEdited) handles this correctly.
	Describe("K2-realworld — recent wiki edit + later Keep check toggle", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			// TODO(task #76): rewrite this test around content fingerprints
			// once the divergence rule is implemented. For now, the
			// stub-test setup lets the sync run; the assertions below
			// will need a fingerprint-baseline rebaseline when task #76
			// lands.
			thirtyMinAgo := timestamppb.New(tNow.Add(-30 * time.Minute))
			_ = store

			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", Checked: false, SortOrder: 1000, UpdatedAt: thirtyMinAgo},
			}
			// User toggled check in Keep web UI. Keep's API returns
			// epoch-sentinel timestamps for LIST_ITEM nodes regardless
			// (verified via cmd/keep-debug). The apply must fire
			// because wiki UpdatedAt (30min ago) <= LastPushedAt
			// (25min ago), NOT because Keep timestamps look fresh.
			n := keepItem("srv-A", "client-A", listSrv, "Apples", true, "1000")
			n.Timestamps.Updated = tEpochPlusMs
			n.Timestamps.UserEdited = time.Time{}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should apply the Keep check to wiki even when Keep timestamps are epoch sentinels", func() {
			Expect(chk.upsCalls).To(HaveLen(1))
			Expect(chk.upsCalls[0].Checked).To(BeTrue())
		})
	})

	// E1 — multi-item update where every item is byte-identical.
	Describe("E1 — multi-item byte-identical content skip", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
				"uid-B": "srv-B",
				"uid-C": "srv-C",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
				{Uid: "uid-B", Text: "Bread", SortOrder: 2000, UpdatedAt: recentTime()},
				{Uid: "uid-C", Text: "Cheese", SortOrder: 3000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
					keepItem("srv-B", "client-B", listSrv, "Bread", false, "2000"),
					keepItem("srv-C", "client-C", listSrv, "Cheese", false, "3000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should not push anything — every wiki item matches Keep byte-for-byte", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// C-05 — wiki absent, Keep alive, id_map stale (points at *different*
	// serverID than the alive Keep item carries). Rare: maybe the wiki
	// item was removed and Keep also rotated the serverID via re-create.
	Describe("C-05 — wiki absent, Keep alive, id_map stale", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-old": "srv-gone",
			})
			chk.items = []*apiv1.ChecklistItem{}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-fresh", "client-fresh", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should add the fresh Keep item to wiki", func() {
			Expect(chk.addCalls).To(HaveLen(1))
			Expect(chk.addCalls[0].Text).To(Equal("Apples"))
		})

		It("should not push (Keep already has it; wiki had nothing real)", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// C-07 — wiki absent, Keep trashed, id_map correct. Wiki removed
	// the item; Keep marked it trashed. Drop the id_map; do NOT push
	// a delete (already gone Keep-side).
	Describe("C-07 — wiki absent, Keep trashed, id_map correct", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{}
			n := keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000")
			n.Timestamps.Trashed = tKeepRecent2
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should not push (Keep already trashed it)", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})

		// The inbound apply class-3 branch calls DeleteItemForSync via
		// id_map without first checking whether the wiki has the uid.
		// The real mutator's DeleteItem returns nil for missing uids
		// (silent no-op), so this is harmless. The id_map gets cleaned
		// up either way.
		It("should call DeleteItemForSync via id_map (mutator silently no-ops on missing uid)", func() {
			Expect(chk.delCalls).To(HaveLen(1)); Expect(chk.delCalls[0].UID).To(Equal("uid-A"))
		})
	})

	// C-10 — wiki absent, Keep deleted, id_map correct. Same as C-07
	// but Deleted instead of Trashed.
	Describe("C-10 — wiki absent, Keep deleted, id_map correct", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{}
			n := keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000")
			n.Timestamps.Deleted = tKeepRecent2
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should not push", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// C-13 — wiki has fresh item, id_map points at gone Keep serverID.
	// Edge: if a wiki item has a stale id_map mapping (say, Keep had
	// the item, then phone-deleted it, but the wiki item still has
	// uid → that-now-gone serverID in its map). The push diff loop
	// sees serverID set, but Keep doesn't have it. Should drop the
	// stale entry and re-push as fresh.
	//
	// Current behavior: the push diff treats it as an existing item
	// (serverID set), tries to update, Keep returns "no such item".
	// The cleaner fix is to detect "id_map points at gone serverID"
	// and re-push as fresh; but the current code path produces a
	// stale push that fails. This test documents the existing gap.
	Describe("C-13 — wiki present, Keep absent for that serverID, id_map stale", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-gone",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{}, // Keep doesn't have srv-gone
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		// Current behavior: the diff loop sees serverID set and pushes
		// an update. Future cleanup: detect gone serverID in id_map
		// and re-push as fresh OR drop and add. For now pin behavior.
		It("should attempt one push for the orphaned serverID", func() {
			Expect(kc.pushCount()).To(Equal(1))
		})
	})

	// C-16 — wiki present, Keep alive (under correct serverID), id_map
	// stale (pointing at a different/gone serverID). Adoption-by-text
	// kicks in: the alive Keep item gets adopted by the wiki uid,
	// updating id_map.
	Describe("C-16 — wiki present, Keep alive, id_map stale", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-stale",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			// Keep has the item under a DIFFERENT serverID (post-recreate).
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A-fresh", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should not duplicate via AddItemForSync", func() {
			Expect(chk.addCalls).To(BeEmpty())
		})
	})

	// C-17 — wiki present, Keep trashed, id_map none. Unusual: Keep
	// returns a trashed item we don't know about. Bridge ignores
	// (the rev-map lookup fails → not in any switch arm).
	Describe("C-17 — wiki present, Keep trashed, id_map none", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			n := keepItem("srv-trash", "client-trash", listSrv, "Apples", false, "1000")
			n.Timestamps.Trashed = tKeepRecent2
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should not delete the wiki item (no id_map link)", func() {
			Expect(chk.delCalls).To(BeEmpty())
		})

		It("should push wiki item as fresh (id_map empty for this uid)", func() {
			// Push happens because wiki has uid-A with no serverID in id_map.
			Expect(kc.pushCount()).To(Equal(1))
		})
	})

	// C-19 — wiki present, Keep trashed, id_map correct. Same as K4;
	// already covered. This entry confirms the cell.
	// (skipped — covered by K4)

	// C-20 — wiki present, Keep deleted, id_map none. Symmetric to C-17.
	Describe("C-20 — wiki present, Keep deleted, id_map none", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			n := keepItem("srv-del", "client-del", listSrv, "Apples", false, "1000")
			n.Timestamps.Deleted = tKeepRecent2
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should not delete wiki item (no id_map link)", func() {
			Expect(chk.delCalls).To(BeEmpty())
		})
	})

	// 15e — wiki edited, Keep stayed at synced baseline. Under the
	// fingerprint divergence rule, the synced baseline determines
	// the direction; timestamps are no longer consulted.
	Describe("15e — wiki edit only, Keep at synced baseline", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			equalTime := tKeepRecent2
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "wiki", SortOrder: 1000, UpdatedAt: timestamppb.New(equalTime)},
			}
			n := keepItem("srv-A", "client-A", listSrv, "keep", false, "1000")
			n.Timestamps.Updated = equalTime
			n.Timestamps.UserEdited = equalTime
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should NOT apply Keep's value (¬keep_diverged)", func() {
			Expect(chk.upsCalls).To(BeEmpty())
		})

		It("should push wiki's value to Keep", func() {
			Expect(kc.pushCount()).To(Equal(1))
		})
	})

	// 15g — wiki UpdatedAt is nil. Should ALWAYS apply Keep's value
	// (no wiki signal → defer to Keep).
	Describe("15g — wiki UpdatedAt is nil, Keep has edit", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "wiki", SortOrder: 1000, UpdatedAt: nil}, // nil!
			}
			n := keepItem("srv-A", "client-A", listSrv, "keep", true, "1000")
			n.Timestamps.UserEdited = tKeepRecent2
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{n},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should apply Keep to wiki (no wiki UpdatedAt = pull always wins)", func() {
			Expect(chk.upsCalls).To(HaveLen(1))
			Expect(chk.upsCalls[0].Checked).To(BeTrue())
		})
	})

	// W2+W3 combined — text AND checked toggled in wiki.
	Describe("combined — wiki edits both text and checked at once", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Green Apples", Checked: true, SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push exactly one node carrying both changes", func() {
			Expect(kc.pushCount()).To(Equal(1))
			n := kc.findPushedItem("srv-A")
			Expect(n.Text).To(Equal("Green Apples"))
			Expect(n.Checked).To(BeTrue())
		})
	})

	// Multi-item mixed — some items match, some differ. Push only the diffs.
	Describe("multi-item mixed — push only items that actually differ", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
				"uid-B": "srv-B",
				"uid-C": "srv-C",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},     // same
				{Uid: "uid-B", Text: "Bread NEW", SortOrder: 2000, UpdatedAt: recentTime()},  // text diff
				{Uid: "uid-C", Text: "Cheese", Checked: true, SortOrder: 3000, UpdatedAt: recentTime()}, // checked diff
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
					keepItem("srv-B", "client-B", listSrv, "Bread", false, "2000"),
					keepItem("srv-C", "client-C", listSrv, "Cheese", false, "3000"),
				},
			}
			// Seed synced_fp from Keep so per-item kd=false and only
			// wiki-edited items diverge.
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push exactly two nodes (skipping the byte-identical one)", func() {
			Expect(kc.pushCount()).To(Equal(1))
			Expect(kc.lastPush().Nodes).To(HaveLen(2))
		})

		It("should push uid-B and uid-C, not uid-A", func() {
			pushed := kc.lastPush().Nodes
			ids := []string{pushed[0].ServerID, pushed[1].ServerID}
			Expect(ids).To(ConsistOf("srv-B", "srv-C"))
		})
	})

	// L1 — page has tags, none exist as Keep labels yet. Push label
	// CRUD entries plus a LIST node referencing the new label IDs.
	Describe("L1 — page tags new to Keep, push label CRUD + LIST node", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			// Seed page frontmatter with tags.
			pageID := wikipage.PageIdentifier(page)
			fm := wikipage.FrontMatter{
				"tags": []any{"household", "groceries"},
			}
			Expect(store.WriteFrontMatter(pageID, fm)).To(Succeed())

			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: timestamppb.New(tStaleA)},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
				Labels: []protocol.LabelEntry{}, // Keep has no labels yet
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push two new label CRUD entries", func() {
			Expect(kc.pushCount()).To(Equal(1))
			Expect(kc.lastPush().Labels).To(HaveLen(2))
		})

		It("should push one for 'household' and one for 'groceries'", func() {
			labelNames := []string{kc.lastPush().Labels[0].Name, kc.lastPush().Labels[1].Name}
			Expect(labelNames).To(ConsistOf("household", "groceries"))
		})

		It("should also push a LIST node referencing the new label IDs", func() {
			var listNode *protocol.Node
			for i, n := range kc.lastPush().Nodes {
				if n.Type == protocol.NodeTypeList {
					listNode = &kc.lastPush().Nodes[i]
					break
				}
			}
			Expect(listNode).NotTo(BeNil(), "expected a LIST node in push")
			Expect(listNode.LabelIDs).To(HaveLen(2))
		})
	})

	// L1b — page uses inline #hashtag content syntax (no frontmatter
	// tags array). Bridge should still extract them and push as labels.
	// This was the "labels not actually working" bug: most wiki pages
	// use #tag content syntax, and the old code only read frontmatter.
	Describe("L1b — page uses inline #hashtag content, push as labels", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			pageID := wikipage.PageIdentifier(page)
			// No frontmatter tags. Just inline content with hashtags.
			Expect(store.WriteMarkdown(pageID, wikipage.Markdown("#household #weekly\n\nshopping list lives here"))).To(Succeed())

			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: timestamppb.New(tStaleA)},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
				Labels: []protocol.LabelEntry{},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push two label CRUD entries for the inline hashtags", func() {
			Expect(kc.lastPush().Labels).To(HaveLen(2))
		})

		It("should push 'household' and 'weekly' (in first-occurrence order)", func() {
			labelNames := []string{kc.lastPush().Labels[0].Name, kc.lastPush().Labels[1].Name}
			Expect(labelNames).To(ConsistOf("household", "weekly"))
		})
	})

	// L2 — page tag matches existing Keep label by name; reuse it.
	Describe("L2 — page tag has existing Keep label, reuse mainID", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			pageID := wikipage.PageIdentifier(page)
			fm := wikipage.FrontMatter{
				"tags": []any{"household"},
			}
			Expect(store.WriteFrontMatter(pageID, fm)).To(Succeed())

			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: timestamppb.New(tStaleA)},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
				Labels: []protocol.LabelEntry{
					{MainID: "existing-label-id", Name: "household"},
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should NOT push new label CRUD entries", func() {
			Expect(kc.lastPush().Labels).To(BeEmpty())
		})

		It("should push a LIST node referencing the existing label ID", func() {
			var listNode *protocol.Node
			for i, n := range kc.lastPush().Nodes {
				if n.Type == protocol.NodeTypeList {
					listNode = &kc.lastPush().Nodes[i]
					break
				}
			}
			Expect(listNode).NotTo(BeNil())
			Expect(listNode.LabelIDs).To(ConsistOf("existing-label-id"))
		})
	})

	// E2 — fresh bind (KeepNoteID is empty).
	Describe("E2 — fresh bind, no KeepNoteID yet", func() {
		var (
			create *fakeKeepClient
			err    error
		)

		BeforeEach(func() {
			c, _, create, chk = freshConnector(profile, page, listName, "", map[string]string{})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			err = c.SyncToKeep(ctx, profile, page, listName)
		})

		// This branch goes through CreateListWithItems which our fake
		// rejects — so we expect the error path. The point is that
		// SyncToKeep DELEGATES to bootstrapKeepListForBinding when
		// KeepNoteID is empty and DOESN'T attempt a push/pull on the
		// missing list.
		It("should attempt the bundled CreateListWithItems path (delegating to bootstrap)", func() {
			Expect(err).To(MatchError(ContainSubstring("bootstrap create-list")))
			Expect(create.pushCount()).To(Equal(0))
		})
	})

	// cursor_passed_as_target_version_on_pull — task #75. The pull request
	// must carry the binding's KeepCursor as TargetVersion so Keep returns
	// only the delta since the last successful sync.
	Describe("cursor_passed_as_target_version_on_pull", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			// Pre-populate the binding's KeepCursor.
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			st.Bindings[0].KeepCursor = "v-cursor-from-last-sync"
			Expect(bs.SaveState(profile, st)).To(Succeed())

			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-after-pull",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should send the binding's KeepCursor as TargetVersion on the pull", func() {
			Expect(kc.lastPull().TargetVersion).To(Equal("v-cursor-from-last-sync"))
		})
	})

	// cursor_advances_after_successful_pull — task #75. After a successful
	// non-truncated pull (and no push), the binding's KeepCursor must
	// advance to pull.ToVersion.
	Describe("cursor_advances_after_successful_pull", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			st.Bindings[0].KeepCursor = "v-1"
			Expect(bs.SaveState(profile, st)).To(Succeed())

			// Wiki and Keep agree → no push happens, so only the pull
			// should advance the cursor.
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-2",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should advance KeepCursor to pull.ToVersion", func() {
			b, found, ferr := c.FindBinding(ctx, profile, page, listName)
			Expect(ferr).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(b.KeepCursor).To(Equal("v-2"))
		})

		It("should not push (wiki and Keep agree)", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// cursor_advances_after_successful_push — task #75. The push response's
	// ToVersion is preferred over the pull's because the push is the more
	// recent server state.
	Describe("cursor_advances_after_successful_push", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			st.Bindings[0].KeepCursor = "v-1"
			Expect(bs.SaveState(profile, st)).To(Succeed())

			// Wiki text differs → push happens.
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Green Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-2",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			kc.pushResponse = protocol.ChangesResponse{
				ToVersion: "v-3",
			}
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should advance KeepCursor to push.ToVersion (newer than pull's)", func() {
			b, found, ferr := c.FindBinding(ctx, profile, page, listName)
			Expect(ferr).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(b.KeepCursor).To(Equal("v-3"))
		})

		It("should have pushed once", func() {
			Expect(kc.pushCount()).To(Equal(1))
		})
	})

	// cursor_does_not_advance_when_pull_is_truncated — task #75. A truncated
	// pull leaves the cursor stale so the next tick re-pulls from the same
	// point. Without this, items missed by the truncation would never be
	// fetched.
	Describe("cursor_does_not_advance_when_pull_is_truncated", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			st.Bindings[0].KeepCursor = "v-1"
			Expect(bs.SaveState(profile, st)).To(Succeed())

			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-9",
				Truncated: true,
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should NOT advance KeepCursor (truncation means missing data)", func() {
			b, found, ferr := c.FindBinding(ctx, profile, page, listName)
			Expect(ferr).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(b.KeepCursor).To(Equal("v-1"))
		})
	})

	// cursor_advances_on_empty_incremental_pull — task #75. An empty
	// incremental pull (Keep returned no nodes) still advances the cursor;
	// otherwise we'd re-pull the same delta forever.
	Describe("cursor_advances_on_empty_incremental_pull", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{})
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			st.Bindings[0].KeepCursor = "v-1"
			Expect(bs.SaveState(profile, st)).To(Succeed())

			// No wiki items, no Keep items, empty Nodes slice in pull.
			chk.items = []*apiv1.ChecklistItem{}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-2",
				Nodes:     []protocol.Node{},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should advance KeepCursor even when no nodes changed", func() {
			b, found, ferr := c.FindBinding(ctx, profile, page, listName)
			Expect(ferr).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(b.KeepCursor).To(Equal("v-2"))
		})
	})

	// truncation_streak_increments_on_truncated_pull — task #79. Each
	// truncated pull bumps Binding.TruncatedTickStreak by one.
	Describe("truncation_streak_increments_on_truncated_pull", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-trunc",
				Truncated: true,
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			// First truncated pull: 0 → 1.
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should bump TruncatedTickStreak from 0 to 1 after first truncated pull", func() {
			b, found, ferr := c.FindBinding(ctx, profile, page, listName)
			Expect(ferr).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(b.TruncatedTickStreak).To(Equal(1))
		})

		Describe("after a second truncated pull", func() {
			BeforeEach(func() {
				// Pull stays truncated; tick again. 1 → 2.
				Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
			})

			It("should bump TruncatedTickStreak from 1 to 2", func() {
				b, found, ferr := c.FindBinding(ctx, profile, page, listName)
				Expect(ferr).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(b.TruncatedTickStreak).To(Equal(2))
			})
		})
	})

	// truncation_streak_resets_on_non_truncated_pull — task #79. Any
	// non-truncated pull clears Binding.TruncatedTickStreak back to 0,
	// even if it was previously elevated.
	Describe("truncation_streak_resets_on_non_truncated_pull", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			// Pre-set the streak to 3 so we can observe the reset.
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			st.Bindings[0].TruncatedTickStreak = 3
			Expect(bs.SaveState(profile, st)).To(Succeed())

			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-clean",
				Truncated: false,
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should reset TruncatedTickStreak to 0 on a non-truncated pull", func() {
			b, found, ferr := c.FindBinding(ctx, profile, page, listName)
			Expect(ferr).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(b.TruncatedTickStreak).To(Equal(0))
		})
	})

	// chronic_truncation_with_progress_does_not_force_resync — task #79.
	// Streak ≥ threshold but progress is being made (cursor advances OR
	// synced_fp updates) → escape hatch must NOT fire. This is the
	// large-account legitimate-pagination case.
	Describe("chronic_truncation_with_progress_does_not_force_resync", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			// Pre-existing streak just under threshold; a wiki edit will
			// push and advance synced_fp this tick (progress observed).
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			st.Bindings[0].KeepCursor = "v-prior"
			st.Bindings[0].TruncatedTickStreak = truncationResyncThresholdInTests()
			Expect(bs.SaveState(profile, st)).To(Succeed())

			// Wiki text differs from Keep → push happens (synced_fp advances).
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Green Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			seedSyncedFromKeep(profile, page, listName, store, []protocol.Node{
				keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
			})
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-prior", // cursor doesn't advance (truncated)
				Truncated: true,
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should NOT drop KeepCursor (progress was made via synced_fp advance)", func() {
			b, found, ferr := c.FindBinding(ctx, profile, page, listName)
			Expect(ferr).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(b.KeepCursor).ToNot(BeEmpty())
		})

		It("should still increment the streak (truncated tick) but not reset it via the escape hatch", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.TruncatedTickStreak).To(BeNumerically(">=", truncationResyncThresholdInTests()+1))
		})
	})

	// chronic_truncation_without_progress_forces_full_resync — task #79.
	// Streak ≥ threshold AND no progress (cursor not advancing AND no
	// synced_fp updates) → drop cursor, reset streak, force full resync
	// next tick.
	Describe("chronic_truncation_without_progress_forces_full_resync", func() {
		var (
			store *fakeStore
			dbg   *fakeInfoLogger
		)

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			dbg = &fakeInfoLogger{}
			c.SetDebugLogger(dbg)

			// Pre-set streak to one below threshold; this tick will be
			// the threshold-crossing one.
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			st.Bindings[0].KeepCursor = "v-stuck"
			st.Bindings[0].TruncatedTickStreak = truncationResyncThresholdInTests() - 1
			Expect(bs.SaveState(profile, st)).To(Succeed())

			// Wiki and Keep agree, no synced_fp advance, no inbound mutation.
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			seedSyncedFromWiki(profile, page, listName, store, chk.items)
			// Truncated pull whose ToVersion equals the prior cursor:
			// cursor cannot advance, no items to apply.
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v-stuck",
				Truncated: true,
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should drop KeepCursor to empty (forces full resync next tick)", func() {
			b, found, ferr := c.FindBinding(ctx, profile, page, listName)
			Expect(ferr).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(b.KeepCursor).To(BeEmpty())
		})

		It("should reset TruncatedTickStreak to 0 after triggering the escape hatch", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.TruncatedTickStreak).To(Equal(0))
		})

		It("should log an INFO-level message via debug.Info about the forced resync", func() {
			Expect(dbg.containsSubstring("truncation")).To(BeTrue(),
				"expected at least one Info log mentioning truncation; got: %v", dbg.formats)
		})
	})

	// outbound_skips_items_at_synced_baseline — task #77. The outbound
	// diff loop's push gate is now fingerprint divergence, not byte-
	// equality against Keep. When wiki_fp == synced_fp (wiki has not
	// been edited since the last successful sync), the item is
	// skipped — even if Keep's content happens to differ would only
	// matter via the inbound apply pass.
	Describe("outbound_skips_items_at_synced_baseline", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			// Seed synced_fp from the wiki item — wiki_fp == synced_fp,
			// so the outbound diff sees no edit and pushes nothing.
			seedSyncedFromWiki(profile, page, listName, store, chk.items)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should not push anything (wiki at synced baseline)", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})
	})

	// outbound_pushes_when_wiki_diverges_from_synced — task #77. The
	// converse of the previous spec: with wiki_fp != synced_fp, the
	// outbound loop must include the item in the push body.
	Describe("outbound_pushes_when_wiki_diverges_from_synced", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Green Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			// Seed synced_fp from Keep — wiki has the edit, baseline is
			// at the previously-synced "Apples", so wiki_fp != synced_fp.
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push exactly one node (wiki edit)", func() {
			Expect(kc.pushCount()).To(Equal(1))
			Expect(kc.lastPush().Nodes).To(HaveLen(1))
		})

		It("should push the wiki-edited text", func() {
			Expect(kc.findPushedItem("srv-A").Text).To(Equal("Green Apples"))
		})
	})

	// outbound_advances_synced_fp_after_successful_push — task #77.
	// After a successful push, the ItemBinding's synced_fp must be
	// advanced to the just-pushed content. Without this, the next tick
	// would re-read wiki_fp, compare against the unchanged synced_fp,
	// and re-push the same edit forever.
	Describe("outbound_advances_synced_fp_after_successful_push", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Green Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			// Pre-push synced baseline = Keep's "Apples".
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should advance ItemBinding.SyncedText to the pushed value", func() {
			b, found, ferr := c.FindBinding(ctx, profile, page, listName)
			Expect(ferr).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			ib, ok := b.ItemIDMap["uid-A"]
			Expect(ok).To(BeTrue())
			Expect(ib.SyncedText).To(Equal("Green Apples"))
		})

		It("should advance ItemBinding.SyncedSortValue to the pushed value", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].SyncedSortValue).To(Equal("1000"))
		})
	})

	// outbound_advances_synced_fp_for_fresh_items_after_push — task #77.
	// Fresh items (no id_map entry pre-push) get an ItemBinding created
	// during the response walk when Keep echoes back a server-assigned
	// ID. That new entry must also carry synced_fp at the pushed values
	// so the next tick sees wiki_fp == synced_fp.
	Describe("outbound_advances_synced_fp_for_fresh_items_after_push", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-NEW", Text: "Bread", Checked: false, SortOrder: 2000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes:     []protocol.Node{},
			}
			// The default fakeKeepClient does not echo pushed nodes
			// back in its response; without an echo, the connector's
			// response walk has nothing to match against and never
			// populates id_map for fresh items. Wrap with an echoing
			// client so the response carries an echoed node per pushed
			// LIST_ITEM, with a fresh server-assigned ServerID — that
			// triggers the id_map creation + synced_fp advance path.
			echoing := &echoingPushClient{inner: kc}
			c.SetClientBuilder(func(_ string) bridge.KeepClient { return echoing })
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should populate id_map[uid-NEW] with a serverID", func() {
			b, found, ferr := c.FindBinding(ctx, profile, page, listName)
			Expect(ferr).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			ib, ok := b.ItemIDMap["uid-NEW"]
			Expect(ok).To(BeTrue())
			Expect(ib.ServerID).ToNot(BeEmpty())
		})

		It("should advance the new id_map entry's SyncedText", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-NEW"].SyncedText).To(Equal("Bread"))
		})

		It("should advance the new id_map entry's SyncedChecked", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-NEW"].SyncedChecked).To(BeFalse())
		})

		It("should advance the new id_map entry's SyncedSortValue", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-NEW"].SyncedSortValue).To(Equal("2000"))
		})
	})

	// push_partial_failure_does_not_advance_synced_fp_for_failed_item — task #78.
	// Two items pushed; Keep reports SUCCESS for the first and ERROR for
	// the second. The successful item's synced_fp advances and its
	// PushFailureCount is zero; the failed item's synced_fp is unchanged
	// and its PushFailureCount is 1 + LastFailureCode is "ERROR".
	Describe("push_partial_failure_does_not_advance_synced_fp_for_failed_item", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
				"uid-B": "srv-B",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Green Apples", SortOrder: 1000, UpdatedAt: recentTime()},
				{Uid: "uid-B", Text: "Bananas New", SortOrder: 2000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
					keepItem("srv-B", "client-B", listSrv, "Bananas", false, "2000"),
				},
			}
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			kc.pushResponse = protocol.ChangesResponse{
				ToVersion: "v-after-push",
				WriteResults: []protocol.NodeWriteResult{
					{ID: "client-A", Status: "SUCCESS"},
					{ID: "client-B", Status: "ERROR"},
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should advance synced_fp for the SUCCESS uid", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].SyncedText).To(Equal("Green Apples"))
		})

		It("should reset PushFailureCount to 0 for the SUCCESS uid", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].PushFailureCount).To(Equal(0))
		})

		It("should leave synced_fp at its prior baseline for the ERROR uid", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-B"].SyncedText).To(Equal("Bananas"))
		})

		It("should set PushFailureCount=1 for the ERROR uid", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-B"].PushFailureCount).To(Equal(1))
		})

		It("should set LastFailureCode to the Keep status for the ERROR uid", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-B"].LastFailureCode).To(Equal("ERROR"))
		})
	})

	// push_failure_increments_count_and_does_not_advance_synced_fp — task #78.
	// Single failed push: SyncedText stays at the prior baseline,
	// PushFailureCount=1, LastFailureCode is set.
	Describe("push_failure_increments_count_and_does_not_advance_synced_fp", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Green Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			kc.pushResponse = protocol.ChangesResponse{
				ToVersion: "v-after-push",
				WriteResults: []protocol.NodeWriteResult{
					{ID: "client-A", Status: "ERROR"},
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should leave SyncedText unchanged at the prior baseline", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].SyncedText).To(Equal("Apples"))
		})

		It("should set PushFailureCount to 1", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].PushFailureCount).To(Equal(1))
		})

		It("should set LastFailureCode to the returned status", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].LastFailureCode).To(Equal("ERROR"))
		})

		It("should set NextAttemptAt to now+60s", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			expected := tNow.Add(60 * time.Second)
			Expect(b.ItemIDMap["uid-A"].NextAttemptAt).To(BeTemporally("~", expected, time.Second))
		})
	})

	// push_failure_with_no_response_status_uses_no_response_status_code — task #78.
	// Keep returned no per-node WriteResults entry (e.g. the response shape
	// lacked the array, or the entry didn't match by clientID). Counts as
	// failure: bump PushFailureCount, set LastFailureCode to the
	// "no_response_status" sentinel, do NOT advance synced_fp.
	Describe("push_failure_with_no_response_status_uses_no_response_status_code", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Green Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			// Push response carries no WriteResults at all.
			kc.pushResponse = protocol.ChangesResponse{
				ToVersion: "v-after-push",
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should leave SyncedText at the prior baseline", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].SyncedText).To(Equal("Apples"))
		})

		It("should bump PushFailureCount to 1", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].PushFailureCount).To(Equal(1))
		})

		It("should set LastFailureCode to the no_response_status sentinel", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].LastFailureCode).To(Equal("no_response_status"))
		})
	})

	// push_success_resets_failure_count_and_clears_failure_code — task #78.
	// Pre-existing PushFailureCount=5 + LastFailureCode + NextAttemptAt;
	// a successful push clears all three.
	Describe("push_success_resets_failure_count_and_clears_failure_code", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Green Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			// Pre-set the failure state directly.
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			ib := st.Bindings[0].ItemIDMap["uid-A"]
			ib.PushFailureCount = 5
			ib.LastFailureCode = "rate_limited"
			ib.NextAttemptAt = tStaleA // already-past, won't gate
			st.Bindings[0].ItemIDMap["uid-A"] = ib
			Expect(bs.SaveState(profile, st)).To(Succeed())

			kc.pushResponse = protocol.ChangesResponse{
				ToVersion: "v-after-push",
				WriteResults: []protocol.NodeWriteResult{
					{ID: "client-A", Status: "SUCCESS"},
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should reset PushFailureCount to 0", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].PushFailureCount).To(Equal(0))
		})

		It("should clear LastFailureCode", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].LastFailureCode).To(Equal(""))
		})

		It("should clear NextAttemptAt", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].NextAttemptAt.IsZero()).To(BeTrue())
		})
	})

	// dead_lettered_item_is_skipped_in_outbound_diff — task #78. With
	// PushFailureCount >= deadLetterThreshold (10), the diff loop
	// omits the uid from the push entirely.
	Describe("dead_lettered_item_is_skipped_in_outbound_diff", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
				"uid-B": "srv-B",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Green Apples", SortOrder: 1000, UpdatedAt: recentTime()},
				{Uid: "uid-B", Text: "Green Bananas", SortOrder: 2000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
					keepItem("srv-B", "client-B", listSrv, "Bananas", false, "2000"),
				},
			}
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			// Dead-letter uid-A.
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			ib := st.Bindings[0].ItemIDMap["uid-A"]
			ib.PushFailureCount = 10
			ib.LastFailureCode = "permanent_failure"
			st.Bindings[0].ItemIDMap["uid-A"] = ib
			Expect(bs.SaveState(profile, st)).To(Succeed())
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should still push (uid-B is not dead-lettered)", func() {
			Expect(kc.pushCount()).To(Equal(1))
		})

		It("should omit the dead-lettered uid from the push body", func() {
			for _, n := range kc.lastPush().Nodes {
				Expect(n.ServerID).ToNot(Equal("srv-A"))
			}
		})

		It("should include the non-dead-lettered uid in the push body", func() {
			found := false
			for _, n := range kc.lastPush().Nodes {
				if n.ServerID == "srv-B" {
					found = true
				}
			}
			Expect(found).To(BeTrue())
		})

		It("should preserve the dead-lettered item's PushFailureCount", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].PushFailureCount).To(Equal(10))
		})
	})

	// wiki_side_re_edit_resets_push_failure_count — task #78. The user
	// edits the wiki side after a dead-letter; the diff loop sees that
	// wiki_fp differs from LastObservedWiki* and resets the failure
	// count BEFORE the dead-letter check, so the item gets pushed again.
	Describe("wiki_side_re_edit_resets_push_failure_count", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			// New text — user just edited on the wiki side.
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "new text", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "old text", false, "1000"),
				},
			}
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			// Pre-set: dead-lettered + LastObservedWiki* records the
			// PRE-edit wiki state. Wiki_fp now differs from
			// LastObservedWiki* → reset.
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			ib := st.Bindings[0].ItemIDMap["uid-A"]
			ib.PushFailureCount = 10
			ib.LastFailureCode = "rate_limited"
			ib.LastObservedWikiText = "old text"
			ib.LastObservedWikiSortValue = "1000"
			st.Bindings[0].ItemIDMap["uid-A"] = ib
			Expect(bs.SaveState(profile, st)).To(Succeed())
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should re-push the item (failure count is reset)", func() {
			Expect(kc.pushCount()).To(Equal(1))
			Expect(kc.findPushedItem("srv-A").Text).To(Equal("new text"))
		})

		It("should reset PushFailureCount to 0", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].PushFailureCount).To(Equal(0))
		})

		It("should clear LastFailureCode", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].LastFailureCode).To(Equal(""))
		})
	})

	// last_observed_wiki_fields_written_at_end_of_tick — task #78. After
	// SyncToKeep, every uid in id_map has LastObservedWiki* populated to
	// reflect the post-tick wiki state.
	Describe("last_observed_wiki_fields_written_at_end_of_tick", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should populate LastObservedWikiText for items in id_map", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].LastObservedWikiText).To(Equal("Apples"))
		})

		It("should populate LastObservedWikiSortValue for items in id_map", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].LastObservedWikiSortValue).To(Equal("1000"))
		})
	})

	// next_attempt_at_skips_recently_failed_item — task #78. NextAttemptAt
	// 60s in the future and clock.Now() before that → diff loop skips
	// the item even though wiki_fp != synced_fp.
	Describe("next_attempt_at_skips_recently_failed_item", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Green Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			ib := st.Bindings[0].ItemIDMap["uid-A"]
			ib.PushFailureCount = 1
			ib.LastFailureCode = "rate_limited"
			ib.NextAttemptAt = tNow.Add(60 * time.Second)
			st.Bindings[0].ItemIDMap["uid-A"] = ib
			Expect(bs.SaveState(profile, st)).To(Succeed())
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should not push the item still inside its backoff window", func() {
			Expect(kc.pushCount()).To(Equal(0))
		})

		It("should preserve PushFailureCount during the backoff skip", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].PushFailureCount).To(Equal(1))
		})
	})

	// next_attempt_at_advances_after_failure — task #78. After a failed
	// push, NextAttemptAt is set to now + min(60 * 2^(n-1), 3600) seconds.
	Describe("next_attempt_at_advances_after_failure", func() {
		var store *fakeStore

		BeforeEach(func() {
			c, store, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			chk.items = []*apiv1.ChecklistItem{
				{Uid: "uid-A", Text: "Green Apples", SortOrder: 1000, UpdatedAt: recentTime()},
			}
			kc.pullState = protocol.ChangesResponse{
				ToVersion: "v0",
				Nodes: []protocol.Node{
					keepItem("srv-A", "client-A", listSrv, "Apples", false, "1000"),
				},
			}
			seedSyncedFromKeep(profile, page, listName, store, kc.pullState.Nodes)
			// Pre-existing 2 failures → this one becomes the 3rd, so
			// NextAttemptAt = now + 60 * 2^2 = 240s.
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			ib := st.Bindings[0].ItemIDMap["uid-A"]
			ib.PushFailureCount = 2
			ib.LastFailureCode = "rate_limited"
			st.Bindings[0].ItemIDMap["uid-A"] = ib
			Expect(bs.SaveState(profile, st)).To(Succeed())

			kc.pushResponse = protocol.ChangesResponse{
				ToVersion: "v-after-push",
				WriteResults: []protocol.NodeWriteResult{
					{ID: "client-A", Status: "ERROR"},
				},
			}
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should advance PushFailureCount to 3", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			Expect(b.ItemIDMap["uid-A"].PushFailureCount).To(Equal(3))
		})

		It("should set NextAttemptAt to now + 240s (60 * 2^2)", func() {
			b, _, _ := c.FindBinding(ctx, profile, page, listName)
			expected := tNow.Add(240 * time.Second)
			Expect(b.ItemIDMap["uid-A"].NextAttemptAt).To(BeTemporally("~", expected, time.Second))
		})
	})
})

// echoingPushClient wraps fakeKeepClient so that on push, the response
// echoes back each pushed LIST_ITEM with a server-assigned ServerID
// derived from the request's client_id. Used by the
// outbound_advances_synced_fp_for_fresh_items_with_echo test to drive
// the response-walk path that creates id_map entries for fresh items.
type echoingPushClient struct {
	inner *fakeKeepClient
}

func (e *echoingPushClient) Changes(ctx context.Context, req protocol.ChangesRequest) (protocol.ChangesResponse, error) {
	if len(req.Nodes) == 0 && len(req.Labels) == 0 {
		return e.inner.Changes(ctx, req)
	}
	// Invoke the inner so the request is recorded; then synthesize
	// an echo by mapping each request node to an echoed response node
	// with a fresh ServerID.
	_, err := e.inner.Changes(ctx, req)
	if err != nil {
		return protocol.ChangesResponse{}, err
	}
	echoed := make([]protocol.Node, 0, len(req.Nodes))
	writeResults := make([]protocol.NodeWriteResult, 0, len(req.Nodes))
	for _, n := range req.Nodes {
		if n.Type != protocol.NodeTypeListItem {
			continue
		}
		echo := n
		if echo.ServerID == "" {
			echo.ServerID = "echoed-" + n.ID
		}
		echoed = append(echoed, echo)
		writeResults = append(writeResults, protocol.NodeWriteResult{
			ID:     n.ID,
			Status: "SUCCESS",
		})
	}
	return protocol.ChangesResponse{
		ToVersion:    "v-after-push",
		Nodes:        echoed,
		WriteResults: writeResults,
	}, nil
}

func (e *echoingPushClient) CreateList(ctx context.Context, title string) (string, error) {
	return e.inner.CreateList(ctx, title)
}

func (e *echoingPushClient) CreateListWithItems(ctx context.Context, title string, items []protocol.ListItemSpec) (protocol.CreateListResult, error) {
	return e.inner.CreateListWithItems(ctx, title, items)
}

// fakeInfoLogger captures Info(...) calls for assertion. Satisfies
// protocol.DebugLogger so SetDebugLogger accepts it.
type fakeInfoLogger struct {
	mu       sync.Mutex
	formats  []string
	rendered []string
}

func (l *fakeInfoLogger) Info(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.formats = append(l.formats, format)
	l.rendered = append(l.rendered, fmt.Sprintf(format, args...))
}

// containsSubstring reports whether any captured Info call (format or
// rendered output) contains the given substring (case-insensitive).
func (l *fakeInfoLogger) containsSubstring(s string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	needle := strings.ToLower(s)
	for _, f := range l.formats {
		if strings.Contains(strings.ToLower(f), needle) {
			return true
		}
	}
	for _, r := range l.rendered {
		if strings.Contains(strings.ToLower(r), needle) {
			return true
		}
	}
	return false
}

// truncationResyncThresholdInTests mirrors the connector-package
// constant for the truncation escape hatch threshold. Kept as a helper
// so the test reads as "the threshold" rather than a magic number, and
// the source of truth stays in connector.go.
func truncationResyncThresholdInTests() int {
	return 5
}

// SyncToKeep skip-gate: legacy bindings (MigratedFingerprints=false)
// must short-circuit BEFORE any pull. The eager migration job
// rebaselines synced_fp first; until it stamps MigratedFingerprints=
// true, the sync engine has no synced_fp to test divergence against
// and would silently rebaseline (the "lazy first-tick" approach the
// plan rejected). The gate also throttles its INFO log to the first
// skip per binding per process so a stuck-un-migrated binding doesn't
// spam the journal at the cron cadence.
var _ = Describe("SyncToKeep — un-migrated binding gate", func() {
	const (
		profile  = wikipage.PageIdentifier("profile_test")
		page     = "shopping"
		listName = "Grocery"
		listSrv  = "list-server-id"
	)

	Describe("when binding has MigratedFingerprints=false", func() {
		var (
			c     *bridge.Connector
			kc    *fakeKeepClient
			store *fakeStore
			err   error
		)

		BeforeEach(func() {
			c, store, kc, _ = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			// Flip the default born-migrated to false to express
			// the legacy-on-disk shape.
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			st.Bindings[0].MigratedFingerprints = false
			Expect(bs.SaveState(profile, st)).To(Succeed())

			err = c.SyncToKeep(context.Background(), profile, page, listName)
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should NOT call Changes() on the Keep client", func() {
			Expect(kc.pulledRequests).To(BeEmpty())
			Expect(kc.pushedRequests).To(BeEmpty())
		})
	})

	Describe("when SyncToKeep skips the same binding twice", func() {
		var (
			c   *bridge.Connector
			lg  *fakeInfoLogger
		)

		BeforeEach(func() {
			var store *fakeStore
			c, store, _, _ = freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			lg = &fakeInfoLogger{}
			c.SetDebugLogger(lg)

			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			st.Bindings[0].MigratedFingerprints = false
			Expect(bs.SaveState(profile, st)).To(Succeed())

			Expect(c.SyncToKeep(context.Background(), profile, page, listName)).To(Succeed())
			Expect(c.SyncToKeep(context.Background(), profile, page, listName)).To(Succeed())
		})

		It("should log the skip exactly once per process", func() {
			count := 0
			lg.mu.Lock()
			defer lg.mu.Unlock()
			for _, f := range lg.formats {
				if strings.Contains(strings.ToLower(f), "skip") {
					count++
				}
			}
			Expect(count).To(Equal(1))
		})
	})

	Describe("when a freshly-bound binding is created via Bind", func() {
		var b bridge.Binding

		BeforeEach(func() {
			c, store, _, _ := freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			// Drop the seeded binding so Bind() runs against a clean
			// connected-but-no-bindings state.
			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			st.Bindings = nil
			Expect(bs.SaveState(profile, st)).To(Succeed())

			var bindErr error
			b, bindErr = c.Bind(context.Background(), profile, "newpage", "newlist", "", nil)
			Expect(bindErr).ToNot(HaveOccurred())
		})

		It("should stamp MigratedFingerprints=true on the new binding", func() {
			Expect(b.MigratedFingerprints).To(BeTrue())
		})
	})

	Describe("when a fresh Connector instance encounters the same un-migrated binding", func() {
		var (
			lg2 *fakeInfoLogger
		)

		BeforeEach(func() {
			c1, store, _, _ := freshConnector(profile, page, listName, listSrv, map[string]string{
				"uid-A": "srv-A",
			})
			lg1 := &fakeInfoLogger{}
			c1.SetDebugLogger(lg1)

			bs := bridge.NewBindingStore(store)
			st, loadErr := bs.LoadState(profile)
			Expect(loadErr).ToNot(HaveOccurred())
			st.Bindings[0].MigratedFingerprints = false
			Expect(bs.SaveState(profile, st)).To(Succeed())

			Expect(c1.SyncToKeep(context.Background(), profile, page, listName)).To(Succeed())

			// Simulate a process restart: build a NEW Connector against
			// the same store. The throttle map must reset.
			c2 := bridge.NewConnector(bridge.NewBindingStore(store), nil, fakeClock{})
			c2.SetClientBuilder(func(_ string) bridge.KeepClient { return &fakeKeepClient{} })
			c2.SetAuthBuilder(func(_ string) bridge.AuthExchanger { return fakeAuth{} })
			c2.SetChecklistReader(&fakeChecklist{})
			c2.SetChecklistMutator(&fakeChecklist{})
			c2.SetSyncSuppressor(fakeSuppressor{})
			lg2 = &fakeInfoLogger{}
			c2.SetDebugLogger(lg2)

			Expect(c2.SyncToKeep(context.Background(), profile, page, listName)).To(Succeed())
		})

		It("should log the skip again for the new Connector", func() {
			count := 0
			lg2.mu.Lock()
			defer lg2.mu.Unlock()
			for _, f := range lg2.formats {
				if strings.Contains(strings.ToLower(f), "skip") {
					count++
				}
			}
			Expect(count).To(Equal(1))
		})
	})
})
