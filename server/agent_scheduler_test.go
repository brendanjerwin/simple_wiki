//revive:disable:dot-imports
package server_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// fakeFrontmatterIndex returns a fixed list of pages for QueryKeyExistence
// regardless of the requested key. Sufficient for AgentScheduler tests where
// the only key the scheduler queries is "agent.schedules".
type fakeFrontmatterIndex struct {
	pages []wikipage.PageIdentifier
}

func (*fakeFrontmatterIndex) QueryExactMatch(_, _ string) []wikipage.PageIdentifier {
	return nil
}

func (f *fakeFrontmatterIndex) QueryKeyExistence(_ string) []wikipage.PageIdentifier {
	return f.pages
}

func (*fakeFrontmatterIndex) GetValue(_ wikipage.PageIdentifier, _ string) string {
	return ""
}

// fakeCronRegistrar records every Schedule/Remove call for assertions.
type fakeCronRegistrar struct {
	mu        sync.Mutex
	scheduled []fakeCronSchedule
	removed   []int
	nextID    int
}

type fakeCronSchedule struct {
	cron string
	job  interface{ GetName() string }
	id   int
}

func (f *fakeCronRegistrar) Schedule(cron string, job server.CronJob) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	f.scheduled = append(f.scheduled, fakeCronSchedule{cron: cron, job: job, id: f.nextID})
	return f.nextID, nil
}

func (f *fakeCronRegistrar) Remove(id int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, id)
}

var _ = Describe("AgentScheduler.LoadAll", func() {
	var (
		pages    *fakePageStore
		store    *server.AgentScheduleStore
		idx      *fakeFrontmatterIndex
		cronReg  *fakeCronRegistrar
		dispatch *fakeDispatcher
	)

	BeforeEach(func() {
		pages = newFakePageStore()
		store = server.NewAgentScheduleStore(pages)
		idx = &fakeFrontmatterIndex{}
		cronReg = &fakeCronRegistrar{}
		dispatch = &fakeDispatcher{}
	})

	Describe("when there are three pages with mixed enabled/disabled and bad cron", func() {
		BeforeEach(func() {
			Expect(store.Upsert("a", &apiv1.AgentSchedule{
				Id: "x", Cron: "0 0 9 * * 1", Prompt: "p", Enabled: true,
			})).To(Succeed())
			Expect(store.Upsert("b", &apiv1.AgentSchedule{
				Id: "y", Cron: "0 0 9 * * 1", Prompt: "p", Enabled: false,
			})).To(Succeed())
			Expect(store.Upsert("c", &apiv1.AgentSchedule{
				Id: "z", Cron: "not a real cron", Prompt: "p", Enabled: true,
			})).To(Succeed())

			idx.pages = []wikipage.PageIdentifier{"a", "b", "c"}

			scheduler := server.NewAgentScheduler(store, dispatch, idx, cronReg, time.Minute)
			Expect(scheduler.LoadAll()).To(Succeed())
		})

		It("should register exactly one schedule (only the valid+enabled one)", func() {
			Expect(cronReg.scheduled).To(HaveLen(1))
		})

		It("should register the cron expression from the schedule", func() {
			Expect(cronReg.scheduled[0].cron).To(Equal("0 0 9 * * 1"))
		})

		It("should register an AgentTurn job", func() {
			Expect(cronReg.scheduled[0].job.GetName()).To(Equal("AgentTurn"))
		})
	})
})

var _ = Describe("AgentScheduler.UnregisterPage", func() {
	var (
		pages    *fakePageStore
		store    *server.AgentScheduleStore
		idx      *fakeFrontmatterIndex
		cronReg  *fakeCronRegistrar
		dispatch *fakeDispatcher
	)

	BeforeEach(func() {
		pages = newFakePageStore()
		store = server.NewAgentScheduleStore(pages)
		idx = &fakeFrontmatterIndex{}
		cronReg = &fakeCronRegistrar{}
		dispatch = &fakeDispatcher{}
	})

	Describe("when one page has registered schedules and another also has one", func() {
		var keepers, removed []int

		BeforeEach(func() {
			Expect(store.Upsert("doomed", &apiv1.AgentSchedule{
				Id: "a", Cron: "0 0 9 * * 1", Prompt: "p", Enabled: true,
			})).To(Succeed())
			Expect(store.Upsert("doomed", &apiv1.AgentSchedule{
				Id: "b", Cron: "0 0 9 * * 2", Prompt: "p", Enabled: true,
			})).To(Succeed())
			Expect(store.Upsert("keeper", &apiv1.AgentSchedule{
				Id: "z", Cron: "0 0 9 * * 3", Prompt: "p", Enabled: true,
			})).To(Succeed())

			idx.pages = []wikipage.PageIdentifier{"doomed", "keeper"}

			scheduler := server.NewAgentScheduler(store, dispatch, idx, cronReg, time.Minute)
			Expect(scheduler.LoadAll()).To(Succeed())

			// Capture entry IDs for the keeper before unregister.
			keepers = nil
			for _, e := range cronReg.scheduled {
				if e.cron == "0 0 9 * * 3" {
					keepers = append(keepers, e.id)
				}
			}

			scheduler.UnregisterPage("doomed")
			removed = append([]int{}, cronReg.removed...)
		})

		It("should remove both 'doomed' page entries", func() {
			Expect(removed).To(HaveLen(2))
		})

		It("should not remove the 'keeper' page entry", func() {
			for _, kept := range keepers {
				Expect(removed).NotTo(ContainElement(kept))
			}
		})
	})

	Describe("when called for a page that has no registered schedules", func() {
		BeforeEach(func() {
			scheduler := server.NewAgentScheduler(store, dispatch, idx, cronReg, time.Minute)
			scheduler.UnregisterPage("never_registered")
		})

		It("should not call the cron registrar", func() {
			Expect(cronReg.removed).To(BeEmpty())
		})
	})
})
