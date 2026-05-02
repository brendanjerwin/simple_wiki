//revive:disable:dot-imports
package sync_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
				ItemEtags:    map[string]string{"task-1": "etag-stale"},
			}
			store := newConfiguredStore(pages, &sub)
			client = newFakeTasksClient()
			client.patchErrors = []error{gateway.ErrPreconditionFailed}
			reader := newFakeChecklistReader()
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Edited"},
			})
			c := buildTestConnector(store, readyLeaseTable(), client, newFakeClock(now), reader, nil, nil)
			Expect(c.Sync(context.Background(), subscriptionKey())).To(Succeed())
		})

		It("should retry the patch once with empty If-Match", func() {
			Expect(client.patched).To(HaveLen(2))
			Expect(client.patched[0].Etag).To(Equal("etag-stale"))
			Expect(client.patched[1].Etag).To(Equal(""))
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

	When("remoteListID is empty (Bind to a new Tasks list)", func() {
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
			reader.Set(syncTestPage, syncTestListName, []*apiv1.ChecklistItem{
				{Uid: "wiki-1", Text: "Eggs"},
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

		It("should produce an empty initial ItemIDMap (fresh list has no tasks)", func() {
			Expect(subscribed.ItemIDMap).To(BeEmpty())
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

