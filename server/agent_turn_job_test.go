//revive:disable:dot-imports
package server_test

import (
	"errors"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// seedRunningWithAge drives the schedule identified by (page, scheduleID) into
// RUNNING status and then backdates its last_run by the given age. This
// simulates a zombie RUNNING schedule — one where the process died mid-run and
// no terminal transition was ever recorded.
//
// The on-disk representation uses protojson with UseProtoNames=true, so the
// timestamp sits under the key "last_run" and is serialized as an RFC3339Nano
// string (e.g. "2026-06-13T10:00:00Z"). scheduleFromMap round-trips through
// protojson.Unmarshal, which accepts both RFC3339 and RFC3339Nano.
func seedRunningWithAge(store *server.AgentScheduleStore, pages *fakePageStore, page, scheduleID string, age time.Duration) {
	// First, transition the schedule to RUNNING so the status fields are stamped.
	err := store.TransitionStatus(page, scheduleID, apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)
	ExpectWithOffset(1, err).To(Succeed(), fmt.Sprintf("TransitionStatus to RUNNING failed for %s/%s", page, scheduleID))

	// Now reach into the raw frontmatter and backdate last_run.
	_, fm, readErr := pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	ExpectWithOffset(1, readErr).To(Succeed())

	agent, ok := fm["agent"].(map[string]any)
	ExpectWithOffset(1, ok).To(BeTrue(), "agent namespace missing from frontmatter after RUNNING transition")

	schedulesList, ok := agent["schedules"].([]any)
	ExpectWithOffset(1, ok).To(BeTrue(), "agent.schedules missing from frontmatter after RUNNING transition")

	var targetSchedule map[string]any

	for _, item := range schedulesList {
		sc, ok := item.(map[string]any)
		if !ok {
			continue
		}

		if sc["id"] == scheduleID {
			targetSchedule = sc
			break
		}
	}

	ExpectWithOffset(1, targetSchedule).NotTo(BeNil(), fmt.Sprintf("schedule %q not found in frontmatter", scheduleID))

	// Overwrite last_run with a backdated RFC3339Nano string.
	backdatedTime := time.Now().UTC().Add(-age)
	targetSchedule["last_run"] = backdatedTime.Format(time.RFC3339Nano)

	writeErr := pages.WriteFrontMatter(wikipage.PageIdentifier(page), fm, wikipage.AnonymousIdentity)
	ExpectWithOffset(1, writeErr).To(Succeed())
}

// fakeDispatcher records Dispatch calls and lets tests inject completion
// outcomes synchronously.
type fakeDispatcher struct {
	mu               sync.Mutex
	dispatched       []*apiv1.ScheduledTurnRequest
	dispatchErr      error
	completionToSend *server.ScheduledTurnOutcome
	dispatchFn       func(*apiv1.ScheduledTurnRequest) (<-chan *server.ScheduledTurnOutcome, error)
	// neverComplete, when true, returns a channel that never receives and is
	// never closed — used to exercise the hard-timeout path in awaitOutcome.
	neverComplete bool
}

func (f *fakeDispatcher) Dispatch(req *apiv1.ScheduledTurnRequest) (<-chan *server.ScheduledTurnOutcome, error) {
	if f.dispatchFn != nil {
		return f.dispatchFn(req)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dispatchErr != nil {
		return nil, f.dispatchErr
	}
	f.recordLocked(req)
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

func (f *fakeDispatcher) record(req *apiv1.ScheduledTurnRequest) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordLocked(req)
}

func (f *fakeDispatcher) recordLocked(req *apiv1.ScheduledTurnRequest) {
	f.dispatched = append(f.dispatched, req)
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
			Id:           "s1",
			Cron:         "0 * * * * *",
			Prompt:       "do thing",
			MaxTurns:     10,
			Enabled:      true,
			AllowedTools: []string{"Bash(mkdir:*)"},
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
			Expect(req.GetAllowedTools()).To(ConsistOf("Bash(mkdir:*)"))
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

		It("should record terminal TIMEOUT status", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_TIMEOUT))
		})

		It("should mention 'scheduled turn timed out' in last_error_message", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastErrorMessage()).To(ContainSubstring("scheduled turn timed out"))
		})

		It("should record the timeout wait duration", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastDurationSeconds()).To(Equal(int32(1)))
		})
	})

	Describe("Execute when the hard timeout wins before a late OK completion", func() {
		var (
			completion chan *server.ScheduledTurnOutcome
			executeErr error
		)

		BeforeEach(func() {
			completion = make(chan *server.ScheduledTurnOutcome, 1)
			dispatcher.dispatchFn = func(req *apiv1.ScheduledTurnRequest) (<-chan *server.ScheduledTurnOutcome, error) {
				dispatcher.record(req)
				return completion, nil
			}
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

			completion <- &server.ScheduledTurnOutcome{
				TerminalStatus:  apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				DurationSeconds: 900,
			}
		})

		It("should still return nil from Execute", func() {
			Expect(executeErr).NotTo(HaveOccurred())
		})

		It("should keep the timed-out run authoritative", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_TIMEOUT))
		})

		It("should record the timeout duration instead of the late completion duration", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastDurationSeconds()).To(Equal(int32(1)))
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

	Describe("Execute when the schedule is RUNNING but stale", func() {
		// Seed a RUNNING schedule that has been running for 1 hour. With a
		// hardTimeout of 5s the reclaim threshold is 10s — so 1 hour is well
		// past the threshold and the schedule should be treated as a zombie.
		BeforeEach(func() {
			seedRunningWithAge(store, pages, "p", "s1", time.Hour)

			dispatcher.completionToSend = &server.ScheduledTurnOutcome{
				TerminalStatus:  apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				DurationSeconds: 4,
			}
			job = server.NewAgentTurnJob(store, dispatcher, "p", "s1", 5*time.Second)

			Expect(job.Execute()).To(Succeed())
		})

		It("should dispatch one request (zombie was reclaimed, fresh run dispatched)", func() {
			Expect(dispatcher.dispatched).To(HaveLen(1))
		})

		It("should record terminal OK status after the fresh run completes", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_OK))
		})
	})

	Describe("Execute when the pool reports MAX_TURNS", func() {
		BeforeEach(func() {
			dispatcher.completionToSend = &server.ScheduledTurnOutcome{
				TerminalStatus:  apiv1.ScheduleStatus_SCHEDULE_STATUS_MAX_TURNS,
				ErrorMessage:    "max_turns (10) reached after 42s",
				DurationSeconds: 42,
			}
			job = server.NewAgentTurnJob(store, dispatcher, "p", "s1", 5*time.Second)
			Expect(job.Execute()).To(Succeed())
		})

		It("should record terminal MAX_TURNS status", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_MAX_TURNS))
		})

		It("should record the error message from the pool", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastErrorMessage()).To(ContainSubstring("max_turns"))
		})

		It("should record the actual duration from the pool", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastDurationSeconds()).To(Equal(int32(42)))
		})
	})

	Describe("Execute when a late MAX_TURNS completion arrives after the hard timeout", func() {
		var (
			completion chan *server.ScheduledTurnOutcome
			executeErr error
		)

		BeforeEach(func() {
			completion = make(chan *server.ScheduledTurnOutcome, 1)
			dispatcher.dispatchFn = func(req *apiv1.ScheduledTurnRequest) (<-chan *server.ScheduledTurnOutcome, error) {
				dispatcher.record(req)
				return completion, nil
			}
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

			// Send the late MAX_TURNS completion after Execute has returned.
			completion <- &server.ScheduledTurnOutcome{
				TerminalStatus:  apiv1.ScheduleStatus_SCHEDULE_STATUS_MAX_TURNS,
				ErrorMessage:    "max_turns (10) reached after 1800s",
				DurationSeconds: 1800,
			}
		})

		It("should return nil from Execute", func() {
			Expect(executeErr).NotTo(HaveOccurred())
		})

		It("should eventually override provisional TIMEOUT with MAX_TURNS", func() {
			Eventually(func() apiv1.ScheduleStatus {
				schedules, _ := store.List("p")
				return schedules[0].GetLastStatus()
			}, 5*time.Second, 20*time.Millisecond).Should(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_MAX_TURNS))
		})

		It("should record the actual duration from the late MAX_TURNS completion", func() {
			Eventually(func() int32 {
				schedules, _ := store.List("p")
				return schedules[0].GetLastDurationSeconds()
			}, 5*time.Second, 20*time.Millisecond).Should(Equal(int32(1800)))
		})

		It("should record the error message from the late MAX_TURNS completion", func() {
			Eventually(func() string {
				schedules, _ := store.List("p")
				return schedules[0].GetLastErrorMessage()
			}, 5*time.Second, 20*time.Millisecond).Should(ContainSubstring("max_turns"))
		})
	})

	Describe("Execute when the hard timeout fires and no late completion arrives", func() {
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
				Fail("Execute did not return within 1s")
			}
			// Allow the late-completion goroutine to run but nothing will arrive.
			time.Sleep(100 * time.Millisecond)
		})

		It("should still return nil from Execute", func() {
			Expect(executeErr).NotTo(HaveOccurred())
		})

		It("should keep the TIMEOUT status when nothing arrives", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_TIMEOUT))
		})
	})

	Describe("Execute when a late MAX_TURNS arrives after a new run has started", func() {
		// Scenario: run A times out, run B starts (RUNNING), then run A's late
		// MAX_TURNS completion arrives. The late completion must NOT overwrite
		// run B's RUNNING status.
		var (
			completion chan *server.ScheduledTurnOutcome
			executeErr error
		)

		BeforeEach(func() {
			completion = make(chan *server.ScheduledTurnOutcome, 1)
			dispatcher.dispatchFn = func(req *apiv1.ScheduledTurnRequest) (<-chan *server.ScheduledTurnOutcome, error) {
				dispatcher.record(req)
				return completion, nil
			}
			job = server.NewAgentTurnJob(store, dispatcher, "p", "s1", 50*time.Millisecond)

			done := make(chan error, 1)
			go func() {
				done <- job.Execute()
			}()
			select {
			case executeErr = <-done:
			case <-time.After(time.Second):
				Fail("Execute did not return within 1s")
			}

			// Simulate run B starting: TIMEOUT→RUNNING is legal.
			Expect(store.TransitionStatus("p", "s1",
				apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)).To(Succeed())

			// Now send run A's late MAX_TURNS completion.
			completion <- &server.ScheduledTurnOutcome{
				TerminalStatus:  apiv1.ScheduleStatus_SCHEDULE_STATUS_MAX_TURNS,
				ErrorMessage:    "max_turns (10) reached after 1800s",
				DurationSeconds: 1800,
			}
			// Allow the late-completion goroutine to process.
			time.Sleep(100 * time.Millisecond)
		})

		It("should return nil from Execute", func() {
			Expect(executeErr).NotTo(HaveOccurred())
		})

		It("should not overwrite run B's RUNNING status with stale MAX_TURNS", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING))
		})
	})

	Describe("Execute when the completion channel is closed without a value after hard timeout", func() {
		// Scenario: hard timeout fires, then the pool closes the completion
		// channel without sending an outcome (nil receive). The late-completion
		// goroutine should treat nil as a non-MAX_TURNS signal and leave the
		// provisional TIMEOUT status intact.
		var (
			completion chan *server.ScheduledTurnOutcome
			executeErr error
		)

		BeforeEach(func() {
			completion = make(chan *server.ScheduledTurnOutcome)
			dispatcher.dispatchFn = func(req *apiv1.ScheduledTurnRequest) (<-chan *server.ScheduledTurnOutcome, error) {
				dispatcher.record(req)
				return completion, nil
			}
			job = server.NewAgentTurnJob(store, dispatcher, "p", "s1", 50*time.Millisecond)

			done := make(chan error, 1)
			go func() {
				done <- job.Execute()
			}()
			select {
			case executeErr = <-done:
			case <-time.After(time.Second):
				Fail("Execute did not return within 1s")
			}

			// Close the channel without sending a value — nil outcome.
			close(completion)
			// Allow the late-completion goroutine to process.
			time.Sleep(100 * time.Millisecond)
		})

		It("should return nil from Execute", func() {
			Expect(executeErr).NotTo(HaveOccurred())
		})

		It("should keep the TIMEOUT status when a nil outcome arrives", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_TIMEOUT))
		})
	})

	Describe("Execute when the schedule is RUNNING and fresh", func() {
		// Seed a RUNNING schedule that just started (last_run ≈ now). With a
		// hardTimeout of 5s the reclaim threshold is 10s — so a fresh run should
		// NOT be reclaimed.
		BeforeEach(func() {
			// TransitionStatus to RUNNING without backdating — last_run is "now".
			Expect(store.TransitionStatus("p", "s1",
				apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)).To(Succeed())

			job = server.NewAgentTurnJob(store, dispatcher, "p", "s1", 5*time.Second)

			Expect(job.Execute()).To(Succeed())
		})

		It("should not dispatch a second turn (genuine in-flight)", func() {
			Expect(dispatcher.dispatched).To(BeEmpty())
		})

		It("should leave the schedule in RUNNING (no spurious transition)", func() {
			schedules, _ := store.List("p")
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING))
		})
	})
})
