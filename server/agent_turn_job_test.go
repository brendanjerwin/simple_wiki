//revive:disable:dot-imports
package server_test

import (
	"errors"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
)

// fakeDispatcher records Dispatch calls and lets tests inject completion
// outcomes synchronously.
type fakeDispatcher struct {
	mu               sync.Mutex
	dispatched       []*apiv1.ScheduledTurnRequest
	dispatchErr      error
	completionToSend *server.ScheduledTurnOutcome
	// neverComplete, when true, returns a channel that never receives and is
	// never closed — used to exercise the hard-timeout path in awaitOutcome.
	neverComplete bool
}

func (f *fakeDispatcher) Dispatch(req *apiv1.ScheduledTurnRequest) (<-chan *server.ScheduledTurnOutcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dispatchErr != nil {
		return nil, f.dispatchErr
	}
	f.dispatched = append(f.dispatched, req)
	ch := make(chan *server.ScheduledTurnOutcome, 1)
	if f.neverComplete {
		return ch, nil
	}
	if f.completionToSend != nil {
		ch <- f.completionToSend
		close(ch)
	}
	return ch, nil
}

var _ = Describe("AgentTurnJob", func() {
	var (
		pages      *fakePageStore
		store      *server.AgentScheduleStore
		dispatcher *fakeDispatcher
		job        *server.AgentTurnJob
	)

	BeforeEach(func() {
		pages = newFakePageStore()
		store = server.NewAgentScheduleStore(pages)
		dispatcher = &fakeDispatcher{}
		Expect(store.Upsert("p", &apiv1.AgentSchedule{
			Id:       "s1",
			Cron:     "0 * * * * *",
			Prompt:   "do thing",
			MaxTurns: 10,
			Enabled:  true,
		})).To(Succeed())
	})

	Describe("GetName", func() {
		BeforeEach(func() {
			job = server.NewAgentTurnJob(store, dispatcher, "p", "s1", 5*time.Second)
		})

		It("should return AgentTurn (single shared queue)", func() {
			Expect(job.GetName()).To(Equal("AgentTurn"))
		})
	})

	Describe("Execute when the pool reports OK", func() {
		BeforeEach(func() {
			dispatcher.completionToSend = &server.ScheduledTurnOutcome{
				TerminalStatus:  apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				DurationSeconds: 7,
			}
			job = server.NewAgentTurnJob(store, dispatcher, "p", "s1", 5*time.Second)
			Expect(job.Execute()).To(Succeed())
		})

		It("should dispatch one request", func() {
			Expect(dispatcher.dispatched).To(HaveLen(1))
		})

		It("should populate the request page and prompt from the schedule", func() {
			req := dispatcher.dispatched[0]
			Expect(req.GetPage()).To(Equal("p"))
			Expect(req.GetPrompt()).To(Equal("do thing"))
			Expect(req.GetMaxTurns()).To(Equal(int32(10)))
		})

		It("should record terminal OK status on the schedule", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_OK))
		})

		It("should record the duration", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastDurationSeconds()).To(Equal(int32(7)))
		})
	})

	Describe("Execute when the pool reports ERROR", func() {
		BeforeEach(func() {
			dispatcher.completionToSend = &server.ScheduledTurnOutcome{
				TerminalStatus:  apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR,
				ErrorMessage:    "ntfy unreachable",
				DurationSeconds: 1,
			}
			job = server.NewAgentTurnJob(store, dispatcher, "p", "s1", 5*time.Second)
			Expect(job.Execute()).To(Succeed())
		})

		It("should record the error message", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastErrorMessage()).To(Equal("ntfy unreachable"))
		})

		It("should record terminal ERROR status", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR))
		})
	})

	Describe("Execute when Dispatch fails (no subscribers)", func() {
		BeforeEach(func() {
			dispatcher.dispatchErr = errors.New("no scheduled-turn subscribers connected")
			job = server.NewAgentTurnJob(store, dispatcher, "p", "s1", 5*time.Second)
			Expect(job.Execute()).To(Succeed())
		})

		It("should record terminal ERROR status without going through RUNNING", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR))
		})

		It("should include the dispatch failure in last_error_message", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastErrorMessage()).To(ContainSubstring("subscribers"))
		})
	})

	Describe("Execute when the schedule is missing", func() {
		BeforeEach(func() {
			job = server.NewAgentTurnJob(store, dispatcher, "p", "missing", 5*time.Second)
		})

		It("should not dispatch", func() {
			_ = job.Execute()
			Expect(dispatcher.dispatched).To(BeEmpty())
		})

		It("should not return an error from Execute (logged, not panicked)", func() {
			Expect(job.Execute()).To(Succeed())
		})
	})

	Describe("Execute when the schedule is already RUNNING (single-in-flight guard)", func() {
		BeforeEach(func() {
			// Manually drive the schedule into RUNNING the same way a prior
			// fire would have.
			Expect(store.TransitionStatus("p", "s1",
				apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)).To(Succeed())

			job = server.NewAgentTurnJob(store, dispatcher, "p", "s1", 5*time.Second)
			Expect(job.Execute()).To(Succeed())
		})

		It("should not dispatch a second turn", func() {
			Expect(dispatcher.dispatched).To(BeEmpty())
		})

		It("should leave the schedule in RUNNING (no spurious transition)", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING))
		})
	})

	Describe("Execute when hardTimeout is zero", func() {
		// hardTimeout <= 0 takes the indefinite-wait branch in awaitOutcome.
		// The fakeDispatcher writes the outcome and closes the channel, so the
		// blocking <-completion still receives without deadlocking.
		BeforeEach(func() {
			dispatcher.completionToSend = &server.ScheduledTurnOutcome{
				TerminalStatus:  apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				DurationSeconds: 3,
			}
			job = server.NewAgentTurnJob(store, dispatcher, "p", "s1", 0)
			Expect(job.Execute()).To(Succeed())
		})

		It("should still dispatch the request", func() {
			Expect(dispatcher.dispatched).To(HaveLen(1))
		})

		It("should record the terminal OK status", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_OK))
		})

		It("should record the duration from the outcome", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastDurationSeconds()).To(Equal(int32(3)))
		})
	})

	Describe("Execute when the hard timeout elapses", func() {
		// The dispatcher returns a channel that never completes. Wrapping the
		// Execute call in a select with a 1s safety net guarantees the test
		// itself doesn't hang if the timeout branch regresses.
		var executeErr error

		BeforeEach(func() {
			dispatcher.neverComplete = true
			job = server.NewAgentTurnJob(store, dispatcher, "p", "s1", 50*time.Millisecond)

			done := make(chan error, 1)
			go func() {
				done <- job.Execute()
			}()
			select {
			case executeErr = <-done:
			case <-time.After(time.Second):
				Fail("Execute did not return within 1s; awaitOutcome timeout branch is likely broken")
			}
		})

		It("should still return nil from Execute (errors are recorded on the schedule)", func() {
			Expect(executeErr).NotTo(HaveOccurred())
		})

		It("should record terminal ERROR status", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR))
		})

		It("should mention 'scheduled turn timed out' in last_error_message", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastErrorMessage()).To(ContainSubstring("scheduled turn timed out"))
		})
	})

	Describe("Execute when the page store fails to read", func() {
		// Replace the underlying page store with one that errors on
		// ReadFrontMatter so the List call inside Execute fails.
		var brokenJob *server.AgentTurnJob

		BeforeEach(func() {
			errStore := &errorPageStore{readErr: errors.New("disk gone")}
			brokenStore := server.NewAgentScheduleStore(errStore)
			brokenJob = server.NewAgentTurnJob(brokenStore, dispatcher, "p", "s1", 5*time.Second)
		})

		It("should not return an error from Execute (logged, not propagated)", func() {
			Expect(brokenJob.Execute()).To(Succeed())
		})

		It("should not dispatch", func() {
			_ = brokenJob.Execute()
			Expect(dispatcher.dispatched).To(BeEmpty())
		})
	})
})
