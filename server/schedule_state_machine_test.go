//revive:disable:dot-imports
package server_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
)

var _ = Describe("ScheduleStateMachine", func() {
	Describe("ValidateTransition", func() {
		const (
			unspecified = apiv1.ScheduleStatus_SCHEDULE_STATUS_UNSPECIFIED
			running     = apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING
			ok          = apiv1.ScheduleStatus_SCHEDULE_STATUS_OK
			errored     = apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR
			timeout     = apiv1.ScheduleStatus_SCHEDULE_STATUS_TIMEOUT
		)

		legalCases := []struct {
			name string
			from apiv1.ScheduleStatus
			to   apiv1.ScheduleStatus
		}{
			{"UNSPECIFIED -> RUNNING (initial fire)", unspecified, running},
			{"RUNNING -> OK (success)", running, ok},
			{"RUNNING -> ERROR (failed)", running, errored},
			{"RUNNING -> TIMEOUT (max_turns hit)", running, timeout},
			{"OK -> RUNNING (next fire)", ok, running},
			{"ERROR -> RUNNING (next fire)", errored, running},
			{"TIMEOUT -> RUNNING (next fire)", timeout, running},
		}

		for _, tc := range legalCases {
			tc := tc
			Describe("legal: "+tc.name, func() {
				var err error

				BeforeEach(func() {
					err = server.ValidateScheduleTransition(tc.from, tc.to)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})
			})
		}

		// Build the full set of states for the illegal sweep.
		allStates := []apiv1.ScheduleStatus{unspecified, running, ok, errored, timeout}

		legal := map[[2]apiv1.ScheduleStatus]bool{}
		for _, c := range legalCases {
			legal[[2]apiv1.ScheduleStatus{c.from, c.to}] = true
		}

		for _, from := range allStates {
			for _, to := range allStates {
				if legal[[2]apiv1.ScheduleStatus{from, to}] {
					continue
				}
				from, to := from, to
				Describe("illegal: "+from.String()+" -> "+to.String(), func() {
					var err error

					BeforeEach(func() {
						err = server.ValidateScheduleTransition(from, to)
					})

					It("should return an error", func() {
						Expect(err).To(HaveOccurred())
					})

					It("should return an IllegalScheduleTransitionError", func() {
						var typed *server.IllegalScheduleTransitionError
						Expect(errors.As(err, &typed)).To(BeTrue())
					})

					It("should expose the From state", func() {
						var typed *server.IllegalScheduleTransitionError
						Expect(errors.As(err, &typed)).To(BeTrue())
						Expect(typed.From).To(Equal(from))
					})

					It("should expose the To state", func() {
						var typed *server.IllegalScheduleTransitionError
						Expect(errors.As(err, &typed)).To(BeTrue())
						Expect(typed.To).To(Equal(to))
					})
				})
			}
		}
	})
})
