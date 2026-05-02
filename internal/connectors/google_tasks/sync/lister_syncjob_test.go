//revive:disable:dot-imports
package sync_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	taskssync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/sync"
)

var _ = Describe("ActiveSubscriptions.Lister", func() {
	var active *taskssync.ActiveSubscriptions

	BeforeEach(func() {
		active = taskssync.NewActiveSubscriptions()
	})

	When("the tracker is empty", func() {
		var listerFn connectors.SubscriptionLister

		BeforeEach(func() {
			listerFn = active.Lister()
		})

		It("should return an empty snapshot via the closure", func() {
			Expect(listerFn()).To(BeEmpty())
		})
	})

	When("subscriptions have been added", func() {
		var (
			listerFn connectors.SubscriptionLister
			k1, k2   connectors.SubscriptionKey
		)

		BeforeEach(func() {
			k1 = connectors.SubscriptionKey{ProfileID: "alice", Page: "p1", ListName: "l1"}
			k2 = connectors.SubscriptionKey{ProfileID: "alice", Page: "p2", ListName: "l2"}
			active.Add(k1)
			active.Add(k2)
			listerFn = active.Lister()
		})

		It("should return all added keys when the closure is called", func() {
			Expect(listerFn()).To(ConsistOf(k1, k2))
		})

		It("should reflect later removals on subsequent calls", func() {
			active.Remove(k1)
			Expect(listerFn()).To(ConsistOf(k2))
		})
	})
})

var _ = Describe("TasksOutboundSyncJob", func() {
	var (
		pages  *fakePages
		clock  *fakeClock
		client *fakeTasksClient
	)

	BeforeEach(func() {
		clock = newFakeClock(time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC))
		pages = newFakePages()
		client = newFakeTasksClient()
	})

	Describe("GetName", func() {
		It("should return the registered queue name constant", func() {
			store := newConfiguredStore(pages, nil)
			c := newConnector(store, readyLeaseTable(), client, clock, nil, nil, nil)
			job := taskssync.NewTasksOutboundSyncJob(c, aliceProfile, syncTestPage, syncTestListName)
			Expect(job.GetName()).To(Equal(taskssync.TasksOutboundSyncJobName))
		})
	})

	Describe("Execute", func() {
		When("the subscription does not exist for the given key", func() {
			var execErr error

			BeforeEach(func() {
				// Store has no subscription for (syncTestPage, syncTestListName).
				store := newConfiguredStore(pages, nil)
				c := newConnector(store, readyLeaseTable(), client, clock, nil, nil, nil)
				job := taskssync.NewTasksOutboundSyncJob(c, aliceProfile, syncTestPage, syncTestListName)
				execErr = job.Execute()
			})

			It("should return nil (no-op — nothing to sync)", func() {
				Expect(execErr).ToNot(HaveOccurred())
			})
		})

		When("the subscription exists and the tasks list is empty", func() {
			var execErr error

			BeforeEach(func() {
				sub := &taskssync.Subscription{
					Page:         syncTestPage,
					ListName:     syncTestListName,
					RemoteListID: syncTestRemote,
					State:        taskssync.SubscriptionStateActive,
					SubscribedAt: clock.Now(),
				}
				// Empty tasks list so the sync is a no-op push.
				client.listAllForList[syncTestRemote] = nil
				reader := newFakeChecklistReader()
				reader.Set(syncTestPage, syncTestListName, nil)
				store := newConfiguredStore(pages, sub)
				c := newConnector(store, readyLeaseTable(), client, clock, reader, nil, nil)
				job := taskssync.NewTasksOutboundSyncJob(c, aliceProfile, syncTestPage, syncTestListName)
				execErr = job.Execute()
			})

			It("should succeed when there is nothing to sync", func() {
				Expect(execErr).ToNot(HaveOccurred())
			})
		})
	})
})
