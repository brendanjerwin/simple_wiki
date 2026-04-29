//revive:disable:dot-imports
package bridge_test

import (
	"context"
	"errors"
	"fmt"
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
// pullState (set per-test); push responses are an empty downSync with
// the next toVersion. Implements bridge.KeepClient.
type fakeKeepClient struct {
	mu             sync.Mutex
	pullState      protocol.ChangesResponse
	pushResponse   protocol.ChangesResponse
	pushedRequests []protocol.ChangesRequest
}

func (c *fakeKeepClient) Changes(_ context.Context, req protocol.ChangesRequest) (protocol.ChangesResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(req.Nodes) == 0 && len(req.Labels) == 0 {
		// pull
		return c.pullState, nil
	}
	c.pushedRequests = append(c.pushedRequests, req)
	if c.pushResponse.ToVersion == "" {
		return protocol.ChangesResponse{ToVersion: "v-after-push"}, nil
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

func (c *fakeChecklist) AddItemForSync(_ context.Context, _, _, ownerEmail, text string, checked bool, tags []string, description, _ string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addCalls = append(c.addCalls, addRecord{OwnerEmail: ownerEmail, Text: text, Description: description, Tags: tags, Checked: checked})
	c.uidCount++
	uid := fmt.Sprintf("test-uid-%d", c.uidCount)
	now := timestamppb.New(time.Now())
	item := &apiv1.ChecklistItem{
		Uid:       uid,
		Text:      text,
		Tags:      tags,
		Checked:   checked,
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

	// Seed LastPushedAt to a value between staleTime and recentTime
	// so the default test scenario looks like "wiki is synced" — wiki
	// items with stale UpdatedAt are below this anchor (apply Keep ok),
	// items with recent UpdatedAt are above (uncommitted, push wins).
	// Tests that need to override (e.g. fresh-bind tests) set their
	// own binding state.
	state := bridge.ConnectorState{
		Email:       "test@example.com",
		MasterToken: "token",
		Bindings: []bridge.Binding{{
			Page: page, ListName: listName,
			KeepNoteID: keepNoteID, KeepNoteTitle: listName,
			BoundAt:      time.Now().UTC(),
			LastPushedAt: tStaleB,
			ItemIDMap:    idMap,
		}},
	}
	Expect(bridge.NewBindingStore(store).SaveState(profileID, state)).To(Succeed())
	return c, store, keep, chk
}

// Time anchors used across the matrix. Keeping these as named values
// avoids the lint nag about repeated time.Date(year, month, ...) calls
// and makes the relative ordering at a glance.
//
// Ordering: tStaleA < tStaleB < tKeepRecent < tKeepRecent2 < tNow < tFuture.
var (
	tStaleA      = time.Date(2026, time.April, 26, 0, 0, 0, 0, time.UTC)         //nolint:gochecknoglobals
	tStaleB      = time.Date(2026, time.April, 28, 0, 0, 0, 0, time.UTC)         //nolint:gochecknoglobals
	tKeepRecent  = time.Date(2026, time.April, 28, 12, 0, 0, 0, time.UTC)        //nolint:gochecknoglobals
	tKeepRecent2 = time.Date(2026, time.April, 30, 12, 0, 0, 0, time.UTC)        //nolint:gochecknoglobals
	tWikiAnchor  = time.Date(2026, time.May, 1, 9, 0, 0, 0, time.UTC)            //nolint:gochecknoglobals
	tNow         = time.Date(2026, time.May, 1, 12, 0, 0, 0, time.UTC)           //nolint:gochecknoglobals
	tFuture      = time.Date(2026, time.May, 2, 12, 0, 0, 0, time.UTC)           //nolint:gochecknoglobals
	tEpochPlusMs = time.Date(1970, time.January, 1, 0, 0, 0, 1000000, time.UTC)  //nolint:gochecknoglobals
	tFiveMinAgo  = time.Date(2026, time.May, 1, 11, 55, 0, 0, time.UTC)          //nolint:gochecknoglobals
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
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
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
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
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
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
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
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push the encoded text with #fruit and #produce appended", func() {
			Expect(kc.findPushedItem("srv-A").Text).To(Equal("Apples #fruit #produce"))
		})
	})

	// W5 — wiki description edit.
	Describe("W5 — wiki adds a description", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
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
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should push text with the description after the separator", func() {
			Expect(kc.findPushedItem("srv-A").Text).To(Equal("Apples\n— the red kind"))
		})
	})

	// W6 — wiki sort order changed.
	Describe("W6 — wiki sort order changed", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
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
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
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

	// B2 — both edited; wiki newer.
	Describe("B2 — both edited, wiki newer", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
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
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should not call UpdateItemForSync (gate skipped)", func() {
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
			// Wiki was edited 30 min ago, pushed 25 min ago. Override
			// LastPushedAt to capture the post-push synced state.
			thirtyMinAgo := timestamppb.New(tNow.Add(-30 * time.Minute))
			twentyFiveMinAgo := tNow.Add(-25 * time.Minute)
			bs := bridge.NewBindingStore(store)
			stateNow, err := bs.LoadState(profile)
			Expect(err).NotTo(HaveOccurred())
			stateNow.Bindings[0].LastPushedAt = twentyFiveMinAgo
			Expect(bs.SaveState(profile, stateNow)).To(Succeed())

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

	// 15e — content differs, wiki UpdatedAt EQUAL to Keep timestamp.
	// Gate is strict After; equal timestamps mean "wiki side wins" via
	// the inbound apply being skipped, then push happens (wiki -> Keep).
	Describe("15e — content differs, timestamps equal", func() {
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
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
			Expect(c.SyncToKeep(ctx, profile, page, listName)).To(Succeed())
		})

		It("should NOT apply Keep's value (gate is strict After)", func() {
			Expect(chk.upsCalls).To(BeEmpty())
		})

		It("should push wiki's value to Keep (wiki UpdatedAt is not strictly newer either, but content diff still pushes)", func() {
			// shouldPushWikiUpdate is also strict After. With equal
			// timestamps, neither side pushes via the gate, but the
			// content-equality skip is the OUTER gate — equal timestamps
			// don't block the diff. With diff content the push still
			// happens because the gate isn't actually applied to the
			// content-diff loop in the current code; the gate runs in
			// shouldPushWikiUpdate which IS NOT called in the
			// content-diff path (only the keepNodes content-equality
			// check runs). So push happens.
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
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
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
		BeforeEach(func() {
			c, _, kc, chk = freshConnector(profile, page, listName, listSrv, map[string]string{
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
})
