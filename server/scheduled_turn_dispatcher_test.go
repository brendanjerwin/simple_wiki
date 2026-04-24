//revive:disable:dot-imports
package server_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
)

var _ = Describe("ScheduledTurnDispatcher", func() {
	var dispatcher *server.ScheduledTurnDispatcher

	BeforeEach(func() {
		dispatcher = server.NewScheduledTurnDispatcher()
	})

	Describe("Dispatch", func() {
		Describe("when there are no subscribers", func() {
			var dispatchErr error

			BeforeEach(func() {
				_, dispatchErr = dispatcher.Dispatch(&apiv1.ScheduledTurnRequest{
					RequestId: "r1", Page: "p", Prompt: "do",
				})
			})

			It("should return an error", func() {
				Expect(dispatchErr).To(HaveOccurred())
			})
		})

		Describe("when one subscriber is connected", func() {
			var requests <-chan *apiv1.ScheduledTurnRequest
			var unsubscribe func()
			var completion <-chan *server.ScheduledTurnOutcome
			var dispatchErr error

			BeforeEach(func() {
				requests, unsubscribe = dispatcher.Subscribe()
				completion, dispatchErr = dispatcher.Dispatch(&apiv1.ScheduledTurnRequest{
					RequestId: "r2", Page: "p", Prompt: "do",
				})
			})

			AfterEach(func() {
				unsubscribe()
			})

			It("should not return an error", func() {
				Expect(dispatchErr).NotTo(HaveOccurred())
			})

			It("should forward the request to the subscriber", func() {
				Eventually(requests).Should(Receive())
			})

			Describe("when Complete is called for the request", func() {
				BeforeEach(func() {
					// Drain the request from the subscriber channel so the system is
					// in a steady state.
					Eventually(requests).Should(Receive())
					Expect(dispatcher.Complete(&apiv1.CompleteScheduledTurnRequest{
						RequestId:       "r2",
						TerminalStatus:  apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
						DurationSeconds: 12,
					})).To(Succeed())
				})

				It("should deliver an outcome on the completion channel", func() {
					var outcome *server.ScheduledTurnOutcome
					Eventually(completion, 2*time.Second).Should(Receive(&outcome))
					Expect(outcome).NotTo(BeNil())
					Expect(outcome.TerminalStatus).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_OK))
					Expect(outcome.DurationSeconds).To(Equal(int32(12)))
				})
			})

			Describe("when Complete is called twice for the same request", func() {
				var firstErr, secondErr error

				BeforeEach(func() {
					Eventually(requests).Should(Receive())
					firstErr = dispatcher.Complete(&apiv1.CompleteScheduledTurnRequest{
						RequestId: "r2", TerminalStatus: apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
					})
					secondErr = dispatcher.Complete(&apiv1.CompleteScheduledTurnRequest{
						RequestId: "r2", TerminalStatus: apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
					})
				})

				It("should accept the first completion", func() {
					Expect(firstErr).NotTo(HaveOccurred())
				})

				It("should reject the second completion", func() {
					Expect(secondErr).To(HaveOccurred())
				})
			})
		})

		Describe("Complete with an unknown request_id", func() {
			var unsubscribe func()
			var err error

			BeforeEach(func() {
				_, unsubscribe = dispatcher.Subscribe()
				err = dispatcher.Complete(&apiv1.CompleteScheduledTurnRequest{
					RequestId: "never-dispatched",
				})
			})

			AfterEach(func() {
				unsubscribe()
			})

			It("should return an error (orphan complete)", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
