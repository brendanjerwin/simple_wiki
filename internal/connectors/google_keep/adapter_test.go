//revive:disable:dot-imports
package google_keep_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	googlekeep "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

func TestGoogleKeepAdapter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "google_keep adapter Suite")
}

// --- fakes -----------------------------------------------------------

// fakeKeepClient is the in-memory stand-in for *gateway.KeepClient.
type fakeKeepClient struct {
	mu sync.Mutex

	// changesResponses is the queue of canned responses Changes returns;
	// drained as called. If the queue is empty, changesDefault is used.
	changesResponses []gateway.ChangesResponse
	changesDefault   gateway.ChangesResponse
	changesErr       error
	changes          []gateway.ChangesRequest

	createListErr error
	createList    []string

	createListWithItemsErr error
	createListWithItems    []createListWithItemsCall

	nextCreatedListID    int
	nextCreatedItemID    int
}

type createListWithItemsCall struct {
	Title string
	Items []gateway.ListItemSpec
}

func newFakeKeepClient() *fakeKeepClient {
	return &fakeKeepClient{}
}

func (f *fakeKeepClient) Changes(_ context.Context, req gateway.ChangesRequest) (gateway.ChangesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.changes = append(f.changes, req)
	if f.changesErr != nil {
		return gateway.ChangesResponse{}, f.changesErr
	}
	if len(f.changesResponses) > 0 {
		resp := f.changesResponses[0]
		f.changesResponses = f.changesResponses[1:]
		return resp, nil
	}
	return f.changesDefault, nil
}

func (f *fakeKeepClient) CreateList(_ context.Context, title string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createList = append(f.createList, title)
	if f.createListErr != nil {
		return "", f.createListErr
	}
	f.nextCreatedListID++
	return fmt.Sprintf("created-list-%d", f.nextCreatedListID), nil
}

func (f *fakeKeepClient) CreateListWithItems(_ context.Context, title string, items []gateway.ListItemSpec) (gateway.CreateListResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createListWithItems = append(f.createListWithItems, createListWithItemsCall{Title: title, Items: items})
	if f.createListWithItemsErr != nil {
		return gateway.CreateListResult{}, f.createListWithItemsErr
	}
	f.nextCreatedListID++
	listServerID := fmt.Sprintf("created-list-%d", f.nextCreatedListID)
	itemIDs := make([]string, len(items))
	for i := range items {
		f.nextCreatedItemID++
		itemIDs[i] = fmt.Sprintf("created-item-%d", f.nextCreatedItemID)
	}
	return gateway.CreateListResult{
		ListServerID:  listServerID,
		ListClientID:  "client-" + listServerID,
		ItemServerIDs: itemIDs,
	}, nil
}

// fakeCredentialReader returns a fixed master token + email + device id.
type fakeCredentialReader struct {
	token  string
	email  string
	device string
	err    error
}

func (f *fakeCredentialReader) LoadMasterToken(_ context.Context, _ wikipage.PageIdentifier) (googlekeep.MasterTokenBundle, error) {
	if f.err != nil {
		return googlekeep.MasterTokenBundle{}, f.err
	}
	return googlekeep.MasterTokenBundle{MasterToken: f.token, Email: f.email, AndroidID: f.device}, nil
}

type silentLogger struct{}

func (silentLogger) Info(string, ...any)  {}
func (silentLogger) Error(string, ...any) {}

// fakeFrontmatterReadWriter is the in-memory stand-in for the wiki's
// page reader/writer. Used for FrontmatterCredentialStore tests.
type fakeFrontmatterReadWriter struct {
	pages map[wikipage.PageIdentifier]wikipage.FrontMatter
	err   map[wikipage.PageIdentifier]error
}

func newFakeFrontmatterReadWriter() *fakeFrontmatterReadWriter {
	return &fakeFrontmatterReadWriter{
		pages: map[wikipage.PageIdentifier]wikipage.FrontMatter{},
		err:   map[wikipage.PageIdentifier]error{},
	}
}

func (f *fakeFrontmatterReadWriter) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	if e, ok := f.err[id]; ok {
		return id, nil, e
	}
	fm, ok := f.pages[id]
	if !ok {
		return id, nil, os.ErrNotExist
	}
	return id, fm, nil
}

func (f *fakeFrontmatterReadWriter) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	f.pages[id] = fm
	return nil
}

// fixedClock is a deterministic Clock for adapter tests.
type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }

// --- specs -----------------------------------------------------------

var _ = Describe("KeepAdapter", func() {
	var (
		ctx           context.Context
		fakeClient    *fakeKeepClient
		creds         *fakeCredentialReader
		clientFactory googlekeep.KeepClientFactory
		factoryErr    error
		adapter       *googlekeep.KeepAdapter
		profile       wikipage.PageIdentifier
		remoteHandle  string
		clock         *fixedClock
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClient = newFakeKeepClient()
		creds = &fakeCredentialReader{token: "mt-abc", email: "u@example.com", device: "android-1"}
		factoryErr = nil
		clientFactory = func(_ context.Context, _ wikipage.PageIdentifier, _, _ string) (googlekeep.KeepClient, error) {
			if factoryErr != nil {
				return nil, factoryErr
			}
			return fakeClient, nil
		}
		clock = &fixedClock{now: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)}
		var err error
		adapter, err = googlekeep.NewKeepAdapter(creds, clientFactory, clock, silentLogger{})
		Expect(err).NotTo(HaveOccurred())
		profile = wikipage.PageIdentifier("profile_alice")
		remoteHandle = "list-server-id-1"
	})

	Describe("constructor", func() {
		When("any dependency is nil", func() {
			It("should reject nil credentials", func() {
				_, err := googlekeep.NewKeepAdapter(nil, clientFactory, clock, silentLogger{})
				Expect(err).To(MatchError(ContainSubstring("credentials must not be nil")))
			})

			It("should reject nil clientFactory", func() {
				_, err := googlekeep.NewKeepAdapter(creds, nil, clock, silentLogger{})
				Expect(err).To(MatchError(ContainSubstring("clientFactory must not be nil")))
			})

			It("should reject nil clock", func() {
				_, err := googlekeep.NewKeepAdapter(creds, clientFactory, nil, silentLogger{})
				Expect(err).To(MatchError(ContainSubstring("clock must not be nil")))
			})

			It("should reject nil logger", func() {
				_, err := googlekeep.NewKeepAdapter(creds, clientFactory, clock, nil)
				Expect(err).To(MatchError(ContainSubstring("logger must not be nil")))
			})
		})
	})

	Describe("Kind", func() {
		It("should return ConnectorKindGoogleKeep", func() {
			Expect(adapter.Kind()).To(Equal(connectors.ConnectorKindGoogleKeep))
		})
	})

	Describe("SupportsSubtasks", func() {
		It("should report false (Keep has flat lists)", func() {
			Expect(adapter.SupportsSubtasks()).To(BeFalse())
		})
	})

	Describe("PullRemote", func() {
		var (
			binding connectors.Binding
			result  connectors.RemotePullResult
			pullErr error
		)

		BeforeEach(func() {
			binding = connectors.Binding{
				ProfileID:    profile,
				RemoteHandle: remoteHandle,
			}
		})

		When("the remote returns LIST_ITEM children", func() {
			BeforeEach(func() {
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v123",
					Nodes: []gateway.Node{
						{
							ID:       "client-1",
							ServerID: "srv-1",
							Type:     gateway.NodeTypeListItem,
							ParentID: remoteHandle,
							Text:     "milk",
						},
						{
							ID:       "client-2",
							ServerID: "srv-2",
							Type:     gateway.NodeTypeListItem,
							ParentServerID: remoteHandle,
							Text:     "eggs",
							Checked:  true,
						},
						{
							ID:       "client-3",
							ServerID: "srv-3",
							Type:     gateway.NodeTypeListItem,
							ParentID: "other-list",
							Text:     "should be filtered",
						},
					},
				}
				result, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should not error", func() {
				Expect(pullErr).NotTo(HaveOccurred())
			})

			It("should return only items under the bound list", func() {
				Expect(result.Items).To(HaveLen(2))
			})

			It("should populate refs from ServerID", func() {
				Expect(result.Items[0].Ref).To(Equal(connectors.RemoteRef("srv-1")))
				Expect(result.Items[1].Ref).To(Equal(connectors.RemoteRef("srv-2")))
			})

			It("should expose the new cursor as a string", func() {
				cursor, ok := result.NewCursor.(string)
				Expect(ok).To(BeTrue())
				Expect(cursor).To(Equal("v123"))
			})
		})

		When("the credential reader fails", func() {
			BeforeEach(func() {
				creds.err = googlekeep.ErrCredentialMissing
				_, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should propagate the error", func() {
				Expect(pullErr).To(MatchError(googlekeep.ErrCredentialMissing))
			})
		})

		When("the gateway returns an auth-revoked error", func() {
			BeforeEach(func() {
				fakeClient.changesErr = gateway.ErrAuthRevoked
				_, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should bubble the gateway error so ClassifyError can map it", func() {
				Expect(pullErr).To(MatchError(gateway.ErrAuthRevoked))
			})
		})

		When("the binding's adapter state has a cursor", func() {
			BeforeEach(func() {
				binding.AdapterState = connectors.AdapterState{
					googlekeep.AdapterStateKeyKeepCursor: "v100",
				}
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v200",
					Incremental: true,
				}
				_, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should send the prior cursor as TargetVersion", func() {
				Expect(pullErr).NotTo(HaveOccurred())
				Expect(fakeClient.changes).To(HaveLen(1))
				Expect(fakeClient.changes[0].TargetVersion).To(Equal("v100"))
			})
		})

		When("the response signals truncation", func() {
			BeforeEach(func() {
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v9",
					Truncated: true,
				}
				result, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should propagate Truncated=true", func() {
				Expect(pullErr).NotTo(HaveOccurred())
				Expect(result.Truncated).To(BeTrue())
			})
		})

		// Production bug 2026-05-07: user deletes item in Keep app →
		// engine doesn't mirror to wiki. Keep tombstones can arrive
		// with cleared parent linkage (ParentID/ParentServerID empty
		// when the node is removed from a list). The strict parent
		// filter dropped these tombstones, so the engine never
		// observed the deletion and the wiki kept the stale item.
		// Fix: also accept items whose ServerID is in our
		// item_id_map — we know about them, a tombstone for them is
		// in scope regardless of parent.
		When("a deleted node arrives with cleared parent linkage but its ServerID is in our item_id_map", func() {
			BeforeEach(func() {
				binding.AdapterState = connectors.AdapterState{
					"item_id_map": map[string]any{
						"uid-tomb-1": "srv-deleted-by-user",
					},
				}
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v100",
					Nodes: []gateway.Node{
						{
							// Tombstone: parent linkage cleared by Keep.
							ID:       "client-deleted",
							ServerID: "srv-deleted-by-user",
							Type:     gateway.NodeTypeListItem,
							// ParentID + ParentServerID intentionally empty
							Timestamps: gateway.Timestamps{
								Deleted: time.Date(2026, 5, 7, 23, 55, 0, 0, time.UTC),
							},
						},
					},
				}
				result, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should not error", func() {
				Expect(pullErr).NotTo(HaveOccurred())
			})

			It("should include the tombstone in the pulled items so the engine can mirror the delete", func() {
				Expect(result.Items).To(HaveLen(1))
				Expect(result.Items[0].Ref).To(Equal(connectors.RemoteRef("srv-deleted-by-user")))
				Expect(result.Items[0].Deleted).To(BeTrue())
			})
		})

		// Production regression 2026-05-08: deeper variant of the
		// cleared-parent tombstone case above. Real Keep tombstones
		// for user-deleted items can arrive with NO `type` field at
		// all (not even `LIST_ITEM`) — just an id, serverId, and a
		// `deleted` timestamp. The earlier filter at the top of
		// PullRemote (`if n.Type != NodeTypeListItem { continue }`)
		// dropped these BEFORE the cleared-parent exception could
		// catch them. Result: engine never observed the deletion;
		// wiki kept the stale item; the same stale item appeared in
		// every subsequent pull. Fix: the known-ref tombstone
		// exception must also override the type check, not just the
		// parent check. See ServerID lookup against item_id_map.
		When("a deleted node arrives with NO type field but its ServerID is in our item_id_map", func() {
			BeforeEach(func() {
				binding.AdapterState = connectors.AdapterState{
					"item_id_map": map[string]any{
						"uid-tomb-no-type-1": "srv-deleted-no-type",
					},
				}
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v200",
					Nodes: []gateway.Node{
						{
							// Production-shape barebones tombstone:
							// no Type, no parent linkage, just id +
							// serverId + a deleted timestamp.
							ID:       "cbx.z39p6v77cl6t",
							ServerID: "srv-deleted-no-type",
							Timestamps: gateway.Timestamps{
								Deleted: time.Date(2026, 5, 8, 10, 36, 50, 1000000, time.UTC),
							},
						},
					},
				}
				result, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should not error", func() {
				Expect(pullErr).NotTo(HaveOccurred())
			})

			It("should include the typeless tombstone so the engine can mirror the delete to wiki", func() {
				Expect(result.Items).To(HaveLen(1),
					"engine dropped a typeless tombstone for a known item — production regression 2026-05-08, Keep deletes not propagating to wiki")
				Expect(result.Items[0].Ref).To(Equal(connectors.RemoteRef("srv-deleted-no-type")))
				Expect(result.Items[0].Deleted).To(BeTrue())
			})
		})

		When("a deleted node arrives with cleared parent linkage AND its ServerID is NOT in our item_id_map", func() {
			// Negative case: Keep account-wide pulls return tombstones
			// for items in OTHER lists that we don't track. The
			// item_id_map lookup keeps us from accidentally adopting
			// those.
			BeforeEach(func() {
				binding.AdapterState = connectors.AdapterState{
					"item_id_map": map[string]any{
						"uid-known": "srv-in-our-list",
					},
				}
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v101",
					Nodes: []gateway.Node{
						{
							ID:       "client-foreign",
							ServerID: "srv-in-other-list",
							Type:     gateway.NodeTypeListItem,
							Timestamps: gateway.Timestamps{
								Deleted: time.Date(2026, 5, 7, 23, 55, 0, 0, time.UTC),
							},
						},
					},
				}
				result, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should NOT include the foreign tombstone", func() {
				Expect(result.Items).To(BeEmpty())
			})
		})

		// keep_note_client_id self-heal — legacy keepsync had this and
		// it was lost in the Phase 5-A port. Bindings with empty
		// keep_note_client_id permanently fail label CRUD pushes
		// (Keep stage3-500 because the LIST node violates the
		// id != serverId invariant on outbound push). Self-heal: when
		// the LIST node is in the pull response and our stored
		// keep_note_client_id is empty, capture the LIST node's client
		// id and write it back into binding.AdapterState.
		When("the binding has an empty keep_note_client_id and the LIST node appears in the pull", func() {
			BeforeEach(func() {
				binding.AdapterState = connectors.AdapterState{
					googlekeep.AdapterStateKeyKeepNoteClientID: "",
				}
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v10",
					Nodes: []gateway.Node{
						{
							ID:       "list-client-id-from-pull",
							ServerID: remoteHandle,
							Type:     gateway.NodeTypeList,
							Title:    "Groceries",
						},
					},
				}
				_, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should not error", func() {
				Expect(pullErr).NotTo(HaveOccurred())
			})

			It("should self-heal keep_note_client_id from the LIST node", func() {
				Expect(binding.AdapterState[googlekeep.AdapterStateKeyKeepNoteClientID]).
					To(Equal("list-client-id-from-pull"))
			})
		})

		When("the binding has an empty keep_note_client_id but the LIST node is absent from the incremental pull", func() {
			BeforeEach(func() {
				binding.AdapterState = connectors.AdapterState{
					googlekeep.AdapterStateKeyKeepNoteClientID: "",
				}
				// Pull response carries no LIST node (incremental pulls
				// only return changed nodes; LIST node hasn't changed).
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v11",
					Nodes:     []gateway.Node{},
				}
				result, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should not error", func() {
				Expect(pullErr).NotTo(HaveOccurred())
			})

			It("should signal Truncated=true to force a full resync (which will include the LIST node)", func() {
				Expect(result.Truncated).To(BeTrue(),
					"adapter should request a full resync when keep_note_client_id is empty and the LIST node didn't appear in the incremental pull")
			})
		})

		When("the binding has an empty remote_handle (broken-migration case)", func() {
			// Self-heal must NOT trigger Truncated=true when remote_handle
			// is empty — the binding is fundamentally broken (legacy
			// keep_note_id alias was never translated to remote_handle by
			// the buggy Phase 7 migration). Production 2026-05-07: the
			// ingredients-on-hand binding had remote_handle="" and the
			// initial self-heal added Truncated=true on every tick,
			// causing a perpetual ForceFullResync loop that hammered
			// Keep's API without making progress.
			BeforeEach(func() {
				binding.RemoteHandle = ""
				binding.AdapterState = connectors.AdapterState{
					googlekeep.AdapterStateKeyKeepNoteClientID: "",
				}
				fakeClient.changesDefault = gateway.ChangesResponse{ToVersion: "v13"}
				result, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should not error", func() {
				Expect(pullErr).NotTo(HaveOccurred())
			})

			It("should NOT set Truncated=true (self-heal can't help; need re-bind)", func() {
				Expect(result.Truncated).To(BeFalse(),
					"self-heal must bail on empty remote_handle to avoid perpetual ForceFullResync loop")
			})
		})

		When("the binding already has a populated keep_note_client_id", func() {
			BeforeEach(func() {
				binding.AdapterState = connectors.AdapterState{
					googlekeep.AdapterStateKeyKeepNoteClientID: "preexisting-client-id",
				}
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v12",
					// LIST node appears with a different client id (would
					// be unusual in practice, but tests the no-clobber
					// semantic).
					Nodes: []gateway.Node{
						{
							ID:       "different-client-id",
							ServerID: remoteHandle,
							Type:     gateway.NodeTypeList,
							Title:    "Groceries",
						},
					},
				}
				_, pullErr = adapter.PullRemote(ctx, binding)
			})

			It("should NOT clobber the existing keep_note_client_id", func() {
				Expect(binding.AdapterState[googlekeep.AdapterStateKeyKeepNoteClientID]).
					To(Equal("preexisting-client-id"))
			})
		})
	})

	Describe("InsertRemote", func() {
		var (
			binding connectors.Binding
			ref     connectors.RemoteRef
			err     error
		)

		BeforeEach(func() {
			binding = connectors.Binding{
				ProfileID: profile, RemoteHandle: remoteHandle,
			}
			fakeClient.changesDefault = gateway.ChangesResponse{
				ToVersion: "v50",
				// The fake echoes the inserted node back with a server id;
				// we use a closure-style stub here via changesResponses.
			}
			// We need to populate the response based on the request's
			// node, but the simple fake doesn't have request awareness.
			// Use changesResponses instead.
			fakeClient.changesResponses = []gateway.ChangesResponse{
				{
					ToVersion: "v50",
					Nodes: []gateway.Node{
						{
							// ID here will be matched against the request's
							// item id; tests below assert the wire shape.
							Type:     gateway.NodeTypeListItem,
							ServerID: "newly-assigned-id",
							ParentID: remoteHandle,
							Text:     "milk",
						},
					},
				},
			}
			item := connectors.WikiItem{
				UID:  "u-1",
				Text: "milk",
			}
			ref, err = adapter.InsertRemote(ctx, binding, item)
		})

		When("the response does not echo the client id", func() {
			It("should error with protocol drift", func() {
				Expect(err).To(MatchError(ContainSubstring("did not echo our client id")))
				Expect(ref).To(Equal(connectors.RemoteRef("")))
			})
		})

		When("the response echoes the client id", func() {
			var (
				binding2 connectors.Binding
				ref2     connectors.RemoteRef
				err2     error
			)

			BeforeEach(func() {
				binding2 = connectors.Binding{
					ProfileID: profile, RemoteHandle: remoteHandle,
				}
				// Pre-compute the deterministic client id the adapter
				// will mint for uid "u-1" at the test clock's instant.
				expected := mustClientID(clock.now, "u-1")
				fakeClient.changesResponses = []gateway.ChangesResponse{
					{
						ToVersion: "v60",
						Nodes: []gateway.Node{
							{
								ID:       expected,
								ServerID: "srv-new-id",
								Type:     gateway.NodeTypeListItem,
								ParentID: remoteHandle,
								Text:     "milk",
							},
						},
					},
				}
				item := connectors.WikiItem{UID: "u-1", Text: "milk"}
				// Reset call log before second insert.
				fakeClient.changes = nil
				ref2, err2 = adapter.InsertRemote(ctx, binding2, item)
			})

			It("should not error", func() {
				Expect(err2).NotTo(HaveOccurred())
			})

			It("should return the server-assigned ref", func() {
				Expect(ref2).To(Equal(connectors.RemoteRef("srv-new-id")))
			})

			It("should send a single-node changes request", func() {
				Expect(fakeClient.changes).To(HaveLen(1))
				Expect(fakeClient.changes[0].Nodes).To(HaveLen(1))
				Expect(fakeClient.changes[0].Nodes[0].Type).To(Equal(gateway.NodeTypeListItem))
				Expect(fakeClient.changes[0].Nodes[0].ParentID).To(Equal(remoteHandle))
			})
		})
	})

	Describe("PatchRemote", func() {
		var (
			binding connectors.Binding
			ref     connectors.RemoteRef
			err     error
		)

		BeforeEach(func() {
			binding = connectors.Binding{
				ProfileID: profile, RemoteHandle: remoteHandle,
				AdapterState: connectors.AdapterState{
					googlekeep.AdapterStateKeyItemMapping: map[string]any{
						"srv-9": map[string]any{
							"server_id":    "srv-9",
							"base_version": "bv-9",
							"client_id":    "cli-9",
						},
					},
				},
			}
			fakeClient.changesResponses = []gateway.ChangesResponse{
				{
					ToVersion: "v80",
					WriteResults: []gateway.NodeWriteResult{
						{ID: "cli-9", Status: "SUCCESS"},
					},
				},
			}
			item := connectors.WikiItem{UID: "u-9", Text: "updated text", Checked: true}
			ref, err = adapter.PatchRemote(ctx, binding, "srv-9", item)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the same ref", func() {
			Expect(ref).To(Equal(connectors.RemoteRef("srv-9")))
		})

		It("should send the persisted base_version + client_id on the wire", func() {
			Expect(fakeClient.changes).To(HaveLen(1))
			Expect(fakeClient.changes[0].Nodes).To(HaveLen(1))
			node := fakeClient.changes[0].Nodes[0]
			Expect(node.ID).To(Equal("cli-9"))
			Expect(node.ServerID).To(Equal("srv-9"))
			Expect(node.BaseVersion).To(Equal("bv-9"))
			Expect(node.Checked).To(BeTrue())
		})
	})

	Describe("DeleteRemote", func() {
		var (
			binding connectors.Binding
			err     error
		)

		BeforeEach(func() {
			binding = connectors.Binding{
				ProfileID: profile, RemoteHandle: remoteHandle,
				AdapterState: connectors.AdapterState{
					googlekeep.AdapterStateKeyItemMapping: map[string]any{
						"srv-7": map[string]any{
							"server_id":    "srv-7",
							"base_version": "bv-7",
							"client_id":    "cli-7",
						},
					},
				},
			}
			fakeClient.changesResponses = []gateway.ChangesResponse{
				{
					ToVersion: "v90",
					WriteResults: []gateway.NodeWriteResult{
						{ID: "cli-7", Status: "SUCCESS"},
					},
				},
			}
			err = adapter.DeleteRemote(ctx, binding, "srv-7")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should send a tombstone (Deleted timestamp) for the item", func() {
			// Production bug 2026-05-07: setting only Trashed caused
			// Keep to apply other fields (checked=false via omitempty)
			// WITHOUT actually deleting — items appeared unchecked in
			// Keep instead of being removed. The legacy connector used
			// Deleted timestamp, with the explicit note: "only `deleted`
			// makes it through Keep's Changes API on incremental updates."
			Expect(fakeClient.changes).To(HaveLen(1))
			Expect(fakeClient.changes[0].Nodes).To(HaveLen(1))
			node := fakeClient.changes[0].Nodes[0]
			Expect(node.ServerID).To(Equal("srv-7"))
			Expect(node.Timestamps.Deleted.IsZero()).To(BeFalse(),
				"Keep's Changes API requires Deleted timestamp (not just Trashed) for incremental updates")
		})
	})

	Describe("SyncCollectionState", func() {
		// Restored from legacy keepsync (Phase 5-A port regression
		// 2026-05-07): hashtag-derived label sync to Keep.
		var (
			binding connectors.Binding
			err     error
		)

		When("wiki items carry hashtags not yet mapped to Keep label MainIDs", func() {
			BeforeEach(func() {
				binding = connectors.Binding{
					ProfileID: profile, RemoteHandle: remoteHandle,
					AdapterState: connectors.AdapterState{
						googlekeep.AdapterStateKeyKeepNoteClientID: "list-cli",
						googlekeep.AdapterStateKeyLabelIDs:         map[string]any{},
					},
				}
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v200",
				}
				items := []connectors.WikiItem{
					{UID: "uid-1", Tags: []string{"household"}},
					{UID: "uid-2", Tags: []string{"household", "chores"}},
				}
				binding, err = adapter.SyncCollectionState(ctx, binding, items)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should send exactly one Changes request with the new labels", func() {
				Expect(fakeClient.changes).To(HaveLen(1))
				Expect(fakeClient.changes[0].Labels).To(HaveLen(2))
			})

			It("should include the LIST node carrying the new label IDs", func() {
				Expect(fakeClient.changes[0].Nodes).To(HaveLen(1))
				Expect(fakeClient.changes[0].Nodes[0].Type).To(Equal(gateway.NodeTypeList))
				Expect(fakeClient.changes[0].Nodes[0].LabelIDs).To(HaveLen(2))
			})

			It("should persist the new label MainIDs in adapter_state.label_ids", func() {
				labelIDs, ok := binding.AdapterState[googlekeep.AdapterStateKeyLabelIDs].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(labelIDs).To(HaveKey("household"))
				Expect(labelIDs).To(HaveKey("chores"))
			})
		})

		When("all wiki tags already map to existing Keep labels", func() {
			BeforeEach(func() {
				binding = connectors.Binding{
					ProfileID: profile, RemoteHandle: remoteHandle,
					AdapterState: connectors.AdapterState{
						googlekeep.AdapterStateKeyKeepNoteClientID: "list-cli",
						googlekeep.AdapterStateKeyLabelIDs: map[string]any{
							"household": "main-household",
							"chores":    "main-chores",
						},
					},
				}
				items := []connectors.WikiItem{
					{UID: "uid-1", Tags: []string{"household", "chores"}},
				}
				binding, err = adapter.SyncCollectionState(ctx, binding, items)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not send any Changes request (idempotent no-op)", func() {
				Expect(fakeClient.changes).To(BeEmpty())
			})
		})

		When("the binding has no keep_note_client_id yet (pre-self-heal)", func() {
			BeforeEach(func() {
				binding = connectors.Binding{
					ProfileID: profile, RemoteHandle: remoteHandle,
					AdapterState: connectors.AdapterState{
						googlekeep.AdapterStateKeyKeepNoteClientID: "",
						googlekeep.AdapterStateKeyLabelIDs:         map[string]any{},
					},
				}
				items := []connectors.WikiItem{
					{UID: "uid-1", Tags: []string{"household"}},
				}
				binding, err = adapter.SyncCollectionState(ctx, binding, items)
			})

			It("should not error (defer the push until self-heal lands)", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not send any Changes request (would 500 on stage3 invariant)", func() {
				Expect(fakeClient.changes).To(BeEmpty())
			})
		})

		When("wiki items have no tags", func() {
			BeforeEach(func() {
				binding = connectors.Binding{
					ProfileID: profile, RemoteHandle: remoteHandle,
					AdapterState: connectors.AdapterState{
						googlekeep.AdapterStateKeyKeepNoteClientID: "list-cli",
						googlekeep.AdapterStateKeyLabelIDs:         map[string]any{},
					},
				}
				items := []connectors.WikiItem{
					{UID: "uid-1", Tags: []string{}},
				}
				binding, err = adapter.SyncCollectionState(ctx, binding, items)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not send any Changes request", func() {
				Expect(fakeClient.changes).To(BeEmpty())
			})
		})
	})

	Describe("RemoteToWiki", func() {
		var (
			remote connectors.RemoteItem
			wiki   connectors.WikiItem
			err    error
		)

		BeforeEach(func() {
			remote = connectors.RemoteItem{
				Ref:      "srv-1",
				Title:    "milk #shopping",
				Status:   "completed",
				Position: "1000",
				Vendor: map[string]any{
					"checked":   true,
					"node_text": "milk #shopping",
					"sort_value": "1000",
				},
			}
			wiki, err = adapter.RemoteToWiki(remote)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should split tags out of the head line", func() {
			Expect(wiki.Text).To(Equal("milk"))
			Expect(wiki.Tags).To(ContainElement("shopping"))
		})

		It("should map checked flag", func() {
			Expect(wiki.Checked).To(BeTrue())
		})

		It("should parse SortOrder from the wire SortValue", func() {
			Expect(wiki.SortOrder).To(Equal(int64(1000)))
		})
	})

	Describe("WikiToRemote", func() {
		var (
			remote connectors.RemoteItem
			err    error
		)

		BeforeEach(func() {
			wiki := connectors.WikiItem{
				UID: "u-1", Text: "milk", Tags: []string{"shopping"}, Checked: true,
				SortOrder: 5000,
			}
			remote, err = adapter.WikiToRemote(wiki)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should encode tags into the title head", func() {
			Expect(remote.Title).To(Equal("milk #shopping"))
		})

		It("should pass the sort order through Position", func() {
			Expect(remote.Position).To(Equal("5000"))
		})
	})

	Describe("AdvanceCursor", func() {
		When("NewCursor is a non-empty string", func() {
			var (
				binding connectors.Binding
				updated connectors.Binding
			)

			BeforeEach(func() {
				binding = connectors.Binding{
					ProfileID: profile, RemoteHandle: remoteHandle,
					AdapterState: connectors.AdapterState{},
				}
				updated = adapter.AdvanceCursor(binding, connectors.RemotePullResult{NewCursor: "v500"})
			})

			It("should write to AdapterState", func() {
				Expect(updated.AdapterState[googlekeep.AdapterStateKeyKeepCursor]).To(Equal("v500"))
			})
		})

		When("NewCursor is empty", func() {
			var (
				binding connectors.Binding
				updated connectors.Binding
			)

			BeforeEach(func() {
				binding = connectors.Binding{
					ProfileID: profile, RemoteHandle: remoteHandle,
					AdapterState: connectors.AdapterState{
						googlekeep.AdapterStateKeyKeepCursor: "v100",
					},
				}
				updated = adapter.AdvanceCursor(binding, connectors.RemotePullResult{NewCursor: ""})
			})

			It("should leave AdapterState unchanged", func() {
				Expect(updated.AdapterState[googlekeep.AdapterStateKeyKeepCursor]).To(Equal("v100"))
			})
		})

		When("NewCursor is a non-string type", func() {
			var (
				binding connectors.Binding
				updated connectors.Binding
			)

			BeforeEach(func() {
				binding = connectors.Binding{
					ProfileID: profile, RemoteHandle: remoteHandle,
					AdapterState: connectors.AdapterState{},
				}
				updated = adapter.AdvanceCursor(binding, connectors.RemotePullResult{NewCursor: 42})
			})

			It("should leave AdapterState unchanged", func() {
				_, ok := updated.AdapterState[googlekeep.AdapterStateKeyKeepCursor]
				Expect(ok).To(BeFalse())
			})
		})
	})

	Describe("SeedBindingState", func() {
		var (
			state connectors.AdapterState
			err   error
		)

		BeforeEach(func() {
			fakeClient.changesDefault = gateway.ChangesResponse{
				ToVersion: "v0",
				Nodes: []gateway.Node{
					{
						ID:       "client-list",
						ServerID: remoteHandle,
						Type:     gateway.NodeTypeList,
						Title:    "Groceries",
					},
					{
						ID:          "cli-1",
						ServerID:    "srv-1",
						Type:        gateway.NodeTypeListItem,
						ParentID:    remoteHandle,
						BaseVersion: "bv-1",
						Text:        "milk",
					},
					{
						ID:          "cli-2",
						ServerID:    "srv-2",
						Type:        gateway.NodeTypeListItem,
						ParentServerID: remoteHandle,
						BaseVersion: "bv-2",
						Text:        "eggs",
					},
				},
				Labels: []gateway.LabelEntry{
					{MainID: "label-x", Name: "shopping"},
				},
			}
			state, err = adapter.SeedBindingState(ctx, profile, remoteHandle, nil)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should record per-server-id mapping fields", func() {
			mapping, ok := state[googlekeep.AdapterStateKeyItemMapping].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(mapping).To(HaveKey("srv-1"))
			Expect(mapping).To(HaveKey("srv-2"))
		})

		It("should record the LIST node's client id", func() {
			Expect(state[googlekeep.AdapterStateKeyKeepNoteClientID]).To(Equal("client-list"))
		})

		It("should index labels by name", func() {
			labels, ok := state[googlekeep.AdapterStateKeyLabelIDs].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(labels).To(HaveKeyWithValue("shopping", "label-x"))
		})

		// Architectural fix 2026-05-08: bind-time alignment populates
		// item_id_map by matching wiki uids to pulled Keep items via
		// text. Without this, the first reconcile after Bind treated
		// every Keep item as "unknown" (Keep has no wiki-uid marker)
		// and either created duplicates (legacy) or relied on the
		// engine's applyInbound dedup-by-text (downstream).
		When("wikiItems are supplied with text matching some pulled items", func() {
			var (
				richState connectors.AdapterState
				richErr   error
			)

			BeforeEach(func() {
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v-rich",
					Nodes: []gateway.Node{
						{ServerID: remoteHandle, Type: gateway.NodeTypeList},
						{ID: "cli-A", ServerID: "srv-A", Type: gateway.NodeTypeListItem, ParentID: remoteHandle, Text: "milk"},
						{ID: "cli-B", ServerID: "srv-B", Type: gateway.NodeTypeListItem, ParentID: remoteHandle, Text: "eggs"},
						{ID: "cli-C", ServerID: "srv-C", Type: gateway.NodeTypeListItem, ParentID: remoteHandle, Text: "orphan-no-wiki-match"},
					},
				}
				richErr = nil
				richState, richErr = adapter.SeedBindingState(ctx, profile, remoteHandle, []connectors.WikiItem{
					{UID: "uid-A", Text: "milk"},
					{UID: "uid-B", Text: "eggs"},
					{UID: "uid-C", Text: "no-keep-match"},
				})
			})

			It("should not error", func() {
				Expect(richErr).NotTo(HaveOccurred())
			})

			It("should populate item_id_map for wiki uids that text-match a pulled item", func() {
				idMap, ok := richState["item_id_map"].(map[string]any)
				Expect(ok).To(BeTrue(),
					"SeedBindingState did not populate item_id_map for bind-time alignment — production regression 2026-05-08")
				Expect(idMap).To(HaveKeyWithValue("uid-A", "srv-A"))
				Expect(idMap).To(HaveKeyWithValue("uid-B", "srv-B"))
			})

			It("should NOT invent idMap entries for wiki uids without a Keep match", func() {
				idMap, _ := richState["item_id_map"].(map[string]any)
				Expect(idMap).NotTo(HaveKey("uid-C"))
			})
		})

		It("should record the to_version cursor", func() {
			Expect(state[googlekeep.AdapterStateKeyKeepCursor]).To(Equal("v0"))
		})
	})

	Describe("ValidateRemoteBinding", func() {
		When("remote_handle is empty", func() {
			It("should error", func() {
				err := adapter.ValidateRemoteBinding(ctx, profile, "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("remote_handle must not be empty"))
			})
		})

		When("the note exists and is a LIST node", func() {
			var err error

			BeforeEach(func() {
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v0",
					Nodes: []gateway.Node{
						{ServerID: remoteHandle, Type: gateway.NodeTypeList, Title: "Groceries"},
					},
				}
				err = adapter.ValidateRemoteBinding(ctx, profile, remoteHandle)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the note exists but is not a LIST node", func() {
			var err error

			BeforeEach(func() {
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v0",
					Nodes: []gateway.Node{
						{ServerID: remoteHandle, Type: gateway.NodeTypeNote, Title: "free-form"},
					},
				}
				err = adapter.ValidateRemoteBinding(ctx, profile, remoteHandle)
			})

			It("should return ErrKeepNoteNotAList", func() {
				Expect(err).To(MatchError(googlekeep.ErrKeepNoteNotAList))
			})
		})

		When("the note is not present in the user's account", func() {
			var err error

			BeforeEach(func() {
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v0",
					Nodes: []gateway.Node{
						{ServerID: "other-list", Type: gateway.NodeTypeList},
					},
				}
				err = adapter.ValidateRemoteBinding(ctx, profile, remoteHandle)
			})

			It("should error with bound-note-deleted", func() {
				Expect(err).To(MatchError(gateway.ErrBoundNoteDeleted))
			})
		})

		When("the note is trashed", func() {
			var err error

			BeforeEach(func() {
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v0",
					Nodes: []gateway.Node{
						{
							ServerID: remoteHandle,
							Type:     gateway.NodeTypeList,
							Timestamps: gateway.Timestamps{
								Trashed: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
							},
						},
					},
				}
				err = adapter.ValidateRemoteBinding(ctx, profile, remoteHandle)
			})

			It("should error with bound-note-deleted", func() {
				Expect(err).To(MatchError(gateway.ErrBoundNoteDeleted))
			})
		})
	})

	Describe("RebuildAdapterState", func() {
		var (
			binding connectors.Binding
			state   connectors.AdapterState
			err     error
		)

		BeforeEach(func() {
			binding = connectors.Binding{
				ProfileID: profile, RemoteHandle: remoteHandle,
				AdapterState: connectors.AdapterState{
					googlekeep.AdapterStateKeyKeepCursor: "v-stale",
				},
			}
			fakeClient.changesDefault = gateway.ChangesResponse{
				ToVersion: "v-fresh",
				Nodes: []gateway.Node{
					{ServerID: remoteHandle, Type: gateway.NodeTypeList},
					{
						ID:          "cli-x",
						ServerID:    "srv-x",
						Type:        gateway.NodeTypeListItem,
						ParentID:    remoteHandle,
						BaseVersion: "bv-x",
					},
				},
			}
			state, err = adapter.RebuildAdapterState(ctx, binding)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reset the cursor to empty", func() {
			Expect(state[googlekeep.AdapterStateKeyKeepCursor]).To(Equal(""))
		})

		It("should rebuild the per-server-id mapping", func() {
			mapping, ok := state[googlekeep.AdapterStateKeyItemMapping].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(mapping).To(HaveKey("srv-x"))
		})

		// Architectural fix 2026-05-08 (user-approved): the legacy
		// rebuild call wiped item_id_map (the wiki-uid → server-id
		// map). On the next reconcile the engine treated every wiki
		// item as new and re-Inserted them — duplicates at Keep.
		// Fix: preserve idMap entries whose refs still appear in the
		// rebuilt item_mapping.
		When("the binding has an existing item_id_map", func() {
			var (
				preservedState connectors.AdapterState
				preservedErr   error
			)

			BeforeEach(func() {
				preservedBinding := connectors.Binding{
					ProfileID: profile, RemoteHandle: remoteHandle,
					AdapterState: connectors.AdapterState{
						googlekeep.AdapterStateKeyKeepCursor: "v-stale",
						"item_id_map": map[string]string{
							"uid-still-here": "srv-x",       // ref still in rebuilt mapping → preserved
							"uid-gone":       "srv-deleted", // ref no longer in remote → dropped
						},
					},
				}
				preservedState, preservedErr = adapter.RebuildAdapterState(ctx, preservedBinding)
			})

			It("should not error", func() {
				Expect(preservedErr).NotTo(HaveOccurred())
			})

			It("should preserve item_id_map entries whose refs still exist in the rebuilt mapping", func() {
				idMap, ok := preservedState["item_id_map"].(map[string]any)
				Expect(ok).To(BeTrue(),
					"engine wiped item_id_map on rebuild — duplicate-Insert hazard, production regression 2026-05-08")
				Expect(idMap).To(HaveKeyWithValue("uid-still-here", "srv-x"))
			})

			It("should drop item_id_map entries whose refs are no longer in the remote", func() {
				idMap, _ := preservedState["item_id_map"].(map[string]any)
				Expect(idMap).NotTo(HaveKey("uid-gone"),
					"rebuild kept stale uid → server-id mapping; would route through PATCH against a deleted ref")
			})
		})
	})

	Describe("FetchRemoteListTitle", func() {
		var (
			title string
			ok    bool
			err   error
		)

		When("the LIST node exists", func() {
			BeforeEach(func() {
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v0",
					Nodes: []gateway.Node{
						{ServerID: remoteHandle, Type: gateway.NodeTypeList, Title: "Groceries"},
					},
				}
				title, ok, err = adapter.FetchRemoteListTitle(ctx, profile, remoteHandle)
			})

			It("should return the title and ok=true", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(title).To(Equal("Groceries"))
				Expect(ok).To(BeTrue())
			})
		})

		When("the LIST node does not exist", func() {
			BeforeEach(func() {
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v0",
					Nodes: []gateway.Node{
						{ServerID: "other-list", Type: gateway.NodeTypeList},
					},
				}
				title, ok, err = adapter.FetchRemoteListTitle(ctx, profile, remoteHandle)
			})

			It("should return ok=false silently", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).To(BeFalse())
			})
		})

		When("remote_handle is empty", func() {
			BeforeEach(func() {
				title, ok, err = adapter.FetchRemoteListTitle(ctx, profile, "")
			})

			It("should return ok=false silently", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).To(BeFalse())
				Expect(title).To(BeEmpty())
			})
		})
	})

	Describe("ListRemoteCollections", func() {
		var (
			collections []connectors.RemoteCollection
			err         error
		)

		BeforeEach(func() {
			fakeClient.changesDefault = gateway.ChangesResponse{
				ToVersion: "v0",
				Nodes: []gateway.Node{
					{ServerID: "list-1", Type: gateway.NodeTypeList, Title: "Groceries"},
					{ServerID: "list-2", Type: gateway.NodeTypeList, Title: "Travel"},
					{ServerID: "list-3", Type: gateway.NodeTypeList, Title: "Trashed", Timestamps: gateway.Timestamps{Trashed: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
					{ServerID: "note-1", Type: gateway.NodeTypeNote, Title: "Free-form"},
				},
			}
			collections, err = adapter.ListRemoteCollections(ctx, profile)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return only LIST nodes that aren't trashed", func() {
			handles := make([]string, 0, len(collections))
			for _, c := range collections {
				handles = append(handles, c.Handle)
			}
			Expect(handles).To(ConsistOf("list-1", "list-2"))
		})

		It("should report HasSubtasks=false on every collection", func() {
			for _, c := range collections {
				Expect(c.Capabilities.HasSubtasks).To(BeFalse())
			}
		})
	})

	Describe("EncodeAdapterState / DecodeAdapterState", func() {
		When("encoding a populated state", func() {
			It("should round-trip the keys", func() {
				input := connectors.AdapterState{
					googlekeep.AdapterStateKeyItemMapping: map[string]any{
						"srv-1": map[string]any{"server_id": "srv-1", "client_id": "cli-1"},
					},
					googlekeep.AdapterStateKeyKeepCursor: "v9",
				}
				encoded, err := adapter.EncodeAdapterState(input)
				Expect(err).NotTo(HaveOccurred())
				decoded, err := adapter.DecodeAdapterState(encoded)
				Expect(err).NotTo(HaveOccurred())
				Expect(decoded[googlekeep.AdapterStateKeyKeepCursor]).To(Equal("v9"))
			})
		})

		When("encoding nil", func() {
			It("should produce an envelope with all required keys", func() {
				encoded, err := adapter.EncodeAdapterState(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(encoded).To(HaveKey(googlekeep.AdapterStateKeyItemMapping))
				Expect(encoded).To(HaveKey(googlekeep.AdapterStateKeyKeepCursor))
				Expect(encoded).To(HaveKey(googlekeep.AdapterStateKeyLabelIDs))
				Expect(encoded).To(HaveKey(googlekeep.AdapterStateKeyKeepNoteClientID))
			})
		})
	})

	Describe("ReadRemoteByRef", func() {
		var (
			binding connectors.Binding
			remote  connectors.RemoteItem
			err     error
		)

		BeforeEach(func() {
			binding = connectors.Binding{
				ProfileID: profile, RemoteHandle: remoteHandle,
			}
		})

		When("the item is present in the pull", func() {
			BeforeEach(func() {
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v0",
					Nodes: []gateway.Node{
						{ID: "cli-9", ServerID: "srv-9", Type: gateway.NodeTypeListItem, ParentID: remoteHandle, Text: "milk"},
					},
				}
				remote, err = adapter.ReadRemoteByRef(ctx, binding, "srv-9")
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should populate the RemoteItem", func() {
				Expect(remote.Ref).To(Equal(connectors.RemoteRef("srv-9")))
				Expect(remote.Deleted).To(BeFalse())
			})
		})

		When("the item is trashed", func() {
			BeforeEach(func() {
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v0",
					Nodes: []gateway.Node{
						{
							ID: "cli-9", ServerID: "srv-9",
							Type: gateway.NodeTypeListItem, ParentID: remoteHandle,
							Timestamps: gateway.Timestamps{Trashed: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
						},
					},
				}
				remote, err = adapter.ReadRemoteByRef(ctx, binding, "srv-9")
			})

			It("should report Deleted=true with no error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(remote.Deleted).To(BeTrue())
			})
		})

		When("the item is not in the pull at all", func() {
			BeforeEach(func() {
				fakeClient.changesDefault = gateway.ChangesResponse{
					ToVersion: "v0",
					Nodes: []gateway.Node{
						{ID: "cli-other", ServerID: "srv-other", Type: gateway.NodeTypeListItem, ParentID: remoteHandle},
					},
				}
				remote, err = adapter.ReadRemoteByRef(ctx, binding, "srv-9")
			})

			It("should report Deleted=true with no error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(remote.Deleted).To(BeTrue())
			})
		})

		When("the gateway returns ErrBoundNoteDeleted", func() {
			BeforeEach(func() {
				fakeClient.changesErr = gateway.ErrBoundNoteDeleted
				remote, err = adapter.ReadRemoteByRef(ctx, binding, "srv-9")
			})

			It("should report Deleted=true with no error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(remote.Deleted).To(BeTrue())
			})
		})
	})

	Describe("ClassifyError", func() {
		It("should map ErrCredentialMissing → ErrorClassAuthFailed", func() {
			Expect(adapter.ClassifyError(googlekeep.ErrCredentialMissing)).To(Equal(connectors.ErrorClassAuthFailed))
		})

		It("should map ErrInvalidCredentials → ErrorClassAuthFailed", func() {
			Expect(adapter.ClassifyError(gateway.ErrInvalidCredentials)).To(Equal(connectors.ErrorClassAuthFailed))
		})

		It("should map ErrAuthRevoked → ErrorClassAuthFailed", func() {
			Expect(adapter.ClassifyError(gateway.ErrAuthRevoked)).To(Equal(connectors.ErrorClassAuthFailed))
		})

		It("should map ErrProtocolDrift → ErrorClassPreconditionFailed (stage3-500 recovery path)", func() {
			Expect(adapter.ClassifyError(gateway.ErrProtocolDrift)).To(Equal(connectors.ErrorClassPreconditionFailed))
		})

		It("should map ErrRateLimited → ErrorClassRateLimited", func() {
			Expect(adapter.ClassifyError(gateway.ErrRateLimited)).To(Equal(connectors.ErrorClassRateLimited))
		})

		It("should map ErrBoundNoteDeleted → ErrorClassNotFound", func() {
			Expect(adapter.ClassifyError(gateway.ErrBoundNoteDeleted)).To(Equal(connectors.ErrorClassNotFound))
		})

		It("should map ErrServiceDisabled → ErrorClassFatal", func() {
			Expect(adapter.ClassifyError(gateway.ErrServiceDisabled)).To(Equal(connectors.ErrorClassFatal))
		})

		It("should map ErrPermissionDenied → ErrorClassFatal", func() {
			Expect(adapter.ClassifyError(gateway.ErrPermissionDenied)).To(Equal(connectors.ErrorClassFatal))
		})

		It("should map nil → ErrorClassNone", func() {
			Expect(adapter.ClassifyError(nil)).To(Equal(connectors.ErrorClassNone))
		})

		It("should map unknown errors to ErrorClassRetryable", func() {
			Expect(adapter.ClassifyError(errors.New("some random error"))).To(Equal(connectors.ErrorClassRetryable))
		})
	})

	Describe("compile-time contract", func() {
		It("should satisfy connectors.BackendAdapter", func() {
			var _ connectors.BackendAdapter = adapter
		})
	})
})

var _ = Describe("FrontmatterCredentialStore", func() {
	var (
		ctx    context.Context
		fmRead *fakeFrontmatterReadWriter
		store  *googlekeep.FrontmatterCredentialStore
		pid    wikipage.PageIdentifier
		clock  *fixedClock
	)

	BeforeEach(func() {
		ctx = context.Background()
		fmRead = newFakeFrontmatterReadWriter()
		clock = &fixedClock{now: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)}
		var err error
		store, err = googlekeep.NewFrontmatterCredentialStore(
			fmRead,
			clock,
			silentLogger{},
			nil, nil,
		)
		Expect(err).NotTo(HaveOccurred())
		pid = wikipage.PageIdentifier("profile_alice")
	})

	When("the profile page does not exist", func() {
		It("should return ErrCredentialMissing on LoadMasterToken", func() {
			_, err := store.LoadMasterToken(ctx, pid)
			Expect(err).To(MatchError(googlekeep.ErrCredentialMissing))
		})
	})

	When("the page exists but has no Keep frontmatter", func() {
		BeforeEach(func() {
			fmRead.pages[pid] = wikipage.FrontMatter{}
		})

		It("should return ErrCredentialMissing on LoadMasterToken", func() {
			_, err := store.LoadMasterToken(ctx, pid)
			Expect(err).To(MatchError(googlekeep.ErrCredentialMissing))
		})
	})

	When("the master_token is non-empty", func() {
		BeforeEach(func() {
			fmRead.pages[pid] = wikipage.FrontMatter{
				"wiki": map[string]any{
					"connectors": map[string]any{
						"google_keep": map[string]any{
							"master_token": "mt-real",
							"email":        "u@example.com",
							"android_id":   "fixed-device-id",
						},
					},
				},
			}
		})

		It("should return the token and email", func() {
			bundle, err := store.LoadMasterToken(ctx, pid)
			Expect(err).NotTo(HaveOccurred())
			Expect(bundle.MasterToken).To(Equal("mt-real"))
			Expect(bundle.Email).To(Equal("u@example.com"))
			Expect(bundle.AndroidID).To(Equal("fixed-device-id"))
		})
	})

	When("PersistMasterToken is called for a fresh profile", func() {
		var err error

		BeforeEach(func() {
			err = store.PersistMasterToken(ctx, pid, "mt-fresh", "device-1", "u@example.com")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should write the bundle into the profile frontmatter", func() {
			bundle, loadErr := store.LoadMasterToken(ctx, pid)
			Expect(loadErr).NotTo(HaveOccurred())
			Expect(bundle.MasterToken).To(Equal("mt-fresh"))
			Expect(bundle.Email).To(Equal("u@example.com"))
			Expect(bundle.AndroidID).To(Equal("device-1"))
		})

		It("should stamp connected_at + last_verified_at", func() {
			bundle, err := store.LoadCredentials(ctx, pid)
			Expect(err).NotTo(HaveOccurred())
			Expect(bundle.ConnectedAt).To(Equal(clock.now))
			Expect(bundle.LastVerifiedAt).To(Equal(clock.now))
		})
	})

	When("ClearCredentials is called for a configured profile", func() {
		var bundle googlekeep.CredentialBundle
		var err error

		BeforeEach(func() {
			Expect(store.PersistMasterToken(ctx, pid, "mt-fresh", "device-1", "u@example.com")).To(Succeed())
			bundle, err = store.ClearCredentials(ctx, pid)
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should report the bundle as not configured post-clear", func() {
			Expect(bundle.IsConfigured()).To(BeFalse())
		})

		It("should preserve the android_id across Disconnect", func() {
			postClear, loadErr := store.LoadCredentials(ctx, pid)
			Expect(loadErr).NotTo(HaveOccurred())
			Expect(postClear.AndroidID).To(Equal("device-1"))
		})
	})
})

// mustClientID computes the deterministic client id the adapter mints
// for a given (now, salt) pair. Mirrors the adapter-internal
// buildKeepItemID helper (sha256(salt)[:8] hex suffix).
func mustClientID(now time.Time, salt string) string {
	sum := sha256.Sum256([]byte(salt))
	return fmt.Sprintf("%x.%s", now.UnixMilli(), hex.EncodeToString(sum[:8]))
}
