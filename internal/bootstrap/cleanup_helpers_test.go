//revive:disable:dot-imports
package bootstrap

import (
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("composeCleanup", func() {
	When("both functions are non-nil", func() {
		var calls []string
		var cleanup func()

		BeforeEach(func() {
			calls = nil
			first := func() {
				calls = append(calls, "first")
			}
			second := func() {
				calls = append(calls, "second")
			}
			cleanup = composeCleanup(first, second)
			cleanup()
		})

		It("should call both functions", func() {
			Expect(calls).To(HaveLen(2))
		})

		It("should call first before second", func() {
			Expect(calls).To(Equal([]string{"first", "second"}))
		})
	})

	When("first function is nil", func() {
		var secondCalled bool
		var cleanup func()

		BeforeEach(func() {
			secondCalled = false
			second := func() {
				secondCalled = true
			}
			cleanup = composeCleanup(nil, second)
			cleanup()
		})

		It("should call the second function", func() {
			Expect(secondCalled).To(BeTrue())
		})
	})

	When("second function is nil", func() {
		var firstCalled bool
		var cleanup func()

		BeforeEach(func() {
			firstCalled = false
			first := func() {
				firstCalled = true
			}
			cleanup = composeCleanup(first, nil)
			cleanup()
		})

		It("should call the first function", func() {
			Expect(firstCalled).To(BeTrue())
		})
	})

	When("both functions are nil", func() {
		var cleanup func()

		BeforeEach(func() {
			cleanup = composeCleanup(nil, nil)
		})

		It("should be callable without panicking", func() {
			Expect(cleanup).NotTo(Panic())
		})
	})
})

var _ = Describe("stopSiteCron", func() {
	When("site is nil", func() {
		var cleanup func()

		BeforeEach(func() {
			cleanup = stopSiteCron(nil)
		})

		It("should be callable without panicking", func() {
			Expect(cleanup).NotTo(Panic())
		})
	})

	When("site has a nil CronScheduler", func() {
		var cleanup func()

		BeforeEach(func() {
			site := &server.Site{}
			cleanup = stopSiteCron(site)
		})

		It("should be callable without panicking", func() {
			Expect(cleanup).NotTo(Panic())
		})
	})

	When("site has a started CronScheduler", func() {
		var cleanup func()
		var scheduler *jobs.CronScheduler

		BeforeEach(func() {
			logger := lumber.NewConsoleLogger(lumber.WARN)
			scheduler = jobs.NewCronScheduler(logger)
			scheduler.Start()
			site := &server.Site{CronScheduler: scheduler}
			cleanup = stopSiteCron(site)
		})

		It("should call Stop without panicking", func() {
			Expect(cleanup).NotTo(Panic())
		})
	})
})

var _ = Describe("drainScheduledTurns", func() {
	When("site is nil", func() {
		var cleanup func()

		BeforeEach(func() {
			cleanup = drainScheduledTurns(nil)
		})

		It("should be callable without panicking", func() {
			Expect(cleanup).NotTo(Panic())
		})
	})

	When("site has a nil ScheduledTurnDispatcher", func() {
		var cleanup func()

		BeforeEach(func() {
			site := &server.Site{}
			cleanup = drainScheduledTurns(site)
		})

		It("should be callable without panicking", func() {
			Expect(cleanup).NotTo(Panic())
		})
	})

	When("the dispatcher has no pending turns", func() {
		var cleanup func()

		BeforeEach(func() {
			logger := lumber.NewConsoleLogger(lumber.WARN)
			site := &server.Site{
				ScheduledTurnDispatcher: server.NewScheduledTurnDispatcher(),
				Logger:                  logger,
			}
			cleanup = drainScheduledTurns(site)
		})

		It("should be callable without panicking", func() {
			Expect(cleanup).NotTo(Panic())
		})
	})

	When("the dispatcher has pending turns", func() {
		var drainedOutcome *server.ScheduledTurnOutcome

		BeforeEach(func() {
			logger := lumber.NewConsoleLogger(lumber.WARN)
			dispatcher := server.NewScheduledTurnDispatcher()

			_, unsubscribe := dispatcher.Subscribe()
			DeferCleanup(unsubscribe)

			completion, err := dispatcher.Dispatch(&apiv1.ScheduledTurnRequest{
				RequestId: "test-drain-request",
				Page:      "test-page",
				Prompt:    "do something",
			})
			Expect(err).NotTo(HaveOccurred())

			site := &server.Site{
				ScheduledTurnDispatcher: dispatcher,
				Logger:                  logger,
			}
			drainScheduledTurns(site)()

			drainedOutcome = <-completion
		})

		It("should signal ERROR terminal status to the completion channel", func() {
			Expect(drainedOutcome.TerminalStatus).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR))
		})

		It("should include the shutdown reason in the error message", func() {
			Expect(drainedOutcome.ErrorMessage).To(ContainSubstring("process shutdown"))
		})
	})
})
