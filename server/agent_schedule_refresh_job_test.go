//revive:disable:dot-imports
package server_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("AgentScheduleRefreshJob", func() {
	var (
		pages     *fakePageStore
		store     *server.AgentScheduleStore
		idx       *fakeFrontmatterIndex
		cronReg   *fakeCronRegistrar
		dispatch  *fakeDispatcher
		scheduler *server.AgentScheduler
	)

	BeforeEach(func() {
		pages = newFakePageStore()
		store = server.NewAgentScheduleStore(pages)
		idx = &fakeFrontmatterIndex{}
		cronReg = &fakeCronRegistrar{}
		dispatch = &fakeDispatcher{}
		scheduler = server.NewAgentScheduler(store, dispatch, idx, cronReg, time.Minute)
	})

	Describe("GetName", func() {
		var job *server.AgentScheduleRefreshJob

		BeforeEach(func() {
			job = server.NewAgentScheduleRefreshJob(scheduler, "any-page")
		})

		It("should return AgentScheduleRefresh", func() {
			Expect(job.GetName()).To(Equal("AgentScheduleRefresh"))
		})
	})

	Describe("Execute when the page has a valid enabled schedule", func() {
		var (
			job *server.AgentScheduleRefreshJob
			err error
		)

		BeforeEach(func() {
			Expect(store.Upsert("page-a", &apiv1.AgentSchedule{
				Id: "x", Cron: "0 0 9 * * 1", Prompt: "p", Enabled: true,
			})).To(Succeed())
			job = server.NewAgentScheduleRefreshJob(scheduler, "page-a")
			err = job.Execute()
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should refresh the supplied page (one cron registration occurs)", func() {
			Expect(cronReg.scheduled).To(HaveLen(1))
		})

		It("should register the cron expression from the schedule with the default UTC prefix", func() {
			Expect(cronReg.scheduled[0].cron).To(Equal("CRON_TZ=UTC 0 0 9 * * 1"))
		})
	})

	Describe("Execute when called for a different page than the one with schedules", func() {
		var (
			job *server.AgentScheduleRefreshJob
			err error
		)

		BeforeEach(func() {
			Expect(store.Upsert("page-a", &apiv1.AgentSchedule{
				Id: "x", Cron: "0 0 9 * * 1", Prompt: "p", Enabled: true,
			})).To(Succeed())
			job = server.NewAgentScheduleRefreshJob(scheduler, "page-b")
			err = job.Execute()
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not register any cron entries (refresh targeted page-b, not page-a)", func() {
			Expect(cronReg.scheduled).To(BeEmpty())
		})
	})

	Describe("Execute when the underlying store returns malformed schedules", func() {
		var (
			job *server.AgentScheduleRefreshJob
			err error
		)

		BeforeEach(func() {
			// Inject malformed agent.schedules data so List() (and therefore
			// Refresh) returns an error from decodeSchedules.
			Expect(pages.WriteFrontMatter("bad-page", wikipage.FrontMatter{
				"agent": map[string]any{
					"schedules": "not-a-list",
				},
			})).To(Succeed())
			job = server.NewAgentScheduleRefreshJob(scheduler, "bad-page")
			err = job.Execute()
		})

		It("should return the error from Refresh", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should not register any cron entries", func() {
			Expect(cronReg.scheduled).To(BeEmpty())
		})
	})
})
