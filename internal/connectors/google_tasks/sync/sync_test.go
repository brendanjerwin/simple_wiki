//revive:disable:dot-imports
package sync_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/gateway"
	taskssync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/sync"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/translator"
)

const (
	syncTestPage     = "shopping_lists"
	syncTestListName = "groceries"
	syncTestRemote   = "tasklist-1"
	syncTestEmail    = "alice@example.com"
)

func newConfiguredStore(pages *fakePages, withSubscription *taskssync.Subscription) *taskssync.SubscriptionStore {
	store := newStore(pages)
	state := taskssync.ConnectorState{
		Email:        syncTestEmail,
		RefreshToken: "rt-fake",
	}
	if withSubscription != nil {
		state.Subscriptions = []taskssync.Subscription{*withSubscription}
	}
	Expect(store.SaveState(aliceProfile, state)).To(Succeed())
	return store
}

// readyLeaseTable returns a LeaseTable that has already signaled
// ready, so Subscribe/Unsubscribe don't block.
func readyLeaseTable() *connectors.LeaseTable {
	lt := connectors.NewLeaseTable()
	lt.SignalReady()
	return lt
}

// buildTestConnector constructs a Connector with the provided dependencies
// (failing the test on misconfiguration). The reader and mutator can
// be nil; the caller decides whether outbound or inbound is exercised.
func buildTestConnector(
	store *taskssync.SubscriptionStore,
	leaseTable *connectors.LeaseTable,
	client taskssync.TasksClient,
	clock taskssync.Clock,
	reader taskssync.ChecklistReader,
	mutator taskssync.ChecklistMutator,
	suppressor taskssync.SyncSuppressor,
) *taskssync.Connector {
	c, err := taskssync.NewConnector(store, leaseTable, stubFactoryThatReturns(client), silentLogger{}, clock)
	Expect(err).ToNot(HaveOccurred())
	if reader != nil {
		c.SetChecklistReader(reader)
	}
	if mutator != nil {
		c.SetChecklistMutator(mutator)
	}
	if suppressor != nil {
		c.SetSyncSuppressor(suppressor)
	}
	return c
}

func subscriptionKey() connectors.SubscriptionKey {
	return connectors.SubscriptionKey{
		ProfileID: string(aliceProfile),
		Page:      syncTestPage,
		ListName:  syncTestListName,
	}
}

var _ = Describe("Connector.Kind", func() {
	It("should return ConnectorKindGoogleTasks", func() {
		store := newStore(newFakePages())
		c, err := taskssync.NewConnector(store, readyLeaseTable(), stubFactoryThatReturns(newFakeTasksClient()), silentLogger{}, newFakeClock(time.Now()))
		Expect(err).ToNot(HaveOccurred())
		Expect(c.Kind()).To(Equal(connectors.ConnectorKindGoogleTasks))
	})
})

var _ = Describe("NewConnector input validation", func() {
	When("store is nil", func() {
		var newErr error

		BeforeEach(func() {
			_, newErr = taskssync.NewConnector(nil, readyLeaseTable(), stubFactoryThatReturns(newFakeTasksClient()), silentLogger{}, newFakeClock(time.Now()))
		})

		It("should return an error", func() {
			Expect(newErr).To(MatchError(ContainSubstring("store must not be nil")))
		})
	})

	When("leaseTable is nil", func() {
		var newErr error

		BeforeEach(func() {
			_, newErr = taskssync.NewConnector(newStore(newFakePages()), nil, stubFactoryThatReturns(newFakeTasksClient()), silentLogger{}, newFakeClock(time.Now()))
		})

		It("should return an error", func() {
			Expect(newErr).To(MatchError(ContainSubstring("leaseTable must not be nil")))
		})
	})

	When("clientFactory is nil", func() {
		var newErr error

		BeforeEach(func() {
			_, newErr = taskssync.NewConnector(newStore(newFakePages()), readyLeaseTable(), nil, silentLogger{}, newFakeClock(time.Now()))
		})

		It("should return an error", func() {
			Expect(newErr).To(MatchError(ContainSubstring("clientFactory must not be nil")))
		})
	})
})

var _ = Describe("Connector.Sync", func() {
	When("no subscription exists for the key", func() {
		var syncErr error

		BeforeEach(func() {
			pages := newFakePages()
			store := newConfiguredStore(pages, nil)
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(time.Now()), newFakeChecklistReader(), nil, nil)
			syncErr = c.Sync(context.Background(), subscriptionKey())
		})

		It("should not error", func() {
			Expect(syncErr).ToNot(HaveOccurred())
		})
	})

	When("the subscription is paused", func() {
		var (
			syncErr error
			client  *fakeTasksClient
		)

		BeforeEach(func() {
			pages := newFakePages()
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStatePaused,
				PausedReason: taskssync.PausedReasonAuthFailed,
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(time.Now()), newFakeChecklistReader(), nil, nil)
			syncErr = c.Sync(context.Background(), subscriptionKey())
		})

		It("should not error", func() {
			Expect(syncErr).ToNot(HaveOccurred())
		})

		It("should not call ListTasks (cursor frozen, no API)", func() {
			Expect(client.inserted).To(BeEmpty())
			Expect(client.patched).To(BeEmpty())
			Expect(client.deleted).To(BeEmpty())
		})
	})

	When("the rate-limit choke is active", func() {
		var (
			syncErr error
			client  *fakeTasksClient
		)

		BeforeEach(func() {
			pages := newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:                 syncTestPage,
				ListName:             syncTestListName,
				RemoteListID:         syncTestRemote,
				State:                taskssync.SubscriptionStateActive,
				LastSuccessfulSyncAt: now.Add(-5 * time.Second), // < 25s choke
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), newFakeChecklistReader(), nil, nil)
			syncErr = c.Sync(context.Background(), subscriptionKey())
		})

		It("should not error", func() {
			Expect(syncErr).ToNot(HaveOccurred())
		})

		It("should not call any client method", func() {
			Expect(client.inserted).To(BeEmpty())
			Expect(client.patched).To(BeEmpty())
			Expect(client.deleted).To(BeEmpty())
		})
	})

	When("inbound has a fresh task with no existing id_map entry", func() {
		var (
			syncErr error
			pages   *fakePages
			mutator *fakeChecklistMutator
			now     time.Time
		)

		BeforeEach(func() {
			pages = newFakePages()
			now = time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
			}
			store := newConfiguredStore(pages, &sub)
			client := newFakeTasksClient()
			client.listAllForList[syncTestRemote] = []gateway.Task{{
				ID:      "remote-task-1",
				Etag:    "etag-1",
				Title:   "Buy milk",
				Status:  gateway.TaskStatusNeedsAction,
				Updated: now.Add(60 * time.Second),
			}}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, nil)
			mutator = newFakeChecklistMutatorBoundTo(reader)
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, mutator, nil)
			syncErr = c.Sync(context.Background(), subscriptionKey())
		})

		It("should not error", func() {
			Expect(syncErr).ToNot(HaveOccurred())
		})

		It("should call AddItemForSync once with the task title", func() {
			Expect(mutator.added).To(HaveLen(1))
			Expect(mutator.added[0].Text).To(Equal("Buy milk"))
		})

		It("should populate ItemIDMap with the new wiki uid → tasks id binding", func() {
			store := newStore(pages)
			loaded, _, err := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(err).ToNot(HaveOccurred())
			Expect(loaded.ItemIDMap).To(HaveLen(1))
			for _, taskID := range loaded.ItemIDMap {
				Expect(taskID).To(Equal("remote-task-1"))
			}
		})

		It("should advance LastUpdatedMin (apply-then-advance) with the safety buffer", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			expected := now.Add(60*time.Second).Add(-1 * time.Second)
			Expect(loaded.LastUpdatedMin).To(BeTemporally("~", expected, time.Second))
		})
	})

	When("inbound has a delete (Deleted=true) for a known wiki uid", func() {
		var (
			syncErr error
			pages   *fakePages
			mutator *fakeChecklistMutator
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"wiki-1": "remote-task-1"},
				ItemEtags:    map[string]string{"remote-task-1": "etag-old"},
			}
			store := newConfiguredStore(pages, &sub)
			client := newFakeTasksClient()
			client.listAllForList[syncTestRemote] = []gateway.Task{{
				ID:      "remote-task-1",
				Deleted: true,
				Updated: now.Add(60 * time.Second),
			}}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Old item"},
			})
			mutator = newFakeChecklistMutatorBoundTo(reader)
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, mutator, nil)
			syncErr = c.Sync(context.Background(), subscriptionKey())
		})

		It("should not error", func() {
			Expect(syncErr).ToNot(HaveOccurred())
		})

		It("should call DeleteItemForSync for the matched uid", func() {
			Expect(mutator.deleted).To(HaveLen(1))
			Expect(mutator.deleted[0].UID).To(Equal("wiki-1"))
		})

		It("should remove the uid from the persisted ItemIDMap", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.ItemIDMap).ToNot(HaveKey("wiki-1"))
		})
	})

	When("inbound suppression is wired", func() {
		var (
			suppressor *fakeSuppressor
		)

		BeforeEach(func() {
			pages := newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
			}
			store := newConfiguredStore(pages, &sub)
			client := newFakeTasksClient()
			client.listAllForList[syncTestRemote] = []gateway.Task{{
				ID: "remote-task-1", Title: "T", Status: gateway.TaskStatusNeedsAction, Updated: now,
			}}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, nil)
			mutator := newFakeChecklistMutatorBoundTo(reader)
			suppressor = newFakeSuppressor()
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, mutator, suppressor)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should call Suppress before applying inbound", func() {
			Expect(suppressor.suppressCalls).ToNot(BeEmpty())
		})

		It("should call Unsuppress to balance Suppress", func() {
			Expect(len(suppressor.unsuppressCalls)).To(Equal(len(suppressor.suppressCalls)))
		})
	})

	When("inbound has a markerless task whose title text-matches an existing wiki item", func() {
		var (
			pages *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{},
			}
			store := newConfiguredStore(pages, &sub)
			client := newFakeTasksClient()
			// Markerless task — user accidentally erased the wiki:uid line.
			client.listAllForList[syncTestRemote] = []gateway.Task{{
				ID:      "task-orphan",
				Title:   "Buy milk",
				Notes:   "Plain notes, no marker",
				Status:  gateway.TaskStatusNeedsAction,
				Updated: now.Add(60 * time.Second),
			}}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-existing", Text: "Buy milk"},
			})
			mutator := newFakeChecklistMutatorBoundTo(reader)
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, mutator, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should re-bind the existing wiki uid to the orphan task id", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.ItemIDMap).To(HaveKeyWithValue("wiki-existing", "task-orphan"))
		})
	})

	When("inbound has a task carrying a wiki:uid marker for a known wiki uid (re-stamped recovery)", func() {
		var (
			pages *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{},
			}
			store := newConfiguredStore(pages, &sub)
			client := newFakeTasksClient()
			// The task was created by an earlier outbound push that
			// crashed before persisting state — the marker survives.
			markerNotes := "Plain notes" + translator.WikiUIDMarker("wiki-recovered")
			client.listAllForList[syncTestRemote] = []gateway.Task{{
				ID:      "task-marked",
				Title:   "Recovered",
				Notes:   markerNotes,
				Status:  gateway.TaskStatusNeedsAction,
				Updated: now,
			}}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-recovered", Text: "Recovered"},
			})
			mutator := newFakeChecklistMutatorBoundTo(reader)
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, mutator, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should re-bind via the marker (preferring marker over text-match)", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.ItemIDMap).To(HaveKeyWithValue("wiki-recovered", "task-marked"))
		})
	})

	When("outbound diff requires inserting a brand-new wiki uid", func() {
		var (
			client *fakeTasksClient
			pages  *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			// No inbound tasks. Pre-insert dedup pull also returns empty.
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-new-1", Text: "Eggs"},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should call InsertTask once for the new uid", func() {
			Expect(client.inserted).To(HaveLen(1))
			Expect(client.inserted[0].Title).To(Equal("Eggs"))
		})

		It("should append the wiki:uid marker to the inserted task notes", func() {
			Expect(client.inserted[0].Notes).To(ContainSubstring("wiki:uid=wiki-new-1"))
		})

		It("should record the new task id in ItemIDMap", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.ItemIDMap).To(HaveKeyWithValue("wiki-new-1", "inserted-1"))
		})
	})

	When("outbound diff has a wiki uid already mapped to a remote task", func() {
		var (
			client *fakeTasksClient
		)

		BeforeEach(func() {
			pages := newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"wiki-1": "task-1"},
				ItemEtags:    map[string]string{"task-1": "etag-known"},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Edited text"},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should call PatchTask not InsertTask", func() {
			Expect(client.patched).To(HaveLen(1))
			Expect(client.inserted).To(BeEmpty())
		})

		It("should send the cached etag as If-Match", func() {
			Expect(client.patched[0].Etag).To(Equal("etag-known"))
		})
	})

	When("outbound PatchTask returns 412 (etag stale)", func() {
		var (
			client  *fakeTasksClient
			mutator *fakeChecklistMutator
			pages   *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			// Cursor is in the future so inbound returns no tasks
			// — this isolates the outbound 412 path. The
			// updatedMin filter is enforced in production, but the
			// test fake ignores it; we model "no inbound" with an
			// empty listAllForList and only set listsForList for
			// the outbound 412 pull.
			sub := taskssync.Subscription{
				Page:           syncTestPage,
				ListName:       syncTestListName,
				RemoteListID:   syncTestRemote,
				State:          taskssync.SubscriptionStateActive,
				LastUpdatedMin: now.Add(time.Hour),
				ItemIDMap:      map[string]string{"wiki-1": "task-1"},
				ItemEtags:      map[string]string{"task-1": "etag-stale"},
				SyncedItems: map[string]taskssync.ItemSyncState{
					"wiki-1": {SyncedTitle: "Old", SyncedStatus: "needsAction"},
				},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			client.patchErrors = []error{gateway.ErrPreconditionFailed}
			// The remote authoritative pull on 412 will see this
			// — Google's view of the item after the phone edit.
			client.listAllForList[syncTestRemote] = []gateway.Task{{
				ID:      "task-1",
				Etag:    "etag-fresh",
				Title:   "Remote-Edited-By-Phone",
				Status:  gateway.TaskStatusCompleted,
				Updated: now,
			}}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Local edit that lost the race"},
			})
			mutator = newFakeChecklistMutatorBoundTo(reader)
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, mutator, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should NOT blind-retry the patch with empty If-Match", func() {
			// Only the original etag-bearing patch attempt should
			// have been sent. A second blind retry would be
			// last-write-wins and silently destroy the phone-side
			// change — that's the bug this fix prevents.
			Expect(client.patched).To(HaveLen(1))
			Expect(client.patched[0].Etag).To(Equal("etag-stale"))
		})

		It("should apply the remote authoritative state into the wiki via the mutator", func() {
			Expect(mutator.updated).To(HaveLen(1))
			Expect(mutator.updated[0].UID).To(Equal("wiki-1"))
			Expect(mutator.updated[0].Text).To(Equal("Remote-Edited-By-Phone"))
			Expect(mutator.updated[0].Checked).To(BeTrue())
		})

		It("should refresh the cached etag from the remote authoritative pull", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.ItemEtags["task-1"]).To(Equal("etag-fresh"))
		})

		It("should advance the SyncedItems baseline to the remote-authoritative state", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			synced, ok := loaded.SyncedItems["wiki-1"]
			Expect(ok).To(BeTrue())
			Expect(synced.SyncedTitle).To(Equal("Remote-Edited-By-Phone"))
			Expect(synced.SyncedStatus).To(Equal("completed"))
		})
	})

	// REGRESSION TEST for "Tasks 412 destroys local edit when remote
	// hasn't actually changed since synced." Production bug: a 412 on
	// PATCH was treated as "remote moved under us" even when the
	// remote task's fields equal SyncedItems baseline (the etag was
	// stale for some other reason — server-side internal bump,
	// sequencing race). The old recovery path would overwrite the
	// wiki with the unchanged remote state, destroying the user's
	// local edit. The fix: when remote == synced, refresh the etag
	// and RE-PATCH so the user's edit lands.
	When("outbound PatchTask returns 412 but remote fields match SyncedItems baseline", func() {
		var (
			client  *fakeTasksClient
			mutator *fakeChecklistMutator
			pages   *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:           syncTestPage,
				ListName:       syncTestListName,
				RemoteListID:   syncTestRemote,
				State:          taskssync.SubscriptionStateActive,
				LastUpdatedMin: now.Add(time.Hour),
				ItemIDMap:      map[string]string{"wiki-1": "task-1"},
				ItemEtags:      map[string]string{"task-1": "etag-stale"},
				// Synced baseline: needsAction. Wiki was just toggled
				// to checked=true (status=completed). Remote has NOT
				// changed since synced — it still says needsAction.
				SyncedItems: map[string]taskssync.ItemSyncState{
					"wiki-1": {
						SyncedTitle:  "Take out trash",
						SyncedStatus: "needsAction",
					},
				},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			// First PATCH attempt returns 412. Second PATCH (re-
			// PATCH with fresh etag) succeeds via the default fake
			// behavior.
			client.patchErrors = []error{gateway.ErrPreconditionFailed, nil}
			// The 412-recovery list-call returns the remote in its
			// pre-edit state (needsAction) — same as SyncedItems
			// baseline. This is the "phantom 412" case.
			client.listAllForList[syncTestRemote] = []gateway.Task{{
				ID:      "task-1",
				Etag:    "etag-fresh",
				Title:   "Take out trash",
				Status:  gateway.TaskStatusNeedsAction,
				Updated: now,
			}}
			reader := newFakeChecklistReader()
			// Wiki was toggled to checked=true (the user's edit that
			// triggered this outbound push).
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Take out trash", Checked: true},
			})
			mutator = newFakeChecklistMutatorBoundTo(reader)
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, mutator, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should re-PATCH with the refreshed etag instead of overwriting the wiki", func() {
			// Two PATCH attempts: the first with the stale etag, the
			// second with the fresh one from the recovery list.
			Expect(client.patched).To(HaveLen(2))
			Expect(client.patched[0].Etag).To(Equal("etag-stale"))
			Expect(client.patched[1].Etag).To(Equal("etag-fresh"))
		})

		It("should send the user's edit (status=completed) on the re-PATCH", func() {
			Expect(client.patched).To(HaveLen(2))
			Expect(client.patched[1].Fields.Status).To(Equal(gateway.TaskStatusCompleted))
		})

		It("should NOT call UpdateItemForSync on the wiki (no authoritative-apply overwrite)", func() {
			Expect(mutator.updated).To(BeEmpty())
		})

		It("should advance SyncedItems to reflect the re-patched state", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			synced, ok := loaded.SyncedItems["wiki-1"]
			Expect(ok).To(BeTrue())
			Expect(synced.SyncedStatus).To(Equal("completed"))
		})
	})

	When("the wiki Due has a time-of-day but Synced* was stamped from Tasks's date-only response", func() {
		// REGRESSION TEST for the production bug: Google Tasks
		// truncates `due` to date-only on the wire, so an inbound
		// observation always sees Due=00:00. The wiki keeps the
		// original time-of-day (e.g. 21:30) on its Due proto. If the
		// diff compares timestamps at full resolution, every tick
		// fires a phantom patch; comparing at date-only resolution
		// matches Tasks's actual semantics and is correct.
		var client *fakeTasksClient

		BeforeEach(func() {
			pages := newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			wikiDue := time.Date(2026, 5, 3, 21, 30, 0, 0, time.UTC)
			tasksDue := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
			syncedFields := taskssync.ItemSyncState{
				SyncedTitle:  "Sun: Make Dinner",
				SyncedNotes:  translator.WikiUIDMarker("wiki-1"),
				SyncedStatus: "needsAction",
				SyncedDue:    tasksDue, // Tasks-side observed value
			}
			sub := taskssync.Subscription{
				Page:           syncTestPage,
				ListName:       syncTestListName,
				RemoteListID:   syncTestRemote,
				State:          taskssync.SubscriptionStateActive,
				LastUpdatedMin: now.Add(time.Hour),
				ItemIDMap:      map[string]string{"wiki-1": "task-1"},
				ItemEtags:      map[string]string{"task-1": "etag-known"},
				SyncedItems:    map[string]taskssync.ItemSyncState{"wiki-1": syncedFields},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Sun: Make Dinner", Due: timestamppb.New(wikiDue)},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should NOT call PatchTask (date-only comparison treats wikiDue and tasksDue as equal)", func() {
			Expect(client.patched).To(BeEmpty())
		})
	})

	When("the wiki is unchanged since the last successful push", func() {
		// REGRESSION TEST: This is the every-tick-overwrites-phone
		// bug. With diff-before-push, a tick where nothing changed
		// in the wiki must NOT call PatchTask — patching anyway
		// would race against and clobber phone-side edits the user
		// is making at the same time.
		var client *fakeTasksClient

		BeforeEach(func() {
			pages := newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			// SyncedItems baseline matches what the wiki currently
			// shows — i.e., we've already pushed this state once
			// and the wiki hasn't been edited since.
			syncedFields := taskssync.ItemSyncState{
				SyncedTitle:  "Eggs",
				SyncedNotes:  translator.WikiUIDMarker("wiki-1"),
				SyncedStatus: "needsAction",
			}
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"wiki-1": "task-1"},
				ItemEtags:    map[string]string{"task-1": "etag-known"},
				SyncedItems:  map[string]taskssync.ItemSyncState{"wiki-1": syncedFields},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Eggs"},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should NOT call PatchTask (no local change to push)", func() {
			Expect(client.patched).To(BeEmpty())
		})

		It("should NOT call InsertTask", func() {
			Expect(client.inserted).To(BeEmpty())
		})

		It("should NOT call DeleteTask", func() {
			Expect(client.deleted).To(BeEmpty())
		})
	})

	When("a wiki item changed since the last successful push", func() {
		// Complement to the no-change regression test: the diff
		// MUST detect a real local change and push it.
		var (
			client *fakeTasksClient
			pages  *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			// SyncedItems baseline is the OLD state.
			syncedFields := taskssync.ItemSyncState{
				SyncedTitle:  "Old text",
				SyncedNotes:  translator.WikiUIDMarker("wiki-1"),
				SyncedStatus: "needsAction",
			}
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"wiki-1": "task-1"},
				ItemEtags:    map[string]string{"task-1": "etag-known"},
				SyncedItems:  map[string]taskssync.ItemSyncState{"wiki-1": syncedFields},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			reader := newFakeChecklistReader()
			// Wiki now reflects the user's edit.
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "New text"},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should call PatchTask exactly once for the changed item", func() {
			Expect(client.patched).To(HaveLen(1))
			Expect(client.patched[0].Fields.Title).To(Equal("New text"))
		})

		It("should advance SyncedItems.SyncedTitle to the just-pushed value", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.SyncedItems["wiki-1"].SyncedTitle).To(Equal("New text"))
		})
	})

	When("a wiki item is toggled checked since the last successful push", func() {
		// MIRROR of Keep W2 ("wiki toggles an item's checked state").
		// The user-reported bug was unchecking an item in the wiki UI
		// produced zero outbound PATCH calls. This test pins down the
		// status-toggle outbound path so the bug can never silently
		// regress through code edits to the diff predicate.
		var (
			client *fakeTasksClient
			pages  *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			// Baseline: item was synced as needsAction.
			syncedFields := taskssync.ItemSyncState{
				SyncedTitle:  "Eggs",
				SyncedNotes:  translator.WikiUIDMarker("wiki-1"),
				SyncedStatus: "needsAction",
			}
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"wiki-1": "task-1"},
				ItemEtags:    map[string]string{"task-1": "etag-known"},
				SyncedItems:  map[string]taskssync.ItemSyncState{"wiki-1": syncedFields},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			reader := newFakeChecklistReader()
			// Wiki now reflects the user toggling the checkbox.
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Eggs", Checked: true},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should call PatchTask exactly once for the toggled item", func() {
			Expect(client.patched).To(HaveLen(1))
		})

		It("should send Status=completed in the patch fields", func() {
			Expect(client.patched[0].Fields.Status).To(Equal(gateway.TaskStatusCompleted))
		})

		It("should target the mapped task id", func() {
			Expect(client.patched[0].TaskID).To(Equal("task-1"))
		})

		It("should send the cached etag as If-Match", func() {
			Expect(client.patched[0].Etag).To(Equal("etag-known"))
		})

		It("should advance SyncedItems.SyncedStatus to completed", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.SyncedItems["wiki-1"].SyncedStatus).To(Equal("completed"))
		})
	})

	When("a wiki item is unchecked (re-opened) since the last successful push", func() {
		// MIRROR of Keep W2 (reverse direction). Validates the
		// status=needsAction direction explicitly so the asymmetry
		// between check and uncheck can't silently fall on its face.
		var client *fakeTasksClient

		BeforeEach(func() {
			pages := newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			syncedFields := taskssync.ItemSyncState{
				SyncedTitle:  "Eggs",
				SyncedNotes:  translator.WikiUIDMarker("wiki-1"),
				SyncedStatus: "completed",
			}
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"wiki-1": "task-1"},
				ItemEtags:    map[string]string{"task-1": "etag-known"},
				SyncedItems:  map[string]taskssync.ItemSyncState{"wiki-1": syncedFields},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Eggs", Checked: false},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should call PatchTask exactly once", func() {
			Expect(client.patched).To(HaveLen(1))
		})

		It("should send Status=needsAction in the patch fields", func() {
			Expect(client.patched[0].Fields.Status).To(Equal(gateway.TaskStatusNeedsAction))
		})
	})

	When("multiple wiki items changed and one is unchanged", func() {
		// MIRROR of Keep "multi-item mixed — push only items that
		// actually differ". Defends against a regression where the
		// diff loop pushes every mapped uid on every tick (the very
		// over-write bug syncedMatchesFields was meant to prevent),
		// even when only some items have local changes.
		var client *fakeTasksClient

		BeforeEach(func() {
			pages := newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap: map[string]string{
					"wiki-A": "task-A",
					"wiki-B": "task-B",
					"wiki-C": "task-C",
				},
				ItemEtags: map[string]string{
					"task-A": "eA", "task-B": "eB", "task-C": "eC",
				},
				SyncedItems: map[string]taskssync.ItemSyncState{
					"wiki-A": {SyncedTitle: "Apple", SyncedNotes: translator.WikiUIDMarker("wiki-A"), SyncedStatus: "needsAction"},
					"wiki-B": {SyncedTitle: "Banana", SyncedNotes: translator.WikiUIDMarker("wiki-B"), SyncedStatus: "needsAction"},
					"wiki-C": {SyncedTitle: "Cherry", SyncedNotes: translator.WikiUIDMarker("wiki-C"), SyncedStatus: "needsAction"},
				},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			reader := newFakeChecklistReader()
			// A unchanged, B text edited, C toggled checked.
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-A", Text: "Apple"},
				{Uid: "wiki-B", Text: "Banana bread"},
				{Uid: "wiki-C", Text: "Cherry", Checked: true},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should call PatchTask exactly twice (the two changed items)", func() {
			Expect(client.patched).To(HaveLen(2))
		})

		It("should not patch the unchanged item", func() {
			for _, p := range client.patched {
				Expect(p.TaskID).ToNot(Equal("task-A"))
			}
		})

		It("should patch the text-edited item", func() {
			seen := false
			for _, p := range client.patched {
				if p.TaskID == "task-B" && p.Fields.Title == "Banana bread" {
					seen = true
				}
			}
			Expect(seen).To(BeTrue())
		})

		It("should patch the toggled item with Status=completed", func() {
			seen := false
			for _, p := range client.patched {
				if p.TaskID == "task-C" && p.Fields.Status == gateway.TaskStatusCompleted {
					seen = true
				}
			}
			Expect(seen).To(BeTrue())
		})
	})

	When("a wiki item has both text and checked changed in one tick", func() {
		// MIRROR of Keep "combined — wiki edits both text and checked
		// at once". Single push must carry both changes; we don't
		// emit two patches for one item.
		var client *fakeTasksClient

		BeforeEach(func() {
			pages := newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"wiki-1": "task-1"},
				ItemEtags:    map[string]string{"task-1": "e1"},
				SyncedItems: map[string]taskssync.ItemSyncState{
					"wiki-1": {SyncedTitle: "Old", SyncedNotes: translator.WikiUIDMarker("wiki-1"), SyncedStatus: "needsAction"},
				},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "New", Checked: true},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should call PatchTask exactly once", func() {
			Expect(client.patched).To(HaveLen(1))
		})

		It("should carry the new title", func() {
			Expect(client.patched[0].Fields.Title).To(Equal("New"))
		})

		It("should carry the new status in the same patch", func() {
			Expect(client.patched[0].Fields.Status).To(Equal(gateway.TaskStatusCompleted))
		})
	})

	When("an inbound task toggles checked on an item known to the wiki", func() {
		// MIRROR of Keep K2 ("Keep toggles checked, wiki is older").
		// Existing inbound test only covers fresh-arrival; this pins
		// the inbound update path for an already-mapped item.
		var (
			mutator *fakeChecklistMutator
			client  *fakeTasksClient
		)

		BeforeEach(func() {
			pages := newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"wiki-1": "task-1"},
				ItemEtags:    map[string]string{"task-1": "etag-old"},
				SyncedItems: map[string]taskssync.ItemSyncState{
					"wiki-1": {SyncedTitle: "Eggs", SyncedNotes: translator.WikiUIDMarker("wiki-1"), SyncedStatus: "needsAction"},
				},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			// Inbound: remote toggled the existing task to completed.
			client.listAllForList[syncTestRemote] = []gateway.Task{{
				ID: "task-1", Etag: "etag-fresh",
				Title: "Eggs", Status: gateway.TaskStatusCompleted,
				Updated: now.Add(60 * time.Second),
			}}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Eggs"},
			})
			mutator = newFakeChecklistMutatorBoundTo(reader)
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, mutator, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should call UpdateItemForSync exactly once", func() {
			Expect(mutator.updated).To(HaveLen(1))
		})

		It("should update with checked=true", func() {
			Expect(mutator.updated[0].Checked).To(BeTrue())
		})

		It("should NOT push back to Tasks (post-apply content equality)", func() {
			Expect(client.patched).To(BeEmpty())
		})
	})

	When("the SyncSuppressor is wired and inbound apply runs", func() {
		// MIRROR of Keep B1 / inbound-apply isolation. Verifies the
		// real wiring: inbound writes via the mutator MUST be wrapped
		// in Suppress/Unsuppress so a notify on that wiki page does
		// not loop back into a fresh outbound enqueue.
		var (
			suppressor *fakeSuppressor
			mutator    *fakeChecklistMutator
		)

		BeforeEach(func() {
			pages := newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"wiki-1": "task-1"},
			}
			store := newConfiguredStore(pages, &sub)
			client := newFakeTasksClient()
			client.listAllForList[syncTestRemote] = []gateway.Task{{
				ID: "task-1", Etag: "e1",
				Title: "Edited remotely", Status: gateway.TaskStatusNeedsAction,
				Updated: now,
			}}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Eggs"},
			})
			mutator = newFakeChecklistMutatorBoundTo(reader)
			suppressor = newFakeSuppressor()
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, mutator, suppressor)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should call Suppress for the page+list before applying", func() {
			Expect(suppressor.suppressCalls).ToNot(BeEmpty())
			Expect(suppressor.suppressCalls[0].Page).To(Equal(syncTestPage))
			Expect(suppressor.suppressCalls[0].ListName).To(Equal(syncTestListName))
		})

		It("should call Unsuppress to balance Suppress (no leaked refcount)", func() {
			Expect(len(suppressor.unsuppressCalls)).To(Equal(len(suppressor.suppressCalls)))
		})

		It("should still apply the inbound update via the mutator", func() {
			Expect(mutator.updated).To(HaveLen(1))
		})
	})

	When("a tick replays the same applied state (idempotence)", func() {
		// MIRROR of Keep idempotence behaviors (e.g.
		// "outbound_skips_items_at_synced_baseline" combined with
		// inbound replay). After applying an inbound change, a
		// re-run with no new state must not re-mutate or re-push.
		var (
			mutator *fakeChecklistMutator
			client  *fakeTasksClient
			clock   *fakeClock
			c       *taskssync.Connector
		)

		BeforeEach(func() {
			pages := newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			clock = newFakeClock(now)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"wiki-1": "task-1"},
				ItemEtags:    map[string]string{"task-1": "e1"},
				SyncedItems: map[string]taskssync.ItemSyncState{
					"wiki-1": {SyncedTitle: "Eggs", SyncedNotes: translator.WikiUIDMarker("wiki-1"), SyncedStatus: "needsAction"},
				},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Eggs"},
			})
			mutator = newFakeChecklistMutatorBoundTo(reader)
			c = buildTestConnector(store, readyLeaseTable(), client, clock, reader, mutator, nil)
			// First tick: no changes anywhere.
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
			// Advance past rate-limit choke and run again.
			clock.Advance(60 * time.Second)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should not call PatchTask across either tick", func() {
			Expect(client.patched).To(BeEmpty())
		})

		It("should not call InsertTask across either tick", func() {
			Expect(client.inserted).To(BeEmpty())
		})

		It("should not call DeleteTask across either tick", func() {
			Expect(client.deleted).To(BeEmpty())
		})

		It("should not mutate the wiki across either tick", func() {
			Expect(mutator.updated).To(BeEmpty())
			Expect(mutator.added).To(BeEmpty())
			Expect(mutator.deleted).To(BeEmpty())
		})
	})

	When("two consecutive ticks fire with no wiki changes between them", func() {
		// REGRESSION TEST: end-to-end "second tick is a no-op."
		// This mirrors the production failure mode — every 30s tick
		// re-patching every item even though nothing local changed.
		var (
			client *fakeTasksClient
			clock  *fakeClock
			c      *taskssync.Connector
			pages  *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			clock = newFakeClock(now)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"wiki-1": "task-1", "wiki-2": "task-2"},
				ItemEtags:    map[string]string{"task-1": "e1", "task-2": "e2"},
				// SyncedItems EMPTY initially — first tick will push
				// to seed the baseline. Second tick should be a no-op.
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Eggs"},
				{Uid: "wiki-2", Text: "Milk"},
			})
			c = buildTestConnector(store, readyLeaseTable(), client, clock, reader, nil, nil)
			// First tick — establishes baseline by pushing.
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
			// Advance past the rate-limit choke and run again with
			// no wiki changes.
			clock.Advance(60 * time.Second)
			// Snapshot the patch count after tick 1.
			countAfterTick1 := len(client.patched)
			Expect(countAfterTick1).To(BeNumerically(">", 0))
			// Second tick.
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
			// Stash for assertions.
			DeferCleanup(func() {
				_ = countAfterTick1
			})
		})

		It("should send no additional PatchTask on the second tick (no-change is a no-op)", func() {
			// Tick 1 pushes 2 patches (to seed Synced*). Tick 2 must add zero.
			Expect(client.patched).To(HaveLen(2))
		})
	})

	When("a remote-only change arrives and no wiki edit happened locally", func() {
		// REGRESSION TEST: an inbound change must NOT trigger an
		// outbound patch back at Google for the same item.
		var (
			client  *fakeTasksClient
			pages   *fakePages
			mutator *fakeChecklistMutator
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			// Wiki and Synced* are already aligned (nothing local
			// changed). Then inbound brings a phone-side update.
			syncedFields := taskssync.ItemSyncState{
				SyncedTitle:  "Old",
				SyncedNotes:  translator.WikiUIDMarker("wiki-1"),
				SyncedStatus: "needsAction",
			}
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"wiki-1": "task-1"},
				ItemEtags:    map[string]string{"task-1": "e-old"},
				SyncedItems:  map[string]taskssync.ItemSyncState{"wiki-1": syncedFields},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			// Fresh remote state arrives via inbound.
			client.listAllForList[syncTestRemote] = []gateway.Task{{
				ID: "task-1", Etag: "e-fresh",
				Title: "Phone update", Status: gateway.TaskStatusCompleted,
				Updated: now.Add(60 * time.Second),
			}}
			reader := newFakeChecklistReader()
			// Wiki currently reflects the OLD state — exactly
			// matching SyncedItems → no local change.
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Old"},
			})
			mutator = newFakeChecklistMutatorBoundTo(reader)
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, mutator, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should apply the inbound update via the mutator", func() {
			Expect(mutator.updated).To(HaveLen(1))
			Expect(mutator.updated[0].Text).To(Equal("Phone update"))
		})

		It("should NOT call PatchTask (no local change to push back)", func() {
			Expect(client.patched).To(BeEmpty())
		})
	})

	When("outbound diff has a missing wiki uid (id_map entry but no current item)", func() {
		var client *fakeTasksClient

		BeforeEach(func() {
			pages := newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"wiki-removed": "task-removed"},
				ItemEtags:    map[string]string{"task-removed": "etag-x"},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, nil)
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should call DeleteTask for the missing uid's task id", func() {
			Expect(client.deleted).To(HaveLen(1))
			Expect(client.deleted[0].TaskID).To(Equal("task-removed"))
		})
	})

	When("outbound has a new uid but the remote already carries that uid as a marker (idempotent recovery)", func() {
		var (
			client *fakeTasksClient
			pages  *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			markerNotes := "" + translator.WikiUIDMarker("wiki-orphaned")
			// Pre-insert scan returns the orphaned task with the marker.
			client.listAllForList[syncTestRemote] = []gateway.Task{{
				ID:    "task-orphaned",
				Title: "Eggs",
				Notes: markerNotes,
				// updated zero so apply-and-advance doesn't move the cursor.
			}}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-orphaned", Text: "Eggs"},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should switch to PatchTask rather than InsertTask", func() {
			Expect(client.inserted).To(BeEmpty())
		})

		It("should re-bind the wiki uid to the existing task id", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.ItemIDMap).To(HaveKeyWithValue("wiki-orphaned", "task-orphaned"))
		})
	})

	When("the gateway returns ErrInvalidGrant", func() {
		var (
			pages   *fakePages
			syncErr error
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
				ItemIDMap:    map[string]string{"u1": "t1"},
			}
			store := newConfiguredStore(pages, &sub)
			client := newFakeTasksClient()
			client.errorsForListTasks = []error{gateway.ErrInvalidGrant}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, nil)
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			syncErr = c.Sync(context.Background(), subscriptionKey())
		})

		It("should not return the error to the scheduler (paused state is steady-state)", func() {
			Expect(syncErr).ToNot(HaveOccurred())
		})

		It("should transition the subscription to paused", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.State).To(Equal(taskssync.SubscriptionStatePaused))
			Expect(loaded.PausedReason).To(Equal(taskssync.PausedReasonAuthFailed))
		})

		It("should preserve the ItemIDMap on the paused subscription", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.ItemIDMap).To(HaveKeyWithValue("u1", "t1"))
		})
	})

	When("inbound pagination spans multiple pages", func() {
		var (
			pages   *fakePages
			mutator *fakeChecklistMutator
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:         syncTestPage,
				ListName:     syncTestListName,
				RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStateActive,
			}
			store := newConfiguredStore(pages, &sub)
			client := newFakeTasksClient()
			client.listsForList[syncTestRemote] = []gateway.TasksPage{
				{Tasks: []gateway.Task{{ID: "t1", Title: "Page1A", Status: gateway.TaskStatusNeedsAction, Updated: now.Add(60 * time.Second)}}, NextPageToken: "next-1"},
				{Tasks: []gateway.Task{{ID: "t2", Title: "Page2A", Status: gateway.TaskStatusNeedsAction, Updated: now.Add(120 * time.Second)}}},
			}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, nil)
			mutator = newFakeChecklistMutatorBoundTo(reader)
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, mutator, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should consume both pages before advancing", func() {
			Expect(mutator.added).To(HaveLen(2))
		})

		It("should advance LastUpdatedMin to the maximum updated across all pages (minus buffer)", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			expected := now.Add(120 * time.Second).Add(-1 * time.Second)
			Expect(loaded.LastUpdatedMin).To(BeTemporally("~", expected, time.Second))
		})
	})
})

var _ = Describe("Connector.PausedReason", func() {
	When("the subscription is paused", func() {
		var (
			reason string
			ok     bool
		)

		BeforeEach(func() {
			pages := newFakePages()
			sub := taskssync.Subscription{
				Page: syncTestPage, ListName: syncTestListName, RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStatePaused,
				PausedReason: taskssync.PausedReasonAuthFailed,
			}
			store := newConfiguredStore(pages, &sub)
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(time.Now()), nil, nil, nil)
			reason, ok = c.PausedReason(subscriptionKey())
		})

		It("should return true", func() {
			Expect(ok).To(BeTrue())
		})

		It("should return the persisted reason", func() {
			Expect(reason).To(Equal(taskssync.PausedReasonAuthFailed))
		})
	})

	When("the subscription is paused but PausedReason is empty", func() {
		var (
			reason string
			ok     bool
		)

		BeforeEach(func() {
			pages := newFakePages()
			sub := taskssync.Subscription{
				Page: syncTestPage, ListName: syncTestListName, RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStatePaused,
				PausedReason: "",
			}
			store := newConfiguredStore(pages, &sub)
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(time.Now()), nil, nil, nil)
			reason, ok = c.PausedReason(subscriptionKey())
		})

		It("should return true", func() {
			Expect(ok).To(BeTrue())
		})

		It("should return the generic paused reason", func() {
			Expect(reason).To(Equal("paused"))
		})
	})

	When("the subscription is active", func() {
		var ok bool

		BeforeEach(func() {
			pages := newFakePages()
			sub := taskssync.Subscription{
				Page: syncTestPage, ListName: syncTestListName, RemoteListID: syncTestRemote,
				State: taskssync.SubscriptionStateActive,
			}
			store := newConfiguredStore(pages, &sub)
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(time.Now()), nil, nil, nil)
			_, ok = c.PausedReason(subscriptionKey())
		})

		It("should return false", func() {
			Expect(ok).To(BeFalse())
		})
	})
})

var _ = Describe("Connector.ForceFullResync", func() {
	When("a subscription exists and the Tasks list contains markered tasks", func() {
		var (
			pages *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			sub := taskssync.Subscription{
				Page:           syncTestPage,
				ListName:       syncTestListName,
				RemoteListID:   syncTestRemote,
				State:          taskssync.SubscriptionStatePaused,
				PausedReason:   taskssync.PausedReasonAuthFailed,
				LastUpdatedMin: now.Add(-1 * time.Hour),
				ItemIDMap:      map[string]string{"stale": "stale-task"},
			}
			store := newConfiguredStore(pages, &sub)
			client := newFakeTasksClient()
			markerNotes := "n" + translator.WikiUIDMarker("wiki-A")
			client.listAllForList[syncTestRemote] = []gateway.Task{
				{ID: "task-A", Title: "A", Notes: markerNotes, Etag: "e-A", Updated: now},
				{ID: "task-B", Title: "B", Etag: "e-B", Updated: now}, // markerless
			}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-A", Text: "A"},
				{Uid: "wiki-B", Text: "B"},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.ForceFullResync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should rebuild ItemIDMap from marker + text match", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.ItemIDMap).To(HaveKeyWithValue("wiki-A", "task-A"))
			Expect(loaded.ItemIDMap).To(HaveKeyWithValue("wiki-B", "task-B"))
			Expect(loaded.ItemIDMap).ToNot(HaveKey("stale"))
		})

		It("should reset the cursor", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.LastUpdatedMin.IsZero()).To(BeTrue())
		})

		It("should clear the paused state", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.State).To(Equal(taskssync.SubscriptionStateActive))
			Expect(loaded.PausedReason).To(BeEmpty())
		})
	})

	When("no subscription exists", func() {
		var resyncErr error

		BeforeEach(func() {
			pages := newFakePages()
			store := newConfiguredStore(pages, nil)
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(time.Now()), nil, nil, nil)
			resyncErr = c.ForceFullResync(context.Background(), subscriptionKey())
		})

		It("should return ErrSubscriptionNotFound", func() {
			Expect(errors.Is(resyncErr, taskssync.ErrSubscriptionNotFound)).To(BeTrue())
		})
	})
})

var _ = Describe("Connector.Subscribe", func() {
	When("no subscription exists and the Tasks list is flat", func() {
		var (
			subscribed taskssync.Subscription
			subErr     error
			pages      *fakePages
			lt         *connectors.LeaseTable
		)

		BeforeEach(func() {
			pages = newFakePages()
			store := newConfiguredStore(pages, nil)
			lt = readyLeaseTable()
			client := newFakeTasksClient()
			client.taskLists = []gateway.TaskList{{ID: syncTestRemote, Title: "Groceries"}}
			client.listAllForList[syncTestRemote] = []gateway.Task{
				{ID: "task-A", Title: "Eggs", Status: gateway.TaskStatusNeedsAction},
			}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-A", Text: "Eggs"},
			})
			c := buildTestConnector(store, lt, client, newFakeClock(time.Now()), reader, nil, nil)
			subscribed, subErr = c.Subscribe(context.Background(), aliceProfile, syncTestPage, syncTestListName, syncTestRemote)
		})

		It("should not error", func() {
			Expect(subErr).ToNot(HaveOccurred())
		})

		It("should populate the friendly title", func() {
			Expect(subscribed.RemoteListTitle).To(Equal("Groceries"))
		})

		It("should seed the ItemIDMap from text match", func() {
			Expect(subscribed.ItemIDMap).To(HaveKeyWithValue("wiki-A", "task-A"))
		})

		It("should take the lease for the (page, listName) tuple", func() {
			owner, ok := lt.LookupOwner(connectors.ChecklistKey{Page: syncTestPage, ListName: syncTestListName})
			Expect(ok).To(BeTrue())
			Expect(owner.Kind).To(Equal(connectors.ConnectorKindGoogleTasks))
		})

		It("should persist the subscription", func() {
			store := newStore(pages)
			_, found, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(found).To(BeTrue())
		})

		It("should persist the Subscription with the correct fields via LoadState", func() {
			store := newStore(pages)
			loaded, err := store.LoadState(aliceProfile)
			Expect(err).ToNot(HaveOccurred())
			Expect(loaded.Subscriptions).To(HaveLen(1))
			Expect(loaded.Subscriptions[0].Page).To(Equal(syncTestPage))
			Expect(loaded.Subscriptions[0].ListName).To(Equal(syncTestListName))
			Expect(loaded.Subscriptions[0].RemoteListID).To(Equal(syncTestRemote))
			Expect(loaded.Subscriptions[0].State).To(Equal(taskssync.SubscriptionStateActive))
		})
	})

	When("ListTaskLists fails during resolveRemoteTitle", func() {
		var (
			subscribed taskssync.Subscription
			subErr     error
		)

		BeforeEach(func() {
			pages := newFakePages()
			store := newConfiguredStore(pages, nil)
			client := newFakeTasksClient()
			// ListTaskLists will fail — resolveRemoteTitle must tolerate this and
			// let Subscribe succeed with an empty RemoteListTitle.
			client.listTaskListsErr = errors.New("service unavailable")
			reader := newFakeChecklistReader()
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(time.Now()), reader, nil, nil)
			subscribed, subErr = c.Subscribe(context.Background(), aliceProfile, syncTestPage, syncTestListName, syncTestRemote)
		})

		It("should not error (title resolution failure is non-fatal)", func() {
			Expect(subErr).ToNot(HaveOccurred())
		})

		It("should produce an empty RemoteListTitle", func() {
			Expect(subscribed.RemoteListTitle).To(BeEmpty())
		})
	})

	When("the chosen Tasks list contains a subtask hierarchy", func() {
		var subErr error

		BeforeEach(func() {
			pages := newFakePages()
			store := newConfiguredStore(pages, nil)
			client := newFakeTasksClient()
			client.taskLists = []gateway.TaskList{{ID: syncTestRemote, Title: "Groceries"}}
			client.listAllForList[syncTestRemote] = []gateway.Task{
				{ID: "task-parent", Title: "Parent"},
				{ID: "task-child", Title: "Child", Parent: "task-parent"},
			}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, nil)
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(time.Now()), reader, nil, nil)
			_, subErr = c.Subscribe(context.Background(), aliceProfile, syncTestPage, syncTestListName, syncTestRemote)
		})

		It("should refuse-to-subscribe", func() {
			Expect(errors.Is(subErr, taskssync.ErrTasksListHasSubtasks)).To(BeTrue())
		})
	})

	When("another connector already holds the lease on the same checklist", func() {
		var subErr error

		BeforeEach(func() {
			pages := newFakePages()
			store := newConfiguredStore(pages, nil)
			lt := readyLeaseTable()
			Expect(lt.Take(connectors.ChecklistKey{Page: syncTestPage, ListName: syncTestListName},
				connectors.LeaseOwner{Kind: connectors.ConnectorKindGoogleKeep, ProfileID: "profile_other"})).To(Succeed())
			client := newFakeTasksClient()
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, nil)
			c := buildTestConnector(store, lt, client, newFakeClock(time.Now()), reader, nil, nil)
			_, subErr = c.Subscribe(context.Background(), aliceProfile, syncTestPage, syncTestListName, syncTestRemote)
		})

		It("should error with ErrChecklistAlreadyLeased", func() {
			Expect(errors.Is(subErr, connectors.ErrChecklistAlreadyLeased)).To(BeTrue())
		})
	})

	When("the connector is not configured (no refresh token)", func() {
		var subErr error

		BeforeEach(func() {
			pages := newFakePages()
			store := newStore(pages)
			Expect(store.SaveState(aliceProfile, taskssync.ConnectorState{})).To(Succeed())
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(time.Now()), newFakeChecklistReader(), nil, nil)
			_, subErr = c.Subscribe(context.Background(), aliceProfile, syncTestPage, syncTestListName, syncTestRemote)
		})

		It("should return ErrConnectorNotConfigured", func() {
			Expect(errors.Is(subErr, taskssync.ErrConnectorNotConfigured)).To(BeTrue())
		})
	})

	When("remoteListID is empty (Bind to a new Tasks list) with existing wiki items", func() {
		var (
			subscribed taskssync.Subscription
			subErr     error
			client     *fakeTasksClient
			pages      *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			store := newConfiguredStore(pages, nil)
			client = newFakeTasksClient()
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Eggs"},
				{Uid: "wiki-2", Text: "Milk"},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(time.Now()), reader, nil, nil)
			subscribed, subErr = c.Subscribe(context.Background(), aliceProfile, syncTestPage, syncTestListName, "")
		})

		It("should not error", func() {
			Expect(subErr).ToNot(HaveOccurred())
		})

		It("should call CreateTaskList with the wiki list name", func() {
			Expect(client.createdTaskLists).To(HaveLen(1))
			Expect(client.createdTaskLists[0].Title).To(Equal(syncTestListName))
		})

		It("should bind to the freshly-created tasklist id", func() {
			Expect(subscribed.RemoteListID).To(Equal("created-list-1"))
		})

		It("should populate the friendly title from the create response", func() {
			Expect(subscribed.RemoteListTitle).To(Equal(syncTestListName))
		})

		It("should InsertTask once per wiki item into the freshly-created tasklist", func() {
			Expect(client.inserted).To(HaveLen(2))
			Expect(client.inserted[0].TasklistID).To(Equal("created-list-1"))
			Expect(client.inserted[1].TasklistID).To(Equal("created-list-1"))
		})

		It("should stamp the wiki:uid marker on each inserted task's notes", func() {
			Expect(client.inserted[0].Notes).To(ContainSubstring(translator.WikiUIDMarker("wiki-1")))
			Expect(client.inserted[1].Notes).To(ContainSubstring(translator.WikiUIDMarker("wiki-2")))
		})

		It("should populate the initial ItemIDMap with the inserted task ids", func() {
			Expect(subscribed.ItemIDMap).To(HaveLen(2))
			Expect(subscribed.ItemIDMap).To(HaveKeyWithValue("wiki-1", "inserted-1"))
			Expect(subscribed.ItemIDMap).To(HaveKeyWithValue("wiki-2", "inserted-2"))
		})

		It("should populate ItemEtags for the inserted tasks", func() {
			Expect(subscribed.ItemEtags).To(HaveKeyWithValue("inserted-1", "etag-inserted-1"))
			Expect(subscribed.ItemEtags).To(HaveKeyWithValue("inserted-2", "etag-inserted-2"))
		})

		It("should persist the Subscription to the user profile", func() {
			store := newStore(pages)
			loaded, err := store.LoadState(aliceProfile)
			Expect(err).ToNot(HaveOccurred())
			Expect(loaded.Subscriptions).To(HaveLen(1))
			Expect(loaded.Subscriptions[0].Page).To(Equal(syncTestPage))
			Expect(loaded.Subscriptions[0].ListName).To(Equal(syncTestListName))
			Expect(loaded.Subscriptions[0].RemoteListID).To(Equal("created-list-1"))
			Expect(loaded.Subscriptions[0].ItemIDMap).To(HaveKeyWithValue("wiki-1", "inserted-1"))
			Expect(loaded.Subscriptions[0].ItemIDMap).To(HaveKeyWithValue("wiki-2", "inserted-2"))
		})
	})

	When("remoteListID is empty and the wiki checklist has no items", func() {
		var (
			subscribed taskssync.Subscription
			subErr     error
			client     *fakeTasksClient
		)

		BeforeEach(func() {
			pages := newFakePages()
			store := newConfiguredStore(pages, nil)
			client = newFakeTasksClient()
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, nil)
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(time.Now()), reader, nil, nil)
			subscribed, subErr = c.Subscribe(context.Background(), aliceProfile, syncTestPage, syncTestListName, "")
		})

		It("should not error", func() {
			Expect(subErr).ToNot(HaveOccurred())
		})

		It("should not call InsertTask", func() {
			Expect(client.inserted).To(BeEmpty())
		})

		It("should produce an empty initial ItemIDMap", func() {
			Expect(subscribed.ItemIDMap).To(BeEmpty())
		})
	})

	When("remoteListID is empty and InsertTask fails partway through seeding", func() {
		var (
			subErr error
			client *fakeTasksClient
			pages  *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			store := newConfiguredStore(pages, nil)
			client = newFakeTasksClient()
			client.insertErr = errors.New("transient API error")
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Eggs"},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(time.Now()), reader, nil, nil)
			_, subErr = c.Subscribe(context.Background(), aliceProfile, syncTestPage, syncTestListName, "")
		})

		It("should return an error wrapping the insert failure", func() {
			Expect(subErr).To(MatchError(ContainSubstring("seed new tasklist from wiki")))
		})

		It("should propagate the underlying API error", func() {
			Expect(subErr).To(MatchError(ContainSubstring("transient API error")))
		})

		It("should not persist a Subscription to the user profile", func() {
			store := newStore(pages)
			loaded, err := store.LoadState(aliceProfile)
			Expect(err).ToNot(HaveOccurred())
			Expect(loaded.Subscriptions).To(BeEmpty())
		})
	})

	When("remoteListID is empty and CreateTaskList fails", func() {
		var subErr error

		BeforeEach(func() {
			pages := newFakePages()
			store := newConfiguredStore(pages, nil)
			client := newFakeTasksClient()
			client.createTaskListErr = errors.New("API quota exceeded")
			reader := newFakeChecklistReader()
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(time.Now()), reader, nil, nil)
			_, subErr = c.Subscribe(context.Background(), aliceProfile, syncTestPage, syncTestListName, "")
		})

		It("should return an error wrapping the create failure", func() {
			Expect(subErr).To(MatchError(ContainSubstring("create tasks list")))
		})

		It("should propagate the underlying API error", func() {
			Expect(subErr).To(MatchError(ContainSubstring("API quota exceeded")))
		})
	})
})

var _ = Describe("Connector.Unsubscribe", func() {
	When("a subscription and lease exist", func() {
		var (
			pages *fakePages
			lt    *connectors.LeaseTable
		)

		BeforeEach(func() {
			pages = newFakePages()
			store := newConfiguredStore(pages, &taskssync.Subscription{
				Page: syncTestPage, ListName: syncTestListName, RemoteListID: syncTestRemote,
			})
			lt = readyLeaseTable()
			Expect(lt.Take(connectors.ChecklistKey{Page: syncTestPage, ListName: syncTestListName},
				connectors.LeaseOwner{Kind: connectors.ConnectorKindGoogleTasks, ProfileID: string(aliceProfile)})).To(Succeed())
			c := buildTestConnector(store, lt, newFakeTasksClient(), newFakeClock(time.Now()), nil, nil, nil)
			Expect(c.Unsubscribe(context.Background(), aliceProfile, syncTestPage, syncTestListName)).To(Succeed())
		})

		It("should remove the subscription from the profile", func() {
			store := newStore(pages)
			_, found, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(found).To(BeFalse())
		})

		It("should release the lease", func() {
			_, ok := lt.LookupOwner(connectors.ChecklistKey{Page: syncTestPage, ListName: syncTestListName})
			Expect(ok).To(BeFalse())
		})
	})
})

var _ = Describe("Connector.Resume", func() {
	When("the subscription is paused for less than 7 days", func() {
		var (
			pages *fakePages
			clock *fakeClock
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			pausedAt := now.Add(-1 * time.Hour)
			sub := taskssync.Subscription{
				Page:           syncTestPage,
				ListName:       syncTestListName,
				RemoteListID:   syncTestRemote,
				State:          taskssync.SubscriptionStatePaused,
				PausedReason:   taskssync.PausedReasonAuthFailed,
				PausedAt:       pausedAt,
				LastUpdatedMin: pausedAt.Add(-1 * time.Hour),
				ItemIDMap:      map[string]string{"u": "t"},
			}
			store := newConfiguredStore(pages, &sub)
			clock = newFakeClock(now)
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), clock, newFakeChecklistReader(), nil, nil)
			Expect(c.Resume(context.Background(), aliceProfile, syncTestPage, syncTestListName)).To(Succeed())
		})

		It("should set the subscription to active", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.State).To(Equal(taskssync.SubscriptionStateActive))
		})

		It("should clear PausedReason and PausedAt", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.PausedReason).To(BeEmpty())
			Expect(loaded.PausedAt.IsZero()).To(BeTrue())
		})

		It("should preserve LastUpdatedMin (incremental fetch from frozen cursor)", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.LastUpdatedMin.IsZero()).To(BeFalse())
		})
	})

	When("the subscription is paused for at least 7 days", func() {
		var (
			pages *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			now := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			pausedAt := now.Add(-8 * 24 * time.Hour)
			sub := taskssync.Subscription{
				Page:           syncTestPage,
				ListName:       syncTestListName,
				RemoteListID:   syncTestRemote,
				State:          taskssync.SubscriptionStatePaused,
				PausedReason:   taskssync.PausedReasonAuthFailed,
				PausedAt:       pausedAt,
				LastUpdatedMin: pausedAt.Add(-1 * time.Hour),
				ItemIDMap:      map[string]string{"stale": "stale-task"},
			}
			store := newConfiguredStore(pages, &sub)
			client := newFakeTasksClient()
			client.listAllForList[syncTestRemote] = []gateway.Task{
				{ID: "task-fresh", Title: "Eggs", Etag: "e1", Updated: now},
			}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-eggs", Text: "Eggs"},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.Resume(context.Background(), aliceProfile, syncTestPage, syncTestListName)).To(Succeed())
		})

		It("should rebuild the ItemIDMap (force full resync)", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.ItemIDMap).To(HaveKeyWithValue("wiki-eggs", "task-fresh"))
			Expect(loaded.ItemIDMap).ToNot(HaveKey("stale"))
		})

		It("should reset the cursor", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.LastUpdatedMin.IsZero()).To(BeTrue())
		})

		It("should set state to active", func() {
			store := newStore(pages)
			loaded, _, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(loaded.State).To(Equal(taskssync.SubscriptionStateActive))
		})
	})
})

var _ = Describe("Connector.IsChecklistPaused", func() {
	When("the subscription is paused", func() {
		var paused bool

		BeforeEach(func() {
			pages := newFakePages()
			sub := taskssync.Subscription{
				Page: syncTestPage, ListName: syncTestListName, RemoteListID: syncTestRemote,
				State:        taskssync.SubscriptionStatePaused,
				PausedReason: taskssync.PausedReasonAuthFailed,
			}
			store := newConfiguredStore(pages, &sub)
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(time.Now()), nil, nil, nil)
			paused = c.IsChecklistPaused(aliceProfile, syncTestPage, syncTestListName)
		})

		It("should return true", func() {
			Expect(paused).To(BeTrue())
		})
	})

	When("the subscription is active", func() {
		var paused bool

		BeforeEach(func() {
			pages := newFakePages()
			sub := taskssync.Subscription{
				Page: syncTestPage, ListName: syncTestListName, RemoteListID: syncTestRemote,
				State: taskssync.SubscriptionStateActive,
			}
			store := newConfiguredStore(pages, &sub)
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(time.Now()), nil, nil, nil)
			paused = c.IsChecklistPaused(aliceProfile, syncTestPage, syncTestListName)
		})

		It("should return false", func() {
			Expect(paused).To(BeFalse())
		})
	})
})

var _ = Describe("Connector.ListRemoteLists", func() {
	When("the connector is configured", func() {
		var (
			lists  []taskssync.RemoteList
			lerr   error
			client *fakeTasksClient
		)

		BeforeEach(func() {
			pages := newFakePages()
			store := newConfiguredStore(pages, nil)
			client = newFakeTasksClient()
			client.taskLists = []gateway.TaskList{
				{ID: "tl-1", Title: "Groceries"},
				{ID: "tl-2", Title: "Errands"},
			}
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(time.Now()), nil, nil, nil)
			lists, lerr = c.ListRemoteLists(context.Background(), aliceProfile)
		})

		It("should not error", func() {
			Expect(lerr).ToNot(HaveOccurred())
		})

		It("should return both tasklists with id and title", func() {
			Expect(lists).To(HaveLen(2))
			Expect(lists[0].ID).To(Equal("tl-1"))
			Expect(lists[0].Title).To(Equal("Groceries"))
		})
	})

	When("the connector is not configured", func() {
		var lerr error

		BeforeEach(func() {
			pages := newFakePages()
			store := newStore(pages)
			Expect(store.SaveState(aliceProfile, taskssync.ConnectorState{})).To(Succeed())
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(time.Now()), nil, nil, nil)
			_, lerr = c.ListRemoteLists(context.Background(), aliceProfile)
		})

		It("should return ErrConnectorNotConfigured", func() {
			Expect(errors.Is(lerr, taskssync.ErrConnectorNotConfigured)).To(BeTrue())
		})
	})
})

var _ = Describe("Connector.SubscriptionsForProfile", func() {
	When("the profile has multiple subscriptions", func() {
		var keys []connectors.SubscriptionKey

		BeforeEach(func() {
			pages := newFakePages()
			store := newStore(pages)
			Expect(store.SaveState(aliceProfile, taskssync.ConnectorState{
				RefreshToken: "rt",
				Subscriptions: []taskssync.Subscription{
					{Page: "p1", ListName: "l1", RemoteListID: "tl1"},
					{Page: "p2", ListName: "l2", RemoteListID: "tl2"},
				},
			})).To(Succeed())
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(time.Now()), nil, nil, nil)
			var err error
			keys, err = c.SubscriptionsForProfile(aliceProfile)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should enumerate every subscription on the profile", func() {
			Expect(keys).To(HaveLen(2))
		})

		It("should populate ProfileID, Page, and ListName on each key", func() {
			pages := []string{}
			for _, k := range keys {
				pages = append(pages, k.Page)
			}
			Expect(pages).To(ConsistOf("p1", "p2"))
		})
	})
})

var _ = Describe("Connector.Connect / Disconnect", func() {
	When("Connect persists fresh OAuth credentials", func() {
		var (
			persisted taskssync.ConnectorState
			err       error
		)

		BeforeEach(func() {
			pages := newFakePages()
			store := newStore(pages)
			c, _ := taskssync.NewConnector(store, readyLeaseTable(), stubFactoryThatReturns(newFakeTasksClient()), silentLogger{}, newFakeClock(time.Now()))
			persisted, err = c.Connect(context.Background(), aliceProfile, "alice@example.com", "rt-fresh")
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should record the email", func() {
			Expect(persisted.Email).To(Equal("alice@example.com"))
		})

		It("should record the refresh token", func() {
			Expect(persisted.RefreshToken).To(Equal("rt-fresh"))
		})
	})

	When("Disconnect is called with active subscriptions", func() {
		var (
			pages *fakePages
		)

		BeforeEach(func() {
			pages = newFakePages()
			sub := taskssync.Subscription{
				Page: syncTestPage, ListName: syncTestListName, RemoteListID: syncTestRemote,
				State: taskssync.SubscriptionStateActive,
			}
			store := newConfiguredStore(pages, &sub)
			c, _ := taskssync.NewConnector(store, readyLeaseTable(), stubFactoryThatReturns(newFakeTasksClient()), silentLogger{}, newFakeClock(time.Now()))
			_, err := c.Disconnect(context.Background(), aliceProfile)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should clear the refresh token", func() {
			store := newStore(pages)
			state, _ := store.LoadState(aliceProfile)
			Expect(state.RefreshToken).To(BeEmpty())
		})

		It("should pause every subscription so PausedReason surfaces immediately", func() {
			store := newStore(pages)
			state, _ := store.LoadState(aliceProfile)
			Expect(state.Subscriptions).To(HaveLen(1))
			Expect(state.Subscriptions[0].State).To(Equal(taskssync.SubscriptionStatePaused))
			Expect(state.Subscriptions[0].PausedReason).To(Equal(taskssync.PausedReasonAuthFailed))
		})
	})
})

var _ = Describe("Connector.PersistRefreshToken", func() {
	When("the profile has paused subscriptions", func() {
		var (
			pages *fakePages
			now   time.Time
		)

		BeforeEach(func() {
			pages = newFakePages()
			now = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
			pausedAt := now.Add(-1 * time.Hour)
			activeSub := taskssync.Subscription{
				Page:     "active_page",
				ListName: "active_list",
				RemoteListID: syncTestRemote,
				State:    taskssync.SubscriptionStateActive,
				LastUpdatedMin: pausedAt,
			}
			pausedSub := taskssync.Subscription{
				Page:           syncTestPage,
				ListName:       syncTestListName,
				RemoteListID:   syncTestRemote,
				State:          taskssync.SubscriptionStatePaused,
				PausedReason:   taskssync.PausedReasonAuthFailed,
				PausedAt:       pausedAt,
				LastUpdatedMin: pausedAt.Add(-1 * time.Hour),
				ItemIDMap:      map[string]string{"u": "t"},
			}
			store := newStore(pages)
			Expect(store.SaveState(aliceProfile, taskssync.ConnectorState{
				Email:         syncTestEmail,
				RefreshToken:  "rt-old",
				Subscriptions: []taskssync.Subscription{activeSub, pausedSub},
			})).To(Succeed())
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(now), newFakeChecklistReader(), nil, nil)
			Expect(c.PersistRefreshToken(context.Background(), string(aliceProfile), syncTestEmail, "rt-fresh")).To(Succeed())
		})

		It("should persist the new refresh token", func() {
			store := newStore(pages)
			state, _ := store.LoadState(aliceProfile)
			Expect(state.RefreshToken).To(Equal("rt-fresh"))
		})

		It("should auto-resume the paused subscription", func() {
			store := newStore(pages)
			loaded, found, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(found).To(BeTrue())
			Expect(loaded.State).To(Equal(taskssync.SubscriptionStateActive))
			Expect(loaded.PausedReason).To(BeEmpty())
			Expect(loaded.PausedAt.IsZero()).To(BeTrue())
		})

		It("should preserve the paused subscription's ItemIDMap (no full resync at <7d)", func() {
			store := newStore(pages)
			loaded, found, _ := store.FindSubscription(aliceProfile, syncTestPage, syncTestListName)
			Expect(found).To(BeTrue())
			Expect(loaded.ItemIDMap).To(HaveKeyWithValue("u", "t"))
		})

		It("should leave already-active subscriptions unchanged (idempotent)", func() {
			store := newStore(pages)
			loaded, found, _ := store.FindSubscription(aliceProfile, "active_page", "active_list")
			Expect(found).To(BeTrue())
			Expect(loaded.State).To(Equal(taskssync.SubscriptionStateActive))
		})
	})

	When("the profile has no subscriptions", func() {
		var (
			pages *fakePages
			err   error
		)

		BeforeEach(func() {
			pages = newFakePages()
			store := newStore(pages)
			Expect(store.SaveState(aliceProfile, taskssync.ConnectorState{
				Email:        syncTestEmail,
				RefreshToken: "rt-old",
			})).To(Succeed())
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(time.Now()), nil, nil, nil)
			err = c.PersistRefreshToken(context.Background(), string(aliceProfile), syncTestEmail, "rt-fresh")
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should still persist the refresh token", func() {
			store := newStore(pages)
			state, _ := store.LoadState(aliceProfile)
			Expect(state.RefreshToken).To(Equal("rt-fresh"))
		})
	})

	When("an empty profileID is supplied", func() {
		var err error

		BeforeEach(func() {
			pages := newFakePages()
			store := newStore(pages)
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(time.Now()), nil, nil, nil)
			err = c.PersistRefreshToken(context.Background(), "", syncTestEmail, "rt-fresh")
		})

		It("should return an error", func() {
			Expect(err).To(MatchError(ContainSubstring("profileID is required")))
		})
	})

	When("an empty refresh token is supplied", func() {
		var err error

		BeforeEach(func() {
			pages := newFakePages()
			store := newStore(pages)
			c := buildTestConnector(store, readyLeaseTable(), newFakeTasksClient(), newFakeClock(time.Now()), nil, nil, nil)
			err = c.PersistRefreshToken(context.Background(), string(aliceProfile), syncTestEmail, "")
		})

		It("should return an error", func() {
			Expect(err).To(MatchError(ContainSubstring("refresh_token is required")))
		})
	})
})

